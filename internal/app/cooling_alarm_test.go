package app

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCoolingAlarmOrder(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	rpm := 3000
	ticked := now.Add(-time.Second)
	pump := CoolingOutputView{ID: "pump", Name: "Pumpe", Pump: true, RPM: &rpm}
	fan := CoolingOutputView{ID: "port1-fan1", Name: "Oben", RPM: &rpm}
	with := func(f func(*CoolingOutputView)) CoolingOutputView {
		o := fan
		f(&o)
		return o
	}
	for name, tc := range map[string]struct {
		v    CoolingView
		tick time.Time
		kind string
		who  string
	}{
		"all well":      {CoolingView{Live: true, Outputs: []CoolingOutputView{pump, fan}}, ticked, "", ""},
		"preview":       {CoolingView{Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.Stalled = true })}}, ticked, "", ""},
		"broken file":   {CoolingView{Error: "bad"}, time.Time{}, coolingAlarmConfig, ""},
		"not ticked":    {CoolingView{Live: true, Outputs: []CoolingOutputView{{ID: "pump", Pump: true}}}, time.Time{}, "", ""},
		"unknown pump":  {CoolingView{Live: true, Outputs: []CoolingOutputView{{ID: "pump", Name: "Pumpe", Pump: true}}}, ticked, coolingAlarmPumpUnknown, "Pumpe"},
		"loop stopped":  {CoolingView{Live: true, Outputs: []CoolingOutputView{pump, fan}}, now.Add(-time.Minute), coolingAlarmStale, ""},
		"rival":         {CoolingView{Live: true, Blocked: coolingBlockedProgram, Program: "Rival", Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.DeviceLost = true })}}, ticked, coolingBlockedProgram, ""},
		"device":        {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.DeviceLost = true })}}, ticked, coolingAlarmDevice, "Oben"},
		"write":         {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.WriteFail = true })}}, ticked, coolingAlarmWrite, "Oben"},
		"sensor":        {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.SensorLost = true })}}, ticked, coolingAlarmSensor, "Oben"},
		"stalled":       {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.Stalled = true })}}, ticked, coolingAlarmStalled, "Oben"},
		"emergency":     {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.Stalled = true }), {ID: "pump", Name: "Pumpe", Pump: true, Emergency: true}}}, ticked, coolingAlarmEmergency, "Pumpe"},
		"unguarded":     {CoolingView{Live: true, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.Unguarded = true })}}, ticked, "", ""},
		"pump is first": {CoolingView{Live: true, Blocked: coolingBlockedProgram, Outputs: []CoolingOutputView{with(func(o *CoolingOutputView) { o.Stalled = true }), {ID: "pump", Name: "Pumpe", Pump: true, PumpDown: true}}}, now.Add(-time.Minute), coolingAlarmPump, "Pumpe"},
	} {
		a := coolingAlarmOf(tc.v, tc.tick, now)
		switch {
		case a == nil && tc.kind != "":
			t.Errorf("%s: no alarm", name)
		case a != nil && (a.Kind != tc.kind || a.Name != tc.who):
			t.Errorf("%s: %+v", name, *a)
		}
	}
}

func TestEveryAlarmHasASentenceInBothLanguages(t *testing.T) {
	for _, kind := range []string{coolingAlarmPump, coolingAlarmPumpUnknown, coolingAlarmStale, coolingAlarmConfig, coolingBlockedProgram, coolingBlockedCheck, coolingBlockedDisclaimer, coolingAlarmDevice, coolingAlarmWrite, coolingAlarmSensor, coolingAlarmStalled, coolingAlarmEmergency} {
		if coolingAlarmGerman[kind] == "" || coolingAlarmEnglish[kind] == "" {
			t.Errorf("%s has no text", kind)
		}
	}
	a := CoolingAlarm{Kind: coolingBlockedProgram, Program: "Rival"}
	if got := a.Message(true); got == coolingAlarmGerman[coolingBlockedProgram] {
		t.Errorf("not filled in: %s", got)
	}
}

func TestAlarmIsToldWhenItLastsAndNotAgainAtOnce(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var w alarmWatch
	pump := &CoolingAlarm{Kind: coolingAlarmPump, Name: "Pumpe"}

	// A short blip interrupts nobody.
	if _, tell := w.see(pump, now); tell {
		t.Fatal("told at once")
	}
	if _, tell := w.see(nil, now.Add(5*time.Second)); tell {
		t.Fatal("told a blip's end")
	}

	now = now.Add(time.Minute)
	w.see(pump, now)
	if _, tell := w.see(pump, now.Add(coolingAlarmAfter-time.Second)); tell {
		t.Fatal("told early")
	}
	a, tell := w.see(pump, now.Add(coolingAlarmAfter))
	if !tell || a != *pump {
		t.Fatalf("not told: %+v %v", a, tell)
	}
	// Once is enough, however long it lasts.
	if _, tell := w.see(pump, now.Add(time.Minute)); tell {
		t.Fatal("told twice")
	}
	// Its end is told, so the tray can drop it.
	end := now.Add(2 * time.Minute)
	if a, tell := w.see(nil, end); !tell || a.Kind != "" {
		t.Fatalf("end: %+v %v", a, tell)
	}
	// Coming back soon, it stays quiet; later it is told again.
	w.see(pump, end.Add(time.Second))
	if _, tell := w.see(pump, end.Add(time.Second+coolingAlarmAfter)); tell {
		t.Fatal("flapping alarm told again")
	}
	if _, tell := w.see(pump, end.Add(coolingAlarmRepeat+time.Minute)); !tell {
		t.Fatal("not told after the pause")
	}
}

