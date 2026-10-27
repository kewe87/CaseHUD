// Package asusaura drives the USB lighting controller of ASUS mainboards
// (onboard LEDs, 12 V RGB headers, 5 V addressable headers).
//
// Wire format: HID output reports of 65 bytes on interface 2, first byte 0xEC.
//
//	EC 82                       identify; reply EC 02 + 16 bytes model (ASCII)
//	EC B0                       layout; reply EC 30 + layout bytes (see parseLayout)
//	EC 52 53 00 01              newer controllers: let the host drive the LEDs
//	EC 35 ch 00 00 FF           put effect channel ch under host control
//	EC 40 px first count RGB…   pixels for pixel channel px, at most 20 per report;
//	                            px|0x80 on the last report of a frame shows it
package asusaura

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/internal/conflicts"
	"github.com/kewe87/casehud/internal/rgb/internal/hid"
)

const vendorID = 0x0B05

// opcodes (second byte of a report)
const (
	marker       = 0xEC
	opIdentify   = 0x82
	opLayout     = 0xB0
	opHostMode   = 0x52
	opEffectMode = 0x35
	opPixels     = 0x40

	replyIdentify = 0x02
	replyLayout   = 0x30

	hostOwned = 0xFF // effect mode value: colors come from the host
	showFrame = 0x80 // pixel channel flag on the last report of a frame

	// onboardPixels is the pixel channel of the onboard group (fixed index,
	// independent of the number of headers).
	onboardPixels = 4
)

const (
	reportSize      = 65
	pixelsPerReport = 20
	headerMaxLEDs   = 120 // limit a controller reports for an addressable header
	ioWait          = 500 * time.Millisecond
)

// supported product IDs; hostModePIDs need opHostMode first.
var (
	supported    = map[uint16]bool{0x18F3: true, 0x1939: true, 0x19AF: true, 0x1AA6: true, 0x1BED: true}
	hostModePIDs = map[uint16]bool{0x1AA6: true, 0x1BED: true}
)

// layout is what the controller reports about its outputs.
type layout struct {
	raw     []byte
	model   string
	headers int   // addressable 5 V headers (resizable)
	fixed   []int // further pixel channels with a fixed LED count
	onboard int   // onboard LEDs plus one per 12 V header
}

// parseLayout reads the layout reply (offsets count from the report ID):
//
//	6         number of pixel channels
//	10 + 6*i  descriptor of pixel channel i, first byte = its LED limit
//	31        onboard LEDs
//	33        12 V headers + 1
//
// A channel whose limit is the header maximum is an addressable header; a
// smaller limit is a fixed LED group (on the reference board: the I/O cover).
func parseLayout(r []byte) (layout, error) {
	const (
		offChannels = 6
		offDesc     = 10
		descSize    = 6
		offOnboard  = 31
		offRGB12V   = 33
	)
	if len(r) <= offRGB12V || r[1] != replyLayout {
		return layout{}, errors.New("layout reply too short")
	}
	l := layout{raw: append([]byte(nil), r...)}
	for i := 0; i < int(r[offChannels]); i++ {
		off := offDesc + descSize*i
		if off >= len(r) {
			break
		}
		if limit := int(r[off]); limit >= headerMaxLEDs || limit == 0 {
			l.headers++
		} else {
			l.fixed = append(l.fixed, limit)
		}
	}
	l.onboard = int(r[offOnboard]) + max(int(r[offRGB12V])-1, 0)
	return l, nil
}

// Backend discovers the controllers.
type Backend struct{}

// New returns the backend.
func New() *Backend { return &Backend{} }

// Name implements leds.Backend.
func (*Backend) Name() string { return "asusaura" }

// Discover implements leds.Backend. It only asks for model and layout; the
// lighting is not changed before the first frame.
func (*Backend) Discover(ctx context.Context) ([]leds.Device, error) {
	infos, err := hid.Enumerate(vendorID, 0)
	if err != nil {
		if errors.Is(err, hid.ErrUnsupported) {
			return nil, nil
		}
		return nil, err
	}
	var out []leds.Device
	var errs []error
	for _, info := range infos {
		if !supported[info.ProductID] || info.Interface != 2 {
			continue
		}
		h, err := hid.Open(info)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		d := &device{port: h, pid: info.ProductID, inSize: max(info.InputLen, reportSize)}
		if d.lay, err = d.probe(); err != nil {
			h.Close()
			errs = append(errs, fmt.Errorf("asusaura %04x: %w", info.ProductID, err))
			continue
		}
		out = append(out, d)
	}
	return out, errors.Join(errs...)
}

// port is the subset of *hid.Device the driver uses (replaced in tests).
type port interface {
	Write(report []byte, timeout time.Duration) error
	Read(buf []byte, timeout time.Duration) (int, error)
	Close() error
}

type device struct {
	port   port
	pid    uint16
	inSize int
	lay    layout

	mu    sync.Mutex
	owned bool // effect channels handed to the host
}

// ask sends a query and returns the first reply that carries want.
func (d *device) ask(op, want byte) ([]byte, error) {
	buf := make([]byte, d.inSize)
	for i := 0; i < 8; i++ { // skip stale replies
		if _, err := d.port.Read(buf, 5*time.Millisecond); err != nil {
			break
		}
	}
	if err := d.port.Write([]byte{marker, op}, ioWait); err != nil {
		return nil, err
	}
	for i := 0; i < 4; i++ {
		n, err := d.port.Read(buf, ioWait)
		if err != nil {
			return nil, err
		}
		if n > 1 && buf[1] == want {
			return append([]byte(nil), buf[:n]...), nil
		}
	}
	return nil, fmt.Errorf("no reply to %#02x", op)
}

