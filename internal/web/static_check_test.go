package web

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Static checks for the GUI sources in static/. The GUI is plain JavaScript
// without a build step, so mistakes such as a missing translation, a typo in
// a data-i18n attribute or a $("#id") for an element that does not exist
// would only show up in the browser. These checks read the files from disk
// and need no Node.js.

// jsFiles are the scripts in the order index.html loads them.
var jsFiles = []string{"i18n.js", "app.js", "disclaimer.js", "lighting.js", "devices.js", "tlstart.js", "conflicts.js", "sketch.js"}

var (
	// dictionary literals: "  de: {" in i18n.js, Object.assign(STRINGS.de, { elsewhere
	dictOpenRe  = regexp.MustCompile(`(?m)^  (de|en): \{|Object\.assign\(STRINGS\.(de|en),\s*\{`)
	placeholdRe = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)
	// t("key") / t('key'), not followed by a concatenation
	tCallRe = regexp.MustCompile(`(?:^|[^\w$.])t\(\s*(?:"([^"\\]+)"|'([^'\\]+)')\s*[,)]`)
	// $("#a .b") / $$("...") selector literals
	selCallRe = regexp.MustCompile(`\$\$?\(\s*(?:"([^"\\]*)"|'([^'\\]*)')`)
	idTokenRe = regexp.MustCompile(`#([A-Za-z_][\w-]*)`)
	idAttrRe  = regexp.MustCompile(`\bid="([^"]+)"`)
	// runtime ids: el("div", {id: "x"}) and node.id = "x"
	elIDRe       = regexp.MustCompile(`\bel\(\s*"[\w-]+",\s*\{[^}]*?\bid:\s*"([\w-]+)"`)
	assignIDRe   = regexp.MustCompile(`\.id\s*=\s*"([\w-]+)"`)
	htmlI18nAttr = regexp.MustCompile(`\bdata-i18n(?:-html|-title|-aria|-placeholder)?="([^"]+)"`)
)

func readStatic(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("static", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// dictBlock is one dictionary object literal.
type dictBlock struct {
	lang, file string
	line       int
	keys       []string
	vals       map[string]string
}

func lineOf(src string, off int) int { return strings.Count(src[:off], "\n") + 1 }

// first returns the first capture group that matched at the submatch indexes m.
func first(src string, m []int) string {
	for i := 2; i+1 < len(m); i += 2 {
		if m[i] >= 0 {
			return src[m[i]:m[i+1]]
		}
	}
	return ""
}

// parseDicts extracts every dictionary literal of a script. Entries must be
// "key": "string" (strings may be joined with +); anything else fails the test.
func parseDicts(t *testing.T, file, src string) []dictBlock {
	t.Helper()
	var out []dictBlock
	for _, m := range dictOpenRe.FindAllStringSubmatchIndex(src, -1) {
		b := dictBlock{lang: first(src, m), file: file, line: lineOf(src, m[0]), vals: map[string]string{}}
		pos := m[1]
		for {
			pos = skipSpace(src, pos)
			if pos >= len(src) {
				t.Fatalf("%s:%d: dictionary is not closed", file, b.line)
			}
			if src[pos] == '}' {
				break
			}
			key, next, ok := readString(src, pos)
			if !ok {
				t.Fatalf("%s:%d: expected a quoted key in the dictionary", file, lineOf(src, pos))
			}
			pos = skipSpace(src, next)
			if pos >= len(src) || src[pos] != ':' {
				t.Fatalf("%s:%d: expected ':' after key %q", file, lineOf(src, pos), key)
			}
			pos++
			var val strings.Builder
			for {
				pos = skipSpace(src, pos)
				s, n, ok := readString(src, pos)
				if !ok {
					t.Fatalf("%s:%d: value of %q is not a string literal", file, lineOf(src, pos), key)
				}
				val.WriteString(s)
				pos = skipSpace(src, n)
				if pos < len(src) && src[pos] == '+' {
					pos++
					continue
				}
				break
			}
			b.keys = append(b.keys, key)
			b.vals[key] = val.String()
			if pos < len(src) && src[pos] == ',' {
				pos++
			} else if pos >= len(src) || src[pos] != '}' {
				t.Fatalf("%s:%d: expected ',' or '}' after the value of %q", file, lineOf(src, pos), key)
			}
		}
		out = append(out, b)
	}
	return out
}

// skipSpace skips white space and JS comments.
func skipSpace(s string, i int) int {
	for i < len(s) {
		switch {
		case s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r':
			i++
		case strings.HasPrefix(s[i:], "//"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return len(s)
			}
			i += j + 4
		default:
			return i
		}
	}
	return i
}

// readString reads a '..' or ".." literal at i and returns it raw (escapes kept).
func readString(s string, i int) (string, int, bool) {
	if i >= len(s) || (s[i] != '"' && s[i] != '\'') {
		return "", i, false
	}
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return s[i+1 : j], j + 1, true
		case '\n':
			return "", i, false
		}
	}
	return "", i, false
}

