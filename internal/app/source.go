package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/lcd"
)

// source produces a panel's frames: a design, media or a blank screen.
type source interface {
	// frame returns the image for time t since the source started and how
	// long it may stay on screen.
	frame(t time.Duration) (*image.RGBA, time.Duration)
}

// backgroundFPS is the frame rate of animated backgrounds: smooth enough for
// slow motion while keeping USB traffic and CPU low.
const backgroundFPS = 20

// newSource builds what c says the panel should show.
func (w *worker) newSource(c PanelConfig) (source, error) {
	sz := w.sizeFor(c.Rotation)
	if c.Mode == "design" && c.Grid != "" {
		return w.newGridSource(c, sz)
	}
	return w.newSourceAtOpt(c, sz, true)
}

// newSourceAt builds the source for c on a canvas of sz: one field of a grid.
func (w *worker) newSourceAt(c PanelConfig, sz image.Point) (source, error) {
	return w.newSourceAtOpt(c, sz, false)
}

// newSourceAtOpt builds the source for c on a canvas of sz. top is set for a
// source that fills the whole panel: only such a source may use the layer
// mode and the video playback of the device (see layers.go).
func (w *worker) newSourceAtOpt(c PanelConfig, sz image.Point, top bool) (source, error) {
	switch c.Mode {
	case "design":
		l, err := w.e.Store.DesignFor(c.Design, sz)
		if err != nil {
			return nil, err
		}
		l.Lang = w.e.Language()
		l = lcd.ApplyTheme(l, c.Theme)
		scaled := l.FitTo(sz.X, sz.Y)
		r, err := lcd.NewRenderer(scaled)
		if err != nil {
			return nil, err
		}
		w.e.localize(r)
		interval := time.Duration(l.IntervalMS) * time.Millisecond
		if o := w.e.intervalOverride(); o > 0 {
			interval = o
		}
		if !w.e.animationOff() {
			r.SetAnimation(animationTau)
		}
		r.SplitIcons = true // icons are drawn per display frame on top of the cached HUD
		s := &designSource{w: w, r: r, interval: interval, frameDur: time.Second / time.Duration(min(maxFPS(w.p), 30))}
		if top {
			w.attachLayers(s, c, sz)
		}
		if c.Background != "" {
			if s.video != nil {
				// The device plays the video; the animated layer is only built if it
				// is needed after all (see bgFunc).
				s.mkBG = func() func(time.Duration) *image.RGBA { return w.backgroundFor(c, scaled) }
			} else {
				s.bg = w.backgroundFor(c, scaled)
				if s.bg != nil {
					r.Transparent = true
				}
			}
		}
		return s, nil
	case "media":
		build := func() (source, error) {
			clip, err := w.e.clip(c.Media, sz.X, sz.Y)
			if err != nil {
				return nil, err
			}
			fps := maxFPS(w.p)
			return sourceFunc(func(t time.Duration) (*image.RGBA, time.Duration) {
				if clip.Static() {
					return toRGBA(clip.At(0)), time.Second // resend as keepalive
				}
				return toRGBA(clip.At(t)), time.Second / time.Duration(fps)
			}), nil
		}
		if top {
			if s := w.deviceMediaSource(c, sz, build); s != nil {
				return s, nil
			}
		}
		return build()
	default: // off
		black := image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y))
		for i := 3; i < len(black.Pix); i += 4 {
			black.Pix[i] = 0xff
		}
		return sourceFunc(func(time.Duration) (*image.RGBA, time.Duration) { return black, time.Second }), nil
	}
}

type sourceFunc func(t time.Duration) (*image.RGBA, time.Duration)

func (f sourceFunc) frame(t time.Duration) (*image.RGBA, time.Duration) { return f(t) }

// backgroundFor returns the animated layer for c, or nil.
func (w *worker) backgroundFor(c PanelConfig, l lcd.Layout) func(time.Duration) *image.RGBA {
	bg, err := w.e.background(c.Background, c.BgStrength, c.BgBloom, l)
	if err != nil {
		w.setErr(ErrContent, err)
	}
	return bg
}

// background builds the animated layer bg ("" = none) for layout l at
// strength percent (0 = default).
func (e *Engine) background(bg string, strengthPct int, bloom bool, l lcd.Layout) (func(time.Duration) *image.RGBA, error) {
	f, err := e.backgroundLayer(bg, strengthPct, bloom, l)
	if f == nil || err != nil || !bloom || strings.HasPrefix(bg, "media:") { // media blooms before dimming
		return f, err
	}
	return func(t time.Duration) *image.RGBA {
		img := f(t)
		lcd.Bloom(img)
		return img
	}, nil
}