func (d *device) probe() (layout, error) {
	id, err := d.ask(opIdentify, replyIdentify)
	if err != nil {
		return layout{}, fmt.Errorf("identify: %w", err)
	}
	r, err := d.ask(opLayout, replyLayout)
	if err != nil {
		return layout{}, fmt.Errorf("layout: %w", err)
	}
	l, err := parseLayout(r)
	if err != nil {
		return layout{}, err
	}
	if len(id) > 2 {
		l.model = strings.TrimSpace(strings.Trim(string(id[2:min(len(id), 18)]), "\x00"))
	}
	return l, nil
}

// zoneIDs: argb1..n for the headers, then ch<n> for fixed groups (numbered
// after the headers, so the reference board's I/O cover is "ch4").
func (d *device) zoneIDs() (headers, fixed []string) {
	for i := 0; i < d.lay.headers; i++ {
		headers = append(headers, fmt.Sprintf("argb%d", i+1))
	}
	for i := range d.lay.fixed {
		fixed = append(fixed, fmt.Sprintf("ch%d", d.lay.headers+i+1))
	}
	return headers, fixed
}

func (d *device) ID() string { return fmt.Sprintf("asusaura:%04x:%04x", vendorID, d.pid) }

func (d *device) Name() string {
	if d.lay.model != "" {
		return "ASUS Aura " + d.lay.model
	}
	return "ASUS Aura mainboard"
}

func (d *device) Zones() []leds.Zone {
	var zs []leds.Zone
	if d.lay.onboard > 0 {
		zs = append(zs, leds.Zone{ID: "onboard", Name: "Onboard + 12V headers", Count: d.lay.onboard, Max: d.lay.onboard, Order: "RGB"})
	}
	headers, fixed := d.zoneIDs()
	for i, id := range headers {
		zs = append(zs, leds.Zone{ID: id, Name: fmt.Sprintf("ARGB header %d", i+1), Max: headerMaxLEDs, Resizable: true, Order: "RGB"})
	}
	for i, id := range fixed {
		n := d.lay.fixed[i]
		zs = append(zs, leds.Zone{ID: id, Name: fmt.Sprintf("Fixed LED group %d", i+1), Count: n, Max: n, Order: "RGB"})
	}
	return zs
}

// Firmware is the model string of the identify reply (it carries the firmware
// revision, e.g. "AULA3-AR32-0207").
func (d *device) Firmware() string { return d.lay.model }

// Describe returns diagnostic details (model and raw layout reply).
func (d *device) Describe() string {
	return fmt.Sprintf("model %q, layout % x", d.lay.model, d.lay.raw)
}

func (d *device) Conflicts() []string { return conflicts.Mainboard }

// takeOver hands every effect channel to the host: channel 0 is the onboard
// group, 1.. the pixel channels.
func (d *device) takeOver() error {
	var reports [][]byte
	if hostModePIDs[d.pid] {
		reports = append(reports, []byte{marker, opHostMode, 0x53, 0x00, 0x01})
	}
	for ch := 0; ch <= d.lay.headers+len(d.lay.fixed); ch++ {
		reports = append(reports, []byte{marker, opEffectMode, byte(ch), 0x00, 0x00, hostOwned})
	}
	return d.send(reports)
}

// pixelReports splits the colors of one pixel channel into reports; the last
// one carries the show flag.
func pixelReports(channel byte, cols []color.RGBA) [][]byte {
	cols = cols[:min(len(cols), headerMaxLEDs)]
	var out [][]byte
	for first := 0; first < len(cols); first += pixelsPerReport {
		part := cols[first:min(first+pixelsPerReport, len(cols))]
		flags := channel
		if first+len(part) == len(cols) {
			flags |= showFrame
		}
		r := []byte{marker, opPixels, flags, byte(first), byte(len(part))}
		for _, c := range part {
			r = append(r, c.R, c.G, c.B)
		}
		out = append(out, r)
	}
	return out
}

func (d *device) send(reports [][]byte) error {
	for _, r := range reports {
		if err := d.port.Write(r, ioWait); err != nil {
			return err
		}
	}
	return nil
}

// SetFrame implements leds.Device. Pixel channels are numbered headers first,
// then fixed groups; the onboard group has its own channel.
func (d *device) SetFrame(frame map[string][]color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.owned {
		if err := d.takeOver(); err != nil {
			return err
		}
		d.owned = true
	}
	headers, fixed := d.zoneIDs()
	var reports [][]byte
	for i, id := range append(headers, fixed...) {
		reports = append(reports, pixelReports(byte(i), frame[id])...)
	}
	reports = append(reports, pixelReports(onboardPixels, frame["onboard"])...)
	return d.send(reports)
}

// RestoreDeviceEffect implements leds.DeviceEffectRestorer. Controllers that
// were put into host mode leave it again and show their stored effect; for
// the others no way back is known (leds.ErrNoDeviceEffect: switched off).
func (d *device) RestoreDeviceEffect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !hostModePIDs[d.pid] {
		return leds.ErrNoDeviceEffect
	}
	if !d.owned {
		return nil
	}
	if err := d.send([][]byte{{marker, opHostMode, 0x53, 0x00, 0x00}}); err != nil {
		return err
	}
	d.owned = false
	return nil
}

func (d *device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.port.Close()
}
