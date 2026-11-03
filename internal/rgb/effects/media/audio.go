package media

import (
	"image"
	"image/color"
	"math"
	"math/cmplx"
	"sync"
	"sync/atomic"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

const (
	fftSize  = 2048
	ringSize = 8192
	minHz    = 35.0
	maxHz    = 16000.0
)

// newCapture starts capturing what the default output device plays
// (loopback) and calls push with mono samples; replaced in tests.
var newCapture = openLoopback

// audioAvailable reports whether the platform can capture audio; replaced in tests.
var audioAvailable = platformAudio

// samples is the ring buffer between the capture goroutine and Render.
type samples struct {
	mu   sync.Mutex
	buf  [ringSize]float32
	pos  int
	rate int
}

func (s *samples) push(mono []float32, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = rate
	for _, v := range mono {
		s.buf[s.pos] = v
		s.pos = (s.pos + 1) % ringSize
	}
}

// reset forgets the samples.
func (s *samples) reset() {
	s.mu.Lock()
	s.buf = [ringSize]float32{}
	s.mu.Unlock()
}

// last copies the newest len(dst) samples into dst and returns the sample rate.
func (s *samples) last(dst []float32) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := (s.pos - len(dst) + ringSize) % ringSize
	for i := range dst {
		dst[i] = s.buf[(start+i)%ringSize]
	}
	return s.rate
}

func audioEffect() leds.EffectInfo {
	return leds.EffectInfo{
		Name: "audio", Label: "Audio visualizer",
		Params: []leds.Param{
			{Key: "mode", Label: "Mode", Type: leds.ParamEnum, Values: []string{"spectrum", "pulse"}, Default: "spectrum"},
			{Key: "bands", Label: "Bands", Type: leds.ParamNumber, Min: 4, Max: 64, Default: 24.0},
			{Key: "sensitivity", Label: "Sensitivity (%)", Type: leds.ParamNumber, Min: 10, Max: 400, Default: 100.0},
			{Key: "decay", Label: "Decay", Type: leds.ParamNumber, Min: 0, Max: 100, Default: 60.0},
			{Key: "rainbow", Label: "Rainbow colors", Type: leds.ParamBool, Default: true},
			{Key: "low", Label: "Low color", Type: leds.ParamColor, Default: "#0050ff"},
			{Key: "high", Label: "High color", Type: leds.ParamColor, Default: "#ff0040"},
			{Key: "useAccent", Label: "Use design accent", Type: leds.ParamBool, Default: false},
		},
		New: func(p leds.Params) (leds.Effect, error) {
			a := &audio{
				mode:    p.String("mode"),
				bands:   make([]float64, int(math.Max(4, math.Min(64, p.Float("bands"))))),
				gain:    math.Max(p.Float("sensitivity"), 1) / 100,
				decay:   math.Max(0, math.Min(p.Float("decay"), 99)) / 100,
				rainbow: p.Bool("rainbow"), low: p.Color("low"), high: p.Color("high"),
				accent: p.Bool("useAccent"), window: make([]float32, fftSize), peak: 1e-3,
				spec: make([]complex128, fftSize),
			}
			if err := audioAvailable(); err != nil {
				return nil, err
			}
			a.raw = make([]float64, len(a.bands))
			// The device is opened on the first Render, not here: the probe
			// instance of SetEffect must not touch it.
			a.wake, a.stop = make(chan struct{}, 1), make(chan struct{})
			a.done.Add(1)
			go a.run()
			return a, nil
		},
	}
}

type audio struct {
	src        samples
	lastRender atomic.Int64 // unix nanoseconds of the latest Render call
	wake       chan struct{}
	stop       chan struct{}
	stopOnce   sync.Once
	done       sync.WaitGroup

	mode      string
	bands     []float64 // smoothed levels 0..1
	gain      float64
	decay     float64
	rainbow   bool
	low, high color.RGBA
	accent    bool

	window []float32
	spec   []complex128 // FFT buffer, reused
	raw    []float64
	peak   float64 // automatic gain reference
}

func (a *audio) active() bool {
	last := a.lastRender.Load()
	return last != 0 && time.Since(time.Unix(0, last)) <= idleAfter
}

