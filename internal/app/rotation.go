package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

// Scene is a saved assignment of contents to displays.
type Scene struct {
	Name   string                 `json:"name"`
	Panels map[string]PanelConfig `json:"panels"`
	// NoRotation keeps the scene out of the rotation (it stays available to
	// the automation, applying and the API).
	NoRotation bool `json:"no_rotation,omitempty"`
}

// rotationScenes are the scenes the rotation cycles through.
func rotationScenes(all []Scene) []Scene {
	out := make([]Scene, 0, len(all))
	for _, s := range all {
		if !s.NoRotation {
			out = append(out, s)
		}
	}
	return out
}

// RotationConfig makes the displays change by themselves.
type RotationConfig struct {
	Enabled bool `json:"enabled"`
	// Mode "scenes" cycles saved scenes; "conveyor" moves a list of designs
	// down through the displays, a new one entering at the top.
	Mode      string   `json:"mode"`
	IntervalS int      `json:"interval_s"`
	Effect    string   `json:"effect"`  // see Transitions
	Cascade   bool     `json:"cascade"` // change displays one after another, top to bottom
	Scenes    []Scene  `json:"scenes,omitempty"`
	Conveyor  []string `json:"conveyor,omitempty"` // design ids
	Order     []string `json:"order,omitempty"`    // display ids top to bottom ("" = GUI order)
	// PauseGaming and GamingScene are the pre-automation gaming options. They
	// are still read (and turned into an automation rule, see
	// absorbLegacyGaming) but no longer written.
	PauseGaming bool   `json:"pause_gaming,omitempty"`
	GamingScene string `json:"gaming_scene,omitempty"`
}

const (
	minRotationInterval = 5 // the shortest option in the GUI
	cascadeStep         = 180 * time.Millisecond
	gamingHold          = 5 * time.Second // FPS seen this long = a game is running
)

// rotationState is what the rotation loop remembers.
type rotationState struct {
	active    bool      // rotation currently overrides the displays
	step      int       // scene index or conveyor offset
	lastStep  time.Time // when the displays last changed
	shown     string    // name of the scene on screen ("" for conveyor)
	transient string    // scene put on screen by a one-off activation (not saved)
	fpsSince  time.Time // first FPS reading of the current streak
	skip      chan struct{}
	gamingNow bool   // a game is running (set by the automation tick)
	jump      string // scene to show at the next step, set by ActivateScene
}

// Rotation returns the rotation settings.
func (e *Engine) Rotation() RotationConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.cfg.Rotation
	if r.Mode == "" {
		r.Mode = "scenes"
	}
	if r.IntervalS == 0 {
		r.IntervalS = 15
	}
	if r.Effect == "" {
		r.Effect = "fade"
	}
	return r
}

// RotationStatus tells the GUI what the rotation is doing.
type RotationStatus struct {
	Active bool   `json:"active"`
	Scene  string `json:"scene,omitempty"`
	Gaming bool   `json:"gaming"`
}

func (e *Engine) RotationStatus() RotationStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return RotationStatus{Active: e.rot.active, Scene: e.rot.shown, Gaming: e.rot.gamingNow}
}

// Scene limits.
const (
	maxScenes        = 50
	maxSceneNameRune = 64
)

// cleanSceneName trims a scene name and drops control characters; it must not
// end up empty or longer than 64 characters.
func cleanSceneName(n string) (string, error) {
	n = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, n))
	if n == "" {
		return "", fmt.Errorf("please name the scene")
	}
	if utf8.RuneCountInString(n) > maxSceneNameRune {
		return "", fmt.Errorf("scene names are limited to %d characters", maxSceneNameRune)
	}
	return n, nil
}

// SetRotation validates and stores rotation settings; they take effect at
// once.
func (e *Engine) SetRotation(r RotationConfig) error {
	e.rotMu.Lock()
	defer e.rotMu.Unlock()
	return e.setRotation(r)
}

