// Package lianlitl drives the lighting of Lian Li UNI FAN TL / TL LCD fans
// through the TL controller (USB 0416:7372, HID interface 2, usage page
// 0xFF1B).
//
// Reports: output/input report ID 0x01, 64 bytes:
//
//	0       0x01
//	1       command
//	2..4    0
//	5       payload length
//	6..     payload
//
// Commands used (nothing else is ever sent):
//
//	A1      status query without payload. The answer lists the fans as
//	        3-byte records: an address byte (0x80 present, 0x40 firmware update
//	        running, port in bits 4-5, chain position in bits 0-3), then the
//	        speed in rpm, high byte first.
//	AD      fan grouping [group, count, members...]; member = top<<7 |
//	        bottom<<6 | port<<4 | fan. The default grouping is one group per
//	        port, group number (port*4 + 0)*2, every fan with both sides (0xC0).
//	        Fans that are in no group stay dark.
//	B0      group light, 20 bytes: 0, group, mode, brightness, speed,
//	        4 colors R G B, direction, disable, color count.
//	B3      test light [type, number, bottom]: type 1 fan (port<<4|fan),
//	        3 port; B4 blink port [port]. Only on explicit request
//	        (TestLight, BlinkPort), and then in every mode, passive included.
//	A3      per-fan light, same layout with [0] = port<<4 | MB-sync bit and
//	        [1] = port<<4 | fan. Used here only for MB sync (sync bit set,
//	        everything else 0).
//
// Modes (setting "mode"):
//
//	motherboard  default: one A3 with the sync bit per port and start. A
//	             hardware switch per port then connects the controller's ARGB input to all
//	             ports in parallel: every port shows the same chain, fan 1,
//	             fan 2, ... with 26 LEDs each. This is the per-LED path (lay
//	             the fans out on that ARGB header's zone, see "syncSource")
//	             and it writes the flash only once.
//	passive      the engine sends only the A1 status query (explicit identify
//	             requests, B3/B4, still write).
//	hardware     firmware effect per port: AD (default grouping) + B0, once per
//	             change.
//	canvas       one canvas color per ring (two zone LEDs per fan: top ring,
//	             bottom ring; one group per ring, 8 per port; ports with more
//	             than 4 fans get one color) as static B0, at most once per
//	             interval (default 60 s, minimum 30 s) and only on visible
//	             change.
//
// hardware and canvas store every change in the controller's flash; they are
// refused (ErrFlashWrites) unless the setting allowFlashWrites is set.
//
// Per-fan A3 lighting is not used: on the reference PC it only reached the
// first fan of a TL LCD chain and left the others dark until a cold boot.
// Ports are lit through groups.
//
// The controller has no per-LED streaming (the finest unit is a ring of 13
// LEDs through its own group). It stores every A3/B0 in its data flash, and
// a fan control program may talk to the same device at the same time, so
// writes are kept rare. Fan speed commands (AA, B1) are never sent.
package lianlitl

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strings"
	"sync"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/internal/conflicts"
	"github.com/kewe87/casehud/internal/rgb/internal/hid"
)

const (
	vendorID  = 0x0416
	productTL = 0x7372
	usagePage = 0xFF1B
	usage     = 0x0091
	iface     = 2

	reportID     = 0x01
	reportLen    = 64
	offCmd       = 1
	offLen       = 5
	offPayload   = 6
	cmdHandshake = 0xA1
	cmdFanLight  = 0xA3
	cmdFanGroup  = 0xAD
	cmdGroupLite = 0xB0
	cmdTestLight = 0xB3
	cmdBlinkPort = 0xB4
	mergedGroup  = 0
	lightLen     = 20
	modeStatic   = 3
	maxColors    = 4
	maxLevel     = 4

	flagDetected  = 0x80
	flagUpgrading = 0x40

	ioTimeout      = 300 * time.Millisecond
	answerTimeout  = 150 * time.Millisecond
	ackTimeout     = 100 * time.Millisecond
	drainTimeout   = 2 * time.Millisecond
	staleReads     = 8
	sendAttempts   = 3
	writesPerFrame = 2 // spread fan updates over frames, keep the engine tick short
	// Every A3/B0 is stored in the controller's data flash (two 4 KiB pages,
	// one erase per roughly 37 light commands), so canvas updates are rare.
	defaultInterval = 60 * time.Second
	minInterval     = 30 * time.Second
	maxInterval     = time.Hour
	defaultMinDelta = 12
)

