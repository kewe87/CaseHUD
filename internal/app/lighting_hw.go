package app

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// Hardware that is part of the lighting integration besides the TL fans: the
// mainboard controller, the RAM sticks (only through the SMBus bridge of the
// sensor service) and the LED frame of the 8.8" display.

const (
	// ringDeviceID is the LED frame of the Universal Screen 8.8" (USB
	// 0416:8050). The display itself (1CBE:A088) is a different device that
	// this backend never opens.
	ringDeviceID = "lianli:0416:8050"
	ringName     = "Universal Screen 8.8\" \u2013 LED-Rahmen"
	ringLibrary  = "universal-screen-8-8" // optional library part with the ring outline

	boardBackend    = "asusaura"
	ramBackend      = "enram"
	unsupportedPref = "enram-unsupported:"

	ramHintText = "RAM-Beleuchtung braucht die SMBus-Br\u00fccke im Sensor-Dienst (Einstellungen \u2192 Datenquellen; Datei C:\\ProgramData\\CaseHUD\\sensors.json mit \"smbusBridge\": true, siehe docs/SMBUS-BRIDGE.md)."

	flashWarnText = "Der Lüfter-Controller speichert jede Beleuchtungs-Änderung dauerhaft in seinem Flash-Speicher. Häufige Änderungen verschleißen ihn. Dieser Modus ist aus, bis du ihn bestätigst. Empfohlen: „Mit Mainboard synchronisieren“."
)

// smbusBridgePipe is the only way to the SMBus: the named pipe of the
// sensor service. CaseHUD has no code path that opens the bus itself.
const smbusBridgePipe = enram.BridgePipe