func TestAnotherAlarmIsToldToo(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var w alarmWatch
	a := &CoolingAlarm{Kind: coolingAlarmSensor, Name: "Oben"}
	b := &CoolingAlarm{Kind: coolingAlarmPump, Name: "Pumpe"}
	w.see(a, now)
	if _, tell := w.see(a, now.Add(coolingAlarmAfter)); !tell {
		t.Fatal("first not told")
	}
	w.see(b, now.Add(coolingAlarmAfter+time.Second))
	if got, tell := w.see(b, now.Add(2*coolingAlarmAfter+time.Second)); !tell || got != *b {
		t.Fatalf("pump not told while the sensor alarm was on: %+v %v", got, tell)
	}
}

// The gate closes: the fans must not stay at a low value, whether the other program
// drives them or not.
func TestHandOverLeavesFansSafe(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	e, _ := liveEngine(t, g)
	e.cooling.ctrl.Tick() // 50 %
	g.reason, g.program = coolingBlockedProgram, "Rival"
	e.cooling.ctrl.Tick()
	got := g.dev.duty["port1-fan1"]
	if got[len(got)-1] != 70 {
		t.Fatalf("left at %v", got)
	}
	if g.dev.closed != 1 {
		t.Fatal("kept the handle")
	}
	for range 3 {
		e.cooling.ctrl.Tick()
	}
	if len(g.dev.duty["port1-fan1"]) != len(got) {
		t.Fatalf("wrote while blocked: %v", g.dev.duty["port1-fan1"])
	}
	if e.CoolingView().Alarm == nil {
		t.Fatal("no alarm while another program is in charge")
	}
}

func TestHandOverKeepsAHigherDuty(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	_, fans := liveEngine(t, g)
	if _, err := fans.Speeds(); err != nil {
		t.Fatal(err)
	}
	if err := fans.SetDuty("port1-fan1", 95); err != nil {
		t.Fatal(err)
	}
	g.reason = coolingBlockedProgram
	if _, err := fans.Speeds(); !errors.Is(err, errCoolingBlocked) {
		t.Fatal(err)
	}
	got := g.dev.duty["port1-fan1"]
	if got[len(got)-1] != 95 {
		t.Fatalf("%v", got)
	}
}

func TestAFailedLookKeepsTheLastAnswer(t *testing.T) {
	broken := fakeProcs{err: errors.New("denied")}
	// Never known: blocked, and asked again.
	b, known := lookForRivals(broken, coolingBlock{}, false)
	if b.reason != coolingBlockedCheck || known {
		t.Fatalf("%+v %v", b, known)
	}
	b, known = lookForRivals(fakeProcs{}, b, known)
	if b.reason != "" || !known {
		t.Fatalf("%+v %v", b, known)
	}
	// Known to be clear: a failed look does not stop the control.
	b, known = lookForRivals(broken, b, known)
	if b.reason != "" || !known {
		t.Fatalf("%+v %v", b, known)
	}
	// Known to be taken: nor does it free the controller.
	b, _ = lookForRivals(fakeProcs{names: []string{"FanControl.exe"}}, b, known)
	if b.reason != coolingBlockedProgram {
		t.Fatalf("%+v", b)
	}
	if b, _ = lookForRivals(broken, b, true); b.reason != coolingBlockedProgram {
		t.Fatalf("%+v", b)
	}
}

// An English text that equals the German one is a German text left in the
// English table.
func TestEnglishAlarmTextsAreNotGerman(t *testing.T) {
	if len(coolingAlarmEnglish) != len(coolingAlarmGerman) {
		t.Errorf("%d English texts, %d German", len(coolingAlarmEnglish), len(coolingAlarmGerman))
	}
	for kind, de := range coolingAlarmGerman {
		en := coolingAlarmEnglish[kind]
		if en == "" || en == de {
			t.Errorf("%s: English text missing or equal to the German one: %q", kind, en)
		}
		if strings.ContainsAny(en, "äöüÄÖÜß") {
			t.Errorf("%s: umlaut in the English text: %q", kind, en)
		}
	}
}
