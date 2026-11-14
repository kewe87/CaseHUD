package cooling

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// calFan is a fan with a lowest running duty and a duty it needs to start. The
// clock advances only through Wait.
type calFan struct {
	now       time.Duration
	perDuty   int
	stopBelow int // it stops under this duty while running
	startAt   int // it starts at this duty from rest
	twitchAt  int // from here to startAt it only twitches for 3 s
	never     bool
	byItself  time.Duration // a stop not sent again within this time ends: it runs (0: never)
	always    bool          // it restarts by itself whatever is sent

	on        bool
	duty      int
	lastZero  time.Duration
	twitchEnd time.Duration
	stoppedAt time.Duration
	writes    []int
	zeroAt    []time.Duration
	taken     bool
}

func (f *calFan) TakeControl(string) error { f.taken, f.on = true, true; return nil }

func (f *calFan) SetDuty(_ string, d int) error {
	f.writes = append(f.writes, d)
	prev := f.duty
	f.duty = d
	switch {
	case d == 0 && !f.never:
		if f.on || prev != 0 {
			f.stoppedAt = f.now
		}
		f.on = false
		f.lastZero = f.now
		f.zeroAt = append(f.zeroAt, f.now)
	case f.on && d < f.stopBelow && !f.never:
		f.on = false
		f.stoppedAt = f.now
	case !f.on && d >= f.startAt:
		f.on = true
	case !f.on && f.twitchAt > 0 && d >= f.twitchAt:
		f.twitchEnd = f.now + 3*time.Second
	}
	return nil
}

func (f *calFan) rpmNow() int {
	switch {
	case f.on && f.duty > 0:
		return f.duty * f.perDuty
	case f.on:
		return 12 * f.perDuty // never stops at 0
	case f.always && f.duty == 0 && f.now-f.stoppedAt >= 10*time.Second:
		return 12 * f.perDuty
	case f.byItself > 0 && f.duty == 0 && f.now-f.lastZero >= f.byItself:
		return 12 * f.perDuty
	case f.now < f.twitchEnd:
		return 300
	}
	return 0
}

func (f *calFan) Speeds() (map[string]int, error) { return map[string]int{"f": f.rpmNow()}, nil }

func (f *calFan) wait(ctx context.Context, d time.Duration) error {
	f.now += d
	return ctx.Err()
}

func newCal(f *calFan) Measurement {
	return Measurement{Device: f, ID: "f", Steps: []int{100, 50, 20}, Wait: f.wait}
}

func TestCalibrateFindsStopAndStart(t *testing.T) {
	f := &calFan{perDuty: 20, stopBelow: 8, startAt: 14, twitchAt: 10, byItself: 8 * time.Second}
	steps, cal, err := newCal(f).Calibrate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cal.Stops != StopYes || cal.Stop != 8 || !cal.Starts || cal.Start != 14 {
		t.Fatalf("%+v", cal)
	}
	// 100, 50, 20 and every percent down to the one where it stood; nothing below that.
	if last := steps[len(steps)-1]; last.Duty != 7 || last.RPM != 0 {
		t.Fatalf("last step %+v", last)
	}
	if _, err := SpeedMapFrom(steps); err != nil {
		t.Fatalf("the map is no longer valid: %v", err)
	}
	// The standstill was kept up by sending the stop again, at least every second.
	if len(f.zeroAt) < int(CalibrationHold/time.Second) {
		t.Fatalf("%d stops sent", len(f.zeroAt))
	}
	for i := 1; i < len(f.zeroAt); i++ {
		if f.zeroAt[i]-f.zeroAt[i-1] > time.Second {
			t.Fatalf("stop repeated after %v", f.zeroAt[i]-f.zeroAt[i-1])
		}
	}
}

func TestCalibrateIgnoresATwitchAndWaitsForSteadyRunning(t *testing.T) {
	f := &calFan{perDuty: 20, stopBelow: 8, startAt: 14, twitchAt: 10}
	var longest time.Duration
	m := newCal(f)
	m.Update = func(p CalibrationProgress) {
		if p.Phase == PhaseStart && p.Duty < 14 && p.Held > longest {
			longest = p.Held
		}
	}
	_, cal, err := m.Calibrate(context.Background())
	if err != nil || cal.Start != 14 {
		t.Fatalf("%+v %v", cal, err)
	}
	if longest > 3*time.Second {
		t.Fatalf("a twitch counted for %v", longest)
	}
	// Every failed step lasted the whole hold time.
	if f.now < 4*CalibrationHold {
		t.Fatalf("restart took only %v", f.now)
	}
}

func TestCalibrateWithoutRepeatedStopTheFanStartsItself(t *testing.T) {
	// The fan above runs on its own 8 s after a stop; here it does so whatever is sent.
	f := &calFan{perDuty: 20, stopBelow: 8, startAt: 14, always: true}
	_, cal, err := newCal(f).Calibrate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cal.Stops != StopUnheld || cal.Starts {
		t.Fatalf("%+v", cal)
	}
}

