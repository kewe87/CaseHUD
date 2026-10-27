package app

import (
	"fmt"
	"slices"
	"time"
)

// This file holds the controls the external API drives: the "game running"
// hint, the global power switch, scene activation and the rotation switch.

// gamingHint is a game report from outside (a launcher or a TV shell). It
// expires with its lease so a crashed reporter cannot pin the displays.
type gamingHint struct {
	active bool
	game   string
	pid    int
	until  time.Time
}

// Lease limits for SetGamingHint.
const (
	DefaultGamingLease = 120 * time.Second
	minGamingLease     = 10 * time.Second
	maxGamingLease     = time.Hour
)

// GamingStatus tells whether a game is running and who says so.
type GamingStatus struct {
	Active bool
	Source string // "external" (API hint), "fps" (FPS readings) or ""
	Game   string // executable or name of the game, if known
}

// Clock is the current time (Now, or the real clock).
func (e *Engine) Clock() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// SetGamingHint reports that a game starts (active) or ends. It takes effect
// at once, without the hold the FPS detection needs, and lapses after lease
// (default 120 s, limited to 10 s..1 h) unless reported again.
func (e *Engine) SetGamingHint(active bool, game string, pid int, lease time.Duration) {
	if lease <= 0 {
		lease = DefaultGamingLease
	}
	lease = max(minGamingLease, min(maxGamingLease, lease))
	e.mu.Lock()
	if active {
		e.gamingHint = gamingHint{active: true, game: game, pid: pid, until: e.Clock().Add(lease)}
	} else {
		e.gamingHint = gamingHint{}
	}
	e.mu.Unlock()
	e.kickRotation()
}

// Gaming combines the external hint (wins) with the FPS detection.
func (e *Engine) Gaming(now time.Time) GamingStatus {
	fps := e.isGaming(now) // always, so the FPS hold keeps counting under a hint
	e.mu.Lock()
	h := e.gamingHint
	if h.active && !now.Before(h.until) {
		e.gamingHint = gamingHint{} // lease ran out
		h = gamingHint{}
	}
	e.mu.Unlock()
	if h.active {
		return GamingStatus{Active: true, Source: "external", Game: h.game}
	}
	if fps {
		return GamingStatus{Active: true, Source: "fps", Game: e.prov.GameName()}
	}
	return GamingStatus{}
}

// SetPower switches all displays off (false) or back on (true). The state
// is not saved: after a restart the displays are on.
func (e *Engine) SetPower(on bool) {
	e.mu.Lock()
	e.powerOff = !on
	e.mu.Unlock()
	e.syncSleep()
}

// Power reports whether the displays are switched on.
func (e *Engine) Power() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.powerOff
}

// SceneState is a saved scene and whether the rotation shows it now.
type SceneState struct {
	Name   string
	Active bool
}

// Scenes lists the saved scenes.
func (e *Engine) Scenes() []SceneState {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]SceneState, 0, len(e.cfg.Rotation.Scenes))
	for _, s := range e.cfg.Rotation.Scenes {
		out = append(out, SceneState{Name: s.Name, Active: (e.rot.active && e.rot.shown == s.Name && e.auto.key == "") || (e.auto.key != "" && e.auto.scene == s.Name) ||
			(e.auto.key == "" && !e.rot.active && e.rot.transient == s.Name)})
	}
	return out
}

// ErrNoScene and ErrGamingHold tell ActivateScene why it did not act.
var (
	ErrNoScene    = fmt.Errorf("scene not found")
	ErrGamingHold = fmt.Errorf("the displays are held while the automation shows a scene")
)

// ActivateScene shows scene name now. While the rotation runs in scenes mode
// and the scene is part of it (with at least two scenes in the rotation),
// the rotation jumps to it and restarts its interval. In every other case
// (rotation off, other mode, a single scene, a scene kept out of the
// rotation) the scene is applied: with persist as the current setup, like
// "Apply" in the GUI (replacing the displays' saved settings); without, it is
// only shown for now, until the user changes a display, the automation or the
// rotation takes over, or another scene is activated, and the saved settings
// stay as they are. It does not matter whether the scene is part of the
// rotation.
func (e *Engine) ActivateScene(name string, persist bool) error {
	e.mu.Lock()
	r := e.cfg.Rotation
	if !slices.ContainsFunc(r.Scenes, func(s Scene) bool { return s.Name == name }) {
		e.mu.Unlock()
		return ErrNoScene
	}
	if e.auto.key != "" {
		e.mu.Unlock()
		return ErrGamingHold
	}
	if !r.Enabled || (r.Mode != "" && r.Mode != "scenes") || len(rotationScenes(r.Scenes)) < 2 ||
		slices.ContainsFunc(r.Scenes, func(s Scene) bool { return s.Name == name && s.NoRotation }) {
		if !persist {
			return e.showSceneNow(name)
		}
		e.mu.Unlock()
		if err := e.ApplyScene(name); err != nil {
			return err
		}
		// A running rotation keeps its own content on screen: show the scene
		// there too and let the rotation carry on after one interval.
		e.mu.Lock()
		type change struct {
			w *worker
			c PanelConfig
		}
		var changes []change
		if e.rot.active && e.auto.key == "" {
			if i := slices.IndexFunc(e.cfg.Rotation.Scenes, func(s Scene) bool { return s.Name == name }); i >= 0 {
				for id, c := range e.cfg.Rotation.Scenes[i].Panels {
					if w := e.workers[id]; w != nil {
						changes = append(changes, change{w, c.Clone()})
					}
				}
				e.rot.shown, e.rot.lastStep = name, e.Clock()
			}
		}
		e.mu.Unlock()
		for _, ch := range changes {
			ch.w.switchTo(ch.c, "fade", 0)
		}
		return nil
	}
	e.rot.jump = name
	e.rot.lastStep = time.Time{}
	e.mu.Unlock()
	e.kickRotation()
	return nil
}

