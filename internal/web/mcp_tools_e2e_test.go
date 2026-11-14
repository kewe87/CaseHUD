package web

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/mcp"
)

// pairedMCP pairs an MCP connection with the scope the user grants and returns
// its tools and a caller.
func pairedMCP(t *testing.T, s *Server, ts *httptest.Server, scope string, system bool) (map[string]mcp.Tool, func(name, args string) (string, error)) {
	t.Helper()
	setMCP(t, s, true)
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
	go func() {
		for range 300 {
			if p := s.acc().Pending(); len(p) == 1 {
				if p[0].System {
					t.Errorf("the MCP server must not ask for the system right: %+v", p[0])
				}
				b, _ := json.Marshal(map[string]any{"allow": true, "scope": scope, "system": system, "remember": false})
				gui(t, ts, "POST", "/api/access/requests/"+p[0].ID, string(b), nil)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	call := func(name, args string) (string, error) {
		t.Helper()
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("no tool %s", name)
		}
		res, err := tool.Handler(json.RawMessage(args))
		if err != nil {
			return "", err
		}
		if len(res.Content) == 0 {
			return "", nil
		}
		return res.Content[0].Text, nil
	}
	return tools, call
}

func TestMCPCuratedTools(t *testing.T) {
	s, ts := testServer(t)
	_, call := pairedMCP(t, s, ts, "control", false)
	configureCooling(t, ts, demoCoolingAt("30"))
	ok := func(name, args string) string {
		t.Helper()
		out, err := call(name, args)
		if err != nil {
			t.Fatalf("%s %s: %v", name, args, err)
		}
		return out
	}
	fail := func(name, args, want string) {
		t.Helper()
		_, err := call(name, args)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s %s: %v, want %q", name, args, err, want)
		}
	}

	// status: one call
	var st map[string]any
	if err := json.Unmarshal([]byte(ok("status", `{}`)), &st); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"areas", "displays", "cooling", "lighting", "version", "scene", "power"} {
		if _, has := st[k]; !has {
			t.Errorf("status lacks %s: %v", k, st)
		}
	}
	if d, _ := st["displays"].([]any); len(d) == 0 {
		t.Errorf("no displays: %v", st["displays"])
	}
	cool, _ := st["cooling"].(map[string]any)
	if outs, _ := cool["outputs"].([]any); len(outs) != 2 {
		t.Errorf("cooling summary: %v", cool)
	}

	// cooling
	if out := ok("cooling_status", `{}`); !strings.Contains(out, "port1-fan1") {
		t.Fatalf("cooling_status: %s", out)
	}
	cfg := ok("cooling_get_config", `{}`)
	var doc struct {
		Revision int64 `json:"revision"`
	}
	if err := json.Unmarshal([]byte(cfg), &doc); err != nil || doc.Revision == 0 {
		t.Fatalf("cooling_get_config: %v %s", err, cfg)
	}
	rev := func() string { return `,"revision":` + itoa(doc.Revision) }
	// a manual value on a fan, then back on the curve
	if out := ok("cooling_set_output", `{"output_id":"port1-fan1","manual":{"value":55}`+rev()+`}`); !strings.Contains(out, "revision") {
		t.Fatalf("set manual: %s", out)
	}
	if got := getCooling(t, ts).Outputs[0]; got.Mode != "manual" || got.Manual == nil || got.Manual.Value != 55 {
		t.Fatalf("%+v", got)
	}
	fail("cooling_set_output", `{"output_id":"port1-fan1","curve_id":"c"`+rev()+`}`, "revision_mismatch") // the revision moved on
	ok("cooling_set_output", `{"output_id":"port1-fan1","curve_id":"c"}`)
	if got := getCooling(t, ts).Outputs[0]; got.Mode != "curve" {
		t.Fatalf("%+v", got)
	}
	// the pump is never below 30 %: the server says no
	fail("cooling_set_output", `{"output_id":"pump","manual":{"value":10}}`, "invalid_value")
	fail("cooling_set_output", `{"output_id":"pump","manual":{"value":10,"unit":"percent"}}`, "30")
	fail("cooling_set_output", `{"output_id":"nope","manual":{"value":50}}`, "not_found")
	fail("cooling_set_output", `{"output_id":"port1-fan1","curve_id":"nope"}`, "not_found")
	// curves
	curve := `{"id":"quiet","name":"Quiet","sources":["Board/Temperatures/Coolant"],"points":[[20,30],[40,60]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}`
	if out := ok("cooling_set_curve", `{"curve":`+curve+`}`); !strings.Contains(out, "created") {
		t.Fatalf("set curve: %s", out)
	}
	if out := ok("cooling_set_curve", `{"curve":`+curve+`}`); !strings.Contains(out, "replaced") {
		t.Fatalf("replace curve: %s", out)
	}
	fail("cooling_set_curve", `{"curve":{"id":"bad","points":[[20,30],[10,60]]}}`, "invalid_value")
	ok("cooling_set_output", `{"output_id":"port1-fan1","curve_id":"quiet"}`)
	fail("cooling_delete_curve", `{"curve_id":"quiet"}`, "curve_in_use")
	ok("cooling_set_output", `{"output_id":"port1-fan1","curve_id":"c"}`)
	ok("cooling_delete_curve", `{"curve_id":"quiet"}`)
	fail("cooling_delete_curve", `{"curve_id":"quiet"}`, "not_found")
	// presets
	saved := ok("cooling_preset_save", `{"name":"Evening"}`)
	var pr struct{ ID string }
	if err := json.Unmarshal([]byte(saved), &pr); err != nil || pr.ID == "" {
		t.Fatalf("preset save: %v %s", err, saved)
	}
	if out := ok("cooling_presets", `{}`); !strings.Contains(out, pr.ID) {
		t.Fatalf("presets: %s", out)
	}
	if out := ok("cooling_preset_activate", `{"preset_id":"`+pr.ID+`"}`); !strings.Contains(out, "port1-fan1") {
		t.Fatalf("activate: %s", out)
	}
	fail("cooling_preset_activate", `{"preset_id":"nope"}`, "not_found")
	// calibration: no request without confirm, and the server's refusal otherwise
	fail("cooling_calibrate", `{"target":"all","confirm":false}`, "confirm must be true")
	if _, err := call("cooling_calibrate", `{"target":"one","output_id":"port1-fan1","confirm":true}`); err == nil || strings.Contains(err.Error(), "confirm must") {
		t.Fatalf("the demo has no hardware to measure: %v", err)
	}
	if out := ok("cooling_calibrate_status", `{}`); !strings.Contains(out, `"running": false`) {
		t.Fatalf("status: %s", out)
	}
	ok("cooling_calibrate_stop", `{}`)

	// lighting
	if out := ok("lighting_status", `{}`); !strings.Contains(out, "effects") {
		t.Fatalf("lighting_status: %s", out)
	}
	ok("lighting_set", `{"brightness":40}`)
	if out := ok("lighting_status", `{}`); !strings.Contains(out, `"brightness": 40`) {
		t.Fatalf("brightness not set: %.300s", out)
	}
	fail("lighting_set", `{"brightness":400}`, "brightness")
	fail("lighting_set", `{"effect":"does-not-exist"}`, "bad_request")
	ok("lighting_presets", `{}`)
	fail("lighting_preset_apply", `{"preset_id":"does-not-exist"}`, "not_found")

	// scenes and displays
	if out := ok("list_scenes", `{}`); strings.TrimSpace(out) != "[]" {
		t.Fatalf("scenes: %s", out)
	}
	if err := s.Engine.SaveScene("Evening look"); err != nil {
		t.Fatal(err)
	}
	ok("apply_scene", `{"name":"Evening look"}`)
	fail("apply_scene", `{"name":"Nope"}`, "not_found")
	panel := s.Engine.Panels()[0]
	if out := ok("set_display", `{"display_id":"`+panel.ID+`","brightness":42,"theme":"light"}`); !strings.Contains(out, `"brightness": 42`) {
		t.Fatalf("set_display: %s", out)
	}
	fail("set_display", `{"display_id":"`+panel.ID+`","brightness":420}`, "bad_request")
	fail("set_display", `{"display_id":"nope","brightness":42}`, `no display with id "nope"`)
}

