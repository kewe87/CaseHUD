package leds_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

func newDev() *fake.Device {
	return &fake.Device{
		DeviceID: "fake:1", DeviceName: "Fake",
		ZoneList: []leds.Zone{
			{ID: "strip", Count: 10, Max: 10, Order: "RGB"},
			{ID: "grb", Count: 2, Max: 2, Order: "GRB"},
		},
		Conflict: []string{"Other.exe"},
	}
}

func waitFrames(t *testing.T, d *fake.Device, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for d.Frames() < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d frames", d.Frames())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func noProcs([]string) []string { return nil }

func TestEngineSolidBrightnessOrder(t *testing.T) {
	dev := newDev()
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Brightness = 50
	cfg.Effect = "solid"
	cfg.Params = map[string]leds.Params{"solid": {"color": "#c86432"}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})

	var saved []leds.Config
	e.OnConfigChange(func(c leds.Config) { saved = append(saved, c) })
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFrames(t, dev, 2)
	last := dev.Last()
	want := color.RGBA{100, 50, 25, 255} // #c86432 at 50 %
	if len(last["strip"]) != 10 || last["strip"][3] != want {
		t.Fatalf("strip: %v", last["strip"])
	}
	if last["grb"][0] != (color.RGBA{50, 100, 25, 255}) {
		t.Fatalf("grb order: %v", last["grb"][0])
	}

	e.SetBrightness(100)
	if len(saved) != 1 || saved[0].Brightness != 100 {
		t.Fatalf("config callback: %+v", saved)
	}

	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if dev.Closed() != 1 {
		t.Fatalf("closed %d times", dev.Closed())
	}
	if c := dev.Last()["strip"][0]; c != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("shutdown frame: %v", c)
	}
}

