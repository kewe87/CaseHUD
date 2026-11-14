// Package cooling regulates fans and coolant pumps from temperature readings.
//
// Every output has its own curve, speed map and state, so each fan can run at
// its own speed. A system with only fans needs nothing beyond plain Fan
// entries. Water cooling adds two things, both optional: a pump is an output
// of its own kind (see Fan.Pump), and a slow source can be followed ahead of
// time (see Fan.LeadTicks).
//
// Failure rules:
//   - A sensor without a reading sends its outputs to at least SafeDuty. This
//     never lowers them: an overheating part can be what the reading lost.
//   - A pump that stands still gets full duty, every other output goes to at
//     least SafeDuty, and Tick reports ErrPumpStopped.
//   - While a device cannot be read nothing is written to it; afterwards control
//     is taken again and every duty is sent again. Other devices go on.
//   - After a pause of the loop, such as a suspend, every duty is sent again. So
//     it is once a minute anyway, in case a device lost them unnoticed.
//   - When the loop ends every output is left at SafeDuty or above, because fans
//     keep their last duty after that. A higher duty stays: the part may be hot.
package cooling

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Readings outside these ranges are treated as missing.
	lowestPlausibleTemp  = 0.0 // exclusive: a disconnected probe often reads 0
	highestPlausibleTemp = 125.0
	highestPlausibleRPM  = 10000

	resendEvery = time.Minute      // control is taken and duties are sent again this often
	sleepGap    = 10 * time.Second // a pause this long between two ticks means the PC slept
)

var (
	// ErrPumpStopped is reported while a pump does not turn.
	ErrPumpStopped = errors.New("cooling: pump stopped")
	// ErrSensorLost is reported while an output has no valid temperature.
	ErrSensorLost = errors.New("cooling: sensor lost")
	// ErrClosed is returned after Shutdown.
	ErrClosed = errors.New("cooling: controller shut down")
	// ErrLeftSafe is what a device reports from Speeds when it has been let go on
	// purpose with its outputs at a safe duty, for another program that takes over.
	// Nothing is written to it then, and an end of the control does not count as
	// having left outputs low.
	ErrLeftSafe = errors.New("cooling: outputs left at a safe duty")
	// ErrDetached is what EndError reports after Detach.
	ErrDetached = errors.New("cooling: outputs left to another controller")
)

// Hardware drives the outputs. Calls may block for the time the device takes
// to answer.
type Hardware interface {
	// TakeControl makes an output follow SetDuty.
	TakeControl(id string) error
	// SetDuty sets an output in percent; 0 stops it.
	SetDuty(id string, percent int) error
	// Speeds reads the current rpm. Outputs without a speed sensor are left out.
	Speeds() (map[string]int, error)
}

// Sensors returns the current temperatures in °C by sensor name. It must leave
// out readings that are no longer fresh: a frozen value looks like a valid one.
type Sensors func() map[string]float64

// NoOutput is a Hardware that reaches no device: writes are dropped, so a
// Controller can run next to another fan program and only show its decisions.
type NoOutput struct {
	// RPM supplies speeds read elsewhere; nil means none are known.
	RPM func() map[string]int
}

// TakeControl implements Hardware and does nothing.
func (NoOutput) TakeControl(string) error { return nil }

// SetDuty implements Hardware and does nothing.
func (NoOutput) SetDuty(string, int) error { return nil }

// Speeds implements Hardware.
func (n NoOutput) Speeds() (map[string]int, error) {
	if n.RPM == nil {
		return nil, nil
	}
	return n.RPM(), nil
}

// Status is the state of one output after the last tick.
type Status struct {
	ID string
	// Temp is the hottest source; HasTemp is false without a valid reading.
	Temp    float64
	HasTemp bool
	// RPM is the reported speed; HasRPM is false if none is known.
	RPM    int
	HasRPM bool
	// Duty is the percentage the controller decided on.
	Duty int
	// TargetRPM is the speed Duty should give per Fan.Speeds, 0 without a map.
	TargetRPM  int
	SensorLost bool
	PumpDown   bool
	// DeviceLost is set while the output's device cannot be read: Duty is then
	// only the decision and does not reach the output.
	DeviceLost bool
	// Held is set while something else drives the output, see Controller.Hold.
	Held bool
	// Stalled: a fan stands still although it was asked to run.
	Stalled bool
	// WriteFailed: the device refused this tick's duty.
	WriteFailed bool
	// Emergency: a watched sensor stayed at the emergency temperature, so the
	// output runs at full duty whatever its curve or manual value says.
	Emergency bool
}

