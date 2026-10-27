package app

import "github.com/kewe87/casehud/internal/lcd"

// Sensor source expressions shared by the built-in designs.
const (
	srcCPULoad   = "Load/CPU Total"
	srcCPUTemp   = "Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)|Temperatures/Core (Tctl)"
	srcGPULoad   = "Load/GPU Core|Load/D3D 3D"
	srcGPUTemp   = "Temperatures/GPU Core|Temperatures/GPU Package"
	srcCoolant   = "Temperatures/T Sensor|Temperatures/Coolant|Temperatures/Water|Temperatures/Liquid"
	srcPump      = "Fans/Pump|Fans/AIO Pump|Fans/W_PUMP+"
	srcFPS       = "FPS/Framerate"
	srcFrametime = "FPS/Frametime"
	srcLatency   = "FPS/Latency"
	srcCPUPower  = "Powers/CPU Package|Powers/Package"
	srcGPUPower  = "Powers/GPU Package|Powers/GPU Power|Powers/GPU Core"
	srcPowerSum  = srcCPUPower + " + " + srcGPUPower
	srcGPUHot    = "Temperatures/GPU Hot Spot|Temperatures/GPU Hotspot"
	srcVRAMUsed  = "Data/GPU Memory Used|Data/D3D Dedicated Memory Used"
	srcVRAMTotal = "Data/GPU Memory Total"
	srcVRAMLoad  = "Load/GPU Memory"
	colorCPU     = "#22d3ee"
	colorGPU     = "#76e04a"
	colorWater   = "#3b82f6"
	colorAccent  = "#a78bfa"
)

func base(name string, round bool) lcd.Layout {
	l := lcd.DefaultLayout()
	l.Name, l.Round = name, round
	if round {
		l.Panel = "" // tiles get clipped on round screens
	}
	l.Widgets = nil
	return l
}

// ring is a big round gauge that fills a round screen, with glow and a
// color that fades towards warn/crit.
func ring(name, label, src, sub, color string, min, max, warn, crit float64, unit, subUnit string, decimals int) lcd.Layout {
	l := base(name, true)
	l.Widgets = []lcd.Widget{{Type: "gauge", X: 10, Y: 10, W: 460, H: 460, Label: label,
		Source: src, Unit: unit, Min: min, Max: max, Warn: warn, Crit: crit, Color: color, Decimals: decimals,
		SubSource: sub, SubUnit: subUnit, Thickness: 0.055, Glow: true, Smooth: true}}
	return l
}

// dualRing puts reading a on an outer and b on an inner ring, with both
// numbers centred and tinted to match. Labels are optional.
func dualRing(l lcd.Layout, aLabel, aSrc, bLabel, bSrc, unit, aColor, bColor string, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit float64) lcd.Layout {
	l.Widgets = []lcd.Widget{
		{Type: "gauge", X: 8, Y: 8, W: 464, H: 464, Source: aSrc, Min: aMin, Max: aMax, Warn: aWarn, Crit: aCrit,
			Color: aColor, Thickness: 0.05, Glow: true, Smooth: true, RingOnly: true},
		{Type: "gauge", X: 70, Y: 74, W: 340, H: 340, Source: bSrc, Min: bMin, Max: bMax, Warn: bWarn, Crit: bCrit,
			Color: bColor, Thickness: 0.065, Glow: true, Smooth: true, RingOnly: true},
	}
	l.Widgets = append(l.Widgets, centredPair(aLabel, aSrc, bLabel, bSrc, unit, aColor, bColor, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit)...)
	return l
}

// centredPair stacks two tinted numbers in the middle of a round screen.
func centredPair(aLabel, aSrc, bLabel, bSrc, unit, aColor, bColor string, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit float64) []lcd.Widget {
	y1, y2, h := 156, 268, 160
	if aLabel != "" {
		y1, y2, h = 132, 254, 158
	}
	return []lcd.Widget{
		{Type: "value", Align: "center", X: 130, Y: y1, W: 220, H: h, Label: aLabel, Source: aSrc, Unit: unit,
			Min: aMin, Max: aMax, Warn: aWarn, Crit: aCrit, Smooth: true, Color: aColor, Tint: true},
		{Type: "value", Align: "center", X: 130, Y: y2, W: 220, H: h, Label: bLabel, Source: bSrc, Unit: unit,
			Min: bMin, Max: bMax, Warn: bWarn, Crit: bCrit, Smooth: true, Color: bColor, Tint: true},
	}
}

