package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/app"
)

// The demo offers these sensors and the fans port1-fan1 to port3-fan1.
const demoCooling = `{"mode":"preview","curves":[
 {"id":"c","name":"C","sources":["Board/Temperatures/Coolant"],"points":[[20,%s],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"port1-fan1","name":"Radiator","curve":"c","step_up":100,"step_down":100,"safe_duty":70},
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"Board/Fans/Pump","curve":"c","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}
]}`

func demoCoolingAt(low string) string { return strings.Replace(demoCooling, "%s", low, 1) }

// gui sends a request with the token of the window.
func gui(t *testing.T, ts *httptest.Server, method, path, body string, hdr map[string]string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	req.Header.Set("X-Token", "secret")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func configureCooling(t *testing.T, ts *httptest.Server, cfg string) {
	t.Helper()
	if code, body := gui(t, ts, "PUT", "/api/cooling/config", cfg, nil); code != http.StatusNoContent {
		t.Fatalf("config: %d %s", code, body)
	}
}

func savePreset(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	code, body := gui(t, ts, "POST", "/api/cooling/presets", `{"name":"`+name+`"}`, nil)
	if code != http.StatusOK {
		t.Fatalf("save %s: %d %s", name, code, body)
	}
	var v app.CoolingPresetsView
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	for _, p := range v.Presets {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("%s not listed: %s", name, body)
	return ""
}

func getCooling(t *testing.T, ts *httptest.Server) app.CoolingAPI {
	t.Helper()
	var v app.CoolingAPI
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling", "", &v), 200)
	return v
}

func TestV1CoolingNeedsAToken(t *testing.T) {
	_, ts := testServer(t)
	for _, tc := range []struct{ method, path, tok string }{
		{"GET", "/api/v1/cooling", ""}, {"GET", "/api/v1/cooling", "wrong"},
		{"POST", "/api/v1/cooling/preset", ""}, {"POST", "/api/v1/cooling/preset", "wrong"},
	} {
		req, _ := http.NewRequest(tc.method, ts.URL+tc.path, strings.NewReader(`{"id":"x"}`))
		if tc.tok != "" {
			req.Header.Set("X-Token", tc.tok)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s token %q: %d", tc.method, tc.path, tc.tok, resp.StatusCode)
		}
	}
	// The client token and the one of the window both count.
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling", "", nil), 200)
	if code, _ := gui(t, ts, "GET", "/api/v1/cooling", "", nil); code != 200 {
		t.Errorf("window token: %d", code)
	}
	resp := v1(t, ts, "PUT", "/api/v1/cooling", "", nil)
	wantError(t, resp, 405, "method_not_allowed")
}

func TestV1CoolingAnswerShape(t *testing.T) {
	_, ts := testServer(t)
	var raw map[string]json.RawMessage
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling", "", &raw), 200)
	for _, k := range []string{"enabled", "state", "blockedBy", "preset", "presets", "alarm", "outputs", "temperatures"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("%s missing", k)
		}
	}
	v := getCooling(t, ts)
	if !v.Enabled || v.State != "unconfigured" || string(raw["preset"]) != "null" || string(raw["alarm"]) != "null" || string(raw["outputs"]) != "[]" || string(raw["presets"]) != "[]" {
		t.Fatalf("%s", raw)
	}
	configureCooling(t, ts, demoCoolingAt("60"))
	v = getCooling(t, ts)
	if v.State != "test" || len(v.Outputs) != 2 || v.Outputs[0].Kind != "fan" || v.Outputs[1].Kind != "pump" {
		t.Fatalf("%+v", v)
	}
	if len(v.Temps) != 1 || v.Temps[0].ID != "Board/Temperatures/Coolant" {
		t.Fatalf("%+v", v.Temps)
	}
}

func TestV1CoolingPresetSwitch(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	day := savePreset(t, ts, "Tag")
	configureCooling(t, ts, demoCoolingAt("30"))
	evening := savePreset(t, ts, "Abend")
	if v := getCooling(t, ts); v.Preset == nil || *v.Preset != evening || len(v.Presets) != 2 {
		t.Fatalf("%+v", v)
	}

	var got app.CoolingAPI
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"`+day+`"}`, &got), 200)
	if got.Preset == nil || *got.Preset != day || got.State != "test" {
		t.Fatalf("%+v", got)
	}
	if again := getCooling(t, ts); again.Preset == nil || *again.Preset != day {
		t.Fatalf("%+v", again)
	}
	// The one that is active already: 200 and nothing happens.
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"`+day+`"}`, &got), 200)

	// A change by hand leaves no preset.
	configureCooling(t, ts, demoCoolingAt("45"))
	if v := getCooling(t, ts); v.Preset != nil {
		t.Fatalf("%+v", v.Preset)
	}

	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"nope"}`, nil), 404, "not_found")
	for _, body := range []string{``, `{`, `{}`, `{"id":""}`, `{"id":1}`, `{"id":"a","x":1}`, `[]`} {
		wantError(t, v1(t, ts, "POST", "/api/v1/cooling/preset", body, nil), 400, "bad_request")
	}
}

func TestV1CoolingPresetRefusals(t *testing.T) {
	s, ts := testServer(t)
	e := s.Engine
	// Nothing set up.
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"x"}`, nil), 404, "not_found")
	configureCooling(t, ts, demoCoolingAt("60"))
	day := savePreset(t, ts, "Tag")
	configureCooling(t, ts, demoCoolingAt("30"))
	before, _ := os.ReadFile(filepath.Join(e.Store.Dir, "cooling.json"))

	// A preset that does not fit the hardware any more: invalid_preset, nothing changed.
	bad := strings.Replace(demoCoolingAt("60"), "port1-fan1", "port4-fan1", 1)
	list := `{"presets":[{"id":"alt","name":"Alt","config":` + strings.Replace(bad, `"mode":"preview",`, "", 1) + `}]}`
	if err := os.WriteFile(filepath.Join(e.Store.Dir, "cooling-presets.json"), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"alt"}`, nil), 409, "invalid_preset")
	after, _ := os.ReadFile(filepath.Join(e.Store.Dir, "cooling.json"))
	if string(before) != string(after) {
		t.Fatal("a refused preset changed the configuration")
	}
	_ = day

	// A file that cannot be read is an error, not an empty list.
	if err := os.WriteFile(filepath.Join(e.Store.Dir, "cooling-presets.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	resp := v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"alt"}`, nil)
	wantError(t, resp, 500, "internal")
	if b, _ := os.ReadFile(filepath.Join(e.Store.Dir, "cooling-presets.json")); string(b) != "{" {
		t.Fatal("the unreadable file was overwritten")
	}

	// Switched off: cooling_off, and the answer to GET is the off state.
	if err := e.SetModules(app.Modules{Display: true, Lighting: true}); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"alt"}`, nil), 409, "cooling_off")
	if v := getCooling(t, ts); v.Enabled || v.State != "off" || v.Preset != nil || len(v.Outputs) != 0 || len(v.Temps) != 0 || len(v.Presets) != 0 {
		t.Fatalf("%+v", v)
	}
}

// Every reason a preset can be refused has its status and code.
func TestV1PresetErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{
		{app.PresetNotFound, 404}, {app.PresetBadRequest, 400}, {app.PresetStorage, 500},
		{app.PresetCoolingOff, 409}, {app.PresetUnconfigured, 409}, {app.PresetBlocked, 409},
		{app.PresetBusy, 409}, {app.PresetInvalid, 409},
	} {
		rec := httptest.NewRecorder()
		v1PresetError(rec, &app.CoolingPresetError{Code: tc.code, Msg: "m"})
		var e apiError
		json.Unmarshal(rec.Body.Bytes(), &e)
		want := tc.code
		if tc.code == app.PresetStorage {
			want = "internal"
		}
		if rec.Code != tc.status || e.Error.Code != want || e.Error.Message == "" {
			t.Errorf("%s: %d %s", tc.code, rec.Code, rec.Body)
		}
		if got := presetStatus(&app.CoolingPresetError{Code: tc.code}); got != tc.status {
			t.Errorf("%s: internal status %d", tc.code, got)
		}
	}
}

func TestCoolingPresetRoutesOfTheWindow(t *testing.T) {
	_, ts := testServer(t)
	if code, _ := gui(t, ts, "POST", "/api/cooling/presets", `{"name":"x"}`, nil); code != 409 {
		t.Fatalf("without a setup: %d", code)
	}
	configureCooling(t, ts, demoCoolingAt("60"))
	id := savePreset(t, ts, "Tag")
	if code, _ := gui(t, ts, "POST", "/api/cooling/presets", `{"name":"tag"}`, nil); code != 400 {
		t.Fatalf("same name: %d", code)
	}
	if code, _ := gui(t, ts, "POST", "/api/cooling/presets", `{`, nil); code != 400 {
		t.Fatalf("bad body: %d", code)
	}
	if code, body := gui(t, ts, "PUT", "/api/cooling/presets/"+id, `{"name":"Neu"}`, nil); code != 200 || !strings.Contains(body, `"id":"`+id+`"`) || !strings.Contains(body, `"Neu"`) {
		t.Fatalf("rename: %d %s", code, body)
	}
	configureCooling(t, ts, demoCoolingAt("30"))
	if code, body := gui(t, ts, "POST", "/api/cooling/presets/"+id+"/overwrite", ``, nil); code != 200 || !strings.Contains(body, `"active":"`+id+`"`) {
		t.Fatalf("overwrite: %d %s", code, body)
	}
	for _, p := range []struct{ method, path string }{{"PUT", "/api/cooling/presets/nope"}, {"POST", "/api/cooling/presets/nope/overwrite"}, {"POST", "/api/cooling/presets/nope/activate"}, {"DELETE", "/api/cooling/presets/nope"}} {
		if code, _ := gui(t, ts, p.method, p.path, `{"name":"x"}`, nil); code != 404 {
			t.Errorf("%s %s: %d", p.method, p.path, code)
		}
	}
	// The singular route only activates.
	for _, p := range []struct{ method, path string }{{"DELETE", "/api/v1/cooling/preset"}, {"PUT", "/api/v1/cooling/preset"}} {
		if r := v1(t, ts, p.method, p.path, `{"name":"x"}`, nil); r.StatusCode != 404 && r.StatusCode != 405 {
			t.Errorf("%s %s: %d", p.method, p.path, r.StatusCode)
		}
	}
	if code, _ := gui(t, ts, "DELETE", "/api/cooling/presets/"+id, ``, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	// The routes of the window need its token.
	req, _ := http.NewRequest("POST", ts.URL+"/api/cooling/presets/"+id+"/activate", nil)
	req.Header.Set("X-Token", "client")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 && resp.StatusCode != 401 {
		t.Fatalf("client token on a window route: %d", resp.StatusCode)
	}
}

func TestCoolingRevisionOfTheWindow(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	day := savePreset(t, ts, "Tag")
	configureCooling(t, ts, demoCoolingAt("30"))
	var set struct {
		Revision int64 `json:"revision"`
	}
	read := func() int64 {
		_, body := gui(t, ts, "GET", "/api/cooling/setup", "", nil)
		if err := json.Unmarshal([]byte(body), &set); err != nil || set.Revision == 0 {
			t.Fatalf("%v %s", err, body)
		}
		return set.Revision
	}
	rev := read()
	// A preset is switched while the page has unsaved changes.
	if code, _ := gui(t, ts, "POST", "/api/cooling/presets/"+day+"/activate", ``, nil); code != 200 {
		t.Fatalf("activate: %d", code)
	}
	code, body := gui(t, ts, "PUT", "/api/cooling/config", demoCoolingAt("50"), map[string]string{"If-Match": itoa(rev)})
	if code != http.StatusConflict {
		t.Fatalf("stale save: %d %s", code, body)
	}
	if v := getCooling(t, ts); v.Preset == nil || *v.Preset != day {
		t.Fatal("the stale save changed the configuration")
	}
	code, _ = gui(t, ts, "PUT", "/api/cooling/config", demoCoolingAt("50"), map[string]string{"If-Match": itoa(read())})
	if code != http.StatusNoContent {
		t.Fatalf("current save: %d", code)
	}
	// The mode switch tells the new revision, so the page can follow it.
	cur := read()
	req, _ := http.NewRequest("PUT", ts.URL+"/api/cooling/mode", strings.NewReader(`{"mode":"preview"}`))
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 || resp.Header.Get("ETag") != itoa(cur+1) {
		t.Fatalf("mode: %d etag %q (was %d)", resp.StatusCode, resp.Header.Get("ETag"), cur)
	}
	code, _ = gui(t, ts, "PUT", "/api/cooling/config", demoCoolingAt("50"), map[string]string{"If-Match": itoa(cur)})
	if code != http.StatusConflict {
		t.Fatalf("a save from before the mode switch: %d", code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// The cooling event comes with a change of the preset, and not otherwise.
func TestV1CoolingEvent(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	day := savePreset(t, ts, "Tag")
	configureCooling(t, ts, demoCoolingAt("30"))
	evening := savePreset(t, ts, "Abend")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/events", nil)
	req.Header.Set("X-Token", "client")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	events := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		name := ""
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				name = v
			}
			if v, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				events <- name + " " + v
			}
		}
		close(events)
	}()
	next := func(wait time.Duration) (string, bool) {
		select {
		case ev, ok := <-events:
			return ev, ok
		case <-time.After(wait):
			return "", false
		}
	}
	if ev, _ := next(3 * time.Second); !strings.HasPrefix(ev, "status ") {
		t.Fatalf("first event: %.60s", ev)
	}
	// Nothing changes: no cooling event, even though measured values move.
	if ev, ok := next(1500 * time.Millisecond); ok && strings.HasPrefix(ev, "cooling ") {
		t.Fatalf("an event without a change: %.80s", ev)
	}
	// The preset that is active already: no event either.
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"`+evening+`"}`, nil), 200)
	if ev, ok := next(1500 * time.Millisecond); ok && strings.HasPrefix(ev, "cooling ") {
		t.Fatalf("an event for no effect: %.80s", ev)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/preset", `{"id":"`+day+`"}`, nil), 200)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			name, data, _ := strings.Cut(ev, " ")
			if name != "cooling" {
				continue
			}
			var v app.CoolingAPI
			if err := json.Unmarshal([]byte(data), &v); err != nil || v.Preset == nil || *v.Preset != day {
				t.Fatalf("%v %s", err, data)
			}
			return
		case <-deadline:
			t.Fatal("no cooling event after the switch")
		}
	}
}

