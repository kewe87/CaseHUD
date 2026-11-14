package web

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/access"
	"github.com/kewe87/casehud/internal/app"
)

// raw sends a request without any default header.
func raw(t *testing.T, ts *httptest.Server, method, path, body string, hdr map[string]string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

var jsonHdr = map[string]string{"Content-Type": "application/json"}

func withHdr(h map[string]string, kv ...string) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

// pairing helpers --------------------------------------------------------

func setMCP(t *testing.T, s *Server, on bool) {
	t.Helper()
	st := s.Engine.Settings()
	st.MCP = on
	if err := s.Engine.ApplySettings(st); err != nil {
		t.Fatal(err)
	}
}

func setAPI(t *testing.T, s *Server, on bool) {
	t.Helper()
	st := s.Engine.Settings()
	st.API = on
	if err := s.Engine.ApplySettings(st); err != nil {
		t.Fatal(err)
	}
	s.SyncAPI() // the token of api.json follows the switch, as in the app
}

var testClocks sync.Map // *Server -> *fakeClock

// skipRateLimit moves the clock of the paired programs on, so the next request
// is not held back by the limit of one per 10 s.
func skipRateLimit(s *Server) {
	v, loaded := testClocks.LoadOrStore(s, &fakeClock{})
	c := v.(*fakeClock)
	if !loaded {
		c.ns.Store(time.Now().UnixNano())
		s.acc().SetClock(c.now)
	}
	c.advance(access.PairInterval)
}

// pairRequest asks for access and returns the id of the request.
func pairRequest(t *testing.T, s *Server, ts *httptest.Server, name string, scopes ...string) string {
	t.Helper()
	skipRateLimit(s)
	b, _ := json.Marshal(map[string]any{"name": name, "scopes": scopes})
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", string(b), jsonHdr)
	if code != 202 {
		t.Fatalf("pair: %d %s", code, body)
	}
	var r v1PairAccepted
	if err := json.Unmarshal([]byte(body), &r); err != nil || r.ID == "" || r.Status != "pending" || r.ExpiresIn != 120 {
		t.Fatalf("pair answer %s", body)
	}
	return r.ID
}

func decide(t *testing.T, ts *httptest.Server, id string, allow bool, scope string, remember bool) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"allow": allow, "scope": scope, "remember": remember})
	if code, body := gui(t, ts, "POST", "/api/access/requests/"+id, string(b), nil); code != 200 {
		t.Fatalf("decide: %d %s", code, body)
	}
}

