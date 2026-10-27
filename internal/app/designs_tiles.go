package app

import (
	"fmt"
	"strings"

	"github.com/kewe87/casehud/internal/lcd"
)

// Designs for the fields of a grid (see lcd.GridPresets). A grid field is a
// small display of its own, so these are ordinary designs in the library,
// drawn for the field sizes:
//
//	card-*          480x480 squares with a panel card (clean set)
//	half-*          480x240 half fields: one big number, a label, a thin bar
//	w640-*, w960-*  640x480 and 960x480 wide fields: ring or big number plus history graphs
//	h640-*, h960-*  the same for the upright bar (480x640, 480x960)
//
// Every kind exists in all four sets (prefix "", neon-, retro-, cyber-).
// Legibility rules for the 8.8" bar (viewed from 60-100 cm, 0.113 mm
// pixels): main values >= 60 px digit height, labels and secondary text
// >= 28 px.

// tileReading is what a tile kind shows.
type tileReading struct {
	label string // field label ("CPU", "GPU TEMP")
	role  string // colour role: cpu, gpu, water, power, accent, mem
	src   string
	unit  string
	dec   int
	min   float64
	max   float64
	warn  float64
	crit  float64
	// invert: lower is worse (FPS).
	invert bool
	// secondary reading
	subLabel string
	sub      string
	subUnit  string
	subMin   float64
	subMax   float64
	subDec   int
	auto     bool   // one decimal below 10, none from 10 on (GB, ms)
	barSrc   string // half bar shows this percentage reading instead of the value
}

// tileKinds are the readings, in the order they are offered.
var tileKinds = []string{"cpu", "gpu", "cpu-temp", "gpu-temp", "coolant", "pump", "power", "fps", "ram", "vram", "clock"}

// cardKinds are the square cards that exist in every set (the clean ones are
// cardDesigns).
var cardKinds = []string{"cpu", "gpu", "coolant", "power", "fps"}

// wideKinds are the kinds that exist as wide fields.
var wideKinds = []string{"cpu", "gpu", "coolant", "power", "fps", "clock"}

const (
	srcRAMLoad = "Load/Memory"
	srcRAMUsed = "Data/Memory Used"
)

var tileReadings = map[string]tileReading{
	"cpu": {label: "CPU", role: "cpu", src: srcCPULoad, unit: "%", max: 100, warn: 85, crit: 97,
		subLabel: "TEMP", sub: srcCPUTemp, subUnit: "°", subMin: 30, subMax: 95},
	"gpu": {label: "GPU", role: "gpu", src: srcGPULoad, unit: "%", max: 100, warn: 85, crit: 97,
		subLabel: "TEMP", sub: srcGPUTemp, subUnit: "°", subMin: 30, subMax: 90},
	"cpu-temp": {label: "CPU TEMP", role: "cpu", src: srcCPUTemp, unit: "°", min: 30, max: 95, warn: 85, crit: 95,
		subLabel: "LOAD", sub: srcCPULoad, subUnit: "%", subMax: 100},
	"gpu-temp": {label: "GPU TEMP", role: "gpu", src: srcGPUTemp, unit: "°", min: 30, max: 90, warn: 80, crit: 88,
		subLabel: "LOAD", sub: srcGPULoad, subUnit: "%", subMax: 100},
	"coolant": {label: "COOLANT", role: "water", src: srcCoolant, unit: "°", dec: 1, min: 20, max: 45, warn: 38, crit: 42,
		subLabel: "PUMP", sub: srcPump, subUnit: " rpm", subMax: 3500},
	"pump": {label: "PUMP", role: "accent", src: srcPump, unit: " rpm", max: 3500,
		subLabel: "COOLANT", sub: srcCoolant, subUnit: "°", subDec: 1, subMin: 20, subMax: 45},
	"power": {label: "GPU POWER", role: "power", src: srcGPUPower, unit: "W", max: 450,
		subLabel: "CPU", sub: srcCPUPower, subUnit: " W", subMax: 250},
	"fps": {label: "FPS", role: "accent", src: srcFPS, unit: "none", max: 144, warn: 60, crit: 30, invert: true,
		subLabel: "LATENCY", sub: srcLatency + "|" + srcFrametime, subUnit: " ms", subMax: 50},
	"ram": {label: "RAM", role: "mem", src: srcRAMLoad, unit: "%", max: 100, warn: 85, crit: 95,
		subLabel: "USED", sub: srcRAMUsed, subUnit: " GB", subMax: 64},
	"vram": {label: "VRAM", role: "mem", src: srcVRAMLoad, unit: "%", max: 100, warn: 90, crit: 97,
		subLabel: "USED", sub: srcVRAMUsed, subUnit: " GB", subMax: 24},
}