type output struct {
	reg     regulator
	curve   *curveNode // nil for a pump that is only watched
	owned   bool       // TakeControl succeeded since the hardware was last lost
	touched bool       // a duty was written at some time, so the output may be left low
	written int        // last duty the hardware acknowledged, -1 = unknown
}

// Controller runs the regulators of all outputs.
//
// Locking: tickMu is held across device I/O and guards outputs and closed. mu
// only guards the published status, so asking for it never waits for a device.
type Controller struct {
	hw      Hardware
	sensors Sensors

	tickMu  sync.Mutex
	outputs []*output
	curves  []*curveNode // each after those it takes its value from
	closed  bool
	endErr  error // how the controller ended, see EndError

	reacquire atomic.Bool
	resend    atomic.Bool
	lastTick  atomic.Int64 // when a tick last ended, Unix nanoseconds

	mu       sync.Mutex
	statuses []Status
	held     map[string]bool
	resume   map[string]int // duty an output stands at when it is released or taken over
}

// New checks the configuration and returns a controller for outputs that each
// describe their own curve. It does not touch the hardware.
func New(fans []Fan, hw Hardware, sensors Sensors) (*Controller, error) {
	curves := make([]CurveSpec, 0, len(fans))
	fans = append([]Fan(nil), fans...)
	seen := map[string]bool{}
	for i, f := range fans {
		if seen[f.ID] {
			return nil, fmt.Errorf("cooling: output %q configured twice", f.ID)
		}
		seen[f.ID] = true
		if f.CurveID != "" || f.watchOnly() {
			continue
		}
		curves = append(curves, f.ownCurve())
		fans[i].CurveID = f.ID
	}
	return NewShared(curves, fans, hw, sensors)
}

// NewShared checks the configuration and returns a controller whose outputs hang
// on the given curves. Outputs on one curve share its state, so they cannot drift
// apart. It does not touch the hardware.
func NewShared(curves []CurveSpec, fans []Fan, hw Hardware, sensors Sensors) (*Controller, error) {
	if len(fans) == 0 {
		return nil, errors.New("cooling: no outputs")
	}
	curveOf := make(map[string]string, len(fans))
	index := make(map[string]int, len(fans))
	for i, f := range fans {
		if _, twice := curveOf[f.ID]; twice {
			return nil, fmt.Errorf("cooling: output %q configured twice", f.ID)
		}
		curveOf[f.ID], index[f.ID] = f.CurveID, i
	}
	if err := CheckCurves(curves, curveOf); err != nil {
		return nil, err
	}
	ordered, err := curveOrder(curves, curveOf)
	if err != nil {
		return nil, err
	}
	c := &Controller{hw: hw, sensors: sensors}
	nodes := make(map[string]*curveNode, len(ordered))
	for _, s := range ordered {
		s.Sources = append([]string(nil), s.Sources...)
		s.Guard = append([]string(nil), s.Guard...)
		n := &curveNode{spec: s, eval: newEvaluator(s)}
		for _, need := range s.needs(curveOf) {
			n.inputs = append(n.inputs, nodes[need])
		}
		n.follow = index[s.Follow]
		n.settle()
		nodes[s.ID] = n
		c.curves = append(c.curves, n)
	}
	for i, f := range fans {
		o := &output{written: -1}
		var top float64
		if f.CurveID != "" {
			n, ok := nodes[f.CurveID]
			if !ok {
				return nil, fmt.Errorf("cooling: %q: no curve %q", f.ID, f.CurveID)
			}
			if n.noSensor {
				return nil, fmt.Errorf("cooling: %q: no sensor", f.ID)
			}
			f.RPMCurve = n.spec.RPM
			top = n.most
			o.curve = n
			n.outs = append(n.outs, i)
		}
		if err := f.validate(top); err != nil {
			return nil, err
		}
		o.reg = regulator{cfg: f}
		c.outputs = append(c.outputs, o)
		c.statuses = append(c.statuses, Status{ID: f.ID})
	}
	return c, nil
}

