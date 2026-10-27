package lcd

import (
	"image/color"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

func TestRenderDefaultLayout(t *testing.T) {
	l := DefaultLayout()
	r, err := NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	p := metrics.NewMock()
	for i := 0; i < 5; i++ {
		r.Sample(p)
	}
	img := r.Render(p)
	if img.Bounds().Dx() != l.Width || img.Bounds().Dy() != l.Height {
		t.Fatalf("frame is %v, want %dx%d", img.Bounds(), l.Width, l.Height)
	}
	bg := parseHex(l.Background, color.RGBA{})
	drawn := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != bg.R || img.Pix[i+1] != bg.G || img.Pix[i+2] != bg.B {
			drawn++
		}
	}
	if drawn < len(img.Pix)/4/4 {
		t.Fatalf("only %d pixels drawn, widgets seem missing", drawn)
	}
}

func TestParseHex(t *testing.T) {
	if c := parseHex("#22d3ee", color.RGBA{}); c != (color.RGBA{0x22, 0xd3, 0xee, 0xff}) {
		t.Fatalf("parseHex = %v", c)
	}
	fb := color.RGBA{1, 2, 3, 255}
	if c := parseHex("nope", fb); c != fb {
		t.Fatalf("invalid hex should fall back, got %v", c)
	}
}

func TestLocalizeDate(t *testing.T) {
	cases := map[string]string{
		"Monday, 02.01.": "Montag, 02.01.",
		"Mon 02.01.":     "Mo 02.01.",
		"Sun, 5 March":   "So, 5 März",
		"15:04":          "15:04",
	}
	for in, want := range cases {
		if got := localizeDate(in, "de"); got != want {
			t.Errorf("localizeDate(%q) = %q, want %q", in, got, want)
		}
	}
	if localizeDate("Monday", "en") != "Monday" {
		t.Error("English must stay unchanged")
	}
}

func TestInvertedThresholds(t *testing.T) {
	r, _ := NewRenderer(DefaultLayout())
	w := Widget{Color: "#22d3ee", Warn: 60, Crit: 30, Invert: true}
	base := parseHex(w.Color, color.RGBA{})
	if c := r.colorFor(w, 120, true); c != base {
		t.Errorf("120 fps should use the base colour, got %v", c)
	}
	if c := r.colorFor(w, 45, true); c != r.warn {
		t.Errorf("45 fps should warn, got %v", c)
	}
	if c := r.colorFor(w, 20, true); c != r.crit {
		t.Errorf("20 fps should be critical, got %v", c)
	}
}
