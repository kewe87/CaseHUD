package lcd

import (
	"image/color"
	"testing"
)

func TestApplyThemeLight(t *testing.T) {
	l := DefaultLayout()
	lt := ApplyTheme(l, "light")
	if lt.Background == l.Background || !isLight(parseHex(lt.Background, color.RGBA{})) {
		t.Fatalf("light background = %s", lt.Background)
	}
	for i, w := range lt.Widgets {
		if w.Color == "" {
			continue
		}
		L, _, _ := toOKLab(parseHex(w.Color, color.RGBA{}))
		if L > 0.63 {
			t.Errorf("widget %d colour %s too light for a white background (L=%.2f)", i, w.Color, L)
		}
	}
	if ApplyTheme(l, "dark").Background != l.Background {
		t.Error("dark must leave the design unchanged")
	}
	if got := forLight("#f1f4fa"); got == "#f1f4fa" {
		t.Error("near-white accents must turn dark")
	}
}
