package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestV1Lighting(t *testing.T) {
	_, ts := testServer(t)
	var v struct {
		Enabled    bool   `json:"enabled"`
		Active     bool   `json:"active"`
		Effect     string `json:"effect"`
		Brightness int    `json:"brightness"`
		Effects    []struct {
			Name     string `json:"name"`
			Label    string `json:"label"`
			LabelDE  string `json:"labelDe"`
			Featured bool   `json:"featured"`
		} `json:"effects"`
		Devices []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"devices"`
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting", "", &v), 200)
	if v.Enabled || v.Active || len(v.Devices) != 0 {
		t.Fatalf("lighting must start off: %+v", v)
	}
	featured := map[string]bool{}
	for _, e := range v.Effects {
		if e.Featured {
			featured[e.Name] = true
			if e.Name == "sensor" && (e.Label != "Temperature" || e.LabelDE != "Temperatur") {
				t.Fatalf("sensor labels: %+v", e)
			}
		}
	}
	if len(featured) != 9 || !featured["sensor"] || !featured["meteor"] || featured["fill"] || len(v.Effects) < 8 {
		t.Fatalf("featured effects: %v of %d", featured, len(v.Effects))
	}
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"enabled":true,"brightness":55,"effect":"breathing","params":{"period":99}}`, &v), 200)
	if !v.Enabled || !v.Active || v.Brightness != 55 || v.Effect != "breathing" || len(v.Devices) == 0 {
		t.Fatalf("after patch: %+v", v)
	}
	for _, body := range []string{
		`{"effect":"video"}`, `{"brightness":500}`, `{"params":{"nope":1}}`, `{"bogus":1}`,
		`{"devices":{"x":{"disabled":true}}}`, `not json`,
	} {
		wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", body, nil), 400, "bad_request")
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"tl":true}`, nil), 403, "gui_only")
	// screen and audio read private data: 409 until the window confirmed it
	for _, fx := range []string{"screen", "audio"} {
		wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"effect":"`+fx+`"}`, nil), 409, "ack_required")
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"devices":{"demo:board":{"ignoreConflicts":true}}}`, nil), 403, "gui_only")
	big := `{"params":{"a":"` + strings.Repeat("x", 2<<20) + `"}}`
	wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", big, nil), 413, "payload_too_large")
	var st struct {
		Lighting struct{ Enabled, Active bool } `json:"lighting"`
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if !st.Lighting.Enabled || !st.Lighting.Active {
		t.Fatalf("status lighting: %+v", st)
	}
	resp, _ := http.Get(ts.URL + "/api/v1/lighting")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no token: %d", resp.StatusCode)
	}
}

// The flash-writing TL modes, the sync header and every other device setting
// can only be changed from the CaseHUD window, never through API v1.
func TestFlashAndSyncSettingsAreGUIOnly(t *testing.T) {
	_, ts := testServer(t)
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"enabled":true}`, nil), 200)
	for _, body := range []string{
		`{"devices":{"demo:fans":{"settings":{"allowFlashWrites":true}}}}`,
		`{"devices":{"demo:fans":{"settings":{"mode":"hardware","allowFlashWrites":true}}}}`,
		`{"devices":{"demo:fans":{"settings":{"syncSource":"demo:board/argb1"}}}}`,
	} {
		wantError(t, v1(t, ts, "PATCH", "/api/v1/lighting", body, nil), 403, "gui_only")
	}
	if c := v1(t, ts, "POST", "/api/v1/lighting/sync", `{"device":"demo:fans","source":"demo:board/argb1"}`, nil).StatusCode; c == 200 {
		t.Fatalf("sync helper reachable through v1: %d", c)
	}
	// the window's token may use the helper; the client token may not
	if c := guiReq(t, ts, "POST", ts.URL+"/api/lighting/sync", "client").StatusCode; c != http.StatusForbidden {
		t.Fatalf("client token on the GUI route: %d", c)
	}
	var v struct {
		Devices []struct {
			ID            string         `json:"id"`
			SettingValues map[string]any `json:"settingValues"`
		} `json:"devices"`
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting", "", &v), 200)
	for _, d := range v.Devices {
		if d.ID == "demo:fans" && d.SettingValues["allowFlashWrites"] != false {
			t.Fatalf("allowFlashWrites changed: %v", d.SettingValues)
		}
	}
}

