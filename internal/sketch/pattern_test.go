package sketch

import (
	"errors"
	"math"
	"testing"
)

func orientCode(err error) string {
	var oe *OrientError
	if errors.As(err, &oe) {
		return oe.Code
	}
	return ""
}

// partNet are the LEDs of part id on the net, as the engine samples them.
func partNet(t *testing.T, s Sketch, c Catalog, rings RingZones, id string) ([]Point, Point) {
	t.Helper()
	s = ResolveFaces(s, c)
	faces, n := SketchNet(s, c)
	i := findPart(s, id)
	pts, _, mid, err := partLEDs(s, c, rings, i, faces[i], n)
	if err != nil {
		t.Fatal(err)
	}
	return pts, mid
}

// fixed applies what the user reports to the LEDs as the preview shows
// them: where the part really has them.
func fixed(pts []Point, mid Point, fix string) []Point {
	out := make([]Point, len(pts))
	for k, q := range pts {
		dx, dy := q.X-mid.X, q.Y-mid.Y
		switch fix {
		case FixRot90: // clockwise, y down: (dx, dy) -> (-dy, dx)
			out[k] = Point{X: mid.X - dy, Y: mid.Y + dx}
		case FixRot180:
			out[k] = Point{X: mid.X - dx, Y: mid.Y - dy}
		case FixRot270:
			out[k] = Point{X: mid.X + dy, Y: mid.Y - dx}
		case FixMirror:
			out[k] = Point{X: mid.X - dx, Y: q.Y}
		case FixReverse:
			out[k] = pts[len(pts)-1-k]
		}
	}
	return out
}

func samePts(a, b []Point) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if math.Hypot(a[k].X-b[k].X, a[k].Y-b[k].Y) > 1e-3 {
			return false
		}
	}
	return true
}

func orientSketch(parts ...Placement) Sketch {
	return Sketch{Version: Version, Case: Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 290, H: 471, D: 478}}, Parts: parts}
}

func bound(p Placement, zone string, order int) Placement {
	p.Bind = &Binding{Device: "asusaura:0b05:1aa6", Zone: zone, Order: order}
	return p
}

// Every report of the user (turned, mirrored) puts every LED of a fan where
// the part really has it, whatever the face, spin, mirror and flow it
// starts from; turning back and mirroring twice give the start again.
func TestOrientFan(t *testing.T) {
	c := autoCatalog(t)
	starts := []Placement{
		{ID: "f", Part: "uni-fan-tl-lcd-reverse-120", Facing: FacingSide, Pos: Vec3{X: 14, Y: 112, Z: 300}},
		{ID: "f", Part: "uni-fan-tl-lcd-reverse-120", Facing: FacingSide, Pos: Vec3{X: 14, Y: 236, Z: 300}, Spin: 90, Mirror: true},
		{ID: "f", Part: "uni-fan-tl-reverse-120", Facing: FacingTop, Pos: Vec3{X: 145, Y: 14, Z: 237}, Flow: "exhaust", Spin: 270},
		{ID: "f", Part: "uni-fan-tl-lcd-120", Facing: FacingRear, Pos: Vec3{X: 60, Y: 90, Z: 18}, Spin: 180},
		{ID: "f", Part: "generic-fan-120", Facing: FacingFront, Pos: Vec3{X: 145, Y: 200, Z: 464}, Mirror: true},
	}
	back := map[string]string{FixRot90: FixRot270, FixRot180: FixRot180, FixRot270: FixRot90, FixMirror: FixMirror}
	for _, st := range starts {
		s := ResolveFaces(orientSketch(bound(st, "argb1", 0)), c)
		before, mid := partNet(t, s, c, nil, "f")
		for _, fix := range []string{FixRot90, FixRot180, FixRot270, FixMirror} {
			got, err := Orient(s, c, nil, "f", fix)
			if err != nil {
				t.Fatalf("%s %+v: %v", fix, st, err)
			}
			after, _ := partNet(t, got, c, nil, "f")
			if !samePts(after, fixed(before, mid, fix)) {
				t.Fatalf("%s %+v: LEDs at %v, want %v", fix, st, after[:3], fixed(before, mid, fix)[:3])
			}
			if p := got.Parts[0]; p.Spin < 0 || p.Spin >= 360 || p.Pos != st.Pos || p.Facing != st.Facing || p.Flow != st.Flow {
				t.Fatalf("%s: placement %+v", fix, p)
			}
			again, err := Orient(got, c, nil, "f", back[fix])
			if err != nil {
				t.Fatal(err)
			}
			if pts, _ := partNet(t, again, c, nil, "f"); !samePts(pts, before) {
				t.Fatalf("%s then %s: not back at the start", fix, back[fix])
			}
		}
	}
}

