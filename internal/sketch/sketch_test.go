package sketch

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func mustProject(t *testing.T, id string, pl Placement) Projection {
	t.Helper()
	pr, err := Project(Builtin().Parts[id], pl)
	if err != nil {
		t.Fatal(err)
	}
	return pr
}

func TestBuiltinCatalog(t *testing.T) {
	c := Builtin()
	want := []string{"generic-distro-rect", "generic-fan-120", "generic-fan-140", "generic-gpu-box",
		"generic-pump-cyl", "generic-ram-dimm", "generic-strip"}
	got := c.PartIDs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("parts = %v", got)
	}
	if _, ok := c.Cases[CustomCaseID]; !ok {
		t.Fatal("custom box case missing")
	}
	if !near(c.Parts["generic-ram-dimm"].Dimensions.W, 133.35) {
		t.Error("DIMM width is not the JEDEC 133.35 mm")
	}
	for id, p := range c.Parts {
		if g := p.LEDs.Generator; g != nil {
			// the stored default layout equals the generated one
			r, err := Resolve(p, 0, 0)
			if err != nil {
				t.Fatalf("%s: %v", id, err)
			}
			if len(r.LEDs.Positions) != len(p.LEDs.Positions) || len(p.LEDs.Positions) != g.Default {
				t.Fatalf("%s: default count mismatch", id)
			}
			for i := range p.LEDs.Positions {
				if !near(r.LEDs.Positions[i][0], p.LEDs.Positions[i][0]) || !near(r.LEDs.Positions[i][1], p.LEDs.Positions[i][1]) {
					t.Fatalf("%s: LED %d differs from the generated layout", id, i)
				}
			}
		}
		if len(p.Shapes("front")) == 0 || len(p.Shapes("side")) == 0 || len(p.Shapes("top")) == 0 {
			t.Errorf("%s: missing shapes", id)
		}
	}
	// independent copies
	delete(c.Parts, "generic-strip")
	if _, ok := Builtin().Parts["generic-strip"]; !ok {
		t.Error("Builtin shares its maps")
	}
}

func TestAddPart(t *testing.T) {
	c := Builtin()
	p := c.Parts["generic-strip"]
	if err := c.AddPart(p); err == nil {
		t.Error("duplicate id accepted")
	}
	p.ID = "sketch.x"
	if err := c.AddPart(p); err == nil {
		t.Error("reserved prefix accepted")
	}
	p.ID = "vendor-part"
	if err := c.AddPart(p); err != nil {
		t.Errorf("valid part refused: %v", err)
	}
	bad := Part{Format: 1, ID: "bad", Dimensions: PartDims{W: 10, H: 10, D: 5}, LEDs: PartLEDs{Positions: [][2]float64{{11, 1}}}}
	if err := c.AddPart(bad); err == nil {
		t.Error("LED outside the part accepted")
	}
}

