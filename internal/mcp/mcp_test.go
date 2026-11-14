package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServe(t *testing.T) {
	s := &Server{Name: "t", Version: "1", Tools: []Tool{{
		Name: "echo", InputSchema: map[string]any{"type": "object"},
		Handler: func(a json.RawMessage) (ToolResult, error) { return Text(string(a)), nil },
	}}}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"x":1}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"nope"}`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 responses (no reply to notifications), got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion":"2025-03-26"`) {
		t.Errorf("initialize should echo a supported version: %s", lines[0])
	}
	if !strings.Contains(lines[1], `{\"x\":1}`) {
		t.Errorf("tool result: %s", lines[1])
	}
	if !strings.Contains(lines[2], `-32601`) {
		t.Errorf("unknown method: %s", lines[2])
	}
}

func TestNewAPI(t *testing.T) {
	a, err := NewAPI("http://127.0.0.1:1234/?t=abc\n")
	if err != nil || a.Base != "http://127.0.0.1:1234" {
		t.Fatalf("NewAPI = %+v, %v", a, err)
	}
	if strings.Contains(a.Base, "abc") || a.token != "" {
		t.Fatal("the token of the window must not be taken over")
	}
	if _, err := NewAPI("garbage"); err == nil {
		t.Fatal("invalid instance URL accepted")
	}
}

