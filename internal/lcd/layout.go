// Package lcd renders sensor dashboards for PC case displays and pushes the
// frames to a display (the real panel, or a PNG file for previews).
package lcd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Layout is the dashboard definition, stored as JSON.
type Layout struct {
	Name        string   `json:"name,omitempty"`
	Round       bool     `json:"round,omitempty"`     // designed for round screens
	Lang        string   `json:"lang,omitempty"`      // clock day/month names: "de" or "" (English)
	Set         string   `json:"set,omitempty"`       // design set / style, e.g. "clean", "neon", "retro"
	Scanlines   bool     `json:"scanlines,omitempty"` // CRT-style scanline overlay
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	IntervalMS  int      `json:"interval_ms"`  // frame interval
	HistorySize int      `json:"history_size"` // samples kept for graphs
	LHMURL      string   `json:"lhm_url"`      // LibreHardwareMonitor data.json
	Background  string   `json:"background"`   // #RRGGBB
	Panel       string   `json:"panel"`        // widget panel color, "" = none
	Foreground  string   `json:"foreground"`   // text color
	Muted       string   `json:"muted"`        // label / track color
	Accent      string   `json:"accent"`       // default widget color
	Widgets     []Widget `json:"widgets"`
}

// Widget is one element on the screen. Fields that don't apply to a type are
// ignored.
type Widget struct {
	Type     string  `json:"type"` // gauge | value | bar | graph | text | clock | icon | box (just the panel card, to group widgets)
	X        int     `json:"x"`
	Y        int     `json:"y"`
	W        int     `json:"w"`
	H        int     `json:"h"`
	Label    string  `json:"label,omitempty"`
	Source   string  `json:"source,omitempty"` // sensor, see metrics.Provider.Get
	Unit     string  `json:"unit,omitempty"`   // overrides the sensor's unit; "none" hides it
	Decimals int     `json:"decimals,omitempty"`
	Min      float64 `json:"min,omitempty"`
	Max      float64 `json:"max,omitempty"`
	Warn     float64 `json:"warn,omitempty"`   // value >= warn draws in warn color (0 = off)
	Crit     float64 `json:"crit,omitempty"`   // value >= crit draws in crit color (0 = off)
	Invert   bool    `json:"invert,omitempty"` // lower is worse: warn/crit apply below the thresholds
	Color    string  `json:"color,omitempty"`

	// gauge only
	Thickness float64 `json:"thickness,omitempty"` // ring width as a fraction of the size
	Glow      bool    `json:"glow,omitempty"`      // soft halo around the value arc
	Smooth    bool    `json:"smooth,omitempty"`    // fade color towards warn/crit instead of switching
	RingOnly  bool    `json:"ring_only,omitempty"` // draw just the ring (combine with centred values)
	Tint      bool    `json:"tint,omitempty"`      // gauge/centred value: number in the widget colour
	Segments  int     `json:"segments,omitempty"`  // gauge/bar: draw as N separate segments
	Ticks     int     `json:"ticks,omitempty"`     // gauge: N scale marks outside the ring
	Sweep     int     `json:"sweep,omitempty"`     // gauge: arc in degrees, 0 = 270; 360 = full circle from the top

	// value only: "center" stacks label/value/second value centred.
	Align string `json:"align,omitempty"`
	// Uniform (centred value): label (Size) and number are drawn at their
	// nominal size on fixed baselines, whatever the text or an inline icon:
	// nothing shrinks or moves, so neighbouring cells line up.
	// The number is ValueSize px (0 = from the height) and the second value
	// as big as the label.
	Uniform   bool    `json:"uniform,omitempty"`
	ValueSize float64 `json:"value_size,omitempty"`
	// AutoDecimals (centred value): one decimal below 10, none from 10 on
	// (GB, ms), whatever Decimals says.
	AutoDecimals bool `json:"auto_decimals,omitempty"`

	// gauge and centred value: secondary reading shown under the main value.
	SubSource   string `json:"sub_source,omitempty"`
	SubUnit     string `json:"sub_unit,omitempty"`
	SubDecimals int    `json:"sub_decimals,omitempty"`

	// icon only: animated vector icon, speed/intensity driven by Source
	// (mapped with Min/Max, colour from Color/Warn/Crit like the others).
	Icon   string  `json:"icon,omitempty"`   // pump | fan | flow | temp | chip | power | fps | clock
	Style  string  `json:"style,omitempty"`  // clean | neon | retro | cyberpunk; "" = the layout's Set
	Color2 string  `json:"color2,omitempty"` // second colour (cyberpunk offset copy), default red
	Speed  float64 `json:"speed,omitempty"`  // animation speed factor, 0 = 1
	// Inline puts the icon in front of a text of the value or gauge widget
	// before it: "main" (the big number), "sub" (second value) or "label".
	// Icon, a gap of 0.3 x its size and the text are centred as one group,
	// the icon on the middle of the digit height. The icon's x and y are
	// ignored; its width is the size (0 = 1.2 x digit height, min. 44 px).
	Inline string `json:"inline,omitempty"`

	// text / clock: static text, or a Go time layout for clocks.
	Text string  `json:"text,omitempty"`
	Size float64 `json:"size,omitempty"` // font size in px, 0 = auto
}

