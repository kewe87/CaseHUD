package panel

// Models of the WinUSB screens that share the universal88 driver code, and
// their registry entries. See docs/DRIVERS.md for what each one does.

import (
	"fmt"
	"image"
	"sort"
	"strconv"
	"strings"

	"github.com/kewe87/casehud/internal/winusb"
)

// usFraming is how a command block is built.
type usFraming int

const (
	usFrame88 usFraming = iota // 500 bytes of arguments, 6 zero bytes, A1 1A
	usFrameSL                  // 504 bytes plaintext, 512 bytes ciphertext, fixed-size data transfers
)

// usInit selects the start-up sequence.
type usInit int

const (
	usInitBar  usInit = iota // halt, hide clock, (rotate), version, clear overlay, backlight
	usInitSL                 // new-LCD query, frame rate, version, rotation, backlight
	usInitFlex               // new-LCD query, version, clear overlay, rotation, backlight
)

// usModel holds everything that differs between the screens.
type usModel struct {
	driver   string // registry ID
	idPrefix string // panel ID prefix
	name     string // model name
	pid      uint16

	// short x long is the native (portrait) frame; fan screens are square.
	short, long int
	square      bool
	round       bool

	framing usFraming
	init    usInit

	brightMax int      // raw backlight maximum (percent when it equals 100)
	brightMap [][2]int // optional percent -> raw curve (points, ascending)
	defBright int      // brightness before the first SetBrightness (0 = 100)
	maxJPEG   int      // largest picture the device takes
	fps       int      // animation limit offered to the engine
	sendTurn  bool     // write 0x0D 0 at start (a stored setting; 8.8" only)
	hwTurn    bool     // SetRotation is sent to the device
	turnShift int      // added to the quarter turns on the wire (panel mounted offset)
	ackFrames bool     // every picture is acknowledged on the bulk pipe
	lenient   bool     // missing answers are no error (they may arrive elsewhere)
	zlp       bool     // zero-length packet after transfers of a multiple of 512
	heartbeat bool     // version query after a second of silence
	version   func(string) (string, bool)

	// Bars only: the device sets its clock from the host (0x33, flag 2) and can
	// keep it as an offline clock; takes a static picture plus a PNG layer
	// (layers) and plays H.264 itself (video).
	clock, layers, video bool
	turn180              bool // understands the device-side 180 degree rotation (see SetDeviceFlip)
	exitImage            bool // Close can leave a picture (SetExitImage); 8.8" only
}

func (m *usModel) landscape() image.Point {
	if m.square {
		return image.Pt(m.short, m.short)
	}
	return image.Pt(m.long, m.short)
}

func (m *usModel) jpegMax() int {
	if m.maxJPEG > 0 {
		return m.maxJPEG
	}
	return us88MaxJPEG
}

func (m *usModel) fpsMax() int {
	if m.fps > 0 {
		return m.fps
	}
	return us88FPS
}

// devBrightness maps percent (0-100) to the device's backlight value.
func (m *usModel) devBrightness(percent int) byte {
	percent = min(100, max0(percent))
	if len(m.brightMap) > 0 {
		pts := m.brightMap
		for i := 1; i < len(pts); i++ {
			if percent <= pts[i][0] {
				x0, y0, x1, y1 := pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]
				return byte(y0 + ((percent-x0)*(y1-y0)+(x1-x0)/2)/(x1-x0))
			}
		}
		return byte(pts[len(pts)-1][1])
	}
	max := m.brightMax
	if max == 0 {
		max = usMaxBacklight
	}
	return usScale(percent, max)
}

// wireTurn is the rotation value for the device: quarter turns plus the
// mounting offset, modulo 4.
func (m *usModel) wireTurn(degrees int) byte {
	return byte((degrees/90 + m.turnShift) & 3)
}

var usModel88 = usModel{
	driver: "us88", idPrefix: "us88", name: "Universal Screen 8.8\"", pid: us88PID,
	short: us88Width, long: us88Height, framing: usFrame88, init: usInitBar,
	brightMax: usMaxBacklight, maxJPEG: us88MaxJPEG, fps: us88FPS,
	sendTurn: true, ackFrames: true, zlp: true,
	version: usVersion88,
	clock:   true, layers: true, video: true, turn180: true, exitImage: true,
}

