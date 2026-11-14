package sketch

import (
	"math"
	"testing"
)

// FuzzDecodeValidate feeds arbitrary bytes through Decode, Validate and, for
// valid sketches, Derive and Overlaps. Nothing may panic and a valid sketch
// must derive finite, in-box LED positions. Version 1 and version 2 documents
// are both seeded.
func FuzzDecodeValidate(f *testing.F) {
	f.Add([]byte(`{"version":2,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450},"orientation":{"glass_side":"right"}},"canvasView":"top","parts":[{"id":"p1","part":"generic-fan-120","pos":{"x":40,"y":60,"z":10},"facing":"top","spin":90,"mirror":true,"flow":"exhaust","leds":16,"label":"x","bind":{"device":"d","zone":"z","order":0,"parallel":true},"on":"p2","group":"a","layer":3},{"id":"p2","part":"generic-strip","pos":{"x":1,"y":2,"z":3},"facing":"rear"}]}`))
	f.Add([]byte(`{"version":2,"case":{"source":"library","id":"abc","dims":{"w":1,"h":1,"d":1},"chambers":[{"x":0,"w":1}]},"parts":[{"id":"a","part":"generic-strip","facing":"bottom","len":20,"leds":300}]}`))
	f.Add([]byte(`{"version":1,"case":{"source":"custom","id":"","dims":{"w":230,"h":480,"d":450}},"canvasView":"top","parts":[{"id":"p1","part":"generic-fan-120","view":"side","x":40,"y":60,"rot":90,"face":"top","mirror":true,"flow":"exhaust","leds":16,"label":"x","bind":{"device":"d","zone":"z","order":0,"parallel":true}}]}`))
	f.Add([]byte(`{"version":1,"case":{"source":"library","id":"abc","dims":{"w":1,"h":1,"d":1}},"parts":[{"id":"a","part":"generic-strip","len":20,"leds":300},{"id":"b","part":"nope","view":"top","face":"side"}]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"version":1,"parts":[{"x":1e308,"rot":-1e308}]}`))
	f.Add([]byte(`{"version":2,"parts":[{"pos":{"x":1e308},"spin":-1e308}]}`))
	cat := Builtin()
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := Decode(data)
		if err != nil {
			return
		}
		if Validate(s, cat) != nil {
			return
		}
		d, err := Derive(s, cat)
		if err != nil {
			t.Fatalf("valid sketch does not derive: %v", err)
		}
		_ = Overlaps(s, cat)
		for _, c := range d.Components {
			for _, p := range c.LEDs {
				if math.IsNaN(p.X) || math.IsNaN(p.Y) || p.X < 0 || p.Y < 0 || p.X > c.SizeMM.X || p.Y > c.SizeMM.Y {
					t.Fatalf("LED %v outside %v", p, c.SizeMM)
				}
			}
		}
		for _, z := range d.Zones {
			for _, sg := range z.Segments {
				if math.IsNaN(sg.X+sg.Y+sg.W+sg.H) || math.IsInf(sg.X+sg.Y+sg.W+sg.H, 0) || sg.Start == nil || *sg.Start < 0 {
					t.Fatalf("bad segment %+v", sg)
				}
			}
		}
	})
}

// FuzzPartParse checks that ParsePart never panics and that accepted parts
// project without error.
func FuzzPartParse(f *testing.F) {
	f.Add([]byte(`{"format":1,"id":"a","dimensions_mm":{"w":10,"h":10,"d":5},"leds":{"positions_mm":[[1,1]]}}`))
	f.Add([]byte(`{"format":1,"id":"a","dimensions_mm":{"w":10,"h":10,"d":5},"leds":{"generator":{"kind":"ring","min":1,"max":8,"default":4,"radius":3}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := ParsePart(data)
		if err != nil {
			return
		}
		if _, err := Project3D(p, Placement{ID: "a", Facing: FacingSide}); err != nil {
			t.Fatalf("accepted part does not project: %v", err)
		}
	})
}

// FuzzParseCase checks that ParseCase never panics and that an accepted
// template has a valid orientation.
func FuzzParseCase(f *testing.F) {
	f.Add([]byte(goodCase))
	f.Add([]byte(`{"format":1,"id":"c","model":"M","dimensions_mm":{"w":1,"h":1,"d":1},"orientation":{"side_view_left":"front","glass_side":"right"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := ParseCase(data)
		if err != nil {
			return
		}
		if err := c.Orientation.validate(); err != nil {
			t.Fatal(err)
		}
	})
}
