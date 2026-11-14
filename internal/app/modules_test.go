package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

func TestModulesAllOnByDefault(t *testing.T) {
	e := startDemo(t)
	if m := e.Modules(); m != (Modules{true, true, true}) {
		t.Fatalf("%+v", m)
	}
	// An existing config.json has none of the fields.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"language":"de","panels":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := (&Store{Dir: dir}).LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if m := modulesOf(&c); m != (Modules{true, true, true}) {
		t.Fatalf("%+v", m)
	}
}

func TestModulesRefuseEverythingOff(t *testing.T) {
	e := startDemo(t)
	if err := e.SetModules(Modules{}); !errors.Is(err, ErrNoModule) {
		t.Fatalf("got %v", err)
	}
	if m := e.Modules(); m != (Modules{true, true, true}) {
		t.Fatalf("state changed: %+v", m)
	}
	if err := e.SetModules(Modules{Cooling: true}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetModules(Modules{}); !errors.Is(err, ErrNoModule) {
		t.Fatalf("got %v", err)
	}
	if m := e.Modules(); m != (Modules{Cooling: true}) {
		t.Fatalf("%+v", m)
	}
}

func TestModulesFileWithEverythingOffIsRepaired(t *testing.T) {
	dir := t.TempDir()
	bad := `{"no_display":true,"no_lighting":true,"no_cooling":true,"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if m := e.Modules(); m != (Modules{true, true, true}) {
		t.Fatalf("%+v", m)
	}
	if len(e.Panels()) == 0 {
		t.Fatal("an empty app")
	}
}

func TestModulesSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := e.SetModules(Modules{Display: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e2.Stop()
	if m := e2.Modules(); m != (Modules{Display: true, Cooling: true}) {
		t.Fatalf("%+v", m)
	}
	if e2.light.eng != nil {
		t.Fatal("lighting started although it is off")
	}
}

func TestDisplayOffClosesDisplaysAndOnBringsThemBack(t *testing.T) {
	e := startDemo(t)
	if len(e.Panels()) == 0 {
		t.Fatal("no demo displays")
	}
	if err := e.SetModules(Modules{Lighting: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	if n := len(e.Panels()); n != 0 {
		t.Fatalf("%d displays still open", n)
	}
	e.Rescan() // a manual rescan or a hot-plug must not open them either
	if n := len(e.Panels()); n != 0 {
		t.Fatalf("%d displays after a rescan", n)
	}
	if err := e.SetModules(Modules{true, true, true}); err != nil {
		t.Fatal(err)
	}
	if len(e.Panels()) == 0 {
		t.Fatal("displays did not come back")
	}
}

func TestLightingOffStopsTheEngineAndKeepsItsSettings(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if !e.Lighting().Active {
		t.Fatal("not running")
	}
	if err := e.SetModules(Modules{Display: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	v := e.Lighting()
	if v.Active || !v.Enabled {
		t.Fatalf("active %v enabled %v: the engine must stop, the user's own switch stay", v.Active, v.Enabled)
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err == nil {
		t.Fatal("lighting was switched on while the area is off")
	}
	if _, err := e.PatchLighting(LightingPatch{Brightness: ptr(40)}); err != nil {
		t.Fatalf("settings of a switched-off area must stay editable: %v", err)
	}
	if e.light.eng != nil {
		t.Fatal("engine started")
	}
	if err := e.SetModules(Modules{true, true, true}); err != nil {
		t.Fatal(err)
	}
	if v := e.Lighting(); !v.Active || v.Brightness != 40 {
		t.Fatalf("not resumed: active %v brightness %d", v.Active, v.Brightness)
	}
}

// liveRig is an engine with a live cooling loop on fakes.
type liveRig struct {
	e    *Engine
	tl   *fakeTL
	line *fakePumpLine
	file string
}

func newLiveRig(t *testing.T, demo bool) *liveRig {
	t.Helper()
	tl := newFakeTL(map[string]int{"port1-fan1": 900})
	line := useFakePumpLine(t)
	before := coolingOpenFans
	coolingOpenFans = func(context.Context) (lianlitl.FanDevice, error) { return tl, nil }
	t.Cleanup(func() { coolingOpenFans = before })
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: demo}
	e.prov = NewSafeProvider(&coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: "/lpc/chip/0/fan/6", Value: 3000, Unit: "RPM"},
	}})
	e.conf.sys = &procSys{}
	e.cfg = Config{Panels: map[string]PanelConfig{}}
	e.cfg.Disclaimer = DisclaimerAck{Version: DisclaimerVersion}
	e.ctx, e.cancelFn = context.WithCancel(context.Background())
	t.Cleanup(e.cancelFn)
	file := filepath.Join(e.Store.Dir, coolingFile)
	if err := os.WriteFile(file, []byte(coolingLiveExample), 0o600); err != nil {
		t.Fatal(err)
	}
	// The pump is proven to sit on its header already.
	proof := `{"v":1,"pumps":{"pump":{"control":"/lpc/chip/0/control/6","sensor":"/lpc/chip/0/fan/6","result":"ok","board":"chip"}}}`
	if err := os.WriteFile(filepath.Join(e.Store.Dir, coolingMatchFile), []byte(proof), 0o600); err != nil {
		t.Fatal(err)
	}
	return &liveRig{e: e, tl: tl, line: line, file: file}
}

// run starts the live loop the way a start with cooling.json does (not in demo).
func (r *liveRig) run(t *testing.T) {
	t.Helper()
	r.e.startCooling()
	if r.e.coolingSnapshot().fans == nil {
		t.Fatal("no live loop")
	}
	// The loop ticks once a second; one tick now makes the outputs ours.
	if err := r.e.coolingSnapshot().ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
}

func writes(f *fakeTL) int {
	n := 0
	for _, d := range f.duty {
		n += len(d)
	}
	return n
}

func TestCoolingOffSendsNothingAndGivesTheHardwareBack(t *testing.T) {
	r := newLiveRig(t, false)
	r.run(t)
	if len(r.tl.duty["port1-fan1"]) == 0 || len(r.line.holds) == 0 {
		t.Fatalf("nothing driven: %v %v", r.tl.duty, r.line.holds)
	}
	fileBefore, _ := os.ReadFile(r.file)

	if err := r.e.SetModules(Modules{Display: true, Lighting: true}); err != nil {
		t.Fatal(err)
	}
	duty := r.tl.duty["port1-fan1"]
	if last := duty[len(duty)-1]; last < 70 {
		t.Fatalf("fan left at %d %%, the safe duty is 70", last)
	}
	if r.line.closed == 0 {
		t.Fatal("the pump was not handed back to the board")
	}
	if r.tl.closed == 0 {
		t.Fatal("the fan controller was kept open")
	}
	w, h := writes(r.tl), len(r.line.holds)
	time.Sleep(1300 * time.Millisecond) // longer than one loop tick
	if writes(r.tl) != w || len(r.line.holds) != h {
		t.Fatal("something was sent after cooling was switched off")
	}
	if v := r.e.CoolingView(); !v.Disabled || len(v.Outputs) != 0 || v.Alarm != nil {
		t.Fatalf("%+v", v)
	}
	if a := r.e.currentCoolingAlarm(); a != nil {
		t.Fatalf("alarm %+v", a)
	}
	if err := r.e.SetCoolingConfigJSON([]byte(coolingLiveExample)); err == nil {
		t.Fatal("cooling.json was written while cooling is off")
	}
	if err := r.e.SetCoolingMode("preview"); err == nil {
		t.Fatal("mode changed while cooling is off")
	}
	if after, _ := os.ReadFile(r.file); !bytes.Equal(fileBefore, after) {
		t.Fatal("cooling.json changed")
	}

	// Back on: the loop runs from the file as before.
	if err := r.e.SetModules(Modules{true, true, true}); err != nil {
		t.Fatal(err)
	}
	if c := r.e.coolingSnapshot(); c.fans == nil || c.cfg.Mode != "live" || len(c.cfg.Outputs) != 2 {
		t.Fatalf("%+v", c.cfg)
	}
	r.e.stopCoolingModule()
}

// installLive puts a live loop on fakes into a running (demo) engine, next to
// its lighting. Demo never drives hardware by itself, so this is the only way
// to see both areas at work in one engine.
func installLive(t *testing.T, e *Engine) (*fakeTL, *fakePumpLine, *tlFans) {
	t.Helper()
	tl := newFakeTL(map[string]int{"port1-fan1": 900})
	line := useFakePumpLine(t)
	cfg, err := parseCoolingConfig([]byte(coolingLiveExample))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(e.ctx)
	fans := &tlFans{ctx: ctx, now: time.Now, gate: func() (string, string) { return "", "" },
		open: func(context.Context) (lianlitl.FanDevice, error) { return tl, nil }}
	pumps := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
	pumps.markMatched(pumps.ids()...)
	ctrl, err := newCoolingLive(cfg, e.prov, fans, pumps)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	e.setCoolingState(coolingState{ctrl: ctrl, cfg: cfg, fans: fans, pumps: pumps, ctx: ctx, cancel: cancel, done: done, configured: true})
	go e.runCooling(ctx, ctrl, cfg, fans, pumps, done, nil, 0)
	_ = ctrl.Tick() // reports the lost sensor of the demo; the safe duty goes out all the same
	return tl, line, fans
}

func TestLightingOffKeepsTheFansRunning(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	tl, line, _ := installLive(t, e)
	if err := e.SetModules(Modules{Display: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	if e.Lighting().Active {
		t.Fatal("the light kept running")
	}
	c := e.coolingSnapshot()
	if c.fans == nil || tl.closed != 0 || line.closed != 0 {
		t.Fatal("the cooling was disturbed")
	}
	before := writes(tl)
	c.ctrl.Reacquire() // sends every output again
	_ = c.ctrl.Tick()
	if writes(tl) <= before {
		t.Fatal("the fans are no longer driven")
	}
	if v := e.CoolingView(); v.Disabled || !v.Live || len(v.Outputs) != 2 {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingOffKeepsTheLightRunning(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetModules(Modules{Display: true, Lighting: true}); err != nil {
		t.Fatal(err)
	}
	if !e.Lighting().Active {
		t.Fatal("the light stopped with the cooling")
	}
	if _, err := e.PatchLighting(LightingPatch{Brightness: ptr(55)}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetModules(Modules{true, true, true}); err != nil {
		t.Fatal(err)
	}
	if !e.Lighting().Active {
		t.Fatal("light stopped")
	}
}

func TestCoolingModeSwitchKeepsTheRestOfTheConfiguration(t *testing.T) {
	r := newLiveRig(t, false)
	r.run(t)
	var before coolingConfig
	if c, err := r.e.Store.loadCoolingConfig(); err != nil {
		t.Fatal(err)
	} else {
		before = c
	}
	if err := r.e.SetCoolingMode("preview"); err != nil {
		t.Fatal(err)
	}
	c, err := r.e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "preview" {
		t.Fatalf("mode %q", c.Mode)
	}
	c.Mode = before.Mode
	if a, b := mustJSON(t, c), mustJSON(t, before); a != b {
		t.Fatalf("configuration changed:\n%s\n%s", a, b)
	}
	if v := r.e.CoolingView(); v.Live || v.Mode != "preview" {
		t.Fatalf("%+v", v)
	}
	if err := r.e.SetCoolingMode("live"); err != nil {
		t.Fatal(err)
	}
	if v := r.e.CoolingView(); !v.Live || v.Mode != "live" {
		t.Fatalf("%+v", v)
	}
	if err := r.e.SetCoolingMode("fast"); err == nil {
		t.Fatal("unknown mode accepted")
	}
	r.e.stopCoolingModule()
}

func TestCoolingModeSwitchUsesTheChecksOfSaving(t *testing.T) {
	// A pump on a header that cannot be set passes as a test, not as live.
	file := strings.Replace(pumpExample, `"mode":"live"`, `"mode":"preview"`, 1)
	file = strings.Replace(file, `"rpm_sensor":"`+pumpRPMKey+`"`, `"rpm_sensor":"/lpc/chip/0/temperature/6"`, 1)
	r := newLiveRig(t, false)
	if err := os.WriteFile(r.file, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	r.e.startCooling()
	before, _ := os.ReadFile(r.file)
	if err := r.e.SetCoolingMode("live"); err == nil {
		t.Fatal("accepted")
	}
	if after, _ := os.ReadFile(r.file); !bytes.Equal(before, after) {
		t.Fatal("file changed by a refused switch")
	}
	r.e.stopCoolingModule()
}

func TestCoolingModeSwitchNeedsAUsableFile(t *testing.T) {
	r := newLiveRig(t, false)
	if err := os.WriteFile(r.file, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.e.startCooling()
	if err := r.e.SetCoolingMode("live"); err == nil {
		t.Fatal("a broken file was overwritten")
	}
	if b, _ := os.ReadFile(r.file); string(b) != "{" {
		t.Fatalf("%q", b)
	}
}

func mustJSON(t *testing.T, c coolingConfig) string {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
