package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
)

// Calibration of fans and of the pump. A run takes one output out of the control
// at a time (Controller.Hold) and gives it back at a safe duty; the others go on.
// Nothing found is applied: the page offers it as a suggestion (CoolingSuggestion).

// coolingMeasureCooldown is the pause between two fans of a run over all of them:
// the parts cool down, and the next fan does not start from the heat of the last.
const coolingMeasureCooldown = 30 * time.Second

// pumpMeasureLowest is the lowest duty a pump measurement ever sets: the floor
// of every pump.
const pumpMeasureLowest = cooling.LowestPumpDuty

// Values of coolingCalibration.Kind.
const (
	calibrationFan  = "fan"
	calibrationPump = "pump"
)

// coolingCalibration is what the last calibration of an output found.
type coolingCalibration struct {
	Kind string `json:"kind"`
	At   string `json:"at,omitempty"`
	// Fan: Stops is "yes", "never" or "unheld" (it stopped but started by itself),
	// Stop the lowest duty with a stable speed. Start is the duty that gets it
	// going from standstill if Starts.
	Stops  string `json:"stops,omitempty"`
	Stop   int    `json:"stop,omitempty"`
	Starts bool   `json:"starts,omitempty"`
	Start  int    `json:"start,omitempty"`
	// Pump: the lowest duty with a stable speed, and whether that is only the
	// limit of the measurement because nothing broke down before.
	Minimum int  `json:"minimum,omitempty"`
	AtLimit bool `json:"at_limit,omitempty"`
}

func (c *coolingCalibration) check() error {
	if c == nil {
		return nil
	}
	if c.Kind != calibrationFan && c.Kind != calibrationPump {
		return errors.New("unknown calibration")
	}
	switch c.Stops {
	case "", string(cooling.StopYes), string(cooling.StopNever), string(cooling.StopUnheld):
	default:
		return errors.New("unknown calibration result")
	}
	for _, v := range []int{c.Stop, c.Start, c.Minimum} {
		if v < 0 || v > 100 {
			return errors.New("calibration out of range")
		}
	}
	return nil
}

// CoolingSuggestion is what a calibration suggests for an output, with the
// margins of the cooling package. A value of 0 is no suggestion.
type CoolingSuggestion struct {
	// StopAt and StartDuty are for a fan that may stand still.
	StopAt    int `json:"stopAt,omitempty"`
	StartDuty int `json:"startDuty,omitempty"`
	// MinDuty is for every other output.
	MinDuty int `json:"minDuty,omitempty"`
}

// suggestionOf derives the suggestion from the stored calibration. A case fan
// never gets the one for standing still: it runs in operation, always.
func suggestionOf(o coolingOutputConfig) *CoolingSuggestion {
	c := o.Calibration
	if c == nil {
		return nil
	}
	clamp := func(v int) int { return min(100, max(0, v)) }
	switch {
	case c.Kind == calibrationPump && o.Pump:
		if c.Minimum == 0 {
			return nil
		}
		return &CoolingSuggestion{MinDuty: clamp(max(cooling.LowestPumpDuty, c.Minimum+cooling.SuggestPumpMargin))}
	case c.Kind == calibrationFan && !o.Pump && c.Stops == string(cooling.StopYes) && c.Stop > 0:
		s := &CoolingSuggestion{MinDuty: clamp(c.Stop + cooling.SuggestMinMargin)}
		if stopAt := c.Stop + cooling.SuggestStopMargin; o.Role != coolingRoleCase && c.Starts && stopAt <= cooling.SuggestStopCeiling {
			s.StopAt = clamp(stopAt)
			s.StartDuty = clamp(c.Start + cooling.SuggestStartMargin)
		}
		return s
	}
	return nil
}

