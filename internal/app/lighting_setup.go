package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// ARGB headers cannot report how many LEDs hang on them. This file helps the
// user to tell CaseHUD: an assistant that lights LEDs 1..N, a picker that adds
// up library parts, and ready-made setups (templates) for known hardware.
// Everything here is live only: no persistent write to any device.

const (
	lightAssistTimeout = 5 * time.Minute // a forgotten assistant ends by itself
	maxChainParts      = 16
)

//go:embed lighting_templates.json
var lightingTemplatesJSON []byte

// textMap holds a text per language ("de", "en").
type textMap map[string]string

// LightingTemplateZone is one zone of a template.
type LightingTemplateZone struct {
	Backend string  `json:"backend"`
	Zone    string  `json:"zone"`
	Label   textMap `json:"label,omitempty"`
	// Name is the name the zone gets (what the user would call it); none: unchanged.
	Name     textMap        `json:"name,omitempty"`
	Note     textMap        `json:"note,omitempty"`
	Count    int            `json:"count,omitempty"`
	Segments []leds.Segment `json:"segments,omitempty"`
}

// LightingTemplate is a setup for known hardware, stored as data.
type LightingTemplate struct {
	ID          string                 `json:"id"`
	Name        textMap                `json:"name"`
	Description textMap                `json:"description,omitempty"`
	Requires    []string               `json:"requires"` // backends that must be found
	Zones       []LightingTemplateZone `json:"zones"`
	// TLSync: the fan controller follows this mainboard zone ("backend/zone").
	TLSync string `json:"tlSync,omitempty"`
}

func loadLightingTemplates() []LightingTemplate {
	var f struct {
		Version   int                `json:"version"`
		Templates []LightingTemplate `json:"templates"`
	}
	if json.Unmarshal(lightingTemplatesJSON, &f) != nil || f.Version != 1 {
		return nil
	}
	return f.Templates
}

// ---------- LED count: set / chain ----------

// LightingPart is a library part that can be picked for a zone.
type LightingPart struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	LEDs  int     `json:"leds"`
	SizeX float64 `json:"sizeX,omitempty"` // mm
	SizeY float64 `json:"sizeY,omitempty"`
}

// LightingParts lists the parts with LEDs that are known to the LED module.
func (e *Engine) LightingParts() []LightingPart {
	e.loadLibraryComponents()
	out := []LightingPart{}
	for _, c := range leds.Components() {
		if len(c.LEDs) == 0 {
			continue
		}
		out = append(out, LightingPart{ID: c.Name, Label: c.Label, LEDs: len(c.LEDs), SizeX: c.SizeMM.X, SizeY: c.SizeMM.Y})
	}
	return out
}

// chainCount is the LED count of parts on one zone: the sum in chain order,
// or the longest part for parallel wiring (a splitter feeds the same signal
// to every branch).
func chainCount(counts []int, parallel bool) int {
	n := 0
	for _, c := range counts {
		if parallel {
			n = max(n, c)
		} else {
			n += c
		}
	}
	return n
}

// chainSegments lays parts out in a row on the canvas, in chain order, each
// part in its real proportion.
func chainSegments(parts []leds.Component) []leds.Segment {
	if len(parts) == 0 {
		return nil
	}
	cw, ch := float64(leds.CanvasWidth), float64(leds.CanvasHeight)
	const gap = 0.02
	// Sizes come from the library; out-of-range values fall back to the default
	// so a bad entry cannot bring Inf/NaN into the layout.
	size := func(c leds.Component) (float64, float64) {
		x, y := c.SizeMM.X, c.SizeMM.Y
		if !(x >= 1 && x <= libMaxSizeMM && y >= 1 && y <= libMaxSizeMM) {
			return 100, 100
		}
		return x, y
	}
	width := func(c leds.Component) float64 { w, _ := size(c); return w }
	height := func(c leds.Component) float64 { _, h := size(c); return h }
	sumW := 0.0
	for _, c := range parts {
		sumW += width(c)
	}
	scale := (0.9 - gap*float64(len(parts)-1)) / sumW // normalised x per mm
	for _, c := range parts {
		if height(c)*scale*cw/ch > 0.8 {
			scale = 0.8 / (height(c) * cw / ch)
		}
	}
	total := scale*sumW + gap*float64(len(parts)-1)
	x := (1 - total) / 2
	segs := make([]leds.Segment, 0, len(parts))
	for _, c := range parts {
		w, h := width(c)*scale, height(c)*scale*cw/ch
		h = max(h, 0.02)
		name := c.Label
		if len(name) > 60 {
			name = c.Name
		}
		segs = append(segs, leds.Segment{Name: name, Shape: "component", Component: c.Name, Count: len(c.LEDs),
			X: round3(x), Y: round3((1 - h) / 2), W: round3(w), H: round3(h)})
		x += w + gap
	}
	return segs
}