// showSceneNow shows scene name on the displays without saving it as their
// configuration. e.mu is held on entry and released here.
func (e *Engine) showSceneNow(name string) error {
	type change struct {
		w *worker
		c PanelConfig
	}
	var changes []change
	i := slices.IndexFunc(e.cfg.Rotation.Scenes, func(s Scene) bool { return s.Name == name })
	if i < 0 {
		e.mu.Unlock()
		return ErrNoScene
	}
	for id, c := range e.cfg.Rotation.Scenes[i].Panels {
		if w := e.workers[id]; w != nil {
			changes = append(changes, change{w, c.Clone()})
		}
	}
	if e.rot.active {
		// A running rotation carries on after one interval.
		e.rot.shown, e.rot.lastStep = name, e.Clock()
	} else {
		e.rot.transient = name
	}
	e.mu.Unlock()
	for _, ch := range changes {
		ch.w.switchTo(ch.c, "fade", 0)
	}
	return nil
}

// SetRotationEnabled switches the rotation on or off, keeping the rest of
// its settings.
func (e *Engine) SetRotationEnabled(on bool) error {
	e.rotMu.Lock()
	defer e.rotMu.Unlock()
	r := e.Rotation()
	// Rotation() copies shallowly and SetRotation rewrites scene maps: give it its own.
	r.Scenes = slices.Clone(r.Scenes)
	for i, s := range r.Scenes {
		m := make(map[string]PanelConfig, len(s.Panels))
		for k, v := range s.Panels {
			m[k] = v.Clone()
		}
		r.Scenes[i].Panels = m
	}
	r.Enabled = on
	return e.setRotation(r)
}

// SetAwtrixEnabled enables or disables all configured AWTRIX clocks.
func (e *Engine) SetAwtrixEnabled(on bool) error {
	e.awCfgMu.Lock()
	defer e.awCfgMu.Unlock()
	e.mu.Lock()
	ds := slices.Clone(e.cfg.Awtrix)
	e.mu.Unlock()
	for i := range ds {
		ds[i].Disabled = !on
	}
	return e.setAwtrixDevices(ds)
}

// SetOrder lists the design sets in the order the GUI offers them.
var SetOrder = []string{"clean", "neon", "retro", "cyberpunk"}

// squareDesign is the set's design for a square (non-round) display, such as
// the pump head; the sets are drawn for round fans.
var squareDesign = map[string]string{
	"clean": "water-cooling", "neon": "neon-cpu-gpu", "retro": "retro-cpu-gpu", "cyberpunk": "cyber-cpu-gpu",
}

// wideDesign is the set's design for a bar display (the 1920x480 screen).
var wideDesign = map[string]string{"neon": "neon-wide", "retro": "retro-wide", "cyberpunk": "cyber-wide"}

// ErrNoPanel is returned for a display that is not connected.
var ErrNoPanel = fmt.Errorf("panel not connected")

// ErrUnknownSet is returned for a design set that does not exist.
var ErrUnknownSet = fmt.Errorf("unknown design set")

// SetDesign picks the design of set for the i-th round display (CPU, GPU,
// coolant, repeating, like the GUI's "style for all fans") or, for a square
// display, the set's combined design.
func SetDesign(set string, round bool, i int) (string, error) {
	return setDesign(set, round, false, i)
}

// setDesign is SetDesign that also knows bar displays (the 1920x480 screen),
// where the clean set has its own wide dashboard.
func setDesign(set string, round, wide bool, i int) (string, error) {
	trio, ok := FanSets[set]
	if !ok {
		return "", ErrUnknownSet
	}
	if wide {
		if set == "clean" {
			return "water-wide", nil
		}
		return wideDesign[set], nil
	}
	if !round {
		return squareDesign[set], nil
	}
	return trio[i%len(trio)], nil
}

// ApplySet shows the set (and theme "dark" or "light") on the displays in
// ids, or on all if ids is empty, and returns the ids it changed.
func (e *Engine) ApplySet(set, theme string, ids []string) ([]string, error) {
	if _, ok := FanSets[set]; !ok {
		return nil, ErrUnknownSet
	}
	if theme != "dark" && theme != "light" {
		return nil, fmt.Errorf("unknown theme %q", theme)
	}
	var targets []PanelState
	for _, p := range e.Panels() {
		if len(ids) == 0 || slices.Contains(ids, p.ID) {
			targets = append(targets, p)
		}
	}
	for _, id := range ids {
		if !slices.ContainsFunc(targets, func(p PanelState) bool { return p.ID == id }) {
			return nil, fmt.Errorf("%w: %q", ErrNoPanel, id)
		}
	}
	var done []string
	round := 0
	for _, p := range targets {
		// IsBar: an upright bar (rotation 90/270) reports the turned size
		// (480x1920); the renderer stacks the wide design for it.
		d, _ := setDesign(set, p.Round, IsBar(p.Width, p.Height), round)
		if p.Round {
			round++
		}
		_, err := e.UpdatePanelConfig(p.ID, func(c *PanelConfig) error {
			c.Mode = "design"
			if c.Grid != "" {
				// A grid stays; every field gets the set's equivalent design.
				restyleTiles(c, set)
			} else {
				c.Design = d
			}
			c.Theme = ""
			if theme == "light" {
				c.Theme = "light"
			}
			return nil
		})
		if err != nil {
			return done, err
		}
		done = append(done, p.ID)
	}
	return done, nil
}