func (e *Engine) setRotation(r RotationConfig) error {
	switch r.Mode {
	case "scenes", "conveyor":
	default:
		return fmt.Errorf("unknown rotation mode %q", r.Mode)
	}
	if r.IntervalS < minRotationInterval {
		r.IntervalS = minRotationInterval
	}
	if !validTransition(r.Effect) {
		return fmt.Errorf("unknown effect %q", r.Effect)
	}
	if len(r.Scenes) > maxScenes {
		return fmt.Errorf("at most %d scenes", maxScenes)
	}
	names := map[string]bool{}
	// Fresh slice and maps: readers hold the old ones.
	r.Scenes = slices.Clone(r.Scenes)
	for i, s := range r.Scenes {
		name, err := cleanSceneName(s.Name)
		if err != nil {
			return err
		}
		if names[name] {
			return fmt.Errorf("scene names must be unique and not empty")
		}
		names[name] = true
		r.Scenes[i].Name = name
		s.Name = name
		m := make(map[string]PanelConfig, len(s.Panels))
		for id, c := range s.Panels {
			nc, err := e.cleanPanelConfig(c)
			if err != nil {
				return fmt.Errorf("scene %q: %w", s.Name, err)
			}
			m[id] = nc
		}
		r.Scenes[i].Panels = m
	}
	e.mu.Lock()
	absorbLegacyGaming(&e.cfg, &r)
	e.cfg.Rotation = r
	// No auto.dirty here: the automation notices a changed scene by its content signature.
	e.rot.lastStep = time.Time{} // apply right away
	e.mu.Unlock()
	e.kickRotation()
	return e.persist()
}

// SaveScene stores what the displays are configured to show as scene name
// (replacing a scene with that name).
func (e *Engine) SaveScene(name string) error { return e.SaveSceneWith(name, nil) }

// SaveSceneWith is SaveScene that can also set whether the scene stays out of
// the rotation (nil keeps the flag of a scene being replaced).
func (e *Engine) SaveSceneWith(name string, noRotation *bool) error {
	name, err := cleanSceneName(name)
	if err != nil {
		return err
	}
	e.rotMu.Lock()
	defer e.rotMu.Unlock()
	e.mu.Lock()
	panels := map[string]PanelConfig{}
	for id, c := range e.cfg.Panels {
		if _, ok := e.workers[id]; ok {
			panels[id] = c.Clone()
		}
	}
	e.mu.Unlock()
	return e.putScene(Scene{Name: name, Panels: panels}, noRotation)
}

// putScene stores sc, replacing a scene of that name (which keeps its
// no-rotation flag unless noRotation is given). The caller holds rotMu.
func (e *Engine) putScene(sc Scene, noRotation *bool) error {
	e.mu.Lock()
	r := e.cfg.Rotation
	r.Scenes = slices.Clone(r.Scenes) // Rotation() hands out the old slice
	sc.NoRotation = noRotation != nil && *noRotation
	if i := slices.IndexFunc(r.Scenes, func(s Scene) bool { return s.Name == sc.Name }); i >= 0 {
		if noRotation == nil {
			sc.NoRotation = r.Scenes[i].NoRotation
		}
		r.Scenes[i] = sc
	} else {
		r.Scenes = append(r.Scenes, sc)
	}
	if r.Mode == "" {
		r.Mode = "scenes"
	}
	if r.Effect == "" {
		r.Effect = "fade"
	}
	if r.IntervalS == 0 {
		r.IntervalS = 15
	}
	e.mu.Unlock()
	return e.setRotation(r)
}

// NextRotation skips to the next scene or conveyor step now.
func (e *Engine) NextRotation() {
	e.mu.Lock()
	e.rot.lastStep = time.Time{} // due now; rotate advances the step
	e.mu.Unlock()
	e.kickRotation()
}

func (e *Engine) kickRotation() {
	select {
	case e.rot.skip <- struct{}{}:
	default:
	}
}

// isGaming reports whether FPS readings have been coming in for a while.
func (e *Engine) isGaming(now time.Time) bool {
	r, ok := metrics.Eval(e.prov, srcFPS)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !ok || r.Value <= 0 {
		e.rot.fpsSince = time.Time{}
		return false
	}
	if e.rot.fpsSince.IsZero() {
		e.rot.fpsSince = now
	}
	return now.Sub(e.rot.fpsSince) >= gamingHold
}

