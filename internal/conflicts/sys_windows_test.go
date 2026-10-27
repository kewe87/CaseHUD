//go:build windows

package conflicts

import (
	"os"
	"path/filepath"
	"testing"
)

// The real system is only read here; the test process itself is the subject
// and is never asked to close or end.
func TestWinSysIdentity(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(exe)
	procs, err := NewSystem().Processes(func(n string) bool { return n == name })
	if err != nil {
		t.Fatal(err)
	}
	var self Proc
	for _, p := range procs {
		if p.PID == uint32(os.Getpid()) {
			self = p
		}
	}
	if self.PID == 0 || self.Path == "" || self.Created == 0 {
		t.Fatalf("own process not described: %+v", self)
	}
	if self.Service {
		t.Error("a test run is no service")
	}
	// A stale identity is refused before anything happens to the process.
	stale := self
	stale.Created++
	if err := NewSystem().Terminate(stale); err == nil {
		t.Fatal("Terminate accepted a wrong start time")
	}
	if _, err := NewSystem().PostClose(stale); err == nil {
		t.Fatal("PostClose accepted a wrong start time")
	}
	wrong := self
	wrong.Exe = "other.exe"
	if err := NewSystem().Terminate(wrong); err == nil {
		t.Fatal("Terminate accepted a wrong image name")
	}
	if _, err := open(Proc{PID: 0, Exe: name}, 0); err == nil {
		t.Fatal("open accepted an unidentified process")
	}
}
