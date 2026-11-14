package sketch

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// viewOf maps a version 1 view name to the view it becomes in the 3D model.
// The version 1 top view had the front at the top of the drawing; the model's
// Oben view has the front at the bottom, so top-view results are compared
// with the vertical axis flipped: v_new = D - v_old.

func flipY(dims Dims, view string, y float64) float64 {
	if view == ViewTop {
		return dims.D - y
	}
	return y
}

// The property test: for every catalog part, every version 1 view/face pair,
// rotation, mirror and flow, the old projection into the view the part was
// drawn in equals the projection of the upgraded part into the same view of
// the 3D model (top view: vertically flipped, see flipY).
func TestUpgradePreservesTheDrawnView(t *testing.T) {
	cat := Builtin()
	cs := box()
	pairs := [][2]string{
		{"front", "front"}, {"front", "side"}, {"front", "top"},
		{"side", "side"}, {"side", "front"}, {"side", "top"},
		{"top", "top"}, {"top", "front"}, {"top", "side"},
	}
	rots := []float64{0, 90, 180, 270, -90, -180, 360, -360, 37, -45, 123.5, 300}
	flows := []string{"", "intake", "exhaust"}
	n := 0
	for _, id := range cat.PartIDs() {
		for _, pair := range pairs {
			for _, rot := range rots {
				for _, mirror := range []bool{false, true} {
					for _, flow := range flows {
						for _, leds := range []int{0, 7} {
							part := cat.Parts[id]
							if _, err := Resolve(part, leds, 0); err != nil {
								continue // fixed LED count part
							}
							old := PlacementV1{ID: "a", Part: id, View: pair[0], Face: pair[1], X: 40.5, Y: 60.25, Rot: rot,
								Mirror: mirror, Flow: flow, LEDs: leds}
							want, err := projectV1(part, old)
							if err != nil {
								t.Fatal(err)
							}
							up, notes, err := Upgrade(SketchV1{Version: 1, Case: cs, Parts: []PlacementV1{old}}, cat)
							if err != nil || len(notes) != 0 {
								t.Fatalf("%+v: %v %v", old, err, notes)
							}
							if err := Validate(up, cat); err != nil {
								t.Fatalf("%+v: upgraded sketch invalid: %v", old, err)
							}
							got, err := ProjectView(part, up.Parts[0], up.Case, pair[0])
							if err != nil {
								t.Fatal(err)
							}
							n++
							label := fmt.Sprintf("%s %s/%s rot %g mirror %v flow %q leds %d -> %s spin %g mirror %v",
								id, pair[0], pair[1], rot, mirror, flow, leds, up.Parts[0].Facing, up.Parts[0].Spin, up.Parts[0].Mirror)
							if got.EdgeOn != want.EdgeOn || got.Away {
								t.Fatalf("%s: edge-on %v away %v, want edge-on %v", label, got.EdgeOn, got.Away, want.EdgeOn)
							}
							wantBoxY := want.Box.Y
							if pair[0] == ViewTop {
								wantBoxY = cs.Dims.D - want.Box.Y - want.Box.H
							}
							if !close6(got.Box.X, want.Box.X) || !close6(got.Box.Y, wantBoxY) || !close6(got.Box.W, want.Box.W) || !close6(got.Box.H, want.Box.H) {
								t.Fatalf("%s: box %+v, want %+v (y %g)", label, got.Box, want.Box, wantBoxY)
							}
							if len(got.LEDs) != len(want.LEDs) {
								t.Fatalf("%s: %d LEDs, want %d", label, len(got.LEDs), len(want.LEDs))
							}
							for i := range got.LEDs {
								wy := flipY(cs.Dims, pair[0], want.LEDs[i].Y)
								if !close6(got.LEDs[i].X, want.LEDs[i].X) || !close6(got.LEDs[i].Y, wy) {
									t.Fatalf("%s: LED %d at %v, want (%g, %g)", label, i, got.LEDs[i], want.LEDs[i].X, wy)
								}
							}
						}
					}
				}
			}
		}
	}
	if n < 5000 {
		t.Fatalf("only %d combinations checked", n)
	}
}