func TestCalibrateReportsAFanThatDoesNotStop(t *testing.T) {
	f := &calFan{perDuty: 20, never: true, startAt: 1}
	_, cal, err := newCal(f).Calibrate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cal.Stops != StopNever || cal.Starts {
		t.Fatalf("%+v", cal)
	}
	if !slices.Contains(f.writes, 1) || slices.Contains(f.writes, 51) {
		t.Fatalf("writes %v", f.writes)
	}
}

func TestCalibrateRestartIsLimited(t *testing.T) {
	f := &calFan{perDuty: 20, stopBelow: 8, startAt: 99}
	_, cal, err := newCal(f).Calibrate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cal.Stops != StopYes || cal.Starts || slices.Max(f.writes[len(f.writes)-3:]) > calibrationStartLimit {
		t.Fatalf("%+v", cal)
	}
}

func TestCalibrateStoppedFanStartsAtAFirstHalfPercentNothing(t *testing.T) {
	f := &calFan{perDuty: 0}
	if _, _, err := newCal(f).Calibrate(context.Background()); !errors.Is(err, ErrNoSpeed) {
		t.Fatalf("%v", err)
	}
}

// Cancelling and a guard that trips end the run in every phase.
func TestCalibrateEndsInEveryPhase(t *testing.T) {
	boom := errors.New("too hot")
	for _, phase := range []Phase{PhaseDown, PhaseStop, PhaseStart} {
		for _, byGuard := range []bool{false, true} {
			f := &calFan{perDuty: 20, stopBelow: 8, startAt: 14, byItself: 8 * time.Second}
			ctx, cancel := context.WithCancel(context.Background())
			var at Phase
			var n int
			m := newCal(f)
			m.Update = func(p CalibrationProgress) {
				at = p.Phase
				if p.Phase == phase {
					n++
				}
				if p.Phase == phase && n == 3 && !byGuard {
					cancel()
				}
			}
			m.Check = func() error {
				if byGuard && at == phase && n >= 3 {
					return boom
				}
				return nil
			}
			_, _, err := m.Calibrate(ctx)
			want := context.Canceled
			if byGuard {
				want = boom
			}
			if !errors.Is(err, want) {
				t.Errorf("%s guard=%v: %v", phase, byGuard, err)
			}
			cancel()
		}
	}
}

// pumpRig: the pump collapses under a duty, or loses speed far faster than its duty.
type pumpRig struct {
	now      time.Duration
	duty     int
	collapse int  // rpm 0 under this duty
	halves   int  // half the speed under this duty
	dead     bool // does not turn after the collapse, even at 100
	broke    bool
	writes   []int
}

func (p *pumpRig) TakeControl(string) error { return nil }
func (p *pumpRig) SetDuty(_ string, d int) error {
	p.writes = append(p.writes, d)
	p.duty = d
	if d < p.collapse {
		p.broke = true
	}
	return nil
}
func (p *pumpRig) Speeds() (map[string]int, error) {
	r := p.duty * 40
	switch {
	case p.dead && p.broke:
		r = 0
	case p.duty < p.collapse:
		r = 0
	case p.halves > 0 && p.duty < p.halves:
		r /= 3
	}
	return map[string]int{"p": r}, nil
}
func (p *pumpRig) wait(ctx context.Context, d time.Duration) error { p.now += d; return ctx.Err() }

func newPump(p *pumpRig, floor int) PumpMeasurement {
	return PumpMeasurement{Device: p, ID: "p", Floor: floor, StoppedBelow: 500, Wait: p.wait}
}

func TestPumpMeasurementStopsAtTheCollapseAndReturnsToFull(t *testing.T) {
	p := &pumpRig{collapse: 48}
	res, err := newPump(p, 30).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Minimum != 50 || res.Failed != 45 || res.AtLimit {
		t.Fatalf("%+v", res)
	}
	if p.writes[len(p.writes)-1] != 100 || p.duty != 100 {
		t.Fatalf("writes %v", p.writes)
	}
	// Right after the collapse comes full duty, not the next lower step.
	if p.writes[len(p.writes)-2] != 45 {
		t.Fatalf("writes %v", p.writes)
	}
}

func TestPumpMeasurementNeverGoesBelowTheFloorOrToZero(t *testing.T) {
	p := &pumpRig{}
	res, err := newPump(p, 30).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.AtLimit || res.Minimum != 30 || res.Failed != 0 {
		t.Fatalf("%+v", res)
	}
	if slices.Min(p.writes) < 30 || slices.Contains(p.writes, 0) {
		t.Fatalf("writes %v", p.writes)
	}
	for _, floor := range []int{0, 29, 101} {
		q := &pumpRig{}
		if _, err := newPump(q, floor).Run(context.Background()); err == nil || len(q.writes) != 0 {
			t.Fatalf("floor %d accepted: %v %v", floor, err, q.writes)
		}
	}
}

