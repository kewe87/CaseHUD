package app

// Layer mode and video on the display.
//
// A bar display that can keep a picture and a transparent overlay apart (see
// panel.LayeredPanel) gets a design with a plain background as one picture,
// sent once, and the live values as an overlay that is sent only when they
// change, at most five times a second. That saves encoding every frame and
// most of the USB traffic. When the design has an animated layer, animated
// icons or a transition runs, frames go out as before (the panel clears the
// overlay when it switches back to pictures).
//
// A video background or a video shown on a whole display can be played by the
// display itself: the file is encoded once into the device's H.264 format with
// the user's ffmpeg (see devvideo.go) and the values go on top as an overlay.
// While the encoding runs (or when it fails) the frames are drawn as before.

import (
	"hash/maphash"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/media"
	"github.com/kewe87/casehud/internal/panel"
)

// layerMinWait is the shortest time between two layered frames (5 Hz); the
// panel enforces it too.
const layerMinWait = 200 * time.Millisecond

// layerSource is a source that can describe its frame as a backdrop with an
// overlay instead of one picture.
type layerSource interface {
	source
	// layerFrame returns the frame in layers and how long it may stay; ok is
	// false when this frame has to be a plain picture (frame).
	layerFrame(t time.Duration) (lf layerFrame, wait time.Duration, ok bool)
}

type layerFrame struct {
	backdrop panel.Backdrop
	overlay  *image.RGBA // premultiplied; nil = nothing on top
	// compose builds the whole picture (preview, start of a transition).
	compose func() *image.RGBA
}

// layerCaps is what the worker's panel may do with layers right now.
func (w *worker) layerCaps() panel.LayerCaps {
	lp, ok := w.p.(panel.LayeredPanel)
	if !ok || w.e.layersOff() {
		return panel.LayerCaps{}
	}
	c := lp.LayerCaps()
	if !w.e.deviceVideoEnabled() {
		c.Video = false
	}
	return c
}

func (e *Engine) layersOff() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.NoLayers
}

// attachLayers prepares a design source that fills a whole panel for the layer
// mode or for a video played by the display.
func (w *worker) attachLayers(s *designSource, c PanelConfig, sz image.Point) {
	caps := w.layerCaps()
	if !caps.Static && !caps.Video {
		return
	}
	if s.r.HasIcons() {
		return // icons are drawn per display frame
	}
	if c.Background == "" {
		if caps.Static {
			s.layerable, s.solid = true, opaque(s.r.BackgroundColor())
			s.r.Transparent = true
		}
		return
	}
	name, ok := strings.CutPrefix(c.Background, "media:")
	if !ok || !caps.Video || c.BgBloom {
		return
	}
	strength := float64(c.BgStrength) / 100
	if c.BgStrength <= 0 {
		strength = 0.35 // as in backgroundLayer
	}
	spec, ok := w.videoSpec(c.Rotation, sz, min(1, max(0.01, strength)))
	if !ok {
		return
	}
	if job := w.e.deviceVideoJob(name, spec); job != nil && job.state() != jobFailed {
		s.video, s.solid = job, color.RGBA{A: 255}
		s.r.Transparent = true
	}
}

// videoSpec says how a video must be encoded for this panel at rotation: the
// panel knows how its canvas is turned into the device frame.
func (w *worker) videoSpec(rotation int, canvas image.Point, dim float64) (media.DeviceVideoSpec, bool) {
	vo, ok := w.p.(panel.VideoOrienter)
	if !ok {
		return media.DeviceVideoSpec{}, false
	}
	return media.DeviceVideoSpec{W: canvas.X, H: canvas.Y, Turn: vo.VideoTurn(rotation), Dim: dim}, true
}

func opaque(c color.RGBA) color.RGBA { c.A = 255; return c }

