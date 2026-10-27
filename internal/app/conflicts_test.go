package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/kewe87/casehud/internal/conflicts"
)

// procSys is a process list with window and terminate calls recorded.
type procSys struct {
	mu     sync.Mutex
	procs  []conflicts.Proc
	posted []string
}

func (s *procSys) Processes(want func(string) bool) ([]conflicts.Proc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []conflicts.Proc
	for _, p := range s.procs {
		if want(p.Exe) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *procSys) PostClose(p conflicts.Proc) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posted = append(s.posted, p.Exe)
	s.procs = slices.DeleteFunc(s.procs, func(q conflicts.Proc) bool { return q.PID == p.PID })
	return 1, nil
}

func (s *procSys) Terminate(conflicts.Proc) error { return errors.New("unused") }

func conflictEngine(t *testing.T, sys conflicts.System) *Engine {
	t.Helper()
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	e.conf.sys = sys
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e
}

func TestConflictsStatusFollowsLighting(t *testing.T) {
	sys := &procSys{procs: []conflicts.Proc{
		{PID: 1, Exe: "OpenRGB.exe", Created: 1},
		{PID: 2, Exe: "L-Connect 3.exe", Created: 1},
	}}
	e := conflictEngine(t, sys)
	e.scanConflicts()
	pending := func() []string {
		var out []string
		for _, it := range e.ConflictsStatus().Items {
			if it.Running && !it.Ignored && len(it.Affects) > 0 {
				out = append(out, it.Exe)
			}
		}
		return out
	}
	if got := pending(); !slices.Equal(got, []string{"L-Connect 3.exe"}) {
		t.Fatalf("lighting off: pending = %v", got)
	}
	e.mu.Lock()
	e.cfg.Lighting.Enabled = true
	e.mu.Unlock()
	if got := pending(); len(got) != 2 {
		t.Fatalf("lighting on: pending = %v", got)
	}
	if names := e.ConflictsRunning(); len(names) != 2 {
		t.Fatalf("running = %v", names)
	}
}

func TestConflictIgnoreAndAsk(t *testing.T) {
	e := conflictEngine(t, &procSys{})
	if err := e.SetConflictIgnore("notepad.exe", true); err == nil {
		t.Fatal("an unlisted program was accepted")
	}
	if err := e.SetConflictIgnore("openrgb.EXE", true); err != nil {
		t.Fatal(err)
	}
	if err := e.SetConflictIgnore("OpenRGB.exe", true); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	got := slices.Clone(e.cfg.ConflictsIgnore)
	e.mu.Unlock()
	if !slices.Equal(got, []string{"OpenRGB.exe"}) {
		t.Fatalf("ignore = %v", got)
	}
	if err := e.SetConflictsAsk(false); err != nil || e.ConflictsStatus().Ask {
		t.Fatalf("ask not switched off: %v", err)
	}
	if err := e.SetConflictIgnore("OpenRGB.exe", false); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	n := len(e.cfg.ConflictsIgnore)
	e.mu.Unlock()
	if n != 0 {
		t.Fatal("not removed")
	}
	// the stored values survive a restart; foreign names do not
	if got := cleanConflictIgnore([]string{"OpenRGB.exe", "cmd.exe", "openrgb.exe"}); !slices.Equal(got, []string{"OpenRGB.exe"}) {
		t.Fatalf("clean = %v", got)
	}
}

func TestCloseConflictRescans(t *testing.T) {
	sys := &procSys{procs: []conflicts.Proc{{PID: 1, Exe: "SignalRgb.exe", Created: 1}}}
	e := conflictEngine(t, sys)
	e.scanConflicts()
	// no readable path: unusual, refused without the second confirmation
	if _, err := e.CloseConflict("SignalRgb.exe", false, false); !errors.Is(err, conflicts.ErrUnusualPath) {
		t.Fatalf("unconfirmed: %v", err)
	}
	res, err := e.CloseConflict("SignalRgb.exe", false, true)
	if err != nil || res.Status != "closed" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if len(e.ConflictsRunning()) != 0 {
		t.Fatal("still listed")
	}
	if _, err := e.CloseConflict("explorer.exe", false, false); !errors.Is(err, conflicts.ErrUnknown) {
		t.Fatalf("err = %v", err)
	}
	if len(sys.posted) != 1 {
		t.Fatalf("posted = %v", sys.posted)
	}
}
