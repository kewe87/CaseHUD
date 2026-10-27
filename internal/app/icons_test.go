package app

import (
	"context"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
)

// textRects are the parts of widget w that carry text, as far as a layout
// can know: whole rectangles, except that a centred value with a second value
// leaves the gap between number and second value free, and a gauge with
// text only uses its middle.
func textRects(w lcd.Widget) []image.Rectangle {
	r := image.Rect(w.X, w.Y, w.X+w.W, w.Y+w.H)
	switch w.Type {
	case "gauge":
		if w.RingOnly {
			return nil
		}
		s := float64(min(w.W, w.H))
		cx, cy := float64(w.X)+float64(w.W)/2, float64(w.Y)+float64(w.H)/2
		return []image.Rectangle{image.Rect(int(cx-0.3*s), int(cy-0.26*s), int(cx+0.3*s), int(cy+0.36*s))}
	case "value":
		if w.Align == "center" && w.SubSource != "" {
			h := float64(w.H)
			top := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+int(h*(0.18*1.05+0.46*0.92)))
			sub := image.Rect(r.Min.X, r.Max.Y-int(h*0.2), r.Max.X, r.Max.Y)
			return []image.Rectangle{top, sub}
		}
		return []image.Rectangle{r}
	case "bar", "graph", "text", "clock":
		return []image.Rectangle{r}
	}
	return nil
}

// checkIconsClear fails when an icon of l overlaps text or lies on a ring,
// and returns the number of icons.
func checkIconsClear(t *testing.T, id string, l lcd.Layout) (icons int) {
	t.Helper()
	for i, ic := range l.Widgets {
		if ic.Type != "icon" {
			continue
		}
		icons++
		if ic.Inline != "" {
			continue // placed at render time, see TestInlineIcons
		}
		sq := image.Rect(ic.X, ic.Y, ic.X+ic.W, ic.Y+ic.H)
		if !sq.In(image.Rect(0, 0, l.Width, l.Height)) {
			t.Errorf("%s: icon %v outside %dx%d", id, sq, l.Width, l.Height)
		}
		if ic.W < 40 || ic.W != ic.H {
			t.Errorf("%s: icon size %dx%d", id, ic.W, ic.H)
		}
		for j, w := range l.Widgets {
			if j == i || w.Type == "icon" {
				continue
			}
			for _, tr := range textRects(w) {
				if sq.Overlaps(tr) {
					t.Errorf("%s: icon %v overlaps %s widget %d (%v)", id, sq, w.Type, j, tr)
				}
			}
			if w.Type == "gauge" {
				s := float64(min(w.W, w.H))
				cx, cy := float64(w.X)+float64(w.W)/2, float64(w.Y)+float64(w.H)/2
				// the icon must lie outside the ring or in its opening
				// (below the ends of the 270 degree arc)
				if w.Sweep == 0 && float64(sq.Min.Y) >= cy+0.33*s && float64(sq.Min.X) > cx-0.3*s && float64(sq.Max.X) < cx+0.3*s {
					continue
				}
				for _, c := range []image.Point{sq.Min, {sq.Max.X, sq.Min.Y}, {sq.Min.X, sq.Max.Y}, sq.Max} {
					dx, dy := float64(c.X)-cx, float64(c.Y)-cy
					if dx*dx+dy*dy < (s*0.46)*(s*0.46) {
						t.Errorf("%s: icon %v on the ring of gauge %d", id, sq, j)
						break
					}
				}
			}
		}
	}
	return icons
}

func TestBuiltinIconsDoNotCollide(t *testing.T) {
	perSet := map[string]int{}
	for id, l := range builtinDesigns() {
		n := checkIconsClear(t, id, l)
		isRow := strings.HasSuffix(id, "wide") || id == "wide-rings" || id == "water-wide"
		if n > 1 && !isRow {
			t.Errorf("%s: %d icons in one cell", id, n)
		}
		if isRow && n != 4 {
			t.Errorf("%s: %d icons, want 4", id, n)
		}
		if n > 0 {
			perSet[l.Set]++
		}
	}
	for _, set := range SetOrder {
		if perSet[set] < 30 {
			t.Errorf("set %s: only %d designs with icons", set, perSet[set])
		}
	}
	// the upright bar designs too
	for id := range wideParts {
		l, ok := uprightDesign(id)
		if !ok || checkIconsClear(t, id+"-upright", l) != 4 {
			t.Errorf("%s upright: icons missing", id)
		}
	}
	if l, _ := uprightDesign("water-wide"); checkIconsClear(t, "water-wide-upright", l) != 4 {
		t.Error("water-wide upright: icons missing")
	}
}

