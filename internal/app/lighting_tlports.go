package app

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// Per-port firmware effects of the TL fan controller ("Eigener Effekt").
//
// In the hardware mode every controller port runs a firmware effect of its
// own. The user can override effect and direction per port (settings
// port<N>Effect, port<N>Direction). With the setting "follow" CaseHUD derives
// them: the colours and the speed from the active CaseHUD effect, the firmware
// effect where one obviously matches, and the direction of every port from
// the case sketch (which fans hang on the port, how the effect runs over
// them; sketch.TLDirection). A manual override always wins.
//
// The derived values are an overlay of the LED engine (SetDeviceOverlay):
// they reach the controller's Configure but are never stored in config.json,
// and the backend writes its flash only when the resulting light of a port
// really changes. They are recomputed whenever the sketch, the effect, its
// parameters, the device settings or the found fans change (syncTLPorts runs
// on every lighting view, after every change and every 2 s; it does nothing
// while its inputs are unchanged).

// LightingPortView is one port of the TL controller in the hardware mode:
// what it runs and why.
type LightingPortView struct {
	Port      int      `json:"port"`             // 1-based
	Parts     []string `json:"parts,omitempty"`  // sketch placements on the port, in chain order
	Labels    []string `json:"labels,omitempty"` // their labels
	Effect    string   `json:"effect"`           // firmware effect in use
	Direction string   `json:"direction"`        // firmware direction in use
	// Source of the direction: "sketch" (derived), "manual" (port
	// override) or "all" (the device-wide setting); EffectSource the same
	// for the effect.
	Source       string `json:"source"`
	EffectSource string `json:"effectSource"`
	// Whole: the CaseHUD effect does not run across these fans in any
	// firmware direction; they light as a whole (breathing).
	Whole bool `json:"whole,omitempty"`
}

// tlFirmwareEffect maps CaseHUD effects to the firmware effect that obviously
// matches; other effects keep the firmware effect the user chose.
var tlFirmwareEffect = map[string]string{
	"rainbow": "rainbow", "solid": "static", "breathing": "breathing", "wave": "wave",
	"meteor": "meteor", "orbit": "meteor", "cycle": "color-cycle", "stack": "stack",
}

const (
	tlMaxColors   = 4
	tlMaxSpeed    = 4
	tlWholeEffect = "breathing"
)

var tlPortKey = regexp.MustCompile(`^port(\d+)(Effect|Direction)$`)

// dropStalePortSettings removes per-port settings of ports the device does
// not report (any more): a fan chain that was unplugged must not make the
// stored settings invalid.
func dropStalePortSettings(decl []leds.Param, p leds.Params) leds.Params {
	var stale []string
	for k := range p {
		if tlPortKey.MatchString(k) && !hasSetting(decl, k) {
			stale = append(stale, k)
		}
	}
	if len(stale) == 0 {
		return p
	}
	out := make(leds.Params, len(p))
	for k, v := range p {
		out[k] = v
	}
	for _, k := range stale {
		delete(out, k)
	}
	return out
}

// tlPortState is the cache of syncTLPorts.
type tlPortState struct {
	mu      sync.Mutex // one computation at a time; never held by the callers of Lighting
	sig     string
	views   map[string][]LightingPortView // device id -> ports (guarded by lightingState.mu)
	overlay map[string]bool               // devices that got an overlay
}

