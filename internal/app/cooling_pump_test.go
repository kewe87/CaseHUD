package app

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
)

type fakePumpLine struct {
	holds  []int
	keys   []string
	key    string
	err    error
	closed int
}

func (l *fakePumpLine) Hold(key string, percent int) error {
	if l.err != nil {
		return l.err
	}
	l.key = key
	l.keys = append(l.keys, key)
	l.holds = append(l.holds, percent)
	return nil
}
func (l *fakePumpLine) Close() error { l.closed++; return nil }

// useFakePumpLine lets the test see what would go to the service.
func useFakePumpLine(t *testing.T) *fakePumpLine {
	t.Helper()
	l := &fakePumpLine{}
	before := coolingOpenPumpLine
	coolingOpenPumpLine = func() pumpLine { return l }
	t.Cleanup(func() { coolingOpenPumpLine = before })
	return l
}

const pumpRPMKey = "/lpc/chip/0/fan/6"

const pumpExample = `{"mode":"live","curves":[
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","curve":"water","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}
]}`

func pumpEngine(t *testing.T, gate *string) (*Engine, *boardPumps) {
	t.Helper()
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: pumpRPMKey, Value: 3000, Unit: "RPM"},
	}}
	e := coolingEngine(t, pumpExample, p)
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	pumps := newBoardPumps(cfg, e.prov, func() (string, string) { return *gate, "" })
	pumps.markMatched(pumps.ids()...)
	if pumps == nil {
		t.Fatal("no pump to drive")
	}
	ctrl, err := newCoolingLive(cfg, e.prov, nil, pumps)
	if err != nil {
		t.Fatal(err)
	}
	e.cooling = coolingState{ctrl: ctrl, cfg: cfg, fans: &tlFans{}, pumps: pumps}
	return e, pumps
}