// modes are the firmware effects (wire byte = index + 1).
var modes = []string{
	"rainbow", "rainbow-morph", "static", "breathing", "runway", "meteor", "color-cycle",
	"staggered", "tide", "mixing", "voice", "door", "render", "ripple", "reflect",
	"tail-chasing", "paint", "ping-pong", "stack", "cover-cycle", "wave", "racing",
	"lottery", "intertwine", "meteor-shower", "collide", "electric-current", "kaleidoscope",
}

var directions = []string{"clockwise", "counter-clockwise", "up", "down", "spread", "gather"}

// ErrFlashWrites refuses the hardware and canvas modes until the user allowed
// them (setting allowFlashWrites): the controller stores every light command
// in its flash.
var ErrFlashWrites = errors.New("lianlitl: hardware and canvas lighting write the controller's flash on every change; enable allowFlashWrites to use them")

// Backend discovers TL controllers.
type Backend struct{}

// New returns the backend.
func New() *Backend { return &Backend{} }

// Name implements leds.Backend.
func (*Backend) Name() string { return "lianlitl" }

// Discover implements leds.Backend. It sends one read-only A1 status query to
// learn which fans are connected; the lighting is not touched.
func (*Backend) Discover(ctx context.Context) ([]leds.Device, error) {
	infos, err := hid.Enumerate(vendorID, productTL)
	if err != nil {
		if errors.Is(err, hid.ErrUnsupported) {
			return nil, nil
		}
		return nil, err
	}
	var out []leds.Device
	var errs []error
	for _, info := range infos {
		if info.Interface != iface || info.UsagePage != usagePage || info.Usage != usage {
			continue
		}
		h, err := hid.Open(info)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		d, err := newDevice(h, max(info.InputLen, reportLen))
		if err != nil {
			h.Close()
			errs = append(errs, err)
			continue
		}
		out = append(out, d)
	}
	return out, errors.Join(errs...)
}

// transport is the subset of *hid.Device the driver uses (replaced in tests).
type transport interface {
	Write(report []byte, timeout time.Duration) error
	Read(buf []byte, timeout time.Duration) (int, error)
	Close() error
}

type fan struct{ port, index int }

type settings struct {
	hardware    bool
	passive     bool
	motherboard bool
	mode        byte
	brightness  byte
	speed       byte
	direction   byte
	colors      []color.RGBA
	interval    time.Duration // canvas: minimum time between two writes of a port
	minDelta    int
	merge       bool  // one group over all fans: the effect runs across every port
	portOrder   []int // port order of the merged group (0-based)
}

type device struct {
	h     transport
	inLen int
	fans  []fan  // sorted by port, index
	ports []int  // ports with fans, ascending
	raw   []byte // last A1 answer (diagnostics)
	fw    string // controller firmware version, "" if unknown
	now   func() time.Time

	mu          sync.Mutex
	set         settings
	dirty       bool         // the mode's state must be (re)sent
	pending     []int        // ports (hardware, canvas) or fans (motherboard) waiting for a write
	grouped     map[int]bool // default grouping written (hardware mode)
	rings       []ring
	ringGrouped map[int]bool       // canvas grouping written
	sent        map[int]color.RGBA // last color per port (canvas mode)
	unlimited   bool               // flush everything pending in this frame
	synced      bool               // output may follow the ARGB input (power-up default)
	sentAt      map[int]time.Time
}

func newDevice(h transport, inLen int) (*device, error) {
	d := &device{h: h, inLen: inLen, now: time.Now, synced: true}
	fans, err := d.handshake()
	if err != nil {
		return nil, err
	}
	if len(fans) == 0 {
		return nil, errors.New("lianlitl: no fans detected")
	}
	d.fans = fans
	if info, err := d.productInfo(0, 0); err == nil {
		d.fw = firmwareVersion(info[5])
	}
	for _, f := range fans {
		if len(d.ports) == 0 || d.ports[len(d.ports)-1] != f.port {
			d.ports = append(d.ports, f.port)
		}
	}
	d.grouped = map[int]bool{}
	d.rings = buildRings(fans)
	d.ringGrouped = map[int]bool{}
	d.sent = map[int]color.RGBA{}
	d.sentAt = map[int]time.Time{}
	d.set = defaultSettings()
	d.dirty = true
	return d, nil
}