// loadDicts merges all blocks per language and reports duplicates inside a block.
func loadDicts(t *testing.T) map[string]map[string]string {
	t.Helper()
	merged := map[string]map[string]string{"de": {}, "en": {}}
	for _, f := range jsFiles {
		for _, b := range parseDicts(t, f, readStatic(t, f)) {
			seen := map[string]bool{}
			for _, k := range b.keys {
				if seen[k] {
					t.Errorf("%s:%d: duplicate key %q in this %s dictionary block", b.file, b.line, k, b.lang)
				}
				seen[k] = true
				merged[b.lang][k] = b.vals[k]
			}
		}
	}
	if len(merged["de"]) < 100 || len(merged["en"]) < 100 {
		t.Fatalf("found only %d DE and %d EN keys; is the parser out of sync with the sources?", len(merged["de"]), len(merged["en"]))
	}
	return merged
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func placeholders(s string) []string {
	set := map[string]bool{}
	for _, p := range placeholdRe.FindAllString(s, -1) {
		set[p] = true
	}
	return sortedKeys(set)
}

// TestStaticI18nParity: DE and EN define the same keys with the same
// placeholders, and no dictionary literal repeats a key.
func TestStaticI18nParity(t *testing.T) {
	d := loadDicts(t)
	for _, k := range sortedKeys(d["de"]) {
		en, ok := d["en"][k]
		if !ok {
			t.Errorf("key %q exists in DE but not in EN", k)
			continue
		}
		pd, pe := placeholders(d["de"][k]), placeholders(en)
		if strings.Join(pd, " ") != strings.Join(pe, " ") {
			t.Errorf("key %q: placeholders differ, DE %v vs EN %v", k, pd, pe)
		}
	}
	for _, k := range sortedKeys(d["en"]) {
		if _, ok := d["de"][k]; !ok {
			t.Errorf("key %q exists in EN but not in DE", k)
		}
	}
}

// TestStaticI18nKeysExist: every literal t("key") in the scripts and every
// data-i18n* attribute in index.html names a defined key.
func TestStaticI18nKeysExist(t *testing.T) {
	d := loadDicts(t)
	var missing []string
	check := func(where, key string) {
		if _, ok := d["de"][key]; !ok {
			missing = append(missing, fmt.Sprintf("%s: %q", where, key))
		}
	}
	for _, f := range jsFiles {
		src := readStatic(t, f)
		for _, m := range tCallRe.FindAllStringSubmatchIndex(src, -1) {
			check(fmt.Sprintf("%s:%d t()", f, lineOf(src, m[0])), first(src, m))
		}
	}
	html := readStatic(t, "index.html")
	for _, m := range htmlI18nAttr.FindAllStringSubmatchIndex(html, -1) {
		attr := strings.SplitN(html[m[0]:m[1]], "=", 2)[0]
		check(fmt.Sprintf("index.html:%d %s", lineOf(html, m[0]), attr), first(html, m))
	}
	if len(missing) > 0 {
		t.Errorf("translation keys used but not defined:\n  %s", strings.Join(missing, "\n  "))
	}
}

// TestStaticSelectorIDsExist: every #id in a literal $("...") / $$("...")
// selector is in index.html or created at runtime by the scripts.
func TestStaticSelectorIDsExist(t *testing.T) {
	known := map[string]bool{}
	for _, m := range idAttrRe.FindAllStringSubmatch(readStatic(t, "index.html"), -1) {
		known[m[1]] = true
	}
	for _, f := range jsFiles {
		src := readStatic(t, f)
		for _, re := range []*regexp.Regexp{elIDRe, assignIDRe, idAttrRe} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				known[m[1]] = true
			}
		}
	}
	var missing []string
	for _, f := range jsFiles {
		src := readStatic(t, f)
		for _, m := range selCallRe.FindAllStringSubmatchIndex(src, -1) {
			for _, id := range idTokenRe.FindAllStringSubmatch(first(src, m), -1) {
				if !known[id[1]] {
					missing = append(missing, fmt.Sprintf("%s:%d: #%s", f, lineOf(src, m[0]), id[1]))
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("selectors for ids that are neither in index.html nor created at runtime:\n  %s", strings.Join(missing, "\n  "))
	}
}