func collect(t *testing.T, ts *httptest.Server, id string) access.Result {
	t.Helper()
	code, body, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", nil)
	if code != 200 {
		t.Fatalf("poll: %d %s", code, body)
	}
	var r access.Result
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func withTok(tok string) map[string]string { return map[string]string{"X-Token": tok} }

// pairedToken runs the whole flow and returns the token.
func pairedToken(t *testing.T, s *Server, ts *httptest.Server, name, scope string, remember bool) string {
	t.Helper()
	id := pairRequest(t, s, ts, name, scope)
	decide(t, ts, id, true, scope, remember)
	r := collect(t, ts, id)
	if r.Status != "approved" || r.Token == "" {
		t.Fatalf("not approved: %+v", r)
	}
	return r.Token
}

func capturedLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// tests -------------------------------------------------------------------

func TestPairingFlow(t *testing.T) {
	s, ts := testServer(t)
	logs := capturedLog(t)
	id := pairRequest(t, s, ts, "Couch HUD", "read", "control")

	// The window sees the request with the scope asked for.
	code, body := gui(t, ts, "GET", "/api/access", "", nil)
	if code != 200 || !strings.Contains(body, `"Couch HUD"`) || !strings.Contains(body, id) {
		t.Fatalf("window: %d %s", code, body)
	}
	if r := collect(t, ts, id); r.Status != "pending" || r.Token != "" {
		t.Fatalf("before the answer: %+v", r)
	}
	// Nothing works without the answer.
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", nil); code != 403 {
		t.Fatalf("status without token: %d", code)
	}

	decide(t, ts, id, true, "read", true)
	r := collect(t, ts, id)
	if r.Status != "approved" || len(r.Token) != 64 || r.Scope != "read" || !r.Remember {
		t.Fatalf("approved: %+v", r)
	}
	// The secret is shown once.
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", nil); code != 404 {
		t.Fatalf("second poll: %d", code)
	}

	// Read works, control does not.
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok(r.Token)); code != 200 {
		t.Fatalf("GET with the read token: %d", code)
	}
	code, body, _ = raw(t, ts, "PATCH", "/api/v1/panels/x", `{"brightness":5}`, withHdr(jsonHdr, "X-Token", r.Token))
	var e apiError
	json.Unmarshal([]byte(body), &e)
	if code != 403 || e.Error.Code != "insufficient_scope" {
		t.Fatalf("PATCH with the read token: %d %s", code, body)
	}
	if strings.Contains(logs.String(), "by \"Couch HUD\"") {
		t.Fatal("a refused request was logged as done")
	}
	// A POST that only reads works with read access.
	if code, _, _ := raw(t, ts, "POST", "/api/v1/render", `{"width":100,"height":100,"widgets":[]}`, withHdr(jsonHdr, "X-Token", r.Token)); code != 200 {
		t.Fatalf("render with read: %d", code)
	}
	// The window's own routes never answer to a paired token.
	for _, p := range []string{"/api/state", "/api/access"} {
		if code, _, _ := raw(t, ts, "GET", p, "", withTok(r.Token)); code != 403 {
			t.Errorf("%s with a paired token: %d", p, code)
		}
	}

	// Rights change in the settings.
	if code, body := gui(t, ts, "PUT", "/api/access/clients/"+id, `{"scope":"control"}`, nil); code != 200 {
		t.Fatalf("scope: %d %s", code, body)
	}
	pname := s.Engine.Panels()[0].ID
	if code, body, _ := raw(t, ts, "PATCH", "/api/v1/panels/"+pname, `{"brightness":5}`, withHdr(jsonHdr, "X-Token", r.Token)); code != 200 {
		t.Fatalf("PATCH with control: %d %s", code, body)
	}
	if l := logs.String(); !strings.Contains(l, `api: PATCH /api/v1/panels/`+pname+` by "Couch HUD" (paired)`) {
		t.Fatalf("the change was not logged with the name:\n%s", l)
	}
	if strings.Contains(logs.String(), r.Token) {
		t.Fatal("the token is in the log")
	}

	// GET /token says who the caller is; the program can give its access up.
	if code, body, _ := raw(t, ts, "GET", "/api/v1/token", "", withTok(r.Token)); code != 200 || !strings.Contains(body, `"scope":"control"`) || !strings.Contains(body, `"Couch HUD"`) {
		t.Fatalf("token info: %d %s", code, body)
	}

	// Revoked: the next request is refused.
	if code, body := gui(t, ts, "DELETE", "/api/access/clients/"+id, "", nil); code != 200 || strings.Contains(body, `"Couch HUD"`) {
		t.Fatalf("revoke: %d %s", code, body)
	}
	code, body, _ = raw(t, ts, "GET", "/api/v1/status", "", withTok(r.Token))
	json.Unmarshal([]byte(body), &e)
	if code != 403 || e.Error.Code != "revoked" {
		t.Fatalf("after revoke: %d %s", code, body)
	}
}

func TestPairingRefused(t *testing.T) {
	s, ts := testServer(t)
	id := pairRequest(t, s, ts, "Nosy", "control")
	decide(t, ts, id, false, "", false)
	if r := collect(t, ts, id); r.Status != "denied" || r.Token != "" {
		t.Fatalf("%+v", r)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", nil); code != 404 {
		t.Fatal("a refused request is reported once")
	}
	if code, _ := gui(t, ts, "POST", "/api/access/requests/"+id, `{"allow":true,"scope":"read"}`, nil); code != 404 {
		t.Fatalf("deciding a gone request: %d", code)
	}
	if len(s.acc().List()) != 0 {
		t.Fatal("a refused program is listed")
	}
}

