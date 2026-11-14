package app

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

func intPtr(n int) *int { return &n }

const manualSpeeds = `"speeds":[[20,500],[100,1500]]`

func manualFan(manual, extra string) string {
	return `{"outputs":[{"id":"port1-fan1","name":"Top","manual":` + manual + `,` + manualSpeeds + `,"step_up":100,"step_down":100,"safe_duty":80` + extra + `}]}`
}

func manualPump(manual, extra string) string {
	return `{"outputs":[{"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","manual":` + manual +
		`,` + manualSpeeds + `,"step_up":100,"step_down":100,"min_duty":50,"safe_duty":100` + extra + `}]}`
}

func TestManualValueIsCheckedLikeACurve(t *testing.T) {
	for name, tc := range map[string]struct {
		file string
		ok   bool
	}{
		"percent":              {manualFan(`{"value":50}`, ``), true},
		"percent with unit":    {manualFan(`{"value":50,"unit":"percent"}`, ``), true},
		"rpm":                  {manualFan(`{"value":900,"unit":"rpm"}`, ``), true},
		"zero":                 {manualFan(`{"value":0}`, ``), false},
		"above 100":            {manualFan(`{"value":101}`, ``), false},
		"negative":             {manualFan(`{"value":-3}`, ``), false},
		"unknown unit":         {manualFan(`{"value":50,"unit":"volt"}`, ``), false},
		"below the minimum":    {manualFan(`{"value":20}`, `,"min_duty":30`), false},
		"below the controller": {manualFan(`{"value":5}`, ``), false},
		"rpm too high":         {manualFan(`{"value":99999,"unit":"rpm"}`, ``), false},
		"pump 50":              {manualPump(`{"value":50}`, ``), true},
		"pump 20":              {manualPump(`{"value":20}`, ``), false},
		"pump rpm too slow":    {manualPump(`{"value":400,"unit":"rpm"}`, ``), false},
		"pump rpm":             {manualPump(`{"value":1000,"unit":"rpm"}`, ``), true},
	} {
		_, err := parseCoolingConfig([]byte(tc.file))
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A pump is refused below 30 % whatever its own minimum says.
	for _, v := range []string{"10", "29"} {
		file := strings.Replace(manualPump(`{"value":`+v+`}`, ``), `"min_duty":50`, `"min_duty":10`, 1)
		if _, err := parseCoolingConfig([]byte(file)); err == nil {
			t.Errorf("a pump at %s %% was accepted", v)
		}
	}
	file := strings.Replace(manualPump(`{"value":30}`, ``), `"min_duty":50`, `"min_duty":30`, 1)
	if _, err := parseCoolingConfig([]byte(file)); err != nil {
		t.Errorf("a pump at 30 %%: %v", err)
	}
}

func TestManualRPMNeedsASpeedTable(t *testing.T) {
	file := `{"outputs":[{"id":"port1-fan1","manual":{"value":900,"unit":"rpm"},"step_up":100,"step_down":100,"safe_duty":80}]}`
	_, err := parseCoolingConfig([]byte(file))
	if err == nil || !strings.Contains(err.Error(), "speed table") {
		t.Fatalf("%v", err)
	}
	// Percent needs none.
	if _, err := parseCoolingConfig([]byte(strings.Replace(file, `{"value":900,"unit":"rpm"}`, `{"value":50}`, 1))); err != nil {
		t.Fatal(err)
	}
}

func TestAnOutputHasACurveOrAManualValueNeverBoth(t *testing.T) {
	file := strings.Replace(coolingExample, `"curve":"air",`, `"curve":"air","manual":{"value":50},`, 1)
	if _, err := parseCoolingConfig([]byte(file)); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("%v", err)
	}
	reserved := strings.Replace(coolingExample, `"id":"air"`, `"id":"@manual:fan"`, 1)
	if _, err := parseCoolingConfig([]byte(reserved)); err == nil {
		t.Fatal("a curve took the reserved ID")
	}
}

