package app

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/lcd"
)

// worstCase answers every source with the widest reading it can show, so a
// cell that would have to shrink or spill shows it.
func worstCase() fixedProv {
	return fixedProv{func(src string) float64 {
		switch {
		case strings.Contains(src, "Pump"):
			return 3500
		case strings.Contains(src, " + "):
			return 700
		case strings.Contains(src, "Clock"):
			return 5800
		case strings.Contains(src, "Power"):
			return 450
		case strings.Contains(src, "Data/"):
			return 64
		case strings.Contains(src, "Frametime"):
			return 16.5
		case strings.Contains(src, "Latency"):
			return 60
		case strings.Contains(src, "Framerate"):
			return 144
		case strings.Contains(src, "Temperatures"):
			return 99.9
		}
		return 100
	}}
}

// statsOf renders l and returns the layout of its uniform value cells.
func statsOf(t *testing.T, id string, l lcd.Layout) []lcd.ValueStat {
	t.Helper()
	r, err := lcd.NewRenderer(l)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	r.SplitIcons = true
	r.Render(worstCase())
	var out []lcd.ValueStat
	for _, s := range r.ValueStats() {
		if l.Widgets[s.Widget].Uniform {
			out = append(out, s)
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.6 }

// TestHalfCellsAreUniform: every cell of the half fields (the combined ones
// and the single readings) of every style has the same label size and
// baseline, the same digit height and baseline and the same bar; the two
// columns of a combined cell are equal and symmetric; every combined cell has
// exactly one icon, in front of its first label; nothing shrinks or spills.
func TestHalfCellsAreUniform(t *testing.T) {
	all := builtinDesigns()
	for _, s := range tileStyles {
		var ref *lcd.ValueStat
		check := func(id string, st lcd.ValueStat) {
			if ref == nil {
				ref = &st
			}
			if !near(st.LabelSize, cellLabelSize) {
				t.Errorf("%s: label size %.1f, want %.0f", id, st.LabelSize, cellLabelSize)
			}
			if !near(st.LabelBase, ref.LabelBase) {
				t.Errorf("%s: label baseline %.1f, others %.1f", id, st.LabelBase, ref.LabelBase)
			}
			if !near(st.ValueSize, ref.ValueSize) || !near(st.ValueBase, ref.ValueBase) || !near(st.DigitH, ref.DigitH) {
				t.Errorf("%s: number %.1f px (digits %.1f) at %.1f, others %.1f (%.1f) at %.1f",
					id, st.ValueSize, st.DigitH, st.ValueBase, ref.ValueSize, ref.DigitH, ref.ValueBase)
			}
			if st.DigitH < 60 || st.LabelSize < 28 {
				t.Errorf("%s: digits %.1f px / label %.1f px below the readable minimum", id, st.DigitH, st.LabelSize)
			}
			if st.LabelW > float64(st.Rect.Dx())-2 {
				t.Errorf("%s: label %q is %.0f px wide in a %d px column", id, st.Label, st.LabelW, st.Rect.Dx())
			}
			if st.ValueW > float64(st.Rect.Dx()) {
				t.Errorf("%s: number %.0f px wide in a %d px column", id, st.ValueW, st.Rect.Dx())
			}
			if len([]rune(st.Label)) > 9 {
				t.Errorf("%s: label %q longer than 9 characters", id, st.Label)
			}
		}
		for _, k := range tileKinds {
			if k == "clock" {
				continue
			}
			id := s.prefix + "half-" + k
			sts := statsOf(t, id, all[id])
			if len(sts) != 1 {
				t.Errorf("%s: %d cells", id, len(sts))
				continue
			}
			check(id, sts[0])
		}
		for _, d := range comboDefs() {
			if d.layout != "half" {
				continue
			}
			id := comboID(s, d)
			l := all[id]
			sts := statsOf(t, id, l)
			if len(sts) != 2 {
				t.Errorf("%s: %d cells, want 2", id, len(sts))
				continue
			}
			for i, st := range sts {
				check(fmt.Sprintf("%s[%d]", id, i), st)
				if c := halfCols[i]; st.Rect.Min.X != c.x || st.Rect.Dx() != c.w {
					t.Errorf("%s[%d]: column %v, want x=%d w=%d", id, i, st.Rect, c.x, c.w)
				}
			}
			if a, b := sts[0].Rect, sts[1].Rect; a.Dx() != b.Dx() || a.Min.X+b.Max.X != 480 {
				t.Errorf("%s: columns %v %v not equal and symmetric", id, a, b)
			}
			// exactly one icon, inline in front of the first label; two bars
			icons, bars := 0, 0
			for i, w := range l.Widgets {
				switch w.Type {
				case "icon":
					icons++
					if w.Inline != "label" || l.Widgets[i-1].Type != "value" || l.Widgets[i-1].X != halfCols[0].x {
						t.Errorf("%s: icon is not inline on the first label", id)
					}
					if want := cellIconSizes[w.Icon]; w.W != want || w.H != want {
						t.Errorf("%s: icon %dx%d, want %d", id, w.W, w.H, want)
					}
				case "bar":
					bars++
					if w.Y != cellBarY || w.H != cellBarH || w.W != halfCols[0].barW || (w.X != halfCols[0].barX && w.X != halfCols[1].barX) {
						t.Errorf("%s: bar %d,%d %dx%d off the grid", id, w.X, w.Y, w.W, w.H)
					}
				}
			}
			if icons != 1 || bars != 2 {
				t.Errorf("%s: %d icons and %d bars, want 1 and 2", id, icons, bars)
			}
		}
	}
	// the two bars are symmetric about the middle of the card
	if a, b := halfCols[0], halfCols[1]; a.barX+b.barX+b.barW != 480 {
		t.Errorf("bars at %d and %d not symmetric", a.barX, b.barX)
	}
}

// TestCardCellsAreUniform: the same for the squares and the value cards of
// the wide fields: label size, second value size and digit height are the
// same in every cell of a kind, in every style, and nothing spills.
func TestCardCellsAreUniform(t *testing.T) {
	all := builtinDesigns()
	var ids []string
	for _, s := range tileStyles {
		for _, k := range cardKinds {
			ids = append(ids, s.prefix+"card-"+k)
		}
		for _, d := range comboDefs() {
			if d.layout != "half" {
				ids = append(ids, comboID(s, d))
			}
		}
		for _, k := range wideKinds {
			for _, tag := range []string{"w640", "w960", "h640", "h960"} {
				ids = append(ids, fmt.Sprintf("%s%s-%s", s.prefix, tag, k))
			}
		}
	}
	digits := map[float64]float64{} // number font size -> digit height
	n := 0
	for _, id := range ids {
		l, ok := all[id]
		if !ok {
			t.Errorf("design %s missing", id)
			continue
		}
		for _, st := range statsOf(t, id, l) {
			if l.Widgets[st.Widget].Size == cellLabelSize {
				continue // a half-field cell inside a card: see TestHalfCellsAreUniform
			}
			n++
			if st.Label != "" && !near(st.LabelSize, cardLabelSize) {
				t.Errorf("%s: label %q is %.1f px, want %.0f", id, st.Label, st.LabelSize, cardLabelSize)
			}
			if st.SubSize != 0 && !near(st.SubSize, cardLabelSize) {
				t.Errorf("%s: second value %.1f px, want %.0f", id, st.SubSize, cardLabelSize)
			}
			if st.LabelW > float64(st.Rect.Dx())-12 || st.ValueW > float64(st.Rect.Dx())*0.94 {
				t.Errorf("%s: label %.0f / number %.0f px wide in %d px", id, st.LabelW, st.ValueW, st.Rect.Dx())
			}
			if !near(st.ValueSize, cardRingValue) && !near(st.ValueSize, cardFlatValue) && !near(st.ValueSize, cardSubValue) {
				t.Errorf("%s: number size %.1f outside the card scale", id, st.ValueSize)
			}
			if d, ok := digits[st.ValueSize]; ok && !near(d, st.DigitH) {
				t.Errorf("%s: digits %.1f px, others %.1f at font %.0f", id, st.DigitH, d, st.ValueSize)
			}
			digits[st.ValueSize] = st.DigitH
		}
	}
	if n < 100 {
		t.Errorf("only %d cells checked", n)
	}
}

// TestTemplatesUseUniformCells: every field of the ready-made compositions in
// every set is a design with uniform cells (the clock and the gaming square
// are drawn by their own widgets).
func TestTemplatesUseUniformCells(t *testing.T) {
	all := builtinDesigns()
	for _, set := range SetOrder {
		for _, tpl := range GridTemplates(set) {
			for i, tile := range tpl.Tiles {
				l, ok := all[tile.Design]
				if !ok {
					t.Errorf("%s/%s field %d: design %q missing", tpl.ID, set, i, tile.Design)
					continue
				}
				uniform := false
				for _, w := range l.Widgets {
					uniform = uniform || w.Uniform
				}
				if !uniform && !strings.Contains(tile.Design, "clock") && !strings.HasSuffix(tile.Design, "gaming") {
					t.Errorf("%s/%s field %d: %s has no uniform cell", tpl.ID, set, i, tile.Design)
				}
			}
		}
	}
}

// TestAutoDecimals: GB and ms show one decimal below 10 and none from 10 on.
func TestAutoDecimals(t *testing.T) {
	l := builtinDesigns()["half-mem"]
	width := func(v float64) float64 {
		p := fixedProv{func(string) float64 { return v }}
		r, err := lcd.NewRenderer(l)
		if err != nil {
			t.Fatal(err)
		}
		r.Render(p)
		return r.ValueStats()[0].ValueW
	}
	// "3.5" has two digit cells and a point, "12" (12.4 rounded) just two cells
	if a, b := width(3.5), width(12.4); a <= b {
		t.Errorf("3.5 is %.0f px wide, 12.4 %.0f: the decimal below 10 is missing", a, b)
	}
}
