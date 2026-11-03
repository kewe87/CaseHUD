package media

import (
	"os"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Manual check of the real capture code: set CASEHUD_CAPTURE_SMOKE=1. The
// result is only counted, never written anywhere. Not run in CI.
func TestCaptureSmoke(t *testing.T) {
	if os.Getenv("CASEHUD_CAPTURE_SMOKE") != "1" {
		t.Skip("set CASEHUD_CAPTURE_SMOKE=1 to capture the real screen")
	}
	p := leds.Params{}
	for _, d := range screenEffect().Params {
		p[d.Key] = d.Default
	}
	eff, err := screenEffect().New(p)
	if err != nil {
		t.Fatal(err)
	}
	c := newCanvas()
	for i := 0; i < 20; i++ {
		eff.Render(leds.Frame{}, c)
		time.Sleep(50 * time.Millisecond)
	}
	if err := eff.(*screen).Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("screen smoke ok")
}
