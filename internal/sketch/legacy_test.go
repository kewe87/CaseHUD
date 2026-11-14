package sketch

import (
	"math"
	"testing"
)

// Tests of the version 1 projection, which defines what an old sketch looked
// like (see upgrade_test.go for the check against the 3D model).

func mustProject(t *testing.T, id string, pl PlacementV1) Projection {
	t.Helper()
	pr, err := projectV1(Builtin().Parts[id], pl)
	if err != nil {
		t.Fatal(err)
	}
	return pr
}

func TestProjectExhaustMirror(t *testing.T) {
	in := mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "side", Flow: "intake", LEDs: 8, X: 10, Y: 20})
	ex := mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "side", Flow: "exhaust", LEDs: 8, X: 10, Y: 20})
	if !near(in.Box.W, 120) || !near(in.Box.H, 120) || in.EdgeOn {
		t.Fatalf("box %+v", in.Box)
	}
	// LED 0 at the top centre in both, LED 2 right of centre (intake) vs left (exhaust)
	if !near(in.LEDs[0].X, 70) || !near(ex.LEDs[0].X, 70) || !near(in.LEDs[0].Y, 36) {
		t.Errorf("LED 0: %v %v", in.LEDs[0], ex.LEDs[0])
	}
	if !near(in.LEDs[2].X, 114) || !near(ex.LEDs[2].X, 26) || !near(in.LEDs[2].Y, ex.LEDs[2].Y) {
		t.Errorf("LED 2: %v %v", in.LEDs[2], ex.LEDs[2])
	}
	for i := range in.LEDs {
		if !near(in.LEDs[i].X+ex.LEDs[i].X, 2*70) { // mirror image about the fan centre line x = 70
			t.Errorf("LED %d not mirrored: %v %v", i, in.LEDs[i], ex.LEDs[i])
		}
	}
	// Mirror and exhaust cancel; strips ignore flow
	both := mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "side", Flow: "exhaust", Mirror: true, LEDs: 8, X: 10, Y: 20})
	for i := range in.LEDs {
		if !near(both.LEDs[i].X, in.LEDs[i].X) {
			t.Fatal("mirror + exhaust should cancel")
		}
	}
	s1 := mustProject(t, "generic-strip", PlacementV1{ID: "a", View: "side", Flow: "exhaust"})
	s2 := mustProject(t, "generic-strip", PlacementV1{ID: "a", View: "side"})
	if s1.LEDs[1] != s2.LEDs[1] {
		t.Error("flow changed a strip")
	}
	// mirror of an asymmetric part: strip first LED moves to the right end
	m := mustProject(t, "generic-strip", PlacementV1{ID: "a", View: "side", Mirror: true})
	if !near(m.LEDs[0].X, 297) {
		t.Errorf("mirrored strip first LED %v", m.LEDs[0])
	}
}

func TestProjectEdgeOnLine(t *testing.T) {
	// a fan on the top panel seen from the side: a line along the top edge
	pr := mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "side", Face: "top", LEDs: 16, X: 100, Y: 5})
	if !pr.EdgeOn || !near(pr.Box.W, 120) || !near(pr.Box.H, 25) {
		t.Fatalf("box %+v edge-on %v", pr.Box, pr.EdgeOn)
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, p := range pr.LEDs {
		if !near(p.Y, 5) { // pointing up: LED line on the top edge
			t.Fatalf("LED not on the line: %v", p)
		}
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
	}
	if minX < 100 || maxX > 220 || maxX-minX < 80 {
		t.Errorf("x range %v..%v", minX, maxX)
	}
	// front face seen from the side points left: vertical line on the left edge
	pr = mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "side", Face: "front", LEDs: 16, X: 0, Y: 100})
	if !near(pr.Box.W, 25) || !near(pr.Box.H, 120) {
		t.Fatalf("box %+v", pr.Box)
	}
	for _, p := range pr.LEDs {
		if !near(p.X, 0) || p.Y < 100 || p.Y > 220 {
			t.Fatalf("LED %v", p)
		}
	}
	// top view: face front points to the top; face side to the right (x = depth edge)
	pr = mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "top", Face: "side", X: 10, Y: 10})
	for _, p := range pr.LEDs {
		if !near(p.X, 35) {
			t.Fatalf("right edge expected, LED %v", p)
		}
	}
	// downward-looking top face in the front view is the same as in the side view
	pr = mustProject(t, "generic-fan-120", PlacementV1{ID: "a", View: "front", Face: "top", X: 0, Y: 0})
	if !pr.EdgeOn || !near(pr.Box.H, 25) {
		t.Errorf("%+v", pr.Box)
	}
}

