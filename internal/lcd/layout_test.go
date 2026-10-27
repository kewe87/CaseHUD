package lcd

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	l := Layout{Width: 480, Height: 480, HistorySize: 1e6, Widgets: []Widget{{Type: "gauge", W: 100, H: 100, Segments: 2e9, Ticks: -5, Size: 1e7, Text: strings.Repeat("x", 5000)}}}
	Sanitize(&l)
	w := l.Widgets[0]
	if w.Segments != maxSegments || w.Ticks != 0 || w.Size != 0 || len(w.Text) != maxTextRunes || l.HistorySize != maxHistorySize {
		t.Fatalf("not sanitized: %+v history %d", w, l.HistorySize)
	}
}

func TestFitToKeepsAspect(t *testing.T) {
	l := DefaultLayout()
	l.Widgets = []Widget{{Type: "text", X: 0, Y: 0, W: 480, H: 480, Size: 100}}
	// Same aspect ratio: plain scaling.
	if g := l.FitTo(400, 400); g.Widgets[0].W != 400 || g.Widgets[0].X != 0 {
		t.Fatalf("square: %+v", g.Widgets[0])
	}
	// A square design on the 1920x480 bar: not stretched, centred.
	g := l.FitTo(1920, 480)
	w := g.Widgets[0]
	if g.Width != 1920 || g.Height != 480 || w.W != 480 || w.H != 480 || w.X != 720 || w.Y != 0 || w.Size != 100 {
		t.Fatalf("bar: %dx%d widget %+v", g.Width, g.Height, w)
	}
	// And the other way round: a bar design on a square screen is letterboxed.
	b := DefaultLayout()
	b.Width, b.Height = 1920, 480
	b.Widgets = []Widget{{Type: "text", X: 0, Y: 0, W: 1920, H: 480}}
	s := b.FitTo(400, 400).Widgets[0]
	if s.W != 400 || s.H != 100 || s.Y != 150 {
		t.Fatalf("letterbox: %+v", s)
	}
}