var usModel92 = usModel{
	driver: "us92", idPrefix: "us92", name: "Vision-M 9.2\"", pid: 0xA092,
	short: 464, long: 1920, framing: usFrame88, init: usInitBar,
	brightMax: 100, maxJPEG: us88MaxJPEG, fps: us88FPS,
	ackFrames: true, zlp: true,
	version: usVersionDigits,
	clock:   true, layers: true, video: true, turn180: true,
}

var usModel207 = usModel{
	driver: "us207", idPrefix: "us207", name: "Lancool 207 Digital", pid: 0xA065,
	short: 720, long: 1472, framing: usFrame88, init: usInitBar,
	brightMax: 100, maxJPEG: us88MaxJPEG, fps: us88FPS,
	ackFrames: true, zlp: true,
	version: usVersion207,
	clock:   true, layers: true, video: true,
}

// The SL V3 / TL V2 LCD: 400x400 round, own block framing, rotation offset
// by one quarter (the panel sits turned in the housing), backlight curve.
var (
	usModelSL = usModel{
		driver: "slv3", idPrefix: "slv3", name: "SL V3 LCD", pid: 0x0005,
		short: 400, long: 400, square: true, round: true, framing: usFrameSL, init: usInitSL,
		brightMax: 100, brightMap: [][2]int{{0, 0}, {25, 10}, {50, 30}, {75, 40}, {100, 100}},
		maxJPEG: slMaxJPEG, fps: 20, hwTurn: true, turnShift: 1, lenient: true, heartbeat: true,
		version: usVersionRaw,
	}
	usModelTL2 = func() usModel { m := usModelSL; m.name, m.pid = "TL V2 LCD", 0x0006; return m }()
)

// The Flex fan screens: 400x400, framing of the 8.8".
var (
	usModelTLFlex = usModel{
		driver: "flex", idPrefix: "flex", name: "TL Flex LCD", pid: 0xA018,
		short: 400, long: 400, square: true, framing: usFrame88, init: usInitFlex,
		brightMax: 100, maxJPEG: us88MaxJPEG, fps: 20, hwTurn: true, turnShift: 1,
		ackFrames: true, zlp: true,
		version: usVersionLastPart,
	}
	usModelSLFlex = func() usModel { m := usModelTLFlex; m.name, m.pid = "SL INF Flex LCD", 0xA019; return m }()
)

// --- version strings ---------------------------------------------------

// usVersion88 parses "lianli88_XXXX_YYYY": version = (YYYY-8)/10 . (YYYY-8)%10,
// invalid below YYYY 18.
func usVersion88(s string) (string, bool) {
	n, ok := lastNumber(s)
	if !ok || n < 18 {
		return "", false
	}
	return fmt.Sprintf("%d.%d", (n-8)/10, (n-8)%10), true
}

// usVersion207 parses "lianli207_XXXX_YYYY": version = YYYY/10 . YYYY%10,
// invalid below YYYY 10.
func usVersion207(s string) (string, bool) {
	n, ok := lastNumber(s)
	if !ok || n < 10 {
		return "", false
	}
	return fmt.Sprintf("%d.%d", n/10, n%10), true
}

