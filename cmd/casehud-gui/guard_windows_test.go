//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/coolguard"
)

// These tests run the real processes: a stand-in for CaseHUD that is killed, and
// the guard it started. The guard writes to a file and not to a fan controller.

const (
	roleEnv   = "CASEHUD_TEST_ROLE"
	dirEnv    = "CASEHUD_TEST_DIR"
	parentEnv = "CASEHUD_TEST_PARENT"
	modeEnv   = "CASEHUD_TEST_MODE"
	nsEnv     = "CASEHUD_TEST_NS"
)

func TestMain(m *testing.M) {
	switch os.Getenv(roleEnv) {
	case "parent":
		os.Exit(helperParent())
	case "guard":
		os.Exit(helperGuard())
	case "lock":
		os.Exit(helperLock())
	}
	os.Exit(m.Run())
}

// fileFans writes what the guard would send to the controller into a file.
type fileFans struct{ path string }

func (f fileFans) put(line string) error {
	h, err := os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer h.Close()
	_, err = fmt.Fprintln(h, line)
	return err
}
func (f fileFans) TakeControl(id string) error    { return f.put("take " + id) }
func (f fileFans) SetDuty(id string, p int) error { return f.put(fmt.Sprintf("duty %s %d", id, p)) }
func (f fileFans) Close() error                   { return nil }
func openFileFans(dir string) func(context.Context) (coolguard.Fans, error) {
	return func(context.Context) (coolguard.Fans, error) { return fileFans{filepath.Join(dir, "writes.txt")}, nil }
}

// helperGuard is the guard process of the test.
func helperGuard() int {
	coolguard.UseNamespace(os.Getenv(nsEnv))
	pid, _ := strconv.Atoi(os.Getenv(parentEnv))
	return runGuard(pid, os.Getenv(dirEnv), openFileFans(os.Getenv(dirEnv)), func() (bool, error) { return false, nil })
}

// helperParent stands in for CaseHUD: it starts the guard, keeps it informed, and
// either waits to be killed or, on request, ends cleanly.
func helperParent() int {
	dir := os.Getenv(dirEnv)
	exe, _ := os.Executable()
	os.Setenv(roleEnv, "guard") // for the child
	os.Setenv(parentEnv, strconv.Itoa(os.Getpid()))
	link := &coolguard.Link{
		Spawn: func() (coolguard.Child, error) {
			// As Spawn does it: from a copy under another name.
			own, err := coolguard.GuardExe(dir, exe)
			if err != nil {
				return nil, err
			}
			return coolguard.SpawnProcess(own)
		},
		Source: func() (coolguard.State, bool) {
			return coolguard.State{Active: true,
				Safe: map[string]int{"port1-fan1": 70, "port1-fan2": 70},
				Last: map[string]int{"port1-fan1": 20, "port1-fan2": 95}}, true
		},
		Poll: 50 * time.Millisecond,
	}
	link.Arm()
	os.WriteFile(filepath.Join(dir, "ready"), nil, 0o600)
	if os.Getenv(modeEnv) == "clean" {
		for {
			if _, err := os.Stat(filepath.Join(dir, "quit")); err == nil {
				link.Disarm()
				return 0
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	select {} // waits to be killed
}

func startParent(t *testing.T, mode string) (cmd *exec.Cmd, dir string) {
	t.Helper()
	dir = t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(exe)
	cmd.Env = append(os.Environ(), roleEnv+"=parent", dirEnv+"="+dir, modeEnv+"="+mode,
		nsEnv+"=.test"+strconv.Itoa(os.Getpid())+mode)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	waitFor(t, "the stand-in for CaseHUD", func() bool { _, err := os.Stat(filepath.Join(dir, "ready")); return err == nil })
	waitFor(t, "the guard to be armed", func() bool { return strings.Contains(guardLogOf(dir), "armed: 2 fans") })
	return cmd, dir
}

func guardLogOf(dir string) string {
	b, _ := os.ReadFile(filepath.Join(dir, guardLogFile))
	return string(b)
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestKilledCaseHUDLeavesTheFansAtTheirSafeDuty(t *testing.T) {
	cmd, dir := startParent(t, "hard")
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	writes := filepath.Join(dir, "writes.txt")
	waitFor(t, "the guard to set the fans", func() bool { return strings.Contains(guardLogOf(dir), "guard ends") })
	b, err := os.ReadFile(writes)
	if err != nil {
		t.Fatalf("nothing was written: %v\nlog:\n%s", err, guardLogOf(dir))
	}
	// 70 % safe duty, but the second fan ran at 95 % and is not lowered.
	for _, want := range []string{"duty port1-fan1 70", "duty port1-fan2 95"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %q in\n%s", want, b)
		}
	}
	if !strings.Contains(guardLogOf(dir), "guard ends: acted") {
		t.Errorf("log:\n%s", guardLogOf(dir))
	}
}

func TestCleanEndOfCaseHUDLeavesTheGuardWithNothingToDo(t *testing.T) {
	cmd, dir := startParent(t, "clean")
	if err := os.WriteFile(filepath.Join(dir, "quit"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the stand-in ended with %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the stand-in did not end")
	}
	waitFor(t, "the guard to end", func() bool { return strings.Contains(guardLogOf(dir), "guard ends") })
	if _, err := os.Stat(filepath.Join(dir, "writes.txt")); err == nil {
		t.Fatalf("the guard touched the fans after a clean end:\n%s", guardLogOf(dir))
	}
	if !strings.Contains(guardLogOf(dir), "guard ends: clean") {
		t.Errorf("log:\n%s", guardLogOf(dir))
	}
}

// Killing only the guard must not disturb the stand-in: it starts a new one, which
// is then armed again.
func TestKilledGuardIsReplaced(t *testing.T) {
	cmd, dir := startParent(t, "hard")
	pid := func() string {
		for _, l := range strings.Split(guardLogOf(dir), "\n") {
			if strings.Contains(l, "guard of process") {
				return l
			}
		}
		return ""
	}
	if pid() == "" {
		t.Fatal("no guard log")
	}
	// Find the guard: the child of the stand-in.
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf("(Get-CimInstance Win32_Process -Filter 'ParentProcessId=%d').ProcessId", cmd.Process.Pid)).Output()
	if err != nil {
		t.Skipf("cannot find the guard: %v", err)
	}
	gp, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if gp == 0 {
		t.Skip("no child found")
	}
	proc, _ := os.FindProcess(gp)
	proc.Kill()
	waitFor(t, "a second guard", func() bool { return strings.Count(guardLogOf(dir), "armed: 2 fans") >= 2 })
	if _, err := os.Stat(filepath.Join(dir, "writes.txt")); err == nil {
		t.Fatal("something was written while CaseHUD lived")
	}
	// Then the stand-in is killed: the new guard acts.
	cmd.Process.Kill()
	waitFor(t, "the new guard to act", func() bool { return strings.Contains(guardLogOf(dir), "guard ends: acted") })
}