// Tick advances every output once. The returned error reports what went wrong
// in this tick; the loop keeps working around it.
func (c *Controller) Tick() error {
	c.tickMu.Lock()
	defer c.tickMu.Unlock()
	if c.closed {
		return ErrClosed
	}
	defer func() { c.lastTick.Store(time.Now().UnixNano()) }()
	if c.reacquire.Swap(false) {
		c.forgetHardware(false)
	}
	if c.resend.Swap(false) {
		// Control is taken again with the duties: a controller that lost it (a reset
		// no tick noticed) would otherwise ignore them until the PC sleeps.
		c.forgetHardware(false)
	}
	var errs []error
	lost := map[string]bool{}
	leftSafe := map[string]bool{}
	speeds, err := c.hw.Speeds()
	if err != nil {
		var part *PartlyLost
		if !errors.As(err, &part) {
			c.forgetHardware(errors.Is(err, ErrLeftSafe))
			c.publishLost()
			return fmt.Errorf("cooling: reading speeds: %w", err)
		}
		for _, id := range part.IDs {
			lost[id] = true
		}
		for _, id := range part.Safe {
			leftSafe[id] = true
		}
		errs = append(errs, fmt.Errorf("cooling: reading speeds: %w", err))
	}
	temps := c.sensors()
	c.mu.Lock()
	held := maps.Clone(c.held)
	resume := c.resume
	c.resume = nil
	c.mu.Unlock()
	for _, o := range c.outputs {
		id := o.reg.cfg.ID
		if duty, ok := resume[id]; ok {
			if held[id] {
				c.noteResume(id, duty) // still held: the duty waits for the release
				continue
			}
			o.reg.resumeAt(duty)
		}
	}

	results := make([]result, len(c.outputs))
	statuses := make([]Status, len(c.outputs))
	pumpDown := false
	// Follow curves read the decision of the output they follow, so outputs get
	// theirs curve by curve.
	decide := func(i int, sig signal) {
		o := c.outputs[i]
		rpm, rpmOK := speeds[o.reg.cfg.ID]
		rpmOK = rpmOK && rpm >= 0 && rpm <= highestPlausibleRPM
		results[i] = o.reg.step(sig, rpm, rpmOK)
		statuses[i] = Status{ID: o.reg.cfg.ID, Temp: sig.temp, HasTemp: sig.hasTemp, RPM: rpm, HasRPM: rpmOK,
			SensorLost: results[i].sensorLost, PumpDown: results[i].pumpDown, Emergency: results[i].emergency, DeviceLost: lost[o.reg.cfg.ID],
			Stalled: !o.reg.cfg.Pump && o.reg.idle >= spinUpTicks}
		if results[i].sensorLost {
			errs = append(errs, fmt.Errorf("%w: %s", ErrSensorLost, o.reg.cfg.ID))
		}
		if results[i].pumpDown {
			pumpDown = true
			errs = append(errs, fmt.Errorf("%w: %s", ErrPumpStopped, o.reg.cfg.ID))
		}
	}
	for i, o := range c.outputs {
		if o.curve == nil {
			decide(i, signal{})
		}
	}
	for _, n := range c.curves {
		n.advance(temps, func(i int) (float64, bool) {
			if held[c.outputs[i].reg.cfg.ID] {
				// Whatever holds an output drives it, a calibration even stops it: its
				// measured speed is no demand another output should copy. Follow what the
				// controller decided for it instead.
				if sp := c.outputs[i].reg.cfg.Speeds; n.spec.RPM && sp != nil {
					return float64(sp.rpmFor(results[i].duty)), true
				}
				if !n.spec.RPM {
					return float64(results[i].duty), true
				}
				return 0, false
			}
			if n.spec.RPM {
				// Standing at a running duty it is broken or misread, and no speed
				// another fan should copy.
				if statuses[i].RPM == 0 && results[i].duty > 0 {
					return 0, false
				}
				return float64(statuses[i].RPM), statuses[i].HasRPM
			}
			return float64(results[i].duty), true
		})
		for _, i := range n.outs {
			decide(i, n.sig)
		}
	}
	for i, o := range c.outputs {
		duty := results[i].duty
		if pumpDown {
			duty = max(duty, o.reg.cfg.SafeDuty)
		}
		statuses[i].Duty = duty
		if o.reg.cfg.Speeds != nil {
			statuses[i].TargetRPM = o.reg.cfg.Speeds.rpmFor(duty)
			// A curve in rpm asks for a speed of its own: that is the target, not
			// what the duty it was turned into gives back from the speed table.
			if n := o.curve; n != nil && n.spec.RPM && n.sig.ok && duty == results[i].duty {
				statuses[i].TargetRPM = int(math.Round(n.sig.value))
			}
		}
		if lost[o.reg.cfg.ID] {
			o.owned, o.written = false, -1
			if leftSafe[o.reg.cfg.ID] {
				o.touched = false
			}
			continue
		}
		if held[o.reg.cfg.ID] {
			statuses[i].Held = true
			if !o.reg.cfg.Pump {
				// Whatever holds a fan drives it, and a calibration stops it on
				// purpose: its standstill is no fault. A pump is excepted, its
				// standstill always counts. The count starts again after Release.
				o.reg.idle, statuses[i].Stalled = 0, false
			}
			o.written = -1 // whatever holds it leaves another duty behind
			continue
		}
		if err := c.apply(o, duty); err != nil {
			statuses[i].WriteFailed = true
			errs = append(errs, err)
		}
	}
	c.mu.Lock()
	c.statuses = statuses
	c.mu.Unlock()
	return errors.Join(errs...)
}

