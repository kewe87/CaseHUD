package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPart = `{"format":1,"id":"my-fan","vendor":"V","model":"Fan","category":"fan","dimensions_mm":{"w":120,"h":120},
 "leds":{"count":2,"positions_mm":[[10,10],[100,100]]}}`

const testPreset = `{"format":1,"id":"red","name":{"de":"Rot","en":"Red"},"effect":"meteor",
 "params":{"color1":"#ff0000","speed":40},"path":{"mode":"around","mirror":true},"palette":["#ff0000","zzz"],"tags":["calm","BAD TAG"],"author":"kewe87"}`

func libServer(t *testing.T, fail *bool) *httptest.Server {
	t.Helper()
	files := map[string]string{
		"/lib/components.json": `{"version":1,"components":[
			{"id":"my-fan","file":"components/v/my-fan.json"},
			{"id":"fan-tl-26","file":"components/v/my-fan.json"},
			{"id":"evil","file":"../secret.json"},
			{"id":"evil2","file":"https://example.com/x.json"},
			{"id":"evil3","file":"components\\x.json"},
			{"id":"BAD ID","file":"components/x.json"},
			{"id":"ctl","file":"components/v/ctl.json"}]}`,
		"/lib/components/v/my-fan.json": testPart,
		"/lib/components/v/ctl.json":    `{"format":1,"id":"ctl","category":"controller","dimensions_mm":{"w":50,"h":50},"leds":{"count":0}}`,
		"/lib/cases.json":               `{"version":1,"cases":[{"id":"case1","file":"cases/c.json"}]}`,
		"/lib/cases/c.json":             `{"format":1,"id":"case1"}`,
		"/lib/lighting.json":            `{"version":1,"presets":[{"id":"red","file":"lighting/red.json"},{"id":"broken","file":"lighting/broken.json"}]}`,
		"/lib/lighting/red.json":        testPreset,
		"/lib/lighting/broken.json":     `{"format":1,"id":"broken","name":{"de":"x"},"effect":"Bad Effect","params":{}}`,
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail != nil && *fail {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		if b, ok := files[r.URL.Path]; ok {
			w.Write([]byte(b))
			return
		}
		http.NotFound(w, r)
	}))
}

func TestLibraryRefreshAndOffline(t *testing.T) {
	down := false
	srv := libServer(t, &down)
	defer srv.Close()
	dir := t.TempDir()
	l, err := NewLibrary(srv.URL+"/lib/index.json", dir)
	if err != nil {
		t.Fatal(err)
	}
	l.Reserved = func(id string) bool { return id == "fan-tl-26" }
	s, err := l.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Components != 1 || s.Cases != 1 || s.Presets != 1 || !s.Cached {
		t.Fatalf("summary %+v (warnings %v)", s, s.Warnings)
	}
	if len(s.Warnings) < 5 {
		t.Fatalf("refused entries must be reported: %v", s.Warnings)
	}
	for _, f := range []string{"components/my-fan.json", "cases/case1.json", "lighting/red.json", "components.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("cache file %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "secret.json")); err == nil {
		t.Error("path traversal wrote a file")
	}
	ps := l.Presets()
	if len(ps) != 1 || ps[0].Params["path"] != "around" || ps[0].Params["mirror"] != true || len(ps[0].Palette) != 1 || len(ps[0].Tags) != 1 {
		t.Fatalf("presets %+v", ps)
	}
	// Offline: the server is down, the cache stays.
	down = true
	l.last = l.last.Add(-libRefreshWait * 2)
	if _, err := l.Refresh(context.Background()); err == nil {
		t.Fatal("refresh against a dead server must fail")
	}
	if got := l.Status(); got.Components != 1 || got.Presets != 1 {
		t.Fatalf("cache lost: %+v", got)
	}
	if len(l.Presets()) != 1 {
		t.Fatal("presets lost offline")
	}
}

func TestLibraryRefreshRateLimit(t *testing.T) {
	srv := libServer(t, nil)
	defer srv.Close()
	l, _ := NewLibrary(srv.URL+"/lib/index.json", t.TempDir())
	if _, err := l.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	hits := 0
	l.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) { hits++; return nil, http.ErrAbortHandler })}
	if _, err := l.Refresh(context.Background()); err != nil || hits != 0 {
		t.Fatalf("second refresh within the wait must not hit the network (hits %d, err %v)", hits, err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLibrarySizeLimit(t *testing.T) {
	big := strings.Repeat("x", maxLibFile+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/components.json":
			w.Write([]byte(`{"version":1,"components":[{"id":"big","file":"big.json"}]}`))
		case "/big.json":
			w.Write([]byte(big))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	l, _ := NewLibrary(srv.URL+"/index.json", t.TempDir())
	s, err := l.Refresh(context.Background())
	if err != nil || s.Components != 0 {
		t.Fatalf("oversized file: %+v %v", s, err)
	}
}

func TestLibraryRedirectOtherHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"version":1,"components":[]}`)) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()
	l, _ := NewLibrary(srv.URL+"/index.json", t.TempDir())
	if _, err := l.Refresh(context.Background()); err == nil {
		t.Fatal("redirect to another host must be refused")
	}
}

func TestParsePresetRejects(t *testing.T) {
	for name, in := range map[string]string{
		"format":  `{"format":2,"id":"a","name":{"en":"x"},"effect":"solid","params":{}}`,
		"id":      `{"format":1,"id":"A b","name":{"en":"x"},"effect":"solid","params":{}}`,
		"param":   `{"format":1,"id":"a","name":{"en":"x"},"effect":"solid","params":{"bad key":1}}`,
		"object":  `{"format":1,"id":"a","name":{"en":"x"},"effect":"solid","params":{"k":{"a":1}}}`,
		"path":    `{"format":1,"id":"a","name":{"en":"x"},"effect":"solid","params":{},"path":{"mode":"x"}}`,
		"noname":  `{"format":1,"id":"a","name":{},"effect":"solid","params":{}}`,
		"control": "{\"format\":1,\"id\":\"a\",\"name\":{\"en\":\"x\"},\"effect\":\"solid\",\"params\":{\"k\":\"a\u0001\"}}",
	} {
		if _, err := ParsePreset([]byte(in)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestValidLibID(t *testing.T) {
	for _, id := range []string{"con", "nul", "aux", "prn", "com1", "lpt9", "Upper", "a b", ""} {
		if validLibID(id) {
			t.Errorf("%q accepted", id)
		}
	}
	for _, id := range []string{"my-fan", "com", "console", "o11-dynamic-evo"} {
		if !validLibID(id) {
			t.Errorf("%q refused", id)
		}
	}
	if _, err := NewLibrary("http://example.com/index.json", t.TempDir()); err == nil {
		t.Error("plain http to a remote host accepted")
	}
}
