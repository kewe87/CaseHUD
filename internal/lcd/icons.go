package lcd

import (
	"math"
)

// IconNames lists the built-in animated icons in GUI order.
var IconNames = []string{"pump", "fan", "flow", "temp", "chip", "power", "fps", "clock", "memory"}

// iconRate is the animation speed (revolutions / cycles per second) at a
// reading of lvl (0..1). Rotors are capped so that a 25 fps animation never
// turns more than half a blade pitch per frame (no wagon-wheel effect).
func iconRate(name string, lvl float64) float64 {
	switch name {
	case "fan", "pump":
		if lvl < 0.01 {
			return 0
		}
		return 0.15 + 1.35*lvl
	case "flow":
		if lvl < 0.01 {
			return 0
		}
		return 0.06 + 0.5*lvl
	case "chip":
		return 0.3 + 1.7*lvl
	case "memory":
		return 0.2 + 1.0*lvl
	case "fps":
		return 0.25 + 1.0*lvl
	case "temp":
		return 0.15 + 0.5*lvl
	}
	return 0.5
}

var iconDrawers = map[string]func(*iconCtx){
	"pump": drawPump, "fan": drawFan, "flow": drawFlow, "temp": drawTemp,
	"chip": drawChip, "power": drawPower, "fps": drawFPS, "clock": drawClock, "memory": drawMemory,
}

// bladeSDF is the signed distance to the blades of a rotor with `count`
// blades, turned by spin radians. The plane is folded into one sector, so the
// cost does not depend on the blade count. skew0/skew1 sweep the blade
// sideways from root to tip (fractions of the pitch); hw0/hw1 are its half
// widths and rad the corner rounding.
func bladeSDF(cx, cy, r0, r1, spin float64, count int, skew0, skew1, hw0, hw1, rad float64) func(x, y float64) float64 {
	seg := 2 * math.Pi / float64(count)
	return func(x, y float64) float64 {
		dx, dy := x-cx, y-cy
		r := hyp(dx, dy)
		if r > r1+hw1+4 {
			return 1e3
		}
		a := fastAtan2(dy, dx) - spin
		a -= seg * math.Floor(a/seg)
		t := cl01((r - r0) / (r1 - r0))
		th := seg * (0.5 + skew0 + (skew1-skew0)*t)
		arc := (a - th) * r
		hw := hw0 + (hw1-hw0)*t
		rd := min(rad, hw)
		q1 := math.Abs(r-(r0+r1)/2) - (r1-r0)/2 + rd
		q2 := math.Abs(arc) - hw + rd
		return hyp(max(q1, 0), max(q2, 0)) + min(max(q1, q2), 0) - rd
	}
}

func box(cx, cy, e float64) [4]float64 { return [4]float64{cx - e, cy - e, cx + e, cy + e} }

func drawFan(c *iconCtx) {
	n := c.nf
	cx, cy := n/2, n/2
	R, r0 := 0.43*n, 0.1*n
	count := 5
	hw1 := 0.15 * n
	if c.st.pixel > 0 {
		count, hw1 = 4, 0.12*n
	}
	spin := c.phase * 2 * math.Pi
	mk := func(s float64) func(x, y float64) float64 {
		return bladeSDF(cx, cy, r0, R, s, count, -0.2, 0.12, 0.05*n, hw1, (0.05+0.12*c.st.round)*n)
	}
	c.ring(cx, cy, 0.475*n, c.sw*0.7, c.col, 0.4)
	if c.anim && !c.ghost && c.lvl > 0.25 {
		// motion blur: a fainter copy trailing the blades
		seg := 2 * math.Pi / float64(count)
		c.fill(box(cx, cy, R+4), c.col, 0.22*c.lvl, mk(spin-0.28*seg*c.lvl))
	}
	c.fill(box(cx, cy, R+4), c.col, 1, mk(spin))
	c.disc(cx, cy, 0.105*n, c.hi, 1)
	c.disc(cx, cy, 0.04*n, c.col, 1)
}

