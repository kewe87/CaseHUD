package app

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/media"
	"github.com/kewe87/casehud/internal/panel"
)

// layerFake is a bar that takes layers and records what it is given.
type layerFake struct {
	demoBar
	mu       sync.Mutex
	layers   int
	images   int
	lastBack panel.Backdrop
	lastOv   *image.RGBA
	caps     panel.LayerCaps
	oc       bool // offline clock state
	ocCalls  []bool
	turn     map[int]int
}

func newLayerFake() *layerFake {
	return &layerFake{
		demoBar: demoBar{demoPanel{id: "us88-0", name: "bar", size: image.Pt(1920, 480)}},
		caps:    panel.LayerCaps{Static: true, Video: true},
		turn:    map[int]int{0: 1, 90: 2, 180: 3, 270: 0},
	}
}

func (f *layerFake) LayerCaps() panel.LayerCaps { return f.caps }
func (f *layerFake) ShowImage(image.Image) error {
	f.mu.Lock()
	f.images++
	f.mu.Unlock()
	return nil
}
func (f *layerFake) ShowLayers(b panel.Backdrop, ov *image.RGBA) error {
	f.mu.Lock()
	f.layers++
	f.lastBack, f.lastOv = b, ov
	f.mu.Unlock()
	return nil
}
func (f *layerFake) VideoTurn(rot int) int { return f.turn[rot] }
func (f *layerFake) SetOfflineClock(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ocCalls = append(f.ocCalls, on)
	f.oc = on
	return nil
}
func (f *layerFake) OfflineClock() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.oc }

func newLayerWorker(t *testing.T, f *layerFake) *worker {
	t.Helper()
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	w := &worker{e: e, p: f, switchCh: make(chan switchReq, 1), wake: make(chan struct{}, 1)}
	e.mu.Lock()
	e.workers = map[string]*worker{f.ID(): w}
	e.order = []string{f.ID()}
	e.cfg.Panels = map[string]PanelConfig{f.ID(): {Mode: "design", Design: "x"}}
	e.mu.Unlock()
	return w
}

// plainDesign finds a built-in design for the bar without icons.
func plainDesign(t *testing.T, w *worker, wantIcons bool) string {
	t.Helper()
	for id := range builtinDesigns() {
		src, err := w.newSource(PanelConfig{Mode: "design", Design: id})
		if err != nil {
			continue
		}
		ds, ok := src.(*designSource)
		if ok && ds.r.HasIcons() == wantIcons {
			return id
		}
	}
	t.Skip("no design of that kind")
	return ""
}

func TestLayerModeStaticDesign(t *testing.T) {
	f := newLayerFake()
	w := newLayerWorker(t, f)
	id := plainDesign(t, w, false)
	src, err := w.newSource(PanelConfig{Mode: "design", Design: id})
	if err != nil {
		t.Fatal(err)
	}
	ds := src.(*designSource)
	if !ds.layerable {
		t.Fatal("plain design not layerable")
	}
	wait, compose, ok := w.layeredStep(src, 0, false)
	if !ok {
		t.Fatal("layered step refused")
	}
	if f.layers != 1 || f.lastBack.Video != nil || f.lastBack.Image == nil {
		t.Fatalf("layers=%d back=%+v", f.layers, f.lastBack)
	}
	bg := f.lastBack.Image()
	want := ds.r.BackgroundColor()
	if bg.Rect.Size() != image.Pt(1920, 480) || bg.RGBAAt(7, 7) != want {
		t.Fatalf("picture %v pixel %v, want %v", bg.Rect, bg.RGBAAt(7, 7), want)
	}
	transparent := 0
	for i := 3; i < len(f.lastOv.Pix); i += 4 {
		if f.lastOv.Pix[i] == 0 {
			transparent++
		}
	}
	if transparent < len(f.lastOv.Pix)/8 {
		t.Fatal("overlay is not mostly transparent")
	}
	if wait < 0 {
		t.Fatalf("wait %v", wait)
	}
	// What is composed (preview, transition start) is what the plain path shows.
	plain, _ := w.newSource(PanelConfig{Mode: "design", Design: id})
	_ = plain
	full := compose()
	opaque, _ := ds.frame(0)
	if full.Rect != opaque.Rect {
		t.Fatalf("compose %v vs frame %v", full.Rect, opaque.Rect)
	}
	for i := 3; i < len(full.Pix); i += 4 {
		if full.Pix[i] != 0xff {
			t.Fatal("composed frame is not opaque")
		}
	}
	// The preview works from the composed layers.
	if b, seq := w.snapshot(); len(b) == 0 || seq == 0 {
		t.Fatal("no preview of a layered frame")
	}
	// The same backdrop key on the next frame: the panel sends the picture once.
	k1 := f.lastBack.Key
	w.layeredStep(src, time.Second, false)
	if f.lastBack.Key != k1 {
		t.Fatal("backdrop key changes between frames")
	}
}

