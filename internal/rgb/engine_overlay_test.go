package leds_test

import (
	"context"
	"sync"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

// configurable records the settings it is configured with.
type configurable struct {
	fake.Device
	mu   sync.Mutex
	last leds.Params
	n    int
}

func (c *configurable) Settings() []leds.Param {
	return []leds.Param{{Key: "a", Type: leds.ParamText, Default: "da"}, {Key: "b", Type: leds.ParamText, Default: "db"}}
}

func (c *configurable) Configure(p leds.Params) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last, c.n = p, c.n+1
	return nil
}

func (c *configurable) seen() (leds.Params, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last, c.n
}

// An overlay reaches Configure on top of the stored settings, is never part of
// the configuration and configures the device again only when it changes.
func TestDeviceOverlay(t *testing.T) {
	dev := &configurable{Device: fake.Device{DeviceID: "fake:o", ZoneList: []leds.Zone{{ID: "z", Count: 2, Max: 2, Order: "RGB"}}}}
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Devices = map[string]leds.DeviceConfig{"fake:o": {Settings: leds.Params{"a": "stored", "b": "stored"}}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{oneBackend{dev}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	waitFor(t, "configured", func() bool { _, n := dev.seen(); return n > 0 })

	e.SetDeviceOverlay("fake:o", leds.Params{"b": "derived"})
	waitFor(t, "overlay applied", func() bool { p, _ := dev.seen(); return p.String("b") == "derived" })
	if p, _ := dev.seen(); p.String("a") != "stored" {
		t.Fatalf("stored setting lost: %v", p)
	}
	if got := e.Config().Devices["fake:o"].Settings.String("b"); got != "stored" {
		t.Fatalf("overlay leaked into the configuration: %q", got)
	}
	if ov := e.DeviceOverlay("fake:o"); ov.String("b") != "derived" {
		t.Fatalf("overlay %v", ov)
	}

	_, n := dev.seen()
	e.SetDeviceOverlay("fake:o", leds.Params{"b": "derived"}) // unchanged: no new Configure
	e.SetEffect("solid", nil)
	waitFrames(t, &dev.Device, dev.Frames()+5)
	if _, m := dev.seen(); m != n {
		t.Fatalf("unchanged overlay configured again (%d -> %d)", n, m)
	}

	e.SetDeviceOverlay("fake:o", nil)
	waitFor(t, "overlay removed", func() bool { p, _ := dev.seen(); return p.String("b") == "stored" })
}