// DefaultLayout is a 480x480 dashboard for a water-cooled system, using
// LibreHardwareMonitor sensor names that work across Intel/AMD CPUs and
// NVIDIA/AMD/Intel GPUs.
func DefaultLayout() Layout {
	return Layout{
		Width: 480, Height: 480,
		IntervalMS:  1000,
		HistorySize: 90,
		Background:  "#000000", // true black: LCD greys look washed out
		Panel:       "#10141c",
		Foreground:  "#f1f4fa",
		Muted:       "#8a94a8",
		Accent:      "#22d3ee",
		Widgets: []Widget{
			{Type: "gauge", X: 12, Y: 12, W: 222, H: 222, Label: "CPU",
				Source: "Load/CPU Total", Unit: "%", Min: 0, Max: 100, Warn: 85, Crit: 95, Color: "#22d3ee",
				SubSource: "Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)|Temperatures/Core (Tctl)", SubUnit: "°"},
			{Type: "gauge", X: 246, Y: 12, W: 222, H: 222, Label: "GPU",
				Source: "Load/GPU Core|Load/D3D 3D", Unit: "%", Min: 0, Max: 100, Warn: 85, Crit: 95, Color: "#76e04a",
				SubSource: "Temperatures/GPU Core|Temperatures/GPU Package", SubUnit: "°"},
			{Type: "value", X: 12, Y: 246, W: 148, H: 110, Label: "COOLANT",
				Source: "Temperatures/T Sensor|Temperatures/Coolant|Temperatures/Water|Temperatures/Liquid", Unit: "°", Decimals: 1,
				Warn: 38, Crit: 42, Color: "#3b82f6"},
			{Type: "value", X: 166, Y: 246, W: 148, H: 110, Label: "PUMP",
				Source: "Fans/Pump|Fans/AIO Pump|Fans/W_PUMP+", Unit: "rpm", Color: "#a78bfa"},
			{Type: "value", X: 320, Y: 246, W: 148, H: 110, Label: "GPU POWER",
				Source: "Powers/GPU Package|Powers/GPU Power|Powers/GPU Core", Unit: "W", Color: "#76e04a"},
			{Type: "graph", X: 12, Y: 368, W: 456, H: 100, Label: "COOLANT",
				Source: "Temperatures/T Sensor|Temperatures/Coolant|Temperatures/Water|Temperatures/Liquid", Unit: "°", Decimals: 1,
				Min: 20, Max: 45, Warn: 38, Crit: 42, Color: "#3b82f6"},
		},
	}
}

// DefaultLayoutPath returns the per-user config path
// (%APPDATA%\casehud\layout.json on Windows, ~/.config/casehud/layout.json on Linux).
func DefaultLayoutPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "layout.json"
	}
	return filepath.Join(dir, "casehud", "layout.json")
}

