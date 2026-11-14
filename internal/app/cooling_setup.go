package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
	"github.com/kewe87/casehud/internal/sysperm"
)

const coolingFansTTL = 30 * time.Second

// CoolingSensor is one reading offered in the setup.
type CoolingSensor struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// CoolingSetup is what the setup page works with.
type CoolingSetup struct {
	Config coolingConfig   `json:"config"`
	Temps  []CoolingSensor `json:"temps"`
	RPMs   []CoolingSensor `json:"rpms"`
	Fans   []string        `json:"fans"`
	// Controls are the control channels of the mainboard with the duty they have now.
	Controls []CoolingControl `json:"controls"`
	// FansHeld means the controller was left alone, so Fans may be out of date.
	FansHeld bool `json:"fansHeld"`
	// Revision is what a save has to send back, see ErrCoolingRevision.
	Revision int64              `json:"revision"`
	Presets  CoolingPresetsView `json:"presets"`
	// Suggestions are what the last calibration of an output suggests, by output.
	Suggestions map[string]CoolingSuggestion `json:"suggestions"`
}

// CoolingControl is one control channel of a mainboard header. Duty is nil while
// it is unknown.
type CoolingControl struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Duty *int   `json:"duty"`
}

// CoolingMeasureView is the state of the last speed measurement.
type CoolingMeasureView struct {
	ID      string                 `json:"id"`
	Running bool                   `json:"running"`
	Step    int                    `json:"step"`
	Steps   int                    `json:"steps"`
	Duty    int                    `json:"duty"`
	RPM     int                    `json:"rpm"`
	Results []cooling.MeasuredStep `json:"results,omitempty"`
	Error   string                 `json:"error,omitempty"`
	// Kind is "fan" or "pump". Phase is "down", "stop", "start" or "pump".
	Kind  string `json:"kind,omitempty"`
	Phase string `json:"phase,omitempty"`
	// HeldSec of HoldSec: how long the standstill or the restart has lasted.
	HeldSec int `json:"heldSec,omitempty"`
	HoldSec int `json:"holdSec,omitempty"`
	// All marks a run over all fans; Queue lists them and Index is the one running.
	All   bool     `json:"all,omitempty"`
	Queue []string `json:"queue,omitempty"`
	Index int      `json:"index,omitempty"`
	// Canceled: the run was ended by the user. Nothing of it is stored unless the
	// user asked to keep the finished fans; Outcomes lists what was kept.
	Canceled bool `json:"canceled,omitempty"`
	// Outcomes are the results of the finished fans of the last run.
	Outcomes []CoolingOutcome `json:"outcomes,omitempty"`
}

// coolingMeasureWait is the pause of a measurement (tests replace it).
var coolingMeasureWait = cooling.WaitContext

// coolingListFans is the fan list of the controller; only reads (tests replace it).
var coolingListFans = func(ctx context.Context) ([]string, error) {
	d, err := lianlitl.OpenFans(ctx)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.FanIDs(), nil
}

// coolingFansAllowed says whether the fan list may be read from the controller:
// under the same conditions as the live control, since asking is talking to it.
var coolingFansAllowed = func(e *Engine) bool {
	if !e.outputAllowed() || !e.coolingOn() {
		return false
	}
	reason, _ := checkRivals(e.conflictSystem())
	return reason == ""
}

// CoolingSetup returns the configuration and everything that can be chosen.
func (e *Engine) CoolingSetup() CoolingSetup {
	rev := e.coolingRevision() // before the snapshot: at worst too old, never ahead of the configuration
	c := e.coolingSnapshot()
	fans, held := e.coolingFans()
	set := CoolingSetup{Config: c.cfg, Temps: []CoolingSensor{}, RPMs: []CoolingSensor{}, Fans: fans, FansHeld: held, Controls: e.coolingControls(),
		Revision: rev, Presets: e.coolingPresetsOf(c), Suggestions: map[string]CoolingSuggestion{}}
	for _, o := range c.cfg.Outputs {
		if s := e.suggestionFor(c.cfg, o); s != nil {
			set.Suggestions[o.ID] = *s
		}
	}
	if set.Config.Outputs == nil {
		set.Config.Outputs = []coolingOutputConfig{}
	}
	if set.Config.Curves == nil {
		set.Config.Curves = []coolingCurveConfig{}
	}
	for _, r := range e.prov.All() {
		if !finite(r.Value) {
			continue
		}
		s := CoolingSensor{Key: r.Key, Name: r.Path, Value: r.Value}
		switch r.Unit {
		case "°C":
			set.Temps = append(set.Temps, s)
		case "RPM":
			set.RPMs = append(set.RPMs, s)
		}
	}
	byName := func(l []CoolingSensor) {
		sort.Slice(l, func(i, j int) bool { return l[i].Name < l[j].Name })
	}
	byName(set.Temps)
	byName(set.RPMs)
	return set
}

