package media

import (
	"image"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// idleAfter is how long the capture goroutine keeps grabbing after the last
// Render call. Without a renderer (lighting paused, effect being replaced)
// nothing is captured and the display handles are released.
const idleAfter = time.Second

// grabber captures a display scaled to the canvas size. Close releases what
// the grabber holds; a later Grab may set it up again (the capture goroutine
// closes it while idle).
type grabber interface {
	Grab(dst *image.RGBA) error
	Close() error
}

// newGrabber is replaced in tests; display 0 is the primary monitor.
var newGrabber = openScreen

// maxScreenFPS caps the capture rate: more only costs CPU, the LEDs cannot show it.
const maxScreenFPS = 30

// A failed grab (locked desktop, mode switch, no display) is retried after
// retryMin, doubling up to retryMax, instead of at the capture rate.
var (
	retryMin = time.Second
	retryMax = 10 * time.Second
)

const (
	areaFull  = "full"
	areaEdges = "edges"
)

func screenEffect() leds.EffectInfo {
	return leds.EffectInfo{
		Name: "screen", Label: "Screen ambience",
		Params: []leds.Param{
			{Key: "display", Label: "Display (0 = primary)", Type: leds.ParamNumber, Min: 0, Max: 8, Default: 0.0},
			{Key: "area", Label: "Area", Type: leds.ParamEnum, Values: []string{areaEdges, areaFull}, Default: areaEdges},
			{Key: "saturation", Label: "Saturation (%)", Type: leds.ParamNumber, Min: 0, Max: 300, Default: 130.0},
			{Key: "smoothing", Label: "Smoothing", Type: leds.ParamNumber, Min: 0, Max: 95, Default: 40.0},
			{Key: "fps", Label: "Capture rate", Type: leds.ParamNumber, Min: 5, Max: maxScreenFPS, Default: 15.0},
		},
		New: func(p leds.Params) (leds.Effect, error) {
			g, err := newGrabber(int(p.Float("display")))
			if err != nil {
				return nil, err
			}
			return startScreen(g, p.String("area"), p.Float("saturation")/100, p.Float("smoothing")/100, p.Float("fps")), nil
		},
	}
}

type screen struct {
	src        latest
	frame      *image.RGBA
	edge       *image.RGBA
	seen       uint64
	gen        atomic.Uint64 // counts idle periods; Render drops its frames when it changes
	seenGen    uint64
	area       string
	saturation float64
	smoothing  float64
	lastRender atomic.Int64 // unix nanoseconds of the latest Render call
	wake       chan struct{}
	stop       chan struct{}
	stopOnce   sync.Once
	done       sync.WaitGroup
}

// startScreen starts the capture goroutine. It grabs only while Render is
// being called: the first grab happens on the first Render.
func startScreen(g grabber, area string, saturation, smoothing, fps float64) *screen {
	s := &screen{area: area, saturation: saturation, smoothing: smoothing, frame: newCanvas(), edge: newCanvas(),
		wake: make(chan struct{}, 1), stop: make(chan struct{})}
	s.done.Add(1)
	go s.run(g, time.Duration(float64(time.Second)/min(max(fps, 1), maxScreenFPS)))
	return s
}

func (s *screen) active() bool {
	last := s.lastRender.Load()
	return last != 0 && time.Since(time.Unix(0, last)) <= idleAfter
}

func (s *screen) run(g grabber, every time.Duration) {
	runtime.LockOSThread() // GDI objects belong to the thread that creates them
	defer s.done.Done()
	defer g.Close()
	defer func() {
		if r := recover(); r != nil {
			log.Print("media: screen capture stopped after an internal error") // no data in the message
		}
	}()
	buf := newCanvas()
	var warned time.Time
	for {
		// idle: nothing is captured and nothing is kept
		g.Close()
		s.src.clear()
		clear(buf.Pix)
		s.gen.Add(1)
		select {
		case <-s.stop:
			return
		case <-s.wake:
		}
		t := time.NewTicker(every)
		backoff := time.Duration(0)
		for s.active() {
			wait := (<-chan time.Time)(t.C)
			if err := g.Grab(buf); err != nil {
				s.src.fail(err) // keep trying: the desktop may be locked or switching modes
				if backoff == 0 {
					backoff = retryMin
				} else {
					backoff = min(backoff*2, retryMax)
				}
				if time.Since(warned) > time.Minute {
					warned = time.Now()
					log.Printf("media: screen capture failing, retrying: %v", err)
				}
				wait = time.After(backoff)
			} else {
				backoff = 0
				buf = s.src.publish(buf)
			}
			select {
			case <-s.stop:
				t.Stop()
				return
			case <-wait:
			}
		}
		t.Stop()
	}
}

func (s *screen) Render(f leds.Frame, c *image.RGBA) {
	s.lastRender.Store(time.Now().UnixNano())
	select {
	case s.wake <- struct{}{}:
	default:
	}
	if g := s.gen.Load(); g != s.seenGen {
		// the capture idled since the last frame: nothing old is shown after
		// the resume, only black until the first new grab arrives
		s.seenGen = g
		clear(s.frame.Pix)
		clear(s.edge.Pix)
	}
	s.src.copyTo(s.frame, &s.seen)
	src := s.frame
	if s.area == areaEdges {
		edgeFill(s.edge, s.frame)
		src = s.edge
	}
	grade(c, src, s.smoothing, s.saturation)
}

// Close stops the capture goroutine and releases the grabber. It is safe to
// call more than once.
func (s *screen) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.done.Wait()
	s.src.clear()
	clear(s.frame.Pix) // Close runs on the engine goroutine, the one that renders
	clear(s.edge.Pix)
	return nil
}

