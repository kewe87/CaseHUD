package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

func sketchReq(t *testing.T, ts *httptest.Server, token, method, path, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

const goodSketch = `{"version":1,"case":{"source":"custom","id":"custom-box","dims":{"w":230,"h":480,"d":450}},"canvasView":"side",
 "parts":[{"id":"a","part":"generic-fan-120","view":"side","x":10,"y":40,"rot":0,"face":"","mirror":false,"flow":"intake","leds":0,"label":"Front","bind":{"device":"demo:board","zone":"argb1","order":0,"parallel":false}}]}`

func cleanSketchComponents() {
	for _, c := range leds.Components() {
		if strings.HasPrefix(c.Name, sketch.ReservedPrefix) {
			leds.UnregisterComponent(c.Name)
		}
	}
}

type sketchAnswer struct {
	Sketch   *sketch.Sketch `json:"sketch"`
	Lighting struct {
		Devices []struct {
			ID    string `json:"id"`
			Zones []struct {
				ID       string         `json:"id"`
				Segments []leds.Segment `json:"segments"`
			} `json:"zones"`
		} `json:"devices"`
	} `json:"lighting"`
}

func (a sketchAnswer) segments(dev, zone string) []leds.Segment {
	for _, d := range a.Lighting.Devices {
		for _, z := range d.Zones {
			if d.ID == dev && z.ID == zone {
				return z.Segments
			}
		}
	}
	return nil
}

func TestSketchAPIRoundTrip(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	if code, b := sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`); code != 200 {
		t.Fatalf("enable: %d %s", code, b)
	}

	code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/catalog", "")
	var cat sketchCatalogView
	if code != 200 || json.Unmarshal(b, &cat) != nil || cat.Parts["generic-fan-120"].ID == "" || cat.Cases[sketch.CustomCaseID].ID == "" || cat.Limits.MaxParts != 64 {
		t.Fatalf("catalog: %d %.200s", code, b)
	}

	code, b = sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", "")
	if code != 200 || strings.TrimSpace(string(b)) != `{"sketch":null}` {
		t.Fatalf("empty get: %d %s", code, b)
	}

	code, b = sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch)
	var put sketchAnswer
	if code != 200 || json.Unmarshal(b, &put) != nil || put.Sketch == nil || len(put.Sketch.Parts) != 1 {
		t.Fatalf("put: %d %.300s", code, b)
	}
	if segs := put.segments("demo:board", "argb1"); len(segs) != 1 || segs[0].Component != "sketch.a" {
		t.Fatalf("segments: %+v", segs)
	}

	code, b = sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", "")
	var got struct {
		Sketch *sketch.Sketch `json:"sketch"`
	}
	if code != 200 || json.Unmarshal(b, &got) != nil || got.Sketch == nil || got.Sketch.Parts[0].Label != "Front" {
		t.Fatalf("get: %d %s", code, b)
	}

	code, b = sketchReq(t, ts, "secret", "DELETE", "/api/lighting/sketch", "")
	var del sketchAnswer
	if code != 200 || json.Unmarshal(b, &del) != nil || del.Sketch != nil || len(del.segments("demo:board", "argb1")) != 0 {
		t.Fatalf("delete: %d %.300s", code, b)
	}
	if _, ok := leds.LookupComponent("sketch.a"); ok {
		t.Error("component survived the delete")
	}
	if code, _ = sketchReq(t, ts, "secret", "DELETE", "/api/lighting/sketch", ""); code != 200 {
		t.Errorf("second delete: %d", code)
	}
}

func TestSketchAPIBadInput(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	bad := map[string]string{
		"not json":       `nope`,
		"empty":          ``,
		"unknown field":  strings.Replace(goodSketch, `"canvasView"`, `"extra":1,"canvasView"`, 1),
		"trailing":       goodSketch + ` {}`,
		"version":        strings.Replace(goodSketch, `"version":1`, `"version":2`, 1),
		"unknown part":   strings.Replace(goodSketch, `generic-fan-120`, `nope`, 1),
		"reserved id":    strings.Replace(goodSketch, `"id":"a"`, `"id":"SKETCH.a"`, 1),
		"unknown device": strings.Replace(goodSketch, `demo:board`, `no:device`, 1),
		"unknown zone":   strings.Replace(goodSketch, `"argb1"`, `"zzz"`, 1),
		"far away":       strings.Replace(goodSketch, `"x":10`, `"x":99999`, 1),
		"control label":  strings.Replace(goodSketch, `"Front"`, `"a\u0007b"`, 1),
		"big number":     strings.Replace(goodSketch, `"x":10`, `"x":1e999`, 1),
	}
	for name, body := range bad {
		code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", body)
		if code != http.StatusBadRequest || len(b) == 0 {
			t.Errorf("%s: %d %s", name, code, b)
		}
	}
	big := `{"version":1,"x":"` + strings.Repeat("a", 2<<20) + `"}`
	if code, _ := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", big); code != http.StatusRequestEntityTooLarge && code != http.StatusBadRequest {
		t.Errorf("oversized body: %d", code)
	}
	if code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", ""); code != 200 || strings.TrimSpace(string(b)) != `{"sketch":null}` {
		t.Fatalf("rejected input stored something: %s", b)
	}
	if code, _ := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch", goodSketch); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", code)
	}
}

// The sketch endpoints are for the GUI only: the API token (v1) and a missing
// token are refused, and the v1 tree has no such routes.
func TestSketchAPIIsGUIOnly(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/lighting/catalog"}, {"GET", "/api/lighting/sketch"},
		{"PUT", "/api/lighting/sketch"}, {"DELETE", "/api/lighting/sketch"},
	} {
		for _, tok := range []string{"", "client", "wrong"} {
			if code, _ := sketchReq(t, ts, tok, tc.method, tc.path, goodSketch); code != http.StatusForbidden {
				t.Errorf("%s %s token %q: %d, want 403", tc.method, tc.path, tok, code)
			}
		}
		if code, _ := sketchReq(t, ts, "client", tc.method, strings.Replace(tc.path, "/api/", "/api/v1/", 1), goodSketch); code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
			t.Errorf("v1 %s %s: %d, want 404", tc.method, tc.path, code)
		}
	}
	if code, _ := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch?t=secret", ""); code != 200 {
		t.Errorf("GUI token: %d", code)
	}
	if code, _ := sketchReq(t, ts, "", "PUT", "/api/lighting/sketch?t=secret", goodSketch); code != http.StatusForbidden {
		t.Errorf("a query token must not write: %d", code)
	}
}

// The sketch and its component segments come back after a restart.
func TestSketchAPIRestart(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	srv, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	if code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch); code != 200 {
		t.Fatalf("put: %d %s", code, b)
	}
	dir := srv.Engine.Store.Dir
	srv.Engine.Stop()
	cleanSketchComponents()

	e2 := &app.Engine{Store: &app.Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e2.Stop)
	ts2 := httptest.NewServer((&Server{Engine: e2, Version: "test", token: "secret", clientToken: "client"}).Handler())
	t.Cleanup(ts2.Close)
	code, b := sketchReq(t, ts2, "secret", "GET", "/api/lighting", "")
	var v sketchAnswer
	if code != 200 || json.Unmarshal(b, &v.Lighting) != nil {
		t.Fatalf("lighting: %d %.200s", code, b)
	}
	if segs := v.segments("demo:board", "argb1"); len(segs) != 1 || segs[0].Shape != "component" || segs[0].Component != "sketch.a" {
		t.Fatalf("segments after restart: %+v", segs)
	}
	if code, b = sketchReq(t, ts2, "secret", "GET", "/api/lighting/sketch", ""); code != 200 || !strings.Contains(string(b), `"id":"a"`) {
		t.Fatalf("sketch after restart: %d %s", code, b)
	}
}

// The catalog carries only what the editor draws and offers, and announces
// the per-zone part limit.
func TestSketchCatalogIsSlim(t *testing.T) {
	_, ts := testServer(t)
	code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/catalog", "")
	if code != 200 {
		t.Fatalf("catalog: %d", code)
	}
	var raw struct {
		Parts  map[string]map[string]json.RawMessage `json:"parts"`
		Limits map[string]json.RawMessage            `json:"limits"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Limits["maxSegments"]) != "16" {
		t.Errorf("limits %v", raw.Limits)
	}
	fan, ok := raw.Parts["generic-fan-120"]
	if !ok {
		t.Fatal("generic fan missing")
	}
	for _, k := range []string{"id", "category", "dimensions_mm", "outline", "leds"} {
		if _, ok := fan[k]; !ok {
			t.Errorf("part field %q missing", k)
		}
	}
	allowed := map[string]bool{"id": true, "vendor": true, "model": true, "category": true, "dimensions_mm": true, "outline": true, "leds": true}
	for k := range fan {
		if !allowed[k] {
			t.Errorf("unexpected part field %q", k)
		}
	}
	var ledsBlock, outline map[string]json.RawMessage
	_ = json.Unmarshal(fan["leds"], &ledsBlock)
	_ = json.Unmarshal(fan["outline"], &outline)
	for _, k := range []string{"zones", "direction"} {
		if _, ok := ledsBlock[k]; ok {
			t.Errorf("leds.%s sent to the GUI", k)
		}
	}
	for _, k := range []string{"side", "top"} {
		if _, ok := outline[k]; ok {
			t.Errorf("outline.%s sent to the GUI", k)
		}
	}
	if _, ok := ledsBlock["positions_mm"]; !ok {
		t.Error("leds.positions_mm missing")
	}
}

