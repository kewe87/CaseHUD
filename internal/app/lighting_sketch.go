package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// The case sketch (package sketch) is stored in config.json next to the LED
// engine configuration. Everything the engine needs is derived from it: one
// component "sketch.<placementID>" per placement (registered with the LED
// module) and, for every bound (device, zone), the segments of the chain of
// parts. The derived data is never stored on its own: the components are
// rebuilt at every start (before the stored layout is cleaned) and a zone's
// segments are rewritten whenever the sketch changes.
//
// The "sketch." namespace belongs to the sketch: segments that name such a
// component are replaced when the sketch changes and removed when the
// sketch (or the placement) goes away; the zone then shows its default
// layout again. Zones that are not bound by the sketch are left alone.

// SketchView is the sketch together with the lighting state after a change.
type SketchView struct {
	Sketch *sketch.Sketch `json:"sketch"`
	// Net is the layout of the unfolded case the LED canvas shows (nil
	// without a sketch).
	Net      *sketch.Net  `json:"net,omitempty"`
	Lighting LightingView `json:"lighting"`
}

// ResolvedSketch returns a copy of the stored sketch with the face of every
// part filled in and the lighting of its case brought in line with the case
// template (SyncCaseLights; stored with the next change), and the case net
// (nil, nil: no sketch).
func (e *Engine) ResolvedSketch() (*sketch.Sketch, *sketch.Net) {
	sk := e.GetSketch()
	if sk == nil {
		return nil, nil
	}
	cat := e.SketchCatalog()
	r := sketch.ResolveFaces(sketch.SyncCaseLights(*sk, cat), cat)
	_, n := sketch.SketchNet(r, cat)
	return &r, &n
}

func cloneSketch(s *sketch.Sketch) *sketch.Sketch {
	if s == nil {
		return nil
	}
	out := *s
	out.Case.Chambers = append([]sketch.Chamber(nil), s.Case.Chambers...)
	out.Parts = make([]sketch.Placement, len(s.Parts))
	for i, p := range s.Parts {
		if p.Bind != nil {
			b := *p.Bind
			p.Bind = &b
		}
		p.Path = append([]sketch.Vec3(nil), p.Path...)
		out.Parts[i] = p
	}
	return &out
}

// demoFansID is the demo fan controller (lighting backend "lianlitl").
const demoFansID = "demo:fans"

// sketchRings tells the sketch which zones take one colour per fan ring: the
// ports of the TL fan controller. It goes by the device id, so the stored
// sketch is derived the same way at start, before any device is found.
func sketchRings(device, zone string) bool {
	return (strings.HasPrefix(device, "lianlitl:") || device == demoFansID) && sketch.RingZone("lianlitl", zone)
}

// GetSketch returns a copy of the stored sketch (nil: none).
func (e *Engine) GetSketch() *sketch.Sketch {
	e.mu.Lock()
	defer e.mu.Unlock()
	return cloneSketch(e.cfg.Lighting.Sketch)
}

// SetSketch validates the sketch against the catalog, stores it and rewrites
// the segments of the zones it binds. A rejected sketch changes nothing.
// Bindings must name a device and zone the running engine knows; while
// lighting is off the devices are not known and any well-formed id is kept.
func (e *Engine) SetSketch(s sketch.Sketch) (SketchView, error) {
	l := &e.light
	l.sketchMu.Lock()
	defer l.sketchMu.Unlock()
	return e.setSketchLocked(s)
}

// setSketchLocked is SetSketch with l.sketchMu held.
func (e *Engine) setSketchLocked(s sketch.Sketch) (SketchView, error) {
	cat := e.SketchCatalog()
	s = sketch.SyncCaseLights(s, cat) // the case decides where its lighting runs
	d, err := sketch.DeriveWith(s, cat, sketchRings)
	if err != nil {
		return SketchView{}, lightErr("sketch: %v", err)
	}
	s = sketch.ResolveFaces(s, cat) // the face of every part is stored explicitly
	if err := e.applySketch(&s, d); err != nil {
		return SketchView{}, err
	}
	sk, n := e.ResolvedSketch()
	return SketchView{Sketch: sk, Net: n, Lighting: e.Lighting()}, nil
}

// DeleteSketch removes the sketch, its components and the segments derived
// from it. It is not an error if there is none.
func (e *Engine) DeleteSketch() (SketchView, error) {
	l := &e.light
	l.sketchMu.Lock()
	defer l.sketchMu.Unlock()
	if err := e.applySketch(nil, sketch.Derived{}); err != nil {
		return SketchView{}, err
	}
	return SketchView{Lighting: e.Lighting()}, nil
}