func TestPairingNotRemembered(t *testing.T) {
	dir := t.TempDir()
	s, ts := testServer(t)
	s.AccessFile = filepath.Join(dir, "access.json")
	keep := pairedToken(t, s, ts, "Keeper", "control", true)
	temp := pairedToken(t, s, ts, "Temp", "read", false)
	b, err := os.ReadFile(s.AccessFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), keep) || strings.Contains(string(b), temp) || strings.Contains(string(b), "Temp") {
		t.Fatalf("access.json holds a secret or a program that was not remembered:\n%s", b)
	}
	// A new run of CaseHUD: the remembered token works, the other does not.
	s2 := &Server{Engine: s.Engine, Version: "test", token: "secret2", clientToken: "client2", AccessFile: s.AccessFile}
	ts2 := httptest.NewServer(s2.Handler())
	defer ts2.Close()
	if code, _, _ := raw(t, ts2, "GET", "/api/v1/status", "", withTok(keep)); code != 200 {
		t.Fatalf("remembered token after a restart: %d", code)
	}
	if code, _, _ := raw(t, ts2, "GET", "/api/v1/status", "", withTok(temp)); code != 403 {
		t.Fatalf("token that was not remembered: %d", code)
	}
}

func TestPairRequestGuards(t *testing.T) {
	s, ts := testServer(t)
	body := `{"name":"x","scopes":["read"]}`
	cases := []struct {
		name string
		hdr  map[string]string
		body string
		want int
	}{
		{"web page (Origin)", withHdr(jsonHdr, "Origin", "https://evil.example"), body, 403},
		{"simple form post", map[string]string{"Content-Type": "text/plain"}, body, 415},
		{"no content type", nil, body, 415},
		{"wrong Host", withHdr(jsonHdr, "Host", "evil.example:80"), body, 403},
		{"unknown field", jsonHdr, `{"name":"x","admin":true}`, 400},
		{"unknown scope", jsonHdr, `{"name":"x","scopes":["root"]}`, 400},
		{"unknown interface", jsonHdr, `{"name":"x","interface":"ftp"}`, 400},
		{"empty name", jsonHdr, `{"name":"  "}`, 400},
		{"not JSON", jsonHdr, `name=x`, 400},
		{"empty body", jsonHdr, ``, 400},
	}
	for _, tc := range cases {
		skipRateLimit(s)
		if code, b, _ := raw(t, ts, "POST", "/api/v1/pair", tc.body, tc.hdr); code != tc.want {
			t.Errorf("%s: %d %s, want %d", tc.name, code, b, tc.want)
		}
	}
	// Too large.
	skipRateLimit(s)
	big := `{"name":"` + strings.Repeat("a", 2<<20) + `"}`
	if code, _, _ := raw(t, ts, "POST", "/api/v1/pair", big, jsonHdr); code != 413 {
		t.Errorf("oversized body: %d", code)
	}
	// The poll refuses web pages and a spoofed Host too.
	id := pairRequest(t, s, ts, "x", "read")
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", map[string]string{"Origin": "http://evil.example"}); code != 403 {
		t.Errorf("poll with Origin: %d", code)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", map[string]string{"Host": "evil.example:80"}); code != 403 {
		t.Errorf("poll with a wrong Host: %d", code)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id+"/x", "", nil); code != 403 {
		t.Errorf("a longer path is not public: %d", code)
	}
	// A name with control characters is cleaned before the user sees it.
	skipRateLimit(s)
	code, _, _ := raw(t, ts, "POST", "/api/v1/pair", `{"name":"Fine\u0000\n\u202eevil"}`, jsonHdr)
	if code != 202 {
		t.Fatalf("%d", code)
	}
	for _, p := range s.acc().Pending() {
		if strings.ContainsAny(p.Name, "\x00\n\u202e") {
			t.Errorf("name %q", p.Name)
		}
	}
}

