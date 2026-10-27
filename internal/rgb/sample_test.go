package leds

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// golden canvas: four vertical color bands
func bands() *image.RGBA {
	c := image.NewRGBA(image.Rect(0, 0, CanvasWidth, CanvasHeight))
	cols := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	for x := 0; x < CanvasWidth; x++ {
		col := cols[x*4/CanvasWidth]
		for y := 0; y < CanvasHeight; y++ {
			set(c, x, y, col)
		}
	}
	return c
}

func TestSampleGolden(t *testing.T) {
	pts := []Point{{0, 0.5}, {0.375, 0.5}, {0.625, 0.5}, {1, 1}, {0.5, 0.5}}
	want := []color.RGBA{
		{255, 0, 0, 255},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{255, 255, 255, 255},
		// x=160 lies on the green/blue border: 5x5 box, 2 green + 3 blue columns
		{0, 102, 153, 255},
	}
	got := make([]color.RGBA, len(pts))
	Sample(bands(), pts, got)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d %v: got %v, want %v", i, pts[i], got[i], want[i])
		}
	}
}

func TestReorderAndScale(t *testing.T) {
	cols := []color.RGBA{{10, 20, 30, 255}}
	reorder(cols, "GRB")
	if cols[0] != (color.RGBA{20, 10, 30, 255}) {
		t.Fatalf("GRB: %v", cols[0])
	}
	cols = []color.RGBA{{200, 100, 50, 255}}
	scale(cols, 128)
	if cols[0] != (color.RGBA{100, 50, 25, 255}) {
		t.Fatalf("scale: %v", cols[0])
	}
}

func TestPositions(t *testing.T) {
	z := Zone{ID: "argb1", Resizable: true, Max: 120}
	zc := ZoneConfig{Segments: []Segment{
		{Count: 3, Shape: "line", X: 0, Y: 0.5, W: 1, H: 0},
		{Count: 4, Shape: "ring", X: 0.25, Y: 0.25, W: 0.5, H: 0.5},
	}}
	pts := Positions(z, zc)
	if len(pts) != 7 || LEDCount(z, zc) != 7 {
		t.Fatalf("got %d points", len(pts))
	}
	want := []Point{{0, 0.5}, {0.5, 0.5}, {1, 0.5}, {0.5, 0.25}, {0.75, 0.5}, {0.5, 0.75}, {0.25, 0.5}}
	for i, w := range want {
		if math.Abs(pts[i].X-w.X) > 1e-9 || math.Abs(pts[i].Y-w.Y) > 1e-9 {
			t.Errorf("point %d: got %v, want %v", i, pts[i], w)
		}
	}
	// limit
	zc.Segments[0].Count = 200
	if n := len(Positions(z, zc)); n != 120 {
		t.Errorf("limit: got %d", n)
	}
	// default layout for fixed zones
	if n := len(Positions(Zone{Count: 5, Max: 5}, ZoneConfig{})); n != 5 {
		t.Errorf("default layout: got %d", n)
	}
}

// Parts on a splitter: every segment starts at LED 0, the zone is as long as
// the longest part, the first segment places the shared LEDs.
func TestParallelSegments(t *testing.T) {
	z := Zone{ID: "argb3", Resizable: true, Max: 120}
	zero := 0
	zc := ZoneConfig{Segments: []Segment{
		{Count: 18, Shape: "line", X: 0, Y: 0.1, W: 1, H: 0},
		{Count: 11, Shape: "line", X: 0, Y: 0.5, W: 1, H: 0, Start: &zero},
		{Count: 8, Shape: "line", X: 0, Y: 0.9, W: 1, H: 0, Start: &zero},
		{Count: 2, Shape: "point", X: 0.5, Y: 0.5}, // continues after the last one: LEDs 8, 9
	}}
	pts := Positions(z, zc)
	if len(pts) != 18 || LEDCount(z, zc) != 18 {
		t.Fatalf("got %d points, count %d", len(pts), LEDCount(z, zc))
	}
	for i, p := range pts {
		if p.Y != 0.1 {
			t.Fatalf("LED %d placed by a later segment: %v", i, p)
		}
	}
	gap := ZoneConfig{Segments: []Segment{{Count: 2, Shape: "point", X: 0.2, Y: 0.2, Start: new(int)}, {Count: 1, Shape: "point", X: 0.8, Y: 0.8, Start: &[]int{4}[0]}}}
	if p := Positions(z, gap); len(p) != 5 || p[3].X != 0.8 || LEDCount(z, gap) != 5 {
		t.Fatalf("gap: %v", p)
	}
}

func TestHSV(t *testing.T) {
	for _, tc := range []struct {
		h    float64
		want color.RGBA
	}{{0, color.RGBA{255, 0, 0, 255}}, {120, color.RGBA{0, 255, 0, 255}}, {240, color.RGBA{0, 0, 255, 255}}, {-120, color.RGBA{0, 0, 255, 255}}} {
		if got := HSV(tc.h, 1, 1); got != tc.want {
			t.Errorf("HSV(%v): got %v", tc.h, got)
		}
	}
}
