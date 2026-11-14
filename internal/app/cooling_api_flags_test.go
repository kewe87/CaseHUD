package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAPIPercentNullCases(t *testing.T) {
	run := CoolingMeasureView{Running: true, ID: "pump", Phase: "pump", Duty: 40}
	idle := CoolingMeasureView{}
	cases := []struct {
		name string
		o    CoolingOutputView
		m    CoolingMeasureView
		want int // -1: null
	}{
		{"curve or manual", CoolingOutputView{Duty: 55}, idle, 55},
		{"over 100 is cut", CoolingOutputView{Duty: 140}, idle, 100},
		{"unregulated", CoolingOutputView{Unregulated: true}, idle, -1},
		{"not released", CoolingOutputView{Duty: 55, NotReleased: true}, idle, -1},
		{"not matched", CoolingOutputView{Duty: 55, NotMatched: true}, idle, -1},
		{"no source", CoolingOutputView{Duty: 55, NoSource: true}, idle, -1},
		{"unproven (pending or checking)", CoolingOutputView{Duty: 55, Unproven: true}, idle, -1},
		{"calibrating shows the sent duty", CoolingOutputView{Duty: 55, Held: true, Calibrating: true}, run, 40},
		{"calibrating, standstill step", CoolingOutputView{Duty: 55, Calibrating: true}, CoolingMeasureView{Running: true, Phase: "stop"}, 0},
		{"calibrating, nothing sent yet", CoolingOutputView{Duty: 55, Calibrating: true}, CoolingMeasureView{Running: true}, -1},
	}
	for _, c := range cases {
		got := apiPercent(c.o, c.m)
		switch {
		case c.want < 0 && got != nil:
			t.Errorf("%s: %d, want null", c.name, *got)
		case c.want >= 0 && (got == nil || *got != c.want):
			t.Errorf("%s: %v, want %d", c.name, got, c.want)
		}
	}
}

func TestUnprovenForOnlyBoardPumps(t *testing.T) {
	var nilPumps *boardPumps
	if nilPumps.unprovenFor("pump") {
		t.Fatal("no board pumps")
	}
	cfg, err := parseCoolingConfig([]byte(strings.Replace(manualPump(`{"value":50}`, ``), `{"outputs"`, `{"mode":"live","outputs"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	e := coolingEngine(t, "", &coolingReadings{})
	b := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
	for _, s := range []pumpState{pumpPending, pumpChecking} {
		b.setState("pump", s)
		if !b.unprovenFor("pump") {
			t.Errorf("state %d is not proven", s)
		}
	}
	b.setState("pump", pumpNotMatched)
	if b.unprovenFor("pump") {
		t.Error("a failed check is notMatched, not unproven")
	}
	b.setState("pump", pumpMatched)
	if b.unprovenFor("pump") || b.unprovenFor("port1-fan1") {
		t.Error("proven pump or a fan")
	}
}

// Only a running measurement on this output counts as calibrating; the hold
// that waits for the pump check does not.
func TestCoolingAPICalibratingFollowsTheMeasurement(t *testing.T) {
	file := `{"curves":[{"id":"c","sources":["/t"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1}],"outputs":[
	 {"id":"port1-fan1","name":"A","curve":"c","step_up":100,"step_down":100,"safe_duty":80},
	 {"id":"port1-fan2","name":"B","curve":"c","step_up":100,"step_down":100,"safe_duty":80}]}`
	e := setupEngine(t, file)
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	c := e.coolingSnapshot()
	c.ctrl.Tick()
	for _, o := range e.CoolingAPIView().Outputs {
		if o.Calibrating || o.Percent == nil {
			t.Fatalf("idle: %+v", o)
		}
	}
	c.ctrl.Hold("port1-fan1") // held, but no measurement runs
	c.ctrl.Tick()
	if o := e.CoolingAPIView().Outputs[0]; o.Calibrating || o.Percent == nil {
		t.Fatalf("held without a measurement: %+v", o)
	}
	e.coolMu.Lock()
	e.coolMeasure = CoolingMeasureView{ID: "port1-fan1", Running: true, Phase: "down", Duty: 35}
	e.coolMu.Unlock()
	c.ctrl.Tick()
	out := e.CoolingAPIView().Outputs
	if !out[0].Calibrating || out[0].Percent == nil || *out[0].Percent != 35 {
		t.Fatalf("measured: %+v", out[0])
	}
	if out[1].Calibrating || out[1].Percent == nil {
		t.Fatalf("the other fan: %+v", out[1])
	}
}

func TestCoolingAPIUnprovenFlagIsInTheJSON(t *testing.T) {
	b, _ := json.Marshal(CoolingAPIOutput{Unproven: true})
	if !strings.Contains(string(b), `"unproven":true`) || !strings.Contains(string(b), `"percent":null`) {
		t.Fatalf("%s", b)
	}
}