func TestPairSpamIsLimited(t *testing.T) {
	_, ts := testServer(t)
	post := func() (int, http.Header) {
		code, _, h := raw(t, ts, "POST", "/api/v1/pair", `{"name":"spam"}`, jsonHdr)
		return code, h
	}
	if code, _ := post(); code != 202 {
		t.Fatalf("first: %d", code)
	}
	code, h := post()
	if code != 429 || h.Get("Retry-After") == "" {
		t.Fatalf("second at once: %d %v", code, h)
	}
}

func TestTooManyPendingRequests(t *testing.T) {
	s, ts := testServer(t)
	for i := 0; i < access.MaxPending; i++ {
		pairRequest(t, s, ts, "p", "read")
	}
	skipRateLimit(s)
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", `{"name":"one more"}`, jsonHdr)
	if code != 429 || !strings.Contains(body, "too_many_pending") {
		t.Fatalf("%d %s", code, body)
	}
}

func TestPairingNeedsTheSwitch(t *testing.T) {
	s, ts := testServer(t)
	skipRateLimit(s)
	// MCP is off by default.
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", `{"name":"x","interface":"mcp"}`, jsonHdr)
	if code != 403 || !strings.Contains(body, "mcp_disabled") {
		t.Fatalf("mcp pairing with the switch off: %d %s", code, body)
	}
	setAPI(t, s, false)
	skipRateLimit(s)
	code, body, _ = raw(t, ts, "POST", "/api/v1/pair", `{"name":"x"}`, jsonHdr)
	if code != 403 || !strings.Contains(body, "api_disabled") {
		t.Fatalf("api pairing with the switch off: %d %s", code, body)
	}
}

// The MCP switch is enforced by the server: a token that was paired for MCP
// stops working when the switch goes off, and an API token does not reach
// through an MCP switch.
func TestMCPSwitchIsEnforcedByTheServer(t *testing.T) {
	s, ts := testServer(t)
	setMCP(t, s, true)
	skipRateLimit(s)
	b, _ := json.Marshal(map[string]any{"name": "MCP (some client)", "scopes": []string{"control"}, "interface": "mcp"})
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", string(b), jsonHdr)
	if code != 202 {
		t.Fatalf("%d %s", code, body)
	}
	var acc v1PairAccepted
	json.Unmarshal([]byte(body), &acc)
	decide(t, ts, acc.ID, true, "control", true)
	mcpTok := collect(t, ts, acc.ID).Token
	apiTok := pairedToken(t, s, ts, "API program", "control", false)

	get := func(tok string) (int, string) {
		code, body, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok(tok))
		return code, body
	}
	if code, _ := get(mcpTok); code != 200 {
		t.Fatalf("mcp token, switch on: %d", code)
	}
	setMCP(t, s, false)
	if code, body := get(mcpTok); code != 403 || !strings.Contains(body, "mcp_disabled") {
		t.Fatalf("mcp token, switch off: %d %s", code, body)
	}
	if code, _ := get(apiTok); code != 200 {
		t.Fatalf("api token while only MCP is off: %d", code)
	}
	// API off, MCP on: the MCP token works, the API tokens do not.
	setMCP(t, s, true)
	setAPI(t, s, false)
	if code, _ := get(mcpTok); code != 200 {
		t.Fatalf("mcp token with the API off: %d", code)
	}
	if code, body := get(apiTok); code != 403 || !strings.Contains(body, "api_disabled") {
		t.Fatalf("api token with the API off: %d %s", code, body)
	}
	if code, _ := get("client"); code != 403 {
		t.Fatalf("the api.json token with the API off: %d", code)
	}
	// Both off: nothing but the window's own token.
	setMCP(t, s, false)
	if code, body := get("nope"); code != 403 || !strings.Contains(body, "api_disabled") {
		t.Fatalf("%d %s", code, body)
	}
	if code, _ := get("secret"); code != 200 {
		t.Fatalf("the window's token: %d", code)
	}
}