// usVersionDigits takes the last ten characters, which must be the digits
// yyyyMMddNN, as the version.
func usVersionDigits(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 10 {
		return "", false
	}
	d := s[len(s)-10:]
	for _, r := range d {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return d, true
}

// usVersionLastPart takes the text behind the last underscore.
func usVersionLastPart(s string) (string, bool) {
	s = strings.TrimSpace(s)
	i := strings.LastIndexByte(s, '_')
	if i < 0 || i == len(s)-1 {
		return "", false
	}
	return s[i+1:], true
}

func usVersionRaw(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != ""
}

func lastNumber(s string) (int, bool) {
	i := strings.LastIndexByte(s, '_')
	n, err := strconv.Atoi(strings.TrimSpace(s[i+1:]))
	return n, err == nil && n >= 0
}

// --- chain position ----------------------------------------------------

// usbChain splits a LocationPaths entry such as
// "PCIROOT(0)#PCI(0801)#USBROOT(0)#USB(3)#USB(2)" into the hub it hangs on
// (everything before the last USB part) and the 0-based port on that hub.
func usbChain(loc string) (hub string, port int, ok bool) {
	parts := strings.Split(loc, "#")
	for len(parts) > 0 {
		last := parts[len(parts)-1]
		if strings.HasPrefix(last, "USB(") && strings.HasSuffix(last, ")") {
			n, err := strconv.Atoi(last[4 : len(last)-1])
			if err != nil || n < 1 {
				return "", 0, false
			}
			return strings.Join(parts[:len(parts)-1], "#"), n - 1, true
		}
		return "", 0, false
	}
	return "", 0, false
}

// identify names the panel: bars by their count, fan screens by their
// serial number (the same device keeps its ID), with the chain position in
// the name when known.
func (m *usModel) identify(info winusb.Info, n int, all []winusb.Info, taken map[string]bool) (id, name string) {
	if !m.square {
		id = fmt.Sprintf("%s-%d", m.idPrefix, n)
		name = m.name
		if n > 0 {
			name = fmt.Sprintf("%s #%d", name, n+1)
		}
		return id, name
	}
	stem := m.idPrefix + "-" + tlSafe(info.Serial)
	if info.Serial == "" || tlSafe(info.Serial) == "" {
		stem = m.idPrefix + "-u"
	}
	id = stem
	for k := 2; taken[id]; k++ {
		id = fmt.Sprintf("%s-%d", stem, k)
	}
	taken[id] = true
	name = m.name
	if hub, port, ok := usbChain(info.Location); ok {
		hubs := map[string]bool{}
		for _, o := range all {
			if h, _, ok := usbChain(o.Location); ok {
				hubs[h] = true
			}
		}
		var order []string
		for h := range hubs {
			order = append(order, h)
		}
		sort.Strings(order)
		g := sort.SearchStrings(order, hub) + 1
		name = fmt.Sprintf("%s (group %d, position %d)", m.name, g, port+1)
	} else if s := tlSafe(info.Serial); s != "" {
		name = fmt.Sprintf("%s (%s)", m.name, s)
	}
	return id, name
}

// --- registry ----------------------------------------------------------

func init() {
	reg := func(id, name string, experimental bool, order int, models []*usModel, caps Capabilities) {
		var ms []Model
		var ids []winusb.ID
		for _, m := range models {
			ms = append(ms, Model{VendorID: us88VID, ProductID: m.pid, Name: m.name})
			ids = append(ids, winusb.ID{VendorID: us88VID, ProductID: m.pid})
		}
		Register(Driver{
			ID: id, Name: name, Vendor: "USB 1CBE", Kind: KindDisplay, Transport: TransportWinUSB,
			Experimental: experimental, Order: order, TracksPaths: true, Models: ms, Caps: caps,
			Enumerate: func() ([]string, error) { return winusbPaths(ids...) },
			Discover:  func() ([]Panel, error) { return discoverUSB(models...) },
		})
	}
	reg("us88", "Universal Screen 8.8\"", false, 30, []*usModel{&usModel88}, Capabilities{
		Width: 1920, Height: 480, Rotation: RotationSoftware, Rotations: []int{0, 90, 180, 270}, BrightnessMax: usMaxBacklight})
	reg("us92", "Vision-M 9.2\"", true, 31, []*usModel{&usModel92}, Capabilities{
		Width: 1920, Height: 464, Rotation: RotationSoftware, Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100})
	reg("us207", "Lancool 207 Digital", true, 32, []*usModel{&usModel207}, Capabilities{
		Width: 1472, Height: 720, Rotation: RotationSoftware, Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100})
	reg("slv3", "SL V3 LCD / TL V2 LCD", true, 33, []*usModel{&usModelSL, &usModelTL2}, Capabilities{
		Width: 400, Height: 400, Round: true, Rotation: RotationDevice, Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100})
	reg("flex", "TL Flex LCD / SL INF Flex LCD", true, 34, []*usModel{&usModelTLFlex, &usModelSLFlex}, Capabilities{
		Width: 400, Height: 400, Rotation: RotationDevice, Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100})
}
