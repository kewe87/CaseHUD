package app

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kewe87/casehud/internal/lcd"
)

// A layout is a named composition for bar displays: a grid and what each of
// its fields shows. Displays refer to a layout by id (PanelConfig.Layout), so
// changing the layout changes every display and scene that uses it. The
// panel keeps the resolved Grid and Tiles next to the id: that is what runs,
// and it is all an older config (or a deleted layout) leaves behind.

// maxLayoutName keeps names readable on a card.
const maxLayoutName = 60

// Layout is one stored or ready-made composition.
type Layout struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Grid    string `json:"grid"`
	Tiles   []Tile `json:"tiles"`
	Builtin bool   `json:"builtin,omitempty"`
	// Tpl and Set name a ready-made layout (the GUI writes its title).
	Tpl string `json:"tpl,omitempty"`
	Set string `json:"set,omitempty"`
}

func (l Layout) clone() Layout {
	l.Tiles = slices.Clone(l.Tiles)
	return l
}

// BuiltinLayouts are the ready-made compositions in every design set.
func BuiltinLayouts() []Layout {
	var out []Layout
	for _, set := range SetOrder {
		for _, t := range GridTemplates(set) {
			out = append(out, Layout{ID: t.ID + "-" + set, Grid: t.Grid, Tiles: t.Tiles, Builtin: true, Tpl: t.ID, Set: set})
		}
	}
	return out
}

func builtinLayout(id string) (Layout, bool) {
	for _, l := range BuiltinLayouts() {
		if l.ID == id {
			return l, true
		}
	}
	return Layout{}, false
}

// Layouts lists the ready-made and the stored layouts (stored ones last, by name).
func (s *Store) Layouts() []Layout {
	out := BuiltinLayouts()
	var mine []Layout
	files, _ := filepath.Glob(s.path("layouts", "*.json"))
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		if !ValidID(id) {
			continue
		}
		if l, err := s.loadLayout(id); err == nil {
			mine = append(mine, l)
		}
	}
	sort.Slice(mine, func(i, j int) bool { return strings.ToLower(mine[i].Name) < strings.ToLower(mine[j].Name) })
	return append(out, mine...)
}

func (s *Store) loadLayout(id string) (Layout, error) {
	b, err := os.ReadFile(s.path("layouts", id+".json"))
	if err != nil {
		return Layout{}, err
	}
	var l Layout
	if err := json.Unmarshal(b, &l); err != nil {
		return Layout{}, err
	}
	l.ID, l.Builtin, l.Tpl, l.Set = id, false, "", ""
	return l, nil
}

// Layout returns a layout by id.
func (s *Store) Layout(id string) (Layout, bool) {
	if !ValidID(id) {
		return Layout{}, false
	}
	if l, ok := builtinLayout(id); ok {
		return l, true
	}
	l, err := s.loadLayout(id)
	return l, err == nil
}

// barPresetSize is the canvas the layouts are drawn on.
var barPresetSize = image.Pt(1920, 480)

// cleanLayout validates l and returns it in storable form.
func (e *Engine) cleanLayout(l Layout) (Layout, error) {
	l = l.clone()
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		return l, fmt.Errorf("a layout needs a name")
	}
	if r := []rune(l.Name); len(r) > maxLayoutName {
		l.Name = string(r[:maxLayoutName])
	}
	if _, ok := lcd.GridPresetByID(barPresetSize.X, barPresetSize.Y, l.Grid); !ok || l.Grid == "" {
		return l, fmt.Errorf("unknown grid %q", l.Grid)
	}
	c, err := e.cleanGrid(PanelConfig{Grid: l.Grid, Tiles: l.Tiles})
	if err != nil {
		return l, err
	}
	if err := e.checkTiles(c, PanelConfig{}); err != nil {
		return l, err
	}
	l.Tiles = c.Tiles
	l.Builtin, l.Tpl, l.Set = false, "", ""
	return l, nil
}

// Layouts lists all layouts.
func (e *Engine) Layouts() []Layout { return e.Store.Layouts() }

// SaveLayout stores a layout of your own and updates every display and scene
// that uses it. The ready-made ones cannot be overwritten.
func (e *Engine) SaveLayout(id string, l Layout) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid layout id %q (use a-z, 0-9 and -)", id)
	}
	if _, ok := builtinLayout(id); ok {
		return fmt.Errorf("layout %q is ready-made and cannot be changed: customize a copy", id)
	}
	l, err := e.cleanLayout(l)
	if err != nil {
		return err
	}
	l.ID = id
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	e.setMu.Lock()
	defer e.setMu.Unlock()
	dir := e.Store.path("layouts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	e.refreshLayoutUsers(id, l)
	return nil
}