func TestDefaultClientKeepsWorking(t *testing.T) {
	s, ts := testServer(t)
	logs := capturedLog(t)
	pname := s.Engine.Panels()[0].ID
	hdr := withHdr(jsonHdr, "X-Token", "client")
	if code, body, _ := raw(t, ts, "PATCH", "/api/v1/panels/"+pname, `{"brightness":7}`, hdr); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	if !strings.Contains(logs.String(), `by "Standard-Zugang" (default)`) {
		t.Fatalf("log:\n%s", logs.String())
	}
	// It is listed, and renewing it ends the old token.
	if code, body := gui(t, ts, "GET", "/api/access", "", nil); code != 200 || !strings.Contains(body, `"builtin":true`) || !strings.Contains(body, "Standard-Zugang") {
		t.Fatalf("%d %s", code, body)
	}
	if code, _ := gui(t, ts, "PUT", "/api/access/clients/default", `{"scope":"read"}`, nil); code != 400 {
		t.Fatalf("scope of the default access: %d", code)
	}
	if code, _ := gui(t, ts, "DELETE", "/api/access/clients/default", "", nil); code != 200 {
		t.Fatalf("renew: %d", code)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok("client")); code != 403 {
		t.Fatalf("old token after renewing: %d", code)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok(s.clientTok())); code != 200 {
		t.Fatalf("new token: %d", code)
	}
}

// Revoking a program ends its open event stream.
func TestRevokeEndsTheStream(t *testing.T) {
	s, ts := testServer(t)
	tok := pairedToken(t, s, ts, "Streamer", "read", false)
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/events", nil)
	req.Header.Set("X-Token", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("%d", resp.StatusCode)
	}
	done := make(chan struct{})
	go func() { io.Copy(io.Discard, resp.Body); close(done) }()
	id := s.acc().List()[0].ID
	if code, _ := gui(t, ts, "DELETE", "/api/access/clients/"+id, "", nil); code != 200 {
		t.Fatalf("revoke: %d", code)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream of a revoked program is still open")
	}
}

func TestProgramsCannotDecideAboutThemselves(t *testing.T) {
	s, ts := testServer(t)
	id := pairRequest(t, s, ts, "Sneaky", "control")
	for _, tok := range []string{"", "client"} {
		hdr := map[string]string{}
		if tok != "" {
			hdr["X-Token"] = tok
		}
		if code, _, _ := raw(t, ts, "POST", "/api/access/requests/"+id, `{"allow":true,"scope":"control"}`, hdr); code != 403 {
			t.Errorf("deciding with token %q: %d", tok, code)
		}
	}
	// Not through /api/v1 either.
	if code, _, _ := raw(t, ts, "POST", "/api/v1/access/requests/"+id, `{"allow":true}`, withHdr(jsonHdr, "X-Token", "client")); code != 404 {
		t.Errorf("/api/v1/access: %d", code)
	}
	if len(s.acc().Pending()) != 1 {
		t.Fatal("the request was decided")
	}
}

