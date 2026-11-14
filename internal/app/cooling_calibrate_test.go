package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// calibrationWait is the pause of a test run: nothing sleeps, the readings are
// taken again, and the hooks run, for a test to change the world meanwhile.
func calibrationWait(t *testing.T, e *Engine, hooks ...func(n int)) {
	t.Helper()
	saved := coolingMeasureWait
	n := 0
	coolingMeasureWait = func(ctx context.Context, _ time.Duration) error {
		n++
		_ = e.prov.Refresh()
		for _, h := range hooks {
			h(n)
		}
		return ctx.Err()
	}
	t.Cleanup(func() { coolingMeasureWait = saved })
}

func waitMeasureEnd(t *testing.T, e *Engine) CoolingMeasureView {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for e.coolingMeasuring() {
		if time.Now().After(deadline) {
			t.Fatal("the measurement did not end")
		}
		time.Sleep(2 * time.Millisecond)
	}
	return e.CoolingMeasure()
}

// pumpCalibrationRig is an engine with one board pump on the simulated board.
func pumpCalibrationRig(t *testing.T, s *simBoard) *matchRig {
	t.Helper()
	r := newMatchRig(t, s)
	r.pumps.markMatched("pump")
	r.e.Demo = true // what follows the measurement must not reach a real device
	ctx, cancel := context.WithCancel(context.Background())
	r.e.ctx = ctx
	c := r.e.coolingSnapshot()
	c.ctx, c.cancel = ctx, cancel
	r.e.setCoolingState(c)
	t.Cleanup(func() {
		cancel()
		r.e.waitCooling()
	})
	_ = r.e.prov.Refresh()
	return r
}

func TestPumpMeasurementNeedsAProvenPump(t *testing.T) {
	s := newSimBoard(60)
	r := pumpCalibrationRig(t, s)
	for name, state := range map[string]pumpState{"pending": pumpPending, "checking": pumpChecking, "failed": pumpNotMatched} {
		r.pumps.setState("pump", state)
		if err := r.e.StartCoolingPumpMeasure("pump"); err == nil {
			t.Errorf("%s: measured", name)
		}
		if why := r.e.measureBlockOf(r.e.coolingSnapshot(), r.cfg.Outputs[0]); why == "" {
			t.Errorf("%s: no reason given", name)
		}
	}
	r.pumps.markMatched("pump")
	r.pumps.notReleased.Store("pump", struct{}{})
	if err := r.e.StartCoolingPumpMeasure("pump"); err == nil {
		t.Error("measured a pump that is not released")
	}
	r.pumps.notReleased.Delete("pump")
	r.gate = "program"
	_, _ = r.pumps.Speeds() // the control sees the gate with every tick
	if err := r.e.StartCoolingPumpMeasure("pump"); err == nil {
		t.Error("measured while another program is in charge")
	}
	if err := r.e.StartCoolingMeasure("pump"); err == nil {
		t.Error("the fan button measured a pump")
	}
	if len(s.holds) != 0 {
		t.Fatalf("something was sent: %v", s.holds)
	}
}

func TestPumpMeasurementFindsTheMinimumAndKeepsTheFloor(t *testing.T) {
	s := newSimBoard(60)
	s.collapse = 38
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e)
	if err := r.e.StartCoolingPumpMeasure("pump"); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, r.e)
	if m.Error != "" || m.Canceled || len(m.Outcomes) != 1 {
		t.Fatalf("%+v", m)
	}
	c := m.Outcomes[0].Calibration
	if c == nil || c.Kind != "pump" || c.Minimum != 40 || c.AtLimit {
		t.Fatalf("%+v", c)
	}
	// Never lower than the floor of the service, never 0, and back at full duty.
	if s.lowest < cooling.LowestPumpDuty || slices.Contains(s.holds, 0) {
		t.Fatalf("holds %v", s.holds)
	}
	if s.holds[len(s.holds)-1] != 100 {
		t.Fatalf("last duty %d", s.holds[len(s.holds)-1])
	}
	stored, _ := r.e.Store.loadCoolingConfig()
	if sg := suggestionOf(stored.Outputs[0]); sg == nil || sg.MinDuty != 45 || sg.StopAt != 0 {
		t.Fatalf("%+v", sg)
	}
	if r.e.CoolingSetup().Suggestions["pump"].MinDuty != 45 {
		t.Fatalf("%+v", r.e.CoolingSetup().Suggestions)
	}
	// Nothing was applied by itself.
	if stored.Outputs[0].MinDuty != 50 {
		t.Fatalf("min_duty is now %d", stored.Outputs[0].MinDuty)
	}
}

