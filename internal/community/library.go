package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// The library next to the design gallery: components.json, cases.json and
// lighting.json with one JSON file per entry. CaseHUD downloads them only on
// request (the user opens the presets or presses "update"), validates every
// file and keeps a copy in the data directory, so the lighting works offline
// from the last copy. Nothing is sent to the server except the requests for
// these public files.

const (
	maxLibIndex     = 1 << 20
	maxLibFile      = 256 << 10
	maxLibCase      = 512 << 10
	maxLibEntries   = 400
	maxLibTotal     = 24 << 20
	libRefreshWait  = 5 * time.Second
	libRefreshLimit = 90 * time.Second
	libWorkers      = 4
	libFormat       = 1
)

var (
	libIDRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	libSegRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	libKeyRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,31}$`)
	libTagRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
	libFxRe  = regexp.MustCompile(`^[a-z][a-z-]{0,31}$`)
	libHexRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)
)

// Library fetches and caches components, cases and lighting presets.
type Library struct {
	BaseURL string // directory of the index, with a trailing slash
	Dir     string // cache directory
	HTTP    *http.Client
	// Reserved reports IDs that must not be replaced by the library (the
	// built-in components).
	Reserved func(id string) bool

	mu   sync.Mutex // one refresh at a time
	last time.Time
}

// NewLibrary returns a library next to the gallery index at indexURL
// ("" = DefaultIndexURL), cached in dir.
func NewLibrary(indexURL, dir string) (*Library, error) {
	if indexURL == "" {
		indexURL = DefaultIndexURL
	}
	u, err := url.Parse(indexURL)
	if err != nil || u.Host == "" || !(u.Scheme == "https" || (u.Scheme == "http" && isLoopback(u.Hostname()))) {
		return nil, fmt.Errorf("library: invalid address")
	}
	base := u.ResolveReference(&url.URL{Path: "./"})
	base.RawQuery, base.Fragment = "", ""
	return &Library{BaseURL: base.String(), Dir: dir, HTTP: newClient()}, nil
}

// newClient returns a client that follows redirects only within the same
// host and never from https to http.
func newClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("too many redirects")
		}
		first := via[0].URL
		if req.URL.Host != first.Host || req.URL.Scheme != first.Scheme {
			return errors.New("redirect to another host refused")
		}
		return nil
	}}
}

// Preset is a lighting preset as the GUI shows it.
type Preset struct {
	Format      int               `json:"format"`
	ID          string            `json:"id"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description,omitempty"`
	Effect      string            `json:"effect"`
	Params      map[string]any    `json:"params"`
	Palette     []string          `json:"palette,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Author      string            `json:"author,omitempty"`
}

// Summary describes the cached library.
type Summary struct {
	Cached     bool      `json:"cached"`
	Updated    time.Time `json:"updated,omitzero"`
	Components int       `json:"components"`
	Cases      int       `json:"cases"`
	Presets    int       `json:"presets"`
	Warnings   []string  `json:"warnings,omitempty"`
}

type libEntry struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

type libIndex struct {
	Version    int        `json:"version"`
	Components []libEntry `json:"components,omitempty"`
	Cases      []libEntry `json:"cases,omitempty"`
	Presets    []libEntry `json:"presets,omitempty"`
}

// safeRel checks a file reference of an index: a relative path of plain
// segments ending in .json. Anything else (absolute, "..", backslashes, a
// scheme or host) is refused.
func safeRel(f string) bool {
	if f == "" || len(f) > 200 || !strings.HasSuffix(f, ".json") {
		return false
	}
	segs := strings.Split(f, "/")
	if len(segs) > 4 {
		return false
	}
	for _, s := range segs {
		if !libSegRe.MatchString(s) || strings.Contains(s, "..") {
			return false
		}
	}
	return true
}

func (l *Library) get(ctx context.Context, rel string, limit int64) ([]byte, error) {
	base, err := url.Parse(l.BaseURL)
	if err != nil {
		return nil, err
	}
	u := base.ResolveReference(&url.URL{Path: rel})
	if u.Host != base.Host || u.Scheme != base.Scheme {
		return nil, errors.New("file outside the library")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CaseHUD")
	resp, err := l.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("library not reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", rel, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: file too large", rel)
	}
	return b, nil
}

// group is one kind of library content.
type group struct {
	name    string // "components", "cases" or "lighting"
	index   string // index file name
	limit   int64
	check   func(id string, data []byte) ([]byte, error) // returns what to store
	skipNil bool                                         // a nil result means "valid but not stored"
}

func (l *Library) groups() []group {
	return []group{
		{name: "components", index: "components.json", limit: maxLibFile, skipNil: true, check: func(id string, b []byte) ([]byte, error) {
			c, err := leds.ParseLibraryComponent(b)
			if errors.Is(err, leds.ErrNoLEDs) {
				return nil, nil // controllers and sensors: nothing to place
			}
			if err != nil {
				return nil, err
			}
			if c.Name != id {
				return nil, fmt.Errorf("file declares id %q", c.Name)
			}
			return b, nil
		}},
		{name: "cases", index: "cases.json", limit: maxLibCase, check: func(id string, b []byte) ([]byte, error) {
			var f struct {
				Format int    `json:"format"`
				ID     string `json:"id"`
			}
			if err := json.Unmarshal(b, &f); err != nil {
				return nil, err
			}
			if f.Format != libFormat || f.ID != id {
				return nil, errors.New("unsupported format or wrong id")
			}
			return b, nil
		}},
		{name: "lighting", index: "lighting.json", limit: maxLibFile, check: func(id string, b []byte) ([]byte, error) {
			p, err := ParsePreset(b)
			if err != nil {
				return nil, err
			}
			if p.ID != id {
				return nil, fmt.Errorf("file declares id %q", p.ID)
			}
			return json.Marshal(p)
		}},
	}
}

func entriesOf(idx libIndex, name string) []libEntry {
	switch name {
	case "components":
		return idx.Components
	case "cases":
		return idx.Cases
	}
	return idx.Presets
}

// downloaded is one validated file ready for the cache.
type downloaded struct {
	id   string
	data []byte
}

// fetchGroup downloads and validates one group. Single bad files are
// reported as warnings and skipped.
func (l *Library) fetchGroup(ctx context.Context, g group) (files []downloaded, warns []string, err error) {
	raw, err := l.get(ctx, g.index, maxLibIndex)
	if err != nil {
		return nil, nil, err
	}
	var idx libIndex
	// The index key differs per group; decode into the matching field.
	var generic struct {
		Version    int        `json:"version"`
		Components []libEntry `json:"components"`
		Cases      []libEntry `json:"cases"`
		Presets    []libEntry `json:"presets"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, nil, fmt.Errorf("%s: invalid index: %w", g.index, err)
	}
	idx = libIndex(generic)
	if idx.Version != libFormat {
		return nil, nil, fmt.Errorf("%s: unsupported version %d", g.index, idx.Version)
	}
	entries := entriesOf(idx, g.name)
	if len(entries) > maxLibEntries {
		warns = append(warns, fmt.Sprintf("%s: only the first %d entries are used", g.index, maxLibEntries))
		entries = entries[:maxLibEntries]
	}
	var todo []libEntry
	seen := map[string]bool{}
	for _, e := range entries {
		switch {
		case !validLibID(e.ID) || seen[e.ID]:
			warns = append(warns, fmt.Sprintf("%s: entry skipped (invalid or duplicate id)", g.name))
		case !safeRel(e.File):
			warns = append(warns, fmt.Sprintf("%s/%s: file reference refused", g.name, e.ID))
		case g.name == "components" && l.Reserved != nil && l.Reserved(e.ID):
			warns = append(warns, fmt.Sprintf("%s/%s: built-in id, skipped", g.name, e.ID))
		default:
			seen[e.ID] = true
			todo = append(todo, e)
		}
	}
	type result struct {
		d    *downloaded
		warn string
	}
	var got int64
	res := make([]result, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, libWorkers)
	for i, e := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if atomic.LoadInt64(&got) > maxLibTotal {
				res[i].warn = g.name + ": library too large, rest skipped"
				return
			}
			b, err := l.get(ctx, e.File, g.limit)
			atomic.AddInt64(&got, int64(len(b)))
			if err != nil {
				res[i].warn = fmt.Sprintf("%s/%s: %v", g.name, e.ID, err)
				return
			}
			out, err := g.check(e.ID, b)
			if err != nil {
				res[i].warn = fmt.Sprintf("%s/%s: %v", g.name, e.ID, err)
				return
			}
			if out != nil {
				res[i].d = &downloaded{e.ID, out}
			}
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	total := 0
	for _, r := range res {
		if r.warn != "" {
			warns = append(warns, r.warn)
		}
		if r.d != nil {
			total += len(r.d.data)
			if total > maxLibTotal {
				return nil, nil, fmt.Errorf("%s: library too large", g.name)
			}
			files = append(files, *r.d)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].id < files[j].id })
	return files, warns, nil
}

