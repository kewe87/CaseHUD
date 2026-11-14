package sketch

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// An L-shaped strip of 47 LEDs: along the front from the far end to the
// glass corner, then along the glass side to the rear.
const lCase = `{"format":1,"id":"l-case","vendor":"V","model":"L","dimensions_mm":{"w":290,"h":471,"d":478},%s
 "integrated_lighting":[
  {"id":"bottom","component":"l-strip","view":"front","position_mm":[%s,464],"path_mm":[[%s,464],[%s,464]],
   "legs":[{"view":"side","path_mm":[[%s,464],[%s,464]]}]},
  {"id":"top","component":"l-strip","view":"front","position_mm":[%s,7],"path_mm":[[%s,7],[%s,7]],
   "legs":[{"view":"side","path_mm":[[%s,7],[%s,7]]}]}]}`

// lCaseJSON builds the case: the far end of the front run and the side run in
// view coordinates of the given orientation.
func lCaseJSON(glassRight, frontLeft bool) string {
	orient := ""
	far, corner := "285", "0" // front view: glass on the left
	if glassRight {
		far, corner = "5", "290"
	}
	sFront, sRear := "478", "18" // side view: rear on the left
	if frontLeft {
		sFront, sRear = "0", "460"
	}
	switch {
	case glassRight && frontLeft:
		orient = `"orientation":{"glass_side":"right","side_view_left":"front"},`
	case glassRight:
		orient = `"orientation":{"glass_side":"right"},`
	case frontLeft:
		orient = `"orientation":{"side_view_left":"front"},`
	}
	return fmt.Sprintf(lCase, orient, far, far, corner, sFront, sRear, far, far, corner, sFront, sRear)
}

func stripPart() Part {
	p := Part{Format: 1, ID: "l-strip", Category: "strip", Dimensions: PartDims{W: 750, H: 10, D: 3}}
	for i := 0; i < 47; i++ {
		p.LEDs.Positions = append(p.LEDs.Positions, [2]float64{745 - float64(i)*740/46, 5})
	}
	p.LEDs.Count = 47
	p.LEDs.Zones = []PartZone{{ID: "front-leg", First: 0, Count: 17}, {ID: "side-leg", First: 17, Count: 30}}
	return p
}

