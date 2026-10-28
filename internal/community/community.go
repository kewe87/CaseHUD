// Package community reads the community design gallery: an index.json with
// design JSON files next to it, by default in the CaseHUD-Library GitHub
// repository. Previews are rendered locally, so the gallery needs no images.
package community

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
)

// DefaultIndexURL is the gallery shipped with CaseHUD.
const DefaultIndexURL = "https://raw.githubusercontent.com/kewe87/CaseHUD-Library/main/index.json"

// Entry is one design in the gallery.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description,omitempty"`
	Round       bool   `json:"round"`
	File        string `json:"file"` // relative to the index
}

type index struct {
	Version int     `json:"version"`
	Designs []Entry `json:"designs"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

const (
	cacheFor     = 10 * time.Minute
	maxIndexSize = 1 << 20
	maxDesign    = 256 << 10
	maxWidgets   = 64
)

// Gallery fetches and caches the index and designs.
type Gallery struct {
	IndexURL string
	HTTP     *http.Client

	mu      sync.Mutex
	entries []Entry
	fetched time.Time
	designs map[string]lcd.Layout
}

// New returns a gallery for indexURL ("" = DefaultIndexURL).
func New(indexURL string) *Gallery {
	if indexURL == "" {
		indexURL = DefaultIndexURL
	}
	// Same rule as NewLibrary: https (http only for loopback), else the default.
	if u, err := url.Parse(indexURL); err != nil || u.Host == "" || !(u.Scheme == "https" || (u.Scheme == "http" && isLoopback(u.Hostname()))) {
		indexURL = DefaultIndexURL
	}
	return &Gallery{IndexURL: indexURL, HTTP: newClient(), designs: map[string]lcd.Layout{}}
}

func (g *Gallery) get(ctx context.Context, u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CaseHUD")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("community gallery not reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("community gallery: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("community gallery: file too large")
	}
	return b, nil
}

// Entries returns the gallery index, cached for a few minutes.
func (g *Gallery) Entries(ctx context.Context) ([]Entry, error) {
	g.mu.Lock()
	if g.entries != nil && time.Since(g.fetched) < cacheFor {
		defer g.mu.Unlock()
		return g.entries, nil
	}
	g.mu.Unlock()
	b, err := g.get(ctx, g.IndexURL, maxIndexSize)
	if err != nil {
		return nil, err
	}
	var idx index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("community gallery: invalid index: %w", err)
	}
	out := []Entry{}
	seen := map[string]bool{}
	for _, e := range idx.Designs {
		if idRe.MatchString(e.ID) && !seen[e.ID] && e.File != "" {
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	g.mu.Lock()
	g.entries, g.fetched, g.designs = out, time.Now(), map[string]lcd.Layout{}
	g.mu.Unlock()
	return out, nil
}

// Design downloads and validates one gallery design.
func (g *Gallery) Design(ctx context.Context, id string) (lcd.Layout, Entry, error) {
	entries, err := g.Entries(ctx)
	if err != nil {
		return lcd.Layout{}, Entry{}, err
	}
	var e Entry
	for _, x := range entries {
		if x.ID == id {
			e = x
		}
	}
	if e.ID == "" {
		return lcd.Layout{}, Entry{}, fmt.Errorf("design %q is not in the community gallery", id)
	}
	g.mu.Lock()
	l, ok := g.designs[id]
	g.mu.Unlock()
	if ok {
		return l, e, nil
	}
	base, err := url.Parse(g.IndexURL)
	if err != nil {
		return lcd.Layout{}, e, err
	}
	ref, err := url.Parse(e.File)
	if err != nil || ref.IsAbs() || ref.Host != "" {
		return lcd.Layout{}, e, fmt.Errorf("design %q: file must be relative to the index", id)
	}
	b, err := g.get(ctx, base.ResolveReference(ref).String(), maxDesign)
	if err != nil {
		return lcd.Layout{}, e, err
	}
	l, err = Parse(b)
	if err != nil {
		return lcd.Layout{}, e, fmt.Errorf("design %q: %w", id, err)
	}
	if l.Name == "" {
		l.Name = e.Name
	}
	g.mu.Lock()
	g.designs[id] = l
	g.mu.Unlock()
	return l, e, nil
}

// Parse reads and sanity-checks a design file from the internet.
func Parse(b []byte) (lcd.Layout, error) {
	l := lcd.DefaultLayout()
	l.Widgets = nil
	if err := json.Unmarshal(b, &l); err != nil {
		return l, err
	}
	if l.Width <= 0 || l.Height <= 0 || l.Width > 1024 || l.Height > 1024 {
		return l, fmt.Errorf("invalid size %dx%d", l.Width, l.Height)
	}
	if len(l.Widgets) == 0 || len(l.Widgets) > maxWidgets {
		return l, fmt.Errorf("a design needs 1-%d widgets", maxWidgets)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw)
	if _, ok := raw["panel"]; !ok && l.Round {
		l.Panel = "" // tiles get clipped on round screens, like the built-ins
	}
	lcd.Sanitize(&l)
	l.LHMURL = "" // never let a downloaded design point the app at another server
	if l.IntervalMS < 250 {
		l.IntervalMS = 1000
	}
	if l.HistorySize < 2 || l.HistorySize > 600 {
		l.HistorySize = 90
	}
	return l, nil
}
