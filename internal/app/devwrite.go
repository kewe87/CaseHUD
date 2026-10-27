package app

import (
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

// Brightness and rotation are written to the device, and several devices
// keep them in flash memory, which survives only a limited number of writes.
// devWrites sits between the configuration and the panel: requests are
// collected, at most one write per panel and deviceWriteGap goes out, the last
// requested values win, and values the panel was already given are not sent
// again. (Panels that store settings also compare against what they were last
// given across restarts, see panel.SettingsCache.)
type devWrites struct {
	mu      sync.Mutex
	flushMu sync.Mutex // one flush at a time; held during device calls
	lastAt  time.Time  // when the last write went out
	pend    *devReq    // newest request not yet sent
	timer   *time.Timer
	closed  bool
	sentB   int
	sentR   int
	hasB    bool
	hasR    bool
}

type devReq struct{ brightness, rotation int }

// deviceWriteGap is the shortest time between two settings writes to a panel.
var deviceWriteGap = panel.SettingsRateLimit

// request records the values and sends them now or, when the last write was
// less than deviceWriteGap ago, once the gap has passed.
func (w *worker) writeDevice(brightness, rotation int) {
	d := &w.dw
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.pend = &devReq{brightness, rotation}
	if d.timer != nil {
		d.mu.Unlock() // the pending timer sends the newest values
		return
	}
	wait := deviceWriteGap - time.Since(d.lastAt)
	if wait > 0 {
		d.timer = time.AfterFunc(wait, w.flushDevice)
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()
	w.flushDevice()
}

func (w *worker) flushDevice() {
	d := &w.dw
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.mu.Lock()
	d.timer = nil
	req := d.pend
	d.pend = nil
	closed := d.closed
	b, r := d.sentB, d.sentR
	hasB, hasR := d.hasB, d.hasR
	d.mu.Unlock()
	if req == nil || closed {
		return
	}
	wrote := false
	if !hasB || req.brightness != b {
		wrote = true
		if err := w.p.SetBrightness(req.brightness); err != nil {
			w.setErr(ErrDevice, err)
		} else {
			d.mu.Lock()
			d.sentB, d.hasB = req.brightness, true
			d.mu.Unlock()
		}
	}
	if (!hasR && req.rotation != 0) || (hasR && req.rotation != r) {
		wrote = true
		if err := w.p.SetRotation(req.rotation); err != nil {
			w.setErr(ErrDevice, err)
		} else {
			d.mu.Lock()
			d.sentR, d.hasR = req.rotation, true
			d.mu.Unlock()
		}
	}
	if !hasR {
		// The first rotation request: the panel starts unrotated.
		d.mu.Lock()
		if !d.hasR {
			d.sentR, d.hasR = req.rotation, true
		}
		d.mu.Unlock()
	}
	if wrote {
		d.mu.Lock()
		d.lastAt = time.Now()
		d.mu.Unlock()
	}
}

// close ends the pending write: the panel is about to be closed.
func (d *devWrites) close() {
	d.mu.Lock()
	d.closed = true
	d.pend = nil
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.mu.Unlock()
	d.flushMu.Lock() // wait for a write in flight
	d.flushMu.Unlock()
}
