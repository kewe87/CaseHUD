package lcd

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// Animated icons ("icon" widgets): small vector drawings whose motion is
// driven by a sensor, e.g. a pump impeller turning with the pump rpm.
//
// JSON: {"type":"icon","icon":"pump","x":..,"y":..,"w":..,"h":..,"source":"Fans/Pump",
// "min":0,"max":3000,"color":"#22d3ee","warn":..,"crit":..,"invert":false,
// "style":"","color2":"","speed":1}. The icon is drawn in the largest centred
// square of the rectangle. Icons: pump, fan, flow (rotation / flow speed ~
// reading), temp (liquid level, shimmer when hot), chip (pulse rate and die
// brightness ~ load), power (brightness and flicker ~ watts), fps (heartbeat
// sweep speed), clock (real time, source ignored). Without a source the icon
// runs at medium speed. Style defaults to the layout's set (clean, neon,
// retro, cyberpunk); neon halos stay inside the square.
//
// Time comes from Renderer.Now, so frames are reproducible in tests. The
// speed is integrated (phase += rate*dt), so a changing reading speeds the
// icon up or down smoothly without jumps. With animations off (no
// SetAnimation) icons are drawn in a fixed pose that still shows the level.
//
// Cheap per-frame use: set SplitIcons, draw the HUD only when values change
// (Frame) and put the icons on a copy of it for every display frame with
// ComposeIcons / DrawIcons. Only the icon rectangles are touched.

// IconFrameInterval is the suggested frame time while icons animate (25 fps).
const IconFrameInterval = 40 * time.Millisecond

// iconState is the per-widget animation state.
type iconState struct {
	started  bool
	last     time.Time
	readAt   time.Time
	val      float64
	ok       bool
	lvl      float64 // smoothed reading 0..1
	phase    float64 // cycles, kept in [0,1)
	buf      []uint8
	t0       time.Time
	staticAt float64
}

// HasIcons reports whether the layout has icon widgets.
func (r *Renderer) HasIcons() bool {
	for _, w := range r.draftL.Widgets {
		if w.Type == "icon" {
			return true
		}
	}
	return false
}

// AnimatedIcons reports whether icons need frames of their own: the layout
// has icons and animation is on.
func (r *Renderer) AnimatedIcons() bool { return r.animTau > 0 && r.HasIcons() }

// IconRects are the screen rectangles of the icons (output pixels), e.g. to
// update only these on a display.
func (r *Renderer) IconRects() []image.Rectangle {
	var out []image.Rectangle
	for i, w := range r.draftL.Widgets {
		if w.Type != "icon" {
			continue
		}
		if w.Inline != "" {
			if sq, ok := r.inlOut[i]; ok {
				out = append(out, sq)
			}
			continue
		}
		out = append(out, iconSquare(image.Rect(w.X, w.Y, w.X+w.W, w.Y+w.H)))
	}
	return out
}

// ComposeIcons returns a copy of base (the HUD without icons, or any frame)
// with the icons drawn on top. base is not modified.
func (r *Renderer) ComposeIcons(base *image.RGBA, p metrics.Provider) *image.RGBA {
	img := image.NewRGBA(base.Bounds())
	copy(img.Pix, base.Pix)
	r.DrawIcons(img, p)
	return img
}

// DrawIcons draws every icon onto dst (output size), in place.
func (r *Renderer) DrawIcons(dst *image.RGBA, p metrics.Provider) {
	r.ss = 1 // dst is output size
	p = clockSource{p, r.Now}
	now := r.Now()
	for i, w := range r.draftL.Widgets {
		if w.Type != "icon" {
			continue
		}
		rect := image.Rect(w.X, w.Y, w.X+w.W, w.Y+w.H)
		if w.Inline != "" {
			sq, ok := r.inlOut[i]
			if !ok {
				continue // the HUD has not been drawn yet
			}
			rect = sq
		}
		r.drawIcon(dst, i, w, rect, p, now, r.draftL.Scanlines)
	}
}