// ringValue is a ring for one reading (e.g. VRAM %) around a centred
// number from another (e.g. VRAM in GB) with an optional second value.
func ringValue(l lcd.Layout, gauge lcd.Widget, label, src, unit string, decimals int, sub, subUnit string) lcd.Layout {
	gauge.RingOnly = true
	l.Widgets = []lcd.Widget{gauge,
		{Type: "value", Align: "center", X: 90, Y: 110, W: 300, H: 260, Label: label, Source: src, Unit: unit, Decimals: decimals,
			Color: gauge.Color, SubSource: sub, SubUnit: subUnit},
	}
	return l
}

// wideDashboard is the Dashboard for the 1920x480 bar: the four tiles of the
// square one side by side, each about as big as the whole square screen, so
// the numbers stay readable from a distance.
func wideDashboard(vertical bool) lcd.Layout {
	l := lcd.DefaultLayout()
	l.Name, l.Width, l.Height = "Dashboard wide", 1920, 480
	if vertical {
		l.Name, l.Width, l.Height = "Dashboard upright", 480, 1920
	}
	// at returns the origin of the i-th 480x480 square.
	at := func(i int) (int, int) {
		if vertical {
			return 0, 480 * i
		}
		return 480 * i, 0
	}
	ring := func(i int, label, src, sub, color string) []lcd.Widget {
		x, y := at(i)
		x, y = x+12, y+12
		return []lcd.Widget{
			{Type: "gauge", X: x, Y: y, W: 456, H: 456, Source: src, Max: 100, Warn: 85, Crit: 97, Color: color, RingOnly: true},
			{Type: "value", Align: "center", X: x + 70, Y: y + 98, W: 316, H: 270, Label: label, Source: src, Unit: "%",
				Warn: 85, Crit: 97, Color: color, SubSource: sub, SubUnit: "°"},
		}
	}
	tile := func(i int, label, src, unit string, dec int, warn, crit float64, color, sub, subUnit string) lcd.Widget {
		x, y := at(i)
		return lcd.Widget{Type: "value", Align: "center", X: x + 12, Y: y + 12, W: 456, H: 456, Label: label, Source: src, Unit: unit, Decimals: dec,
			Warn: warn, Crit: crit, Color: color, Tint: true, SubSource: sub, SubUnit: subUnit}
	}
	cs := tileStyles[0]
	// every field is followed by its inline icon (chip, pump, power)
	add := func(kind string, ws ...lcd.Widget) {
		l.Widgets = append(append(l.Widgets, ws...), cs.ringIcon(tileReadings[kind])...)
	}
	add("cpu", ring(0, "CPU", srcCPULoad, srcCPUTemp, colorCPU)...)
	add("gpu", ring(1, "GPU", srcGPULoad, srcGPUTemp, colorGPU)...)
	add("coolant", tile(2, "COOLANT", srcCoolant, "°", 1, 38, 42, colorWater, srcPump, " rpm"))
	add("power", tile(3, "POWER", srcGPUPower, "W", 0, 0, 0, colorGPU, srcCPUPower, " W CPU"))
	return l
}

// wideRow lines up square designs side by side on a 480-high bar; colours and
// style come from the first one.
func wideRow(name string, vertical bool, parts ...lcd.Layout) lcd.Layout {
	l := parts[0]
	l.Name, l.Round = name, false
	l.Width, l.Height = 480*len(parts), 480
	if vertical {
		l.Name, l.Width, l.Height = name+" upright", 480, 480*len(parts)
	}
	l.Widgets = nil
	for i, p := range parts {
		for _, w := range p.ScaledTo(480, 480).Widgets {
			if vertical {
				w.Y += 480 * i
			} else {
				w.X += 480 * i
			}
			l.Widgets = append(l.Widgets, w)
		}
	}
	return l
}

// wideParts are the four-in-a-row designs of the 1920x480 bar and the square
// designs they are built from.
var wideParts = map[string][4]string{
	"wide-rings": {"cpu-temp", "gpu-temp", "loop", "power-total"},
	"neon-wide":  {"neon-cpu", "neon-gpu", "neon-loop", "neon-power-total"},
	"retro-wide": {"retro-cpu", "retro-gpu", "retro-loop", "retro-power-total"},
	"cyber-wide": {"cyber-cpu", "cyber-gpu", "cyber-loop", "cyber-power-total"},
}

// uprightDesign is the stacked version of a fixed landscape bar design (for
// the bar mounted upright, 480x1920); false for any other design.
func uprightDesign(id string) (lcd.Layout, bool) {
	if id == "water-wide" {
		return wideDashboard(true), true
	}
	parts, ok := wideParts[id]
	if !ok {
		return lcd.Layout{}, false
	}
	all := builtinDesigns()
	return withWideIcons(wideRow("Wide", true, all[parts[0]], all[parts[1]], all[parts[2]], all[parts[3]]), true), true
}

