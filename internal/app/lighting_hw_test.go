package app

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

func backendNames(bs []leds.Backend) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Name())
	}
	return out
}

func TestLightBackendsGating(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	b, tap := e.lightBackends(false)
	if tap != nil || strings.Join(backendNames(b), ",") != "asusaura,lianli,enram" {
		t.Fatalf("without the TL switch: %v tap=%v", backendNames(b), tap)
	}
	b, tap = e.lightBackends(true)
	if tap == nil || strings.Join(backendNames(b), ",") != "asusaura,lianli,enram,lianlitl" {
		t.Fatalf("with the TL switch: %v", backendNames(b))
	}
}

// stubBackend returns fixed results.
type stubBackend struct {
	name  string
	devs  []leds.Device
	err   error
	calls int
}

func (s *stubBackend) Name() string { return s.name }
func (s *stubBackend) Discover(context.Context) ([]leds.Device, error) {
	s.calls++
	return s.devs, s.err
}

func TestRAMNeedsTheBridge(t *testing.T) {
	inner := &stubBackend{name: "enram", devs: []leds.Device{&fake.Device{DeviceID: "enram:0b00:71", DeviceName: "stick"}}}
	var notes ramNotes
	avail := false
	g := newRAMBackend(inner, func() bool { return avail }, &notes)
	devs, err := g.Discover(context.Background())
	if err != nil || len(devs) != 0 || inner.calls != 0 {
		t.Fatalf("no bridge: devs=%d err=%v calls=%d (the bus must not be asked)", len(devs), err, inner.calls)
	}
	avail = true
	devs, err = g.Discover(context.Background())
	if err != nil || len(devs) != 1 || inner.calls != 1 {
		t.Fatalf("bridge: devs=%d err=%v", len(devs), err)
	}
}

func TestRAMViewHint(t *testing.T) {
	old := smbusBridgeProbe
	defer func() { smbusBridgeProbe = old }()
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	probe := func(up bool) {
		smbusBridgeProbe = func() bool { return up }
		e.light.bridgeAt.Store(0)
		e.refreshBridgeProbe()
	}
	probe(false)
	if r := e.ramView(); r.Available || !strings.Contains(r.Hint, "SMBus-Brücke") || !strings.Contains(r.Hint, "sensors.json") {
		t.Fatalf("hint: %+v", r)
	}
	probe(true)
	if r := e.ramView(); !r.Available || r.Hint != "" {
		t.Fatalf("bridge there: %+v", r)
	}
}

func TestRAMUnknownStickBusyDenied(t *testing.T) {
	var notes ramNotes
	unknown := fmt.Errorf("%#02x: %w", 0x72, enram.ErrUnknownStick{Addr: 0x72, Name: "AUMA-E8K4"})
	inner := &stubBackend{name: "enram", err: errors.Join(unknown)}
	g := newRAMBackend(inner, func() bool { return true }, &notes)
	devs, err := g.Discover(context.Background())
	if err != nil || len(devs) != 1 {
		t.Fatalf("unknown stick: %d devices, err %v", len(devs), err)
	}
	u := devs[0]
	if !strings.HasPrefix(u.ID(), unsupportedPref) || len(u.Zones()) != 0 || !strings.Contains(u.Name(), "nicht unterstützt") {
		t.Fatalf("placeholder: %s %q", u.ID(), u.Name())
	}
	if err := u.SetFrame(map[string][]color.RGBA{"x": {{R: 1}}}); err != nil {
		t.Fatal(err)
	}
	if notes.get() == "" {
		t.Fatal("note missing")
	}

	inner.err = errors.Join(fmt.Errorf("0x70: %w", enram.ErrBusy))
	_, err = g.Discover(context.Background())
	if err == nil || softLightErr(err) != "" {
		t.Fatalf("busy must keep the known devices (error) but stay out of the status line: %v / %q", err, softLightErr(err))
	}
	inner.err = enram.ErrDenied
	_, err = g.Discover(context.Background())
	if err == nil || !strings.Contains(softLightErr(err), "refused") || !strings.Contains(notes.get(), "abgelehnt") {
		t.Fatalf("denied: %v note=%q", err, notes.get())
	}
}

func TestRingOnlyTouchesTheFrame(t *testing.T) {
	display := &fake.Device{DeviceID: "lianli:1cbe:a088", DeviceName: "display"}
	ring := &fake.Device{DeviceID: ringDeviceID, DeviceName: "ring", ZoneList: []leds.Zone{{ID: "ring", Count: 60, Max: 240, Resizable: true}}}
	b := &ringBackend{inner: &stubBackend{name: "lianli", devs: []leds.Device{display, ring}}}
	devs, err := b.Discover(context.Background())
	if err != nil || len(devs) != 1 || devs[0].ID() != ringDeviceID || devs[0].Name() != ringName {
		t.Fatalf("devices: %v %v", devs, err)
	}
	if !strings.Contains(ringName, "LED-Rahmen") {
		t.Fatal(ringName)
	}
}

func TestDefaultRingLayout(t *testing.T) {
	segs := defaultRingSegments(60)
	clean, err := cleanSegments(segs)
	if err != nil || len(clean) != 1 || clean[0].Count != 60 {
		t.Fatalf("%v %v", clean, err)
	}
}

