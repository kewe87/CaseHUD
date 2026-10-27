package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

func newBarWorker(t testing.TB) *worker {
	t.Helper()
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return &worker{e: e, p: &demoBar{demoPanel{id: "us88-0", name: "bar", size: image.Pt(1920, 480)}}}
}

// fitDesign picks the design of kind for a cell of w x h in set.
func fitDesign(set string, cell lcd.Cell, kind string) string {
	p := setPrefix(set)
	switch {
	case cell.W == cell.H:
		if id := squareRoles[kind][set]; id != "" {
			return id
		}
		return squareRoles["cpu"][set]
	case cell.H == 240 && cell.W == 480:
		return p + "half-" + kind
	}
	tag := map[[2]int]string{{640, 480}: "w640", {960, 480}: "w960", {480, 640}: "h640", {480, 960}: "h960"}[[2]int{cell.W, cell.H}]
	if kind == "pump" || kind == "ram" || kind == "vram" || kind == "cpu-temp" || kind == "gpu-temp" {
		kind = "cpu"
	}
	return p + tag + "-" + kind
}

func TestGridPresets(t *testing.T) {
	for _, sz := range []image.Point{{1920, 480}, {480, 1920}} {
		gs := lcd.GridPresets(sz.X, sz.Y)
		if len(gs) != len(lcd.GridIDs) {
			t.Fatalf("%v: %d presets", sz, len(gs))
		}
		for _, g := range gs {
			if len(g.Cells) != lcd.GridCellCount(g.ID) {
				t.Errorf("%v %s: %d cells, want %d", sz, g.ID, len(g.Cells), lcd.GridCellCount(g.ID))
			}
			// The cells tile the bar exactly: no gap, no overlap.
			area := 0
			covered := image.NewGray(image.Rect(0, 0, sz.X, sz.Y))
			for _, c := range g.Cells {
				r := image.Rect(c.X, c.Y, c.X+c.W, c.Y+c.H)
				if !r.In(covered.Bounds()) {
					t.Errorf("%v %s: cell %v outside", sz, g.ID, r)
				}
				area += r.Dx() * r.Dy()
				for y := r.Min.Y; y < r.Max.Y; y++ {
					for x := r.Min.X; x < r.Max.X; x++ {
						if covered.GrayAt(x, y).Y != 0 {
							t.Fatalf("%v %s: overlap at %d,%d", sz, g.ID, x, y)
						}
						covered.SetGray(x, y, color.Gray{Y: 1})
					}
				}
			}
			if area != sz.X*sz.Y {
				t.Errorf("%v %s: covers %d of %d px", sz, g.ID, area, sz.X*sz.Y)
			}
		}
	}
	if lcd.GridPresets(480, 480) != nil || lcd.GridPresets(400, 400) != nil {
		t.Error("square panels must not get grids")
	}
}

// Every preset x every set renders a full frame, each cell with a design of
// its shape, landscape and upright.
func TestGridRenderAllPresets(t *testing.T) {
	w := newBarWorker(t)
	for _, sz := range []image.Point{{1920, 480}, {480, 1920}} {
		rot := 0
		if sz.X == 480 {
			rot = 270
		}
		for _, g := range lcd.GridPresets(sz.X, sz.Y) {
			for _, set := range SetOrder {
				c := PanelConfig{Mode: "design", Grid: g.ID, Rotation: rot}
				kinds := []string{"cpu", "gpu", "coolant", "power", "fps", "clock", "cpu-temp", "gpu-temp"}
				for i, cell := range g.Cells {
					c.Tiles = append(c.Tiles, Tile{Mode: "design", Design: fitDesign(set, cell, kinds[i%len(kinds)])})
				}
				src, err := w.newSource(c)
				if err != nil {
					t.Fatalf("%v %s %s: %v", sz, g.ID, set, err)
				}
				var img *image.RGBA
				for f := 0; f < 3; f++ {
					img, _ = src.frame(time.Duration(f) * 50 * time.Millisecond)
				}
				if img.Bounds().Size() != sz {
					t.Fatalf("%v %s %s: frame is %v", sz, g.ID, set, img.Bounds().Size())
				}
			}
		}
	}
}

