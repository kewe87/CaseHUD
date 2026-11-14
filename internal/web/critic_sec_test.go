package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/access"
	"github.com/kewe87/casehud/internal/aiconnect"
	"github.com/kewe87/casehud/internal/app"
)

// mcpToken pairs a token of the MCP interface with the given rights.
func mcpToken(t *testing.T, s *Server, ts *httptest.Server, name, scope string, system bool) string {
	t.Helper()
	skipRateLimit(s)
	b, _ := json.Marshal(map[string]any{"name": name, "scopes": []string{scope}, "interface": "mcp"})
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", string(b), jsonHdr)
	if code != 202 {
		t.Fatalf("pair: %d %s", code, body)
	}
	var acc v1PairAccepted
	_ = json.Unmarshal([]byte(body), &acc)
	d, _ := json.Marshal(map[string]any{"allow": true, "scope": scope, "system": system, "remember": false})
	if code, body := gui(t, ts, "POST", "/api/access/requests/"+acc.ID, string(d), nil); code != 200 {
		t.Fatalf("decide: %d %s", code, body)
	}
	r := collect(t, ts, acc.ID)
	if r.Token == "" {
		t.Fatalf("no token: %+v", r)
	}
	return r.Token
}

// What the MCP client does not offer through api_call, the server refuses to
// a token of the MCP interface as well, whatever rights it has.
func TestMCPTokenCannotReachWhatMCPBlocks(t *testing.T) {
	s, ts := testServer(t)
	setMCP(t, s, true)
	s.OnQuit = func() { t.Error("quit was called") }
	tok := mcpToken(t, s, ts, "MCP (t)", "control", true)
	time.Sleep(50 * time.Millisecond) // the pairing may bring up the window
	s.OnShow = func() error { return nil }
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok(tok)); code != 200 {
		t.Fatalf("status with an MCP token: %d", code)
	}
	for _, rt := range []struct{ method, path string }{
		{"POST", "/api/v1/quit"}, {"POST", "/api/v1/update/install"}, {"POST", "/api/v1/update/check"},
		{"POST", "/api/v1/gui/open"}, {"GET", "/api/v1/events"}, {"DELETE", "/api/v1/token"},
	} {
		code, body, _ := raw(t, ts, rt.method, rt.path, `{}`, withHdr(jsonHdr, "X-Token", tok))
		if code != 403 || !strings.Contains(body, "not_for_mcp") {
			t.Errorf("%s %s: %d %s", rt.method, rt.path, code, body)
		}
	}
	// A token of the API with the same rights still can give its access up.
	api := pairedWith(t, s, ts, "Prog", "control", true, false)
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", withTok(api)); code != 200 {
		t.Errorf("API token: %d", code)
	}
	if code, _, _ := raw(t, ts, "DELETE", "/api/v1/token", "", withTok(api)); code != 204 {
		t.Errorf("API token gives up its access: %d", code)
	}
}

func TestSystemRightNeedsControlOnTheServer(t *testing.T) {
	s, ts := testServer(t)
	tok := pairedWith(t, s, ts, "RS", "read", true, false)
	_, body, _ := raw(t, ts, "GET", "/api/v1/token", "", withTok(tok))
	if !strings.Contains(body, `"system":false`) {
		t.Errorf("token info: %s", body)
	}
	var id string
	for _, c := range s.acc().List() {
		if c.Name == "RS" {
			id = c.ID
		}
	}
	// Granting system to a read-only program changes nothing.
	if code, _ := gui(t, ts, "PUT", "/api/access/clients/"+id, `{"system":true}`, nil); code != 200 {
		t.Errorf("grant system to read: %d", code)
	}
	for _, c := range s.acc().List() {
		if c.Name == "RS" && c.System {
			t.Error("read-only program has system")
		}
	}
	if code, _, _ := raw(t, ts, "POST", "/api/v1/quit", `{}`, withHdr(jsonHdr, "X-Token", tok)); code != 403 {
		t.Errorf("quit with read+system: %d", code)
	}
}

func TestReservedNameOverTheAPI(t *testing.T) {
	s, ts := testServer(t)
	skipRateLimit(s)
	code, body, _ := raw(t, ts, "POST", "/api/v1/pair", `{"name":"MCP (Claude Desktop)","scopes":["control"]}`, jsonHdr)
	if code != 400 {
		t.Errorf("api pairing under an MCP name: %d %s", code, body)
	}
	setMCP(t, s, true)
	skipRateLimit(s)
	code, body, _ = raw(t, ts, "POST", "/api/v1/pair", `{"name":"MCP (Claude Desktop)","scopes":["control"],"interface":"mcp"}`, jsonHdr)
	if code != 202 {
		t.Errorf("mcp pairing: %d %s", code, body)
	}
	if code, _ := gui(t, ts, "POST", "/api/access/tokens", `{"name":"MCP (x)","scope":"read","system":false}`, nil); code != 400 {
		t.Errorf("token created in the window under an MCP name: %d", code)
	}
}

