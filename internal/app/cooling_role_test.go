package app

import (
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

const roleCurve = `{"id":"c","name":"C","sources":["/t"],"points":[[20,0],[25,0],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}`

func TestACaseFanAlwaysRuns(t *testing.T) {
	for name, tc := range map[string]struct{ output, want string }{
		"no minimum":  {`{"id":"port1-fan1","role":"case","curve":"c","step_up":1,"step_down":1,"safe_duty":70}`, "always runs"},
		"with a stop": {`{"id":"port1-fan1","role":"case","curve":"c","step_up":1,"step_down":1,"min_duty":20,"stop_at":6,"safe_duty":70}`, "always runs"},
		"as a pump":   {`{"id":"pump","role":"case","pump":true,"rpm_sensor":"/r","step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}`, "not a case fan"},
		"other role":  {`{"id":"port1-fan1","role":"radio","curve":"c","step_up":1,"step_down":1,"safe_duty":70}`, "unknown role"},
	} {
		_, err := parseCoolingConfig([]byte(`{"curves":[` + roleCurve + `],"outputs":[` + tc.output + `]}`))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	ok := `{"id":"port1-fan1","role":"case","curve":"c","step_up":1,"step_down":1,"min_duty":20,"safe_duty":70}`
	if _, err := parseCoolingConfig([]byte(`{"curves":[` + roleCurve + `],"outputs":[` + ok + `]}`)); err != nil {
		t.Fatal(err)
	}
}

// Cold water at the start: the fan stands, nobody calls that a fault, and it starts
// again when the water warms up. The case fan on the same curve never stops.
func TestAFanMayStandWhileItIsColdAndACaseFanNot(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 0, "port1-fan2": 900})}
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 18, Unit: "°C"}}}
	e := coolingEngine(t, `{"mode":"live","curves":[`+roleCurve+`],"outputs":[
	 {"id":"port1-fan1","name":"Radiator","curve":"c","step_up":100,"step_down":100,"stop_at":6,"start_duty":15,"safe_duty":70},
	 {"id":"port1-fan2","name":"Hinten","role":"case","curve":"c","step_up":100,"step_down":100,"min_duty":20,"safe_duty":70}
	]}`, p)
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	fans := g.fans()
	ctrl, err := newCoolingLive(cfg, e.prov, fans, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.cooling = coolingState{ctrl: ctrl, cfg: cfg, fans: fans}

	for range 10 {
		if err := ctrl.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	last := func(id string) int { l := g.dev.duty[id]; return l[len(l)-1] }
	if last("port1-fan1") != 0 {
		t.Fatalf("radiator at %d", last("port1-fan1"))
	}
	if last("port1-fan2") != 20 {
		t.Fatalf("case fan at %d", last("port1-fan2"))
	}
	v := e.CoolingView()
	if v.Outputs[0].Stalled || v.Alarm != nil {
		t.Fatalf("a wanted standstill raised %+v", v)
	}
	// The controller starts a stopped fan again unless it is told again, so a stop is
	// repeated. A running fan is not written to all the time.
	if l := g.dev.duty["port1-fan1"]; len(l) < 9 {
		t.Fatalf("the stop was not repeated: %v", l)
	}
	if l := g.dev.duty["port1-fan2"]; len(l) > 3 {
		t.Fatalf("a running fan is written to all the time: %v", l)
	}

	// Warm water: it starts with its kick.
	p.readings[0].Value = 35
	ctrl.Tick()
	if last("port1-fan1") < 15 {
		t.Fatalf("did not start: %v", g.dev.duty["port1-fan1"])
	}
}

func TestSensorNamesStayWithinLimits(t *testing.T) {
	ok := `{"sensor_names":{"/lpc/ec/temperature/4":"Wasser"},"outputs":[]}`
	c, err := parseCoolingConfig([]byte(ok))
	if err != nil || c.SensorNames["/lpc/ec/temperature/4"] != "Wasser" {
		t.Fatalf("%+v %v", c, err)
	}
	long := `{"sensor_names":{"/a":"` + strings.Repeat("x", coolingMaxNameLen+1) + `"},"outputs":[]}`
	if _, err := parseCoolingConfig([]byte(long)); err == nil {
		t.Fatal("a very long name was accepted")
	}
}
