package lcd

import (
	"image/color"
	"math"
	"time"
)

// Vector kit for animated icons: every shape is a signed distance function
// rasterised into a small premultiplied canvas, so icons stay crisp at any
// size and need no bitmaps. The styles of the design sets (clean, neon,
// retro, cyberpunk) differ in how shapes are painted: halos, hard pixels,
// ghost segments, angular corners and offset colour copies.

// iconStyle describes how one design set paints its icons.
type iconStyle struct {
	name   string
	glow   bool    // soft halo around shapes
	pixel  int     // >0: render on a pixel grid of this size and scale up
	round  float64 // 1 = round corners, small = angular
	stroke float64 // stroke width multiplier
	ghost  bool    // dim "unlit" copy under the icon (LCD segments)
	shadow bool    // offset copy in the second colour (chromatic glitch)
}

func styleFor(name string, lightBG bool) iconStyle {
	switch name {
	case "neon":
		return iconStyle{name: name, glow: !lightBG, round: 1, stroke: 0.85}
	case "retro":
		return iconStyle{name: name, pixel: 44, round: 0.5, stroke: 1, ghost: true}
	case "cyberpunk":
		return iconStyle{name: name, round: 0.18, stroke: 0.8, shadow: true}
	}
	return iconStyle{name: "clean", round: 1, stroke: 1}
}

// iconCtx is the canvas an icon draws into, plus the animation inputs.
type iconCtx struct {
	n     int
	px    int // size of the icon on screen
	nf    float64
	pix   []uint8 // premultiplied RGBA, n*n*4
	st    iconStyle
	col   color.RGBA // main colour
	col2  color.RGBA // second colour
	hi    color.RGBA // lighter tone of col, for highlights
	lvl   float64    // 0..1 smoothed reading
	phase float64    // revolutions / cycles, accumulated by speed
	t     float64    // seconds, for flicker
	anim  bool       // false = static pose
	tm    time.Time  // wall clock (clock icon)
	ghost bool       // painting the static dim layer
	sw    float64    // base stroke width in canvas px

	alphaMul float64
	dx, dy   float64 // offset of the current pass
	ox, oy   float64 // where the drawing starts inside the canvas (room for halos)
	curCol   *color.RGBA
}

func (c *iconCtx) glowR() float64 { return max(1.5, c.nf*0.04) }

// paint fills the shape sdf (negative inside) within the box [x0,x1]x[y0,y1].
// af optionally modulates alpha per pixel.
func (c *iconCtx) paint(x0, y0, x1, y1 float64, col color.RGBA, a float64, sdf, af func(x, y float64) float64) {
	gr := c.glowR()
	pad := 2.0
	if c.st.glow {
		pad += gr * 3
	}
	sx, sy := c.dx+c.ox, c.dy+c.oy
	ix0, iy0 := max(0, int(math.Floor(x0+sx-pad))), max(0, int(math.Floor(y0+sy-pad)))
	ix1, iy1 := min(c.n, int(math.Ceil(x1+sx+pad))), min(c.n, int(math.Ceil(y1+sy+pad)))
	a *= c.alphaMul
	if a <= 0 {
		return
	}
	for y := iy0; y < iy1; y++ {
		py := float64(y) + 0.5
		for x := ix0; x < ix1; x++ {
			px := float64(x) + 0.5
			d := sdf(px-sx, py-sy)
			var cov float64
			if c.st.pixel > 0 {
				if d <= 0 {
					cov = 1
				}
			} else {
				cov = max(0, min(1, 0.5-d))
				if c.st.glow && d > 0 && d < gr*3.2 {
					cov += (1 - cov) * 0.5 * glowLUT[int(d/gr*20)]
				}
			}
			if cov <= 0 {
				continue
			}
			k := a * cov
			if af != nil {
				k *= af(px-c.ox, py-c.oy)
			}
			if k <= 0 {
				continue
			}
			i := (y*c.n + x) * 4
			ik := 1 - k
			c.pix[i] = uint8(float64(col.R)*k + float64(c.pix[i])*ik)
			c.pix[i+1] = uint8(float64(col.G)*k + float64(c.pix[i+1])*ik)
			c.pix[i+2] = uint8(float64(col.B)*k + float64(c.pix[i+2])*ik)
			c.pix[i+3] = uint8(255*k + float64(c.pix[i+3])*ik)
		}
	}
}

// fill paints the shape with its bounding box given.
func (c *iconCtx) fill(bb [4]float64, col color.RGBA, a float64, sdf func(x, y float64) float64) {
	c.paint(bb[0], bb[1], bb[2], bb[3], col, a, sdf, nil)
}

