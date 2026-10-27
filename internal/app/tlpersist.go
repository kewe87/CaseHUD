// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/icon"
	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/panel"
)

// The UNI FAN TL LCD keeps a start image in its own memory: what it shows when
// no host feeds it. CaseHUD writes its logo there once per fan (a permanent
// write), when the stored hash of that fan is missing or differs from the hash
// of the current image, at most once per fan and app run, never before the
// disclaimer was accepted and never while the setting is off.

const (
	tlStartSize = 400
	// tlStartVersion is part of the hash: raise it when the drawing changes.
	tlStartVersion = "1"
)

// tlpState is the per-run bookkeeping of the start image writes.
type tlpState struct {
	run   sync.Mutex      // one write series at a time
	mu    sync.Mutex      // guards tried
	tried map[string]bool // fan keys a write was attempted for in this run
	gen   atomic.Int64    // raised when the panels are closed: older series stop before their next step
}

// endStartImages is called before panels are closed (Rescan, Stop): a series
// running for the old panels starts no further step, and the transfer in
// progress is given time to finish so a fan is not left in transfer lock.
func (e *Engine) endStartImages() {
	e.tlp.gen.Add(1)
	deadline := time.Now().Add(15 * time.Second)
	for !e.tlp.run.TryLock() {
		if time.Now().After(deadline) {
			log.Println("start image: write still running, closing anyway")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.tlp.run.Unlock()
}

// startImagePixels draws the start image: the logo on a dark background, in
// the middle of the round safe area, the name below it.
func startImagePixels() (*image.RGBA, error) {
	l := lcd.DefaultLayout()
	l.Width, l.Height = tlStartSize, tlStartSize
	l.Background, l.Panel, l.Round = "#0b0f14", "", true
	l.Widgets = []lcd.Widget{
		{Type: "text", X: 100, Y: 296, W: 200, H: 40, Text: "CaseHUD", Size: 34, Align: "center", Color: "#e6edf3"},
	}
	r, err := lcd.NewRenderer(l)
	if err != nil {
		return nil, err
	}
	img := r.Render(&metrics.Func{})
	if img.Bounds().Dx() != tlStartSize || img.Bounds().Dy() != tlStartSize {
		return nil, fmt.Errorf("start image has size %v", img.Bounds())
	}
	const logo = 220
	at := image.Pt((tlStartSize-logo)/2, 52)
	draw.Draw(img, image.Rectangle{Min: at, Max: at.Add(image.Pt(logo, logo))}, icon.Image(logo), image.Point{}, draw.Over)
	return img, nil
}

// StartImage returns the JPEG to write and its hash. The hash covers the
// pixels, not the encoder's output, so a toolchain update does not trigger
// a new write.
func StartImage() (jpg []byte, hash string, err error) {
	img, err := startImagePixels()
	if err != nil {
		return nil, "", err
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, "", err
	}
	if b.Len() > 65535 {
		return nil, "", fmt.Errorf("start image too large (%d bytes)", b.Len())
	}
	h := sha256.New()
	h.Write([]byte("casehud-tl-start-image-" + tlStartVersion + "\n"))
	h.Write(img.Pix)
	return b.Bytes(), hex.EncodeToString(h.Sum(nil))[:32], nil
}

// TLStartImage reports whether the start image is written (default on).
func (e *Engine) TLStartImage() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.cfg.TLStartImageOff
}

// SetTLStartImage switches the start image on or off. Switching it on writes
// to fans that do not have the current image yet.
func (e *Engine) SetTLStartImage(on bool) error {
	e.mu.Lock()
	e.cfg.TLStartImageOff = !on
	e.mu.Unlock()
	if err := e.persist(); err != nil {
		return err
	}
	if on {
		e.startImagesAfterScan(e.connectedPanels())
	}
	return nil
}

// TLSerialAssign reports whether a serial number is assigned to TL LCD fans
// that have none (default off).
func (e *Engine) TLSerialAssign() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.TLSerialOn
}

// SetTLSerialAssign switches the serial number assignment on or off. It takes
// effect at the next device search.
func (e *Engine) SetTLSerialAssign(on bool) error {
	e.mu.Lock()
	e.cfg.TLSerialOn, e.cfg.TLSerialOff = on, !on
	panel.SetOptions(panelOptions(e.cfg)) // under the lock, like the other options
	e.mu.Unlock()
	return e.persist()
}

