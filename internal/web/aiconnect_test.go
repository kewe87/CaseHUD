// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/aiconnect"
)

// aiServer is a test server whose AI apps live in temporary folders.
func aiServer(t *testing.T) (*Server, *httptest.Server, aiconnect.Env) {
	t.Helper()
	s, ts := testServer(t)
	root := t.TempDir()
	env := aiconnect.Env{AppData: filepath.Join(root, "appdata"), Home: filepath.Join(root, "home"), Exe: filepath.Join(root, "bin", "casehud-cli.exe"),
		LookPath: func(string) (string, error) { return "", errors.New("none") }}
	if err := os.MkdirAll(filepath.Join(env.AppData, "Claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(env.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	s.AI = &env
	s.DataDir = t.TempDir()
	return s, ts, env
}

func guiDo(t *testing.T, ts *httptest.Server, method, path, body string) (int, string) {
	t.Helper()
	code, out, _ := raw(t, ts, method, path, body, withHdr(jsonHdr, "X-Token", "secret"))
	return code, out
}

func aiState(t *testing.T, ts *httptest.Server, id string) aiAppView {
	t.Helper()
	_, b := guiDo(t, ts, "GET", "/api/ai", "")
	var v aiView
	if err := json.Unmarshal([]byte(b), &v); err != nil {
		t.Fatal(err, b)
	}
	for _, a := range v.Apps {
		if a.ID == id {
			return a
		}
	}
	t.Fatal("no app", id)
	return aiAppView{}
}

func TestAIConnectAndDisconnect(t *testing.T) {
	s, ts, env := aiServer(t)
	if st := aiState(t, ts, "claude-desktop"); st.State != aiconnect.Found {
		t.Fatalf("%+v", st)
	}
	if st := aiState(t, ts, "cursor"); st.State != aiconnect.Missing {
		t.Fatalf("%+v", st)
	}
	cfg := filepath.Join(env.AppData, "Claude", "claude_desktop_config.json")
	if err := os.WriteFile(cfg, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	setMCP(t, s, false)
	code, body := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"read"}`)
	if code != 200 {
		t.Fatal(code, body)
	}
	if !s.Engine.MCPEnabled() {
		t.Fatal("MCP switch not turned on")
	}
	if st := aiState(t, ts, "claude-desktop"); st.State != aiconnect.Connected || st.Scope != "read" {
		t.Fatalf("%+v", st)
	}
	if m, _ := filepath.Glob(cfg + ".casehud-backup-*"); len(m) != 1 {
		t.Fatal("backup", m)
	}
	cfgBytes, _ := os.ReadFile(cfg)
	// The token reaches the MCP server through its own file, never the app's config.
	tokBytes, err := os.ReadFile(filepath.Join(s.DataDir, "mcp-token-claude-desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tf struct{ Token string }
	_ = json.Unmarshal(tokBytes, &tf)
	if tf.Token == "" || strings.Contains(string(cfgBytes), tf.Token) {
		t.Fatal("token missing or leaked into the config")
	}
	// The token is a remembered MCP client with the chosen rights and works on /api/v1.
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", map[string]string{"X-Token": tf.Token}); code != 200 {
		t.Fatalf("v1 status %d", code)
	}
	if code, _, _ := raw(t, ts, "POST", "/api/v1/scenes/x/activate", "", withHdr(jsonHdr, "X-Token", tf.Token)); code != 403 {
		t.Fatalf("read token may change: %d", code)
	}
	var cl struct {
		Clients []accessClientView
	}
	_, ab := guiDo(t, ts, "GET", "/api/access", "")
	_ = json.Unmarshal([]byte(ab), &cl)
	found := 0
	for _, c := range cl.Clients {
		if c.Name == "MCP (Claude Desktop)" && c.Interface == "mcp" && c.Remember && !c.System && c.Scope == "read" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("clients %+v", cl.Clients)
	}
	// Changing the rights replaces the token.
	if code, body := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"control"}`); code != 200 {
		t.Fatal(code, body)
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", map[string]string{"X-Token": tf.Token}); code != 401 && code != 403 {
		t.Fatalf("old token still works: %d", code)
	}
	// Disconnect: entry gone, token revoked, token file gone.
	if code, body := guiDo(t, ts, "DELETE", "/api/ai/claude-desktop", ""); code != 200 {
		t.Fatal(code, body)
	}
	if b, _ := os.ReadFile(cfg); strings.Contains(string(b), "casehud") || !strings.Contains(string(b), "theme") {
		t.Fatalf("config %s", b)
	}
	if _, err := os.Stat(filepath.Join(s.DataDir, "mcp-token-claude-desktop.json")); !os.IsNotExist(err) {
		t.Fatal("token file stays")
	}
	for _, c := range s.acc().List() {
		if c.Interface == "mcp" {
			t.Fatalf("client left: %+v", c)
		}
	}
}

func TestAIRejections(t *testing.T) {
	s, ts, env := aiServer(t)
	cfg := filepath.Join(env.AppData, "Claude", "claude_desktop_config.json")
	_ = os.WriteFile(cfg, []byte("{ not json"), 0o600)
	if code, _ := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"read"}`); code != http.StatusUnprocessableEntity {
		t.Fatal("invalid json:", code)
	}
	if b, _ := os.ReadFile(cfg); string(b) != "{ not json" {
		t.Fatal("file changed")
	}
	if len(s.acc().List()) != 0 {
		t.Fatal("a token was minted for a refused write")
	}
	for _, c := range []struct{ path, body string }{
		{"/api/ai/claude-desktop/connect", `{"scope":"system"}`},
		{"/api/ai/claude-desktop/connect", `{"scope":"read","path":"C:/x"}`},
		{"/api/ai/..%5Cevil/connect", `{"scope":"read"}`},
		{"/api/ai/unknown/connect", `{"scope":"read"}`},
	} {
		if code, _ := guiDo(t, ts, "POST", c.path, c.body); code != 400 && code != 404 {
			t.Errorf("%s %s: %d", c.path, c.body, code)
		}
	}
	// A foreign entry needs "replace".
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"casehud":{"command":"other.exe"}}}`), 0o600)
	if code, _ := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"read"}`); code != http.StatusConflict {
		t.Fatal("foreign entry:", code)
	}
	if code, _ := guiDo(t, ts, "DELETE", "/api/ai/claude-desktop", ""); code != http.StatusConflict {
		t.Fatal("foreign entry removed:", code)
	}
	if code, body := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"read","replace":true}`); code != 200 {
		t.Fatal(code, body)
	}
}

