package app

import "github.com/kewe87/casehud/internal/lcd"

// Round designs with a big animated icon: for the LCD fans and the round
// screens, in every design set. The icon sits in the upper half of the
// circle, label, main value and an optional second value below it. They are
// ordinary designs ("CPU · Icon") next to the ring designs: the choice is
// visible in the design picker with a preview, and nothing sits behind
// numbers.
//
// Safe area of the 480 px circle: the icon (176 px) starts 30 px from the
// top, so its corners are 224 px from the centre; the text block ends 36 px
// above the bottom edge where the circle is still 224 px wide.

const srcFan = "Fans/CPU Fan|Fans/Fan #1|Fans/System Fan #1|Fans/Chassis Fan #1"

// iconKinds are the readings that exist as icon designs, in picker order.
var iconKinds = []string{"coolant", "cpu", "gpu", "fps", "power", "fan", "clock"}

var iconKindNames = map[string]string{
	"coolant": "Coolant", "cpu": "CPU", "gpu": "GPU", "fps": "FPS", "power": "Power", "fan": "Fan", "clock": "Clock",
}

// iconReading is the main reading of an icon design and the icon it shows.
func iconReading(kind string) (r tileReading, icon iconSpec) {
	switch kind {
	case "fan":
		r = tileReading{label: "FAN RPM", role: "accent", src: srcFan, unit: "none", max: 2500}
		return r, iconSpec{icon: "fan", src: srcFan, max: 2500, role: "accent"}
	case "power":
		r = tileReadings["power"]
		r.label, r.subUnit = "POWER", " W CPU"
	default:
		r = tileReadings[kind]
		if kind == "cpu" || kind == "gpu" {
			r.subLabel = ""
		}
	}
	sp, _ := iconSpecFor(r)
	return r, sp
}

// iconDesign is the icon design of kind in style s.
func (s tileStyle) iconDesign(kind string) lcd.Layout {
	l := s.base(iconKindNames[kind] + " · Icon")
	l.Round, l.Set, l.Width, l.Height = true, s.set, 480, 480
	if s.set == "clean" {
		l.Panel = ""
	}
	if kind == "clock" {
		sp := iconSpec{icon: "clock", role: "accent"}
		ic := s.iconWidget(sp, 152, 30, 176)
		fg := s.color("accent")
		if s.set == "clean" {
			fg = ""
		}
		l.Widgets = []lcd.Widget{ic,
			{Type: "clock", X: 60, Y: 226, W: 360, H: 130, Text: "15:04", Size: 118, Color: fg},
			{Type: "clock", X: 80, Y: 362, W: 320, H: 48, Text: "Mon 02.01.", Size: 40, Color: s.dateColor()}}
		return l
	}
	r, sp := iconReading(kind)
	vy, vh := 222, 222
	if kind == "fan" {
		vy, vh = 236, 208 // no second line: sit lower to centre the stack
	}
	v := s.thresholds(r, lcd.Widget{Type: "value", Align: "center", X: 90, Y: vy, W: 300, H: vh, Label: r.label,
		Tint: true, Smooth: true, SubSource: r.sub, SubUnit: r.subUnit, SubDecimals: r.subDec})
	if r.unit == "none" {
		v.Unit = "none"
	}
	l.Widgets = []lcd.Widget{s.iconWidget(sp, 152, 30, 176), v}
	return l
}

func iconDesigns() map[string]lcd.Layout {
	out := map[string]lcd.Layout{}
	for _, s := range tileStyles {
		for _, k := range iconKinds {
			out[s.prefix+"icon-"+k] = s.iconDesign(k)
		}
	}
	return out
}

// iconDesignOrder lists the ids of iconDesigns in picker order.
func iconDesignOrder() []string {
	var ids []string
	for _, s := range tileStyles {
		for _, k := range iconKinds {
			ids = append(ids, s.prefix+"icon-"+k)
		}
	}
	return ids
}