func TestParsePartLibraryFormat(t *testing.T) {
	const js = `{"format":1,"id":"acme-fan","vendor":"Acme","model":"F","category":"fan",
	 "dimensions_mm":{"w":120,"h":124,"d":28},
	 "outline":{"front":[{"type":"rect","x":0,"y":0,"w":120,"h":124,"r":6,"role":"body"}],
	            "side":[{"type":"rect","x":0,"y":0,"w":124,"h":28}]},
	 "leds":{"count":2,"positions_mm":[[10,10],[110,10]],"zones":[{"id":"a","first":0,"count":2}]},
	 "connection":{"type":"argb-header"},"verified_by":["community"],"sources":[]}`
	p, err := ParsePart([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LEDs.Positions) != 2 || p.Outline.Side[0].H != 28 {
		t.Errorf("parsed %+v", p)
	}
	if _, err := ParsePart([]byte(strings.Replace(js, `"count":2`, `"count":3`, 1))); err == nil {
		t.Error("count mismatch accepted")
	}
}

func TestResolve(t *testing.T) {
	c := Builtin()
	fan := c.Parts["generic-fan-120"]
	r, err := Resolve(fan, 8, 0)
	if err != nil || len(r.LEDs.Positions) != 8 {
		t.Fatalf("fan 8: %v %d", err, len(r.LEDs.Positions))
	}
	// first LED at the top centre, clockwise: LED 2 of 8 is right of the centre
	if !near(r.LEDs.Positions[0][0], 60) || !near(r.LEDs.Positions[0][1], 16) {
		t.Errorf("first ring LED %v", r.LEDs.Positions[0])
	}
	if !near(r.LEDs.Positions[2][0], 104) || !near(r.LEDs.Positions[2][1], 60) {
		t.Errorf("third ring LED %v", r.LEDs.Positions[2])
	}
	for _, n := range []int{3, 65, -1} {
		if _, err := Resolve(fan, n, 0); err == nil {
			t.Errorf("fan LED count %d accepted", n)
		}
	}
	if _, err := Resolve(fan, 0, 100); err == nil {
		t.Error("length on a fan accepted")
	}
	s, err := Resolve(c.Parts["generic-strip"], 10, 500)
	if err != nil {
		t.Fatal(err)
	}
	if s.Dimensions.W != 500 || s.Outline.Front[0].W != 500 || !near(s.LEDs.Positions[9][0], 497) || !near(s.LEDs.Positions[0][0], 3) {
		t.Errorf("strip %+v %v", s.Dimensions, s.LEDs.Positions)
	}
	for _, l := range []float64{19, 1001, math.NaN()} {
		if _, err := Resolve(c.Parts["generic-strip"], 10, l); err == nil {
			t.Errorf("strip length %v accepted", l)
		}
	}
	rect, _ := Resolve(c.Parts["generic-distro-rect"], 0, 0)
	if !near(rect.LEDs.Positions[0][0], 6) || !near(rect.LEDs.Positions[0][1], 6) {
		t.Errorf("rect start %v", rect.LEDs.Positions[0])
	}
	if _, err := Resolve(c.Parts["generic-gpu-box"], 5, 0); err != nil {
		t.Error(err)
	}
}

func TestProjectExhaustMirror(t *testing.T) {
	in := mustProject(t, "generic-fan-120", Placement{ID: "a", View: "side", Flow: "intake", LEDs: 8, X: 10, Y: 20})
	ex := mustProject(t, "generic-fan-120", Placement{ID: "a", View: "side", Flow: "exhaust", LEDs: 8, X: 10, Y: 20})
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
	both := mustProject(t, "generic-fan-120", Placement{ID: "a", View: "side", Flow: "exhaust", Mirror: true, LEDs: 8, X: 10, Y: 20})
	for i := range in.LEDs {
		if !near(both.LEDs[i].X, in.LEDs[i].X) {
			t.Fatal("mirror + exhaust should cancel")
		}
	}
	s1 := mustProject(t, "generic-strip", Placement{ID: "a", View: "side", Flow: "exhaust"})
	s2 := mustProject(t, "generic-strip", Placement{ID: "a", View: "side"})
	if s1.LEDs[1] != s2.LEDs[1] {
		t.Error("flow changed a strip")
	}
	// mirror of an asymmetric part: strip first LED moves to the right end
	m := mustProject(t, "generic-strip", Placement{ID: "a", View: "side", Mirror: true})
	if !near(m.LEDs[0].X, 297) {
		t.Errorf("mirrored strip first LED %v", m.LEDs[0])
	}
}

func TestProjectEdgeOnLine(t *testing.T) {
	// a fan on the top panel seen from the side: a line along the top edge
	pr := mustProject(t, "generic-fan-120", Placement{ID: "a", View: "side", Face: "top", LEDs: 16, X: 100, Y: 5})
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
	pr = mustProject(t, "generic-fan-120", Placement{ID: "a", View: "side", Face: "front", LEDs: 16, X: 0, Y: 100})
	if !near(pr.Box.W, 25) || !near(pr.Box.H, 120) {
		t.Fatalf("box %+v", pr.Box)
	}
	for _, p := range pr.LEDs {
		if !near(p.X, 0) || p.Y < 100 || p.Y > 220 {
			t.Fatalf("LED %v", p)
		}
	}
	// top view: face front points to the top; face side to the right (x = depth edge)
	pr = mustProject(t, "generic-fan-120", Placement{ID: "a", View: "top", Face: "side", X: 10, Y: 10})
	for _, p := range pr.LEDs {
		if !near(p.X, 35) {
			t.Fatalf("right edge expected, LED %v", p)
		}
	}
	// downward-looking top face in the front view is the same as in the side view
	pr = mustProject(t, "generic-fan-120", Placement{ID: "a", View: "front", Face: "top", X: 0, Y: 0})
	if !pr.EdgeOn || !near(pr.Box.H, 25) {
		t.Errorf("%+v", pr.Box)
	}
}

func TestProjectRAMUpright(t *testing.T) {
	flat := mustProject(t, "generic-ram-dimm", Placement{ID: "r", View: "side", X: 10, Y: 10})
	up := mustProject(t, "generic-ram-dimm", Placement{ID: "r", View: "side", X: 10, Y: 10, Rot: 90})
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
	full := mustProject(t, "generic-ram-dimm", Placement{ID: "r", View: "side", X: 10, Y: 10, Rot: 360})
	for i := range flat.LEDs {
		if !near(full.LEDs[i].X, flat.LEDs[i].X) || !near(full.LEDs[i].Y, flat.LEDs[i].Y) {
			t.Fatal("rot 360 differs")
		}
	}
}

func TestProjectGPUVerticalVsHorizontal(t *testing.T) {
	h := mustProject(t, "generic-gpu-box", Placement{ID: "g", View: "side", X: 0, Y: 0})
	v := mustProject(t, "generic-gpu-box", Placement{ID: "g", View: "side", X: 0, Y: 0, Rot: 90})
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
	v3 := mustProject(t, "generic-gpu-box", Placement{ID: "g", View: "side", Rot: 270})
	if !near(v3.LEDs[0].X, 10) || v3.LEDs[0].Y <= v3.LEDs[len(v3.LEDs)-1].Y {
		t.Errorf("270: %v", v3.LEDs[0])
	}
}

func TestProjectErrors(t *testing.T) {
	c := Builtin().Parts["generic-fan-120"]
	for _, pl := range []Placement{
		{ID: "a", View: "bogus"}, {ID: "a", View: "side", Face: "bogus"},
		{ID: "a", X: math.NaN()}, {ID: "a", Rot: math.Inf(1)}, {ID: "a", LEDs: 3},
	} {
		if _, err := Project(c, pl); err == nil {
			t.Errorf("%+v accepted", pl)
		}
	}
}

func TestLetterbox(t *testing.T) {
	// wide box: fills the width, centred vertically
	ox, oy, sx, sy := Letterbox(640, 200)
	if !near(ox, 0) || !near(sx*640, 1) || !near(sy*200, 0.5) || !near(oy, 0.25) {
		t.Errorf("wide: %v %v %v %v", ox, oy, sx, sy)
	}
	// tall box 100 x 400: scale 0.5 px/mm -> 50 x 200 px, centred horizontally
	ox, oy, sx, sy = Letterbox(100, 400)
	if !near(oy, 0) || !near(sy*400, 1) || !near(sx*100*320, 50) || !near(ox*320, 135) {
		t.Errorf("tall: %v %v %v %v", ox, oy, sx, sy)
	}
	// 1 mm is the same number of pixels in both axes
	if !near(sx*320, sy*200) {
		t.Error("aspect not kept")
	}
	if _, _, a, b := Letterbox(0, 5); a != 0 || b != 0 {
		t.Error("degenerate box")
	}
	// case 450 x 480 (side view) fits by height
	_, _, _, sy = Letterbox(450, 480)
	if !near(sy*480, 1) {
		t.Errorf("side view of the default case: %v", sy*480)
	}
}

func box() Case { return Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 230, H: 480, D: 450}} }