func TestAnOldConfigLoadsAndStoresNoManualField(t *testing.T) {
	c, err := parseCoolingConfig([]byte(coolingExample))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range c.Outputs {
		if o.Manual != nil || o.mode() != coolingModeCurve {
			t.Fatalf("%+v", o)
		}
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "manual") {
		t.Fatalf("%s", b)
	}
}

func manualEngine(t *testing.T, file string, readings ...metrics.Reading) *Engine {
	t.Helper()
	e := coolingEngine(t, file, &coolingReadings{readings: readings})
	startCoolingOnce(t, e)
	return e
}

func TestManualOutputFollowsItsValueWithoutASensor(t *testing.T) {
	// No temperature anywhere: a curve would lose its sensor, a manual value does not.
	e := manualEngine(t, manualFan(`{"value":50}`, ``))
	for range 5 {
		if err := e.cooling.ctrl.Tick(); err != nil {
			t.Fatalf("manual output reported %v", err)
		}
	}
	o := e.CoolingView().Outputs[0]
	if o.Duty != 50 || o.SensorLost || o.Unregulated || o.Manual == nil || o.Manual.Value != 50 || o.Manual.Unit != "percent" {
		t.Fatalf("%+v", o)
	}
}

func TestManualRPMGoesThroughTheSpeedTable(t *testing.T) {
	e := manualEngine(t, manualFan(`{"value":1000,"unit":"rpm"}`, ``))
	o := e.CoolingView().Outputs[0]
	// 500 rpm at 20 %, 1500 at 100 %: 1000 rpm is half way.
	if o.Duty != 60 || o.TargetRPM != 1000 || o.Manual.Unit != "rpm" {
		t.Fatalf("%+v", o)
	}
}

func TestManualOutputKeepsTheRamp(t *testing.T) {
	file := strings.Replace(manualFan(`{"value":80}`, ``), `"step_up":100,"step_down":100`, `"step_up":10,"step_down":10`, 1)
	e := manualEngine(t, file)
	// The first tick goes straight to the value, a change of it is then taken in steps.
	if d := e.CoolingView().Outputs[0].Duty; d != 80 {
		t.Fatalf("first duty %d", d)
	}
	cfg := e.cooling.cfg
	cfg.Outputs[0].Manual = &coolingManual{Value: 40}
	ctrl, err := newCoolingPreview(cfg, e.prov)
	if err != nil {
		t.Fatal(err)
	}
	_ = ctrl.Tick()
	if d := ctrl.Status()[0].Duty; d != 40 {
		t.Fatalf("%d", d)
	}
	cfg.Outputs[0].Manual = &coolingManual{Value: 80}
	ctrl2, _ := newCoolingPreview(cfg, e.prov)
	_ = ctrl2.Tick()
	_ = ctrl2.Tick()
	if ctrl2.Status()[0].Duty != 80 {
		t.Fatalf("%d", ctrl2.Status()[0].Duty)
	}
}

func TestManualPumpIsRegulatedAndAStandingPumpIsAnAlarm(t *testing.T) {
	e := manualEngine(t, manualPump(`{"value":50}`, ``), metrics.Reading{Key: pumpRPMKey, Value: 3000, Unit: "RPM"})
	o := e.CoolingView().Outputs[0]
	if !o.Pump || o.Duty != 50 || o.Unregulated || o.PumpDown {
		t.Fatalf("%+v", o)
	}
	e = manualEngine(t, manualPump(`{"value":50}`, ``), metrics.Reading{Key: pumpRPMKey, Value: 0, Unit: "RPM"})
	for range 8 {
		_ = e.cooling.ctrl.Tick()
	}
	if o := e.CoolingView().Outputs[0]; !o.PumpDown {
		t.Fatalf("%+v", o)
	}
}