func TestPumpMeasurementSeesALossOutOfProportion(t *testing.T) {
	p := &pumpRig{halves: 70}
	res, err := newPump(p, 30).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Minimum != 70 || res.Failed != 65 {
		t.Fatalf("%+v", res)
	}
}

func TestPumpMeasurementReportsAPumpThatDoesNotComeBack(t *testing.T) {
	p := &pumpRig{collapse: 48, dead: true}
	_, err := newPump(p, 30).Run(context.Background())
	if !errors.Is(err, ErrPumpStopped) {
		t.Fatalf("%v", err)
	}
	if p.duty != 100 {
		t.Fatalf("left at %d", p.duty)
	}
}

func TestPumpMeasurementCancelAndGuardReturnToFull(t *testing.T) {
	boom := errors.New("too hot")
	p := &pumpRig{}
	ctx, cancel := context.WithCancel(context.Background())
	m := newPump(p, 30)
	n := 0
	m.Update = func(CalibrationProgress) {
		if n++; n == 40 {
			cancel()
		}
	}
	if _, err := m.Run(ctx); !errors.Is(err, context.Canceled) || p.duty != 100 {
		t.Fatalf("%v, duty %d", err, p.duty)
	}
	p = &pumpRig{}
	m = newPump(p, 30)
	m.Check = func() error {
		if p.duty < 80 {
			return boom
		}
		return nil
	}
	if _, err := m.Run(context.Background()); !errors.Is(err, boom) || p.duty != 100 {
		t.Fatalf("%v, duty %d", err, p.duty)
	}
}

// calFanLow is a fan on a controller that raises every duty from 1 up to low to low.
type calFanLow struct {
	*calFan
	low int
}

func (f calFanLow) LowestDuty() int { return f.low }
func (f calFanLow) SetDuty(id string, d int) error {
	if d > 0 && d < f.low {
		d = f.low
	}
	return f.calFan.SetDuty(id, d)
}

func TestCalibrateLeavesOutDutiesBelowTheLowestEffectiveOne(t *testing.T) {
	f := calFanLow{&calFan{perDuty: 20, stopBelow: 1, startAt: 16, twitchAt: 13, byItself: 8 * time.Second}, 12}
	m := Measurement{Device: f, ID: "f", Steps: []int{100, 50, 20, 15}, Wait: f.wait}
	steps, cal, err := m.Calibrate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.writes {
		if d != 0 && d < 12 {
			t.Fatalf("duty %d is no speed of its own, but was set: %v", d, f.writes)
		}
	}
	// It turns at 12 and stands at 0: the stop value is the lowest effective duty.
	if cal.Stops != StopYes || cal.Stop != 12 || !cal.Starts || cal.Start != 16 {
		t.Fatalf("%+v", cal)
	}
	if first := f.writes[len(f.writes)-1]; first < 16 {
		t.Fatalf("last duty %d", first)
	}
	// The restart began at 12, not at 1.
	if !slices.Contains(f.writes, 12) || f.writes[slices.Index(f.writes, 0)+31] != 12 {
		t.Fatalf("restart did not begin at 12: %v", f.writes)
	}
	// The table comes from the way down alone: 100, 50, 20, 15, 14, 13, 12.
	var duties []int
	for _, s := range steps {
		duties = append(duties, s.Duty)
	}
	if !slices.Equal(duties, []int{100, 50, 20, 15, 14, 13, 12}) {
		t.Fatalf("steps %v", duties)
	}
	table, err := SpeedMapFrom(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != len(duties) {
		t.Fatalf("table %v", table)
	}
	// What the suggestion would set stays on speeds the controller really has.
	if cal.Stop+SuggestStopMargin < 12 || cal.Start+SuggestStartMargin < 12 {
		t.Fatal("a suggestion under the lowest effective duty")
	}
}

func TestCalibrationTimeWithAndWithoutTheLowSteps(t *testing.T) {
	run := func(f interface {
		MeasureDevice
		wait(context.Context, time.Duration) error
	}, clock func() time.Duration) time.Duration {
		m := Measurement{Device: f, ID: "f", Steps: MeasureSteps, Wait: f.wait}
		if _, _, err := m.Calibrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		return clock()
	}
	a := &calFan{perDuty: 20, stopBelow: 1, startAt: 16, twitchAt: 13, byItself: 8 * time.Second}
	b := calFanLow{&calFan{perDuty: 20, stopBelow: 1, startAt: 16, twitchAt: 13, byItself: 8 * time.Second}, 12}
	before := run(a, func() time.Duration { return a.now })
	after := run(b, func() time.Duration { return b.now })
	t.Logf("with every percent: %v, from the lowest effective duty: %v", before, after)
	if after >= before {
		t.Fatalf("%v is not shorter than %v", after, before)
	}
}
