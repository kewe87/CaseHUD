package app

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

func ptr[T any](v T) *T { return &v }

func patchDev(id string, zone string, zp LightingZonePatch) LightingPatch {
	return LightingPatch{Devices: map[string]LightingDevicePatch{id: {Zones: map[string]LightingZonePatch{zone: zp}}}}
}

func TestLightingOffByDefault(t *testing.T) {
	e := startDemo(t)
	v := e.Lighting()
	if v.Enabled || v.Active || len(v.Devices) != 0 {
		t.Fatalf("lighting must be off by default: %+v", v)
	}
	if len(v.Effects) < 6 {
		t.Fatalf("effects: %d", len(v.Effects))
	}
}

func TestLightingEnablePatchAndPersist(t *testing.T) {
	e := startDemo(t)
	v, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Brightness: ptr(40), Effect: ptr("wave"),
		Params: map[string]any{"speed": 500.0, "color": "#FF0000"}})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Enabled || !v.Active || v.Effect != "wave" || v.Brightness != 40 {
		t.Fatalf("view: %+v", v)
	}
	if v.Params["speed"] != 100.0 || v.Params["color"] != "#ff0000" {
		t.Fatalf("params not clamped/normalised: %v", v.Params)
	}
	if len(v.Devices) != 4 {
		t.Fatalf("devices: %d", len(v.Devices))
	}
	states := map[string]leds.DeviceState{}
	for _, d := range v.Devices {
		states[d.ID] = d.State
	}
	if states["demo:keyboard"] != leds.StateConflict || states["demo:ram"] != leds.StateOK {
		t.Fatalf("states: %v", states)
	}
	v, err = e.PatchLighting(patchDev("demo:board", "argb1", LightingZonePatch{Count: ptr(30)}))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range v.Devices {
		for _, z := range d.Zones {
			if d.ID == "demo:board" && z.ID == "argb1" {
				n = z.LEDs
			}
		}
	}
	if n != 30 {
		t.Fatalf("argb1 LEDs %d", n)
	}
	e.saveLighting()
	cfg, err := e.Store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Lighting.Enabled || cfg.Lighting.Engine == nil || cfg.Lighting.Engine.Effect != "wave" || cfg.Lighting.Engine.Brightness != 40 {
		t.Fatalf("not persisted: %+v", cfg.Lighting)
	}
	b, _ := os.ReadFile(filepath.Join(e.Store.Dir, "config.json"))
	if !strings.Contains(string(b), `"lighting"`) {
		t.Fatal("config.json has no lighting section")
	}
	v, err = e.PatchLighting(LightingPatch{Enabled: ptr(false)})
	if err != nil || v.Active || len(v.Devices) != 0 || v.Effect != "wave" {
		t.Fatalf("disable: %+v %v", v, err)
	}
}

func TestLightingRejectsBadInput(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	before := e.Lighting()
	many := make([]leds.Segment, maxLightSegments+1)
	for i := range many {
		many[i] = leds.Segment{Count: 1, Shape: "point", X: .5, Y: .5}
	}
	seg := func(s leds.Segment) LightingPatch {
		return patchDev("demo:board", "argb1", LightingZonePatch{Segments: &[]leds.Segment{s}})
	}
	bad := []LightingPatch{
		{Effect: ptr("screen")},
		{Effect: ptr("../../x")},
		{Brightness: ptr(101)},
		{Brightness: ptr(-1)},
		{Params: map[string]any{"nope": 1.0}},
		{Params: map[string]any{"speed": "fast"}},
		{Effect: ptr("solid"), Params: map[string]any{"color": "red"}},
		{Effect: ptr("solid"), Params: map[string]any{"color": "#12"}},
		{Params: map[string]any{"speed": math.NaN()}},
		{TL: ptr(true)},
		{Devices: map[string]LightingDevicePatch{"nope": {Disabled: ptr(true)}}},
		patchDev("demo:board", "nozone", LightingZonePatch{Count: ptr(3)}),
		patchDev("demo:board", "argb1", LightingZonePatch{Count: ptr(5000)}),
		patchDev("demo:board", "argb1", LightingZonePatch{Count: ptr(121)}),
		patchDev("demo:board", "onboard", LightingZonePatch{Count: ptr(3)}),
		patchDev("demo:board", "argb1", LightingZonePatch{Order: ptr("RRR")}),
		patchDev("demo:board", "argb1", LightingZonePatch{Segments: &many}),
		seg(leds.Segment{Count: 10, Shape: "line", X: 2}),
		seg(leds.Segment{Count: 10, Shape: "blob"}),
		seg(leds.Segment{Count: 10, Shape: "component", Component: "../x"}),
		seg(leds.Segment{Count: 1 << 30, Shape: "line"}),
		seg(leds.Segment{Count: 10, Shape: "line", W: math.NaN()}),
		{Devices: map[string]LightingDevicePatch{"demo:board": {Settings: map[string]any{"x": 1.0}}}},
		{Brightness: ptr(10), Effect: ptr("nonexistent")}, // the valid part must not apply either
	}
	for i, p := range bad {
		if _, err := e.PatchLighting(p); err == nil {
			t.Errorf("case %d accepted: %+v", i, p)
		}
	}
	after := e.Lighting()
	if after.Brightness != before.Brightness || after.Effect != before.Effect {
		t.Fatalf("rejected requests changed state: %+v -> %+v", before, after)
	}
}