func TestPumpMeasurementReportsTheLimit(t *testing.T) {
	s := newSimBoard(60)
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e)
	if err := r.e.StartCoolingPumpMeasure("pump"); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, r.e)
	if c := m.Outcomes[0].Calibration; m.Error != "" || c == nil || !c.AtLimit || c.Minimum != cooling.LowestPumpDuty {
		t.Fatalf("%+v %+v", m, c)
	}
	stored, _ := r.e.Store.loadCoolingConfig()
	if sg := suggestionOf(stored.Outputs[0]); sg.MinDuty != 35 {
		t.Fatalf("%+v", sg)
	}
}

// A pump that does not turn after the way back to full duty is still an alarm.
func TestPumpAlarmStaysAfterAMeasurement(t *testing.T) {
	s := newSimBoard(60)
	s.collapse, s.dead = 38, true
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e)
	if err := r.e.StartCoolingPumpMeasure("pump"); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, r.e)
	if m.Error == "" || len(m.Outcomes) != 0 {
		t.Fatalf("%+v", m)
	}
	if s.holds[len(s.holds)-1] != 100 {
		t.Fatalf("left at %d", s.holds[len(s.holds)-1])
	}
	ctrl := r.e.coolingSnapshot().ctrl
	for range 8 {
		_ = r.e.prov.Refresh()
		_ = ctrl.Tick()
	}
	v := r.e.CoolingView()
	if len(v.Outputs) != 1 || !v.Outputs[0].PumpDown || v.Alarm == nil || v.Alarm.Kind != coolingAlarmPump {
		t.Fatalf("%+v alarm %+v", v.Outputs, v.Alarm)
	}
}

func TestPumpMeasurementDoesNotDisturbTheCheckAndTheCheckNotIt(t *testing.T) {
	s := newSimBoard(60)
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e)
	r.e.coolMeasure.Running = true
	if r.e.mayCheck(r.e.coolingSnapshot().ctrl, r.pumps, "pump") {
		t.Fatal("a check may start during a measurement")
	}
	r.e.coolMeasure.Running = false
	r.pumps.setState("pump", pumpChecking)
	if err := r.e.StartCoolingPumpMeasure("pump"); err == nil {
		t.Fatal("measured during a check")
	}
}

const allFansExample = `{"mode":"live","curves":[
 {"id":"air","name":"Air","sources":["/t"],"points":[[20,30],[40,70]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port1-fan1","name":"A","curve":"air","step_up":100,"step_down":100,"safe_duty":70},
 {"id":"port1-fan2","name":"B","curve":"air","step_up":100,"step_down":100,"safe_duty":60},
 {"id":"port1-fan3","name":"C","curve":"air","role":"case","min_duty":20,"step_up":100,"step_down":100,"safe_duty":50},
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"/lpc/chip/0/fan/6","curve":"air","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}
]}`

func allFansEngine(t *testing.T) (*Engine, *openGate) {
	t.Helper()
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 1000, "port1-fan2": 1000, "port1-fan3": 1000})}
	g.dev.rpmPerDuty = 20
	e, _ := liveEngineFor(t, g, allFansExample)
	e.Demo = true
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	c := e.coolingSnapshot()
	loop, stopLoop := context.WithCancel(ctx) // the loop's own context: replacing it must not end the engine's
	c.ctx, c.cancel = loop, stopLoop
	e.setCoolingState(c)
	t.Cleanup(func() {
		cancel()
		e.waitCooling()
	})
	if err := c.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	g.dev.order = nil
	return e, g
}

