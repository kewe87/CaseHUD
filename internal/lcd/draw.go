package lcd

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
)

// Drawing primitives. Shapes are rasterised into an alpha mask using signed
// distances (1px soft edge = cheap anti-aliasing) and composited with
// draw.DrawMask. Fast enough for a few frames per second at LCD resolutions.

func parseHex(s string, fallback color.RGBA) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return fallback
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return fallback
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

func withAlpha(c color.RGBA, a float64) color.RGBA {
	return color.RGBA{
		uint8(float64(c.R) * a), uint8(float64(c.G) * a), uint8(float64(c.B) * a), uint8(255 * a),
	}
}

func coverage(d float64) uint8 {
	// d: signed distance, negative inside. 1px linear ramp across the edge.
	a := 0.5 - d
	if a <= 0 {
		return 0
	}
	if a >= 1 {
		return 255
	}
	return uint8(a * 255)
}

// fillMask paints c through a mask computed by sdf over rect r.
func fillMask(dst draw.Image, r image.Rectangle, c color.Color, sdf func(x, y float64) float64) {
	r = r.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	mask := image.NewAlpha(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			mask.SetAlpha(x, y, color.Alpha{coverage(sdf(float64(x)+0.5, float64(y)+0.5))})
		}
	}
	draw.DrawMask(dst, r, image.NewUniform(c), image.Point{}, mask, r.Min, draw.Over)
}

// roundRect fills a rectangle with corner radius rad. The straight parts
// are filled directly; only the four corner squares need anti-aliasing.
func roundRect(dst draw.Image, r image.Rectangle, rad float64, c color.Color) {
	r = r.Canon()
	ir := int(math.Ceil(rad))
	if ir*2 > r.Dx() || ir*2 > r.Dy() {
		ir = min(r.Dx(), r.Dy()) / 2
		rad = float64(ir)
	}
	u := image.NewUniform(c)
	draw.Draw(dst, image.Rect(r.Min.X+ir, r.Min.Y, r.Max.X-ir, r.Max.Y), u, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y+ir, r.Min.X+ir, r.Max.Y-ir), u, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(r.Max.X-ir, r.Min.Y+ir, r.Max.X, r.Max.Y-ir), u, image.Point{}, draw.Over)
	corners := []struct {
		box    image.Rectangle
		cx, cy float64
	}{
		{image.Rect(r.Min.X, r.Min.Y, r.Min.X+ir, r.Min.Y+ir), float64(r.Min.X) + rad, float64(r.Min.Y) + rad},
		{image.Rect(r.Max.X-ir, r.Min.Y, r.Max.X, r.Min.Y+ir), float64(r.Max.X) - rad, float64(r.Min.Y) + rad},
		{image.Rect(r.Min.X, r.Max.Y-ir, r.Min.X+ir, r.Max.Y), float64(r.Min.X) + rad, float64(r.Max.Y) - rad},
		{image.Rect(r.Max.X-ir, r.Max.Y-ir, r.Max.X, r.Max.Y), float64(r.Max.X) - rad, float64(r.Max.Y) - rad},
	}
	for _, k := range corners {
		cx, cy := k.cx, k.cy
		fillMask(dst, k.box, c, func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - rad })
	}
}

// arc draws a ring segment with round caps. Angles are in degrees, measured
// clockwise from 3 o'clock (screen coordinates), sweep may be up to 360.
func arc(dst draw.Image, cx, cy, radius, thickness, startDeg, sweepDeg float64, c color.Color) {
	arcWith(dst, cx, cy, radius, thickness, startDeg, sweepDeg, c, true)
}

// arcFlat is arc with straight (radial) ends, for segmented gauges.
func arcFlat(dst draw.Image, cx, cy, radius, thickness, startDeg, sweepDeg float64, c color.Color) {
	arcWith(dst, cx, cy, radius, thickness, startDeg, sweepDeg, c, false)
}

