package leds

import (
	"image"
	"image/color"
	"math"
)

// BuiltinEffects returns the effects that ship with the package.
func BuiltinEffects() []EffectInfo {
	return append(append(baseEffects(), motionEffects()...), fireEffect())
}

func baseEffects() []EffectInfo {
	return []EffectInfo{
		{
			Name: "rainbow", Label: "Rainbow",
			Params: []Param{
				{Key: "speed", Label: "Speed", Type: ParamNumber, Min: 0, Max: 100, Default: 50.0},
				{Key: "scale", Label: "Width", Type: ParamNumber, Min: 10, Max: 400, Default: 100.0},
				{Key: "vertical", Label: "Vertical", Type: ParamBool, Default: false},
			},
			New: func(p Params) (Effect, error) {
				return &rainbow{speed: p.Float("speed"), scale: p.Float("scale"), vertical: p.Bool("vertical")}, nil
			},
		},
		{
			Name: "solid", Label: "Solid color",
			Params: []Param{
				{Key: "color", Label: "Color", Type: ParamColor, Default: "#3a8dff"},
				{Key: "useAccent", Label: "Use design accent", Type: ParamBool, Default: false},
			},
			New: func(p Params) (Effect, error) {
				return &solid{c: p.Color("color"), accent: p.Bool("useAccent")}, nil
			},
		},
		{
			Name: "breathing", Label: "Breathing",
			Params: []Param{
				{Key: "color", Label: "Color", Type: ParamColor, Default: "#3a8dff"},
				{Key: "useAccent", Label: "Use design accent", Type: ParamBool, Default: false},
				{Key: "period", Label: "Period (s)", Type: ParamNumber, Min: 0.5, Max: 20, Default: 4.0},
			},
			New: func(p Params) (Effect, error) {
				return &breathing{c: p.Color("color"), accent: p.Bool("useAccent"), period: math.Max(p.Float("period"), 0.5)}, nil
			},
		},
		{
			Name: "wave", Label: "Color wave",
			Params: []Param{
				{Key: "color", Label: "Color", Type: ParamColor, Default: "#3a8dff"},
				{Key: "background", Label: "Background", Type: ParamColor, Default: "#000000"},
				{Key: "useAccent", Label: "Use design accent", Type: ParamBool, Default: true},
				{Key: "speed", Label: "Speed", Type: ParamNumber, Min: 0, Max: 100, Default: 40.0},
				{Key: "width", Label: "Width", Type: ParamNumber, Min: 5, Max: 100, Default: 30.0},
			},
			New: func(p Params) (Effect, error) {
				return &wave{c: p.Color("color"), bg: p.Color("background"), accent: p.Bool("useAccent"),
					speed: p.Float("speed"), width: p.Float("width")}, nil
			},
		},
		{
			Name: "sensor", Label: "Sensor gradient",
			Params: []Param{
				{Key: "sensor", Label: "Sensor path", Type: ParamEnum, Default: ""},
				{Key: "min", Label: "Value for low color", Type: ParamNumber, Min: -1000, Max: 100000, Default: 40.0},
				{Key: "max", Label: "Value for high color", Type: ParamNumber, Min: -1000, Max: 100000, Default: 85.0},
				{Key: "low", Label: "Low color", Type: ParamColor, Default: "#0060ff"},
				{Key: "high", Label: "High color", Type: ParamColor, Default: "#ff2000"},
				{Key: "smoothing", Label: "Smoothing", Type: ParamNumber, Min: 0, Max: 100, Default: 70.0},
			},
			New: func(p Params) (Effect, error) {
				return &sensorGradient{key: p.String("sensor"), lo: p.Float("min"), hi: p.Float("max"),
					low: p.Color("low"), high: p.Color("high"), smooth: p.Float("smoothing"), v: math.NaN()}, nil
			},
		},
		{
			Name: "cycle", Label: "Color cycle",
			Params: []Param{
				{Key: "speed", Label: "Speed", Type: ParamNumber, Min: 0, Max: 100, Default: 20.0},
			},
			New: func(p Params) (Effect, error) { return &cycle{speed: p.Float("speed")}, nil },
		},
	}
}