func TestPumpFollowsItsCurveAndRepeatsItself(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, _ := pumpEngine(t, &gate)
	for range 3 {
		if err := e.cooling.ctrl.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	if l.key != "/lpc/chip/0/control/6" {
		t.Fatalf("key %q", l.key)
	}
	// Every tick says so again, or the service would hand the pump back.
	if len(l.holds) < 3 || l.holds[len(l.holds)-1] != 70 {
		t.Fatalf("%v", l.holds)
	}
	if v := e.CoolingView().Outputs[0]; v.Duty != 70 || v.RPM == nil || *v.RPM != 3000 || v.WriteFail {
		t.Fatalf("%+v", v)
	}
}

func TestPumpGoesBackToTheBoardWhenAnotherProgramRuns(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, _ := pumpEngine(t, &gate)
	e.cooling.ctrl.Tick()
	sent := len(l.holds)

	gate = coolingBlockedProgram
	if err := e.cooling.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	if l.closed != 1 || len(l.holds) != sent {
		t.Fatalf("closed %d, holds %v", l.closed, l.holds)
	}
	if v := e.CoolingView().Outputs[0]; v.RPM == nil || v.WriteFail || v.PumpDown {
		t.Fatalf("%+v", v)
	}

	gate = ""
	e.cooling.ctrl.Tick()
	if len(l.holds) == sent {
		t.Fatal("did not take it again")
	}
}

func TestABrokenLineIsToldAndTheStandstillStillCounts(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, pumps := pumpEngine(t, &gate)
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	pumps.now = func() time.Time { return now }
	e.cooling.ctrl.Tick()

	l.err = errors.New("service gone")
	e.cooling.ctrl.Tick()
	v := e.CoolingView()
	if !v.Outputs[0].WriteFail || v.Alarm == nil || v.Alarm.Kind != coolingAlarmWrite {
		t.Fatalf("%+v", v)
	}

	// The line is not hammered while it is down.
	closed := l.closed
	e.cooling.ctrl.Tick()
	if l.closed != closed {
		t.Fatal("tried again at once")
	}

	// A pump that stands is the worse news, and it does not depend on the line.
	e.prov.p.(*coolingReadings).readings[1].Value = 0
	for range 6 {
		e.cooling.ctrl.Tick()
	}
	if v := e.CoolingView(); !v.Outputs[0].PumpDown || v.Alarm == nil || v.Alarm.Kind != coolingAlarmPump {
		t.Fatalf("%+v", v)
	}

	l.err = nil
	now = now.Add(coolingReopenAfter + time.Second)
	e.prov.p.(*coolingReadings).readings[1].Value = 3000
	e.cooling.ctrl.Tick()
	if e.CoolingView().Outputs[0].WriteFail {
		t.Fatal("still failing")
	}
}

// The rate limit of the service is no reason to hand the pump to the board.
func TestABusyServiceKeepsTheLine(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, _ := pumpEngine(t, &gate)
	e.cooling.ctrl.Tick()
	l.err = enram.ErrBusy
	e.cooling.ctrl.Tick()
	if l.closed != 0 {
		t.Fatal("line closed")
	}
	if e.cooling.pumps.failing("pump") {
		t.Fatal("told as a failure")
	}
}

// A service that keeps refusing is a problem to see, not to wait out in silence.
func TestAServiceThatStaysBusyIsAProblem(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, pumps := pumpEngine(t, &gate)
	now := time.Now()
	pumps.now = func() time.Time { return now }
	e.cooling.ctrl.Tick()
	l.err = enram.ErrBusy
	for range 3 {
		now = now.Add(pumpBusyAfter / 4)
		e.cooling.ctrl.Tick()
	}
	if pumps.failing("pump") {
		t.Fatal("told too early")
	}
	now = now.Add(pumpBusyAfter)
	e.cooling.ctrl.Tick()
	if !pumps.failing("pump") || !e.CoolingView().Outputs[0].WriteFail {
		t.Fatal("not told")
	}
	if l.closed != 0 {
		t.Fatal("the line was closed")
	}
	// It answers again: the problem goes away, and a later short busy spell starts over.
	l.err = nil
	e.cooling.ctrl.Tick()
	if pumps.failing("pump") {
		t.Fatal("still told")
	}
	l.err = enram.ErrBusy
	e.cooling.ctrl.Tick()
	if pumps.failing("pump") {
		t.Fatal("the count did not start over")
	}
}

// With only a pump on a curve no fan loop would tell who is in charge.
func TestAPumpAloneTellsWhoIsInCharge(t *testing.T) {
	useFakePumpLine(t)
	gate := ""
	e, _ := pumpEngine(t, &gate)
	e.cooling.ctrl.Tick()
	if v := e.CoolingView(); v.Blocked != "" {
		t.Fatalf("%+v", v)
	}
	gate = coolingBlockedProgram
	e.cooling.ctrl.Tick()
	if v := e.CoolingView(); v.Blocked != coolingBlockedProgram {
		t.Fatalf("%+v", v)
	}
	gate = ""
	e.cooling.ctrl.Tick()
	if v := e.CoolingView(); v.Blocked != "" {
		t.Fatalf("%+v", v)
	}
}

func TestClosingGivesThePumpBack(t *testing.T) {
	l := useFakePumpLine(t)
	gate := ""
	e, pumps := pumpEngine(t, &gate)
	e.cooling.ctrl.Tick()
	pumps.Close()
	if l.closed != 1 {
		t.Fatal("line left open")
	}
	var none *boardPumps
	none.Close()
	if none.failing("pump") {
		t.Fatal("nothing can fail without pumps")
	}
}

func TestPumpControlKey(t *testing.T) {
	for in, want := range map[string]string{
		"/lpc/nct6701d/0/fan/6":   "/lpc/nct6701d/0/control/6",
		"/lpc/nct6701d/0/temp/6":  "",
		"/gpu-nvidia/0/fan/0":     "",
		"/lpc/nct6701d/0/fan/6/x": "",
		"":                        "",
	} {
		got, ok := pumpControlKey(in)
		if got != want || ok != (want != "") {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
}

func TestPumpSettingsThatCannotWorkAreRefused(t *testing.T) {
	curve := `{"id":"w","name":"W","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}`
	for name, tc := range map[string]struct{ output, want string }{
		"not on a header":   {`{"id":"pump","pump":true,"curve":"w","rpm_sensor":"/usb/pump","step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}`, "cannot be set"},
		"too low":           {`{"id":"pump","pump":true,"curve":"w","rpm_sensor":"` + pumpRPMKey + `","step_up":1,"step_down":1,"min_duty":20,"safe_duty":100}`, "at least 30"},
		"on controller":     {`{"id":"port1-fan1","pump":true,"step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}`, "needs a curve"},
		"low on controller": {`{"id":"port1-fan1","pump":true,"curve":"w","step_up":1,"step_down":1,"min_duty":1,"safe_duty":100}`, "at least"},
	} {
		_, err := parseCoolingConfig([]byte(`{"mode":"live","curves":[` + curve + `],"outputs":[` + tc.output + `]}`))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, out := range map[string]string{
		"on a header":   `{"id":"pump","pump":true,"curve":"w","rpm_sensor":"` + pumpRPMKey + `","step_up":1,"step_down":1,"min_duty":30,"safe_duty":100}`,
		"only watched":  `{"id":"pump","pump":true,"rpm_sensor":"/usb/pump","step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}`,
		"on controller": `{"id":"port1-fan1","pump":true,"curve":"w","step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}`,
	} {
		if _, err := parseCoolingConfig([]byte(`{"mode":"live","curves":[` + curve + `],"outputs":[` + out + `]}`)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A pump on the fan controller goes the same way as a fan there.
func TestPumpOnTheControllerUsesTheController(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 2500})}
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}}}
	e := coolingEngine(t, `{"mode":"live","curves":[{"id":"w","name":"W","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}],
	 "outputs":[{"id":"port1-fan1","name":"P","pump":true,"curve":"w","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}]}`, p)
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := newCoolingLive(cfg, e.prov, g.fans(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	if got := g.dev.duty["port1-fan1"]; len(got) != 1 || got[0] != 70 {
		t.Fatalf("%v", got)
	}
}

// A header nobody released is no failure: the board regulates, the line stays, and
// the control takes over by itself once the header is released.
func TestAnUnreleasedPumpIsToldButNotAnAlarm(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	l := useFakePumpLine(t)
	gate := ""
	e, _ := pumpEngine(t, &gate)

	l.err = enram.ErrDenied
	for range 4 {
		if err := e.cooling.ctrl.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	v := e.CoolingView()
	if !v.Outputs[0].NotReleased || v.Outputs[0].WriteFail || v.Alarm != nil || v.Outputs[0].RPM == nil {
		t.Fatalf("%+v alarm %+v", v.Outputs[0], v.Alarm)
	}
	if l.closed != 0 {
		t.Fatal("the line was given up")
	}
	if n := strings.Count(logged.String(), "not released"); n != 1 {
		t.Fatalf("logged %d times:\n%s", n, logged.String())
	}

	l.err = nil
	e.cooling.ctrl.Tick()
	v = e.CoolingView()
	if v.Outputs[0].NotReleased || v.Outputs[0].WriteFail || len(l.holds) == 0 {
		t.Fatalf("%+v holds %v", v.Outputs[0], l.holds)
	}
	var none *boardPumps
	if none.notReleasedFor("pump") {
		t.Fatal("nothing to release without pumps")
	}
}

func TestOnlyAPumpOnAMainboardHeaderCanBeReleased(t *testing.T) {
	cfgJSON := `{"mode":"preview","curves":[
 {"id":"c","name":"C","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"P","pump":true,"rpm_sensor":"` + pumpRPMKey + `","curve":"c","min_duty":50,"safe_duty":100},
 {"id":"fan1","name":"F","curve":"c","min_duty":20,"safe_duty":70},
 {"id":"nocurve","name":"N","pump":true,"rpm_sensor":"/lpc/chip/0/fan/7","safe_duty":100},
 {"id":"ctrlpump","name":"T","pump":true,"rpm_sensor":"/pump","curve":"c","min_duty":50,"safe_duty":100},
 {"id":"port1-fan1","name":"TL","curve":"c","min_duty":20,"safe_duty":70}
]}`
	var asked []string
	before := coolingAllowHeader
	coolingAllowHeader = func(key string) error { asked = append(asked, key); return nil }
	t.Cleanup(func() { coolingAllowHeader = before })

	e := coolingEngine(t, cfgJSON, &coolingReadings{})
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	pumps := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
	e.cooling = coolingState{cfg: cfg, configured: true, pumps: pumps}
	// None of these is a pump the service refused, but a refused one still must be a pump on a header.
	pumps.notReleased.Store("nocurve", struct{}{})
	pumps.notReleased.Store("fan1", struct{}{})

	for _, id := range []string{"", "nope", "fan1", "nocurve", "ctrlpump", "port1-fan1", "/lpc/chip/0/control/6"} {
		if err := e.ReleaseCoolingPump(id); err == nil {
			t.Errorf("%q accepted", id)
		}
	}
	if len(asked) != 0 {
		t.Fatalf("asked for %v", asked)
	}
	// A pump the service has not refused needs no release.
	if err := e.ReleaseCoolingPump("pump"); err == nil || len(asked) != 0 {
		t.Fatalf("asked for a pump that is not refused: %v %v", err, asked)
	}
	pumps.notReleased.Store("pump", struct{}{})
	if err := e.ReleaseCoolingPump("pump"); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "/lpc/chip/0/control/6" {
		t.Fatalf("asked for %v", asked)
	}

	e.Demo = true
	if err := e.ReleaseCoolingPump("pump"); err == nil || len(asked) != 1 {
		t.Fatal("released in the demo")
	}
}

func TestAskingForTheReleaseIsOneAtATimeAndNotInARow(t *testing.T) {
	now := time.Now()
	g := &releaseGate{now: func() time.Time { return now }}
	asked := 0
	ask := func() error { asked++; return nil }
	if err := g.run(ask); err != nil || asked != 1 {
		t.Fatalf("%v %d", err, asked)
	}
	for _, d := range []time.Duration{0, time.Second, 10 * time.Second} {
		now = now.Add(d)
		if err := g.run(ask); !errors.Is(err, ErrReleaseWait) || asked != 1 {
			t.Fatalf("after %v: %v %d", d, err, asked)
		}
	}
	now = now.Add(releaseEvery)
	if err := g.run(ask); err != nil || asked != 2 {
		t.Fatalf("%v %d", err, asked)
	}

	// While a prompt is open, a second request is turned away at once.
	inPrompt, leave := make(chan struct{}), make(chan struct{})
	done := make(chan error)
	now = now.Add(time.Hour)
	go func() { done <- g.run(func() error { close(inPrompt); <-leave; return nil }) }()
	<-inPrompt
	if err := g.run(ask); !errors.Is(err, ErrReleaseWait) || asked != 2 {
		t.Fatalf("%v %d", err, asked)
	}
	close(leave)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// A prompt that was declined counts too.
	now = now.Add(time.Hour)
	g.run(func() error { return errors.New("setup was cancelled") })
	if err := g.run(ask); !errors.Is(err, ErrReleaseWait) {
		t.Fatalf("%v", err)
	}
}

func TestAPumpStaysWithCaseHUDWhileAnEqualControlTakesOver(t *testing.T) {
	line := useFakePumpLine(t)
	cfg, err := parseCoolingConfig([]byte(pumpExample))
	if err != nil {
		t.Fatal(err)
	}
	gate := func() (string, string) { return "", "" }
	old := newBoardPumps(cfg, nil, gate)
	old.markMatched(old.ids()...)
	if err := old.SetDuty("pump", 70); err != nil {
		t.Fatal(err)
	}
	old.keepLine()
	old.Close() // the old loop ends
	if line.closed != 0 {
		t.Fatalf("the line was closed (%d), the board had the pump", line.closed)
	}
	next := newBoardPumps(cfg, nil, gate)
	next.markMatched(next.ids()...)
	next.takeOver(old)
	next.rpm = func() map[string]int { return nil }
	if _, err := next.Speeds(); err != nil {
		t.Fatal(err)
	}
	if line.closed != 0 || len(line.holds) != 2 || line.holds[1] != 70 {
		t.Fatalf("holds %v, closed %d", line.holds, line.closed)
	}
	old.endKept() // nothing left of the line: no second close
	if line.closed != 0 {
		t.Fatal("the line of the new control was closed")
	}
	next.Close()
	if line.closed != 1 {
		t.Fatalf("closed %d", line.closed)
	}
}

func TestAPumpThatIsNotProvenIsNotCarriedOver(t *testing.T) {
	line := useFakePumpLine(t)
	cfg, _ := parseCoolingConfig([]byte(pumpExample))
	gate := func() (string, string) { return "", "" }
	old := newBoardPumps(cfg, nil, gate)
	old.markMatched(old.ids()...)
	old.SetDuty("pump", 70)
	old.keepLine()
	old.Close()
	next := newBoardPumps(cfg, nil, gate) // its pair is not proven
	next.takeOver(old)
	next.rpm = func() map[string]int { return nil }
	next.Speeds()
	if len(line.holds) != 1 {
		t.Fatalf("holds %v", line.holds)
	}
	if line.closed != 1 {
		t.Fatalf("the line stayed open (%d)", line.closed)
	}
}

func TestEndKeptGivesTheHeaderBack(t *testing.T) {
	line := useFakePumpLine(t)
	cfg, _ := parseCoolingConfig([]byte(pumpExample))
	old := newBoardPumps(cfg, nil, func() (string, string) { return "", "" })
	old.markMatched(old.ids()...)
	old.SetDuty("pump", 70)
	old.keepLine()
	old.Close()
	old.endKept()
	if line.closed != 1 {
		t.Fatalf("closed %d", line.closed)
	}
}
