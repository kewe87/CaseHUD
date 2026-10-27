package app

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeGame struct {
	mu sync.Mutex
	g  GamingStatus
	t  time.Time
}

func (f *fakeGame) set(active bool, game string) {
	f.mu.Lock()
	f.g = GamingStatus{Active: active, Game: game, Source: "external"}
	f.mu.Unlock()
}
func (f *fakeGame) advance(d time.Duration) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
	return f.t
}
func (f *fakeGame) now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.t }
func (f *fakeGame) status(time.Time) GamingStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.g
}

func autoEngine(t *testing.T) (*Engine, *fakeGame) {
	t.Helper()
	f := &fakeGame{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true, Now: f.now, gamingFn: f.status}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	for _, n := range []string{"Normal", "Gaming", "Elden"} {
		if err := e.SaveScene(n); err != nil {
			t.Fatal(err)
		}
	}
	return e, f
}

func TestAutomationRuleOrderAndDebounce(t *testing.T) {
	e, f := autoEngine(t)
	err := e.SetAutomation(AutomationConfig{Enabled: true, Rules: []AutomationRule{
		{When: "game:EldenRing.exe", Scene: "Elden"},
		{When: "game", Scene: "Gaming"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	step := func(d time.Duration) AutomationStatus {
		e.automate(f.advance(d))
		return e.AutomationStatus()
	}
	if st := step(time.Second); st.ActiveRule != -1 {
		t.Fatalf("idle: %+v", st)
	}
	// Some other game: second rule, but only after 3 s.
	f.set(true, "other.exe")
	if st := step(0); st.ActiveRule != -1 || st.Pending != 1 {
		t.Fatalf("debouncing: %+v", st)
	}
	if st := step(2 * time.Second); st.ActiveRule != -1 {
		t.Fatalf("too early: %+v", st)
	}
	if st := step(time.Second); st.ActiveRule != 1 || st.Scene != "Gaming" {
		t.Fatalf("not entered: %+v", st)
	}
	// Alt-Tab: gone for 5 s, back: stays.
	f.set(false, "")
	step(5 * time.Second)
	f.set(true, "other.exe")
	if st := step(time.Second); st.ActiveRule != 1 || st.Pending != -1 {
		t.Fatalf("flicker: %+v", st)
	}
	// Gone for good: leaves after 10 s.
	f.set(false, "")
	step(0)
	if st := step(9 * time.Second); st.ActiveRule != 1 {
		t.Fatalf("left early: %+v", st)
	}
	if st := step(time.Second); st.ActiveRule != -1 {
		t.Fatalf("did not leave: %+v", st)
	}
	// The specific game wins over "any game", case-insensitively, even without .exe.
	f.set(true, "eldenring.EXE")
	step(0)
	if st := step(3 * time.Second); st.ActiveRule != 0 || st.Scene != "Elden" {
		t.Fatalf("specific rule: %+v", st)
	}
	// Disabling releases at once.
	a := e.Automation()
	a.Enabled = false
	if err := e.SetAutomation(a); err != nil {
		t.Fatal(err)
	}
	if st := step(0); st.ActiveRule != -1 {
		t.Fatalf("disable: %+v", st)
	}
}

func TestAutomationSkipsMissingSceneAndOffRules(t *testing.T) {
	e, f := autoEngine(t)
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{
		{When: "game", Scene: "Nope"},
		{When: "game", Scene: "Elden", Off: true},
		{When: "game", Scene: ""},
	}}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x.exe")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 2 || st.Scene != "" {
		t.Fatalf("%+v", st)
	}
}

func TestAutomationValidation(t *testing.T) {
	e, _ := autoEngine(t)
	for _, w := range []string{"", "media", "game:", "game:a/b.exe"} {
		if err := e.SetAutomation(AutomationConfig{Rules: []AutomationRule{{When: w}}}); err == nil {
			t.Errorf("%q accepted", w)
		}
	}
	if err := e.SetAutomation(AutomationConfig{EnterS: -1}); err == nil {
		t.Error("negative delay accepted")
	}
	if !sameExe("Game", "game.exe") || sameExe("", "") || sameExe("a.exe", "b.exe") {
		t.Error("sameExe")
	}
}

func TestAutomationMigration(t *testing.T) {
	c := Config{Rotation: RotationConfig{Enabled: true, PauseGaming: true, GamingScene: "G",
		Scenes: []Scene{{Name: "G"}}}}
	migrateLegacyGaming(&c)
	if !c.Automation.Enabled || len(c.Automation.Rules) != 1 ||
		c.Automation.Rules[0] != (AutomationRule{When: "game", Scene: "G"}) {
		t.Fatalf("%+v", c.Automation)
	}
	if c.Rotation.PauseGaming || c.Rotation.GamingScene != "" {
		t.Fatal("legacy fields kept")
	}
	// Running again changes nothing; an unknown scene becomes "hold".
	migrateLegacyGaming(&c)
	if len(c.Automation.Rules) != 1 {
		t.Fatal("migrated twice")
	}
	c2 := Config{Rotation: RotationConfig{PauseGaming: true, GamingScene: "Gone"}}
	migrateLegacyGaming(&c2)
	if len(c2.Automation.Rules) != 1 || c2.Automation.Rules[0].Scene != "" || !c2.Automation.Enabled {
		t.Fatalf("%+v", c2.Automation)
	}
	// A GamingScene without pause did nothing before: no rule.
	c3 := Config{Rotation: RotationConfig{Enabled: true, GamingScene: "G", Scenes: []Scene{{Name: "G"}}}}
	migrateLegacyGaming(&c3)
	if len(c3.Automation.Rules) != 0 {
		t.Fatalf("%+v", c3.Automation)
	}
}

func TestAutomationMigrationNotRepeatedAfterDelete(t *testing.T) {
	e, _ := autoEngine(t)
	r := e.Rotation()
	r.PauseGaming = true
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	a := e.Automation()
	if !a.Enabled || len(a.Rules) != 1 {
		t.Fatalf("%+v", a)
	}
	a.Rules = nil
	if err := e.SetAutomation(a); err != nil {
		t.Fatal(err)
	}
	// an old client posts the legacy option again
	r = e.Rotation()
	r.PauseGaming = true
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	if got := e.Automation().Rules; len(got) != 0 {
		t.Fatalf("deleted rule came back: %+v", got)
	}
}

func TestAutomationLegacyViaSetRotation(t *testing.T) {
	e, _ := autoEngine(t)
	r := e.Rotation()
	r.Enabled, r.PauseGaming, r.GamingScene = true, true, "Gaming"
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	a := e.Automation()
	if !a.Enabled || len(a.Rules) != 1 || a.Rules[0].Scene != "Gaming" || e.Rotation().PauseGaming {
		t.Fatalf("%+v %+v", a, e.Rotation())
	}
}

func TestAutomationRenameScene(t *testing.T) {
	e, f := autoEngine(t)
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{{When: "game", Scene: "Gaming"}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.RenameScene("Gaming", "Normal"); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if err := e.RenameScene("Gaming", "Zocken"); err != nil {
		t.Fatal(err)
	}
	if got := e.Automation().Rules[0].Scene; got != "Zocken" {
		t.Fatalf("rule scene %q", got)
	}
	f.set(true, "x")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.Scene != "Zocken" {
		t.Fatalf("%+v", st)
	}
	if err := e.RenameScene("none", "x"); err != ErrNoScene {
		t.Fatalf("%v", err)
	}
}

func TestAutomationHoldsRotation(t *testing.T) {
	e, f := autoEngine(t)
	r := e.Rotation()
	r.Enabled, r.IntervalS = true, 5
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{{When: "game", Scene: "Gaming"}}}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	if err := e.ActivateScene("Normal", true); err != ErrGamingHold {
		t.Fatalf("ActivateScene: %v", err)
	}
	ss := e.Scenes()
	if !(ss[1].Name == "Gaming" && ss[1].Active) {
		t.Fatalf("%+v", ss)
	}
	e.rotate(f.advance(time.Minute)) // must not touch anything
	if st := e.AutomationStatus(); st.ActiveRule != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestCreateGamingSceneAndNoRotation(t *testing.T) {
	e, _ := autoEngine(t)
	if err := e.CreateGamingScene(""); err != nil {
		t.Fatal(err)
	}
	var sc Scene
	for _, s := range e.Rotation().Scenes {
		if s.Name == GamingSceneName {
			sc = s
		}
	}
	if sc.Name == "" || !sc.NoRotation || len(sc.Panels) == 0 {
		t.Fatalf("%+v", sc)
	}
	first := true
	for _, p := range e.Panels() {
		c := sc.Panels[p.ID]
		if c.Mode != "design" {
			t.Errorf("%s: %+v", p.ID, c)
		}
		if first && c.Grid == "" && c.Design != "gaming" {
			t.Errorf("first display shows %q, want the FPS design", c.Design)
		}
		if len(p.Grids) > 0 && c.Grid != "2+4" {
			t.Errorf("bar display: grid %q", c.Grid)
		}
		first = false
	}
	// Saving over it keeps it out of the rotation; the rotation skips it.
	if err := e.SaveScene(GamingSceneName); err != nil {
		t.Fatal(err)
	}
	for _, s := range e.Rotation().Scenes {
		if s.Name == GamingSceneName && !s.NoRotation {
			t.Fatal("flag lost on overwrite")
		}
	}
	if got := rotationScenes(e.Rotation().Scenes); len(got) != len(e.Rotation().Scenes)-1 {
		t.Fatalf("rotation scenes %d", len(got))
	}
	// And it can still be applied by automation.
	if err := e.SetAutomation(AutomationConfig{Enabled: true, Rules: []AutomationRule{{When: "game", Scene: GamingSceneName}}}); err != nil {
		t.Fatal(err)
	}
}

func TestAutomationReappliesOnlyOnRealChange(t *testing.T) {
	e, f := autoEngine(t)
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{{When: "game", Scene: "Gaming"}}}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x.exe")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	applied := func() int { e.mu.Lock(); defer e.mu.Unlock(); return e.auto.applied }
	base := applied()
	if base != 1 || e.AutomationStatus().ActiveRule != 0 {
		t.Fatalf("applied=%d %+v", base, e.AutomationStatus())
	}
	// Unrelated settings change (delays, ignore list, another rule below): no re-apply.
	a := e.Automation()
	a.LeaveS = 20
	a.IgnoreGames = []string{"foo.exe"}
	a.Rules = append(a.Rules, AutomationRule{When: "game:y.exe", Scene: "Elden"})
	if err := e.SetAutomation(a); err != nil {
		t.Fatal(err)
	}
	e.automate(f.advance(time.Second))
	// Another scene changed, not the one shown.
	if err := e.SaveScene("Elden"); err != nil {
		t.Fatal(err)
	}
	e.automate(f.advance(time.Second))
	if applied() != base {
		t.Fatalf("re-applied without a change: %d", applied())
	}
	// The shown scene changed: applied again, once.
	p := e.Panels()[0]
	c := p.Config
	c.Design = "gaming"
	if err := e.SetPanelConfig(p.ID, c); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveScene("Gaming"); err != nil {
		t.Fatal(err)
	}
	e.automate(f.advance(time.Second))
	e.automate(f.advance(time.Second))
	if applied() != base+1 {
		t.Fatalf("applied=%d, want %d", applied(), base+1)
	}
}

func TestActivateSceneOutsideRotation(t *testing.T) {
	e, _ := autoEngine(t)
	yes := true
	if err := e.SaveSceneWith("Solo", &yes); err != nil {
		t.Fatal(err)
	}
	// Rotation off: applies the scene.
	if err := e.ActivateScene("Solo", true); err != nil {
		t.Fatalf("rotation off: %v", err)
	}
	// Rotation on with two scenes in it, "Solo" excluded: still works.
	r := e.Rotation()
	r.Enabled, r.IntervalS = true, 60
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	if err := e.ActivateScene("Solo", true); err != nil {
		t.Fatalf("excluded scene: %v", err)
	}
	if err := e.ActivateScene("Nope", true); err != ErrNoScene {
		t.Fatalf("%v", err)
	}
}

func TestActivateSceneTransientDoesNotPersist(t *testing.T) {
	e, _ := autoEngine(t)
	id := e.Panels()[0].ID
	orig := e.Panels()[0].Config
	// "Normal" was saved with the original setup; now the displays change.
	if _, err := e.UpdatePanelConfig(id, func(c *PanelConfig) error { c.Brightness = 37; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := e.ActivateScene("Normal", false); err != nil {
		t.Fatal(err)
	}
	if got := e.Panels()[0].Config; got.Brightness != 37 {
		t.Fatalf("a transient activation changed the saved setup: %+v", got)
	}
	var active bool
	for _, s := range e.Scenes() {
		if s.Name == "Normal" {
			active = s.Active
		}
	}
	if !active {
		t.Error("scene not reported as active")
	}
	// the user changes something: the scene is no longer "on screen"
	if _, err := e.UpdatePanelConfig(id, func(c *PanelConfig) error { c.Brightness = 40; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, s := range e.Scenes() {
		if s.Active {
			t.Errorf("scene %q still active after a change", s.Name)
		}
	}
	// persist: saved as the setup
	if err := e.ActivateScene("Normal", true); err != nil {
		t.Fatal(err)
	}
	if got := e.Panels()[0].Config.Brightness; got != orig.Brightness {
		t.Fatalf("persist: brightness %d, want %d", got, orig.Brightness)
	}
}