// upgradeStoredSketch derives the sketch from the stored version 1 backup
// (if there is one) with the current catalog, so library parts are placed by
// their real size. The backup stays until the editor saves, so a catalog that
// is incomplete at one start does not fix an approximate result for good.
func (e *Engine) upgradeStoredSketch() {
	e.mu.Lock()
	raw := e.cfg.Lighting.SketchV1
	e.mu.Unlock()
	if len(raw) == 0 {
		return
	}
	var old sketch.SketchV1
	if err := json.Unmarshal(raw, &old); err != nil {
		log.Printf("lighting: stored version 1 sketch not readable: %v", err)
		return
	}
	cat := e.SketchCatalog()
	up, notes, err := sketch.Upgrade(old, cat)
	if err != nil {
		log.Printf("lighting: stored version 1 sketch not upgraded: %v", err)
		return
	}
	for _, n := range notes {
		log.Printf("lighting: sketch upgrade: %s: %s", n.ID, n.Msg)
	}
	up = sketch.ResolveFaces(up, cat)
	e.mu.Lock()
	e.cfg.Lighting.Sketch = &up
	e.mu.Unlock()
}

func sketchComponentNames() map[string]leds.Component {
	out := map[string]leds.Component{}
	for _, c := range leds.Components() {
		if strings.HasPrefix(c.Name, sketch.ReservedPrefix) {
			out[c.Name] = c
		}
	}
	return out
}

// registerSketchComponents adds or replaces the given components. A
// component a layout already names is replaced in one step, so a layout
// rebuild sees either the old or the new one (it resolves each component
// with a single lookup).
func registerSketchComponents(want []leds.Component) error {
	for _, c := range want {
		if err := leds.RegisterComponent(c); err != nil {
			return err
		}
	}
	return nil
}

// unregisterStaleSketchComponents removes every registered "sketch."
// component that is not in want. Layouts must no longer name them: callers
// do this after the configuration without their segments is in place.
func unregisterStaleSketchComponents(want []leds.Component) {
	keep := make(map[string]bool, len(want))
	for _, c := range want {
		keep[c.Name] = true
	}
	for name := range sketchComponentNames() {
		if !keep[name] {
			leds.UnregisterComponent(name)
		}
	}
}

// placeholderComponent stands in for a placement that cannot be derived any
// more (its part left the library): same name, as many LEDs as the placement
// asks for (at least one) stacked in one point. It keeps the stored segments
// of the placement alive, so neither the other placements nor the stored
// layout change because the catalog is incomplete. The next PUT of the
// sketch replaces it.
func placeholderComponent(p sketch.Placement) (leds.Component, bool) {
	if !sketch.PlacementIDRe.MatchString(p.ID) {
		return leds.Component{}, false
	}
	label := p.Label
	if label == "" {
		label = p.ID
	}
	c := leds.Component{Name: sketch.ComponentName(p.ID), Label: label, Width: 1, Height: 1,
		SizeMM: leds.Point{X: 1, Y: 1}, Outline: "rect", PositionsMM: true}
	for i := 0; i < min(max(p.LEDs, 1), sketch.MaxLEDs); i++ {
		c.LEDs = append(c.LEDs, leds.Point{X: 0.5, Y: 0.5})
	}
	return c, true
}

// restoreSketchComponents registers the components of the stored sketch. It
// runs at start, before the stored layout is cleaned, so that zones keep
// their component segments. Placements that no longer validate (a library
// part was removed, the cache is incomplete) are skipped one by one: the
// others are derived as usual, and the skipped ones get a placeholder so
// their stored segments stay in the layout and nothing is stripped from the
// stored configuration because of a catalog failure.
func (e *Engine) restoreSketchComponents() {
	sk := e.GetSketch()
	if sk == nil {
		return
	}
	cat := e.SketchCatalog()
	usable, skipped := sketch.Usable(*sk, cat)
	var comps []leds.Component
	if len(usable.Parts) > 0 {
		d, err := sketch.DeriveWith(usable, cat, sketchRings)
		if err != nil {
			log.Printf("lighting: stored sketch not used: %v", err)
			skipped = skipped[:0]
			for i := range sk.Parts {
				skipped = append(skipped, i)
			}
		} else {
			comps = d.Components
		}
	}
	have := make(map[string]bool, len(comps))
	for _, c := range comps {
		have[c.Name] = true
	}
	for _, i := range skipped {
		if c, ok := placeholderComponent(sk.Parts[i]); ok && !have[c.Name] {
			have[c.Name] = true
			comps = append(comps, c)
		}
	}
	if len(skipped) > 0 {
		log.Printf("lighting: %d placements of the stored sketch cannot be derived (part missing?), kept as placeholders", len(skipped))
	}
	unregisterStaleSketchComponents(comps)
	if err := registerSketchComponents(comps); err != nil {
		log.Printf("lighting: sketch components not registered: %v", err)
	}
}

// dropDanglingSketch removes segments that name a "sketch." component that is
// not registered (stored configuration only): without the component the
// whole lighting configuration would be refused.
func dropDanglingSketch(in leds.Config) leds.Config {
	return rewriteSketchSegments(in, func(s leds.Segment) bool {
		if s.Shape != "component" || !strings.HasPrefix(s.Component, sketch.ReservedPrefix) {
			return true
		}
		_, ok := leds.LookupComponent(s.Component)
		return ok
	}, nil)
}