func TestInvalidParams(t *testing.T) {
	s := &Server{Name: "t", Version: "1", Tools: []Tool{{
		Name: "args", InputSchema: map[string]any{"type": "object"},
		Handler: func(a json.RawMessage) (ToolResult, error) {
			var p struct{ N int }
			if err := ParseArgs(a, &p); err != nil {
				return ToolResult{}, err
			}
			return Text("ok"), nil
		},
	}}}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"args","arguments":{"N":"x"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"args","arguments":{"N":2}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"args"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"initialize","params":[1]}`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 responses, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `-32602`) || strings.Contains(lines[0], `"result"`) {
		t.Errorf("bad arguments should be an invalid-params error: %s", lines[0])
	}
	for _, l := range lines[1:3] {
		if !strings.Contains(l, `"ok"`) {
			t.Errorf("valid or missing arguments should succeed: %s", l)
		}
	}
	if !strings.Contains(lines[3], `-32602`) {
		t.Errorf("malformed initialize params: %s", lines[3])
	}
}

// fakeCaseHUD answers like API v1: it pairs once, then wants the token.
type fakeCaseHUD struct {
	token    string
	approve  string // "approved", "denied" or "pending"
	pairs    int
	requests []string
	status   string // body of GET /status
	patchErr string // body of a refused PATCH (409)
	paired   []string
}

func (f *fakeCaseHUD) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("X-Token"))
	switch {
	case r.Method == "POST" && r.URL.Path == "/api/v1/pair":
		f.pairs++
		b, _ := io.ReadAll(r.Body)
		f.paired = append(f.paired, string(b))
		w.WriteHeader(202)
		io.WriteString(w, `{"id":"req1","status":"pending","expiresIn":120}`)
	case r.Method == "GET" && r.URL.Path == "/api/v1/pair/req1":
		switch f.approve {
		case "approved":
			io.WriteString(w, `{"status":"approved","token":"`+f.token+`","scope":"control"}`)
		case "denied":
			io.WriteString(w, `{"status":"denied"}`)
		default:
			io.WriteString(w, `{"status":"pending"}`)
		}
	case r.Header.Get("X-Token") != f.token || f.token == "":
		w.WriteHeader(403)
		io.WriteString(w, `{"error":{"code":"unauthorized","message":"missing or wrong token"}}`)
	case r.Method == "GET" && r.URL.Path == "/api/v1/status":
		io.WriteString(w, f.status)
	case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/api/v1/panels/"):
		if f.patchErr != "" {
			w.WriteHeader(409)
			io.WriteString(w, f.patchErr)
			return
		}
		io.WriteString(w, `{"id":"p1","name":"P","mode":"hud","design":"d"}`)
	default:
		w.WriteHeader(404)
		io.WriteString(w, `{"error":{"code":"not_found","message":"no"}}`)
	}
}

func newFake(t *testing.T, f *fakeCaseHUD) (*API, map[string]Tool) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	api, err := NewAPI(srv.URL + "/?t=windowtoken")
	if err != nil {
		t.Fatal(err)
	}
	api.TokenFile = filepath.Join(t.TempDir(), TokenFileName)
	api.PairWait, api.PollEvery = 30*time.Millisecond, 10*time.Millisecond
	tools := map[string]Tool{}
	for _, tool := range NewCaseHUD(api, "t").Tools {
		tools[tool.Name] = tool
	}
	return api, tools
}

const panelsStatus = `{"areas":{"display":DISPLAY},"panels":[{"id":"p1","name":"P"}]}`

func statusWith(display string) string { return strings.Replace(panelsStatus, "DISPLAY", display, 1) }

// A display tool refuses to change anything while the display area is off, and
// says so while listing.
func TestToolsRespectAnAreaThatIsOff(t *testing.T) {
	f := &fakeCaseHUD{token: "tok", approve: "approved", status: statusWith("false"),
		patchErr: `{"error":{"code":"area_off","message":"the display area is switched off"}}`}
	_, tools := newFake(t, f)
	list, listErr := tools["list_displays"].Handler(nil)
	if listErr != nil || !strings.Contains(list.Content[0].Text, "area_off") {
		t.Fatalf("list_displays: %v %+v", listErr, list)
	}
	_, applyErr := tools["apply_design"].Handler(json.RawMessage(`{"display_id":"p1","design_id":"d"}`))
	if applyErr == nil || !strings.Contains(applyErr.Error(), "area_off") {
		t.Fatalf("apply_design: %v", applyErr)
	}
	f.status = statusWith("true")
	list, _ = tools["list_displays"].Handler(nil)
	if strings.Contains(list.Content[0].Text, "area_off") || !strings.Contains(list.Content[0].Text, `"p1"`) {
		t.Fatalf("area on: %+v", list)
	}
}

// Without a token the server asks for access, tells the user what to do while
// nobody answered, and stores the token once it is granted.
func TestFirstCallPairsAndStoresTheToken(t *testing.T) {
	f := &fakeCaseHUD{token: "secret-token", approve: "pending", status: statusWith("true")}
	api, tools := newFake(t, f)
	api.SetClient("Some Assistant")

	_, err := tools["list_displays"].Handler(nil)
	if err == nil || !strings.Contains(err.Error(), "Bitte in CaseHUD den Zugriff bestätigen") {
		t.Fatalf("unanswered: %v", err)
	}
	if f.pairs != 1 || !strings.Contains(f.paired[0], `"name":"MCP (Some Assistant)"`) || !strings.Contains(f.paired[0], `"interface":"mcp"`) {
		t.Fatalf("pair request %v", f.paired)
	}
	if _, err := os.Stat(api.TokenFile); err == nil {
		t.Fatal("a token file without a token")
	}
	// The user answers: the next call finishes the same request, no second one.
	f.approve = "approved"
	res, err := tools["list_displays"].Handler(nil)
	if err != nil || !strings.Contains(res.Content[0].Text, `"p1"`) {
		t.Fatalf("after approval: %v %+v", err, res)
	}
	if f.pairs != 1 {
		t.Fatalf("%d pair requests", f.pairs)
	}
	b, err := os.ReadFile(api.TokenFile)
	if err != nil || !strings.Contains(string(b), "secret-token") {
		t.Fatalf("token file: %v %s", err, b)
	}
	for _, r := range f.requests {
		if strings.Contains(r, "windowtoken") {
			t.Fatal("the token of the window was used")
		}
	}
	// A new run reads the token and does not ask again.
	api2, _ := NewAPI(api.Base)
	api2.TokenFile = api.TokenFile
	if _, err := api2.call("GET", "/api/v1/status", nil, nil); err != nil || f.pairs != 1 {
		t.Fatalf("second run: %v, %d pairs", err, f.pairs)
	}
}

func TestDeniedAndRevoked(t *testing.T) {
	f := &fakeCaseHUD{token: "tok", approve: "denied", status: statusWith("true")}
	api, tools := newFake(t, f)
	if _, err := tools["list_displays"].Handler(nil); err == nil || !strings.Contains(err.Error(), "abgelehnt") {
		t.Fatalf("denied: %v", err)
	}
	// Granted later; then the user revokes the access: the stored token is dropped
	// and the server asks again.
	f.approve = "approved"
	if _, err := tools["list_displays"].Handler(nil); err != nil {
		t.Fatal(err)
	}
	f.token = "tok2" // CaseHUD forgot the old token
	f.approve = "pending"
	if _, err := tools["list_displays"].Handler(nil); err == nil || !strings.Contains(err.Error(), "Bitte in CaseHUD") {
		t.Fatalf("after revoke: %v", err)
	}
	if _, err := os.Stat(api.TokenFile); err == nil {
		t.Fatal("a revoked token stays on disk")
	}
	if f.pairs != 3 {
		t.Fatalf("%d pair requests", f.pairs)
	}
}

func TestMCPSwitchOffAndReadOnlyAreExplained(t *testing.T) {
	cases := map[string]string{
		"mcp_disabled":       "turned off in CaseHUD",
		"insufficient_scope": "Einstellungen > Zugriffe",
	}
	for code, want := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			io.WriteString(w, `{"error":{"code":"`+code+`","message":"x"}}`)
		}))
		api, _ := NewAPI(srv.URL)
		api.token, api.loaded = "t", true
		_, err := api.call("GET", "/api/v1/status", nil, nil)
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", code, err)
		}
	}
}

func TestClientNameFromInitialize(t *testing.T) {
	var got string
	s := &Server{Name: "t", Version: "1", OnClient: func(n string) { got = n }}
	var out bytes.Buffer
	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"some-assistant","version":"1"}}}`
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if got != "some-assistant" {
		t.Fatalf("client %q", got)
	}
	if (&API{}).pairName() != "MCP" {
		t.Fatal("name without a client")
	}
}

func TestTokenFileFor(t *testing.T) {
	if n, ok := TokenFileFor("claude-desktop"); !ok || n != "mcp-token-claude-desktop.json" {
		t.Fatal(n, ok)
	}
	for _, bad := range []string{"", "..", `a\b`, "a/b", "A", "a.b", strings.Repeat("a", 33)} {
		if _, ok := TokenFileFor(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