func drawPump(c *iconCtx) {
	n := c.nf
	cx, cy := n/2, n/2
	R, r0 := 0.34*n, 0.08*n
	spin := c.phase * 2 * math.Pi
	count, hw := 6, 0.052*n
	if c.st.pixel > 0 {
		count, hw = 5, 0.06*n
	}
	vane := bladeSDF(cx, cy, r0, R, spin, count, -0.05, 0.55, hw*0.8, hw, hw)
	c.ring(cx, cy, 0.455*n, c.sw*1.1, c.col, 0.95)
	c.ring(cx, cy, 0.385*n, c.sw*0.5, c.col, 0.3)
	c.fill(box(cx, cy, R+hw+4), c.col, 1, vane)
	c.disc(cx, cy, 0.1*n, c.col, 1)
	c.disc(cx, cy, 0.04*n, c.hi, 1)
	if !c.ghost {
		// liquid running round the casing, slower than the impeller
		for i := 0; i < 4; i++ {
			a := (c.phase*0.45 + float64(i)/4) * 2 * math.Pi
			c.disc(cx+0.42*n*math.Cos(a), cy+0.42*n*math.Sin(a), 0.022*n, c.hi, 0.9)
		}
	}
}

// track is the racetrack loop of the flow icon.
type track struct{ cx, cy, hx, hy, rc float64 }

func (t track) length() float64 {
	return 2*(2*t.hx-2*t.rc) + 2*(2*t.hy-2*t.rc) + 2*math.Pi*t.rc
}

// at returns the point s px along the track, clockwise from the top middle.
func (t track) at(s float64) (float64, float64) {
	s = math.Mod(s, t.length())
	if s < 0 {
		s += t.length()
	}
	top := t.hx - t.rc
	side := t.hy - t.rc // half length of the straight sides
	arc := math.Pi * t.rc / 2
	segs := [9]float64{top, arc, 2 * side, arc, 2 * top, arc, 2 * side, arc, top}
	for i, l := range segs {
		if s > l && i < 8 {
			s -= l
			continue
		}
		switch i {
		case 0:
			return t.cx + s, t.cy - t.hy
		case 1:
			a := -math.Pi/2 + s/t.rc
			return t.cx + top + t.rc*math.Cos(a), t.cy - side + t.rc*math.Sin(a)
		case 2:
			return t.cx + t.hx, t.cy - side + s
		case 3:
			a := s / t.rc
			return t.cx + top + t.rc*math.Cos(a), t.cy + side + t.rc*math.Sin(a)
		case 4:
			return t.cx + top - s, t.cy + t.hy
		case 5:
			a := math.Pi/2 + s/t.rc
			return t.cx - top + t.rc*math.Cos(a), t.cy + side + t.rc*math.Sin(a)
		case 6:
			return t.cx - t.hx, t.cy + side - s
		case 7:
			a := math.Pi + s/t.rc
			return t.cx - top + t.rc*math.Cos(a), t.cy - side + t.rc*math.Sin(a)
		}
		return t.cx - top + s, t.cy - t.hy
	}
	return t.cx, t.cy - t.hy
}

func drawFlow(c *iconCtx) {
	n := c.nf
	tr := track{n / 2, n / 2, 0.43 * n, 0.33 * n, (0.16 + 0.1*c.st.round) * n}
	tr.rc = min(tr.rc, tr.hy)
	c.outline(box(tr.cx, tr.cy, 0.47*n), c.sw*0.9, c.col, 0.3,
		func(x, y float64) float64 { return roundBox(x, y, tr.cx, tr.cy, tr.hx, tr.hy, tr.rc) })
	// droplet in the middle
	cr, cyc := 0.1*n, n*0.54
	tip := pt{n / 2, n * 0.30}
	th := math.Acos(cr / (cyc - tip.y))
	tri := []pt{tip, {n/2 + cr*math.Sin(th), cyc - cr*math.Cos(th)}, {n/2 - cr*math.Sin(th), cyc - cr*math.Cos(th)}}
	if c.st.round < 0.3 {
		tri = []pt{tip, {n/2 + cr, cyc}, {n / 2, cyc + cr}, {n/2 - cr, cyc}}
	}
	c.fill([4]float64{n/2 - cr - 2, tip.y - 2, n/2 + cr + 2, cyc + cr + 2}, c.col, 0.9, func(x, y float64) float64 {
		return min(hyp(x-n/2, y-cyc)-cr, polyDist(x, y, tri))
	})
	if c.ghost {
		return
	}
	// comets of three dots running clockwise
	const dots = 12
	L := tr.length()
	for i := 0; i < dots; i++ {
		k := float64(2 - i%3)
		x, y := tr.at(c.phase*L + float64(i)*L/dots)
		c.disc(x, y, n*(0.028+0.008*k), c.hi, 0.35+0.325*k)
	}
}

