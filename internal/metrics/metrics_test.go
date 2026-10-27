package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleLHM = `{"id":0,"Text":"Sensor","Value":"","Children":[
 {"id":1,"Text":"GAMING-PC","Value":"","Children":[
  {"id":2,"Text":"AMD Ryzen 7 7800X3D","Value":"","Children":[
   {"id":3,"Text":"Temperatures","Value":"","Children":[
    {"id":4,"Text":"Core (Tctl/Tdie)","Value":"61,4 °C","SensorId":"/amdcpu/0/temperature/2","Children":[]}]},
   {"id":5,"Text":"Load","Value":"","Children":[
    {"id":6,"Text":"CPU Total","Value":"23.5 %","SensorId":"/amdcpu/0/load/0","Children":[]}]}]},
  {"id":7,"Text":"NVIDIA GeForce RTX 4080","Value":"","Children":[
   {"id":8,"Text":"Temperatures","Value":"","Children":[
    {"id":9,"Text":"GPU Core","Value":"54.0 °C","SensorId":"/gpu-nvidia/0/temperature/0","Children":[]}]}]},
  {"id":10,"Text":"ASUS ROG STRIX X670E","Value":"","Children":[
   {"id":11,"Text":"Nuvoton NCT6799D","Value":"","Children":[
    {"id":12,"Text":"Fans","Value":"","Children":[
     {"id":13,"Text":"Pump","Value":"2150 RPM","Children":[]},
     {"id":14,"Text":"Fan #2","Value":"-","Children":[]}]}]}]}]}]}`

func TestFlattenAndMatch(t *testing.T) {
	var root lhmNode
	if err := json.Unmarshal([]byte(sampleLHM), &root); err != nil {
		t.Fatal(err)
	}
	rs := flattenLHM(root)
	if len(rs) != 4 {
		t.Fatalf("got %d readings, want 4: %+v", len(rs), rs)
	}

	cases := []struct {
		src  string
		want float64
		unit string
	}{
		{"/gpu-nvidia/0/temperature/0", 54, "°C"},
		{"Temperatures/Core (Tctl/Tdie)", 61.4, "°C"},
		{"Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)", 61.4, "°C"},
		{"load/cpu total", 23.5, "%"},
		{"ASUS ROG STRIX X670E/Nuvoton NCT6799D/Fans/Pump", 2150, "RPM"},
	}
	for _, c := range cases {
		r, ok := match(rs, c.src)
		if !ok || r.Value != c.want || r.Unit != c.unit {
			t.Errorf("match(%q) = %+v, %v; want %v %s", c.src, r, ok, c.want, c.unit)
		}
	}
	if _, ok := match(rs, "Fans/Fan #2"); ok {
		t.Error("sensor without value should not resolve")
	}
}

func TestHistory(t *testing.T) {
	h := NewHistory(3)
	for i := 1; i <= 5; i++ {
		h.Push("x", float64(i))
	}
	got := h.Get("x")
	if len(got) != 3 || got[0] != 3 || got[2] != 5 {
		t.Fatalf("history = %v", got)
	}
}

func TestPreferDedicatedGPU(t *testing.T) {
	rs := []Reading{
		{Key: "/gpu-amd/0/load/0", Path: "AMD Radeon(TM) Graphics/Load/GPU Core", Value: 3},
		{Key: "/gpu-nvidia/0/load/0", Path: "NVIDIA GeForce RTX 4080/Load/GPU Core", Value: 97},
	}
	if r, ok := match(rs, "Load/GPU Core"); !ok || r.Value != 97 {
		t.Fatalf("got %+v, want the dedicated GPU", r)
	}
	if r, ok := match(rs[:1], "Load/GPU Core"); !ok || r.Value != 3 {
		t.Fatalf("iGPU-only system should still resolve, got %+v", r)
	}
}

func TestPickRTSSApp(t *testing.T) {
	apps := []rtssApp{
		{pid: 0},
		{pid: 10, time0: 9000, time1: 10000, frames: 60, ftime: 16667}, // background game, 60 fps
		{pid: 20, time0: 9500, time1: 10000, frames: 72, ftime: 6944},  // foreground, 144 fps
		{pid: 30, time0: 1000, time1: 2000, frames: 99},                // stale
	}
	fps, ft, ok := pickRTSSApp(apps, 20, 10100)
	if !ok || fps != 144 || ft < 6.9 || ft > 7 {
		t.Fatalf("foreground: fps=%v ft=%v ok=%v", fps, ft, ok)
	}
	fps, _, ok = pickRTSSApp(apps, 999, 10100)
	if !ok || fps != 60 {
		t.Fatalf("fallback to the most recent app: fps=%v ok=%v", fps, ok)
	}
	if _, _, ok := pickRTSSApp(apps[3:], 30, 10100); ok {
		t.Fatal("stale entries must be ignored")
	}
}

