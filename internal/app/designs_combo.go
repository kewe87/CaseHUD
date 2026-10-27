package app

import "github.com/kewe87/casehud/internal/lcd"

// Combined cell designs: several readings of one thing in one cell, so a
// grid composition shows every value exactly once.
//
//	half-<id>   480x240, two readings side by side, each with its own bar
//	card-<id>   480x480, ring or stack with up to three readings
//
// Every design exists in all four sets (prefix "", neon-, retro-, cyber-).

const (
	srcCPUClock = "Clocks/Cores (Average)|Clocks/CPU Core #1"
	srcGPUClock = "Clocks/GPU Core"
)

func auto(r tileReading) tileReading { r.auto = true; return r }

func rd(label, role, src, unit string, dec int, min, max, warn, crit float64) tileReading {
	return tileReading{label: label, role: role, src: src, unit: unit, dec: dec, min: min, max: max, warn: warn, crit: crit}
}

var (
	stCPULoad  = rd("CPU LOAD", "cpu", srcCPULoad, "%", 0, 0, 100, 85, 97)
	stCPUTemp  = rd("CPU TEMP", "cpu", srcCPUTemp, "°", 0, 30, 95, 85, 95)
	stGPULoad  = rd("GPU LOAD", "gpu", srcGPULoad, "%", 0, 0, 100, 85, 97)
	stGPUTemp  = rd("GPU TEMP", "gpu", srcGPUTemp, "°", 0, 30, 90, 80, 88)
	stGPUHot   = rd("HOTSPOT", "gpu", srcGPUHot, "°", 0, 30, 105, 90, 100)
	stGPUClock = rd("GPU MHZ", "gpu", srcGPUClock, "none", 0, 0, 3200, 0, 0)
	stCPUClock = rd("CPU MHZ", "cpu", srcCPUClock, "none", 0, 0, 5800, 0, 0)
	stGPUPower = rd("GPU WATT", "power", srcGPUPower, "W", 0, 0, 450, 0, 0)
	stCPUPower = rd("CPU WATT", "cpu", srcCPUPower, "W", 0, 0, 250, 0, 0)
	stCoolant  = rd("COOLANT", "water", srcCoolant, "°", 1, 20, 45, 38, 42)
	stPump     = rd("PUMP RPM", "accent", srcPump, "none", 0, 0, 3500, 0, 0)
	stRAMUsed  = auto(rd("RAM USED", "mem", srcRAMUsed, "GB", 0, 0, 64, 0, 0))
	stRAMLoad  = rd("RAM LOAD", "mem", srcRAMLoad, "%", 0, 0, 100, 85, 95)
)

// comboDef describes a combined design: stats[0] is the main reading.
type comboDef struct {
	id, name string
	layout   string // half | stack | ring | fps
	stats    []tileReading
	shared   bool // half: one bar for both, of barSrc
	barSrc   string
	barMax   float64
}

func comboDefs() []comboDef {
	vramUsed := auto(rd("VRAM", "mem", srcVRAMUsed, "GB", 0, 0, 24, 0, 0))
	vramTotal := auto(rd("TOTAL", "mem", srcVRAMTotal, "GB", 0, 0, 24, 0, 0))
	fps := rd("FPS", "accent", srcFPS, "none", 0, 0, 144, 60, 30)
	fps.invert = true
	frame := auto(rd("FRAMETIME", "accent", srcFrametime, "ms", 0, 0, 16, 0, 0))
	frameG := frame
	frameG.dec, frameG.auto = 1, false // the graph shows it with a decimal
	sysPower := rd("SYSTEM", "accent", srcPowerSum, "W", 0, 0, 700, 0, 0)
	gpuW, cpuW := stGPUPower, stCPUPower
	gpuW.label, cpuW.label, gpuW.role = "GPU", "CPU", "gpu"
	ramGB := stRAMUsed
	ramGB.label, ramGB.barSrc = "RAM", srcRAMLoad
	vramGB := vramUsed
	vramGB.label, vramGB.barSrc = "VRAM", srcVRAMLoad
	lat := rd("LATENCY", "accent", srcLatency, "ms", 0, 0, 60, 0, 0)
	gpuMain := rd("GPU", "gpu", srcGPULoad, "%", 0, 0, 100, 85, 97)
	gpuP := stGPUPower
	gpuP.role = "gpu"
	return []comboDef{
		{id: "cpu-lt", name: "CPU load + temperature", layout: "half", stats: []tileReading{stCPULoad, stCPUTemp}},
		{id: "gpu-lt", name: "GPU load + temperature", layout: "half", stats: []tileReading{stGPULoad, stGPUTemp}},
		{id: "coolant-pump", name: "Coolant + pump", layout: "half", stats: []tileReading{stCoolant, stPump}},
		{id: "power-split", name: "GPU + CPU power", layout: "half", stats: []tileReading{stGPUPower, stCPUPower}},
		{id: "vram-gb", name: "VRAM used / total", layout: "half", stats: []tileReading{vramUsed, vramTotal},
			shared: true, barSrc: srcVRAMLoad, barMax: 100},
		{id: "mem", name: "RAM + VRAM", layout: "half", stats: []tileReading{ramGB, vramGB}},
		{id: "lat-cpu", name: "Latency + CPU power", layout: "half", stats: []tileReading{lat, stCPUPower}},
		{id: "gpu-clock", name: "GPU clock + hotspot", layout: "half", stats: []tileReading{stGPUClock, stGPUHot}},
		{id: "cpu-clock", name: "CPU clock + power", layout: "half", stats: []tileReading{stCPUClock, stCPUPower}},
		{id: "fps-ft", name: "FPS + frametime", layout: "half", stats: []tileReading{fps, frame}},
		{id: "gpu-full", name: "GPU load, temperature, power", layout: "ring", stats: []tileReading{gpuMain, stGPUTemp, gpuP}},
		{id: "system", name: "System power", layout: "stack", stats: []tileReading{sysPower, gpuW, cpuW}},
		{id: "fps-graph", name: "FPS + frametime graph", layout: "fps", stats: []tileReading{fps, frameG}},
	}
}

