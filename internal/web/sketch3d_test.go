package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/sketch"
)

// The new endpoints are for the GUI only.
func TestSketch3DEndpointsAreGUIOnly(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/lighting/sketch/upgrade"}, {"GET", "/api/lighting/sketch/suggest"}, {"GET", "/api/lighting/live"},
		{"GET", "/api/lighting/sketch/autobind"}, {"POST", "/api/lighting/sketch/autobind"},
		{"POST", "/api/lighting/sketch/pattern"}, {"POST", "/api/lighting/sketch/orient"},
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
	if code, _ := sketchReq(t, ts, "", "POST", "/api/lighting/sketch/upgrade?t=secret", goodSketch); code != http.StatusForbidden {
		t.Errorf("a query token must not write: %d", code)
	}
	if code, _ := sketchReq(t, ts, "", "GET", "/api/lighting/live?t=secret", ""); code != 200 {
		t.Errorf("GUI token in the query: %d", code)
	}
	if code, _ := sketchReq(t, ts, "secret", "PUT", "/api/lighting/live", ""); code != http.StatusMethodNotAllowed {
		t.Errorf("PUT live: %d", code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type upgradeAnswer struct {
	Sketch  sketch.Sketch        `json:"sketch"`
	Notes   []sketch.UpgradeNote `json:"notes"`
	Invalid string               `json:"invalid"`
}

func TestSketchUpgradeEndpoint(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	for name, body := range map[string]string{
		"plain":   goodSketch,
		"wrapped": `{"sketch":` + goodSketch + `}`, // the answer of GET /api/lighting/sketch
	} {
		code, b := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", body)
		var a upgradeAnswer
		if code != 200 || json.Unmarshal(b, &a) != nil || a.Sketch.Version != 2 || len(a.Sketch.Parts) != 1 || a.Invalid != "" || a.Notes == nil {
			t.Fatalf("%s: %d %.300s", name, code, b)
		}
		if p := a.Sketch.Parts[0]; p.Facing != sketch.FacingSide || p.Bind == nil {
			t.Errorf("%s: part %+v", name, p)
		}
		// a version 2 document passes through unchanged
		var again upgradeAnswer
		code, b = sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", string(mustJSON(t, a.Sketch)))
		if code != 200 || json.Unmarshal(b, &again) != nil || len(again.Sketch.Parts) != 1 || again.Sketch.Parts[0].Pos != a.Sketch.Parts[0].Pos {
			t.Errorf("%s: version 2 through upgrade: %d %s", name, code, b)
		}
	}
	// nothing was stored
	if _, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", ""); strings.TrimSpace(string(b)) != `{"sketch":null}` {
		t.Fatalf("upgrade stored something: %s", b)
	}
	// a document the model would refuse is returned with the reason
	var a upgradeAnswer
	far := strings.Replace(goodSketch, `"x":10`, `"x":99999`, 1)
	code, b := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", far)
	if code != 200 || json.Unmarshal(b, &a) != nil || a.Invalid == "" {
		t.Errorf("far away: %d %s", code, b)
	}
	for name, body := range map[string]string{"not json": `nope`, "empty": ``, "unknown field": strings.Replace(goodSketch, `"canvasView"`, `"extra":1,"canvasView"`, 1)} {
		if code, _ := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", body); code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, code)
		}
	}
	big := `{"version":1,"x":"` + strings.Repeat("a", 2<<20) + `"}`
	if code, _ := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", big); code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: %d", code)
	}
}

// PUT takes the old format and stores the upgraded sketch.
func TestSketchPutV1StoresV2(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch)
	var put sketchAnswer
	if code != 200 || json.Unmarshal(b, &put) != nil || put.Sketch == nil || put.Sketch.Version != 2 {
		t.Fatalf("put v1: %d %.300s", code, b)
	}
	// the same sketch sent as version 2 gives the same segments
	code, b2 := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", string(mustJSON(t, put.Sketch)))
	var put2 sketchAnswer
	if code != 200 || json.Unmarshal(b2, &put2) != nil {
		t.Fatalf("put v2: %d %.300s", code, b2)
	}
	s1, s2 := put.segments("demo:board", "argb1"), put2.segments("demo:board", "argb1")
	if len(s1) != 1 || len(s2) != 1 || s1[0].Component != s2[0].Component || *s1[0].Start != *s2[0].Start {
		t.Fatalf("segments differ: %+v %+v", s1, s2)
	}
}

