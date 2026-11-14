package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/conflicts"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// fakeTL records what reaches the controller.
type fakeTL struct {
	speeds  map[string]int
	readErr error
	duty    map[string][]int
	taken   map[string]int
	closed  int
	// rpmPerDuty lets the speeds follow the last duty; without it they are fixed.
	rpmPerDuty int
	// order lists the outputs in the order they were written to.
	order []string
}

func newFakeTL(speeds map[string]int) *fakeTL {
	return &fakeTL{speeds: speeds, duty: map[string][]int{}, taken: map[string]int{}}
}

func (f *fakeTL) FanIDs() []string            { return nil }
func (f *fakeTL) TakeControl(id string) error { f.taken[id]++; return nil }
func (f *fakeTL) SetDuty(id string, p int) error {
	f.duty[id] = append(f.duty[id], p)
	f.order = append(f.order, id)
	return nil
}
func (f *fakeTL) Close() error { f.closed++; return nil }
func (f *fakeTL) Speeds() (map[string]int, error) {
	if f.rpmPerDuty == 0 {
		return f.speeds, f.readErr
	}
	out := map[string]int{}
	for id, rpm := range f.speeds {
		out[id] = rpm
		if d := f.duty[id]; len(d) > 0 {
			out[id] = d[len(d)-1] * f.rpmPerDuty
		}
	}
	return out, f.readErr
}

type openGate struct {
	opens   int
	openErr error
	dev     *fakeTL
	reason  string
	program string
	at      time.Time
}

func (o *openGate) fans() *tlFans {
	return &tlFans{
		ctx: context.Background(),
		open: func(context.Context) (lianlitl.FanDevice, error) {
			o.opens++
			if o.openErr != nil {
				return nil, o.openErr
			}
			return o.dev, nil
		},
		gate: func() (string, string) { return o.reason, o.program },
		now:  func() time.Time { return o.at },
	}
}

const coolingLiveExample = `{"mode":"live","curves":[
 {"id":"air","name":"Air","sources":["/t"],"points":[[20,30],[40,70]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1},
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port1-fan1","name":"Top","curve":"air","step_up":100,"step_down":100,"safe_duty":70},
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"/lpc/chip/0/fan/6","curve":"water","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}
]}`

func liveEngine(t *testing.T, g *openGate) (*Engine, *tlFans) {
	t.Helper()
	return liveEngineFor(t, g, coolingLiveExample)
}

func liveEngineFor(t *testing.T, g *openGate, file string) (*Engine, *tlFans) {
	t.Helper()
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: "/lpc/chip/0/fan/6", Value: 3000, Unit: "RPM"},
	}}
	e := coolingEngine(t, file, p)
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	fans := g.fans()
	useFakePumpLine(t)
	pumps := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
	pumps.markMatched(pumps.ids()...)
	ctrl, err := newCoolingLive(cfg, e.prov, fans, pumps)
	if err != nil {
		t.Fatal(err)
	}
	e.cooling = coolingState{ctrl: ctrl, cfg: cfg, fans: fans, pumps: pumps}
	return e, fans
}

func TestLiveKeepsThePumpOffTheFanController(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	e, _ := liveEngine(t, g)
	if err := e.cooling.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	if got := g.dev.duty["port1-fan1"]; len(got) != 1 || got[0] != 50 {
		t.Fatalf("fan duty %v", got)
	}
	if _, ok := g.dev.duty["pump"]; ok {
		t.Fatal("pump reached the fan controller")
	}
	v := e.CoolingView()
	if !v.Live || v.Blocked != "" {
		t.Fatalf("%+v", v)
	}
	if pump := v.Outputs[1]; pump.RPM == nil || *pump.RPM != 3000 || pump.DeviceLost {
		t.Fatalf("pump: %+v", pump)
	}
}

