package sketch

import (
	"math"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

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
