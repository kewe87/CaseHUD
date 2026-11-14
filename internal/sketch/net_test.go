package sketch

import (
	"math"
	"testing"
)

// allFaces puts one part on every face so no face is collapsed.
func allFaces(cs Case) []string {
	return []string{FacingSide, FacingTop, FacingBottom, FacingRear, FacingFront, FacingTray}
}

// Neighbouring faces of the net share their common case edge: every point
// on that edge lands on the same net position from both faces, in both
// orientations. An effect therefore runs on across the edges.
func TestNetEdgesAreContinuous(t *testing.T) {
	for _, glass := range []string{OrientLeft, OrientRight} {
		cs := box()
		cs.Orientation.GlassSide = glass
		d := cs.Dims
		n := NetLayout(cs, allFaces(cs))
		edges := []struct {
			a, b string
			at   func(s float64) Vec3 // s runs 0..1 along the edge
		}{
			{FacingSide, FacingTop, func(s float64) Vec3 { return Vec3{0, 0, s * d.D} }},
			{FacingSide, FacingBottom, func(s float64) Vec3 { return Vec3{0, d.H, s * d.D} }},
			{FacingSide, FacingRear, func(s float64) Vec3 { return Vec3{0, s * d.H, 0} }},
			{FacingSide, FacingFront, func(s float64) Vec3 { return Vec3{0, s * d.H, d.D} }},
			{FacingFront, FacingTray, func(s float64) Vec3 { return Vec3{d.W, s * d.H, d.D} }},
		}
		for _, e := range edges {
			for _, s := range []float64{0, 0.25, 0.5, 1} {
				p := e.at(s)
				pa, oka := n.Point(e.a, p)
				pb, okb := n.Point(e.b, p)
				if !oka || !okb || math.Abs(pa.X-pb.X) > 1e-6 || math.Abs(pa.Y-pb.Y) > 1e-6 {
					t.Errorf("glass %s: edge %s/%s at %v: %v vs %v", glass, e.a, e.b, p, pa, pb)
				}
			}
		}
	}
}

// Every face covers exactly the area of the case face, faces do not overlap
// and every case point of a face lands inside its net rectangle.
func TestNetFacesDoNotOverlap(t *testing.T) {
	for _, glass := range []string{OrientLeft, OrientRight} {
		cs := box()
		cs.Orientation.GlassSide = glass
		n := NetLayout(cs, allFaces(cs))
		if len(n.Faces) != 6 {
			t.Fatalf("faces %+v", n.Faces)
		}
		for i, a := range n.Faces {
			w, h, _ := FaceSize(cs.Dims, a.Face)
			if !near(a.W, w) || !near(a.H, h) || a.X < 0 || a.Y < 0 || a.X+a.W > n.W+1e-9 || a.Y+a.H > n.H+1e-9 {
				t.Errorf("face %+v (size %g x %g) in net %g x %g", a, w, h, n.W, n.H)
			}
			for _, b := range n.Faces[i+1:] {
				if a.X < b.X+b.W-1e-9 && b.X < a.X+a.W-1e-9 && a.Y < b.Y+b.H-1e-9 && b.Y < a.Y+a.H-1e-9 {
					t.Errorf("faces %s and %s overlap", a.Face, b.Face)
				}
			}
			d := cs.Dims
			for _, p := range []Vec3{{0, 0, 0}, {d.W, d.H, d.D}, {d.W / 2, d.H / 3, d.D / 4}} {
				q, _ := n.Point(a.Face, p)
				if q.X < a.X-1e-9 || q.X > a.X+a.W+1e-9 || q.Y < a.Y-1e-9 || q.Y > a.Y+a.H+1e-9 {
					t.Errorf("point %v on %s at %v outside %+v", p, a.Face, q, a)
				}
			}
		}
	}
}

// The glass face is in the middle with the rear on its left (glass left) or
// on its right (glass right, the net mirrored), the top above it, the bottom
// below; FromFace inverts ToFace.
func TestNetOrientation(t *testing.T) {
	cs := box()
	n := NetLayout(cs, allFaces(cs))
	side, _ := n.Face(FacingSide)
	rear, _ := n.Face(FacingRear)
	front, _ := n.Face(FacingFront)
	top, _ := n.Face(FacingTop)
	bottom, _ := n.Face(FacingBottom)
	tray, _ := n.Face(FacingTray)
	if !(rear.X < side.X && side.X < front.X && front.X < tray.X && top.Y < side.Y && side.Y < bottom.Y) {
		t.Errorf("glass left layout %+v", n.Faces)
	}
	cs.Orientation.GlassSide = OrientRight
	m := NetLayout(cs, allFaces(cs))
	mside, _ := m.Face(FacingSide)
	mrear, _ := m.Face(FacingRear)
	mfront, _ := m.Face(FacingFront)
	if !(mfront.X < mside.X && mside.X < mrear.X) || !mside.Mirrored {
		t.Errorf("glass right layout %+v", m.Faces)
	}
	// the rear of the case is on the left of the glass face with the glass on
	// the left, on the right with the glass on the right
	r1, _ := n.Point(FacingSide, Vec3{0, 0, 0})
	r2, _ := m.Point(FacingSide, Vec3{0, 0, 0})
	if !near(r1.X, side.X) || !near(r2.X, mside.X+mside.W) {
		t.Errorf("rear corner %v / %v", r1, r2)
	}
	for _, f := range allFaces(cs) {
		p := Vec3{40, 70, 90}
		q, _ := ToFace(cs.Dims, f, p)
		back, free, ok := FromFace(cs.Dims, f, q.U, q.V)
		if !ok {
			t.Fatal(f)
		}
		switch free {
		case "x":
			back.X = p.X
		case "y":
			back.Y = p.Y
		case "z":
			back.Z = p.Z
		}
		if back != p {
			t.Errorf("face %s: %v -> %v -> %v", f, p, q, back)
		}
	}
}