func (e *Engine) backgroundLayer(bg string, strengthPct int, bloom bool, l lcd.Layout) (func(time.Duration) *image.RGBA, error) {
	strength := float64(strengthPct) / 100
	if strengthPct <= 0 {
		strength = 0.35
	}
	if name, ok := strings.CutPrefix(bg, "media:"); ok {
		clip, err := e.clip(name, l.Width, l.Height)
		if err != nil {
			return nil, err
		}
		// Media backgrounds are dimmed so values stay readable (after the
		// bloom, which needs the full brightness to glow).
		return func(t time.Duration) *image.RGBA {
			img := image.NewRGBA(image.Rect(0, 0, l.Width, l.Height))
			draw.Draw(img, img.Bounds(), clip.At(t), image.Point{}, draw.Src)
			if bloom {
				lcd.Bloom(img)
			}
			for i := 0; i < len(img.Pix); i += 4 {
				img.Pix[i] = uint8(float64(img.Pix[i]) * strength)
				img.Pix[i+1] = uint8(float64(img.Pix[i+1]) * strength)
				img.Pix[i+2] = uint8(float64(img.Pix[i+2]) * strength)
			}
			return img
		}, nil
	}
	for _, fx := range lcd.BackgroundEffects {
		if fx == bg {
			return lcd.NewBackground(fx, l.Width, l.Height, l.Background, l.Accent, strength).Frame, nil
		}
	}
	return nil, nil
}

// RenderDesignWithBackground renders a preview frame of l over background
// bg (as the panels show it a few seconds in).
func (e *Engine) RenderDesignWithBackground(l lcd.Layout, w, h int, bg string, strength int, bloom bool) (*image.RGBA, error) {
	l.Lang = e.Language()
	if bg == "" {
		return e.RenderDesign(l, w, h)
	}
	scaled := l.FitTo(w, h)
	layer, err := e.background(bg, strength, bloom, scaled)
	if err != nil || layer == nil {
		return e.RenderDesign(l, w, h)
	}
	r, err := lcd.NewRenderer(scaled)
	if err != nil {
		return nil, err
	}
	e.localize(r)
	r.Transparent = true
	r.Sample(e.prov)
	r.Sample(e.prov)
	img := layer(3 * time.Second)
	draw.Draw(img, img.Bounds(), r.Render(e.prov), image.Point{}, draw.Over)
	return img, nil
}

// designSource renders a design, optionally over an animated background.
// With a background the HUD layer is only redrawn when values change and
// is composited onto every background frame.
type designSource struct {
	w        *worker
	r        *lcd.Renderer
	interval time.Duration
	frameDur time.Duration
	bg       func(time.Duration) *image.RGBA

	// Layer mode (see layers.go): layerable = the HUD is drawn transparent and
	// goes to the device as an overlay over a picture of colour solid; video =
	// the device plays this video behind the HUD; mkBG builds bg when it is
	// needed after all.
	layerable bool
	solid     color.RGBA
	video     *devVideoJob
	mkBG      func() func(time.Duration) *image.RGBA
	opaque    *image.RGBA // the HUD over solid, for frames that cannot be layered
	opaqueFor *image.RGBA

	sampled time.Time
	hud     *image.RGBA
	settled bool

	// A field of a grid is copied into the panel frame right away, so its
	// frames may share one buffer: out holds the HUD plus icons and only the
	// icon squares are restored and redrawn between HUD changes.
	reuse      bool
	out        *image.RGBA
	outFromHUD *image.RGBA
	hudNew     bool
}

// update samples the sensors and redraws the HUD when due.
func (s *designSource) update() {
	fresh := time.Since(s.sampled) >= s.interval-s.frameDur/2
	if fresh {
		// Sensors are sampled at the design's interval; in between, frames
		// only run while values are still gliding.
		s.r.Sample(s.w.e.prov)
		s.sampled = time.Now()
	}
	if s.hud == nil || fresh || !s.settled {
		s.hud, s.settled = s.r.Frame(s.w.e.prov)
		s.hudNew = true
	}
}

