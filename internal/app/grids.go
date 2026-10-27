package app

import (
	"fmt"
	"image"
	"os"
	"slices"
	"strings"

	"github.com/kewe87/casehud/internal/lcd"
)

// This file holds what makes grid panels work: validation, the ready-made
// compositions and restyling a grid to another design set.

// sizeFor is the canvas the panel is drawn on at the given rotation. A bar
// mounted upright (software quarter turn, 90/270) is a 480x1920 canvas; all
// other panels turn the picture on the device and keep their size.
func (w *worker) sizeFor(rotation int) image.Point {
	sz := w.p.Size()
	if q, ok := w.p.(interface{ SoftQuarterTurns() bool }); ok && q.SoftQuarterTurns() && (rotation == 90 || rotation == 270) {
		return image.Pt(sz.Y, sz.X)
	}
	return sz
}

// usesDesign reports whether the config shows design id (on the panel or in
// one of its fields).
func (c PanelConfig) usesDesign(id string) bool {
	if c.Mode != "design" {
		return false
	}
	if c.Grid == "" {
		return c.Design == id
	}
	return slices.ContainsFunc(c.Tiles, func(t Tile) bool { return t.Mode != "media" && t.Mode != "off" && t.Design == id })
}

// usesMedia reports whether the config shows the media file name as content
// or background.
func (c PanelConfig) usesMedia(name string) bool {
	bg := "media:" + name
	switch {
	case c.Mode == "media" && c.Media == name, c.Background == bg:
		return true
	}
	if c.Mode == "design" && c.Grid != "" {
		return slices.ContainsFunc(c.Tiles, func(t Tile) bool { return t.Background == bg || t.Mode == "media" && t.Media == name })
	}
	return false
}

// cleanGrid validates the grid fields of c.
func (e *Engine) cleanGrid(c PanelConfig) (PanelConfig, error) {
	if c.Grid == "" {
		c.Tiles = nil
		return c, nil
	}
	n := lcd.GridCellCount(c.Grid)
	if n == 0 {
		return c, fmt.Errorf("unknown grid %q", c.Grid)
	}
	if len(c.Tiles) != n {
		return c, fmt.Errorf("grid %q has %d fields, got %d tiles", c.Grid, n, len(c.Tiles))
	}
	for i, t := range c.Tiles {
		switch t.Mode {
		case "":
			t.Mode = "design"
		case "design", "media", "off":
		default:
			return c, fmt.Errorf("tile %d: unknown mode %q", i, t.Mode)
		}
		if t.Theme != "light" && t.Theme != "dark" {
			t.Theme = ""
		}
		if !e.validBackground(t.Background) {
			return c, fmt.Errorf("tile %d: unknown background %q", i, t.Background)
		}
		t.BgStrength = max(0, min(100, t.BgStrength))
		if t.Mode != "design" {
			t.Design = ""
		} else if t.Design != "" && !ValidID(t.Design) {
			return c, fmt.Errorf("tile %d: invalid design id", i)
		}
		if t.Mode != "media" {
			t.Media = ""
		}
		c.Tiles[i] = t
	}
	return c, nil
}

// checkTiles is checkContent for the fields: new references must exist.
func (e *Engine) checkTiles(c, old PanelConfig) error {
	for i, t := range c.Tiles {
		var prev Tile
		if old.Grid == c.Grid && i < len(old.Tiles) {
			prev = old.Tiles[i]
		}
		switch t.Mode {
		case "design":
			if t.Design == "" {
				return fmt.Errorf("tile %d: no design chosen", i)
			}
			if t.Design != prev.Design {
				if _, err := e.Store.Design(t.Design); err != nil {
					return fmt.Errorf("tile %d: %w", i, err)
				}
			}
		case "media":
			p, err := e.Store.MediaPath(t.Media)
			if err != nil {
				return fmt.Errorf("tile %d: %w", i, err)
			}
			if t.Media != prev.Media {
				if _, err := os.Stat(p); err != nil {
					return fmt.Errorf("tile %d: media file %q not found", i, t.Media)
				}
			}
		}
	}
	return nil
}

