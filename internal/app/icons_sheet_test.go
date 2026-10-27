package app

import (
	xdraw "golang.org/x/image/draw"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

// renderDesignAnimated renders layout l the way a panel shows it after a few
// seconds: values settled, graphs filled, icons turning.
func renderDesignAnimated(t testing.TB, e *Engine, l lcd.Layout, frames int) *image.RGBA {
	t.Helper()
	l.Lang = "en"
	r, err := lcd.NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	e.localize(r)
	r.SetAnimation(animationTau)
	mock := metrics.NewMock()
	t0 := time.Now()
	for k := 0; k < 60; k++ {
		mock.SetTime(t0.Add(time.Duration(k) * 3 * time.Second))
		r.Sample(mock)
	}
	now := t0
	r.Now = func() time.Time { return now }
	var img *image.RGBA
	for k := 0; k < frames; k++ {
		mock.SetTime(t0.Add(time.Duration(60*3)*time.Second + time.Duration(k)*40*time.Millisecond))
		now = now.Add(40 * time.Millisecond)
		img = r.Render(mock)
	}
	return img
}

// TestIconSheets writes contact sheets of the designs with icons when
// ICON_SHEETS is set to a directory (manual visual check).
func TestIconSheets(t *testing.T) {
	dir := os.Getenv("ICON_SHEETS")
	if dir == "" {
		t.Skip("set ICON_SHEETS to a directory")
	}
	os.MkdirAll(dir, 0o755)
	w := newBarWorker(t)
	sheet := func(name string, w0, h0 int, ids []string, cols int) {
		rows := (len(ids) + cols - 1) / cols
		img := image.NewRGBA(image.Rect(0, 0, cols*(w0+8), rows*(h0+8)))
		draw.Draw(img, img.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
		for i, id := range ids {
			l, err := w.e.Store.Design(id)
			if err != nil {
				t.Fatal(err)
			}
			f := renderDesignAnimated(t, w.e, l, 30)
			x, y := (i%cols)*(w0+8), (i/cols)*(h0+8)
			draw.Draw(img, image.Rect(x, y, x+w0, y+h0), f, image.Point{}, draw.Src)
		}
		fh, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer fh.Close()
		png.Encode(fh, img)
	}
	for _, set := range SetOrder {
		p := setPrefix(set)
		var halves, cards, w640, w960, h640, h960, combos []string
		for _, k := range tileKinds {
			halves = append(halves, p+"half-"+k)
		}
		for _, k := range cardKinds {
			cards = append(cards, p+"card-"+k)
		}
		for _, k := range wideKinds {
			w640 = append(w640, p+"w640-"+k)
			w960 = append(w960, p+"w960-"+k)
			h640 = append(h640, p+"h640-"+k)
			h960 = append(h960, p+"h960-"+k)
		}
		var cardCombos, halfCombos []string
		for _, d := range comboDefs() {
			if d.layout == "half" {
				halfCombos = append(halfCombos, comboID(setStyle(set), d))
			} else {
				cardCombos = append(cardCombos, comboID(setStyle(set), d))
			}
		}
		_ = combos
		sheet("half-"+set+".png", 480, 240, halves, 4)
		sheet("halfcombo-"+set+".png", 480, 240, halfCombos, 4)
		sheet("card-"+set+".png", 480, 480, cards, 5)
		sheet("cardcombo-"+set+".png", 480, 480, cardCombos, 4)
		sheet("w640-"+set+".png", 640, 480, w640, 3)
		sheet("w960-"+set+".png", 960, 480, w960, 2)
		sheet("h640-"+set+".png", 480, 640, h640, 6)
		sheet("h960-"+set+".png", 480, 960, h960, 6)
	}
	wide := map[string]string{"clean": "wide-rings", "neon": "neon-wide", "retro": "retro-wide", "cyberpunk": "cyber-wide"}
	var ids []string
	for _, set := range SetOrder {
		ids = append(ids, wide[set])
	}
	sheet("wide.png", 1920, 480, append(ids, "water-wide"), 1)
}

func setStyle(set string) tileStyle {
	for _, s := range tileStyles {
		if s.set == set {
			return s
		}
	}
	return tileStyles[0]
}

// TestIconGIFs records animated GIFs of panels with icons (ICON_SHEETS).
func TestIconGIFs(t *testing.T) {
	dir := os.Getenv("ICON_SHEETS")
	if dir == "" {
		t.Skip("set ICON_SHEETS to a directory")
	}
	os.MkdirAll(dir, 0o755)
	record := func(name string, w *worker, c PanelConfig, scale int, frames int) {
		src, err := w.newSource(c)
		if err != nil {
			t.Fatal(err)
		}
		g := &gif.GIF{}
		start := time.Now()
		for i := 0; i < frames; i++ {
			img, _ := src.frame(time.Since(start))
			sz := img.Bounds().Size()
			small := image.NewRGBA(image.Rect(0, 0, sz.X/scale, sz.Y/scale))
			xdraw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Src, nil)
			pal := image.NewPaletted(small.Bounds(), palette.Plan9)
			draw.FloydSteinberg.Draw(pal, pal.Bounds(), small, image.Point{})
			g.Image = append(g.Image, pal)
			g.Delay = append(g.Delay, 4)
			time.Sleep(40 * time.Millisecond)
		}
		fh, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer fh.Close()
		gif.EncodeAll(fh, g)
	}
	bar := newBarWorker(t)
	for _, set := range []string{"clean", "neon"} {
		for _, id := range []string{"gaming", "watercooling"} {
			tpl, _ := GridTemplateByID(id, set)
			record("grid-"+id+"-"+set+".gif", bar, PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles}, 2, 60)
		}
	}
	fan := newFanWorker(t)
	record("round-coolant.gif", fan, PanelConfig{Mode: "design", Design: "icon-coolant"}, 1, 60)
	record("round-coolant-neon.gif", fan, PanelConfig{Mode: "design", Design: "neon-icon-coolant"}, 1, 60)
}

