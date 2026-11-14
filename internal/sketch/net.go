package sketch

import (
	"fmt"
	"math"
)

// CanvasNet is the canvas mapping of a sketch: the unfolded case net (see
// NetLayout). It is the only mapping Derive uses; the older per-view values
// of Sketch.CanvasView are still accepted and mean the same.
const CanvasNet = "net"

// Face assignment.
const (
	// faceNearMM is how close (box distance) a part must be to a wall to
	// belong to it.
	faceNearMM = 40
	// netCollapse is the share of its thickness an empty face keeps in the net.
	netCollapse = 0.2
)

func validFace(f string) bool { return f == "" || validFacing(f) }

func validCanvas(v string) bool { return v == "" || v == CanvasNet || validView(v) }

// wallDist returns the distance from the box to the wall of the case on the
// facing side f (0 when the box touches or crosses it).
func wallDist(d Dims, p3 Projection3D, f string) float64 {
	var v float64
	switch f {
	case FacingSide:
		v = p3.Min.X
	case FacingTray:
		v = d.W - p3.Max.X
	case FacingTop:
		v = p3.Min.Y
	case FacingBottom:
		v = d.H - p3.Max.Y
	case FacingRear:
		v = p3.Min.Z
	case FacingFront:
		v = d.D - p3.Max.Z
	}
	return math.Max(v, 0)
}

// opposite returns the facing on the other side of the case.
func opposite(f string) string {
	switch f {
	case FacingSide:
		return FacingTray
	case FacingTray:
		return FacingSide
	case FacingTop:
		return FacingBottom
	case FacingBottom:
		return FacingTop
	case FacingRear:
		return FacingFront
	case FacingFront:
		return FacingRear
	}
	return ""
}

// DefaultFace is the face of the case net a placement belongs to when it has
// no explicit Face:
//
//  1. a part whose LED side points at a wall it is near (at most 40 mm away)
//     belongs to that wall;
//  2. a fan mounted near a wall with its LED side pointing into the case
//     belongs to that wall, except the tray wall: those are seen through the
//     glass;
//  3. everything else (mainboard, memory, graphics card, interior parts)
//     belongs to the glass side.
func DefaultFace(part Part, pl Placement, c Case) string {
	p3, err := Project3DIn(part, pl, c.Orientation)
	if err != nil {
		return FacingSide
	}
	if wallDist(c.Dims, p3, pl.Facing) <= faceNearMM {
		return pl.Facing
	}
	res, err := Resolve(part, pl.LEDs, pl.Len)
	if err == nil && isFan(res) {
		if w := opposite(pl.Facing); w != "" && w != FacingTray && wallDist(c.Dims, p3, w) <= faceNearMM {
			return w
		}
	}
	return FacingSide
}

// FaceOf returns the explicit face of a placement or its default face.
func FaceOf(part Part, pl Placement, c Case) string {
	if len(pl.Path) > 0 {
		return pathFaces(part, pl, c.Dims)[0] // a case light spans faces: that of its first LED
	}
	if pl.Face != "" {
		return pl.Face
	}
	return DefaultFace(part, pl, c)
}

// ResolveFaces returns the sketch with the face of every placement filled
// in (FaceOf). Placements with an unknown part keep theirs.
func ResolveFaces(s Sketch, c Catalog) Sketch {
	if len(s.Parts) == 0 {
		return s
	}
	parts := make([]Placement, len(s.Parts))
	copy(parts, s.Parts)
	for i, p := range parts {
		if part, ok := c.Parts[p.Part]; ok && p.Face == "" && len(p.Path) == 0 && validFacing(p.Facing) {
			parts[i].Face = DefaultFace(part, p, s.Case)
		}
	}
	s.Parts = parts
	return s
}

// FaceSize returns the extent of a face of the case: side and tray D x H,
// front and rear W x H, top and bottom D x W.
func FaceSize(d Dims, face string) (w, h float64, ok bool) {
	switch face {
	case FacingSide, FacingTray:
		return d.D, d.H, true
	case FacingFront, FacingRear:
		return d.W, d.H, true
	case FacingTop, FacingBottom:
		return d.D, d.W, true
	}
	return 0, 0, false
}

