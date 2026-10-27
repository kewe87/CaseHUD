package app

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

// fixedProv answers every source with a value chosen per source by val.
type fixedProv struct{ val func(src string) float64 }

func (fixedProv) Refresh() error         { return nil }
func (fixedProv) All() []metrics.Reading { return nil }
func (p fixedProv) Get(src string) (metrics.Reading, bool) {
	return metrics.Reading{Key: src, Path: src, Value: p.val(src)}, true
}

// valuesFor makes a provider: pump rpm, power and load/temperature each set
// to one of a few magnitudes.
func valuesFor(rpm, small, big float64) fixedProv {
	return fixedProv{func(src string) float64 {
		switch {
		case strings.Contains(src, "Pump") || src == srcPump:
			return rpm
		case strings.Contains(src, "Power") || strings.Contains(src, "Clock"):
			return big
		}
		return small
	}}
}

func lum(c color.RGBA) float64 {
	return 0.3*float64(c.R) + 0.59*float64(c.G) + 0.11*float64(c.B)
}

// inkBox is the bounding box of the pixels in r that differ from ref.
func inkBox(img *image.RGBA, r image.Rectangle, ref color.RGBA) (image.Rectangle, bool) {
	r = r.Intersect(img.Bounds())
	box := image.Rectangle{Min: r.Max, Max: r.Min}
	found := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if math.Abs(lum(img.RGBAAt(x, y))-lum(ref)) > 70 {
				found = true
				box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
				box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
			}
		}
	}
	return box, found
}

type inlineCase struct {
	id   string
	l    lcd.Layout // text only: no rings, bars, graphs or cards
	icon int        // widget index of the inline icon
	host lcd.Widget
}

// textOnly drops everything that is not text, so the pixels left are glyphs.
func textOnly(l lcd.Layout) lcd.Layout {
	var ws []lcd.Widget
	for _, w := range l.Widgets {
		switch {
		case w.Type == "box" || w.Type == "bar" || w.Type == "graph" || (w.Type == "gauge" && w.RingOnly):
			continue
		case w.Type == "gauge":
			w.Thickness, w.Ticks, w.Segments, w.Glow = 0.0001, 0, 0, false
		}
		ws = append(ws, w)
	}
	l.Widgets = ws
	return l
}

// inlineCases lists every inline icon of the built-in designs.
func inlineCases() []inlineCase {
	var out []inlineCase
	all := builtinDesigns()
	for id := range wideParts {
		if l, ok := uprightDesign(id); ok {
			all[id+"-upright"] = l
		}
	}
	if l, ok := uprightDesign("water-wide"); ok {
		all["water-wide-upright"] = l
	}
	for id, l := range all {
		l = textOnly(l)
		host := -1
		for i, w := range l.Widgets {
			switch {
			case w.Type == "value" || w.Type == "gauge":
				host = i
			case w.Type == "icon" && w.Inline != "" && host >= 0:
				out = append(out, inlineCase{id: id, l: l, icon: i, host: l.Widgets[host]})
			}
		}
	}
	return out
}

// TestInlineIcons renders every design with an inline icon with short, medium
// and long values and checks, per icon: the icon and its text form a group
// centred on the widget's axis (+-2 px), a gap of 0.2 x the icon apart, the
// icon is centred on the text's height, and nothing but that text comes
// within 16 px of the icon (above and below only counts where the icon
// reaches beyond the text's height).
func TestInlineIcons(t *testing.T) {
	cases := inlineCases()
	if len(cases) < 150 {
		t.Fatalf("only %d inline icons", len(cases))
	}
	provs := map[string]fixedProv{"short": valuesFor(980, 5, 9), "mid": valuesFor(1072, 58, 263), "long": valuesFor(3500, 100, 450)}
	for _, c := range cases {
		for name, p := range provs {
			l := c.l
			l.Lang = "en"
			r, err := lcd.NewRenderer(l)
			if err != nil {
				t.Fatal(err)
			}
			r.SplitIcons = true
			img := r.Render(p)
			var g lcd.InlineGroup
			found := false
			for _, x := range r.InlineGroups() {
				if x.Widget == c.icon {
					g, found = x, true
				}
			}
			tag := c.id + "/" + c.l.Widgets[c.icon].Inline + "/" + name
			if !found {
				t.Errorf("%s: icon not placed", tag)
				continue
			}
			sq, tx := g.Icon, g.Text
			size := float64(sq.Dx())
			if sq.Dx() < 44 || sq.Dx() != sq.Dy() {
				t.Errorf("%s: icon %v too small or not square", tag, sq)
			}
			if !sq.In(img.Bounds()) {
				t.Errorf("%s: icon %v outside the display", tag, sq)
				continue
			}
			if d := float64(sq.Min.X+tx.Max.X)/2 - g.Axis; math.Abs(d) > 2 {
				t.Errorf("%s: group %d..%d off its axis %.1f by %.1f px", tag, sq.Min.X, tx.Max.X, g.Axis, d)
			}
			if gap := float64(tx.Min.X - sq.Max.X); math.Abs(gap-0.2*size) > 2 {
				t.Errorf("%s: gap %.0f px, want %.0f (0.2 x icon %.0f)", tag, gap, 0.2*size, size)
			}
			if d := float64(sq.Min.Y+sq.Max.Y)/2 - float64(tx.Min.Y+tx.Max.Y)/2; math.Abs(d) > 2 {
				t.Errorf("%s: icon centre %.1f px off the text height", tag, d)
			}
			// the drawn glyphs
			ref := img.RGBAAt((sq.Min.X+sq.Max.X)/2, (sq.Min.Y+sq.Max.Y)/2)
			zone := sq.Inset(-16)
			if b, ok := inkBox(img, image.Rect(zone.Min.X, zone.Min.Y, sq.Min.X, zone.Max.Y), ref); ok {
				t.Errorf("%s: ink %v left of icon %v within 16 px", tag, b, sq)
			}
			if sq.Min.Y < tx.Min.Y-1 {
				if b, ok := inkBox(img, image.Rect(sq.Min.X, zone.Min.Y, sq.Max.X, sq.Min.Y), ref); ok {
					t.Errorf("%s: ink %v above icon %v within 16 px", tag, b, sq)
				}
			}
			if sq.Max.Y > tx.Max.Y+1 {
				if b, ok := inkBox(img, image.Rect(sq.Min.X, sq.Max.Y, sq.Max.X, zone.Max.Y), ref); ok {
					t.Errorf("%s: ink %v below icon %v within 16 px", tag, b, sq)
				}
			}
			// the text really is where the group says
			ink, ok := inkBox(img, image.Rect(sq.Max.X, tx.Min.Y-2, tx.Max.X+10, tx.Max.Y+2), ref)
			if !ok || ink.Min.X < tx.Min.X-4 || ink.Min.X > tx.Min.X+int(0.25*size)+8 {
				t.Errorf("%s: text ink %v not at %v", tag, ink, tx)
			}
		}
	}
}