func lCatalog(t *testing.T, data string) (Catalog, CaseTemplate) {
	t.Helper()
	tpl, err := ParseCase([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	cat := Builtin()
	cat.Parts["l-strip"] = stripPart()
	if err := cat.AddCase(tpl); err != nil {
		t.Fatal(err)
	}
	return cat, tpl
}

func lSketch(tpl CaseTemplate, cat Catalog) Sketch {
	s := Sketch{Version: Version, CanvasView: CanvasNet,
		Case: Case{Source: "library", ID: tpl.ID, Dims: tpl.Dims, Orientation: tpl.Orientation}}
	return SyncCaseLights(s, cat)
}

func TestParseCaseLighting(t *testing.T) {
	for _, o := range []struct{ glassRight, frontLeft bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		_, tpl := lCatalog(t, lCaseJSON(o.glassRight, o.frontLeft))
		if len(tpl.Lighting) != 2 || tpl.Lighting[0].ID != "bottom" || tpl.Lighting[1].ID != "top" {
			t.Fatalf("%+v: %+v", o, tpl.Lighting)
		}
		// whatever the orientation, the model path is the same: front far end,
		// glass front corner, glass side rear end (the shared corner once)
		want := []Vec3{{285, 464, 478}, {0, 464, 478}, {0, 464, 18}}
		if o.glassRight {
			want[0].X = 285 // u 5 seen from the front is 285 mm from the glass
		}
		got := tpl.Lighting[0].Path
		if len(got) != 3 {
			t.Fatalf("%+v: path %v", o, got)
		}
		for i := range want {
			if dist3(got[i], want[i]) > 1e-6 {
				t.Errorf("%+v: point %d = %v, want %v", o, i, got[i], want[i])
			}
		}
	}
}

func TestParseCaseLightingTolerant(t *testing.T) {
	// The first published template: two single-view runs sharing one part's
	// zones, without ids; plus entries the sketch cannot place.
	old := `{"format":1,"id":"old-case","vendor":"V","model":"O","dimensions_mm":{"w":290,"h":471,"d":478},
	 "integrated_lighting":[
	  {"component":"l-strip","zone":"front-leg","view":"front","position_mm":[280,464],"path_mm":[[280,464],[10,464]]},
	  {"component":"l-strip","zone":"side-leg","view":"side","position_mm":[468,464],"path_mm":[[468,464],[10,464]]},
	  {"component":"l-strip","view":"front","position_mm":[10,10]},
	  {"component":"l-strip","view":"top","position_mm":[10,10],"path_mm":[[0,0],[5,5]]},
	  {"component":"l-strip","view":"front","position_mm":[0,0],"path_mm":[[0,0],[5000,0]]}]}`
	cat, tpl := lCatalog(t, old)
	if len(tpl.Lighting) != 2 || tpl.Lighting[0].ID != "front-leg" || tpl.Lighting[1].ID != "side-leg" {
		t.Fatalf("%+v", tpl.Lighting)
	}
	s := lSketch(tpl, cat)
	if len(s.Parts) != 2 || s.Parts[0].LEDs != 17 || s.Parts[1].LEDs != 30 {
		t.Fatalf("%+v", s.Parts)
	}
	if err := Validate(s, cat); err != nil {
		t.Fatal(err)
	}
	if _, err := Derive(s, cat); err != nil {
		t.Fatal(err)
	}
}

// netLEDs projects the case light at index i onto the net.
func netLEDs(t *testing.T, s Sketch, cat Catalog, i int) ([]Point, []PathLED, Net) {
	t.Helper()
	_, n := SketchNet(s, cat)
	p := s.Parts[i]
	pr, err := ProjectNet(cat.Parts[p.Part], p, s.Case, n, "")
	if err != nil {
		t.Fatal(err)
	}
	return pr.LEDs, placementLEDs(cat.Parts[p.Part], p, s.Case.Dims), n
}

func TestCaseLightContinuousAcrossTheCorner(t *testing.T) {
	for _, glassRight := range []bool{false, true} {
		cat, tpl := lCatalog(t, lCaseJSON(glassRight, false))
		s := lSketch(tpl, cat)
		for i := range s.Parts {
			pts, ls, n := netLEDs(t, s, cat, i)
			if len(pts) != 47 {
				t.Fatalf("%d LEDs", len(pts))
			}
			// both faces are drawn at full size
			for _, f := range []string{FacingSide, FacingFront} {
				if nf, ok := n.Face(f); !ok || nf.Collapsed {
					t.Fatalf("face %s collapsed or missing: %+v", f, n.Faces)
				}
			}
			step := (285.0 + 460.0) / 47
			corner := -1
			for k := 1; k < len(pts); k++ {
				d := math.Hypot(pts[k].X-pts[k-1].X, pts[k].Y-pts[k-1].Y)
				if math.Abs(d-step) > 0.01 {
					t.Errorf("glassRight=%v light %d: LEDs %d-%d are %.3f mm apart in the net, want %.3f", glassRight, i, k-1, k, d, step)
				}
				if ls[k].Face != ls[k-1].Face {
					corner = k
				}
			}
			if corner < 0 || ls[corner-1].Face != FacingFront || ls[corner].Face != FacingSide {
				t.Fatalf("glassRight=%v: no front -> side change: %v", glassRight, ls)
			}
		}
	}
}

func TestCaseLightLEDOrder(t *testing.T) {
	cat, tpl := lCatalog(t, lCaseJSON(false, false))
	s := lSketch(tpl, cat)
	_, ls, _ := netLEDs(t, s, cat, 0)
	first, last := ls[0], ls[len(ls)-1]
	if first.Face != FacingFront || first.Pos.X < 270 || last.Face != FacingSide || last.Pos.Z > 40 {
		t.Fatalf("LED 1 %+v, last %+v", first, last)
	}
	for k := 1; k < len(ls); k++ { // runs towards the corner, then to the rear
		a, b := ls[k-1].Pos, ls[k].Pos
		if b.X > a.X+1e-9 || b.Z > a.Z+1e-9 {
			t.Fatalf("LED %d goes back: %v -> %v", k, a, b)
		}
	}
	// Mirror reverses the order on the same positions
	s.Parts[0].Mirror = true
	_, rs, _ := netLEDs(t, s, cat, 0)
	for k := range ls {
		if rs[k] != ls[len(ls)-1-k] {
			t.Fatalf("reversed LED %d = %+v, want %+v", k, rs[k], ls[len(ls)-1-k])
		}
	}
}

func TestCaseLightOrientationMirror(t *testing.T) {
	catL, tplL := lCatalog(t, lCaseJSON(false, false))
	catR, tplR := lCatalog(t, lCaseJSON(true, true))
	sL, sR := lSketch(tplL, catL), lSketch(tplR, catR)
	pl, _, nl := netLEDs(t, sL, catL, 0)
	pr, _, nr := netLEDs(t, sR, catR, 0)
	if nl.W != nr.W || nl.H != nr.H {
		t.Fatalf("nets differ: %vx%v vs %vx%v", nl.W, nl.H, nr.W, nr.H)
	}
	for k := range pl {
		if math.Abs(pr[k].X-(nl.W-pl[k].X)) > 1e-6 || math.Abs(pr[k].Y-pl[k].Y) > 1e-6 {
			t.Fatalf("LED %d: glass right %v, glass left %v (net width %v)", k, pr[k], pl[k], nl.W)
		}
	}
	// glass left: the front is right of the glass side; glass right: left of it
	if pl[0].X < pl[46].X || pr[0].X > pr[46].X {
		t.Fatalf("front run on the wrong side: left %v..%v, right %v..%v", pl[0], pl[46], pr[0], pr[46])
	}
}

func TestCaseLightEachLEDOnce(t *testing.T) {
	cat, tpl := lCatalog(t, lCaseJSON(false, false))
	s := lSketch(tpl, cat)
	s.Parts[0].Bind = &Binding{Device: "dev", Zone: "argb1"}
	s.Parts[1].Bind = &Binding{Device: "dev", Zone: "argb1", Order: 1}
	d, err := Derive(s, cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Components) != 2 || len(d.Zones) != 1 || d.Zones[0].LEDs != 94 || len(d.Zones[0].Segments) != 2 {
		t.Fatalf("%+v", d)
	}
	_, _, sx, sy := Letterbox(func() (float64, float64) { _, n := SketchNet(s, cat); return n.W, n.H }())
	seen := map[[2]int64]bool{}
	for ci, comp := range d.Components {
		if len(comp.LEDs) != 47 {
			t.Fatalf("component %d has %d LEDs", ci, len(comp.LEDs))
		}
		seg := d.Zones[0].Segments[ci]
		for _, q := range comp.LEDs {
			// canvas position of the LED: segment box plus its offset
			x := seg.X + q.X*sx
			y := seg.Y + q.Y*sy
			key := [2]int64{int64(math.Round(x * 1e5)), int64(math.Round(y * 1e5))}
			if seen[key] {
				t.Fatalf("two LEDs at %v", key)
			}
			seen[key] = true
		}
	}
	if len(seen) != 94 {
		t.Fatalf("%d distinct LEDs", len(seen))
	}
}

func TestSyncCaseLights(t *testing.T) {
	cat, tpl := lCatalog(t, lCaseJSON(false, false))
	s := lSketch(tpl, cat)
	if len(s.Parts) != 2 || s.Parts[0].ID != "case-bottom" || s.Parts[1].ID != "case-top" || s.Parts[0].LEDs != 47 {
		t.Fatalf("%+v", s.Parts)
	}
	// user data survives a sync, geometry comes from the template
	s.Parts[0].Bind = &Binding{Device: "d", Zone: "z", Order: 2}
	s.Parts[0].Label, s.Parts[0].Mirror, s.Parts[0].Hidden = "Unten", true, true
	s.Parts[0].Path[0].X = 100
	s.Parts[0].LEDs = 3
	again := SyncCaseLights(s, cat)
	p := again.Parts[0]
	if p.Bind == nil || p.Bind.Order != 2 || p.Label != "Unten" || !p.Mirror || !p.Hidden || p.Path[0].X != 285 || p.LEDs != 47 {
		t.Fatalf("%+v", p)
	}
	// a user part with the id a light would take
	u := Sketch{Version: Version, Case: s.Case, Parts: []Placement{{ID: "case-top", Part: "generic-fan-120", Pos: Vec3{100, 100, 100}, Facing: FacingSide}}}
	u = SyncCaseLights(u, cat)
	if len(u.Parts) != 3 || u.Parts[0].ID != "case-top" || u.Parts[0].IsCaseLight() || u.Parts[2].ID != "case-top-2" {
		t.Fatalf("%+v", u.Parts)
	}
	// a custom box has no case lights
	c := again
	c.Case = Case{Source: "custom", ID: CustomCaseID, Dims: s.Case.Dims}
	if got := SyncCaseLights(c, cat); len(got.Parts) != 0 {
		t.Fatalf("%+v", got.Parts)
	}
	// an unknown library case keeps them
	k := again
	k.Case.ID = "gone"
	if got := SyncCaseLights(k, cat); len(got.Parts) != 2 {
		t.Fatalf("%+v", got.Parts)
	}
	if err := Validate(again, cat); err != nil {
		t.Fatal(err)
	}
	if len(Overlaps(again, cat)) != 0 {
		t.Fatal("case lights overlap")
	}
}

func TestValidateCaseLight(t *testing.T) {
	cat, tpl := lCatalog(t, lCaseJSON(false, false))
	base := lSketch(tpl, cat)
	for name, f := range map[string]func(*Placement){
		"path without caseLight": func(p *Placement) { p.CaseLight = "" },
		"one point":              func(p *Placement) { p.Path = p.Path[:1] },
		"outside":                func(p *Placement) { p.Path[0].X = 900 },
		"nan":                    func(p *Placement) { p.Path[0].Y = math.NaN() },
		"length":                 func(p *Placement) { p.Len = 300 },
		"group":                  func(p *Placement) { p.Group = "g" },
		"bad id":                 func(p *Placement) { p.CaseLight = "Bad Id" },
		"too many LEDs":          func(p *Placement) { p.LEDs = MaxLEDs + 1 },
	} {
		s := base
		s.Parts = append([]Placement(nil), base.Parts...)
		s.Parts[0].Path = append([]Vec3(nil), base.Parts[0].Path...)
		f(&s.Parts[0])
		if err := Validate(s, cat); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	mounted := base
	mounted.Parts = append([]Placement{{ID: "f", Part: "generic-fan-120", Pos: Vec3{100, 100, 100}, Facing: FacingSide, On: "case-top"}}, base.Parts...)
	if err := Validate(mounted, cat); err == nil || !strings.Contains(err.Error(), "case light") {
		t.Errorf("mounted on a case light: %v", err)
	}
}