func TestDeriveComponentAndSegment(t *testing.T) {
	s := Sketch{Version: 1, Case: box(), CanvasView: "side", Parts: []Placement{
		{ID: "f1", Part: "generic-fan-120", View: "side", X: 330, Y: 20, LEDs: 8, Label: "Front fan",
			Bind: &Binding{Device: "dev", Zone: "argb1"}},
		{ID: "free", Part: "generic-strip", View: "side", X: 0, Y: 0},
	}}
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Components) != 2 || d.Components[0].Name != "sketch.f1" || d.Components[1].Name != "sketch.free" {
		t.Fatalf("components %+v", d.Components)
	}
	c := d.Components[0]
	if !c.PositionsMM || len(c.LEDs) != 8 || c.SizeMM.X != 120 || c.Label != "Front fan" {
		t.Fatalf("component %+v", c)
	}
	if got := len(c.LEDsMM()); got != 8 {
		t.Errorf("LEDsMM %d", got)
	}
	if len(d.Zones) != 1 || d.Zones[0].Device != "dev" || d.Zones[0].LEDs != 8 {
		t.Fatalf("zones %+v", d.Zones)
	}
	sg := d.Zones[0].Segments[0]
	// side view 450 x 480: scale 200/480 px/mm, centred horizontally
	scale := 200.0 / 480
	ox := (320 - 450*scale) / 2 / 320
	if sg.Shape != "component" || sg.Component != "sketch.f1" || !near(sg.X, ox+330*scale/320) || !near(sg.Y, 20*scale/200) ||
		!near(sg.W, 120*scale/320) || !near(sg.H, 120*scale/200) || sg.Start == nil || *sg.Start != 0 {
		t.Errorf("segment %+v", sg)
	}
	// the engine reproduces the mm layout on the canvas: LED 0 at the fan's top centre
	if err := leds.RegisterComponent(c); err != nil {
		t.Fatal(err)
	}
	pts := leds.Positions(leds.Zone{ID: "argb1", Count: 8, Resizable: true}, leds.ZoneConfig{Segments: d.Zones[0].Segments})
	if len(pts) != 8 || !near(pts[0].X, ox+(330+60)*scale/320) || !near(pts[0].Y, (20+16)*scale/200) {
		t.Errorf("engine LED 0 = %v", pts[0])
	}
}

