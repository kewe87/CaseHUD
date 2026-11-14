package app

import (
	"errors"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

func TestAlarmsOfHardEnds(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ticked := now.Add(-time.Second)
	rpm := 3000
	fan := CoolingOutputView{ID: "port1-fan1", Name: "Oben", RPM: &rpm}
	lost := fan
	lost.SensorLost = true
	unknownPump := CoolingOutputView{ID: "pump", Name: "Pumpe", Pump: true, SensorLost: true}
	for name, tc := range map[string]struct {
		v    CoolingView
		tick time.Time
		kind string
		who  string
	}{
		"service down":                         {CoolingView{Live: true, SensorsDown: true, Outputs: []CoolingOutputView{lost, unknownPump}}, ticked, coolingAlarmService, ""},
		"service down before the loop has run": {CoolingView{Live: true, SensorsDown: true}, time.Time{}, "", ""},
		"service down in a preview":            {CoolingView{SensorsDown: true, Outputs: []CoolingOutputView{lost}}, ticked, "", ""},
		"a standing pump beats the service":    {CoolingView{Live: true, SensorsDown: true, Outputs: []CoolingOutputView{{ID: "pump", Name: "Pumpe", Pump: true, PumpDown: true}}}, ticked, coolingAlarmPump, "Pumpe"},
		"a rival beats the service":            {CoolingView{Live: true, SensorsDown: true, Blocked: coolingBlockedProgram, Program: "X"}, ticked, coolingBlockedProgram, ""},
		"stale readings name the sensor":       {CoolingView{Live: true, SensorsStale: true, Outputs: []CoolingOutputView{lost, unknownPump}}, ticked, coolingAlarmSensor, "Oben"},
		"a pump without its speed alone":       {CoolingView{Live: true, Outputs: []CoolingOutputView{{ID: "pump", Name: "Pumpe", Pump: true}}}, ticked, coolingAlarmPumpUnknown, "Pumpe"},
		"safety copy in use":                   {CoolingView{Live: true, Restored: true, RestoredFile: "cooling.json.broken-1", Outputs: []CoolingOutputView{fan}}, ticked, coolingAlarmConfigCopy, "cooling.json.broken-1"},
		"safety copy in a preview":             {CoolingView{Restored: true, RestoredFile: "cooling.json.broken-1"}, time.Time{}, coolingAlarmConfigCopy, "cooling.json.broken-1"},
		"safety copy is the least urgent":      {CoolingView{Live: true, Restored: true, RestoredFile: "x", Outputs: []CoolingOutputView{lost}}, ticked, coolingAlarmSensor, "Oben"},
		"an unreadable file beats the copy":    {CoolingView{Error: "bad", Restored: true}, time.Time{}, coolingAlarmConfig, ""},
	} {
		a := coolingAlarmOf(tc.v, tc.tick, now)
		switch {
		case a == nil && tc.kind != "":
			t.Errorf("%s: no alarm", name)
		case a != nil && tc.kind == "":
			t.Errorf("%s: unexpected %+v", name, *a)
		case a != nil && (a.Kind != tc.kind || a.Name != tc.who):
			t.Errorf("%s: %+v", name, *a)
		}
	}
}

func TestHardEndAlarmsHaveASentenceInBothLanguages(t *testing.T) {
	for _, kind := range []string{coolingAlarmService, coolingAlarmConfigCopy} {
		if coolingAlarmGerman[kind] == "" || coolingAlarmEnglish[kind] == "" {
			t.Errorf("%s has no text", kind)
		}
	}
	a := CoolingAlarm{Kind: coolingAlarmConfigCopy, Name: "cooling.json.broken-1"}
	if got := a.Message(false); got == coolingAlarmEnglish[coolingAlarmConfigCopy] {
		t.Errorf("name not filled in: %s", got)
	}
}

// failingReadings is a sensor source that does not answer.
type failingReadings struct{ coolingReadings }

func (failingReadings) Refresh() error { return errors.New("sensors not reachable") }

// The service goes away while CaseHUD runs: the sensors drop, the fans go to their
// safe duty, and the alarm says that the service is the problem.
func TestAVanishingServiceIsReportedAsSuch(t *testing.T) {
	f := newFrozenRig(t)
	for range 30 {
		f.step(true)
	}
	if v := f.e.CoolingView(); v.Alarm != nil || v.SensorsDown {
		t.Fatalf("%+v", v.Alarm)
	}
	f.e.prov.Swap(&failingReadings{coolingReadings{readings: []metrics.Reading{}}})
	for range 5 {
		f.e.prov.Refresh()
		f.e.coolingSnapshot().ctrl.Tick()
	}
	v := f.e.CoolingView()
	if !v.SensorsDown || v.Alarm == nil || v.Alarm.Kind != coolingAlarmService {
		t.Fatalf("alarm %+v, down %v", v.Alarm, v.SensorsDown)
	}
	if o := outputByID(v, "port1-fan1"); !o.SensorLost || o.Duty < 70 {
		t.Fatalf("%+v", o)
	}
	if got := f.tl.duty["port1-fan1"]; got[len(got)-1] < 70 {
		t.Fatalf("the fan stayed at %d %%", got[len(got)-1])
	}
	// The sentence that reaches the user names the service.
	if msg := v.Alarm.Message(true); msg == "" || msg == coolingAlarmGerman[coolingAlarmStale] {
		t.Fatal(msg)
	}
	f.e.stopCoolingModule()
}
