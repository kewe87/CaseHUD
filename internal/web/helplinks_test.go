package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// The "?" links (static/help.js) mirror docs/handbuch/links.json; every target
// section must exist in the manual and every data-help in the page must be a
// known area.
func TestHelpLinksMatchManual(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "docs", "handbuch", "links.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Areas map[string]struct{ De, En string } `json:"areas"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	js, err := os.ReadFile(filepath.Join("static", "help.js"))
	if err != nil {
		t.Fatal(err)
	}
	entry := regexp.MustCompile(`(?m)^  "([^"]+)": \["([^"]+)", "([^"]+)"\],$`)
	got := map[string][2]string{}
	for _, m := range entry.FindAllStringSubmatch(string(js), -1) {
		got[m[1]] = [2]string{m[2], m[3]}
	}
	if len(got) != len(doc.Areas) {
		t.Errorf("help.js has %d areas, links.json %d", len(got), len(doc.Areas))
	}
	for k, v := range doc.Areas {
		if g, ok := got[k]; !ok || g[0] != v.De || g[1] != v.En {
			t.Errorf("area %q: help.js %v, links.json [%s %s]", k, g, v.De, v.En)
		}
	}

	anchors := map[string]map[string]bool{}
	for k, v := range doc.Areas {
		for _, p := range []string{v.De, v.En} {
			file, frag, _ := strings.Cut(p, "#")
			if _, ok := anchors[file]; !ok {
				md, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
				if err != nil {
					t.Errorf("area %q: %v", k, err)
					anchors[file] = map[string]bool{}
					continue
				}
				anchors[file] = headingAnchors(string(md))
			}
			if frag != "" && !anchors[file][frag] {
				t.Errorf("area %q: no heading for #%s in %s", k, frag, file)
			}
		}
	}

	page, err := os.ReadFile(filepath.Join("static", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	uses := regexp.MustCompile(`data-help="([^"]+)"`).FindAllStringSubmatch(string(page), -1)
	if len(uses) == 0 {
		t.Error("index.html has no data-help")
	}
	for _, m := range uses {
		if _, ok := doc.Areas[m[1]]; !ok {
			t.Errorf("index.html: data-help=%q is not an area of links.json", m[1])
		}
	}
}

// headingAnchors are the GitHub anchors of the markdown headings: lower case,
// letters, digits, spaces, hyphens and underscores kept, spaces to hyphens,
// repeats numbered -1, -2 ...
func headingAnchors(md string) map[string]bool {
	out := map[string]bool{}
	fence := false
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "```") {
			fence = !fence
			continue
		}
		if fence || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		var b strings.Builder
		for _, r := range strings.ToLower(text) {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
				b.WriteRune(r)
			case r == ' ':
				b.WriteRune('-')
			}
		}
		a := b.String()
		for n, base := 1, a; out[a]; n++ {
			a = base + "-" + string(rune('0'+n))
		}
		out[a] = true
	}
	return out
}