// arcBounds is the pixel box that can be touched by an arc: the sector's
// end points on both radii plus every axis extreme the sweep passes, padded
// by the cap radius and the anti-aliasing edge. Short arcs (gauge segments)
// then scan a small box instead of the whole ring.
func arcBounds(cx, cy, radius, half, startDeg, sweepDeg float64) image.Rectangle {
	ext := radius + half + 2
	if sweepDeg >= 360 {
		return image.Rect(int(cx-ext), int(cy-ext), int(math.Ceil(cx+ext)), int(math.Ceil(cy+ext)))
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(deg float64) {
		a := deg * math.Pi / 180
		x, y := cx+radius*math.Cos(a), cy+radius*math.Sin(a)
		minX, minY, maxX, maxY = math.Min(minX, x), math.Min(minY, y), math.Max(maxX, x), math.Max(maxY, y)
	}
	add(startDeg)
	add(startDeg + sweepDeg)
	for k := math.Ceil(startDeg/90) * 90; k < startDeg+sweepDeg; k += 90 {
		add(k)
	}
	pad := half + 2
	return image.Rect(int(minX-pad), int(minY-pad), int(math.Ceil(maxX+pad)), int(math.Ceil(maxY+pad)))
}

func arcWith(dst draw.Image, cx, cy, radius, thickness, startDeg, sweepDeg float64, c color.Color, roundCaps bool) {
	if sweepDeg <= 0 {
		return
	}
	half := thickness / 2
	s := startDeg * math.Pi / 180
	e := (startDeg + sweepDeg) * math.Pi / 180
	sx, sy := cx+radius*math.Cos(s), cy+radius*math.Sin(s)
	ex, ey := cx+radius*math.Cos(e), cy+radius*math.Sin(e)
	r := arcBounds(cx, cy, radius, half, startDeg, sweepDeg).Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	// Only pixels within the ring band (plus 1px for anti-aliasing) can be
	// covered, caps included, so each row is scanned only where it crosses
	// the annulus instead of across the whole bounding square.
	rOut, rIn := radius+half+1, math.Max(0, radius-half-1)
	mask := image.NewAlpha(r)
	sdf := func(x, y float64) float64 {
		dx, dy := x-cx, y-cy
		d := math.Sqrt(dx*dx + dy*dy)
		ring := math.Abs(d-radius) - half
		a := sweepAngle(dx, dy, startDeg)
		if a <= sweepDeg {
			if !roundCaps {
				// Soften the radial edges by their distance in pixels.
				edge := -math.Min(a, sweepDeg-a) * math.Pi / 180 * d
				return math.Max(ring, edge)
			}
			return ring
		}
		if !roundCaps {
			out := math.Min(a-sweepDeg, 360-a) * math.Pi / 180 * d
			return math.Max(ring, out)
		}
		return capDist(x, y, sx, sy, ex, ey) - half
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		dy := float64(y) + 0.5 - cy
		if math.Abs(dy) > rOut {
			continue
		}
		xo := math.Sqrt(rOut*rOut - dy*dy)
		xi := 0.0
		if math.Abs(dy) < rIn {
			xi = math.Sqrt(rIn*rIn - dy*dy)
		}
		for _, span := range [2][2]float64{{cx - xo, cx - xi}, {cx + xi, cx + xo}} {
			x0 := max(r.Min.X, int(math.Floor(span[0])))
			x1 := min(r.Max.X, int(math.Ceil(span[1]))+1)
			for x := x0; x < x1; x++ {
				if v := coverage(sdf(float64(x)+0.5, float64(y)+0.5)); v > 0 {
					mask.SetAlpha(x, y, color.Alpha{v})
				}
			}
		}
	}
	draw.DrawMask(dst, r, image.NewUniform(c), image.Point{}, mask, r.Min, draw.Over)
}

// polyline draws a connected line through pts.
func polyline(dst draw.Image, pts []image.Point, thickness float64, c color.Color) {
	if len(pts) < 2 {
		return
	}
	r := image.Rectangle{Min: pts[0], Max: pts[0]}
	for _, p := range pts {
		r = r.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
	}
	pad := int(thickness) + 2
	r = r.Inset(-pad)
	half := thickness / 2
	fillMask(dst, r, c, func(x, y float64) float64 {
		best := math.MaxFloat64
		for i := 1; i < len(pts); i++ {
			ax, ay := float64(pts[i-1].X), float64(pts[i-1].Y)
			bx, by := float64(pts[i].X), float64(pts[i].Y)
			if x < math.Min(ax, bx)-half-1 || x > math.Max(ax, bx)+half+1 {
				continue
			}
			best = math.Min(best, segDist(x, y, ax, ay, bx, by))
		}
		return best - half
	})
}

func segDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// formatValue renders v with the given number of decimals.
// With comma set (German UI) the decimal separator is a comma.
func formatValue(v float64, decimals int, comma bool) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if comma {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// sweepAngle is the clockwise angle of (dx, dy) from startDeg in [0, 360).
func sweepAngle(dx, dy, startDeg float64) float64 {
	a := math.Mod(math.Atan2(dy, dx)*180/math.Pi-startDeg, 360)
	if math.IsNaN(a) { // NaN or infinite input
		return 0
	}
	if a < 0 {
		a += 360
	}
	if a >= 360 { // a tiny negative a rounds up to 360
		a = 0
	}
	return a
}

func capDist(x, y, sx, sy, ex, ey float64) float64 {
	ds := (x-sx)*(x-sx) + (y-sy)*(y-sy)
	de := (x-ex)*(x-ex) + (y-ey)*(y-ey)
	return math.Sqrt(math.Min(ds, de))
}

// arcGlow draws a soft halo around a round-capped arc: len(thicknesses)
// translucent arcs of the given widths stacked on top of each other, each
// with alpha a. It is computed in one pass, since every layer shares the
// same distance field and only its width differs.
func arcGlow(dst draw.Image, cx, cy, radius float64, thicknesses []float64, startDeg, sweepDeg float64, c color.RGBA, a float64) {
	if sweepDeg <= 0 || len(thicknesses) == 0 {
		return
	}
	maxHalf := 0.0
	for _, t := range thicknesses {
		maxHalf = math.Max(maxHalf, t/2)
	}
	s := startDeg * math.Pi / 180
	e := (startDeg + sweepDeg) * math.Pi / 180
	sx, sy := cx+radius*math.Cos(s), cy+radius*math.Sin(s)
	ex, ey := cx+radius*math.Cos(e), cy+radius*math.Sin(e)
	r := arcBounds(cx, cy, radius, maxHalf, startDeg, sweepDeg).Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	rOut, rIn := radius+maxHalf+1, math.Max(0, radius-maxHalf-1)
	mask := image.NewAlpha(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		py := float64(y) + 0.5
		dy := py - cy
		if math.Abs(dy) > rOut {
			continue
		}
		xo := math.Sqrt(rOut*rOut - dy*dy)
		xi := 0.0
		if math.Abs(dy) < rIn {
			xi = math.Sqrt(rIn*rIn - dy*dy)
		}
		for _, span := range [2][2]float64{{cx - xo, cx - xi}, {cx + xi, cx + xo}} {
			x0 := max(r.Min.X, int(math.Floor(span[0])))
			x1 := min(r.Max.X, int(math.Ceil(span[1]))+1)
			for x := x0; x < x1; x++ {
				px := float64(x) + 0.5
				dx := px - cx
				var base float64
				if sweepAngle(dx, dy, startDeg) <= sweepDeg {
					base = math.Abs(math.Sqrt(dx*dx+dy*dy) - radius)
				} else {
					base = capDist(px, py, sx, sy, ex, ey)
				}
				keep := 1.0 // 1 - accumulated alpha of the stacked layers
				for _, t := range thicknesses {
					keep *= 1 - a*float64(coverage(base-t/2))/255
				}
				if v := uint8((1 - keep) * 255); v > 0 {
					mask.SetAlpha(x, y, color.Alpha{v})
				}
			}
		}
	}
	solid := color.RGBA{c.R, c.G, c.B, 0xff}
	draw.DrawMask(dst, r, image.NewUniform(solid), image.Point{}, mask, r.Min, draw.Over)
}
