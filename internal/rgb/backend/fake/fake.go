// Package fake provides an in-memory backend for tests and UI development.
package fake

import (
	"context"
	"image/color"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Backend returns its devices from every Discover call.
type Backend struct {
	mu      sync.Mutex
	Devices []*Device
}

// Name implements leds.Backend.
func (b *Backend) Name() string { return "fake" }

// Discover implements leds.Backend.
func (b *Backend) Discover(ctx context.Context) ([]leds.Device, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]leds.Device, len(b.Devices))
	for i, d := range b.Devices {
		out[i] = d
	}
	return out, nil
}

// Device records the frames it receives.
type Device struct {
	DeviceID   string
	DeviceName string
	ZoneList   []leds.Zone
	Conflict   []string

	mu     sync.Mutex
	err    error
	frames int
	last   map[string][]color.RGBA
	closed int
}

func (d *Device) ID() string          { return d.DeviceID }
func (d *Device) Name() string        { return d.DeviceName }
func (d *Device) Zones() []leds.Zone  { return d.ZoneList }
func (d *Device) Conflicts() []string { return d.Conflict }

func (d *Device) SetFrame(frame map[string][]color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return d.err
	}
	d.frames++
	d.last = make(map[string][]color.RGBA, len(frame))
	for k, v := range frame {
		d.last[k] = append([]color.RGBA(nil), v...)
	}
	return nil
}

func (d *Device) Close() error {
	d.mu.Lock()
	d.closed++
	d.mu.Unlock()
	return nil
}

// FailWith makes SetFrame return err (nil to recover).
func (d *Device) FailWith(err error) {
	d.mu.Lock()
	d.err = err
	d.mu.Unlock()
}

// Frames returns the number of frames received.
func (d *Device) Frames() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.frames
}

// Last returns a copy of the last frame.
func (d *Device) Last() map[string][]color.RGBA {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string][]color.RGBA, len(d.last))
	for k, v := range d.last {
		out[k] = append([]color.RGBA(nil), v...)
	}
	return out
}

// Closed returns how often Close was called.
func (d *Device) Closed() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}
