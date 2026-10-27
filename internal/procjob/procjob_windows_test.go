//go:build windows

package procjob

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestKillOnJobClose(t *testing.T) {
	j, err := newJob()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Getenv("ComSpec"), "/c", "ping -n 30 127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if err := assignTo(j, cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
		t.Fatal("child exited before job close")
	case <-time.After(300 * time.Millisecond):
	}
	windows.CloseHandle(j)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("child still running after job handle closed")
	}
}

func TestAssignNilAndReal(t *testing.T) {
	Assign(nil)
	cmd := exec.Command(os.Getenv("ComSpec"), "/c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	Assign(cmd.Process)
	cmd.Wait()
}
