package web

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

func TestV1ReadRotationAutomationAwtrix(t *testing.T) {
	s, ts := testServer(t)
	var rot v1Rotation
	wantStatus(t, v1(t, ts, "GET", "/api/v1/rotation", "", &rot), 200)
	if rot.Enabled || rot.Conveyor == nil || rot.Order == nil {
		t.Fatalf("%+v", rot)
	}
	if err := s.Engine.SaveScene("Alpha"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/rotation", `{"enabled":true}`, nil), 200)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/rotation", "", &rot), 200)
	if !rot.Enabled || rot.Mode == "" {
		t.Fatalf("%+v", rot)
	}
	var auto v1Automation
	wantStatus(t, v1(t, ts, "GET", "/api/v1/automation", "", &auto), 200)
	if auto.Enabled || auto.Rules == nil || auto.IgnoreGames == nil || auto.EnterSeconds <= 0 || auto.LeaveSeconds <= 0 {
		t.Fatalf("%+v", auto)
	}
	ac := s.Engine.Automation()
	ac.Rules = []app.AutomationRule{{When: "game", Scene: "Alpha"}}
	if err := s.Engine.SetAutomation(ac); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/automation", "", &auto), 200)
	if len(auto.Rules) != 1 || auto.Rules[0].Scene != "Alpha" || auto.Rules[0].When != "game" {
		t.Fatalf("%+v", auto)
	}
	var aw v1Awtrix
	wantStatus(t, v1(t, ts, "GET", "/api/v1/awtrix", "", &aw), 200)
	if aw.Enabled || aw.Clocks == nil || !slices.Contains(aw.Apps, "cpu") {
		t.Fatalf("%+v", aw)
	}
	if err := s.Engine.SetAwtrixDevices([]app.AwtrixDevice{{ID: "a", Name: "Desk", Host: "192.168.1.50", Apps: []string{"cpu"}, Disabled: true}}); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/awtrix", "", &aw), 200)
	if len(aw.Clocks) != 1 || aw.Clocks[0].Name != "Desk" || aw.Clocks[0].Host != "192.168.1.50" || !aw.Clocks[0].Disabled || aw.Enabled {
		t.Fatalf("%+v", aw)
	}
}

func TestV1DeleteDesign(t *testing.T) {
	s, ts := testServer(t)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/designs/nope", "", nil), 404, "not_found")
	wantError(t, v1(t, ts, "DELETE", "/api/v1/designs/..%2Fx", "", nil), 404, "not_found")
	var ds []v1Design
	v1(t, ts, "GET", "/api/v1/designs", "", &ds)
	var builtin string
	for _, d := range ds {
		if d.Builtin && builtin == "" {
			builtin = d.ID
		}
	}
	wantError(t, v1(t, ts, "DELETE", "/api/v1/designs/"+builtin, "", nil), 409, "builtin")
	var l json.RawMessage
	v1(t, ts, "GET", "/api/v1/designs/"+builtin, "", &l)
	layout := string(l)
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/designs/mine", layout, nil), 204)
	// In use by a display: refused; after another design is on it: deleted.
	p := s.Engine.Panels()[0]
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+p.ID, `{"design":"mine"}`, nil), 200)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/designs/mine", "", nil), 409, "in_use")
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+p.ID, `{"design":"`+builtin+`"}`, nil), 200)
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/designs/mine", "", nil), 204)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/designs/mine", "", nil), 404, "not_found")
	// A copy over a built-in design may go; the original stays.
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/designs/"+builtin, layout, nil), 204)
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/designs/"+builtin, "", nil), 204)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/designs/"+builtin, "", nil), 200)
}