// outline paints only the contour of sdf, w px wide.
func (c *iconCtx) outline(bb [4]float64, w float64, col color.RGBA, a float64, sdf func(x, y float64) float64) {
	h := w / 2
	c.paint(bb[0], bb[1], bb[2], bb[3], col, a, func(x, y float64) float64 { return math.Abs(sdf(x, y)) - h }, nil)
}

func (c *iconCtx) disc(cx, cy, r float64, col color.RGBA, a float64) {
	c.fill([4]float64{cx - r, cy - r, cx + r, cy + r}, col, a, func(x, y float64) float64 { return hyp(x-cx, y-cy) - r })
}

func (c *iconCtx) ring(cx, cy, r, w float64, col color.RGBA, a float64) {
	e := r + w
	c.outline([4]float64{cx - e, cy - e, cx + e, cy + e}, w, col, a, func(x, y float64) float64 { return hyp(x-cx, y-cy) - r })
}

// line paints a capsule from (ax,ay) to (bx,by), w px wide.
func (c *iconCtx) line(ax, ay, bx, by, w float64, col color.RGBA, a float64) {
	h := w / 2
	c.fill([4]float64{min(ax, bx) - h, min(ay, by) - h, max(ax, bx) + h, max(ay, by) + h}, col, a,
		func(x, y float64) float64 { return segDist(x, y, ax, ay, bx, by) - h })
}

// roundBox is the signed distance to a rounded box centred on (cx,cy).
func roundBox(x, y, cx, cy, hx, hy, r float64) float64 {
	qx, qy := math.Abs(x-cx)-hx+r, math.Abs(y-cy)-hy+r
	return hyp(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - r
}

type pt struct{ x, y float64 }

// polyDist is the signed distance to a simple polygon (negative inside).
func polyDist(x, y float64, p []pt) float64 {
	d := (x-p[0].x)*(x-p[0].x) + (y-p[0].y)*(y-p[0].y)
	s := 1.0
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		ex, ey := p[j].x-p[i].x, p[j].y-p[i].y
		wx, wy := x-p[i].x, y-p[i].y
		t := max(0, min(1, (wx*ex+wy*ey)/(ex*ex+ey*ey)))
		bx, by := wx-ex*t, wy-ey*t
		d = min(d, bx*bx+by*by)
		c1, c2, c3 := y >= p[i].y, y < p[j].y, ex*wy > ey*wx
		if c1 && c2 && c3 || !c1 && !c2 && !c3 {
			s = -s
		}
	}
	return s * math.Sqrt(d)
}

func polyBox(p []pt) [4]float64 {
	b := [4]float64{p[0].x, p[0].y, p[0].x, p[0].y}
	for _, q := range p {
		b[0], b[1], b[2], b[3] = min(b[0], q.x), min(b[1], q.y), max(b[2], q.x), max(b[3], q.y)
	}
	return b
}

// hash01 is a deterministic pseudo-random value in [0,1) for an integer.
func hash01(n int64) float64 {
	x := uint64(n)*0x9E3779B97F4A7C15 + 0x7F4A7C15
	x ^= x >> 29
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 32
	return float64(x&0xFFFFFF) / float64(0x1000000)
}

// hyp is sqrt(x*x+y*y): math.Hypot guards against overflow and is several
// times slower, which matters in per-pixel loops.
func hyp(x, y float64) float64 { return math.Sqrt(x*x + y*y) }

// fastAtan2 is atan2 with an error below 1e-5 rad, about 5x faster.
func fastAtan2(y, x float64) float64 {
	ax, ay := math.Abs(x), math.Abs(y)
	if ax == 0 && ay == 0 {
		return 0
	}
	swap := ay > ax
	var t float64
	if swap {
		t = ax / ay
	} else {
		t = ay / ax
	}
	t2 := t * t
	a := t * (0.99997726 + t2*(-0.33262347+t2*(0.19354346+t2*(-0.11643287+t2*(0.05265332+t2*-0.01172120)))))
	if swap {
		a = math.Pi/2 - a
	}
	if x < 0 {
		a = math.Pi - a
	}
	if y < 0 {
		a = -a
	}
	return a
}

// glowLUT is exp(-d/gr) sampled for d/gr in [0,3.2).
var glowLUT = func() (t [65]float64) {
	for i := range t {
		t[i] = math.Exp(-float64(i) / 20)
	}
	return
}()

func lighten(c color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		uint8(float64(c.R) + (255-float64(c.R))*t),
		uint8(float64(c.G) + (255-float64(c.G))*t),
		uint8(float64(c.B) + (255-float64(c.B))*t), 255}
}

func cl01(v float64) float64 { return max(0, min(1, v)) }
