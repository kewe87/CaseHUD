package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/app"
)

// v1 sends a request with the client token and decodes a JSON answer into out
// (if not nil).
func v1(t *testing.T, ts *httptest.Server, method, path, body string, out any) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	req.Header.Set("X-Token", "client")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: bad JSON %q: %v", method, path, b, err)
		}
	}
	resp.Body = io.NopCloser(strings.NewReader(string(b)))
	return resp
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status %d, want %d (%s)", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, b)
	}
}

func wantError(t *testing.T, resp *http.Response, status int, code string) {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	var e apiError
	if err := json.Unmarshal(b, &e); err != nil || e.Error.Code != code || e.Error.Message == "" {
		t.Fatalf("%s %s: body %q, want error code %q", resp.Request.Method, resp.Request.URL.Path, b, code)
	}
	if resp.StatusCode != status {
		t.Fatalf("%s %s: status %d, want %d", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, status)
	}
}

func TestV1Auth(t *testing.T) {
	_, ts := testServer(t)
	do := func(method, path, tok string, host string) *http.Response {
		req, _ := http.NewRequest(method, ts.URL+path, nil)
		if tok != "" {
			req.Header.Set("X-Token", tok)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	wantStatus(t, do("GET", "/api/v1/status", "client", ""), 200)
	wantStatus(t, do("GET", "/api/v1/status", "secret", ""), 200) // GUI token works on v1
	wantError(t, do("GET", "/api/v1/status", "", ""), 403, "unauthorized")
	wantError(t, do("GET", "/api/v1/status", "nope", ""), 403, "unauthorized")
	// The client token never opens the internal API.
	if r := do("GET", "/api/state", "client", ""); r.StatusCode != 403 {
		t.Fatalf("client token on /api/state: %d, want 403", r.StatusCode)
	}
	// ?t= only for GET .../preview.png.
	wantError(t, do("GET", "/api/v1/status?t=client", "", ""), 403, "unauthorized")
	wantStatus(t, do("GET", "/api/v1/designs/clock/preview.png?t=client", "", ""), 200)
	wantError(t, do("POST", "/api/v1/identify?t=client", "", ""), 403, "unauthorized")
	// DNS rebinding: wrong Host header.
	wantError(t, do("GET", "/api/v1/status", "client", "evil.example:80"), 403, "forbidden")
	wantStatus(t, do("GET", "/api/v1/status", "client", "localhost:1234"), 200)
	wantStatus(t, do("GET", "/api/v1/status", "client", "LocalHost:1234"), 200)
	wantError(t, do("GET", "/api/v1/status", "client", "localhost"), 403, "forbidden")
}

func TestV1HostPort(t *testing.T) {
	s := &Server{}
	if !s.hostOK("127.0.0.1:99") {
		t.Fatal("no listener: any port should pass")
	}
	for _, h := range []string{"", "127.0.0.1", "[::1]:80", "127.0.0.2:80", "localhost.evil.com:80"} {
		if s.hostOK(h) {
			t.Errorf("host %q accepted", h)
		}
	}
}

func TestV1StatusAndPanels(t *testing.T) {
	_, ts := testServer(t)
	var st v1Status
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.APIVersion != 1 || st.Version != "test" || !st.Power || len(st.Panels) != 5 || !st.Sensors.OK {
		t.Fatalf("status %+v", st)
	}
	if st.Awtrix.Devices == nil || st.Gaming.Active || st.UpdateAvailable != "" {
		t.Fatalf("status %+v", st)
	}
	p := st.Panels[1] // the pump head sorts first
	if p.Model != "UNI FAN TL LCD" || p.W != 400 || p.Mode != "hud" || p.FanNo != 1 || !p.Connected || p.Theme != "dark" || p.Error != "" {
		t.Fatalf("panel %+v", p)
	}
	if st.Panels[0].Model != "HydroShift LCD" || st.Panels[0].FanNo != 5 || st.Panels[0].W != 480 {
		t.Fatalf("panel %+v", st.Panels[0])
	}
	var ps []v1Panel
	wantStatus(t, v1(t, ts, "GET", "/api/v1/panels", "", &ps), 200)
	if len(ps) != 5 {
		t.Fatal(len(ps))
	}
	// Panel preview becomes available once a frame was rendered.
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp := v1(t, ts, "GET", "/api/v1/panels/"+p.ID+"/preview.png", "", nil)
		if resp.StatusCode == 200 {
			if _, err := png.Decode(resp.Body); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no preview, last status %d", resp.StatusCode)
		}
		time.Sleep(50 * time.Millisecond)
	}
	wantError(t, v1(t, ts, "GET", "/api/v1/panels/nope/preview.png", "", nil), 404, "not_found")
}

func TestV1PatchPanel(t *testing.T) {
	s, ts := testServer(t)
	var before v1Panel
	var ps []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	before = ps[0]
	path := "/api/v1/panels/" + before.ID

	var after v1Panel
	wantStatus(t, v1(t, ts, "PATCH", path, `{"brightness":33}`, &after), 200)
	want := before
	want.Brightness = 33
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("only brightness should change:\n got %+v\nwant %+v", after, want)
	}
	wantStatus(t, v1(t, ts, "PATCH", path, `{"design":"clock","rotation":90,"theme":"light"}`, &after), 200)
	want.Design, want.Rotation, want.Theme = "clock", 90, "light"
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("got %+v want %+v", after, want)
	}
	wantStatus(t, v1(t, ts, "PATCH", path, `{"mode":"off"}`, &after), 200)
	if after.Mode != "off" || after.Design != "clock" || after.Brightness != 33 {
		t.Fatalf("%+v", after)
	}
	wantStatus(t, v1(t, ts, "PATCH", path, `{"mode":"hud"}`, &after), 200)
	if after.Mode != "hud" {
		t.Fatalf("%+v", after)
	}
	// The change is persisted in the panel list, not just echoed.
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	if !reflect.DeepEqual(ps[0], after) {
		t.Fatalf("list %+v differs from %+v", ps[0], after)
	}

	for _, bad := range []string{
		`{"brightness":101}`, `{"brightness":-1}`, `{"rotation":45}`, `{"mode":"design"}`, `{"mode":"media"}`,
		`{"theme":"blue"}`, `{"design":"nonexistent"}`, `{"colour":"red"}`, `not json`,
	} {
		wantError(t, v1(t, ts, "PATCH", path, bad, nil), 400, "bad_request")
	}
	// The media check only applies when the mode is set to media.
	if err := s.Engine.SetPanelConfig(before.ID, app.PanelConfig{Mode: "media", Design: "clock", Brightness: 50}); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "PATCH", path, `{"brightness":44}`, &after), 200)
	wantError(t, v1(t, ts, "PATCH", path, `{"mode":"media"}`, nil), 400, "bad_request")
	v1(t, ts, "PATCH", path, `{"mode":"hud","brightness":33}`, &after)
	// A rejected patch changes nothing.
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	if !reflect.DeepEqual(ps[0], after) {
		t.Fatalf("rejected patches changed the panel: %+v", ps[0])
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/panels/nope", `{"brightness":1}`, nil), 404, "not_found")
}

