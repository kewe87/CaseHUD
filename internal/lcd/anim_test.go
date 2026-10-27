package lcd

import (
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// fixedProvider returns the same reading for every source.
type fixedProvider struct{ v float64 }

func (f *fixedProvider) Refresh() error { return nil }
func (f *fixedProvider) Get(string) (metrics.Reading, bool) {
	return metrics.Reading{Value: f.v, Unit: "%"}, true
}
func (f *fixedProvider) All() []metrics.Reading { return nil }

func TestAnimationGlidesAndSettles(t *testing.T) {
	l := Layout{Width: 100, Height: 100, IntervalMS: 1000, HistorySize: 2,
		Widgets: []Widget{{Type: "gauge", X: 0, Y: 0, W: 100, H: 100, Source: "Load/CPU Total", Max: 100}}}
	r, err := NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	r.SetAnimation(50 * time.Millisecond)
	p := &fixedProvider{v: 10}
	if _, settled := r.Frame(p); !settled {
		t.Fatal("first frame should show the reading directly")
	}
	p.v = 90
	// Windows' monotonic clock can tick in steps of up to ~16 ms; without a
	// pause both frames may see the same time and nothing has moved yet.
	time.Sleep(20 * time.Millisecond)
	img, settled := r.Frame(p)
	if settled {
		t.Fatal("a jump from 10 to 90 should animate")
	}
	if img.Bounds().Dx() != 100 {
		t.Fatalf("draft frame size %v, want output size", img.Bounds())
	}
	if v := r.anim["Load/CPU Total"].shown; v <= 10 || v >= 90 {
		t.Fatalf("shown value %v should be between 10 and 90", v)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !settled && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		_, settled = r.Frame(p)
	}
	if !settled || r.anim["Load/CPU Total"].shown != 90 {
		t.Fatalf("animation did not settle on 90: %+v", r.anim["Load/CPU Total"])
	}
}

// unitProvider reports a fixed °C reading for every term.
type unitProvider struct{ v float64 }

func (u unitProvider) Refresh() error { return nil }
func (u unitProvider) Get(string) (metrics.Reading, bool) {
	return metrics.Reading{Value: u.v, Unit: "°C"}, true
}
func (u unitProvider) All() []metrics.Reading { return nil }

func TestFahrenheit(t *testing.T) {
	r, err := NewRenderer(Layout{Width: 10, Height: 10, HistorySize: 2})
	if err != nil {
		t.Fatal(err)
	}
	r.Fahrenheit = true
	p := unitProvider{v: 25}
	r.rawReading(p, "Temperatures/CPU", "°")
	if got := r.num("Temperatures/CPU", 25, 0); got != "77" {
		t.Errorf("25 °C = %s °F, want 77", got)
	}
	// A difference only scales: 5 °C apart is 9 °F apart.
	r.rawReading(p, "Temperatures/T Sensor - Temperatures/Ambient", "°")
	if got := r.num("Temperatures/T Sensor - Temperatures/Ambient", 5, 0); got != "9" {
		t.Errorf("delta 5 °C = %s °F, want 9", got)
	}
	if got := clockLayout("Monday, 02.01.2006", false, "en"); got != "Monday, Jan 2, 2006" {
		t.Errorf("english date layout %q", got)
	}
}

func TestDecimalComma(t *testing.T) {
	if got := formatValue(6.5, 1, true); got != "6,5" {
		t.Errorf("de = %q", got)
	}
	if got := formatValue(6.5, 1, false); got != "6.5" {
		t.Errorf("en = %q", got)
	}
	if got := formatValue(1234.5, 0, true); got != "1235" && got != "1234" {
		t.Errorf("no grouping: %q", got)
	}
	for lang, want := range map[string]string{"de": "27,6", "en": "27.6", "": "27.6"} {
		r, err := NewRenderer(Layout{Width: 10, Height: 10, HistorySize: 2, Lang: lang})
		if err != nil {
			t.Fatal(err)
		}
		if got := r.num("x", 27.6, 1); got != want {
			t.Errorf("lang %q: %q, want %q", lang, got, want)
		}
	}
}
