package cooling

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// bench is a fake of the hardware: it records every call and reports whatever
// speed the test set.
type bench struct {
	rpm       map[string]int
	duty      map[string]int
	writes    map[string][]int
	taken     map[string]int
	readErr   error
	writeErr  error
	takeErr   error
	followers map[string]int // rpm = duty * factor, unless set in rpm
}

func newBench() *bench {
	return &bench{rpm: map[string]int{}, duty: map[string]int{}, writes: map[string][]int{}, taken: map[string]int{}, followers: map[string]int{}}
}

func (b *bench) TakeControl(id string) error {
	if b.takeErr != nil {
		return b.takeErr
	}
	b.taken[id]++
	return nil
}

func (b *bench) SetDuty(id string, percent int) error {
	if b.writeErr != nil {
		return b.writeErr
	}
	b.duty[id] = percent
	b.writes[id] = append(b.writes[id], percent)
	return nil
}

func (b *bench) Speeds() (map[string]int, error) {
	if b.readErr != nil {
		return nil, b.readErr
	}
	out := map[string]int{}
	for id, factor := range b.followers {
		out[id] = b.duty[id] * factor
	}
	for id, rpm := range b.rpm {
		out[id] = rpm
	}
	return out, nil
}

// plain is a fan that follows a percent curve from 20 °C/20 % to 60 °C/100 %
// without any delay.
func plain(id string) Fan {
	return Fan{
		ID: id, Sources: []string{"water"},
		Curve:     Curve{{20, 20}, {60, 100}},
		RiseAfter: 1, FallAfter: 1,
		StepUp: 100, StepDown: 100, SafeDuty: 90,
	}
}

func pump(id string) Fan {
	f := plain(id)
	f.Pump, f.MinDuty = true, 40
	return f
}

type rig struct {
	t     *testing.T
	c     *Controller
	hw    *bench
	temps map[string]float64
}

func newRig(t *testing.T, fans ...Fan) *rig {
	t.Helper()
	r := &rig{t: t, hw: newBench(), temps: map[string]float64{"water": 30}}
	for _, f := range fans {
		r.hw.followers[f.ID] = 20
	}
	c, err := New(fans, r.hw, func() map[string]float64 { return r.temps })
	if err != nil {
		t.Fatal(err)
	}
	r.c = c
	return r
}

func (r *rig) tick(n int) {
	r.t.Helper()
	for i := 0; i < n; i++ {
		if err := r.c.Tick(); err != nil {
			r.t.Fatal(err)
		}
	}
}

func TestCurve(t *testing.T) {
	c := Curve{{20, 10}, {40, 50}, {60, 100}}
	for temp, want := range map[float64]float64{0: 10, 20: 10, 30: 30, 40: 50, 50: 75, 60: 100, 90: 100} {
		if got := c.at(temp); got != want {
			t.Errorf("%v °C: %v, want %v", temp, got, want)
		}
	}
}

func TestSpeedMapFindsLowestDuty(t *testing.T) {
	m := SpeedMap{{12, 240}, {50, 1380}, {70, 1860}, {80, 2040}, {100, 2040}}
	for rpm, want := range map[float64]int{0: 0, 100: 12, 240: 12, 1380: 50, 1620: 60, 2040: 80, 2041: 100, 9000: 100} {
		if got := m.dutyFor(rpm); got != want {
			t.Errorf("%v rpm: %d, want %d", rpm, got, want)
		}
	}
}

