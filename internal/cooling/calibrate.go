package cooling

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Calibration finds the two duties a fan needs to be stopped and started by its
// duty: the lowest one at which it still turns (Stop) and the lowest one that
// gets it going again from standstill (Start). A pump has no standstill; there
// only the lowest stable duty is searched (PumpMeasurement).
const (
	// CalibrationHold is how long a standstill has to last, and how long a
	// restarted fan has to go on turning. Fans on the controller are slow, and a
	// short start-up pulse is no start.
	CalibrationHold = 30 * time.Second
	// calibrationPoll is the time between two readings in the phases at standstill
	// and restart. A stop is sent again at every reading: the controller starts a
	// stopped fan by itself after 7 to 10 seconds otherwise.
	calibrationPoll = time.Second
	// calibrationStartLimit is the duty up to which a restart is tried. A fan that
	// does not turn at half speed is blocked, not slow.
	calibrationStartLimit = 50

	// Margins of the suggestions made from a calibration.
	SuggestStopMargin  = 2 // stop_at = stop + this
	SuggestStartMargin = 3 // start_duty = start + this
	SuggestMinMargin   = 3 // min_duty of a fan = stop + this
	SuggestPumpMargin  = 5 // min_duty of a pump = lowest stable + this

	// SuggestStopCeiling is the highest stop_at that is suggested. A fan that only
	// runs stably from a higher duty would stand at every curve value below it, and
	// the suggestion would switch it off over most of its range: that is left to
	// the user, who gets the minimum only.
	SuggestStopCeiling = 40
)

// Phase is the part of a calibration that is running.
type Phase string

// The phases of a fan calibration, and the one of a pump.
const (
	PhaseDown  Phase = "down"  // stepping down from full speed
	PhaseStop  Phase = "stop"  // standstill is held
	PhaseStart Phase = "start" // restart from standstill, upwards
	PhasePump  Phase = "pump"
)

// StopState says whether a fan can be stopped by its duty.
type StopState string

// The results of the standstill phase.
const (
	StopYes    StopState = "yes"    // it stood still for the whole hold time
	StopNever  StopState = "never"  // it kept turning at 0 %
	StopUnheld StopState = "unheld" // it stopped, but started again by itself
)

// Calibration is the result of one fan.
type Calibration struct {
	Stops StopState
	// Stop is the lowest duty with a stable speed above zero on the way down.
	Stop int
	// Starts says whether a restart was found, at the duty Start.
	Starts bool
	Start  int
}

// CalibrationProgress is what a calibration tells with every reading.
type CalibrationProgress struct {
	Phase Phase
	// Step of Steps counts the duties of the phase.
	Step, Steps int
	Duty, RPM   int
	// Held is how long the standstill or the restart has lasted so far.
	Held time.Duration
}

func (m Measurement) update(p CalibrationProgress) {
	if m.Update != nil {
		m.Update(p)
	}
}

// LowestDutyDevice is a MeasureDevice that says the lowest duty above 0 it acts
// on. Smaller duties are raised to it by the device, so they are no steps of
// their own and a calibration leaves them out. Without it the lowest is 1.
type LowestDutyDevice interface {
	LowestDuty() int
}

func lowestOf(d MeasureDevice) int {
	if l, ok := d.(LowestDutyDevice); ok {
		return max(1, l.LowestDuty())
	}
	return 1
}

// calibrationSteps are the duties of the way down: the given steps that the
// device acts on, and every duty below the lowest of them down to its lowest.
func calibrationSteps(steps []int, lowest int) []int {
	out := slices.DeleteFunc(slices.Clone(steps), func(d int) bool { return d < lowest })
	if len(out) == 0 {
		return out
	}
	for d := slices.Min(out) - 1; d >= lowest; d-- {
		out = append(out, d)
	}
	return out
}

// Calibrate measures the fan stepwise downwards, to every percent below the
// lowest step while it turns, then holds standstill and tries to restart it.
// The caller leaves the fan at a safe duty afterwards, as for Run. On an error
// what was measured so far is returned with it.
func (m Measurement) Calibrate(ctx context.Context) ([]MeasuredStep, Calibration, error) {
	var out []MeasuredStep
	var cal Calibration
	if err := m.Device.TakeControl(m.ID); err != nil {
		return out, cal, fmt.Errorf("cooling: taking over %s: %w", m.ID, err)
	}
	floor := lowestOf(m.Device)
	steps := calibrationSteps(m.Steps, floor)
	if len(steps) == 0 {
		return out, cal, errors.New("cooling: no step to measure")
	}
	lowest := 0
	for i, duty := range steps {
		st, err := m.step(ctx, i, len(steps), duty)
		if err != nil {
			return out, cal, err
		}
		out = append(out, st)
		if st.Settled && st.RPM > 0 {
			lowest = duty
		} else if st.Settled {
			break // the steps below are skipped
		}
	}
	if lowest == 0 {
		return out, cal, ErrNoSpeed
	}
	cal.Stop = lowest
	state, err := m.holdStopped(ctx)
	cal.Stops = state
	if err != nil || state != StopYes {
		return out, cal, err
	}
	cal.Start, cal.Starts, err = m.findStart(ctx, floor)
	return out, cal, err
}