func TestIconKindsFollowReading(t *testing.T) {
	d := builtinDesigns()
	want := map[string]string{"half-cpu": "chip", "neon-half-gpu": "chip", "half-cpu-temp": "temp", "retro-half-coolant": "pump",
		"cyber-half-pump": "pump", "half-power": "power", "half-fps": "fps", "card-coolant": "pump", "neon-card-fps": "fps",
		"w640-gpu": "chip", "h960-coolant": "pump", "half-ram": "memory", "retro-half-vram": "memory", "half-mem": "memory", "half-lat-cpu": "clock", "half-gpu-clock": "chip", "card-system": "power", "card-fps-graph": "fps", "half-coolant-pump": "pump"}
	for id, kind := range want {
		l, ok := d[id]
		if !ok {
			t.Errorf("design %s missing", id)
			continue
		}
		got := ""
		for _, w := range l.Widgets {
			if w.Type == "icon" {
				got = w.Icon
				if w.Source == "" {
					t.Errorf("%s: icon without sensor", id)
				}
			}
		}
		if got != kind {
			t.Errorf("%s: icon %q, want %q", id, got, kind)
		}
	}
	// readings without an icon stay free of one
	for _, id := range []string{"half-clock", "card-clock"} {
		for _, w := range d[id].Widgets {
			if w.Type == "icon" {
				t.Errorf("%s has an icon", id)
			}
		}
	}
}

func frameWaits(t *testing.T, w *worker, c PanelConfig) (lo, hi time.Duration) {
	t.Helper()
	src, err := w.newSource(c)
	if err != nil {
		t.Fatal(err)
	}
	lo = time.Hour
	for i := 0; i < 30; i++ {
		_, wait := src.frame(time.Duration(i) * 40 * time.Millisecond)
		lo, hi = min(lo, wait), max(hi, wait)
	}
	return
}

