package web

import (
	"os"
	"strings"
	"testing"
)

// The disclaimer exists in one wording: installer/disclaimer-*.txt is the
// source, README and the GUI (i18n.js) must carry the same paragraphs.
func TestDisclaimerTextIsIdenticalEverywhere(t *testing.T) {
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	readme := read("../../README.md")
	i18n := read("static/i18n.js")
	for _, lang := range []string{"de", "en"} {
		parts := strings.Split(strings.TrimSpace(read("../../installer/disclaimer-"+lang+".txt")), "\n\n")
		if len(parts) != 8 {
			t.Fatalf("%s: %d paragraphs", lang, len(parts))
		}
		for _, p := range parts[1:] {
			if !strings.Contains(readme, p) {
				t.Errorf("%s: README lacks %q", lang, p)
			}
			if !strings.Contains(i18n, p) {
				t.Errorf("%s: i18n.js lacks %q", lang, p)
			}
		}
		if !strings.Contains(i18n, `"disc.title": "`+parts[0]+`"`) {
			t.Errorf("%s: i18n.js title differs", lang)
		}
	}
}
