package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	return testServerWith(t, nil)
}

// testServerWith lets a test adjust the engine before it starts.
func testServerWith(t *testing.T, setup func(*app.Engine)) (*Server, *httptest.Server) {
	t.Helper()
	return testServerAck(t, setup, true)
}

// testServerAck is testServerWith; confirmed = the one-time notices for
// lighting and experimental devices are on record.
func testServerAck(t *testing.T, setup func(*app.Engine), confirmed bool) (*Server, *httptest.Server) {
	t.Helper()
	e := &app.Engine{Store: &app.Store{Dir: t.TempDir()}, Demo: true}
	if setup != nil {
		setup(e)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	if confirmed {
		e.Acknowledge("lighting")
		e.Acknowledge("experimental")
	}
	st := e.Settings()
	st.API = true // the v1 tests talk to an enabled API
	if err := e.ApplySettings(st); err != nil {
		t.Fatal(err)
	}
	s := &Server{Engine: e, Version: "test", token: "secret", clientToken: "client"}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func TestAPIRequiresToken(t *testing.T) {
	_, ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d without token, want 403", resp.StatusCode)
	}
	resp, _ = http.Get(ts.URL + "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status %d", resp.StatusCode)
	}
}

func TestStateAndPanelConfig(t *testing.T) {
	_, ts := testServer(t)
	get := func(path string) *http.Response {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		req.Header.Set("X-Token", "secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	var st struct {
		Panels  []app.PanelState `json:"panels"`
		Designs []app.DesignInfo `json:"designs"`
	}
	json.NewDecoder(get("/api/state").Body).Decode(&st)
	if len(st.Panels) != 5 || len(st.Designs) < 5 {
		t.Fatalf("state: %d panels, %d designs", len(st.Panels), len(st.Designs))
	}

	body := `{"mode":"design","design":"clock","brightness":40,"rotation":90}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/panels/"+st.Panels[0].ID, strings.NewReader(body))
	req.Header.Set("X-Token", "secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("put status %d", resp.StatusCode)
	}
	if r := get("/api/designs/clock/preview?w=200"); r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("preview status %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}

	for _, bad := range []string{
		`{"mode":"design","design":"nonexistent"}`,
		`{"mode":"media","media":"../../etc/x"}`,
	} {
		req, _ := http.NewRequest("PUT", ts.URL+"/api/panels/"+st.Panels[0].ID, strings.NewReader(bad))
		req.Header.Set("X-Token", "secret")
		resp, _ := http.DefaultClient.Do(req)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("put %s: status %d, want 400", bad, resp.StatusCode)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"../../evil.png":   "evil.png",
		`C:\x\my cat.gif`:  "my_cat.gif",
		".hidden.jpg":      "hidden.jpg",
		"Bild äöü (1).mp4": "Bild__1.mp4",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryTokenOnlyForReads(t *testing.T) {
	_, ts := testServer(t)
	do := func(method, path string) int {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader("{}"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := do("GET", "/api/designs/clock/preview?w=200&t=secret"); c != 200 {
		t.Fatalf("GET preview with ?t=: %d", c)
	}
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if c := do(m, "/api/lighting/blink?port=1&t=secret"); c != http.StatusForbidden {
			t.Fatalf("%s with ?t=: %d, want 403", m, c)
		}
	}
	// the header still works for writes
	req, _ := http.NewRequest("POST", ts.URL+"/api/lighting/blink?port=1", nil)
	req.Header.Set("X-Token", "secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode == http.StatusForbidden {
		t.Fatal("header token refused")
	}
}
