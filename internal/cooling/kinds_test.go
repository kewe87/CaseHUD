package cooling

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func graphSpec() CurveSpec {
	return CurveSpec{ID: "g", Sources: []string{"t"}, Points: Curve{{Temp: 20, Value: 20}, {Temp: 60, Value: 60}}, RiseAfter: 1, FallAfter: 1}
}

func feed(t *testing.T, e evaluator, temps []float64) []float64 {
	t.Helper()
	out := make([]float64, len(temps))
	for i, x := range temps {
		lost := math.IsNaN(x)
		v, ok := e.advance(x, !lost)
		if ok == lost {
			t.Fatalf("tick %d: ok=%v for reading %v", i, ok, x)
		}
		out[i] = v
	}
	return out
}

func sameValues(t *testing.T, got, want []float64) {
	t.Helper()
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("tick %d: got %v, want %v", i, got, want)
		}
	}
}

var lost = math.NaN()

func TestDrawnHoldsSmallChangesBack(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy = 2, 2
	sameValues(t, feed(t, newEvaluator(s), []float64{40, 41, 39, 42, 40}), []float64{40, 40, 40, 42, 40})
}

func TestDrawnWaitsForItsDelays(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy, s.RiseAfter, s.FallAfter = 2, 2, 2, 3
	sameValues(t, feed(t, newEvaluator(s), []float64{40, 45, 40, 45, 45, 40, 40, 40}), []float64{40, 40, 40, 40, 45, 45, 45, 40})
}

func TestDrawnTakesTheFirstReadingAfterAGapAtOnce(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy = 5, 5
	sameValues(t, feed(t, newEvaluator(s), []float64{40, lost, 41, 42}), []float64{40, 0, 41, 41})
}

// Falling below the curve must not skip the wait a falling temperature has to
// keep: a long delay is there to keep the fans from dropping.
func TestGoingBeyondTheLowEndKeepsTheFallDelay(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy, s.RiseAfter, s.FallAfter = 2, 2, 1, 3
	sameValues(t, feed(t, newEvaluator(s), []float64{40, 10, 10, 10}), []float64{40, 40, 40, 20})
}

func TestTheEndsNeedNoDistanceUnlessAskedTo(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy = 5, 5
	sameValues(t, feed(t, newEvaluator(s), []float64{58, 60}), []float64{58, 60})
	s.DistanceAtEnds = true
	sameValues(t, feed(t, newEvaluator(s), []float64{58, 60}), []float64{58, 58})
}

// Far beyond the end the temperature can wander without leaving a mark: the
// distance on the way back counts from the end, not from the peak.
func TestWanderingBeyondTheEndLeavesNoStaleReference(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.FallBy, s.FallAfter = 3, 3, 1
	sameValues(t, feed(t, newEvaluator(s), []float64{90, 60, 59}), []float64{60, 60, 60})
}

func TestSwitchSwitchesAtItsThresholds(t *testing.T) {
	s := CurveSpec{Kind: KindSwitch, IdleTemp: 40, LoadTemp: 60, IdleValue: 25, LoadValue: 80, RiseAfter: 2, FallAfter: 3}
	sameValues(t, feed(t, newEvaluator(s), []float64{50, 60, 59, 60, 61, 50, 40, 40, 39, 39}),
		[]float64{25, 25, 25, 25, 80, 80, 80, 80, 25, 25})
}

func TestSwitchStartsOnLoadOnlyAboveTheThreshold(t *testing.T) {
	s := CurveSpec{Kind: KindSwitch, IdleTemp: 40, LoadTemp: 60, IdleValue: 25, LoadValue: 80, RiseAfter: 1, FallAfter: 1}
	sameValues(t, feed(t, newEvaluator(s), []float64{61}), []float64{80})
	sameValues(t, feed(t, newEvaluator(s), []float64{60}), []float64{25})
}

// After a lost sensor the curve must still reach its load value when hot.
func TestSwitchSwitchesUpAfterASensorLoss(t *testing.T) {
	s := CurveSpec{Kind: KindSwitch, IdleTemp: 40, LoadTemp: 60, IdleValue: 25, LoadValue: 80, RiseAfter: 1, FallAfter: 1}
	e := newEvaluator(s)
	sameValues(t, feed(t, e, []float64{70, lost, 50}), []float64{80, 0, 80}) // keeps its state on the way back
	sameValues(t, feed(t, e, []float64{30, lost, 50, 70}), []float64{25, 0, 25, 80})
}

