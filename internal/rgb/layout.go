package leds

import "math"

// Positions computes the canvas position of every LED of a zone, in wire order.
//
// Without configured segments the zone gets a default layout: a horizontal line
// across the middle of the canvas.
func Positions(z Zone, zc ZoneConfig) []Point {
	segs := zc.Segments
	if len(segs) == 0 {
		n := z.Count
		if n == 0 {
			return nil
		}
		segs = []Segment{{Count: n, Shape: "line", X: 0.05, Y: 0.5, W: 0.9, H: 0}}
	}
	var pts []Point
	var placed []bool
	next := 0
	for _, s := range segs {
		start := segmentStart(s, next)
		for i, p := range segmentPoints(s) {
			k := start + i
			for len(pts) <= k {
				pts, placed = append(pts, p), append(placed, false) // a gap takes this segment's first LED
			}
			if !placed[k] {
				pts[k], placed[k] = p, true
			}
		}
		next = start + segmentCount(s)
	}
	if limit := zoneLimit(z); len(pts) > limit {
		pts = pts[:limit]
	}
	return pts
}

// LEDCount is the number of LEDs a zone drives with the given configuration.
func LEDCount(z Zone, zc ZoneConfig) int {
	if len(zc.Segments) == 0 {
		return z.Count
	}
	n, next := 0, 0
	for _, s := range zc.Segments {
		next = segmentStart(s, next) + segmentCount(s)
		n = max(n, next)
	}
	return min(n, zoneLimit(z))
}

func segmentStart(s Segment, next int) int {
	if s.Start != nil && *s.Start >= 0 {
		return *s.Start
	}
	return next
}

func zoneLimit(z Zone) int {
	if z.Max > 0 {
		return z.Max
	}
	if z.Resizable {
		return math.MaxInt
	}
	return z.Count
}

// segmentCount is the number of LEDs a segment drives.
func segmentCount(s Segment) int {
	if s.Shape == "component" {
		c, ok := LookupComponent(s.Component)
		if !ok {
			return 0
		}
		if s.Count <= 0 || s.Count > len(c.LEDs) {
			return len(c.LEDs)
		}
	}
	return max(s.Count, 0)
}

func segmentPoints(s Segment) []Point {
	n := segmentCount(s)
	if n == 0 {
		return nil
	}
	pts := make([]Point, n)
	cx, cy := s.X+s.W/2, s.Y+s.H/2
	rad := s.Angle * math.Pi / 180

	switch s.Shape {
	case "component":
		// grid cell centres scaled into the rectangle, rotated around its centre
		c, _ := LookupComponent(s.Component)
		sin, cos := math.Sin(rad), math.Cos(rad)
		for i := range pts {
			ux, uy := c.unit(c.LEDs[i])
			dx := (ux - 0.5) * s.W
			dy := (uy - 0.5) * s.H
			pts[i] = Point{cx + dx*cos - dy*sin, cy + dx*sin + dy*cos}
		}
	case "point":
		for i := range pts {
			pts[i] = Point{cx, cy}
		}
	case "rect":
		// Evenly along the rectangle outline from the top centre, clockwise
		// (Reverse: counter-clockwise, same first LED); Angle moves the start
		// along the outline (360 = once around).
		per := 2 * (s.W + s.H)
		dir := 1.0
		if s.Reverse {
			dir = -1
		}
		for i := range pts {
			d := math.Mod(per*(dir*float64(i)/float64(n)+s.Angle/360)+per*10, per)
			pts[i] = rectPoint(s, d)
		}
	case "ring":
		// Clockwise around the ellipse inscribed in the rectangle, starting at
		// Angle (0 = top).
		for i := range pts {
			a := rad + 2*math.Pi*float64(i)/float64(s.Count)
			pts[i] = Point{cx + s.W/2*math.Sin(a), cy - s.H/2*math.Cos(a)}
		}
	default: // line through the rectangle centre, Angle 0 = left to right
		dx, dy := math.Cos(rad), math.Sin(rad)
		// half length: distance from the centre to the rectangle edge along the direction
		half := math.Inf(1)
		if dx != 0 {
			half = math.Min(half, math.Abs(s.W/2/dx))
		}
		if dy != 0 {
			half = math.Min(half, math.Abs(s.H/2/dy))
		}
		if math.IsInf(half, 1) {
			half = 0
		}
		for i := range pts {
			t := 0.0
			if s.Count > 1 {
				t = float64(i)/float64(s.Count-1)*2 - 1
			}
			pts[i] = Point{cx + dx*half*t, cy + dy*half*t}
		}
	}

	if s.Reverse && s.Shape != "rect" {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	for i := range pts {
		pts[i].X = clamp01(pts[i].X)
		pts[i].Y = clamp01(pts[i].Y)
	}
	return pts
}

// rectPoint walks d along the outline of s clockwise, starting at the top centre.
func rectPoint(s Segment, d float64) Point {
	edges := []struct {
		len    float64
		x0, y0 float64
		dx, dy float64
	}{
		{s.W / 2, s.X + s.W/2, s.Y, 1, 0},
		{s.H, s.X + s.W, s.Y, 0, 1},
		{s.W, s.X + s.W, s.Y + s.H, -1, 0},
		{s.H, s.X, s.Y + s.H, 0, -1},
		{s.W / 2, s.X, s.Y, 1, 0},
	}
	for _, e := range edges {
		if d <= e.len {
			return Point{e.x0 + e.dx*d, e.y0 + e.dy*d}
		}
		d -= e.len
	}
	return Point{s.X + s.W/2, s.Y}
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