// iconSquare is the centred square an icon occupies in its widget rectangle.
func iconSquare(rect image.Rectangle) image.Rectangle {
	s := min(rect.Dx(), rect.Dy())
	c := image.Pt((rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2)
	return image.Rect(c.X-s/2, c.Y-s/2, c.X-s/2+s, c.Y-s/2+s)
}

func (r *Renderer) iconState(i int) *iconState {
	if r.icons == nil {
		r.icons = map[int]*iconState{}
	}
	st := r.icons[i]
	if st == nil {
		st = &iconState{}
		r.icons[i] = st
	}
	return st
}

// staticPhase is the pose of icons without animation.
const staticPhase = 0.07

// drawIcon advances the state of icon widget i to now and paints it.
func (r *Renderer) drawIcon(dst *image.RGBA, i int, w Widget, rect image.Rectangle, p metrics.Provider, now time.Time, scan bool) {
	st := r.iconState(i)
	// Read the sensor at most 5 times a second; icons run at 25 fps.
	if !st.started || now.Sub(st.readAt) >= 200*time.Millisecond || now.Before(st.readAt) {
		st.readAt = now
		if w.Source == "" {
			st.val, st.ok = 0.6, true
		} else {
			v, _, ok := r.rawReading(p, w.Source, "")
			st.val, st.ok = v, ok
		}
	}
	col := parseHex(w.Color, r.acc)
	target := 0.0
	switch {
	case w.Source == "":
		target = st.val
	case st.ok:
		target = r.fraction(w, st.val)
		col = r.colorFor(w, st.val, true)
	default:
		col = r.mut
	}
	animate := r.animTau > 0
	dt := 0.0
	if st.started {
		dt = min(0.25, max(0, now.Sub(st.last).Seconds()))
	}
	if !st.started {
		st.t0 = now
	}
	st.started, st.last = true, now
	if animate {
		if dt == 0 && st.lvl == 0 {
			st.lvl = target
		}
		st.lvl += (target - st.lvl) * (1 - math.Exp(-dt/0.35))
		sp := w.Speed
		if sp <= 0 {
			sp = 1
		}
		st.phase = math.Mod(st.phase+iconRate(w.Icon, st.lvl)*sp*dt, 1)
	} else {
		st.lvl, st.phase = target, staticPhase
	}

	// painting
	sq := iconSquare(rect)
	size := sq.Dx()
	if size < 8 {
		return
	}
	styleName := w.Style
	if styleName == "" {
		styleName = r.L.Set
	}
	style := styleFor(styleName, isLight(r.bg))
	n := size
	if style.pixel > 0 {
		n = style.pixel
	} else if size <= 110 {
		n = size * 2
	}
	if len(st.buf) != n*n*4 {
		st.buf = make([]uint8, n*n*4)
	} else {
		clear(st.buf)
	}
	// Halos and the offset copy need room around the drawing.
	scale := 1.0
	switch {
	case style.glow:
		scale = 0.84
	case style.shadow:
		scale = 0.94
	}
	c := &iconCtx{n: n, nf: float64(n) * scale, ox: float64(n) * (1 - scale) / 2, oy: float64(n) * (1 - scale) / 2, pix: st.buf, st: style, col: col, hi: lighten(col, 0.45),
		px: int(float64(size) / max(1, r.ss)), lvl: st.lvl, phase: st.phase, anim: animate, tm: now, alphaMul: 1,
		t: now.Sub(st.t0).Seconds()}
	if !animate {
		c.t = 0
	}
	c.sw = max(1.2, float64(n)*0.05*style.stroke)
	if style.pixel > 0 {
		c.sw = max(1.4, float64(n)*0.055)
	}
	c.col2 = parseHex(w.Color2, color.RGBA{255, 0, 60, 255})
	draw := iconDrawers[w.Icon]
	if draw == nil {
		draw = drawFan
	}
	if style.ghost {
		c.ghost, c.alphaMul = true, 0.16
		c.lvl, c.phase = 1, staticPhase
		draw(c)
		c.ghost, c.alphaMul, c.lvl, c.phase = false, 1, st.lvl, st.phase
	}
	if style.shadow {
		main := c.col
		c.col, c.hi, c.dx, c.alphaMul = c.col2, lighten(c.col2, 0.2), 0.035*c.nf, 0.8
		draw(c)
		c.col, c.hi, c.dx, c.alphaMul = main, lighten(main, 0.45), 0, 1
	}
	draw(c)
	blitIcon(dst, sq, c, scan)
}

// blitIcon composites the premultiplied canvas onto dst, scaling n -> size
// (averaging when downscaling, replicating for pixel grids).
func blitIcon(dst *image.RGBA, sq image.Rectangle, c *iconCtx, scan bool) {
	size, n := sq.Dx(), c.n
	for y := 0; y < size; y++ {
		dy := sq.Min.Y + y
		if dy < dst.Rect.Min.Y || dy >= dst.Rect.Max.Y {
			continue
		}
		dim := scan && (dy/1)%2 == 1
		for x := 0; x < size; x++ {
			dx := sq.Min.X + x
			if dx < dst.Rect.Min.X || dx >= dst.Rect.Max.X {
				continue
			}
			var s [4]uint32
			if n == 2*size {
				for _, o := range [4]int{0, 4, n * 4, n*4 + 4} {
					i := (2*y*n+2*x)*4 + o
					s[0] += uint32(c.pix[i])
					s[1] += uint32(c.pix[i+1])
					s[2] += uint32(c.pix[i+2])
					s[3] += uint32(c.pix[i+3])
				}
				s[0], s[1], s[2], s[3] = s[0]/4, s[1]/4, s[2]/4, s[3]/4
			} else {
				sx, sy := x*n/size, y*n/size
				i := (sy*n + sx) * 4
				s[0], s[1], s[2], s[3] = uint32(c.pix[i]), uint32(c.pix[i+1]), uint32(c.pix[i+2]), uint32(c.pix[i+3])
			}
			if s[3] == 0 {
				continue
			}
			if dim {
				s[0], s[1], s[2] = s[0]*3/5, s[1]*3/5, s[2]*3/5
			}
			o := dst.PixOffset(dx, dy)
			k := 255 - s[3]
			dst.Pix[o] = uint8(min(255, s[0]+(uint32(dst.Pix[o])*k+127)/255))
			dst.Pix[o+1] = uint8(min(255, s[1]+(uint32(dst.Pix[o+1])*k+127)/255))
			dst.Pix[o+2] = uint8(min(255, s[2]+(uint32(dst.Pix[o+2])*k+127)/255))
			dst.Pix[o+3] = uint8(min(255, s[3]+(uint32(dst.Pix[o+3])*k+127)/255))
		}
	}
}