// zoneLine is a straight line of n LEDs; an existing single line keeps its
// position and direction.
func zoneLine(old []leds.Segment, name string, n int) []leds.Segment {
	if n <= 0 {
		return []leds.Segment{}
	}
	seg := leds.Segment{Name: name, Count: n, Shape: "line", X: 0.05, Y: 0.5, W: 0.9}
	if len(old) == 1 && (old[0].Shape == "line" || old[0].Shape == "") {
		seg = old[0]
		seg.Count = n
	}
	return []leds.Segment{seg}
}

func orMax(m int) int {
	if m <= 0 {
		return maxSegmentLEDs
	}
	return min(m, maxSegmentLEDs)
}

// LightingZoneSetup sets the LED count of a resizable zone. With parts it
// takes the count from library parts (see chainCount); without, from count.
// The assistant marker ends. GUI only.
func (e *Engine) LightingZoneSetup(device, zone string, count int, parts []string, parallel bool) (LightingView, error) {
	if !lightIDRe.MatchString(device) || !lightIDRe.MatchString(zone) {
		return LightingView{}, lightErr("invalid device or zone")
	}
	if len(parts) > maxChainParts {
		return LightingView{}, lightErr("at most %d parts", maxChainParts)
	}
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return LightingView{}, lightErr("lighting is off")
	}
	var z *leds.Zone
	for _, d := range eng.Devices() {
		if d.ID != device {
			continue
		}
		for i := range d.Zones {
			if d.Zones[i].ID == zone {
				z = &d.Zones[i]
			}
		}
	}
	if z == nil {
		return LightingView{}, lightErr("zone %q not found", zone)
	}
	if !z.Resizable {
		return LightingView{}, lightErr("the LED count of this zone is fixed")
	}
	cur := eng.Config().Devices[device].Zones[zone]

	var segs []leds.Segment
	if len(parts) == 0 {
		if count < 0 || count > orMax(z.Max) {
			return LightingView{}, lightErr("count must be 0..%d", orMax(z.Max))
		}
		segs = zoneLine(cur.Segments, z.Name, count)
	} else {
		e.loadLibraryComponents()
		comps := make([]leds.Component, 0, len(parts))
		counts := make([]int, 0, len(parts))
		for _, id := range parts {
			c, ok := leds.LookupComponent(id)
			if !ok || len(c.LEDs) == 0 {
				return LightingView{}, lightErr("unknown part %q", id)
			}
			comps = append(comps, c)
			counts = append(counts, len(c.LEDs))
		}
		n := chainCount(counts, parallel && len(parts) > 1)
		if n > orMax(z.Max) {
			return LightingView{}, lightErr("%s takes at most %d LEDs (the parts add up to %d)", z.Name, orMax(z.Max), n)
		}
		segs = chainSegments(comps)
		for i := range segs {
			if parallel && len(parts) > 1 {
				zero := 0
				segs[i].Start = &zero
			}
		}
	}
	e.LightingEndCalibration()
	return e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{
		device: {Zones: map[string]LightingZonePatch{zone: {Segments: &segs}}}}})
}

// LightingAssist shows the calibration marker on LED n of a zone for the
// assistant (live only, nothing is stored).
func (e *Engine) LightingAssist(device, zone string, n int) error {
	return e.lightingMarker(device, zone, LightingCalib{Device: device, Zone: zone, Pos: n, Range: true}, lightAssistTimeout,
		func(eng *leds.Engine) error {
			for _, d := range eng.Devices() {
				if d.ID != device {
					continue
				}
				for _, z := range d.Zones {
					if z.ID == zone && !z.Resizable {
						return fmt.Errorf("the LED count of this zone is fixed")
					}
				}
			}
			if n < 1 {
				return fmt.Errorf("count must be at least 1")
			}
			// The marker head (red) sits on LED n; the two LEDs before it are
			// blue. It only shows while LED n exists.
			return eng.Calibrate(device, zone, n-1)
		})
}