// syncTLPorts recomputes the per-port plan of every TL controller when its
// inputs changed and hands the derived settings to the engine. No lock of the
// caller may be held (it reads the configuration, the sketch and the fan list).
func (e *Engine) syncTLPorts() {
	l := &e.light
	st := &l.tlPorts
	st.mu.Lock()
	defer st.mu.Unlock()
	eng := l.engPtr.Load()
	if eng == nil {
		if st.sig != "" {
			st.sig = ""
			l.mu.Lock()
			st.views = nil
			l.mu.Unlock()
		}
		return
	}
	cfg := eng.Config()
	var tl []leds.DeviceStatus
	for _, d := range eng.Devices() {
		if d.Backend == "lianlitl" && hasSetting(d.Settings, "follow") {
			tl = append(tl, d)
		}
	}
	e.mu.Lock()
	skPtr := e.cfg.Lighting.Sketch // replaced as a whole on change, never modified in place
	e.mu.Unlock()
	var fans []sketch.DetectedFan
	if len(tl) > 0 {
		for _, p := range e.arrangement(false).Ports {
			for _, f := range p.Fans {
				fans = append(fans, sketch.DetectedFan{Port: f.Port, Index: f.Index, LCD: f.HasLCD})
			}
		}
	}
	var sig strings.Builder
	fmt.Fprintf(&sig, "%p|%s|%v|%v|%v|", skPtr, cfg.Effect, cfg.Params[cfg.Effect], eng.Accent(), fans)
	for _, d := range tl {
		fmt.Fprintf(&sig, "%s|%v|", d.ID, cfg.Devices[d.ID].Settings)
		for _, z := range d.Zones {
			fmt.Fprintf(&sig, "%s:%d,", z.ID, z.Count)
		}
	}
	if sig.String() == st.sig {
		return
	}

	var sk *sketch.Sketch
	var cat sketch.Catalog
	if skPtr != nil && len(tl) > 0 {
		sk, _ = e.ResolvedSketch()
		cat = e.SketchCatalog()
	}
	var eparams leds.Params
	if info, ok := lightingEffect(cfg.Effect); ok {
		eparams = paramsWithDefaults(info.Params, cfg.Params[cfg.Effect])
	}
	views := map[string][]LightingPortView{}
	had := st.overlay
	st.overlay = map[string]bool{}
	for _, d := range tl {
		values := paramsWithDefaults(d.Settings, dropStalePortSettings(d.Settings, cfg.Devices[d.ID].Settings))
		var layout []sketch.TLPortFans
		if sk != nil {
			dd := sketch.DetectedDevice{ID: d.ID, Backend: d.Backend, Mode: values.String("mode")}
			for _, z := range d.Zones {
				dd.Zones = append(dd.Zones, sketch.DetectedZone{ID: z.ID, Count: z.Count, Max: z.Max})
			}
			layout = sketch.TLPortLayout(*sk, cat, dd, fans)
		}
		v, ov := tlPortPlan(d, values, cfg.Effect, eparams, eng.Accent(), layout)
		views[d.ID] = v
		eng.SetDeviceOverlay(d.ID, ov)
		if len(ov) > 0 {
			st.overlay[d.ID] = true
		}
	}
	for id := range had {
		if !st.overlay[id] {
			eng.SetDeviceOverlay(id, nil)
		}
	}
	st.sig = sig.String()
	l.mu.Lock()
	st.views = views
	l.mu.Unlock()
}

// tlPortViewsLocked returns a copy of the port plan of a device; l.mu is held.
func (l *lightingState) tlPortViewsLocked(id string) []LightingPortView {
	v := l.tlPorts.views[id]
	if len(v) == 0 {
		return nil
	}
	return append([]LightingPortView(nil), v...)
}

// fillTLPorts puts the current port plans into a view built before the last
// syncTLPorts.
func (e *Engine) fillTLPorts(v *LightingView) {
	l := &e.light
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range v.Devices {
		v.Devices[i].Ports = l.tlPortViewsLocked(v.Devices[i].ID)
	}
}