func TestV1LayoutsWriteAndDelete(t *testing.T) {
	t.Setenv("CASEHUD_DEMO_BAR", "1")
	s, ts := testServer(t)
	var ls []v1Layout
	v1(t, ts, "GET", "/api/v1/layouts", "", &ls)
	var ready string
	for _, l := range ls {
		if l.Builtin {
			ready = l.ID
			break
		}
	}
	var one v1Layout
	wantStatus(t, v1(t, ts, "GET", "/api/v1/layouts/"+ready, "", &one), 200)
	if one.ID != ready || !one.Builtin || len(one.Tiles) == 0 {
		t.Fatalf("%+v", one)
	}
	wantError(t, v1(t, ts, "GET", "/api/v1/layouts/nope", "", nil), 404, "not_found")
	body := `{"name":"Mine","grid":"4","tiles":[{"mode":"hud","design":"gaming"},{"design":"gaming"},{"design":"gaming"},{"design":"gaming"}]}`
	wantError(t, v1(t, ts, "PUT", "/api/v1/layouts/"+ready, body, nil), 409, "builtin")
	wantError(t, v1(t, ts, "DELETE", "/api/v1/layouts/"+ready, "", nil), 409, "builtin")
	wantError(t, v1(t, ts, "PUT", "/api/v1/layouts/Big", body, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PUT", "/api/v1/layouts/..%2Fx", body, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PUT", "/api/v1/layouts/mine", `{"name":"","grid":"4","tiles":[]}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PUT", "/api/v1/layouts/mine", `{"name":"x","extra":1}`, nil), 400, "bad_request")
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/layouts/mine", body, nil), 204)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/layouts/mine", "", &one), 200)
	if one.Name != "Mine" || one.Builtin || one.Grid != "4" || len(one.Tiles) != 4 || one.Tiles[0].Design != "gaming" {
		t.Fatalf("%+v", one)
	}
	// Displays that use it follow a change, and keep their fields after a delete.
	p := app.PanelState{ID: "us88-0"}
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+p.ID, `{"layout":"mine"}`, nil), 200)
	if c := panelConfig(s, p.ID); c.Layout != "mine" || c.Tiles[0].Design != "gaming" {
		t.Fatalf("%+v", c)
	}
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/layouts/mine", strings.Replace(body, "gaming", "clock", 1), nil), 204)
	if c := panelConfig(s, p.ID); c.Tiles[0].Design != "clock" || c.Layout != "mine" {
		t.Fatalf("the display did not follow the layout: %+v", c)
	}
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/layouts/mine", "", nil), 204)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/layouts/mine", "", nil), 404, "not_found")
	var pv []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &pv)
	for _, x := range pv {
		if x.ID == p.ID && x.Layout != "" {
			t.Fatalf("the display still points to the deleted layout: %+v", x)
		}
	}
}

func panelConfig(s *Server, id string) app.PanelConfig {
	for _, p := range s.Engine.Panels() {
		if p.ID == id {
			return p.Config
		}
	}
	return app.PanelConfig{}
}

func TestV1SaveAndRenameScene(t *testing.T) {
	s, ts := testServer(t)
	var sc v1Scene
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"  "}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"`+strings.Repeat("x", 65)+`"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"a","bogus":1}`, nil), 400, "bad_request")
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":" Evening\u0007 "}`, &sc), 201)
	if sc.ID != "Evening" || sc.Name != "Evening" {
		t.Fatalf("%+v", sc)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"Evening"}`, nil), 409, "exists")
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"Evening","overwrite":true,"noRotation":true}`, nil), 201)
	if rc := s.Engine.Rotation(); len(rc.Scenes) != 1 || !rc.Scenes[0].NoRotation {
		t.Fatalf("%+v", rc.Scenes)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes", `{"name":"Other"}`, nil), 201)
	// An automation rule follows a rename.
	ac := s.Engine.Automation()
	ac.Rules = []app.AutomationRule{{When: "game", Scene: "Evening"}}
	if err := s.Engine.SetAutomation(ac); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/scenes/Nope", `{"name":"x"}`, nil), 404, "not_found")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/scenes/Evening", `{"name":"Other"}`, nil), 409, "exists")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/scenes/Evening", `{"name":""}`, nil), 400, "bad_request")
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/scenes/Evening", `{"name":"Night"}`, &sc), 200)
	if sc.ID != "Night" || s.Engine.Automation().Rules[0].Scene != "Night" {
		t.Fatalf("%+v %+v", sc, s.Engine.Automation().Rules)
	}
	var list []v1Scene
	v1(t, ts, "GET", "/api/v1/scenes", "", &list)
	if len(list) != 2 {
		t.Fatalf("%+v", list)
	}
	// The limit of scenes is the engine's.
	for i := range 60 {
		if v1(t, ts, "POST", "/api/v1/scenes", `{"name":"s`+strings.Repeat("y", i)+`"}`, nil).StatusCode == 400 {
			return
		}
	}
	t.Fatal("no limit of scenes")
}

