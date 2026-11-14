package sketch

import (
	"fmt"
	"math"
)

// Orientation tells how a case is built, for labelling and drawing the views.
// Empty fields mean the default: the side view shows the rear on its left and
// the glass is on the left of the case when seen from the front.
//
// The model itself does not change with the orientation: x = 0 is always the
// glass side and z = 0 the rear. Case templates that draw their side view with
// the front on the left are converted to the rear-left convention when they
// are parsed (ParseCase), so SideViewLeft only records what the template
// source said. GlassSide "right" mirrors the horizontal axis of the views that
// show x (Vorne, Hinten, Oben, Unten), see ToView, and the right-hand side of
// every part, see FrameIn.
type Orientation struct {
	SideViewLeft string `json:"side_view_left,omitempty"` // "rear" (default) or "front"
	GlassSide    string `json:"glass_side,omitempty"`     // "left" (default) or "right"
}

// Orientation values.
const (
	OrientRear  = "rear"
	OrientFront = "front"
	OrientLeft  = "left"
	OrientRight = "right"
)

func (o Orientation) validate() error {
	switch o.SideViewLeft {
	case "", OrientRear, OrientFront:
	default:
		return fmt.Errorf("invalid side_view_left %q", o.SideViewLeft)
	}
	switch o.GlassSide {
	case "", OrientLeft, OrientRight:
	default:
		return fmt.Errorf("invalid glass_side %q", o.GlassSide)
	}
	return nil
}

// Normalized returns the orientation with the defaults filled in.
func (o Orientation) Normalized() Orientation {
	if o.SideViewLeft == "" {
		o.SideViewLeft = OrientRear
	}
	if o.GlassSide == "" {
		o.GlassSide = OrientLeft
	}
	return o
}

// FrontOnSideLeft reports whether the template source drew the side view with
// the front on the left.
func (o Orientation) FrontOnSideLeft() bool { return o.SideViewLeft == OrientFront }

// GlassRight reports whether the glass is on the right of the case seen from
// the front.
func (o Orientation) GlassRight() bool { return o.GlassSide == OrientRight }

// Frame is the local frame of a facing: the part's right (U) and down (V)
// directions seen from the facing side, and the outward normal N of the LED
// side. Looking along -N with V down, U points right.
type Frame struct {
	U, V, N Vec3
}

// FrameOf returns the frame of a facing.
//
//	front   U +x  V +y  N +z
//	rear    U -x  V +y  N -z
//	side    U +z  V +y  N -x   (glass side)
//	tray    U -z  V +y  N +x
//	top     U +x  V +z  N -y
//	bottom  U +x  V -z  N +y
func FrameOf(facing string) (Frame, bool) {
	switch facing {
	case FacingFront:
		return Frame{Vec3{1, 0, 0}, Vec3{0, 1, 0}, Vec3{0, 0, 1}}, true
	case FacingRear:
		return Frame{Vec3{-1, 0, 0}, Vec3{0, 1, 0}, Vec3{0, 0, -1}}, true
	case FacingSide:
		return Frame{Vec3{0, 0, 1}, Vec3{0, 1, 0}, Vec3{-1, 0, 0}}, true
	case FacingTray:
		return Frame{Vec3{0, 0, -1}, Vec3{0, 1, 0}, Vec3{1, 0, 0}}, true
	case FacingTop:
		return Frame{Vec3{1, 0, 0}, Vec3{0, 0, 1}, Vec3{0, -1, 0}}, true
	case FacingBottom:
		return Frame{Vec3{1, 0, 0}, Vec3{0, 0, -1}, Vec3{0, 1, 0}}, true
	}
	return Frame{}, false
}

// FrameIn is FrameOf for a case with the given orientation. With the glass on
// the right the case is the mirror image of the default one (x runs from the
// glass), so what someone looking at a part from outside sees as its right
// is the opposite direction: U is negated. Parts then look the same in a
// glass-right case as in a glass-left one, and the LED order follows.
func FrameIn(facing string, o Orientation) (Frame, bool) {
	fr, ok := FrameOf(facing)
	if ok && o.GlassRight() {
		fr.U = Vec3{-fr.U.X + 0, -fr.U.Y + 0, -fr.U.Z + 0}
	}
	return fr, ok
}

func validFacing(f string) bool { _, ok := FrameOf(f); return ok }

func validView(v string) bool {
	_, ok := ViewFacing(v)
	return ok
}