// DesignFor loads a design for a canvas of the given size: the fixed
// landscape bar designs have a stacked counterpart for the upright bar.
func (s *Store) DesignFor(id string, sz image.Point) (lcd.Layout, error) {
	if !ValidID(id) {
		return lcd.Layout{}, fmt.Errorf("design %q not found", id)
	}
	if IsWide(sz.Y, sz.X) {
		if _, err := lcd.LoadLayout(s.path("designs", id+".json")); err != nil {
			if l, ok := uprightDesign(id); ok {
				return l, nil
			}
		}
	}
	return s.Design(id)
}

// --- ready-made compositions ---

// GridTemplate is a ready-made grid with its fields filled.
type GridTemplate struct {
	ID    string `json:"id"`
	Grid  string `json:"grid"`
	Tiles []Tile `json:"tiles"`
}

// templateSpecs lists, per composition, the grid and a role for each field in
// cell order: "card:x" (water-wide style squares in the clean set, rings in
// the others), "sq:x" (the set's square design) or "half:x" (a half field).
var templateSpecs = []struct {
	id, grid string
	fields   []string
}{
	{"overview", "4", []string{"card:cpu", "card:gpu", "card:coolant", "combo:system"}},
	{"gaming", "2+4", []string{"combo:fps-graph", "combo:gpu-full", "combo:cpu-lt", "combo:vram-gb", "combo:gpu-clock", "combo:lat-cpu"}},
	{"watercooling", "1+6", []string{"sq:coolant", "combo:cpu-lt", "combo:gpu-lt", "combo:power-split", "combo:gpu-clock", "combo:mem", "half:clock"}},
	{"clock", "1+6", []string{"sq:clock", "combo:cpu-lt", "combo:gpu-lt", "combo:coolant-pump", "combo:mem", "combo:power-split", "combo:gpu-clock"}},
}

// squareRoles maps a role to each set's square design.
var squareRoles = map[string]map[string]string{
	"cpu":      {"clean": "cpu-temp", "neon": "neon-cpu", "retro": "retro-cpu", "cyberpunk": "cyber-cpu"},
	"gpu":      {"clean": "gpu-temp", "neon": "neon-gpu", "retro": "retro-gpu", "cyberpunk": "cyber-gpu"},
	"gpu-temp": {"clean": "gpu-temp", "neon": "neon-gpu", "retro": "retro-gpu", "cyberpunk": "cyber-gpu"},
	"coolant":  {"clean": "loop", "neon": "neon-loop", "retro": "retro-loop", "cyberpunk": "cyber-loop"},
	"power":    {"clean": "power-total", "neon": "neon-power-total", "retro": "retro-power-total", "cyberpunk": "cyber-power-total"},
	"fps":      {"clean": "gaming", "neon": "neon-gaming", "retro": "retro-gaming", "cyberpunk": "cyber-gaming"},
	"clock":    {"clean": "clock", "neon": "neon-clock", "retro": "retro-clock", "cyberpunk": "cyber-clock"},
	"vram":     {"clean": "vram", "neon": "neon-vram", "retro": "retro-vram", "cyberpunk": "cyber-vram"},
}

// cardRoles are the water-wide style squares; the other sets use their rings.
var cardRoles = map[string]string{"cpu": "card-cpu", "gpu": "card-gpu", "gpu-temp": "card-gpu", "coolant": "card-coolant", "power": "card-power", "fps": "card-fps", "clock": "card-clock"}

func setPrefix(set string) string {
	for _, s := range tileStyles {
		if s.set == set {
			return s.prefix
		}
	}
	return ""
}

func templateDesign(field, set string) string {
	kind, role, _ := strings.Cut(field, ":")
	switch kind {
	case "combo":
		for _, d := range comboDefs() {
			if d.id == role {
				for _, st := range tileStyles {
					if st.set == set {
						return comboID(st, d)
					}
				}
			}
		}
		return ""
	case "card", "sq":
		if id := cardRoles[role]; id != "" && (set == "clean" || role != "clock") {
			return setPrefix(set) + id
		}
		return squareRoles[role][set]
	}
	return setPrefix(set) + "half-" + role
}

