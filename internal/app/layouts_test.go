package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func startBar(t *testing.T, dir string) *Engine {
	t.Helper()
	t.Setenv("CASEHUD_DEMO_BAR", "1")
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e
}

func ownLayout(name string) Layout {
	g, _ := GridTemplateByID("overview", "neon")
	return Layout{Name: name, Grid: g.Grid, Tiles: g.Tiles}
}

func TestLayoutSaveLoadList(t *testing.T) {
	e := startBar(t, t.TempDir())
	if err := e.SaveLayout("mein-layout", ownLayout("  Mein Layout  ")); err != nil {
		t.Fatal(err)
	}
	l, ok := e.Store.Layout("mein-layout")
	if !ok || l.Name != "Mein Layout" || l.Grid != "4" || len(l.Tiles) != 4 || l.Builtin {
		t.Fatalf("loaded %+v ok=%v", l, ok)
	}
	var ids []string
	builtin := 0
	for _, x := range e.Layouts() {
		ids = append(ids, x.ID)
		if x.Builtin {
			builtin++
		}
	}
	if builtin != 16 || ids[len(ids)-1] != "mein-layout" {
		t.Fatalf("builtin=%d ids=%v", builtin, ids)
	}
	if _, ok := e.Store.Layout("gaming-neon"); !ok {
		t.Fatal("ready-made layout missing")
	}
}

func TestLayoutRejectsBadInput(t *testing.T) {
	e := startBar(t, t.TempDir())
	good := ownLayout("x")
	for _, id := range []string{"", "../evil", "a/b", `a\b`, "Big", "-x", strings.Repeat("a", 65), "..", "con.json"} {
		if err := e.SaveLayout(id, good); err == nil {
			t.Errorf("id %q accepted", id)
		}
	}
	if err := e.SaveLayout("gaming-clean", good); err == nil {
		t.Error("ready-made layout overwritten")
	}
	if err := e.DeleteLayout("gaming-clean"); err == nil {
		t.Error("ready-made layout deleted")
	}
	bad := good
	bad.Grid = "9"
	if err := e.SaveLayout("a", bad); err == nil {
		t.Error("unknown grid accepted")
	}
	bad = good
	bad.Tiles = good.Tiles[:2]
	if err := e.SaveLayout("a", bad); err == nil {
		t.Error("wrong tile count accepted")
	}
	bad = good
	bad.Name = " "
	if err := e.SaveLayout("a", bad); err == nil {
		t.Error("empty name accepted")
	}
	bad = ownLayout("x")
	bad.Tiles = slices.Clone(bad.Tiles)
	bad.Tiles[0].Design = "../../secret"
	if err := e.SaveLayout("a", bad); err == nil {
		t.Error("path in design id accepted")
	}
	if err := e.DeleteLayout("../x"); err == nil {
		t.Error("path delete accepted")
	}
	if files, _ := filepath.Glob(filepath.Join(e.Store.Dir, "layouts", "*")); len(files) != 0 {
		t.Errorf("files written: %v", files)
	}
}

