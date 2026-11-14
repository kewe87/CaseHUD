package cooling

import (
	"strings"
	"testing"
)

func settled(pairs ...int) []MeasuredStep {
	var out []MeasuredStep
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, MeasuredStep{Duty: pairs[i], RPM: pairs[i+1], Settled: true})
	}
	return out
}

func TestSpeedMapRefusesWhatWouldCapTheFan(t *testing.T) {
	for name, steps := range map[string][]MeasuredStep{
		"only the slow end": settled(15, 500, 20, 600),
		"falls":             settled(20, 1500, 50, 900, 100, 2000),
		"flat":              settled(20, 1000, 50, 1020, 100, 1020),
	} {
		if m, err := SpeedMapFrom(steps); err == nil {
			t.Errorf("%s: accepted %v", name, m)
		}
	}
	m, err := SpeedMapFrom(settled(20, 600, 30, 540, 100, 2000))
	if err != nil {
		t.Fatal(err)
	}
	if m[1].RPM != 600 {
		t.Fatalf("a small dip is not lifted: %v", m)
	}
}

func TestMeasureStopsWhenTooHot(t *testing.T) {
	check := RiseGuard(func() (float64, bool) { return measureMaxTemp, true })
	if err := check(); err == nil || !strings.Contains(err.Error(), "too hot") {
		t.Fatalf("%v", err)
	}
}

func validFan() Fan {
	return Fan{ID: "a", Sources: []string{"t"}, Curve: Curve{{Temp: 30, Value: 30}, {Temp: 80, Value: 100}},
		RiseAfter: 1, FallAfter: 1, StepUp: 5, StepDown: 5, SafeDuty: 70}
}

func TestValidationRefusesOutputsThatBarelyRespond(t *testing.T) {
	if err := checkOwn(validFan()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Fan){
		"slow to rise":  func(f *Fan) { f.RiseAfter = longestRiseTicks + 1 },
		"low safe duty": func(f *Fan) { f.SafeDuty = LowestSafeDuty - 1 },
		"stop at top":   func(f *Fan) { f.StopAt = 100 },
	} {
		f := validFan()
		change(&f)
		if err := checkOwn(f); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	f := validFan()
	f.FallAfter = longestDelayTicks // falling slowly is only louder
	if err := checkOwn(f); err != nil {
		t.Fatal(err)
	}
}

func TestEmergencyTempIgnoresCurveFilterAndSteps(t *testing.T) {
	f := validFan()
	f.Curve = Curve{{Temp: 30, Value: 5}, {Temp: 125, Value: 5}}
	f.RiseAfter, f.StepUp = longestRiseTicks, 1
	r := ownRegulator(f)
	r.step(40, true, 1000, true)
	if res := r.step(emergencyTemp, true, 1000, true); res.duty == 100 || res.emergency {
		t.Fatalf("one reading set it off: %+v", res)
	}
	r.step(emergencyTemp, true, 1000, true)
	if res := r.step(emergencyTemp, true, 1000, true); res.duty != 100 || !res.emergency {
		t.Fatalf("%+v", res)
	}
}

func TestLowDemandBelowStopAtStaysStopped(t *testing.T) {
	f := validFan()
	f.Curve = Curve{{Temp: 30, Value: 5}, {Temp: 80, Value: 100}}
	f.StopAt, f.StartDuty, f.StepUp, f.StepDown = 6, 40, 100, 100
	r := ownRegulator(f)
	for i := range 20 {
		if res := r.step(30, true, 0, true); res.duty != 0 {
			t.Fatalf("tick %d: %+v", i, res)
		}
	}
	if res := r.step(70, true, 0, true); res.duty == 0 {
		t.Fatal("did not start when needed")
	}
}

func TestAnRPMCurveNeedsAMeasuredFan(t *testing.T) {
	f := validFan()
	f.Curve = Curve{{Temp: 30, Value: 500}, {Temp: 80, Value: 1500}}
	f.RPMCurve = true
	if err := checkOwn(f); err == nil {
		t.Fatal("accepted")
	}
	f.Speeds = SpeedMap{{Duty: 20, RPM: 500}, {Duty: 100, RPM: 1500}}
	if err := checkOwn(f); err != nil {
		t.Fatal(err)
	}
}

// Two fans on one rpm curve reach the same speed with different duties.
func TestOneCurveDrivesFansThroughTheirOwnSpeeds(t *testing.T) {
	f := validFan()
	f.Curve, f.RPMCurve, f.StepUp, f.StepDown = Curve{{Temp: 30, Value: 1000}, {Temp: 80, Value: 1000}}, true, 100, 100
	a, b := f, f
	a.Speeds = SpeedMap{{Duty: 20, RPM: 500}, {Duty: 100, RPM: 1500}}
	b.Speeds = SpeedMap{{Duty: 20, RPM: 800}, {Duty: 100, RPM: 2400}}
	ra, rb := ownRegulator(a), ownRegulator(b)
	da, db := ra.step(40, true, 1000, true).duty, rb.step(40, true, 1000, true).duty
	if da != 60 || db != 30 {
		t.Fatalf("%d %d", da, db)
	}
}

// With a percent curve a measurement only adds the expected speed.
func TestAPercentCurveStaysPercentWithSpeeds(t *testing.T) {
	f := validFan()
	f.Curve, f.StepUp = Curve{{Temp: 30, Value: 40}, {Temp: 80, Value: 40}}, 100
	f.Speeds = SpeedMap{{Duty: 20, RPM: 500}, {Duty: 100, RPM: 1500}}
	r := ownRegulator(f)
	if d := r.step(40, true, 1000, true).duty; d != 40 {
		t.Fatalf("%d", d)
	}
}

// testRegulator is a regulator with the curve its Fan describes.
type testRegulator struct {
	regulator
	curve evaluator
}

func ownRegulator(f Fan) *testRegulator {
	return &testRegulator{regulator{cfg: f}, newEvaluator(f.ownCurve())}
}

func (r *testRegulator) step(temp float64, tempOK bool, rpm int, rpmOK bool) result {
	v, ok := r.curve.advance(temp, tempOK)
	return r.regulator.step(signal{value: v, ok: ok, temp: temp, hasTemp: tempOK}, rpm, rpmOK)
}

// checkOwn is everything New checks about an output with its own curve.
func checkOwn(f Fan) error {
	_, err := New([]Fan{f}, NoOutput{}, nil)
	return err
}