func TestAIHint(t *testing.T) {
	s, ts, _ := aiServer(t)
	hint := func() string {
		_, b := guiDo(t, ts, "GET", "/api/ai", "")
		var v aiView
		_ = json.Unmarshal([]byte(b), &v)
		return v.Hint
	}
	if hint() != "Claude Desktop" {
		t.Fatal("no hint:", hint())
	}
	if code, _ := guiDo(t, ts, "POST", "/api/ai/hint/dismiss", ""); code != 200 {
		t.Fatal(code)
	}
	if hint() != "" || !s.Engine.AIHintDismissed() {
		t.Fatal("hint stays")
	}
}

func TestAccessTokenForOtherProgram(t *testing.T) {
	s, ts, _ := aiServer(t)
	setAPI(t, s, false)
	code, body := guiDo(t, ts, "POST", "/api/access/tokens", `{"name":"Heimautomation","scope":"read"}`)
	if code != 200 {
		t.Fatal(code, body)
	}
	var r struct{ Token, Address, Openapi string }
	_ = json.Unmarshal([]byte(body), &r)
	if len(r.Token) < 32 || !strings.HasSuffix(r.Address, "/api/v1/") || !strings.HasSuffix(r.Openapi, "/api/v1/openapi.json") {
		t.Fatalf("%+v", r)
	}
	if !s.Engine.APIEnabled() {
		t.Fatal("API not switched on")
	}
	if code, _, _ := raw(t, ts, "GET", "/api/v1/status", "", map[string]string{"X-Token": r.Token}); code != 200 {
		t.Fatal("status", code)
	}
	if code, _, _ := raw(t, ts, "POST", "/api/v1/quit", "", withHdr(jsonHdr, "X-Token", r.Token)); code != 403 {
		t.Fatal("quit with a read token:", code)
	}
	// The secret is never listed again.
	if _, ab := guiDo(t, ts, "GET", "/api/access", ""); strings.Contains(ab, r.Token) {
		t.Fatal("token in the list")
	}
	if code, _ := guiDo(t, ts, "POST", "/api/access/tokens", `{"name":" ","scope":"read"}`); code != 400 {
		t.Fatal("empty name:", code)
	}
	if code, _ := guiDo(t, ts, "POST", "/api/access/tokens", `{"name":"x","scope":"root"}`); code != 400 {
		t.Fatal("bad scope:", code)
	}
}

// External programs (every kind of v1 token) cannot reach these routes.
func TestAIRoutesAreGUIOnly(t *testing.T) {
	s, ts, _ := aiServer(t)
	_, tok, err := s.acc().Mint("Prog", "control", true, "api", true)
	if err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"none": "", "paired control+system": tok, "default": s.clientToken} {
		for _, r := range []struct{ m, p, b string }{
			{"GET", "/api/ai", ""},
			{"POST", "/api/ai/claude-desktop/connect", `{"scope":"control"}`},
			{"DELETE", "/api/ai/claude-desktop", ""},
			{"POST", "/api/ai/hint/dismiss", ""},
			{"POST", "/api/access/tokens", `{"name":"x","scope":"control","system":true}`},
		} {
			h := withHdr(jsonHdr)
			if token != "" {
				h["X-Token"] = token
			}
			if code, _, _ := raw(t, ts, r.m, r.p, r.b, h); code != 403 && code != 401 {
				t.Errorf("%s: %s %s answered %d", name, r.m, r.p, code)
			}
		}
	}
	if len(s.acc().List()) != 1 {
		t.Fatal("a program created a token")
	}
}

func TestAIDemoWithoutEnvIsEmpty(t *testing.T) {
	_, ts := testServer(t)
	_, b := guiDo(t, ts, "GET", "/api/ai", "")
	var v aiView
	_ = json.Unmarshal([]byte(b), &v)
	if v.Available || len(v.Apps) != 0 {
		t.Fatalf("%+v", v)
	}
	if code, _ := guiDo(t, ts, "POST", "/api/ai/claude-desktop/connect", `{"scope":"read"}`); code != 404 {
		t.Fatal(code)
	}
}