func TestV1AreasThatAreOff(t *testing.T) {
	s, ts := testServer(t)
	e := s.Engine
	panels := e.Panels()
	if len(panels) == 0 {
		t.Fatal("no demo panels")
	}
	id := panels[0].ID
	if err := e.SetModules(app.Modules{Display: false, Lighting: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{"PATCH", "/api/v1/panels/" + id, `{"brightness":50}`},
		{"POST", "/api/v1/panels/" + id + "/identify", ``},
		{"POST", "/api/v1/identify", ``},
		{"POST", "/api/v1/sets/apply", `{"set":"x"}`},
		{"POST", "/api/v1/scenes/x/activate", ``},
		{"PATCH", "/api/v1/rotation", `{"enabled":true}`},
		{"POST", "/api/v1/rotation/next", ``},
		{"PATCH", "/api/v1/automation", `{"enabled":true}`},
		{"POST", "/api/v1/power", `{"on":false}`},
		{"POST", "/api/v1/gaming", `{"active":true}`},
		{"PATCH", "/api/v1/awtrix", `{"enabled":true}`},
		{"PUT", "/api/v1/layouts/x", `{}`},
		{"DELETE", "/api/v1/layouts/x", ``},
	} {
		wantError(t, v1(t, ts, tc.method, tc.path, tc.body, nil), 409, "area_off")
	}
	// Reading stays possible and tells the truth.
	var st v1Status
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.Areas.Display || !st.Areas.Lighting || len(st.Panels) != 0 {
		t.Fatalf("%+v %d panels", st.Areas, len(st.Panels))
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/panels", "", nil), 200)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/designs", "", nil), 200)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting", "", nil), 200)
	// Cooling and lighting go on.
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling", "", nil), 200)

	if err := e.SetModules(app.Modules{Display: true, Lighting: false, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"brightness":10}`, nil), 409, "area_off")
	wantError(t, v1(t, ts, "POST", "/api/v1/lighting/presets/x/apply", ``, nil), 409, "area_off")
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting", "", nil), 200)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting/presets", "", nil), 200)
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.Lighting.Enabled || st.Areas.Lighting {
		t.Fatalf("%+v", st.Lighting)
	}
	// With the display back, panels answer again.
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+id, `{"brightness":50}`, nil), 200)
}

func TestAreaOfOnlyNamesWhatChangesAnArea(t *testing.T) {
	for _, tc := range []struct{ method, path, want string }{
		{"GET", "/api/v1/status", ""}, {"GET", "/api/v1/panels", ""}, {"GET", "/api/v1/cooling", ""},
		{"PATCH", "/api/v1/panels/a", "display"}, {"POST", "/api/v1/panels/a/identify", "display"},
		{"POST", "/api/v1/power", "display"}, {"POST", "/api/v1/cooling/preset", "cooling"},
		{"PATCH", "/api/v1/lighting", "lighting"}, {"POST", "/api/v1/lighting/presets/a/apply", "lighting"},
		{"POST", "/api/v1/lighting/presets", ""}, {"DELETE", "/api/v1/lighting/presets/a", ""},
		{"POST", "/api/v1/community/a/import", ""}, {"POST", "/api/v1/quit", ""}, {"POST", "/api/v1/update/check", ""},
		{"PUT", "/api/v1/layouts/a", "display"}, {"DELETE", "/api/v1/layouts/a", "display"}, {"GET", "/api/v1/layouts/a", ""},
		{"DELETE", "/api/v1/designs/a", ""}, {"POST", "/api/v1/scenes", ""}, {"PATCH", "/api/v1/scenes/a", ""},
		{"PATCH", "/api/v1/settings", ""}, {"GET", "/api/v1/settings", ""},
	} {
		if got := areaOf(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestCoolingMeasureStopTakesOnlyKnownChoices(t *testing.T) {
	_, ts := testServer(t)
	for q, want := range map[string]int{"": 204, "?keep=finished": 204, "?keep=all": 400, "?keep=": 204} {
		if code, _ := gui(t, ts, "DELETE", "/api/cooling/measure"+q, "", nil); code != want {
			t.Errorf("%q: %d, want %d", q, code, want)
		}
	}
}