func fill(c *image.RGBA, col color.RGBA) {
	px := [4]uint8{col.R, col.G, col.B, 255}
	for i := 0; i < len(c.Pix); i += 4 {
		copy(c.Pix[i:i+4], px[:])
	}
}

func set(c *image.RGBA, x, y int, col color.RGBA) {
	o := c.PixOffset(x, y)
	c.Pix[o], c.Pix[o+1], c.Pix[o+2], c.Pix[o+3] = col.R, col.G, col.B, 255
}

// HSV converts hue (degrees), saturation and value (0..1) to RGB.
func HSV(h, s, v float64) color.RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

type rainbow struct {
	speed, scale float64
	vertical     bool
}

func (r *rainbow) Render(f Frame, c *image.RGBA) {
	off := f.T.Seconds() * r.speed * 3.6
	w, h := c.Rect.Dx(), c.Rect.Dy()
	if r.vertical {
		for y := 0; y < h; y++ {
			col := HSV(float64(y)*360/r.scale-off, 1, 1)
			for x := 0; x < w; x++ {
				set(c, x, y, col)
			}
		}
		return
	}
	for x := 0; x < w; x++ {
		col := HSV(float64(x)*360/r.scale-off, 1, 1)
		for y := 0; y < h; y++ {
			set(c, x, y, col)
		}
	}
}

type solid struct {
	c      color.RGBA
	accent bool
}

func (s *solid) Render(f Frame, c *image.RGBA) {
	if s.accent {
		fill(c, f.Accent)
		return
	}
	fill(c, s.c)
}

type breathing struct {
	c      color.RGBA
	accent bool
	period float64
}

func (b *breathing) Render(f Frame, c *image.RGBA) {
	base := b.c
	if b.accent {
		base = f.Accent
	}
	t := (1 - math.Cos(2*math.Pi*f.T.Seconds()/b.period)) / 2
	fill(c, lerp(color.RGBA{A: 255}, base, 0.05+0.95*t*t))
}

type wave struct {
	c, bg        color.RGBA
	accent       bool
	speed, width float64
}

func (wv *wave) Render(f Frame, c *image.RGBA) {
	fg := wv.c
	if wv.accent {
		fg = f.Accent
	}
	w, h := c.Rect.Dx(), c.Rect.Dy()
	period := float64(w) * (1 + wv.width/100)
	pos := math.Mod(f.T.Seconds()*wv.speed*4, period)
	half := float64(w) * wv.width / 200
	for x := 0; x < w; x++ {
		d := math.Abs(float64(x) - pos)
		d = math.Min(d, period-d)
		t := 0.0
		if d < half {
			t = (1 + math.Cos(math.Pi*d/half)) / 2
		}
		col := lerp(wv.bg, fg, t)
		for y := 0; y < h; y++ {
			set(c, x, y, col)
		}
	}
}

type sensorGradient struct {
	key       string
	lo, hi    float64
	low, high color.RGBA
	smooth    float64
	v         float64
}

func (s *sensorGradient) Render(f Frame, c *image.RGBA) {
	raw, ok := f.Sensors[s.key]
	if !ok {
		if math.IsNaN(s.v) {
			fill(c, s.low)
			return
		}
		raw = s.v
	}
	if math.IsNaN(s.v) {
		s.v = raw
	} else {
		// exponential smoothing, frame-rate independent (time constant up to ~3 s)
		tau := s.smooth / 100 * 3
		a := 1.0
		if tau > 0 {
			a = 1 - math.Exp(-f.Dt.Seconds()/tau)
		}
		s.v += (raw - s.v) * a
	}
	t := 0.0
	if s.hi != s.lo {
		t = (s.v - s.lo) / (s.hi - s.lo)
	}
	fill(c, lerp(s.low, s.high, t))
}

type cycle struct{ speed float64 }

func (cy *cycle) Render(f Frame, c *image.RGBA) {
	fill(c, HSV(f.T.Seconds()*cy.speed*1.8, 1, 1))
}