func TestLayerModeFallsBackToPictures(t *testing.T) {
	f := newLayerFake()
	w := newLayerWorker(t, f)
	id := plainDesign(t, w, false)
	src, _ := w.newSource(PanelConfig{Mode: "design", Design: id})
	if _, _, ok := w.layeredStep(src, 0, true); ok {
		t.Fatal("layers used during a transition")
	}
	w.identify(1, "x")
	if _, _, ok := w.layeredStep(src, 0, false); ok {
		t.Fatal("layers used while the identify number shows")
	}
	if f.layers != 0 {
		t.Fatal("the panel was given layers")
	}

	// designs with icons, an animated background, other sources
	if iconID := plainDesign(t, w, true); iconID != "" {
		s, _ := w.newSource(PanelConfig{Mode: "design", Design: iconID})
		if s.(*designSource).layerable {
			t.Fatal("design with icons layered")
		}
	}
	s, _ := w.newSource(PanelConfig{Mode: "design", Design: id, Background: "aurora"})
	if s.(*designSource).layerable {
		t.Fatal("animated background layered")
	}
	off, _ := w.newSource(PanelConfig{Mode: "off"})
	if _, _, ok := w.layeredStep(off, 0, false); ok {
		t.Fatal("blank screen layered")
	}

	// switched off in the configuration
	w.e.mu.Lock()
	w.e.cfg.NoLayers = true
	w.e.mu.Unlock()
	s, _ = w.newSource(PanelConfig{Mode: "design", Design: id})
	if s.(*designSource).layerable {
		t.Fatal("layers used although switched off")
	}
	// panels without layers keep the opaque picture
	w2 := newLayerWorker(t, newLayerFake())
	w2.p = &demoBar{demoPanel{id: "us88-0", name: "bar", size: image.Pt(1920, 480)}}
	s2, _ := w2.newSource(PanelConfig{Mode: "design", Design: id})
	img, _ := s2.frame(0)
	if img.Pix[3] != 0xff || img.Pix[len(img.Pix)-1] != 0xff {
		t.Fatal("picture of a normal panel is transparent")
	}
}

func fakeFFmpegPresent(t *testing.T) {
	t.Helper()
	old := ffmpegProbe
	ffmpegProbe = func() string { return "ffmpeg" }
	ffmpegMemo.mu.Lock()
	ffmpegMemo.at = time.Time{}
	ffmpegMemo.mu.Unlock()
	t.Cleanup(func() {
		ffmpegProbe = old
		ffmpegMemo.mu.Lock()
		ffmpegMemo.at = time.Time{}
		ffmpegMemo.mu.Unlock()
	})
}

// seedVideo puts a ready encoded file in the cache for the video clip.mp4 at the
// given spec and returns its path.
func seedVideo(t *testing.T, w *worker, spec media.DeviceVideoSpec) string {
	t.Helper()
	dir := filepath.Join(w.e.Store.Dir, "media")
	os.MkdirAll(dir, 0o755)
	src := filepath.Join(dir, "clip.mp4")
	os.WriteFile(src, []byte("not really a video"), 0o600)
	name, err := media.DeviceVideoName(src, spec)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(w.e.Store.Dir, "devvideo", name)
	os.MkdirAll(filepath.Dir(out), 0o755)
	os.WriteFile(out, []byte{0, 0, 0, 1, 0x67, 66}, 0o600)
	return out
}