func TestLayoutFollowsEditsAndDelete(t *testing.T) {
	e := startBar(t, t.TempDir())
	if err := e.SaveLayout("mein", ownLayout("Mein")); err != nil {
		t.Fatal(err)
	}
	cfg := PanelConfig{Mode: "design", Brightness: 80, Layout: "mein"}
	if err := e.SetPanelConfig("us88-0", cfg); err != nil {
		t.Fatal(err)
	}
	get := func() PanelConfig {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.cfg.Panels["us88-0"].Clone()
	}
	c := get()
	if c.Layout != "mein" || c.Grid != "4" || len(c.Tiles) != 4 {
		t.Fatalf("not resolved: %+v", c)
	}
	// a scene taken now keeps the reference
	e.mu.Lock()
	e.cfg.Rotation.Scenes = append(e.cfg.Rotation.Scenes, Scene{Name: "S", Panels: map[string]PanelConfig{"us88-0": c.Clone()}})
	e.mu.Unlock()

	// editing the layout reaches the display and the scene
	l := ownLayout("Mein")
	l.Grid = "2+4"
	g, _ := GridTemplateByID("gaming", "clean")
	l.Tiles = g.Tiles
	if err := e.SaveLayout("mein", l); err != nil {
		t.Fatal(err)
	}
	c = get()
	if c.Grid != "2+4" || len(c.Tiles) != 6 || c.Brightness != 80 {
		t.Fatalf("display did not follow: %+v", c)
	}
	e.mu.Lock()
	sc := e.cfg.Rotation.Scenes[len(e.cfg.Rotation.Scenes)-1].Panels["us88-0"]
	e.mu.Unlock()
	if sc.Grid != "2+4" || len(sc.Tiles) != 6 || sc.Layout != "mein" {
		t.Fatalf("scene did not follow: %+v", sc)
	}

	// deleting keeps what was shown, as an own composition
	if err := e.DeleteLayout("mein"); err != nil {
		t.Fatal(err)
	}
	c = get()
	if c.Layout != "" || c.Grid != "2+4" || len(c.Tiles) != 6 {
		t.Fatalf("after delete: %+v", c)
	}
	e.mu.Lock()
	sc = e.cfg.Rotation.Scenes[len(e.cfg.Rotation.Scenes)-1].Panels["us88-0"]
	e.mu.Unlock()
	if sc.Layout != "" || sc.Grid != "2+4" {
		t.Fatalf("scene after delete: %+v", sc)
	}
	// the display is still editable
	c.Brightness = 50
	if err := e.SetPanelConfig("us88-0", c); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutUnknownReference(t *testing.T) {
	e := startBar(t, t.TempDir())
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Design: "loop", Layout: "nope"}); err == nil {
		t.Error("unknown layout without fields accepted")
	}
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Layout: "../x"}); err == nil {
		t.Error("path as layout id accepted")
	}
	g, _ := GridTemplateByID("overview", "clean")
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Grid: g.Grid, Tiles: g.Tiles, Layout: "gone"}); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	c := e.cfg.Panels["us88-0"]
	e.mu.Unlock()
	if c.Layout != "" || c.Grid != "4" {
		t.Fatalf("stale reference kept: %+v", c)
	}
	// a ready-made layout on an upright bar turns its wide fields
	if err := e.SetPanelConfig("us88-0", PanelConfig{Mode: "design", Layout: "gaming-clean", Rotation: 90}); err != nil {
		t.Fatal(err)
	}
}

// The owner's setup before layouts existed: a scene with grid 2+4 and an
// automation rule, normal mode on 1+6. It must load and run unchanged.
func TestLayoutMigrationOfOldConfig(t *testing.T) {
	dir := t.TempDir()
	water, _ := GridTemplateByID("watercooling", "clean")
	game, _ := GridTemplateByID("gaming", "clean")
	old := map[string]any{
		"panels": map[string]any{"us88-0": map[string]any{"mode": "design", "brightness": 100, "grid": water.Grid, "tiles": water.Tiles}},
		"rotation": map[string]any{"scenes": []any{map[string]any{"name": "Gaming 8.8",
			"panels": map[string]any{"us88-0": map[string]any{"mode": "design", "brightness": 100, "grid": game.Grid, "tiles": game.Tiles}}}}},
		"automation": map[string]any{"enabled": true, "rules": []any{map[string]any{"when": "game", "scene": "Gaming 8.8"}}},
	}
	b, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	e := startBar(t, dir)
	var p PanelState
	for _, x := range e.Panels() {
		if x.ID == "us88-0" {
			p = x
		}
	}
	if p.Config.Grid != "1+6" || len(p.Config.Tiles) != 7 || p.Config.Layout != "" {
		t.Fatalf("normal setup changed: %+v", p.Config)
	}
	if err := e.ApplyScene("Gaming 8.8"); err != nil {
		t.Fatal(err)
	}
	for _, x := range e.Panels() {
		if x.ID == "us88-0" && (x.Config.Grid != "2+4" || len(x.Config.Tiles) != 6) {
			t.Fatalf("scene applied wrongly: %+v", x.Config)
		}
	}
	// the old composition can still be edited as before, and saved as a layout
	c := p.Config.Clone()
	c.Brightness = 60
	if err := e.SetPanelConfig("us88-0", c); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveLayout("aus-altem", Layout{Name: "Aus altem", Grid: c.Grid, Tiles: c.Tiles}); err != nil {
		t.Fatal(err)
	}
	c.Layout = "aus-altem"
	if err := e.SetPanelConfig("us88-0", c); err != nil {
		t.Fatal(err)
	}
	// and the file keeps its shape when saved again
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !strings.Contains(string(raw), `"layout": "aus-altem"`) {
		t.Errorf("layout not persisted:\n%s", raw)
	}
}