// poll waits, checks and reads the speed of the fan.
func (m Measurement) poll(ctx context.Context) (int, error) {
	if err := m.Wait(ctx, calibrationPoll); err != nil {
		return 0, err
	}
	if m.Check != nil {
		if err := m.Check(); err != nil {
			return 0, err
		}
	}
	speeds, err := m.Device.Speeds()
	if err != nil {
		return 0, fmt.Errorf("cooling: reading speeds: %w", err)
	}
	rpm, ok := speeds[m.ID]
	if !ok {
		return 0, fmt.Errorf("cooling: %s is gone", m.ID)
	}
	return rpm, nil
}

// holdStopped sends 0 % every second until the fan has stood still for the
// hold time. A stop sent once does not last on the controller.
func (m Measurement) holdStopped(ctx context.Context) (StopState, error) {
	since := time.Duration(-1)
	sawStill := false
	for elapsed := time.Duration(0); elapsed < measureMaxWait+CalibrationHold; {
		if err := m.Device.SetDuty(m.ID, 0); err != nil {
			return StopNever, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
		}
		rpm, err := m.poll(ctx)
		if err != nil {
			return StopNever, err
		}
		elapsed += calibrationPoll
		if rpm == 0 {
			sawStill = true
			if since < 0 {
				since = elapsed
			}
		} else {
			since = -1
		}
		held := time.Duration(0)
		if since >= 0 {
			held = elapsed - since
		}
		m.update(CalibrationProgress{Phase: PhaseStop, Step: 0, Steps: 1, Duty: 0, RPM: rpm, Held: held})
		if held >= CalibrationHold {
			return StopYes, nil
		}
	}
	if sawStill {
		return StopUnheld, nil
	}
	return StopNever, nil
}

// findStart raises the duty from the lowest one the device acts on in single steps, each held for at least
// the hold time, and returns the first one the fan keeps turning at.
func (m Measurement) findStart(ctx context.Context, from int) (int, bool, error) {
	for duty := from; duty <= calibrationStartLimit; duty++ {
		ok, spinning, err := m.tryStart(ctx, duty)
		if err != nil {
			return 0, false, err
		}
		if ok {
			return duty, true, nil
		}
		if spinning {
			if err := m.stopAgain(ctx); err != nil {
				return 0, false, err
			}
		}
	}
	return 0, false, nil
}

// tryStart holds one duty. The fan has started when it turns without a break
// for the hold time; a twitch that dies out does not count. A fan that turns at
// the end of the hold time gets the time it still needs, up to twice as long.
func (m Measurement) tryStart(ctx context.Context, duty int) (started, spinning bool, err error) {
	if err := m.Device.SetDuty(m.ID, duty); err != nil {
		return false, false, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
	}
	since := time.Duration(-1)
	for elapsed := time.Duration(0); ; {
		rpm, err := m.poll(ctx)
		if err != nil {
			return false, false, err
		}
		elapsed += calibrationPoll
		spinning = rpm > 0
		if spinning {
			if since < 0 {
				since = elapsed
			}
		} else {
			since = -1
		}
		held := time.Duration(0)
		if since >= 0 {
			held = elapsed - since
		}
		m.update(CalibrationProgress{Phase: PhaseStart, Step: duty - 1, Steps: calibrationStartLimit, Duty: duty, RPM: rpm, Held: held})
		switch {
		case held >= CalibrationHold:
			return true, true, nil
		case elapsed >= 2*CalibrationHold, elapsed >= CalibrationHold && since < 0:
			return false, spinning, nil
		}
	}
}

// stopAgain brings a fan that turns on to standstill, so the next duty starts
// from rest again.
func (m Measurement) stopAgain(ctx context.Context) error {
	still := 0
	for elapsed := time.Duration(0); elapsed < measureMaxWait; elapsed += calibrationPoll {
		if err := m.Device.SetDuty(m.ID, 0); err != nil {
			return fmt.Errorf("cooling: setting %s: %w", m.ID, err)
		}
		rpm, err := m.poll(ctx)
		if err != nil {
			return err
		}
		if rpm == 0 {
			still++
		} else {
			still = 0
		}
		if still >= 3 {
			return nil
		}
	}
	return errors.New("cooling: the fan does not stand still again")
}

// Pump measurement. A pump must never stand, so the way down is short, careful
// and ends at the first sign of trouble.
const (
	pumpPoll      = time.Second
	pumpMinWait   = 6 * time.Second
	pumpMaxWait   = 40 * time.Second
	pumpWindow    = 5
	pumpTolerance = 100 // rpm
	// pumpDropMargin: a step may lose this much more speed than its duty fell
	// (as a share of the speed before) before it counts as a collapse.
	pumpDropMargin = 0.15
	pumpRecover    = 15 * time.Second // time to see the pump turn again at full duty
	// PumpStep is the distance between the duties tried.
	PumpStep = 5
)

