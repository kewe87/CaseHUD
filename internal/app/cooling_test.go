package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

type coolingReadings struct{ readings []metrics.Reading }

func (c *coolingReadings) Refresh() error { return nil }
func (c *coolingReadings) Get(string) (metrics.Reading, bool) {
	return metrics.Reading{}, false
}
func (c *coolingReadings) All() []metrics.Reading { return c.readings }

const coolingExample = `{"curves":[
 {"id":"air","name":"Air","sources":["/t"],"unit":"rpm","points":[[20,500],[40,1500]],"rise_after":1,"fall_after":1},
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,60]],"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"fan","name":"Top","curve":"air","speeds":[[20,500],[100,1500]],"step_up":100,"step_down":100,"safe_duty":80},
 {"id":"pump","name":"Pump","pump":true,"curve":"water","rpm_sensor":"/pump","step_up":100,"step_down":100,"min_duty":60,"safe_duty":100}
]}`

func coolingEngine(t *testing.T, file string, p metrics.Provider) *Engine {
	t.Helper()
	dir := t.TempDir()
	if file != "" {
		if err := os.WriteFile(filepath.Join(dir, coolingFile), []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &Engine{Store: &Store{Dir: dir}, prov: NewSafeProvider(p)}
}

func TestCoolingStaysAwayWithoutAFile(t *testing.T) {
	e := coolingEngine(t, "", &coolingReadings{})
	e.startCooling()
	if v := e.CoolingView(); v.Configured || v.Error != "" {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingReportsABrokenFile(t *testing.T) {
	for name, file := range map[string]string{
		"syntax":        "{",
		"unknown field": `{"outputs":[],"nope":1}`,
		"trailing data": coolingExample + " GARBAGE",
		"short point":   strings.Replace(coolingExample, "[20,500],[40,1500]", "[20],[40]", 1),
		"long point":    strings.Replace(coolingExample, "[20,500],[40,1500]", "[20,500,9],[40,1500]", 1),
		"pump no rpm":   strings.Replace(coolingExample, `"rpm_sensor":"/pump",`, "", 1),
		"too large":     `{"outputs":[]}` + strings.Repeat(" ", coolingMaxBytes),
	} {
		e := coolingEngine(t, file, &coolingReadings{})
		e.startCooling()
		if v := e.CoolingView(); !v.Configured || v.Error == "" || e.cooling.ctrl != nil {
			t.Errorf("%s: %+v", name, v)
		}
	}
}

func TestCoolingErrorsStayShortAndHideThePath(t *testing.T) {
	e := coolingEngine(t, `{"outputs":[{"id":"`+strings.Repeat("x", 5000)+`","curve":"none"}]}`, &coolingReadings{})
	e.startCooling()
	if v := e.CoolingView(); v.Error == "" || len(v.Error) > coolingMaxErrText+10 {
		t.Errorf("error of %d bytes", len(v.Error))
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, coolingFile), 0o700); err != nil {
		t.Fatal(err)
	}
	d := &Engine{Store: &Store{Dir: dir}, prov: NewSafeProvider(&coolingReadings{})}
	d.startCooling()
	if v := d.CoolingView(); v.Error == "" || strings.Contains(v.Error, dir) {
		t.Errorf("%q", v.Error)
	}
}

func TestCoolingViewShowsDecisions(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: "/pump", Value: 4455.6, Unit: "RPM"},
		{Key: "/load", Value: 99, Unit: "%"},
	}}
	e := coolingEngine(t, coolingExample, p)
	startCoolingOnce(t, e)
	v := e.CoolingView()
	if !v.Configured || len(v.Outputs) != 2 {
		t.Fatalf("%+v", v)
	}
	fan, pump := v.Outputs[0], v.Outputs[1]
	if fan.Name != "Top" || fan.Temp == nil || *fan.Temp != 30 || fan.Duty != 60 || fan.TargetRPM != 1000 || fan.RPM != nil {
		t.Errorf("fan: %+v", fan)
	}
	if !pump.Pump || pump.Duty != 60 || pump.RPM == nil || *pump.RPM != 4456 {
		t.Errorf("pump: %+v", pump)
	}
}

// Sensors matched loosely could pick the wrong device, and a value in another
// unit is no temperature.
func TestCoolingTakesOnlyExactTemperatures(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/x/t", Path: "t", Value: 30, Unit: "°C"},
		{Key: "/t", Value: 30, Unit: "%"},
	}}
	e := coolingEngine(t, coolingExample, p)
	startCoolingOnce(t, e)
	if fan := e.CoolingView().Outputs[0]; fan.Temp != nil {
		t.Errorf("%+v", fan)
	}
}

// startCoolingOnce builds the preview and ticks it once, without the loop.
func startCoolingOnce(t *testing.T, e *Engine) {
	t.Helper()
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := newCoolingPreview(cfg, e.prov)
	if err != nil {
		t.Fatal(err)
	}
	e.cooling = coolingState{ctrl: ctrl, cfg: cfg}
	if err := ctrl.Tick(); err != nil && !strings.Contains(err.Error(), "sensor lost") {
		t.Fatal(err)
	}
}