var kindNames = map[string]string{
	"cpu": "CPU load", "gpu": "GPU load", "cpu-temp": "CPU temperature", "gpu-temp": "GPU temperature",
	"coolant": "Coolant", "pump": "Pump", "power": "GPU power", "fps": "FPS", "ram": "RAM", "vram": "VRAM", "clock": "Clock",
}

// tileStyle is one design set's look for tile designs.
type tileStyle struct {
	set    string
	prefix string
	base   func(name string) lcd.Layout
	colors map[string]string // role -> colour
	// bar / ring look
	barSegments int
	ring        func(t tileStyle, x, y int, r tileReading) []lcd.Widget
	cards       bool
}

var tileStyles = []tileStyle{
	{set: "clean", prefix: "", cards: true,
		base: func(name string) lcd.Layout { l := base(name, false); return l },
		colors: map[string]string{"cpu": colorCPU, "gpu": colorGPU, "water": colorWater, "power": colorGPU,
			"accent": colorAccent, "mem": "#f59e0b"},
		ring: cleanRing},
	{set: "neon", prefix: "neon-", base: neonBase,
		colors: map[string]string{"cpu": neonCyan, "gpu": neonMagenta, "water": neonBlue, "power": neonViolet,
			"accent": neonViolet, "mem": neonBlue},
		barSegments: 24, ring: neonTileRing},
	{set: "retro", prefix: "retro-", base: retroBase,
		colors:      map[string]string{"cpu": retroAmber, "gpu": retroAmber, "water": retroAmber, "power": retroAmber, "accent": retroAmber, "mem": retroAmber},
		barSegments: 12, ring: retroTileRing},
	{set: "cyberpunk", prefix: "cyber-", base: cyberBase,
		colors: map[string]string{"cpu": cyberYellow, "gpu": cyberCyan, "water": cyberCyan, "power": cyberYellow,
			"accent": cyberYellow, "mem": cyberCyan},
		barSegments: 24, ring: cyberTileRing},
}

func (s tileStyle) color(role string) string { return s.colors[role] }

// layout is the style's empty layout of the given size.
func (s tileStyle) layout(name string, w, h int) lcd.Layout {
	l := s.base(name)
	l.Round = false
	l.Set = s.set
	l.Width, l.Height = w, h
	return l
}

// mainWidget is the reading as a widget (thresholds, colour).
func (s tileStyle) thresholds(r tileReading, wd lcd.Widget) lcd.Widget {
	wd.Source, wd.Unit, wd.Decimals = r.src, r.unit, r.dec
	wd.Min, wd.Max, wd.Warn, wd.Crit, wd.Invert = r.min, r.max, r.warn, r.crit, r.invert
	wd.Color = s.color(r.role)
	return wd
}

// --- rings for 480x480 regions (x, y = origin of the region) ---

// cleanRing: ring with the number inside, like the water-wide tiles.
func cleanRing(t tileStyle, x, y int, r tileReading) []lcd.Widget {
	g := t.thresholds(r, lcd.Widget{Type: "gauge", X: x + 12, Y: y + 12, W: 456, H: 456, RingOnly: true, Smooth: true})
	v := t.cardValue(r, x+98, y+122, 284, 270, true)
	return append([]lcd.Widget{g, v}, t.cardIcon(r, ringTarget(r, r.sub != ""))...)
}