// edgeFill paints dst like an ambient light: the border band of src is
// averaged along each edge, and every pixel of dst takes the colour of the
// edge point that lies in its direction from the centre.
func edgeFill(dst, src *image.RGBA) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	bw, bh := max(1, w/8), max(1, h/8)
	// strips: average colour across the band for every position along an edge
	top, bottom := make([][3]float64, w), make([][3]float64, w)
	left, right := make([][3]float64, h), make([][3]float64, h)
	avg := func(out *[3]float64, n int, at func(i int) int) {
		for i := 0; i < n; i++ {
			o := at(i)
			out[0] += float64(src.Pix[o])
			out[1] += float64(src.Pix[o+1])
			out[2] += float64(src.Pix[o+2])
		}
		out[0], out[1], out[2] = out[0]/float64(n), out[1]/float64(n), out[2]/float64(n)
	}
	for x := 0; x < w; x++ {
		avg(&top[x], bh, func(i int) int { return src.PixOffset(x, i) })
		avg(&bottom[x], bh, func(i int) int { return src.PixOffset(x, h-1-i) })
	}
	for y := 0; y < h; y++ {
		avg(&left[y], bw, func(i int) int { return src.PixOffset(i, y) })
		avg(&right[y], bw, func(i int) int { return src.PixOffset(w-1-i, y) })
	}
	top, bottom = smoothStrip(top), smoothStrip(bottom)
	left, right = smoothStrip(left), smoothStrip(right)

	cx, cy := float64(w-1)/2, float64(h-1)/2
	for y := 0; y < h; y++ {
		dy := float64(y) - cy
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			var c [3]float64
			if abs(dx)*cy > abs(dy)*cx { // leaves through the left or right edge
				yy := clampIdx(int(cy+dy*cx/abs(dx)+0.5), h)
				if dx < 0 {
					c = left[yy]
				} else {
					c = right[yy]
				}
			} else {
				xx := w / 2
				if dy != 0 {
					xx = clampIdx(int(cx+dx*cy/abs(dy)+0.5), w)
				}
				if dy < 0 || dy == 0 {
					c = top[xx]
				} else {
					c = bottom[xx]
				}
			}
			o := dst.PixOffset(x, y)
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = uint8(c[0]+0.5), uint8(c[1]+0.5), uint8(c[2]+0.5), 255
		}
	}
}

// smoothStrip blurs a strip with a box of +-4 so single bright pixels do not
// flicker a whole ray.
func smoothStrip(in [][3]float64) [][3]float64 {
	const r = 4
	out := make([][3]float64, len(in))
	for i := range in {
		var sum [3]float64
		n := 0
		for j := max(0, i-r); j <= min(len(in)-1, i+r); j++ {
			sum[0] += in[j][0]
			sum[1] += in[j][1]
			sum[2] += in[j][2]
			n++
		}
		out[i] = [3]float64{sum[0] / float64(n), sum[1] / float64(n), sum[2] / float64(n)}
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func clampIdx(i, n int) int { return max(0, min(n-1, i)) }
