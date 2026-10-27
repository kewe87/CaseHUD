package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
)

// Automation switches to a saved scene by itself while a condition holds
// (today: a game is running) and returns to the normal setup afterwards. It
// is independent of the rotation; the rotation merely pauses while a rule is
// active and resumes when it ends.
//
// Rules are checked top to bottom, the first match wins. A rule needs to
// match for EnterS seconds before it takes over and to stop matching for
// LeaveS seconds before it lets go, so Alt-Tab does not make the displays
// flicker. Manual changes to a display are saved and shown at once; the
// automation does not undo them while the same rule stays active, and the
// normal setup (with the changes) comes back when the rule ends.
//
// Not offered: a "media is playing" condition. CaseHUD has no now-playing
// detection to base it on.

// AutomationRule is one "when ... show scene ..." line.
type AutomationRule struct {
	// When is "game" (any game) or "game:<exe>" (one game, e.g.
	// "game:eldenring.exe", compared case-insensitively).
	When string `json:"when"`
	// Scene is the scene to show; "" keeps the normal setup (and pauses the
	// rotation) while the rule matches.
	Scene string `json:"scene,omitempty"`
	// Off disables the rule without deleting it.
	Off bool `json:"off,omitempty"`
}

// AutomationConfig holds the automation settings.
type AutomationConfig struct {
	Enabled bool             `json:"enabled"`
	Rules   []AutomationRule `json:"rules,omitempty"`
	// EnterS and LeaveS are the debounce times in seconds (0 = default).
	EnterS int `json:"enter_s,omitempty"`
	LeaveS int `json:"leave_s,omitempty"`
	// IgnoreGames lists executables (e.g. "slack.exe") that are never
	// treated as games, on top of the built-in list of desktop programs.
	IgnoreGames []string `json:"ignore_games,omitempty"`
}

// Automation defaults and limits.
const (
	DefaultAutoEnterS = 3
	DefaultAutoLeaveS = 10
	maxAutoDelayS     = 300
	maxAutoRules      = 20
	maxIgnoreGames    = 100
)

// AutomationStatus tells what the automation is doing.
type AutomationStatus struct {
	Enabled bool `json:"enabled"`
	// ActiveRule is the index of the rule in charge, -1 for none.
	ActiveRule int    `json:"activeRule"`
	When       string `json:"when,omitempty"`
	Scene      string `json:"scene,omitempty"`
	Game       string `json:"game,omitempty"`
	// Pending is the rule index about to take over (-1 none, -2 about to
	// let go): the debounce is running.
	Pending int `json:"pending"`
}

// autoState is what the automation remembers (under Engine.mu).
type autoState struct {
	key       string // identity of the rule in charge ("" none): when + NUL + scene
	idx       int
	when      string
	scene     string
	game      string
	cand      string // key the debounce is counting for
	candIdx   int
	candSince time.Time
	candSet   bool
	dirty     bool   // settings or scenes changed: check the active rule again
	sig       string // content signature of the scene the active rule applied
	applied   int    // how often a rule took effect (tests)
}

// sceneSig identifies what a scene shows (its displays' settings), so the
// automation can tell a real change from an unrelated edit.
func sceneSig(s Scene) string {
	b, err := json.Marshal(s.Panels)
	if err != nil {
		return fmt.Sprint(s.Panels)
	}
	return string(b)
}

func ruleKey(r AutomationRule) string { return r.When + "\x00" + r.Scene }

func (a AutomationConfig) enter() time.Duration {
	if a.EnterS <= 0 {
		return DefaultAutoEnterS * time.Second
	}
	return time.Duration(a.EnterS) * time.Second
}

func (a AutomationConfig) leave() time.Duration {
	if a.LeaveS <= 0 {
		return DefaultAutoLeaveS * time.Second
	}
	return time.Duration(a.LeaveS) * time.Second
}

// Automation returns the automation settings (with defaults filled in).
func (e *Engine) Automation() AutomationConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.cfg.Automation
	a.Rules = slices.Clone(a.Rules)
	a.IgnoreGames = slices.Clone(a.IgnoreGames)
	if a.EnterS == 0 {
		a.EnterS = DefaultAutoEnterS
	}
	if a.LeaveS == 0 {
		a.LeaveS = DefaultAutoLeaveS
	}
	return a
}

// AutomationStatus reports the rule in charge.
func (e *Engine) AutomationStatus() AutomationStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := AutomationStatus{Enabled: e.cfg.Automation.Enabled, ActiveRule: -1, Pending: -1}
	if e.auto.key != "" {
		st.ActiveRule, st.When, st.Scene, st.Game = e.auto.idx, e.auto.when, e.auto.scene, e.auto.game
	}
	if e.auto.candSet {
		st.Pending = e.auto.candIdx
		if e.auto.cand == "" {
			st.Pending = -2
		}
	}
	return st
}

