// Package media provides effects fed by the screen and the audio output:
// screen ambience (ambilight) and an audio visualizer. Both read data that can
// be private, so CaseHUD offers them only after a one-time confirmation, keeps
// no frame or sample beyond the next render and never writes them anywhere.
//
// Sources run in their own goroutine and publish canvas-sized frames;
// Render only copies the latest one, so a slow source never stalls the
// engine. All effects implement io.Closer.
package media

import (
	"errors"
	"image"
	"image/color"
	"math"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// ErrUnsupported is returned when a source is not available on this platform.
var ErrUnsupported = errors.New("media: not supported on this platform")

// Effects returns the media effects.
func Effects() []leds.EffectInfo {
	return []leds.EffectInfo{screenEffect(), audioEffect()}
}

// latest holds the most recent frame of a source.
type latest struct {
	mu    sync.Mutex
	frame *image.RGBA
	seq   uint64
	err   error
}

// publish swaps buf in as the current frame and returns the previous buffer
// for reuse.
func (l *latest) publish(buf *image.RGBA) *image.RGBA {
	l.mu.Lock()
	defer l.mu.Unlock()
	old := l.frame
	l.frame = buf
	l.seq++
	if old == nil {
		old = newCanvas()
	}
	return old
}

func (l *latest) fail(err error) {
	l.mu.Lock()
	l.err = err
	l.mu.Unlock()
}

// copyTo copies the current frame into dst if it is newer than seen.
func (l *latest) copyTo(dst *image.RGBA, seen *uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.frame == nil || l.seq == *seen {
		return false
	}
	*seen = l.seq
	copy(dst.Pix, l.frame.Pix)
	return true
}

func newCanvas() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, leds.CanvasWidth, leds.CanvasHeight))
}

// grade blends src into the canvas: smoothing 0..1 keeps that share of the
// previous canvas (against flicker), saturation scales the chroma (1 = as is).
func grade(c, src *image.RGBA, smoothing, saturation float64) {
	keep := math.Max(0, math.Min(smoothing, 0.98))
	for i := 0; i+3 < len(src.Pix) && i+3 < len(c.Pix); i += 4 {
		r, g, b := float64(src.Pix[i]), float64(src.Pix[i+1]), float64(src.Pix[i+2])
		if saturation != 1 {
			y := 0.299*r + 0.587*g + 0.114*b
			r, g, b = y+(r-y)*saturation, y+(g-y)*saturation, y+(b-y)*saturation
		}
		c.Pix[i] = mix(c.Pix[i], r, keep)
		c.Pix[i+1] = mix(c.Pix[i+1], g, keep)
		c.Pix[i+2] = mix(c.Pix[i+2], b, keep)
		c.Pix[i+3] = 255
	}
}

func mix(old uint8, v, keep float64) uint8 {
	v = float64(old)*keep + v*(1-keep)
	return uint8(math.Max(0, math.Min(255, v+0.5)))
}

func fill(c *image.RGBA, col color.RGBA) {
	for i := 0; i+3 < len(c.Pix); i += 4 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, 255
	}
}

// clear drops the current frame (screen content is not kept while idle).
func (l *latest) clear() {
	l.mu.Lock()
	l.frame = nil
	l.mu.Unlock()
}