// ToFace projects a point of the case box onto a face, as laid out in the
// net with the glass on the left (see NetLayout). U runs right and V down in
// millimetres from the top left of the face; Depth is the distance from
// someone looking at the face from outside.
//
//	side    u = z      v = y      depth = x
//	front   u = x      v = y      depth = D - z
//	rear    u = W - x  v = y      depth = z
//	tray    u = D - z  v = y      depth = W - x
//	top     u = z      v = W - x  depth = y
//	bottom  u = z      v = x      depth = H - y
//
// Neighbouring faces share their common edge, so a point on an edge has the
// same net position on both faces. NetLayout mirrors the whole net for a
// case with the glass on the right.
func ToFace(d Dims, face string, p Vec3) (ViewPoint, bool) {
	switch face {
	case FacingSide:
		return ViewPoint{p.Z, p.Y, p.X}, true
	case FacingFront:
		return ViewPoint{p.X, p.Y, d.D - p.Z}, true
	case FacingRear:
		return ViewPoint{d.W - p.X, p.Y, p.Z}, true
	case FacingTray:
		return ViewPoint{d.D - p.Z, p.Y, d.W - p.X}, true
	case FacingTop:
		return ViewPoint{p.Z, d.W - p.X, p.Y}, true
	case FacingBottom:
		return ViewPoint{p.Z, p.X, d.H - p.Y}, true
	}
	return ViewPoint{}, false
}

// FromFace inverts ToFace for the two coordinates a face shows. It returns
// the point with those coordinates set and the name of the one coordinate
// the face does not determine; that one is 0.
func FromFace(d Dims, face string, u, v float64) (p Vec3, free string, ok bool) {
	switch face {
	case FacingSide:
		return Vec3{0, v, u}, "x", true
	case FacingFront:
		return Vec3{u, v, 0}, "z", true
	case FacingRear:
		return Vec3{d.W - u, v, 0}, "z", true
	case FacingTray:
		return Vec3{0, v, d.D - u}, "x", true
	case FacingTop:
		return Vec3{d.W - v, 0, u}, "y", true
	case FacingBottom:
		return Vec3{v, 0, u}, "y", true
	}
	return Vec3{}, "", false
}

// NetFace is one face of the case net.
type NetFace struct {
	Face string `json:"face"`
	// X, Y, W, H place the face in the net, millimetres.
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
	// Parts is the number of placements on the face.
	Parts int `json:"parts"`
	// Collapsed is true for an empty face drawn thin.
	Collapsed bool `json:"collapsed,omitempty"`
	// Mirrored is true when the face's u axis runs right to left in the net
	// (glass on the right).
	Mirrored bool `json:"mirrored,omitempty"`
}

// Net is the unfolded case: the glass side in the middle, the top above it,
// the bottom below, the rear and the front on either side and the tray side
// next to the front (only when parts are on it).
type Net struct {
	W     float64   `json:"w"`
	H     float64   `json:"h"`
	Faces []NetFace `json:"faces"`
	dims  Dims
}

// Face returns the net face with the given name.
func (n Net) Face(face string) (NetFace, bool) {
	for _, f := range n.Faces {
		if f.Face == face {
			return f, true
		}
	}
	return NetFace{}, false
}

// Point returns the net position of a case point on a face.
func (n Net) Point(face string, p Vec3) (Point, bool) {
	f, ok := n.Face(face)
	if !ok {
		return Point{}, false
	}
	q, _ := ToFace(n.dims, face, p)
	u := q.U
	if f.Collapsed {
		fw, fh, _ := FaceSize(n.dims, face)
		u, q.V = u*f.W/fw, q.V*f.H/fh
	}
	if f.Mirrored {
		return Point{f.X + f.W - u, f.Y + q.V}, true
	}
	return Point{f.X + u, f.Y + q.V}, true
}

