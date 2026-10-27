package app

import "github.com/kewe87/casehud/internal/lcd"

// Animated icons in the built-in designs (grid fields and the wide bar
// designs). One icon per field, inline: it stands directly in front of the
// text it belongs to and forms one centred group with it (lcd.Widget.Inline):
//
//	half fields, wide fields, stacked combos   in front of the big number
//	ring fields, value cards, combo labels     in front of the label of the main reading
//
// The icon follows the field's main reading: chip for CPU/GPU load and
// clocks, temp for temperatures, pump for coolant and pump, power, fps,
// memory for RAM and VRAM, clock for latency and frametime. Icons take their
// colour and style from the design set; with animations off they stand still.

// iconSpec is the icon of a reading: kind, the sensor driving it and the
// mapping of that sensor.
type iconSpec struct {
	icon, src  string
	min, max   float64
	warn, crit float64
	invert     bool
	role       string
}

// iconSpecFor returns the icon that goes with reading r, if there is one.
func iconSpecFor(r tileReading) (iconSpec, bool) {
	sp := iconSpec{src: r.src, min: r.min, max: r.max, warn: r.warn, crit: r.crit, invert: r.invert, role: r.role}
	switch r.src {
	case srcCPULoad, srcGPULoad, srcCPUClock, srcGPUClock:
		sp.icon = "chip"
		if r.src == srcCPUClock || r.src == srcGPUClock {
			sp.src, sp.min, sp.max, sp.warn, sp.crit = r.src, 0, r.max, 0, 0
		}
	case srcCPUTemp, srcGPUTemp, srcGPUHot:
		sp.icon = "temp"
	case srcCoolant, srcPump:
		sp = iconSpec{icon: "pump", src: srcPump, max: 3500, role: r.role}
		if r.src == srcCoolant {
			sp.role = "water"
		}
	case srcGPUPower, srcCPUPower, srcPowerSum:
		sp.icon = "power"
	case srcFPS:
		sp.icon = "fps"
	case srcRAMLoad, srcRAMUsed, srcVRAMLoad, srcVRAMUsed, srcVRAMTotal:
		sp.icon = "memory"
	case srcLatency, srcFrametime:
		sp.icon = "clock" // time between frames; the hands run in real time
	default:
		return iconSpec{}, false
	}
	return sp, true
}

// iconColor is the colour of the icon of a field in style s.
func (s tileStyle) iconColor(sp iconSpec) string {
	if s.set == "cyberpunk" && sp.icon == "temp" {
		return cyberYellow
	}
	return s.color(sp.role)
}

// iconWidget places the icon of sp at x, y with the given size.
func (s tileStyle) iconWidget(sp iconSpec, x, y, size int) lcd.Widget {
	return lcd.Widget{Type: "icon", Icon: sp.icon, X: x, Y: y, W: size, H: size, Source: sp.src,
		Min: sp.min, Max: sp.max, Warn: sp.warn, Crit: sp.crit, Invert: sp.invert, Color: s.iconColor(sp)}
}

// inlineIcon is the icon of sp as an inline icon: it stands in front of the
// text target ("main", "sub" or "label") of the widget before it in the
// layout, centred with that text as one group (see lcd.Widget.Inline). The
// size is 1.2 x the digit height of the text, at least 44 px, unless size > 0.
func (s tileStyle) inlineIcon(sp iconSpec, target string, size int) lcd.Widget {
	w := s.iconWidget(sp, 0, 0, size)
	w.Inline = target
	return w
}

// ringTarget is the text of a ring or value card the icon of r belongs to:
// always the label of its main reading (the big number is too large to
// carry an icon, and one rule keeps the cards alike).
func ringTarget(r tileReading, hasSub bool) string {
	return "label" // one rule everywhere: the icon stands in front of the label
}

// ringIcon is the inline icon of the main reading r of a ring field or value
// card, if it has one. It must directly follow the widget that shows r.
func (s tileStyle) ringIcon(r tileReading) []lcd.Widget {
	return s.ringIconAt(r, ringTarget(r, r.sub != ""))
}

// ringIconAt is ringIcon with the text chosen by the caller.
func (s tileStyle) ringIconAt(r tileReading, target string) []lcd.Widget {
	if sp, ok := iconSpecFor(r); ok {
		return []lcd.Widget{s.inlineIcon(sp, target, 0)}
	}
	return nil
}

// mainIcon is the inline icon in front of the big number of r.
func (s tileStyle) mainIcon(r tileReading) []lcd.Widget {
	if sp, ok := iconSpecFor(r); ok {
		target := "main"
		if r.src == srcCoolant {
			target = "label" // the temperature is not what the pump icon shows
		}
		return []lcd.Widget{s.inlineIcon(sp, target, 0)}
	}
	return nil
}

// wideRowIcons are the icons of the four squares of the wide bar designs
// (CPU and GPU temperature, pump, system power).
var wideRowIcons = [4]iconSpec{
	{icon: "temp", src: srcCPUTemp, min: 30, max: 95, warn: 85, crit: 95, role: "cpu"},
	{icon: "temp", src: srcGPUTemp, min: 30, max: 90, warn: 80, crit: 88, role: "gpu"},
	{icon: "pump", src: srcPump, max: 3500, role: "water"},
	{icon: "power", src: srcPowerSum, max: 700, role: "accent"},
}

// withWideIcons adds the inline icons to a row of four square designs of
// style set made by wideRow: each goes right after the labelled ring or
// value of its square (the pump in front of the rpm line when there is one).
func withWideIcons(l lcd.Layout, vertical bool) lcd.Layout {
	st := tileStyles[0]
	for _, s := range tileStyles {
		if s.set == l.Set {
			st = s
		}
	}
	var out []lcd.Widget
	next := 0
	for _, w := range l.Widgets {
		out = append(out, w)
		if next >= len(wideRowIcons) || (w.Type != "gauge" && w.Type != "value") || w.RingOnly || w.Label == "" {
			continue
		}
		pos := w.X
		if vertical {
			pos = w.Y
		}
		if pos/480 != next {
			continue
		}
		target := "label"
		if wideRowIcons[next].icon == "pump" && w.SubSource != "" && w.Type == "gauge" {
			target = "sub"
		}
		out = append(out, st.inlineIcon(wideRowIcons[next], target, 0))
		next++
	}
	l.Widgets = out
	return l
}