// builtinDesigns are always available; users can copy and edit them.
// Rules for the round fan screens (~2", viewed from 50-80 cm): one big
// number (>= 120 px in the 480 space), at most one secondary value
// (>= 44 px), labels >= 32 px, neutral English text on the panel.
func builtinDesigns() map[string]lcd.Layout {
	// Dashboard for the big square pump screen: CPU and GPU rings on top,
	// coolant, pump and GPU power as large tiles below (readable from a
	// distance, no small graph labels).
	water := lcd.DefaultLayout()
	water.Name = "Dashboard"
	ringTile := func(x int, label, src, sub, color string) []lcd.Widget {
		return []lcd.Widget{
			{Type: "gauge", X: x, Y: 12, W: 222, H: 222, Source: src, Max: 100, Warn: 85, Crit: 97, Color: color, RingOnly: true},
			{Type: "value", Align: "center", X: x + 36, Y: 56, W: 150, H: 140, Label: label, Source: src, Unit: "%",
				Warn: 85, Crit: 97, Color: color, SubSource: sub, SubUnit: "°"},
		}
	}
	water.Widgets = append(append(ringTile(12, "CPU", srcCPULoad, srcCPUTemp, colorCPU), ringTile(246, "GPU", srcGPULoad, srcGPUTemp, colorGPU)...),
		[]lcd.Widget{
			{Type: "value", Align: "center", X: 12, Y: 246, W: 222, H: 222, Label: "COOLANT", Source: srcCoolant, Unit: "°", Decimals: 1,
				Warn: 38, Crit: 42, Color: colorWater, Tint: true, SubSource: srcPump, SubUnit: " rpm"},
			{Type: "value", Align: "center", X: 246, Y: 246, W: 222, H: 222, Label: "GPU POWER", Source: srcGPUPower, Unit: "W",
				Color: colorGPU, Tint: true, SubSource: srcCPUPower, SubUnit: " W CPU"},
		}...)

	// CPU + GPU temperatures as concentric rings, numbers tinted to match.
	combo := func(name, cpuSrc, gpuSrc, unit string, cpuMin, cpuMax, cpuWarn, cpuCrit, gpuMin, gpuMax, gpuWarn, gpuCrit float64) lcd.Layout {
		return dualRing(base(name, true), "CPU", cpuSrc, "GPU", gpuSrc, unit, colorCPU, colorGPU, cpuMin, cpuMax, cpuWarn, cpuCrit, gpuMin, gpuMax, gpuWarn, gpuCrit)
	}

	gaming := base("Gaming", true)
	gaming.Widgets = []lcd.Widget{
		{Type: "gauge", X: 10, Y: 10, W: 460, H: 460, Source: srcFPS, Max: 144, Warn: 60, Crit: 30, Invert: true,
			Color: colorAccent, Thickness: 0.055, Glow: true, Smooth: true, RingOnly: true},
		{Type: "value", Align: "center", X: 90, Y: 100, W: 300, H: 280, Label: "FPS", Source: srcFPS, Unit: "none",
			Warn: 60, Crit: 30, Invert: true, Smooth: true,
			Color: colorAccent, SubSource: srcLatency + "|" + srcFrametime, SubUnit: " ms", SubDecimals: 0},
	}

	minimal := base("Minimal", true)
	minimal.Widgets = []lcd.Widget{
		{Type: "gauge", X: 10, Y: 10, W: 460, H: 460, Source: srcCPUTemp, Min: 30, Max: 95, Warn: 85, Crit: 95,
			Color: "#f1f4fa", Thickness: 0.02, RingOnly: true},
		{Type: "value", Align: "center", X: 90, Y: 130, W: 300, H: 220, Label: "CPU", Source: srcCPUTemp, Unit: "°", Color: "#8a94a8"},
	}

	clock := base("Clock", true)
	clock.Widgets = []lcd.Widget{
		{Type: "clock", X: 40, Y: 140, W: 400, H: 150, Text: "15:04", Size: 150},
		{Type: "clock", X: 40, Y: 300, W: 400, H: 50, Text: "Mon 02.01.", Size: 40, Color: "#8a94a8"},
	}

	temps := base("Temperatures", true)
	temps.Widgets = []lcd.Widget{
		{Type: "bar", X: 72, Y: 70, W: 336, H: 116, Label: "CPU", Source: srcCPUTemp, Unit: "°", Min: 30, Max: 95, Warn: 85, Crit: 95, Smooth: true, Color: colorCPU},
		{Type: "bar", X: 72, Y: 182, W: 336, H: 116, Label: "GPU", Source: srcGPUTemp, Unit: "°", Min: 30, Max: 90, Warn: 80, Crit: 88, Smooth: true, Color: colorGPU},
		{Type: "bar", X: 72, Y: 294, W: 336, H: 116, Label: "COOLANT", Source: srcCoolant, Unit: "°", Decimals: 1, Min: 20, Max: 45, Warn: 38, Crit: 42, Smooth: true, Color: colorWater},
	}

	designs := map[string]lcd.Layout{
		"cpu-gpu":       combo("CPU + GPU", srcCPUTemp, srcGPUTemp, "°", 30, 95, 85, 95, 30, 90, 80, 88),
		"cpu-gpu-load":  combo("CPU + GPU load", srcCPULoad, srcGPULoad, "%", 0, 100, 85, 97, 0, 100, 85, 97),
		"cpu-temp":      ring("CPU", "CPU", srcCPUTemp, srcCPULoad, colorCPU, 30, 95, 85, 95, "°", "%", 0),
		"gpu-temp":      ring("GPU", "GPU", srcGPUTemp, srcGPULoad, colorGPU, 30, 90, 80, 88, "°", "%", 0),
		"loop":          ring("Coolant", "COOLANT", srcCoolant, srcPump, colorWater, 20, 45, 38, 42, "°", " rpm", 1),
		"gaming":        gaming,
		"minimal":       minimal,
		"temps":         temps,
		"clock":         clock,
		"water-cooling": water,
		"water-wide":    wideDashboard(false),
		"power":         combo("Power", srcCPUPower, srcGPUPower, "W", 0, 250, 0, 0, 0, 450, 0, 0),
		"power-total":   ring("System power", "SYSTEM", srcPowerSum, "", colorAccent, 0, 700, 0, 0, "W", "", 0),
		"vram": ringValue(base("VRAM", true), lcd.Widget{Type: "gauge", X: 10, Y: 10, W: 460, H: 460, Source: srcVRAMLoad,
			Max: 100, Warn: 90, Crit: 97, Color: colorGPU, Thickness: 0.055, Glow: true, Smooth: true},
			"VRAM", srcVRAMUsed, "GB", 1, srcVRAMTotal, " GB"),
		"gpu-hotspot": dualRing(base("GPU hotspot", true), "CORE", srcGPUTemp, "HOTSPOT", srcGPUHot, "°", colorGPU, "#f59e0b",
			30, 90, 80, 88, 30, 105, 90, 100),
		// Hidden aliases so configs saved with older ids keep working.
		"cpu-gauge": ring("CPU load", "CPU", srcCPULoad, srcCPUTemp, colorCPU, 0, 100, 85, 97, "%", "°", 0),
		"gpu-gauge": ring("GPU load", "GPU", srcGPULoad, srcGPUTemp, colorGPU, 0, 100, 85, 97, "%", "°", 0),
	}
	for id, l := range designs {
		if l.Set == "" {
			l.Set = "clean"
			designs[id] = l
		}
	}
	for id, l := range neonDesigns() {
		designs[id] = l
	}
	for id, l := range retroDesigns() {
		designs[id] = l
	}
	for id, l := range cyberDesigns() {
		designs[id] = l
	}
	// Four-in-a-row variants for the 1920x480 bar, built from each set's
	// single-reading designs.
	for id, parts := range wideParts {
		designs[id] = withWideIcons(wideRow("Wide", false, designs[parts[0]], designs[parts[1]], designs[parts[2]], designs[parts[3]]), false)
	}
	for id, l := range iconDesigns() {
		designs[id] = l
	}
	for id, l := range tileDesigns() {
		if l.Set == "" {
			l.Set = "clean"
		}
		designs[id] = l
	}
	return designs
}

