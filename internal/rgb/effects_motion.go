package leds

import (
	"image"
	"image/color"
	"math"
)

// Motion effects: a pattern moves along a path through the canvas. The path
// coordinate u (0..1) of every pixel depends on the "path" parameter:
//
//	horizontal  left to right
//	vertical    top to bottom
//	radial      centre outwards
//	around      clockwise around the centre, starting at the top
//
// "mirror" folds the path at its middle, so the pattern runs from both ends
// towards the middle (or outwards, reversed). Every effect is time based and
// independent of the frame rate.

// pathParams are shared by all motion effects.
func pathParams() []Param {
	return []Param{
		{Key: "path", Label: "Path", Type: ParamEnum, Values: []string{"around", "horizontal", "vertical", "radial"}, Default: "around"},
		{Key: "mirror", Label: "Mirror at the middle", Type: ParamBool, Default: false},
		{Key: "reverse", Label: "Reverse direction", Type: ParamBool, Default: false},
		{Key: "speed", Label: "Speed", Type: ParamNumber, Min: 0, Max: 100, Default: 50.0},
	}
}

// paletteParams declares n user colors (color1..colorN) plus how many are used.
func paletteParams(n int, defaults ...string) []Param {
	var ps []Param
	for i := 0; i < n; i++ {
		d := "#ffffff"
		if i < len(defaults) {
			d = defaults[i]
		}
		ps = append(ps, Param{Key: "color" + string(rune('1'+i)), Label: "Color " + string(rune('1'+i)), Type: ParamColor, Default: d})
	}
	if n > 1 {
		ps = append(ps, Param{Key: "colors", Label: "Colors used", Type: ParamNumber, Min: 1, Max: float64(n), Default: float64(n)})
	}
	return ps
}

func palette(p Params, n int) []color.RGBA {
	use := n
	if n > 1 {
		use = int(math.Max(1, math.Min(float64(n), p.Float("colors"))))
	}
	cols := make([]color.RGBA, use)
	for i := range cols {
		cols[i] = p.Color("color" + string(rune('1'+i)))
	}
	return cols
}

// field maps every canvas pixel to its path coordinate; built once per
// canvas size and path settings.
type field struct {
	path        string
	mirror, rev bool
	w, h        int
	u           []float64
}

func newField(p Params) *field {
	return &field{path: p.String("path"), mirror: p.Bool("mirror"), rev: p.Bool("reverse")}
}

func (f *field) coords(c *image.RGBA) []float64 {
	w, h := c.Bounds().Dx(), c.Bounds().Dy()
	if f.u != nil && f.w == w && f.h == h {
		return f.u
	}
	f.w, f.h = w, h
	f.u = make([]float64, w*h)
	cx, cy := float64(w)/2, float64(h)/2
	maxR := math.Hypot(cx, cy)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			var u float64
			switch f.path {
			case "horizontal":
				u = px / float64(w)
			case "vertical":
				u = py / float64(h)
			case "radial":
				u = math.Hypot(px-cx, py-cy) / maxR
			default: // around: clockwise from the top
				a := math.Atan2(px-cx, -(py - cy)) // 0 at the top, clockwise positive
				u = math.Mod(a/(2*math.Pi)+1, 1)
			}
			if f.mirror {
				u = 1 - math.Abs(2*u-1) // 0 at both ends, 1 in the middle
			}
			if f.rev {
				u = 1 - u
			}
			f.u[y*w+x] = u
		}
	}
	return f.u
}

// paint fills the canvas from a function of the path coordinate.
func (f *field) paint(c *image.RGBA, shade func(u float64) color.RGBA) {
	us := f.coords(c)
	for i, u := range us {
		col := shade(u)
		o := i * 4
		c.Pix[o], c.Pix[o+1], c.Pix[o+2], c.Pix[o+3] = col.R, col.G, col.B, 255
	}
}

// rate converts the speed parameter (0..100) to passes per second.
func rate(p Params, slow, fast float64) float64 {
	s := math.Max(0, math.Min(100, p.Float("speed"))) / 100
	return slow * math.Pow(fast/slow, s)
}

func scaleColor(c color.RGBA, v float64) color.RGBA {
	v = math.Max(0, math.Min(1, v))
	return color.RGBA{uint8(float64(c.R)*v + 0.5), uint8(float64(c.G)*v + 0.5), uint8(float64(c.B)*v + 0.5), 255}
}

// smooth is a soft 0..1 ramp.
func smooth(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return x * x * (3 - 2*x)
}

// hash01 is a deterministic pseudo random number in [0,1) for an integer key.
func hash01(k uint64) float64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return float64(k>>11) / float64(1<<53)
}

