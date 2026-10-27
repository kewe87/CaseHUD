package conflicts

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSys is a process list with a fake window poster.
type fakeSys struct {
	mu        sync.Mutex
	procs     []Proc
	closeEnds bool // PostClose makes the process go away
	termEnds  bool
	posted    []uint32
	ended     []uint32
}

func (f *fakeSys) Processes(want func(string) bool) ([]Proc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Proc
	for _, p := range f.procs {
		if want(p.Exe) {
			if p.Path == "" {
				p.Path = `C:\Apps\` + p.Exe // the tests' default: a usual place under Roots
			}
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeSys) drop(pid uint32) {
	for i, p := range f.procs {
		if p.PID == pid {
			f.procs = append(f.procs[:i], f.procs[i+1:]...)
			return
		}
	}
}

func (f *fakeSys) same(p Proc) bool {
	for _, q := range f.procs {
		if q.PID == p.PID && q.Created == p.Created && q.Exe == p.Exe {
			return true
		}
	}
	return false
}

func (f *fakeSys) PostClose(p Proc) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.same(p) {
		return 0, errors.New("the process changed")
	}
	f.posted = append(f.posted, p.PID)
	if f.closeEnds {
		f.drop(p.PID)
	}
	return 2, nil
}

func (f *fakeSys) Terminate(p Proc) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.same(p) {
		return errors.New("the process changed")
	}
	f.ended = append(f.ended, p.PID)
	if f.termEnds {
		f.drop(p.PID)
	}
	return nil
}

type logs struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logs) f(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.WriteString(format + "\n")
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func newMgr(f *fakeSys, set Settings) (*Manager, *logs) {
	l := &logs{}
	return &Manager{Sys: f, Settings: func() Settings { return set }, Logf: l.f, Roots: []string{`C:\`},
		GraceTimeout: 60 * time.Millisecond, HardTimeout: 60 * time.Millisecond, Poll: 5 * time.Millisecond}, l
}

// allSys returns every process whatever want says.
type allSys struct{ *fakeSys }

func (a allSys) Processes(func(string) bool) ([]Proc, error) {
	return a.fakeSys.Processes(func(string) bool { return true })
}

func TestScanOnlyCatalog(t *testing.T) {
	f := &fakeSys{procs: []Proc{
		{PID: 1, Exe: "notepad.exe", Created: 1},
		{PID: 2, Exe: "signalrgb.EXE", Created: 2},
	}}
	m, _ := newMgr(f, Settings{Ask: true})
	m.Sys = allSys{f} // a system that ignores want must not leak unknown programs
	m.Scan()
	st := m.Status()
	if st.Pending != 1 {
		t.Fatalf("pending = %d", st.Pending)
	}
	for _, it := range st.Items {
		if it.Running && it.Exe != "SignalRgb.exe" {
			t.Errorf("unexpected running %s", it.Exe)
		}
	}
	if r := m.Running(); len(r) != 1 || r[0] != "SignalRgb.exe" {
		t.Errorf("running = %v", r)
	}
}

func TestStatusIgnoredAndActive(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 2, Exe: "OpenRGB.exe", Created: 2}}}
	set := Settings{Ask: true, Ignored: map[string]bool{"openrgb.exe": true}, Active: map[string]bool{DevFanLED: true}}
	m, _ := newMgr(f, set)
	m.Scan()
	st := m.Status()
	if st.Pending != 0 {
		t.Errorf("an ignored program counted: %d", st.Pending)
	}
	for _, it := range st.Items {
		if it.Exe == "OpenRGB.exe" && (!it.Running || !it.Ignored || len(it.Affects) != 1 || it.Affects[0] != DevFanLED) {
			t.Errorf("item = %+v", it)
		}
	}
	// Lighting off: lighting-only programs do not matter.
	set.Ignored, set.Active = nil, map[string]bool{DevTLLCD: true}
	m, _ = newMgr(f, set)
	m.Scan()
	if m.Status().Pending != 0 {
		t.Error("a program without active devices counted")
	}
}

func TestCloseGraceful(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "SignalRgb.exe", Path: `C:\x\SignalRgb.exe`, Created: 5}}, closeEnds: true}
	m, l := newMgr(f, Settings{Ask: true})
	closed := make(chan string, 1)
	m.OnClosed = func(e string) { closed <- e }
	res, err := m.Close("signalrgb.exe", false)
	if err != nil || res.Status != "closed" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if len(f.posted) != 1 || len(f.ended) != 0 {
		t.Errorf("posted %v ended %v", f.posted, f.ended)
	}
	select {
	case e := <-closed:
		if e != "SignalRgb.exe" {
			t.Errorf("OnClosed(%q)", e)
		}
	case <-time.After(time.Second):
		t.Error("OnClosed not called")
	}
	if !strings.Contains(l.String(), "asked") || !strings.Contains(l.String(), "is closed") {
		t.Errorf("log: %s", l.String())
	}
}

func TestCloseRefusesUnknown(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 1, Exe: "notepad.exe", Created: 1}}}
	m, _ := newMgr(f, Settings{Ask: true})
	for _, n := range []string{"notepad.exe", "", `..\SignalRgb.exe`, "SignalRgb.exe "} {
		if _, err := m.Close(n, false); !errors.Is(err, ErrUnknown) {
			t.Errorf("%q: err = %v", n, err)
		}
		if _, err := m.Close(n, true); !errors.Is(err, ErrUnknown) {
			t.Errorf("%q hard: err = %v", n, err)
		}
	}
	if len(f.posted)+len(f.ended) != 0 {
		t.Error("an unlisted process was touched")
	}
}

func TestHardNeedsPoliteFirst(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "OpenRGB.exe", Created: 5}}, termEnds: true}
	m, _ := newMgr(f, Settings{Ask: true})
	if _, err := m.Close("OpenRGB.exe", true); !errors.Is(err, ErrNeedsSoft) {
		t.Fatalf("err = %v", err)
	}
	if len(f.ended) != 0 {
		t.Fatal("ended without a polite request")
	}
	res, err := m.Close("OpenRGB.exe", false) // stays: its window ignores the request
	if err != nil || res.Status != "still_running" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	res, err = m.Close("OpenRGB.exe", true)
	if err != nil || res.Status != "closed" || len(f.ended) != 1 {
		t.Fatalf("res = %+v err = %v ended %v", res, err, f.ended)
	}
}

func TestHardWindowExpires(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "OpenRGB.exe", Created: 5}}}
	m, _ := newMgr(f, Settings{Ask: true})
	m.Close("OpenRGB.exe", false)
	m.mu.Lock()
	for k := range m.soft {
		m.soft[k] = time.Now().Add(-softWindow - time.Second)
	}
	m.mu.Unlock()
	if _, err := m.Close("OpenRGB.exe", true); !errors.Is(err, ErrNeedsSoft) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceAndElevatedNotTouched(t *testing.T) {
	f := &fakeSys{procs: []Proc{
		{PID: 3, Exe: "L-Connect-Service.exe", Created: 1, Service: true, Restricted: true},
		{PID: 4, Exe: "ArmouryCrate.exe", Created: 1, Restricted: true},
	}, closeEnds: true, termEnds: true}
	m, _ := newMgr(f, Settings{Ask: true})
	m.Scan()
	for _, it := range m.Status().Items {
		switch it.Exe {
		case "L-Connect-Service.exe":
			if it.Closable || it.Reason != "service" {
				t.Errorf("service item = %+v", it)
			}
		case "ArmouryCrate.exe":
			if it.Closable || it.Reason != "elevated" {
				t.Errorf("elevated item = %+v", it)
			}
		}
	}
	for _, hard := range []bool{false, true} {
		for _, n := range []string{"L-Connect-Service.exe", "ArmouryCrate.exe"} {
			res, err := m.Close(n, hard)
			if err != nil || res.Status != "needs_admin" {
				t.Errorf("%s hard=%v: %+v %v", n, hard, res, err)
			}
		}
	}
	if len(f.posted)+len(f.ended) != 0 {
		t.Error("a protected process was touched")
	}
}

func TestPIDReuse(t *testing.T) {
	// The process we asked is replaced by an unrelated one with the same PID.
	f := &fakeSys{procs: []Proc{{PID: 9, Exe: "SignalRgb.exe", Created: 100}}}
	m, _ := newMgr(f, Settings{Ask: true})
	stale := f.procs[0]
	f.procs = []Proc{{PID: 9, Exe: "SignalRgb.exe", Created: 200}}
	if _, err := f.PostClose(stale); err == nil {
		t.Error("the fake must refuse a stale identity")
	}
	if err := f.Terminate(stale); err == nil {
		t.Error("the fake must refuse a stale identity")
	}
	if left := m.stillRunning("SignalRgb.exe", []Proc{stale}); len(left) != 0 {
		t.Errorf("reused PID counted as the old process: %v", left)
	}
}

func TestPathMustMatchName(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 9, Exe: "SignalRgb.exe", Path: `C:\tmp\other.exe`, Created: 1}}, closeEnds: true}
	m, _ := newMgr(f, Settings{Ask: true})
	res, _ := m.Close("SignalRgb.exe", false)
	if res.Status != "not_running" || len(f.posted) != 0 {
		t.Errorf("res = %+v posted %v", res, f.posted)
	}
}

func TestDemoSystem(t *testing.T) {
	m := &Manager{Sys: NewDemoSystem(), Roots: []string{`C:\Demo`}, Logf: func(string, ...any) {}, Poll: time.Millisecond, GraceTimeout: 20 * time.Millisecond}
	m.Scan()
	if m.Status().Pending != 2 {
		t.Fatalf("pending = %d", m.Status().Pending)
	}
	if res, _ := m.Close("L-Connect 3.exe", false); res.Status != "closed" {
		t.Errorf("res = %+v", res)
	}
	if res, _ := m.Close("L-Connect-Service.exe", false); res.Status != "needs_admin" {
		t.Errorf("res = %+v", res)
	}
}

func TestCatalogLookup(t *testing.T) {
	if _, ok := Lookup("l-connect 3.EXE"); !ok {
		t.Error("case-insensitive lookup failed")
	}
	if _, ok := Lookup("explorer.exe"); ok {
		t.Error("explorer must not be listed")
	}
	for _, e := range Catalog() {
		if !strings.HasSuffix(strings.ToLower(e.Exe), ".exe") || len(e.Devices) == 0 || e.Name == "" {
			t.Errorf("bad entry %+v", e)
		}
	}
}

func TestHardOnlyForAskedInstance(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "OpenRGB.exe", Created: 5}}, termEnds: true}
	m, _ := newMgr(f, Settings{Ask: true})
	m.Close("OpenRGB.exe", false)
	// a new instance appears after the polite request
	f.procs = []Proc{{PID: 8, Exe: "OpenRGB.exe", Created: 6}}
	if _, err := m.Close("OpenRGB.exe", true); !errors.Is(err, ErrNeedsSoft) {
		t.Fatalf("err = %v", err)
	}
	if len(f.ended) != 0 {
		t.Fatal("an instance that was never asked was ended")
	}
}

func TestLookAlikeNames(t *testing.T) {
	if _, ok := Lookup("ſignalRgb.exe"); ok {
		t.Error("look-alike name matched")
	}
	if !sameName("SIGNALRGB.EXE", "SignalRgb.exe") {
		t.Error("ASCII case folding failed")
	}
}

func TestCloseBusy(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "OpenRGB.exe", Created: 5}}}
	m, _ := newMgr(f, Settings{Ask: true})
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if _, err := m.Close("OpenRGB.exe", false); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnusualLocationNeedsSecondConfirmation(t *testing.T) {
	f := &fakeSys{procs: []Proc{{PID: 7, Exe: "SignalRgb.exe", Path: `C:\Users\x\Downloads\SignalRgb.exe`, Created: 5}}, closeEnds: true}
	m, _ := newMgr(f, Settings{Ask: true})
	m.Roots = []string{`C:\Program Files`, `C:\Users\x\AppData\Local\Programs`}
	m.Scan()
	var it Item
	for _, i := range m.Status().Items {
		if i.Exe == "SignalRgb.exe" {
			it = i
		}
	}
	if !it.Unusual || len(it.Paths) != 1 || it.Paths[0] != `C:\Users\x\Downloads\SignalRgb.exe` {
		t.Fatalf("item %+v", it)
	}
	if _, err := m.Close("SignalRgb.exe", false); !errors.Is(err, ErrUnusualPath) {
		t.Fatalf("unusual location closed without confirmation: %v", err)
	}
	if _, err := m.Close("SignalRgb.exe", true); !errors.Is(err, ErrUnusualPath) {
		t.Fatalf("hard close: %v", err)
	}
	if len(f.posted) != 0 || len(f.ended) != 0 {
		t.Fatal("process touched")
	}
	res, err := m.CloseConfirmed("SignalRgb.exe", false, true)
	if err != nil || res.Status != "closed" {
		t.Fatalf("confirmed close: %+v %v", res, err)
	}
}

func TestUsualPathRoots(t *testing.T) {
	roots := []string{`C:\Program Files`, `c:\users\x\appdata\local\programs`}
	for p, want := range map[string]bool{
		`C:\Program Files\A\a.exe`:                true,
		`c:\program files\a.exe`:                  true,
		`C:\Program Files (x86)\a.exe`:            false,
		`C:\Program Files2\a.exe`:                 false,
		`C:\Users\X\AppData\Local\Programs\a.exe`: true,
		`C:\Users\X\AppData\Local\Temp\a.exe`:     false,
		``:                                        false,
	} {
		if usualPath(p, roots) != want {
			t.Errorf("usualPath(%q) = %v", p, !want)
		}
	}
}

func TestSoftEntriesArePruned(t *testing.T) {
	m := &Manager{Sys: &fakeSys{}, soft: map[ident]time.Time{
		{1, 1}: time.Now().Add(-2 * softWindow),
		{2, 2}: time.Now(),
	}}
	m.Scan()
	if len(m.soft) != 1 {
		t.Fatalf("soft = %v", m.soft)
	}
	if _, ok := m.soft[ident{2, 2}]; !ok {
		t.Fatal("fresh entry dropped")
	}
}