// ViewFacing is the facing whose LED side points at someone looking at the
// view: parts with this facing are seen flat-on, parts with the opposite
// facing from behind.
func ViewFacing(view string) (string, bool) {
	switch view {
	case ViewFront:
		return FacingFront, true
	case ViewRear:
		return FacingRear, true
	case ViewSide:
		return FacingSide, true
	case ViewTop:
		return FacingTop, true
	case ViewBottom:
		return FacingBottom, true
	}
	return "", false
}

// ViewSize returns the extent of a view of a case: Seite D x H, Vorne and
// Hinten W x H, Oben and Unten W x D.
func ViewSize(d Dims, view string) (w, h float64, ok bool) {
	switch view {
	case ViewFront, ViewRear:
		return d.W, d.H, true
	case ViewSide:
		return d.D, d.H, true
	case ViewTop, ViewBottom:
		return d.W, d.D, true
	}
	return 0, 0, false
}

// ViewPoint is a point of the case box in a view: U to the right and V down
// in millimetres from the top left of the view, Depth the distance from the
// viewer (0 at the nearest wall; larger is further away, for draw order).
type ViewPoint struct {
	U, V, Depth float64
}

// ToView projects a point of the case box into a view.
//
//	side    u = z      v = y      depth = x
//	front   u = x      v = y      depth = D - z
//	rear    u = W - x  v = y      depth = z
//	top     u = x      v = z      depth = y      (front at the bottom)
//	bottom  u = x      v = D - z  depth = H - y  (front at the top)
//
// With the glass on the right (Case.Orientation) u of front, top and bottom is
// W - x and u of rear is x, so the picture matches the real case. Side keeps
// u = z: its data is always rear-left.
func ToView(c Case, view string, p Vec3) (ViewPoint, bool) {
	d := c.Dims
	fx := p.X // horizontal position as seen from the front
	if c.Orientation.GlassRight() {
		fx = d.W - p.X
	}
	switch view {
	case ViewSide:
		return ViewPoint{p.Z, p.Y, p.X}, true
	case ViewFront:
		return ViewPoint{fx, p.Y, d.D - p.Z}, true
	case ViewRear:
		return ViewPoint{d.W - fx, p.Y, p.Z}, true
	case ViewTop:
		return ViewPoint{fx, p.Z, p.Y}, true
	case ViewBottom:
		return ViewPoint{fx, d.D - p.Z, d.H - p.Y}, true
	}
	return ViewPoint{}, false
}

// FromView inverts ToView for the two coordinates a view shows. It returns
// the point with those coordinates set and the name ("x", "y" or "z") of the
// one coordinate the view does not determine; that one is 0.
func FromView(c Case, view string, u, v float64) (p Vec3, free string, ok bool) {
	d := c.Dims
	// fx is the horizontal position seen from the front, x or W - x.
	fromFx := func(fx float64) float64 {
		if c.Orientation.GlassRight() {
			return d.W - fx
		}
		return fx
	}
	switch view {
	case ViewSide:
		return Vec3{0, v, u}, "x", true
	case ViewFront:
		return Vec3{fromFx(u), v, 0}, "z", true
	case ViewRear:
		return Vec3{fromFx(d.W - u), v, 0}, "z", true
	case ViewTop:
		return Vec3{fromFx(u), 0, v}, "y", true
	case ViewBottom:
		return Vec3{fromFx(u), 0, d.D - v}, "y", true
	}
	return Vec3{}, "", false
}

// Projection3D is a placed part in the case box.
type Projection3D struct {
	// LEDs are the LED positions in wire order.
	LEDs []Vec3 `json:"leds"`
	// Min and Max span the part's axis-aligned box.
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
	// Normal is the outward unit vector of the LED side.
	Normal Vec3 `json:"normal"`
}

// Project3D places a part in a case with the default orientation. Its box is
// the same in every orientation; use Project3DIn for the LED positions.
func Project3D(part Part, pl Placement) (Projection3D, error) {
	return Project3DIn(part, pl, Orientation{})
}