// close6 allows the rounding noise of two 6-digit roundings.
func close6(a, b float64) bool { return math.Abs(a-b) <= 2.5e-6 }

// A mirrored case keeps positions: the upgraded parts sit where the old
// drawing had them in the view the template is drawn in (glass on the right:
// Vorne/Oben horizontal axis mirrored in ToView and x mirrored in the model;
// side view starting at the front: z = D - u).
func TestUpgradeKeepsPositionsOnMirroredTemplates(t *testing.T) {
	cat := Builtin()
	for _, o := range []Orientation{{GlassSide: "right"}, {SideViewLeft: "front"}, {SideViewLeft: "front", GlassSide: "right"}} {
		cs := box()
		cs.Orientation = o
		for _, view := range []string{"front", "side", "top"} {
			old := PlacementV1{ID: "a", Part: "generic-fan-120", View: view, X: 40, Y: 70}
			up, _, err := Upgrade(SketchV1{Version: 1, Case: cs, Parts: []PlacementV1{old}}, cat)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ProjectView(cat.Parts["generic-fan-120"], up.Parts[0], up.Case, view)
			if err != nil {
				t.Fatal(err)
			}
			wantU, wantV := 40.0, 70.0
			if view == ViewSide && o.FrontOnSideLeft() {
				wantU = cs.Dims.D - 40 - 120
			}
			if view == ViewTop {
				wantV = cs.Dims.D - 70 - 120
			}
			if !close6(got.Box.X, wantU) || !close6(got.Box.Y, wantV) {
				t.Errorf("%+v %s: box %+v, want (%g, %g)", o, view, got.Box, wantU, wantV)
			}
		}
	}
}

func TestUpgradeFacingsAndDepth(t *testing.T) {
	cat := Builtin()
	cs := box()
	up, notes, err := Upgrade(SketchV1{Version: 1, Case: cs, CanvasView: "top", Parts: []PlacementV1{
		{ID: "fr", Part: "generic-fan-120", View: "front", X: 10, Y: 20},
		{ID: "sd", Part: "generic-fan-120", View: "side", X: 100, Y: 20},
		{ID: "tp", Part: "generic-fan-120", View: "top", X: 10, Y: 30},
		{ID: "sf", Part: "generic-fan-120", View: "side", Face: "front", X: 0, Y: 20},
	}}, cat)
	if err != nil || len(notes) != 0 {
		t.Fatal(err, notes)
	}
	if up.Version != Version || up.CanvasView != "top" {
		t.Fatalf("header %+v", up)
	}
	by := map[string]Placement{}
	for _, p := range up.Parts {
		by[p.ID] = p
	}
	// front view part: against the front panel, centre of the drawn box
	if p := by["fr"]; p.Facing != FacingFront || p.Pos != (Vec3{70, 80, 450 - 12.5}) {
		t.Errorf("front part %+v", p)
	}
	// side view part: against the glass, z from the drawn u (from the rear)
	if p := by["sd"]; p.Facing != FacingSide || p.Pos != (Vec3{12.5, 80, 160}) {
		t.Errorf("side part %+v", p)
	}
	// top view part: under the top panel, z = D - drawn v, spin 180 - rot, mirror toggled
	if p := by["tp"]; p.Facing != FacingTop || p.Pos != (Vec3{70, 12.5, 450 - 90}) || p.Spin != 180 || !p.Mirror {
		t.Errorf("top part %+v", p)
	}
	// a side view part whose LED side pointed left (drawn) faces the rear
	if p := by["sf"]; p.Facing != FacingRear || p.Pos.X != 60 {
		// edge-on towards the left: centre 12.5 from the left edge, x spans the 120 mm part
		t.Errorf("edge-on side part %+v", p)
	}
}