func TestMCPApiCall(t *testing.T) {
	s, ts := testServer(t)
	tools, call := pairedMCP(t, s, ts, "control", false)
	ok := func(args string) string {
		t.Helper()
		out, err := call("api_call", args)
		if err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		return out
	}
	// the list, one route in detail, then use it
	if list, err := call("api_spec", `{}`); err != nil || !strings.Contains(list, "/api/v1/rotation") || !strings.Contains(list, "[control]") || !strings.Contains(list, "/api/v1/settings") {
		t.Fatalf("api_spec: %v %.300s", err, list)
	}
	if detail, err := call("api_spec", `{"path":"/api/v1/settings","method":"PATCH"}`); err != nil || !strings.Contains(detail, "fahrenheit") || strings.Contains(detail, `"$ref"`) {
		t.Fatalf("api_spec detail: %v %.600s", err, detail)
	}
	if detail, err := call("api_spec", `{"path":"layouts/{id}"}`); err != nil || !strings.Contains(detail, "PUT") || !strings.Contains(detail, "DELETE") {
		t.Fatalf("api_spec short path: %v %.300s", err, detail)
	}
	if out := ok(`{"method":"GET","path":"/api/v1/rotation"}`); !strings.Contains(out, `"intervalSeconds"`) {
		t.Fatalf("rotation: %s", out)
	}
	if out := ok(`{"method":"GET","path":"/api/v1/token"}`); !strings.Contains(out, `"system": false`) || !strings.Contains(out, `"scope": "control"`) {
		t.Fatalf("token: %s", out)
	}
	if out := ok(`{"method":"POST","path":"/api/v1/scenes","body":{"name":"Via call"}}`); !strings.Contains(out, "Via call") {
		t.Fatalf("scene: %s", out)
	}
	if out := ok(`{"method":"PATCH","path":"/api/v1/scenes/Via%20call","body":{"name":"Renamed"}}`); !strings.Contains(out, "Renamed") {
		t.Fatalf("rename: %s", out)
	}
	if out := ok(`{"method":"PATCH","path":"/api/v1/settings","body":{"fahrenheit":true}}`); !strings.Contains(out, `"fahrenheit": true`) {
		t.Fatalf("settings: %s", out)
	}
	if out := ok(`{"method":"POST","path":"/api/v1/rotation/next"}`); !strings.HasPrefix(out, "OK (HTTP 204)") {
		t.Fatalf("204: %s", out)
	}
	// an image comes back as an image
	img, err := tools["api_call"].Handler(json.RawMessage(`{"method":"GET","path":"/api/v1/designs/` + s.Engine.Store.Designs()[0].ID + `/preview.png?size=64&theme=dark"}`))
	if err != nil || len(img.Content) != 1 || img.Content[0].Type != "image" || img.Content[0].MimeType != "image/png" {
		t.Fatalf("image: %+v %v", img, err)
	}

	// the server's refusals reach the model as they are
	for args, want := range map[string]string{
		`{"method":"PATCH","path":"/api/v1/settings","body":{"api":false}}`:      "bad_request",
		`{"method":"PATCH","path":"/api/v1/settings","body":{"mcp":false}}`:      "bad_request",
		`{"method":"DELETE","path":"/api/v1/designs/nope"}`:                      "not_found",
		`{"method":"GET","path":"/api/v1/nothing-here"}`:                         "not_found",
		`{"method":"POST","path":"/api/v1/cooling/measure","body":{"all":true}}`: "confirmation_required",
	} {
		if _, err := call("api_call", args); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", args, err, want)
		}
	}
	// Never through the tool: quit, update, window, pairing, streams, other paths and hosts.
	for _, args := range []string{
		`{"method":"POST","path":"/api/v1/quit"}`, `{"method":"POST","path":"/api/v1/update/install"}`, `{"method":"POST","path":"/api/v1/update/check"}`,
		`{"method":"POST","path":"/api/v1/gui/open"}`, `{"method":"DELETE","path":"/api/v1/token"}`, `{"method":"POST","path":"/api/v1/pair","body":{"name":"x"}}`,
		`{"method":"GET","path":"/api/v1/events"}`,
		`{"method":"GET","path":"/api/state"}`, `{"method":"POST","path":"/api/quit"}`, `{"method":"PUT","path":"/api/settings/advanced","body":{"api":false}}`,
		`{"method":"GET","path":"/api/v1/../state"}`, `{"method":"GET","path":"/api/v1/%2e%2e/state"}`, `{"method":"GET","path":"/api/v1/..%2f..%2fapi/state"}`,
		`{"method":"GET","path":"http://127.0.0.1:1/api/v1/status"}`, `{"method":"GET","path":"//example.org/api/v1/status"}`,
		`{"method":"GET","path":"/api/v1/status?x=1#frag"}`, `{"method":"GET","path":"/api/v1/status\r\nX-Token: secret"}`,
	} {
		if _, err := call("api_call", args); err == nil {
			t.Errorf("api_call %s was allowed", args)
		}
	}
	if s.ending.Load() {
		t.Fatal("CaseHUD was told to quit")
	}
	if !s.Engine.APIEnabled() || !s.Engine.MCPEnabled() {
		t.Fatal("an interface switch was changed through MCP")
	}
}