func TestIconFrameCadence(t *testing.T) {
	w := newBarWorker(t)
	// a design with an icon runs at the icon frame rate ...
	lo, hi := frameWaits(t, w, PanelConfig{Mode: "design", Design: "half-cpu"})
	if hi > lcd.IconFrameInterval || lo <= 0 {
		t.Errorf("icon design waits %v..%v, want <= %v", lo, hi, lcd.IconFrameInterval)
	}
	// ... also over an animated background (which runs at 20 fps otherwise)
	_, hi = frameWaits(t, w, PanelConfig{Mode: "design", Design: "half-cpu", Background: "aurora"})
	if hi > lcd.IconFrameInterval {
		t.Errorf("icon over background waits up to %v", hi)
	}
	// a design without an icon keeps its slow cadence
	_, hi = frameWaits(t, w, PanelConfig{Mode: "design", Design: "half-clock"})
	if hi <= lcd.IconFrameInterval {
		t.Errorf("design without icon refreshes every %v", hi)
	}
	// a grid is as fast as its fastest field
	tpl, _ := GridTemplateByID("overview", "clean")
	lo, _ = frameWaits(t, w, PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
	if lo > lcd.IconFrameInterval {
		t.Errorf("grid with icons never reaches %v (min wait %v)", lcd.IconFrameInterval, lo)
	}
	// the global switch stops the motion: no icon frames
	w.e.mu.Lock()
	w.e.cfg.NoAnimation = true
	w.e.mu.Unlock()
	_, hi = frameWaits(t, w, PanelConfig{Mode: "design", Design: "half-cpu"})
	if hi <= lcd.IconFrameInterval {
		t.Errorf("animations off, but frames every %v", hi)
	}
}

// With icons the HUD behind them is not redrawn: between two display frames
// only the icon square changes.
func TestIconFramesTouchOnlyIconRect(t *testing.T) {
	w := newBarWorker(t)
	src, err := w.newSource(PanelConfig{Mode: "design", Design: "half-cpu"})
	if err != nil {
		t.Fatal(err)
	}
	ds := src.(*designSource)
	a, _ := src.frame(0)
	first := append([]uint8(nil), a.Pix...)
	// stay inside the sampling interval so the HUD itself does not change
	ds.interval = time.Hour
	time.Sleep(120 * time.Millisecond)
	b, _ := src.frame(120 * time.Millisecond)
	rects := ds.r.IconRects()
	if len(rects) != 1 {
		t.Fatalf("icon rects %v", rects)
	}
	changed := 0
	for y := 0; y < b.Rect.Dy(); y++ {
		for x := 0; x < b.Rect.Dx(); x++ {
			i := b.PixOffset(x, y)
			if string(first[i:i+4]) != string(b.Pix[i:i+4]) {
				changed++
				if !image.Pt(x, y).In(rects[0]) {
					t.Fatalf("pixel %d,%d changed outside the icon %v", x, y, rects[0])
				}
			}
		}
	}
	if changed == 0 {
		t.Error("icon did not move")
	}
}

func TestOldDesignsAndConfigsUnchanged(t *testing.T) {
	// round designs from before icons stay free of icons and opacity
	for _, id := range []string{"loop", "cpu-temp", "gaming", "neon-loop", "retro-gaming", "cyber-clock", "cpu-gpu"} {
		for _, w := range builtinDesigns()[id].Widgets {
			if w.Type == "icon" {
				t.Fatalf("%s must stay free of icons", id)
			}
		}
	}
	e := newBarEngine(t)
	if !e.validBackground("aurora") || !e.validBackground("") {
		t.Error("old effect backgrounds must stay valid")
	}
	// ids of stored configs keep resolving after the card changes
	for _, id := range []string{"card-cpu", "card-coolant", "neon-cpu", "half-cpu", "w960-coolant", "neon-w960-coolant", "neon-card-cpu"} {
		if _, err := e.Store.Design(id); err != nil {
			t.Errorf("design %s: %v", id, err)
		}
	}
	if restyleDesign("card-cpu", "neon") != "neon-card-cpu" || restyleDesign("neon-card-cpu", "clean") != "card-cpu" ||
		restyleDesign("card-clock", "retro") != "retro-clock" {
		t.Error("restyle of cards")
	}
}

func benchFrames(b *testing.B, w *worker, c PanelConfig) {
	b.Helper()
	src, err := w.newSource(c)
	if err != nil {
		b.Fatal(err)
	}
	var tt time.Duration
	src.frame(tt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt += 40 * time.Millisecond
		src.frame(tt)
	}
}

// BenchmarkIconGrid1920 is one display frame of a 1920x480 grid with four
// animated icons (the "overview" composition) at the icon frame rate.
func BenchmarkIconGrid1920(b *testing.B) {
	w := newBarWorker(b)
	tpl, _ := GridTemplateByID("overview", "clean")
	benchFrames(b, w, PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
}

// BenchmarkIconGrid1920Static is the same grid with animations off: the
// frame is only redrawn when values change.
func BenchmarkIconGrid1920Static(b *testing.B) {
	w := newBarWorker(b)
	w.e.mu.Lock()
	w.e.cfg.NoAnimation = true
	w.e.mu.Unlock()
	tpl, _ := GridTemplateByID("overview", "clean")
	benchFrames(b, w, PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
}

func newFanWorker(b testing.TB) *worker {
	b.Helper()
	e := &Engine{Store: &Store{Dir: b.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(e.Stop)
	return &worker{e: e, round: true, p: &demoPanel{id: "fan-0", name: "fan", size: image.Pt(400, 400), round: true}}
}

// BenchmarkIconFan400 is a 400x400 fan with the animated icon background,
// BenchmarkIconFan400Plain the same design without it.
func BenchmarkIconFan400(b *testing.B) {
	benchFrames(b, newFanWorker(b), PanelConfig{Mode: "design", Design: "icon-coolant"})
}

func BenchmarkIconFan400Plain(b *testing.B) {
	benchFrames(b, newFanWorker(b), PanelConfig{Mode: "design", Design: "loop"})
}

func TestIconDesignsRound(t *testing.T) {
	d := builtinDesigns()
	for _, s := range tileStyles {
		for _, k := range iconKinds {
			id := s.prefix + "icon-" + k
			l, ok := d[id]
			if !ok {
				t.Errorf("%s missing", id)
				continue
			}
			if !l.Round || l.Width != 480 || l.Set != s.set {
				t.Errorf("%s: round=%v %dx%d set=%q", id, l.Round, l.Width, l.Height, l.Set)
			}
			if checkIconsClear(t, id, l) != 1 {
				t.Errorf("%s: want exactly one icon", id)
			}
			for _, w := range l.Widgets {
				if w.Type == "icon" {
					if w.W < 150 || w.W > 190 {
						t.Errorf("%s: icon %d px", id, w.W)
					}
					// all corners inside the circle with margin
					for _, c := range [][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
						dx, dy := float64(w.X)+c[0]*float64(w.W)-240, float64(w.Y)+c[1]*float64(w.H)-240
						if dx*dx+dy*dy > 230*230 {
							t.Errorf("%s: icon corner outside the safe circle", id)
						}
					}
				} else if w.Y+w.H > 450 || w.W > 360 {
					t.Errorf("%s: %s widget %d,%d %dx%d leaves the round safe area", id, w.Type, w.X, w.Y, w.W, w.H)
				}
			}
		}
	}
	// every icon design is offered in the picker and has a German name
	order := map[string]bool{}
	for _, id := range BuiltinOrder {
		order[id] = true
	}
	for _, id := range iconDesignOrder() {
		if !order[id] {
			t.Errorf("%s not in BuiltinOrder", id)
		}
	}
	// the watermark mode is gone
	e := newBarEngine(t)
	if e.validBackground("icon:auto") {
		t.Error("icon backgrounds must not exist")
	}
}