func TestUpgradeMountsPartsOnTheMainboard(t *testing.T) {
	cat := Builtin()
	board, _ := cat.Parts["generic-gpu-box"], 0
	board.ID, board.Category = "test-board", "mainboard"
	board.Dimensions = PartDims{W: 300, H: 250, D: 30}
	board.LEDs = PartLEDs{Positions: [][2]float64{{10, 10}}}
	if err := cat.AddPart(board); err != nil {
		t.Fatal(err)
	}
	cs := box()
	old := SketchV1{Version: 1, Case: cs, Parts: []PlacementV1{
		{ID: "mb", Part: "test-board", View: "side", X: 50, Y: 100},
		{ID: "ram", Part: "generic-ram-dimm", View: "side", X: 100, Y: 120},   // centre over the board
		{ID: "pump", Part: "generic-pump-cyl", View: "side", X: 380, Y: 120},  // outside the board
		{ID: "fan", Part: "generic-fan-120", View: "side", X: 100, Y: 140},    // not a mounted category
		{ID: "ram2", Part: "generic-ram-dimm", View: "side", X: 100, Y: 400},  // below the board
		{ID: "ramf", Part: "generic-ram-dimm", View: "front", X: 100, Y: 120}, // other view: no board
		{ID: "ramd", Part: "generic-ram-dimm", View: "side", Face: "top", X: 100, Y: 200},
	}}
	up, notes, err := Upgrade(old, cat)
	if err != nil || len(notes) != 0 {
		t.Fatal(err, notes)
	}
	if err := Validate(up, cat); err != nil {
		t.Fatal(err)
	}
	by := map[string]Placement{}
	for _, p := range up.Parts {
		by[p.ID] = p
	}
	if p := by["mb"]; p.On != "" || !near(p.Pos.X, 230-15) {
		t.Errorf("board %+v", p)
	}
	if p := by["ram"]; p.On != "mb" || !near(p.Pos.X, 196) {
		// the board's glass side is at x = 230 - 30 = 200; the 8 mm deep DIMM sits in front of it
		t.Errorf("ram %+v", p)
	}
	for _, id := range []string{"pump", "fan", "ram2", "ramf"} {
		if by[id].On != "" {
			t.Errorf("%s mounted: %+v", id, by[id])
		}
	}
	if by["pump"].Pos.X > 30 || by["fan"].Pos.X != 12.5 {
		t.Errorf("parts not against the glass: %+v %+v", by["pump"], by["fan"])
	}
	if by["ramd"].On != "mb" {
		t.Errorf("edge-on ram over the board: %+v", by["ramd"])
	}
	// the mounted part sits in front of the board's glass side without overlapping it
	if ov := Overlaps(up, cat); len(ov) != 0 {
		t.Errorf("overlaps after upgrade: %v", ov)
	}
}

func TestUpgradeNotes(t *testing.T) {
	cat := Builtin()
	cs := box()
	up, notes, err := Upgrade(SketchV1{Version: 1, Case: cs, Parts: []PlacementV1{
		{ID: "gone", Part: "library-fan", View: "front", X: 10, Y: 20, Rot: 90, Mirror: true},
		{ID: "bad", Part: "generic-fan-120", View: "side", Face: "nonsense", X: 10, Y: 20},
		{ID: "far", Part: "generic-fan-120", View: "front", X: 10, Y: 800},
		{ID: "leds", Part: "generic-strip", View: "front", LEDs: 999},
	}}, cat)
	if err != nil {
		t.Fatal(err)
	}
	msgs := map[string]string{}
	for _, n := range notes {
		msgs[n.ID] += n.Msg + ";"
	}
	for id, want := range map[string]string{"gone": "not in the catalog", "bad": "invalid view", "leds": "cannot be projected"} {
		if !strings.Contains(msgs[id], want) {
			t.Errorf("note for %s = %q, want %q", id, msgs[id], want)
		}
	}
	if len(up.Parts) != 4 || up.Parts[0].Pos.X != 10 || up.Parts[0].Spin != 90 || !up.Parts[0].Mirror {
		t.Errorf("unknown part: %+v", up.Parts[0])
	}
	if up.Parts[1].Facing != FacingSide {
		t.Errorf("fixed face: %+v", up.Parts[1])
	}
	if Validate(Sketch{Version: Version, Case: cs, Parts: up.Parts[2:3]}, cat) == nil {
		t.Errorf("a part far outside the case validates: %+v", up.Parts[2])
	}
	if _, _, err := Upgrade(SketchV1{Version: 2}, cat); err == nil {
		t.Error("upgraded a version 2 sketch")
	}
}