func drawTemp(c *iconCtx) {
	n := c.nf
	cx := n * 0.42
	tw := 0.095 * n
	yTop, yBulb, br := 0.09*n+tw, 0.74*n, 0.17*n
	shape := func(x, y float64) float64 {
		return min(segDist(x, y, cx, yTop, cx, yBulb)-tw, hyp(x-cx, y-yBulb)-br)
	}
	c.outline([4]float64{cx - br, yTop - tw, cx + br, yBulb + br}, c.sw*1.1, c.col, 1, shape)
	// liquid
	lv := 0.1 + 0.9*c.lvl
	liqTop := yBulb - (yBulb-yTop+tw*0.2)*lv
	iw, ibr := tw-c.sw*1.1, br-c.sw*1.1
	c.fill([4]float64{cx - ibr, yTop - tw, cx + ibr, yBulb + ibr}, c.col, 1, func(x, y float64) float64 {
		d := min(segDist(x, y, cx, yTop, cx, yBulb)-iw, hyp(x-cx, y-yBulb)-ibr)
		return max(d, liqTop-y)
	})
	// scale marks
	for i := 0; i < 5 && c.px >= 80; i++ { // scale marks only where they can be told apart
		y := yTop + float64(i)*(yBulb-yTop-br*0.3)/4
		l := 0.05 * n
		if i%2 == 1 {
			l *= 0.6
		}
		c.line(cx+tw+0.035*n, y, cx+tw+0.035*n+l, y, c.sw*0.7, c.col, 0.5)
	}
	if c.ghost {
		return
	}
	// rising bubbles in the liquid
	if c.anim && liqTop < yBulb-br && c.px >= 90 {
		for i := 0; i < 2; i++ {
			ph := math.Mod(c.phase*1.4+float64(i)*0.5, 1)
			y := yBulb - ph*(yBulb-liqTop-iw)
			c.disc(cx+(float64(i)-0.5)*iw*0.7, y, iw*0.28, c.hi, 0.75*(1-ph*ph))
		}
	}
	// heat shimmer to the right when hot
	hot := smoothstep(0.55, 0.95, c.lvl)
	if c.px < 110 {
		hot = 0
	}
	if hot > 0 {
		x0 := cx + tw + 0.19*n
		for i := 0; i < 2; i++ {
			xs := x0 + float64(i)*0.1*n
			amp := 0.022 * n
			k := 2 * math.Pi / (0.2 * n)
			ph := c.phase*2*math.Pi*2 + float64(i)*2
			c.paint(xs-amp-2, 0.12*n, xs+amp+2, 0.62*n, c.col, hot, func(x, y float64) float64 {
				return math.Abs(x-(xs+amp*math.Sin(k*y-ph))) - c.sw*0.6
			}, func(x, y float64) float64 {
				return smoothstep(0.12*n, 0.28*n, y) * (1 - smoothstep(0.45*n, 0.62*n, y))
			})
		}
	}
}