func neonTileRing(t tileStyle, x, y int, r tileReading) []lcd.Widget {
	g := t.thresholds(r, lcd.Widget{Type: "gauge", X: x + 22, Y: y + 22, W: 436, H: 436, Thickness: 0.06,
		Segments: 30, Ticks: 30, Glow: true, Smooth: true, RingOnly: true})
	v := t.cardValue(r, x+98, y+122, 284, 270, true)
	return append([]lcd.Widget{g, v}, t.cardIcon(r, ringTarget(r, r.sub != ""))...)
}

func retroTileRing(t tileStyle, x, y int, r tileReading) []lcd.Widget {
	rv := r
	rv.sub = "" // the LED meter below shows the second reading
	v := t.cardValue(rv, x+60, y+66, 360, 220, true)
	if r.sub == "" {
		return append([]lcd.Widget{v}, t.cardIcon(r, "label")...)
	}
	b := lcd.Widget{Type: "bar", X: x + 84, Y: y + 290, W: 312, H: 124, Label: r.subLabel, Source: r.sub,
		Unit: strings.TrimSpace(r.subUnit), Decimals: r.subDec, Min: r.subMin, Max: r.subMax, Segments: 12, Color: retroAmber}
	// no rpm line in the number: the icon goes with the label
	return append(append([]lcd.Widget{v}, t.cardIcon(r, "label")...), b)
}

func cyberTileRing(t tileStyle, x, y int, r tileReading) []lcd.Widget {
	ring := t.thresholds(r, lcd.Widget{Type: "gauge", X: x + 18, Y: y + 18, W: 444, H: 444, Thickness: 0.04,
		Segments: 48, Ticks: 24, RingOnly: true})
	ghost := ring
	ghost.X, ghost.Y, ghost.Color, ghost.Ticks = x+21, y+15, cyberGhost, 0
	ghost.Warn, ghost.Crit = 0, 0
	ring.Color = cyberYellow
	v := t.cardValue(r, x+90, y+120, 300, 260, true)
	v.Color = cyberYellow
	return append([]lcd.Widget{ghost, ring, v}, t.cardIcon(r, ringTarget(r, r.sub != ""))...)
}

// --- half fields: 480x240 ---

func (s tileStyle) bar(r tileReading, x, y, w, h int) lcd.Widget {
	return s.thresholds(r, lcd.Widget{Type: "bar", X: x, Y: y, W: w, H: h, Segments: s.barSegments, Smooth: s.set != "retro"})
}

func (s tileStyle) half(kind string) lcd.Layout {
	l := s.layout(kindNames[kind], 480, 240)
	if kind == "clock" {
		return s.clockHalf(l)
	}
	r := tileReadings[kind]
	l.Widgets = append([]lcd.Widget{{Type: "box", X: 12, Y: 12, W: 456, H: 216}}, s.halfCell(r, halfSingle, true)...)
	return l
}

func (s tileStyle) clockText(l *lcd.Layout, x, y, w, h, hSize, dSize int, dateY int) {
	fg := s.color("accent")
	if s.set == "clean" {
		fg = ""
	}
	l.Widgets = append(l.Widgets,
		lcd.Widget{Type: "clock", X: x, Y: y, W: w, H: h, Text: "15:04", Size: float64(hSize), Color: fg},
		lcd.Widget{Type: "clock", X: x, Y: dateY, W: w, H: dSize + 8, Text: "Mon 02.01.", Size: float64(dSize), Color: s.dateColor()})
}

func (s tileStyle) dateColor() string {
	switch s.set {
	case "neon":
		return neonCyan
	case "retro":
		return retroDim
	case "cyberpunk":
		return cyberCyan
	}
	return "#8a94a8"
}

func (s tileStyle) clockHalf(l lcd.Layout) lcd.Layout {
	l.Widgets = []lcd.Widget{{Type: "box", X: 12, Y: 12, W: 456, H: 216}}
	s.clockText(&l, 12, 22, 456, 130, 120, 36, 160)
	return l
}

// --- wide and upright fields ---