// run keeps the loopback capture open only while Render is being called: it
// opens on demand and releases the device (and its COM objects) after idleAfter
// without a Render. Nothing that was heard is kept while idle.
func (a *audio) run() {
	defer a.done.Done()
	for {
		select {
		case <-a.stop:
			return
		case <-a.wake:
		}
		if !a.active() {
			continue
		}
		c, err := newCapture(a.src.push)
		if err != nil {
			// no output device right now: try again at the next Render
			select {
			case <-a.stop:
				return
			case <-time.After(time.Second):
			}
			continue
		}
		t := time.NewTicker(100 * time.Millisecond)
		for a.active() {
			select {
			case <-a.stop:
				t.Stop()
				c.Close()
				a.src.reset()
				return
			case <-t.C:
			}
		}
		t.Stop()
		c.Close()
		a.src.reset()
	}
}

// Close stops the capture and releases the device; safe to call twice.
func (a *audio) Close() error {
	a.stopOnce.Do(func() { close(a.stop) })
	a.done.Wait()
	return nil
}

// analyze updates the band levels from the newest samples.
func (a *audio) analyze() {
	rate := a.src.last(a.window)
	if rate == 0 {
		rate = 48000
	}
	spec := a.spec
	for i, v := range a.window {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/(fftSize-1)) // Hann
		spec[i] = complex(float64(v)*w, 0)
	}
	fft(spec)

	raw := a.raw
	binHz := float64(rate) / fftSize
	top := math.Min(maxHz, float64(rate)/2)
	maxRaw := 0.0
	for b := range raw {
		lo := minHz * math.Pow(top/minHz, float64(b)/float64(len(raw)))
		hi := minHz * math.Pow(top/minHz, float64(b+1)/float64(len(raw)))
		i0, i1 := int(lo/binHz), int(math.Ceil(hi/binHz))
		i1 = max(i1, i0+1)
		sum := 0.0
		for i := i0; i < i1 && i < fftSize/2; i++ {
			sum += cmplx.Abs(spec[i])
		}
		// mean magnitude, tilted up with frequency (music falls off ~3 dB/oct)
		raw[b] = sum / float64(i1-i0) * math.Sqrt(hi/minHz)
		maxRaw = math.Max(maxRaw, raw[b])
	}
	// automatic gain: follow peaks quickly, release slowly, ignore silence
	a.peak = math.Max(maxRaw, math.Max(a.peak*0.995, 1e-3))
	for b, v := range raw {
		level := math.Min(1, v/a.peak*a.gain)
		if level >= a.bands[b] {
			a.bands[b] = level
		} else {
			a.bands[b] = a.bands[b]*a.decay + level*(1-a.decay)
		}
	}
}

func (a *audio) Render(f leds.Frame, c *image.RGBA) {
	a.lastRender.Store(time.Now().UnixNano())
	select {
	case a.wake <- struct{}{}:
	default:
	}
	a.analyze()
	low, high := a.low, a.high
	if a.accent {
		high = f.Accent
	}
	w, h := c.Bounds().Dx(), c.Bounds().Dy()
	n := len(a.bands)

	if a.mode == "pulse" {
		bass := 0.0
		for _, v := range a.bands[:max(1, n/6)] {
			bass = math.Max(bass, v)
		}
		col := lerp(low, high, bass)
		if a.rainbow {
			col = leds.HSV(f.T.Seconds()*20, 1, 1)
		}
		fill(c, scale(col, bass))
		return
	}

	for x := 0; x < w; x++ {
		b := x * n / w
		level := a.bands[b]
		hue := leds.HSV(float64(b)/float64(n)*300, 1, 1)
		for y := 0; y < h; y++ {
			fromBottom := float64(h-y) / float64(h)
			var col color.RGBA
			if fromBottom <= level {
				if a.rainbow {
					col = hue
				} else {
					col = lerp(low, high, fromBottom)
				}
			}
			i := c.PixOffset(x, y)
			c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, 255
		}
	}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

func scale(c color.RGBA, v float64) color.RGBA {
	v = math.Max(0, math.Min(1, v))
	return color.RGBA{uint8(float64(c.R) * v), uint8(float64(c.G) * v), uint8(float64(c.B) * v), 255}
}

// fft is an in-place iterative radix-2 FFT; len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < size/2; k++ {
				u, v := x[start+k], x[start+k+size/2]*w
				x[start+k], x[start+k+size/2] = u+v, u-v
				w *= step
			}
		}
	}
}