// No in-process bus access: nothing of CaseHUD may reach the PawnIO path.
func TestNoDirectBusAccessInTree(t *testing.T) {
	root := filepath.Join("..", "rgb")
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		b, _ := os.ReadFile(p)
		s := strings.ToLower(string(b))
		for _, bad := range []string{"newlocal", "pawnio", "piix4(", "openpiix4"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s mentions %q", p, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "pawnio")); err == nil {
		t.Fatal("pawnio is vendored")
	}
	src, _ := os.ReadFile("lighting.go")
	if strings.Contains(string(src), "NewLocal") || !strings.Contains(string(src), "enram.New(smbusBridgePipe)") {
		t.Fatal("RAM must be built with enram.New(bridge pipe)")
	}
}

func enableDemoLighting(t *testing.T) *Engine {
	t.Helper()
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	return e
}

func demoDevice(v LightingView, id string) (LightingDeviceView, bool) {
	for _, d := range v.Devices {
		if d.ID == id {
			return d, true
		}
	}
	return LightingDeviceView{}, false
}

func TestFlashModesNeedConsent(t *testing.T) {
	e := enableDemoLighting(t)
	// The engine applies settings on its next frame; wait for the warning to settle.
	set := func(s map[string]any) LightingView {
		if _, err := e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{"demo:fans": {Settings: s}}}); err != nil {
			t.Fatal(err)
		}
		want := s["mode"] == "hardware" && s["allowFlashWrites"] != true
		for i := 0; i < 100; i++ {
			v := e.Lighting()
			if d, _ := demoDevice(v, "demo:fans"); (d.Warning != "") == want {
				return v
			}
			time.Sleep(20 * time.Millisecond)
		}
		return e.Lighting()
	}
	d, _ := demoDevice(e.Lighting(), "demo:fans")
	if d.SettingValues["mode"] != "motherboard" || d.SettingValues["allowFlashWrites"] != false || d.Warning != "" {
		t.Fatalf("defaults: %+v", d.SettingValues)
	}
	d, _ = demoDevice(set(map[string]any{"mode": "hardware"}), "demo:fans")
	if !strings.Contains(d.Warning, "Flash-Speicher") {
		t.Fatalf("hardware mode without consent must warn: %q", d.Warning)
	}
	d, _ = demoDevice(set(map[string]any{"mode": "hardware", "allowFlashWrites": true}), "demo:fans")
	if d.Warning != "" {
		t.Fatalf("consent given, still a warning: %q", d.Warning)
	}
	d, _ = demoDevice(set(map[string]any{"mode": "motherboard", "allowFlashWrites": false}), "demo:fans")
	if d.Warning != "" || d.SettingValues["allowFlashWrites"] != false {
		t.Fatalf("back to sync: %+v", d)
	}
}

func TestSyncHelper(t *testing.T) {
	e := enableDemoLighting(t)
	tl, _ := demoDevice(e.Lighting(), "demo:fans")
	if tl.Sync == nil || tl.Sync.Fans != 3 || len(tl.Sync.Choices) != 2 || tl.Sync.Choices[0].ID != "demo:board/argb1" {
		t.Fatalf("sync info: %+v", tl.Sync)
	}
	if !tl.Zones[0].Mirrored || tl.Zones[0].MirroredFrom != "" {
		t.Fatalf("mirrored without source: %+v", tl.Zones[0])
	}
	v, err := e.LightingSetSync("demo:fans", "demo:board/argb1")
	if err != nil {
		t.Fatal(err)
	}
	tl, _ = demoDevice(v, "demo:fans")
	if tl.SettingValues["syncSource"] != "demo:board/argb1" || tl.Zones[1].MirroredFrom != "ARGB header 1" {
		t.Fatalf("tl: %+v / %+v", tl.SettingValues, tl.Zones)
	}
	board, _ := demoDevice(v, "demo:board")
	var z LightingZoneView
	for _, zz := range board.Zones {
		if zz.ID == "argb1" {
			z = zz
		}
	}
	if len(z.Segments) != 3 || z.LEDs != 78 || z.Segments[2].Component != "fan-tl-26" || z.Segments[0].Shape != "component" {
		t.Fatalf("segments: %+v", z)
	}
	if _, err := cleanLightingConfig(e.light.eng.Config(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.LightingSetSync("demo:fans", "demo:board/onboard"); err == nil {
		t.Fatal("a fixed zone is no ARGB header")
	}
	if _, err := e.LightingSetSync("demo:board", "demo:board/argb1"); err == nil {
		t.Fatal("the board is no fan controller")
	}
	if _, err := e.LightingSetSync("demo:fans", ""); err != nil {
		t.Fatal(err)
	}
}

func TestSyncSegmentsAreValid(t *testing.T) {
	for n := 1; n <= 10; n++ {
		segs := syncSegments(n)
		if _, err := cleanSegments(segs); err != nil {
			t.Fatalf("%d fans: %v", n, err)
		}
		total := 0
		for _, s := range segs {
			total += s.Count
		}
		if total != 26*n {
			t.Fatalf("%d fans: %d LEDs", n, total)
		}
	}
}

// A stored "lighting on" without the lighting notice on record starts nothing.
func TestStoredLightingNeedsTheNotice(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"lighting":{"enabled":true},"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if v := e.Lighting(); v.Active {
		t.Fatalf("lighting started without the notice: %+v", v)
	}
	e.Acknowledge("lighting")
	e.resumeLighting()
	if v := e.Lighting(); !v.Active {
		t.Fatal("lighting did not start after the notice")
	}
}