// publishLost marks every output as out of reach and keeps its other values.
func (c *Controller) publishLost() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.statuses {
		c.statuses[i].DeviceLost = true
	}
}

// Status returns the state of every output after the last tick, in
// configuration order. Before the first tick only the ID is set.
func (c *Controller) Status() []Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Status(nil), c.statuses...)
}

// LastTick returns when a tick last ended, the zero time before the first.
func (c *Controller) LastTick() time.Time {
	n := c.lastTick.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// Reacquire takes control and sends every duty again on the next tick. Use it
// after a suspend or anything else that may have reset the hardware.
func (c *Controller) Reacquire() { c.reacquire.Store(true) }

// Hold keeps the controller from writing to an output until Release, while
// something else drives it. The decision of the output is still worked out in
// that time and shown, but it is not sent and does not count as the duty the
// output stands at: after Release the output ramps from the duty it really has.
func (c *Controller) Hold(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.held == nil {
		c.held = map[string]bool{}
	}
	c.held[id] = true
}

// Release gives an output back. standingAt is the duty it really has now, in
// percent, which the caller knows because it set it last; 0 or less means
// unknown, and the output is then taken to stand at full duty, so that it only
// ever comes down by its steps. From there the rise and fall limits apply like
// in any other tick. The duty is sent on the next tick.
func (c *Controller) Release(id string, standingAt int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.held, id)
	if standingAt <= 0 {
		standingAt = 100
	}
	c.noteResumeLocked(id, min(standingAt, 100))
}

// StartAt tells a controller that has not ticked yet what its outputs stand at,
// when it takes over from another one: the first tick then ramps from these
// duties instead of jumping. Outputs not named start as unknown, and so do
// negative duties.
func (c *Controller) StartAt(duties map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, d := range duties {
		if d >= 0 {
			c.noteResumeLocked(id, min(d, 100))
		}
	}
}

func (c *Controller) noteResume(id string, duty int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.noteResumeLocked(id, duty)
}

func (c *Controller) noteResumeLocked(id string, duty int) {
	if c.resume == nil {
		c.resume = map[string]int{}
	}
	c.resume[id] = duty
}

// Duties returns the duty each output stands at, by ID: the last one sent, or the
// one a Release or StartAt named that no tick has used yet. Outputs whose duty is
// unknown are left out. It waits for a tick in progress.
func (c *Controller) Duties() map[string]int {
	c.tickMu.Lock()
	defer c.tickMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for _, o := range c.outputs {
		id := o.reg.cfg.ID
		if d, ok := c.resume[id]; ok {
			out[id] = d
		} else if o.written >= 0 {
			out[id] = o.written
		}
	}
	return out
}

// Detach ends the controller and leaves every output where it is, for a new
// controller that takes over at once.
func (c *Controller) Detach() {
	c.tickMu.Lock()
	defer c.tickMu.Unlock()
	if !c.closed {
		c.endErr = ErrDetached
	}
	c.closed = true
}