func TestZoneSetupRoutesAreGUIOnly(t *testing.T) {
	_, ts := testServer(t)
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"enabled":true}`, nil), 200)
	for _, p := range []string{"/assist", "/zone-setup", "/templates/apply"} {
		if c := v1(t, ts, "POST", "/api/v1/lighting"+p, `{}`, nil).StatusCode; c == 200 || c == 204 {
			t.Fatalf("%s reachable through v1: %d", p, c)
		}
		if c := guiReq(t, ts, "POST", ts.URL+"/api/lighting"+p, "client").StatusCode; c != http.StatusForbidden {
			t.Fatalf("client token on %s: %d", p, c)
		}
	}
	for _, p := range []string{"/parts", "/templates"} {
		if c := guiReq(t, ts, "GET", ts.URL+"/api/lighting"+p, "client").StatusCode; c != http.StatusForbidden {
			t.Fatalf("client token on %s: %d", p, c)
		}
	}
	// the zone label is part of the v1 state and can be set there
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"devices":{"demo:board":{"zones":{"argb1":{"label":"Case strip"}}}}}`, nil), 200)
	var v struct {
		Devices []struct {
			ID    string `json:"id"`
			Zones []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				Count int    `json:"count"`
			} `json:"zones"`
		} `json:"devices"`
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting", "", &v), 200)
	found := false
	for _, d := range v.Devices {
		for _, z := range d.Zones {
			if d.ID == "demo:board" && z.ID == "argb1" {
				found = z.Label == "Case strip"
			}
		}
	}
	if !found {
		t.Fatalf("label missing: %+v", v.Devices)
	}
}

func TestV1LightingPresets(t *testing.T) {
	_, ts := testServer(t)
	type preset struct {
		ID      string            `json:"id"`
		Name    map[string]string `json:"name"`
		Effect  string            `json:"effect"`
		Palette []string          `json:"palette"`
		Source  string            `json:"source"`
		Active  bool              `json:"active"`
	}
	var list []preset
	wantStatus(t, v1(t, ts, "GET", "/api/v1/lighting/presets", "", &list), 200)
	if list == nil || len(list) != 0 {
		t.Fatalf("a fresh install must list no presets: %+v", list)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/lighting/presets/nope/apply", "", nil), 404, "not_found")

	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"effect":"rainbow"}`, nil), 200)
	var own preset
	wantStatus(t, v1(t, ts, "POST", "/api/v1/lighting/presets", `{"name":"Couch"}`, &own), 200)
	if own.ID != "my-couch" || own.Source != "user" || !own.Active || own.Effect != "rainbow" || own.Name["de"] != "Couch" {
		t.Fatalf("saved: %+v", own)
	}
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/lighting", `{"effect":"solid"}`, nil), 200)
	var applied struct {
		Lighting struct {
			Effect string `json:"effect"`
			Preset string `json:"preset"`
		} `json:"lighting"`
		Ignored []string `json:"ignored"`
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/lighting/presets/my-couch/apply", "", &applied), 200)
	if applied.Lighting.Effect != "rainbow" || applied.Lighting.Preset != "my-couch" || applied.Ignored == nil {
		t.Fatalf("apply: %+v", applied)
	}
	var st struct {
		Lighting struct {
			Preset string `json:"preset"`
		} `json:"lighting"`
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.Lighting.Preset != "my-couch" {
		t.Fatalf("status preset %q", st.Lighting.Preset)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/lighting/presets", `{"name":""}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/lighting/presets", `{"name":"x","bogus":1}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "DELETE", "/api/v1/lighting/presets/rainbow-slow", "", nil), 400, "bad_request")
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/lighting/presets/my-couch", "", nil), http.StatusNoContent)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/lighting/presets/my-couch", "", nil), 404, "not_found")
}
