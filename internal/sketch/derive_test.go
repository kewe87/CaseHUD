package sketch

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func box() Case { return Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 230, H: 480, D: 450}} }

// flatSide places a part seen flat in the side view with its top left corner
// at (u, v) there, against the glass.
func flatSide(id, part string, u, v float64, nLEDs int, length float64) Placement {
	r, err := Resolve(Builtin().Parts[part], nLEDs, length)
	if err != nil {
		panic(err)
	}
	w, h, d := r.Dimensions.W, r.Dimensions.H, r.Dimensions.D
	return Placement{ID: id, Part: part, Facing: FacingSide, Pos: Vec3{d / 2, v + h/2, u + w/2}, LEDs: nLEDs, Len: length}
}

func TestDeriveComponentAndSegment(t *testing.T) {
	f1 := flatSide("f1", "generic-fan-120", 330, 20, 8, 0)
	f1.Label = "Front fan"
	f1.Bind = &Binding{Device: "dev", Zone: "argb1"}
	s := Sketch{Version: Version, Case: box(), CanvasView: "side", Parts: []Placement{f1, flatSide("free", "generic-strip", 0, 0, 0, 0)}}
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
	// the net: empty faces collapsed to 46 mm around the 450 x 480 glass face
	_, net := SketchNet(s, Builtin())
	if !near(net.W, 46+450+46) || !near(net.H, 46+480+46) {
		t.Fatalf("net %g x %g", net.W, net.H)
	}
	scale := math.Min(320/net.W, 200/net.H)
	ox, oy := (320-net.W*scale)/2/320, (200-net.H*scale)/2/200
	gx, gy := 46.0, 46.0 // glass face origin in the net
	if sg.Shape != "component" || sg.Component != "sketch.f1" || !near(sg.X, ox+(gx+330)*scale/320) || !near(sg.Y, oy+(gy+20)*scale/200) ||
		!near(sg.W, 120*scale/320) || !near(sg.H, 120*scale/200) || sg.Start == nil || *sg.Start != 0 {
		t.Errorf("segment %+v", sg)
	}
	// the engine reproduces the mm layout on the canvas: LED 0 at the fan's top centre
	if err := leds.RegisterComponent(c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { leds.UnregisterComponent(c.Name) })
	pts := leds.Positions(leds.Zone{ID: "argb1", Count: 8, Resizable: true}, leds.ZoneConfig{Segments: d.Zones[0].Segments})
	if len(pts) != 8 || !near(pts[0].X, ox+(gx+330+60)*scale/320) || !near(pts[0].Y, oy+(gy+20+16)*scale/200) {
		t.Errorf("engine LED 0 = %v", pts[0])
	}
}