// RewriteStartImages writes the image to every connected TL LCD once more,
// whatever is recorded. It returns the number of fans it will write to.
// confirm must be true: the caller has shown the user the permanent-write
// warning.
func (e *Engine) RewriteStartImages(confirm bool) (int, error) {
	if !confirm {
		return 0, fmt.Errorf("%w: confirm the permanent write first", ErrAckRequired)
	}
	if !e.outputAllowed() {
		return 0, fmt.Errorf("%w: accept the disclaimer in the CaseHUD window first", ErrAckRequired)
	}
	ps := e.connectedPanels()
	n := 0
	for _, p := range ps {
		if _, ok := p.(panel.StartImager); ok {
			n++
		}
	}
	if n > 0 {
		go e.writeStartImages(ps, true)
	}
	return n, nil
}

func (e *Engine) connectedPanels() []panel.Panel {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]panel.Panel, 0, len(e.order))
	for _, id := range e.order {
		if w := e.workers[id]; w != nil {
			out = append(out, w.p)
		}
	}
	return out
}

// startImagesAfterScan is called after every discovery (start, hot-plug,
// manual search). It never blocks: the writes run in the background.
func (e *Engine) startImagesAfterScan(ps []panel.Panel) {
	if e.Demo || len(ps) == 0 {
		return
	}
	go e.writeStartImages(ps, false)
}

// writeStartImages writes the start image to the TL LCDs in ps that need it.
// force ignores the record and the once-per-run rule (the user asked).
func (e *Engine) writeStartImages(ps []panel.Panel, force bool) {
	gen := e.tlp.gen.Load()
	e.tlp.run.Lock()
	defer e.tlp.run.Unlock()
	if !e.outputAllowed() || e.ctx == nil || e.ctx.Err() != nil {
		return
	}
	if !force && !e.TLStartImage() {
		return
	}
	var jpg []byte
	var hash string
	for _, p := range ps {
		si, ok := p.(panel.StartImager)
		if !ok {
			continue
		}
		if e.tlp.gen.Load() != gen || e.ctx.Err() != nil {
			return // the panels were closed meanwhile
		}
		key := si.SettingsKey()
		if key == "" {
			key = p.ID()
		}
		if jpg == nil {
			var err error
			if jpg, hash, err = StartImage(); err != nil {
				log.Println("start image:", err)
				return
			}
		}
		e.writeOne(p, key, hash, jpg, force, "start image", "", si.WriteStartImage)
		if e.tlp.gen.Load() != gen || e.ctx.Err() != nil {
			return
		}
		if ri, ok := p.(panel.RestImager); ok {
			e.writeOne(p, key, hash, jpg, force, "stored image", "rest", ri.WriteRestImage)
		}
	}
}

// writeOne writes one picture to one fan unless the record or the once-per-run
// rule says it is done. kind "" is the start image (tl_start_images), "rest"
// the stored picture shown when CaseHUD is not running (tl_rest_images).
func (e *Engine) writeOne(p panel.Panel, key, hash string, jpg []byte, force bool, what, kind string, write func([]byte) error) {
	triedKey := kind + "|" + key
	e.mu.Lock()
	have := e.cfg.TLStartImages[key]
	if kind == "rest" {
		have = e.cfg.TLRestImages[key]
	}
	e.mu.Unlock()
	e.tlp.mu.Lock()
	tried := e.tlp.tried[triedKey]
	e.tlp.mu.Unlock()
	if !force && (tried || have == hash) {
		return
	}
	err := write(jpg)
	if errors.Is(err, panel.ErrClosed) {
		return // nothing reached the fan; the next scan tries again
	}
	e.tlp.mu.Lock()
	if e.tlp.tried == nil {
		e.tlp.tried = map[string]bool{}
	}
	e.tlp.tried[triedKey] = true
	e.tlp.mu.Unlock()
	if errors.Is(err, panel.ErrImageWritten) {
		// The image went in and was acknowledged; only the follow-up
		// settings were not confirmed. Record it so it is not written again.
		log.Printf("%s written to %s (warning: %v)", what, p.Name(), err)
	} else if err != nil {
		log.Printf("%s on %s: %v", what, p.Name(), err)
		return
	} else {
		log.Printf("%s written to %s", what, p.Name())
	}
	e.mu.Lock()
	if kind == "rest" {
		if e.cfg.TLRestImages == nil {
			e.cfg.TLRestImages = map[string]string{}
		}
		e.cfg.TLRestImages[key] = hash
	} else {
		if e.cfg.TLStartImages == nil {
			e.cfg.TLStartImages = map[string]string{}
		}
		e.cfg.TLStartImages[key] = hash
	}
	e.mu.Unlock()
	if err := e.persist(); err != nil {
		log.Println(what+" record:", err)
	}
}
