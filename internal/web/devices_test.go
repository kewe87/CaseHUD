package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/panel"
)

func TestDevicesAPI(t *testing.T) {
	_, ts := testServer(t)
	t.Cleanup(func() { panel.SetOptions(panel.Options{}) })
	call := func(method, body string) (*http.Response, app.DevicesState) {
		req, _ := http.NewRequest(method, ts.URL+"/api/devices", strings.NewReader(body))
		req.Header.Set("X-Token", "secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var st app.DevicesState
		json.NewDecoder(resp.Body).Decode(&st)
		return resp, st
	}
	resp, st := call("GET", "")
	if resp.StatusCode != 200 || st.Experimental || len(st.Drivers) == 0 {
		t.Fatalf("GET: %d %+v", resp.StatusCode, st)
	}
	// without the page token nothing can be switched, and the client token
	// (API v1 / integrations) is not enough either
	for _, tok := range []string{"", "client"} {
		req, _ := http.NewRequest("PUT", ts.URL+"/api/devices", strings.NewReader(`{"experimental":true}`))
		if tok != "" {
			req.Header.Set("X-Token", tok)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusForbidden {
			t.Fatalf("PUT with token %q: %d", tok, r.StatusCode)
		}
	}
	if _, st := call("GET", ""); st.Experimental {
		t.Fatal("switched on without the page token")
	}
	for _, bad := range []string{`{"off":["tllcd"]}`, `{"off":["nope"]}`, `{"experimental":true,"bogus":1}`, `not json`} {
		if resp, _ := call("PUT", bad); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("PUT %s: %d, want 400", bad, resp.StatusCode)
		}
	}
	resp, st = call("PUT", `{"experimental":true,"off":["flex"]}`)
	if resp.StatusCode != 200 || !st.Experimental {
		t.Fatalf("PUT: %d %+v", resp.StatusCode, st)
	}
	for _, d := range st.Drivers {
		if d.ID == "flex" && (d.Enabled || !d.Off) {
			t.Fatalf("flex should be off: %+v", d)
		}
		if d.ID == "slv3" && !d.Enabled {
			t.Fatalf("slv3 should be on: %+v", d)
		}
	}
	// omitting "off" keeps the individual switches
	_, st = call("PUT", `{"experimental":false}`)
	for _, d := range st.Drivers {
		if d.ID == "flex" && !d.Off {
			t.Fatal("individual switch lost")
		}
	}
}
