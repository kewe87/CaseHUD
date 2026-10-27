package leds

import (
	"math"
	"testing"
)

func TestComponentSegment(t *testing.T) {
	z := Zone{ID: "argb1", Resizable: true, Max: 120}
	zc := ZoneConfig{Segments: []Segment{
		{Shape: "component", Component: "fan-tl-26", X: 0, Y: 0, W: 0.26, H: 0.26},
		{Shape: "component", Component: "fan-tl-26", X: 0.5, Y: 0.5, W: 0.26, H: 0.26, Angle: 90},
	}}
	if n := LEDCount(z, zc); n != 52 {
		t.Fatalf("count %d", n)
	}
	p := Positions(z, zc)
	// LED 0 of the first fan: bottom bar, left end -> lower left of its box
	if p[0].X > 0.05 || p[0].Y < 0.2 {
		t.Fatalf("LED 0 at %v", p[0])
	}
	// LED 13 (top bar, left end) sits above LED 0
	if math.Abs(p[13].X-p[0].X) > 1e-9 || p[13].Y > 0.05 {
		t.Fatalf("LED 13 at %v", p[13])
	}
	// rotated 90° clockwise around (0.63, 0.63): lower left becomes upper left
	if q := p[26]; q.X > 0.63 || q.Y > 0.63 {
		t.Fatalf("rotated LED 0 at %v", q)
	}
	if n := LEDCount(z, ZoneConfig{Segments: []Segment{{Shape: "component", Component: "missing"}}}); n != 0 {
		t.Fatalf("unknown component counts %d", n)
	}
}

func TestRectSegment(t *testing.T) {
	z := Zone{ID: "ring", Count: 8, Max: 8}
	seg := Segment{Shape: "rect", Count: 8, X: 0, Y: 0, W: 0.4, H: 0.2}
	p := Positions(z, ZoneConfig{Segments: []Segment{seg}})
	// perimeter 1.2, step 0.15, clockwise from the top centre (0.2, 0)
	want := []Point{{0.2, 0}, {0.35, 0}, {0.4, 0.1}, {0.35, 0.2}, {0.2, 0.2}, {0.05, 0.2}, {0, 0.1}, {0.05, 0}}
	for i := range want {
		if math.Abs(p[i].X-want[i].X) > 1e-9 || math.Abs(p[i].Y-want[i].Y) > 1e-9 {
			t.Fatalf("LED %d at %v, want %v", i, p[i], want[i])
		}
	}
	seg.Reverse = true // counter-clockwise from the same first LED
	r := Positions(z, ZoneConfig{Segments: []Segment{seg}})
	if r[0] != p[0] || math.Abs(r[1].X-p[7].X) > 1e-9 || math.Abs(r[1].Y-p[7].Y) > 1e-9 {
		t.Fatalf("reverse: %v", r)
	}
}

func TestComponentMM(t *testing.T) {
	c, _ := LookupComponent("fan-tl-26")
	mm := c.LEDsMM()
	if len(mm) != 26 || mm[0].X < 7 || mm[0].X > 9 || mm[0].Y < 113 || mm[13].Y > 8 {
		t.Fatalf("mm %v %v", mm[0], mm[13])
	}
	s, _ := LookupComponent("strip-47")
	if s.LEDsMM() != nil {
		t.Fatal("unknown size gave positions")
	}
}