// GridTemplates returns the ready-made compositions in the given design set.
func GridTemplates(set string) []GridTemplate {
	if _, ok := FanSets[set]; !ok {
		set = "clean"
	}
	var out []GridTemplate
	for _, t := range templateSpecs {
		g := GridTemplate{ID: t.id, Grid: t.grid}
		for _, f := range t.fields {
			g.Tiles = append(g.Tiles, Tile{Mode: "design", Design: templateDesign(f, set)})
		}
		out = append(out, g)
	}
	return out
}

// GridTemplateByID returns one ready-made composition.
func GridTemplateByID(id, set string) (GridTemplate, bool) {
	for _, t := range GridTemplates(set) {
		if t.ID == id {
			return t, true
		}
	}
	return GridTemplate{}, false
}

// restyleDesign returns the equivalent of design id in another set; ids it
// does not know (user designs, other shapes) stay as they are.
func restyleDesign(id, set string) string {
	if _, ok := FanSets[set]; !ok {
		return id
	}
	for role, m := range squareRoles {
		for _, v := range m {
			if v == id {
				return squareRoles[role][set]
			}
		}
	}
	for role, id2 := range cardRoles {
		if id2 == id {
			if role == "clock" && set != "clean" {
				return squareRoles[role][set]
			}
			return setPrefix(set) + id
		}
	}
	// half-*, w640-* ... under any set prefix
	rest := id
	for _, p := range []string{"neon-", "retro-", "cyber-"} {
		rest = strings.TrimPrefix(rest, p)
	}
	for _, tag := range []string{"half-", "card-", "w640-", "w960-", "h640-", "h960-"} {
		if strings.HasPrefix(rest, tag) {
			return setPrefix(set) + rest
		}
	}
	return id
}

// restyleTiles points the design fields of c to the equivalent designs of set.
func restyleTiles(c *PanelConfig, set string) {
	c.Layout = "" // no longer the stored composition
	tiles := slices.Clone(c.Tiles)
	for i, t := range tiles {
		if t.Mode == "" || t.Mode == "design" {
			tiles[i].Design = restyleDesign(t.Design, set)
		}
	}
	c.Tiles = tiles
}

// reorientDesign swaps a wide-field design between landscape (w640, w960)
// and upright (h640, h960) form; other designs stay.
func reorientDesign(id string, upright bool) string {
	from, to := [2]string{"w640-", "w960-"}, [2]string{"h640-", "h960-"}
	if !upright {
		from, to = to, from
	}
	for i := range from {
		if before, after, ok := strings.Cut(id, from[i]); ok && (before == "" || strings.HasSuffix(before, "-")) {
			return before + to[i] + after
		}
	}
	return id
}

// reorientTiles adapts the fields of a grid to a changed orientation of the
// bar (rotation 90/270 vs 0/180): wide fields turn into their upright forms.
func reorientTiles(c *PanelConfig, upright bool) {
	tiles := slices.Clone(c.Tiles)
	for i, t := range tiles {
		if t.Mode == "" || t.Mode == "design" {
			tiles[i].Design = reorientDesign(t.Design, upright)
		}
	}
	c.Tiles = tiles
}

// sanitizeLoadedConfig switches off grid fields whose design id is not a valid
// id (a hand-edited config.json), so a bad id never reaches a file path.
func sanitizeLoadedConfig(cfg *Config) {
	fix := func(c PanelConfig) PanelConfig {
		for i, t := range c.Tiles {
			if t.Design != "" && !ValidID(t.Design) {
				c.Tiles[i].Design, c.Tiles[i].Mode = "", "off"
			}
		}
		return c
	}
	for id, c := range cfg.Panels {
		cfg.Panels[id] = fix(c)
	}
	for _, s := range cfg.Rotation.Scenes {
		for id, c := range s.Panels {
			s.Panels[id] = fix(c)
		}
	}
}