// The owner's real configuration, as it was written before layouts existed:
// scene "Gaming 8.8" (2+4), automation rule game -> that scene, and the
// everyday composition 1+6 on the 8.8" display.
func TestLayoutOwnerConfigShape(t *testing.T) {
	dir := t.TempDir()
	mk := func(ids ...string) []map[string]any {
		var out []map[string]any
		for _, id := range ids {
			out = append(out, map[string]any{"mode": "design", "design": id})
		}
		return out
	}
	cfg := map[string]any{
		"panels": map[string]any{"us88-0": map[string]any{"mode": "design", "brightness": 90, "grid": "1+6",
			"tiles": mk("card-coolant", "half-cpu-lt", "half-gpu-lt", "half-power-split", "half-gpu-clock", "half-mem", "half-clock")}},
		"rotation": map[string]any{"scenes": []any{map[string]any{"name": "Gaming 8.8", "panels": map[string]any{
			"us88-0": map[string]any{"mode": "design", "brightness": 100, "grid": "2+4",
				"tiles": mk("card-fps-graph", "card-gpu-full", "half-cpu-lt", "half-vram-gb", "half-gpu-clock", "half-lat-cpu")}}}}},
		"automation": map[string]any{"enabled": true, "rules": []any{map[string]any{"when": "game", "scene": "Gaming 8.8"}}},
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	e := startBar(t, dir)
	find := func() PanelConfig {
		for _, x := range e.Panels() {
			if x.ID == "us88-0" {
				return x.Config
			}
		}
		t.Fatal("no us88-0")
		return PanelConfig{}
	}
	c := find()
	if c.Grid != "1+6" || len(c.Tiles) != 7 || c.Layout != "" || c.Tiles[0].Design != "card-coolant" || c.Brightness != 90 {
		t.Fatalf("everyday setup changed: %+v", c)
	}
	if err := e.ApplyScene("Gaming 8.8"); err != nil {
		t.Fatal(err)
	}
	g := find()
	if g.Grid != "2+4" || len(g.Tiles) != 6 || g.Tiles[0].Design != "card-fps-graph" || g.Layout != "" {
		t.Fatalf("scene changed: %+v", g)
	}
	// save the scene's composition as a layout, edit it, and the scene follows
	if err := e.SaveLayout("gaming-88", Layout{Name: "Gaming 8.8", Grid: g.Grid, Tiles: g.Tiles}); err != nil {
		t.Fatal(err)
	}
	g.Layout = "gaming-88"
	if err := e.SetPanelConfig("us88-0", g); err != nil {
		t.Fatal(err)
	}
	l, _ := e.Store.Layout("gaming-88")
	l.Tiles = slices.Clone(l.Tiles)
	l.Tiles[5].Design = "half-cpu-lt"
	if err := e.SaveLayout("gaming-88", l); err != nil {
		t.Fatal(err)
	}
	if find().Tiles[5].Design != "half-cpu-lt" {
		t.Fatalf("display did not follow: %+v", find().Tiles[5])
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !strings.Contains(string(raw), `"scene": "Gaming 8.8"`) || !strings.Contains(string(raw), `"when": "game"`) {
		t.Errorf("automation lost:\n%s", raw)
	}
}