// coolingControls lists the control channels a pump may be set on.
func (e *Engine) coolingControls() []CoolingControl {
	out := []CoolingControl{}
	if e.Demo {
		for i, d := range []int{35, 40, 55, 70, 100} {
			out = append(out, CoolingControl{Key: fmt.Sprintf("/lpc/chip/0/control/%d", i+1), Name: fmt.Sprintf("Demo/Control #%d", i+1), Duty: &d})
		}
		return out
	}
	for _, r := range e.prov.All() {
		if r.Unit != "%" || !sysperm.ValidCoolingHeader(r.Key) {
			continue
		}
		c := CoolingControl{Key: r.Key, Name: r.Path}
		if finite(r.Value) && r.Value >= 0 && r.Value <= 100 {
			d := int(math.Round(r.Value))
			c.Duty = &d
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// coolingFans returns the fans of the controller. While the controller must be
// left alone it returns the last known list and says so.
func (e *Engine) coolingFans() (ids []string, held bool) {
	if e.Demo {
		return []string{"port1-fan1", "port1-fan2", "port1-fan3", "port2-fan1", "port2-fan2", "port2-fan3", "port3-fan1"}, false
	}
	e.coolFansMu.Lock()
	defer e.coolFansMu.Unlock()
	if !e.coolFansAt.IsZero() && time.Since(e.coolFansAt) < coolingFansTTL {
		return append([]string{}, e.coolFansList...), false
	}
	if !coolingFansAllowed(e) {
		return append([]string{}, e.coolFansList...), true
	}
	ids, err := coolingListFans(e.ctx)
	if err != nil {
		log.Printf("cooling: reading the fan list: %v", err)
		ids = nil
	}
	e.coolFansAt, e.coolFansList = time.Now(), ids
	return append([]string{}, ids...), false
}

// SetCoolingConfigJSON checks a new configuration, stores it and applies it.
// Nothing is stored or changed unless all of it is valid.
func (e *Engine) SetCoolingConfigJSON(b []byte) error {
	return e.SetCoolingConfigAt(b, 0)
}

// SetCoolingConfigAt is SetCoolingConfigJSON for a page that worked on the
// configuration it got with revision rev (0: unknown, not checked). A
// configuration stored since then, by a preset or a mode switch, is not
// overwritten: the answer is ErrCoolingRevision.
func (e *Engine) SetCoolingConfigAt(b []byte, rev int64) error {
	_, err := e.SetCoolingConfigRev(b, rev)
	return err
}

// SetCoolingConfigRev is SetCoolingConfigAt that also returns the revision of
// the configuration it stored, so a client can go on from it.
func (e *Engine) SetCoolingConfigRev(b []byte, rev int64) (int64, error) {
	cfg, err := parseCoolingConfig(b)
	if err != nil {
		return 0, err
	}
	if !e.coolingOn() {
		return 0, errCoolingOff
	}
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	if rev != 0 && rev != e.coolingRevision() {
		return 0, ErrCoolingRevision
	}
	if e.coolingMeasuring() {
		return 0, ErrCoolingBusy
	}
	for _, o := range cfg.Outputs {
		if o.ControlKey != "" && e.channelMissing(o.ControlKey) {
			return 0, fmt.Errorf("%q: the board has no control channel %s", o.ID, o.ControlKey)
		}
	}
	if err := e.storeCooling(cfg); err != nil {
		return 0, err
	}
	return e.coolingRevision(), nil
}

// storeCooling needs coolOps.
func (e *Engine) storeCooling(cfg coolingConfig) error {
	if err := cfg.check(); err != nil { // whatever is stored must load again
		return fmt.Errorf("%s: %w", coolingFile, err)
	}
	if len(cfg.regulated()) > 0 {
		if _, err := newCoolingPreview(cfg, e.prov); err != nil {
			return fmt.Errorf("%s: %w", coolingFile, err)
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := e.saveCoolingFile(data); err != nil {
		return withoutPath(err)
	}
	e.replaceCooling(cfg)
	e.coolRev.Add(1) // after the state is replaced: a reader never sees a revision ahead of its configuration
	return nil
}

// replaceCooling ends the running loop and starts one for cfg. Fans stay where
// they are if a live loop takes over the same outputs; in every other case the
// old loop leaves them at their safe duty, because nothing may keep them low.
func (e *Engine) replaceCooling(cfg coolingConfig) {
	old := e.coolingSnapshot()
	e.dropChangedPairs(old.cfg, cfg)
	var prev <-chan struct{} // the old loop, if it is still stuck after coolingStopWait
	var from *boardPumps     // its pumps, if the new loop takes over their line
	sensorWait := coolingSensorWait
	handed := false // fans were left where they are for the new loop
	if old.ctrl != nil {
		if old.fans != nil && cfg.Mode == coolingModeLive && !e.Demo && e.coolingOn() && sameOutputs(old.cfg, cfg) {
			old.ctrl.Detach()
			handed = true
			old.pumps.keepLine() // the pump stays with CaseHUD while the new loop starts
			sensorWait = 0       // the fans are running on what the old loop set: regulate from the first tick
		}
		old.cancel()
		if old.done != nil {
			select {
			case <-old.done:
			case <-time.After(coolingStopWait):
				log.Printf("cooling: the old loop is stuck in a call; the new one waits for it")
				prev = old.done
			}
		}
		if prev == nil {
			from = old.pumps
		} else if old.pumps != nil {
			go func(p *boardPumps, done <-chan struct{}) { <-done; p.endKept() }(old.pumps, old.done)
		}
	}
	var start map[string]int
	if old.ctrl != nil && prev == nil {
		// The outputs stand where the old loop left them (a pump of the board went back
		// to the board, so only a handed-over line counts): the new loop ramps from there.
		start = old.ctrl.Duties()
		for id := range start {
			if !handed && !tlOutputID.MatchString(id) {
				delete(start, id)
			}
		}
	}
	if err := e.launchCoolingAfter(cfg, sensorWait, prev, from, start); err != nil { // checked before; the file is in place
		e.setCoolingState(coolingState{errMsg: shortCoolingError(err), configured: true})
		if handed {
			e.guardHandoverFailed(err)
		}
	}
}

// dropChangedPairs forgets the check result of every pump whose speed sensor or
// control channel differs from before: that is a pair nobody has proven.
func (e *Engine) dropChangedPairs(from, to coolingConfig) {
	for _, o := range to.Outputs {
		if !o.Pump || tlOutputID.MatchString(o.ID) {
			continue
		}
		i := slices.IndexFunc(from.Outputs, func(x coolingOutputConfig) bool { return x.ID == o.ID })
		if i < 0 {
			continue
		}
		was, _ := pumpControl(from.Outputs[i])
		now, _ := pumpControl(o)
		if was != now || from.Outputs[i].RPMSensor != o.RPMSensor {
			e.storeMatch(o.ID, nil)
		}
	}
}

func sameOutputs(a, b coolingConfig) bool {
	ids := func(c coolingConfig) []string {
		var out []string
		for _, o := range c.regulated() {
			out = append(out, fmt.Sprintf("%s/%v", o.ID, o.Pump))
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(ids(a), ids(b))
}

// CoolingMeasure returns the state of the last measurement.
func (e *Engine) CoolingMeasure() CoolingMeasureView {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	v := e.coolMeasure
	v.Results = slices.Clone(v.Results)
	v.Queue = slices.Clone(v.Queue)
	v.Outcomes = slices.Clone(v.Outcomes)
	return v
}

func (e *Engine) coolingMeasuring() bool {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	return e.coolMeasure.Running
}

// StopCoolingMeasure ends a running measurement and throws away its results:
// nothing of this run is stored.
func (e *Engine) StopCoolingMeasure() { e.stopCoolingMeasure(false) }

// StopCoolingMeasureKeep ends a run over all fans and stores the fans that were
// finished. A single fan has nothing finished until its end, so nothing is kept.
func (e *Engine) StopCoolingMeasureKeep() { e.stopCoolingMeasure(true) }

func (e *Engine) stopCoolingMeasure(keep bool) {
	e.coolMu.Lock()
	cancel := e.coolMeasureCancel
	if cancel != nil && !e.coolMeasureStopped {
		e.coolMeasureStopped, e.coolMeasureKeep = true, keep // the first answer counts
	}
	e.coolMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// waitCoolingMeasure waits until a measured fan is back at its safe duty. It says
// whether that happened in time: if not, the last write of the measurement may
// come after the controller is closed, and the fan may be left low.
func (e *Engine) waitCoolingMeasure() bool {
	e.coolMu.Lock()
	done := e.coolMeasureDone
	e.coolMu.Unlock()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	case <-time.After(coolingStopWait):
		return false
	}
}

// coolingMeasureStale is how old the last tick of the control may be while a
// measurement runs. The guard reads what the control published; if the control
// stands, that is no longer the state of the machine.
const coolingMeasureStale = 5 * time.Second

// coolingMeasureNow is the clock of that check (tests replace it).
var coolingMeasureNow = time.Now

var (
	errMeasureLoopStopped = errors.New("the control has stopped, the temperatures are not current")
	errMeasureDeviceLost  = errors.New("a fan or pump cannot be read any more")
)

// measureSources lists every sensor that any curve of the configuration depends on:
// the curves of the outputs and all others. A pump on a manual value has no curve of
// its own, so the temperature guard of its check and its calibration is this union.
func (c coolingConfig) measureSources() []string {
	var out []string
	add := func(list []string) {
		for _, s := range list {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	for _, o := range c.regulated() {
		if o.Curve != "" {
			add(c.sensors(o.Curve, map[string]bool{}))
		}
	}
	for _, x := range c.Curves {
		add(c.sensors(x.ID, map[string]bool{}))
	}
	return out
}

// hottestTemp returns the highest of the temperatures of the sources. It is
// unusable if one of them is missing: the others could hide the hot part.
func (e *Engine) hottestTemp(sources []string) (float64, bool) {
	return cooling.Hottest(sources, coolingTemps(e.prov))
}

// coolingMeasureCheck ends a run when the output is needed for cooling after all.
// It ends when the control has stopped or lost a device, when a pump stands and
// when the temperature rises. For a pump the temperature is the hottest of all
// sources of the configuration, with the lower limits of a pump: the coolant
// follows the parts slowly. A source that is gone ends the run. For a fan it is
// the temperature of its own curve, or the hottest source of all if it has none.
func coolingMeasureCheck(ctrl *cooling.Controller, id string, pump bool, hottest func() (float64, bool)) func() error {
	own := func() (float64, bool) {
		for _, s := range ctrl.Status() {
			if s.ID == id && s.HasTemp {
				return s.Temp, true
			}
		}
		// An output with no temperature of its own, a fan on a manual value, is
		// guarded by the hottest source of the configuration like a pump.
		return hottest()
	}
	var guard func() error
	if pump {
		guard = cooling.RiseGuardAt(hottest, cooling.PumpMeasureMaxRise, cooling.PumpMeasureMaxTemp)
	} else {
		guard = cooling.RiseGuard(own)
	}
	return func() error {
		status := ctrl.Status()
		for _, s := range status {
			if s.PumpDown {
				return cooling.ErrPumpStopped
			}
		}
		if last := ctrl.LastTick(); last.IsZero() || coolingMeasureNow().Sub(last) > coolingMeasureStale {
			return errMeasureLoopStopped
		}
		for _, s := range status {
			if s.DeviceLost {
				return errMeasureDeviceLost
			}
		}
		return guard()
	}
}

// withMeasuredSpeeds stores a speed map. The curve is not touched: it may be
// shared, and a percent curve keeps meaning the same.
func withMeasuredSpeeds(o coolingOutputConfig, m cooling.SpeedMap) coolingOutputConfig {
	o.Speeds = make([][]int, len(m))
	for i, p := range m {
		o.Speeds[i] = []int{p.Duty, p.RPM}
	}
	return o
}

// coolingAllowHeader asks an administrator to release a header (tests replace it).
var coolingAllowHeader = sysperm.AllowCoolingHeader

// ErrReleaseWait: a request for the release was made a moment ago.
var ErrReleaseWait = errors.New("a release was asked for a moment ago, wait before asking again")

// releaseEvery is the pause after a request for the release. It stops a program
// that can reach the interface from opening one prompt after another.
const releaseEvery = 30 * time.Second

// releaseGate lets one request for the release run at a time, and one every releaseEvery.
type releaseGate struct {
	mu   sync.Mutex
	next time.Time
	now  func() time.Time // time.Now, replaced in tests
}

func (g *releaseGate) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

// run calls ask if no other request runs and the pause is over.
func (g *releaseGate) run(ask func() error) error {
	if !g.mu.TryLock() {
		return ErrReleaseWait
	}
	defer g.mu.Unlock()
	if g.clock().Before(g.next) {
		return ErrReleaseWait
	}
	// Counted from the end: the prompt may stay open for a long time.
	defer func() { g.next = g.clock().Add(releaseEvery) }()
	return ask()
}

// ReleaseCoolingPump asks an administrator to release the mainboard header behind
// a configured pump, so that CaseHUD may set it. The header is taken from the
// configuration; nothing else can be named.
func (e *Engine) ReleaseCoolingPump(id string) error {
	c := e.coolingSnapshot()
	i := slices.IndexFunc(c.cfg.Outputs, func(o coolingOutputConfig) bool { return o.ID == id })
	if i < 0 || !c.cfg.Outputs[i].Pump || !c.cfg.Outputs[i].driven() || tlOutputID.MatchString(id) {
		return fmt.Errorf("no pump %q on a mainboard header", id)
	}
	key, ok := pumpControl(c.cfg.Outputs[i])
	if !ok {
		return fmt.Errorf("the pump %q is not on a mainboard header", id)
	}
	if e.Demo {
		return errors.New("not available in the demo")
	}
	// Only a pump the service refused: a released one needs nothing, and a pump that
	// was never tried gives no reason to ask an administrator.
	if !c.pumps.notReleasedFor(id) {
		return fmt.Errorf("the pump %q is not waiting for a release", id)
	}
	return e.release.run(func() error { return coolingAllowHeader(key) })
}