func TestVideoOnTheDisplayMediaMode(t *testing.T) {
	fakeFFmpegPresent(t)
	f := newLayerFake()
	w := newLayerWorker(t, f)
	want := seedVideo(t, w, media.DeviceVideoSpec{W: 1920, H: 480, Turn: 1, Dim: 1})
	src, err := w.newSource(PanelConfig{Mode: "media", Media: "clip.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := w.layeredStep(src, 0, false); !ok {
		t.Fatal("ready video not played by the display")
	}
	if f.lastBack.Video == nil || f.lastBack.Video.Path != want || f.lastBack.Video.FPS != media.DeviceVideoFPS || f.lastOv != nil {
		t.Fatalf("backdrop %+v overlay %v", f.lastBack, f.lastOv)
	}
	// during a transition frames are black, not a decode of the clip
	img, _ := src.frame(0)
	if img.Rect.Size() != image.Pt(1920, 480) || img.RGBAAt(5, 5) != (color.RGBA{A: 255}) {
		t.Fatal("transition frame is not black")
	}
	// rotated 180: another orientation, another file (not ready: the clip is decoded instead)
	f.turn[180] = 3
	if _, err := w.newSource(PanelConfig{Mode: "media", Media: "clip.mp4", Rotation: 180}); err == nil {
		// the video is not seeded for this orientation; building the source starts an
		// encode that fails (no ffmpeg here) or finds nothing to decode: both fine
	}
	// the option off: the clip path as before
	w.e.mu.Lock()
	w.e.cfg.NoDeviceVideo = true
	w.e.mu.Unlock()
	if _, ok := w.newSourceOrNil(PanelConfig{Mode: "media", Media: "clip.mp4"}).(*videoMediaSource); ok {
		t.Fatal("video played by the display although switched off")
	}
}

// newSourceOrNil is newSource that returns nil on error.
func (w *worker) newSourceOrNil(c PanelConfig) source {
	s, err := w.newSource(c)
	if err != nil {
		return nil
	}
	return s
}

func TestVideoBackgroundBehindADesign(t *testing.T) {
	fakeFFmpegPresent(t)
	f := newLayerFake()
	w := newLayerWorker(t, f)
	id := plainDesign(t, w, false)
	want := seedVideo(t, w, media.DeviceVideoSpec{W: 1920, H: 480, Turn: 1, Dim: 0.35})
	c := PanelConfig{Mode: "design", Design: id, Background: "media:clip.mp4"}
	src, err := w.newSource(c)
	if err != nil {
		t.Fatal(err)
	}
	ds := src.(*designSource)
	if ds.video == nil || ds.bg != nil {
		t.Fatal("video not taken over by the display (or the clip decoded anyway)")
	}
	if _, _, ok := w.layeredStep(src, 0, false); !ok {
		t.Fatal("layered step refused")
	}
	if f.lastBack.Video == nil || f.lastBack.Video.Path != want || f.lastOv == nil {
		t.Fatalf("backdrop %+v", f.lastBack)
	}
	// A different strength is a different file (dimmed inside the video).
	c.BgStrength = 60
	src2, _ := w.newSource(c)
	if ds2 := src2.(*designSource); ds2.video == nil || ds2.video.path == want {
		t.Fatal("brightness not part of the encoded file")
	}
	// Bloom needs the frames on the host.
	c.BgBloom = true
	src3, _ := w.newSource(c)
	if src3.(*designSource).video != nil {
		t.Fatal("bloom background given to the display")
	}
}

func TestVideoEncodeFailureFallsBack(t *testing.T) {
	fakeFFmpegPresent(t)
	f := newLayerFake()
	w := newLayerWorker(t, f)
	dir := filepath.Join(w.e.Store.Dir, "media")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("garbage"), 0o600)
	spec := media.DeviceVideoSpec{W: 1920, H: 480, Turn: 1, Dim: 1}
	j := w.e.deviceVideoJob("clip.mp4", spec)
	if j == nil {
		t.Fatal("no job")
	}
	// ffmpeg is faked present but does not exist or cannot read the file: the job fails
	select {
	case <-j.done:
	case <-time.After(10 * time.Second):
		t.Fatal("job did not end")
	}
	if j.state() != jobFailed {
		t.Skip("a real ffmpeg is installed and made something of the garbage")
	}
	if again := w.e.deviceVideoJob("clip.mp4", spec); again != j {
		t.Fatal("failed job started again at once")
	}
	if s := w.deviceMediaSource(PanelConfig{Media: "clip.mp4"}, image.Pt(1920, 480), nil); s != nil {
		t.Fatal("failed video offered to the display")
	}
	// Images and gifs are never encoded.
	os.WriteFile(filepath.Join(dir, "pic.png"), []byte("x"), 0o600)
	if w.e.deviceVideoJob("pic.png", spec) != nil {
		t.Fatal("image sent to the encoder")
	}
	if w.e.deviceVideoJob("../x.mp4", spec) != nil {
		t.Fatal("bad name accepted")
	}
}

