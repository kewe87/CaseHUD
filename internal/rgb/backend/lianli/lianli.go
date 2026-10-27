// Package lianli drives the LED ring of the Lian Li Universal Screen 8.8"
// (USB 0416:8050, WinUSB).
//
// Protocol: per frame one 64-byte bulk OUT packet per 20 LEDs.
//
//	0       0x11 (set LEDs)
//	1       index of the first LED in this packet (0, 20, 40)
//	2..3    0
//	4..63   20 × R G B
//
// There is no init, apply or save command; the ring shows what it receives.
// The display itself (1CBE:A088) is a separate device and is not touched.
package lianli

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"sync"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/internal/conflicts"
	"github.com/kewe87/casehud/internal/rgb/internal/winusb"
)

const (
	vendorID      = 0x0416
	productRing88 = 0x8050

	cmdSetLEDs    = 0x11
	packetLen     = 64
	ledsPerPacket = 20
	ring88LEDs    = 60  // measured on the reference ring; resizable for other variants
	maxLEDs       = 240 // the offset byte addresses at most 255 LEDs
	ioTimeout     = 500 * time.Millisecond
)

// interfaceGUID is the DeviceInterfaceGUID the ring registers under winusb.inf.
var interfaceGUID = winusb.GUID{Data1: 0x1D4B2365, Data2: 0x4749, Data3: 0x48EA, Data4: [8]byte{0xB3, 0x8A, 0x7C, 0x6F, 0xDD, 0xDD, 0x7E, 0x26}}

// Backend discovers Lian Li WinUSB LED devices.
type Backend struct{}

// New returns the backend.
func New() *Backend { return &Backend{} }

// Name implements leds.Backend.
func (*Backend) Name() string { return "lianli" }

// Discover implements leds.Backend. It only enumerates: WinUSB access is
// exclusive, so the device is opened on the first frame, not before.
func (*Backend) Discover(ctx context.Context) ([]leds.Device, error) {
	paths, err := winusb.Paths(interfaceGUID, vendorID, productRing88)
	if err != nil {
		if errors.Is(err, winusb.ErrUnsupported) {
			return nil, nil
		}
		return nil, err
	}
	var out []leds.Device
	for _, p := range paths {
		path := p
		out = append(out, &device{open: func() (transport, error) { return winusb.Open(path, ioTimeout) }, leds: ring88LEDs})
	}
	return out, nil
}

// transport is the subset of *winusb.Device the driver uses (replaced in tests).
type transport interface {
	Write(buf []byte) error
	Close() error
}

type device struct {
	open func() (transport, error)
	leds int

	mu sync.Mutex
	h  transport
}

func (d *device) ID() string   { return fmt.Sprintf("lianli:%04x:%04x", vendorID, productRing88) }
func (d *device) Name() string { return `Lian Li Universal Screen 8.8" LED ring` }

func (d *device) Zones() []leds.Zone {
	return []leds.Zone{{ID: "ring", Name: "LED ring", Count: d.leds, Max: maxLEDs, Resizable: true, Order: "RGB"}}
}

func (d *device) Conflicts() []string {
	return conflicts.Ring
}

func (d *device) SetFrame(frame map[string][]color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.h == nil {
		h, err := d.open()
		if err != nil {
			return fmt.Errorf("lianli: %w (another program may hold the device)", err)
		}
		d.h = h
	}
	cols := frame["ring"]
	n := min(max(len(cols), d.leds), maxLEDs)
	for start := 0; start < n; start += ledsPerPacket {
		pkt := make([]byte, packetLen)
		pkt[0], pkt[1] = cmdSetLEDs, byte(start)
		for i := 0; i < ledsPerPacket && start+i < n; i++ {
			if start+i < len(cols) {
				c := cols[start+i]
				pkt[4+3*i], pkt[5+3*i], pkt[6+3*i] = c.R, c.G, c.B
			}
		}
		if err := d.h.Write(pkt); err != nil {
			// unplugged or taken over: reopen on the next frame
			d.h.Close()
			d.h = nil
			return err
		}
	}
	return nil
}

func (d *device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.h != nil {
		d.h.Close()
		d.h = nil
	}
	return nil
}