// Turned by 90° clockwise, LED 1 of a TL fan (bottom left in the preview)
// is really at the top left.
func TestOrientFanTurnedClockwise(t *testing.T) {
	c := autoCatalog(t)
	s := ResolveFaces(orientSketch(bound(Placement{ID: "f", Part: "uni-fan-tl-lcd-reverse-120", Facing: FacingSide, Pos: Vec3{X: 14, Y: 236, Z: 300}}, "argb1", 0)), c)
	pts, mid := partNet(t, s, c, nil, "f")
	if !(pts[0].X < mid.X && pts[0].Y > mid.Y) {
		t.Fatalf("LED 1 not bottom left: %v (centre %v)", pts[0], mid)
	}
	got, err := Orient(s, c, nil, "f", FixRot90)
	if err != nil {
		t.Fatal(err)
	}
	pts, _ = partNet(t, got, c, nil, "f")
	if !(pts[0].X < mid.X && pts[0].Y < mid.Y) {
		t.Fatalf("LED 1 not top left: %v", pts[0])
	}
}

func TestOrientStrip(t *testing.T) {
	c := autoCatalog(t)
	for _, st := range []Placement{
		{ID: "s", Part: "generic-strip", Facing: FacingSide, Pos: Vec3{X: 2, Y: 20, Z: 239}},
		{ID: "s", Part: "generic-strip", Facing: FacingSide, Pos: Vec3{X: 2, Y: 235, Z: 20}, Spin: 90},
		{ID: "s", Part: "generic-strip", Facing: FacingTop, Pos: Vec3{X: 145, Y: 2, Z: 239}, Mirror: true},
	} {
		s := ResolveFaces(orientSketch(bound(st, "argb2", 0)), c)
		before, mid := partNet(t, s, c, nil, "s")
		got, err := Orient(s, c, nil, "s", FixReverse)
		if err != nil {
			t.Fatalf("%+v: %v", st, err)
		}
		if after, _ := partNet(t, got, c, nil, "s"); !samePts(after, fixed(before, mid, FixReverse)) {
			t.Fatalf("%+v: not reversed", st)
		}
		for _, fix := range []string{FixRot90, FixMirror, "nonsense"} {
			if _, err := Orient(s, c, nil, "s", fix); orientCode(err) != "fix" {
				t.Fatalf("strip %s: %v", fix, err)
			}
		}
	}
	// a fan is not reversed
	s := ResolveFaces(orientSketch(bound(Placement{ID: "f", Part: "generic-fan-120", Facing: FacingSide, Pos: Vec3{X: 14, Y: 112, Z: 300}}, "argb1", 0)), c)
	if _, err := Orient(s, c, nil, "f", FixReverse); orientCode(err) != "fix" {
		t.Fatalf("fan reversed: %v", err)
	}
}

func TestOrientCaseLight(t *testing.T) {
	c := autoCatalog(t)
	pl := Placement{ID: "case-top", Part: "generic-strip", CaseLight: "top", LEDs: 10,
		Path: []Vec3{{X: 10, Y: 1, Z: 10}, {X: 280, Y: 1, Z: 10}}}
	s := ResolveFaces(orientSketch(bound(pl, "argb2", 0)), c)
	got, err := Orient(s, c, nil, "case-top", FixReverse)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Parts[0].Mirror || s.Parts[0].Mirror {
		t.Fatalf("case light not reversed (or the input changed)")
	}
	if _, err := Orient(s, c, nil, "case-top", FixRot90); orientCode(err) != "fix" {
		t.Fatalf("case light turned: %v", err)
	}
	p, err := Pattern(s, c, nil, "case-top")
	if err != nil || p.Kind != PatternStrip || len(p.Colors) != 10 || p.Colors[0] != "#00ff00" || p.Colors[9] != "#ff0000" {
		t.Fatalf("case light pattern %+v %v", p, err)
	}
}