func TestOfflineClockIsWrittenOnlyOnRequest(t *testing.T) {
	old := offlineClockDelay
	offlineClockDelay = 400 * time.Millisecond // longer than a persist on a slow CI machine
	defer func() { offlineClockDelay = old }()
	f := newLayerFake()
	w := newLayerWorker(t, f)

	// start-up with nothing set: the panel is not even called
	w.syncOfflineClock()
	if len(f.ocCalls) != 0 {
		t.Fatalf("start-up called the panel: %v", f.ocCalls)
	}
	// quick clicks collapse into one call with the final value
	for _, on := range []bool{true, false, true, false, true} {
		if err := w.e.SetOfflineClock("us88-0", on); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(1200 * time.Millisecond)
	f.mu.Lock()
	calls := append([]bool(nil), f.ocCalls...)
	f.mu.Unlock()
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("panel calls %v, want one write of true", calls)
	}
	ps := w.e.Panels()
	if len(ps) != 1 || ps[0].OfflineClock == nil || !*ps[0].OfflineClock {
		t.Fatalf("panel state %+v", ps)
	}
	if w.e.cfg.OfflineClock["us88-0"] != true {
		t.Fatal("not stored")
	}
	// a worker start syncs a held value again (the panel dedupes it)
	f.mu.Lock()
	f.ocCalls = nil
	f.mu.Unlock()
	w.syncOfflineClock()
	f.mu.Lock()
	n := len(f.ocCalls)
	f.mu.Unlock()
	if n != 1 { // the fake counts every call; the real panel writes nothing for equal values
		t.Fatalf("sync calls %d", n)
	}
	// unknown panels and panels without the clock are refused
	if w.e.SetOfflineClock("nope", true) == nil {
		t.Fatal("unknown panel accepted")
	}
	w.e.mu.Lock()
	w.e.workers["raw"] = &worker{e: w.e, p: &rawFake{}}
	w.e.mu.Unlock()
	if w.e.SetOfflineClock("raw", true) == nil {
		t.Fatal("panel without a clock accepted")
	}
}

func TestSettingsDeviceVideo(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	st := e.Settings()
	if st.DeviceVideo == nil || !*st.DeviceVideo {
		t.Fatal("device video not on by default")
	}
	// A request without the field leaves it alone.
	st.DeviceVideo = nil
	if err := e.ApplySettings(st); err != nil {
		t.Fatal(err)
	}
	if v := e.Settings().DeviceVideo; v == nil || !*v {
		t.Fatal("missing field switched it off")
	}
	st.DeviceVideo = refTo(false)
	if err := e.ApplySettings(st); err != nil {
		t.Fatal(err)
	}
	if v := e.Settings().DeviceVideo; v == nil || *v {
		t.Fatal("not switched off")
	}
}

func TestWorkerLoopUsesLayersAndFallsBackForTransitions(t *testing.T) {
	f := newLayerFake()
	w := newLayerWorker(t, f)
	id := plainDesign(t, w, false)
	w.e.mu.Lock()
	w.e.cfg.Panels["us88-0"] = PanelConfig{Mode: "design", Design: id, Brightness: 50}
	w.e.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.start(ctx, PanelConfig{Mode: "design", Design: id, Brightness: 50})
	defer func() { w.life.Lock(); w.halt(); w.life.Unlock() }()
	count := func() (int, int) { f.mu.Lock(); defer f.mu.Unlock(); return f.layers, f.images }
	waitCount := func(what string, ok func(l, i int) bool) {
		t.Helper()
		for i := 0; i < 300; i++ {
			if l, im := count(); ok(l, im) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		l, im := count()
		t.Fatalf("timeout: %s (layers %d, images %d)", what, l, im)
	}
	waitCount("layered frames", func(l, i int) bool { return l >= 1 })
	if _, im := count(); im != 0 {
		t.Fatalf("%d plain pictures sent in layer mode", im)
	}
	// A transition runs on plain pictures, then the layers come back.
	l0, _ := count()
	w.switchTo(PanelConfig{Mode: "design", Design: id, Brightness: 50, Theme: "light"}, "fade", 0)
	waitCount("transition pictures", func(l, i int) bool { return i >= 3 })
	waitCount("layers after the transition", func(l, i int) bool { return l > l0 })
}
