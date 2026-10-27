package app

import (
	"fmt"
	"log"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

// The offline clock is a flag that some bar displays keep in their own memory:
// they show a clock when no host feeds them. It is a permanent write, so it
// goes to the display only when the user changes the setting (and the display
// does not hold the value already, see panel.SettingsCache); starting
// CaseHUD, switching scenes or a rescan never write it. Quick clicks are
// collapsed: the value goes out once the setting has stayed put for a moment.

// offlineClockDelay is how long a change waits for further changes.
var offlineClockDelay = 1500 * time.Millisecond

// SetOfflineClock stores the offline-clock setting of a bar display and, after
// a short pause, writes it to the display.
func (e *Engine) SetOfflineClock(id string, on bool) error {
	e.mu.Lock()
	w, ok := e.workers[id]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrNoPanel, id)
	}
	if _, can := w.p.(panel.OfflineClocker); !can {
		e.mu.Unlock()
		return fmt.Errorf("%q has no offline clock", id)
	}
	if e.cfg.OfflineClock == nil {
		e.cfg.OfflineClock = map[string]bool{}
	}
	if on {
		e.cfg.OfflineClock[id] = true
	} else {
		delete(e.cfg.OfflineClock, id)
	}
	w.ocMu.Lock()
	if w.ocTimer != nil {
		w.ocTimer.Stop()
	}
	w.ocTimer = time.AfterFunc(offlineClockDelay, func() { w.writeOfflineClock(true) })
	w.ocMu.Unlock()
	e.mu.Unlock()
	return e.persist()
}

// wantOfflineClock is the stored setting of the worker's panel.
func (w *worker) wantOfflineClock() bool {
	w.e.mu.Lock()
	defer w.e.mu.Unlock()
	return w.e.cfg.OfflineClock[w.p.ID()]
}

// syncOfflineClock brings the display in line with the stored setting; it
// writes only if the display does not hold the value already.
func (w *worker) syncOfflineClock() { w.writeOfflineClock(false) }

func (w *worker) writeOfflineClock(user bool) {
	oc, ok := w.p.(panel.OfflineClocker)
	if !ok {
		return
	}
	if !user && !w.wantOfflineClock() {
		return // never switched off without the user's request
	}
	if err := oc.SetOfflineClock(w.wantOfflineClock()); err != nil {
		log.Printf("%s: offline clock: %v", w.p.Name(), err)
	}
}