// Manual counts as having a curve wherever the code asks for one.
func TestManualPumpOnTheBoardStillNeedsTheMatch(t *testing.T) {
	cfg, err := parseCoolingConfig([]byte(strings.Replace(manualPump(`{"value":50}`, ``), `{"outputs"`, `{"mode":"live","outputs"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	e := coolingEngine(t, "", &coolingReadings{})
	b := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
	if b == nil || len(b.ids()) != 1 || b.state("pump") != pumpPending {
		t.Fatalf("%+v", b)
	}
	// Unproven: nothing is written.
	if err := b.SetDuty("pump", 50); err != nil || len(b.last) != 0 {
		t.Fatalf("%v %v", err, b.last)
	}
	if got := cfg.regulated(); len(got) != 1 || !got[0].driven() {
		t.Fatalf("%+v", got)
	}
}

func TestManualPumpIsCheckedWithoutATemperature(t *testing.T) {
	s := newSimBoard(60)
	s.noTemp = true
	useSimBoard(t, s)
	r := &matchRig{s: s}
	r.e = coolingEngine(t, manualPumpWithWater, s)
	r.cfg, _ = r.e.Store.loadCoolingConfig()
	r.start(t)
	if d := r.check(); d != 0 || r.pumps.state("pump") != pumpMatched {
		t.Fatalf("delay %v state %v", d, r.pumps.state("pump"))
	}
	// The same pump on a curve does wait for its sensor.
	s2 := newSimBoard(60)
	c := newMatchRig(t, s2)
	s2.set(func(s *simBoard) { s.noTemp = true })
	_ = c.ctrl.Tick()
	if d := c.check(); d != pumpMatchPoll || c.pumps.state("pump") == pumpMatched {
		t.Fatalf("delay %v", d)
	}
}

func TestManualOutputIsDrivenAndHasItsOwnCurve(t *testing.T) {
	cfg, err := parseCoolingConfig([]byte(manualFan(`{"value":50}`, ``)))
	if err != nil {
		t.Fatal(err)
	}
	if o := cfg.Outputs[0]; !o.driven() || o.curveRef() == "" || len(cfg.specs()) != 1 {
		t.Fatalf("%+v", o)
	}
}

func TestPresetsKeepAndTellApartTheManualValue(t *testing.T) {
	a, _ := parseCoolingConfig([]byte(manualFan(`{"value":50}`, ``)))
	b, _ := parseCoolingConfig([]byte(manualFan(`{"value":60}`, ``)))
	c, _ := parseCoolingConfig([]byte(manualFan(`{"value":50,"unit":"rpm"}`, ``)))
	if presetKey(a) == presetKey(b) || presetKey(a) == presetKey(c) {
		t.Fatal("presets with different manual values are equal")
	}
	if m := presetConfigOf(a).Outputs[0].Manual; m == nil || m.Value != 50 {
		t.Fatalf("%+v", m)
	}
}

// A pump nothing regulates is only watched: its duty is the board's, not 30.
func TestAnUnregulatedPumpShowsNoDuty(t *testing.T) {
	file := `{"outputs":[{"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","step_up":100,"step_down":100,"min_duty":30,"safe_duty":100}]}`
	e := manualEngine(t, file, metrics.Reading{Key: pumpRPMKey, Value: 3000, Unit: "RPM"})
	o := e.CoolingView().Outputs[0]
	if !o.Unregulated || o.Duty != 0 || o.TargetRPM != 0 || o.Manual != nil {
		t.Fatalf("%+v", o)
	}
	b, _ := json.Marshal(o)
	if !strings.Contains(string(b), `"unregulated":true`) {
		t.Fatalf("%s", b)
	}
}

func TestCoolingAPIShowsTheModeOfEachOutput(t *testing.T) {
	file := `{"curves":[{"id":"c","sources":["/t"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1}],"outputs":[
	 {"id":"port1-fan1","name":"A","curve":"c","step_up":100,"step_down":100,"safe_duty":80},
	 {"id":"port1-fan2","name":"B","manual":{"value":900,"unit":"rpm"},"speeds":[[20,500],[100,1500]],"step_up":100,"step_down":100,"safe_duty":80},
	 {"id":"pump","name":"P","pump":true,"rpm_sensor":"/pump","step_up":100,"step_down":100,"min_duty":30,"safe_duty":100}]}`
	e := setupEngine(t, file)
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	e.coolingSnapshot().ctrl.Tick()
	out := e.CoolingAPIView().Outputs
	if len(out) != 3 {
		t.Fatalf("%+v", out)
	}
	if out[0].Mode != "curve" || out[0].Percent == nil || out[0].Manual != nil {
		t.Errorf("%+v", out[0])
	}
	if out[1].Mode != "manual" || out[1].Percent == nil || *out[1].Percent != 52 || out[1].Manual == nil || out[1].Manual.Unit != "rpm" {
		t.Errorf("%+v", out[1])
	}
	if out[2].Mode != "none" || out[2].Percent != nil {
		t.Errorf("%+v", out[2])
	}
	b, _ := json.Marshal(out[2])
	if !strings.Contains(string(b), `"percent":null`) {
		t.Errorf("%s", b)
	}
	if v := e.CoolingView().Outputs[2]; !v.Unregulated {
		t.Errorf("%+v", v)
	}
}

func TestCoolingPresetRestoresTheManualValue(t *testing.T) {
	file := manualFan(`{"value":50}`, ``)
	e := setupEngine(t, file)
	p, err := e.SaveCoolingPreset("Hand")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetCoolingConfigJSON([]byte(manualFan(`{"value":70}`, ``))); err != nil {
		t.Fatal(err)
	}
	if e.CoolingPresets().Active != "" {
		t.Fatal("a different value is still the same preset")
	}
	if err := e.ActivateCoolingPreset(p.ID); err != nil {
		t.Fatal(err)
	}
	if m := e.coolingSnapshot().cfg.Outputs[0].Manual; m == nil || m.Value != 50 || e.CoolingPresets().Active != p.ID {
		t.Fatalf("%+v", m)
	}
}

// The pump that the service refused stays "not released" while the next attempt
// runs, and goes only when a write is accepted.
func TestAPumpThatWasRefusedStaysNotReleasedDuringTheNextCheck(t *testing.T) {
	s := newSimBoard(60)
	s.denied = true
	r := newMatchRig(t, s)
	if d := r.check(); d != pumpMatchDenied || !r.pumps.notReleasedFor("pump") {
		t.Fatalf("delay %v", d)
	}
	var seen []bool
	// Looked at in the middle of the next attempt, before it writes.
	pumpMatchWait = func(ctx context.Context, d time.Duration) error {
		if r.pumps.isState("pump", pumpChecking) {
			seen = append(seen, r.e.CoolingView().Outputs[0].NotReleased)
		}
		return s.wait(ctx, d)
	}
	if d := r.check(); d != pumpMatchDenied {
		t.Fatalf("delay %v", d)
	}
	if len(seen) == 0 {
		t.Fatal("never looked during the attempt")
	}
	for _, n := range seen {
		if !n {
			t.Fatalf("not released vanished while checking: %v", seen)
		}
	}
	if !r.pumps.notReleasedFor("pump") || r.pumps.state("pump") != pumpPending {
		t.Fatal("the refusal was forgotten")
	}
	// Released now: the first accepted write ends it and the check goes through.
	s.set(func(s *simBoard) { s.denied = false })
	r.check()
	if r.pumps.notReleasedFor("pump") || r.pumps.state("pump") != pumpMatched {
		t.Fatalf("released %v state %v", r.pumps.notReleasedFor("pump"), r.pumps.state("pump"))
	}
}

// An attempt that ends before it wrote anything does not forget the refusal.
func TestAnAttemptWithoutAWriteKeepsTheRefusal(t *testing.T) {
	s := newSimBoard(60)
	s.denied = true
	r := newMatchRig(t, s)
	r.check()
	r.pumps.block.Store(&coolingBlock{"program", "other"})
	r.check()
	if !r.pumps.notReleasedFor("pump") {
		t.Fatal("the refusal was forgotten without a write")
	}
}

const manualPumpWithWater = `{"mode":"live","curves":[
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","manual":{"value":50},"step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}
]}`

func manualRig(t *testing.T, s *simBoard, file string) *matchRig {
	t.Helper()
	useSimBoard(t, s)
	r := &matchRig{s: s}
	r.e = coolingEngine(t, file, s)
	r.cfg, _ = r.e.Store.loadCoolingConfig()
	r.start(t)
	return r
}

// The pump on a manual value has no curve; the other curves' sensors guard it.
func TestManualPumpUsesTheSourcesOfAllCurvesAsGuard(t *testing.T) {
	// The header sits at 100 %: the check has to lower the pump, which needs the guard.
	r := manualRig(t, newSimBoard(100), manualPumpWithWater)
	if got := r.cfg.measureSources(); len(got) != 1 || got[0] != "/t" {
		t.Fatalf("%v", got)
	}
	if d := r.check(); d != 0 || r.pumps.state("pump") != pumpMatched {
		t.Fatalf("delay %v state %v", d, r.pumps.state("pump"))
	}
	if r.pumps.noSourceFor(r.cfg, "pump") || r.e.CoolingView().Outputs[0].NoSource {
		t.Fatal("reported no source")
	}
	if r.s.lowest < 50 {
		t.Fatalf("sent %d", r.s.lowest)
	}
	// Too warm for the guard: the check does not lower the pump.
	hot := manualRig(t, newSimBoard(100), manualPumpWithWater)
	hot.s.set(func(s *simBoard) { s.hot = 80 })
	hot.check()
	if hot.pumps.state("pump") == pumpMatched {
		t.Fatal("checked although too warm")
	}
}

func TestManualPumpWithoutAnyTemperatureSourceIsRefusedVisibly(t *testing.T) {
	file := strings.Replace(manualPump(`{"value":50}`, ``), `{"outputs"`, `{"mode":"live","outputs"`, 1)
	r := manualRig(t, newSimBoard(60), file)
	if d := r.check(); d != pumpMatchLater || r.pumps.state("pump") == pumpMatched || len(r.sent()) != 0 {
		t.Fatalf("delay %v state %v sent %v", d, r.pumps.state("pump"), r.sent())
	}
	v := r.e.CoolingView().Outputs[0]
	if !v.NoSource || v.MeasureBlock != "noSource" {
		t.Fatalf("%+v", v)
	}
	// With a curve somewhere the reason is gone.
	w := manualRig(t, newSimBoard(60), manualPumpWithWater)
	if v := w.e.CoolingView().Outputs[0]; v.NoSource || v.MeasureBlock == "noSource" {
		t.Fatalf("%+v", v)
	}
}

// guardReadings lets a test change the temperature between ticks.
type guardReadings struct {
	mu   sync.Mutex
	temp map[string]float64
}

func (g *guardReadings) Refresh() error                     { return nil }
func (g *guardReadings) Get(string) (metrics.Reading, bool) { return metrics.Reading{}, false }
func (g *guardReadings) All() []metrics.Reading {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := []metrics.Reading{{Key: pumpRPMKey, Value: 3000, Unit: "RPM"}}
	for k, v := range g.temp {
		out = append(out, metrics.Reading{Key: k, Value: v, Unit: "°C"})
	}
	return out
}

func (g *guardReadings) set(key string, v float64, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ok {
		g.temp[key] = v
	} else {
		delete(g.temp, key)
	}
}

const manualGuarded = `{"curves":[
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","manual":{"value":50},"step_up":100,"step_down":100,"min_duty":50,"safe_duty":100},
 {"id":"port1-fan1","name":"Top","manual":{"value":40},"step_up":100,"step_down":100,"safe_duty":80}
]}`

// Manual outputs follow no curve; the sensors of all curves guard them like one.
func TestManualOutputsGoToFullDutyWhenTooHot(t *testing.T) {
	g := &guardReadings{temp: map[string]float64{"/t": 40}}
	e := coolingEngine(t, manualGuarded, g)
	startCoolingOnce(t, e)
	view := func() (pump, fan CoolingOutputView) {
		t.Helper()
		_ = e.cooling.ctrl.Tick()
		v := e.CoolingView().Outputs
		return v[0], v[1]
	}
	normal := func(when string) {
		t.Helper()
		p, f := view()
		if p.Duty != 50 || f.Duty != 40 || p.Emergency || f.Emergency || outputProblem(p) != "" || outputProblem(f) != "" {
			t.Fatalf("%s: pump %+v fan %+v", when, p, f)
		}
	}
	for range 5 {
		normal("cool")
	}
	g.set("/t", 100, true)
	for i := range 3 {
		p, f := view()
		if hot := i == 2; p.Emergency != hot || f.Emergency != hot || (p.Duty == 100) != hot || (f.Duty == 100) != hot {
			t.Fatalf("hot cycle %d: pump %+v fan %+v", i+1, p, f)
		}
	}
	if p, f := view(); outputProblem(p) != "emergency" || outputProblem(f) != "emergency" {
		t.Fatalf("pump %+v fan %+v", p, f)
	}
	// The sensor is gone while hot: the outputs stay at full duty.
	g.set("/t", 0, false)
	for range 6 {
		if p, f := view(); p.Duty != 100 || f.Duty != 100 || !p.Emergency || !f.Emergency {
			t.Fatalf("lost while hot: pump %+v fan %+v", p, f)
		}
	}
	// Cool again: back to the manual values.
	g.set("/t", 45, true)
	normal("cool again")
	// A manual output shows no temperature, the guard does not change that.
	if p, f := view(); p.Temp != nil || f.Temp != nil || p.Unguarded || f.Unguarded {
		t.Fatalf("pump %+v fan %+v", p, f)
	}
}

// The emergency of a curve or a manual output is the alarm of the cooling, in
// the GUI and in the API.
func TestEmergencyIsTheCoolingAlarm(t *testing.T) {
	for name, file := range map[string]string{
		"manual": manualGuarded,
		"curve": `{"curves":[{"id":"air","name":"Air","sources":["/t"],"points":[[20,30],[125,40]],"rise_after":1,"fall_after":1}],
 "outputs":[{"id":"port1-fan1","name":"Top","curve":"air","step_up":100,"step_down":100,"safe_duty":80}]}`,
	} {
		g := &guardReadings{temp: map[string]float64{"/t": 100}}
		e := coolingEngine(t, file, g)
		startCoolingOnce(t, e)
		for range 3 {
			_ = e.cooling.ctrl.Tick()
		}
		v := e.CoolingView()
		v.Live = true // the order of the alarms is that of a live control
		a := coolingAlarmOf(v, time.Now(), time.Now())
		if a == nil || a.Kind != coolingAlarmEmergency || a.Name == "" {
			t.Fatalf("%s: %+v", name, a)
		}
		v.Alarm = a
		if api := apiAlarm(v); api == nil || api.Code != "emergency" {
			t.Fatalf("%s: %+v", name, api)
		}
		if msg := a.Message(true); !strings.Contains(msg, a.Name) || !strings.Contains(msg, "Übertemperatur") {
			t.Fatalf("%s: %q", name, msg)
		}
	}
}

// A manual fan without any curve has nothing to watch: it is still regulated, and
// says so.
func TestManualFanWithoutAGuardIsRegulatedAndToldUnguarded(t *testing.T) {
	e := manualEngine(t, manualFan(`{"value":50}`, ``))
	o := e.CoolingView().Outputs[0]
	if o.Duty != 50 || !o.Unguarded || outputProblem(o) != "unguarded" {
		t.Fatalf("%+v", o)
	}
	v := e.CoolingView()
	v.Live = true
	if a := coolingAlarmOf(v, time.Now(), time.Now()); a != nil {
		t.Fatalf("a warning, not an alarm: %+v", a)
	}
	// With a curve that has a sensor, the warning is gone.
	g := &guardReadings{temp: map[string]float64{"/t": 40}}
	w := coolingEngine(t, manualGuarded, g)
	startCoolingOnce(t, w)
	for _, o := range w.CoolingView().Outputs {
		if o.Unguarded || outputProblem(o) != "" {
			t.Fatalf("%+v", o)
		}
	}
}
