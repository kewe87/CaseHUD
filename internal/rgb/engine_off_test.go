package leds_test

import (
	"context"
	"errors"
	"image/color"
	"sync"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timeout: " + what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func isBlack(f map[string][]color.RGBA) bool {
	for _, cols := range f {
		for _, c := range cols {
			if c.R|c.G|c.B != 0 {
				return false
			}
		}
	}
	return len(f) > 0
}

// An off device gets one black frame, not a stream; it is sent again on
// request (resume) and at shutdown whatever "on exit" says; switching it on
// returns to the effect.
func TestOffSendsBlackOnce(t *testing.T) {
	off, lit := newDev(), &fake.Device{DeviceID: "fake:2", ZoneList: []leds.Zone{{ID: "z", Count: 4, Max: 4, Order: "RGB"}}}
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Effect, cfg.ShutdownColor = "solid", leds.ShutdownKeep
	cfg.Params = map[string]leds.Params{"solid": {"color": "#ff0000"}}
	cfg.Devices = map[string]leds.DeviceConfig{"fake:1": {Off: true}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{off, lit}}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFrames(t, lit, 10)
	if off.Frames() != 1 || !isBlack(off.Last()) {
		t.Fatalf("off device: %d frames, black %v", off.Frames(), isBlack(off.Last()))
	}
	if !e.Devices()[0].Off {
		t.Fatal("status does not say off")
	}

	e.ReassertOff()
	waitFor(t, "second black frame", func() bool { return off.Frames() == 2 })
	time.Sleep(100 * time.Millisecond)
	if off.Frames() != 2 {
		t.Fatalf("%d frames after reassert", off.Frames())
	}

	// on again: the effect is back
	e.SetDeviceOff("fake:1", false)
	waitFor(t, "effect after off", func() bool { return off.Frames() > 5 && !isBlack(off.Last()) })

	// off again, then exit with "keep": black goes out anyway
	e.SetDeviceOff("fake:1", true)
	time.Sleep(100 * time.Millisecond)
	before := off.Frames()
	time.Sleep(100 * time.Millisecond)
	if off.Frames() != before {
		t.Fatal("frames keep coming while off")
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if off.Frames() != before+1 || !isBlack(off.Last()) {
		t.Fatalf("shutdown: %d -> %d frames, black %v", before, off.Frames(), isBlack(off.Last()))
	}
	if isBlack(lit.Last()) {
		t.Fatal("keep: the lit device must keep its colors")
	}
}

// holder keeps its off state itself.
type holder struct {
	fake.Device
	mu    sync.Mutex
	calls [][2]bool
	err   error
}

func (h *holder) SetOff(off, held bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, [2]bool{off, held})
	return h.err
}

func (h *holder) got() [][2]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([][2]bool(nil), h.calls...)
}

func (h *holder) fail(err error) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
}

type oneHolder struct{ d *holder }

func (oneHolder) Name() string { return "holder" }
func (b oneHolder) Discover(context.Context) ([]leds.Device, error) {
	return []leds.Device{b.d}, nil
}

func TestPersistentOffWritesOncePerChange(t *testing.T) {
	h := &holder{Device: fake.Device{DeviceID: "fake:h", ZoneList: []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}}}
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Effect = "solid"
	cfg.Devices = map[string]leds.DeviceConfig{"fake:h": {Off: true}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{oneHolder{h}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	// start-up while off: the state is taken over (held), nothing else, no frames
	waitFor(t, "takeover", func() bool { return len(h.got()) == 1 })
	time.Sleep(150 * time.Millisecond)
	if c := h.got(); len(c) != 1 || c[0] != [2]bool{true, true} || h.Frames() != 0 {
		t.Fatalf("start: calls %v, frames %d", c, h.Frames())
	}
	// a rescan (resume, reconnect timer) and ReassertOff change nothing
	if err := e.Rescan(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.ReassertOff()
	time.Sleep(150 * time.Millisecond)
	if len(h.got()) != 1 || h.Frames() != 0 {
		t.Fatalf("rescan: calls %v, frames %d", h.got(), h.Frames())
	}
	// on: one call, frames resume
	e.SetDeviceOff("fake:h", false)
	waitFor(t, "frames after on", func() bool { return h.Frames() > 2 })
	if c := h.got(); len(c) != 2 || c[1] != [2]bool{false, false} {
		t.Fatalf("on: calls %v", c)
	}
	// off: one call, then silence; a refused write is retried later, not every tick
	h.fail(errors.New("refused"))
	e.SetDeviceOff("fake:h", true)
	waitFor(t, "off call", func() bool { return len(h.got()) == 3 })
	time.Sleep(200 * time.Millisecond)
	if len(h.got()) != 3 {
		t.Fatalf("retry too fast: %v", h.got())
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentOffSendsNothingAtStop(t *testing.T) {
	h := &holder{Device: fake.Device{DeviceID: "fake:h", ZoneList: []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}}}
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Effect = "solid"
	cfg.Devices = map[string]leds.DeviceConfig{"fake:h": {Off: true}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{oneHolder{h}}, ProcessRunning: noProcs})
	e.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	e.Stop()
	if h.Frames() != 0 || h.Closed() != 1 {
		t.Fatalf("frames %d, closed %d", h.Frames(), h.Closed())
	}
}

// A SetOff that keeps failing is tried three times at most (the first attempt
// and two retries), then the engine leaves the device alone until the setting
// changes.
func TestPersistentOffRetriesAreLimited(t *testing.T) {
	defer leds.SetOffRetryWaitForTest(15 * time.Millisecond)()
	h := &holder{Device: fake.Device{DeviceID: "fake:h", ZoneList: []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}}}
	h.fail(errors.New("write failed"))
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Effect = "solid"
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{oneHolder{h}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	waitFrames(t, &h.Device, 2)
	e.SetDeviceOff("fake:h", true)
	waitFor(t, "three attempts", func() bool { return len(h.got()) >= 3 })
	time.Sleep(300 * time.Millisecond)
	if n := len(h.got()); n != 3 {
		t.Fatalf("%d attempts", n)
	}
	// a change of the setting starts over
	e.SetDeviceOff("fake:h", false)
	time.Sleep(100 * time.Millisecond)
	e.SetDeviceOff("fake:h", true)
	waitFor(t, "fresh attempts after a change", func() bool { return len(h.got()) > 3 })
}