// refreshLayoutUsers brings the displays and scenes that use layout id up to
// date (setMu held). A display that cannot take the new composition keeps its
// old one; the error is only logged.
func (e *Engine) refreshLayoutUsers(id string, l Layout) {
	e.mu.Lock()
	var users []string
	for pid, c := range e.cfg.Panels {
		if c.Layout == id {
			users = append(users, pid)
		}
	}
	for _, s := range e.cfg.Rotation.Scenes {
		for pid, c := range s.Panels {
			if c.Layout == id {
				c.Grid, c.Tiles = l.Grid, slices.Clone(l.Tiles)
				s.Panels[pid] = c
			}
		}
	}
	e.mu.Unlock()
	sort.Strings(users)
	for _, pid := range users {
		e.mu.Lock()
		c := e.cfg.Panels[pid].Clone()
		e.mu.Unlock()
		if err := e.setPanelConfigLocked(pid, c); err != nil {
			log.Printf("layout %s: display %s: %v", id, pid, err)
		}
	}
	_ = e.persist()
}

// DeleteLayout removes a layout of your own. Displays and scenes that used it
// keep what it showed (the resolved fields) as their own, unsaved layout:
// nothing goes dark because a list entry disappeared.
func (e *Engine) DeleteLayout(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid layout id")
	}
	if _, ok := builtinLayout(id); ok {
		return fmt.Errorf("layout %q is ready-made and cannot be deleted", id)
	}
	e.setMu.Lock()
	defer e.setMu.Unlock()
	if err := os.Remove(e.Store.path("layouts", id+".json")); err != nil {
		return err
	}
	e.mu.Lock()
	for pid, c := range e.cfg.Panels {
		if c.Layout == id {
			c.Layout = ""
			e.cfg.Panels[pid] = c
		}
	}
	for _, s := range e.cfg.Rotation.Scenes {
		for pid, c := range s.Panels {
			if c.Layout == id {
				c.Layout = ""
				s.Panels[pid] = c
			}
		}
	}
	e.mu.Unlock()
	return e.persist()
}

// resolveLayout fills the grid fields of c from its layout. A layout that no
// longer exists is dropped when the panel still carries fields (its last
// composition stays); without fields it is an error.
func (e *Engine) resolveLayout(c PanelConfig) (PanelConfig, error) {
	if c.Layout == "" {
		return c, nil
	}
	if !ValidID(c.Layout) {
		return c, fmt.Errorf("invalid layout id %q", c.Layout)
	}
	l, ok := e.Store.Layout(c.Layout)
	if !ok {
		if c.Grid == "" {
			return c, fmt.Errorf("layout %q not found", c.Layout)
		}
		c.Layout = ""
		return c, nil
	}
	c.Grid, c.Tiles = l.Grid, slices.Clone(l.Tiles)
	return c, nil
}

// RenderLayout draws a composition as a still picture of the given width
// (the bar's 4:1 shape), with the current sensor values. theme is the
// panel's scheme for fields that have none of their own.
func (e *Engine) RenderLayout(grid string, tiles []Tile, theme string, width int) (*image.RGBA, error) {
	g, ok := lcd.GridPresetByID(barPresetSize.X, barPresetSize.Y, grid)
	if !ok || len(g.Cells) != len(tiles) {
		return nil, fmt.Errorf("grid %q does not fit", grid)
	}
	width = max(240, min(width, 1920))
	f := float64(width) / float64(barPresetSize.X)
	img := image.NewRGBA(image.Rect(0, 0, width, width*barPresetSize.Y/barPresetSize.X))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xff
	}
	for i, cell := range g.Cells {
		rect := image.Rect(int(float64(cell.X)*f), int(float64(cell.Y)*f), int(float64(cell.X+cell.W)*f), int(float64(cell.Y+cell.H)*f))
		tc := tiles[i].config(theme)
		if tc.Mode != "design" || !ValidID(tc.Design) {
			continue
		}
		l, err := e.Store.DesignFor(tc.Design, image.Pt(cell.W, cell.H))
		if err != nil {
			continue
		}
		l = lcd.ApplyTheme(l, tc.Theme)
		part, err := e.RenderDesignWithBackground(l, rect.Dx(), rect.Dy(), tc.Background, tc.BgStrength, tc.BgBloom)
		if err != nil {
			continue
		}
		draw.Draw(img, rect, part, part.Bounds().Min, draw.Src)
	}
	return img, nil
}