// Shutdown leaves every controlled output at SafeDuty, or above it if it ran
// faster, and ends the controller.
func (c *Controller) Shutdown() error {
	c.tickMu.Lock()
	defer c.tickMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var errs []error
	for _, o := range c.outputs {
		if o.owned {
			// The floor is checked on construction already; it is repeated here
			// because a stopped fan must never be what a shutdown leaves behind.
			errs = append(errs, c.apply(o, max(o.written, o.reg.cfg.SafeDuty, LowestSafeDuty)))
		}
		if !o.owned && o.touched {
			errs = append(errs, fmt.Errorf("cooling: %s could not be set to a safe duty", o.reg.cfg.ID))
		}
	}
	c.endErr = errors.Join(errs...)
	return c.endErr
}

// EndError says how the controller ended: nil if Shutdown left every output at its
// safe duty or above, ErrDetached if the outputs were left where they were for a
// new controller, otherwise what failed. It is nil before the end, too.
func (c *Controller) EndError() error {
	c.tickMu.Lock()
	defer c.tickMu.Unlock()
	return c.endErr
}

// Run ticks until ctx ends and then shuts down. report receives every error
// and may be nil.
func (c *Controller) Run(ctx context.Context, every time.Duration, report func(error)) {
	tell := func(err error) {
		if err != nil && report != nil {
			report(err)
		}
	}
	defer func() { tell(c.Shutdown()) }()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	beat := newPacer(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slept, resend := beat.beat(time.Now())
			if slept {
				c.Reacquire()
			}
			if resend {
				c.resend.Store(true)
			}
			tell(c.Tick())
		}
	}
}

// pacer decides from the time of each tick what is due. It reads the wall clock:
// the monotonic one may stand still while the PC sleeps.
type pacer struct{ last, resent time.Time }

func newPacer(now time.Time) *pacer {
	now = now.Round(0)
	return &pacer{last: now, resent: now}
}

func (p *pacer) beat(now time.Time) (slept, resend bool) {
	now = now.Round(0)
	slept = now.Sub(p.last) >= sleepGap
	p.last = now
	if slept || now.Sub(p.resent) >= resendEvery {
		p.resent = now
		resend = !slept // after a sleep Reacquire sends everything anyway
	}
	return slept, resend
}

// forgetHardware drops what is known of the device. With safe, its outputs were left
// at a safe duty, so a later end of the control has nothing to put right.
func (c *Controller) forgetHardware(safe bool) {
	for _, o := range c.outputs {
		o.owned, o.written = false, -1
		if safe {
			o.touched = false
		}
	}
}

// apply writes a duty if it changed. Whatever fails is repeated next tick.
func (c *Controller) apply(o *output, duty int) error {
	id := o.reg.cfg.ID
	if o.reg.cfg.Pump {
		duty = max(duty, LowestPumpDuty) // whatever asked: a pump is never set below it
	}
	if !o.owned {
		if err := c.hw.TakeControl(id); err != nil {
			return fmt.Errorf("cooling: taking control of %s: %w", id, err)
		}
		o.owned = true
	}
	if duty == o.written {
		return nil
	}
	if err := c.hw.SetDuty(id, duty); err != nil {
		return fmt.Errorf("cooling: setting %s: %w", id, err)
	}
	o.written, o.touched = duty, true
	return nil
}

// warmestPresent returns the highest reading among the sources that have one. It
// is enough for the emergency rule: a part that is known to be too hot stays too
// hot, whatever happened to the sensor next to it.
func warmestPresent(sources []string, temps map[string]float64) (float64, bool) {
	best, found := 0.0, false
	for _, s := range sources {
		if t, ok := temps[s]; ok && !math.IsNaN(t) && t > lowestPlausibleTemp && t <= highestPlausibleTemp {
			best, found = max(best, t), true
		}
	}
	return best, found
}

// Hottest returns the highest reading of the sources, unusable if one is missing.
func Hottest(sources []string, temps map[string]float64) (float64, bool) {
	return hottest(sources, temps)
}

// hottest returns the highest reading of the sources. A missing source makes
// the result unusable: the remaining ones could hide the hot part.
func hottest(sources []string, temps map[string]float64) (float64, bool) {
	if len(sources) == 0 {
		return 0, false
	}
	best := math.Inf(-1)
	for _, s := range sources {
		t, ok := temps[s]
		if !ok || math.IsNaN(t) || t <= lowestPlausibleTemp || t > highestPlausibleTemp {
			return 0, false
		}
		best = max(best, t)
	}
	return best, true
}
