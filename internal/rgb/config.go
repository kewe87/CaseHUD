package leds

import (
	"errors"
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// ConfigVersion is the current version of the Config JSON section.
const ConfigVersion = 1

// Config is the persisted state of the engine. The engine never writes files:
// the host stores it (e.g. as a section of its own config) and passes it to New;
// changes are reported through Engine.OnConfigChange.
type Config struct {
	Version    int                     `json:"version"`
	FPS        int                     `json:"fps,omitempty"`     // default 30
	Brightness int                     `json:"brightness"`        // 0..100
	Effect     string                  `json:"effect,omitempty"`  // effect name
	Params     map[string]Params       `json:"params,omitempty"`  // per effect name
	Devices    map[string]DeviceConfig `json:"devices,omitempty"` // per device ID
	// ShutdownColor is the last frame Stop sends: "#rrggbb", empty = off,
	// ShutdownKeep = no last frame (devices keep what they show),
	// ShutdownDevice = devices that can return to their own effect do so
	// (DeviceEffectRestorer), the others are switched off.
	ShutdownColor string `json:"shutdownColor,omitempty"`
}

// Special ShutdownColor values.
const (
	ShutdownKeep   = "keep"
	ShutdownDevice = "device"
)

// DeviceEffectRestorer is implemented by devices that can hand their light
// back to their own (stored) effect; Stop calls it for ShutdownDevice
// instead of sending a last frame.
type DeviceEffectRestorer interface {
	RestoreDeviceEffect() error
}

// ErrNoDeviceEffect is returned by RestoreDeviceEffect when this particular
// device has no way back; Stop then switches it off.
var ErrNoDeviceEffect = errors.New("leds: device has no own effect to return to")

// DeviceConfig holds per-device settings.
type DeviceConfig struct {
	Disabled bool `json:"disabled,omitempty"`
	// IgnoreConflicts drives the device even while a conflicting program runs.
	IgnoreConflicts bool                  `json:"ignoreConflicts,omitempty"`
	Zones           map[string]ZoneConfig `json:"zones,omitempty"` // per zone ID
	// Settings are device-specific options (see Configurable).
	Settings Params `json:"settings,omitempty"`
}

// ZoneConfig places a zone on the canvas. A zone consists of one or more
// segments laid out one after another along the wire, e.g. three daisy-chained
// fans on one ARGB header.
type ZoneConfig struct {
	Order    string    `json:"order,omitempty"` // wire color order, overrides the device default
	Segments []Segment `json:"segments,omitempty"`
}

// Segment is a run of LEDs with a shape inside a rectangle of the canvas
// (normalized coordinates).
type Segment struct {
	Name  string `json:"name,omitempty"`
	Count int    `json:"count"`           // component: 0 = all LEDs of the component
	Shape string `json:"shape,omitempty"` // "line" (default), "ring", "rect", "point", "component"
	// Component names the LED layout for Shape "component" (see Components).
	Component string  `json:"component,omitempty"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	W         float64 `json:"w"`
	H         float64 `json:"h"`
	Angle     float64 `json:"angle,omitempty"`   // degrees; line: direction, ring: start position, component: clockwise rotation
	Reverse   bool    `json:"reverse,omitempty"` // run against the shape direction
	// Start is the zone LED index of the segment's first LED. Unset: right
	// after the previous segment. Parts in parallel on one header (a
	// splitter) all start at 0: they show the same colors, and an LED driven
	// by several segments takes its canvas position from the first of them.
	Start *int `json:"start,omitempty"`
}

// Params are effect parameter values as they arrive from JSON.
type Params map[string]any

// Float returns a number parameter.
func (p Params) Float(key string) float64 {
	switch v := p[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

// Bool returns a bool parameter.
func (p Params) Bool(key string) bool {
	switch v := p[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return p.Float(key) != 0
}

// String returns a string or enum parameter.
func (p Params) String(key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return fmt.Sprint(p[key])
}

// Color returns a color parameter ("#rrggbb"); invalid values give black.
func (p Params) Color(key string) color.RGBA {
	c, _ := ParseHexColor(p.String(key))
	return c
}

// ParseHexColor parses "#rrggbb" or "rrggbb".
func ParseHexColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.RGBA{A: 255}, fmt.Errorf("invalid color %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{A: 255}, fmt.Errorf("invalid color %q", s)
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, nil
}

// withDefaults returns a copy of params with every declared key present.
func withDefaults(info EffectInfo, params Params) Params {
	return paramsWithDefaults(info.Params, params)
}

func paramsWithDefaults(decl []Param, params Params) Params {
	out := Params{}
	for _, p := range decl {
		out[p.Key] = p.Default
	}
	for k, v := range params {
		out[k] = v
	}
	return out
}

func (c Config) clone() Config {
	out := c
	out.Params = make(map[string]Params, len(c.Params))
	for k, v := range c.Params {
		p := make(Params, len(v))
		for pk, pv := range v {
			p[pk] = pv
		}
		out.Params[k] = p
	}
	out.Devices = make(map[string]DeviceConfig, len(c.Devices))
	for k, v := range c.Devices {
		d := v
		if v.Settings != nil {
			d.Settings = make(Params, len(v.Settings))
			for sk, sv := range v.Settings {
				d.Settings[sk] = sv
			}
		}
		d.Zones = make(map[string]ZoneConfig, len(v.Zones))
		for zk, zv := range v.Zones {
			z := zv
			z.Segments = append([]Segment(nil), zv.Segments...)
			d.Zones[zk] = z
		}
		out.Devices[k] = d
	}
	return out
}

func (c *Config) normalize() {
	if c.Version == 0 {
		c.Version = ConfigVersion
	}
	if c.FPS <= 0 {
		c.FPS = 30
	}
	if c.FPS > 120 {
		c.FPS = 120
	}
	c.Brightness = clampInt(c.Brightness, 0, 100)
	if c.Params == nil {
		c.Params = map[string]Params{}
	}
	if c.Devices == nil {
		c.Devices = map[string]DeviceConfig{}
	}
}

// DefaultConfig is a usable starting configuration.
func DefaultConfig() Config {
	c := Config{Version: ConfigVersion, FPS: 30, Brightness: 100, Effect: "rainbow"}
	c.normalize()
	return c
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
