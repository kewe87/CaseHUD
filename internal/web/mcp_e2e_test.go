package web

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/mcp"
)

// The MCP server pairs like any other program and then does everything through
// /api/v1/ with its own token.
func TestMCPToolsThroughV1(t *testing.T) {
	s, ts := testServer(t)
	setMCP(t, s, true)
	logs := capturedLog(t)

	api, err := mcp.NewAPI(ts.URL + "/?t=secret")
	if err != nil {
		t.Fatal(err)
	}
	api.TokenFile = filepath.Join(t.TempDir(), mcp.TokenFileName)
	api.PollEvery = 20 * time.Millisecond
	api.SetClient("test-assistant")
	tools := map[string]mcp.Tool{}
	for _, tool := range mcp.NewCaseHUD(api, "t").Tools {
		tools[tool.Name] = tool
	}

	// The user answers in the window a moment later.
	answered := make(chan struct{})
	go func() {
		defer close(answered)
		for range 200 {
			if p := s.acc().Pending(); len(p) == 1 {
				if p[0].Name != "MCP (test-assistant)" || p[0].Interface != "mcp" || p[0].Scope != "control" {
					t.Errorf("request %+v", p[0])
				}
				decide(t, ts, p[0].ID, true, "control", true)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Error("no access request arrived")
	}()

	call := func(name, args string) mcp.ToolResult {
		t.Helper()
		res, err := tools[name].Handler(json.RawMessage(args))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return res
	}
	text := func(r mcp.ToolResult) string { return r.Content[0].Text }

	res := call("list_displays", `{}`)
	<-answered
	panel := s.Engine.Panels()[0]
	if !strings.Contains(text(res), panel.ID) {
		t.Fatalf("list_displays: %s", text(res))
	}
	if sens := text(call("list_sensors", `{"filter":"temperature"}`)); !strings.Contains(sens, "Temperatures") {
		t.Fatalf("list_sensors: %s", sens)
	}
	designs := text(call("list_designs", `{}`))
	var ds []struct{ ID string }
	if err := json.Unmarshal([]byte(designs), &ds); err != nil || len(ds) == 0 {
		t.Fatalf("list_designs: %v %s", err, designs)
	}
	design := text(call("get_design", `{"id":"`+ds[0].ID+`"}`))
	if !strings.Contains(design, `"widgets"`) {
		t.Fatalf("get_design: %.200s", design)
	}
	args, _ := json.Marshal(map[string]any{"design": json.RawMessage(design), "theme": "light"})
	img := call("render_design", string(args))
	if len(img.Content) != 1 || img.Content[0].Type != "image" || img.Content[0].MimeType != "image/png" || len(img.Content[0].Data) < 100 {
		t.Fatalf("render_design: %+v", img)
	}
	args, _ = json.Marshal(map[string]any{"id": "mcp-test", "design": json.RawMessage(design)})
	if r := text(call("save_design", string(args))); r != "saved as mcp-test" {
		t.Fatalf("save_design: %s", r)
	}
	if r := text(call("apply_design", `{"display_id":"`+panel.ID+`","design_id":"mcp-test","theme":"dark"}`)); !strings.Contains(r, "now shows mcp-test") {
		t.Fatalf("apply_design: %s", r)
	}
	if _, err := tools["apply_design"].Handler(json.RawMessage(`{"display_id":"nope","design_id":"mcp-test"}`)); err == nil || !strings.Contains(err.Error(), `no display with id "nope"`) {
		t.Fatalf("unknown display: %v", err)
	}
	if got := s.Engine.Panels()[0].Config.Design; got != "mcp-test" {
		t.Fatalf("the display shows %q", got)
	}

	// Every change was logged with the name, none with the token.
	l := logs.String()
	for _, want := range []string{`PUT /api/v1/designs/mcp-test by "MCP (test-assistant)" (paired)`, `PATCH /api/v1/panels/` + panel.ID + ` by "MCP (test-assistant)" (paired)`} {
		if !strings.Contains(l, want) {
			t.Errorf("log lacks %q", want)
		}
	}

	// The switch is the server's: off, every tool is refused with a clear text.
	setMCP(t, s, false)
	if _, err := tools["list_sensors"].Handler(json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "turned off in CaseHUD") {
		t.Fatalf("with the switch off: %v", err)
	}
	// Back on: the stored token still works without a new request.
	setMCP(t, s, true)
	call("list_sensors", `{}`)
	if len(s.acc().Pending()) != 0 {
		t.Fatal("a second access request was made")
	}
}

// A read-only MCP connection can look but not change anything.
func TestMCPReadOnly(t *testing.T) {
	s, ts := testServer(t)
	setMCP(t, s, true)
	api, _ := mcp.NewAPI(ts.URL)
	api.PollEvery = 20 * time.Millisecond
	tools := map[string]mcp.Tool{}
	for _, tool := range mcp.NewCaseHUD(api, "t").Tools {
		tools[tool.Name] = tool
	}
	go func() {
		for range 200 {
			if p := s.acc().Pending(); len(p) == 1 {
				decide(t, ts, p[0].ID, true, "read", false)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	if _, err := tools["list_sensors"].Handler(json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tools["render_design"].Handler(json.RawMessage(`{"design":{"width":100,"height":100,"widgets":[]}}`)); err != nil {
		t.Fatalf("render needs read only: %v", err)
	}
	_, err := tools["save_design"].Handler(json.RawMessage(`{"id":"x","design":{"width":100,"height":100,"widgets":[]}}`))
	if err == nil || !strings.Contains(err.Error(), "Einstellungen > Zugriffe") {
		t.Fatalf("save with read access: %v", err)
	}
}