type fakeClock struct{ ns atomic.Int64 }

func (c *fakeClock) now() time.Time          { return time.Unix(0, c.ns.Load()) }
func (c *fakeClock) advance(d time.Duration) { c.ns.Add(int64(d)) }

func TestV1GamingLease(t *testing.T) {
	clk := &fakeClock{}
	clk.ns.Store(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).UnixNano())
	_, ts := testServerWith(t, func(e *app.Engine) { e.Now = clk.now })
	status := func() v1Status {
		var st v1Status
		v1(t, ts, "GET", "/api/v1/status", "", &st)
		return st
	}
	// The demo sensors report FPS, so the FPS detection can be active too; the
	// hint is what shows as "external".
	external := func() bool { return status().Gaming.Source == "external" }
	if external() {
		t.Fatal("external hint at start")
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/gaming", `{}`, nil), 400, "bad_request")

	var g struct {
		Active bool
		Source string
		Game   string
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/gaming", `{"active":true,"game":"Doom","pid":42,"leaseSeconds":1}`, &g), 200)
	if !g.Active || g.Source != "external" || g.Game != "Doom" {
		t.Fatalf("%+v", g)
	}
	// leaseSeconds is clamped to at least 10 s.
	clk.advance(9 * time.Second)
	if st := status(); !st.Gaming.Active || st.Gaming.Source != "external" || st.Gaming.Game != "Doom" {
		t.Fatalf("hint should still hold after 9 s: %+v", st.Gaming)
	}
	clk.advance(2 * time.Second)
	if external() {
		t.Fatal("lease expired but the hint is still active")
	}
	// active:false ends a hint immediately.
	v1(t, ts, "POST", "/api/v1/gaming", `{"active":true,"leaseSeconds":3600}`, nil)
	if !external() {
		t.Fatal("hint not active")
	}
	v1(t, ts, "POST", "/api/v1/gaming", `{"active":false}`, nil)
	if external() {
		t.Fatal("hint not ended")
	}
	// Default lease is 120 s.
	v1(t, ts, "POST", "/api/v1/gaming", `{"active":true}`, nil)
	clk.advance(119 * time.Second)
	if !external() {
		t.Fatal("default lease shorter than 120 s")
	}
	clk.advance(2 * time.Second)
	if external() {
		t.Fatal("default lease longer than 120 s")
	}
	// FPS detection (the demo sensors report FPS) keeps working, and the hint wins over it.
	// Its hold kept counting under the hint, so a lapsed lease doesn't flicker.
	if st := status(); !st.Gaming.Active || st.Gaming.Source != "fps" || st.Gaming.Game != "" {
		t.Fatalf("fps detection: %+v", st.Gaming)
	}
	v1(t, ts, "POST", "/api/v1/gaming", `{"active":true,"game":"X"}`, nil)
	if st := status(); st.Gaming.Source != "external" {
		t.Fatalf("external should win: %+v", st.Gaming)
	}
}

func TestV1Power(t *testing.T) {
	_, ts := testServer(t)
	var out struct{ Power bool }
	wantStatus(t, v1(t, ts, "POST", "/api/v1/power", `{"on":false}`, &out), 200)
	if out.Power {
		t.Fatal("power should be off")
	}
	var st v1Status
	v1(t, ts, "GET", "/api/v1/status", "", &st)
	if st.Power {
		t.Fatal("status power should be false")
	}
	v1(t, ts, "POST", "/api/v1/power", `{"on":true}`, &out)
	v1(t, ts, "GET", "/api/v1/status", "", &st)
	if !st.Power {
		t.Fatal("power should be on")
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/power", `{}`, nil), 400, "bad_request")
}

func TestV1SetsApply(t *testing.T) {
	_, ts := testServer(t)
	var sets []v1Set
	wantStatus(t, v1(t, ts, "GET", "/api/v1/sets", "", &sets), 200)
	if len(sets) != 8 || sets[0].Set != "clean" || sets[0].Theme != "dark" {
		t.Fatalf("%+v", sets)
	}
	var out struct{ Applied []string }
	wantStatus(t, v1(t, ts, "POST", "/api/v1/sets/apply", `{"set":"neon","theme":"light"}`, &out), 200)
	if len(out.Applied) != 5 {
		t.Fatalf("%+v", out)
	}
	var ps []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	want := []string{"neon-cpu-gpu", "neon-cpu", "neon-gpu", "neon-loop", "neon-cpu"} // pump head: combined; fans: CPU, GPU, coolant, then around again
	for i, p := range ps {
		if p.Design != want[i] || p.Theme != "light" || p.Mode != "hud" {
			t.Errorf("panel %d: %+v, want design %s", i, p, want[i])
		}
	}
	// Only the listed panels; the theme defaults to dark.
	wantStatus(t, v1(t, ts, "POST", "/api/v1/sets/apply", `{"set":"retro","panels":["`+ps[1].ID+`"]}`, &out), 200)
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	if ps[1].Design != "retro-cpu" || ps[1].Theme != "dark" || ps[2].Design != "neon-gpu" {
		t.Fatalf("%+v", ps)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/sets/apply", `{"set":"nope"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/sets/apply", `{"set":"neon","theme":"pink"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/sets/apply", `{"set":"neon","panels":["nope"]}`, nil), 404, "not_found")
}

func TestV1Designs(t *testing.T) {
	_, ts := testServer(t)
	var ds []v1Design
	wantStatus(t, v1(t, ts, "GET", "/api/v1/designs", "", &ds), 200)
	sets := map[string]bool{}
	for _, d := range ds {
		sets[d.Set] = true
		if d.ID == "" || d.W <= 0 || d.H <= 0 || d.Theme != "dark" || !d.Builtin {
			t.Fatalf("design %+v", d)
		}
	}
	for _, s := range []string{"clean", "neon", "retro", "cyberpunk"} {
		if !sets[s] {
			t.Errorf("no design in set %s", s)
		}
	}
	width := func(path string) int {
		resp := v1(t, ts, "GET", path, "", nil)
		wantStatus(t, resp, 200)
		img, err := png.Decode(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return img.Bounds().Dx()
	}
	if w := width("/api/v1/designs/clock/preview.png"); w != 200 {
		t.Errorf("default size %d", w)
	}
	if w := width("/api/v1/designs/clock/preview.png?size=1"); w != 32 {
		t.Errorf("min size %d", w)
	}
	if w := width("/api/v1/designs/clock/preview.png?size=9999&theme=light"); w != 480 {
		t.Errorf("max size %d", w)
	}
	wantError(t, v1(t, ts, "GET", "/api/v1/designs/nope/preview.png", "", nil), 404, "not_found")
	wantError(t, v1(t, ts, "GET", "/api/v1/designs/clock/preview.png?size=x", "", nil), 400, "bad_request")
}

func TestV1ScenesAndRotation(t *testing.T) {
	s, ts := testServer(t)
	var scenes []struct {
		ID, Name string
		Active   bool
	}
	wantStatus(t, v1(t, ts, "GET", "/api/v1/scenes", "", &scenes), 200)
	if scenes == nil || len(scenes) != 0 {
		t.Fatalf("%+v", scenes)
	}
	e := s.Engine
	if err := e.SaveScene("Alpha"); err != nil {
		t.Fatal(err)
	}
	id := e.Panels()[0].ID
	c := e.Panels()[0].Config
	c.Design = "gaming"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveScene("Beta"); err != nil {
		t.Fatal(err)
	}
	// Rotation off: activating applies the scene as the current setup.
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate", "", nil), 204)
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes/Nope/activate", "", nil), 404, "not_found")

	var rot struct{ Enabled bool }
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/rotation", `{"enabled":true}`, &rot), 200)
	if !rot.Enabled {
		t.Fatal("rotation should be enabled")
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/rotation", `{}`, nil), 400, "bad_request")
	waitScene := func(name string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			var st v1Status
			v1(t, ts, "GET", "/api/v1/status", "", &st)
			if st.Rotation.Enabled && st.Rotation.Scene == name {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("scene %q not shown", name)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Beta/activate", "", nil), 204)
	waitScene("Beta")
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate", "", nil), 204)
	waitScene("Alpha")
	v1(t, ts, "GET", "/api/v1/scenes", "", &scenes)
	if len(scenes) != 2 || !scenes[0].Active || scenes[1].Active || scenes[0].ID != "Alpha" {
		t.Fatalf("%+v", scenes)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/rotation/next", "", nil), 204)
	waitScene("Beta")
	v1(t, ts, "PATCH", "/api/v1/rotation", `{"enabled":false}`, &rot)
	if rot.Enabled || e.Rotation().Enabled {
		t.Fatal("rotation should be disabled")
	}
	if len(e.Rotation().Scenes) != 2 {
		t.Fatal("toggling the rotation must keep the scenes")
	}
	// A scene kept out of the rotation can be activated explicitly, too.
	yes := true
	if err := e.SaveSceneWith("Solo", &yes); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Solo/activate", "", nil), 204)
	v1(t, ts, "PATCH", "/api/v1/rotation", `{"enabled":true}`, &rot)
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Solo/activate", "", nil), 204)
}

func TestV1IdentifyAwtrixQuit(t *testing.T) {
	s, ts := testServer(t)
	var ps []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	wantStatus(t, v1(t, ts, "POST", "/api/v1/panels/"+ps[0].ID+"/identify", "", nil), 204)
	wantStatus(t, v1(t, ts, "POST", "/api/v1/identify", "", nil), 204)
	wantError(t, v1(t, ts, "POST", "/api/v1/panels/nope/identify", "", nil), 404, "not_found")

	var aw struct{ Enabled bool }
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/awtrix", `{"enabled":false}`, &aw), 200)
	wantError(t, v1(t, ts, "PATCH", "/api/v1/awtrix", `{}`, nil), 400, "bad_request")

	wantError(t, v1(t, ts, "POST", "/api/v1/quit", "", nil), 500, "internal")
	quit := make(chan struct{})
	s.OnQuit = func() { close(quit) }
	wantStatus(t, v1(t, ts, "POST", "/api/v1/quit", "", nil), 202)
	select {
	case <-quit:
	case <-time.After(3 * time.Second):
		t.Fatal("OnQuit not called")
	}
}

func TestV1GUIOpen(t *testing.T) {
	s, ts := testServer(t)
	wantError(t, v1(t, ts, "POST", "/api/v1/gui/open", "", nil), 503, "unavailable")
	shown := 0
	s.OnShow = func() error { shown++; return nil }
	wantStatus(t, v1(t, ts, "POST", "/api/v1/gui/open", "", nil), 202)
	if shown != 1 {
		t.Fatalf("OnShow called %d times", shown)
	}
	s.OnShow = func() error { return errors.New("no window appeared") }
	wantError(t, v1(t, ts, "POST", "/api/v1/gui/open", "", nil), 503, "unavailable")
	r := v1(t, ts, "GET", "/api/v1/gui/open", "", nil)
	wantError(t, r, 405, "method_not_allowed")
	if r.Header.Get("Allow") != "POST" {
		t.Fatalf("Allow %q", r.Header.Get("Allow"))
	}
}

func TestV1ErrorFormat(t *testing.T) {
	_, ts := testServer(t)
	wantError(t, v1(t, ts, "GET", "/api/v1/nothing", "", nil), 404, "not_found")
	resp405 := v1(t, ts, "DELETE", "/api/v1/status", "", nil)
	wantError(t, resp405, 405, "method_not_allowed")
	if resp405.Header.Get("Allow") != "GET" {
		t.Fatalf("Allow %q", resp405.Header.Get("Allow"))
	}
	wantError(t, v1(t, ts, "GET", "/api/v1/panels/x", "", nil), 405, "method_not_allowed")
	resp := v1(t, ts, "PATCH", "/api/v1/rotation", `{"enabled":"x"}`, nil)
	wantError(t, resp, 400, "bad_request")
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
}

func TestV1Events(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/events", nil)
	req.Header.Set("X-Token", "client")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var event, data string
	for sc.Scan() && data == "" {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "event: "); ok {
			event = v
		}
		if v, ok := strings.CutPrefix(line, "data: "); ok {
			data = v
		}
	}
	var st v1Status
	if event != "status" || json.Unmarshal([]byte(data), &st) != nil || st.APIVersion != 1 || len(st.Panels) != 5 {
		t.Fatalf("event %q data %q", event, data)
	}

	// A change produces a new status event.
	go func() {
		time.Sleep(100 * time.Millisecond)
		v1(t, ts, "POST", "/api/v1/power", `{"on":false}`, nil)
	}()
	deadline := time.AfterFunc(5*time.Second, cancel)
	defer deadline.Stop()
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			var next v1Status
			json.Unmarshal([]byte(v), &next)
			if !next.Power {
				return
			}
		}
	}
	t.Fatal("no status event after the change")
}

func TestAPIFile(t *testing.T) {
	dir := t.TempDir()
	f := APIFile{APIVersion: 1, Version: "1.2.3", Port: 4321, Token: "tok", PID: 77, Exe: `C:\x\CaseHUD.exe`}
	if err := WriteAPIFile(dir, f); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, APIFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]any{"apiVersion": 1.0, "version": "1.2.3", "port": 4321.0, "token": "tok", "pid": 77.0, "exe": `C:\x\CaseHUD.exe`} {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	if len(m) != 6 {
		t.Errorf("unexpected fields: %v", m)
	}
	if fi, _ := os.Stat(p); os.PathSeparator == '/' && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
	// Stale temporary files from a crashed start are cleaned up.
	stale := filepath.Join(dir, APIFileName+".123.tmp")
	os.WriteFile(stale, []byte("x"), 0o600)
	CleanAPIFiles(dir)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale tmp file kept")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("CleanAPIFiles removed api.json")
	}
	// Overwriting leaves no temporary files behind.
	f.Port = 1
	if err := WriteAPIFile(dir, f); err != nil {
		t.Fatal(err)
	}
	if es, _ := os.ReadDir(dir); len(es) != 1 {
		t.Fatalf("files after rewrite: %v", es)
	}
	// Another process' file is left alone; our own is removed.
	RemoveAPIFile(dir, 99)
	if _, err := os.Stat(p); err != nil {
		t.Fatal("foreign api.json removed")
	}
	RemoveAPIFile(dir, 77)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("api.json not removed")
	}
	RemoveAPIFile(dir, 77) // missing file is fine
	if err := WriteAPIFile(filepath.Join(dir, "missing"), f); err == nil {
		t.Fatal("expected error for missing folder")
	}
}

func TestServerTokens(t *testing.T) {
	e, ts := testServer(t)
	_ = ts
	s := &Server{Engine: e.Engine, Version: "9"}
	if _, err := s.Listen(0); err != nil {
		t.Fatal(err)
	}
	defer s.ln.Close()
	if len(s.clientToken) != 32 || len(s.token) != 32 || s.clientToken == s.token {
		t.Fatalf("tokens %q %q", s.token, s.clientToken)
	}
	info := s.APIInfo(5, "x.exe")
	if info.Port != s.port() || info.Port == 0 || info.Token != s.clientToken || info.Version != "9" || info.APIVersion != 1 || info.PID != 5 {
		t.Fatalf("%+v", info)
	}
}

func TestV1EventClientCap(t *testing.T) {
	_, ts := testServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	open := func() *http.Response {
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/events", nil)
		req.Header.Set("X-Token", "client")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	for i := 0; i < maxEventClients; i++ {
		resp := open()
		if resp.StatusCode != 200 {
			t.Fatalf("stream %d: status %d", i, resp.StatusCode)
		}
	}
	wantError(t, open(), 503, "too_many_clients")
}

func TestV1GridPatch(t *testing.T) {
	t.Setenv("CASEHUD_DEMO_BAR", "1")
	_, ts := testServer(t)
	path := "/api/v1/panels/us88-0"
	var p v1Panel
	wantStatus(t, v1(t, ts, "GET", "/api/v1/panels", "", &[]v1Panel{}), 200)
	var all []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &all)
	for _, x := range all {
		if x.ID == "us88-0" {
			p = x
		}
	}
	if p.Grid != "4" || len(p.Tiles) != 4 || p.W != 1920 {
		t.Fatalf("default bar should be the Overview grid: %+v", p)
	}
	// Old-style PATCH bodies keep working: brightness leaves the grid alone.
	var after v1Panel
	wantStatus(t, v1(t, ts, "PATCH", path, `{"brightness":40}`, &after), 200)
	if after.Grid != "4" || after.Brightness != 40 {
		t.Fatalf("%+v", after)
	}
	// Grid and tiles together.
	wantStatus(t, v1(t, ts, "PATCH", path, `{"grid":"2","tiles":[{"mode":"hud","design":"w960-cpu","background":"aurora"},{"design":"w960-gpu","theme":"light"}]}`, &after), 200)
	if after.Grid != "2" || len(after.Tiles) != 2 || after.Tiles[0].Background != "aurora" || after.Tiles[1].Theme != "light" || after.Tiles[1].Mode != "hud" {
		t.Fatalf("%+v", after)
	}
	// Wrong tile count, unknown grid, unknown design, a new grid without tiles.
	wantError(t, v1(t, ts, "PATCH", path, `{"grid":"4","tiles":[{"design":"clock"}]}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", path, `{"grid":"9","tiles":[]}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", path, `{"grid":"2","tiles":[{"design":"nope"},{"design":"clock"}]}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", path, `{"grid":"3"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/panels/tllcd-p1-i0", `{"grid":"2","tiles":[{"design":"clock"},{"design":"clock"}]}`, nil), 400, "bad_request")
	// Tiles of the same grid can be replaced alone.
	wantStatus(t, v1(t, ts, "PATCH", path, `{"tiles":[{"design":"half-cpu"},{"design":"half-gpu"}]}`, &after), 200)
	if after.Tiles[0].Design != "half-cpu" {
		t.Fatalf("%+v", after)
	}
	// A single design replaces the grid.
	wantStatus(t, v1(t, ts, "PATCH", path, `{"design":"water-wide"}`, &after), 200)
	if after.Grid != "" || len(after.Tiles) != 0 || after.Design != "water-wide" {
		t.Fatalf("design must clear the grid: %+v", after)
	}
	// Rotation upright turns the canvas.
	wantStatus(t, v1(t, ts, "PATCH", path, `{"rotation":90}`, &after), 200)
	if after.W != 480 || after.H != 1920 {
		t.Fatalf("upright bar is %dx%d", after.W, after.H)
	}
}

func TestV1Automation(t *testing.T) {
	_, ts := testServer(t)
	var st v1Status
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.Automation.Enabled || st.Automation.ActiveRule != -1 {
		t.Fatalf("%+v", st.Automation)
	}
	var a struct{ Enabled bool }
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/automation", `{"enabled":true}`, &a), 200)
	if !a.Enabled {
		t.Fatal("automation should be enabled")
	}
	wantError(t, v1(t, ts, "PATCH", "/api/v1/automation", `{}`, nil), 400, "bad_request")
	v1(t, ts, "GET", "/api/v1/status", "", &st)
	if !st.Automation.Enabled {
		t.Fatalf("%+v", st.Automation)
	}
}

func TestBodyLimitsAndStrictAutomation(t *testing.T) {
	_, ts := testServer(t)
	big := `{"enabled":true,"pad":"` + strings.Repeat("x", 1<<20+10) + `"}`
	wantError(t, v1(t, ts, "PATCH", "/api/v1/rotation", big, nil), 413, "payload_too_large")

	// legacy endpoints: over the limit is refused, too
	req, _ := http.NewRequest("PUT", ts.URL+"/api/automation", strings.NewReader(`{"enabled":false,"pad":"`+strings.Repeat("x", 1<<20+10)+`"}`))
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("legacy oversize: status %d", resp.StatusCode)
	}
	// unknown fields in the automation body are refused
	req, _ = http.NewRequest("PUT", ts.URL+"/api/automation", strings.NewReader(`{"enabled":false,"typo":1}`))
	req.Header.Set("X-Token", "secret")
	if resp, _ = http.DefaultClient.Do(req); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field: status %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("PUT", ts.URL+"/api/automation", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("X-Token", "secret")
	if resp, _ = http.DefaultClient.Do(req); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("valid automation: status %d", resp.StatusCode)
	}
}

func TestV1ActivateSceneTransientByDefault(t *testing.T) {
	s, ts := testServer(t)
	e := s.Engine
	if err := e.SaveScene("Alpha"); err != nil {
		t.Fatal(err)
	}
	id := e.Panels()[0].ID
	alpha := e.Panels()[0].Config.Design
	c := e.Panels()[0].Config
	c.Design = "gaming"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate", "", nil), 204)
	if got := e.Panels()[0].Config.Design; got != "gaming" {
		t.Fatalf("activate changed the saved setup: %q", got)
	}
	var st v1Status
	wantStatus(t, v1(t, ts, "GET", "/api/v1/status", "", &st), 200)
	if st.Scene != "Alpha" {
		t.Fatalf("status.scene after a manual activation: %q, want Alpha", st.Scene)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate?persist=true", "", nil), 204)
	if got := e.Panels()[0].Config.Design; got != alpha {
		t.Fatalf("persist: %q, want %q", got, alpha)
	}
	c.Design = "gaming"
	e.SetPanelConfig(id, c)
	wantStatus(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate", `{"persist":true}`, nil), 204)
	if got := e.Panels()[0].Config.Design; got != alpha {
		t.Fatalf("persist via body: %q, want %q", got, alpha)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/scenes/Alpha/activate", `{"persist":true,"x":1}`, nil), 400, "bad_request")
}

func TestV1Layouts(t *testing.T) {
	s, ts := testServer(t)
	var ls []v1Layout
	wantStatus(t, v1(t, ts, "GET", "/api/v1/layouts", "", &ls), 200)
	builtin := 0
	for _, l := range ls {
		if l.Builtin && l.Tpl != "" && l.Set != "" && len(l.Tiles) > 0 {
			builtin++
		}
	}
	if builtin != 16 {
		t.Fatalf("%d ready-made layouts: %+v", builtin, ls)
	}
	var ps []v1Panel
	v1(t, ts, "GET", "/api/v1/panels", "", &ps)
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+ps[0].ID, `{"layout":"nope"}`, nil), 400)
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/panels/"+ps[0].ID, `{"layout":"../x"}`, nil), 400)

	// the GUI endpoints refuse ids that are not plain names
	put := func(id string) int {
		req, _ := http.NewRequest("PUT", ts.URL+"/api/layouts/"+id+"?t=secret", strings.NewReader(`{"name":"x","grid":"4","tiles":[{},{},{},{}]}`))
		req.Header.Set("X-Token", "secret")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	for _, id := range []string{"..%2Fevil", "a%5Cb", "Big", "gaming-clean"} {
		if c := put(id); c < 400 {
			t.Errorf("PUT layouts/%s: %d", id, c)
		}
	}
	if files, _ := filepath.Glob(filepath.Join(s.Engine.Store.Dir, "layouts", "*")); len(files) != 0 {
		t.Errorf("files written: %v", files)
	}
}

func TestGUIAPIChecksHostHeader(t *testing.T) {
	_, ts := testServer(t)
	get := func(path, host string) int {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		req.Header.Set("X-Token", "secret")
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := get("/api/state", ""); got != 200 {
		t.Fatalf("own host: %d", got)
	}
	if got := get("/api/state", "evil.example:80"); got != 403 {
		t.Fatalf("rebound host on /api/state: %d, want 403", got)
	}
	if got := get("/api/lighting", "evil.example:80"); got != 403 {
		t.Fatalf("rebound host on /api/lighting: %d, want 403", got)
	}
}

func TestLiveStreamsAreCapped(t *testing.T) {
	s, ts := testServer(t)
	s.liveClients.Store(maxLiveClients)
	req, _ := http.NewRequest("GET", ts.URL+"/api/live", nil)
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", resp.StatusCode)
	}
	if s.liveClients.Load() != maxLiveClients {
		t.Fatal("counter changed by a refused client")
	}
}