func TestSketchAPITooManyPartsInOneZone(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	var parts []string
	for i := 0; i < 17; i++ {
		parts = append(parts, `{"id":"p`+strings.Repeat("x", i)+`","part":"generic-fan-120","view":"side","x":10,"y":40,"leds":4,"bind":{"device":"demo:board","zone":"argb1","order":`+string(rune('0'+i%10))+`}}`)
	}
	body := `{"version":1,"case":{"source":"custom","id":"custom-box","dims":{"w":230,"h":480,"d":450}},"canvasView":"side","parts":[` + strings.Join(parts, ",") + `]}`
	code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", body)
	if code != http.StatusBadRequest || !strings.Contains(string(b), "more than 16 parts in zone demo:board/argb1") {
		t.Fatalf("17 parts: %d %s", code, b)
	}
	for _, c := range leds.Components() {
		if strings.HasPrefix(c.Name, sketch.ReservedPrefix) {
			t.Errorf("component %s registered by a refused sketch", c.Name)
		}
	}
}

// A change that cannot be written to config.json is an error, not a 200.
func TestSketchAPIPersistFailureIs500(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	s, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	path := filepath.Join(s.Engine.Store.Dir, "config.json")
	_ = os.Remove(path)
	if err := os.MkdirAll(filepath.Join(path, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch)
	if code != http.StatusInternalServerError || !strings.Contains(string(b), "could not be written") || strings.Contains(string(b), s.Engine.Store.Dir) {
		t.Fatalf("put: %d %s", code, b)
	}
	code, b = sketchReq(t, ts, "secret", "DELETE", "/api/lighting/sketch", "")
	if code != http.StatusInternalServerError || !strings.Contains(string(b), "could not be written") {
		t.Fatalf("delete: %d %s", code, b)
	}
	if code, _ := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", `nope`); code != http.StatusBadRequest {
		t.Errorf("bad input while the disk fails: %d", code)
	}
}