func TestGridConfigBackCompat(t *testing.T) {
	var old PanelConfig
	if err := json.Unmarshal([]byte(`{"mode":"design","design":"water-wide","media":"","brightness":80,"rotation":0}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Grid != "" || old.Tiles != nil {
		t.Fatal("old config must have no grid")
	}
	b, _ := json.Marshal(old)
	if s := string(b); containsAny(s, "grid", "tiles") {
		t.Fatalf("empty grid must not be written: %s", s)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

func TestGridValidationAndScenes(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if _, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: "nope"}); err == nil {
		t.Error("unknown grid accepted")
	}
	if _, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: "4", Tiles: make([]Tile, 3)}); err == nil {
		t.Error("wrong tile count accepted")
	}
	if _, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: "2", Tiles: []Tile{{Mode: "x"}, {}}}); err == nil {
		t.Error("unknown tile mode accepted")
	}
	if _, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: "2", Tiles: []Tile{{Background: "nope"}, {}}}); err == nil {
		t.Error("unknown tile background accepted")
	}
	tpl, _ := GridTemplateByID("gaming", "neon")
	c, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles, Brightness: 50})
	if err != nil || len(c.Tiles) != 6 {
		t.Fatalf("%v %v", c, err)
	}
	c.Tiles[0].Design = "changed"
	if tpl.Tiles[0].Design == "changed" {
		t.Error("clean must not alias the caller's tiles")
	}
	// Scenes keep grid and tiles through JSON.
	r := RotationConfig{Mode: "scenes", IntervalS: 10, Effect: "fade", Scenes: []Scene{
		{Name: "a", Panels: map[string]PanelConfig{"us88-0": {Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles, Brightness: 50}}}}}
	b, _ := json.Marshal(r)
	var back RotationConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	got := back.Scenes[0].Panels["us88-0"]
	if got.Grid != "2+4" || len(got.Tiles) != 6 || got.Tiles[0].Design != "neon-card-fps-graph" {
		t.Fatalf("scene lost its grid: %+v", got)
	}
}

func TestTemplatesAreValid(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	for _, set := range SetOrder {
		for _, tpl := range GridTemplates(set) {
			c, err := e.cleanPanelConfig(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
			if err != nil {
				t.Fatalf("%s/%s: %v", tpl.ID, set, err)
			}
			if err := e.checkTiles(c, PanelConfig{}); err != nil {
				t.Errorf("%s/%s: %v", tpl.ID, set, err)
			}
			// A cell must get a design of its own shape.
			g, _ := lcd.GridPresetByID(1920, 480, tpl.Grid)
			for i, tl := range c.Tiles {
				l, _ := e.Store.Design(tl.Design)
				cell := g.Cells[i]
				if l.Width != cell.W || l.Height != cell.H {
					t.Errorf("%s/%s field %d: %s is %dx%d, cell is %dx%d", tpl.ID, set, i, tl.Design, l.Width, l.Height, cell.W, cell.H)
				}
			}
		}
	}
	// restyle round trip
	for _, id := range []string{"half-cpu", "neon-half-gpu", "w640-cpu", "retro-h960-fps", "card-cpu", "cpu-temp", "neon-gaming", "mine"} {
		for _, set := range SetOrder {
			if _, err := e.Store.Design(restyleDesign(id, set)); err != nil && id != "mine" {
				t.Errorf("restyle %s -> %s: %v", id, set, err)
			}
		}
	}
}

func TestUprightFixedDesigns(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	for _, id := range []string{"water-wide", "wide-rings", "neon-wide", "retro-wide", "cyber-wide"} {
		l, err := s.DesignFor(id, image.Pt(480, 1920))
		if err != nil || l.Width != 480 || l.Height != 1920 {
			t.Fatalf("%s: %dx%d %v", id, l.Width, l.Height, err)
		}
		for _, w := range l.Widgets {
			if w.X < 0 || w.Y < 0 || w.X+w.W > 480 || w.Y+w.H > 1920 {
				t.Errorf("%s: widget outside the canvas: %+v", id, w)
			}
		}
	}
	if l, _ := s.DesignFor("water-wide", image.Pt(1920, 480)); l.Width != 1920 {
		t.Error("landscape must stay landscape")
	}
}

// BenchmarkGrid1920 renders the Overview composition at 1920x480 per frame.
func BenchmarkGrid1920(b *testing.B) {
	w := newBarWorker(b)
	tpl, _ := GridTemplateByID("overview", "clean")
	src, err := w.newSource(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
	if err != nil {
		b.Fatal(err)
	}
	bench(b, src)
}

// BenchmarkGrid1920Animated is the worst case: four animated backgrounds with
// bloom on a 1920x480 grid, every frame redrawn.
func BenchmarkGrid1920Animated(b *testing.B) {
	w := newBarWorker(b)
	tpl, _ := GridTemplateByID("overview", "neon")
	for i, fx := range lcd.BackgroundEffects[:min(4, len(lcd.BackgroundEffects))] {
		tpl.Tiles[i].Background, tpl.Tiles[i].BgBloom = fx, true
	}
	src, err := w.newSource(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
	if err != nil {
		b.Fatal(err)
	}
	bench(b, src)
}

func bench(b *testing.B, src source) {
	var tt time.Duration
	src.frame(tt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt += 50 * time.Millisecond
		src.frame(tt)
	}
}

// TestGridDumpPNG writes the library and the compositions as PNGs for a
// visual check: CASEHUD_GRID_PNG=<dir> go test ./internal/app -run DumpPNG
func TestGridDumpPNG(t *testing.T) {
	dir := os.Getenv("CASEHUD_GRID_PNG")
	if dir == "" {
		t.Skip("set CASEHUD_GRID_PNG to a directory")
	}
	os.MkdirAll(dir, 0o755)
	w := newBarWorker(t)
	save := func(name string, img image.Image) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, img)
	}
	render := func(c PanelConfig) *image.RGBA {
		src, err := w.newSource(c)
		if err != nil {
			t.Fatal(err)
		}
		var img *image.RGBA
		for f := 0; f < 40; f++ { // let values glide and graphs fill
			img, _ = src.frame(time.Duration(f) * 100 * time.Millisecond)
			time.Sleep(15 * time.Millisecond)
		}
		out := image.NewRGBA(img.Bounds())
		draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Src)
		return out
	}
	for _, set := range SetOrder {
		for _, tpl := range GridTemplates(set) {
			save(fmt.Sprintf("tpl-%s-%s.png", tpl.ID, set), render(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles}))
			// upright: the same fields, shapes adapted
			g, _ := lcd.GridPresetByID(480, 1920, tpl.Grid)
			c := PanelConfig{Mode: "design", Grid: tpl.Grid, Rotation: 270, Tiles: append([]Tile(nil), tpl.Tiles...)}
			for i := range c.Tiles {
				_ = g
				c.Tiles[i].Design = reorientDesign(c.Tiles[i].Design, true)
			}
			save(fmt.Sprintf("tpl-%s-%s-upright.png", tpl.ID, set), render(c))
		}
		// All fields of a size class on one sheet.
		sheet := func(name string, w0, h0 int, ids []string, cols int) {
			rows := (len(ids) + cols - 1) / cols
			img := image.NewRGBA(image.Rect(0, 0, cols*(w0+8), rows*(h0+8)))
			draw.Draw(img, img.Bounds(), image.NewUniform(image.Black), image.Point{}, draw.Src)
			for i, id := range ids {
				l, err := w.e.Store.Design(id)
				if err != nil {
					t.Fatal(err)
				}
				r, _ := lcd.NewRenderer(l)
				w.e.localize(r)
				mock := metrics.NewMock()
				t0 := time.Now()
				for k := 0; k < 80; k++ {
					mock.SetTime(t0.Add(time.Duration(k) * 3 * time.Second))
					r.Sample(mock)
				}
				f := r.Render(mock)
				x, y := (i%cols)*(w0+8), (i/cols)*(h0+8)
				draw.Draw(img, image.Rect(x, y, x+w0, y+h0), f, image.Point{}, draw.Src)
			}
			save(name, img)
		}
		p := setPrefix(set)
		var halves, w640, w960, h640, h960 []string
		for _, k := range tileKinds {
			halves = append(halves, p+"half-"+k)
		}
		for _, k := range wideKinds {
			w640 = append(w640, p+"w640-"+k)
			w960 = append(w960, p+"w960-"+k)
			h640 = append(h640, p+"h640-"+k)
			h960 = append(h960, p+"h960-"+k)
		}
		sheet("half-"+set+".png", 480, 240, halves, 4)
		var combos, cards []string
		for _, d := range comboDefs() {
			if d.layout == "half" {
				combos = append(combos, p+"half-"+d.id)
			} else {
				cards = append(cards, p+"card-"+d.id)
			}
		}
		sheet("combo-"+set+".png", 480, 240, combos, 4)
		for _, k := range cardKinds {
			cards = append(cards, p+"card-"+k)
		}
		sheet("cards-"+set+".png", 480, 480, cards, 4)
		sheet("w640-"+set+".png", 640, 480, w640, 3)
		sheet("w960-"+set+".png", 960, 480, w960, 2)
		sheet("h640-"+set+".png", 480, 640, h640, 6)
		sheet("h960-"+set+".png", 480, 960, h960, 6)
	}
	_ = lcd.GridIDs
}

// BenchmarkGrid1920Resample redraws every field on every frame (sensors
// sampled each time, values gliding): the cost of one frame when nothing
// can be reused.
func BenchmarkGrid1920Resample(b *testing.B) {
	w := newBarWorker(b)
	w.e.mu.Lock()
	w.e.cfg.IntervalMS = 1
	w.e.mu.Unlock()
	tpl, _ := GridTemplateByID("overview", "clean")
	src, err := w.newSource(PanelConfig{Mode: "design", Grid: tpl.Grid, Tiles: tpl.Tiles})
	if err != nil {
		b.Fatal(err)
	}
	bench(b, src)
}

func newBarEngine(t *testing.T) *Engine {
	t.Helper()
	t.Setenv("CASEHUD_DEMO_BAR", "1")
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e
}

func TestGridEngineFlow(t *testing.T) {
	e := newBarEngine(t)
	var p PanelState
	for _, x := range e.Panels() {
		if x.ID == "us88-0" {
			p = x
		}
	}
	if p.Config.Grid != "4" || len(p.Config.Tiles) != 4 || len(p.Grids) != 6 {
		t.Fatalf("default: %+v", p.Config)
	}
	// ApplySet restyles the fields and keeps the grid.
	if _, err := e.ApplySet("neon", "dark", []string{"us88-0"}); err != nil {
		t.Fatal(err)
	}
	c := e.cfgOf("us88-0")
	if c.Grid != "4" || c.Tiles[0].Design != "neon-card-cpu" || c.Tiles[3].Design != "neon-card-system" {
		t.Fatalf("ApplySet: %+v", c.Tiles)
	}
	// A grid cannot be set on a square display.
	if err := e.SetPanelConfig("aio-7398-0", PanelConfig{Mode: "design", Grid: "4", Tiles: c.Tiles}); err == nil {
		t.Error("grid accepted on a square panel")
	}
	// Turning the bar upright keeps the fields and turns wide ones.
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Grid: "3", Brightness: 50,
		Tiles: []Tile{{Design: "w640-cpu"}, {Design: "neon-w640-gpu"}, {Design: "half-cpu"}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Grid: "3", Brightness: 50, Rotation: 90,
		Tiles: []Tile{{Design: "w640-cpu"}, {Design: "neon-w640-gpu"}, {Design: "half-cpu"}}}); err != nil {
		t.Fatal(err)
	}
	c = e.cfgOf("us88-0")
	if c.Tiles[0].Design != "h640-cpu" || c.Tiles[1].Design != "neon-h640-gpu" || c.Tiles[2].Design != "half-cpu" {
		t.Fatalf("upright: %+v", c.Tiles)
	}
	for _, x := range e.Panels() {
		if x.ID == "us88-0" && (x.Width != 480 || x.Height != 1920) {
			t.Fatalf("upright size %dx%d", x.Width, x.Height)
		}
	}
	// Conveyor: designs slide through the fields.
	e.mu.Lock()
	e.cfg.Rotation = RotationConfig{Enabled: true, Mode: "conveyor", IntervalS: 5, Conveyor: []string{"clock", "loop", "gaming", "cpu-temp", "gpu-temp"}}
	e.mu.Unlock()
}

func (e *Engine) cfgOf(id string) PanelConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.Panels[id].Clone()
}

func TestConveyorSlidesThroughFields(t *testing.T) {
	e := newBarEngine(t)
	conv := []string{"clock", "loop", "gaming", "cpu-temp", "gpu-temp"}
	if err := e.SetRotation(RotationConfig{Enabled: true, Mode: "conveyor", IntervalS: 60, Effect: "none", Conveyor: conv}); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	w := e.workers["us88-0"]
	e.mu.Unlock()
	idx := func(id string) int { return slices.Index(conv, id) }
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c := w.current()
		if len(c.Tiles) == 4 && idx(c.Tiles[0].Design) >= 0 {
			for k := 1; k < 4; k++ {
				if want := (idx(c.Tiles[k-1].Design) - 1 + len(conv)) % len(conv); idx(c.Tiles[k].Design) != want {
					t.Fatalf("fields do not follow each other: %+v", c.Tiles)
				}
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("conveyor did not reach the grid")
}

// sensorFamily groups the readings of one thing, so "used" and "load" of the
// same memory count as the same information.
func sensorFamily(src string) string {
	l := strings.ToLower(src)
	switch {
	case strings.Contains(l, "gpu memory") || strings.Contains(l, "d3d dedicated"):
		return "vram"
	case strings.Contains(l, "memory"):
		return "ram"
	}
	return src
}

// A template shows every value once: no sensor family appears in two cells,
// and no cell shows used and load of the same memory. Bars are meters of the
// number next to them and do not count.
func TestTemplatesHaveNoDuplicateValues(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	for _, set := range SetOrder {
		for _, tpl := range GridTemplates(set) {
			seen := map[string]int{}
			for i, tl := range tpl.Tiles {
				l, err := e.Store.Design(tl.Design)
				if err != nil {
					t.Fatal(err)
				}
				cell := map[string]string{} // family -> source
				for _, w := range l.Widgets {
					if w.Type == "bar" {
						continue
					}
					for _, src := range []string{w.Source, w.SubSource} {
						if src == "" || strings.Contains(strings.ToLower(src), "total") && strings.Contains(src, "Memory") {
							continue // a static total next to its used value
						}
						f := sensorFamily(src)
						if prev, ok := cell[f]; ok && prev != src {
							t.Errorf("%s/%s cell %d: %q and %q are the same information", tpl.ID, set, i, prev, src)
						}
						cell[f] = src
					}
				}
				for f := range cell {
					if j, dup := seen[f]; dup {
						t.Errorf("%s/%s: %q shown in cells %d and %d", tpl.ID, set, f, j, i)
					}
					seen[f] = i
				}
			}
		}
	}
}
