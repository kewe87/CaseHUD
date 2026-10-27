package web

import (
	"encoding/json"

	"github.com/kewe87/casehud/internal/app"
	"net/http"
	"strings"
	"testing"
)

// The way back from monitor mode is a page action: no token, no v1 route.
func TestMonitorBackIsGUIOnly(t *testing.T) {
	_, ts := testServer(t)
	do := func(path, token string) int {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(`{"path":"x"}`))
		if token != "" {
			req.Header.Set("X-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := do("/api/monitor-mode/bring-back", ""); c != http.StatusForbidden {
		t.Fatalf("without the token: %d", c)
	}
	if c := do("/api/monitor-mode/bring-back", "secret"); c != http.StatusNotFound { // nothing listed (demo)
		t.Fatalf("unlisted display: %d", c)
	}
	for _, p := range []string{"/api/v1/monitor-mode/bring-back", "/api/v1/panels/x/bring-back"} {
		req, _ := http.NewRequest("POST", ts.URL+p, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer client")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("%s: %d", p, resp.StatusCode)
		}
	}
}

func TestV1PanelCarriesFirmware(t *testing.T) {
	p := toV1Panel(app.PanelState{ID: "a", Name: "UNI FAN TL LCD (port 1, fan 1)", Firmware: "v1.0"}, 1)
	if p.Firmware != "v1.0" {
		t.Fatalf("firmware %q", p.Firmware)
	}
	b, _ := json.Marshal(toV1Panel(app.PanelState{ID: "b"}, 2))
	if !strings.Contains(string(b), `"firmware":""`) {
		t.Fatalf("unknown firmware not an empty string: %s", b)
	}
}
