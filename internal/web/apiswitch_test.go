package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/app"
)

func TestAPIDefaultOff(t *testing.T) {
	e := &app.Engine{Store: &app.Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if e.APIEnabled() || e.Settings().API {
		t.Fatal("API must be off in a new config")
	}
}

func putAdvanced(t *testing.T, ts *httptest.Server, st app.Settings) {
	t.Helper()
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(st); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("PUT", ts.URL+"/api/settings/advanced", b)
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("settings status %d", resp.StatusCode)
	}
}

func TestAPISwitch(t *testing.T) {
	s, ts := testServer(t)
	dir := t.TempDir()
	s.APIFileDir = dir
	file := filepath.Join(dir, APIFileName)

	// The test server starts with the API on; switch it off through the GUI.
	st := s.Engine.Settings()
	st.API = false
	putAdvanced(t, ts, st)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("api.json exists while the API is off")
	}
	wantError(t, v1(t, ts, "GET", "/api/v1/status", "", nil), 403, "api_disabled")
	// The old token is gone for good, also after switching on again.
	old := s.clientTok()
	if old != "" {
		t.Fatalf("client token kept while off: %q", old)
	}
	// The GUI token still opens /api/v1 and the GUI's own endpoints.
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/status", nil)
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GUI token on v1 while off: %v %v", err, resp)
	}
	resp.Body.Close()

	st.API = true
	putAdvanced(t, ts, st)
	f, err := ReadAPIFile(dir)
	if err != nil || f.Token == "" || f.Token != s.clientTok() {
		t.Fatalf("api.json after switching on: %+v %v", f, err)
	}
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/status", nil)
	req.Header.Set("X-Token", f.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("client token while on: %v %v", err, resp)
	}
	resp.Body.Close()

	st.API = false
	putAdvanced(t, ts, st)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("api.json not removed on switching off")
	}
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/status", nil)
	req.Header.Set("X-Token", f.Token)
	resp, _ = http.DefaultClient.Do(req)
	wantError(t, resp, 403, "api_disabled")
}

func TestAPISwitchNotViaV1(t *testing.T) {
	_, ts := testServer(t)
	for _, path := range []string{"/api/v1/settings", "/api/v1/settings/advanced", "/api/settings/advanced"} {
		for _, m := range []string{"PUT", "PATCH", "POST"} {
			req, _ := http.NewRequest(m, ts.URL+path, bytes.NewBufferString(`{"api":false}`))
			req.Header.Set("X-Token", "client")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 404 && resp.StatusCode != 403 && resp.StatusCode != 405 {
				t.Fatalf("%s %s with client token: %d", m, path, resp.StatusCode)
			}
		}
	}
}

func TestAPISwitchEndsStreams(t *testing.T) {
	s, ts := testServer(t)
	st := s.Engine.Settings()
	s.SyncAPI() // creates the token generation of the running API
	s.apiMu.Lock()
	s.clientToken = "client"
	s.apiMu.Unlock()

	open := func(tok string) *http.Response {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/events", nil)
		req.Header.Set("X-Token", tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("events: %v %v", err, resp)
		}
		return resp
	}
	client, gui := open("client"), open("secret")
	defer gui.Body.Close()
	st.API = false
	putAdvanced(t, ts, st)

	done := make(chan struct{})
	go func() { io.Copy(io.Discard, client.Body); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("client stream still open after switching off")
	}
	// The GUI's stream is untouched: it still delivers (ping/status) data.
	buf := make([]byte, 1)
	gui.Body.(interface{ Read([]byte) (int, error) }).Read(buf)
}
