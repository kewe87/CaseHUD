package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

func TestDisclaimerAPI(t *testing.T) {
	// a fresh engine: nothing accepted, nothing confirmed
	_, ts := testServerAck(t, nil, false)
	post := func(tok, path, body string) (*http.Response, app.Acks) {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("X-Token", tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var a app.Acks
		json.NewDecoder(resp.Body).Decode(&a)
		return resp, a
	}
	var st v1Status
	if r := v1(t, ts, "GET", "/api/v1/status", "", &st); r.StatusCode != 200 || st.DisclaimerAccepted {
		t.Fatalf("status before acceptance: %d accepted=%v", r.StatusCode, st.DisclaimerAccepted)
	}
	// API clients (client token) cannot accept for the user
	if r, _ := post("client", "/api/disclaimer/accept", `{"version":1}`); r.StatusCode != http.StatusForbidden {
		t.Fatalf("client token: %d", r.StatusCode)
	}
	if r, _ := post("secret", "/api/disclaimer/accept", `{"version":99}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong version: %d", r.StatusCode)
	}
	r, a := post("secret", "/api/disclaimer/accept", `{"version":`+strconv.Itoa(app.DisclaimerVersion)+`}`)
	if r.StatusCode != 200 || !a.Accepted || a.AcceptedAt == "" {
		t.Fatalf("accept: %d %+v", r.StatusCode, a)
	}
	if r := v1(t, ts, "GET", "/api/v1/status", "", &st); r.StatusCode != 200 || !st.DisclaimerAccepted {
		t.Fatalf("status after acceptance: %d accepted=%v", r.StatusCode, st.DisclaimerAccepted)
	}
	if r, _ := post("secret", "/api/ack", `{"what":"bogus"}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("bogus ack: %d", r.StatusCode)
	}
	if r, a := post("secret", "/api/ack", `{"what":"lighting"}`); r.StatusCode != 200 || !a.AckLighting || a.AckExperimental {
		t.Fatalf("ack lighting: %d %+v", r.StatusCode, a)
	}
}

func TestOneTimeConfirmationAPI(t *testing.T) {
	_, ts := testServerAck(t, nil, false)
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/v1/lighting", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("X-Token", "client")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("lighting without confirmation: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("PUT", ts.URL+"/api/devices", strings.NewReader(`{"experimental":true}`))
	req.Header.Set("X-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("experimental without confirmation: %d", resp.StatusCode)
	}
}

func TestStartImageRewriteNeedsConfirmFlag(t *testing.T) {
	_, ts := testServerAck(t, nil, true)
	post := func(body string) int {
		req, _ := http.NewRequest("POST", ts.URL+"/api/tl-start-image/rewrite", strings.NewReader(body))
		req.Header.Set("X-Token", "secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := post(`{}`); c != http.StatusConflict {
		t.Fatalf("without confirm: %d", c)
	}
	if c := post(`{"confirm":true}`); c != 200 {
		t.Fatalf("with confirm: %d", c)
	}
}
