package web

import (
	"net/http"
	"strings"
	"testing"
)

// The offline clock is a permanent write: only the page (token) may ask, and
// only for a display that has the clock.
func TestOfflineClockAPI(t *testing.T) {
	_, ts := testServer(t)
	post := func(path, body, token string) int {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
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
	if c := post("/api/panels/us88-0/offline-clock", `{"on":true}`, ""); c != http.StatusForbidden {
		t.Fatalf("without the token: %d", c)
	}
	if c := post("/api/panels/nope/offline-clock", `{"on":true}`, "secret"); c != http.StatusBadRequest {
		t.Fatalf("unknown panel: %d", c)
	}
	// a fan has no offline clock
	if c := post("/api/panels/tllcd-p1-i0/offline-clock", `{"on":true}`, "secret"); c != http.StatusBadRequest {
		t.Fatalf("panel without a clock: %d", c)
	}
	if c := post("/api/panels/us88-0/offline-clock", `{bad`, "secret"); c != http.StatusBadRequest {
		t.Fatalf("bad body: %d", c)
	}
}

func TestDeviceFlipAPI(t *testing.T) {
	_, ts := testServer(t)
	post := func(path, body, token string) int {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
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
	if c := post("/api/panels/us88-0/device-flip", `{"on":true}`, ""); c != http.StatusForbidden {
		t.Fatalf("without the token: %d", c)
	}
	if c := post("/api/panels/nope/device-flip", `{"on":true}`, "secret"); c != http.StatusBadRequest {
		t.Fatalf("unknown panel: %d", c)
	}
	if c := post("/api/panels/us88-0/device-flip", `{bad`, "secret"); c != http.StatusBadRequest {
		t.Fatalf("bad body: %d", c)
	}
}