func drawChip(c *iconCtx) {
	n := c.nf
	cx, cy := n/2, n/2
	half := 0.25 * n
	rr := (0.02 + 0.06*c.st.round) * n
	body := func(x, y float64) float64 { return roundBox(x, y, cx, cy, half, half, rr) }
	for i := -1; i <= 1; i++ {
		o := float64(i) * 0.13 * n
		l := 0.1 * n
		c.line(cx+o, cy-half, cx+o, cy-half-l, c.sw*1.1, c.col, 0.85)
		c.line(cx+o, cy+half, cx+o, cy+half+l, c.sw*1.1, c.col, 0.85)
		c.line(cx-half, cy+o, cx-half-l, cy+o, c.sw*1.1, c.col, 0.85)
		c.line(cx+half, cy+o, cx+half+l, cy+o, c.sw*1.1, c.col, 0.85)
	}
	pulse := 1.0
	if c.anim && !c.ghost {
		pulse = 0.5 + 0.5*math.Sin(2*math.Pi*c.phase)
		// sonar ring: starts at the package edge, fades while it grows
		k := math.Mod(c.phase, 1)
		g := 0.045*n + 0.1*n*k
		c.outline(box(cx, cy, half+g+4), c.sw*0.8, c.col, 0.7*c.lvl*(1-k), func(x, y float64) float64 {
			return roundBox(x, y, cx, cy, half+g, half+g, rr+g)
		})
	}
	c.outline(box(cx, cy, half+2), c.sw*1.2, c.col, 1, body)
	dh := 0.125 * n
	die := func(x, y float64) float64 { return roundBox(x, y, cx, cy, dh, dh, rr*0.4) }
	c.outline(box(cx, cy, dh+2), c.sw*0.7, c.col, 0.7, die)
	c.fill(box(cx, cy, dh+2), c.col, 0.12+0.88*c.lvl*(0.45+0.55*pulse), die)
}

var boltPts = []pt{{0.62, 0.04}, {0.24, 0.56}, {0.46, 0.56}, {0.36, 0.97}, {0.78, 0.40}, {0.55, 0.40}, {0.74, 0.04}}

func drawPower(c *iconCtx) {
	n := c.nf
	p := make([]pt, len(boltPts))
	for i, q := range boltPts {
		p[i] = pt{n * (0.5 + (q.x-0.5)*0.94), n * (0.5 + (q.y-0.5)*0.94)}
	}
	bb := polyBox(p)
	fl := 1.0
	slot := int64(c.t * 14)
	if c.anim && !c.ghost {
		// flicker: harder and more often the more power is drawn
		if hash01(slot) < 0.15+0.55*c.lvl {
			fl = 1 - (0.15+0.65*c.lvl)*hash01(slot+7777)
		}
	}
	level := 0.3 + 0.7*c.lvl
	shape := func(x, y float64) float64 { return polyDist(x, y, p) }
	c.fill(bb, c.col, level*fl, shape)
	c.outline(bb, c.sw*1.1, c.col, min(1, 0.55+level*0.45)*fl, shape)
}

// ecg is the waveform of one heartbeat at u in [0,1): amplitude about -0.3..1.
func ecg(u float64) float64 {
	g := func(m, w, a float64) float64 { d := (u - m) / w; return a * math.Exp(-d*d) }
	return g(0.14, 0.05, 0.16) + g(0.35, 0.022, -0.2) + g(0.40, 0.022, 1) + g(0.46, 0.022, -0.36) + g(0.68, 0.07, 0.26)
}

func drawFPS(c *iconCtx) {
	n := c.nf
	x0, x1 := 0.04*n, 0.96*n
	yb, amp := 0.58*n, 0.5*n
	w := x1 - x0
	cols := int(w) + 3
	ys := make([]float64, cols)
	for i := range ys {
		ys[i] = yb - amp*ecg(math.Mod(float64(i)/w*2, 1))
	}
	at := func(x float64) float64 {
		f := x - x0
		i := max(0, min(cols-2, int(f)))
		t := f - float64(i)
		return ys[i]*(1-t) + ys[i+1]*t
	}
	head := math.Mod(c.phase, 1)
	if !c.anim {
		head = 0.78
	}
	hx := x0 + head*w
	c.line(x0, yb, x1, yb, c.sw*0.8, c.col, 0.3)
	half := c.sw * 1.9
	var af func(x, y float64) float64
	if !c.ghost {
		af = func(x, y float64) float64 {
			d := (hx - x) / w // distance behind the head
			if d < 0 {
				d++
			}
			return 0.85 + 0.15*math.Pow(1-d, 1.6)
		}
	}
	c.paint(x0, 0, x1, n, c.col, 1, func(x, y float64) float64 {
		if x < x0 || x > x1 {
			return 1e3
		}
		a, b := at(x-0.5), at(x+0.5)
		lo, hi := min(a, b), max(a, b)
		d := 0.0
		if y < lo {
			d = lo - y
		} else if y > hi {
			d = y - hi
		}
		return d - half
	}, af)
	if !c.ghost {
		c.disc(hx, at(hx), c.sw*1.5, c.hi, 1)
	}
}