func chainSketch(binds ...*Binding) Sketch {
	s := Sketch{Version: 1, Case: box(), CanvasView: "side"}
	ids := []string{"a", "b", "c", "d"}
	parts := []string{"generic-fan-120", "generic-strip", "generic-ram-dimm", "generic-fan-120"}
	leds := []int{16, 10, 5, 4}
	for i, b := range binds {
		s.Parts = append(s.Parts, Placement{ID: ids[i], Part: parts[i], View: "side", X: float64(i * 10), LEDs: leds[i], Bind: b})
	}
	return s
}

func starts(z ZoneSegments) []int {
	var out []int
	for _, sg := range z.Segments {
		out = append(out, *sg.Start)
	}
	return out
}

func names(z ZoneSegments) string {
	var n []string
	for _, sg := range z.Segments {
		n = append(n, strings.TrimPrefix(sg.Component, "sketch."))
	}
	return strings.Join(n, ",")
}

func TestDeriveChainOrderAndParallel(t *testing.T) {
	// order wins over placement order; ties keep the sketch order
	s := chainSketch(
		&Binding{Device: "d", Zone: "z", Order: 2},
		&Binding{Device: "d", Zone: "z", Order: 0},
		&Binding{Device: "d", Zone: "z", Order: 2},
		&Binding{Device: "d", Zone: "other", Order: 0},
	)
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Zones) != 2 || d.Zones[0].Zone != "other" || d.Zones[1].Zone != "z" {
		t.Fatalf("zones %+v", d.Zones)
	}
	z := d.Zones[1]
	if names(z) != "b,a,c" {
		t.Errorf("order %s", names(z))
	}
	if got := starts(z); got[0] != 0 || got[1] != 10 || got[2] != 26 || z.LEDs != 31 {
		t.Errorf("starts %v total %d", got, z.LEDs)
	}
	if d.Zones[0].LEDs != 4 || *d.Zones[0].Segments[0].Start != 0 {
		t.Errorf("second zone %+v", d.Zones[0])
	}

	// parallel: same start as the previous part; the next series part starts behind the longer one
	s = chainSketch(
		&Binding{Device: "d", Zone: "z", Order: 0},                 // a: 16 LEDs
		&Binding{Device: "d", Zone: "z", Order: 1, Parallel: true}, // b: 10 LEDs, shares 0..9
		&Binding{Device: "d", Zone: "z", Order: 2},                 // c: 5 LEDs behind a
		&Binding{Device: "d", Zone: "z", Order: 3, Parallel: true}, // d: 4 LEDs shares c
	)
	d, err = Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	z = d.Zones[0]
	if got := starts(z); len(got) != 4 || got[0] != 0 || got[1] != 0 || got[2] != 16 || got[3] != 16 || z.LEDs != 21 {
		t.Errorf("parallel starts %v total %d", got, z.LEDs)
	}
	// a leading Parallel flag has nothing to share with
	s = chainSketch(&Binding{Device: "d", Zone: "z", Parallel: true})
	d, _ = Derive(s, Builtin())
	if *d.Zones[0].Segments[0].Start != 0 {
		t.Error("leading parallel part not at 0")
	}
}