func TestPresentMonStats(t *testing.T) {
	// 1 s of a game at 120 displayed FPS with 2x frame generation and 30 ms
	// PC latency, plus dwm.exe noise.
	var b strings.Builder
	b.WriteString("Application,ProcessID,FrameType,CPUStartTime,FrameTime,DisplayedTime,MsPCLatency\n")
	for i := 0; i < 120; i++ {
		ft := "Application"
		if i%2 == 1 {
			ft = "NVIDIA"
		}
		fmt.Fprintf(&b, "game.exe,42,%s,%.3f,8.333,8.333,30\n", ft, float64(i)*8.333)
		fmt.Fprintf(&b, "dwm.exe,7,Application,%.3f,16.6,16.6,NA\n", float64(i)*8.333)
	}
	s := newPMStats()
	clock := time.Unix(1000, 0)
	s.nowFn = func() time.Time { return clock }
	s.consume(strings.NewReader(b.String()))
	rs := s.readings(pmFg{PID: 42})
	get := func(path string) float64 {
		r, ok := match(rs, path)
		if !ok {
			t.Fatalf("%s missing in %+v", path, rs)
		}
		return r.Value
	}
	if fps := get("FPS/Framerate"); fps < 119 || fps > 121 {
		t.Errorf("fps = %v", fps)
	}
	if n := get("FPS/Native framerate"); n < 59 || n > 61 {
		t.Errorf("native = %v", n)
	}
	if fg := get("FPS/Frame generation"); fg != 2 {
		t.Errorf("fg = %v", fg)
	}
	if l := get("FPS/Latency"); l != 30 {
		t.Errorf("latency = %v", l)
	}
	// A background process never counts, even if it is the only one presenting
	// (once the grace period of the game that was in front is over).
	clock = clock.Add(pmGameGrace + time.Second)
	if rs := s.readings(pmFg{PID: 999}); len(rs) != 0 || s.game != "" {
		t.Errorf("background process counted: %+v", rs)
	}
}

func TestPresentMonV1Columns(t *testing.T) {
	csvData := "Application,ProcessID,TimeInSeconds,MsBetweenPresents,MsBetweenDisplayChange\n" +
		"game.exe,5,0.000,10,10\ngame.exe,5,0.010,10,10\ngame.exe,5,0.020,10,10\n"
	s := newPMStats()
	s.consume(strings.NewReader(csvData))
	r, ok := match(s.readings(pmFg{PID: 5, Fullscreen: true}), "FPS/Framerate")
	if !ok || r.Value != 100 {
		t.Fatalf("v1 fps = %+v %v", r, ok)
	}
	if _, ok := match(s.readings(pmFg{PID: 5, Fullscreen: true}), "FPS/Frame generation"); ok {
		t.Error("no FrameType column: no frame generation reading expected")
	}
}

func TestLHMFallsBackToSecondURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleLHM))
	}))
	defer srv.Close()
	l := NewLHM("")
	l.URLs = []string{"http://127.0.0.1:1/data.json", srv.URL}
	if err := l.Refresh(); err != nil {
		t.Fatal(err)
	}
	if l.Source() != srv.URL || len(l.All()) == 0 {
		t.Fatalf("source=%s readings=%d", l.Source(), len(l.All()))
	}
}

// pmCSV builds PresentMon CSV for one process: n frames, ms apart.
func pmCSV(app string, pid, n int, ms float64, mode string) string {
	var b strings.Builder
	b.WriteString("Application,ProcessID,PresentMode,CPUStartTime,FrameTime,DisplayedTime\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s,%d,%s,%.3f,%.3f,%.3f\n", app, pid, mode, float64(i)*ms, ms, ms)
	}
	return b.String()
}

