// Package update checks GitHub for a newer CaseHUD release and, on request,
// downloads its setup, verifies it against the signed checksums and starts it.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Repo is where releases are published.
const Repo = "kewe87/casehud"

// ReleasesURL is the page the GUI links to.
const ReleasesURL = "https://github.com/" + Repo + "/releases"

// Release is the latest published version.
type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Notes   string `json:"notes,omitempty"` // release notes as plain text
}

// latestURL is overridable in tests only.
var latestURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

// maxNotes caps the release notes shown in the window.
const maxNotes = 8000

// Latest fetches the latest release.
func Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", latestURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "CaseHUD-update-check")
	c := http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release check: HTTP %d", resp.StatusCode)
	}
	var r struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return Release{}, err
	}
	link := r.HTMLURL
	if !strings.HasPrefix(link, "https://github.com/kewe87/") {
		link = "" // never hand the GUI a link to somewhere else
	}
	return Release{Version: strings.TrimPrefix(r.TagName, "v"), URL: link, Notes: cleanNotes(r.Body)}, nil
}

// cleanNotes keeps the notes printable: no control or invisible formatting
// characters (bidi overrides, zero width, tag characters), and at most
// maxNotes runes. The notes also go to API clients.
func cleanNotes(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxNotes {
			b.WriteString(" …")
			break
		}
		switch {
		case r == '\n' || r == '\t':
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			continue
		case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r), r == unicode.ReplacementChar:
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// Newer reports whether version a is newer than b ("1.2.0" > "1.1.9").
// Development builds ("dev", "0.0.0-dev.N") never report updates as newer
// than themselves being outdated is expected.
func Newer(a, b string) bool {
	pa, pb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parse(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}

// IsDev reports whether the running build is a development build.
func IsDev(version string) bool {
	return version == "" || version == "dev" || strings.Contains(version, "dev")
}
