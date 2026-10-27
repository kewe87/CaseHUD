package conflicts

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Proc is one running process as the snapshot sees it.
type Proc struct {
	PID uint32
	Exe string // image name from the snapshot
	// Path is the full image path ("" when it could not be read).
	Path string
	// Created is the start time (FILETIME ticks); together with the PID it
	// identifies the process, so a reused PID is never mistaken for it.
	Created uint64
	// Service: runs in session 0 (a Windows service).
	Service bool
	// Restricted: this session may not end it (runs elevated or as another user).
	Restricted bool
}

// System is what the manager needs from the operating system. The Windows
// implementation is in sys_windows.go; tests use a fake.
type System interface {
	// Processes lists the running processes whose image name want accepts.
	Processes(want func(exe string) bool) ([]Proc, error)
	// PostClose asks the program's top-level windows to close (WM_CLOSE) and
	// reports how many it asked. It refuses when p is no longer that process.
	PostClose(p Proc) (int, error)
	// Terminate ends the process hard. It refuses when p is no longer that
	// process (PID, start time and image are checked on the opened handle).
	Terminate(p Proc) error
}

// Settings is what the user chose; the manager reads it on every use.
type Settings struct {
	// Ask: show the banner and the dialog.
	Ask bool
	// Ignored holds lower-case image names the user does not want to close.
	Ignored map[string]bool
	// Active holds the device keys that matter right now (lighting devices
	// only while lighting is on). Nil means all.
	Active map[string]bool
}

// Item is one catalog program with its current state.
type Item struct {
	Entry
	Running   bool `json:"running"`
	Instances int  `json:"instances"`
	// Closable is false while an instance runs as a service or elevated.
	Closable bool `json:"closable"`
	// Reason is "service" or "elevated" when Closable is false.
	Reason  string `json:"reason,omitempty"`
	Ignored bool   `json:"ignored"`
	// Paths are the distinct image paths of the running instances ("" when a
	// path could not be read is shown as "?").
	Paths []string `json:"paths"`
	// Unusual: an instance runs from outside the usual install locations
	// (see TrustedRoots); closing it needs a second explicit confirmation.
	Unusual bool `json:"unusual"`
	// Affects are the devices of the program that matter right now.
	Affects []string `json:"affects"`
}

// Status is the whole picture for the GUI.
type Status struct {
	Ask   bool   `json:"ask"`
	Items []Item `json:"items"`
	// Pending counts running programs that matter and are not ignored.
	Pending int `json:"pending"`
}

// Result is the outcome of a close request.
type Result struct {
	Exe string `json:"exe"`
	// Status: closed, still_running, needs_admin, not_running.
	Status string `json:"status"`
	// Reason is "service" or "elevated" for needs_admin.
	Reason string `json:"reason,omitempty"`
}

// Errors of Close.
var (
	ErrUnknown   = errors.New("unknown program")
	ErrNeedsSoft = errors.New("ask the program to close first")
	ErrBusy      = errors.New("another close request is running")
	// ErrUnusualPath: the program runs from an unusual location and the caller
	// did not confirm the second warning.
	ErrUnusualPath = errors.New("unusual location: confirm first")
)

// Manager keeps the process picture and closes programs on request.
type Manager struct {
	Sys      System
	Settings func() Settings
	// OnClosed is called (in its own goroutine) after a program went away.
	OnClosed func(exe string)

	// Timings (zero values mean the defaults).
	GraceTimeout time.Duration // wait after the polite request
	HardTimeout  time.Duration // wait after TerminateProcess
	Poll         time.Duration
	// Roots are the folders programs are normally installed in; nil means
	// TrustedRoots(). A program running from elsewhere is only closed after
	// an extra confirmation.
	Roots []string
	// Logf receives every action; nil means the standard logger.
	Logf func(format string, args ...any)

	mu       sync.Mutex
	last     []Proc
	scanned  bool
	reported map[string]bool
	soft     map[ident]time.Time // polite request per process, enables the hard one
	closeMu  sync.Mutex          // one close request at a time
}

const softWindow = 10 * time.Minute

// TrustedRoots are the usual install locations: Program Files, Program Files
// (x86), WindowsApps (below Program Files), ProgramData and the per-user
// AppData\Local\Programs.
func TrustedRoots() []string {
	var out []string
	for _, k := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "ProgramData"} {
		if v := os.Getenv(k); v != "" {
			out = append(out, v)
		}
	}
	if v := os.Getenv("ProgramFiles"); v != "" {
		out = append(out, filepath.Join(v, "WindowsApps"))
	}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		out = append(out, filepath.Join(v, "Programs"))
	}
	return out
}

