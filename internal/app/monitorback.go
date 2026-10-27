package app

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

// A bar display that is in monitor mode is not a CaseHUD display. The window
// lists it and offers the way back (one restart report, see
// panel.BringBackFromMonitorMode). Listing only reads; the report goes out
// only on the user's click, once per monitorBackEvery.

const (
	monitorBackEvery = 10 * time.Second // between two attempts
	monitorListTTL   = 5 * time.Second  // how long a listing is reused
)

// ErrOutputHeld is returned while no device output is allowed (disclaimer not accepted).
var ErrOutputHeld = errors.New("device output is not allowed yet")

// ErrMonitorBackWait is returned when the last attempt is less than monitorBackEvery ago.
var ErrMonitorBackWait = errors.New("wait a moment before trying again")

// Seams for tests.
var (
	findMonitorMode  = panel.FindMonitorMode
	bringBackMonitor = panel.BringBackFromMonitorMode
	monitorRescan    = 4 * time.Second // wait before the displays are searched again
)

type monitorModeState struct {
	mu     sync.Mutex
	at     time.Time // when list was read
	list   []panel.MonitorModeDevice
	lastTx time.Time // last attempt
}

// MonitorModeDevices lists the bar displays in monitor mode (nothing before
// the disclaimer is accepted, and not in the demo). The listing is reused for
// a few seconds so the state polling does not enumerate all the time.
func (e *Engine) MonitorModeDevices() []panel.MonitorModeDevice {
	if e.Demo || !e.outputAllowed() {
		return []panel.MonitorModeDevice{}
	}
	m := &e.monitor
	m.mu.Lock()
	if m.list != nil && time.Since(m.at) <= monitorListTTL {
		out := append([]panel.MonitorModeDevice{}, m.list...)
		m.mu.Unlock()
		return out
	}
	m.mu.Unlock()
	// The enumeration (slow USB access) runs without the lock, so a click on
	// "bring back" is not held up by the status polling.
	list := findMonitorMode()
	if list == nil {
		list = []panel.MonitorModeDevice{}
	}
	m.mu.Lock()
	m.list, m.at = list, time.Now()
	m.mu.Unlock()
	return append([]panel.MonitorModeDevice{}, list...)
}

// BringBackMonitorMode sends the restart report to the display at path (one
// that is listed right now) and searches for displays again a few seconds later.
func (e *Engine) BringBackMonitorMode(path string) error {
	if e.Demo || !e.outputAllowed() {
		return ErrOutputHeld
	}
	m := &e.monitor
	m.mu.Lock()
	if !m.lastTx.IsZero() && time.Since(m.lastTx) < monitorBackEvery {
		m.mu.Unlock()
		return ErrMonitorBackWait
	}
	m.lastTx = time.Now()
	m.list = nil // listing is stale after this
	m.mu.Unlock()
	if err := bringBackMonitor(path); err != nil {
		return err
	}
	log.Println("display in monitor mode: restart report sent")
	time.AfterFunc(monitorRescan, func() {
		if e.ctx != nil && e.ctx.Err() == nil {
			e.ManualRescan()
		}
	})
	return nil
}
