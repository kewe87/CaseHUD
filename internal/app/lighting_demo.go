package app

import (
	"context"
	"fmt"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// demoController is the demo fan controller with a lighting mode, like the
// real one: it follows the mainboard, shows CaseHUD effects, runs its own
// effect (per port too), or is left alone. It declares the settings of the
// real controller for its ports and only remembers what it was given.
type demoController struct {
	*fake.Device
	mu     sync.Mutex
	mode   string
	last   leds.Params // the settings of the last Configure (tests)
	report func(id string, err error)
}

func (d *demoController) Settings() []leds.Param {
	var ports []int
	for _, z := range d.Device.Zones() {
		var n int
		if _, err := fmt.Sscanf(z.ID, "port%d", &n); err == nil && n > 0 {
			ports = append(ports, n-1)
		}
	}
	return lianlitl.SettingsFor(ports)
}

func (d *demoController) Configure(p leds.Params) error {
	var err error
	if m := p.String("mode"); (m == "hardware" || m == "canvas") && !p.Bool("allowFlashWrites") {
		err = lianlitl.ErrFlashWrites
	}
	if d.report != nil {
		d.report(d.Device.ID(), err)
	}
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.mode = p.String("mode")
	d.last = p
	d.mu.Unlock()
	return nil
}

// configured returns the settings of the last Configure.
func (d *demoController) configured() leds.Params {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.last
}

// demoBackend serves the demo devices, with the fan controller wrapped.
type demoBackend struct {
	inner *fake.Backend
	name  string
	ctl   *demoController
}

func (b *demoBackend) Name() string { return b.name }

func (b *demoBackend) Discover(ctx context.Context) ([]leds.Device, error) {
	devs, err := b.inner.Discover(ctx)
	for i, d := range devs {
		if fd, ok := d.(*fake.Device); ok && b.ctl != nil && fd == b.ctl.Device {
			devs[i] = b.ctl
		}
	}
	return devs, err
}