func TestPresentMonGameHeuristic(t *testing.T) {
	flip := "Hardware: Independent Flip"
	cases := []struct {
		name string
		csv  string
		fg   pmFg
		want bool
	}{
		{"fullscreen game", pmCSV("game.exe", 1, 60, 16.7, flip), pmFg{PID: 1, Fullscreen: true}, true},
		{"windowed flip game at 60 fps", pmCSV("game.exe", 1, 60, 16.7, "Composed: Flip"), pmFg{PID: 1}, true},
		{"windowed low rate", pmCSV("tool.exe", 1, 60, 100, "Composed: Flip"), pmFg{PID: 1}, false},
		{"windowed too few frames", pmCSV("tool.exe", 1, 5, 8, "Composed: Flip"), pmFg{PID: 1}, false},
		{"windowed copy path", pmCSV("tool.exe", 1, 60, 8, "Composed: Copy with GPU GDI"), pmFg{PID: 1}, false},
		{"terminal in front", pmCSV("WindowsTerminal.exe", 1, 60, 8, flip), pmFg{PID: 1}, false},
		{"fullscreen browser", pmCSV("chrome.exe", 1, 60, 8, flip), pmFg{PID: 1, Fullscreen: true}, false},
		{"not the foreground process", pmCSV("game.exe", 1, 60, 8, flip), pmFg{PID: 2, Fullscreen: true}, false},
	}
	for _, c := range cases {
		s := newPMStats()
		s.window = time.Hour // keep every test frame
		s.consume(strings.NewReader(c.csv))
		rs := s.readings(c.fg)
		if got := len(rs) > 0; got != c.want {
			t.Errorf("%s: game=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestPresentMonUserIgnore(t *testing.T) {
	s := newPMStats()
	s.consume(strings.NewReader(pmCSV("Slack2.exe", 1, 60, 8, "Hardware: Independent Flip")))
	fg := pmFg{PID: 1, Fullscreen: true}
	if len(s.readings(fg)) == 0 {
		t.Fatal("unknown program in front should count as a game")
	}
	s.setIgnore([]string{" SLACK2.exe "})
	if rs := s.readings(fg); len(rs) != 0 || s.game != "" {
		t.Fatalf("ignored program still counted: %+v", rs)
	}
	// New frames of an ignored program are dropped too.
	s.consume(strings.NewReader(pmCSV("slack2.exe", 1, 60, 8, "Hardware: Independent Flip")))
	if len(s.readings(fg)) != 0 {
		t.Fatal("ignored program counted after new frames")
	}
	s.setIgnore(nil)
	s.consume(strings.NewReader(pmCSV("Slack2.exe", 1, 60, 8, "Hardware: Independent Flip")))
	if len(s.readings(fg)) == 0 {
		t.Fatal("removing it from the list should bring detection back")
	}
}

func TestPresentMonGameGraceAfterFocusLoss(t *testing.T) {
	s := newPMStats()
	clock := time.Unix(1000, 0)
	s.nowFn = func() time.Time { return clock }
	s.consume(strings.NewReader(pmCSV("game.exe", 1, 60, 8, "Hardware: Independent Flip")))
	if len(s.readings(pmFg{PID: 1, Fullscreen: true})) == 0 {
		t.Fatal("game not detected")
	}
	// the user clicks into a window on the other monitor
	clock = clock.Add(5 * time.Second)
	if len(s.readings(pmFg{PID: 77})) == 0 || s.game != "game.exe" {
		t.Error("game dropped right after losing the focus")
	}
	clock = clock.Add(11 * time.Second) // 16 s after the last time it was in front
	if rs := s.readings(pmFg{PID: 77}); len(rs) != 0 || s.game != "" {
		t.Errorf("game still reported after the grace period: %+v", rs)
	}
	// and a game that never was in front is not picked up
	s2 := newPMStats()
	s2.nowFn = func() time.Time { return clock }
	s2.consume(strings.NewReader(pmCSV("game.exe", 1, 60, 8, "Hardware: Independent Flip")))
	if len(s2.readings(pmFg{PID: 77})) != 0 {
		t.Error("background process counted without ever being in front")
	}
}

func TestCoversMonitor(t *testing.T) {
	mon := pmRect{0, 0, 2560, 1440}
	const caption, thick = 0x00C00000, 0x00040000
	cases := []struct {
		name   string
		w      pmRect
		style  uint32
		zoomed bool
		want   bool
	}{
		{"borderless popup", pmRect{0, 0, 2560, 1440}, 0x80000000, false, true},
		{"exclusive fullscreen", pmRect{0, 0, 2560, 1440}, 0, false, true},
		{"maximized captioned window", pmRect{-8, -8, 2568, 1448}, caption | thick | 0x01000000, true, false},
		{"captioned window covering everything", pmRect{0, 0, 2560, 1440}, caption, false, false},
		{"zoomed resizable without caption", pmRect{0, 0, 2560, 1440}, thick, true, false},
		{"smaller window", pmRect{100, 100, 1300, 900}, 0x80000000, false, false},
	}
	for _, c := range cases {
		if got := coversMonitor(c.w, mon, c.style, c.zoomed); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