// --- Neon: futuristic HUD with segmented rings, scale ticks and glow ---

const (
	neonCyan    = "#00f0ff"
	neonMagenta = "#ff2bd6"
	neonBlue    = "#4d7cff"
	neonViolet  = "#a855f7"
)

func neonBase(name string) lcd.Layout {
	l := base(name, true)
	l.Set = "neon"
	l.Foreground = "#eaf6ff"
	l.Muted = "#8393b3"
	l.Accent = neonCyan
	return l
}

func neonRing(name, label, src, sub, color string, min, max, warn, crit float64, unit, subUnit string, decimals int, invert bool) lcd.Layout {
	l := neonBase(name)
	l.Widgets = []lcd.Widget{
		{Type: "gauge", X: 22, Y: 22, W: 436, H: 436, Label: label, Source: src, Unit: unit, Decimals: decimals,
			Min: min, Max: max, Warn: warn, Crit: crit, Invert: invert, Color: color, Thickness: 0.06,
			Segments: 30, Ticks: 30, Glow: true, Smooth: true, Tint: true, SubSource: sub, SubUnit: subUnit},
	}
	return l
}

func neonDesigns() map[string]lcd.Layout {
	// CPU (outer) and GPU (inner) as segmented rings with tinted numbers.
	combo := func(name, aLabel, aSrc, bLabel, bSrc, unit, aColor, bColor string, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit float64) lcd.Layout {
		l := neonBase(name)
		l.Widgets = []lcd.Widget{
			{Type: "gauge", X: 14, Y: 14, W: 452, H: 452, Source: aSrc, Min: aMin, Max: aMax, Warn: aWarn, Crit: aCrit,
				Color: aColor, Thickness: 0.045, Segments: 36, Ticks: 36, Glow: true, Smooth: true, RingOnly: true},
			{Type: "gauge", X: 82, Y: 86, W: 316, H: 316, Source: bSrc, Min: bMin, Max: bMax, Warn: bWarn, Crit: bCrit,
				Color: bColor, Thickness: 0.06, Segments: 24, Glow: true, Smooth: true, RingOnly: true},
		}
		pair := centredPair(aLabel, aSrc, bLabel, bSrc, unit, aColor, bColor, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit)
		for i := range pair {
			pair[i].X, pair[i].W = 140, 200
		}
		l.Widgets = append(l.Widgets, pair...)
		return l
	}
	vramRing := neonRing("VRAM", "", srcVRAMLoad, "", neonMagenta, 0, 100, 90, 97, "%", "", 0, false).Widgets[0]
	vramRing.Label = ""
	return map[string]lcd.Layout{
		"neon-cpu-gpu":      combo("CPU + GPU", "CPU", srcCPUTemp, "GPU", srcGPUTemp, "°", neonCyan, neonMagenta, 30, 95, 85, 95, 30, 90, 80, 88),
		"neon-cpu-gpu-load": combo("CPU + GPU load", "CPU", srcCPULoad, "GPU", srcGPULoad, "%", neonCyan, neonMagenta, 0, 100, 85, 97, 0, 100, 85, 97),
		"neon-power":        combo("Power", "CPU", srcCPUPower, "GPU", srcGPUPower, "W", neonCyan, neonMagenta, 0, 250, 0, 0, 0, 450, 0, 0),
		"neon-power-total":  neonRing("System power", "SYSTEM", srcPowerSum, "", neonViolet, 0, 700, 0, 0, "W", "", 0, false),
		"neon-vram":         ringValue(neonBase("VRAM"), vramRing, "VRAM", srcVRAMUsed, "GB", 1, srcVRAMTotal, " GB"),
		"neon-gpu-hotspot":  combo("GPU hotspot", "CORE", srcGPUTemp, "HOTSPOT", srcGPUHot, "°", neonMagenta, neonViolet, 30, 90, 80, 88, 30, 105, 90, 100),
		"neon-cpu":          neonRing("CPU", "CPU", srcCPUTemp, srcCPULoad, neonCyan, 30, 95, 85, 95, "°", "%", 0, false),
		"neon-gpu":          neonRing("GPU", "GPU", srcGPUTemp, srcGPULoad, neonMagenta, 30, 90, 80, 88, "°", "%", 0, false),
		"neon-loop":         neonRing("Coolant", "COOLANT", srcCoolant, srcPump, neonBlue, 20, 45, 38, 42, "°", " rpm", 1, false),
		"neon-gaming":       neonRing("Gaming", "FPS", srcFPS, srcLatency+"|"+srcFrametime, neonViolet, 0, 144, 60, 30, "none", " ms", 0, true),
		"neon-clock":        neonClock(),
	}
}