func (e *Engine) runRotation(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		now := e.Clock()
		e.automate(now)
		e.rotate(now)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-e.rot.skip:
		}
	}
}

// panelOrder is the display order for cascades and the conveyor.
func (e *Engine) panelOrder(r RotationConfig) []string {
	var out []string
	for _, id := range r.Order {
		if _, ok := e.workers[id]; ok && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	for _, id := range e.order {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// rotate advances the rotation if it is due.
func (e *Engine) rotate(now time.Time) {
	r := e.Rotation()
	r.Scenes = rotationScenes(r.Scenes)
	e.mu.Lock()
	if e.auto.key != "" {
		e.mu.Unlock() // the automation holds the displays
		return
	}
	n := len(r.Scenes)
	if r.Mode == "conveyor" {
		n = len(r.Conveyor)
	}
	if !r.Enabled || n == 0 || (r.Mode == "scenes" && n < 2) {
		wasActive := e.rot.active
		e.rot.active, e.rot.shown, e.rot.jump = false, "", ""
		restore := map[*worker]PanelConfig{}
		if wasActive {
			for id, w := range e.workers {
				restore[w] = e.cfg.Panels[id].Clone()
			}
		}
		e.mu.Unlock()
		for w, c := range restore {
			w.switchTo(c, "fade", 0)
		}
		return
	}
	order := e.panelOrder(r)
	due := !e.rot.active || now.Sub(e.rot.lastStep) >= time.Duration(r.IntervalS)*time.Second
	type change struct {
		w     *worker
		c     PanelConfig
		delay time.Duration
	}
	var changes []change
	effect := r.Effect
	delayFor := func(i int) time.Duration {
		if r.Cascade {
			return time.Duration(i) * cascadeStep
		}
		return 0
	}
	switch {
	case r.Mode == "scenes":
		var target *Scene
		if due {
			if i := slices.IndexFunc(r.Scenes, func(s Scene) bool { return s.Name == e.rot.jump }); e.rot.jump != "" && i >= 0 {
				e.rot.step = i
			} else if e.rot.active {
				e.rot.step++
			}
			e.rot.jump = ""
			target = &r.Scenes[((e.rot.step%n)+n)%n]
		}
		if target != nil {
			for i, id := range order {
				if c, ok := target.Panels[id]; ok {
					changes = append(changes, change{e.workers[id], c, delayFor(i)})
				}
			}
			e.rot.shown, e.rot.transient = target.Name, ""
		}
	case r.Mode == "conveyor" && due:
		if e.rot.active {
			e.rot.step++
		}
		if r.Effect == "fade" || r.Effect == "" {
			effect = "slide-down" // the conveyor reads best as content moving down
		}
		// Every display is a slot; a grid display has one slot per field, in
		// field order, so the designs keep sliding on through its fields.
		slot := 0
		pick := func(s int) string { return r.Conveyor[(((e.rot.step-s)%n)+n)%n] }
		for i, id := range order {
			c := e.cfg.Panels[id].Clone()
			c.Mode = "design"
			if c.Grid != "" {
				c.Layout = ""
				for k := range c.Tiles {
					c.Tiles[k].Mode, c.Tiles[k].Design = "design", pick(slot)
					c.Tiles[k].Media = ""
					slot++
				}
			} else {
				c.Design = pick(slot)
				slot++
			}
			changes = append(changes, change{e.workers[id], c, delayFor(i)})
		}
		e.rot.shown = ""
	}
	if len(changes) > 0 {
		e.rot.active, e.rot.lastStep = true, now
	}
	e.mu.Unlock()
	for _, ch := range changes {
		ch.w.switchTo(ch.c, effect, ch.delay)
	}
}

// validBackground reports whether bg names an effect or a media file.
func (e *Engine) validBackground(bg string) bool {
	if bg == "" || slices.Contains(lcd.BackgroundEffects, bg) {
		return true
	}
	if name, ok := strings.CutPrefix(bg, "media:"); ok {
		_, err := e.Store.MediaPath(name)
		return err == nil
	}
	return false
}
