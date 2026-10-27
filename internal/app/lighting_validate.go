package app

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Limits for everything the lighting configuration takes from the GUI, the
// API or a hand-edited config.json.
const (
	maxLightDevices    = 32
	maxLightZones      = 16
	maxLightSegments   = 16
	maxSegmentLEDs     = 1024 // LEDs in one segment
	maxZoneLEDs        = 2048 // LEDs in one zone
	maxLightText       = 260
	maxLightSettingStr = 64
	maxLightAngle      = 3600
	minLightFPS        = 5
	maxLightFPS        = 60
)

var (
	lightIDRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,63}$`)
	lightKeyRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
	segShapes  = map[string]bool{"": true, "line": true, "ring": true, "rect": true, "point": true, "component": true}
)

// lightingEffects are the effects offered; the media effects that read files,
// the screen or the audio are not part of this build.
var lightingEffects = leds.BuiltinEffects()

func lightingEffect(name string) (leds.EffectInfo, bool) {
	for _, info := range lightingEffects {
		if info.Name == name {
			return info, true
		}
	}
	return leds.EffectInfo{}, false
}

func printable(s string, max int) bool {
	if len(s) > max {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// cleanParams checks values against the descriptors of an effect or device:
// numbers are clamped to their range, colours normalised, enums and texts
// checked. Unknown keys and wrong types are errors.
func cleanParams(decl []leds.Param, in leds.Params) (leds.Params, error) {
	out := leds.Params{}
	byKey := make(map[string]leds.Param, len(decl))
	for _, p := range decl {
		byKey[p.Key] = p
	}
	for k, v := range in {
		d, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("unknown parameter %q", k)
		}
		switch d.Type {
		case leds.ParamNumber:
			f, ok := v.(float64)
			if !ok {
				if i, isInt := v.(int); isInt {
					f, ok = float64(i), true
				}
			}
			if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, fmt.Errorf("parameter %q must be a number", k)
			}
			if d.Max > d.Min {
				f = math.Max(d.Min, math.Min(d.Max, f))
			} else {
				f = math.Max(-1e6, math.Min(1e6, f))
			}
			out[k] = f
		case leds.ParamBool:
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("parameter %q must be true or false", k)
			}
			out[k] = b
		case leds.ParamColor:
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %q must be a colour like #3a8dff", k)
			}
			c, err := leds.ParseHexColor(s)
			if err != nil || !strings.HasPrefix(s, "#") {
				return nil, fmt.Errorf("parameter %q must be a colour like #3a8dff", k)
			}
			out[k] = fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
		case leds.ParamEnum:
			s, ok := v.(string)
			if !ok || !printable(s, 200) {
				return nil, fmt.Errorf("parameter %q must be text", k)
			}
			if len(d.Values) > 0 {
				found := false
				for _, x := range d.Values {
					found = found || x == s
				}
				if !found {
					return nil, fmt.Errorf("parameter %q: %q is not one of %s", k, s, strings.Join(d.Values, ", "))
				}
			}
			out[k] = s
		case leds.ParamText:
			s, ok := v.(string)
			if !ok || !printable(s, maxLightText) {
				return nil, fmt.Errorf("parameter %q must be short text", k)
			}
			out[k] = s
		default:
			return nil, fmt.Errorf("parameter %q has an unsupported type", k)
		}
	}
	return out, nil
}

// defaultCanvasInterval is the seconds between canvas updates per port of a
// TL controller (every update is stored in the controller's flash).
const defaultCanvasInterval = 60.0

// migrateLightSettings upgrades stored device settings: the former TL setting
// "rate" (updates per second) is replaced by "interval" (seconds, default 60).
func migrateLightSettings(in leds.Params) leds.Params {
	if _, ok := in["rate"]; !ok {
		return in
	}
	out := make(leds.Params, len(in))
	for k, v := range in {
		if k != "rate" {
			out[k] = v
		}
	}
	if _, ok := out["interval"]; !ok {
		out["interval"] = defaultCanvasInterval
	}
	return out
}

// cleanLooseSettings checks settings of a device that is not connected right
// now (no descriptors): only plain values with short names.
func cleanLooseSettings(in leds.Params) (leds.Params, error) {
	out := leds.Params{}
	for k, v := range in {
		if !lightKeyRe.MatchString(k) {
			return nil, fmt.Errorf("invalid setting name %q", k)
		}
		switch x := v.(type) {
		case bool:
			out[k] = x
		case float64:
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, fmt.Errorf("setting %q is not a number", k)
			}
			out[k] = x
		case string:
			if !printable(x, maxLightSettingStr) {
				return nil, fmt.Errorf("setting %q is too long or has control characters", k)
			}
			out[k] = x
		default:
			return nil, fmt.Errorf("setting %q has an unsupported value", k)
		}
	}
	return out, nil
}

func finite01(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return false
		}
	}
	return true
}

// cleanSegments checks a zone layout: shapes from the list, components that
// exist, geometry inside the canvas, LED counts bounded.
func cleanSegments(segs []leds.Segment) ([]leds.Segment, error) {
	if len(segs) > maxLightSegments {
		return nil, fmt.Errorf("at most %d segments per zone", maxLightSegments)
	}
	total, next := 0, 0
	out := make([]leds.Segment, 0, len(segs))
	for i, s := range segs {
		if !segShapes[s.Shape] {
			return nil, fmt.Errorf("segment %d: unknown shape %q", i+1, s.Shape)
		}
		if !printable(s.Name, 64) {
			return nil, fmt.Errorf("segment %d: invalid name", i+1)
		}
		if s.Shape == "component" {
			if _, ok := leds.LookupComponent(s.Component); !ok {
				return nil, fmt.Errorf("segment %d: unknown component %q", i+1, s.Component)
			}
		} else if s.Component != "" {
			return nil, fmt.Errorf("segment %d: component only with shape component", i+1)
		}
		if s.Count < 0 || s.Count > maxSegmentLEDs {
			return nil, fmt.Errorf("segment %d: count must be 0..%d", i+1, maxSegmentLEDs)
		}
		if s.Shape != "component" && s.Count == 0 {
			return nil, fmt.Errorf("segment %d: count must be at least 1", i+1)
		}
		if !finite01(s.X, s.Y, s.W, s.H) {
			return nil, fmt.Errorf("segment %d: x, y, w and h must be between 0 and 1", i+1)
		}
		if math.IsNaN(s.Angle) || math.IsInf(s.Angle, 0) || math.Abs(s.Angle) > maxLightAngle {
			return nil, fmt.Errorf("segment %d: invalid angle", i+1)
		}
		n := s.Count
		if s.Shape == "component" {
			c, _ := leds.LookupComponent(s.Component)
			if n <= 0 || n > len(c.LEDs) {
				n = len(c.LEDs)
			}
		}
		start := next
		if s.Start != nil {
			if *s.Start < 0 || *s.Start > maxZoneLEDs {
				return nil, fmt.Errorf("segment %d: invalid start", i+1)
			}
			start = *s.Start
		}
		next = start + n
		total = max(total, next)
		if total > maxZoneLEDs {
			return nil, fmt.Errorf("at most %d LEDs per zone", maxZoneLEDs)
		}
		out = append(out, s)
	}
	return out, nil
}

// maxLabelRunes is the longest name a zone can be given.
const maxLabelRunes = 40

// cleanLabel checks a zone name: printable text, no control characters,
// at most maxLabelRunes characters. Surrounding blanks go.
func cleanLabel(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxLabelRunes {
		return "", fmt.Errorf("the name may have at most %d characters", maxLabelRunes)
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return "", fmt.Errorf("the name has characters that cannot be shown")
		}
	}
	return s, nil
}

func copyLabels(in map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for d, zs := range in {
		out[d] = map[string]string{}
		for z, l := range zs {
			out[d][z] = l
		}
	}
	return out
}

func countLabels(m map[string]map[string]string) int {
	n := 0
	for _, zs := range m {
		n += len(zs)
	}
	return n
}

// cleanStoredLabels drops stored names that do not pass the check (config.json is edited by hand).
func cleanStoredLabels(in map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for d, zs := range in {
		if !lightIDRe.MatchString(d) {
			continue
		}
		for z, l := range zs {
			c, err := cleanLabel(l)
			if err != nil || c == "" || !(lightKeyRe.MatchString(z) || lightIDRe.MatchString(z)) || countLabels(out) >= maxLightDevices*maxLightZones {
				continue
			}
			if out[d] == nil {
				out[d] = map[string]string{}
			}
			out[d][z] = c
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanOrder(o string) (string, error) {
	if o == "" {
		return "", nil
	}
	o = strings.ToUpper(o)
	if len(o) != 3 || strings.Count(o, "R") != 1 || strings.Count(o, "G") != 1 || strings.Count(o, "B") != 1 {
		return "", fmt.Errorf("colour order must be a permutation of RGB, e.g. GRB")
	}
	return o, nil
}

// cleanLightingConfig validates a whole engine configuration. devSettings
// holds the setting descriptors of the devices that are connected right now;
// other devices only get plain values. The result is a deep copy.
func cleanLightingConfig(in leds.Config, devSettings map[string][]leds.Param) (leds.Config, error) {
	out := leds.DefaultConfig()
	out.Version = leds.ConfigVersion
	if in.FPS != 0 {
		out.FPS = max(minLightFPS, min(maxLightFPS, in.FPS))
	}
	out.Brightness = max(0, min(100, in.Brightness))
	if in.Effect != "" {
		if _, ok := lightingEffect(in.Effect); !ok {
			return out, fmt.Errorf("unknown effect %q", in.Effect)
		}
		out.Effect = in.Effect
	}
	for name, p := range in.Params {
		info, ok := lightingEffect(name)
		if !ok {
			return out, fmt.Errorf("parameters for unknown effect %q", name)
		}
		cp, err := cleanParams(info.Params, p)
		if err != nil {
			return out, fmt.Errorf("%s: %w", name, err)
		}
		out.Params[name] = cp
	}
	switch in.ShutdownColor {
	case "", leds.ShutdownKeep, leds.ShutdownDevice:
		out.ShutdownColor = in.ShutdownColor
	default:
		c, err := leds.ParseHexColor(in.ShutdownColor)
		if err != nil || !strings.HasPrefix(in.ShutdownColor, "#") {
			return out, fmt.Errorf("invalid shutdown colour")
		}
		out.ShutdownColor = fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	if len(in.Devices) > maxLightDevices {
		return out, fmt.Errorf("at most %d devices", maxLightDevices)
	}
	for id, dc := range in.Devices {
		if !lightIDRe.MatchString(id) {
			return out, fmt.Errorf("invalid device id %q", id)
		}
		nd := leds.DeviceConfig{Disabled: dc.Disabled, IgnoreConflicts: dc.IgnoreConflicts}
		if len(dc.Settings) > 0 {
			var err error
			dc.Settings = migrateLightSettings(dc.Settings)
			if decl, known := devSettings[id]; known {
				nd.Settings, err = cleanParams(decl, dc.Settings)
			} else {
				nd.Settings, err = cleanLooseSettings(dc.Settings)
			}
			if err != nil {
				return out, fmt.Errorf("%s: %w", id, err)
			}
		}
		if len(dc.Zones) > maxLightZones {
			return out, fmt.Errorf("%s: at most %d zones", id, maxLightZones)
		}
		if len(dc.Zones) > 0 {
			nd.Zones = map[string]leds.ZoneConfig{}
		}
		for zid, zc := range dc.Zones {
			if !lightKeyRe.MatchString(zid) && !lightIDRe.MatchString(zid) {
				return out, fmt.Errorf("%s: invalid zone id %q", id, zid)
			}
			order, err := cleanOrder(zc.Order)
			if err != nil {
				return out, fmt.Errorf("%s/%s: %w", id, zid, err)
			}
			segs, err := cleanSegments(zc.Segments)
			if err != nil {
				return out, fmt.Errorf("%s/%s: %w", id, zid, err)
			}
			nd.Zones[zid] = leds.ZoneConfig{Order: order, Segments: segs}
		}
		out.Devices[id] = nd
	}
	return out, nil
}