// --- Retro: amber CRT terminal with LED segment meters and scanlines ---

const (
	retroAmber = "#ffb000"
	retroDim   = "#bd7d00" // >= 4.5:1 on the CRT black
)

func retroBase(name string) lcd.Layout {
	l := base(name, true)
	l.Set = "retro"
	l.Background = "#0b0700"
	l.Foreground = retroAmber
	l.Muted = retroDim
	l.Accent = retroAmber
	l.Scanlines = true
	return l
}

// retroSingle: big value on top, an LED meter for a second reading below.
func retroSingle(name, label, src, unit string, decimals int, warn, crit float64, invert bool, barLabel, barSrc, barUnit string, barMin, barMax float64) lcd.Layout {
	l := retroBase(name)
	l.Widgets = []lcd.Widget{
		{Type: "value", Align: "center", X: 60, Y: 66, W: 360, H: 220, Label: label, Source: src, Unit: unit, Decimals: decimals,
			Warn: warn, Crit: crit, Invert: invert, Color: retroAmber, Tint: true},
		{Type: "bar", X: 84, Y: 290, W: 312, H: 124, Label: barLabel, Source: barSrc, Unit: barUnit,
			Min: barMin, Max: barMax, Segments: 12, Color: retroAmber},
	}
	return l
}