func packet(cmd byte, payload []byte) []byte {
	p := make([]byte, reportLen)
	p[0], p[offCmd], p[offLen] = reportID, cmd, byte(len(payload))
	copy(p[offPayload:], payload)
	return p
}

// handshake sends A1 and parses the fan list. Answers to other commands (also
// those of other programs on the same device) are skipped.
func (d *device) handshake() ([]fan, error) {
	buf := make([]byte, d.inLen)
	for i := 0; i < 16; i++ { // drain stale reports
		if _, err := d.h.Read(buf, 5*time.Millisecond); err != nil {
			break
		}
	}
	if err := d.h.Write(packet(cmdHandshake, nil), ioTimeout); err != nil {
		return nil, fmt.Errorf("lianlitl: handshake: %w", err)
	}
	for tries := 0; tries < 8; tries++ {
		n, err := d.h.Read(buf, answerTimeout)
		if err != nil {
			return nil, fmt.Errorf("lianlitl: handshake answer: %w", err)
		}
		if n <= offPayload || buf[0] != reportID || buf[offCmd] != cmdHandshake {
			continue
		}
		d.raw = append([]byte(nil), buf[:n]...)
		return parseHandshake(buf[:n])
	}
	return nil, errors.New("lianlitl: no handshake answer")
}

func parseHandshake(r []byte) ([]fan, error) {
	n := int(r[offLen]) / 3
	var fans []fan
	for i := 0; i < n && offPayload+3*i < len(r); i++ {
		b := r[offPayload+3*i]
		if b&flagUpgrading != 0 {
			return nil, errors.New("lianlitl: controller firmware update in progress, not touching it")
		}
		if b&flagDetected != 0 {
			fans = append(fans, fan{port: int(b>>4) & 3, index: int(b & 0x0F)})
		}
	}
	sort.Slice(fans, func(i, j int) bool {
		if fans[i].port != fans[j].port {
			return fans[i].port < fans[j].port
		}
		return fans[i].index < fans[j].index
	})
	return fans, nil
}

// Describe returns the raw status answer, one entry per reported fan slot.
func (d *device) Describe() string {
	if len(d.raw) <= offLen {
		return ""
	}
	var b strings.Builder
	n := int(d.raw[offLen]) / 3
	fmt.Fprintf(&b, "A1: %d slots:", n)
	for i := 0; i < n && offPayload+3*i+2 < len(d.raw); i++ {
		e := d.raw[offPayload+3*i : offPayload+3*i+3]
		fmt.Fprintf(&b, " [%02x port%d fan%d det=%t rpm=%d]", e[0], int(e[0]>>4)&3+1, int(e[0]&0x0F)+1, e[0]&flagDetected != 0, int(e[1])<<8|int(e[2]))
	}
	return b.String()
}

func (d *device) ID() string   { return fmt.Sprintf("lianlitl:%04x:%04x", vendorID, productTL) }
func (d *device) Name() string { return "Lian Li UNI FAN TL controller" }

// Zones: one per controller port, two LEDs per fan in chain order (top ring,
// bottom ring), used by canvas mode.
func (d *device) Zones() []leds.Zone {
	var zs []leds.Zone
	for _, f := range d.fans {
		id := fmt.Sprintf("port%d", f.port+1)
		if len(zs) == 0 || zs[len(zs)-1].ID != id {
			zs = append(zs, leds.Zone{ID: id, Name: fmt.Sprintf("Port %d fans (top, bottom ring each)", f.port+1), Order: "RGB"})
		}
		zs[len(zs)-1].Count += 2
		zs[len(zs)-1].Max += 2
	}
	return zs
}

func (d *device) Conflicts() []string {
	return conflicts.TL
}

func defaultSettings() settings {
	return settings{motherboard: true, mode: 1, brightness: maxLevel, speed: 2,
		colors: []color.RGBA{{0, 155, 222, 255}}, interval: defaultInterval, minDelta: defaultMinDelta}
}

