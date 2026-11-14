package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCheckAPICall(t *testing.T) {
	good := map[string]string{
		"/api/v1/status": "/api/v1/status",
		"/api/v1/designs/abc/preview.png?size=200": "/api/v1/designs/abc/preview.png?size=200",
		"/api/v1/scenes/My%20Scene/activate":       "/api/v1/scenes/My%20Scene/activate",
		"/api/v1/cooling/measure?keep=finished":    "/api/v1/cooling/measure?keep=finished",
		"/api/v1/token":                            "/api/v1/token",
	}
	for in, want := range good {
		m, p, err := CheckAPICall("get", in)
		if err != nil || p != want || m != "GET" {
			t.Errorf("%q: %q %q %v", in, m, p, err)
		}
	}
	// The query is rebuilt, not passed through.
	_, p, err := CheckAPICall("GET", "/api/v1/x?b=2&a=1")
	if err != nil || p != "/api/v1/x?a=1&b=2" {
		t.Errorf("query: %q %v", p, err)
	}
	bad := []string{
		"", "status", "/api/v1", "/api/v2/status", "/api/status", "/api/", "/",
		"http://127.0.0.1:1/api/v1/status", "//evil.example/api/v1/status", "https://evil.example/api/v1/status",
		"/api/v1/../status", "/api/v1/%2e%2e/status", "/api/v1/..%2fstatus", "/api/v1/%2E%2E%2Fquit", "/api/v1/a/./b", "/api/v1/a/%2e/b",
		"/api/v1/..\\status", "/api/v1/a\\b", "/api/v1/%5c..%5cx", "/api/v1/status#frag", "/api/v1/status\n", "/api/v1/sta tus", "/api/v1/st\x00atus",
		"/api/v1/status?a=1#x", "/api/v1/status?a b=1", "/api/v1/status?a%0d%0ab=1", "/api/v1/status?=x", "/api/v1/status?a=%00", "/api/v1/status?" + "a=" + strings.Repeat("x", 300),
		"/api/v1/%zz", "/api/v1/status?%zz=1",
		"/api/v1/quit", "/api/v1/quit/", "/api/v1/%71uit", "/api/v1/update/install", "/api/v1/update/check", "/api/v1/gui/open",
		"/api/v1/events", "/api/v1/pair", "/api/v1/pair/abc", "/api/v1/pair/",
		"/api/v1/" + strings.Repeat("a", 2100),
	}
	for _, in := range bad {
		if _, p, err := CheckAPICall("GET", in); err == nil {
			t.Errorf("%q was accepted as %q", in, p)
		}
	}
	for _, m := range []string{"", "TRACE", "CONNECT", "OPTIONS", "HEAD", "GET /x", "G ET"} {
		if _, _, err := CheckAPICall(m, "/api/v1/status"); err == nil {
			t.Errorf("method %q accepted", m)
		}
	}
	// Giving the token up is not for a tool call; reading it is.
	if _, _, err := CheckAPICall("DELETE", "/api/v1/token"); err == nil {
		t.Error("DELETE /token accepted")
	}
	if _, _, err := CheckAPICall("POST", "/api/v1/quit"); err == nil {
		t.Error("POST /quit accepted")
	}
}

func TestToolNames(t *testing.T) {
	f := &fakeCaseHUD{token: "tok", approve: "approved", status: statusWith("true")}
	_, tools := newFake(t, f)
	for _, name := range []string{
		"design_guide", "list_displays", "list_sensors", "list_designs", "get_design", "render_design", "save_design", "apply_design",
		"status", "cooling_status", "cooling_get_config", "cooling_set_output", "cooling_set_curve", "cooling_delete_curve", "cooling_presets",
		"cooling_preset_activate", "cooling_preset_save", "cooling_calibrate", "cooling_calibrate_status", "cooling_calibrate_stop",
		"lighting_status", "lighting_set", "lighting_presets", "lighting_preset_apply", "list_scenes", "apply_scene", "set_display",
		"api_spec", "api_call",
	} {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %s is missing", name)
			continue
		}
		if len(tool.Description) < 40 || tool.InputSchema == nil || tool.Handler == nil {
			t.Errorf("tool %s is not complete", name)
		}
	}
	// No tool can quit or update CaseHUD.
	for name := range tools {
		if strings.Contains(name, "quit") || strings.Contains(name, "update") || strings.Contains(name, "install") {
			t.Errorf("tool %s", name)
		}
	}
}