func TestEnginePauseConflictError(t *testing.T) {
	dev := newDev()
	var running atomic.Bool
	procs := func([]string) []string {
		if running.Load() {
			return []string{"Other.exe"}
		}
		return nil
	}
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: procs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	waitFrames(t, dev, 1)

	e.SetPaused(true)
	time.Sleep(50 * time.Millisecond)
	n := dev.Frames()
	if c := dev.Last()["strip"][0]; c != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("pause did not switch off: %v", c)
	}
	time.Sleep(50 * time.Millisecond)
	if dev.Frames() != n {
		t.Fatalf("frames sent while paused: %d -> %d", n, dev.Frames())
	}
	e.SetPaused(false)
	waitFrames(t, dev, n+1)

	// conflicts are detected on rescan
	running.Store(true)
	if err := e.Rescan(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := e.Devices()
	if len(st) != 1 || st[0].State != leds.StateConflict {
		t.Fatalf("status: %+v", st)
	}
	running.Store(false)

	// write errors put the device into error state
	if err := e.Rescan(context.Background()); err != nil {
		t.Fatal(err)
	}
	dev.FailWith(errors.New("unplugged"))
	deadline := time.Now().Add(2 * time.Second)
	for e.Devices()[0].State != leds.StateError {
		if time.Now().After(deadline) {
			t.Fatalf("no error state: %+v", e.Devices())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSetEffectUnknown(t *testing.T) {
	e := leds.New(leds.DefaultConfig(), leds.Options{ProcessRunning: noProcs})
	if err := e.SetEffect("nope", nil); err == nil {
		t.Fatal("expected error")
	}
	if err := e.SetEffect("wave", leds.Params{"speed": 10.0}); err != nil {
		t.Fatal(err)
	}
	if c := e.Config(); c.Effect != "wave" || c.Params["wave"]["speed"] != 10.0 || c.Params["wave"]["width"] != 30.0 {
		t.Fatalf("config: %+v", c)
	}
}

type closingEffect struct{ closed *atomic.Int32 }

func (closingEffect) Render(leds.Frame, *image.RGBA) {}
func (c closingEffect) Close() error                 { c.closed.Add(1); return nil }

func TestEngineClosesEffects(t *testing.T) {
	var closed atomic.Int32
	mk := func(name string) leds.EffectInfo {
		return leds.EffectInfo{Name: name, New: func(leds.Params) (leds.Effect, error) { return closingEffect{&closed}, nil }}
	}
	cfg := leds.DefaultConfig()
	cfg.Effect = "a"
	e := leds.New(cfg, leds.Options{Effects: []leds.EffectInfo{mk("a"), mk("b")}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	e.SetEffect("b", nil)
	time.Sleep(100 * time.Millisecond)
	// SetEffect's validation instance and the replaced effect
	if closed.Load() != 2 {
		t.Fatalf("closed %d times after SetEffect", closed.Load())
	}
	e.Stop()
	if closed.Load() != 3 {
		t.Fatalf("closed %d times after Stop", closed.Load())
	}
}

func TestCalibrate(t *testing.T) {
	dev := &fake.Device{DeviceID: "fake:c", ZoneList: []leds.Zone{
		{ID: "strip", Count: 10, Max: 50, Resizable: true, Order: "GRB"},
		{ID: "other", Count: 3, Max: 3, Order: "RGB"},
	}}
	cfg := leds.DefaultConfig()
	cfg.Effect = "solid"
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if err := e.Calibrate("fake:c", "strip", 50); err == nil {
		t.Fatal("position past the hardware limit accepted")
	}
	if err := e.Calibrate("fake:c", "strip", 30); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	f := dev.Last()
	s := f["strip"]
	if len(s) != 50 {
		t.Fatalf("calibration frame has %d LEDs, want the hardware limit 50", len(s))
	}
	// GRB wire order: red head = (0,255,0), blue tail stays blue
	if s[30] != (color.RGBA{0, 255, 0, 255}) || s[29].B == 0 || s[31] != (color.RGBA{A: 255}) {
		t.Fatalf("marker: %v %v %v", s[29], s[30], s[31])
	}
	if f["other"][0] != (color.RGBA{A: 255}) {
		t.Fatalf("other zone not dark: %v", f["other"][0])
	}
	e.EndCalibration()
	time.Sleep(150 * time.Millisecond)
	if len(dev.Last()["strip"]) != 10 {
		t.Fatal("effect not restored")
	}
}

func TestShutdownKeep(t *testing.T) {
	dev := newDev()
	cfg := leds.DefaultConfig()
	cfg.Effect, cfg.ShutdownColor = "solid", "keep"
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})
	e.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	e.Stop()
	if c := dev.Last()["strip"][0]; c == (color.RGBA{A: 255}) || dev.Closed() != 1 {
		t.Fatalf("last frame %v (want the effect color, not off), closed %d", c, dev.Closed())
	}
}

// restorer can return to its own effect (or not, with ErrNoDeviceEffect).
type restorer struct {
	fake.Device
	err      error
	restored atomic.Int32
}

func (r *restorer) RestoreDeviceEffect() error {
	r.restored.Add(1)
	return r.err
}

func TestShutdownDevice(t *testing.T) {
	zones := []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}
	back := &restorer{Device: fake.Device{DeviceID: "fake:back", ZoneList: zones}}
	none := &restorer{Device: fake.Device{DeviceID: "fake:none", ZoneList: zones}, err: leds.ErrNoDeviceEffect}
	plain := newDev()
	cfg := leds.DefaultConfig()
	cfg.Effect, cfg.ShutdownColor = "solid", leds.ShutdownDevice
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{plain}},
		oneBackend{back}, oneBackend{none}}, ProcessRunning: noProcs})
	e.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	e.Stop()
	black := color.RGBA{A: 255}
	if back.restored.Load() != 1 || back.Last()["z"][0] == black {
		t.Fatalf("restorer: %d calls, last %v (no black frame wanted)", back.restored.Load(), back.Last()["z"][0])
	}
	if none.restored.Load() != 1 || none.Last()["z"][0] != black {
		t.Fatalf("no device effect: last %v, want off", none.Last()["z"][0])
	}
	if plain.Last()["strip"][0] != black {
		t.Fatal("device without restore not switched off")
	}
}

// SetShutdown on a running engine is reported and used by Stop.
func TestSetShutdown(t *testing.T) {
	zones := []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}
	back := &restorer{Device: fake.Device{DeviceID: "fake:back", ZoneList: zones}}
	cfg := leds.DefaultConfig()
	cfg.Effect = "solid"
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{oneBackend{back}}, ProcessRunning: noProcs})
	var saved string
	e.OnConfigChange(func(c leds.Config) { saved = c.ShutdownColor })
	e.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	e.SetShutdown(leds.ShutdownDevice)
	if saved != leds.ShutdownDevice {
		t.Fatalf("OnConfigChange got %q", saved)
	}
	e.Stop()
	if back.restored.Load() != 1 {
		t.Fatalf("restorer: %d calls, want 1", back.restored.Load())
	}
}