// Project3DIn places a part in the case box of a case with orientation o.
//
// Steps: resolve the parametric layout; mirror horizontally (Mirror, flipped
// again for a fan with flow "exhaust"); rotate clockwise by Spin around the
// part centre in the plane of the facing; map the LED (a, b) to
// Pos + du*U + dv*V + (d/2)*N with the frame of the facing. The box is
// spanned by the eight corners of the rotated part.
func Project3DIn(part Part, pl Placement, o Orientation) (Projection3D, error) {
	if !finite(pl.Pos.X) || !finite(pl.Pos.Y) || !finite(pl.Pos.Z) || !finite(pl.Spin) {
		return Projection3D{}, fmt.Errorf("placement %q: coordinates not finite", pl.ID)
	}
	fr, ok := FrameIn(pl.Facing, o)
	if !ok {
		return Projection3D{}, fmt.Errorf("placement %q: invalid facing %q", pl.ID, pl.Facing)
	}
	res, err := Resolve(part, pl.LEDs, pl.Len)
	if err != nil {
		return Projection3D{}, err
	}
	w, h, d := res.Dimensions.W, res.Dimensions.H, res.Dimensions.D
	mirror := pl.Mirror
	if pl.Flow == "exhaust" && isFan(res) {
		mirror = !mirror
	}
	rad := pl.Spin * math.Pi / 180
	sin, cos := math.Sin(rad), math.Cos(rad)
	tf := func(a, b float64) (float64, float64) {
		if mirror {
			a = w - a
		}
		dx, dy := a-w/2, b-h/2
		return round6(dx*cos - dy*sin), round6(dx*sin + dy*cos)
	}
	at := func(du, dv, dn float64) Vec3 {
		return Vec3{
			pl.Pos.X + du*fr.U.X + dv*fr.V.X + dn*fr.N.X,
			pl.Pos.Y + du*fr.U.Y + dv*fr.V.Y + dn*fr.N.Y,
			pl.Pos.Z + du*fr.U.Z + dv*fr.V.Z + dn*fr.N.Z,
		}
	}
	out := Projection3D{LEDs: make([]Vec3, len(res.LEDs.Positions)), Normal: fr.N}
	first := true
	for _, c := range [][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}} {
		du, dv := tf(c[0], c[1])
		for _, dn := range []float64{-d / 2, d / 2} {
			q := at(du, dv, dn)
			if first {
				out.Min, out.Max, first = q, q, false
				continue
			}
			out.Min = Vec3{math.Min(out.Min.X, q.X), math.Min(out.Min.Y, q.Y), math.Min(out.Min.Z, q.Z)}
			out.Max = Vec3{math.Max(out.Max.X, q.X), math.Max(out.Max.Y, q.Y), math.Max(out.Max.Z, q.Z)}
		}
	}
	out.Min, out.Max = roundVec(out.Min), roundVec(out.Max)
	for i, q := range res.LEDs.Positions {
		du, dv := tf(q[0], q[1])
		p := roundVec(at(du, dv, d/2))
		out.LEDs[i] = Vec3{clamp(p.X, out.Min.X, out.Max.X), clamp(p.Y, out.Min.Y, out.Max.Y), clamp(p.Z, out.Min.Z, out.Max.Z)}
	}
	return out, nil
}

func roundVec(v Vec3) Vec3 { return Vec3{round6(v.X), round6(v.Y), round6(v.Z)} }

// ProjectView projects a placed part into a view of the case. The result is
// flat (LED side towards the viewer), Away (seen from behind: the picture is
// the part mirrored) or EdgeOn (a thin box with the LEDs on the edge the LED
// side points to). Box is the projected 3D box.
func ProjectView(part Part, pl Placement, c Case, view string) (Projection, error) {
	vf, ok := ViewFacing(view)
	if !ok {
		return Projection{}, fmt.Errorf("invalid view %q", view)
	}
	p3, err := Project3DIn(part, pl, c.Orientation)
	if err != nil {
		return Projection{}, err
	}
	vfr, _ := FrameOf(vf)
	dot := p3.Normal.X*vfr.N.X + p3.Normal.Y*vfr.N.Y + p3.Normal.Z*vfr.N.Z
	out := Projection{LEDs: make([]Point, len(p3.LEDs)), EdgeOn: dot == 0, Away: dot < 0}
	minU, minV, maxU, maxV := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, x := range []float64{p3.Min.X, p3.Max.X} {
		for _, y := range []float64{p3.Min.Y, p3.Max.Y} {
			for _, z := range []float64{p3.Min.Z, p3.Max.Z} {
				q, _ := ToView(c, view, Vec3{x, y, z})
				minU, maxU = math.Min(minU, q.U), math.Max(maxU, q.U)
				minV, maxV = math.Min(minV, q.V), math.Max(maxV, q.V)
			}
		}
	}
	out.Box = Rect{X: round6(minU), Y: round6(minV), W: round6(maxU - minU), H: round6(maxV - minV)}
	for i, q := range p3.LEDs {
		v, _ := ToView(c, view, q)
		out.LEDs[i] = Point{X: round6(clamp(v.U, minU, maxU)), Y: round6(clamp(v.V, minV, maxV))}
	}
	return out, nil
}