// What the user has not granted is refused by CaseHUD, and the tool says where
// to grant it.
func TestMCPWithoutControl(t *testing.T) {
	s, ts := testServer(t)
	_, call := pairedMCP(t, s, ts, "read", false)
	for _, name := range []string{"status", "cooling_status", "cooling_get_config", "cooling_presets", "lighting_status", "lighting_presets", "list_scenes", "api_spec", "cooling_calibrate_status"} {
		if _, err := call(name, `{}`); err != nil {
			t.Errorf("%s with read: %v", name, err)
		}
	}
	for name, args := range map[string]string{
		"cooling_set_output":      `{"output_id":"port1-fan1","manual":{"value":50}}`,
		"cooling_set_curve":       `{"curve":{"id":"x","points":[[20,30],[40,60]]}}`,
		"cooling_delete_curve":    `{"curve_id":"x"}`,
		"cooling_preset_activate": `{"preset_id":"x"}`,
		"cooling_preset_save":     `{"name":"x"}`,
		"cooling_calibrate":       `{"target":"all","confirm":true}`,
		"cooling_calibrate_stop":  `{}`,
		"lighting_set":            `{"brightness":10}`,
		"lighting_preset_apply":   `{"preset_id":"x"}`,
		"apply_scene":             `{"name":"x"}`,
		"set_display":             `{"display_id":"x","brightness":10}`,
		"api_call":                `{"method":"PATCH","path":"/api/v1/settings","body":{"fahrenheit":true}}`,
	} {
		_, err := call(name, args)
		if err == nil || !strings.Contains(err.Error(), "insufficient_scope") || !strings.Contains(err.Error(), "Einstellungen > Zugriffe") || !strings.Contains(err.Error(), "Settings > Access") {
			t.Errorf("%s with read: %v", name, err)
		}
	}
	if s.Engine.Settings().Fahrenheit {
		t.Fatal("a read-only connection changed a setting")
	}
	// Even with the system right on the connection the tool does not reach it.
	s2, ts2 := testServer(t)
	_, call2 := pairedMCP(t, s2, ts2, "control", true)
	if _, err := call2("api_call", `{"method":"POST","path":"/api/v1/quit"}`); err == nil || !strings.Contains(err.Error(), "not available through MCP") {
		t.Fatalf("quit with system granted: %v", err)
	}
	if s2.ending.Load() {
		t.Fatal("quit happened")
	}
}
