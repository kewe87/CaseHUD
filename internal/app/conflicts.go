package app

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/conflicts"
)

// Timing of the process check: one look shortly after the start, then a
// cheap snapshot every few seconds.
const (
	conflictFirst    = time.Second
	conflictInterval = 5 * time.Second
	// conflictRelease is how long the closed program gets to let go of its
	// devices before CaseHUD searches again.
	conflictRelease = 1500 * time.Millisecond
)

// confState is the process check of the engine.
type confState struct {
	once sync.Once
	mu   sync.RWMutex // guards mgr (the HTTP handlers read it while Start sets it)
	mgr  *conflicts.Manager
	sys  conflicts.System // tests replace it before Start
	// rescanning: a search after a close is already waiting (coalesces).
	rescanning atomic.Bool
}

func (e *Engine) confMgr() *conflicts.Manager {
	e.conf.mu.RLock()
	defer e.conf.mu.RUnlock()
	return e.conf.mgr
}

// conflictSystem is the process access: the demo list in demo mode.
func (e *Engine) conflictSystem() conflicts.System {
	if e.conf.sys != nil {
		return e.conf.sys
	}
	if e.Demo {
		return conflicts.NewDemoSystem()
	}
	return conflicts.NewSystem()
}

// conflictSettings is what the user chose, as the manager reads it.
func (e *Engine) conflictSettings() conflicts.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := conflicts.Settings{Ask: !e.cfg.ConflictsAskOff, Ignored: map[string]bool{}, Active: map[string]bool{}}
	for _, x := range e.cfg.ConflictsIgnore {
		s.Ignored[strings.ToLower(x)] = true
	}
	for _, d := range []string{conflicts.DevTLLCD, conflicts.DevUS88, conflicts.DevAIO} {
		s.Active[d] = true
	}
	if e.cfg.Lighting.Enabled {
		for _, d := range conflicts.LightingDevices {
			s.Active[d] = true
		}
	}
	return s
}

// startConflicts starts the periodic check (once per engine).
func (e *Engine) startConflicts() {
	e.conf.once.Do(func() {
		e.mu.Lock()
		e.cfg.ConflictsIgnore = cleanConflictIgnore(e.cfg.ConflictsIgnore)
		e.mu.Unlock()
		m := &conflicts.Manager{Sys: e.conflictSystem(), Settings: e.conflictSettings}
		m.OnClosed = e.afterConflictClosed
		if e.Demo && e.conf.sys == nil {
			m.Roots = []string{`C:\Demo`} // the made-up list lives there
		}
		e.conf.mu.Lock()
		e.conf.mgr = m
		e.conf.mu.Unlock()
		go func() {
			t := time.NewTimer(conflictFirst)
			defer t.Stop()
			for {
				select {
				case <-e.ctx.Done():
					return
				case <-t.C:
				}
				m.Scan()
				t.Reset(conflictInterval)
			}
		}()
	})
}

// scanConflicts refreshes the picture right now (before a device search).
func (e *Engine) scanConflicts() {
	if m := e.confMgr(); m != nil {
		m.Scan()
	}
}

// afterConflictClosed searches for displays and lighting again once the
// program let go of its devices.
func (e *Engine) afterConflictClosed(exe string) {
	log.Printf("conflicts: %s closed, searching for devices again", exe)
	if !e.conf.rescanning.CompareAndSwap(false, true) {
		return
	}
	defer e.conf.rescanning.Store(false)
	select {
	case <-e.ctx.Done():
		return
	case <-time.After(conflictRelease):
	}
	e.Rescan()
	e.mu.Lock()
	light := e.cfg.Lighting.Enabled
	e.mu.Unlock()
	if light {
		if err := e.RescanLighting(); err != nil {
			log.Println("conflicts: lighting search:", err)
		}
	}
}

// ConflictsStatus lists the known programs and which of them run.
func (e *Engine) ConflictsStatus() conflicts.Status {
	m := e.confMgr()
	if m == nil {
		return conflicts.Status{Ask: true, Items: []conflicts.Item{}}
	}
	return m.Status()
}

// ConflictsRunning lists the image names of the running programs of the list
// (names only, for the read-only API).
func (e *Engine) ConflictsRunning() []string {
	m := e.confMgr()
	if m == nil {
		return []string{}
	}
	return m.Running()
}

// CloseConflict asks a program of the list to close (hard: ends it). Only the
// GUI calls this, on an explicit click.
func (e *Engine) CloseConflict(exe string, hard, allowUnusual bool) (conflicts.Result, error) {
	m := e.confMgr()
	if m == nil {
		return conflicts.Result{}, conflicts.ErrUnknown
	}
	return m.CloseConfirmed(exe, hard, allowUnusual)
}

// SetConflictIgnore adds or removes a program of the list to or from the
// "do not close" list.
func (e *Engine) SetConflictIgnore(exe string, on bool) error {
	ent, ok := conflicts.Lookup(exe)
	if !ok {
		return fmt.Errorf("unknown program %q", exe)
	}
	e.mu.Lock()
	list := slices.DeleteFunc(slices.Clone(e.cfg.ConflictsIgnore), func(s string) bool { return strings.EqualFold(s, ent.Exe) })
	if on {
		list = append(list, ent.Exe)
	}
	slices.Sort(list)
	e.cfg.ConflictsIgnore = list
	e.mu.Unlock()
	return e.persist()
}

// SetConflictsAsk switches the banner and the question on or off.
func (e *Engine) SetConflictsAsk(on bool) error {
	e.mu.Lock()
	e.cfg.ConflictsAskOff = !on
	e.mu.Unlock()
	return e.persist()
}

// cleanConflictIgnore drops entries that are not in the list (a hand-edited
// config cannot name other programs).
func cleanConflictIgnore(in []string) []string {
	var out []string
	for _, s := range in {
		if ent, ok := conflicts.Lookup(s); ok && !slices.Contains(out, ent.Exe) {
			out = append(out, ent.Exe)
		}
	}
	return out
}