// ---------- templates ----------

// LightingTemplateView describes a template that fits the found devices.
type LightingTemplateView struct {
	ID          string                 `json:"id"`
	Name        textMap                `json:"name"`
	Description textMap                `json:"description,omitempty"`
	Zones       []LightingTemplateInfo `json:"zones"`
	TLSync      bool                   `json:"tlSync"` // the fan controller is set to follow the header
}

// LightingTemplateInfo is one zone a template will set.
type LightingTemplateInfo struct {
	Device string  `json:"device"`
	Zone   string  `json:"zone"`
	Label  textMap `json:"label,omitempty"`
	Note   textMap `json:"note,omitempty"`
	LEDs   int     `json:"leds"`
}

// LightingTemplateResult is the answer of applying a template.
type LightingTemplateResult struct {
	View    LightingView `json:"view"`
	Applied []string     `json:"applied"`
	Skipped []string     `json:"skipped"`
}

func templateSegmentLEDs(segs []leds.Segment) int {
	n, next := 0, 0
	for _, s := range segs {
		cnt := s.Count
		if c, ok := leds.LookupComponent(s.Component); s.Shape == "component" && ok && s.Count <= 0 {
			cnt = len(c.LEDs)
		}
		start := next
		if s.Start != nil {
			start = *s.Start
		}
		next = start + cnt
		n = max(n, next)
	}
	return n
}

// echoPoints returns the canvas positions of LEDs that a later segment
// drives again (parallel parts on a splitter): the first segment that covers
// an LED places it, the others only repeat it. Display only.
func echoPoints(segs []leds.Segment) [][2]float64 {
	var out [][2]float64
	var covered []bool
	next := 0
	for _, s := range segs {
		start := next
		if s.Start != nil && *s.Start >= 0 {
			start = *s.Start
		}
		one := s
		one.Start = nil
		pts := leds.Positions(leds.Zone{Resizable: true}, leds.ZoneConfig{Segments: []leds.Segment{one}})
		for i, p := range pts {
			k := start + i
			for len(covered) <= k {
				covered = append(covered, false)
			}
			if covered[k] {
				if len(out) < 256 {
					out = append(out, [2]float64{round3(p.X), round3(p.Y)})
				}
			}
			covered[k] = true
		}
		next = start + len(pts)
	}
	return out
}

func (z LightingTemplateZone) leds() int {
	if len(z.Segments) > 0 {
		return templateSegmentLEDs(z.Segments)
	}
	return z.Count
}

// zoneTarget finds the device of a backend that has the zone.
func zoneTarget(devs []leds.DeviceStatus, backend, zone string) (leds.DeviceStatus, leds.Zone, bool) {
	for _, d := range devs {
		if d.Backend != backend {
			continue
		}
		for _, z := range d.Zones {
			if z.ID == zone {
				return d, z, true
			}
		}
	}
	return leds.DeviceStatus{}, leds.Zone{}, false
}