// PumpResult is the outcome of a pump measurement.
type PumpResult struct {
	Steps []MeasuredStep
	// Minimum is the lowest duty with a stable speed; 0 if none.
	Minimum int
	// AtLimit: it was stable down to the floor, there was no collapse.
	AtLimit bool
	// Failed is the duty at which the speed broke down; 0 if it did not.
	Failed int
}

// PumpMeasurement steps a pump down from full duty to Floor and back to 100 %.
type PumpMeasurement struct {
	Device MeasureDevice
	ID     string
	// Floor is the lowest duty tried. It is never lower than LowestSafeDuty.
	Floor int
	// StoppedBelow is the speed under which the pump counts as stopped (at least 1).
	StoppedBelow int
	Wait         func(context.Context, time.Duration) error
	Check        func() error
	Update       func(CalibrationProgress)
}

// Run measures. Whatever happens, the pump is set to 100 % again before it
// returns; if it does not turn then, the error is ErrPumpStopped and the
// normal alarm of the control takes over.
func (p PumpMeasurement) Run(ctx context.Context) (res PumpResult, err error) {
	if p.Floor < LowestSafeDuty || p.Floor > 100 {
		return res, fmt.Errorf("cooling: a pump is not measured below %d %%", LowestSafeDuty)
	}
	if err := p.Device.TakeControl(p.ID); err != nil {
		return res, fmt.Errorf("cooling: taking over %s: %w", p.ID, err)
	}
	stopped := max(1, p.StoppedBelow)
	restored := false
	defer func() {
		if restored {
			return
		}
		if rerr := p.restore(stopped); rerr != nil && err == nil {
			err = rerr
		}
	}()

	duties := []int{100}
	for d := 100 - PumpStep; d > p.Floor; d -= PumpStep {
		duties = append(duties, d)
	}
	if p.Floor < 100 {
		duties = append(duties, p.Floor)
	}
	prevDuty, prevRPM := 0, 0
	for i, duty := range duties {
		if err := p.Device.SetDuty(p.ID, duty); err != nil {
			return res, fmt.Errorf("cooling: setting %s: %w", p.ID, err)
		}
		rpm, collapsed, err := p.settle(ctx, i, len(duties), duty, stopped)
		if err != nil {
			return res, err
		}
		if !collapsed && prevRPM > 0 {
			expected := float64(prevRPM) * (float64(duty)/float64(prevDuty) - pumpDropMargin)
			collapsed = float64(rpm) < expected
		}
		res.Steps = append(res.Steps, MeasuredStep{Duty: duty, RPM: rpm, Settled: !collapsed})
		if collapsed {
			res.Failed = duty
			break
		}
		res.Minimum, prevDuty, prevRPM = duty, duty, rpm
	}
	res.AtLimit = res.Failed == 0 && res.Minimum == p.Floor
	restored = true
	if rerr := p.restore(stopped); rerr != nil {
		return res, rerr
	}
	if res.Minimum == 0 {
		return res, errors.New("cooling: the pump is not stable at full duty")
	}
	return res, nil
}

// settle waits until the speed of a step is stable. It returns early with
// collapsed set when the pump drops under the stopped speed or never settles.
func (p PumpMeasurement) settle(ctx context.Context, index, total, duty, stopped int) (rpm int, collapsed bool, err error) {
	var seen []int
	for waited := time.Duration(0); ; {
		if err := p.Wait(ctx, pumpPoll); err != nil {
			return 0, false, err
		}
		waited += pumpPoll
		if p.Check != nil {
			if err := p.Check(); err != nil {
				return 0, false, err
			}
		}
		speeds, err := p.Device.Speeds()
		if err != nil {
			return 0, false, fmt.Errorf("cooling: reading speeds: %w", err)
		}
		rpm, ok := speeds[p.ID]
		if !ok {
			return 0, false, fmt.Errorf("cooling: %s is gone", p.ID)
		}
		if p.Update != nil {
			p.Update(CalibrationProgress{Phase: PhasePump, Step: index, Steps: total, Duty: duty, RPM: rpm})
		}
		if rpm < stopped {
			return rpm, true, nil
		}
		seen = append(seen, rpm)
		if len(seen) > pumpWindow {
			seen = seen[1:]
		}
		if len(seen) == pumpWindow && spread(seen) <= pumpTolerance && waited >= pumpMinWait {
			return median(seen), false, nil
		}
		if waited >= pumpMaxWait {
			return median(seen), true, nil
		}
	}
}

// restore sets full duty and waits for the pump to turn. It does not depend on
// the context of the run: the end of a run must not leave the pump low.
func (p PumpMeasurement) restore(stopped int) error {
	if err := p.Device.SetDuty(p.ID, 100); err != nil {
		return fmt.Errorf("cooling: setting %s: %w", p.ID, err)
	}
	for waited := time.Duration(0); waited < pumpRecover; waited += pumpPoll {
		if err := p.Wait(context.Background(), pumpPoll); err != nil {
			return err
		}
		speeds, err := p.Device.Speeds()
		if err == nil && speeds[p.ID] >= stopped {
			return nil
		}
		if err == nil {
			_ = p.Device.SetDuty(p.ID, 100) // said again: one lost write must not leave it low
		}
	}
	return ErrPumpStopped
}