// A paired program with read access cannot reach any route that changes state;
// with control access it can. Probes every route of the table.
func TestEveryRouteEnforcesItsScope(t *testing.T) {
	s, ts := testServer(t)
	readTok := pairedToken(t, s, ts, "Reader", "read", false)
	ctlTok := pairedToken(t, s, ts, "Controller", "control", false)
	for _, r := range s.v1Routes() {
		method, path, _ := strings.Cut(r.Pattern, " ")
		if r.Pattern == "DELETE /api/v1/token" {
			continue // it really revokes the token it is called with; TestPairingFlow covers it
		}
		path = strings.ReplaceAll(path, "{id}", "nope")
		hdr := withHdr(jsonHdr, "X-Token", readTok)
		code, body := 0, ""
		if r.Pattern != "GET /api/v1/events" { // an open stream; TestRevokeEndsTheStream covers it
			code, body, _ = raw(t, ts, method, path, `{}`, hdr)
		}
		switch r.Scope {
		case scopeControl, scopeSystem:
			if code != 403 || !strings.Contains(body, "insufficient_scope") {
				t.Errorf("%s with a read token: %d %.100s", r.Pattern, code, body)
			}
		case scopeRead:
			if code == 403 {
				t.Errorf("%s with a read token: %d %.100s", r.Pattern, code, body)
			}
		}
		// No token, a wrong token: refused everywhere except the pairing.
		for _, tok := range []string{"", "wrong"} {
			h := jsonHdr
			if tok != "" {
				h = withHdr(jsonHdr, "X-Token", tok)
			}
			c, b, _ := raw(t, ts, method, path, `{}`, h)
			if r.Scope != scopePublic && (c != 403 || !strings.Contains(b, "unauthorized")) {
				t.Errorf("%s with token %q: %d %.100s", r.Pattern, tok, c, b)
			}
		}
		_ = ctlTok
	}
}

func TestControlTokenReachesControlRoutes(t *testing.T) {
	s, ts := testServer(t)
	ctl := pairedToken(t, s, ts, "Controller", "control", false)
	hdr := withHdr(jsonHdr, "X-Token", ctl)
	if code, body, _ := raw(t, ts, "POST", "/api/v1/power", `{"on":true}`, hdr); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	if code, body, _ := raw(t, ts, "GET", "/api/v1/openapi.json", "", withTok(ctl)); code != 200 || !strings.Contains(body, `"openapi":"3.1.0"`) {
		t.Fatalf("openapi: %d %.80s", code, body)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/openapi.json", "", nil); code != 403 {
		t.Fatalf("openapi without a token: %d", code)
	}
}

func TestAccessFileTempIsCleaned(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "access.json.123.tmp")
	os.WriteFile(stale, []byte("x"), 0o600)
	s := &Server{Engine: &app.Engine{}, AccessFile: filepath.Join(dir, "access.json")}
	s.acc()
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale temp file stays")
	}
}

// A request brings the window up unless the window is already watching.
func TestPairRequestBringsUpTheWindow(t *testing.T) {
	s, ts := testServer(t)
	shown := make(chan struct{}, 4)
	s.OnShow = func() error { shown <- struct{}{}; return nil }
	pairRequest(t, s, ts, "first", "read")
	select {
	case <-shown:
	case <-time.After(2 * time.Second):
		t.Fatal("the window was not asked to show")
	}
	// The window polls: no second pop-up.
	if code, _ := gui(t, ts, "GET", "/api/access", "", nil); code != 200 {
		t.Fatal(code)
	}
	pairRequest(t, s, ts, "second", "read")
	select {
	case <-shown:
		t.Fatal("the window was asked to show while it watches")
	case <-time.After(200 * time.Millisecond):
	}
}

// pairedWith runs the pairing flow with a scope and the system right as the
// user decided them.
func pairedWith(t *testing.T, s *Server, ts *httptest.Server, name, scope string, system, remember bool) string {
	t.Helper()
	scopes := []string{scope}
	if system {
		scopes = append(scopes, "system")
	}
	id := pairRequest(t, s, ts, name, scopes...)
	b, _ := json.Marshal(map[string]any{"allow": true, "scope": scope, "system": system, "remember": remember})
	if code, body := gui(t, ts, "POST", "/api/access/requests/"+id, string(b), nil); code != 200 {
		t.Fatalf("decide: %d %s", code, body)
	}
	r := collect(t, ts, id)
	if r.Status != "approved" || r.System != (system && scope == "control") {
		t.Fatalf("answer %+v", r)
	}
	return r.Token
}