// NetLayout lays out the case net for a sketch. faces gives the face of
// every placement (FaceOf, in placement order). Faces without parts are drawn
// at a fifth of their thickness; the tray side only appears with parts on it.
// With the glass on the right the net is mirrored left to right, so it looks
// like the real case.
func NetLayout(c Case, faces []string) Net {
	d := c.Dims
	count := map[string]int{}
	for _, f := range faces {
		count[f]++
	}
	thick := func(face string, t float64) (float64, bool) {
		if count[face] == 0 && face != FacingSide {
			return round6(t * netCollapse), true
		}
		return t, false
	}
	wr, cr := thick(FacingRear, d.W)
	wf, cf := thick(FacingFront, d.W)
	ht, ct := thick(FacingTop, d.W)
	hb, cb := thick(FacingBottom, d.W)
	n := Net{dims: d}
	add := func(face string, x, y, w, h float64, collapsed bool) {
		n.Faces = append(n.Faces, NetFace{Face: face, X: x, Y: y, W: w, H: h, Parts: count[face], Collapsed: collapsed})
	}
	add(FacingSide, wr, ht, d.D, d.H, false)
	add(FacingTop, wr, 0, d.D, ht, ct)
	add(FacingBottom, wr, ht+d.H, d.D, hb, cb)
	add(FacingRear, 0, ht, wr, d.H, cr)
	add(FacingFront, wr+d.D, ht, wf, d.H, cf)
	n.W = wr + d.D + wf
	if count[FacingTray] > 0 {
		add(FacingTray, n.W, ht, d.D, d.H, false)
		n.W += d.D
	}
	n.H = ht + d.H + hb
	n.W, n.H = round6(n.W), round6(n.H)
	if c.Orientation.GlassRight() {
		for i := range n.Faces {
			f := &n.Faces[i]
			f.X = round6(n.W - f.X - f.W)
			f.Mirrored = true
		}
	}
	return n
}

// SketchNet returns the faces of the placements (FaceOf) and the net layout
// of a sketch. Placements with an unknown part count as glass side. A case
// light counts on every face it has LEDs on.
func SketchNet(s Sketch, c Catalog) ([]string, Net) {
	faces := make([]string, len(s.Parts))
	used := make([]string, 0, len(s.Parts))
	for i, p := range s.Parts {
		faces[i] = FacingSide
		part, ok := c.Parts[p.Part]
		switch {
		case ok && len(p.Path) > 0:
			all := pathFaces(part, p, s.Case.Dims)
			faces[i] = all[0]
			used = append(used, all...)
			continue
		case ok && validFacing(p.Facing):
			faces[i] = FaceOf(part, p, s.Case)
		}
		used = append(used, faces[i])
	}
	return faces, NetLayout(s.Case, used)
}

// ProjectNet projects a placed part onto its face of the net: LED positions
// and the projected box in net millimetres. A case light (Path) is projected
// LED by LED onto the faces it runs over; face is then ignored.
func ProjectNet(part Part, pl Placement, c Case, n Net, face string) (Projection, error) {
	if len(pl.Path) > 0 {
		return projectPath(part, pl, c, n)
	}
	if _, ok := n.Face(face); !ok {
		return Projection{}, fmt.Errorf("placement %q: face %q not in the net", pl.ID, face)
	}
	p3, err := Project3DIn(part, pl, c.Orientation)
	if err != nil {
		return Projection{}, err
	}
	f, _ := FrameOf(face)
	dot := p3.Normal.X*f.N.X + p3.Normal.Y*f.N.Y + p3.Normal.Z*f.N.Z
	out := Projection{LEDs: make([]Point, len(p3.LEDs)), EdgeOn: dot == 0, Away: dot < 0}
	minU, minV, maxU, maxV := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, x := range []float64{p3.Min.X, p3.Max.X} {
		for _, y := range []float64{p3.Min.Y, p3.Max.Y} {
			for _, z := range []float64{p3.Min.Z, p3.Max.Z} {
				q, _ := n.Point(face, Vec3{x, y, z})
				minU, maxU = math.Min(minU, q.X), math.Max(maxU, q.X)
				minV, maxV = math.Min(minV, q.Y), math.Max(maxV, q.Y)
			}
		}
	}
	out.Box = Rect{X: round6(minU), Y: round6(minV), W: round6(maxU - minU), H: round6(maxV - minV)}
	for i, q := range p3.LEDs {
		v, _ := n.Point(face, q)
		out.LEDs[i] = Point{X: round6(clamp(v.X, minU, maxU)), Y: round6(clamp(v.Y, minV, maxV))}
	}
	return out, nil
}
