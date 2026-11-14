package web

import (
	"net/http"
	"strings"
	"testing"
)

const releaseConfig = `{"mode":"preview","curves":[
 {"id":"c","name":"C","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"P","pump":true,"rpm_sensor":"/lpc/chip/0/fan/6","curve":"c","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100},
 {"id":"fan1","name":"F","curve":"c","step_up":100,"step_down":100,"min_duty":20,"safe_duty":100}
]}`

func postJSON(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestReleasePumpRefusesWhatIsNoPump(t *testing.T) {
	_, ts := testServer(t)
	put, _ := http.NewRequest("PUT", ts.URL+"/api/cooling/config", strings.NewReader(releaseConfig))
	put.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(put)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("config status %d", resp.StatusCode)
	}

	url := ts.URL + "/api/cooling/release-pump"
	for name, body := range map[string]string{
		"unknown": `{"id":"nope"}`,
		"fan":     `{"id":"fan1"}`,
		"empty":   `{}`,
		"a key":   `{"id":"/lpc/chip/0/control/6"}`,
		"bad":     `{`,
	} {
		if r := postJSON(t, url, "secret", body); r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d", name, r.StatusCode)
		}
	}
	// A real pump is only refused here because this engine is the demo: nothing is launched.
	r := postJSON(t, url, "secret", `{"id":"pump"}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("demo: status %d", r.StatusCode)
	}
}

func TestReleasePumpNeedsTheToken(t *testing.T) {
	_, ts := testServer(t)
	for _, tok := range []string{"", "wrong", "client"} {
		if r := postJSON(t, ts.URL+"/api/cooling/release-pump", tok, `{"id":"pump"}`); r.StatusCode != http.StatusUnauthorized && r.StatusCode != http.StatusForbidden {
			t.Errorf("token %q: status %d", tok, r.StatusCode)
		}
	}
}
