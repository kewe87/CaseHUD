package app

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"
)

func TestRotationScenes(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if err := e.SaveScene("Temps"); err != nil {
		t.Fatal(err)
	}
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	c.Design = "gaming"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveScene("Gaming"); err != nil {
		t.Fatal(err)
	}
	r := e.Rotation()
	r.Enabled, r.IntervalS, r.Effect, r.Cascade = true, 60, "slide-down", true
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if st := e.RotationStatus(); st.Active && st.Scene == want {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("rotation status %+v, want scene %q", e.RotationStatus(), want)
	}
	wait("Temps")
	e.NextRotation()
	wait("Gaming")

	r.Enabled = false
	if err := e.SetRotation(r); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if e.RotationStatus().Active {
		t.Fatal("rotation should stop")
	}
	if err := e.SetRotation(RotationConfig{Mode: "bogus"}); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestBlend(t *testing.T) {
	mk := func(v uint8) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 40, 40))
		for i := range img.Pix {
			img.Pix[i] = v
		}
		return img
	}
	old, next := mk(0), mk(200)
	for _, fx := range Transitions {
		out := blend(fx, 0.5, old, next)
		if out.Bounds() != old.Bounds() {
			t.Fatalf("%s: size %v", fx, out.Bounds())
		}
	}
	if c := blend("fade", 0.5, old, next).RGBAAt(5, 5); c.R < 95 || c.R > 105 {
		t.Errorf("fade midpoint %v", c)
	}
	// Sliding down: the new content enters from the top.
	if c := blend("slide-down", 0.5, old, next).RGBAAt(5, 5); c != (color.RGBA{200, 200, 200, 200}) {
		t.Errorf("slide-down top %v", c)
	}
	if c := blend("slide-down", 0.5, old, next).RGBAAt(5, 35); c.R != 0 {
		t.Errorf("slide-down bottom %v", c)
	}
}

func TestRotationConveyor(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	err := e.SetRotation(RotationConfig{Enabled: true, Mode: "conveyor", IntervalS: 60, Effect: "fade",
		Conveyor: []string{"cpu-temp", "gpu-temp", "loop"}, Cascade: true})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !e.RotationStatus().Active && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !e.RotationStatus().Active {
		t.Fatal("conveyor did not start")
	}
	e.mu.Lock()
	step := e.rot.step
	e.mu.Unlock()
	e.NextRotation()
	for time.Now().Before(deadline) {
		e.mu.Lock()
		s := e.rot.step
		e.mu.Unlock()
		if s == step+1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("conveyor did not advance")
}
