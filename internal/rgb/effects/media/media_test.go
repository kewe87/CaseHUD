package media

import (
	"errors"
	"image"
	"image/color"
	"math"
	"math/cmplx"
	"sync"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

type fakeGrabber struct {
	col    color.RGBA
	mu     sync.Mutex
	grabs  int
	closes int
}

func (g *fakeGrabber) Grab(dst *image.RGBA) error {
	g.mu.Lock()
	g.grabs++
	g.mu.Unlock()
	g.mu.Lock()
	col := g.col
	g.mu.Unlock()
	fill(dst, col)
	return nil
}

func (g *fakeGrabber) Close() error { g.mu.Lock(); g.closes++; g.mu.Unlock(); return nil }

func (g *fakeGrabber) counts() (int, int) { g.mu.Lock(); defer g.mu.Unlock(); return g.grabs, g.closes }

func TestScreen(t *testing.T) {
	g := &fakeGrabber{col: color.RGBA{200, 100, 50, 255}}
	s := startScreen(g, areaFull, 1, 0, 60)
	c := newCanvas()
	deadline := time.Now().Add(2 * time.Second)
	for c.Pix[0] != 200 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		s.Render(leds.Frame{}, c)
	}
	if got := c.RGBAAt(10, 10); got != (color.RGBA{200, 100, 50, 255}) {
		t.Fatalf("canvas %v", got)
	}
	s.Close()
	s.Close() // twice is fine
	if _, closes := g.counts(); closes == 0 {
		t.Fatal("grabber not closed")
	}
}

// Without Render calls nothing is captured; Render brings the capture back;
// after the idle time the grabber is released again.
func TestScreenIdles(t *testing.T) {
	g := &fakeGrabber{col: color.RGBA{1, 2, 3, 255}}
	s := startScreen(g, areaFull, 1, 0, 60)
	defer s.Close()
	time.Sleep(100 * time.Millisecond)
	if grabs, _ := g.counts(); grabs != 0 {
		t.Fatalf("%d grabs before the first Render", grabs)
	}
	c := newCanvas()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.Render(leds.Frame{}, c)
		if grabs, _ := g.counts(); grabs > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if grabs, _ := g.counts(); grabs == 0 {
		t.Fatal("no capture after Render")
	}
	_, closesBefore := g.counts()
	time.Sleep(idleAfter + 300*time.Millisecond)
	grabs1, closes := g.counts()
	if closes <= closesBefore {
		t.Fatal("grabber not released while idle")
	}
	time.Sleep(150 * time.Millisecond)
	if grabs2, _ := g.counts(); grabs2 != grabs1 {
		t.Fatalf("captured while idle: %d -> %d", grabs1, grabs2)
	}
	for time.Now().Before(deadline.Add(2 * time.Second)) {
		s.Render(leds.Frame{}, c)
		if grabs3, _ := g.counts(); grabs3 > grabs1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("capture did not resume")
}

func TestEdgeFill(t *testing.T) {
	src := newCanvas()
	fill(src, color.RGBA{0, 0, 0, 255})
	w, h := leds.CanvasWidth, leds.CanvasHeight
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < w/8:
				src.SetRGBA(x, y, color.RGBA{255, 0, 0, 255}) // left red
			case x >= w-w/8:
				src.SetRGBA(x, y, color.RGBA{0, 255, 0, 255}) // right green
			case y < h/8:
				src.SetRGBA(x, y, color.RGBA{0, 0, 255, 255}) // top blue
			case y >= h-h/8:
				src.SetRGBA(x, y, color.RGBA{255, 255, 0, 255}) // bottom yellow
			}
		}
	}
	dst := newCanvas()
	edgeFill(dst, src)
	dominant := func(c color.RGBA) string {
		switch {
		case c.R > 200 && c.G < 60 && c.B < 60:
			return "red"
		case c.G > 200 && c.R < 60 && c.B < 60:
			return "green"
		case c.B > 200 && c.R < 60 && c.G < 60:
			return "blue"
		case c.R > 200 && c.G > 200 && c.B < 60:
			return "yellow"
		}
		return "other"
	}
	for _, tc := range []struct {
		x, y int
		want string
	}{{5, h / 2, "red"}, {w - 5, h / 2, "green"}, {w / 2, 5, "blue"}, {w / 2, h - 5, "yellow"},
		{w / 2, h/2 - 30, "blue"}, {w/2 + 10, h/2 + 30, "yellow"}, {w/2 + 40, h/2 + 10, "green"}, {60, h/2 + 5, "red"}} {
		if got := dominant(dst.RGBAAt(tc.x, tc.y)); got != tc.want {
			t.Errorf("(%d,%d): %s (%v), want %s", tc.x, tc.y, got, dst.RGBAAt(tc.x, tc.y), tc.want)
		}
	}
}