func templateMatches(t LightingTemplate, devs []leds.DeviceStatus) bool {
	for _, b := range t.Requires {
		found := false
		for _, d := range devs {
			if d.Backend == b && !strings.HasPrefix(d.ID, unsupportedPref) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return len(t.Requires) > 0
}

// LightingTemplates lists the templates that fit the devices found now.
func (e *Engine) LightingTemplates() []LightingTemplateView {
	out := []LightingTemplateView{}
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return out
	}
	e.loadLibraryComponents()
	devs := eng.Devices()
	tlThere := false
	for _, d := range devs {
		tlThere = tlThere || hasSetting(d.Settings, "syncSource")
	}
	for _, t := range loadLightingTemplates() {
		if !templateMatches(t, devs) {
			continue
		}
		v := LightingTemplateView{ID: t.ID, Name: t.Name, Description: t.Description, Zones: []LightingTemplateInfo{}, TLSync: t.TLSync != "" && tlThere}
		for _, z := range t.Zones {
			d, dz, ok := zoneTarget(devs, z.Backend, z.Zone)
			if !ok || (!dz.Resizable && len(z.Segments) == 0) {
				continue
			}
			v.Zones = append(v.Zones, LightingTemplateInfo{Device: d.ID, Zone: z.Zone, Label: z.Label, Note: z.Note, LEDs: z.leds()})
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// ApplyLightingTemplate sets the zones of a template that exist on this
// system. Zones that are not there are reported, not an error. Parts that the
// library does not provide (yet) become plain lines with the same LED count.
// GUI only.
func (e *Engine) ApplyLightingTemplate(id, lang string) (LightingTemplateResult, error) {
	var tpl *LightingTemplate
	all := loadLightingTemplates()
	for i := range all {
		if all[i].ID == id {
			tpl = &all[i]
		}
	}
	if tpl == nil {
		return LightingTemplateResult{}, lightErr("unknown template")
	}
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return LightingTemplateResult{}, lightErr("lighting is off")
	}
	e.loadLibraryComponents()
	devs := eng.Devices()
	if !templateMatches(*tpl, devs) {
		return LightingTemplateResult{}, lightErr("the template does not fit the devices found")
	}
	e.LightingEndCalibration()
	res := LightingTemplateResult{Applied: []string{}, Skipped: []string{}}
	patch := LightingPatch{Devices: map[string]LightingDevicePatch{}}
	label := func(z LightingTemplateZone) string {
		if s := z.Label["de"]; s != "" {
			return s
		}
		return z.Backend + "/" + z.Zone
	}
	for _, z := range tpl.Zones {
		d, dz, ok := zoneTarget(devs, z.Backend, z.Zone)
		if !ok {
			res.Skipped = append(res.Skipped, label(z)+": Zone nicht gefunden")
			continue
		}
		if d.State != leds.StateOK {
			res.Skipped = append(res.Skipped, label(z)+": Gerät nicht aktiv")
			continue
		}
		var segs []leds.Segment
		switch {
		case len(z.Segments) > 0:
			segs = make([]leds.Segment, len(z.Segments))
			for i, s := range z.Segments {
				if _, known := leds.LookupComponent(s.Component); s.Shape == "component" && !known {
					s.Shape, s.Component = "line", ""
					res.Skipped = append(res.Skipped, label(z)+": Bauteil nicht geladen, als Linie angelegt (Library aktualisieren)")
				}
				segs[i] = s
			}
		case dz.Resizable:
			segs = zoneLine(nil, dz.Name, z.Count)
		default:
			continue // a fixed zone has its count already
		}
		if n := templateSegmentLEDs(segs); dz.Max > 0 && n > dz.Max {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: höchstens %d LEDs", label(z), dz.Max))
			continue
		}
		dp := patch.Devices[d.ID]
		if dp.Zones == nil {
			dp.Zones = map[string]LightingZonePatch{}
		}
		s := segs
		zp := LightingZonePatch{Segments: &s}
		if nm := z.Name[lang]; nm != "" {
			zp.Label = &nm
		} else if nm := z.Name["de"]; nm != "" {
			zp.Label = &nm
		}
		dp.Zones[z.Zone] = zp
		patch.Devices[d.ID] = dp
		res.Applied = append(res.Applied, label(z))
	}
	view := e.Lighting()
	if len(patch.Devices) > 0 {
		var err error
		if view, err = e.PatchLighting(patch); err != nil {
			return LightingTemplateResult{}, err
		}
	}
	// The fan controller follows the header (only when its backend runs).
	if tpl.TLSync != "" {
		backend, zone, _ := strings.Cut(tpl.TLSync, "/")
		if board, _, ok := zoneTarget(devs, backend, zone); ok {
			for _, d := range devs {
				if !hasSetting(d.Settings, "syncSource") {
					continue
				}
				src := board.ID + "/" + zone
				v, err := e.LightingSetSync(d.ID, src)
				if err != nil {
					// Fan count not known yet: remember the header, keep our layout.
					v, err = e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{
						d.ID: {Settings: map[string]any{"syncSource": src}}}})
				}
				if err != nil {
					res.Skipped = append(res.Skipped, "Lüfter-Controller: "+err.Error())
				} else {
					view = v
					res.Applied = append(res.Applied, "Lüfter-Controller folgt "+zone)
				}
			}
		}
	}
	res.View = view
	return res, nil
}