func TestCleanLightingConfigLimits(t *testing.T) {
	c := leds.DefaultConfig()
	c.FPS = 100000
	c.Brightness = 999
	c.Devices = map[string]leds.DeviceConfig{"../evil": {}}
	if _, err := cleanLightingConfig(c, nil); err == nil {
		t.Fatal("bad device id accepted")
	}
	c.Devices = map[string]leds.DeviceConfig{"ok:1": {Settings: leds.Params{"k": []any{1}}}}
	if _, err := cleanLightingConfig(c, nil); err == nil {
		t.Fatal("nested setting accepted")
	}
	c.Devices = nil
	out, err := cleanLightingConfig(c, nil)
	if err != nil || out.FPS != maxLightFPS || out.Brightness != 100 {
		t.Fatalf("clamp: %+v %v", out, err)
	}
	for _, ok := range []string{"", leds.ShutdownKeep, leds.ShutdownDevice, "#FF0080"} {
		c.ShutdownColor = ok
		if _, err := cleanLightingConfig(c, nil); err != nil {
			t.Fatalf("shutdown %q rejected: %v", ok, err)
		}
	}
	c.ShutdownColor = "javascript:1"
	if _, err := cleanLightingConfig(c, nil); err == nil {
		t.Fatal("bad shutdown colour accepted")
	}
}

func TestLightingBadStoredConfigStaysOff(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"lighting":{"enabled":true,"engine":{"version":1,"effect":"screen"}},"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if v := e.Lighting(); v.Enabled || v.Active {
		t.Fatalf("untrusted stored config must not start lighting: %+v", v)
	}
}