// What a model sends wrongly is refused before any request is made.
func TestCuratedToolsRefuseBadArguments(t *testing.T) {
	f := &fakeCaseHUD{token: "tok", approve: "approved", status: statusWith("true")}
	_, tools := newFake(t, f)
	for name, args := range map[string]string{
		"cooling_set_output":        `{}`,
		"cooling_set_output ":       `{"output_id":"a"}`,
		"cooling_set_output  ":      `{"output_id":"a","curve_id":"c","manual":{"value":50}}`,
		"cooling_set_output   ":     `{"output_id":"a","curve_id":""}`,
		"cooling_set_output    ":    `{"output_id":"a","manual":{"value":0}}`,
		"cooling_set_output     ":   `{"output_id":"a","manual":{"value":150}}`,
		"cooling_set_output      ":  `{"output_id":"a","manual":{"value":50,"unit":"kelvin"}}`,
		"cooling_set_output       ": `{"output_id":"a","manual":{"unit":"percent"}}`,
		"cooling_calibrate":         `{"target":"all"}`,
		"cooling_calibrate ":        `{"target":"all","confirm":false}`,
		"cooling_calibrate  ":       `{"target":"one","confirm":true}`,
		"cooling_calibrate   ":      `{"target":"pump","confirm":true}`,
		"cooling_calibrate    ":     `{"target":"everything","confirm":true}`,
		"cooling_set_curve":         `{"curve":{"name":"x"}}`,
		"cooling_delete_curve":      `{}`,
		"lighting_set":              `{}`,
		"lighting_set ":             `{"brightness":101}`,
		"set_display":               `{"display_id":"p1"}`,
		"apply_scene":               `{}`,
		"api_call":                  `{"method":"GET","path":"/api/v1/../x"}`,
		"api_call ":                 `{"method":"GET","path":"/api/v1/status","body":{"a":1}}`,
	} {
		name = strings.TrimSpace(name)
		before := len(f.requests)
		_, err := tools[name].Handler(json.RawMessage(args))
		if err == nil {
			t.Errorf("%s %s was accepted", name, args)
		}
		if len(f.requests) != before {
			t.Errorf("%s %s reached CaseHUD: %v", name, args, f.requests[before:])
		}
	}
	// A misspelled field is an invalid-params error, not silently ignored.
	_, err := tools["cooling_set_output"].Handler(json.RawMessage(`{"output_id":"a","curve":"c"}`))
	var bad invalidParamsError
	if err == nil || !asInvalid(err, &bad) {
		t.Errorf("unknown field: %v", err)
	}
}

func asInvalid(err error, target *invalidParamsError) bool { return errors.As(err, target) }

func TestSpecViewIsCompactAndResolvesSchemas(t *testing.T) {
	doc := map[string]any{
		"info": map[string]any{"version": "1 (test)"},
		"paths": map[string]any{
			"/api/v1/status": map[string]any{"get": map[string]any{"summary": "State", "x-scope": "read", "security": []any{}}},
			"/api/v1/x/{id}": map[string]any{
				"put": map[string]any{"summary": "Put x", "x-scope": "control", "requestBody": map[string]any{
					"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/X"}}}}},
				"get": map[string]any{"summary": "Get x", "x-scope": "read"},
			},
		},
		"components": map[string]any{"schemas": map[string]any{
			"X":    map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "next": map[string]any{"$ref": "#/components/schemas/X"}}},
			"Skip": map[string]any{},
		}},
	}
	list := specView(doc, "", "")
	if !strings.Contains(list, "GET    /api/v1/status  [read]  State") || !strings.Contains(list, "PUT    /api/v1/x/{id}  [control]") {
		t.Fatalf("list:\n%s", list)
	}
	detail := specView(doc, "/x/{id}", "put")
	if !strings.Contains(detail, `"name"`) || !strings.Contains(detail, "PUT") || strings.Contains(detail, "GET") || strings.Contains(detail, "$ref\": \"#") {
		t.Fatalf("detail:\n%s", detail)
	}
	if !strings.Contains(specView(doc, "/api/v1/nope", ""), "No such path") {
		t.Fatal("unknown path")
	}
	if len(detail) > 20000 {
		t.Fatalf("a recursive type blew up: %d bytes", len(detail))
	}
}

func TestInstructionsNameAllAreasAndTheRules(t *testing.T) {
	for _, want := range []string{"cooling", "lighting", "displays", "30 %", "api_call", "api_spec", "insufficient_scope", "Zugriffe", "Access", "cooling_calibrate", "confirm"} {
		if want == "Zugriffe" {
			continue // the German name is in the error texts
		}
		if !strings.Contains(instructions, want) && !strings.Contains(strings.ToLower(instructions), strings.ToLower(want)) {
			t.Errorf("instructions do not mention %q", want)
		}
	}
	for _, msg := range []string{msgNoControl, msgNoSystem} {
		if !strings.Contains(msg, "Einstellungen > Zugriffe") || !strings.Contains(msg, "Settings > Access") {
			t.Errorf("%s", msg)
		}
	}
	if !strings.Contains(instructions, "Start with status") {
		t.Error("status is not named")
	}
}