func TestScreenEdgesParamDefault(t *testing.T) {
	var area, fps any
	for _, p := range screenEffect().Params {
		switch p.Key {
		case "area":
			area = p.Default
		case "fps":
			fps = p.Default
		}
	}
	if area != "edges" || fps != 15.0 {
		t.Fatalf("defaults: area %v fps %v", area, fps)
	}
}

func TestGrade(t *testing.T) {
	c, src := newCanvas(), newCanvas()
	fill(src, color.RGBA{200, 100, 100, 255})
	grade(c, src, 0.5, 1) // half way from black
	if c.Pix[0] != 100 || c.Pix[1] != 50 {
		t.Fatalf("smoothing: %v", c.Pix[:4])
	}
	grade(c, src, 0, 0) // no saturation: grey
	if c.Pix[0] != c.Pix[1] || c.Pix[1] != c.Pix[2] {
		t.Fatalf("saturation 0: %v", c.Pix[:4])
	}
}

func TestFFT(t *testing.T) {
	x := make([]complex128, 64)
	for i := range x {
		x[i] = complex(math.Cos(2*math.Pi*5*float64(i)/64), 0)
	}
	fft(x)
	for i := range x[:32] {
		want := 0.0
		if i == 5 {
			want = 32
		}
		if math.Abs(cmplx.Abs(x[i])-want) > 1e-9 {
			t.Fatalf("bin %d: %v", i, cmplx.Abs(x[i]))
		}
	}
}

type fakeCapture struct{}

func (fakeCapture) Close() error { return nil }

type countCapture struct {
	mu            sync.Mutex
	opens, closes int
}

func (c *countCapture) open(p func([]float32, int)) (interface{ Close() error }, error) {
	c.mu.Lock()
	c.opens++
	c.mu.Unlock()
	return closeFn(func() { c.mu.Lock(); c.closes++; c.mu.Unlock() }), nil
}

func (c *countCapture) get() (int, int) { c.mu.Lock(); defer c.mu.Unlock(); return c.opens, c.closes }

type closeFn func()

func (f closeFn) Close() error { f(); return nil }

// The device is opened by Render, released after the idle time and opened
// again by the next Render; a probe instance never opens it.
func TestAudioIdles(t *testing.T) {
	cc := &countCapture{}
	newCapture = cc.open
	audioAvailable = func() error { return nil }
	defer func() { newCapture = openLoopback; audioAvailable = platformAudio }()

	p := leds.Params{}
	for _, prm := range audioEffect().Params {
		p[prm.Key] = prm.Default
	}
	probe, err := audioEffect().New(p)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	probe.(*audio).Close()
	if o, _ := cc.get(); o != 0 {
		t.Fatalf("probe opened the device %d times", o)
	}

	eff, _ := audioEffect().New(p)
	a := eff.(*audio)
	c := newCanvas()
	wait := func(cond func() bool, msg string) {
		t.Helper()
		for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
			a.Render(leds.Frame{}, c)
			if cond() {
				return
			}
		}
		t.Fatal(msg)
	}
	wait(func() bool { o, _ := cc.get(); return o == 1 }, "not opened by Render")
	time.Sleep(idleAfter + 400*time.Millisecond) // no Render
	if o, cl := cc.get(); o != 1 || cl != 1 {
		t.Fatalf("after idle: opens %d closes %d", o, cl)
	}
	wait(func() bool { o, _ := cc.get(); return o == 2 }, "did not resume")
	a.Close()
	a.Close()
	if o, cl := cc.get(); o != 2 || cl != 2 {
		t.Fatalf("after Close: opens %d closes %d", o, cl)
	}
}