// LoadLayout reads a layout file, filling unset globals from the default.
func LoadLayout(path string) (Layout, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Layout{}, err
	}
	l := DefaultLayout()
	l.Widgets = nil
	if err := json.Unmarshal(b, &l); err != nil {
		return Layout{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if l.Width <= 0 || l.Height <= 0 {
		return Layout{}, fmt.Errorf("%s: width/height must be positive", path)
	}
	if l.IntervalMS < 100 {
		l.IntervalMS = 100
	}
	if l.HistorySize < 2 {
		l.HistorySize = 2
	}
	Sanitize(&l)
	return l, nil
}

// SaveLayout writes l as indented JSON, creating parent directories.
func SaveLayout(path string, l Layout) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ScaledTo returns a copy of l with all widget geometry scaled to w x h, so a
// layout designed for one panel size can drive another (e.g. 480 -> 400).
func (l Layout) ScaledTo(w, h int) Layout {
	if l.Width == w && l.Height == h {
		return l
	}
	sx, sy := float64(w)/float64(l.Width), float64(h)/float64(l.Height)
	out := l
	out.Width, out.Height = w, h
	out.Widgets = make([]Widget, len(l.Widgets))
	for i, wd := range l.Widgets {
		wd.X = int(float64(wd.X) * sx)
		wd.Y = int(float64(wd.Y) * sy)
		wd.W = int(float64(wd.W) * sx)
		wd.H = int(float64(wd.H) * sy)
		wd.Size *= (sx + sy) / 2
		wd.ValueSize *= (sx + sy) / 2
		out.Widgets[i] = wd
	}
	return out
}

// FitTo is ScaledTo for panels whose shape differs from the layout's (e.g. a
// square design on the 1920x480 bar): the layout keeps its proportions and is
// centred, the background fills the rest. Within 2% of the same aspect ratio
// it just scales.
func (l Layout) FitTo(w, h int) Layout {
	if l.Width <= 0 || l.Height <= 0 {
		return l.ScaledTo(w, h)
	}
	a, b := l.Width*h, l.Height*w
	if d := a - b; d < 0 && -d*50 <= b || d >= 0 && d*50 <= b {
		return l.ScaledTo(w, h)
	}
	s := min(float64(w)/float64(l.Width), float64(h)/float64(l.Height))
	dx := (float64(w) - float64(l.Width)*s) / 2
	dy := (float64(h) - float64(l.Height)*s) / 2
	out := l
	out.Width, out.Height = w, h
	out.Widgets = make([]Widget, len(l.Widgets))
	for i, wd := range l.Widgets {
		wd.X = int(dx + float64(wd.X)*s)
		wd.Y = int(dy + float64(wd.Y)*s)
		wd.W = int(float64(wd.W) * s)
		wd.H = int(float64(wd.H) * s)
		wd.Size *= s
		wd.ValueSize *= s
		out.Widgets[i] = wd
	}
	return out
}

// Limits for layouts from files, the GUI or the internet, so a crafted
// design cannot hang or exhaust the renderer.
const (
	maxWidgets     = 64
	maxSegments    = 360
	maxTicks       = 360
	maxTextRunes   = 256
	maxHistorySize = 600
)

// Sanitize clamps fields a crafted layout could abuse (huge segment counts,
// giant fonts, endless text) to sane ranges. It is applied to every layout
// that is loaded, saved or rendered from outside input.
func Sanitize(l *Layout) {
	if len(l.Widgets) > maxWidgets {
		l.Widgets = l.Widgets[:maxWidgets]
	}
	if l.HistorySize > maxHistorySize {
		l.HistorySize = maxHistorySize
	}
	lim := max(l.Width, l.Height) * 2
	clampInt := func(v *int, lo, hi int) { *v = max(lo, min(hi, *v)) }
	for i := range l.Widgets {
		w := &l.Widgets[i]
		clampInt(&w.X, -lim, lim)
		clampInt(&w.Y, -lim, lim)
		clampInt(&w.W, 0, lim)
		clampInt(&w.H, 0, lim)
		clampInt(&w.Segments, 0, maxSegments)
		clampInt(&w.Ticks, 0, maxTicks)
		clampInt(&w.Sweep, 0, 360)
		clampInt(&w.Decimals, 0, 6)
		if w.Type != "icon" || (w.Inline != "main" && w.Inline != "sub" && w.Inline != "label") {
			w.Inline = ""
		}
		w.Speed = math.Max(0, math.Min(w.Speed, 4))
		if math.IsNaN(w.Speed) {
			w.Speed = 0
		}
		clampInt(&w.SubDecimals, 0, 6)
		if w.Size < 0 || w.Size > float64(2*max(w.H, 1)) {
			w.Size = 0 // auto
		}
		if w.ValueSize < 0 || w.ValueSize > float64(4*max(w.H, 1)) {
			w.ValueSize = 0
		}
		if w.Thickness < 0 || w.Thickness > 0.5 {
			w.Thickness = 0
		}
		for _, t := range []*string{&w.Icon, &w.Style, &w.Color2, &w.Text, &w.Label, &w.Unit, &w.SubUnit} {
			if r := []rune(*t); len(r) > maxTextRunes {
				*t = string(r[:maxTextRunes])
			}
		}
	}
}