func TestMeasureAllGoesThroughFansOneByOne(t *testing.T) {
	e, g := allFansEngine(t)
	calibrationWait(t, e)
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if m.Error != "" || m.Canceled || len(m.Outcomes) != 3 || !m.All {
		t.Fatalf("%+v", m)
	}
	// Blocks of writes: the first fan completely, then the second, then the third.
	var blocks []string
	for _, id := range g.dev.order {
		if len(blocks) == 0 || blocks[len(blocks)-1] != id {
			blocks = append(blocks, id)
		}
	}
	if !slices.Equal(blocks, []string{"port1-fan1", "port1-fan2", "port1-fan3"}) {
		t.Fatalf("order %v", blocks)
	}
	if _, touched := g.dev.duty["pump"]; touched {
		t.Fatal("the pump was part of it")
	}
	for id, safe := range map[string]int{"port1-fan1": 70, "port1-fan2": 60, "port1-fan3": 50} {
		d := g.dev.duty[id]
		if d[len(d)-1] != safe {
			t.Errorf("%s left at %d", id, d[len(d)-1])
		}
	}
	stored, _ := e.Store.loadCoolingConfig()
	for _, o := range stored.Outputs[:3] {
		if o.Calibration == nil || o.Calibration.Stops != "yes" {
			t.Errorf("%s: %+v", o.ID, o.Calibration)
		}
	}
	// The case fan never gets the suggestion to stand still.
	if sg := suggestionOf(stored.Outputs[2]); sg == nil || sg.StopAt != 0 || sg.StartDuty != 0 || sg.MinDuty != 15 {
		t.Fatalf("case fan: %+v", sg)
	}
	if sg := suggestionOf(stored.Outputs[0]); sg == nil || sg.StopAt != 14 || sg.StartDuty != 15 || sg.MinDuty != 15 {
		t.Fatalf("fan: %+v", sg)
	}
}

func TestMeasureAllCancelKeepsTheFinishedFans(t *testing.T) {
	e, g := allFansEngine(t)
	cancelled := false
	calibrationWait(t, e, func(n int) {
		// In the second fan.
		if !cancelled && len(g.dev.duty["port1-fan2"]) > 20 {
			cancelled = true
			e.StopCoolingMeasureKeep()
		}
	})
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if !m.Canceled || m.Error != "" {
		t.Fatalf("%+v", m)
	}
	if slices.Contains(g.dev.order, "port1-fan3") {
		t.Fatalf("the third fan was touched: %v", g.dev.duty["port1-fan3"])
	}
	d := g.dev.duty["port1-fan2"]
	if d[len(d)-1] != 60 {
		t.Fatalf("the second fan was left at %d", d[len(d)-1])
	}
	// The finished first fan is kept, the cancelled one is not.
	stored, _ := e.Store.loadCoolingConfig()
	if stored.Outputs[0].Calibration == nil || stored.Outputs[1].Calibration != nil {
		t.Fatalf("%+v %+v", stored.Outputs[0].Calibration, stored.Outputs[1].Calibration)
	}
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatalf("no new run after the end: %v", err)
	}
	e.StopCoolingMeasure()
	waitMeasureEnd(t, e)
}

func TestMeasureAllCancelDiscardsTheWholeRun(t *testing.T) {
	e, g := allFansEngine(t)
	cancelled := false
	calibrationWait(t, e, func(n int) {
		// In the second fan.
		if !cancelled && len(g.dev.duty["port1-fan2"]) > 20 {
			cancelled = true
			e.StopCoolingMeasure()
		}
	})
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if !m.Canceled || m.Error != "" {
		t.Fatalf("%+v", m)
	}
	if slices.Contains(g.dev.order, "port1-fan3") {
		t.Fatalf("the third fan was touched: %v", g.dev.duty["port1-fan3"])
	}
	d := g.dev.duty["port1-fan2"]
	if d[len(d)-1] != 60 {
		t.Fatalf("the second fan was left at %d", d[len(d)-1])
	}
	// Nothing of the run is stored, not even the finished first fan.
	stored, _ := e.Store.loadCoolingConfig()
	for i, o := range stored.Outputs {
		if o.Calibration != nil || len(o.Speeds) > 0 {
			t.Fatalf("output %d: %+v %v", i, o.Calibration, o.Speeds)
		}
	}
	if len(m.Outcomes) != 0 {
		t.Fatalf("outcomes: %+v", m.Outcomes)
	}
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatalf("no new run after the end: %v", err)
	}
	e.StopCoolingMeasure()
	waitMeasureEnd(t, e)
}