// ValidAutomationWhen reports whether w is a condition the engine knows.
func ValidAutomationWhen(w string) bool {
	if w == "game" {
		return true
	}
	exe, ok := strings.CutPrefix(w, "game:")
	return ok && strings.TrimSpace(exe) != "" && len(exe) <= 128 && !strings.ContainsAny(exe, `/\`)
}

// SetAutomation validates and stores the automation settings; they take
// effect at once. A rule naming a scene that does not exist (any more) is
// kept but ignored until the scene is back, and the GUI flags it.
func (e *Engine) SetAutomation(a AutomationConfig) error {
	if len(a.Rules) > maxAutoRules {
		return fmt.Errorf("at most %d rules", maxAutoRules)
	}
	a.Rules = slices.Clone(a.Rules)
	for i := range a.Rules {
		w := strings.TrimSpace(a.Rules[i].When)
		if exe, ok := strings.CutPrefix(w, "game:"); ok {
			w = "game:" + strings.TrimSpace(exe)
		}
		a.Rules[i].When = w
		if !ValidAutomationWhen(w) {
			return fmt.Errorf("rule %d: unknown condition %q", i+1, w)
		}
		if a.Rules[i].Scene != "" {
			sc, err := cleanSceneName(a.Rules[i].Scene)
			if err != nil {
				return fmt.Errorf("rule %d: %w", i+1, err)
			}
			a.Rules[i].Scene = sc
		}
	}
	if a.EnterS < 0 || a.EnterS > maxAutoDelayS || a.LeaveS < 0 || a.LeaveS > maxAutoDelayS {
		return fmt.Errorf("delays must be 0 (default) to %d seconds", maxAutoDelayS)
	}
	a.IgnoreGames = cleanIgnoreGames(a.IgnoreGames)
	if len(a.IgnoreGames) > maxIgnoreGames {
		return fmt.Errorf("at most %d ignored programs", maxIgnoreGames)
	}
	e.mu.Lock()
	e.cfg.Automation = a
	e.auto.dirty = true
	e.mu.Unlock()
	e.prov.SetGameIgnore(a.IgnoreGames)
	e.kickRotation()
	return e.persist()
}

// cleanIgnoreGames trims, lower-cases and de-duplicates executable names and
// drops paths (only the file name counts).
func cleanIgnoreGames(in []string) []string {
	var out []string
	for _, n := range in {
		n = strings.ToLower(strings.TrimSpace(n))
		if i := strings.LastIndexAny(n, `/\`); i >= 0 {
			n = n[i+1:]
		}
		if n != "" && len(n) <= 128 && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// migrateLegacyGaming turns the old rotation options (pause while gaming,
// gaming scene) found in a loaded config into an automation rule.
func migrateLegacyGaming(c *Config) {
	r := c.Rotation
	absorbLegacyGaming(c, &r)
	c.Rotation = r
}

// absorbLegacyGaming clears r's PauseGaming/GamingScene and, if the
// automation has no "game" rule yet, adds one that does the same. The
// automation is switched on with a migrated rule (and only the first time,
// see Config.GamingMigrated).
func absorbLegacyGaming(c *Config, r *RotationConfig) {
	if !r.PauseGaming && r.GamingScene == "" {
		return
	}
	scene := r.GamingScene
	if scene != "" && !slices.ContainsFunc(r.Scenes, func(s Scene) bool { return s.Name == scene }) {
		scene = ""
	}
	if r.PauseGaming && !c.GamingMigrated &&
		!slices.ContainsFunc(c.Automation.Rules, func(x AutomationRule) bool { return x.When == "game" }) &&
		len(c.Automation.Rules) < maxAutoRules {
		c.Automation.Rules = append([]AutomationRule{{When: "game", Scene: scene}}, c.Automation.Rules...)
		c.Automation.Enabled = true
	}
	c.GamingMigrated = true
	r.PauseGaming, r.GamingScene = false, ""
}

// automationMatches reports whether condition when holds for g.
func automationMatches(when string, g GamingStatus) bool {
	if !g.Active {
		return false
	}
	if when == "game" {
		return true
	}
	exe, ok := strings.CutPrefix(when, "game:")
	return ok && g.Game != "" && sameExe(exe, g.Game)
}

// sameExe compares executable names case-insensitively, with or without
// ".exe" and ignoring a leading path.
func sameExe(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if i := strings.LastIndexAny(s, `/\`); i >= 0 {
			s = s[i+1:]
		}
		return strings.TrimSuffix(s, ".exe")
	}
	return norm(a) != "" && norm(a) == norm(b)
}

// automate evaluates the rules and switches the displays when due. It runs
// from the rotation loop, before the rotation.
func (e *Engine) automate(now time.Time) {
	var g GamingStatus
	if e.gamingFn != nil {
		g = e.gamingFn(now)
	} else {
		g = e.Gaming(now)
	}
	e.mu.Lock()
	e.rot.gamingNow = g.Active
	a := e.cfg.Automation
	scenes := e.cfg.Rotation.Scenes
	rot := e.cfg.Rotation

	want, wantKey, wantSig := -1, "", ""
	if a.Enabled {
		for i, r := range a.Rules {
			if r.Off || !automationMatches(r.When, g) {
				continue
			}
			if r.Scene != "" && !slices.ContainsFunc(scenes, func(s Scene) bool { return s.Name == r.Scene }) {
				continue // scene is gone
			}
			want, wantKey = i, ruleKey(r)
			if r.Scene != "" {
				if k := slices.IndexFunc(scenes, func(s Scene) bool { return s.Name == r.Scene }); k >= 0 {
					wantSig = sceneSig(scenes[k])
				}
			}
			break
		}
	}
	// The active rule is applied again only when what it shows changed: a
	// different rule, or the content of its scene. Edits elsewhere in the
	// settings or in other scenes leave the displays alone.
	sameRule := wantKey == e.auto.key
	sigChanged := sameRule && wantKey != "" && wantSig != e.auto.sig
	immediate := !a.Enabled || e.auto.dirty || sigChanged
	e.auto.dirty = false
	if sameRule && !sigChanged {
		e.auto.candSet = false
		if e.auto.key != "" {
			e.auto.game = g.Game
			// The rule may have moved in the list (reorder): keep the status right.
			if want >= 0 {
				e.auto.idx, e.auto.when, e.auto.scene = want, a.Rules[want].When, a.Rules[want].Scene
			}
		}
		e.mu.Unlock()
		return
	}
	if !e.auto.candSet || e.auto.cand != wantKey {
		e.auto.cand, e.auto.candIdx, e.auto.candSince, e.auto.candSet = wantKey, want, now, true
	}
	delay := a.enter()
	if wantKey == "" {
		delay = a.leave()
	}
	if !immediate && now.Sub(e.auto.candSince) < delay {
		e.mu.Unlock()
		return
	}

	// Commit.
	e.auto.candSet = false
	wasActive := e.auto.key != ""
	e.auto.key, e.auto.idx, e.auto.game, e.auto.sig = wantKey, want, g.Game, wantSig
	e.auto.applied++
	e.auto.when, e.auto.scene = "", ""
	e.rot.transient = ""
	effect := rot.Effect
	if effect == "" {
		effect = "fade"
	}
	order := e.panelOrder(rot)
	type change struct {
		w     *worker
		c     PanelConfig
		delay time.Duration
	}
	var changes []change
	delayFor := func(i int) time.Duration {
		if rot.Cascade {
			return time.Duration(i) * cascadeStep
		}
		return 0
	}
	restoreNormal := func() {
		for i, id := range order {
			if w := e.workers[id]; w != nil {
				changes = append(changes, change{w, e.cfg.Panels[id].Clone(), delayFor(i)})
			}
		}
	}
	switch {
	case want >= 0:
		r := a.Rules[want]
		e.auto.when, e.auto.scene = r.When, r.Scene
		e.rot.shown = ""
		if r.Scene == "" {
			restoreNormal()
		} else if i := slices.IndexFunc(scenes, func(s Scene) bool { return s.Name == r.Scene }); i >= 0 {
			for k, id := range order {
				if c, ok := scenes[i].Panels[id]; ok && e.workers[id] != nil {
					changes = append(changes, change{e.workers[id], c.Clone(), delayFor(k)})
				}
			}
		}
	case wasActive:
		// Let go: the rotation takes over (it restores the normal setup
		// itself if it cannot run), otherwise restore here.
		if rot.Enabled {
			e.rot.active, e.rot.lastStep = true, time.Time{}
		} else {
			e.rot.active = false
			restoreNormal()
		}
	}
	e.mu.Unlock()
	for _, ch := range changes {
		ch.w.switchTo(ch.c, effect, ch.delay)
	}
}

// ApplyScene copies a scene's display settings into the displays' normal
// configuration (replacing it). Displays the scene does not know are left
// alone.
func (e *Engine) ApplyScene(name string) error {
	e.mu.Lock()
	i := slices.IndexFunc(e.cfg.Rotation.Scenes, func(s Scene) bool { return s.Name == name })
	if i < 0 {
		e.mu.Unlock()
		return ErrNoScene
	}
	panels := map[string]PanelConfig{}
	for id, c := range e.cfg.Rotation.Scenes[i].Panels {
		panels[id] = c.Clone()
	}
	e.mu.Unlock()
	var first error
	for id, c := range panels {
		if err := e.SetPanelConfig(id, c); err != nil && first == nil && !strings.Contains(err.Error(), "not connected") {
			first = err
		}
	}
	return first
}

// RenameScene renames a scene and the automation rules pointing to it.
func (e *Engine) RenameScene(from, to string) error {
	to, err := cleanSceneName(to)
	if err != nil {
		return err
	}
	e.rotMu.Lock()
	defer e.rotMu.Unlock()
	e.mu.Lock()
	sc := slices.Clone(e.cfg.Rotation.Scenes) // Rotation() hands out the old slice
	i := slices.IndexFunc(sc, func(s Scene) bool { return s.Name == from })
	if i < 0 {
		e.mu.Unlock()
		return ErrNoScene
	}
	if from != to && slices.ContainsFunc(sc, func(s Scene) bool { return s.Name == to }) {
		e.mu.Unlock()
		return fmt.Errorf("a scene named %q exists already", to)
	}
	sc[i].Name = to
	e.cfg.Rotation.Scenes = sc
	rules := slices.Clone(e.cfg.Automation.Rules)
	for k := range rules {
		if rules[k].Scene == from {
			rules[k].Scene = to
		}
	}
	e.cfg.Automation.Rules = rules
	if e.rot.shown == from {
		e.rot.shown = to
	}
	// A rename changes no content: carry the active rule (and a running debounce)
	// over to the new name so the displays are not re-applied.
	if e.auto.key != "" && e.auto.scene == from {
		e.auto.key, e.auto.scene = e.auto.when+"\x00"+to, to
	}
	if e.auto.candSet && strings.HasSuffix(e.auto.cand, "\x00"+from) {
		e.auto.cand = strings.TrimSuffix(e.auto.cand, from) + to
	}
	e.mu.Unlock()
	return e.persist()
}

// GamingSceneName is the name CreateGamingScene uses.
const GamingSceneName = "Gaming"

func designSetOf(id string) string {
	switch {
	case strings.HasPrefix(id, "neon-"):
		return "neon"
	case strings.HasPrefix(id, "retro-"):
		return "retro"
	case strings.HasPrefix(id, "cyber-"):
		return "cyberpunk"
	}
	return "clean"
}

// CreateGamingScene saves a scene "Gaming" made from the current setup with
// gaming designs: the first display shows FPS, the next GPU and CPU, and a
// bar display gets the "Gaming" grid template. Displays keep their design
// set, theme, brightness and background. The scene stays out of the
// rotation. An existing scene of that name is replaced.
func (e *Engine) CreateGamingScene(name string) error {
	if name == "" {
		name = GamingSceneName
	}
	e.rotMu.Lock()
	defer e.rotMu.Unlock()
	e.mu.Lock()
	panels := map[string]PanelConfig{}
	roles := []string{"fps", "gpu", "cpu"}
	n := 0
	for _, id := range e.order {
		w := e.workers[id]
		if w == nil {
			continue
		}
		c := e.cfg.Panels[id].Clone()
		if c.Mode != "design" {
			c.Mode = "design"
			c.Media = ""
		}
		set := designSetOf(c.Design)
		if c.Grid != "" {
			set = designSetOf(func() string {
				for _, t := range c.Tiles {
					if t.Design != "" {
						return t.Design
					}
				}
				return ""
			}())
		}
		sz := w.sizeFor(c.Rotation)
		tpl, ok := GridTemplateByID("gaming", set)
		if _, fits := lcd.GridPresetByID(sz.X, sz.Y, tpl.Grid); ok && fits {
			c.Grid, c.Tiles, c.Layout = tpl.Grid, slices.Clone(tpl.Tiles), tpl.ID+"-"+set
		} else {
			c.Grid, c.Tiles, c.Layout = "", nil, ""
			role := roles[n%len(roles)]
			n++
			c.Design = squareRoles[role][set]
		}
		panels[id] = c
	}
	e.mu.Unlock()
	if len(panels) == 0 {
		return ErrNoPanel
	}
	yes := true
	return e.putScene(Scene{Name: name, Panels: panels}, &yes)
}