func TestLiveWritesNothingWhileARivalRuns(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900}), reason: coolingBlockedProgram, program: "Rival"}
	e, _ := liveEngine(t, g)
	for range 3 {
		e.cooling.ctrl.Tick()
	}
	if g.opens != 0 || len(g.dev.duty) != 0 || len(g.dev.taken) != 0 {
		t.Fatalf("touched the controller: %d %v %v", g.opens, g.dev.duty, g.dev.taken)
	}
	v := e.CoolingView()
	if v.Blocked != coolingBlockedProgram || v.Program != "Rival" {
		t.Fatalf("%+v", v)
	}

	// The program ends: control is taken and the duty sent.
	g.reason, g.program = "", ""
	if err := e.cooling.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	if g.dev.taken["port1-fan1"] != 1 || len(g.dev.duty["port1-fan1"]) != 1 {
		t.Fatalf("%v %v", g.dev.taken, g.dev.duty)
	}
	if v := e.CoolingView(); v.Blocked != "" {
		t.Fatalf("%+v", v)
	}
}

func TestLiveLetsGoWhenARivalStartsAndWritesNothingAfterThat(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	e, _ := liveEngine(t, g)
	e.cooling.ctrl.Tick()

	g.reason, g.program = coolingBlockedProgram, "Rival"
	e.cooling.ctrl.Tick()
	if g.dev.closed != 1 {
		t.Fatalf("closed %d times", g.dev.closed)
	}
	writes := len(g.dev.duty["port1-fan1"])
	// The rival is in charge now: ending CaseHUD must not overwrite it.
	if err := e.cooling.ctrl.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(g.dev.duty["port1-fan1"]) != writes {
		t.Fatalf("wrote behind the rival's back: %v", g.dev.duty)
	}
}

func TestLiveShutdownLeavesTheSafeDuty(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	e, _ := liveEngine(t, g)
	e.cooling.ctrl.Tick()
	if err := e.cooling.ctrl.Shutdown(); err != nil {
		t.Fatal(err)
	}
	got := g.dev.duty["port1-fan1"]
	if got[len(got)-1] != 70 {
		t.Fatalf("%v", got)
	}
	if _, ok := g.dev.duty["pump"]; ok {
		t.Fatal("pump written")
	}
}

func TestLiveReopensALostControllerAfterAPause(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900}), at: now}
	e, _ := liveEngine(t, g)
	e.cooling.ctrl.Tick()
	if g.opens != 1 {
		t.Fatalf("opened %d times", g.opens)
	}

	g.dev.readErr = errors.New("gone")
	e.cooling.ctrl.Tick()
	if g.dev.closed != 1 {
		t.Fatal("kept a broken handle")
	}
	if v := e.CoolingView(); !v.Outputs[0].DeviceLost || v.Outputs[1].DeviceLost {
		t.Fatalf("%+v", v.Outputs)
	}

	e.cooling.ctrl.Tick()
	if g.opens != 1 {
		t.Fatal("reopened without a pause")
	}

	g.dev.readErr = nil
	g.at = now.Add(coolingReopenAfter + time.Second)
	if err := e.cooling.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
	if g.opens != 2 || g.dev.taken["port1-fan1"] != 2 {
		t.Fatalf("opens %d taken %d", g.opens, g.dev.taken["port1-fan1"])
	}
}

func TestLiveOpenFailureKeepsThePumpWatched(t *testing.T) {
	g := &openGate{openErr: errors.New("no controller"), dev: newFakeTL(nil)}
	e, _ := liveEngine(t, g)
	if err := e.cooling.ctrl.Tick(); err == nil || !strings.Contains(err.Error(), "no controller") {
		t.Fatalf("%v", err)
	}
	v := e.CoolingView()
	if !v.Outputs[0].DeviceLost {
		t.Fatalf("%+v", v.Outputs[0])
	}
	if pump := v.Outputs[1]; pump.RPM == nil || pump.DeviceLost {
		t.Fatalf("%+v", pump)
	}
}