// Empty faces are drawn thin and the tray side only appears with parts on it.
func TestNetCollapsesEmptyFaces(t *testing.T) {
	cs := box()
	n := NetLayout(cs, []string{FacingSide, FacingTop})
	if _, ok := n.Face(FacingTray); ok {
		t.Error("empty tray face shown")
	}
	top, _ := n.Face(FacingTop)
	rear, _ := n.Face(FacingRear)
	bottom, _ := n.Face(FacingBottom)
	if top.Collapsed || !near(top.H, cs.Dims.W) || top.Parts != 1 {
		t.Errorf("top %+v", top)
	}
	if !rear.Collapsed || !near(rear.W, cs.Dims.W*netCollapse) || !bottom.Collapsed || !near(bottom.H, cs.Dims.W*netCollapse) {
		t.Errorf("rear %+v bottom %+v", rear, bottom)
	}
	if !near(n.W, cs.Dims.W*netCollapse*2+cs.Dims.D) || !near(n.H, cs.Dims.W+cs.Dims.H+cs.Dims.W*netCollapse) {
		t.Errorf("net %g x %g", n.W, n.H)
	}
	if n := NetLayout(cs, nil); len(n.Faces) != 5 || n.Faces[0].Collapsed {
		t.Errorf("empty sketch net %+v", n.Faces)
	}
}

// The default face: parts facing a near wall belong to it, fans mounted on a
// wall blowing in belong to that wall (not the tray), the rest to the glass.
func TestDefaultFace(t *testing.T) {
	cs := box()
	cat := Builtin()
	fan := cat.Parts["generic-fan-120"]
	ram := cat.Parts["generic-ram-dimm"]
	cases := []struct {
		name string
		part Part
		pl   Placement
		want string
	}{
		{"top fan", fan, Placement{ID: "a", Facing: FacingTop, Pos: Vec3{115, 14, 200}}, FacingTop},
		{"rear fan", fan, Placement{ID: "a", Facing: FacingRear, Pos: Vec3{115, 100, 18}}, FacingRear},
		{"glass fan", fan, Placement{ID: "a", Facing: FacingSide, Pos: Vec3{14, 200, 300}}, FacingSide},
		{"front fan", fan, Placement{ID: "a", Facing: FacingFront, Pos: Vec3{115, 200, 437}}, FacingFront},
		{"bottom fan blowing in", fan, Placement{ID: "a", Facing: FacingTop, Pos: Vec3{115, 466, 200}}, FacingBottom},
		{"top fan blowing in", fan, Placement{ID: "a", Facing: FacingBottom, Pos: Vec3{115, 14, 200}}, FacingTop},
		{"tray fan blowing in", fan, Placement{ID: "a", Facing: FacingSide, Pos: Vec3{216, 200, 200}}, FacingSide},
		{"tray fan outward", fan, Placement{ID: "a", Facing: FacingTray, Pos: Vec3{216, 200, 200}}, FacingTray},
		{"interior fan", fan, Placement{ID: "a", Facing: FacingTop, Pos: Vec3{115, 200, 200}}, FacingSide},
		{"memory near the rear", ram, Placement{ID: "a", Facing: FacingRear, Pos: Vec3{115, 100, 150}}, FacingSide},
		{"memory facing the top wall", ram, Placement{ID: "a", Facing: FacingBottom, Pos: Vec3{115, 20, 150}}, FacingSide},
	}
	for _, c := range cases {
		if got := DefaultFace(c.part, c.pl, cs); got != c.want {
			t.Errorf("%s: face %s, want %s", c.name, got, c.want)
		}
		pl := c.pl
		pl.Face = FacingBottom
		if got := FaceOf(c.part, pl, cs); got != FacingBottom {
			t.Errorf("%s: explicit face ignored: %s", c.name, got)
		}
	}
}

