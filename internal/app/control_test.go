package app

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

func startDemo(t *testing.T) *Engine {
	t.Helper()
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	e.Acknowledge("lighting")
	e.Acknowledge("experimental")
	return e
}

func TestSleepStaysConsistent(t *testing.T) {
	e := startDemo(t)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			e.SetPower(i%2 == 0)
		}(i)
		go func(i int) { // what runIdle does
			defer wg.Done()
			e.mu.Lock()
			e.idleOff = i%3 == 0
			e.mu.Unlock()
			e.syncSleep()
		}(i)
	}
	wg.Wait()
	e.syncSleep()
	e.mu.Lock()
	defer e.mu.Unlock()
	want := e.idleOff || e.powerOff
	if e.asleepNow != want {
		t.Fatalf("asleepNow %v, want %v", e.asleepNow, want)
	}
	for id, w := range e.workers {
		w.mu.Lock()
		got := w.asleep
		w.mu.Unlock()
		if got != want {
			t.Errorf("worker %s asleep=%v, want %v", id, got, want)
		}
	}
}

func TestActivateSceneConflicts(t *testing.T) {
	e := startDemo(t)
	if err := e.ActivateScene("A", true); !errors.Is(err, ErrNoScene) {
		t.Fatalf("err %v", err)
	}
	if err := e.SaveScene("A"); err != nil {
		t.Fatal(err)
	}
	r := e.Rotation()
	r.Enabled, r.IntervalS = true, 60
	e.SetRotation(r)
	if err := e.ActivateScene("A", true); err != nil {
		t.Fatalf("one scene is applied as the setup: %v", err)
	}
	if err := e.SaveScene("B"); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.auto.key = "game-B" // the automation holds the displays
	e.mu.Unlock()
	if err := e.ActivateScene("B", true); !errors.Is(err, ErrGamingHold) {
		t.Fatalf("gaming: err %v, want ErrGamingHold", err)
	}
	// A pending jump is dropped when the rotation stops.
	e.mu.Lock()
	e.auto.key = ""
	e.mu.Unlock()
	if err := e.ActivateScene("B", true); err != nil {
		t.Fatal(err)
	}
	r = e.Rotation()
	r.Enabled = false
	e.SetRotation(r)
	deadline := time.Now().Add(2 * time.Second)
	for {
		e.mu.Lock()
		j := e.rot.jump
		e.mu.Unlock()
		if j == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale jump %q", j)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestGamingFPSHoldCountsUnderHint(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true, Now: func() time.Time { mu.Lock(); defer mu.Unlock(); return now }}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	e.SetGamingHint(true, "G", 0, 10*time.Second)
	if g := e.Gaming(now); g.Source != "external" {
		t.Fatalf("%+v", g)
	}
	later := now.Add(11 * time.Second) // lease over, FPS has been there all along
	if g := e.Gaming(later); g.Source != "fps" {
		t.Fatalf("lapsed lease should fall back to fps at once: %+v", g)
	}
}

func TestWidePanelDefaults(t *testing.T) {
	if !IsWide(1920, 480) || IsWide(480, 480) || IsWide(400, 400) {
		t.Fatal("IsWide")
	}
	if c := defaultConfigFor(image.Pt(1920, 480), false); c.Design != "water-wide" {
		t.Fatalf("wide default: %q", c.Design)
	}
	if c := defaultConfigFor(image.Pt(480, 480), false); c.Design != "water-cooling" {
		t.Fatalf("square default: %q", c.Design)
	}
	for set, want := range map[string]string{"clean": "water-wide", "neon": "neon-wide", "retro": "retro-wide", "cyberpunk": "cyber-wide"} {
		if d, _ := setDesign(set, false, true, 0); d != want {
			t.Errorf("%s on a bar: %q, want %q", set, d, want)
		}
	}
}

func TestWideDesignsListAndRender(t *testing.T) {
	st := &Store{Dir: t.TempDir()}
	wide := map[string]bool{}
	for _, d := range st.Designs() {
		if d.W <= 0 || d.H <= 0 {
			t.Errorf("%s: no size", d.ID)
		}
		if IsWide(d.W, d.H) {
			wide[d.ID] = true
		}
	}
	for _, id := range []string{"water-wide", "wide-rings", "neon-wide", "retro-wide", "cyber-wide"} {
		if !wide[id] {
			t.Errorf("%s is not listed as a wide design", id)
		}
		l := builtinDesigns()[id]
		if l.Width != 1920 || l.Height != 480 {
			t.Errorf("%s: %dx%d", id, l.Width, l.Height)
		}
		for _, w := range l.Widgets {
			if w.X < 0 || w.Y < 0 || w.X+w.W > l.Width || w.Y+w.H > l.Height {
				t.Errorf("%s: widget %+v leaves the canvas", id, w)
			}
		}
	}
	// A square design renders on the bar without stretching it.
	r, err := lcd.NewRenderer(builtinDesigns()["cpu-gpu"].FitTo(1920, 480))
	if err != nil {
		t.Fatal(err)
	}
	if b := r.Render(metrics.NewMock()).Bounds(); b.Dx() != 1920 || b.Dy() != 480 {
		t.Fatalf("frame %v", b)
	}
}