func TestOrientRefused(t *testing.T) {
	c := autoCatalog(t)
	s := ResolveFaces(kevinSketch(false, true), c)
	r := AutoBind(s, c, kevinSyncHW("asusaura:0b05:1aa6/argb1"))
	s = ApplyAutoBind(s, r)
	if _, err := Orient(s, c, nil, "p13", FixRot90); orientCode(err) != "parallel" {
		t.Fatalf("parallel: %v", err)
	}
	if _, err := Pattern(s, c, nil, "p13"); err != nil {
		t.Fatal(err)
	}
	if _, err := Orient(s, c, nil, "p9", FixRot90); orientCode(err) != "unbound" {
		t.Fatalf("unbound: %v", err)
	}
	if _, err := Pattern(s, c, nil, "p9"); orientCode(err) != "unbound" {
		t.Fatalf("unbound pattern: %v", err)
	}
	if _, err := Orient(s, c, nil, "nope", FixRot90); orientCode(err) != "unknown" {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := Orient(s, c, nil, "p1", FixRot90); err != nil {
		t.Fatalf("the model of the chain: %v", err)
	}
}

// The pattern starts where Derive puts the part in its zone; a fan has four
// quadrants around its centre and a white first LED.
func TestPattern(t *testing.T) {
	c := autoCatalog(t)
	s := ResolveFaces(kevinSketch(false, true), c)
	s = ApplyAutoBind(s, AutoBind(s, c, kevinSyncHW("asusaura:0b05:1aa6/argb1")))
	d, err := Derive(s, c)
	if err != nil {
		t.Fatal(err)
	}
	starts := map[string]int{}
	for _, z := range d.Zones {
		for _, seg := range z.Segments {
			starts[seg.Component] = *seg.Start
		}
	}
	for _, id := range []string{"p1", "p3", "p13", "p4", "p5", "p8", "p6"} {
		p, err := Pattern(s, c, nil, id)
		if err != nil {
			t.Fatal(err)
		}
		if p.Start != starts[ComponentName(id)] || p.Device != "asusaura:0b05:1aa6" || p.Zone != "argb1" || p.Kind != PatternFan || len(p.Colors) != 26 {
			t.Fatalf("%s: %+v (start %d)", id, p, starts[ComponentName(id)])
		}
		if want := map[string]string{"p3": "p1", "p13": "p1", "p5": "p4", "p6": "p8"}[id]; p.Parallel != (p.Model != "") || p.Model != want {
			t.Fatalf("%s: model %q", id, p.Model)
		}
		pts, mid := partNet(t, s, c, nil, id)
		seen := map[string]bool{}
		for k, col := range p.Colors {
			want := QuadrantColors[Quadrant(mid, pts[k])]
			if k == 0 {
				want = FirstLEDColor
			}
			if col != want {
				t.Fatalf("%s LED %d: %s, want %s", id, k, col, want)
			}
			seen[col] = true
		}
		// a TL fan has two rows of LEDs: on opposite sides as the canvas shows it
		if !(seen[QuadrantColors[0]] && seen[QuadrantColors[2]]) && !(seen[QuadrantColors[1]] && seen[QuadrantColors[3]]) {
			t.Fatalf("%s: colours %v", id, p.Colors)
		}
	}
	if Quadrant(Point{}, Point{X: 0, Y: -1}) != 0 || Quadrant(Point{}, Point{X: 1}) != 1 || Quadrant(Point{}, Point{Y: 1}) != 2 || Quadrant(Point{}, Point{X: -1}) != 3 {
		t.Fatal("quadrants")
	}
	if g := StripColors(3); g[0] != "#00ff00" || g[1] != "#808000" || g[2] != "#ff0000" {
		t.Fatalf("strip colours %v", g)
	}
}

// On a zone with one LED per ring the pattern has one colour per ring: the
// top ring in the top colour, the bottom ring in the bottom colour.
func TestPatternRings(t *testing.T) {
	c := autoCatalog(t)
	rings := func(device, zone string) bool { return zone == "port3" }
	pl := Placement{ID: "f", Part: "uni-fan-tl-lcd-reverse-120", Facing: FacingSide, Pos: Vec3{X: 14, Y: 112, Z: 300}}
	pl.Bind = &Binding{Device: "lianlitl:0416:7372", Zone: "port3"}
	s := ResolveFaces(orientSketch(pl), c)
	p, err := Pattern(s, c, rings, "f")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != PatternRings || len(p.Colors) != 2 || p.Colors[0] != QuadrantColors[0] || p.Colors[1] != QuadrantColors[2] {
		t.Fatalf("rings: %+v", p)
	}
	// turned upside down: the rings swap places
	got, err := Orient(s, c, rings, "f", FixRot180)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := Pattern(got, c, rings, "f"); p.Colors[0] != QuadrantColors[2] {
		t.Fatalf("turned: %+v", p)
	}
}