// Every part is on exactly one face: it is derived once, inside that face's
// rectangle, and ResolveFaces fills in the default faces without touching
// explicit ones.
func TestDeriveNetEachPartOnce(t *testing.T) {
	cs := box()
	bind := func(z string, o int) *Binding { return &Binding{Device: "d", Zone: z, Order: o} }
	parts := []Placement{
		{ID: "tp", Part: "generic-fan-120", Facing: FacingTop, Pos: Vec3{115, 12.5, 150}, Bind: bind("a", 0)},
		{ID: "rr", Part: "generic-fan-120", Facing: FacingRear, Pos: Vec3{115, 100, 12.5}, Bind: bind("a", 1)},
		flatSide("sd", "generic-fan-120", 100, 200, 0, 0),
		{ID: "tr", Part: "generic-fan-120", Facing: FacingTray, Pos: Vec3{217.5, 300, 200}, Bind: bind("b", 0)},
		{ID: "ex", Part: "generic-fan-120", Facing: FacingSide, Face: FacingFront, Pos: Vec3{100, 240, 400}, Bind: bind("b", 1)},
	}
	parts[2].Bind = bind("a", 2)
	s := Sketch{Version: Version, Case: cs, CanvasView: CanvasNet, Parts: parts}
	r := ResolveFaces(s, Builtin())
	want := []string{FacingTop, FacingRear, FacingSide, FacingTray, FacingFront}
	for i, p := range r.Parts {
		if p.Face != want[i] {
			t.Errorf("part %s: face %q, want %s", p.ID, p.Face, want[i])
		}
	}
	if s.Parts[0].Face != "" {
		t.Error("ResolveFaces changed its input")
	}
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Components) != len(parts) {
		t.Fatalf("components %d", len(d.Components))
	}
	_, net := SketchNet(s, Builtin())
	ox, oy, sx, sy := Letterbox(net.W, net.H)
	seen := map[string]int{}
	for _, z := range d.Zones {
		for _, g := range z.Segments {
			seen[g.Component]++
		}
	}
	for i, p := range parts {
		if seen[ComponentName(p.ID)] != 1 {
			t.Errorf("part %s derived %d times", p.ID, seen[ComponentName(p.ID)])
		}
		f, _ := net.Face(want[i])
		var g = findSeg(d, ComponentName(p.ID))
		x, y := (g.X-ox)/sx, (g.Y-oy)/sy
		w, h := g.W/sx, g.H/sy
		if x < f.X-1e-3 || y < f.Y-1e-3 || x+w > f.X+f.W+1e-3 || y+h > f.Y+f.H+1e-3 {
			t.Errorf("part %s box (%g, %g, %g, %g) outside face %+v", p.ID, x, y, w, h, f)
		}
	}
	// the explicit front face shows the glass-facing fan edge-on: a thin box
	// as deep as the part
	c := d.Components[4]
	if !near(c.SizeMM.X, 25) || !near(c.SizeMM.Y, 120) {
		t.Errorf("edge-on box %v, want 25 x 120", c.SizeMM)
	}
}

// A fan under the top panel right at the glass edge and a fan on the glass
// right below it sit next to each other on the canvas: the effect runs on.
func TestDeriveNetAcrossTheTopEdge(t *testing.T) {
	cs := box()
	top := Placement{ID: "tp", Part: "generic-fan-120", Facing: FacingTop, Pos: Vec3{60, 12.5, 200}, Bind: &Binding{Device: "d", Zone: "a"}}
	side := Placement{ID: "sd", Part: "generic-fan-120", Facing: FacingSide, Pos: Vec3{12.5, 60, 200}, Bind: &Binding{Device: "d", Zone: "b"}}
	d, err := Derive(Sketch{Version: Version, Case: cs, Parts: []Placement{top, side}}, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	gt, gs := findSeg(d, "sketch.tp"), findSeg(d, "sketch.sd")
	if !near(gt.Y+gt.H, gs.Y) || !near(gt.X, gs.X) || !near(gt.W, gs.W) {
		t.Errorf("top %+v and glass %+v do not meet at the edge", gt, gs)
	}
}

// Older sketches name a view as canvas mapping; every value means the net.
func TestDeriveIgnoresLegacyCanvasView(t *testing.T) {
	base := Sketch{Version: Version, Case: box(), Parts: []Placement{
		{ID: "tp", Part: "generic-fan-120", Facing: FacingTop, Pos: Vec3{60, 12.5, 200}, Bind: &Binding{Device: "d", Zone: "a"}},
	}}
	want, err := Derive(base, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	for _, cv := range []string{"", CanvasNet, ViewFront, ViewRear, ViewSide, ViewTop, ViewBottom} {
		s := base
		s.CanvasView = cv
		got, err := Derive(s, Builtin())
		if err != nil {
			t.Fatalf("%q: %v", cv, err)
		}
		if g, w := findSeg(got, "sketch.tp"), findSeg(want, "sketch.tp"); g.X != w.X || g.Y != w.Y || g.W != w.W || g.H != w.H {
			t.Errorf("%q: %+v vs %+v", cv, g, w)
		}
	}
	s := base
	s.Parts = []Placement{base.Parts[0]}
	s.Parts[0].Face = "inside"
	if err := Validate(s, Builtin()); err == nil {
		t.Error("invalid face accepted")
	}
}

func findSeg(d Derived, comp string) (g struct{ X, Y, W, H float64 }) {
	for _, z := range d.Zones {
		for _, s := range z.Segments {
			if s.Component == comp {
				g.X, g.Y, g.W, g.H = s.X, s.Y, s.W, s.H
			}
		}
	}
	return g
}
