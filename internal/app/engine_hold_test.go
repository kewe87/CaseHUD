package app

import (
	"testing"
	"time"
)

func workerDesign(e *Engine, id string) string {
	e.mu.Lock()
	w := e.workers[id]
	e.mu.Unlock()
	return w.current().Design
}

func waitDesign(t *testing.T, e *Engine, id, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if workerDesign(e, id) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("display shows design %q, want %q", workerDesign(e, id), want)
}

func stays(t *testing.T, e *Engine, id, want string) {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	if got := workerDesign(e, id); got != want {
		t.Fatalf("display shows design %q, want it to stay on %q", got, want)
	}
}

// A setup changed while the automation shows a scene is stored but does not
// break the scene; it shows when the automation lets go.
func TestSetPanelConfigWhileAutomationHolds(t *testing.T) {
	e, f := autoEngine(t)
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	normal := c.Design
	c.Design = "gaming"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveScene("Gaming"); err != nil {
		t.Fatal(err)
	}
	c.Design = normal
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	waitDesign(t, e, id, normal)
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, LeaveS: 5, Rules: []AutomationRule{{When: "game", Scene: "Gaming"}}}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 0 {
		t.Fatalf("automation not active: %+v", st)
	}
	waitDesign(t, e, id, "gaming")
	if !e.Panels()[0].Held {
		t.Fatal("panel should report that a scene holds it")
	}

	c.Design = "cpu-temp"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if got := e.Panels()[0].Config.Design; got != "cpu-temp" {
		t.Fatalf("config not stored: %q", got)
	}
	stays(t, e, id, "gaming")

	f.set(false, "")
	e.automate(f.advance(0))
	e.automate(f.advance(5 * time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != -1 {
		t.Fatalf("automation did not let go: %+v", st)
	}
	waitDesign(t, e, id, "cpu-temp")
	if e.Panels()[0].Held {
		t.Fatal("hold should be over")
	}
}

// A rule that shows the normal setup holds nothing: changes show at once.
func TestSetPanelConfigWhileAutomationShowsNormal(t *testing.T) {
	e, f := autoEngine(t)
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{{When: "game", Scene: ""}}}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x")
	e.automate(f.advance(0))
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 0 {
		t.Fatalf("automation not active: %+v", st)
	}
	if e.Panels()[0].Held {
		t.Fatal("a rule without a scene must not hold the display")
	}
	c.Design = "cpu-temp"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	waitDesign(t, e, id, "cpu-temp")
}

func TestSetPanelConfigWhileRotationHolds(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	for _, s := range []struct{ name, design string }{{"A", "gaming"}, {"B", "loop"}} {
		c.Design = s.design
		if err := e.SetPanelConfig(id, c); err != nil {
			t.Fatal(err)
		}
		if err := e.SaveScene(s.name); err != nil {
			t.Fatal(err)
		}
	}
	c.Design = "cpu-temp"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	waitDesign(t, e, id, "cpu-temp")
	if e.Panels()[0].Held {
		t.Fatal("nothing holds the display yet")
	}
	r := e.Rotation()
	r.Enabled, r.IntervalS, r.Effect = true, 3600, "fade"
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	waitDesign(t, e, id, "gaming")
	if !e.Panels()[0].Held {
		t.Fatal("rotation should hold the display")
	}

	c.Design = "gpu-temp"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if got := e.Panels()[0].Config.Design; got != "gpu-temp" {
		t.Fatalf("config not stored: %q", got)
	}
	stays(t, e, id, "gaming")

	r.Enabled = false
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	waitDesign(t, e, id, "gpu-temp")
}