// wide builds a field of w x h: 960x480 / 480x960 (a ring square plus two
// history graphs) or 640x480 / 480x640 (a big number over a graph).
func (s tileStyle) wide(kind string, w, h int) lcd.Layout {
	l := s.layout(kindNames[kind], w, h)
	if kind == "clock" {
		return s.clockWide(l, w, h)
	}
	r := tileReadings[kind]
	portrait := h > w
	long := max(w, h)
	if long >= 2*min(w, h) {
		ox, oy := 0, 0
		gx, gy := 480, 0
		if portrait {
			gx, gy = 0, 480
		}
		l.Widgets = append(l.Widgets, s.ring(s, ox, oy, squareReading(kind))...)
		g1 := s.thresholds(r, lcd.Widget{Type: "graph", X: gx + 12, Y: gy + 12, W: 456, H: 216, Label: r.label, Smooth: true})
		g2 := lcd.Widget{Type: "graph", X: gx + 12, Y: gy + 252, W: 456, H: 216, Label: r.subLabel, Source: r.sub, Unit: r.subUnit,
			Decimals: r.subDec, Min: r.subMin, Max: r.subMax, Color: s.color(r.role)}
		if r.sub == srcLatency+"|"+srcFrametime {
			g2.Unit = " ms"
		}
		if s.set == "cyberpunk" || s.set == "retro" {
			g2.Color = s.dateColor()
			if s.set == "retro" {
				g2.Color = retroAmber
			}
		}
		l.Widgets = append(l.Widgets, g1, g2)
		return l
	}
	// Two cards, a value over a graph; the text keeps clear of the card edges.
	avail := h - 36
	gc := avail * 40 / 100
	if portrait {
		gc = avail * 36 / 100
	}
	vc := avail - gc
	v := s.cardValue(r, 28, 28, w-56, vc-32, false)
	g := s.thresholds(r, lcd.Widget{Type: "graph", X: 20, Y: 12 + vc + 12 + 8, W: w - 40, H: gc - 16, Label: r.label, Smooth: true})
	l.Widgets = append(l.Widgets,
		lcd.Widget{Type: "box", X: 12, Y: 12, W: w - 24, H: vc},
		lcd.Widget{Type: "box", X: 12, Y: 12 + vc + 12, W: w - 24, H: gc}, v)
	// the icon stands in front of the label (the pump before the rpm line)
	l.Widgets = append(l.Widgets, s.cardIcon(r, ringTarget(r, r.sub != ""))...)
	l.Widgets = append(l.Widgets, g)
	return l
}

func (s tileStyle) clockWide(l lcd.Layout, w, h int) lcd.Layout {
	portrait := h > w
	if max(w, h) >= 2*min(w, h) {
		// seconds ring in a square, time and date in the other
		rx, ry, tx, ty := 0, 0, 480, 0
		if portrait {
			tx, ty = 0, 480
		}
		sec := lcd.Widget{Type: "gauge", X: rx + 12, Y: ry + 12, W: 456, H: 456, Source: "Clock/Seconds", Max: 60, RingOnly: true,
			Sweep: 360, Color: s.color("accent")}
		switch s.set {
		case "neon", "cyberpunk":
			sec.Segments, sec.Thickness, sec.Glow = 60, 0.04, s.set == "neon"
		case "retro":
			sec.Segments, sec.Thickness = 60, 0.05
		}
		l.Widgets = append(l.Widgets, sec)
		// time inside the ring, weekday and date on a card of their own
		s.clockText(&l, rx+60, ry+150, 360, 140, 122, 36, ry+296)
		fg := s.color("accent")
		if s.set == "clean" {
			fg = ""
		}
		l.Widgets = append(l.Widgets,
			lcd.Widget{Type: "box", X: tx + 12, Y: ty + 12, W: 456, H: 456},
			lcd.Widget{Type: "clock", X: tx + 12, Y: ty + 120, W: 456, H: 120, Text: "Monday", Size: 84, Color: fg},
			lcd.Widget{Type: "clock", X: tx + 12, Y: ty + 250, W: 456, H: 90, Text: "2. January", Size: 60, Color: s.dateColor()})
		return l
	}
	l.Widgets = append(l.Widgets, lcd.Widget{Type: "box", X: 12, Y: 12, W: w - 24, H: h - 24})
	s.clockText(&l, 12, h/2-110, w-24, 170, 150, 40, h/2+70)
	if s.set == "retro" {
		return l // an unlabeled LED bar says nothing here
	}
	l.Widgets = append(l.Widgets, lcd.Widget{Type: "bar", X: 60, Y: h - 80, W: w - 120, H: 22, Source: "Clock/Seconds", Unit: "none",
		Max: 60, Segments: s.barSegments, Color: s.color("accent")})
	return l
}

