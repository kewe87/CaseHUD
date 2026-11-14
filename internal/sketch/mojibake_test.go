package sketch

import (
	"strings"
	"testing"
)

func TestFixMojibake(t *testing.T) {
	// double-encoded text as produced by Windows-1252 and Latin-1 readers
	fixed := map[string]string{
		"CPU-K\u00c3\u00bchler":                  "CPU-Kühler",
		"Gehäuse":                                "Gehäuse",
		"Geh\u00c3\u00a4use":                     "Gehäuse",
		"Gr\u00c3\u00b6\u00c3\u009fe":            "Größe",
		"Gr\u00c3\u00b6\u00c3\u0178e":            "Größe", // cp1252 shows 0x9F as Ÿ
		"\u00c3\u0153ber":                        "Über",  // 0x9C as œ
		"\u00c3\u201eu\u00c3\u0178ere":           "Äußere",
		"\u00c3\u2013l":                          "Öl",
		"lin\u00c3\u00a0":                        "linà",
		"L\u00c3\u00bcfter \u00e2\u20ac\u201c 1": "L\u00fcfter \u2013 1", // en dash, three bytes
	}
	for in, want := range fixed {
		if got := FixMojibake(in); got != want {
			t.Errorf("FixMojibake(%q) = %q, want %q", in, got, want)
		}
	}
	// idempotent
	if got := FixMojibake(FixMojibake("CPU-K\u00c3\u00bchler")); got != "CPU-Kühler" {
		t.Errorf("second pass changed the text: %q", got)
	}
}

func TestFixMojibakeKeepsRealText(t *testing.T) {
	keep := []string{"", "Front", "CPU-Kühler", "Ärger", "Größe", "日本語", "Ã", "Ã!", "Ab Ã", "ÃÃ", "Ž", "Ã" + strings.Repeat("x", 3)}
	for _, s := range keep {
		if got := FixMojibake(s); got != s {
			t.Errorf("FixMojibake(%q) = %q, want unchanged", s, got)
		}
	}
}

func TestDecodeRepairsLabels(t *testing.T) {
	doc := `{"version":2,"case":{"source":"custom","id":"","dims":{"w":400,"h":400,"d":400}},"canvasView":"net","parts":[{"id":"p1","part":"x","label":"CPU-KÃ¼hler"},{"id":"p2","part":"x","label":"Ã"}]}`
	s, _, err := DecodeWithCatalog([]byte(doc), Builtin())
	if err != nil {
		t.Fatal(err)
	}
	if s.Parts[0].Label != "CPU-Kühler" || s.Parts[1].Label != "Ã" {
		t.Errorf("DecodeWithCatalog labels: %q, %q", s.Parts[0].Label, s.Parts[1].Label)
	}
	var u Sketch
	if err := u.UnmarshalJSON([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if u.Parts[0].Label != "CPU-Kühler" || u.Parts[1].Label != "Ã" {
		t.Errorf("UnmarshalJSON labels: %q, %q", u.Parts[0].Label, u.Parts[1].Label)
	}
}
