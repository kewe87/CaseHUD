package cooling

import (
	"slices"
	"strings"
	"testing"
)

func rampFan(id string, up, down int) Fan {
	return Fan{
		ID: id, Sources: []string{"t"}, Curve: Curve{{20, 30}, {60, 100}},
		RiseAfter: 1, FallAfter: 1, StepUp: up, StepDown: down, SafeDuty: 30,
	}
}

func steady(t *testing.T, f Fan, temp float64) (*Controller, *bench, map[string]float64) {
	t.Helper()
	temps := map[string]float64{"t": temp}
	b := newBench()
	b.rpm[f.ID] = 1000
	c, err := New([]Fan{f}, b, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	return c, b, temps
}

// A fan held while its curve moves on must not jump to the new value on release:
// it ramps from the duty it really has.
func TestReleaseRampsFromTheDutyOnTheOutput(t *testing.T) {
	c, b, temps := steady(t, rampFan("f", 10, 10), 20)
	c.Tick()
	if got := b.writes["f"]; !slices.Equal(got, []int{30}) {
		t.Fatalf("start: %v", got)
	}
	c.Hold("f")
	temps["t"] = 60 // the curve asks for 100 meanwhile
	for range 20 {
		c.Tick()
	}
	c.Release("f", 30)
	for range 3 {
		c.Tick()
	}
	if got := b.writes["f"]; !slices.Equal(got, []int{30, 40, 50, 60}) {
		t.Fatalf("no ramp after the release: %v", got)
	}
}

func TestReleaseRampsDownByStepDown(t *testing.T) {
	f := rampFan("p", 10, 2)
	f.Pump, f.MinDuty, f.SafeDuty = true, 30, 100
	c, b, temps := steady(t, f, 60)
	c.Tick()
	c.Hold("p")
	temps["t"] = 20
	for range 20 {
		c.Tick()
	}
	c.Release("p", 100)
	for range 3 {
		c.Tick()
	}
	w := b.writes["p"]
	if !slices.Equal(w[len(w)-3:], []int{98, 96, 94}) {
		t.Fatalf("pump fell faster than its step: %v", w)
	}
}

// Without a known duty the output is taken to stand at full duty, so it only
// comes down by its steps.
func TestReleaseWithoutADutyFallsByStepsOnly(t *testing.T) {
	c, b, temps := steady(t, rampFan("f", 10, 5), 20)
	temps["t"] = 20
	c.Hold("f")
	c.Tick()
	c.Release("f", 0)
	c.Tick()
	c.Tick()
	if got := b.writes["f"]; !slices.Equal(got, []int{95, 90}) {
		t.Fatalf("%v", got)
	}
}

// A new controller that takes over knows what the outputs stand at and ramps.
func TestAControllerTakingOverRampsFromTheOldDuties(t *testing.T) {
	c, b, _ := steady(t, rampFan("f", 100, 2), 60)
	c.Tick()
	if d := c.Duties(); d["f"] != 100 {
		t.Fatalf("duties %v", d)
	}
	temps := map[string]float64{"t": 20}
	b2 := newBench()
	b2.rpm["f"] = 1000
	next, err := New([]Fan{rampFan("f", 100, 2)}, b2, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	next.StartAt(c.Duties())
	for range 3 {
		next.Tick()
	}
	if got := b2.writes["f"]; !slices.Equal(got, []int{98, 96, 94}) {
		t.Fatalf("the new controller jumped: %v", got)
	}
	_ = b
}

// Without known duties the first tick still goes straight to the wanted duty.
func TestAControllerWithoutDutiesStartsAtTheWantedOne(t *testing.T) {
	c, b, _ := steady(t, rampFan("f", 100, 2), 60)
	c.Tick()
	if got := b.writes["f"]; !slices.Equal(got, []int{100}) {
		t.Fatalf("%v", got)
	}
}

// A release that comes while the output is held again waits for the next one.
func TestAReleaseOfAnOutputHeldAgainIsKept(t *testing.T) {
	c, b, temps := steady(t, rampFan("f", 10, 10), 20)
	c.Tick()
	c.Hold("f")
	c.Release("f", 30)
	c.Hold("f")
	temps["t"] = 60
	c.Tick()
	c.Tick()
	c.Release("f", 30)
	c.Tick()
	if got := b.writes["f"]; !slices.Equal(got, []int{30, 40}) {
		t.Fatalf("%v", got)
	}
}

// A follower in rpm must not take over the speed a calibration forces on the
// output it follows.
func TestAFollowerIgnoresTheSpeedOfAHeldOutput(t *testing.T) {
	temp := map[string]float64{"t": 40}
	h := newSafetyHW()
	h.rpm["lead"], h.rpm["b"] = 800, 800
	lead := CurveSpec{ID: "d", RPM: true, Kind: KindFixed, Value: 800}
	follow := CurveSpec{ID: "f", RPM: true, Kind: KindFollow, Follow: "lead"}
	a, b := safetyFan("lead", "d"), safetyFan("b", "f")
	a.Speeds = SpeedMap{{Duty: 20, RPM: 400}, {Duty: 100, RPM: 1600}}
	b.Speeds = a.Speeds
	c, err := NewShared([]CurveSpec{lead, follow}, []Fan{a, b}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	want := h.duty["b"]
	c.Hold("lead")
	h.rpm["lead"] = 0 // the calibration stops it on purpose
	for range 3 * sensorGraceTicks {
		c.Tick()
	}
	if s := c.Status()[1]; s.SensorLost || s.Duty != want || h.duty["b"] != want {
		t.Fatalf("follower disturbed: duty %d (written %d), want %d, %+v", s.Duty, h.duty["b"], want, s)
	}
	h.rpm["lead"] = 200 // a slow measurement step
	c.Tick()
	if h.duty["b"] != want {
		t.Fatalf("follower copied the measurement: %d", h.duty["b"])
	}
}

// For a curve in rpm the target is the curve's own speed, not what the duty gives
// back from the speed table.
func TestTargetRPMIsTheCurvesOwnSpeed(t *testing.T) {
	temp := map[string]float64{"t": 40}
	h := newSafetyHW()
	h.rpm["a"] = 1000
	curve := CurveSpec{ID: "d", RPM: true, Kind: KindFixed, Value: 777}
	a := safetyFan("a", "d")
	a.Speeds = SpeedMap{{Duty: 20, RPM: 400}, {Duty: 100, RPM: 1600}}
	c, err := NewShared([]CurveSpec{curve}, []Fan{a}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	s := c.Status()[0]
	if s.TargetRPM != 777 {
		t.Fatalf("target %d, duty %d", s.TargetRPM, s.Duty)
	}
	// Percent curve: the speed the table gives for the duty.
	pc := rampFan("p", 100, 100)
	pc.Speeds = a.Speeds
	c2, _, _ := steady(t, pc, 40)
	c2.Tick()
	s = c2.Status()[0]
	if want := pc.Speeds.rpmFor(s.Duty); s.TargetRPM != want {
		t.Fatalf("percent curve target %d, want %d", s.TargetRPM, want)
	}
}

func TestAPumpIsNeverSetBelowTheFloor(t *testing.T) {
	f := rampFan("p", 100, 100)
	f.Pump, f.SafeDuty = true, 100
	for _, min := range []int{0, 1, LowestPumpDuty - 1} {
		f.MinDuty = min
		_, err := New([]Fan{f}, newBench(), func() map[string]float64 { return nil })
		if err == nil || !strings.Contains(err.Error(), "pump") {
			t.Fatalf("MinDuty %d accepted: %v", min, err)
		}
	}
	f.MinDuty = LowestPumpDuty
	c, b, _ := steady(t, f, 20)
	c.Tick()
	if w := b.writes["p"]; len(w) == 0 || slices.Min(w) < LowestPumpDuty {
		t.Fatalf("%v", w)
	}
}