func TestAudioBands(t *testing.T) {
	pushed := make(chan func([]float32, int), 1)
	newCapture = func(p func([]float32, int)) (interface{ Close() error }, error) {
		pushed <- p
		return fakeCapture{}, nil
	}
	audioAvailable = func() error { return nil }
	defer func() { newCapture = openLoopback; audioAvailable = platformAudio }()

	info := audioEffect()
	p := leds.Params{}
	for _, prm := range info.Params {
		p[prm.Key] = prm.Default
	}
	p["rainbow"] = false
	eff, err := info.New(p)
	if err != nil {
		t.Fatal(err)
	}
	a := eff.(*audio)
	defer a.Close()

	c := newCanvas()
	a.Render(leds.Frame{}, c) // opens the capture
	var push func([]float32, int)
	select {
	case push = <-pushed:
	case <-time.After(2 * time.Second):
		t.Fatal("capture not opened by Render")
	}

	// 1 kHz sine: the loudest band must contain 1 kHz
	tone := make([]float32, ringSize)
	for i := range tone {
		tone[i] = float32(0.5 * math.Sin(2*math.Pi*1000*float64(i)/48000))
	}
	push(tone, 48000)
	a.Render(leds.Frame{}, c)
	best := 0
	for b, v := range a.bands {
		if v > a.bands[best] {
			best = b
		}
	}
	n := float64(len(a.bands))
	lo := minHz * math.Pow(maxHz/minHz, float64(best)/n)
	hi := minHz * math.Pow(maxHz/minHz, float64(best+1)/n)
	if lo > 1000 || hi < 1000 || a.bands[best] < 0.9 {
		t.Fatalf("loudest band %d (%.0f-%.0f Hz) level %.2f", best, lo, hi, a.bands[best])
	}

	// silence: levels fall off
	push(make([]float32, ringSize), 48000)
	for i := 0; i < 60; i++ {
		a.Render(leds.Frame{}, c)
	}
	if a.bands[best] > 0.05 {
		t.Fatalf("level after silence %.2f", a.bands[best])
	}
}

type hostSource struct {
	col    color.RGBA
	closed bool
}

func (h *hostSource) Frame(dst *image.RGBA) bool { fill(dst, h.col); return true }
func (h *hostSource) Close() error               { h.closed = true; return nil }

func TestSourceEffect(t *testing.T) {
	src := &hostSource{col: color.RGBA{10, 20, 30, 255}}
	info := SourceEffect("host-video", "Host video", func(leds.Params) (Source, error) { return src, nil })
	p := leds.Params{}
	for _, prm := range info.Params {
		p[prm.Key] = prm.Default
	}
	p["saturation"] = 100.0
	eff, err := info.New(p)
	if err != nil {
		t.Fatal(err)
	}
	c := newCanvas()
	eff.Render(leds.Frame{}, c)
	if got := c.RGBAAt(5, 5); got != (color.RGBA{10, 20, 30, 255}) {
		t.Fatalf("canvas %v", got)
	}
	eff.(interface{ Close() error }).Close()
	if !src.closed {
		t.Fatal("source not closed")
	}
}

type failingGrabber struct {
	mu    sync.Mutex
	grabs int
}

func (g *failingGrabber) Grab(*image.RGBA) error {
	g.mu.Lock()
	g.grabs++
	g.mu.Unlock()
	return errors.New("no display")
}
func (g *failingGrabber) Close() error { return nil }

// A failing grab is retried with a back-off, not at the capture rate.
func TestScreenBacksOffAfterFailure(t *testing.T) {
	oldMin, oldMax := retryMin, retryMax
	retryMin, retryMax = 150*time.Millisecond, 300*time.Millisecond
	defer func() { retryMin, retryMax = oldMin, oldMax }()
	g := &failingGrabber{}
	s := startScreen(g, areaFull, 1, 0, 30)
	defer s.Close()
	c := newCanvas()
	for end := time.Now().Add(700 * time.Millisecond); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		s.Render(leds.Frame{}, c)
	}
	g.mu.Lock()
	n := g.grabs
	g.mu.Unlock()
	if n < 2 || n > 5 { // at 30 fps it would be more than 15
		t.Fatalf("%d grabs in 700 ms", n)
	}
}

// After an idle period no old frame is shown: the first Render after the
// resume is black until a new grab arrives.
func TestScreenNoStaleFrameAfterResume(t *testing.T) {
	g := &fakeGrabber{col: color.RGBA{200, 100, 50, 255}}
	s := startScreen(g, areaFull, 1, 0, 30)
	defer s.Close()
	c := newCanvas()
	for end := time.Now().Add(2 * time.Second); c.Pix[0] != 200 && time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		s.Render(leds.Frame{}, c)
	}
	if c.Pix[0] != 200 {
		t.Fatal("no frame")
	}
	time.Sleep(idleAfter + 300*time.Millisecond)
	g.mu.Lock()
	g.col = color.RGBA{}
	g.mu.Unlock()
	s.Render(leds.Frame{}, c) // first Render after the resume, before any new grab can be published
	if c.Pix[0] != 0 {
		t.Fatalf("stale frame shown: %v", c.Pix[:4])
	}
}

func TestMaxScreenFPS(t *testing.T) {
	for _, p := range screenEffect().Params {
		if p.Key == "fps" && p.Max != 30 {
			t.Fatalf("fps max %v", p.Max)
		}
	}
}