func TestUpgradeTakesTheTemplateOrientation(t *testing.T) {
	cat := Builtin()
	tpl := CaseTemplate{ID: "mirrored", Name: "M", Dims: box().Dims, Orientation: Orientation{GlassSide: "right"}}
	if err := cat.AddCase(tpl); err != nil {
		t.Fatal(err)
	}
	cs := Case{Source: "library", ID: "mirrored", Dims: box().Dims}
	up, _, err := Upgrade(SketchV1{Version: 1, Case: cs}, cat)
	if err != nil || !up.Case.Orientation.GlassRight() {
		t.Fatalf("%v %+v", err, up.Case)
	}
	cs.ID = "unknown-case"
	if up, _, _ := Upgrade(SketchV1{Version: 1, Case: cs}, cat); up.Case.Orientation != (Orientation{}) {
		t.Errorf("orientation from nowhere: %+v", up.Case.Orientation)
	}
}

// compareJSON walks two decoded JSON values; numbers may differ by tol.
func compareJSON(t *testing.T, path string, a, b any, tol float64) {
	t.Helper()
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			t.Errorf("%s: shape differs", path)
			return
		}
		for k, x := range av {
			compareJSON(t, path+"."+k, x, bv[k], tol)
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			t.Errorf("%s: length differs", path)
			return
		}
		for i := range av {
			compareJSON(t, fmt.Sprintf("%s[%d]", path, i), av[i], bv[i], tol)
		}
	case float64:
		bv, ok := b.(float64)
		if !ok || math.Abs(av-bv) > tol {
			t.Errorf("%s: %v vs %v", path, a, b)
		}
	default:
		if a != b {
			t.Errorf("%s: %v vs %v", path, a, b)
		}
	}
}

