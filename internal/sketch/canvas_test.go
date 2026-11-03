package sketch

import (
	"fmt"
	"math"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func TestMapToView(t *testing.T) {
	d := Dims{W: 230, H: 480, D: 450}
	// Front view (w, h) sits at d=0, side view (d, h) at w=W, top view (w, d) at h=0.
	for _, c := range []struct {
		name, from, to string
		x, y           float64
		wx, wy         float64
	}{
		{"same front", "front", "front", 10, 20, 10, 20},
		{"same side", "side", "side", 10, 20, 10, 20},
		{"same top", "top", "top", 10, 20, 10, 20},
		{"front to side", "front", "side", 10, 20, 0, 20},   // d=0, h=y
		{"front to top", "front", "top", 10, 20, 10, 0},     // w=x, d=0
		{"side to front", "side", "front", 10, 20, 230, 20}, // w=W, h=y
		{"side to top", "side", "top", 10, 20, 230, 10},     // w=W, d=x
		{"top to front", "top", "front", 10, 20, 10, 0},     // w=x, h=0
		{"top to side", "top", "side", 10, 20, 20, 0},       // d=y, h=0
	} {
		x, y := MapToView(d, c.from, c.to, c.x, c.y)
		if x != c.wx || y != c.wy {
			t.Errorf("%s: got (%g, %g), want (%g, %g)", c.name, x, y, c.wx, c.wy)
		}
	}
}

// Parts of different views land where they sit relative to each other in the
// canvas view; the axis a view lacks comes from the face it looks at.
func TestDeriveMapsEveryViewIntoTheCanvasView(t *testing.T) {
	dims := box().Dims
	bind := func(z string) *Binding { return &Binding{Device: "d", Zone: z} }
	parts := []Placement{
		{ID: "fr", Part: "generic-fan-120", View: "front", X: 40, Y: 60, Bind: bind("fr")},
		{ID: "sd", Part: "generic-fan-120", View: "side", X: 100, Y: 70, Bind: bind("sd")},
		{ID: "tp", Part: "generic-fan-120", View: "top", X: 30, Y: 90, Bind: bind("tp")},
	}
	for _, cv := range []string{"front", "side", "top"} {
		s := Sketch{Version: 1, Case: box(), CanvasView: cv, Parts: parts}
		d, err := Derive(s, Builtin())
		if err != nil {
			t.Fatalf("%s: %v", cv, err)
		}
		cw, ch, _ := ViewSize(dims, cv)
		ox, oy, sx, sy := Letterbox(cw, ch)
		seg := map[string]leds.Segment{}
		for _, z := range d.Zones {
			seg[z.Zone] = z.Segments[0]
		}
		// expected canvas box (mm) of a 120 mm fan placed flat in its own view
		for _, p := range parts {
			ax, ay := MapToView(dims, p.View, cv, p.X, p.Y)
			bx, by := MapToView(dims, p.View, cv, p.X+120, p.Y+120)
			wx, wy := math.Min(ax, bx), math.Min(ay, by)
			ww, wh := math.Max(math.Abs(ax-bx), minExtentMM), math.Max(math.Abs(ay-by), minExtentMM)
			g := seg[p.ID]
			if !near(g.X, ox+wx*sx) || !near(g.Y, oy+wy*sy) || !near(g.W, ww*sx) || !near(g.H, wh*sy) {
				t.Errorf("canvas %s, part %s: segment %+v, want box (%g, %g, %g, %g)", cv, p.ID, g, wx, wy, ww, wh)
			}
		}
		// the part drawn in the canvas view itself is a flat square
		own := seg[map[string]string{"front": "fr", "side": "sd", "top": "tp"}[cv]]
		if !near(own.W*320, own.H*200) || own.W < 0.1 {
			t.Errorf("canvas %s: own part %+v not a flat square", cv, own)
		}
	}

	// Concrete case: canvas view front, the side view part sits at the case
	// width (right edge) and collapses onto a vertical line.
	s := Sketch{Version: 1, Case: box(), CanvasView: "front", Parts: parts[1:2]}
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	c := d.Components[0]
	for _, p := range c.LEDs {
		if p.X > minExtentMM {
			t.Fatalf("LED %v of a collapsed part is off its line (width %g)", p, c.SizeMM.X)
		}
	}
	ox, _, sx, _ := Letterbox(230, 480)
	if g := d.Zones[0].Segments[0]; !near(g.X, ox+230*sx) {
		t.Errorf("side part at x=%g, want the case width at %g", g.X, ox+230*sx)
	}
	// The engine puts the LEDs on that line.
	if err := leds.RegisterComponent(c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { leds.UnregisterComponent(c.Name) })
	for _, p := range leds.Positions(leds.Zone{ID: "z", Resizable: true}, leds.ZoneConfig{Segments: d.Zones[0].Segments}) {
		if !near(p.X, ox+230*sx) {
			t.Fatalf("engine LED %v off the case width line", p)
		}
	}
}

// Parts in the margin or larger than the case give segment boxes outside the
// canvas; they are clamped so the LED engine accepts the layout.
func TestDeriveClampsSegmentsToTheCanvas(t *testing.T) {
	bind := &Binding{Device: "d", Zone: "z"}
	for _, cv := range []string{"front", "side", "top"} {
		for _, view := range []string{"front", "side", "top"} {
			vw, vh, _ := ViewSize(box().Dims, view)
			s := Sketch{Version: 1, Case: box(), CanvasView: cv, Parts: []Placement{
				{ID: "a", Part: "generic-fan-120", View: view, X: -MarginMM, Y: -MarginMM, Bind: bind},
				{ID: "b", Part: "generic-strip", View: view, X: -MarginMM, Y: 400, Len: 1000, Bind: bind},
				{ID: "c", Part: "generic-fan-120", View: view, X: vw + MarginMM, Y: 10, Bind: bind},
				{ID: "d", Part: "generic-fan-120", View: view, X: 10, Y: vh + MarginMM, Bind: bind},
			}}
			d, err := Derive(s, Builtin())
			if err != nil {
				t.Fatalf("%s/%s: %v", cv, view, err)
			}
			for _, sg := range d.Zones[0].Segments {
				for _, v := range []float64{sg.X, sg.Y, sg.W, sg.H} {
					if math.IsNaN(v) || v < 0 || v > 1 {
						t.Errorf("canvas %s, view %s: segment %+v leaves 0..1", cv, view, sg)
					}
				}
			}
			for _, c := range d.Components {
				if err := leds.RegisterComponent(c); err != nil {
					t.Errorf("%s: %v", c.Name, err)
				}
				leds.UnregisterComponent(c.Name)
			}
		}
	}
}

func TestValidateZoneSegmentLimit(t *testing.T) {
	var parts []Placement
	for i := 0; i < MaxZoneSegments+1; i++ {
		parts = append(parts, Placement{ID: fmt.Sprintf("p%d", i), Part: "generic-strip", View: "side", X: 0, Y: float64(i),
			Bind: &Binding{Device: "d", Zone: "z", Order: i}})
	}
	s := Sketch{Version: 1, Case: box(), CanvasView: "side", Parts: parts}
	err := Validate(s, Builtin())
	if err == nil || !strings.Contains(err.Error(), "more than 16 parts in zone d/z") {
		t.Fatalf("17 parts in one zone: %v", err)
	}
	// 16 in one zone and one in another is fine.
	s.Parts[16].Bind.Zone = "other"
	if err := Validate(s, Builtin()); err != nil {
		t.Fatalf("16 + 1: %v", err)
	}
}

func TestUsable(t *testing.T) {
	c := Builtin()
	mk := func(id, part string) Placement {
		return Placement{ID: id, Part: part, View: "side", Bind: &Binding{Device: "d", Zone: "z"}}
	}
	s := Sketch{Version: 1, Case: box(), CanvasView: "side", Parts: []Placement{
		mk("a", "generic-fan-120"), mk("b", "gone"), mk("c", "generic-fan-140"), mk("a", "generic-fan-120"),
	}}
	if Validate(s, c) == nil {
		t.Fatal("sketch with a missing part validates")
	}
	ok, skipped := Usable(s, c)
	if len(ok.Parts) != 2 || ok.Parts[0].ID != "a" || ok.Parts[1].ID != "c" {
		t.Fatalf("usable %+v", ok.Parts)
	}
	if len(skipped) != 2 || skipped[0] != 1 || skipped[1] != 3 {
		t.Fatalf("skipped %v", skipped)
	}
	if err := Validate(ok, c); err != nil {
		t.Fatalf("usable part does not validate: %v", err)
	}
	if len(s.Parts) != 4 {
		t.Fatal("input changed")
	}
	// A sketch that is broken as a whole gives nothing usable.
	bad := s
	bad.Version = 7
	if ok, sk := Usable(bad, c); len(ok.Parts) != 0 || len(sk) != 4 {
		t.Fatalf("bad version: %d usable, %v skipped", len(ok.Parts), sk)
	}
}

func TestParsePartChecksConnectionAndDirection(t *testing.T) {
	base := `{"format":1,"id":"x","vendor":"v","model":"m","category":"fan","dimensions_mm":{"w":120,"h":120,"d":25},
	"outline":{"front":[{"type":"rect","w":120,"h":120}]},"leds":{"count":1,"positions_mm":[[60,60]]%s}%s}`
	if _, err := ParsePart([]byte(fmt.Sprintf(base, "", ""))); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"direction": fmt.Sprintf(base, `,"direction":"a\u0007b"`, ""),
		"type":      fmt.Sprintf(base, "", `,"connection":{"type":"x\u0000"}`),
		"notes":     fmt.Sprintf(base, "", `,"connection":{"type":"argb","notes":"`+strings.Repeat("n", maxLibText+1)+`"}`),
	} {
		if _, err := ParsePart([]byte(raw)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