func retroDesigns() map[string]lcd.Layout {
	// Two readings stacked, one per chip, labels on top.
	dual := func(name, aLabel, aSrc, bLabel, bSrc, unit string, aWarn, aCrit, bWarn, bCrit float64) lcd.Layout {
		l := retroBase(name)
		l.Widgets = []lcd.Widget{
			{Type: "value", Align: "center", X: 80, Y: 56, W: 320, H: 180, Label: aLabel, Source: aSrc, Unit: unit,
				Warn: aWarn, Crit: aCrit, Color: retroAmber, Tint: true},
			{Type: "value", Align: "center", X: 80, Y: 244, W: 320, H: 180, Label: bLabel, Source: bSrc, Unit: unit,
				Warn: bWarn, Crit: bCrit, Color: retroAmber, Tint: true},
		}
		return l
	}
	// FPS with latency (or frametime) underneath; one secondary value only.
	gaming := retroBase("Gaming")
	gaming.Widgets = []lcd.Widget{{Type: "value", Align: "center", X: 60, Y: 90, W: 360, H: 300, Label: "FPS", Source: srcFPS,
		Unit: "none", Warn: 60, Crit: 30, Invert: true, Color: retroAmber, Tint: true,
		SubSource: srcLatency + "|" + srcFrametime, SubUnit: " ms"}}
	return map[string]lcd.Layout{
		"retro-cpu-gpu":      dual("CPU + GPU", "CPU", srcCPUTemp, "GPU", srcGPUTemp, "°", 85, 95, 80, 88),
		"retro-cpu-gpu-load": dual("CPU + GPU load", "CPU", srcCPULoad, "GPU", srcGPULoad, "%", 85, 97, 85, 97),
		"retro-power":        dual("Power", "CPU", srcCPUPower, "GPU", srcGPUPower, "W", 0, 0, 0, 0),
		"retro-power-total":  retroSingle("System power", "SYSTEM", srcPowerSum, "W", 0, 0, 0, false, "GPU", srcGPUPower, "W", 0, 450),
		"retro-vram":         retroSingle("VRAM", "VRAM", srcVRAMUsed, "GB", 1, 0, 0, false, "LOAD", srcVRAMLoad, "%", 0, 100),
		"retro-gpu-hotspot":  dual("GPU hotspot", "CORE", srcGPUTemp, "HOTSPOT", srcGPUHot, "°", 80, 88, 90, 100),
		"retro-cpu":          retroSingle("CPU", "CPU", srcCPUTemp, "°", 0, 85, 95, false, "LOAD", srcCPULoad, "%", 0, 100),
		"retro-gpu":          retroSingle("GPU", "GPU", srcGPUTemp, "°", 0, 80, 88, false, "LOAD", srcGPULoad, "%", 0, 100),
		"retro-loop":         retroSingle("Coolant", "COOLANT", srcCoolant, "°", 1, 38, 42, false, "PUMP", srcPump, " rpm", 0, 3000),
		"retro-gaming":       gaming,
		"retro-clock":        retroClock(),
	}
}

// retroClock: big amber digits with the date underneath.
func retroClock() lcd.Layout {
	l := retroBase("Clock")
	l.Widgets = []lcd.Widget{
		{Type: "clock", X: 40, Y: 150, W: 400, H: 170, Text: "15:04", Size: 156, Color: retroAmber},
		{Type: "clock", X: 60, Y: 322, W: 360, H: 48, Text: "Mon 02.01.", Size: 40, Color: retroDim},
	}
	return l
}