func TestLightingCalibrateAndSleep(t *testing.T) {
	e := startDemo(t)
	if err := e.LightingCalibrate("demo:ram", "stick1", 0); err == nil {
		t.Fatal("calibration while lighting is off")
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if err := e.LightingCalibrate("demo:ram", "stick1", 2); err != nil {
		t.Fatal(err)
	}
	if v := e.Lighting(); v.Calibrating == nil || v.Calibrating.Pos != 2 {
		t.Fatalf("calibrating: %+v", v.Calibrating)
	}
	type call struct {
		dev, zone string
		pos       int
	}
	for _, c := range []call{{"demo:ram", "stick1", 99}, {"demo:ram", "nozone", 0}, {"nodev", "x", 0}, {"demo:keyboard", "keys", 0}, {"../x", "y", 0}} {
		if err := e.LightingCalibrate(c.dev, c.zone, c.pos); err == nil {
			t.Errorf("calibrate %v accepted", c)
		}
	}
	e.LightingEndCalibration()
	if v := e.Lighting(); v.Calibrating != nil {
		t.Fatal("calibration still on")
	}
	e.SetPower(false)
	if !e.Lighting().Paused {
		t.Fatal("lighting not paused with the displays")
	}
	e.SetPower(true)
	if e.Lighting().Paused {
		t.Fatal("lighting not resumed")
	}
	if err := e.LightingBlink(2); err == nil {
		t.Fatal("blink without TL backend")
	}
}

func TestLightingStartStopNoLeak(t *testing.T) {
	e := startDemo(t)
	for i := 0; i < 5; i++ {
		if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(false)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	e.Stop() // twice is fine
	time.Sleep(50 * time.Millisecond)
	if e.light.eng != nil {
		t.Fatal("engine survived Stop")
	}
}

// slowBackend is a fake backend whose discovery can be held up.
type slowBackend struct {
	*fake.Backend
	hold    atomic.Bool
	entered chan struct{}
	gate    chan struct{}
}

func (b *slowBackend) Discover(ctx context.Context) ([]leds.Device, error) {
	if b.hold.Load() {
		select {
		case b.entered <- struct{}{}:
		default:
		}
		select {
		case <-b.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return b.Backend.Discover(ctx)
}

// A rescan that waits for a device must not hold up the state the GUI polls.
func TestLightingStateNeverWaitsForDeviceIO(t *testing.T) {
	e := startDemo(t)
	sb := &slowBackend{Backend: &fake.Backend{Devices: []*fake.Device{{DeviceID: "slow:1", DeviceName: "Slow",
		ZoneList: []leds.Zone{{ID: "z", Name: "Z", Count: 4, Max: 4, Order: "RGB"}}}}},
		entered: make(chan struct{}, 1), gate: make(chan struct{})}
	eng := leds.New(leds.DefaultConfig(), leds.Options{Backends: []leds.Backend{sb}, Effects: lightingEffects, Sensors: e.lightSensors(), RescanInterval: time.Hour})
	if err := eng.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	l := &e.light
	l.mu.Lock()
	l.eng = eng
	l.engPtr.Store(eng)
	l.mu.Unlock()
	e.mu.Lock()
	e.cfg.Lighting.Enabled = true
	e.mu.Unlock()

	sb.hold.Store(true)
	done := make(chan error, 1)
	go func() { done <- e.RescanLighting() }()
	select {
	case <-sb.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("rescan did not reach the device")
	}
	fast := func(name string, f func()) {
		t.Helper()
		ch := make(chan struct{})
		go func() { f(); close(ch) }()
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("%s waited for device I/O", name)
		}
	}
	fast("Lighting", func() { e.Lighting() })
	fast("LightingBrief", func() { e.LightingBrief() })
	fast("Arrangement", func() { e.Arrangement() })
	fast("PatchLighting", func() {
		if _, err := e.PatchLighting(LightingPatch{Brightness: ptr(33)}); err != nil {
			t.Error(err)
		}
	})
	fast("EndCalibration", func() { e.LightingEndCalibration() })
	if v := e.Lighting(); v.Brightness != 33 {
		t.Fatalf("brightness %d", v.Brightness)
	}
	close(sb.gate)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("rescan did not finish")
	}
	// detach before the cleanup stops the engine through the normal path
	l.mu.Lock()
	l.eng = nil
	l.engPtr.Store(nil)
	l.mu.Unlock()
	if err := eng.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestLightingBadStoredConfigIsBackedUp(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"lighting":{"enabled":true,"engine":{"version":1,"effect":"screen"}},"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	v := e.Lighting()
	if !v.ConfigReset || !strings.HasSuffix(v.ConfigBackup, "config.lighting.bad.json") {
		t.Fatalf("no reset hint: %+v", v)
	}
	b, err := os.ReadFile(v.ConfigBackup)
	if err != nil || !strings.Contains(string(b), `"screen"`) {
		t.Fatalf("backup: %v %s", err, b)
	}
	e.Stop() // saves the defaults over the stored settings
	b, _ = os.ReadFile(filepath.Join(dir, "config.json"))
	if strings.Contains(string(b), `"screen"`) {
		t.Fatal("the bad settings are still in config.json")
	}
	if b, err = os.ReadFile(v.ConfigBackup); err != nil || !strings.Contains(string(b), `"screen"`) {
		t.Fatal("backup lost")
	}
}

// The fan list is read from the controller only on request, or through the
// engine's own handle; never by itself while lighting is off.
func TestTopologyNeverQueriedInTheBackground(t *testing.T) {
	e := startDemo(t)
	var calls atomic.Int32
	query := func(context.Context) ([]lianlitl.Topology, error) {
		calls.Add(1)
		return []lianlitl.Topology{{Fans: []lianlitl.FanInfo{{Port: 2, Index: 1, LCD: true}, {Port: 2, Index: 2}}}}, nil
	}
	src := lightTopology{e}
	for i := 0; i < 5; i++ {
		src.Fans()
		e.Arrangement()
	}
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("topology queried %d times without a request", calls.Load())
	}
	if err := e.scanFans(query); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("scan queried %d times", calls.Load())
	}
	fans := src.Fans()
	if len(fans) != 2 || fans[0].Port != 2 || fans[0].Index != 0 || !fans[0].HasLCD {
		t.Fatalf("fans %+v", fans)
	}
	if si := src.ScanInfo(); si.At == "" || si.Live {
		t.Fatalf("scan info %+v", si)
	}
	e.mu.Lock()
	stored := len(e.cfg.Lighting.Fans)
	e.mu.Unlock()
	if stored != 2 {
		t.Fatalf("stored %d fans", stored)
	}
	if e.ScanFans() == nil {
		t.Fatal("the demo must not touch a controller")
	}
	if calls.Load() != 1 {
		t.Fatal("demo scan reached the query")
	}
}

func TestLoadFansSanitizes(t *testing.T) {
	var l lightingState
	lc := LightingConfig{FansAt: "garbage", Fans: []FanPos{{Port: 2, Index: 0, ID: "x"}, {Port: 9, Index: 0}, {Port: 1, Index: -1}, {Port: 3, Index: 99}}}
	l.loadFans(lc)
	if len(l.topo) != 1 || l.topo[0].ID != "" || !l.topoAt.IsZero() {
		t.Fatalf("%+v %v", l.topo, l.topoAt)
	}
}

func TestMigrateLightSettingsRate(t *testing.T) {
	in := leds.Params{"rate": 2.0, "mode": "canvas"}
	out := migrateLightSettings(in)
	if _, ok := out["rate"]; ok || out["interval"] != 60.0 || out["mode"] != "canvas" {
		t.Fatalf("migrated: %v", out)
	}
	if _, ok := in["rate"]; !ok {
		t.Fatal("input modified")
	}
	keep := migrateLightSettings(leds.Params{"rate": 1.0, "interval": 120.0})
	if keep["interval"] != 120.0 {
		t.Fatalf("existing interval lost: %v", keep)
	}
	cfg := leds.DefaultConfig()
	cfg.Devices = map[string]leds.DeviceConfig{"tl:1": {Settings: leds.Params{"rate": 5.0}}}
	clean, err := cleanLightingConfig(cfg, nil)
	if err != nil || clean.Devices["tl:1"].Settings["interval"] != 60.0 {
		t.Fatalf("config: %v %v", clean.Devices, err)
	}
}

func TestFeaturedEffectsKnownToValidation(t *testing.T) {
	for _, f := range lightingFeatured {
		if _, ok := lightingEffect(f.name); !ok {
			t.Errorf("featured effect %q does not exist", f.name)
		}
	}
	if lightingIsFeatured("fill") || !lightingIsFeatured("sensor") {
		t.Fatal("featured list")
	}
	if _, ok := lightingEffect("fill"); !ok {
		t.Fatal("hidden effects must stay valid")
	}
}

func TestLightingParamLabelsAndSteps(t *testing.T) {
	for _, f := range lightingFeatured {
		info, _ := lightingEffect(f.name)
		for _, p := range lightingParamViews(f.name, info.Params, true) {
			if p.Label == "" || p.LabelDE == "" {
				t.Errorf("%s.%s: missing label/labelDe", f.name, p.Key)
			}
			if (p.Type == leds.ParamNumber) != (p.Step > 0) {
				t.Errorf("%s.%s: step %v for type %s", f.name, p.Key, p.Step, p.Type)
			}
		}
	}
	info, _ := lightingEffect("breathing")
	for _, p := range lightingParamViews("breathing", info.Params, true) {
		if p.Key == "period" && p.Step != 0.1 {
			t.Errorf("period step = %v", p.Step)
		}
	}
	info, _ = lightingEffect("rainbow")
	for _, p := range lightingParamViews("rainbow", info.Params, true) {
		if p.Key == "speed" && p.Step != 1 {
			t.Errorf("speed step = %v", p.Step)
		}
	}
}

func TestLightingShutdownPatch(t *testing.T) {
	e := startDemo(t)
	if v := e.Lighting(); v.Shutdown != "off" {
		t.Fatalf("default shutdown %q", v.Shutdown)
	}
	for _, bad := range []string{"", "on", "#ff0000", "Keep"} {
		if _, err := e.PatchLighting(LightingPatch{Shutdown: ptr(bad)}); !errors.Is(err, ErrLighting) {
			t.Fatalf("shutdown %q: %v", bad, err)
		}
	}
	for want, stored := range map[string]string{"device": leds.ShutdownDevice, "keep": leds.ShutdownKeep, "off": ""} {
		v, err := e.PatchLighting(LightingPatch{Shutdown: ptr(want)})
		if err != nil || v.Shutdown != want {
			t.Fatalf("%s: %+v %v", want, v.Shutdown, err)
		}
		e.light.mu.Lock()
		got := e.light.cfg.ShutdownColor
		e.light.mu.Unlock()
		if got != stored {
			t.Fatalf("%s stored as %q", want, got)
		}
	}
	// other fields keep the setting (merge semantics)
	e.PatchLighting(LightingPatch{Shutdown: ptr("keep")})
	if v, err := e.PatchLighting(LightingPatch{Brightness: ptr(20)}); err != nil || v.Shutdown != "keep" {
		t.Fatalf("merge: %q %v", v.Shutdown, err)
	}
	if shutdownName("#ff0000") != "color" {
		t.Fatal("hex colour name")
	}
}