// Settings implements leds.Configurable.
func (d *device) Settings() []leds.Param {
	return []leds.Param{
		{Key: "mode", Label: "Lighting", Type: leds.ParamEnum, Values: []string{"passive", "motherboard", "hardware", "canvas"}, Default: "motherboard"},
		{Key: "allowFlashWrites", Label: "Allow controller lighting (hardware, canvas): every change is stored in the controller's flash and wears it out", Type: leds.ParamBool, Default: false},
		{Key: "syncSource", Label: "ARGB header the controller's sync input is connected to (device/zone, information for the layout)", Type: leds.ParamText, Default: ""},
		{Key: "effect", Label: "Hardware effect", Type: leds.ParamEnum, Values: modes, Default: "rainbow"},
		{Key: "color1", Label: "Color 1", Type: leds.ParamColor, Default: "#3a8dff"},
		{Key: "color2", Label: "Color 2", Type: leds.ParamColor, Default: "#ff0040"},
		{Key: "color3", Label: "Color 3", Type: leds.ParamColor, Default: "#00ff60"},
		{Key: "color4", Label: "Color 4", Type: leds.ParamColor, Default: "#ffffff"},
		{Key: "colors", Label: "Colors used", Type: leds.ParamNumber, Min: 1, Max: maxColors, Default: 1.0},
		{Key: "brightness", Label: "Brightness", Type: leds.ParamNumber, Min: 0, Max: maxLevel, Default: float64(maxLevel)},
		{Key: "speed", Label: "Effect speed", Type: leds.ParamNumber, Min: 0, Max: maxLevel, Default: 2.0},
		{Key: "direction", Label: "Direction", Type: leds.ParamEnum, Values: directions, Default: "clockwise"},
		{Key: "interval", Label: "Canvas: seconds between updates (each update is stored in the controller's flash)", Type: leds.ParamNumber,
			Min: minInterval.Seconds(), Max: maxInterval.Seconds(), Default: defaultInterval.Seconds()},
		{Key: "minDelta", Label: "Canvas: minimum color change", Type: leds.ParamNumber, Min: 0, Max: 255, Default: float64(defaultMinDelta)},
		{Key: "merge", Label: "Hardware effect across all fans", Type: leds.ParamBool, Default: false},
		{Key: "portOrder", Label: "Port order for the merged effect, e.g. 3,1,2", Type: leds.ParamText, Default: ""},
	}
}