// --- clean card squares (the water-wide tiles as designs of their own) ---

func cardDesigns() map[string]lcd.Layout {
	s := tileStyles[0]
	mk := func(name string, wd []lcd.Widget) lcd.Layout {
		l := s.layout(name, 480, 480)
		l.Widgets = wd
		return l
	}
	out := map[string]lcd.Layout{}
	ring := func(kind string) lcd.Layout {
		r := tileReadings[kind]
		return mk(kindNames[kind], s.ring(s, 0, 0, r))
	}
	out["card-cpu"], out["card-gpu"], out["card-fps"] = ring("cpu"), ring("gpu"), ring("fps")
	{
		l := s.layout("Clock", 480, 480)
		l.Widgets = []lcd.Widget{{Type: "gauge", X: 12, Y: 12, W: 456, H: 456, Source: "Clock/Seconds", Max: 60, RingOnly: true,
			Sweep: 360, Color: s.color("accent")}}
		s.clockText(&l, 60, 150, 360, 140, 122, 36, 296)
		out["card-clock"] = l
	}
	card := func(kind, name, label string) lcd.Layout {
		r := tileReadings[kind]
		r.label = label
		return mk(name, append([]lcd.Widget{s.cardValue(r, 12, 12, 456, 456, false)}, s.cardIcon(r, ringTarget(r, r.sub != ""))...))
	}
	coolant := card("coolant", "Coolant", "COOLANT")
	power := card("power", "Power", "POWER")
	power.Widgets[0].SubUnit = " W CPU"
	out["card-coolant"], out["card-power"] = coolant, power
	return out
}

// squareReading is the reading of kind k on a square card: the label has to
// fit beside its icon inside a ring.
func squareReading(k string) tileReading {
	r := tileReadings[k]
	if k == "power" {
		r.label = "POWER"
	}
	return r
}

// tileDesigns returns all the grid-field designs, keyed by id.
func tileDesigns() map[string]lcd.Layout {
	out := cardDesigns()
	for id, l := range comboDesigns() {
		out[id] = l
	}
	for _, s := range tileStyles {
		if s.prefix != "" {
			// the other sets' counterparts of the clean cards
			for _, k := range cardKinds {
				l := s.layout(kindNames[k], 480, 480)
				l.Widgets = s.ring(s, 0, 0, squareReading(k))
				out[s.prefix+"card-"+k] = l
			}
		}
		for _, k := range tileKinds {
			out[s.prefix+"half-"+k] = s.half(k)
		}
		for _, k := range wideKinds {
			for _, sz := range []struct {
				tag  string
				w, h int
			}{{"w640", 640, 480}, {"w960", 960, 480}, {"h640", 480, 640}, {"h960", 480, 960}} {
				out[fmt.Sprintf("%s%s-%s", s.prefix, sz.tag, k)] = s.wide(k, sz.w, sz.h)
			}
		}
	}
	return out
}

// tileDesignOrder lists the ids of tileDesigns in the order they are offered.
func tileDesignOrder() []string {
	ids := []string{"card-cpu", "card-gpu", "card-coolant", "card-power", "card-fps", "card-clock"}
	for _, s := range tileStyles {
		for _, k := range tileKinds {
			ids = append(ids, s.prefix+"half-"+k)
		}
		if s.prefix != "" {
			for _, k := range cardKinds {
				ids = append(ids, s.prefix+"card-"+k)
			}
		}
		for _, d := range comboDefs() {
			ids = append(ids, comboID(s, d))
		}
		for _, tag := range []string{"w640", "w960", "h640", "h960"} {
			for _, k := range wideKinds {
				ids = append(ids, s.prefix+tag+"-"+k)
			}
		}
	}
	return ids
}

// ringLabel is the label inside a ring. Labels are the same in every style;
// long ones shrink to fit when drawn.
func (r tileReading) ringLabel() string { return r.label }