func TestV1Settings(t *testing.T) {
	s, ts := testServer(t)
	called, enabled := false, true
	was := [3]any{autostartSet, autostartEnabled, autostartSupported}
	autostartSupported = func() bool { return true }
	autostartEnabled = func() bool { return enabled }
	autostartSet = func(on bool) error { called, enabled = true, on; return nil }
	t.Cleanup(func() {
		autostartSet, autostartEnabled, autostartSupported = was[0].(func(bool) error), was[1].(func() bool), was[2].(func() bool)
	})
	var st v1Settings
	wantStatus(t, v1(t, ts, "GET", "/api/v1/settings", "", &st), 200)
	// the first language follows Windows, so it differs between machines
	if !st.Autostart || (st.Language != "de" && st.Language != "en") || !st.Animation || !st.DeviceVideo {
		t.Fatalf("%+v", st)
	}
	other := map[string]string{"de": "en", "en": "de"}[st.Language]
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/settings",
		`{"language":"`+other+`","fahrenheit":true,"clock12h":true,"animation":false,"deviceVideo":false,"screensaverOff":true,"intervalMs":2000,"updateCheck":false,"autostart":false}`, &st), 200)
	want := v1Settings{Autostart: false, AutostartSupported: true, Language: other, Fahrenheit: true, Clock12h: true, ScreensaverOff: true, IntervalMS: 2000}
	if st != want || !called {
		t.Fatalf("%+v, want %+v (autostart called: %v)", st, want, called)
	}
	if e := s.Engine.Settings(); !e.Fahrenheit || e.IntervalMS != 2000 || e.UpdateCheck || e.Animation {
		t.Fatalf("%+v", e)
	}
	// Fields left out stay as they are.
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/settings", `{"fahrenheit":false}`, &st), 200)
	if st.Fahrenheit || st.Language != other || st.IntervalMS != 2000 {
		t.Fatalf("%+v", st)
	}
	// What is not on the whitelist is refused; nothing changes.
	before := s.Engine.Settings()
	for _, body := range []string{`{"api":true}`, `{"mcp":true}`, `{"sensorSource":"custom"}`, `{"sensorUrl":"http://x"}`, `{"modules":{"display":false}}`,
		`{"presentmon":false}`, `{"rtss":false}`, `{"firmwareCheck":true}`, `{"disclaimer":true}`, `{"language":"fr"}`, `{"intervalMs":7}`,
		`{"fahrenheit":true,"api":true}`, `{"intervalMs":"fast"}`, `[]`, `{"fahrenheit":true`} {
		wantError(t, v1(t, ts, "PATCH", "/api/v1/settings", body, nil), 400, "bad_request")
	}
	if after := s.Engine.Settings(); after.API != before.API || after.MCP != before.MCP || after.Fahrenheit != before.Fahrenheit ||
		after.SensorSource != before.SensorSource {
		t.Fatalf("a refused request changed something: %+v -> %+v", before, after)
	}
	// Autostart is refused where it is not supported.
	autostartSupported = func() bool { return false }
	wantError(t, v1(t, ts, "PATCH", "/api/v1/settings", `{"autostart":true}`, nil), 400, "bad_request")
}

// A read token reads the new routes and is refused on the ones that change
// something; the other cases (no token, wrong token) are in
// TestEveryRouteEnforcesItsScope, which walks the whole table.
func TestV1NewRoutesScopes(t *testing.T) {
	s, ts := testServer(t)
	read := pairedToken(t, s, ts, "R", "read", false)
	ctl := pairedToken(t, s, ts, "C", "control", false)
	for _, route := range []string{"GET /api/v1/rotation", "GET /api/v1/automation", "GET /api/v1/awtrix", "GET /api/v1/settings", "GET /api/v1/layouts/gaming-clean"} {
		method, path, _ := strings.Cut(route, " ")
		if code, body, _ := raw(t, ts, method, path, "", withTok(read)); code == 403 {
			t.Errorf("%s with read: %d %s", route, code, body)
		}
	}
	for _, route := range []string{"DELETE /api/v1/designs/x", "PUT /api/v1/layouts/x", "DELETE /api/v1/layouts/x", "POST /api/v1/scenes",
		"PATCH /api/v1/scenes/x", "PATCH /api/v1/settings"} {
		method, path, _ := strings.Cut(route, " ")
		code, body, _ := raw(t, ts, method, path, `{}`, withHdr(jsonHdr, "X-Token", read))
		if code != 403 || !strings.Contains(body, "insufficient_scope") {
			t.Errorf("%s with read: %d %s", route, code, body)
		}
		if code, _, _ := raw(t, ts, method, path, `{}`, withHdr(jsonHdr, "X-Token", ctl)); code == 403 {
			t.Errorf("%s with control: %d", route, code)
		}
	}
	// system alone does not give the right to change settings.
	sys := pairedWith(t, s, ts, "S", "read", true, false)
	if code, _, _ := raw(t, ts, "PATCH", "/api/v1/settings", `{"fahrenheit":true}`, withHdr(jsonHdr, "X-Token", sys)); code != 403 {
		t.Errorf("system without control: %d", code)
	}
}