func TestRenderRefusesMediaAndLimitsLoad(t *testing.T) {
	s, ts := testServer(t)
	read := pairedToken(t, s, ts, "R", "read", false)
	hdr := withHdr(jsonHdr, "X-Token", read)
	code, body, _ := raw(t, ts, "POST", "/api/v1/render?bg=media:clip.gif", `{"width":100,"height":100,"widgets":[]}`, hdr)
	if code != 400 {
		t.Errorf("media background: %d %s", code, body)
	}
	code, _, _ = raw(t, ts, "POST", "/api/v1/render", `{"width":4096,"height":2048,"widgets":[]}`, hdr)
	if code != 200 {
		t.Errorf("large layout: %d", code)
	}
	// All render slots taken: the next request is told to wait.
	for range cap(renderSlots) {
		renderSlots <- struct{}{}
	}
	defer func() {
		for range cap(renderSlots) {
			<-renderSlots
		}
	}()
	code, body, _ = raw(t, ts, "POST", "/api/v1/render", `{"width":100,"height":100,"widgets":[]}`, hdr)
	if code != 503 || !strings.Contains(body, "busy") {
		t.Errorf("busy: %d %s", code, body)
	}
}

func TestWaitingPairingIsListedAndCanBeCancelled(t *testing.T) {
	s, ts := testServer(t)
	id := pairRequest(t, s, ts, "Later", "control")
	decide(t, ts, id, true, "control", true)
	_, body := gui(t, ts, "GET", "/api/access", "", nil)
	var v accessView
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	var found *accessClientView
	for i := range v.Clients {
		if v.Clients[i].ID == id {
			found = &v.Clients[i]
		}
	}
	if found == nil || !found.Waiting || found.Name != "Later" || found.Scope != access.Control {
		t.Fatalf("waiting entry: %s", body)
	}
	if !strings.Contains(body, `"waiting":true`) {
		t.Fatalf("no waiting flag: %s", body)
	}
	if code, _ := gui(t, ts, "DELETE", "/api/access/clients/"+id, "", nil); code != 200 {
		t.Fatalf("cancel: %d", code)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/pair/"+id, "", nil); code != 404 {
		t.Errorf("cancelled pairing: %d", code)
	}
}

func TestAIErrorsCarryACodeAndNoPath(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{&aiconnect.InvalidError{Path: `C:\Users\secret\x.json`, Code: aiconnect.CodeInvalidJSON}, 422, "invalid_json"},
		{&aiconnect.InvalidError{Path: `C:\Users\secret\x.json`, Code: aiconnect.CodeDuplicateKeys}, 422, "duplicate_keys"},
		{&aiconnect.InvalidError{Path: `C:\Users\secret\x.json`, Code: aiconnect.CodeSymlink}, 422, "symlink"},
		{aiconnect.ErrNotFound, 404, "not_found"},
		{aiconnect.ErrUnknownApp, 404, "not_found"},
		{aiconnect.ErrConflict, 409, "conflict"},
		{&aiconnect.CLIError{Err: errors.New(`exec C:\Users\secret\claude.cmd failed`)}, 502, "cli_failed"},
		{errors.New(`open C:\Users\secret\x.json: access denied`), 502, "write_failed"},
	} {
		rec := httptest.NewRecorder()
		aiFail(rec, c.err)
		var b struct{ Code, Message, Error string }
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatalf("%v: %s", c.err, rec.Body)
		}
		if rec.Code != c.status || b.Code != c.code || b.Message == "" || b.Error != b.Message {
			t.Errorf("%v: %d %+v", c.err, rec.Code, b)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("path in the answer: %s", rec.Body)
		}
	}
}

// Switching MCP off ends the open streams of MCP connections.
func TestMCPOffEndsItsStreams(t *testing.T) {
	s, ts := testServer(t)
	setMCP(t, s, true)
	s.SyncAPI()
	tok := mcpToken(t, s, ts, "MCP (t)", "read", false)
	_, end, ok, _, _ := s.authenticate(tok)
	if !ok || len(end) == 0 {
		t.Fatalf("authenticate: %v %d", ok, len(end))
	}
	closed := func() bool {
		for _, c := range end {
			select {
			case <-c:
				return true
			default:
			}
		}
		return false
	}
	if closed() {
		t.Fatal("closed too early")
	}
	if err := s.Engine.PatchSettings(func(st *app.Settings) { st.MCP = false }); err != nil {
		t.Fatal(err)
	}
	s.SyncAPI()
	if !closed() {
		t.Fatal("the end channel of an MCP connection stays open after MCP was switched off")
	}
}

// Two patches at once do not undo each other.
func TestPatchSettingsKeepsBothChanges(t *testing.T) {
	s, _ := testServer(t)
	_ = s.Engine.PatchSettings(func(st *app.Settings) { st.MCP, st.API = false, false })
	done := make(chan struct{}, 2)
	go func() {
		_ = s.Engine.PatchSettings(func(st *app.Settings) { st.MCP = true })
		done <- struct{}{}
	}()
	go func() {
		_ = s.Engine.PatchSettings(func(st *app.Settings) { st.API = true })
		done <- struct{}{}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 2 {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("timeout")
		}
	}
	if st := s.Engine.Settings(); !st.MCP || !st.API {
		t.Fatalf("a change was lost: %+v", st)
	}
}

func TestInternalErrorHidesTheText(t *testing.T) {
	rec := httptest.NewRecorder()
	internalError(rec, errors.New(`open D:\secret\file: denied`))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