func normPath(p string) string {
	return strings.ToLower(strings.TrimRight(strings.ReplaceAll(p, "/", `\`), `\`))
}

// usualPath reports whether path lies below one of the roots. An unknown path
// is not usual.
func usualPath(path string, roots []string) bool {
	if path == "" {
		return false
	}
	p := normPath(path)
	for _, r := range roots {
		if r = normPath(r); r != "" && strings.HasPrefix(p, r+`\`) {
			return true
		}
	}
	return false
}

func (m *Manager) roots() []string {
	if m.Roots != nil {
		return m.Roots
	}
	return TrustedRoots()
}

// ident names one process: PID plus start time.
type ident struct {
	pid     uint32
	created uint64
}

// pruneSoft drops the polite requests older than the window; m.mu is held.
func (m *Manager) pruneSoft() {
	for k, at := range m.soft {
		if time.Since(at) > softWindow {
			delete(m.soft, k)
		}
	}
}

func (m *Manager) logf(f string, a ...any) {
	if m.Logf != nil {
		m.Logf(f, a...)
		return
	}
	log.Printf(f, a...)
}

func dur(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

func (m *Manager) settings() Settings {
	if m.Settings == nil {
		return Settings{Ask: true}
	}
	return m.Settings()
}

// Scan takes a fresh snapshot (cheap: only catalog programs are opened).
func (m *Manager) Scan() {
	procs, err := m.Sys.Processes(isListed)
	if err != nil {
		return
	}
	// Only listed programs, whatever the system returned.
	var keep []Proc
	for _, p := range procs {
		if isListed(p.Exe) {
			keep = append(keep, p)
		}
	}
	var news []Proc
	m.mu.Lock()
	m.last, m.scanned = keep, true
	m.pruneSoft()
	if m.reported == nil {
		m.reported = map[string]bool{}
	}
	seen := map[string]bool{}
	for _, p := range keep {
		k := strings.ToLower(p.Exe)
		seen[k] = true
		if !m.reported[k] {
			m.reported[k] = true
			news = append(news, p)
		}
	}
	for k := range m.reported {
		if !seen[k] {
			delete(m.reported, k)
		}
	}
	m.mu.Unlock()
	for _, p := range news {
		m.logf("conflicts: %s is running (pid %d)", p.Exe, p.PID)
	}
}

// Scanned reports whether at least one snapshot was taken.
func (m *Manager) Scanned() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scanned
}

// Status describes every catalog program against the last snapshot.
func (m *Manager) Status() Status {
	set := m.settings()
	m.mu.Lock()
	last := append([]Proc(nil), m.last...)
	m.mu.Unlock()
	st := Status{Ask: set.Ask, Items: []Item{}}
	roots := m.roots()
	for _, e := range Catalog() {
		it := Item{Entry: e, Closable: true, Affects: []string{}, Paths: []string{}}
		for _, p := range last {
			if !sameName(p.Exe, e.Exe) {
				continue
			}
			it.Running = true
			it.Instances++
			shown := p.Path
			if shown == "" {
				shown = "?"
			}
			if !slices.Contains(it.Paths, shown) {
				it.Paths = append(it.Paths, shown)
			}
			if !usualPath(p.Path, roots) {
				it.Unusual = true
			}
			if p.Service || e.Service {
				it.Closable, it.Reason = false, "service"
			} else if p.Restricted && it.Reason != "service" {
				it.Closable, it.Reason = false, "elevated"
			}
		}
		it.Ignored = set.Ignored[strings.ToLower(e.Exe)]
		for _, d := range e.Devices {
			if set.Active == nil || set.Active[d] {
				it.Affects = append(it.Affects, d)
			}
		}
		if it.Running && !it.Ignored && len(it.Affects) > 0 {
			st.Pending++
		}
		st.Items = append(st.Items, it)
	}
	return st
}

// Running lists the image names of the running catalog programs (names only).
func (m *Manager) Running() []string {
	out := []string{}
	for _, it := range m.Status().Items {
		if it.Running {
			out = append(out, it.Exe)
		}
	}
	return out
}

// matching returns the running instances of exe from a fresh snapshot.
func (m *Manager) matching(exe string) ([]Proc, error) {
	procs, err := m.Sys.Processes(func(n string) bool { return sameName(n, exe) })
	if err != nil {
		return nil, err
	}
	var out []Proc
	for _, p := range procs {
		if !sameName(p.Exe, exe) {
			continue
		}
		// A path whose file name differs from the image name is not what the
		// snapshot claimed: leave it alone.
		if p.Path != "" && !sameName(filepath.Base(strings.ReplaceAll(p.Path, `\`, "/")), p.Exe) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// stillRunning returns those of procs that are alive and the same process.
func (m *Manager) stillRunning(exe string, procs []Proc) []Proc {
	now, err := m.matching(exe)
	if err != nil {
		return procs // unknown: assume they are still there
	}
	var out []Proc
	for _, p := range procs {
		for _, q := range now {
			if p.PID == q.PID && p.Created == q.Created {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func (m *Manager) waitGone(exe string, procs []Proc, limit time.Duration) []Proc {
	poll := dur(m.Poll, 200*time.Millisecond)
	deadline := time.Now().Add(limit)
	for {
		procs = m.stillRunning(exe, procs)
		if len(procs) == 0 || !time.Now().Before(deadline) {
			return procs
		}
		time.Sleep(poll)
	}
}

// Close asks a catalog program to end. Without hard it only posts WM_CLOSE to
// the program's windows and waits for it; with hard it ends what is left, but
// only after a polite request for the same program was made. The program is
// named by its image name; the processes come from a fresh snapshot, never
// from the caller.
func (m *Manager) Close(exe string, hard bool) (Result, error) {
	return m.CloseConfirmed(exe, hard, false)
}

// CloseConfirmed is Close with the answer to the second warning: without
// allowUnusual a process whose image lies outside the usual install locations
// is refused with ErrUnusualPath.
func (m *Manager) CloseConfirmed(exe string, hard, allowUnusual bool) (Result, error) {
	ent, ok := Lookup(exe)
	if !ok {
		return Result{}, ErrUnknown
	}
	if !m.closeMu.TryLock() {
		return Result{}, ErrBusy
	}
	defer m.closeMu.Unlock()

	res := Result{Exe: ent.Exe}
	procs, err := m.matching(ent.Exe)
	if err != nil {
		return res, fmt.Errorf("process list: %w", err)
	}
	if len(procs) == 0 {
		res.Status = "not_running"
		m.Scan()
		return res, nil
	}
	var todo []Proc
	for _, p := range procs {
		switch {
		case p.Service || ent.Service:
			res.Status, res.Reason = "needs_admin", "service"
		case p.Restricted:
			if res.Reason == "" {
				res.Status, res.Reason = "needs_admin", "elevated"
			}
		default:
			todo = append(todo, p)
		}
	}
	if len(todo) == 0 {
		m.logf("conflicts: %s cannot be closed from this session (%s)", ent.Exe, res.Reason)
		return res, nil
	}
	if !allowUnusual {
		roots := m.roots()
		for _, p := range todo {
			if !usualPath(p.Path, roots) {
				m.logf("conflicts: %s pid %d runs from an unusual location (%s): not closed without confirmation", p.Exe, p.PID, p.Path)
				return res, ErrUnusualPath
			}
		}
	}

	if hard {
		// Only processes that were asked politely first are ended.
		var asked []Proc
		m.mu.Lock()
		m.pruneSoft()
		for _, p := range todo {
			if at, ok := m.soft[ident{p.PID, p.Created}]; ok && time.Since(at) <= softWindow {
				asked = append(asked, p)
			}
		}
		m.mu.Unlock()
		if len(asked) == 0 {
			return res, ErrNeedsSoft
		}
		todo = asked
		for _, p := range todo {
			if err := m.Sys.Terminate(p); err != nil {
				m.logf("conflicts: ending %s pid %d refused: %v", p.Exe, p.PID, err)
				continue
			}
			m.logf("conflicts: ended %s pid %d at the user's request", p.Exe, p.PID)
		}
		todo = m.waitGone(ent.Exe, todo, dur(m.HardTimeout, 3*time.Second))
	} else {
		for _, p := range todo {
			n, err := m.Sys.PostClose(p)
			if err != nil {
				m.logf("conflicts: close request for %s pid %d refused: %v", p.Exe, p.PID, err)
				continue
			}
			m.logf("conflicts: asked %s pid %d to close (%d window(s)) at the user's request", p.Exe, p.PID, n)
			m.mu.Lock()
			m.pruneSoft()
			if m.soft == nil {
				m.soft = map[ident]time.Time{}
			}
			m.soft[ident{p.PID, p.Created}] = time.Now()
			m.mu.Unlock()
		}
		todo = m.waitGone(ent.Exe, todo, dur(m.GraceTimeout, 5*time.Second))
	}
	m.Scan()
	switch {
	case len(todo) > 0:
		res.Status, res.Reason = "still_running", ""
	case res.Status == "needs_admin":
		// what could be closed is gone, the rest needs admin rights
	default:
		res.Status = "closed"
		m.logf("conflicts: %s is closed", ent.Exe)
		if m.OnClosed != nil {
			go m.OnClosed(ent.Exe)
		}
	}
	return res, nil
}