func chainSketch(binds ...*Binding) Sketch {
	s := Sketch{Version: Version, Case: box(), CanvasView: "side"}
	ids := []string{"a", "b", "c", "d"}
	parts := []string{"generic-fan-120", "generic-strip", "generic-ram-dimm", "generic-fan-120"}
	nLEDs := []int{16, 10, 5, 4}
	for i, b := range binds {
		p := flatSide(ids[i], parts[i], float64(i*10), 0, nLEDs[i], 0)
		p.Bind = b
		s.Parts = append(s.Parts, p)
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

// Parts in the margin or larger than the case give segment boxes outside the
// canvas; they are clamped so the LED engine accepts the layout.
func TestDeriveClampsSegmentsToTheCanvas(t *testing.T) {
	bind := &Binding{Device: "d", Zone: "z"}
	dims := box().Dims
	for _, cv := range []string{"front", "rear", "side", "top", "bottom"} {
		for _, facing := range []string{"front", "side", "top", "tray"} {
			mk := func(id, part string, x, y, z float64, length float64) Placement {
				return Placement{ID: id, Part: part, Facing: facing, Pos: Vec3{x, y, z}, Len: length, Bind: bind}
			}
			s := Sketch{Version: Version, Case: box(), CanvasView: cv, Parts: []Placement{
				mk("a", "generic-fan-120", -MarginMM, -MarginMM, -MarginMM, 0),
				mk("b", "generic-strip", -MarginMM, 400, dims.D+MarginMM, 1000),
				mk("c", "generic-fan-120", dims.W+MarginMM, 10, 10, 0),
				mk("d", "generic-fan-120", 10, dims.H+MarginMM, 10, 0),
			}}
			d, err := Derive(s, Builtin())
			if err != nil {
				t.Fatalf("%s/%s: %v", cv, facing, err)
			}
			for _, sg := range d.Zones[0].Segments {
				for _, v := range []float64{sg.X, sg.Y, sg.W, sg.H} {
					if math.IsNaN(v) || v < 0 || v > 1 {
						t.Errorf("canvas %s, facing %s: segment %+v leaves 0..1", cv, facing, sg)
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
		p := flatSide(fmt.Sprintf("p%d", i), "generic-strip", 0, float64(i), 0, 0)
		p.Bind = &Binding{Device: "d", Zone: "z", Order: i}
		parts = append(parts, p)
	}
	s := Sketch{Version: Version, Case: box(), CanvasView: "side", Parts: parts}
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
		p := flatSide(id, "generic-fan-120", 0, 0, 0, 0)
		p.Part = part
		p.Bind = &Binding{Device: "d", Zone: "z"}
		return p
	}
	s := Sketch{Version: Version, Case: box(), CanvasView: "side", Parts: []Placement{
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
	// A part mounted on a skipped part goes with it.
	m := Sketch{Version: Version, Case: box(), Parts: []Placement{mk("root", "gone"), mk("kid", "generic-fan-120"), mk("free", "generic-fan-120")}}
	m.Parts[1].On = "root"
	ok, skipped = Usable(m, c)
	if len(ok.Parts) != 1 || ok.Parts[0].ID != "free" || len(skipped) != 2 || skipped[0] != 0 || skipped[1] != 1 {
		t.Fatalf("mounted on a skipped part: %+v %v", ok.Parts, skipped)
	}
	if err := Validate(ok, c); err != nil {
		t.Fatal(err)
	}
}

func TestValidate(t *testing.T) {
	c := Builtin()
	ok := func() Sketch {
		p := flatSide("p1", "generic-fan-120", 10, 10, 16, 0)
		p.Flow, p.Label, p.Bind = "intake", "Fan ü", &Binding{Device: "dev", Zone: "z", Order: 1}
		p.Group, p.Layer = "front-1", 5
		return Sketch{Version: Version, Case: box(), CanvasView: "side", Parts: []Placement{p}}
	}
	if err := Validate(ok(), c); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Sketch{Version: Version, Case: box()}, c); err != nil {
		t.Errorf("empty sketch: %v", err)
	}
	mut := func(f func(*Sketch)) Sketch { s := ok(); f(&s); return s }
	nan, inf := math.NaN(), math.Inf(1)
	cases := map[string]Sketch{
		"version":          mut(func(s *Sketch) { s.Version = 1 }),
		"source":           mut(func(s *Sketch) { s.Case.Source = "x" }),
		"custom id":        mut(func(s *Sketch) { s.Case.ID = "other" }),
		"library id":       mut(func(s *Sketch) { s.Case.Source = "library"; s.Case.ID = "Bad ID" }),
		"dims zero":        mut(func(s *Sketch) { s.Case.Dims.W = 0 }),
		"dims nan":         mut(func(s *Sketch) { s.Case.Dims.H = nan }),
		"dims huge":        mut(func(s *Sketch) { s.Case.Dims.D = 5000 }),
		"chamber":          mut(func(s *Sketch) { s.Case.Chambers = []Chamber{{X: 200, W: 100}} }),
		"chamber nan":      mut(func(s *Sketch) { s.Case.Chambers = []Chamber{{X: nan, W: 1}} }),
		"chambers many":    mut(func(s *Sketch) { s.Case.Chambers = make([]Chamber, 5) }),
		"orientation side": mut(func(s *Sketch) { s.Case.Orientation.SideViewLeft = "top" }),
		"orientation pane": mut(func(s *Sketch) { s.Case.Orientation.GlassSide = "middle" }),
		"canvas view":      mut(func(s *Sketch) { s.CanvasView = "back" }),
		"id empty":         mut(func(s *Sketch) { s.Parts[0].ID = "" }),
		"id upper":         mut(func(s *Sketch) { s.Parts[0].ID = "P1" }),
		"id long":          mut(func(s *Sketch) { s.Parts[0].ID = strings.Repeat("a", 58) }),
		"part":             mut(func(s *Sketch) { s.Parts[0].Part = "nope" }),
		"facing":           mut(func(s *Sketch) { s.Parts[0].Facing = "x" }),
		"facing empty":     mut(func(s *Sketch) { s.Parts[0].Facing = "" }),
		"flow":             mut(func(s *Sketch) { s.Parts[0].Flow = "sideways" }),
		"x nan":            mut(func(s *Sketch) { s.Parts[0].Pos.X = nan }),
		"y inf":            mut(func(s *Sketch) { s.Parts[0].Pos.Y = inf }),
		"z nan":            mut(func(s *Sketch) { s.Parts[0].Pos.Z = nan }),
		"spin nan":         mut(func(s *Sketch) { s.Parts[0].Spin = nan }),
		"spin range":       mut(func(s *Sketch) { s.Parts[0].Spin = 400 }),
		"x outside":        mut(func(s *Sketch) { s.Parts[0].Pos.X = 230 + MarginMM + 100 }),
		"x negative":       mut(func(s *Sketch) { s.Parts[0].Pos.X = -MarginMM - 100 }),
		"y outside":        mut(func(s *Sketch) { s.Parts[0].Pos.Y = 480 + MarginMM + 100 }),
		"z outside":        mut(func(s *Sketch) { s.Parts[0].Pos.Z = 450 + MarginMM + 100 }),
		"z negative":       mut(func(s *Sketch) { s.Parts[0].Pos.Z = -MarginMM - 200 }),
		"leds low":         mut(func(s *Sketch) { s.Parts[0].LEDs = 2 }),
		"leds high":        mut(func(s *Sketch) { s.Parts[0].LEDs = 65 }),
		"leds negative":    mut(func(s *Sketch) { s.Parts[0].LEDs = -1 }),
		"leds fixed":       mut(func(s *Sketch) { s.Parts[0].Part = "generic-strip"; s.Parts[0].LEDs = 301 }),
		"len on fan":       mut(func(s *Sketch) { s.Parts[0].Len = 100 }),
		"len nan":          mut(func(s *Sketch) { s.Parts[0].Part = "generic-strip"; s.Parts[0].Len = nan }),
		"label long":       mut(func(s *Sketch) { s.Parts[0].Label = strings.Repeat("x", 41) }),
		"label newline":    mut(func(s *Sketch) { s.Parts[0].Label = "a\nb" }),
		"label control":    mut(func(s *Sketch) { s.Parts[0].Label = "a\x00" }),
		"label utf8":       mut(func(s *Sketch) { s.Parts[0].Label = "\xff" }),
		"bind no zone":     mut(func(s *Sketch) { s.Parts[0].Bind.Zone = "" }),
		"bind device":      mut(func(s *Sketch) { s.Parts[0].Bind.Device = "a\tb" }),
		"bind order":       mut(func(s *Sketch) { s.Parts[0].Bind.Order = -1 }),
		"bind order big":   mut(func(s *Sketch) { s.Parts[0].Bind.Order = 1001 }),
		"duplicate id":     mut(func(s *Sketch) { s.Parts = append(s.Parts, s.Parts[0]) }),
		"group upper":      mut(func(s *Sketch) { s.Parts[0].Group = "Grp" }),
		"group long":       mut(func(s *Sketch) { s.Parts[0].Group = strings.Repeat("g", 17) }),
		"group space":      mut(func(s *Sketch) { s.Parts[0].Group = "a b" }),
		"layer high":       mut(func(s *Sketch) { s.Parts[0].Layer = MaxLayer + 1 }),
		"layer low":        mut(func(s *Sketch) { s.Parts[0].Layer = -MaxLayer - 1 }),
		"on unknown":       mut(func(s *Sketch) { s.Parts[0].On = "ghost" }),
		"on self":          mut(func(s *Sketch) { s.Parts[0].On = "p1" }),
		"on chain": mut(func(s *Sketch) {
			b, c := s.Parts[0], s.Parts[0]
			b.ID, b.On, b.Bind = "p2", "p1", nil
			c.ID, c.On, c.Bind = "p3", "p2", nil
			s.Parts[0].Bind = nil
			s.Parts = append(s.Parts, b, c)
		}),
	}
	for name, s := range cases {
		if err := Validate(s, c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// one level of mounting is fine
	m := ok()
	kid := m.Parts[0]
	kid.ID, kid.On, kid.Bind, kid.Hidden = "kid", "p1", nil, true
	m.Parts = append(m.Parts, kid)
	if err := Validate(m, c); err != nil {
		t.Errorf("mounted part: %v", err)
	}
	// exactly 64 parts is fine, 65 is not
	many := Sketch{Version: Version, Case: box()}
	for i := 0; i < 65; i++ {
		p := flatSide("p"+string(rune('a'+i/26))+string(rune('a'+i%26)), "generic-fan-120", 0, 0, 4, 0)
		many.Parts = append(many.Parts, p)
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
	big := Sketch{Version: Version, Case: box()}
	for i := 0; i < 14; i++ {
		big.Parts = append(big.Parts, flatSide("s"+string(rune('a'+i)), "generic-strip", 0, 0, 300, 0))
	}
	if err := Validate(big, c); err == nil {
		t.Error("LED total cap not enforced")
	}
	// valid edge values
	e := ok()
	e.Parts[0].Spin, e.Parts[0].Pos.X, e.Parts[0].LEDs, e.Parts[0].Layer = -360, -MarginMM, 64, -MaxLayer
	e.Case.Orientation = Orientation{SideViewLeft: "front", GlassSide: "right"}
	if err := Validate(e, c); err != nil {
		t.Errorf("edge values: %v", err)
	}
	// nil catalog maps do not panic
	if err := Validate(ok(), Catalog{}); err == nil {
		t.Error("empty catalog accepted a part")
	}
}

func TestDecode(t *testing.T) {
	good := `{"version":2,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450},"chambers":[{"x":0,"w":150}],
	"orientation":{"side_view_left":"rear","glass_side":"left"}},
	"canvasView":"side","parts":[{"id":"p1","part":"generic-fan-120","pos":{"x":12.5,"y":120,"z":100},"facing":"side","spin":0,
	"mirror":false,"flow":"intake","leds":16,"label":"","bind":{"device":"d","zone":"argb1","order":0,"parallel":false},
	"on":"","group":"g1","layer":2,"hidden":true}]}`
	s, err := Decode([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(s, Builtin()); err != nil {
		t.Fatal(err)
	}
	if p := s.Parts[0]; p.Pos.Z != 100 || p.Facing != "side" || p.Group != "g1" || p.Layer != 2 || !p.Hidden || s.Case.Orientation.GlassSide != "left" {
		t.Fatalf("decoded %+v", s)
	}
	// round trip, also through the Sketch unmarshaller
	raw, _ := json.Marshal(s)
	s2, err := Decode(raw)
	if err != nil || Validate(s2, Builtin()) != nil {
		t.Fatalf("round trip: %v", err)
	}
	var s3 Sketch
	if err := json.Unmarshal(raw, &s3); err != nil || len(s3.Parts) != 1 || s3.Parts[0].Pos != s.Parts[0].Pos {
		t.Fatalf("unmarshal: %v %+v", err, s3)
	}
	for _, bad := range []string{``, `{`, good + `{}`, strings.Replace(good, `"version"`, `"extra":1,"version"`, 1), `[]`, `{"version":"2"}`,
		strings.Replace(good, `"facing":"side"`, `"facing":"side","bogus":1`, 1),
		`{"version":2,"parts":[{"pos":{"x":1e999}}]}`, strings.Repeat(" ", MaxBodyBytes+1)} {
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
	s.Parts[2].Facing = FacingTop // edge-on in the side view
	d, err := Derive(s, Builtin())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.Components {
		if err := leds.RegisterComponent(c); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
		leds.UnregisterComponent(c.Name)
		for _, p := range c.LEDs {
			if p.X < 0 || p.Y < 0 || p.X > c.SizeMM.X || p.Y > c.SizeMM.Y {
				t.Errorf("%s: LED %v outside %v", c.Name, p, c.SizeMM)
			}
		}
	}
}
