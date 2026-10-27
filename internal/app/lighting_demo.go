package app

import (
	"context"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// demoController is the demo fan controller with a lighting mode, like the
// real one: it follows the mainboard, shows CaseHUD effects, or is left alone.
type demoController struct {
	*fake.Device
	mu     sync.Mutex
	mode   string
	report func(id string, err error)
}

func (d *demoController) Settings() []leds.Param {
	return []leds.Param{
		{Key: "mode", Label: "Lighting", Type: leds.ParamEnum,
			Values: []string{"passive", "motherboard", "hardware", "canvas"}, Default: "motherboard"},
		{Key: "allowFlashWrites", Label: "Allow controller lighting", Type: leds.ParamBool, Default: false},
		{Key: "syncSource", Label: "Sync header", Type: leds.ParamText, Default: ""},
	}
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
	d.mu.Unlock()
	return nil
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