func TestFixedNeedsNoSensor(t *testing.T) {
	c, err := NewShared([]CurveSpec{{ID: "f", Kind: KindFixed, Value: 44}},
		[]Fan{{ID: "a", CurveID: "f", StepUp: 100, StepDown: 100, SafeDuty: 70}}, NoOutput{},
		func() map[string]float64 { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	if s := c.Status()[0]; s.Duty != 44 || s.SensorLost || s.HasTemp {
		t.Fatalf("%+v", s)
	}
}

func TestOffsetCountsBeforeTheMinimum(t *testing.T) {
	f := validFan()
	f.Curve, f.Offset, f.MinDuty, f.StepUp, f.StepDown = Curve{{Temp: 30, Value: 30}, {Temp: 80, Value: 30}}, -20, 15, 100, 100
	if d := ownRegulator(f).step(40, true, 1000, true).duty; d != 15 {
		t.Fatalf("%d", d)
	}
	f.Offset = 200
	if err := checkOwn(f); err == nil {
		t.Fatal("a huge offset was accepted")
	}
	f.Offset = 90
	if d := ownRegulator(f).step(40, true, 1000, true).duty; d != 100 {
		t.Fatalf("%d", d)
	}
}

// Without a speed sensor nobody can tell that it runs, so the start is held out.
func TestStartWithoutASpeedSensorIsHeldOut(t *testing.T) {
	f := validFan()
	f.Curve, f.StopAt, f.StartDuty, f.StepUp, f.StepDown = Curve{{Temp: 30, Value: 10}, {Temp: 80, Value: 10}}, 5, 40, 100, 100
	r := ownRegulator(f)
	for i := range spinUpTicks {
		if d := r.step(40, true, 0, false).duty; d != 40 {
			t.Fatalf("tick %d: %d", i, d)
		}
	}
	if d := r.step(40, true, 0, false).duty; d != 10 {
		t.Fatalf("after the start: %d", d)
	}
}

// A fan that reads 0 while running high is not taken down to the start duty.
func TestAStandingFanIsNotLowered(t *testing.T) {
	f := validFan()
	f.Curve, f.StopAt, f.StartDuty, f.StepUp, f.StepDown = Curve{{Temp: 30, Value: 80}, {Temp: 80, Value: 80}}, 5, 40, 100, 100
	r := ownRegulator(f)
	for range spinUpTicks + 1 {
		r.step(40, true, 1000, true)
	}
	for i := range spinUpTicks {
		if d := r.step(40, true, 0, true).duty; d != 80 {
			t.Fatalf("tick %d: %d", i, d)
		}
	}
	if d := r.step(40, true, 0, true).duty; d != 80+stallStep {
		t.Fatalf("after waiting: %d", d)
	}
}

func TestCombine(t *testing.T) {
	in := []float64{50, 30, 40}
	for f, want := range map[CombineFunc]float64{CombineMax: 50, CombineMin: 30, CombineSum: 100, CombineAverage: 40, CombineSubtract: 0} {
		if got := combine(f, in, 100); got != want {
			t.Errorf("%s: %v", f, got)
		}
	}
	if got := combine(CombineSum, []float64{80, 70}, 100); got != 100 {
		t.Errorf("sum over the top: %v", got)
	}
	if got := combine(CombineSubtract, []float64{70, 20}, 100); got != 50 {
		t.Errorf("subtract: %v", got)
	}
}

func TestFollowingStaysInRange(t *testing.T) {
	for _, c := range []struct {
		offset float64
		prop   bool
		in     float64
		want   float64
	}{{10, false, 50, 60}, {-60, false, 50, 0}, {60, false, 50, 100}, {20, true, 50, 60}, {-150, true, 50, 0}, {200, true, 50, 100}} {
		s := CurveSpec{Kind: KindFollow, Offset: c.offset, Proportional: c.prop}
		if got := following(s, c.in); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func fastFan(id, curve string) Fan {
	return Fan{ID: id, CurveID: curve, StepUp: 100, StepDown: 100, SafeDuty: 70}
}

func TestCombinedFollowsItsInputsAndAnyLostOneCounts(t *testing.T) {
	a, b := graphSpec(), graphSpec()
	a.ID, b.ID = "a", "b"
	a.Sources, b.Sources = []string{"ta"}, []string{"tb"}
	combined := CurveSpec{ID: "m", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"a", "b"}}
	temps := map[string]float64{"ta": 30, "tb": 50}
	c, err := NewShared([]CurveSpec{combined, a, b}, []Fan{fastFan("fan", "m")}, NoOutput{}, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	if _ = c.Tick(); c.Status()[0].Duty != 50 {
		t.Fatalf("%+v", c.Status()[0])
	}
	if s := c.Status()[0]; !s.HasTemp || s.Temp != 50 {
		t.Fatalf("temperature of the hottest input expected: %+v", s)
	}
	delete(temps, "tb")
	var last error
	for range sensorGraceTicks {
		last = c.Tick()
	}
	if s := c.Status()[0]; !errors.Is(last, ErrSensorLost) || s.Duty != 70 {
		t.Fatalf("a lost input must not leave the fan on the others: err=%v %+v", last, s)
	}
}

func TestEmergencyReachesThroughACombined(t *testing.T) {
	a, f := graphSpec(), CurveSpec{ID: "f", Kind: KindFixed, Value: 30}
	a.ID, a.Points = "a", Curve{{Temp: 20, Value: 20}, {Temp: 125, Value: 20}}
	combined := CurveSpec{ID: "m", Kind: KindCombined, Combine: CombineMin, Inputs: []string{"a", "f"}}
	c, err := NewShared([]CurveSpec{a, f, combined}, []Fan{fastFan("fan", "m")}, NoOutput{},
		func() map[string]float64 { return map[string]float64{"t": emergencyTemp} })
	if err != nil {
		t.Fatal(err)
	}
	for range emergencyTicks {
		_ = c.Tick()
	}
	if s := c.Status()[0]; s.Duty != 100 {
		t.Fatalf("%+v", s)
	}
}

func TestFollowFollowsTheDecisionOfTheSameTick(t *testing.T) {
	g := graphSpec()
	followCurve := CurveSpec{ID: "s", Kind: KindFollow, Follow: "lead", Offset: -10}
	temps := map[string]float64{"t": 40}
	// The follower comes first in both lists; the order must not matter.
	c, err := NewShared([]CurveSpec{followCurve, g}, []Fan{fastFan("follower", "s"), fastFan("lead", "g")}, NoOutput{},
		func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	st := c.Status()
	if st[0].Duty != 30 || st[1].Duty != 40 {
		t.Fatalf("%+v", st)
	}
	temps["t"] = 50
	_ = c.Tick()
	if st = c.Status(); st[0].Duty != 40 || st[1].Duty != 50 {
		t.Fatalf("%+v", st)
	}
}

func TestFollowInRPMFollowsTheMeasuredSpeed(t *testing.T) {
	g := graphSpec()
	followCurve := CurveSpec{ID: "s", Kind: KindFollow, RPM: true, Follow: "lead", Offset: 100}
	follower := fastFan("follower", "s")
	follower.Speeds = SpeedMap{{Duty: 10, RPM: 500}, {Duty: 100, RPM: 1500}}
	hw := NoOutput{RPM: func() map[string]int { return map[string]int{"lead": 900} }}
	c, err := NewShared([]CurveSpec{g, followCurve}, []Fan{fastFan("lead", "g"), follower}, hw,
		func() map[string]float64 { return map[string]float64{"t": 40} })
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Tick()
	if got, want := c.Status()[1].Duty, follower.Speeds.dutyFor(1000); got != want {
		t.Fatalf("%d, want %d", got, want)
	}
}

func TestOutputsOnOneCurveShareItsState(t *testing.T) {
	s := graphSpec()
	s.RiseBy, s.RiseAfter = 0, 3
	temps := map[string]float64{"t": 30}
	c, err := NewShared([]CurveSpec{s}, []Fan{fastFan("a", "g"), fastFan("b", "g")}, NoOutput{},
		func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Tick()
	temps["t"] = 50
	for i := range 3 {
		_ = c.Tick()
		st := c.Status()
		want := 30
		if i == 2 {
			want = 50 // one tick of the curve per tick of the loop, not one per output
		}
		if st[0].Duty != want || st[1].Duty != want {
			t.Fatalf("tick %d: %+v", i, st)
		}
	}
}

func TestThermostatStaysBetweenItsLimitsAndClimbsWhileTooHot(t *testing.T) {
	s := CurveSpec{Kind: KindThermostat, IdleTemp: 30, LoadTemp: 60, IdleValue: 20, LoadValue: 90, Step: 5, Deadband: 3, ReactAfter: 1}
	e := newEvaluator(s)
	if v, _ := e.advance(25, true); v != 20 {
		t.Fatalf("idle start: %v", v)
	}
	last := 0.0
	for range 40 {
		v, ok := e.advance(70, true)
		if !ok || v < 20 || v > 90 || v < last {
			t.Fatalf("while too hot: %v after %v", v, last)
		}
		last = v
	}
	if last != 90 {
		t.Fatalf("did not reach the top: %v", last)
	}
}

func TestThermostatKeepsItsValueOverAGap(t *testing.T) {
	s := CurveSpec{Kind: KindThermostat, IdleTemp: 30, LoadTemp: 60, IdleValue: 20, LoadValue: 90, Step: 5, Deadband: 3, ReactAfter: 1}
	e := newEvaluator(s)
	e.advance(25, true)
	for range 10 {
		e.advance(70, true)
	}
	before, _ := e.advance(70, true)
	if _, ok := e.advance(0, false); ok {
		t.Fatal("a value without a reading")
	}
	after, ok := e.advance(55, true)
	if !ok || after != before {
		t.Fatalf("%v then %v", before, after)
	}
}

func TestSlopeTrendSeesDirectionsOnlyWithAFullWindow(t *testing.T) {
	tr := newSlopeTrend(3)
	for i, v := range []float32{10, 12, 14} {
		if d := tr.push(v); d != trendUnknown {
			t.Fatalf("reading %d: %d", i, d)
		}
	}
	if d := tr.push(16); d != trendUp {
		t.Fatalf("rising: %d", d)
	}
	for _, v := range []float32{12, 8} {
		tr.push(v)
	}
	if d := tr.push(4); d != trendDown {
		t.Fatalf("falling: %d", d)
	}
	for range 3 {
		tr.push(4)
	}
	if d := tr.push(4); d != trendFlat {
		t.Fatalf("fixed: %d", d)
	}
}

// Too small for a verdict at once, but adding up over time.
func TestSlopeTrendNoticesASlowDrift(t *testing.T) {
	tr := newSlopeTrend(3)
	v, up := float32(40), false
	for range 40 {
		v += 0.25
		up = up || tr.push(v) == trendUp
	}
	if !up {
		t.Fatal("drift went unnoticed")
	}
}

func TestCheckCurvesRefuses(t *testing.T) {
	fixed := CurveSpec{ID: "f", Kind: KindFixed, Value: 50}
	for name, c := range map[string]struct {
		curves []CurveSpec
		want   string
	}{
		"unknown kind":        {[]CurveSpec{{ID: "x", Kind: "other"}}, "unknown kind"},
		"unknown way":         {[]CurveSpec{fixed, {ID: "m", Kind: KindCombined, Inputs: []string{"f"}, Combine: "x"}}, "unknown combine"},
		"no inputs":           {[]CurveSpec{{ID: "m", Kind: KindCombined, Combine: CombineMax}}, "needs inputs"},
		"missing input":       {[]CurveSpec{{ID: "m", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"nope"}}}, "no curve"},
		"in itself":           {[]CurveSpec{{ID: "m", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"m"}}}, "itself"},
		"in a circle":         {[]CurveSpec{{ID: "a", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"b"}}, {ID: "b", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"a"}}}, "itself"},
		"units mixed":         {[]CurveSpec{fixed, {ID: "m", RPM: true, Kind: KindCombined, Combine: CombineMax, Inputs: []string{"f"}}}, "unit"},
		"follow itself":       {[]CurveSpec{{ID: "s", Kind: KindFollow, Follow: "a"}}, "itself"},
		"follow nothing":      {[]CurveSpec{{ID: "s", Kind: KindFollow, Follow: "nope"}}, "no output"},
		"follow no curve":     {[]CurveSpec{{ID: "s", Kind: KindFollow, Follow: "free"}}, "on no curve"},
		"fixed too high":      {[]CurveSpec{{ID: "x", Kind: KindFixed, Value: 101}}, "out of range"},
		"switch inverted":     {[]CurveSpec{{ID: "x", Kind: KindSwitch, IdleTemp: 60, LoadTemp: 40, RiseAfter: 1, FallAfter: 1, LoadValue: 50}}, "below the load"},
		"switch same":         {[]CurveSpec{{ID: "x", Kind: KindSwitch, IdleTemp: 50, LoadTemp: 50, RiseAfter: 1, FallAfter: 1, LoadValue: 50}}, "below the load"},
		"thermostat deadband": {[]CurveSpec{{ID: "x", Kind: KindThermostat, IdleTemp: 30, LoadTemp: 40, LoadValue: 50, Step: 1, Deadband: 10, ReactAfter: 1}}, "deadband"},
		"thermostat no step":  {[]CurveSpec{{ID: "x", Kind: KindThermostat, IdleTemp: 30, LoadTemp: 40, LoadValue: 50, ReactAfter: 1}}, "step"},
		"straight of three":   {[]CurveSpec{{ID: "x", Kind: KindStraight, Points: Curve{{20, 1}, {30, 2}, {40, 3}}, RiseAfter: 1, FallAfter: 1}}, "two points"},
	} {
		curveOf := map[string]string{"a": "s", "free": ""}
		err := CheckCurves(c.curves, curveOf)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestACombinedOfCurvesWithoutASensorIsRefusedOnceUsed(t *testing.T) {
	g := graphSpec()
	g.Sources = nil
	combined := CurveSpec{ID: "m", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"g"}}
	if _, err := NewShared([]CurveSpec{g, combined}, []Fan{fastFan("a", "m")}, NoOutput{}, nil); err == nil {
		t.Fatal("accepted")
	}
}

func gapSpec() CurveSpec {
	return CurveSpec{ID: "g", Kind: KindGap, Sources: []string{"water"}, Reference: "room",
		Points: Curve{{0, 20}, {10, 60}, {20, 100}}, RiseAfter: 1, FallAfter: 1}
}

func TestGapGoesByTheDistance(t *testing.T) {
	temps := map[string]float64{"water": 35, "room": 25}
	c, err := NewShared([]CurveSpec{gapSpec()}, []Fan{fastFan("fan", "g")}, NoOutput{}, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	if _ = c.Tick(); c.Status()[0].Duty != 60 {
		t.Fatalf("10 degrees apart: %+v", c.Status()[0])
	}
	if s := c.Status()[0]; !s.HasTemp || s.Temp != 35 {
		t.Fatalf("the output must see the real temperature: %+v", s)
	}
	// A warmer room with warmer water is the same distance.
	temps["water"], temps["room"] = 45, 35
	for range 5 {
		_ = c.Tick()
	}
	if s := c.Status()[0]; s.Duty != 60 {
		t.Fatalf("same distance, same value: %+v", s)
	}
	// Colder than the room stays on the first point.
	temps["water"], temps["room"] = 20, 25
	for range 40 {
		_ = c.Tick()
	}
	if s := c.Status()[0]; s.Duty != 20 {
		t.Fatalf("below the room: %+v", s)
	}
}

func TestGapWithoutItsReferenceIsLikeALostSensor(t *testing.T) {
	temps := map[string]float64{"water": 35, "room": 25}
	c, err := NewShared([]CurveSpec{gapSpec()}, []Fan{fastFan("fan", "g")}, NoOutput{}, func() map[string]float64 { return temps })
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Tick()
	delete(temps, "room")
	var last error
	for range sensorGraceTicks {
		last = c.Tick()
	}
	if s := c.Status()[0]; !errors.Is(last, ErrSensorLost) || s.Duty != 70 {
		t.Fatalf("err=%v %+v", last, s)
	}
}

func TestGapRefusals(t *testing.T) {
	same := gapSpec()
	same.Reference = "water"
	if err := CheckCurves([]CurveSpec{same}, nil); err == nil || !strings.Contains(err.Error(), "reference") {
		t.Fatalf("reference among the sources: %v", err)
	}
	none := gapSpec()
	none.Reference = ""
	if _, err := NewShared([]CurveSpec{none}, []Fan{fastFan("fan", "g")}, NoOutput{}, nil); err == nil {
		t.Fatal("a fan on a gap curve without reference must be refused")
	}
}