func drawClock(c *iconCtx) {
	n := c.nf
	cx, cy := n/2, n/2
	R := 0.44 * n
	c.ring(cx, cy, R, c.sw*1.4, c.col, 0.95)
	for i := 0; i < 12; i++ {
		a := float64(i) * math.Pi / 6
		r0, r1 := R-0.1*n, R-0.045*n
		w := c.sw * 0.7
		if i%3 == 0 {
			r0, w = R-0.14*n, c.sw*1.1
		}
		c.line(cx+r0*math.Sin(a), cy-r0*math.Cos(a), cx+r1*math.Sin(a), cy-r1*math.Cos(a), w, c.col, 0.7)
	}
	h, m := float64(c.tm.Hour()%12), float64(c.tm.Minute())
	s := float64(c.tm.Second()) + float64(c.tm.Nanosecond())/1e9
	if c.ghost {
		h, m = 10, 8
	}
	if !c.anim {
		s = 0
	}
	ma := (m + s/60) / 60 * 2 * math.Pi
	ha := (h + m/60) / 12 * 2 * math.Pi
	c.line(cx, cy, cx+0.22*n*math.Sin(ha), cy-0.22*n*math.Cos(ha), c.sw*1.8, c.col, 1)
	c.line(cx, cy, cx+0.33*n*math.Sin(ma), cy-0.33*n*math.Cos(ma), c.sw*1.2, c.col, 1)
	if c.anim && !c.ghost {
		sa := s / 60 * 2 * math.Pi
		c.line(cx-0.07*n*math.Sin(sa), cy+0.07*n*math.Cos(sa), cx+0.37*n*math.Sin(sa), cy-0.37*n*math.Cos(sa), c.sw*0.7, c.hi, 1)
	}
	c.disc(cx, cy, 0.035*n, c.hi, 1)
}

// drawMemory is a RAM module: a long board with four chips, contacts along
// the lower edge and a notch in the middle. The chips light up in a wave that
// runs faster and brighter the fuller the memory is.
func drawMemory(c *iconCtx) {
	n := c.nf
	cx, cy := n/2, 0.44*n
	hx, hy := 0.43*n, 0.2*n
	rr := (0.015 + 0.05*c.st.round) * n
	body := func(x, y float64) float64 { return roundBox(x, y, cx, cy, hx, hy, rr) }
	rect := func(x0, y0, x1, y1 float64) [4]float64 { return [4]float64{x0 - 2, y0 - 2, x1 + 2, y1 + 2} }
	for i := 0; i < 9; i++ {
		if i == 4 {
			continue // the notch
		}
		x := cx - hx + 0.06*n + float64(i)*(2*hx-0.12*n)/8
		c.line(x, cy+hy, x, cy+hy+0.1*n, c.sw*1.1, c.col, 0.85)
	}
	c.outline(rect(cx-hx, cy-hy, cx+hx, cy+hy), c.sw*1.2, c.col, 1, body)
	cw, ch := 0.075*n, 0.095*n
	for i := 0; i < 4; i++ {
		x := cx + (float64(i)-1.5)*0.2*n
		chip := func(px, py float64) float64 { return roundBox(px, py, x, cy, cw, ch, rr*0.4) }
		wave := 1.0
		if c.anim && !c.ghost {
			wave = 0.5 + 0.5*math.Sin(2*math.Pi*(c.phase-float64(i)*0.2))
		}
		bb := rect(x-cw, cy-ch, x+cw, cy+ch)
		c.outline(bb, c.sw*0.7, c.col, 0.75, chip)
		c.fill(bb, c.col, 0.12+0.88*c.lvl*(0.45+0.55*wave), chip)
	}
}