func TestValidate(t *testing.T) {
	c := Builtin()
	ok := func() Sketch {
		return Sketch{Version: 1, Case: box(), CanvasView: "side", Parts: []Placement{
			{ID: "p1", Part: "generic-fan-120", View: "side", X: 10, Y: 10, Face: "side", Flow: "intake", LEDs: 16, Label: "Fan ü",
				Bind: &Binding{Device: "dev", Zone: "z", Order: 1}},
		}}
	}
	if err := Validate(ok(), c); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Sketch{Version: 1, Case: box()}, c); err != nil {
		t.Errorf("empty sketch: %v", err)
	}
	mut := func(f func(*Sketch)) Sketch { s := ok(); f(&s); return s }
	nan, inf := math.NaN(), math.Inf(1)
	cases := map[string]Sketch{
		"version":        mut(func(s *Sketch) { s.Version = 2 }),
		"source":         mut(func(s *Sketch) { s.Case.Source = "x" }),
		"custom id":      mut(func(s *Sketch) { s.Case.ID = "other" }),
		"library id":     mut(func(s *Sketch) { s.Case.Source = "library"; s.Case.ID = "Bad ID" }),
		"dims zero":      mut(func(s *Sketch) { s.Case.Dims.W = 0 }),
		"dims nan":       mut(func(s *Sketch) { s.Case.Dims.H = nan }),
		"dims huge":      mut(func(s *Sketch) { s.Case.Dims.D = 5000 }),
		"chamber":        mut(func(s *Sketch) { s.Case.Chambers = []Chamber{{X: 200, W: 100}} }),
		"chamber nan":    mut(func(s *Sketch) { s.Case.Chambers = []Chamber{{X: nan, W: 1}} }),
		"chambers many":  mut(func(s *Sketch) { s.Case.Chambers = make([]Chamber, 5) }),
		"canvas view":    mut(func(s *Sketch) { s.CanvasView = "back" }),
		"id empty":       mut(func(s *Sketch) { s.Parts[0].ID = "" }),
		"id upper":       mut(func(s *Sketch) { s.Parts[0].ID = "P1" }),
		"id long":        mut(func(s *Sketch) { s.Parts[0].ID = strings.Repeat("a", 58) }),
		"part":           mut(func(s *Sketch) { s.Parts[0].Part = "nope" }),
		"view":           mut(func(s *Sketch) { s.Parts[0].View = "x" }),
		"face":           mut(func(s *Sketch) { s.Parts[0].Face = "x" }),
		"flow":           mut(func(s *Sketch) { s.Parts[0].Flow = "sideways" }),
		"x nan":          mut(func(s *Sketch) { s.Parts[0].X = nan }),
		"y inf":          mut(func(s *Sketch) { s.Parts[0].Y = inf }),
		"rot nan":        mut(func(s *Sketch) { s.Parts[0].Rot = nan }),
		"rot range":      mut(func(s *Sketch) { s.Parts[0].Rot = 400 }),
		"x outside":      mut(func(s *Sketch) { s.Parts[0].X = 450 + MarginMM + 1 }),
		"x negative":     mut(func(s *Sketch) { s.Parts[0].X = -MarginMM - 1 }),
		"y outside":      mut(func(s *Sketch) { s.Parts[0].Y = 480 + MarginMM + 1 }),
		"leds low":       mut(func(s *Sketch) { s.Parts[0].LEDs = 2 }),
		"leds high":      mut(func(s *Sketch) { s.Parts[0].LEDs = 65 }),
		"leds negative":  mut(func(s *Sketch) { s.Parts[0].LEDs = -1 }),
		"leds fixed":     mut(func(s *Sketch) { s.Parts[0].Part = "generic-strip"; s.Parts[0].LEDs = 301 }),
		"len on fan":     mut(func(s *Sketch) { s.Parts[0].Len = 100 }),
		"len nan":        mut(func(s *Sketch) { s.Parts[0].Part = "generic-strip"; s.Parts[0].Len = nan }),
		"label long":     mut(func(s *Sketch) { s.Parts[0].Label = strings.Repeat("x", 41) }),
		"label newline":  mut(func(s *Sketch) { s.Parts[0].Label = "a\nb" }),
		"label control":  mut(func(s *Sketch) { s.Parts[0].Label = "a\x00" }),
		"label utf8":     mut(func(s *Sketch) { s.Parts[0].Label = "\xff" }),
		"bind no zone":   mut(func(s *Sketch) { s.Parts[0].Bind.Zone = "" }),
		"bind device":    mut(func(s *Sketch) { s.Parts[0].Bind.Device = "a\tb" }),
		"bind order":     mut(func(s *Sketch) { s.Parts[0].Bind.Order = -1 }),
		"bind order big": mut(func(s *Sketch) { s.Parts[0].Bind.Order = 1001 }),
		"duplicate id":   mut(func(s *Sketch) { s.Parts = append(s.Parts, s.Parts[0]) }),
	}
	for name, s := range cases {
		if err := Validate(s, c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// exactly 64 parts is fine, 65 is not
	many := Sketch{Version: 1, Case: box()}
	for i := 0; i < 65; i++ {
		many.Parts = append(many.Parts, Placement{ID: "p" + string(rune('a'+i/26)) + string(rune('a'+i%26)), Part: "generic-fan-120", View: "side", LEDs: 4})
		if i == 63 {
			if err := Validate(many, c); err != nil {
				t.Fatalf("64 parts: %v", err)
			}
		}
	}
	if err := Validate(many, c); err == nil {
		t.Error("65 parts accepted")
	}
	// total LED cap
	big := Sketch{Version: 1, Case: box()}
	for i := 0; i < 14; i++ {
		big.Parts = append(big.Parts, Placement{ID: "s" + string(rune('a'+i)), Part: "generic-strip", View: "side", LEDs: 300})
	}
	if err := Validate(big, c); err == nil {
		t.Error("LED total cap not enforced")
	}
	// valid edge values
	e := ok()
	e.Parts[0].Rot, e.Parts[0].X, e.Parts[0].LEDs = -360, -MarginMM, 64
	if err := Validate(e, c); err != nil {
		t.Errorf("edge values: %v", err)
	}
	// nil catalog maps do not panic
	if err := Validate(ok(), Catalog{}); err == nil {
		t.Error("empty catalog accepted a part")
	}
}

func TestDecode(t *testing.T) {
	good := `{"version":1,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450},"chambers":[{"x":0,"w":150}]},
	"canvasView":"side","parts":[{"id":"p1","part":"generic-fan-120","view":"side","x":40,"y":60,"rot":0,"face":"side",
	"mirror":false,"flow":"intake","leds":16,"label":"","bind":{"device":"d","zone":"argb1","order":0,"parallel":false}}]}`
	s, err := Decode([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(s, Builtin()); err != nil {
		t.Fatal(err)
	}
	// round trip
	raw, _ := json.Marshal(s)
	s2, err := Decode(raw)
	if err != nil || Validate(s2, Builtin()) != nil {
		t.Fatalf("round trip: %v", err)
	}
	for _, bad := range []string{``, `{`, good + `{}`, strings.Replace(good, `"version"`, `"extra":1,"version"`, 1), `[]`, `{"version":"1"}`,
		`{"version":1,"parts":[{"x":1e999}]}`, strings.Repeat(" ", MaxBodyBytes+1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("accepted %.40q", bad)
		}
	}
}

func TestDeriveRejectsInvalid(t *testing.T) {
	if _, err := Derive(Sketch{Version: 9}, Builtin()); err == nil {
		t.Error("invalid sketch derived")
	}
}

func TestDerivedComponentsRegister(t *testing.T) {
	s := chainSketch(&Binding{Device: "d", Zone: "z"}, &Binding{Device: "d", Zone: "z", Order: 1}, nil)
	s.Parts[1].Len = 120
	s.Parts[2].Face = "top" // edge-on
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.Components {
		if err := leds.RegisterComponent(c); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
		for _, p := range c.LEDs {
			if p.X < 0 || p.Y < 0 || p.X > c.SizeMM.X || p.Y > c.SizeMM.Y {
				t.Errorf("%s: LED %v outside %v", c.Name, p, c.SizeMM)
			}
		}
	}
}