func TestProjectRAMUpright(t *testing.T) {
	flat := mustProject(t, "generic-ram-dimm", PlacementV1{ID: "r", View: "side", X: 10, Y: 10})
	up := mustProject(t, "generic-ram-dimm", PlacementV1{ID: "r", View: "side", X: 10, Y: 10, Rot: 90})
	if !near(flat.Box.W, 133.35) || !near(flat.Box.H, 40) {
		t.Fatalf("flat box %+v", flat.Box)
	}
	if !near(up.Box.W, 40) || !near(up.Box.H, 133.35) || up.Box.X != 10 || up.Box.Y != 10 {
		t.Fatalf("upright box %+v", up.Box)
	}
	// rotating clockwise by 90: the left end (first LED) ends up at the top,
	// the light bar (top edge) moves to the right
	if up.LEDs[0].Y >= up.LEDs[4].Y {
		t.Errorf("first LED not on top: %v .. %v", up.LEDs[0], up.LEDs[4])
	}
	for i, p := range up.LEDs {
		if !near(p.X, 10+40-6) {
			t.Errorf("LED %d x = %v", i, p.X)
		}
	}
	// 5 LEDs on a 133.35 mm module spaced evenly
	d := up.LEDs[1].Y - up.LEDs[0].Y
	for i := 1; i < 5; i++ {
		if math.Abs(up.LEDs[i].Y-up.LEDs[i-1].Y-d) > 0.003 {
			t.Error("uneven spacing")
		}
	}
	// full turn equals no turn
	full := mustProject(t, "generic-ram-dimm", PlacementV1{ID: "r", View: "side", X: 10, Y: 10, Rot: 360})
	for i := range flat.LEDs {
		if !near(full.LEDs[i].X, flat.LEDs[i].X) || !near(full.LEDs[i].Y, flat.LEDs[i].Y) {
			t.Fatal("rot 360 differs")
		}
	}
}

func TestProjectGPUVerticalVsHorizontal(t *testing.T) {
	h := mustProject(t, "generic-gpu-box", PlacementV1{ID: "g", View: "side", X: 0, Y: 0})
	v := mustProject(t, "generic-gpu-box", PlacementV1{ID: "g", View: "side", X: 0, Y: 0, Rot: 90})
	if !near(h.Box.W, 300) || !near(h.Box.H, 120) || !near(v.Box.W, 120) || !near(v.Box.H, 300) {
		t.Fatalf("boxes %+v %+v", h.Box, v.Box)
	}
	for _, p := range h.LEDs {
		if !near(p.Y, 10) {
			t.Fatalf("horizontal bar not level: %v", p)
		}
	}
	for _, p := range v.LEDs {
		if !near(p.X, 110) { // top edge turns to the right side
			t.Fatalf("vertical bar not upright at the right: %v", p)
		}
	}
	if v.LEDs[0].Y >= v.LEDs[len(v.LEDs)-1].Y {
		t.Error("vertical card runs upwards")
	}
	// 270 puts the bar on the left, running upwards
	v3 := mustProject(t, "generic-gpu-box", PlacementV1{ID: "g", View: "side", Rot: 270})
	if !near(v3.LEDs[0].X, 10) || v3.LEDs[0].Y <= v3.LEDs[len(v3.LEDs)-1].Y {
		t.Errorf("270: %v", v3.LEDs[0])
	}
}

func TestProjectErrors(t *testing.T) {
	c := Builtin().Parts["generic-fan-120"]
	for _, pl := range []PlacementV1{
		{ID: "a", View: "bogus"}, {ID: "a", View: "side", Face: "bogus"},
		{ID: "a", X: math.NaN()}, {ID: "a", Rot: math.Inf(1)}, {ID: "a", LEDs: 3},
	} {
		if _, err := projectV1(c, pl); err == nil {
			t.Errorf("%+v accepted", pl)
		}
	}
}