// TestIconDesignSheets renders the round icon designs of all sets and the
// wide bar designs (ICON_SHEETS).
func TestIconDesignSheets(t *testing.T) {
	dir := os.Getenv("ICON_SHEETS")
	if dir == "" {
		t.Skip("set ICON_SHEETS to a directory")
	}
	os.MkdirAll(dir, 0o755)
	w := newBarWorker(t)
	ids := iconDesignOrder()
	cols := len(iconKinds)
	rows := (len(ids) + cols - 1) / cols
	img := image.NewRGBA(image.Rect(0, 0, cols*488, rows*488))
	draw.Draw(img, img.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
	for i, id := range ids {
		l, err := w.e.Store.Design(id)
		if err != nil {
			t.Fatal(err)
		}
		f := renderDesignAnimated(t, w.e, l, 30)
		// the round screen: corners outside the circle go grey
		for y := 0; y < 480; y++ {
			for x := 0; x < 480; x++ {
				dx, dy := float64(x)-239.5, float64(y)-239.5
				if dx*dx+dy*dy > 240*240 {
					f.SetRGBA(x, y, color.RGBA{40, 40, 44, 255})
				}
			}
		}
		x, y := (i%cols)*488, (i/cols)*488
		draw.Draw(img, image.Rect(x, y, x+480, y+480), f, image.Point{}, draw.Src)
	}
	fh, err := os.Create(filepath.Join(dir, "round-icon.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	png.Encode(fh, img)
}

// TestIconGridSheets renders the ready-made grid compositions of every set
// at 1920x480 (ICON_SHEETS): one PNG per set with the four templates below
// each other.
func TestIconGridSheets(t *testing.T) {
	dir := os.Getenv("ICON_SHEETS")
	if dir == "" {
		t.Skip("set ICON_SHEETS to a directory")
	}
	os.MkdirAll(dir, 0o755)
	w := newBarWorker(t)
	for _, set := range SetOrder {
		tpls := GridTemplates(set)
		img := image.NewRGBA(image.Rect(0, 0, 1920, len(tpls)*488))
		draw.Draw(img, img.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
		for i, tpl := range tpls {
			src, err := w.newSource(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
			if err != nil {
				t.Fatal(err)
			}
			var f *image.RGBA
			for k := 0; k < 30; k++ {
				f, _ = src.frame(time.Duration(k) * 40 * time.Millisecond)
				time.Sleep(30 * time.Millisecond)
			}
			draw.Draw(img, image.Rect(0, i*488, 1920, i*488+480), f, image.Point{}, draw.Src)
		}
		fh, err := os.Create(filepath.Join(dir, "grid-"+set+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(fh, img)
		fh.Close()
	}
}

// TestIconDump renders the designs named in ICON_IDS (comma separated) into
// ICON_SHEETS/<id>.png, upright ones as "<id>-upright".
func TestIconDump(t *testing.T) {
	dir, ids := os.Getenv("ICON_SHEETS"), os.Getenv("ICON_IDS")
	if dir == "" || ids == "" {
		t.Skip("set ICON_SHEETS and ICON_IDS")
	}
	w := newBarWorker(t)
	for _, id := range strings.Split(ids, ",") {
		var l lcd.Layout
		if base, ok := strings.CutSuffix(id, "-upright"); ok {
			l, _ = uprightDesign(base)
		} else {
			var err error
			if l, err = w.e.Store.Design(id); err != nil {
				t.Fatal(err)
			}
		}
		f := renderDesignAnimated(t, w.e, l, 30)
		fh, _ := os.Create(filepath.Join(dir, id+".png"))
		png.Encode(fh, f)
		fh.Close()
	}
}
