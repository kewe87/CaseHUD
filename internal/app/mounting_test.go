package app

import (
	"testing"
	"time"
)

// setSceneRotation rewrites the rotation stored in a scene.
func setSceneRotation(e *Engine, scene, id string, rot int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.cfg.Rotation.Scenes {
		if e.cfg.Rotation.Scenes[i].Name == scene {
			c := e.cfg.Rotation.Scenes[i].Panels[id]
			c.Rotation = rot
			e.cfg.Rotation.Scenes[i].Panels[id] = c
		}
	}
}

func waitRotation(t *testing.T, e *Engine, id string, want int, what string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		e.mu.Lock()
		w := e.workers[id]
		e.mu.Unlock()
		if w.current().Rotation == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: worker rotation %d, want %d", what, e.workers[id].current().Rotation, want)
}

// A scene never changes how the display is mounted: the panel keeps its own
// rotation however the scene was saved.
func TestScenesKeepPanelRotation(t *testing.T) {
	e, f := autoEngine(t)
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	c.Rotation = 180
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	waitRotation(t, e, id, 180, "mounted")
	// "Gaming" saved while the panel stood at 0 (stored 0), "Elden" saved at
	// 180 and the panel turned afterwards below.
	setSceneRotation(e, "Gaming", id, 0)
	setSceneRotation(e, "Elden", id, 90)
	if err := e.SetAutomation(AutomationConfig{Enabled: true, Rules: []AutomationRule{
		{When: "game:elden.exe", Scene: "Elden"},
		{When: "game", Scene: "Gaming"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"other.exe", "elden.exe"} {
		f.set(true, g)
		e.automate(f.advance(time.Second))
		e.automate(f.advance(10 * time.Second))
		if e.AutomationStatus().ActiveRule < 0 {
			t.Fatalf("%s: no rule active", g)
		}
		time.Sleep(100 * time.Millisecond)
		waitRotation(t, e, id, 180, "automation "+g)
	}
	f.set(false, "")
	e.automate(f.advance(time.Minute))
	e.automate(f.advance(time.Minute))
	waitRotation(t, e, id, 180, "after the game")

	// Activating and applying scenes.
	if err := e.ActivateScene("Gaming", false); err != nil {
		t.Fatal(err)
	}
	waitRotation(t, e, id, 180, "show scene")
	if err := e.ActivateScene("Elden", true); err != nil {
		t.Fatal(err)
	}
	waitRotation(t, e, id, 180, "apply scene")
	if got := e.Panels()[0].Config.Rotation; got != 180 {
		t.Fatalf("saved rotation %d, want 180", got)
	}

	// The rotation (scene mode) shows the stored scenes too.
	e.mu.Lock()
	c2 := e.cfg.Panels[id]
	c2.Rotation = 0
	e.cfg.Panels[id] = c2
	e.mu.Unlock()
	sc := e.Rotation()
	sc.Enabled, sc.Mode, sc.IntervalS = true, "scenes", 5
	if err := e.SetRotation(sc); err != nil {
		t.Fatal(err)
	}
	e.rotate(f.advance(time.Minute))
	waitRotation(t, e, id, 0, "conveyor/scene rotation after turning the panel to 0")
}
