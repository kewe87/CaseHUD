package sketch

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/project3d_golden.json")

var allFacings = []string{FacingFront, FacingRear, FacingSide, FacingTray, FacingTop, FacingBottom}
var allViews = []string{ViewSide, ViewFront, ViewRear, ViewTop, ViewBottom}

func TestFramesAreOrthonormalAndRightHanded(t *testing.T) {
	dot := func(a, b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
	cross := func(a, b Vec3) Vec3 {
		return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
	}
	for _, f := range allFacings {
		fr, ok := FrameOf(f)
		if !ok {
			t.Fatal(f)
		}
		if dot(fr.U, fr.V) != 0 || dot(fr.U, fr.N) != 0 || dot(fr.V, fr.N) != 0 || dot(fr.N, fr.N) != 1 {
			t.Errorf("%s: not orthonormal", f)
		}
		// someone looking along -N with V down sees U to the right: (-N) x V = U
		if c := cross(Vec3{-fr.N.X, -fr.N.Y, -fr.N.Z}, fr.V); c != fr.U {
			t.Errorf("%s: (-N) x V = %v, want U %v", f, c, fr.U)
		}
	}
	if _, ok := FrameOf("up"); ok {
		t.Error("unknown facing has a frame")
	}
}

func TestToViewAxesAndDepth(t *testing.T) {
	cs := box() // 230 x 480 x 450
	p := Vec3{X: 10, Y: 20, Z: 30}
	for view, want := range map[string]ViewPoint{
		ViewSide:   {U: 30, V: 20, Depth: 10},
		ViewFront:  {U: 10, V: 20, Depth: 420},
		ViewRear:   {U: 220, V: 20, Depth: 30},
		ViewTop:    {U: 10, V: 30, Depth: 20},
		ViewBottom: {U: 10, V: 420, Depth: 460},
	} {
		got, ok := ToView(cs, view, p)
		if !ok || got != want {
			t.Errorf("%s: %+v, want %+v", view, got, want)
		}
		back, free, ok := FromView(cs, view, got.U, got.V)
		if !ok || !roundTrips(back, p, free) {
			t.Errorf("%s: FromView gave %v (free %q), want %v", view, back, free, p)
		}
	}
	if _, ok := ToView(cs, "diagonal", p); ok {
		t.Error("unknown view accepted")
	}
	// the glass on the right mirrors the horizontal axis of the x views, not the side view
	cs.Orientation = Orientation{GlassSide: "right"}
	for view, want := range map[string]float64{ViewSide: 30, ViewFront: 220, ViewRear: 10, ViewTop: 220, ViewBottom: 220} {
		if got, _ := ToView(cs, view, p); got.U != want {
			t.Errorf("glass right, %s: u = %g, want %g", view, got.U, want)
		}
		got, _ := ToView(cs, view, p)
		back, free, _ := FromView(cs, view, got.U, got.V)
		if !roundTrips(back, p, free) {
			t.Errorf("glass right, %s: FromView gave %v (free %q)", view, back, free)
		}
	}
	// view sizes
	for view, want := range map[string][2]float64{ViewSide: {450, 480}, ViewFront: {230, 480}, ViewRear: {230, 480}, ViewTop: {230, 450}, ViewBottom: {230, 450}} {
		if w, h, ok := ViewSize(box().Dims, view); !ok || w != want[0] || h != want[1] {
			t.Errorf("ViewSize %s = %g x %g", view, w, h)
		}
	}
}

// roundTrips compares a FromView result with the original point on the two
// coordinates the view determines.
func roundTrips(got, want Vec3, free string) bool {
	switch free {
	case "x":
		return got.Y == want.Y && got.Z == want.Z
	case "y":
		return got.X == want.X && got.Z == want.Z
	case "z":
		return got.X == want.X && got.Y == want.Y
	}
	return false
}

func TestProject3DDepthAndBox(t *testing.T) {
	fan := Builtin().Parts["generic-fan-120"]
	for _, f := range allFacings {
		pl := Placement{ID: "a", Facing: f, Pos: Vec3{100, 200, 300}}
		pr, err := Project3D(fan, pl)
		if err != nil {
			t.Fatal(err)
		}
		fr, _ := FrameOf(f)
		// the box is 120 x 120 in the plane of the facing and 25 deep along the normal
		size := Vec3{pr.Max.X - pr.Min.X, pr.Max.Y - pr.Min.Y, pr.Max.Z - pr.Min.Z}
		wantSize := Vec3{120, 120, 120}
		if fr.N.X != 0 {
			wantSize.X = 25
		}
		if fr.N.Y != 0 {
			wantSize.Y = 25
		}
		if fr.N.Z != 0 {
			wantSize.Z = 25
		}
		if size != wantSize {
			t.Errorf("%s: box size %v, want %v", f, size, wantSize)
		}
		for i, q := range pr.LEDs {
			// every LED lies on the LED side: 12.5 mm from the centre along N
			rel := Vec3{q.X - 100, q.Y - 200, q.Z - 300}
			if !near(rel.X*fr.N.X+rel.Y*fr.N.Y+rel.Z*fr.N.Z, 12.5) {
				t.Fatalf("%s: LED %d at %v is not on the LED side", f, i, q)
			}
		}
	}
	if _, err := Project3D(fan, Placement{ID: "a", Facing: "up"}); err == nil {
		t.Error("invalid facing accepted")
	}
	if _, err := Project3D(fan, Placement{ID: "a", Facing: FacingTop, Spin: math.NaN()}); err == nil {
		t.Error("NaN spin accepted")
	}
	if _, err := Project3D(fan, Placement{ID: "a", Facing: FacingTop, LEDs: 3}); err == nil {
		t.Error("bad LED count accepted")
	}
	if _, err := ProjectView(fan, Placement{ID: "a", Facing: FacingTop}, box(), "diagonal"); err == nil {
		t.Error("bad view accepted")
	}
}

// A part facing the viewer is flat, one facing away shows its back, the others
// are thin boxes with the LEDs on the edge the LED side points to.
func TestProjectViewEdgeOn(t *testing.T) {
	cs := box()
	strip := Builtin().Parts["generic-strip"] // 300 x 8 x d
	pl := Placement{ID: "s", Facing: FacingSide, Pos: Vec3{5, 100, 150}}
	flat, _ := ProjectView(strip, pl, cs, ViewSide)
	if flat.EdgeOn || flat.Away || !near(flat.Box.W, 300) {
		t.Errorf("side: %+v", flat)
	}
	top, _ := ProjectView(strip, pl, cs, ViewTop)
	if !top.EdgeOn || top.Away || !near(top.Box.W, 4) || !near(top.Box.H, 300) {
		t.Errorf("top: %+v", top)
	}
	for _, p := range top.LEDs {
		if !near(p.X, top.Box.X) { // the glass is x = 0: the LEDs sit on the near edge
			t.Fatalf("LED %v off the glass-side edge of %+v", p, top.Box)
		}
	}
}

func TestProjectViewAwayIsMirrored(t *testing.T) {
	cs := box()
	strip := Builtin().Parts["generic-strip"]
	// a strip facing the front, centred; Vorne sees its LED side, Hinten its back
	pl := Placement{ID: "s", Facing: FacingFront, Pos: Vec3{115, 100, 400}}
	front, _ := ProjectView(strip, pl, cs, ViewFront)
	rear, _ := ProjectView(strip, pl, cs, ViewRear)
	if front.Away || !rear.Away || front.EdgeOn || rear.EdgeOn {
		t.Fatalf("front %+v rear %+v", front, rear)
	}
	for i := range front.LEDs {
		if !near(front.LEDs[i].X+rear.LEDs[i].X, cs.Dims.W) || !near(front.LEDs[i].Y, rear.LEDs[i].Y) {
			t.Fatalf("LED %d: front %v rear %v are not mirror images about the case centre", i, front.LEDs[i], rear.LEDs[i])
		}
	}
}

// GPU card: upright against the glass (facing side, spin 90), lying on the
// floor (facing bottom is the LED side down; a card lying with its LED side
// up faces top). Distribution plate on the floor, rear exhaust fan.
func TestProjectPlacementsOfCommonParts(t *testing.T) {
	cs := box()
	cat := Builtin()
	gpu := cat.Parts["generic-gpu-box"] // 300 x 120 x d, LED bar along the top edge
	gd := gpu.Dimensions.D

	// vertical card against the glass: tall in the side view, bar at the right
	v := Placement{ID: "g", Facing: FacingSide, Spin: 90, Pos: Vec3{gd / 2, 240, 225}}
	pv, _ := ProjectView(gpu, v, cs, ViewSide)
	if pv.EdgeOn || !near(pv.Box.W, 120) || !near(pv.Box.H, 300) {
		t.Fatalf("vertical card: %+v", pv.Box)
	}
	for _, p := range pv.LEDs {
		if !near(p.X, pv.Box.X+pv.Box.W-10) {
			t.Fatalf("vertical bar not at the right: %v", p)
		}
	}
	if pv.LEDs[0].Y >= pv.LEDs[len(pv.LEDs)-1].Y {
		t.Error("vertical card runs upwards")
	}

	// horizontal card, LED side down: seen from below (Unten) flat, from the side edge-on
	h := Placement{ID: "g", Facing: FacingBottom, Pos: Vec3{115, 400, 225}}
	ph, _ := ProjectView(gpu, h, cs, ViewBottom)
	if ph.EdgeOn || ph.Away || !near(ph.Box.W, 300) || !near(ph.Box.H, 120) {
		t.Fatalf("horizontal card from below: %+v", ph)
	}
	pe, _ := ProjectView(gpu, h, cs, ViewSide)
	if !pe.EdgeOn || !near(pe.Box.W, 120) {
		t.Fatalf("horizontal card from the side: %+v", pe)
	}

	// distribution plate on the floor: LED side up, y = H - extent/2
	dp := cat.Parts["generic-distro-rect"]
	floor := Placement{ID: "d", Facing: FacingTop, Pos: Vec3{115, cs.Dims.H - dp.Dimensions.D/2, 200}}
	pd, _ := ProjectView(dp, floor, cs, ViewTop)
	if pd.EdgeOn || !near(pd.Box.W, dp.Dimensions.W) || !near(pd.Box.H, dp.Dimensions.H) {
		t.Fatalf("floor plate from above: %+v", pd)
	}
	pds, _ := ProjectView(dp, floor, cs, ViewSide)
	if !pds.EdgeOn || !near(pds.Box.Y+pds.Box.H, cs.Dims.H) {
		t.Fatalf("floor plate from the side should end at the floor: %+v", pds)
	}

	// rear exhaust fan: against the rear panel (z = ext/2), LED side towards the front
	fan := cat.Parts["generic-fan-120"]
	rear := Placement{ID: "f", Facing: FacingFront, Flow: "exhaust", Pos: Vec3{115, 100, fan.Dimensions.D / 2}}
	pr, _ := ProjectView(fan, rear, cs, ViewFront)
	in, _ := ProjectView(fan, Placement{ID: "f", Facing: FacingFront, Pos: rear.Pos}, cs, ViewFront)
	if pr.Away || pr.EdgeOn {
		t.Fatalf("rear fan from the front: %+v", pr)
	}
	// exhaust runs the LED order the other way round: mirror image about the fan centre line
	for i := range pr.LEDs {
		if !near(pr.LEDs[i].X+in.LEDs[i].X, 2*115) {
			t.Fatalf("LED %d not mirrored by exhaust: %v %v", i, pr.LEDs[i], in.LEDs[i])
		}
	}
}

// The golden file records the projection of a small asymmetric plate and a
// fan for every facing and view, so the JavaScript port can be checked
// against it. Run `go test ./internal/sketch -run Golden -update` after an
// intended change of the maths.
type goldenSample struct {
	Part        string      `json:"part"`
	Facing      string      `json:"facing"`
	View        string      `json:"view"`
	Pos         Vec3        `json:"pos"`
	Spin        float64     `json:"spin"`
	Mirror      bool        `json:"mirror"`
	Flow        string      `json:"flow,omitempty"`
	Orientation Orientation `json:"orientation,omitzero"`
	LEDs3D      []Vec3      `json:"leds3d"`
	Min         Vec3        `json:"min"`
	Max         Vec3        `json:"max"`
	LEDs        []Point     `json:"leds"`
	Box         Rect        `json:"box"`
	Kind        string      `json:"kind"`  // flat, away or edge
	Depth       []float64   `json:"depth"` // depth of every LED in the view
}

func goldenParts() map[string]Part {
	plate := Part{Format: 1, ID: "golden-plate", Category: "gpu-block", Dimensions: PartDims{W: 60, H: 40, D: 10},
		LEDs: PartLEDs{Count: 4, Positions: [][2]float64{{5, 5}, {55, 5}, {55, 35}, {10, 30}}}}
	fan := Builtin().Parts["generic-fan-120"]
	r, err := Resolve(fan, 4, 0)
	if err != nil {
		panic(err)
	}
	r.ID = "golden-fan"
	r.LEDs.Generator = nil
	return map[string]Part{"golden-plate": plate, "golden-fan": r}
}

func goldenSamples() []goldenSample {
	parts := goldenParts()
	var out []goldenSample
	add := func(part, facing, view string, spin float64, mirror bool, flow string, o Orientation) {
		cs := box()
		cs.Orientation = o
		pl := Placement{ID: "a", Part: part, Facing: facing, Pos: Vec3{100, 200, 150}, Spin: spin, Mirror: mirror, Flow: flow}
		p3, err := Project3DIn(parts[part], pl, o)
		if err != nil {
			panic(err)
		}
		pv, err := ProjectView(parts[part], pl, cs, view)
		if err != nil {
			panic(err)
		}
		kind := "flat"
		if pv.EdgeOn {
			kind = "edge"
		} else if pv.Away {
			kind = "away"
		}
		depth := make([]float64, len(p3.LEDs))
		for i, q := range p3.LEDs {
			vp, _ := ToView(cs, view, q)
			depth[i] = round6(vp.Depth)
		}
		out = append(out, goldenSample{Part: part, Facing: facing, View: view, Pos: pl.Pos, Spin: spin, Mirror: mirror, Flow: flow,
			Orientation: o, LEDs3D: p3.LEDs, Min: p3.Min, Max: p3.Max, LEDs: pv.LEDs, Box: pv.Box, Kind: kind, Depth: depth})
	}
	for _, f := range allFacings {
		for _, v := range allViews {
			for _, spin := range []float64{0, 90, 37} {
				for _, mirror := range []bool{false, true} {
					add("golden-plate", f, v, spin, mirror, "", Orientation{})
				}
			}
		}
	}
	for _, f := range allFacings {
		for _, v := range allViews {
			add("golden-fan", f, v, 0, false, "exhaust", Orientation{})
			add("golden-plate", f, v, 90, false, "", Orientation{GlassSide: "right"})
			add("golden-plate", f, v, 37, true, "", Orientation{GlassSide: "right"})
			add("golden-fan", f, v, 0, false, "exhaust", Orientation{GlassSide: "right"})
			add("golden-plate", f, v, 90, true, "", Orientation{SideViewLeft: "front", GlassSide: "right"})
		}
	}
	return out
}

func TestProject3DGolden(t *testing.T) {
	const path = "testdata/project3d_golden.json"
	var buf bytes.Buffer
	buf.WriteString("{\n")
	hdr := map[string]any{
		"about": "Projection of golden-plate/golden-fan at pos (100,200,150) into the five views of a 230 x 480 x 450 case. " +
			"leds: wire order, mm from the top left of the view; depth: distance from the viewer; kind: flat, away or edge. " +
			"Generated by TestProject3DGolden; the JavaScript port must reproduce every sample to 1e-5.",
		"case":  box(),
		"parts": goldenParts(),
	}
	for _, k := range []string{"about", "case", "parts"} {
		b, _ := json.Marshal(hdr[k])
		fmt.Fprintf(&buf, " %q: %s,\n", k, b)
	}
	buf.WriteString(` "samples": [` + "\n")
	samples := goldenSamples()
	for i, s := range samples {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		sep := ","
		if i == len(samples)-1 {
			sep = ""
		}
		fmt.Fprintf(&buf, "  %s%s\n", b, sep)
	}
	buf.WriteString(" ]\n}\n")
	if *update {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Fatal("projection differs from testdata/project3d_golden.json; run with -update if the change is intended")
	}
	var parsed struct {
		Samples []goldenSample `json:"samples"`
	}
	if err := json.Unmarshal(want, &parsed); err != nil || len(parsed.Samples) != len(samples) || len(samples) < 200 {
		t.Fatalf("golden file unreadable: %v (%d samples)", err, len(parsed.Samples))
	}
}