// tlPortPlan works out what every port of one controller runs (values: its
// settings with defaults) and the settings derived from CaseHUD (nil when
// "follow" is off). Only the hardware mode without merge has per-port
// effects.
func tlPortPlan(d leds.DeviceStatus, values leds.Params, effect string, eparams leds.Params, accent color.RGBA, layout []sketch.TLPortFans) ([]LightingPortView, leds.Params) {
	if values.String("mode") != "hardware" || values.Bool("merge") {
		return nil, nil
	}
	follow := values.Bool("follow")
	baseEffect, baseDir := values.String("effect"), values.String("direction")
	ov := leds.Params{}
	var u func(x, y float64) float64
	var circular, motion bool
	if follow {
		if cols := effectColors(eparams, accent); len(cols) > 0 {
			for i, c := range cols {
				ov[fmt.Sprintf("color%d", i+1)] = c
			}
			ov["colors"] = float64(len(cols))
		}
		if fw, ok := tlFirmwareEffect[effect]; ok {
			ov["effect"], baseEffect = fw, fw
		}
		if sp, ok := eparams["speed"].(float64); ok {
			ov["speed"] = math.Round(math.Max(0, math.Min(100, sp)) / 100 * tlMaxSpeed)
		}
		u, circular, motion = leds.MotionField(effect, eparams)
	}
	byPort := map[int]sketch.TLPortFans{}
	for _, p := range layout {
		byPort[p.Port] = p
	}
	var ports []int
	for _, z := range d.Zones {
		var n int
		if _, err := fmt.Sscanf(z.ID, "port%d", &n); err == nil && n > 0 && hasSetting(d.Settings, fmt.Sprintf("port%dEffect", n)) {
			ports = append(ports, n)
		}
	}
	sort.Ints(ports)
	views := make([]LightingPortView, 0, len(ports))
	for _, n := range ports {
		ek, dk := fmt.Sprintf("port%dEffect", n), fmt.Sprintf("port%dDirection", n)
		pf := byPort[n]
		v := LightingPortView{Port: n, Parts: pf.Parts, Labels: pf.Labels, Effect: baseEffect, Direction: baseDir, Source: "all", EffectSource: "all"}
		if motion && len(pf.Fans) > 0 {
			dir, _, whole := sketch.TLDirection(pf.Fans, u, circular)
			if whole {
				v.Effect, v.EffectSource, v.Whole = tlWholeEffect, "sketch", true
				ov[ek] = tlWholeEffect
			} else {
				v.Direction, v.Source = dir, "sketch"
				ov[dk] = dir
			}
		}
		if m := values.String(ek); m != "" {
			v.Effect, v.EffectSource, v.Whole = m, "manual", false
			delete(ov, ek)
		}
		if m := values.String(dk); m != "" {
			v.Direction, v.Source = m, "manual"
			delete(ov, dk)
		}
		views = append(views, v)
	}
	if len(ov) == 0 {
		ov = nil
	}
	return views, ov
}

// effectColors are the colours of a CaseHUD effect for the controller (up to
// four, "#rrggbb"): the palette color1..colorN (as many as "colors" says), or
// the single colours (color, low, high, tip, background unless it is black).
// The design accent stands in for "color" when the effect uses it. Effects
// without colours of their own (rainbow, color cycle) give none.
func effectColors(p leds.Params, accent color.RGBA) []string {
	hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	var out []string
	if _, ok := p["color1"]; ok {
		n := 0
		for i := 1; ; i++ {
			if _, ok := p[fmt.Sprintf("color%d", i)]; !ok {
				break
			}
			n = i
		}
		if c, ok := p["colors"].(float64); ok {
			n = max(1, min(n, int(c+0.5)))
		}
		for i := 1; i <= n && len(out) < tlMaxColors; i++ {
			out = append(out, hex(p.Color(fmt.Sprintf("color%d", i))))
		}
		return out
	}
	for _, k := range []string{"color", "low", "high", "tip", "background"} {
		if _, ok := p[k]; !ok || len(out) == tlMaxColors {
			continue
		}
		c := p.Color(k)
		if k == "color" && p.Bool("useAccent") {
			c = accent
		}
		if k == "background" && c.R|c.G|c.B == 0 {
			continue
		}
		out = append(out, hex(c))
	}
	return out
}
