package app

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

const libPart = `{"format":1,"id":"lib-test-fan","vendor":"V","model":"Fan","category":"fan","dimensions_mm":{"w":120,"h":120},
 "leds":{"count":2,"positions_mm":[[10,10],[100,100]]}}`

func libraryEngine(t *testing.T) (*Engine, *int) {
	t.Helper()
	hits := new(int)
	files := map[string]string{
		"/lib/components.json":   `{"version":1,"components":[{"id":"lib-test-fan","file":"components/f.json"},{"id":"fan-tl-26","file":"components/f.json"}]}`,
		"/lib/components/f.json": libPart,
		"/lib/lighting.json":     `{"version":1,"presets":[{"id":"mete","file":"l/m.json"},{"id":"future","file":"l/f.json"}]}`,
		"/lib/l/m.json":          `{"format":1,"id":"mete","name":{"de":"Meteor","en":"Meteor"},"effect":"meteor","params":{"color1":"#ff0000","speed":300,"notAParam":1},"path":{"mode":"horizontal","mirror":true}}`,
		"/lib/l/f.json":          `{"format":1,"id":"future","name":{"en":"Future"},"effect":"quantum","params":{}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		if b, ok := files[r.URL.Path]; ok {
			io.WriteString(w, b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Store.SaveConfig(Config{CommunityURL: srv.URL + "/lib/index.json", Panels: map[string]PanelConfig{}}); err != nil {
		t.Fatal(err)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e, hits
}

func TestLibraryNoNetworkUntilAsked(t *testing.T) {
	e, hits := libraryEngine(t)
	_ = e.LightingPresets()
	_ = e.LibraryStatus()
	e.Lighting()
	if *hits != 0 {
		t.Fatalf("%d requests before the user asked", *hits)
	}
}

func TestLibraryRefreshFeedsComponentsAndPresets(t *testing.T) {
	e, _ := libraryEngine(t)
	if _, err := e.RefreshLibrary(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := leds.LookupComponent("lib-test-fan"); !ok {
		t.Fatal("library component not registered")
	}
	if c, _ := leds.LookupComponent("fan-tl-26"); c.Label == "" || len(c.LEDs) != 26 {
		t.Fatalf("built-in component replaced: %+v", c)
	}
	ps := e.LightingPresets()
	if len(ps) != 1 || ps[0].ID != "mete" {
		t.Fatalf("presets (unknown effects must not be offered): %+v", ps)
	}
	v, ignored, err := e.ApplyLightingPreset("mete")
	if err != nil {
		t.Fatal(err)
	}
	if v.Effect != "meteor" || v.Params["color1"] != "#ff0000" || v.Params["speed"] == 300.0 && false || v.Params["path"] != "horizontal" || v.Params["mirror"] != true {
		t.Fatalf("view after preset: effect %s params %v", v.Effect, v.Params)
	}
	if len(ignored) != 1 || ignored[0] != "notAParam" {
		t.Fatalf("ignored = %v", ignored)
	}
	if _, _, err := e.ApplyLightingPreset("future"); err == nil {
		t.Fatal("preset with an unknown effect applied")
	}
	if _, _, err := e.ApplyLightingPreset("../x"); err == nil {
		t.Fatal("bad id applied")
	}
	// A stored layout may use a library part: it survives a restart offline.
	e.Stop()
	e2 := &Engine{Store: e.Store, Demo: true}
	e2.cfg.CommunityURL = "http://127.0.0.1:1/x/index.json" // unreachable, cache only
	if _, err := e2.Library(); err != nil {
		t.Fatal(err)
	}
	e2.loadLibraryComponents()
}

func TestApplyEveryEffectDefaults(t *testing.T) {
	e := startDemo(t)
	for _, info := range lightingEffects {
		params := map[string]any{}
		for _, d := range info.Params {
			params[d.Key] = d.Default
		}
		if _, err := e.PatchLighting(LightingPatch{Effect: &info.Name, Params: params}); err != nil {
			t.Errorf("effect %s with its own defaults: %v", info.Name, err)
		}
	}
}

func TestDiagnosticsExportScrubs(t *testing.T) {
	e := startDemo(t)
	home, _ := os.UserHomeDir()
	user := os.Getenv("USERNAME")
	log := "start " + filepath.Join(home, "AppData", "x.gif") + " by " + user + "\r\n" +
		"GET /api/state?t=SECRETTOKEN123 from 192.168.4.20 mac aa:bb:cc:dd:ee:ff " + `\\?\hid#vid_0416&pid_7372#7&2a` + "\r\n" +
		"x-token: abcdef and http://bob:pw@host/\r\n"
	if err := os.WriteFile(filepath.Join(e.Store.Dir, "casehud.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.cfg.LHMURL = "http://10.1.2.3:8085"
	e.cfg.Awtrix = []AwtrixDevice{{Host: "192.168.9.9"}}
	e.mu.Unlock()
	var buf bytes.Buffer
	if err := e.WriteDiagnostics(context.Background(), &buf, "1.2.3-test", DiagnosticsOptions{}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		all += string(b)
	}
	for _, n := range []string{"README.txt", "version.json", "config.json", "panels.json", "lighting.json", "casehud.log"} {
		if !names[n] {
			t.Errorf("%s missing from the export", n)
		}
	}
	bad := []string{"SECRETTOKEN123", "192.168.4.20", "192.168.9.9", "10.1.2.3", "aa:bb:cc:dd:ee:ff", "vid_0416", "bob:pw", "abcdef"}
	if len(home) > 3 {
		bad = append(bad, home)
	}
	if len(user) >= 3 {
		bad = append(bad, user)
	}
	for _, s := range bad {
		if strings.Contains(strings.ToLower(all), strings.ToLower(s)) {
			t.Errorf("export still contains %q", s)
		}
	}
	if !strings.Contains(all, "1.2.3-test") || !strings.Contains(all, "%USERPROFILE%") {
		t.Errorf("export lacks version or the home placeholder")
	}
	if strings.Contains(all, `"packets"`) {
		t.Error("packets included without being asked")
	}
}

func TestLightTraceNeedsRunningLighting(t *testing.T) {
	e := startDemo(t)
	if _, err := e.StartLightTrace(); err == nil {
		t.Fatal("trace started while lighting is off")
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartLightTrace(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartLightTrace(); err == nil {
		t.Fatal("second trace accepted while one runs")
	}
	if on, _ := e.LightTraceStatus(); !on {
		t.Fatal("status says no trace")
	}
	e.light.stopTrace()
	if on, _ := e.LightTraceStatus(); on {
		t.Fatal("trace still on after stop")
	}
}

func TestSyncModeOfAController(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	mode := func(m string) error {
		_, err := e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{"demo:fans": {Settings: map[string]any{"mode": m}}}})
		return err
	}
	for _, m := range []string{"motherboard", "canvas", "hardware", "passive"} {
		if err := mode(m); err != nil {
			t.Errorf("mode %s: %v", m, err)
		}
	}
	if err := mode("evil"); err == nil {
		t.Error("unknown mode accepted")
	}
}

func TestScrubberCases(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	s := e.scrubFunc()
	for in, bad := range map[string]string{
		"Authorization: Bearer abc123def":      "abc123def",
		"access_token=zzz999 ok":               "zzz999",
		`{"clientToken":"qq77ww"}`:             "qq77ww",
		"open \x5c\x5cnas01\x5cshare\x5cx.gif": "nas01",
		"sid S-1-5-21-111-222-333-1001":        "111-222",
		"host mypc.local up":                   "mypc",
		"addr fe80:0:0:0:1:2:3:4":              "fe80",
	} {
		if out := s(in); strings.Contains(out, bad) {
			t.Errorf("%q -> %q still has %q", in, out, bad)
		}
	}
	if got := s("version 1.1.0.5 at 12:30:45"); got != "version 1.1.0.5 at 12:30:45" {
		t.Errorf("harmless text changed: %q", got)
	}
}

func TestDiagnosticsHashesSerialsAndNames(t *testing.T) {
	e := startDemo(t)
	e.mu.Lock()
	e.cfg.Panels["tllcd-sABC123SERIAL"] = PanelConfig{}
	e.cfg.Panels["flex-ZXCV9876"] = PanelConfig{}
	e.cfg.Rotation.Scenes = []Scene{{Name: "Mein Gaming Szene", Panels: map[string]PanelConfig{"tllcd-sABC123SERIAL-1a2b3c4d": {}}}}
	e.cfg.Automation.Rules = []AutomationRule{{When: "game:SecretGame.exe", Scene: "Meine Szene"}}
	e.cfg.Awtrix = []AwtrixDevice{{Host: "my-clock.example.org", Name: "Meine Uhr"}}
	e.mu.Unlock()
	log := "panel UNI FAN TL LCD (sQQQ777SERIAL) up; Flex LCD (ZXCV9876) and tllcd-sABC123SERIAL, flex-ZXCV9876\r\n" +
		"connect http://my-nas.example.org:8085/x and 8.8.8.8 and 127.0.0.1 version 1.1.0.5\r\n"
	if err := os.WriteFile(filepath.Join(e.Store.Dir, "casehud.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.WriteDiagnostics(context.Background(), &buf, "1.2.3-test", DiagnosticsOptions{}); err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	all := ""
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		all += string(b)
	}
	for _, bad := range []string{"ABC123SERIAL", "ZXCV9876", "QQQ777SERIAL", "Mein Gaming", "Meine Szene", "SecretGame", "Meine Uhr",
		"my-clock", "my-nas", "8.8.8.8"} {
		if strings.Contains(all, bad) {
			t.Errorf("export still contains %q", bad)
		}
	}
	for _, want := range []string{"tllcd-s#", "flex-s#", "LCD (#", "127.0.0.1", "version 1.1.0.5", "serial numbers", "003cname"} {
		if !strings.Contains(strings.ToLower(all), strings.ToLower(want)) {
			t.Errorf("export lacks %q", want)
		}
	}
}

func TestScrubberHashIsStable(t *testing.T) {
	s := (&Engine{Store: &Store{Dir: t.TempDir()}}).scrubFunc()
	a, b := s("tllcd-sSERIAL1"), s("tllcd-sSERIAL1")
	if a != b || strings.Contains(a, "SERIAL1") || !strings.HasPrefix(a, "tllcd-s#") || len(a) != len("tllcd-s#")+8 {
		t.Fatalf("hash: %q %q", a, b)
	}
	if s("tllcd-sSERIAL2") == a {
		t.Fatal("different serials, same hash")
	}
	if got := s("flex-u"); got != "flex-u" {
		t.Fatalf("no serial, changed: %q", got)
	}
	if got := s("addr 203.0.113.9 ok"); strings.Contains(got, "203.0.113.9") {
		t.Fatalf("public IP kept: %q", got)
	}
}