// motionEffects returns the built-in motion effects.
func motionEffects() []EffectInfo {
	six := []string{"#ff0000", "#00ff00", "#0000ff", "#ffff00", "#00ffff", "#ff00ff"}
	mk := func(name, label string, colors int, defaults []string, extra []Param, build func(p Params, f *field, cols []color.RGBA) func(t float64, c *image.RGBA)) EffectInfo {
		params := append(paletteParams(colors, defaults...), pathParams()...)
		params = append(params, extra...)
		return EffectInfo{Name: name, Label: label, Params: params, New: func(p Params) (Effect, error) {
			f := newField(p)
			return funcEffect(build(p, f, palette(p, colors))), nil
		}}
	}
	return []EffectInfo{
		mk("meteor", "Meteor", 6, six, []Param{{Key: "tail", Label: "Tail length", Type: ParamNumber, Min: 1, Max: 100, Default: 25.0}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, tail := rate(p, 0.15, 2.5), math.Max(0.01, p.Float("tail")/100)
				return func(t float64, c *image.RGBA) {
					pass := t * r
					head := math.Mod(pass, 1)*(1+tail) - 0 // runs fully out before the next pass
					col := cols[int(pass)%len(cols)]
					f.paint(c, func(u float64) color.RGBA {
						d := head - u
						if d < 0 || d > tail {
							return color.RGBA{A: 255}
						}
						return scaleColor(col, math.Pow(1-d/tail, 2.2))
					})
				}
			}),
		mk("fill", "Color fill", 6, six, []Param{{Key: "alternate", Label: "Alternate direction", Type: ParamBool, Default: true}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, alt := rate(p, 0.1, 1.5), p.Bool("alternate")
				return func(t float64, c *image.RGBA) {
					pass := int(t * r)
					front := smooth(math.Mod(t*r, 1))
					cur, prev := cols[pass%len(cols)], cols[(pass+len(cols)-1)%len(cols)]
					flip := alt && pass%2 == 1
					f.paint(c, func(u float64) color.RGBA {
						if flip {
							u = 1 - u
						}
						if u <= front {
							return cur
						}
						return prev
					})
				}
			}),
		mk("burst", "Fill and fade", 6, six, nil,
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r := rate(p, 0.1, 1.2)
				return func(t float64, c *image.RGBA) {
					pass := t * r
					ph := math.Mod(pass, 1)
					col := cols[int(pass)%len(cols)]
					f.paint(c, func(u float64) color.RGBA {
						if ph < 0.4 { // grows from the end of the path
							if 1-u <= ph/0.4 {
								return col
							}
							return color.RGBA{A: 255}
						}
						return scaleColor(col, 1-smooth((ph-0.4)/0.45))
					})
				}
			}),
		mk("shuttle", "Shuttle", 2, []string{"#200000", "#ff3000"}, []Param{{Key: "size", Label: "Block size", Type: ParamNumber, Min: 1, Max: 50, Default: 12.0}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, size := rate(p, 0.1, 1.5), p.Float("size")/100
				bg, fg := cols[0], cols[len(cols)-1]
				return func(t float64, c *image.RGBA) {
					x := math.Mod(t*r, 2)
					if x > 1 {
						x = 2 - x // back and forth
					}
					pos := x * (1 - size)
					f.paint(c, func(u float64) color.RGBA {
						if u >= pos && u <= pos+size {
							return fg
						}
						return bg
					})
				}
			}),
		mk("bounce", "Bounce", 6, six, []Param{{Key: "balls", Label: "Balls", Type: ParamNumber, Min: 1, Max: 6, Default: 1.0}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, n := rate(p, 0.1, 1.5), int(math.Max(1, p.Float("balls")))
				const size = 0.08
				return func(t float64, c *image.RGBA) {
					trip := t * r
					x := math.Mod(trip, 2)
					if x > 1 {
						x = 2 - x
					}
					col := cols[int(trip)%len(cols)] // new color at every wall
					f.paint(c, func(u float64) color.RGBA {
						seg := u * float64(n) // each ball has its own lane
						lu := seg - math.Floor(seg)
						pos := x * (1 - size)
						if lu >= pos && lu <= pos+size {
							return col
						}
						return color.RGBA{A: 255}
					})
				}
			}),
		mk("stack", "Stack", 6, six, nil,
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r := rate(p, 0.5, 6) // bullets per second
				const slots = 16
				return func(t float64, c *image.RGBA) {
					// the stack grows by one slot per bullet; a full stack starts over
					k := t * r
					round := int(k / slots)
					n := int(k) % slots
					fly := k - math.Floor(k)
					top := 1 - float64(n)/slots // free space ends here
					bullet := cols[(round*slots+n)%len(cols)]
					stackCol := cols[(round*slots+n+len(cols)-1)%len(cols)]
					f.paint(c, func(u float64) color.RGBA {
						if u >= top {
							if n == 0 {
								return color.RGBA{A: 255}
							}
							return stackCol
						}
						pos := fly * (top - 1.0/slots)
						if u >= pos && u <= pos+1.0/slots {
							return bullet
						}
						return color.RGBA{A: 255}
					})
				}
			}),
		mk("sparkle", "Sparkle", 6, six, []Param{{Key: "density", Label: "Density", Type: ParamNumber, Min: 1, Max: 100, Default: 30.0}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, dens := rate(p, 0.3, 4), p.Float("density")/100
				const cells = 97 // sparkle spots along the path
				return func(t float64, c *image.RGBA) {
					f.paint(c, func(u float64) color.RGBA {
						cell := int(u * cells)
						// each cell flashes once per period at its own random phase
						ph := t*r + hash01(uint64(cell)*7919)
						period := uint64(ph)
						if hash01(uint64(cell)<<20^period) > dens {
							return color.RGBA{A: 255}
						}
						x := ph - math.Floor(ph)
						env := math.Max(0, 1-math.Abs(x-0.5)*4) // short peak in the middle of the period
						col := cols[int(hash01(uint64(cell)<<32^period)*float64(len(cols)))%len(cols)]
						return scaleColor(col, env)
					})
				}
			}),
		mk("stripes", "Stripes", 2, []string{"#0000ff", "#00d7ff"}, []Param{{Key: "count", Label: "Stripes", Type: ParamNumber, Min: 1, Max: 30, Default: 10.0}},
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r, n := rate(p, 0.05, 1), math.Max(1, p.Float("count"))
				return func(t float64, c *image.RGBA) {
					f.paint(c, func(u float64) color.RGBA {
						x := math.Mod(u*n-t*r*n+1e6, 1)
						if x < 0.33 {
							return cols[len(cols)-1]
						}
						return cols[0]
					})
				}
			}),
		mk("sand", "Sand glass", 4, []string{"#ff0096", "#0000ff", "#fff026", "#00ff00"}, nil,
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r := rate(p, 0.05, 0.6)
				return func(t float64, c *image.RGBA) {
					pass := t * r
					ph := math.Mod(pass, 1)
					col := cols[int(pass)%len(cols)]
					fill := smooth(math.Min(1, ph/0.75)) // sand level at the end of the path
					fade := 1 - smooth((ph-0.75)/0.25)
					grain := math.Mod(ph*12, 1) // a falling grain above the level
					f.paint(c, func(u float64) color.RGBA {
						level := 1 - fill
						switch {
						case u >= level:
							return scaleColor(col, fade)
						case ph < 0.75 && math.Abs(u-grain*level) < 0.03:
							return col
						}
						return color.RGBA{A: 255}
					})
				}
			}),
		mk("spark", "Spark", 4, []string{"#ff0000", "#00ff00", "#0000ff", "#ff00ff"}, nil,
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r := rate(p, 0.15, 1.5)
				white := color.RGBA{255, 255, 255, 255}
				const pack = 0.18
				return func(t float64, c *image.RGBA) {
					pass := t * r
					ph := math.Mod(pass, 1)
					col := cols[int(pass)%len(cols)]
					// in: colored head with a white tail runs to the end; out: it comes back white first
					var head float64
					in := ph < 0.6
					if in {
						head = ph / 0.6 * (1 + pack)
					} else {
						head = (1 - (ph-0.6)/0.4) * (1 + pack)
					}
					f.paint(c, func(u float64) color.RGBA {
						d := head - u
						if d < 0 || d > pack {
							return color.RGBA{A: 255}
						}
						if (d < pack/2) == in {
							return col
						}
						return white
					})
				}
			}),
		mk("meet", "Meet and mix", 2, []string{"#ff0000", "#00ff00"}, nil,
			func(p Params, f *field, cols []color.RGBA) func(float64, *image.RGBA) {
				r := rate(p, 0.1, 1.2)
				a, b := cols[0], cols[len(cols)-1]
				mix := color.RGBA{sat(int(a.R) + int(b.R)), sat(int(a.G) + int(b.G)), sat(int(a.B) + int(b.B)), 255}
				const comet = 0.12
				return func(t float64, c *image.RGBA) {
					ph := math.Mod(t*r, 1)
					f.paint(c, func(u float64) color.RGBA {
						if ph < 0.5 { // two comets run towards each other
							x := ph / 0.5 * 0.5
							if u <= x && u >= x-comet {
								return scaleColor(a, 1-(x-u)/comet)
							}
							if 1-u <= x && 1-u >= x-comet {
								return scaleColor(b, 1-(x-(1-u))/comet)
							}
							return color.RGBA{A: 255}
						}
						// the mixed color spreads from the meeting point
						spread := (ph - 0.5) / 0.5 * 0.5
						if math.Abs(u-0.5) <= spread {
							return mix
						}
						return color.RGBA{A: 255}
					})
				}
			}),
		{
			Name: "rainbow-gaps", Label: "Rainbow with gaps",
			Params: append(pathParams(), Param{Key: "gaps", Label: "Gaps", Type: ParamNumber, Min: 1, Max: 20, Default: 5.0}),
			New: func(p Params) (Effect, error) {
				f, r, n := newField(p), rate(p, 0.05, 1), math.Max(1, p.Float("gaps"))
				return funcEffect(func(t float64, c *image.RGBA) {
					f.paint(c, func(u float64) color.RGBA {
						if math.Mod(u*n+t*r*2*n, 1) < 0.4 {
							return color.RGBA{A: 255}
						}
						return HSV(360*(u*2+t*r), 1, 1)
					})
				}), nil
			},
		},
	}
}

func sat(v int) uint8 {
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// funcEffect adapts a time based render function to Effect.
type funcEffect func(t float64, c *image.RGBA)

func (fe funcEffect) Render(f Frame, c *image.RGBA) { fe(f.T.Seconds(), c) }
