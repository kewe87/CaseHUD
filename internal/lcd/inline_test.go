package lcd

import (
	"image"
	"math"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

type constProv struct{ v float64 }

func (constProv) Refresh() error         { return nil }
func (constProv) All() []metrics.Reading { return nil }
func (p constProv) Get(s string) (metrics.Reading, bool) {
	return metrics.Reading{Key: s, Path: s, Value: p.v}, true
}

// An inline icon forms one centred group with each of the three texts,
// whatever the number says, and stays out of the way when it cannot be placed.
func TestInlineGroupCentred(t *testing.T) {
	for _, target := range []string{"main", "sub", "label"} {
		for _, host := range []string{"value", "gauge"} {
			for _, v := range []float64{0, 7, 98, 980, 2400, 3500} {
				l := Layout{Width: 480, Height: 480, IntervalMS: 1000, HistorySize: 2, Set: "clean",
					Background: "#000000", Foreground: "#f1f4fa", Muted: "#8a94a8", Accent: "#22d3ee",
					Widgets: []Widget{
						{Type: host, Align: "center", X: 40, Y: 40, W: 400, H: 400, Label: "PUMP", Source: "a", Unit: " rpm", SubSource: "a", SubUnit: " rpm"},
						{Type: "icon", Icon: "pump", Inline: target, Source: "a", Max: 3500},
					}}
				r, err := NewRenderer(l)
				if err != nil {
					t.Fatal(err)
				}
				r.SplitIcons = true
				r.Render(constProv{v})
				gs := r.InlineGroups()
				if len(gs) != 1 {
					t.Fatalf("%s/%s: %d groups", target, host, len(gs))
				}
				g := gs[0]
				if d := float64(g.Icon.Min.X+g.Text.Max.X)/2 - 240; math.Abs(d) > 1.5 {
					t.Errorf("%s/%s/%v: group %d..%d off centre by %.1f", target, host, v, g.Icon.Min.X, g.Text.Max.X, d)
				}
				if gap := float64(g.Text.Min.X - g.Icon.Max.X); math.Abs(gap-0.2*float64(g.Icon.Dx())) > 1.5 {
					t.Errorf("%s/%s/%v: gap %v for icon %v", target, host, v, gap, g.Icon)
				}
				if g.Icon.Dx() < 40 || !g.Icon.In(image.Rect(0, 0, 480, 480)) {
					t.Errorf("%s/%s/%v: icon %v", target, host, v, g.Icon)
				}
			}
		}
	}
}

// Before the HUD has been drawn an inline icon has no place and is skipped.
func TestInlineNotPlacedBeforeDraw(t *testing.T) {
	l := Layout{Width: 100, Height: 100, IntervalMS: 1000, HistorySize: 2,
		Widgets: []Widget{{Type: "value", Align: "center", W: 100, H: 100}, {Type: "icon", Icon: "pump", Inline: "main"}}}
	r, err := NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.IconRects()) != 0 {
		t.Error("rect before first frame")
	}
	r.DrawIcons(image.NewRGBA(image.Rect(0, 0, 100, 100)), constProv{1})
}