// slowDevice takes 200 ms per frame and counts frames.
type slowDevice struct {
	fake.Device
	n atomic.Int32
}

func (s *slowDevice) SetFrame(f map[string][]color.RGBA) error {
	time.Sleep(200 * time.Millisecond)
	s.n.Add(1)
	return nil
}

func TestSlowDeviceDoesNotBlockOthers(t *testing.T) {
	fast := newDev()
	slow := &slowDevice{Device: fake.Device{DeviceID: "fake:slow", ZoneList: []leds.Zone{{ID: "z", Count: 4, Max: 4, Order: "RGB"}}}}
	cfg := leds.DefaultConfig()
	cfg.FPS = 50
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{
		&fake.Backend{Devices: []*fake.Device{fast}},
		slowBackend{slow},
	}, ProcessRunning: noProcs})
	e.Start(context.Background())
	time.Sleep(500 * time.Millisecond)
	e.Stop()
	if n := fast.Frames(); n < 15 {
		t.Fatalf("fast device got only %d frames while the slow one was busy", n)
	}
	// about 2-3 frames in 500 ms plus the shutdown frame, far fewer than the 25 ticks
	if n := slow.n.Load(); n < 2 || n > 6 {
		t.Fatalf("slow device got %d frames (want only the newest ones)", n)
	}
}

type slowBackend struct{ d *slowDevice }

func (slowBackend) Name() string { return "slow" }
func (b slowBackend) Discover(context.Context) ([]leds.Device, error) {
	return []leds.Device{b.d}, nil
}

// hangDevice never finishes a frame and never closes.
type hangDevice struct{ fake.Device }

func (h *hangDevice) SetFrame(map[string][]color.RGBA) error { select {} }
func (h *hangDevice) Close() error                           { select {} }

type oneBackend struct{ d leds.Device }

func (oneBackend) Name() string { return "one" }
func (b oneBackend) Discover(context.Context) ([]leds.Device, error) {
	return []leds.Device{b.d}, nil
}

func TestStopBoundedWithHangingDevice(t *testing.T) {
	h := &hangDevice{Device: fake.Device{DeviceID: "fake:hang", ZoneList: []leds.Zone{{ID: "z", Count: 1, Max: 1, Order: "RGB"}}}}
	e := leds.New(leds.DefaultConfig(), leds.Options{Backends: []leds.Backend{oneBackend{h}}, ProcessRunning: noProcs})
	e.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	err := e.Stop()
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("Stop took %v", d)
	}
	if err == nil {
		t.Fatal("hang not reported")
	}
}

// flakyBackend finds its device once, then fails.
type flakyBackend struct {
	d     leds.Device
	calls int
}

func (*flakyBackend) Name() string { return "flaky" }
func (b *flakyBackend) Discover(context.Context) ([]leds.Device, error) {
	b.calls++
	if b.calls == 1 {
		return []leds.Device{b.d}, nil
	}
	return nil, errors.New("enumeration failed")
}

func TestDiscoverFailureKeepsDevices(t *testing.T) {
	dev := newDev()
	b := &flakyBackend{d: dev}
	e := leds.New(leds.DefaultConfig(), leds.Options{Backends: []leds.Backend{b}, ProcessRunning: noProcs})
	e.Start(context.Background())
	defer e.Stop()
	e.Rescan(context.Background())
	if n := len(e.Devices()); n != 1 || dev.Closed() != 0 {
		t.Fatalf("devices %d, closed %d after failed discovery", n, dev.Closed())
	}
}

func TestRescanAfterStopReleasesDevices(t *testing.T) {
	first, second := newDev(), newDev()
	calls := 0
	b := backendFunc(func() []leds.Device {
		calls++
		if calls == 1 {
			return []leds.Device{first}
		}
		return []leds.Device{second}
	})
	e := leds.New(leds.DefaultConfig(), leds.Options{Backends: []leds.Backend{b}, ProcessRunning: noProcs})
	e.Start(context.Background())
	done := make(chan struct{})
	go func() { e.Rescan(context.Background()); close(done) }()
	e.Stop()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Rescan blocked after Stop")
	}
	time.Sleep(50 * time.Millisecond)
	if second.Closed() == 0 && len(e.Devices()) == 0 && calls > 1 {
		t.Fatal("device of a rescan finishing after Stop left open")
	}
}

type backendFunc func() []leds.Device

func (backendFunc) Name() string                                      { return "func" }
func (f backendFunc) Discover(context.Context) ([]leds.Device, error) { return f(), nil }