// While a fan is measured the view says so, with the steps found so far.
func TestMeasureShowsHeldAndTheStepsSoFar(t *testing.T) {
	e, _ := allFansEngine(t)
	var held bool
	var steps int
	var mid CoolingMeasureView
	calibrationWait(t, e, func(n int) {
		if n == 12 {
			_ = e.coolingSnapshot().ctrl.Tick()
			v := e.CoolingView()
			for _, o := range v.Outputs {
				if o.ID == "port1-fan1" && o.Held {
					held = true
				}
				if o.ID != "port1-fan1" && o.Held {
					t.Errorf("%s is held, too", o.ID)
				}
			}
			mid = v.Measure
			steps = len(v.Measure.Results)
		}
	})
	if err := e.StartCoolingMeasure("port1-fan1"); err != nil {
		t.Fatal(err)
	}
	waitMeasureEnd(t, e)
	if !held || steps == 0 || mid.Phase == "" || mid.Steps == 0 {
		t.Fatalf("held=%v steps=%d view=%+v", held, steps, mid)
	}
}

func TestMeasureRefusesASecondRunAtTheSameTime(t *testing.T) {
	e, _ := allFansEngine(t)
	calibrationWait(t, e)
	if err := e.StartCoolingMeasure("port1-fan1"); err != nil {
		t.Fatal(err)
	}
	if err := e.StartCoolingMeasure("port1-fan2"); err == nil {
		// The first run may have been through already; only a running one blocks.
		if !e.coolingMeasuring() {
			t.Skip("the first run ended before the second started")
		}
	}
	e.StopCoolingMeasure()
	waitMeasureEnd(t, e)
}

// A temperature that climbs ends the run, whatever the phase, and the fan is
// left at its safe duty.
func TestMeasureEndsWhenTheTemperatureRises(t *testing.T) {
	e, g := allFansEngine(t)
	calibrationWait(t, e, func(n int) {
		if n == 40 {
			e.prov.p.(*coolingReadings).readings[0].Value = 45 // 15 °C more than at the start
		}
		_ = e.prov.Refresh()
		_ = e.coolingSnapshot().ctrl.Tick()
	})
	if err := e.StartCoolingMeasure("port1-fan1"); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if !strings.Contains(m.Error, "temperature rose") {
		t.Fatalf("%+v", m)
	}
	d := g.dev.duty["port1-fan1"]
	if d[len(d)-1] != 70 {
		t.Fatalf("left at %d", d[len(d)-1])
	}
	if stored, _ := e.Store.loadCoolingConfig(); stored.Outputs[0].Calibration != nil {
		t.Fatal("an aborted run was stored")
	}
}