// comboIDs lists the ids of the combined designs with the given layouts.
func comboIDs(layouts ...string) []string {
	var ids []string
	for _, d := range comboDefs() {
		for _, l := range layouts {
			if d.layout == l {
				ids = append(ids, d.id)
			}
		}
	}
	return ids
}

func comboID(s tileStyle, d comboDef) string {
	if d.layout == "half" {
		return s.prefix + "half-" + d.id
	}
	return s.prefix + "card-" + d.id
}

func (s tileStyle) combo(d comboDef) lcd.Layout {
	switch d.layout {
	case "half":
		return s.halfCombo(d)
	case "stack":
		return s.stackCombo(d)
	case "fps":
		return s.fpsCombo(d)
	}
	return s.ringCombo(d)
}

func (s tileStyle) halfCombo(d comboDef) lcd.Layout {
	l := s.layout(d.name, 480, 240)
	l.Widgets = []lcd.Widget{{Type: "box", X: 12, Y: 12, W: 456, H: 216}}
	for i, r := range d.stats {
		if d.shared {
			r.barSrc = d.barSrc // one reading drives the bars of both columns
		}
		l.Widgets = append(l.Widgets, s.halfCell(r, halfCols[i], i == 0)...)
	}
	return l
}

// stackCombo: the main reading as a value card on top, two more in half-field
// cells below it.
func (s tileStyle) stackCombo(d comboDef) lcd.Layout {
	l := s.layout(d.name, 480, 480)
	main := s.cardValue(d.stats[0], 36, 20, 408, 196, false)
	l.Widgets = append([]lcd.Widget{{Type: "box", X: 12, Y: 12, W: 456, H: 456}, main}, s.cardIcon(d.stats[0], "label")...)
	for i, r := range d.stats[1:] {
		c := halfCols[i]
		c.dy = 224
		l.Widgets = append(l.Widgets, s.halfCell(r, c, false)...)
	}
	return l
}

func (s tileStyle) fpsCombo(d comboDef) lcd.Layout {
	l := s.layout(d.name, 480, 480)
	fr := d.stats[1]
	g := s.thresholds(fr, lcd.Widget{Type: "graph", X: 24, Y: 288, W: 432, H: 168, Label: fr.label, Smooth: true})
	g.Unit = " ms"
	main := s.cardValue(d.stats[0], 36, 16, 408, 270, false)
	l.Widgets = append([]lcd.Widget{{Type: "box", X: 12, Y: 12, W: 456, H: 456}, main}, s.cardIcon(d.stats[0], "label")...)
	l.Widgets = append(l.Widgets, g)
	return l
}

// ringCombo: a ring (none in retro) for the main reading with the number
// inside and two more readings below it.
func (s tileStyle) ringCombo(d comboDef) lcd.Layout {
	l := s.layout(d.name, 480, 480)
	r := d.stats[0]
	my, subY := 120, 298 // the same line as the ring cards
	if s.set == "cyberpunk" {
		my = 113
	}
	switch s.set {
	case "clean":
		l.Widgets = append(l.Widgets, s.thresholds(r, lcd.Widget{Type: "gauge", X: 12, Y: 12, W: 456, H: 456, RingOnly: true, Smooth: true}))
	case "neon":
		l.Widgets = append(l.Widgets, s.thresholds(r, lcd.Widget{Type: "gauge", X: 22, Y: 22, W: 436, H: 436, Thickness: 0.06,
			Segments: 30, Ticks: 30, Glow: true, Smooth: true, RingOnly: true}))
	case "cyberpunk":
		ring := s.thresholds(r, lcd.Widget{Type: "gauge", X: 18, Y: 18, W: 444, H: 444, Thickness: 0.04, Segments: 48, Ticks: 24, RingOnly: true})
		ghost := ring
		ghost.X, ghost.Y, ghost.Color, ghost.Ticks, ghost.Warn, ghost.Crit = 21, 15, cyberGhost, 0, 0, 0
		ring.Color = cyberYellow
		l.Widgets = append(l.Widgets, ghost, ring)
	default:
		my, subY = 71, 291
		l.Widgets = append(l.Widgets, s.bar(r, 104, 366, 272, 28)) // the LED meter of the retro cards
	}
	rr := r
	rr.sub = ""
	mv := s.cardValue(rr, 90, my, 300, 210, true)
	if s.set == "cyberpunk" {
		mv.Color = cyberYellow
	}
	l.Widgets = append(append(l.Widgets, mv), s.cardIcon(r, "label")...)
	xs := [2]int{105, 245}
	for i, sr := range d.stats[1:] {
		l.Widgets = append(l.Widgets, s.thresholds(sr, lcd.Widget{Type: "value", Align: "center", X: xs[i], Y: subY, W: 130, H: 72,
			ValueSize: cardSubValue, Uniform: true, Tint: true, Smooth: true}))
	}
	return l
}

func comboDesigns() map[string]lcd.Layout {
	out := map[string]lcd.Layout{}
	for _, s := range tileStyles {
		for _, d := range comboDefs() {
			out[comboID(s, d)] = s.combo(d)
		}
	}
	return out
}