// Side-only sketches give the same Derive output as the version 1 code, with
// the side view now being the glass face of the case net: the fixtures in
// testdata were generated by that code before it was replaced
// (v1_*.sketch.json in, v1_*.derived.json out). Components are unchanged; the
// segment boxes are the version 1 boxes moved onto the net's glass face
// (every part put on that face, as the old side canvas showed them all).
func TestDeriveOfUpgradedSideSketchesMatchesVersion1(t *testing.T) {
	files, err := filepath.Glob("testdata/v1_*.sketch.json")
	if err != nil || len(files) < 2 {
		t.Fatalf("fixtures: %v %v", files, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		wantRaw, err := os.ReadFile(strings.Replace(f, ".sketch.json", ".derived.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		var want any
		if err := json.Unmarshal(wantRaw, &want); err != nil {
			t.Fatal(err)
		}
		// as an exported file, and as a stored config (Sketch.UnmarshalJSON)
		decoded, notes, err := DecodeWithCatalog(raw, Builtin())
		if err != nil || len(notes) != 0 {
			t.Fatalf("%s: %v %v", f, err, notes)
		}
		var holder struct {
			Sketch *Sketch `json:"sketch"`
		}
		if err := json.Unmarshal([]byte(`{"sketch":`+string(raw)+`}`), &holder); err != nil || holder.Sketch == nil {
			t.Fatalf("%s: stored config: %v", f, err)
		}
		for name, s := range map[string]Sketch{"exported": decoded, "stored": *holder.Sketch} {
			s.Parts = append([]Placement(nil), s.Parts...)
			for i := range s.Parts {
				s.Parts[i].Face = FacingSide // the old side canvas showed every part
			}
			d, err := Derive(s, Builtin())
			if err != nil {
				t.Fatalf("%s %s: %v", f, name, err)
			}
			got, _ := json.Marshal(sideToV1(t, s, d))
			var gotAny any
			if err := json.Unmarshal(got, &gotAny); err != nil {
				t.Fatal(err)
			}
			compareJSON(t, filepath.Base(f)+" "+name, want, gotAny, 1e-5)
		}
	}
}

func TestDecodeVersion1(t *testing.T) {
	v1 := `{"version":1,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450}},"canvasView":"front",
	"parts":[{"id":"a","part":"generic-fan-120","view":"top","x":10,"y":20,"rot":30,"face":"front","mirror":true,"flow":"exhaust",
	"leds":8,"label":"L","bind":{"device":"d","zone":"z","order":1,"parallel":false}}]}`
	s, err := Decode([]byte(v1))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != Version || s.CanvasView != "front" || len(s.Parts) != 1 || s.Parts[0].Facing != FacingFront || s.Parts[0].Bind.Order != 1 {
		t.Fatalf("%+v", s)
	}
	if err := Validate(s, Builtin()); err != nil {
		t.Fatal(err)
	}
	// strict like version 2
	for _, bad := range []string{strings.Replace(v1, `"view":"top"`, `"view":"top","extra":1`, 1), v1 + " {}", strings.Replace(v1, `"version":1`, `"version":3`, 1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("accepted %.60q", bad)
		}
	}
	// the stored-config reader ignores unknown fields but still upgrades
	var s2 Sketch
	if err := json.Unmarshal([]byte(strings.Replace(v1, `"view":"top"`, `"view":"top","extra":1`, 1)), &s2); err != nil || s2.Version != Version {
		t.Fatalf("stored: %v %+v", err, s2)
	}
	// a stored version 2 sketch is read as it is
	raw, _ := json.Marshal(s)
	var s3 Sketch
	if err := json.Unmarshal(raw, &s3); err != nil || s3.Parts[0].Pos != s.Parts[0].Pos {
		t.Fatalf("v2 stored: %v", err)
	}
}

func TestTooManyPartsAreRefusedBeforeTheUpgradeWorks(t *testing.T) {
	part := `{"id":"p%d","part":"generic-fan-120","view":"top","x":10,"y":20,"rot":0,"face":"front","mirror":false,"flow":"exhaust","leds":8,"label":"L","bind":{"device":"","zone":"","order":0,"parallel":false}}`
	doc := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf(part, i)
		}
		return `{"version":1,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450}},"canvasView":"front","parts":[` + strings.Join(parts, ",") + `]}`
	}
	if _, _, err := DecodeWithCatalog([]byte(doc(MaxParts)), Builtin()); err != nil {
		t.Fatalf("%d parts: %v", MaxParts, err)
	}
	_, notes, err := DecodeWithCatalog([]byte(doc(MaxParts+1)), Builtin())
	if err == nil || !strings.Contains(err.Error(), "more than") || len(notes) != 0 {
		t.Fatalf("%v %v", err, notes)
	}
	var old SketchV1
	if err := json.Unmarshal([]byte(doc(MaxParts+1)), &old); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Upgrade(old, Builtin()); err == nil {
		t.Fatal("Upgrade did the work")
	}
}

// sideToV1 maps the segment boxes of a net Derive back to the version 1 side
// canvas: every part must be on the glass face, which is the old side view.
func sideToV1(t *testing.T, s Sketch, d Derived) Derived {
	t.Helper()
	faces, net := SketchNet(s, Builtin())
	for i, f := range faces {
		if f != FacingSide {
			t.Fatalf("part %d on face %q, want side", i, f)
		}
	}
	side, _ := net.Face(FacingSide)
	nox, noy, nsx, nsy := Letterbox(net.W, net.H)
	vox, voy, vsx, vsy := Letterbox(s.Case.Dims.D, s.Case.Dims.H)
	for zi := range d.Zones {
		for si := range d.Zones[zi].Segments {
			g := &d.Zones[zi].Segments[si]
			x, y := (g.X-nox)/nsx-side.X, (g.Y-noy)/nsy-side.Y // mm in the side view
			w, h := g.W/nsx, g.H/nsy
			g.X, g.Y = clamp(round6(vox+x*vsx), 0, 1), clamp(round6(voy+y*vsy), 0, 1)
			g.W, g.H = clamp(round6(w*vsx), 0, 1), clamp(round6(h*vsy), 0, 1)
		}
	}
	return d
}
