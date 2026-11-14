package sketch

// Format version 1: parts were placed in one of three flat views (front,
// side, top) with a rotation in the view plane and a face. Version 2 places
// parts in 3D (see orient.go). This file keeps the version 1 types and the
// projection maths that define what a version 1 sketch looked like; Upgrade
// (upgrade.go) converts them.

import (
	"fmt"
	"math"
)

// LegacyVersion is the first sketch format version.
const LegacyVersion = 1

// SketchV1 is the stored sketch of format version 1.
type SketchV1 struct {
	Version    int           `json:"version"`
	Case       Case          `json:"case"`
	CanvasView string        `json:"canvasView"`
	Parts      []PlacementV1 `json:"parts"`
}

// PlacementV1 is one part placed in a view (format version 1).
type PlacementV1 struct {
	ID   string `json:"id"`
	Part string `json:"part"`
	// View holds X and Y: the top left corner of the part's bounding box
	// (after rotation) in that view. "" = side.
	View string  `json:"view"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	// Rot is the clockwise rotation in degrees (-360..360) in the view plane.
	Rot float64 `json:"rot"`
	// Face is the case face the part's LED side points to; "" = the view's own
	// name (seen flat-on). Any other face shows the part edge-on.
	Face string `json:"face"`
	// Mirror flips the part horizontally (before rotation).
	Mirror bool `json:"mirror"`
	// Flow of a fan: "intake", "exhaust" or "".
	Flow string `json:"flow"`
	// LEDs is the LED count of a parametric part (0 = default).
	LEDs int `json:"leds"`
	// Len is the length in mm of a part with a length parameter (0 = default).
	Len   float64  `json:"len,omitempty"`
	Label string   `json:"label"`
	Bind  *Binding `json:"bind,omitempty"`
}

// Projection is a placed part in view coordinates.
type Projection struct {
	// LEDs are the LED positions in wire order, absolute in the view (mm).
	LEDs []Point `json:"leds"`
	// Box is the part's bounding box in the view.
	Box Rect `json:"box"`
	// EdgeOn is true when the part is seen from the side: the LEDs lie on
	// one edge of Box (a line) and the box is as thick as the part is deep.
	EdgeOn bool `json:"edgeOn"`
	// Away is true when the part's LED side points away from the viewer (the
	// part is seen from its back). Only set by ProjectView.
	Away bool `json:"away,omitempty"`
}

// ViewOf returns the placement's view ("" = side).
func (p PlacementV1) ViewOf() string {
	if p.View == "" {
		return ViewSide
	}
	return p.View
}

// FaceOf returns the placement's face ("" = the view's own name).
func (p PlacementV1) FaceOf() string {
	if p.Face == "" {
		return p.ViewOf()
	}
	return p.Face
}

// faceNormal is the screen direction the LED side points to when a part with
// the given face is drawn in a view. flat means it points at the viewer.
// In the front view the side face points right, in the top view the front
// face points to the top of the drawing.
func faceNormalV1(view, face string) (nx, ny float64, flat, ok bool) {
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

// projectV1 places a part according to the placement (View, X, Y, Rot, Face,
// Mirror, Flow, LEDs, Len) and returns its LEDs in view millimetres.
//
// Steps: resolve the parametric layout; mirror horizontally (Mirror, flipped
// again for a fan with flow "exhaust"); rotate clockwise around the part
// centre; shift so the bounding box starts at (X, Y). If the face is not the
// view's own the part is seen edge-on: the box becomes thin (the part's
// depth) and every LED is projected onto the edge its LED side points to.
// Side and top views therefore need no outline of their own, only the
// depth.
func projectV1(part Part, pl PlacementV1) (Projection, error) {
	if !finite(pl.X) || !finite(pl.Y) || !finite(pl.Rot) {
		return Projection{}, fmt.Errorf("placement %q: coordinates not finite", pl.ID)
	}
	view, face := pl.ViewOf(), pl.FaceOf()
	nx, ny, flat, ok := faceNormalV1(view, face)
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