// BuiltinOrder is how built-ins are listed (grouped by set in the GUI);
// ids not listed are hidden aliases.
var BuiltinOrder = append([]string{
	"cpu-gpu", "cpu-gpu-load", "cpu-temp", "gpu-temp", "loop", "gaming", "power", "power-total", "vram", "gpu-hotspot",
	"minimal", "temps", "clock", "water-cooling", "water-wide", "wide-rings",
	"neon-cpu-gpu", "neon-cpu-gpu-load", "neon-cpu", "neon-gpu", "neon-loop", "neon-gaming", "neon-power", "neon-power-total", "neon-vram", "neon-gpu-hotspot", "neon-clock", "neon-wide",
	"retro-cpu-gpu", "retro-cpu-gpu-load", "retro-cpu", "retro-gpu", "retro-loop", "retro-gaming", "retro-power", "retro-power-total", "retro-vram", "retro-gpu-hotspot", "retro-clock", "retro-wide",
	"cyber-cpu-gpu", "cyber-cpu-gpu-load", "cyber-cpu", "cyber-gpu", "cyber-loop", "cyber-gaming", "cyber-power", "cyber-power-total", "cyber-vram",
	"cyber-gpu-hotspot", "cyber-clock", "cyber-wide",
}, append(iconDesignOrder(), tileDesignOrder()...)...)

// FanSets are one-click trios for a row of LCD fans (CPU, GPU, coolant),
// one per design set so all fans share a style.
var FanSets = map[string][]string{
	"clean":     {"cpu-temp", "gpu-temp", "loop"},
	"neon":      {"neon-cpu", "neon-gpu", "neon-loop"},
	"retro":     {"retro-cpu", "retro-gpu", "retro-loop"},
	"cyberpunk": {"cyber-cpu", "cyber-gpu", "cyber-loop"},
}

// neonClock: a glowing seconds ring in magenta around white digits.
func neonClock() lcd.Layout {
	l := neonBase("Clock")
	l.Widgets = []lcd.Widget{
		{Type: "gauge", X: 22, Y: 22, W: 436, H: 436, Source: "Clock/Seconds", Min: 0, Max: 60, Color: neonMagenta,
			Thickness: 0.035, Segments: 60, Glow: true, RingOnly: true, Sweep: 360},
		{Type: "clock", X: 60, Y: 148, W: 360, H: 150, Text: "15:04", Size: 140, Color: "#eaf6ff"},
		{Type: "clock", X: 70, Y: 300, W: 340, H: 46, Text: "Mon 02.01.", Size: 40, Color: neonCyan},
	}
	return l
}

// --- Cyberpunk: yellow-on-black terminal HUD with a red glitch offset ---

const (
	cyberYellow = "#fcee0a"
	cyberCyan   = "#00f0ff"
	cyberRed    = "#ff003c"
	cyberGhost  = "#5c0016" // cyberRed at ~35 % on black
)

func cyberBase(name string) lcd.Layout {
	l := base(name, true)
	l.Set = "cyberpunk"
	l.Background = "#050508"
	l.Foreground = cyberYellow
	l.Muted = "#c2be5c"
	l.Accent = cyberCyan
	return l
}

// cyberRing is a thin segmented ring with a red copy shifted a few pixels
// (a chromatic-aberration glitch), the number in yellow and a cyan second
// value.
func cyberRing(name, label, src, sub string, min, max, warn, crit float64, unit, subUnit string, decimals int, invert bool) lcd.Layout {
	l := cyberBase(name)
	ring := lcd.Widget{Type: "gauge", X: 18, Y: 18, W: 444, H: 444, Source: src, Min: min, Max: max, Warn: warn, Crit: crit,
		Invert: invert, Thickness: 0.04, Segments: 48, Ticks: 24, RingOnly: true}
	ghost := ring
	// A dim red copy 3 px off reads as a chromatic glitch, not as an alarm.
	ghost.X, ghost.Y, ghost.Color, ghost.Ticks = 21, 15, cyberGhost, 0
	ring.Color = cyberYellow
	l.Widgets = []lcd.Widget{ghost, ring,
		{Type: "value", Align: "center", X: 100, Y: 120, W: 280, H: 260, Label: label, Source: src, Unit: unit, Decimals: decimals,
			Min: min, Max: max, Warn: warn, Crit: crit, Invert: invert, Color: cyberYellow, Tint: true, SubSource: sub, SubUnit: subUnit},
	}
	return l
}