func TestSuggestions(t *testing.T) {
	cal := func(stops string, stop, start int, starts bool) *coolingCalibration {
		return &coolingCalibration{Kind: "fan", Stops: stops, Stop: stop, Start: start, Starts: starts}
	}
	cases := []struct {
		name string
		o    coolingOutputConfig
		want *CoolingSuggestion
	}{
		{"fan", coolingOutputConfig{Calibration: cal("yes", 12, 20, true)}, &CoolingSuggestion{StopAt: 14, StartDuty: 23, MinDuty: 15}},
		{"case fan never stands", coolingOutputConfig{Role: "case", Calibration: cal("yes", 12, 20, true)}, &CoolingSuggestion{MinDuty: 15}},
		{"stop value at the ceiling", coolingOutputConfig{Calibration: cal("yes", cooling.SuggestStopCeiling-cooling.SuggestStopMargin, 30, true)}, &CoolingSuggestion{StopAt: cooling.SuggestStopCeiling, StartDuty: 33, MinDuty: 41}},
		{"stop value above the ceiling", coolingOutputConfig{Calibration: cal("yes", cooling.SuggestStopCeiling-cooling.SuggestStopMargin+1, 30, true)}, &CoolingSuggestion{MinDuty: 42}},
		{"capped at 100", coolingOutputConfig{Calibration: cal("yes", 99, 99, true)}, &CoolingSuggestion{MinDuty: 100}},
		{"no restart found", coolingOutputConfig{Calibration: cal("yes", 12, 0, false)}, &CoolingSuggestion{MinDuty: 15}},
		{"never stops", coolingOutputConfig{Calibration: cal("never", 5, 0, false)}, nil},
		{"does not stay stopped", coolingOutputConfig{Calibration: cal("unheld", 5, 0, false)}, nil},
		{"nothing measured", coolingOutputConfig{}, nil},
		{"pump", coolingOutputConfig{Pump: true, Calibration: &coolingCalibration{Kind: "pump", Minimum: 42}}, &CoolingSuggestion{MinDuty: 47}},
		{"pump at the limit", coolingOutputConfig{Pump: true, Calibration: &coolingCalibration{Kind: "pump", Minimum: 30, AtLimit: true}}, &CoolingSuggestion{MinDuty: 35}},
	}
	for _, c := range cases {
		got := suggestionOf(c.o)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestPumpMeasurementDeviceRefusesLowValues(t *testing.T) {
	var sent []int
	d := pumpFloorDevice{&recordingDevice{sent: &sent}}
	for _, p := range []int{0, 1, cooling.LowestPumpDuty - 1, 101} {
		if err := d.SetDuty("p", p); err == nil {
			t.Errorf("%d accepted", p)
		}
	}
	if err := d.SetDuty("p", cooling.LowestPumpDuty); err != nil || len(sent) != 1 {
		t.Fatalf("%v %v", err, sent)
	}
	if pumpMeasureLowest < cooling.LowestPumpDuty || cooling.LowestSafeDuty < cooling.LowestPumpDuty {
		t.Fatal("the measurement floor is below the pump floor")
	}
}

type recordingDevice struct{ sent *[]int }

func (recordingDevice) TakeControl(string) error { return nil }
func (r recordingDevice) SetDuty(_ string, p int) error {
	*r.sent = append(*r.sent, p)
	return nil
}
func (recordingDevice) Speeds() (map[string]int, error) { return nil, nil }

func TestCalibrationInTheFileIsChecked(t *testing.T) {
	base := `{"outputs":[{"id":"a","step_up":1,"step_down":1,"safe_duty":50,"calibration":%s}]}`
	good := strings.Replace(base, "%s", `{"kind":"fan","stops":"yes","stop":8,"starts":true,"start":14}`, 1)
	if _, err := parseCoolingConfig([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`{"kind":"x"}`, `{"kind":"fan","stops":"maybe"}`, `{"kind":"fan","stop":101}`, `{"kind":"fan","extra":1}`} {
		if _, err := parseCoolingConfig([]byte(strings.Replace(base, "%s", bad, 1))); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	// A file from before the calibration still loads.
	if _, err := parseCoolingConfig([]byte(`{"outputs":[{"id":"a","step_up":1,"step_down":1,"safe_duty":50}]}`)); err != nil {
		t.Fatal(err)
	}
}

// The simulated fans of the demo behave as their descriptions say.
func TestDemoFansBehaveLikeTheirModel(t *testing.T) {
	d := &demoFans{fans: map[string]*demoFan{}}
	d.declare(coolingOutputConfig{ID: "demo-pump", Pump: true})
	fast := func(ctx context.Context, dur time.Duration) error {
		d.mu.Lock()
		d.now += dur
		d.mu.Unlock()
		return ctx.Err()
	}
	run := func(id string) cooling.Calibration {
		_, cal, err := cooling.Measurement{Device: d, ID: id, Steps: cooling.MeasureSteps, Wait: fast}.Calibrate(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return cal
	}
	if c := run("port1-fan1"); c.Stops != cooling.StopYes || c.Stop != 12 || !c.Starts || c.Start != 16 {
		t.Errorf("fan1 %+v", c)
	}
	if c := run("port1-fan2"); c.Stops != cooling.StopNever || c.Starts {
		t.Errorf("fan2 %+v", c)
	}
	if c := run("port1-fan3"); c.Stops != cooling.StopYes || c.Stop != 14 || c.Start != 22 {
		t.Errorf("fan3 %+v", c)
	}
	res, err := cooling.PumpMeasurement{Device: d, ID: "demo-pump", Floor: pumpMeasureLowest, Wait: fast}.Run(context.Background())
	if err != nil || res.Minimum != 45 || res.Failed != 40 {
		t.Errorf("pump %+v %v", res, err)
	}
}

// The controller's lowest effective duty comes from the backend, not from a copy.
func TestFansTellTheLowestEffectiveDuty(t *testing.T) {
	var d cooling.LowestDutyDevice = &tlFans{}
	if d.LowestDuty() != lianlitl.LowestDuty || d.LowestDuty() != 12 {
		t.Fatalf("%d", d.LowestDuty())
	}
	if (&demoFans{}).LowestDuty() != lianlitl.LowestDuty {
		t.Fatal("the demo fans have another floor")
	}
}

// A pump is not slowed down while the machine is already hot.
func TestPumpMeasurementDoesNotStartWhenItIsTooHot(t *testing.T) {
	s := newSimBoard(60)
	s.hot = cooling.PumpMeasureMaxTemp
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e)
	err := r.e.StartCoolingPumpMeasure("pump")
	if err == nil || !strings.Contains(err.Error(), "too hot") {
		t.Fatalf("%v", err)
	}
	if len(s.holds) != 0 || r.e.coolingMeasuring() {
		t.Fatalf("started: %v", s.holds)
	}
	s.set(func(s *simBoard) { s.hot = cooling.PumpMeasureMaxTemp - 1 })
	if err := r.e.StartCoolingPumpMeasure("pump"); err != nil {
		t.Fatalf("below the limit: %v", err)
	}
	r.e.StopCoolingMeasure()
	waitMeasureEnd(t, r.e)
}

// The temperature that ends a pump measurement climbs, and the pump goes back to
// full duty.
func TestPumpMeasurementEndsWhenItGetsHot(t *testing.T) {
	s := newSimBoard(60)
	r := pumpCalibrationRig(t, s)
	calibrationWait(t, r.e, func(n int) {
		if n == 6 {
			s.set(func(s *simBoard) { s.hot = 30 + cooling.PumpMeasureMaxRise })
		}
	})
	if err := r.e.StartCoolingPumpMeasure("pump"); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, r.e)
	if !strings.Contains(m.Error, "temperature rose") || len(m.Outcomes) != 0 {
		t.Fatalf("%+v", m)
	}
	if s.holds[len(s.holds)-1] != 100 || s.lowest < cooling.LowestPumpDuty {
		t.Fatalf("holds %v", s.holds)
	}
}

const guardExample = `{"mode":"live","curves":[
 {"id":"water","name":"Water","sources":["/water"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1},
 {"id":"cpu","name":"CPU","sources":["/cpu","/gpu"],"points":[[20,30],[80,100]],"rise_after":1,"fall_after":1},
 {"id":"mix","name":"Mix","kind":"combined","combine":"max","inputs":["water","cpu"],"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port2-fan1","name":"Pump","pump":true,"rpm_sensor":"/p","curve":"water","min_duty":50,"safe_duty":100,"step_up":100,"step_down":100},
 {"id":"port1-fan1","name":"Fan","curve":"mix","safe_duty":70,"step_up":100,"step_down":100}
]}`

func TestMeasureSourcesAreAllSensorsOfAllCurves(t *testing.T) {
	cfg, err := parseCoolingConfig([]byte(guardExample))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.measureSources()
	slices.Sort(got)
	if !slices.Equal(got, []string{"/cpu", "/gpu", "/water"}) {
		t.Fatalf("%v", got)
	}
}

// guardRig is a controller with a pump and a fan, not tied to any hardware.
type guardHW struct{ lost bool }

func (guardHW) TakeControl(string) error  { return nil }
func (guardHW) SetDuty(string, int) error { return nil }
func (h guardHW) Speeds() (map[string]int, error) {
	rpm := map[string]int{"pump": 3000, "fan": 800}
	if h.lost {
		return rpm, &cooling.PartlyLost{IDs: []string{"fan"}}
	}
	return rpm, nil
}

func TestMeasureCheckWatchesTheHottestAndTheControl(t *testing.T) {
	temps := map[string]float64{"/water": 30, "/cpu": 40, "/gpu": 45}
	sources := []string{"/water", "/cpu", "/gpu"}
	hottest := func() (float64, bool) { return cooling.Hottest(sources, temps) }
	cfg := coolingConfig{
		Curves: []coolingCurveConfig{{ID: "c", Sources: sources, Points: [][]float64{{20, 30}, {80, 100}}, RiseAfter: 1, FallAfter: 1}},
		Outputs: []coolingOutputConfig{
			{ID: "pump", Pump: true, Curve: "c", MinDuty: 50, SafeDuty: 100, StepUp: 100, StepDown: 100},
			{ID: "fan", Curve: "c", SafeDuty: 70, StepUp: 100, StepDown: 100},
		},
	}
	hw := &guardHW{}
	var specs []cooling.CurveSpec
	for _, x := range cfg.Curves {
		specs = append(specs, x.spec())
	}
	var fans []cooling.Fan
	for _, o := range cfg.Outputs {
		fans = append(fans, o.fan())
	}
	ctrl, err := cooling.NewShared(specs, fans, hw, func() map[string]float64 { return maps.Clone(temps) })
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	coolingMeasureNow = func() time.Time { return now }
	t.Cleanup(func() { coolingMeasureNow = time.Now })

	check := coolingMeasureCheck(ctrl, "pump", true, hottest)
	if err := check(); !errors.Is(err, errMeasureLoopStopped) {
		t.Fatalf("before the first tick: %v", err)
	}
	ctrl.Tick()
	now = time.Now()
	if err := check(); err != nil {
		t.Fatalf("%v", err)
	}
	// Another part gets hot, not the one of the pump's curve alone.
	temps["/gpu"] = 45 + cooling.PumpMeasureMaxRise
	if err := check(); err == nil || !strings.Contains(err.Error(), "temperature rose") {
		t.Fatalf("hottest rises: %v", err)
	}
	// The limit of a pump is lower than the one of a fan.
	temps["/gpu"] = cooling.PumpMeasureMaxTemp
	if err := coolingMeasureCheck(ctrl, "pump", true, hottest)(); err == nil || !strings.Contains(err.Error(), "too hot") {
		t.Fatalf("limit: %v", err)
	}
	temps["/gpu"] = 45
	// A source that is gone ends the run.
	delete(temps, "/cpu")
	if err := coolingMeasureCheck(ctrl, "pump", true, hottest)(); !errors.Is(err, cooling.ErrSensorLost) {
		t.Fatalf("source gone: %v", err)
	}
	temps["/cpu"] = 40
	// The control stands: its status is not current.
	now = time.Now().Add(coolingMeasureStale + time.Second)
	if err := check(); !errors.Is(err, errMeasureLoopStopped) {
		t.Fatalf("stale: %v", err)
	}
	now = time.Now()
	ctrl.Tick()
	if err := coolingMeasureCheck(ctrl, "pump", true, hottest)(); err != nil {
		t.Fatalf("%v", err)
	}
	// A device that cannot be read.
	hw.lost = true
	ctrl.Tick()
	if err := coolingMeasureCheck(ctrl, "pump", true, hottest)(); !errors.Is(err, errMeasureDeviceLost) {
		t.Fatalf("device lost: %v", err)
	}
}

// pauseWatch wraps the pause of a test run: it counts the cool-down pauses and
// lets a hook act inside the n-th of them.
func pauseWatch(t *testing.T, e *Engine, inPause func(n int)) *int {
	t.Helper()
	calibrationWait(t, e)
	inner := coolingMeasureWait
	pauses := new(int)
	coolingMeasureWait = func(ctx context.Context, d time.Duration) error {
		if d == coolingMeasureCooldown {
			*pauses++
			if inPause != nil {
				inPause(*pauses)
			}
		}
		return inner(ctx, d)
	}
	return pauses
}

func TestMeasureAllCoolsDownBetweenTheFans(t *testing.T) {
	e, _ := allFansEngine(t)
	pauses := pauseWatch(t, e, nil)
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	if m := waitMeasureEnd(t, e); m.Error != "" || len(m.Outcomes) != 3 {
		t.Fatalf("%+v", m)
	}
	if *pauses != 2 {
		t.Fatalf("%d pauses for 3 fans", *pauses)
	}
	if coolingMeasureCooldown != 30*time.Second {
		t.Fatalf("%v", coolingMeasureCooldown)
	}
}

func TestMeasureAllStopsIfItIsHotAfterThePause(t *testing.T) {
	e, g := allFansEngine(t)
	pauseWatch(t, e, func(n int) {
		if n == 1 {
			e.prov.p.(*coolingReadings).readings[0].Value = cooling.MeasureMaxTemp
		}
	})
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if !strings.Contains(m.Error, "too hot") || len(m.Outcomes) != 1 {
		t.Fatalf("%+v", m)
	}
	if slices.Contains(g.dev.order, "port1-fan2") {
		t.Fatalf("the second fan was touched: %v", g.dev.order)
	}
}

func TestMeasureAllCancelWorksInThePause(t *testing.T) {
	e, g := allFansEngine(t)
	pauseWatch(t, e, func(n int) { e.StopCoolingMeasureKeep() })
	if err := e.StartCoolingMeasureAll(); err != nil {
		t.Fatal(err)
	}
	m := waitMeasureEnd(t, e)
	if !m.Canceled || len(m.Outcomes) != 1 {
		t.Fatalf("%+v", m)
	}
	if slices.Contains(g.dev.order, "port1-fan2") {
		t.Fatalf("the second fan was touched: %v", g.dev.order)
	}
}

// A stop value that the configuration would refuse, because the curve of the fan
// never asks for more, is not suggested.
func TestStopSuggestionMustFitTheCurve(t *testing.T) {
	const file = `{"curves":[
 {"id":"low","name":"Low","sources":["/t"],"points":[[20,10],[40,30]],"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port1-fan1","name":"A","curve":"low","step_up":100,"step_down":100,"safe_duty":70}
]}`
	e := coolingEngine(t, file, &coolingReadings{readings: nil})
	cfg, err := parseCoolingConfig([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	o := cfg.Outputs[0]
	o.Calibration = &coolingCalibration{Kind: "fan", Stops: "yes", Stop: 22, Starts: true, Start: 25}
	if s := e.suggestionFor(cfg, o); s == nil || s.StopAt != 24 || s.StartDuty != 28 {
		t.Fatalf("fits: %+v", s)
	}
	o.Calibration.Stop = 28 // 30 reaches the top of the curve
	if s := e.suggestionFor(cfg, o); s == nil || s.StopAt != 0 || s.StartDuty != 0 || s.MinDuty != 31 {
		t.Fatalf("does not fit: %+v", s)
	}
}

// "Remove measured values" in the window saves the configuration without the
// calibration: the speed table and the duties stay, and nothing puts it back.
func TestSavingWithoutTheCalibrationKeepsTheSpeedsAndDuties(t *testing.T) {
	const withCal = `{"mode":"preview","curves":[
 {"id":"air","name":"Air","sources":["/t"],"points":[[20,30],[40,70]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port1-fan1","name":"A","curve":"air","stop_at":12,"start_duty":18,"step_up":100,"step_down":100,"speeds":[[20,400],[50,900],[100,1800]],
  "calibration":{"kind":"fan","at":"2026-01-02T03:04:05Z","stops":"yes","stop":10,"starts":true,"start":15},"safe_duty":70}
]}`
	e := setupEngine(t, "")
	if err := e.SetCoolingConfigJSON([]byte(withCal)); err != nil {
		t.Fatal(err)
	}
	stored, _ := e.Store.loadCoolingConfig()
	if stored.Outputs[0].Calibration == nil {
		t.Fatal("calibration not stored")
	}
	without := strings.Replace(withCal, `"calibration":{"kind":"fan","at":"2026-01-02T03:04:05Z","stops":"yes","stop":10,"starts":true,"start":15},`, "", 1)
	if without == withCal {
		t.Fatal("test config not changed")
	}
	if err := e.SetCoolingConfigJSON([]byte(without)); err != nil {
		t.Fatal(err)
	}
	stored, _ = e.Store.loadCoolingConfig()
	o := stored.Outputs[0]
	if o.Calibration != nil || len(o.Speeds) != 3 || o.StopAt != 12 || o.StartDuty != 18 || o.SafeDuty != 70 {
		t.Fatalf("%+v", o)
	}
	if s := suggestionOf(o); s != nil {
		t.Fatalf("suggestion without a calibration: %+v", s)
	}
}