// Configure implements leds.Configurable.
func (d *device) Configure(p leds.Params) error {
	s := settings{
		hardware:    p.String("mode") == "hardware",
		motherboard: p.String("mode") == "motherboard",
		passive:     p.String("mode") != "motherboard" && p.String("mode") != "hardware" && p.String("mode") != "canvas",
		brightness:  byte(clampInt(p.Float("brightness"), 0, maxLevel)),
		speed:       byte(clampInt(p.Float("speed"), 0, maxLevel)),
		interval:    time.Duration(clampInt(p.Float("interval"), int(minInterval.Seconds()), int(maxInterval.Seconds()))) * time.Second,
		minDelta:    clampInt(p.Float("minDelta"), 0, 255),
		merge:       p.Bool("merge"),
	}
	if (s.hardware || p.String("mode") == "canvas") && !p.Bool("allowFlashWrites") {
		return ErrFlashWrites
	}
	m := indexOf(modes, p.String("effect"))
	if m < 0 {
		return fmt.Errorf("lianlitl: unknown effect %q", p.String("effect"))
	}
	s.mode = byte(m + 1)
	order, err := parsePortOrder(p.String("portOrder"), d.ports)
	if err != nil {
		return err
	}
	s.portOrder = order
	s.direction = byte(max(indexOf(directions, p.String("direction")), 0))
	for i := 1; i <= clampInt(p.Float("colors"), 1, maxColors); i++ {
		s.colors = append(s.colors, p.Color(fmt.Sprintf("color%d", i)))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !equalSettings(s, d.set) {
		d.set = s
		d.dirty = true
		clear(d.sent)
	}
	return nil
}

// send writes a command and waits for its acknowledgement. The controller
// keeps only one incoming packet: a packet that arrives before the previous
// one was handled is lost without an answer. So every write waits for the
// echo [01, cmd], and a write without echo is repeated.
func (d *device) send(cmd byte, payload []byte) error {
	buf := make([]byte, d.inLen)
	for i := 0; i < staleReads; i++ { // answers of earlier commands
		if _, err := d.h.Read(buf, drainTimeout); err != nil {
			break
		}
	}
	for attempt := 0; attempt < sendAttempts; attempt++ {
		if err := d.h.Write(packet(cmd, payload), ioTimeout); err != nil {
			return err
		}
		for tries := 0; tries < 4; tries++ {
			n, err := d.h.Read(buf, ackTimeout)
			if err != nil {
				break
			}
			if n > offCmd && buf[0] == reportID && buf[offCmd] == cmd {
				return nil
			}
		}
	}
	return fmt.Errorf("lianlitl: command %#02x not acknowledged", cmd)
}

// group is the default group number of a port.
func group(port int) byte { return byte(port * 4 * 2) }

// setupGroup writes the default grouping of a port: one group, all its fans,
// both sides.
func (d *device) setupGroup(port int) error {
	p := []byte{group(port), 0}
	for _, f := range d.fans {
		if f.port == port {
			p = append(p, 0xC0|byte(port<<4|f.index&0x0F))
		}
	}
	p[1] = byte(len(p) - 2)
	if err := d.send(cmdFanGroup, p); err != nil {
		return err
	}
	d.grouped[port], d.ringGrouped[port] = true, false
	return nil
}

// SetupGroups writes the default grouping of one port (0-based); ledctl uses
// it to repair a controller left with a foreign grouping.
func (d *device) SetupGroups(port int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !contains(d.ports, port) {
		return fmt.Errorf("lianlitl: no fans on port %d", port+1)
	}
	d.dirty = true
	return d.setupGroup(port)
}

// groupLight sends one B0 group light for a port, grouping it first if needed.
func (d *device) groupLight(port int, mode, brightness, speed, direction byte, colors []color.RGBA) error {
	if !d.grouped[port] {
		if err := d.setupGroup(port); err != nil {
			return err
		}
	}
	p := make([]byte, lightLen)
	p[1], p[2], p[3], p[4] = group(port), mode, brightness, speed
	n := min(len(colors), maxColors)
	for i := 0; i < n; i++ {
		p[5+3*i], p[6+3*i], p[7+3*i] = colors[i].R, colors[i].G, colors[i].B
	}
	p[17], p[19] = direction, byte(n)
	return d.send(cmdGroupLite, p)
}

// Probe sends one command and returns the answers that arrive within a short
// time (research tool). Firmware and serial number commands are refused.
func (d *device) Probe(cmd byte, payload []byte) ([][]byte, error) {
	switch {
	case cmd == 0xA8 || cmd == 0xA9 || cmd == 0xAB || cmd == 0xAC, cmd < 0xA0:
		return nil, fmt.Errorf("lianlitl: command %#02x refused", cmd)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.h.Write(packet(cmd, payload), ioTimeout); err != nil {
		return nil, err
	}
	var out [][]byte
	for tries := 0; tries < 4; tries++ {
		buf := make([]byte, d.inLen)
		n, err := d.h.Read(buf, answerTimeout)
		if err != nil {
			break
		}
		out = append(out, buf[:n])
	}
	return out, nil
}

// RingColors gives every ring (top, bottom) of the fans of a port its own
// group and static color: cols in chain order, top then bottom per fan.
// Experimental.
func (d *device) RingColors(port int, cols []color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var fans []fan
	for _, f := range d.fans {
		if f.port == port {
			fans = append(fans, f)
		}
	}
	if len(fans) == 0 || len(fans) > 4 {
		return fmt.Errorf("lianlitl: port %d has %d fans (1..4 supported)", port+1, len(fans))
	}
	if err := d.leaveSync(); err != nil {
		return err
	}
	d.grouped[port], d.ringGrouped[port] = false, false
	for i, f := range fans {
		for side, bit := range []byte{0x80, 0x40} {
			g := byte(port*8 + 2*i + side)
			if err := d.send(cmdFanGroup, []byte{g, 1, bit | byte(port<<4|f.index&0x0F)}); err != nil {
				return err
			}
		}
	}
	for i := range 2 * len(fans) {
		var c color.RGBA
		if i < len(cols) {
			c = cols[i]
		}
		p := make([]byte, lightLen)
		p[1], p[2], p[3] = byte(port*8+i), modeStatic, maxLevel
		p[5], p[6], p[7] = c.R, c.G, c.B
		p[19] = 1
		if err := d.send(cmdGroupLite, p); err != nil {
			return err
		}
	}
	return nil
}

// FanColors gives every fan of a port its own group (up to four per port)
// and a static color, in chain order. Experimental: the default grouping is
// written again before the next port-wide light.
func (d *device) FanColors(port int, cols []color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var fans []fan
	for _, f := range d.fans {
		if f.port == port {
			fans = append(fans, f)
		}
	}
	if len(fans) == 0 || len(fans) > 4 {
		return fmt.Errorf("lianlitl: port %d has %d fans (1..4 supported)", port+1, len(fans))
	}
	if err := d.leaveSync(); err != nil {
		return err
	}
	d.grouped[port], d.ringGrouped[port] = false, false
	for i, f := range fans {
		g := byte((port*4 + i) * 2)
		if err := d.send(cmdFanGroup, []byte{g, 1, 0xC0 | byte(port<<4|f.index&0x0F)}); err != nil {
			return err
		}
	}
	for i := range fans {
		g := byte((port*4 + i) * 2)
		var c color.RGBA
		if i < len(cols) {
			c = cols[i]
		}
		p := make([]byte, lightLen)
		p[1], p[2], p[3] = g, modeStatic, maxLevel
		p[5], p[6], p[7] = c.R, c.G, c.B
		p[19] = 1
		if err := d.send(cmdGroupLite, p); err != nil {
			return err
		}
	}
	return nil
}

// setSync switches every port between the ARGB input (on) and the
// controller's own output: one A3 per port, addressed to its first fan.
// Verified on hardware: the switch is per port, an A3 on one port leaves the
// others as they are. Leaving sync gives that fan a static black light of its
// own (mode index 0 would start a firmware effect); the group light that
// follows replaces it.
func (d *device) setSync(on bool) error {
	for _, port := range d.ports {
		var f fan
		for _, x := range d.fans {
			if x.port == port {
				f = x
				break
			}
		}
		p := make([]byte, lightLen)
		p[0] = byte(port << 4)
		if on {
			p[0] |= 1
		} else {
			p[2], p[19] = modeStatic, 1
		}
		p[1] = byte(port<<4 | f.index&0x0F)
		if err := d.send(cmdFanLight, p); err != nil {
			return err
		}
	}
	d.synced = on
	return nil
}

// leaveSync switches the ports back to the controller's output if they may
// follow the ARGB input.
func (d *device) leaveSync() error {
	if !d.synced {
		return nil
	}
	return d.setSync(false)
}

// SetFrame implements leds.Device. In motherboard mode the controller is
// switched to its ARGB input once. In hardware mode the canvas is ignored and
// the effect is sent once per change; a frame that is black on every fan
// (pause, shutdown) switches the ports off. In canvas mode every ring gets the
// color of its canvas LED as static light, at most once per interval and only
// when it changed by more than minDelta.
func (d *device) SetFrame(frame map[string][]color.RGBA) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.set
	if s.passive {
		return nil
	}
	if s.motherboard {
		if !d.dirty {
			return nil
		}
		d.dirty = false
		d.pending = d.pending[:0]
		return d.setSync(true)
	}

	if err := d.leaveSync(); err != nil {
		return err
	}
	allBlack := true
	for _, port := range d.ports {
		for _, c := range frame[fmt.Sprintf("port%d", port+1)] {
			if c.R|c.G|c.B != 0 {
				allBlack = false
			}
		}
	}

	if s.hardware && s.merge && !allBlack {
		if !d.dirty {
			return nil
		}
		d.dirty = false
		d.pending = d.pending[:0]
		return d.mergedLight(s)
	}

	if s.hardware && !allBlack {
		if d.dirty {
			d.dirty = false
			d.pending = append(d.pending[:0], d.ports...)
		}
		return d.flush(func(port int) error {
			return d.groupLight(port, s.mode, s.brightness, s.speed, s.direction, s.colors)
		})
	}

	if s.hardware {
		// switched off by a black frame: every port static black, the
		// effect is sent again after the pause
		if !d.dirty {
			d.dirty = true
			d.pending = append(d.pending[:0], d.ports...)
		}
		d.unlimited = true
		defer func() { d.unlimited = false }()
		return d.flush(func(port int) error {
			return d.groupLight(port, modeStatic, maxLevel, 0, 0, []color.RGBA{{}})
		})
	}

	// canvas mode
	if d.dirty {
		d.dirty = false
		d.pending = d.pending[:0]
		clear(d.sent)
	}
	cols := make([]color.RGBA, len(d.rings))
	for i, r := range d.rings {
		cols[i] = mean(frame[fmt.Sprintf("port%d", r.port+1)], r.first, r.n)
	}
	now := d.now()
	for i := range d.rings {
		last, ever := d.sent[i]
		urgent := allBlack && (!ever || last != cols[i])
		due := !ever || (now.Sub(d.sentAt[i]) >= s.interval && delta(last, cols[i]) > s.minDelta)
		if (urgent || due) && !contains(d.pending, i) {
			d.pending = append(d.pending, i)
		}
	}
	if allBlack {
		d.unlimited = true // shutdown or pause: every ring now, not spread over frames
	}
	defer func() { d.unlimited = false }()
	return d.flush(func(i int) error {
		if err := d.ringLight(i, cols[i]); err != nil {
			return err
		}
		d.sent[i], d.sentAt[i] = cols[i], now
		return nil
	})
}

// ring is one canvas light: the top or bottom ring of a fan in its own group,
// or (ports with more than 4 fans, beyond the 8 groups of a port) the whole
// port in its default group.
type ring struct {
	port     int
	group    byte
	members  []byte
	first, n int // LEDs of the port zone that give its color
}

// buildRings lays out the canvas lights; port zones have two LEDs per fan
// (top ring, bottom ring) in chain order.
func buildRings(fans []fan) []ring {
	byPort := map[int][]fan{}
	var ports []int
	for _, f := range fans {
		if len(byPort[f.port]) == 0 {
			ports = append(ports, f.port)
		}
		byPort[f.port] = append(byPort[f.port], f)
	}
	var rs []ring
	for _, port := range ports {
		pf := byPort[port]
		if len(pf) > 4 {
			r := ring{port: port, group: group(port), first: 0, n: 2 * len(pf)}
			for _, f := range pf {
				r.members = append(r.members, 0xC0|byte(port<<4|f.index&0x0F))
			}
			rs = append(rs, r)
			continue
		}
		for i, f := range pf {
			for side, bit := range []byte{0x80, 0x40} {
				rs = append(rs, ring{port: port, group: byte(port*8 + 2*i + side),
					members: []byte{bit | byte(port<<4|f.index&0x0F)}, first: 2*i + side, n: 1})
			}
		}
	}
	return rs
}

// ringLight sends the static color of one canvas light, grouping its port
// for canvas use first if needed.
func (d *device) ringLight(i int, c color.RGBA) error {
	r := d.rings[i]
	if !d.ringGrouped[r.port] {
		for _, o := range d.rings {
			if o.port != r.port {
				continue
			}
			if err := d.send(cmdFanGroup, append([]byte{o.group, byte(len(o.members))}, o.members...)); err != nil {
				return err
			}
		}
		d.ringGrouped[r.port], d.grouped[r.port] = true, false
	}
	p := make([]byte, lightLen)
	p[1], p[2], p[3] = r.group, modeStatic, maxLevel
	p[5], p[6], p[7] = c.R, c.G, c.B
	p[19] = 1
	return d.send(cmdGroupLite, p)
}

func mean(zone []color.RGBA, first, n int) color.RGBA {
	var r, g, b, k int
	for i := first; i < first+n && i < len(zone); i++ {
		r, g, b, k = r+int(zone[i].R), g+int(zone[i].G), b+int(zone[i].B), k+1
	}
	if k == 0 {
		return color.RGBA{A: 255}
	}
	return color.RGBA{uint8(r / k), uint8(g / k), uint8(b / k), 255}
}

// flush writes at most writesPerFrame pending items.
func (d *device) flush(write func(item int) error) error {
	for n := 0; (d.unlimited || n < writesPerFrame) && len(d.pending) > 0; n++ {
		if err := write(d.pending[0]); err != nil {
			return err
		}
		d.pending = d.pending[1:]
	}
	return nil
}

func (d *device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.h.Close()
}

func delta(a, b color.RGBA) int {
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	return abs(int(a.R)-int(b.R)) + abs(int(a.G)-int(b.G)) + abs(int(a.B)-int(b.B))
}

func clampInt(v float64, lo, hi int) int {
	return max(lo, min(hi, int(v+0.5)))
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func equalSettings(a, b settings) bool {
	if a.hardware != b.hardware || a.motherboard != b.motherboard || a.passive != b.passive || a.mode != b.mode || a.brightness != b.brightness || a.speed != b.speed ||
		a.direction != b.direction || a.interval != b.interval || a.minDelta != b.minDelta || len(a.colors) != len(b.colors) ||
		a.merge != b.merge || fmt.Sprint(a.portOrder) != fmt.Sprint(b.portOrder) {
		return false
	}
	for i := range a.colors {
		if a.colors[i] != b.colors[i] {
			return false
		}
	}
	return true
}

// parsePortOrder reads "3,1,2" (1-based) into 0-based ports; ports left out
// follow in ascending order, unknown ports are rejected.
func parsePortOrder(text string, ports []int) ([]int, error) {
	var order []int
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		var n int
		if _, err := fmt.Sscan(f, &n); err != nil || !contains(ports, n-1) || contains(order, n-1) {
			return nil, fmt.Errorf("lianlitl: port order %q: port %q unknown or repeated", text, f)
		}
		order = append(order, n-1)
	}
	for _, p := range ports {
		if !contains(order, p) {
			order = append(order, p)
		}
	}
	return order, nil
}

// mergedLight puts every fan into one group (ports in the configured order,
// fans in chain order) and lights it with the effect, so the effect runs
// across all fans.
func (d *device) mergedLight(s settings) error {
	p := []byte{mergedGroup, 0}
	for _, port := range s.portOrder {
		for _, f := range d.fans {
			if f.port == port {
				p = append(p, 0xC0|byte(port<<4|f.index&0x0F))
			}
		}
	}
	p[1] = byte(len(p) - 2)
	if err := d.send(cmdFanGroup, p); err != nil {
		return err
	}
	clear(d.grouped) // the per-port groups are gone now
	l := make([]byte, lightLen)
	l[1], l[2], l[3], l[4] = mergedGroup, s.mode, s.brightness, s.speed
	n := min(len(s.colors), maxColors)
	for i := 0; i < n; i++ {
		l[5+3*i], l[6+3*i], l[7+3*i] = s.colors[i].R, s.colors[i].G, s.colors[i].B
	}
	l[17], l[19] = s.direction, byte(n)
	return d.send(cmdGroupLite, l)
}

// TestLight lights one fan (fan >= 0) or a whole port (fan < 0), top or bottom
// ring, so it can be found in the case. The next light command ends it. It
// writes to the controller in every mode, passive included.
func (d *device) TestLight(port, fan int, bottom bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	b := byte(0)
	if bottom {
		b = 1
	}
	if fan < 0 {
		if !contains(d.ports, port) {
			return fmt.Errorf("lianlitl: no fans on port %d", port+1)
		}
		return d.send(cmdTestLight, []byte{3, byte(port & 3), b})
	}
	for _, f := range d.fans {
		if f.port == port && f.index == fan {
			return d.send(cmdTestLight, []byte{1, byte((port&3)<<4 | fan&0x0F), b})
		}
	}
	return fmt.Errorf("lianlitl: no fan %d on port %d", fan+1, port+1)
}

// BlinkPort makes the fans of one port blink. Like TestLight it writes in
// every mode, passive included.
func (d *device) BlinkPort(port int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !contains(d.ports, port) {
		return fmt.Errorf("lianlitl: no fans on port %d", port+1)
	}
	return d.send(cmdBlinkPort, []byte{byte(port)})
}