// cyberDual stacks two readings in yellow and cyan inside a decorative
// segmented frame ring.
func cyberDual(name, aLabel, aSrc, bLabel, bSrc, unit string, aMin, aMax, aWarn, aCrit, bMin, bMax, bWarn, bCrit float64) lcd.Layout {
	l := cyberBase(name)
	l.Widgets = []lcd.Widget{
		// Outer ring = first reading (yellow), inner ring = second (cyan).
		{Type: "gauge", X: 18, Y: 18, W: 444, H: 444, Source: aSrc, Min: aMin, Max: aMax, Warn: aWarn, Crit: aCrit,
			Color: cyberYellow, Thickness: 0.03, Segments: 48, Ticks: 24, RingOnly: true},
		{Type: "gauge", X: 62, Y: 62, W: 356, H: 356, Source: bSrc, Min: bMin, Max: bMax, Warn: bWarn, Crit: bCrit,
			Color: cyberCyan, Thickness: 0.035, Segments: 36, RingOnly: true},
		{Type: "value", Align: "center", X: 120, Y: 116, W: 240, H: 146, Label: aLabel, Source: aSrc, Unit: unit,
			Warn: aWarn, Crit: aCrit, Color: cyberYellow, Tint: true},
		{Type: "value", Align: "center", X: 120, Y: 248, W: 240, H: 146, Label: bLabel, Source: bSrc, Unit: unit,
			Warn: bWarn, Crit: bCrit, Color: cyberCyan, Tint: true},
	}
	return l
}

func cyberDesigns() map[string]lcd.Layout {
	vram := cyberRing("VRAM", "VRAM", srcVRAMLoad, "", 0, 100, 90, 97, "%", "", 0, false)
	vram.Widgets[2] = lcd.Widget{Type: "value", Align: "center", X: 100, Y: 120, W: 280, H: 260, Label: "VRAM", Source: srcVRAMUsed,
		Unit: "GB", Decimals: 1, Color: cyberYellow, Tint: true, SubSource: srcVRAMTotal, SubUnit: " GB"}
	return map[string]lcd.Layout{
		"cyber-cpu-gpu":      cyberDual("CPU + GPU", "CPU", srcCPUTemp, "GPU", srcGPUTemp, "°", 30, 95, 85, 95, 30, 90, 80, 88),
		"cyber-cpu-gpu-load": cyberDual("CPU + GPU load", "CPU", srcCPULoad, "GPU", srcGPULoad, "%", 0, 100, 85, 97, 0, 100, 85, 97),
		"cyber-power":        cyberDual("Power", "CPU", srcCPUPower, "GPU", srcGPUPower, "W", 0, 250, 0, 0, 0, 450, 0, 0),
		"cyber-power-total":  cyberRing("System power", "SYSTEM", srcPowerSum, "", 0, 700, 0, 0, "W", "", 0, false),
		"cyber-vram":         vram,
		"cyber-gpu-hotspot":  cyberDual("GPU hotspot", "CORE", srcGPUTemp, "HOTSPOT", srcGPUHot, "°", 30, 90, 80, 88, 30, 105, 90, 100),
		"cyber-cpu":          cyberRing("CPU", "CPU", srcCPUTemp, srcCPULoad, 30, 95, 85, 95, "°", "%", 0, false),
		"cyber-gpu":          cyberRing("GPU", "GPU", srcGPUTemp, srcGPULoad, 30, 90, 80, 88, "°", "%", 0, false),
		"cyber-loop":         cyberRing("Coolant", "COOLANT", srcCoolant, srcPump, 20, 45, 38, 42, "°", " rpm", 1, false),
		"cyber-gaming":       cyberRing("Gaming", "FPS", srcFPS, srcLatency+"|"+srcFrametime, 0, 144, 60, 30, "none", " ms", 0, true),
		"cyber-clock":        cyberClock(),
	}
}

// cyberClock: the seconds run round a 60-segment ring with the red glitch
// copy, the time in yellow and the date in cyan.
func cyberClock() lcd.Layout {
	l := cyberBase("Clock")
	ring := lcd.Widget{Type: "gauge", X: 18, Y: 18, W: 444, H: 444, Source: "Clock/Seconds", Min: 0, Max: 60,
		Thickness: 0.04, Segments: 60, Ticks: 60, RingOnly: true, Sweep: 360}
	ghost := ring
	ghost.X, ghost.Y, ghost.Color, ghost.Ticks = 21, 15, cyberGhost, 0
	ring.Color = cyberYellow
	l.Widgets = []lcd.Widget{ghost, ring,
		{Type: "clock", X: 70, Y: 150, W: 340, H: 140, Text: "15:04", Size: 132, Color: cyberYellow},
		{Type: "clock", X: 90, Y: 292, W: 300, H: 44, Text: "Mon 02.01.", Size: 40, Color: cyberCyan},
	}
	return l
}
