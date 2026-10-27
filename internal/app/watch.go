package app

import (
	"context"
	"log"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

// The watcher keeps the displays connected without the user pressing "search":
// it rescans when a supported device is plugged in or removed, when the
// displays keep failing, and after the PC woke from standby.
const (
	watchPoll       = 2 * time.Second  // how often the device list is looked at
	watchDebounce   = 2 * time.Second  // a changed device list must stay the same this long
	watchResume     = 3 * time.Second  // wait after wake-up before talking to the devices
	watchGap        = 10 * time.Second // a pause this long between two polls means the PC slept
	watchFailAfter  = 8 * time.Second  // device errors this long trigger a rescan
	watchBackoffMin = 10 * time.Second
	watchBackoffMax = 2 * time.Minute
	watchMaxRetries = 5                // error-driven rescans per plug event, then slow tries (watchSlowRetry) until a replug or a manual search
	watchSlowRetry  = 30 * time.Minute // after giving up: one more try this often (a rescan reopens every display)
)

// watcher is a small state machine; step is called with the current time.
type watcher struct {
	fingerprint func() (string, error) // supported devices that are plugged in
	failingFor  func() time.Time       // start of ongoing device errors, zero if none
	scanFailed  func() bool            // the last discovery found nothing although devices are present
	rescan      func()
	manual      func() int64 // counts manual searches; a new value re-arms the watcher (may be nil)
	onGaveUp    func(bool)   // reports the give-up state (may be nil)

	attempts   int  // error-driven rescans since the last plug event
	gaveUp     bool // attempts used up
	manualSeen int64
	started    bool
	known      string    // device list the panels were built from
	cand       string    // changed list waiting out the debounce
	candSince  time.Time // since when cand has been stable
	last       time.Time // time of the previous step
	resumeAt   time.Time // planned rescan after wake-up
	nextRetry  time.Time // earliest next error-driven rescan
	backoff    time.Duration
}

// step looks at the world once and rescans if needed; it reports whether it did.
func (w *watcher) step(now time.Time) bool {
	fp, err := w.fingerprint()
	defer func() { w.last = now }()
	if !w.started {
		if err == nil {
			w.known, w.started = fp, true
		}
		return false
	}
	// Wake-up: the loop didn't run for a long time.
	if !w.last.IsZero() && now.Sub(w.last) >= watchGap {
		w.resumeAt = now.Add(watchResume)
		w.cand = ""
	}
	if !w.resumeAt.IsZero() {
		if now.Before(w.resumeAt) {
			return false
		}
		w.resumeAt = time.Time{}
		return w.doRescan(now, true)
	}
	if err != nil {
		return false
	}
	// Plug or unplug: act once the list has been stable for the debounce time.
	if fp != w.known {
		if fp != w.cand {
			w.cand, w.candSince = fp, now
			return false
		}
		if now.Sub(w.candSince) >= watchDebounce {
			return w.doRescan(now, true)
		}
		return false
	}
	w.cand = ""
	// Devices are there but failing, or were never opened: try again, with back-off.
	failing := w.failingFor()
	broken := (!failing.IsZero() && now.Sub(failing) >= watchFailAfter) || (fp != "" && w.scanFailed())
	if w.manual != nil {
		if m := w.manual(); m != w.manualSeen {
			w.manualSeen = m
			w.setGaveUp(false)
		}
	}
	if !broken {
		w.backoff = 0
		return false
	}
	if now.Before(w.nextRetry) {
		return false
	}
	if w.gaveUp {
		// Rare, slow tries: a rescan of unchanged devices writes nothing, and
		// a device that came back without a change of the list is found.
		w.doRescan(now, false)
		w.nextRetry = now.Add(watchSlowRetry)
		return true
	}
	w.attempts++
	if w.attempts >= watchMaxRetries {
		defer w.setGaveUp(true)
	}
	return w.doRescan(now, false)
}

// setGaveUp changes the give-up state; leaving it also resets the counter.
func (w *watcher) setGaveUp(v bool) {
	if !v {
		w.attempts = 0
	}
	if w.gaveUp != v {
		w.gaveUp = v
		if w.onGaveUp != nil {
			w.onGaveUp(v)
		}
	}
}

func (w *watcher) doRescan(now time.Time, reset bool) bool {
	if reset {
		w.backoff = 0
		if fp, err := w.fingerprint(); err == nil {
			w.known = fp
		}
		w.cand = ""
		w.setGaveUp(false) // a plug event starts afresh
	} else {
		w.backoff = min(watchBackoffMax, max(watchBackoffMin, w.backoff*2))
	}
	log.Println("panels: rescanning")
	w.rescan()
	w.nextRetry = now.Add(max(w.backoff, watchBackoffMin))
	return true
}

func (w *watcher) run(ctx context.Context) {
	t := time.NewTicker(watchPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if w.step(time.Now()) {
			w.last = time.Now() // a long rescan is not a sleep
		}
	}
}

// runWatch starts the watcher for the engine's panels.
func (e *Engine) runWatch(ctx context.Context) {
	w := &watcher{
		fingerprint: panel.Fingerprint,
		failingFor:  e.deviceFailingSince,
		scanFailed: func() bool {
			e.mu.Lock()
			failed := e.scanErr != ""
			e.mu.Unlock()
			return failed || e.displayUnopened()
		},
		rescan:   e.Rescan,
		manual:   e.manualRescans.Load,
		onGaveUp: e.watchGaveUp.Store,
	}
	w.run(ctx)
}

// ManualRescan is the user's "search again": it also re-arms the watcher.
func (e *Engine) ManualRescan() {
	e.manualRescans.Add(1)
	e.Rescan()
}

// RescanGaveUp reports that the automatic rescans for failing devices
// stopped; they continue after a replug or a manual search.
func (e *Engine) RescanGaveUp() bool { return e.watchGaveUp.Load() }

// Seam for tests: the display paths that are present but not open.
var unopenedDisplays = panel.UnopenedDisplays

// displayUnopened reports a present display that no panel uses, for example
// one that re-enumerated after a USB hub dropped out and was not found by the
// rescan that ran meanwhile. It is quiet while a rescan runs (the workers are
// being replaced then) and never opens anything.
func (e *Engine) displayUnopened() bool {
	if !e.outputAllowed() || e.Demo {
		return false
	}
	if !e.rescanMu.TryLock() {
		return false
	}
	defer e.rescanMu.Unlock()
	e.mu.Lock() // Stop replaces the workers without rescanMu
	open := make([]panel.Panel, 0, len(e.workers))
	for _, w := range e.workers {
		open = append(open, w.p)
	}
	e.mu.Unlock()
	return len(unopenedDisplays(open)) > 0
}