func (s *designSource) frame(t time.Duration) (*image.RGBA, time.Duration) {
	s.resolve()
	s.update()
	if img, wait, ok := s.plainFrame(); ok {
		return img, wait
	}
	icons := s.r.AnimatedIcons()
	if s.bg == nil {
		if icons {
			// The cached HUD stays; only the icon squares are redrawn.
			return s.withIcons(), min(s.frameDur, lcd.IconFrameInterval)
		}
		if s.settled {
			return s.hud, max(s.frameDur, s.interval-time.Since(s.sampled))
		}
		return s.hud, s.frameDur
	}
	img := s.bg(t)
	over(img, s.hud)
	d := time.Second / time.Duration(min(maxFPS(s.w.p), backgroundFPS))
	if icons {
		s.r.DrawIcons(img, s.w.e.prov)
		d = min(d, lcd.IconFrameInterval)
	}
	return img, d
}

// withIcons returns the HUD with the icons drawn on top.
func (s *designSource) withIcons() *image.RGBA {
	if !s.reuse {
		return s.r.ComposeIcons(s.hud, s.w.e.prov)
	}
	if s.out == nil || s.out.Rect != s.hud.Rect {
		s.out = image.NewRGBA(s.hud.Rect)
		s.hudNew = true
	}
	if s.hudNew {
		copy(s.out.Pix, s.hud.Pix)
		s.hudNew = false
	} else {
		for _, r := range s.r.IconRects() { // restore what the icons painted over
			r = r.Intersect(s.hud.Rect)
			for y := r.Min.Y; y < r.Max.Y; y++ {
				o := s.hud.PixOffset(r.Min.X, y)
				copy(s.out.Pix[o:o+r.Dx()*4], s.hud.Pix[o:o+r.Dx()*4])
			}
		}
	}
	s.r.DrawIcons(s.out, s.w.e.prov)
	return s.out
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

// over composites the premultiplied src onto the opaque dst of the same
// size. Most HUD pixels are fully transparent or opaque, so those skip the
// blend; it is several times faster than draw.Draw for this case.
func over(dst, src *image.RGBA) {
	d, s := dst.Pix, src.Pix
	for i := 0; i+3 < len(s) && i+3 < len(d); i += 4 {
		a := uint32(s[i+3])
		switch a {
		case 0:
		case 0xff:
			d[i], d[i+1], d[i+2] = s[i], s[i+1], s[i+2]
		default:
			k := 255 - a
			d[i] = uint8(uint32(s[i]) + (uint32(d[i])*k+127)/255)
			d[i+1] = uint8(uint32(s[i+1]) + (uint32(d[i+1])*k+127)/255)
			d[i+2] = uint8(uint32(s[i+2]) + (uint32(d[i+2])*k+127)/255)
		}
	}
}

// gridSource shows a grid: every field is a source of its own (design, theme,
// animated background, media) on a canvas of the field's size, composed into
// the panel frame. A field is only redrawn when its source says its picture
// may have changed, so quiet fields cost nothing.
type gridSource struct {
	canvas *image.RGBA
	cells  []*gridCell
}

type gridCell struct {
	rect image.Rectangle
	src  source
	due  time.Duration // when the field needs a new frame
	have bool
}

func (w *worker) newGridSource(c PanelConfig, sz image.Point) (source, error) {
	g, ok := lcd.GridPresetByID(sz.X, sz.Y, c.Grid)
	if !ok || len(g.Cells) != len(c.Tiles) {
		return nil, fmt.Errorf("grid %q does not fit a %dx%d display", c.Grid, sz.X, sz.Y)
	}
	gs := &gridSource{canvas: image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y))}
	for i := 3; i < len(gs.canvas.Pix); i += 4 {
		gs.canvas.Pix[i] = 0xff
	}
	for i, cell := range g.Cells {
		rect := image.Rect(cell.X, cell.Y, cell.X+cell.W, cell.Y+cell.H)
		tc := c.Tiles[i].config(c.Theme)
		tc.Brightness, tc.Rotation = c.Brightness, 0
		src, err := w.newSourceAt(tc, rect.Size())
		if err != nil {
			w.setErr(ErrContent, fmt.Errorf("field %d: %w", i+1, err))
			src, _ = w.newSourceAt(PanelConfig{Mode: "off"}, rect.Size())
		}
		if ds, ok := src.(*designSource); ok {
			ds.reuse = true
		}
		gs.cells = append(gs.cells, &gridCell{rect: rect, src: src})
	}
	return gs, nil
}

func (g *gridSource) frame(t time.Duration) (*image.RGBA, time.Duration) {
	next := time.Hour
	for _, c := range g.cells {
		if !c.have || t >= c.due {
			img, wait := c.src.frame(t)
			draw.Draw(g.canvas, c.rect, img, img.Bounds().Min, draw.Src)
			c.due, c.have = t+wait, true
		}
		next = min(next, c.due-t)
	}
	return g.canvas, max(next, time.Millisecond)
}
