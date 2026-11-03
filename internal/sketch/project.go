package sketch

import (
	"fmt"
	"math"
)

// Projection is a placed part in view coordinates.
type Projection struct {
	// LEDs are the LED positions in wire order, absolute in the view (mm).
	LEDs []Point `json:"leds"`
	// Box is the part's bounding box in the view.
	Box Rect `json:"box"`
	// EdgeOn is true when the part is seen from the side: the LEDs lie on
	// one edge of Box (a line) and the box is as thick as the part is deep.
	EdgeOn bool `json:"edgeOn"`
}

// ViewOf returns the placement's view ("" = side).
func (p Placement) ViewOf() string {
	if p.View == "" {
		return ViewSide
	}
	return p.View
}

// CanvasViewOf returns the view mapped to the LED canvas ("" = side).
func (s Sketch) CanvasViewOf() string {
	if s.CanvasView == "" {
		return ViewSide
	}
	return s.CanvasView
}

// FaceOf returns the placement's face ("" = the view's own name).
func (p Placement) FaceOf() string {
	if p.Face == "" {
		return p.ViewOf()
	}
	return p.Face
}

// ViewSize returns the extent of a view of a case: front W x H, side D x H,
// top W x D.
func ViewSize(d Dims, view string) (w, h float64, ok bool) {
	switch view {
	case ViewFront:
		return d.W, d.H, true
	case ViewSide:
		return d.D, d.H, true
	case ViewTop:
		return d.W, d.D, true
	}
	return 0, 0, false
}

// faceNormal is the screen direction the LED side points to when a part with
// the given face is drawn in a view. flat means it points at the viewer.
// In the front view the side face points right, in the top view the front
// face points to the top of the drawing.
func faceNormal(view, face string) (nx, ny float64, flat, ok bool) {
	if view == face {
		return 0, 0, true, view == ViewFront || view == ViewSide || view == ViewTop
	}
	switch view {
	case ViewFront:
		switch face {
		case ViewSide:
			return 1, 0, false, true
		case ViewTop:
			return 0, -1, false, true
		}
	case ViewSide:
		switch face {
		case ViewFront:
			return -1, 0, false, true
		case ViewTop:
			return 0, -1, false, true
		}
	case ViewTop:
		switch face {
		case ViewFront:
			return 0, -1, false, true
		case ViewSide:
			return 1, 0, false, true
		}
	}
	return 0, 0, false, false
}

// Project places a part according to the placement (View, X, Y, Rot, Face,
// Mirror, Flow, LEDs, Len) and returns its LEDs in view millimetres.
//
// Steps: resolve the parametric layout; mirror horizontally (Mirror, flipped
// again for a fan with flow "exhaust"); rotate clockwise around the part
// centre; shift so the bounding box starts at (X, Y). If the face is not the
// view's own the part is seen edge-on: the box becomes thin (the part's
// depth) and every LED is projected onto the edge its LED side points to.
// Side and top views therefore need no outline of their own, only the
// depth.
func Project(part Part, pl Placement) (Projection, error) {
	if !finite(pl.X) || !finite(pl.Y) || !finite(pl.Rot) {
		return Projection{}, fmt.Errorf("placement %q: coordinates not finite", pl.ID)
	}
	view, face := pl.ViewOf(), pl.FaceOf()
	nx, ny, flat, ok := faceNormal(view, face)
	if !ok {
		return Projection{}, fmt.Errorf("placement %q: invalid view %q or face %q", pl.ID, view, face)
	}
	res, err := Resolve(part, pl.LEDs, pl.Len)
	if err != nil {
		return Projection{}, err
	}
	w, h, d := res.Dimensions.W, res.Dimensions.H, res.Dimensions.D
	mirror := pl.Mirror
	if pl.Flow == "exhaust" && isFan(res) {
		mirror = !mirror
	}
	rad := pl.Rot * math.Pi / 180
	sin, cos := math.Sin(rad), math.Cos(rad)
	tf := func(x, y float64) (float64, float64) {
		if mirror {
			x = w - x
		}
		dx, dy := x-w/2, y-h/2
		return round6(dx*cos - dy*sin), round6(dx*sin + dy*cos) // relative to the centre
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}} {
		u, v := tf(c[0], c[1])
		minX, maxX = math.Min(minX, u), math.Max(maxX, u)
		minY, maxY = math.Min(minY, v), math.Max(maxY, v)
	}
	bw, bh := maxX-minX, maxY-minY
	out := Projection{LEDs: make([]Point, len(res.LEDs.Positions))}
	switch {
	case flat:
		out.Box = Rect{X: pl.X, Y: pl.Y, W: round6(bw), H: round6(bh)}
	case ny != 0:
		out.EdgeOn = true
		out.Box = Rect{X: pl.X, Y: pl.Y, W: round6(bw), H: d}
	default:
		out.EdgeOn = true
		out.Box = Rect{X: pl.X, Y: pl.Y, W: d, H: round6(bh)}
	}
	for i, q := range res.LEDs.Positions {
		u, v := tf(q[0], q[1])
		u, v = u-minX, v-minY
		var p Point
		switch {
		case flat:
			p = Point{u, v}
		case ny != 0:
			p = Point{u, 0}
			if ny > 0 {
				p.Y = d
			}
		default:
			p = Point{0, v}
			if nx > 0 {
				p.X = d
			}
		}
		out.LEDs[i] = Point{X: round6(pl.X + clamp(p.X, 0, out.Box.W)), Y: round6(pl.Y + clamp(p.Y, 0, out.Box.H))}
	}
	return out, nil
}

// MapToView maps a point of one view of the case into another view through
// the 3D coordinates of the case: w across the width (0 at the left of the
// front view), h down the height, d into the depth (0 at the front). The view
// axes are front (w, h), side (d, h) and top (w, d).
//
// A view lacks one axis; a part drawn there sits on the case face the view
// looks at: front view parts at d = 0 (the front), side view parts at w = W
// (the side panel), top view parts at h = 0 (the top). Mapped into another
// view, a point therefore takes the axis it lacks from that face. A part may
// collapse onto a line in the target view; that is fine.
func MapToView(dims Dims, from, to string, x, y float64) (float64, float64) {
	if from == to {
		return x, y
	}
	var w, h, d float64
	switch from {
	case ViewFront:
		w, h, d = x, y, 0
	case ViewTop:
		w, h, d = x, 0, y
	default: // side
		w, h, d = dims.W, y, x
	}
	switch to {
	case ViewFront:
		return w, h
	case ViewTop:
		return w, d
	default: // side
		return d, h
	}
}