// suggestionFor is suggestionOf for an output of cfg. A stop value that the
// configuration would refuse for this output, because its curve never asks for
// more than that, is left out: only the minimum stays.
func (e *Engine) suggestionFor(cfg coolingConfig, o coolingOutputConfig) *CoolingSuggestion {
	s := suggestionOf(o)
	if s == nil || s.StopAt == 0 {
		return s
	}
	try := cfg
	try.Outputs = slices.Clone(cfg.Outputs)
	if i := slices.IndexFunc(try.Outputs, func(x coolingOutputConfig) bool { return x.ID == o.ID }); i >= 0 {
		try.Outputs[i].StopAt, try.Outputs[i].StartDuty, try.Outputs[i].MinDuty = s.StopAt, s.StartDuty, 0
		if _, err := newCoolingPreview(try, e.prov); err != nil {
			s.StopAt, s.StartDuty = 0, 0
		}
	}
	return s
}

// CoolingOutcome is the result of one output of the last run.
type CoolingOutcome struct {
	ID          string              `json:"id"`
	Calibration *coolingCalibration `json:"calibration,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// calOutcome is a finished output of a run, before it is stored.
type calOutcome struct {
	id    string
	steps []cooling.MeasuredStep
	cal   *coolingCalibration
	err   string
	pump  bool
}

// calRig is the hardware a run works on and its pause.
type calRig struct {
	dev  cooling.MeasureDevice
	wait func(context.Context, time.Duration) error
	demo bool
}

// pumpFloorDevice is a pump as the measurement sees it. It refuses a duty
// under the floor of the pump (and so 0), whatever the measurement asks for.
type pumpFloorDevice struct{ cooling.MeasureDevice }

func (d pumpFloorDevice) SetDuty(id string, percent int) error {
	if percent < max(cooling.LowestPumpDuty, pumpMeasureLowest) || percent > 100 {
		return fmt.Errorf("cooling: refusing to set a pump to %d %%", percent)
	}
	return d.MeasureDevice.SetDuty(id, percent)
}

func calOutputByID(cfg coolingConfig, id string) (coolingOutputConfig, bool) {
	i := slices.IndexFunc(cfg.Outputs, func(o coolingOutputConfig) bool { return o.ID == id })
	if i < 0 {
		return coolingOutputConfig{}, false
	}
	return cfg.Outputs[i], true
}

// simulated tells a demo run, whose fans are the simulated ones.
func (e *Engine) simulated(c coolingState) bool { return e.Demo && c.fans == nil }

// measureBlockOf says why an output cannot be measured now, in a word the page
// turns into a sentence. Empty: it can.
func (e *Engine) measureBlockOf(c coolingState, o coolingOutputConfig) string {
	if e.simulated(c) { // the demo can try it in live mode, like the real thing
		if c.cfg.Mode != coolingModeLive {
			return "preview"
		}
		return ""
	}
	if c.fans == nil {
		return "preview"
	}
	if b := c.fans.blocked(); b.reason != "" {
		return "blocked"
	}
	if !o.Pump || tlOutputID.MatchString(o.ID) {
		return ""
	}
	if c.pumps == nil {
		return "preview"
	}
	switch {
	case len(c.cfg.measureSources()) == 0:
		return "noSource"
	case c.pumps.blockedBy().reason != "":
		return "blocked"
	case c.pumps.notReleasedFor(o.ID):
		return "notReleased"
	case c.pumps.isState(o.ID, pumpChecking):
		return "matching"
	case c.pumps.isState(o.ID, pumpNotMatched):
		return "notMatched"
	case !c.pumps.isState(o.ID, pumpMatched):
		return "pending"
	}
	return ""
}

// calibrationRig returns the hardware for output o, or why there is none.
func (e *Engine) calibrationRig(c coolingState, o coolingOutputConfig) (calRig, error) {
	if e.simulated(c) {
		if c.cfg.Mode != coolingModeLive {
			return calRig{}, errors.New("speeds can only be measured while CaseHUD controls the fans")
		}
		d := e.demoRig()
		d.declare(o)
		if o.Pump {
			return calRig{dev: pumpFloorDevice{d}, wait: d.wait, demo: true}, nil
		}
		return calRig{dev: d, wait: d.wait, demo: true}, nil
	}
	if c.fans == nil {
		return calRig{}, errors.New("speeds can only be measured while CaseHUD controls the fans")
	}
	switch e.measureBlockOf(c, o) {
	case "":
	case "blocked":
		return calRig{}, errors.New("another fan program is in charge")
	case "notReleased":
		return calRig{}, fmt.Errorf("the pump %q is not released for CaseHUD", o.ID)
	case "noSource":
		return calRig{}, fmt.Errorf("no temperature source in the configuration, the pump %q is not measured", o.ID)
	default:
		return calRig{}, fmt.Errorf("the pump %q has not been checked yet", o.ID)
	}
	switch {
	case !o.Pump:
		return calRig{dev: c.fans, wait: coolingMeasureWait}, nil
	case tlOutputID.MatchString(o.ID):
		return calRig{dev: pumpFloorDevice{c.fans}, wait: coolingMeasureWait}, nil
	}
	return calRig{dev: pumpFloorDevice{c.pumps}, wait: coolingMeasureWait}, nil
}

// StartCoolingMeasure calibrates one fan in the background.
func (e *Engine) StartCoolingMeasure(id string) error {
	return e.startCalibration([]string{id}, false, false)
}

// StartCoolingMeasureAll calibrates every fan and case fan on a curve, one after
// the other. The pump is not part of it.
func (e *Engine) StartCoolingMeasureAll() error {
	return e.startCalibration(nil, true, false)
}

// StartCoolingPumpMeasure measures the lowest stable duty of a pump.
func (e *Engine) StartCoolingPumpMeasure(id string) error {
	return e.startCalibration([]string{id}, false, true)
}

func (e *Engine) startCalibration(ids []string, all, pump bool) error {
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	if !e.coolingOn() {
		return errCoolingOff
	}
	c := e.coolingSnapshot()
	if c.ctrl == nil || (c.fans == nil && !e.simulated(c)) {
		return errors.New("speeds can only be measured while CaseHUD controls the fans")
	}
	if all {
		for _, o := range c.cfg.Outputs {
			if !o.Pump && o.driven() {
				ids = append(ids, o.ID)
			}
		}
		if len(ids) == 0 {
			return errors.New("no fan on a curve to measure")
		}
	}
	for _, id := range ids {
		o, ok := calOutputByID(c.cfg, id)
		if !ok || o.Pump != pump || !o.driven() {
			return fmt.Errorf("no %s %q", map[bool]string{false: "fan", true: "pump"}[pump], id)
		}
		if _, err := e.calibrationRig(c, o); err != nil {
			return err
		}
	}
	if pump && !e.simulated(c) {
		// A pump that is slowed down while the machine is hot is not started at all.
		switch t, ok := e.hottestTemp(c.cfg.measureSources()); {
		case !ok:
			return errors.New("a temperature sensor of the curves cannot be read, the pump is not measured")
		case t >= cooling.PumpMeasureMaxTemp:
			return fmt.Errorf("it is %.0f °C, too hot to measure the pump (limit %.0f °C)", t, cooling.PumpMeasureMaxTemp)
		}
	}
	rig, _ := e.calibrationRig(c, c.cfg.Outputs[slices.IndexFunc(c.cfg.Outputs, func(o coolingOutputConfig) bool { return o.ID == ids[0] })])
	ctx, cancel := context.WithCancel(c.ctx)
	e.coolMu.Lock()
	if e.coolMeasure.Running {
		e.coolMu.Unlock()
		cancel()
		return ErrCoolingBusy
	}
	v := CoolingMeasureView{ID: ids[0], Running: true, All: all, Kind: calibrationFan, HoldSec: int(cooling.CalibrationHold / time.Second)}
	if pump {
		v.Kind = calibrationPump
	}
	if all {
		v.Queue = slices.Clone(ids)
	}
	e.coolMeasure = v
	e.coolMeasureStopped, e.coolMeasureKeep = false, false
	e.coolMeasureCancel = cancel
	done := make(chan struct{})
	e.coolMeasureDone = done
	e.coolMu.Unlock()
	go e.runCalibration(ctx, cancel, c, rig, ids, pump, done)
	return nil
}

// noteProgress puts the latest reading into the view.
func (e *Engine) noteProgress(p cooling.CalibrationProgress) {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	v := &e.coolMeasure
	v.Phase, v.Step, v.Steps, v.Duty, v.RPM = string(p.Phase), p.Step, p.Steps, p.Duty, p.RPM
	v.HeldSec = int(p.Held / time.Second)
	if p.Phase == cooling.PhaseDown || p.Phase == cooling.PhasePump {
		// The way down shows its steps as they come; the last one is still moving.
		// At the end of the fan the settled steps replace them.
		if n := len(v.Results); n > 0 && v.Results[n-1].Duty == p.Duty {
			v.Results[n-1].RPM = p.RPM
		} else {
			v.Results = append(v.Results, cooling.MeasuredStep{Duty: p.Duty, RPM: p.RPM})
		}
	}
}

func (e *Engine) runCalibration(ctx context.Context, cancel context.CancelFunc, c coolingState, rig calRig, ids []string, pump bool, done chan struct{}) {
	defer cancel()
	var outs []calOutcome
	var runErr error
	for n, id := range ids {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			break
		}
		out, _ := calOutputByID(c.cfg, id)
		e.coolMu.Lock()
		e.coolMeasure.ID, e.coolMeasure.Index = id, n
		e.coolMeasure.Phase, e.coolMeasure.HeldSec, e.coolMeasure.Step, e.coolMeasure.Results = "", 0, 0, nil
		e.coolMu.Unlock()
		if n > 0 {
			// The fan before is back in the control. Cooling down is part of the run,
			// and so is the cancel; the limit counts before the next fan, too.
			if err := rig.wait(ctx, coolingMeasureCooldown); err != nil {
				runErr = err
				break
			}
			if !rig.demo {
				if t, ok := e.hottestTemp(c.cfg.measureSources()); ok && t >= cooling.MeasureMaxTemp {
					runErr = fmt.Errorf("it is %.0f °C, too hot to measure the next fan", t)
					break
				}
			}
		}
		c.ctrl.Hold(id)

		o, err := e.calibrateOne(ctx, c, rig, out, pump)
		// Back to a safe speed, whatever happened; the control takes it from there.
		safe := out.SafeDuty
		if pump {
			safe = 100
		}
		standing := safe
		if werr := rig.dev.SetDuty(id, safe); werr != nil {
			log.Printf("cooling: %s after measuring: %v", id, werr)
			standing = 0 // unknown: the control then lets the output down by its steps only
		}
		// The output stands at the safe duty; the control ramps from there, so a
		// pump or fan never jumps to the value the curve moved to meanwhile.
		c.ctrl.Release(id, standing)

		e.coolMu.Lock()
		e.coolMeasure.Results = o.steps
		e.coolMu.Unlock()
		if err == nil {
			outs = append(outs, o)
			continue
		}
		if errors.Is(err, cooling.ErrNoSpeed) && len(ids) > 1 {
			outs = append(outs, calOutcome{id: id, err: shortCoolingError(err)})
			continue
		}
		runErr = err
		break
	}
	close(done) // before saving: saving replaces the loop, and that loop waits for this

	// Ended by the user without keeping: nothing of this run is stored.
	e.coolMu.Lock()
	stopped, keep := e.coolMeasureStopped, e.coolMeasureKeep
	e.coolMu.Unlock()
	if stopped && !keep {
		outs = nil
	}
	var saveErr error
	if len(outs) > 0 {
		saveErr = e.saveCalibrations(outs)
	}
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	e.coolMeasure.Running, e.coolMeasureCancel = false, nil
	for _, o := range outs {
		e.coolMeasure.Outcomes = append(e.coolMeasure.Outcomes, CoolingOutcome{ID: o.id, Calibration: o.cal, Error: o.err})
	}
	switch {
	case errors.Is(runErr, context.Canceled), stopped && runErr == nil:
		e.coolMeasure.Canceled = true
	case runErr != nil:
		e.coolMeasure.Error = shortCoolingError(runErr)
	case saveErr != nil:
		e.coolMeasure.Error = shortCoolingError(saveErr)
	}
}

func (e *Engine) calibrateOne(ctx context.Context, c coolingState, rig calRig, out coolingOutputConfig, pump bool) (calOutcome, error) {
	o := calOutcome{id: out.ID, pump: pump}
	now := time.Now().UTC().Format(time.RFC3339)
	check := e.calibrationCheck(c, out.ID, pump, rig.demo)
	if pump {
		res, err := cooling.PumpMeasurement{
			Device: rig.dev, ID: out.ID, Floor: pumpMeasureLowest, StoppedBelow: out.StoppedBelowRPM,
			Wait: rig.wait, Check: check, Update: e.noteProgress,
		}.Run(ctx)
		o.steps = res.Steps
		if err != nil {
			return o, err
		}
		o.cal = &coolingCalibration{Kind: calibrationPump, At: now, Minimum: res.Minimum, AtLimit: res.AtLimit}
		return o, nil
	}
	steps, cal, err := cooling.Measurement{
		Device: rig.dev, ID: out.ID, Steps: cooling.MeasureSteps, Wait: rig.wait, Check: check, Update: e.noteProgress,
	}.Calibrate(ctx)
	o.steps = steps
	if err != nil {
		return o, err
	}
	o.cal = &coolingCalibration{Kind: calibrationFan, At: now, Stops: string(cal.Stops), Stop: cal.Stop, Starts: cal.Starts, Start: cal.Start}
	return o, nil
}

// calibrationCheck ends a run when the fan is needed for cooling after all: a
// pump that stands, a temperature that climbs, a sensor that is lost. The
// readings of a demo are made up, so only the pump counts there.
func (e *Engine) calibrationCheck(c coolingState, id string, pump, demo bool) func() error {
	sources := c.cfg.measureSources()
	check := coolingMeasureCheck(c.ctrl, id, pump, func() (float64, bool) { return e.hottestTemp(sources) })
	if !demo {
		return check
	}
	return func() error {
		for _, s := range c.ctrl.Status() {
			if s.PumpDown {
				return cooling.ErrPumpStopped
			}
		}
		return nil
	}
}

// saveCalibrations stores what the finished outputs found: the calibration, and
// for a fan the speed table if it is usable.
func (e *Engine) saveCalibrations(outs []calOutcome) error {
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	cfg := e.coolingSnapshot().cfg
	cfg.Outputs = slices.Clone(cfg.Outputs)
	bare := cfg // the same with the calibrations only, if a new speed table is not accepted
	bare.Outputs = slices.Clone(cfg.Outputs)
	var mapErr error
	for i := range outs {
		o := &outs[i]
		if o.cal == nil {
			continue
		}
		k := slices.IndexFunc(cfg.Outputs, func(x coolingOutputConfig) bool { return x.ID == o.id })
		if k < 0 {
			continue
		}
		cfg.Outputs[k].Calibration = o.cal
		bare.Outputs[k].Calibration = o.cal
		if o.pump {
			continue
		}
		m, err := cooling.SpeedMapFrom(o.steps)
		if err != nil {
			o.err = "speed table not taken: " + shortCoolingError(err)
			mapErr = err
			continue
		}
		cfg.Outputs[k] = withMeasuredSpeeds(cfg.Outputs[k], m)
	}
	if err := e.storeCooling(cfg); err != nil {
		// A new table can make a manual speed or a minimum unusable. The calibration
		// is kept without it, and the run says so.
		if berr := e.storeCooling(bare); berr != nil {
			return berr
		}
		for i := range outs {
			if outs[i].cal != nil && !outs[i].pump && outs[i].err == "" {
				outs[i].err = "speed table not taken: " + shortCoolingError(err)
			}
		}
		if len(outs) == 1 {
			return err
		}
		return nil
	}
	if len(outs) == 1 {
		return mapErr
	}
	return nil
}