// smbusBridgeProbe tells whether the bridge pipe exists right now. It lists
// the pipe directory instead of opening the pipe, so it never takes one of
// the service's pipe instances. Tests replace it.
var smbusBridgeProbe = func() bool {
	ents, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return false
	}
	want := strings.ToLower(smbusBridgePipe[strings.LastIndex(smbusBridgePipe, `\`)+1:])
	for _, en := range ents {
		if strings.ToLower(en.Name()) == want {
			return true
		}
	}
	return false
}

var (
	errRAMBusy   = errors.New("SMBus busy; the next scan tries again")
	errRAMDenied = errors.New("the SMBus bridge refused the request (switched off or not allowed)")
	// the text of the bridge check is translated here, not in the LED package
	errRAMUntrusted = errors.New("SMBus-Brücke nicht vertrauenswürdig")
)

// ramNotes is the RAM status the Beleuchtung tab shows next to the hint.
type ramNotes struct {
	mu   sync.Mutex
	note string
}

func (r *ramNotes) set(s string) {
	r.mu.Lock()
	r.note = s
	r.mu.Unlock()
}

func (r *ramNotes) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.note
}

// newRAMBackend wraps the bridge backend: it only talks to the bridge while
// the pipe exists, turns sticks of unverified families into inert entries and
// reports busy and denied states instead of failing the whole scan.
func newRAMBackend(inner leds.Backend, avail func() bool, notes *ramNotes) leds.Backend {
	return &ramGate{inner: inner, avail: avail, notes: notes}
}

type ramGate struct {
	inner leds.Backend
	avail func() bool
	notes *ramNotes
}

func (g *ramGate) Name() string { return ramBackend }

func (g *ramGate) Discover(ctx context.Context) ([]leds.Device, error) {
	if !g.avail() {
		g.notes.set("")
		return nil, nil
	}
	devs, err := g.inner.Discover(ctx)
	out := make([]leds.Device, 0, len(devs))
	for _, d := range devs {
		out = append(out, &namedDevice{Device: d})
	}
	var rest []error
	note := ""
	for _, le := range leafErrors(err) {
		var us enram.ErrUnknownStick
		switch {
		case errors.As(le, &us):
			out = append(out, &unsupportedStick{addr: us.Addr, name: cleanDeviceText(us.Name)})
			note = "Ein RAM-Modul ist nicht unterstützt und wird nicht beschrieben."
		case errors.Is(le, enram.ErrBusy):
			rest = append(rest, errRAMBusy)
			note = "Der SMBus ist gerade belegt; CaseHUD versucht es beim nächsten Suchlauf erneut."
		case errors.Is(le, enram.ErrDenied):
			rest = append(rest, errRAMDenied)
			note = "Die SMBus-Brücke hat die Anfrage abgelehnt (Brücke ausgeschaltet oder Zugriff nicht erlaubt)."
		case errors.Is(le, enram.ErrUntrusted):
			rest = append(rest, errRAMUntrusted)
			note = "SMBus-Brücke nicht vertrauenswürdig: CaseHUD spricht sie nicht an."
		default:
			rest = append(rest, le)
		}
	}
	g.notes.set(note)
	return out, errors.Join(rest...)
}

// leafErrors flattens errors.Join trees.
func leafErrors(err error) []error {
	if err == nil {
		return nil
	}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range j.Unwrap() {
			out = append(out, leafErrors(e)...)
		}
		return out
	}
	return []error{err}
}

// softLightErr is the error text for the status line: a busy bus is routine
// and not worth a message.
func softLightErr(err error) string {
	var keep []error
	for _, le := range leafErrors(err) {
		if !errors.Is(le, errRAMBusy) {
			keep = append(keep, le)
		}
	}
	if len(keep) == 0 {
		return ""
	}
	return errors.Join(keep...).Error()
}

// cleanDeviceText keeps printable ASCII of a name that came from hardware.
func cleanDeviceText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r < 0x7f && b.Len() < 40 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unsupportedStick stands for a RAM controller of an unverified family. It
// has no zones and never writes to the bus; it only makes the stick visible.
type unsupportedStick struct {
	addr byte
	name string
}

func (u *unsupportedStick) ID() string { return fmt.Sprintf("%s%02x", unsupportedPref, u.addr) }
func (u *unsupportedStick) Name() string {
	return fmt.Sprintf("RAM-Modul %s (nicht unterstützt)", u.name)
}
func (u *unsupportedStick) Zones() []leds.Zone                     { return nil }
func (u *unsupportedStick) SetFrame(map[string][]color.RGBA) error { return nil }
func (u *unsupportedStick) Conflicts() []string                    { return nil }
func (u *unsupportedStick) Close() error                           { return nil }

// namedDevice shows the name of a device with control characters and
// non-ASCII bytes removed: names come from the hardware and end up in the
// API and in logs.
type namedDevice struct{ leds.Device }

func (d *namedDevice) Name() string { return cleanDeviceText(d.Device.Name()) }

// cleanNames applies that to all devices of a backend.
type cleanNames struct{ inner leds.Backend }

func (b *cleanNames) Name() string { return b.inner.Name() }

func (b *cleanNames) Discover(ctx context.Context) ([]leds.Device, error) {
	devs, err := b.inner.Discover(ctx)
	out := make([]leds.Device, 0, len(devs))
	for _, d := range devs {
		out = append(out, &namedDevice{Device: d})
	}
	return out, err
}

// ringBackend names the LED frame of the 8.8" display for the device list. It
// only ever lists devices of the frame (USB 0416:8050).
type ringBackend struct{ inner leds.Backend }

func (b *ringBackend) Name() string { return b.inner.Name() }

func (b *ringBackend) Discover(ctx context.Context) ([]leds.Device, error) {
	devs, err := b.inner.Discover(ctx)
	out := make([]leds.Device, 0, len(devs))
	for _, d := range devs {
		if d.ID() != ringDeviceID {
			d.Close() // not the frame: never used
			continue
		}
		out = append(out, &ringDevice{Device: d})
	}
	return out, err
}

type ringDevice struct{ leds.Device }

func (*ringDevice) Name() string { return ringName }

// defaultRingSegments is the layout of the frame when the user has not placed
// it yet: the library part with the ring outline if the library has it, else
// a rectangle around the top of the canvas.
func defaultRingSegments(n int) []leds.Segment {
	if c, ok := leds.LookupComponent(ringLibrary); ok && len(c.LEDs) > 0 {
		return []leds.Segment{{Name: ringName, Shape: "component", Component: ringLibrary,
			Count: min(n, len(c.LEDs)), X: 0.30, Y: 0.02, W: 0.40, H: 0.12}}
	}
	return []leds.Segment{{Name: ringName, Count: n, Shape: "rect", Reverse: true, X: 0.30, Y: 0.02, W: 0.40, H: 0.12}}
}

// applyDefaultLayouts gives devices that come with a known shape their
// layout the first time they show up. Only zones without any stored entry
// are touched, so a layout the user changed or cleared stays as it is.
func (e *Engine) applyDefaultLayouts(eng *leds.Engine) {
	for _, d := range eng.Devices() {
		if d.ID != ringDeviceID {
			continue
		}
		dc := eng.Config().Devices[d.ID] // read again for every device: edits that came in meanwhile stay
		for _, z := range d.Zones {
			if z.ID != "ring" || z.Count <= 0 {
				continue
			}
			if _, has := dc.Zones[z.ID]; has {
				continue
			}
			zones := map[string]leds.ZoneConfig{}
			for k, v := range dc.Zones {
				zones[k] = v
			}
			zones[z.ID] = leds.ZoneConfig{Segments: defaultRingSegments(z.Count)}
			dc.Zones = zones
			if clean, err := cleanLightingConfig(leds.Config{Devices: map[string]leds.DeviceConfig{d.ID: dc}}, nil); err == nil {
				eng.SetDeviceConfig(d.ID, clean.Devices[d.ID])
			}
		}
	}
}

// ---------- configuration errors of devices ----------

// setCfgErr remembers why a device refused its settings (shown as a warning
// in the device list); nil clears it.
func (l *lightingState) setCfgErr(id string, err error) {
	l.cfgErrMu.Lock()
	defer l.cfgErrMu.Unlock()
	if err == nil {
		delete(l.cfgErrs, id)
		return
	}
	if l.cfgErrs == nil {
		l.cfgErrs = map[string]error{}
	}
	l.cfgErrs[id] = err
}

func (l *lightingState) cfgErr(id string) error {
	l.cfgErrMu.Lock()
	defer l.cfgErrMu.Unlock()
	return l.cfgErrs[id]
}

// deviceWarning is the user-facing text for a refused configuration.
func (l *lightingState) deviceWarning(id string) string {
	err := l.cfgErr(id)
	switch {
	case err == nil:
		return ""
	case errors.Is(err, lianlitl.ErrFlashWrites):
		return flashWarnText
	default:
		return "Die Einstellungen wurden vom Gerät abgelehnt: " + cleanDeviceText(err.Error())
	}
}

// ---------- view parts ----------

// LightingRAM is the state of the RAM lighting (only through the SMBus bridge).
type LightingRAM struct {
	Available bool   `json:"available"`      // the bridge pipe exists
	Hint      string `json:"hint,omitempty"` // what to do when it does not
	Note      string `json:"note,omitempty"` // busy, denied, unsupported stick
}

// LightingSyncChoice is a mainboard header the TL controller can follow.
type LightingSyncChoice struct {
	ID    string `json:"id"` // "device/zone"
	Label string `json:"label"`
}

// LightingSync describes the helper for a TL controller that follows the
// mainboard header: the headers to pick from and how many fans a chain has.
type LightingSync struct {
	Source  string               `json:"source"`
	Fans    int                  `json:"fans"` // longest chain, from the ports of the controller
	Choices []LightingSyncChoice `json:"choices"`
}

// refreshBridgeProbe looks for the bridge pipe at most every 3 seconds; the
// view only reads the remembered result, so no lock is held across the probe.
func (e *Engine) refreshBridgeProbe() {
	l := &e.light
	now := time.Now().UnixNano()
	last := l.bridgeAt.Load()
	if now-last < int64(3*time.Second) || !l.bridgeAt.CompareAndSwap(last, now) {
		return
	}
	l.bridgeUp.Store(smbusBridgeProbe())
}

func (e *Engine) ramView() LightingRAM {
	if e.Demo {
		if os.Getenv("CASEHUD_DEMO_NO_BRIDGE") != "" {
			return LightingRAM{Hint: ramHintText}
		}
		return LightingRAM{Available: true}
	}
	r := LightingRAM{Available: e.light.bridgeUp.Load(), Note: e.light.ramNotes.get()}
	if !r.Available {
		r.Hint, r.Note = ramHintText, ""
	}
	return r
}

// syncChoices lists the ARGB headers of the mainboard devices.
func syncChoices(devs []leds.DeviceStatus) []LightingSyncChoice {
	var out []LightingSyncChoice
	for _, d := range devs {
		if d.Backend != boardBackend {
			continue
		}
		for _, z := range d.Zones {
			if z.Resizable && strings.HasPrefix(z.ID, "argb") {
				out = append(out, LightingSyncChoice{ID: d.ID + "/" + z.ID, Label: z.Name})
			}
		}
	}
	return out
}

func hasSetting(ps []leds.Param, key string) bool {
	for _, p := range ps {
		if p.Key == key {
			return true
		}
	}
	return false
}

// tlFanCount is the number of fans of the longest chain: the controller
// reports two LEDs (top and bottom ring) per fan in every port zone.
func tlFanCount(d leds.DeviceStatus) int {
	n := 0
	for _, z := range d.Zones {
		n = max(n, z.Count/2)
	}
	return n
}

// zoneLabel is "device · zone" for the mirror hint.
func zoneLabel(devs []leds.DeviceStatus, source string) string {
	dev, zone, ok := strings.Cut(source, "/")
	if !ok {
		return ""
	}
	for _, d := range devs {
		if d.ID != dev {
			continue
		}
		for _, z := range d.Zones {
			if z.ID == zone {
				return z.Name
			}
		}
	}
	return ""
}

// syncSegments is the layout of a mainboard header that feeds TL fans: one
// fan part per fan in chain order, in a row on the canvas.
func syncSegments(fans int) []leds.Segment {
	segs := make([]leds.Segment, 0, fans)
	slot := 0.9 / float64(fans)
	w := min(0.18, slot*0.95)
	h := min(0.28, w*float64(leds.CanvasWidth)/float64(leds.CanvasHeight))
	for i := 0; i < fans; i++ {
		segs = append(segs, leds.Segment{Name: fmt.Sprintf("TL-Lüfter %d", i+1), Shape: "component", Component: "fan-tl-26",
			Count: 26, X: 0.05 + float64(i)*slot + (slot-w)/2, Y: 0.36, W: w, H: h})
	}
	return segs
}

// LightingSetSync is the helper for TL fans that follow a mainboard header
// (the recommended mode): it remembers the header in the controller's
// setting syncSource and lays out that header's zone with one fan per fan of
// the longest chain. source "" only forgets the header. GUI only.
func (e *Engine) LightingSetSync(tlID, source string) (LightingView, error) {
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return LightingView{}, lightErr("lighting is off")
	}
	var tl *leds.DeviceStatus
	devs := eng.Devices()
	for i := range devs {
		if devs[i].ID == tlID && hasSetting(devs[i].Settings, "syncSource") {
			tl = &devs[i]
		}
	}
	if tl == nil {
		return LightingView{}, lightErr("fan controller not found")
	}
	patch := LightingPatch{Devices: map[string]LightingDevicePatch{
		tlID: {Settings: map[string]any{"syncSource": source}},
	}}
	if source != "" {
		var zone *leds.Zone
		var board string
		for _, c := range syncChoices(devs) {
			if c.ID != source {
				continue
			}
			board, _, _ = strings.Cut(source, "/")
			for _, d := range devs {
				if d.ID != board {
					continue
				}
				for i := range d.Zones {
					if board+"/"+d.Zones[i].ID == source {
						zone = &d.Zones[i]
					}
				}
			}
		}
		if zone == nil {
			return LightingView{}, lightErr("header %q not found", source)
		}
		fans := tlFanCount(*tl)
		if fans == 0 {
			return LightingView{}, lightErr("the number of fans is not known yet")
		}
		if zone.Max > 0 && 26*fans > zone.Max {
			return LightingView{}, lightErr("%s takes at most %d LEDs", zone.Name, zone.Max)
		}
		segs := syncSegments(fans)
		patch.Devices[board] = LightingDevicePatch{Zones: map[string]LightingZonePatch{zone.ID: {Segments: &segs}}}
	}
	return e.PatchLighting(patch)
}
