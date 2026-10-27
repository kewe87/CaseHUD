// Package update checks GitHub for a newer CaseHUD release. It only reads
// the public releases API; nothing is downloaded or installed.
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
)

// Repo is where releases are published.
const Repo = "kewe87/casehud"

// ReleasesURL is the page the GUI links to.
const ReleasesURL = "https://github.com/" + Repo + "/releases"

// Release is the latest published version.
type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Latest fetches the latest release.
func Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/"+Repo+"/releases/latest", nil)
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
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return Release{}, err
	}
	link := r.HTMLURL
	if !strings.HasPrefix(link, "https://github.com/kewe87/") {
		link = "" // never hand the GUI a link to somewhere else
	}
	return Release{Version: strings.TrimPrefix(r.TagName, "v"), URL: link}, nil
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
