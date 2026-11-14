package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

func putJSON(t *testing.T, url, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, strings.NewReader(body))
	req.Header.Set("X-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestModulesAPI(t *testing.T) {
	s, ts := testServer(t)
	if m := s.Engine.Modules(); m != (app.Modules{Display: true, Lighting: true, Cooling: true}) {
		t.Fatalf("%+v", m)
	}
	if code, _ := putJSON(t, ts.URL+"/api/modules", `{"display":false,"lighting":false,"cooling":false}`); code != http.StatusBadRequest {
		t.Fatalf("all off: status %d", code)
	}
	if m := s.Engine.Modules(); !m.Display || !m.Lighting || !m.Cooling {
		t.Fatalf("changed by a refused request: %+v", m)
	}
	code, body := putJSON(t, ts.URL+"/api/modules", `{"display":true,"lighting":false,"cooling":true}`)
	if code != http.StatusOK {
		t.Fatalf("status %d %s", code, body)
	}
	var m app.Modules
	if err := json.Unmarshal([]byte(body), &m); err != nil || m.Lighting {
		t.Fatalf("%v %+v", err, m)
	}
	// The page learns the state from /api/state.
	req, _ := http.NewRequest("GET", ts.URL+"/api/state", nil)
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st struct {
		Modules app.Modules `json:"modules"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil || st.Modules.Lighting || !st.Modules.Display {
		t.Fatalf("%v %+v", err, st.Modules)
	}
}

func TestCoolingModeAPI(t *testing.T) {
	_, ts := testServer(t)
	if code, _ := putJSON(t, ts.URL+"/api/cooling/mode", `{"mode":"sideways"}`); code != http.StatusBadRequest {
		t.Fatalf("status %d", code)
	}
	if code, body := putJSON(t, ts.URL+"/api/cooling/mode", `{"mode":"preview"}`); code != http.StatusNoContent {
		t.Fatalf("status %d %s", code, body)
	}
}

func postSetup(t *testing.T, url, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("X-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestSetupAPI(t *testing.T) {
	s, ts := testServerWith(t, func(e *app.Engine) { e.DemoSetup = true })
	if !s.Engine.SetupPending() {
		t.Fatal("setup not due")
	}
	if code, _ := postSetup(t, ts.URL+"/api/setup", `{"display":false,"lighting":false,"cooling":false}`); code != http.StatusBadRequest {
		t.Fatalf("all off: status %d", code)
	}
	if !s.Engine.SetupPending() {
		t.Fatal("refused request ended the setup")
	}
	if code, body := postSetup(t, ts.URL+"/api/setup", `{"display":true,"lighting":true,"cooling":false}`); code != http.StatusOK {
		t.Fatalf("status %d %s", code, body)
	}
	if m := s.Engine.Modules(); m.Cooling || !m.Display || !m.Lighting || s.Engine.SetupPending() {
		t.Fatalf("%+v pending %v", m, s.Engine.SetupPending())
	}
	if code, _ := postSetup(t, ts.URL+"/api/setup", `{"display":true,"lighting":true,"cooling":true}`); code != http.StatusConflict {
		t.Fatalf("second answer: status %d", code)
	}
	if code, _ := postSetup(t, ts.URL+"/api/setup/skip", ``); code != http.StatusConflict {
		t.Fatalf("skip after answer: status %d", code)
	}
	if m := s.Engine.Modules(); m.Cooling {
		t.Fatalf("changed by a late request: %+v", m)
	}
}

func TestSetupSkipKeepsAreas(t *testing.T) {
	s, ts := testServerWith(t, func(e *app.Engine) { e.DemoSetup = true })
	if code, _ := postSetup(t, ts.URL+"/api/setup/skip", ``); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if s.Engine.SetupPending() || s.Engine.Modules() != (app.Modules{Display: true, Lighting: true, Cooling: true}) {
		t.Fatalf("pending %v %+v", s.Engine.SetupPending(), s.Engine.Modules())
	}
}