// solidImage is a picture of one colour.
func solidImage(r image.Rectangle, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(r)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

var solidSeed = maphash.MakeSeed()

// solidKey identifies a plain picture for the panel (it is sent again only
// when the colour or the size changes).
func solidKey(c color.RGBA, r image.Rectangle) uint64 {
	var h maphash.Hash
	h.SetSeed(solidSeed)
	h.Write([]byte{c.R, c.G, c.B, c.A, byte(r.Dx()), byte(r.Dx() >> 8), byte(r.Dy()), byte(r.Dy() >> 8)})
	return h.Sum64()
}

// resolve settles what the source shows: a video the display cannot get (the
// encoding failed or takes too long) is dropped and the animated layer is
// built after all.
func (s *designSource) resolve() {
	if s.video != nil {
		switch s.video.state() {
		case jobFailed:
			s.video = nil
		case jobPending:
			if time.Since(s.video.started) > videoGrace {
				s.video = nil
			}
		}
	}
	if s.video == nil && s.mkBG != nil {
		mk := s.mkBG
		s.mkBG = nil
		if s.bg = mk(); s.bg == nil {
			s.r.Transparent = false // no layer to lay the HUD over
			s.hud = nil
		}
	}
}

// hudWait is how long the HUD frame may stay as it is.
func (s *designSource) hudWait() time.Duration {
	if s.settled {
		return max(s.frameDur, s.interval-time.Since(s.sampled))
	}
	return s.frameDur
}

// plainFrame is the picture for a source whose HUD is transparent but that
// has no animated layer: the HUD over its plain colour (or black while a video
// is being prepared or when the layer mode cannot be used for this frame).
func (s *designSource) plainFrame() (*image.RGBA, time.Duration, bool) {
	switch {
	case s.layerable:
		return s.opaqueHUD(s.solid), s.hudWait(), true
	case s.video != nil:
		wait := s.frameDur
		if s.video.state() == jobPending {
			wait = max(wait, 250*time.Millisecond)
		}
		return s.opaqueHUD(s.solid), wait, true
	}
	return nil, 0, false
}

// opaqueHUD is the HUD over a plain colour, kept while the HUD does not change.
func (s *designSource) opaqueHUD(c color.RGBA) *image.RGBA {
	if s.opaque != nil && s.opaqueFor == s.hud && s.opaque.Rect == s.hud.Rect {
		return s.opaque
	}
	img := solidImage(s.hud.Rect, c)
	over(img, s.hud)
	s.opaque, s.opaqueFor = img, s.hud
	return img
}

// layerFrame implements layerSource.
func (s *designSource) layerFrame(time.Duration) (layerFrame, time.Duration, bool) {
	s.resolve()
	playing := s.video != nil && s.video.state() == jobReady
	if !s.layerable && !playing {
		return layerFrame{}, 0, false
	}
	s.update()
	wait := s.hudWait()
	if !s.settled {
		wait = max(wait, layerMinWait)
	}
	hud, rect, base := s.hud, s.hud.Rect, s.solid
	lf := layerFrame{overlay: hud, compose: func() *image.RGBA {
		img := solidImage(rect, base)
		over(img, hud)
		return img
	}}
	if playing {
		lf.backdrop = panel.Backdrop{Video: &panel.DeviceVideo{Path: s.video.path, FPS: media.DeviceVideoFPS}}
	} else {
		lf.backdrop = panel.Backdrop{Key: solidKey(base, rect), Image: func() *image.RGBA { return solidImage(rect, base) }}
	}
	return lf, wait, true
}

// videoMediaSource shows a video file on a whole display: the display plays it
// itself once the encoding is done; until then (and if it fails) the frames
// come from the decoded clip.
type videoMediaSource struct {
	job   *devVideoJob
	build func() (source, error)
	fb    source // decoded clip, built when needed
	black *image.RGBA
}

// deviceMediaSource returns the source for a video shown on a whole display, or
// nil when the display cannot play it (then build is used as before).
func (w *worker) deviceMediaSource(c PanelConfig, sz image.Point, build func() (source, error)) source {
	if !w.layerCaps().Video {
		return nil
	}
	spec, ok := w.videoSpec(c.Rotation, sz, 1)
	if !ok {
		return nil
	}
	job := w.e.deviceVideoJob(c.Media, spec)
	if job == nil || job.state() == jobFailed {
		return nil
	}
	return &videoMediaSource{job: job, build: build, black: solidImage(image.Rect(0, 0, sz.X, sz.Y), color.RGBA{A: 255})}
}

func (s *videoMediaSource) layerFrame(time.Duration) (layerFrame, time.Duration, bool) {
	if s.fb != nil || s.job.state() != jobReady {
		return layerFrame{}, 0, false
	}
	black := s.black
	return layerFrame{
		backdrop: panel.Backdrop{Video: &panel.DeviceVideo{Path: s.job.path, FPS: media.DeviceVideoFPS}},
		compose:  func() *image.RGBA { return black },
	}, time.Second, true
}

func (s *videoMediaSource) frame(t time.Duration) (*image.RGBA, time.Duration) {
	if s.fb == nil {
		switch s.job.state() {
		case jobPending:
			if time.Since(s.job.started) < videoGrace {
				return s.black, 250 * time.Millisecond
			}
		case jobReady:
			return s.black, time.Second // not layered right now (a transition runs)
		}
		fb, err := s.build()
		if err != nil {
			fb = sourceFunc(func(time.Duration) (*image.RGBA, time.Duration) { return s.black, time.Second })
		}
		s.fb = fb
	}
	return s.fb.frame(t)
}

// identifying reports whether the identify number is on the display.
func (w *worker) identifying() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Now().Before(w.idUntil)
}

// layeredStep shows the source's frame in layers when that is possible right
// now. ok is false when a plain frame has to be drawn instead (no layers, a
// transition or the identify number is on, the source needs frames).
func (w *worker) layeredStep(src source, t time.Duration, busy bool) (wait time.Duration, compose func() *image.RGBA, ok bool) {
	ls, isLayer := src.(layerSource)
	if !isLayer || busy || w.identifying() {
		return 0, nil, false
	}
	lp, isPanel := w.p.(panel.LayeredPanel)
	if !isPanel {
		return 0, nil, false
	}
	t0 := time.Now()
	lf, wait, ok := ls.layerFrame(t)
	if !ok {
		return 0, nil, false
	}
	err := lp.ShowLayers(lf.backdrop, lf.overlay)
	w.mu.Lock()
	w.last, w.lastImg = nil, nil
	w.lastFn = func() image.Image { return lf.compose() }
	w.lastSeq++
	w.mu.Unlock()
	w.setErr(ErrDevice, err)
	return max(0, wait-time.Since(t0)), lf.compose, true
}
