package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func confReq(t *testing.T, ts string, method, path, body, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, e := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if e != nil {
			break
		}
	}
	return resp.StatusCode, sb.String()
}

func TestConflictRoutes(t *testing.T) {
	s, ts := testServer(t)
	s.Engine.ConflictsRunning() // the demo list is in place
	if code, _ := confReq(t, ts.URL, "GET", "/api/conflicts", "", ""); code != http.StatusForbidden {
		t.Fatalf("no token: %d", code)
	}
	if code, _ := confReq(t, ts.URL, "GET", "/api/conflicts", "", "client"); code != http.StatusForbidden {
		t.Fatalf("client token opened the GUI route: %d", code)
	}
	code, body := confReq(t, ts.URL, "GET", "/api/conflicts", "", "secret")
	if code != 200 || !strings.Contains(body, `"items"`) {
		t.Fatalf("status %d %s", code, body)
	}
	for _, bad := range []string{`{"exe":"notepad.exe"}`, `{"exe":""}`, `{"exe":"L-Connect 3.exe","pid":4}`, `{`} {
		if code, _ := confReq(t, ts.URL, "POST", "/api/conflicts/close", bad, "secret"); code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, code)
		}
	}
	if code, _ := confReq(t, ts.URL, "PUT", "/api/conflicts/ignore", `{"exe":"cmd.exe","on":true}`, "secret"); code != http.StatusBadRequest {
		t.Errorf("ignore unlisted: %d", code)
	}
	if code, _ := confReq(t, ts.URL, "PUT", "/api/conflicts/ignore", `{"exe":"OpenRGB.exe","on":true}`, "secret"); code != 200 {
		t.Errorf("ignore: %d", code)
	}
	if code, body := confReq(t, ts.URL, "PUT", "/api/conflicts/ask", `{"on":false}`, "secret"); code != 200 || !strings.Contains(body, `"ask":false`) {
		t.Errorf("ask: %d %s", code, body)
	}
}

func TestConflictCloseFlow(t *testing.T) {
	_, ts := testServer(t)
	// hard needs a polite request first
	if code, _ := confReq(t, ts.URL, "POST", "/api/conflicts/close", `{"exe":"L-Connect 3.exe","hard":true}`, "secret"); code != http.StatusConflict {
		t.Fatalf("hard first: %d", code)
	}
	// a service cannot be closed from here
	_, body := confReq(t, ts.URL, "POST", "/api/conflicts/close", `{"exe":"L-Connect-Service.exe"}`, "secret")
	var r struct{ Status, Reason string }
	json.Unmarshal([]byte(body), &r)
	if r.Status != "needs_admin" || r.Reason != "service" {
		t.Fatalf("service: %s", body)
	}
	_, body = confReq(t, ts.URL, "POST", "/api/conflicts/close", `{"exe":"L-Connect 3.exe"}`, "secret")
	json.Unmarshal([]byte(body), &r)
	if r.Status != "closed" {
		t.Fatalf("close: %s", body)
	}
}

func TestV1StatusListsConflictNames(t *testing.T) {
	s, ts := testServer(t)
	s.Engine.ConflictsRunning()
	code, body := confReq(t, ts.URL, "GET", "/api/v1/status", "", "client")
	if code != 200 {
		t.Fatalf("status %d %s", code, body)
	}
	var st struct{ Conflicts []string }
	if err := json.Unmarshal([]byte(body), &st); err != nil || st.Conflicts == nil {
		t.Fatalf("conflicts missing: %s", body)
	}
	for _, n := range st.Conflicts {
		if strings.ContainsAny(n, `\/`) {
			t.Errorf("a path leaked: %q", n)
		}
	}
}
