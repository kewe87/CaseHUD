package leds

import (
	"image"
	"testing"
	"time"
)

func TestMotionEffectsRender(t *testing.T) {
	for _, info := range motionEffects() {
		for _, path := range []string{"around", "horizontal", "vertical", "radial"} {
			p := withDefaults(info, Params{"path": path, "mirror": path == "around"})
			eff, err := info.New(p)
			if err != nil {
				t.Fatalf("%s: %v", info.Name, err)
			}
			c := image.NewRGBA(image.Rect(0, 0, CanvasWidth, CanvasHeight))
			lit := false
			for ms := 0; ms < 8000 && !lit; ms += 97 {
				eff.Render(Frame{T: time.Duration(ms) * time.Millisecond}, c)
				for i := 0; i < len(c.Pix); i += 4 {
					if c.Pix[i]|c.Pix[i+1]|c.Pix[i+2] != 0 {
						lit = true
						break
					}
				}
			}
			if !lit {
				t.Errorf("%s (%s): stays dark", info.Name, path)
			}
		}
	}
}

func TestFieldAround(t *testing.T) {
	f := &field{path: "around"}
	c := image.NewRGBA(image.Rect(0, 0, 100, 100))
	u := f.coords(c)
	top, right := u[0*100+50], u[50*100+99]
	if top > 0.02 || right < 0.23 || right > 0.27 {
		t.Fatalf("top %.3f right %.3f", top, right)
	}
}