// "system" is a right of its own: control does not include it, and it does not
// include control. Quit, update and the window need it.
func TestSystemRightIsSeparate(t *testing.T) {
	s, ts := testServer(t)
	quit := make(chan struct{}, 1)
	s.OnQuit = func() { quit <- struct{}{} }
	s.OnShow = func() error { return nil }
	if err := s.Engine.SetUpdateCheck(false); err != nil { // no network, 409 instead
		t.Fatal(err)
	}
	toks := map[string]string{
		"read":           pairedWith(t, s, ts, "R", "read", false, false),
		"control":        pairedWith(t, s, ts, "C", "control", false, false),
		"control+system": pairedWith(t, s, ts, "CS", "control", true, false),
		"read+system":    pairedWith(t, s, ts, "RS", "read", true, false),
		"default":        "client",
		"gui":            "secret",
	}
	var sys []string
	for _, r := range s.v1Routes() {
		if r.Scope == scopeSystem {
			sys = append(sys, r.Pattern)
		}
	}
	want := []string{"POST /api/v1/gui/open", "POST /api/v1/quit", "POST /api/v1/update/check", "POST /api/v1/update/install"}
	slices.Sort(sys)
	if !slices.Equal(sys, want) {
		t.Fatalf("system routes: %v", sys)
	}
	for _, pat := range want {
		method, path, _ := strings.Cut(pat, " ")
		for name, tok := range toks {
			code, body, _ := raw(t, ts, method, path, `{}`, withHdr(jsonHdr, "X-Token", tok))
			needs := name == "control+system" || name == "default" || name == "gui" // read+system is read: system needs control
			if needs && code == 403 || !needs && (code != 403 || !strings.Contains(body, "insufficient_scope") || !strings.Contains(body, "system")) {
				t.Errorf("%s with %s: %d %.120s", pat, name, code, body)
			}
		}
	}
	// A system-only program still cannot change anything else.
	if code, _, _ := raw(t, ts, "POST", "/api/v1/power", `{"on":true}`, withHdr(jsonHdr, "X-Token", toks["read+system"])); code != 403 {
		t.Errorf("read+system changed the power: %d", code)
	}
	// What the token says about itself.
	_, body, _ := raw(t, ts, "GET", "/api/v1/token", "", withTok(toks["control"]))
	if !strings.Contains(body, `"system":false`) {
		t.Errorf("token info: %s", body)
	}
	_, body, _ = raw(t, ts, "GET", "/api/v1/token", "", withTok(toks["control+system"]))
	if !strings.Contains(body, `"system":true`) {
		t.Errorf("token info: %s", body)
	}
	// The user takes the right back, at once.
	cl := s.acc().List()
	var id string
	for _, c := range cl {
		if c.Name == "CS" {
			id = c.ID
		}
	}
	if code, body := gui(t, ts, "PUT", "/api/access/clients/"+id, `{"system":false}`, nil); code != 200 || !strings.Contains(body, `"system":false`) {
		t.Fatalf("take back: %d %s", code, body)
	}
	if code, _, _ := raw(t, ts, "POST", "/api/v1/quit", `{}`, withHdr(jsonHdr, "X-Token", toks["control+system"])); code != 403 {
		t.Errorf("quit after the right was taken back: %d", code)
	}
	if code, _ := gui(t, ts, "PUT", "/api/access/clients/"+id, `{}`, nil); code != 400 {
		t.Errorf("empty change: %d", code)
	}
	// Unknown scope names are refused in the request.
	skipRateLimit(s)
	if code, _, _ := raw(t, ts, "POST", "/api/v1/pair", `{"name":"x","scopes":["root"]}`, jsonHdr); code != 400 {
		t.Errorf("unknown scope: %d", code)
	}
	// The built-in access shows system in the list.
	_, body = gui(t, ts, "GET", "/api/access", "", nil)
	if !strings.Contains(body, `"builtin":true`) || !strings.Contains(body, `"system":true`) {
		t.Errorf("access list: %s", body)
	}
}
