package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func decodeCanvas(t *testing.T, c *LightingCanvas) image.Image {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(c.PNG)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() > leds.PreviewWidth || b.Dy() > leds.PreviewHeight || b.Dx() < leds.CanvasWidth/2 || c.W != b.Dx() || c.H != b.Dy() {
		t.Fatalf("canvas %v, reported %d x %d", b, c.W, c.H)
	}
	return img
}

func lit(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 7 {
		for x := b.Min.X; x < b.Max.X; x += 7 {
			if r, g, bl, _ := img.At(x, y).RGBA(); r+g+bl > 0 {
				return true
			}
		}
	}
	return false
}

// Without a running engine the page gets a preview of the configured effect;
// capture effects are never started for it.
func TestLightingCanvasPreview(t *testing.T) {
	e := startIn(t, t.TempDir())
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("rainbow"), Brightness: ptr(100)}); err != nil {
		t.Fatal(err)
	}
	live, err := e.LightingLive(true)
	if err != nil {
		t.Fatal(err)
	}
	c := live.Canvas
	if live.Active || c == nil || c.Source != "preview" || c.Effect != "rainbow" || c.Note != "" {
		t.Fatalf("live %+v canvas %+v", live, c)
	}
	if !lit(decodeCanvas(t, c)) {
		t.Error("preview is black")
	}
	if live, _ := e.LightingLive(false); live.Canvas != nil {
		t.Error("canvas sent without being asked for")
	}
	// brightness 0: the preview shows what the LEDs would show
	if _, err := e.PatchLighting(LightingPatch{Brightness: ptr(0)}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	live, _ = e.LightingLive(true)
	if lit(decodeCanvas(t, live.Canvas)) {
		t.Error("preview lit at brightness 0")
	}
	l := &e.light
	l.mu.Lock()
	l.cfg.Effect = "screen"
	l.mu.Unlock()
	live, _ = e.LightingLive(true)
	if c := live.Canvas; c.Note != "capture" || c.PNG != "" || c.Source != "preview" {
		t.Errorf("capture effect previewed: %+v", c)
	}
}

// With the engine running the page gets the engine's own canvas.
func TestLightingCanvasFromEngine(t *testing.T) {
	e := startLit(t)
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("rainbow"), Brightness: ptr(100)}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		live, err := e.LightingLive(true)
		if err != nil {
			t.Fatal(err)
		}
		c := live.Canvas
		if c == nil || c.Source != "engine" {
			t.Fatalf("canvas %+v", c)
		}
		if c.PNG != "" {
			if !lit(decodeCanvas(t, c)) {
				t.Error("engine canvas is black")
			}
			return
		}
		if c.Note != "starting" || time.Now().After(deadline) {
			t.Fatalf("no engine canvas: %+v", c)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
