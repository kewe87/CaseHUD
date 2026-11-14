package leds

import (
	"image"
	"image/color"
	"sync"
	"time"
)

// Preview size: effects that can be drawn at any size (EffectInfo.Scalable)
// are drawn for the GUI at twice the canvas resolution, so the picture under
// the case stays sharp; the others show the canvas itself.
const (
	PreviewWidth  = CanvasWidth * 2
	PreviewHeight = CanvasHeight * 2
)

const (
	// previewKeep is how long the engine keeps preparing preview frames after
	// the last request.
	previewKeep = 2 * time.Second
	// previewEvery limits preview frames to about thirty per second.
	previewEvery = 33 * time.Millisecond
)

// canvasTap hands downscaled copies of the effect canvas to the GUI. The
// engine only prepares them while they are requested.
type canvasTap struct {
	mu     sync.Mutex
	wanted time.Time   // last request
	at     time.Time   // when img was taken
	img    *image.RGBA // last preview frame (nil: none yet)
}

// due reports whether a preview was requested lately and the last one is old
// enough, so the engine knows whether to draw one.
func (t *canvasTap) due(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return now.Sub(t.wanted) <= previewKeep && now.Sub(t.at) >= previewEvery
}

// take stores a preview of pic (the canvas, or the effect drawn at preview
// size) if it is due. bright is the brightness factor (0..256).
func (t *canvasTap) take(pic *image.RGBA, bright int, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if now.Sub(t.wanted) > previewKeep || now.Sub(t.at) < previewEvery {
		return
	}
	if t.img == nil || t.img.Rect != pic.Rect {
		t.img = image.NewRGBA(pic.Rect)
	}
	Downscale(pic, t.img, bright)
	t.at = now
}

// CanvasPreview returns a copy of the effect canvas as last rendered, at half
// resolution and with the brightness applied (what the LEDs would show), and
// when it was taken. The first call starts the previews: it returns nil, and
// frames follow for two seconds after each call.
func (e *Engine) CanvasPreview() (*image.RGBA, time.Time) {
	t := &e.tap
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wanted = time.Now()
	if t.img == nil {
		return nil, time.Time{}
	}
	out := image.NewRGBA(t.img.Rect)
	copy(out.Pix, t.img.Pix)
	return out, t.at
}

// Downscale averages src into dst (the same or a smaller size) and scales the colours
// by bright/256.
func Downscale(src, dst *image.RGBA, bright int) {
	sb, db := src.Bounds(), dst.Bounds()
	sw, sh, dw, dh := sb.Dx(), sb.Dy(), db.Dx(), db.Dy()
	if sw == 0 || sh == 0 || dw == 0 || dh == 0 {
		return
	}
	bright = max(0, min(256, bright))
	for y := range dh {
		y0, y1 := y*sh/dh, max((y+1)*sh/dh, y*sh/dh+1)
		for x := range dw {
			x0, x1 := x*sw/dw, max((x+1)*sw/dw, x*sw/dw+1)
			var r, g, b, n int
			for yy := y0; yy < y1; yy++ {
				o := src.PixOffset(sb.Min.X+x0, sb.Min.Y+yy)
				for xx := x0; xx < x1; xx++ {
					r += int(src.Pix[o])
					g += int(src.Pix[o+1])
					b += int(src.Pix[o+2])
					o += 4
					n++
				}
			}
			dst.SetRGBA(db.Min.X+x, db.Min.Y+y, color.RGBA{
				R: uint8(r / n * bright / 256), G: uint8(g / n * bright / 256), B: uint8(b / n * bright / 256), A: 255})
		}
	}
}

// Previewer renders an effect for the GUI without devices: the same effect
// with the same parameters runs on, so the picture moves like the real one.
// It is safe for concurrent use.
type Previewer struct {
	mu     sync.Mutex
	sig    string
	effect Effect
	start  time.Time
	prev   time.Time
	n      uint64
	canvas *image.RGBA
	out    *image.RGBA
	at     time.Time
}

// Frame renders the effect (rebuilt when sig changes) and returns a copy of
// the preview. Calls closer together than previewEvery get the last
// picture again.
func (p *Previewer) Frame(info EffectInfo, params Params, sig string, sensors Sensors, bright int, now time.Time) (*image.RGBA, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.effect == nil || p.sig != sig {
		eff, err := info.New(withDefaults(info, params))
		if err != nil {
			return nil, err
		}
		closeEffect(p.effect)
		p.effect, p.sig, p.start, p.prev, p.n, p.out = eff, sig, now, now, 0, nil
		r := image.Rect(0, 0, CanvasWidth, CanvasHeight)
		if info.Scalable {
			r = image.Rect(0, 0, PreviewWidth, PreviewHeight)
		}
		if p.canvas == nil || p.canvas.Rect != r {
			p.canvas = image.NewRGBA(r)
		}
		clear(p.canvas.Pix)
	}
	if p.out == nil || now.Sub(p.at) >= previewEvery {
		p.effect.Render(Frame{T: now.Sub(p.start), Dt: now.Sub(p.prev), N: p.n, Sensors: sensors}, p.canvas)
		p.n++
		p.prev = now
		if p.out == nil || p.out.Rect != p.canvas.Rect {
			p.out = image.NewRGBA(p.canvas.Rect)
		}
		Downscale(p.canvas, p.out, bright)
		p.at = now
	}
	out := image.NewRGBA(p.out.Rect)
	copy(out.Pix, p.out.Pix)
	return out, nil
}

// Close releases the effect.
func (p *Previewer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	closeEffect(p.effect)
	p.effect, p.sig, p.out = nil, "", nil
}