func TestSketchCatalogCarriesMountFaceAndOrientation(t *testing.T) {
	_, ts := testServer(t)
	_, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/catalog", "")
	var cat struct {
		Limits sketchLimits `json:"limits"`
	}
	if err := json.Unmarshal(b, &cat); err != nil || cat.Limits.Version != 2 || cat.Limits.MaxLayer != 100 || cat.Limits.MaxSpin != 360 ||
		cat.Limits.OverlapTolMM != 0.5 || cat.Limits.MaxBody != 1<<20 {
		t.Fatalf("limits: %v %s", err, b)
	}
	view := newCaseView(sketch.CaseTemplate{ID: "c", Dims: sketch.Dims{W: 1, H: 1, D: 1},
		Orientation: sketch.Orientation{SideViewLeft: sketch.OrientFront, GlassSide: sketch.OrientRight},
		Mounts:      []sketch.Mount{{ID: "m", Face: "front", View: "side", MaxThick: 30}}}, sketch.Builtin())
	out, _ := json.Marshal(view)
	for _, want := range []string{`"orientation":{"side_view_left":"front","glass_side":"right"}`, `"face":"front"`, `"max_thickness_mm":30`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%s missing in %s", want, out)
		}
	}
	if out, _ := json.Marshal(newCaseView(sketch.CaseTemplate{ID: "c"}, sketch.Builtin())); strings.Contains(string(out), "orientation") {
		t.Errorf("default orientation sent: %s", out)
	}
	// lighting built into a case comes as ready placements; a light whose
	// part the catalog does not have is left out
	lit := newCaseView(sketch.CaseTemplate{ID: "c", Dims: sketch.Dims{W: 200, H: 400, D: 400}, Lighting: []sketch.CaseLight{
		{ID: "top", Name: "Top strip", Component: "generic-strip", Path: []sketch.Vec3{{X: 100, Y: 5, Z: 400}, {X: 0, Y: 5, Z: 400}, {X: 0, Y: 5, Z: 50}}},
		{ID: "gone", Component: "no-such-part", Path: []sketch.Vec3{{X: 0, Y: 5, Z: 400}, {X: 0, Y: 5, Z: 50}}},
	}}, sketch.Builtin())
	if len(lit.Lighting) != 1 || lit.Lighting[0].ID != "case-top" || lit.Lighting[0].CaseLight != "top" || lit.Lighting[0].Name != "Top strip" ||
		lit.Lighting[0].LEDs <= 0 || len(lit.Lighting[0].Path) != 3 {
		t.Fatalf("%+v", lit.Lighting)
	}
	if out, _ := json.Marshal(lit); !strings.Contains(string(out), `"caseLight":"top"`) || !strings.Contains(string(out), `"name":"Top strip"`) {
		t.Errorf("lighting: %s", out)
	}
}

func TestLightingLiveAndSuggest(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	srv, ts := testServer(t)
	sketchReq(t, ts, "secret", "PATCH", "/api/lighting", `{"enabled":true}`)
	var live app.LightingLive
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/live", "")
		if code != 200 || json.Unmarshal(b, &live) != nil {
			t.Fatalf("live: %d %s", code, b)
		}
		if live.Devices["demo:board"]["onboard"] != "" || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !live.Active || live.Devices["demo:board"]["onboard"] == "" {
		t.Fatalf("live: %+v", live)
	}
	code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch/suggest", "")
	var sug sketch.Suggestion
	if code != 200 || json.Unmarshal(b, &sug) != nil || sug.Parts == nil {
		t.Fatalf("suggest: %d %s", code, b)
	}
	if _, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", ""); strings.TrimSpace(string(b)) != `{"sketch":null}` {
		t.Fatalf("suggest stored something: %s", b)
	}

	// The lighting area off: 409 area_off for both reads, the sketch can still be edited.
	if err := srv.Engine.SetModules(app.Modules{Display: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/lighting/live", "/api/lighting/sketch/suggest"} {
		code, b := sketchReq(t, ts, "secret", "GET", path, "")
		if code != http.StatusConflict || !strings.HasPrefix(string(b), "area_off") {
			t.Errorf("%s with the area off: %d %s", path, code, b)
		}
	}
	if code, _ := sketchReq(t, ts, "secret", "POST", "/api/lighting/sketch/upgrade", goodSketch); code != 200 {
		t.Errorf("upgrade is pure and works with the area off: %d", code)
	}
	if code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch); code != 200 {
		t.Errorf("put with the area off: %d %s", code, b)
	}
}

// The sketch comes back with the face of every part and the case net; the
// live endpoint adds the effect picture only when asked for.
func TestSketchNetAndLiveCanvas(t *testing.T) {
	cleanSketchComponents()
	t.Cleanup(cleanSketchComponents)
	_, ts := testServer(t)
	if code, b := sketchReq(t, ts, "secret", "PUT", "/api/lighting/sketch", goodSketch); code != 200 {
		t.Fatalf("put: %d %s", code, b)
	}
	code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/sketch", "")
	var got struct {
		Sketch *sketch.Sketch `json:"sketch"`
		Net    *sketch.Net    `json:"net"`
	}
	if code != 200 || json.Unmarshal(b, &got) != nil || got.Sketch == nil || got.Net == nil {
		t.Fatalf("get: %d %s", code, b)
	}
	if got.Sketch.Parts[0].Face != sketch.FacingSide || len(got.Net.Faces) != 5 || !(got.Net.W > 0) {
		t.Errorf("faces %q net %+v", got.Sketch.Parts[0].Face, got.Net)
	}
	if side, ok := got.Net.Face(sketch.FacingSide); !ok || side.Parts != 1 {
		t.Errorf("side face %+v", side)
	}
	var live app.LightingLive
	if code, b := sketchReq(t, ts, "secret", "GET", "/api/lighting/live", ""); code != 200 || json.Unmarshal(b, &live) != nil || live.Canvas != nil {
		t.Errorf("live without canvas: %d %s", code, b)
	}
	code, b = sketchReq(t, ts, "secret", "GET", "/api/lighting/live?canvas=1", "")
	if code != 200 || json.Unmarshal(b, &live) != nil || live.Canvas == nil || live.Canvas.Source == "" || live.Canvas.W < 160 {
		t.Errorf("live with canvas: %d %.300s", code, b)
	}
}