func TestCoolingConfigMode(t *testing.T) {
	for name, tc := range map[string]struct {
		file string
		ok   bool
	}{
		"unknown mode": {`{"mode":"on","outputs":[]}`, false},
		"preview":      {`{"mode":"preview","outputs":[]}`, true},
		"live":         {`{"mode":"live","outputs":[]}`, true},
		"live foreign": {`{"mode":"live","outputs":[{"id":"cpu"}]}`, false},
		"preview any":  {`{"outputs":[{"id":"cpu"}]}`, true},
	} {
		e := coolingEngine(t, tc.file, &coolingReadings{})
		if _, err := e.Store.loadCoolingConfig(); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A demo never reaches a device, whatever the file says.
func TestDemoStaysInPreview(t *testing.T) {
	e := coolingEngine(t, coolingLiveExample, &coolingReadings{})
	e.Demo = true
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.startCooling()
	defer func() {
		cancel()
		e.waitCooling()
	}()
	if e.cooling.fans != nil || e.CoolingView().Live {
		t.Fatal("demo went live")
	}
}

type fakeProcs struct {
	conflicts.System
	names []string
	err   error
}

func (f fakeProcs) Processes(want func(string) bool) ([]conflicts.Proc, error) {
	var out []conflicts.Proc
	for _, n := range f.names {
		if want(n) {
			out = append(out, conflicts.Proc{Exe: n})
		}
	}
	return out, f.err
}

func TestCheckRivals(t *testing.T) {
	if r, _ := checkRivals(fakeProcs{names: []string{"notepad.exe"}}); r != "" {
		t.Errorf("%q", r)
	}
	if r, p := checkRivals(fakeProcs{names: []string{"fancontrol.EXE"}}); r != coolingBlockedProgram || p != "fancontrol.EXE" && p != "fancontrol" {
		t.Errorf("%q %q", r, p)
	}
	if r, _ := checkRivals(fakeProcs{err: errors.New("denied")}); r != coolingBlockedCheck {
		t.Errorf("failed check must block, got %q", r)
	}
}

func TestStopWaitsForTheLoop(t *testing.T) {
	done := make(chan struct{})
	e := &Engine{}
	e.cooling.done = done
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()
	start := time.Now()
	e.waitCooling()
	if time.Since(start) < 40*time.Millisecond {
		t.Fatal("did not wait")
	}
}

func TestWaitForSensorsEndsWithTheContext(t *testing.T) {
	cfg := coolingConfig{Curves: []coolingCurveConfig{{ID: "c", Sources: []string{"/t"}}}, Outputs: []coolingOutputConfig{{ID: "a", Curve: "c"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	waitForSensors(ctx, cfg, func() map[string]float64 { return nil }, time.Minute)
	if time.Since(start) > 5*time.Second {
		t.Fatal("waited on")
	}
	waitForSensors(context.Background(), cfg, func() map[string]float64 { return map[string]float64{"/t": 30} }, time.Minute)
}

func TestLiveRefusesAFanPosingAsAPump(t *testing.T) {
	e := coolingEngine(t, `{"mode":"live","outputs":[{"id":"port1-fan1","pump":true}]}`, &coolingReadings{})
	if _, err := e.Store.loadCoolingConfig(); err == nil {
		t.Fatal("accepted")
	}
}

// With everything unplugged no loop runs, and the page must still know that
// CaseHUD and not another program is meant to be in charge.
func TestLiveWithNothingConnectedStaysLive(t *testing.T) {
	e := coolingEngine(t, `{"mode":"live","outputs":[{"id":"port1-fan1"}]}`, &coolingReadings{})
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.launchCooling(cfg, 0); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingView(); !v.Live || len(v.Outputs) != 0 {
		t.Fatalf("%+v", v)
	}
}

func TestSameOutputsComparesTheKind(t *testing.T) {
	c := func(pump bool) coolingConfig {
		return coolingConfig{Outputs: []coolingOutputConfig{{ID: "a", Curve: "c", Pump: pump}}}
	}
	if sameOutputs(c(false), c(true)) {
		t.Fatal("a fan and a pump count as the same")
	}
	if !sameOutputs(c(true), c(true)) {
		t.Fatal("same")
	}
}