// Refresh downloads the library and replaces the cached copy of every group
// that could be read completely. A group that cannot be fetched keeps its
// old copy; if no group can be fetched the error is returned.
func (l *Library) Refresh(ctx context.Context) (Summary, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.last) < libRefreshWait {
		return l.status(), nil
	}
	l.last = time.Now()
	ctx, cancel := context.WithTimeout(ctx, libRefreshLimit)
	defer cancel()
	var warns []string
	ok := 0
	var firstErr error
	for _, g := range l.groups() {
		files, w, err := l.fetchGroup(ctx, g)
		warns = append(warns, w...)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			warns = append(warns, fmt.Sprintf("%s: not updated (%v)", g.name, err))
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if err := l.store(g, files); err != nil {
			return Summary{}, err
		}
		ok++
	}
	if ok == 0 {
		return Summary{}, firstErr
	}
	if b, err := json.Marshal(map[string]any{"updated": time.Now().UTC().Format(time.RFC3339)}); err == nil {
		writeAtomic(filepath.Join(l.Dir, "meta.json"), b)
	}
	s := l.status()
	s.Warnings = warns
	if len(s.Warnings) > 20 {
		s.Warnings = append(s.Warnings[:20], "…")
	}
	return s, nil
}

// store replaces the cached files of a group by files (flat names derived
// from the validated IDs) and writes the index last.
func (l *Library) store(g group, files []downloaded) error {
	dir := filepath.Join(l.Dir, g.name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	keep := map[string]bool{}
	idx := map[string]any{"version": libFormat}
	var list []libEntry
	for _, f := range files {
		name := f.id + ".json"
		if err := writeAtomic(filepath.Join(dir, name), f.data); err != nil {
			return err
		}
		keep[name] = true
		list = append(list, libEntry{ID: f.id, File: g.name + "/" + name})
	}
	if list == nil {
		list = []libEntry{}
	}
	key := g.name
	if key == "lighting" {
		key = "presets"
	}
	idx[key] = list
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(l.Dir, g.index), append(b, '\n')); err != nil {
		return err
	}
	// Remove copies of entries that left the library.
	if old, err := os.ReadDir(dir); err == nil {
		for _, e := range old {
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") && !keep[e.Name()] {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	return nil
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Status describes the cached library (no network).
func (l *Library) Status() Summary {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status()
}

func (l *Library) status() Summary {
	var s Summary
	if b, err := os.ReadFile(filepath.Join(l.Dir, "meta.json")); err == nil {
		var m struct {
			Updated time.Time `json:"updated"`
		}
		if json.Unmarshal(b, &m) == nil {
			s.Updated, s.Cached = m.Updated, true
		}
	}
	s.Components = len(l.cachedIndex("components.json", "components"))
	s.Cases = len(l.cachedIndex("cases.json", "cases"))
	s.Presets = len(l.cachedIndex("lighting.json", "presets"))
	return s
}

func (l *Library) cachedIndex(file, key string) []libEntry {
	b, err := os.ReadFile(filepath.Join(l.Dir, file))
	if err != nil || len(b) > maxLibIndex {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	var es []libEntry
	if json.Unmarshal(m[key], &es) != nil {
		return nil
	}
	if len(es) > maxLibEntries {
		es = es[:maxLibEntries]
	}
	return es
}

// Presets returns the cached lighting presets (no network), sorted by ID.
func (l *Library) Presets() []Preset {
	out := []Preset{}
	for _, e := range l.cachedIndex("lighting.json", "presets") {
		if p, ok := l.readPreset(e); ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PresetByID returns one cached preset.
func (l *Library) PresetByID(id string) (Preset, bool) {
	for _, e := range l.cachedIndex("lighting.json", "presets") {
		if e.ID == id {
			return l.readPreset(e)
		}
	}
	return Preset{}, false
}

func (l *Library) readPreset(e libEntry) (Preset, bool) {
	if !validLibID(e.ID) || !safeRel(e.File) {
		return Preset{}, false
	}
	b, err := os.ReadFile(filepath.Join(l.Dir, filepath.FromSlash(e.File)))
	if err != nil || len(b) > maxLibFile {
		return Preset{}, false
	}
	p, err := ParsePreset(b)
	if err != nil || p.ID != e.ID {
		return Preset{}, false
	}
	return p, true
}

func okText(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// ParsePreset checks a lighting preset file and returns it in the form the
// GUI uses: the optional "path" block is merged into the parameters. Whether
// the effect and parameters exist is decided when the preset is applied.
func ParsePreset(b []byte) (Preset, error) {
	var f struct {
		Format      int                        `json:"format"`
		ID          string                     `json:"id"`
		Name        map[string]string          `json:"name"`
		Description map[string]string          `json:"description"`
		Effect      string                     `json:"effect"`
		Params      map[string]json.RawMessage `json:"params"`
		Path        *struct {
			Mode    string `json:"mode"`
			Mirror  *bool  `json:"mirror"`
			Reverse *bool  `json:"reverse"`
		} `json:"path"`
		Palette []string `json:"palette"`
		Tags    []string `json:"tags"`
		Author  string   `json:"author"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return Preset{}, err
	}
	if f.Format != libFormat {
		return Preset{}, fmt.Errorf("unsupported format %d", f.Format)
	}
	if !validLibID(f.ID) {
		return Preset{}, errors.New("invalid id")
	}
	if !libFxRe.MatchString(f.Effect) {
		return Preset{}, errors.New("invalid effect name")
	}
	p := Preset{Format: libFormat, ID: f.ID, Effect: f.Effect, Params: map[string]any{}, Name: map[string]string{}}
	for _, lang := range []string{"de", "en"} {
		if n := f.Name[lang]; okText(n, 200) {
			p.Name[lang] = n
		}
		if d := f.Description[lang]; okText(d, 200) {
			if p.Description == nil {
				p.Description = map[string]string{}
			}
			p.Description[lang] = d
		}
	}
	if len(p.Name) == 0 {
		return Preset{}, errors.New("name missing")
	}
	if len(f.Params) > 32 {
		return Preset{}, errors.New("too many parameters")
	}
	for k, raw := range f.Params {
		if !libKeyRe.MatchString(k) {
			return Preset{}, fmt.Errorf("invalid parameter name %q", k)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return Preset{}, err
		}
		switch x := v.(type) {
		case float64, bool:
		case string:
			if len(x) > 400 || strings.ContainsFunc(x, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
				return Preset{}, fmt.Errorf("parameter %q: invalid text", k)
			}
		default:
			return Preset{}, fmt.Errorf("parameter %q: unsupported value", k)
		}
		p.Params[k] = v
	}
	if f.Path != nil {
		switch f.Path.Mode {
		case "":
		case "around", "horizontal", "vertical", "radial":
			p.Params["path"] = f.Path.Mode
		default:
			return Preset{}, errors.New("invalid path mode")
		}
		if f.Path.Mirror != nil {
			p.Params["mirror"] = *f.Path.Mirror
		}
		if f.Path.Reverse != nil {
			p.Params["reverse"] = *f.Path.Reverse
		}
	}
	for i, c := range f.Palette {
		if i >= 6 {
			break
		}
		if libHexRe.MatchString(c) {
			p.Palette = append(p.Palette, c)
		}
	}
	for i, t := range f.Tags {
		if i >= 8 {
			break
		}
		if libTagRe.MatchString(t) {
			p.Tags = append(p.Tags, t)
		}
	}
	if okText(f.Author, 64) {
		p.Author = f.Author
	}
	return p, nil
}

// validLibID accepts library IDs that are safe as file names on Windows too
// (no reserved device names such as con, nul or com1).
func validLibID(id string) bool {
	if !libIDRe.MatchString(id) {
		return false
	}
	base := id
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "con", "prn", "aux", "nul":
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "com") || strings.HasPrefix(base, "lpt")) && base[3] >= '0' && base[3] <= '9' {
		return false
	}
	return true
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