func TestConfigRefused(t *testing.T) {
	bad := map[string]func(*Fan){
		"no id":        func(f *Fan) { f.ID = "" },
		"no sensor":    func(f *Fan) { f.Sources = nil },
		"no curve":     func(f *Fan) { f.Curve = nil },
		"falling temp": func(f *Fan) { f.Curve = Curve{{30, 1}, {30, 2}} },
		"NaN":          func(f *Fan) { f.Curve = Curve{{math.NaN(), 1}} },
		"no step":      func(f *Fan) { f.StepUp = 0 },
		"no delay":     func(f *Fan) { f.RiseAfter = 0 },
		"safe off":     func(f *Fan) { f.SafeDuty = 0 },
		"short map":    func(f *Fan) { f.Speeds = SpeedMap{{12, 240}} },
		"falling map":  func(f *Fan) { f.Speeds = SpeedMap{{12, 240}, {50, 100}} },
		"zero point":   func(f *Fan) { f.Speeds = SpeedMap{{0, 0}, {50, 100}} },
		"fan rpm rule": func(f *Fan) { f.StoppedBelowRPM = 100 },
	}
	for name, change := range bad {
		f := plain("a")
		change(&f)
		if _, err := New([]Fan{f}, newBench(), nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := New(nil, newBench(), nil); err == nil {
		t.Error("no outputs accepted")
	}
	if _, err := New([]Fan{plain("a"), plain("a")}, newBench(), nil); err == nil {
		t.Error("duplicate accepted")
	}
	for name, change := range map[string]func(*Fan){
		"pump without floor": func(f *Fan) { f.MinDuty = 0 },
		"pump with stop":     func(f *Fan) { f.StopAt = 5 },
		"pump with start":    func(f *Fan) { f.StartDuty = 50 },
	} {
		f := pump("p")
		change(&f)
		if _, err := New([]Fan{f}, newBench(), nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestEveryFanHasItsOwnSpeed(t *testing.T) {
	strong, weak := plain("strong"), plain("weak")
	curve := Curve{{20, 1000}, {60, 1000}}
	strong.Curve, weak.Curve = curve, curve
	strong.RPMCurve, weak.RPMCurve = true, true
	strong.Speeds = SpeedMap{{10, 500}, {100, 3000}}
	weak.Speeds = SpeedMap{{10, 500}, {100, 1500}}
	other := plain("other")
	other.Curve = Curve{{20, 35}, {60, 35}}
	r := newRig(t, strong, weak, other)
	r.tick(1)
	if a, b, c := r.hw.duty["strong"], r.hw.duty["weak"], r.hw.duty["other"]; a != 28 || b != 55 || c != 35 {
		t.Fatalf("duties %d %d %d", a, b, c)
	}
}

func TestSmallChangesDoNotMoveTheFan(t *testing.T) {
	f := plain("a")
	f.RiseBy, f.FallBy = 2, 2
	r := newRig(t, f)
	r.tick(1)
	for _, temp := range []float64{31, 29, 31.9, 28.1, 30} {
		r.temps["water"] = temp
		r.tick(1)
	}
	if n := len(r.hw.writes["a"]); n != 1 {
		t.Fatalf("%d writes: %v", n, r.hw.writes["a"])
	}
}

func TestChangeNeedsToLast(t *testing.T) {
	f := plain("a")
	f.RiseBy, f.RiseAfter = 2, 3
	r := newRig(t, f)
	r.tick(1)
	before := r.hw.duty["a"]
	r.temps["water"] = 40
	r.tick(2)
	r.temps["water"] = 30 // the spike ends: the count starts over
	r.tick(1)
	r.temps["water"] = 40
	r.tick(2)
	if r.hw.duty["a"] != before {
		t.Fatal("reacted too early")
	}
	r.tick(1)
	if r.hw.duty["a"] != 60 {
		t.Fatalf("duty %d", r.hw.duty["a"])
	}
}

func TestCurveEndReachedWithoutDistance(t *testing.T) {
	f := plain("a")
	f.RiseBy = 5
	r := newRig(t, f)
	r.temps["water"] = 59
	r.tick(1)
	r.temps["water"] = 60
	r.tick(1)
	if r.hw.duty["a"] != 100 {
		t.Fatalf("duty %d", r.hw.duty["a"])
	}
}

func TestRamp(t *testing.T) {
	f := plain("a")
	f.StepUp, f.StepDown = 3, 2
	f.StartDuty = 0
	r := newRig(t, f)
	r.temps["water"] = 20
	r.tick(10) // start phase over, at 20 %
	r.temps["water"] = 60
	r.tick(2)
	if r.hw.duty["a"] != 26 {
		t.Fatalf("up: %d", r.hw.duty["a"])
	}
	r.temps["water"] = 20
	r.tick(2)
	if r.hw.duty["a"] != 22 {
		t.Fatalf("down: %d", r.hw.duty["a"])
	}
}

func TestStopAndRestart(t *testing.T) {
	f := plain("a")
	f.Curve = Curve{{30, 0}, {40, 8}, {60, 60}}
	f.StepDown, f.StopAt, f.StartDuty = 3, 6, 25
	r := newRig(t, f)
	r.temps["water"] = 50
	r.tick(1)
	r.hw.writes["a"] = nil
	r.tick(5)
	r.temps["water"] = 30
	r.tick(40)
	if r.hw.duty["a"] != 0 {
		t.Fatalf("not stopped: %v", r.hw.writes["a"])
	}
	writes := r.hw.writes["a"]
	if last := writes[len(writes)-2]; last <= 6 {
		t.Fatalf("ramp went below the stop point: %v", writes)
	}
	r.temps["water"] = 40 // wants 8 %, needs the start duty first
	r.hw.rpm["a"] = 0
	r.tick(1)
	if r.hw.duty["a"] != 25 {
		t.Fatalf("start: %d", r.hw.duty["a"])
	}
	r.tick(spinUpTicks - 1)
	if r.hw.duty["a"] != 25 {
		t.Fatalf("start not held: %d", r.hw.duty["a"])
	}
	r.hw.rpm["a"] = 600 // it turns, so there is nothing more to wait for
	r.tick(10)
	if r.hw.duty["a"] != 8 {
		t.Fatalf("after start: %d", r.hw.duty["a"])
	}
}

func TestStalledFanGetsMoreDuty(t *testing.T) {
	r := newRig(t, plain("a"))
	r.hw.rpm["a"] = 0
	r.tick(1)
	start := r.hw.duty["a"]
	r.tick(spinUpTicks + 1)
	if r.hw.duty["a"] != start+2*stallStep {
		t.Fatalf("%v", r.hw.writes["a"])
	}
	r.hw.rpm["a"] = 600
	r.tick(1)
	if r.hw.duty["a"] != start {
		t.Fatalf("not back: %d", r.hw.duty["a"])
	}
}

func TestImplausibleSpeedIsNoStall(t *testing.T) {
	r := newRig(t, plain("a"))
	r.hw.rpm["a"] = highestPlausibleRPM + 1
	r.tick(30)
	if n := len(r.hw.writes["a"]); n != 1 {
		t.Fatalf("%v", r.hw.writes["a"])
	}
}

func TestSensorLoss(t *testing.T) {
	r := newRig(t, plain("a"))
	r.tick(1)
	normal := r.hw.duty["a"]
	delete(r.temps, "water")
	r.tick(sensorGraceTicks - 1)
	if r.hw.duty["a"] != normal {
		t.Fatalf("short gap moved the fan: %d", r.hw.duty["a"])
	}
	if err := r.c.Tick(); !errors.Is(err, ErrSensorLost) {
		t.Fatalf("error %v", err)
	}
	if r.hw.duty["a"] != 90 {
		t.Fatalf("not safe: %d", r.hw.duty["a"])
	}
	r.temps["water"] = 30
	r.tick(1)
	if r.hw.duty["a"] != normal {
		t.Fatalf("no recovery: %d", r.hw.duty["a"])
	}
}

func TestImplausibleTemperatureCountsAsLost(t *testing.T) {
	for _, bad := range []float64{0, -40, 200, math.NaN(), math.Inf(1)} {
		r := newRig(t, plain("a"))
		r.temps["water"] = bad
		if err := r.c.Tick(); !errors.Is(err, ErrSensorLost) {
			t.Fatalf("%v: error %v", bad, err)
		}
		if r.hw.duty["a"] != 90 {
			t.Fatalf("%v: duty %d", bad, r.hw.duty["a"])
		}
	}
}

func TestHottestSourceCountsAndAllAreNeeded(t *testing.T) {
	f := plain("a")
	f.Sources = []string{"cpu", "gpu"}
	r := newRig(t, f)
	r.temps = map[string]float64{"cpu": 30, "gpu": 40}
	r.tick(1)
	if r.hw.duty["a"] != 60 {
		t.Fatalf("duty %d", r.hw.duty["a"])
	}
	delete(r.temps, "gpu")
	for i := 0; i < sensorGraceTicks; i++ {
		_ = r.c.Tick()
	}
	if r.hw.duty["a"] != 90 {
		t.Fatalf("duty %d", r.hw.duty["a"])
	}
}

func TestWritesOnlyOnChange(t *testing.T) {
	r := newRig(t, plain("a"), plain("b"))
	r.tick(50)
	if len(r.hw.writes["a"]) != 1 || r.hw.taken["a"] != 1 || r.hw.taken["b"] != 1 {
		t.Fatalf("%v, %v", r.hw.writes, r.hw.taken)
	}
}

func TestLostHardwareIsLeftAloneThenRestored(t *testing.T) {
	r := newRig(t, plain("a"))
	r.tick(5)
	r.hw.readErr = errors.New("gone")
	for i := 0; i < 3; i++ {
		if err := r.c.Tick(); err == nil {
			t.Fatal("no error")
		}
	}
	if r.hw.taken["a"] != 1 || len(r.hw.writes["a"]) != 1 {
		t.Fatalf("touched while lost: %v", r.hw.writes)
	}
	r.hw.readErr = nil
	r.tick(1)
	if r.hw.taken["a"] != 2 || len(r.hw.writes["a"]) != 2 {
		t.Fatalf("not restored: taken %d, writes %v", r.hw.taken["a"], r.hw.writes)
	}
}

func TestReacquire(t *testing.T) {
	r := newRig(t, plain("a"))
	r.tick(1)
	r.c.Reacquire()
	r.tick(1)
	if r.hw.taken["a"] != 2 || len(r.hw.writes["a"]) != 2 {
		t.Fatalf("taken %d, writes %v", r.hw.taken["a"], r.hw.writes)
	}
}

func TestFailedWriteIsRepeated(t *testing.T) {
	r := newRig(t, plain("a"))
	r.hw.writeErr = errors.New("busy")
	if err := r.c.Tick(); err == nil {
		t.Fatal("no error")
	}
	r.hw.writeErr = nil
	r.tick(1)
	if len(r.hw.writes["a"]) != 1 {
		t.Fatalf("%v", r.hw.writes)
	}
	r.hw.takeErr = errors.New("busy")
	r.c.Reacquire()
	if err := r.c.Tick(); err == nil {
		t.Fatal("no error")
	}
	if len(r.hw.writes["a"]) != 1 {
		t.Fatal("wrote without control")
	}
}

func TestShutdownLeavesSafeDuty(t *testing.T) {
	r := newRig(t, plain("a"), pump("p"))
	r.tick(1)
	if err := r.c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if r.hw.duty["a"] != 90 || r.hw.duty["p"] != 90 {
		t.Fatalf("%v", r.hw.duty)
	}
	n := len(r.hw.writes["a"])
	if err := r.c.Tick(); !errors.Is(err, ErrClosed) {
		t.Fatalf("error %v", err)
	}
	if err := r.c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(r.hw.writes["a"]) != n {
		t.Fatal("wrote after shutdown")
	}
}

func TestShutdownBeforeControlTouchesNothing(t *testing.T) {
	r := newRig(t, plain("a"))
	if err := r.c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if r.hw.taken["a"] != 0 || len(r.hw.writes["a"]) != 0 {
		t.Fatalf("%v %v", r.hw.taken, r.hw.writes)
	}
}

func TestRunEndsWithShutdown(t *testing.T) {
	r := newRig(t, plain("a"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.c.Run(ctx, time.Millisecond, nil)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if r.hw.duty["a"] != 90 {
		t.Fatalf("duty %d", r.hw.duty["a"])
	}
}

func TestPumpNeverStops(t *testing.T) {
	p := pump("p")
	p.Curve = Curve{{20, 0}, {60, 0}}
	r := newRig(t, p)
	r.tick(20)
	for _, d := range r.hw.writes["p"] {
		if d < 40 {
			t.Fatalf("%v", r.hw.writes["p"])
		}
	}
	if r.hw.duty["p"] != 40 {
		t.Fatalf("duty %d", r.hw.duty["p"])
	}
}

func TestPumpWithoutACurveIsStillWatched(t *testing.T) {
	p, f := pump("p"), plain("a")
	p.Curve, p.Sources, p.RiseAfter, p.FallAfter = nil, nil, 0, 0
	r := newRig(t, p, f)
	r.tick(1)
	if got := r.c.Status()[0]; got.HasTemp || got.PumpDown {
		t.Fatalf("%+v", got)
	}
	normal := r.hw.duty["a"]
	if normal >= f.SafeDuty {
		t.Fatalf("test setup: %d", normal)
	}
	r.hw.rpm["p"] = 0
	var err error
	for i := 0; i < spinUpTicks; i++ {
		err = r.c.Tick()
	}
	if !errors.Is(err, ErrPumpStopped) || !r.c.Status()[0].PumpDown {
		t.Fatalf("error %v, status %+v", err, r.c.Status()[0])
	}
	if r.hw.duty["a"] != f.SafeDuty {
		t.Fatalf("%v", r.hw.duty)
	}
}

// A fan without a curve keeps needing one.
func TestOnlyAPumpGoesWithoutACurve(t *testing.T) {
	f := plain("a")
	f.Curve = nil
	if _, err := New([]Fan{f}, nil, nil); err == nil {
		t.Fatal("accepted")
	}
}

func TestPumpStopRaisesAlarmAndFans(t *testing.T) {
	p, f := pump("p"), plain("a")
	r := newRig(t, p, f)
	r.tick(1)
	normal := r.hw.duty["a"]
	if normal >= f.SafeDuty {
		t.Fatalf("test setup: %d", normal)
	}
	r.hw.rpm["p"] = 0
	var err error
	for i := 0; i < spinUpTicks; i++ {
		err = r.c.Tick()
	}
	if !errors.Is(err, ErrPumpStopped) {
		t.Fatalf("error %v", err)
	}
	if r.hw.duty["p"] != 100 || r.hw.duty["a"] != f.SafeDuty {
		t.Fatalf("%v", r.hw.duty)
	}
	delete(r.hw.rpm, "p")
	r.tick(1)
	if r.hw.duty["a"] != normal {
		t.Fatalf("no recovery: %v", r.hw.duty)
	}
}

func TestSlowPumpCountsAsStopped(t *testing.T) {
	p := pump("p")
	p.StoppedBelowRPM = 800
	r := newRig(t, p)
	r.hw.rpm["p"] = 500
	r.tick(1) // the first tick starts the pump
	var err error
	for i := 0; i < spinUpTicks; i++ {
		err = r.c.Tick()
	}
	if !errors.Is(err, ErrPumpStopped) {
		t.Fatalf("error %v", err)
	}
}

func TestPumpStopIsSeenWithoutSensor(t *testing.T) {
	p, f := pump("p"), plain("a")
	r := newRig(t, p, f)
	r.tick(1)
	delete(r.temps, "water")
	r.hw.rpm["p"] = 0
	var err error
	for i := 0; i < spinUpTicks; i++ {
		err = r.c.Tick()
	}
	if !errors.Is(err, ErrPumpStopped) || !errors.Is(err, ErrSensorLost) {
		t.Fatalf("error %v", err)
	}
	if r.hw.duty["p"] != 100 || r.hw.duty["a"] != f.SafeDuty {
		t.Fatalf("%v", r.hw.duty)
	}
}

func TestPumpWithoutSpeedSensorRaisesNoAlarm(t *testing.T) {
	p := pump("p")
	r := newRig(t, p)
	delete(r.hw.followers, "p")
	r.tick(30)
	if r.hw.duty["p"] == 100 {
		t.Fatal("unknown speed taken for a stopped pump")
	}
}

func TestFanStallIsNoPumpAlarm(t *testing.T) {
	r := newRig(t, plain("a"))
	r.hw.rpm["a"] = 0
	r.tick(30) // Tick would fail on any alarm
}

func TestLeadFollowsRisingTrend(t *testing.T) {
	ahead := plain("ahead")
	ahead.LeadTicks = 120
	r := newRig(t, plain("now"), ahead)
	for i := 0; i < 60; i++ {
		r.temps["water"] = 30 + float64(i)*0.02 // whole system warms slowly
		r.tick(1)
	}
	now, lead := r.hw.duty["now"], r.hw.duty["ahead"]
	if lead-now < 3 || lead-now > 7 { // 2.4 °C at 2 %/°C
		t.Fatalf("now %d, ahead %d", now, lead)
	}
}

func TestLeadIsCapped(t *testing.T) {
	ahead := plain("ahead")
	ahead.LeadTicks = 10000
	r := newRig(t, plain("now"), ahead)
	for i := 0; i < 60; i++ {
		r.temps["water"] = 30 + float64(i)*0.1
		r.tick(1)
	}
	if d := r.hw.duty["ahead"] - r.hw.duty["now"]; d > maxLeadDegrees*2+1 {
		t.Fatalf("difference %d", d)
	}
}

func TestLeadIgnoresFallAndConstant(t *testing.T) {
	ahead := plain("ahead")
	ahead.LeadTicks = 120
	r := newRig(t, plain("now"), ahead)
	r.tick(60)
	if r.hw.duty["ahead"] != r.hw.duty["now"] {
		t.Fatalf("constant: %v", r.hw.duty)
	}
	for i := 0; i < 60; i++ {
		r.temps["water"] = 30 - float64(i)*0.05
		r.tick(1)
	}
	if r.hw.duty["ahead"] != r.hw.duty["now"] {
		t.Fatalf("falling: %v", r.hw.duty)
	}
}

func TestLeadIsCalmOnWholeDegrees(t *testing.T) {
	ahead := plain("ahead")
	ahead.LeadTicks = 120
	ahead.RiseBy, ahead.FallBy = 1, 2
	r := newRig(t, ahead)
	r.temps["water"] = 35
	r.tick(60)
	r.hw.writes["ahead"] = nil
	for i := 0; i < 600; i++ { // the reading flips between two whole degrees
		r.temps["water"] = 35 + float64(i/7%2)
		r.tick(1)
	}
	if n := len(r.hw.writes["ahead"]); n > 12 {
		t.Fatalf("%d writes in 10 minutes", n)
	}
}

// loop models a closed water loop: heat goes in from the load and leaves
// through the radiator, in proportion to the fans' speed and the distance to
// the room. The water is slow, which is why it is worth modelling.
type loop struct {
	water, room, load float64
	capacity          float64 // J/K
	perRPM            float64 // W/K per rpm over all fans
	hw                *bench
	fans              []string
}

func (l *loop) second() {
	var rpm float64
	speeds, _ := l.hw.Speeds()
	for _, id := range l.fans {
		rpm += float64(speeds[id])
	}
	l.water += (l.load - l.perRPM*rpm*(l.water-l.room)) / l.capacity
}

func TestSimulatedLoopSettles(t *testing.T) {
	fans := []Fan{}
	ids := []string{"top1", "top2", "top3", "side1", "side2", "side3"}
	for i, id := range ids {
		f := plain(id)
		f.Curve = Curve{{28, 500}, {32, 500}, {35, 800}, {38, 1100}, {42, 1450}, {46, 1900}}
		f.RPMCurve = true
		f.Speeds = SpeedMap{{12, 240 + i*10}, {50, 1200 + i*10}, {100, 2100}}
		f.RiseBy, f.FallBy, f.RiseAfter, f.FallAfter = 1, 2, 4, 12
		f.StepUp, f.StepDown = 3, 3
		f.MinDuty, f.StartDuty, f.StopAt = 15, 15, 6
		f.LeadTicks = 60
		fans = append(fans, f)
	}
	p := pump("pump")
	p.Curve = Curve{{20, 60}, {40, 60}, {45, 75}, {48, 100}}
	p.StepUp, p.StepDown = 2, 2
	fans = append(fans, p)

	hw := newBench()
	for _, id := range ids {
		hw.followers[id] = 21
	}
	hw.followers["pump"] = 45
	l := &loop{water: 28, room: 22, load: 500, capacity: 12000, perRPM: 0.0031, hw: hw, fans: ids}
	temps := map[string]float64{}
	c, err := New(fans, hw, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	run := func(seconds int) (peak float64) {
		for i := 0; i < seconds; i++ {
			temps["water"] = math.Round(l.water) // sensors report whole degrees
			if err := c.Tick(); err != nil {
				t.Fatal(err)
			}
			l.second()
			peak = max(peak, l.water)
		}
		return peak
	}

	peak := run(1800) // half an hour of load
	if peak > 46 {
		t.Errorf("water reached %.1f °C", peak)
	}
	if l.water < 30 {
		t.Errorf("fans were pointlessly strong: water %.1f °C", l.water)
	}
	for id := range hw.writes {
		hw.writes[id] = nil
	}
	run(600) // ten more minutes at the same load: it should be quiet
	for id, w := range hw.writes {
		if len(w) > 40 {
			t.Errorf("%s changed %d times in 10 minutes", id, len(w))
		}
	}
	l.load = 60
	run(1800)
	if l.water > 33 {
		t.Errorf("no cooldown: water %.1f °C", l.water)
	}
	if hw.duty["pump"] < 40 {
		t.Errorf("pump at %d", hw.duty["pump"])
	}
}

func TestSimulatedPumpFailureKeepsFansUp(t *testing.T) {
	f := plain("rad")
	p := pump("pump")
	hw := newBench()
	hw.followers["rad"] = 20
	hw.followers["pump"] = 40
	temps := map[string]float64{"water": 35}
	c, err := New([]Fan{f, p}, hw, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := c.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	hw.rpm["pump"] = 0
	alarms := 0
	for i := 0; i < 20; i++ {
		if errors.Is(c.Tick(), ErrPumpStopped) {
			alarms++
		}
	}
	if alarms != 20-spinUpTicks+1 || hw.duty["rad"] != f.SafeDuty {
		t.Fatalf("%d alarms, fan at %d", alarms, hw.duty["rad"])
	}
}

func TestStatusReportsTheDecision(t *testing.T) {
	f := Fan{ID: "a", Sources: []string{"t"}, Curve: Curve{{20, 500}, {40, 1500}},
		RPMCurve: true, Speeds: SpeedMap{{20, 500}, {100, 1500}}, RiseAfter: 1, FallAfter: 1, StepUp: 100, StepDown: 100, SafeDuty: 80}
	temps := map[string]float64{"t": 30}
	c, err := New([]Fan{f}, NoOutput{}, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Status(); len(got) != 1 || got[0].ID != "a" || got[0].HasTemp {
		t.Fatalf("before the first tick: %+v", got)
	}
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	got := c.Status()[0]
	if !got.HasTemp || got.Temp != 30 || got.Duty != 60 || got.TargetRPM != 1000 || got.HasRPM || got.SensorLost {
		t.Fatalf("after a tick: %+v", got)
	}
	delete(temps, "t")
	for range sensorGraceTicks {
		_ = c.Tick()
	}
	if got := c.Status()[0]; got.HasTemp || !got.SensorLost || got.Duty != 80 {
		t.Fatalf("without a sensor: %+v", got)
	}
}

func TestNoOutputStillSeesAStoppedPump(t *testing.T) {
	p := Fan{ID: "p", Pump: true, Sources: []string{"t"}, Curve: Curve{{20, 60}, {40, 60}}, MinDuty: 60,
		RiseAfter: 1, FallAfter: 1, StepUp: 100, StepDown: 100, SafeDuty: 100}
	hw := NoOutput{RPM: func() map[string]int { return map[string]int{"p": 0} }}
	c, err := New([]Fan{p}, hw, func() map[string]float64 { return map[string]float64{"t": 30} })
	if err != nil {
		t.Fatal(err)
	}
	var last error
	for range spinUpTicks + 1 {
		last = c.Tick()
	}
	if !errors.Is(last, ErrPumpStopped) || !c.Status()[0].PumpDown || c.Status()[0].Duty != 100 {
		t.Fatalf("err=%v status=%+v", last, c.Status()[0])
	}
}

func TestSpeedMapRPMFor(t *testing.T) {
	m := SpeedMap{{20, 500}, {50, 800}, {100, 800}}
	for duty, want := range map[int]int{0: 0, 10: 500, 20: 500, 35: 650, 50: 800, 100: 800} {
		if got := m.rpmFor(duty); got != want {
			t.Errorf("rpmFor(%d) = %d, want %d", duty, got, want)
		}
	}
}

func TestValidateRejectsSettingsThatDefeatTheRules(t *testing.T) {
	good := Fan{ID: "a", Sources: []string{"t"}, Curve: Curve{{20, 50}, {40, 100}}, RiseAfter: 1, FallAfter: 1,
		StepUp: 1, StepDown: 1, MinDuty: 20, StopAt: 6, SafeDuty: 70}
	if err := checkOwn(good); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Fan){
		"huge value":     func(f *Fan) { f.Curve = Curve{{20, 0}, {100, 1e19}} },
		"huge temp":      func(f *Fan) { f.Curve = Curve{{-1e300, 0}, {1e300, 50}} },
		"falling":        func(f *Fan) { f.Curve = Curve{{20, 100}, {40, 50}} },
		"always zero":    func(f *Fan) { f.Curve, f.MinDuty, f.StopAt = Curve{{20, 0}, {40, 0}}, 0, 0 },
		"safe below min": func(f *Fan) { f.SafeDuty = 10 },
		"stop above min": func(f *Fan) { f.StopAt = 20 },
		"endless delay":  func(f *Fan) { f.RiseAfter = longestDelayTicks + 1 },
		"endless fall":   func(f *Fan) { f.FallAfter = longestDelayTicks + 1 },
		"huge distance":  func(f *Fan) { f.RiseBy = widestDistance + 1 },
		"huge map rpm":   func(f *Fan) { f.Speeds = SpeedMap{{1, 1}, {100, highestPlausibleRPM + 1}} },
		"rpm over map":   func(f *Fan) { f.Speeds = SpeedMap{{1, 1}, {100, 2}}; f.Curve = Curve{{20, 1}, {40, 1e12}} },
	} {
		f := good
		change(&f)
		if err := checkOwn(f); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLostSensorNeverLowersAnOutput(t *testing.T) {
	f := Fan{ID: "a", Sources: []string{"t"}, Curve: Curve{{20, 100}, {40, 100}}, RiseAfter: 1, FallAfter: 1,
		StepUp: 100, StepDown: 100, SafeDuty: 70}
	temps := map[string]float64{"t": 30}
	c, err := New([]Fan{f}, NoOutput{}, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Tick()
	temps["t"] = highestPlausibleTemp + 1 // too hot counts as no reading
	for range sensorGraceTicks + 1 {
		_ = c.Tick()
	}
	if got := c.Status()[0]; !got.SensorLost || got.Duty != 100 {
		t.Fatalf("%+v", got)
	}
}