// rewriteSketchSegments returns a copy of the configuration in which the
// segments rejected by keep are removed (a zone left without segments gets
// nil) and, for (device, zone) pairs in set, the segments are replaced.
func rewriteSketchSegments(in leds.Config, keep func(leds.Segment) bool, set map[[2]string][]leds.Segment) leds.Config {
	out := cloneLightCfg(in)
	for id, dc := range out.Devices {
		for zid, zc := range dc.Zones {
			var kept []leds.Segment
			for _, s := range zc.Segments {
				if keep(s) {
					kept = append(kept, s)
				}
			}
			if len(kept) != len(zc.Segments) {
				zc.Segments = kept
				dc.Zones[zid] = zc
			}
		}
		out.Devices[id] = dc
	}
	for key, segs := range set {
		dc := out.Devices[key[0]]
		if dc.Zones == nil {
			dc.Zones = map[string]leds.ZoneConfig{}
		}
		zc := dc.Zones[key[1]]
		zc.Segments = append([]leds.Segment(nil), segs...)
		dc.Zones[key[1]] = zc
		out.Devices[key[0]] = dc
	}
	return out
}

// ErrPersist marks a change that is active but could not be written to
// config.json (an I/O failure, not bad input).
var ErrPersist = errors.New("configuration could not be saved")

// applySketch stores sk (nil: none) and its derived data. The caller holds
// sketchMu. Everything is checked before anything is changed; on an error the
// previous components are restored.
//
// Order matters because the LED engine rebuilds layouts at any time: first
// the new components are registered (a replaced one is swapped in one step),
// then the configuration is swapped, and only then are the components that
// nothing references any more removed. A rebuild therefore always finds the
// components its configuration names.
func (e *Engine) applySketch(sk *sketch.Sketch, d sketch.Derived) error {
	l := &e.light
	prev := sketchComponentNames()
	restore := func() {
		for _, c := range d.Components {
			if _, was := prev[c.Name]; !was {
				leds.UnregisterComponent(c.Name)
			}
		}
		for _, c := range prev {
			_ = leds.RegisterComponent(c)
		}
	}
	if err := registerSketchComponents(d.Components); err != nil {
		restore()
		return lightErr("sketch: %v", err)
	}

	l.lifeMu.RLock()
	l.mu.Lock()
	cur := l.cfg
	var devs []leds.DeviceStatus
	if l.eng != nil {
		cur = l.eng.Config()
		devs = l.eng.Devices()
	}
	fail := func(err error) error {
		l.mu.Unlock()
		l.lifeMu.RUnlock()
		restore()
		return err
	}
	zonesOf := map[string]leds.DeviceStatus{}
	for _, dev := range devs {
		zonesOf[dev.ID] = dev
	}
	set := map[[2]string][]leds.Segment{}
	for _, z := range d.Zones {
		if l.eng != nil {
			ds, ok := zonesOf[z.Device]
			if !ok {
				return fail(lightErr("sketch: device %q not found", z.Device))
			}
			var zone *leds.Zone
			for i := range ds.Zones {
				if ds.Zones[i].ID == z.Zone {
					zone = &ds.Zones[i]
				}
			}
			if zone == nil {
				return fail(lightErr("sketch: %s: zone %q not found", z.Device, z.Zone))
			}
			if zone.Max > 0 && z.LEDs > zone.Max {
				return fail(lightErr("sketch: %s/%s: at most %d LEDs", z.Device, z.Zone, zone.Max))
			}
		} else if !lightIDRe.MatchString(z.Device) || !(lightKeyRe.MatchString(z.Zone) || lightIDRe.MatchString(z.Zone)) {
			return fail(lightErr("sketch: invalid device or zone id"))
		}
		set[[2]string{z.Device, z.Zone}] = z.Segments
	}
	next := rewriteSketchSegments(cur, func(s leds.Segment) bool {
		return !(s.Shape == "component" && strings.HasPrefix(s.Component, sketch.ReservedPrefix))
	}, set)
	descr := map[string][]leds.Param{}
	for _, dev := range devs {
		if len(dev.Settings) > 0 {
			descr[dev.ID] = dev.Settings
		}
	}
	clean, err := cleanLightingConfig(next, descr)
	if err != nil {
		return fail(lightErr("sketch: %v", err))
	}
	if l.eng != nil {
		running := l.eng.Config().Devices
		for id, dc := range clean.Devices {
			if cur, ok := running[id]; ok && reflect.DeepEqual(cur, dc) {
				continue // a device is only touched when its configuration changed
			}
			l.eng.SetDeviceConfig(id, dc)
		}
		clean = l.eng.Config()
	}
	l.cfg = clean
	e.mu.Lock()
	e.cfg.Lighting.Sketch = cloneSketch(sk)
	e.cfg.Lighting.SketchV1 = nil // the editor has taken over: no more backup
	e.mu.Unlock()
	l.markDirty()
	l.mu.Unlock()
	l.lifeMu.RUnlock()
	unregisterStaleSketchComponents(d.Components)
	// the sketch must survive a crash, not only a clean stop
	if err := e.saveLightingErr(); err != nil {
		return fmt.Errorf("%w: %w", ErrPersist, err)
	}
	return nil
}
