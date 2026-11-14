//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// These tests start real processes that compete for a mutex with their own
// name; the CaseHUD mutex is never touched.

const lockWaitEnv = "CASEHUD_TEST_LOCKWAIT"

// helperLock waits for the start file in dir, then takes the lock like a CaseHUD
// start does. It writes its result to res.<id>; a winner stays until
// release.<id> exists.
func helperLock() int {
	dir, id, name := os.Getenv(dirEnv), os.Getenv("CASEHUD_TEST_ID"), os.Getenv(nsEnv)
	ms, _ := strconv.Atoi(os.Getenv(lockWaitEnv))
	waitFile(filepath.Join(dir, "go"), 30*time.Second)
	res := acquireInstance(func() bool { return tryHoldNamed(name) }, func() bool { return false },
		time.Duration(ms)*time.Millisecond, 10*time.Millisecond)
	out := map[lockResult]string{lockHeld: "held", lockHandedOver: "handed", lockTimeout: "timeout"}[res]
	_ = os.WriteFile(filepath.Join(dir, "res."+id), []byte(out), 0o600)
	if res == lockHeld {
		waitFile(filepath.Join(dir, "release."+id), 30*time.Second)
	}
	return 0
}

func waitFile(p string, d time.Duration) bool {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if _, err := os.Stat(p); err == nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func startLocker(t *testing.T, dir, name, id string, waitMS int) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), roleEnv+"=lock", dirEnv+"="+dir, nsEnv+"="+name,
		"CASEHUD_TEST_ID="+id, lockWaitEnv+"="+strconv.Itoa(waitMS))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release."+id), nil, 0o600)
		if cmd.Process != nil {
			cmd.Process.Kill() // only this test's own helper
		}
		cmd.Wait()
	})
	return cmd
}

func result(dir, id string, d time.Duration) string {
	p := filepath.Join(dir, "res."+id)
	if !waitFile(p, d) {
		return ""
	}
	b, _ := os.ReadFile(p)
	return strings.TrimSpace(string(b))
}

func mutexName(t *testing.T) string {
	return fmt.Sprintf("CaseHUD.Test.%d.%d", os.Getpid(), time.Now().UnixNano())
}

func TestSingleInstanceLockExactlyOneWins(t *testing.T) {
	dir, name := t.TempDir(), mutexName(t)
	const n = 8
	for i := 0; i < n; i++ {
		startLocker(t, dir, name, strconv.Itoa(i), 100)
	}
	time.Sleep(500 * time.Millisecond) // all helpers are up and waiting for the start file
	_ = os.WriteFile(filepath.Join(dir, "go"), nil, 0o600)
	held, timeouts := 0, 0
	for i := 0; i < n; i++ {
		switch r := result(dir, strconv.Itoa(i), 20*time.Second); r {
		case "held":
			held++
		case "timeout":
			timeouts++
		default:
			t.Fatalf("helper %d: result %q", i, r)
		}
	}
	if held != 1 || timeouts != n-1 {
		t.Fatalf("held %d, timeout %d of %d", held, timeouts, n)
	}
}

func TestSingleInstanceLockWaiterTakesOver(t *testing.T) {
	dir, name := t.TempDir(), mutexName(t)
	startLocker(t, dir, name, "a", 100)
	_ = os.WriteFile(filepath.Join(dir, "go"), nil, 0o600)
	if r := result(dir, "a", 20*time.Second); r != "held" {
		t.Fatalf("first start: %q", r)
	}
	b := startLocker(t, dir, name, "b", 20000)
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "res.b")); err == nil {
		t.Fatal("second start decided while the first still held the lock")
	}
	_ = b
	// The first instance ends (its process exit releases the mutex).
	_ = os.WriteFile(filepath.Join(dir, "release.a"), nil, 0o600)
	if r := result(dir, "b", 20*time.Second); r != "held" {
		t.Fatalf("waiting start after the first ended: %q", r)
	}
}

// A name taken by an object of another type (here an event) is neither "free" nor
// "held by another start": the lock is not won, and the reason is told.
func TestLockNotWonWhenTheNameIsTakenByAnotherKindOfObject(t *testing.T) {
	name := mutexName(t)
	p, _ := windows.UTF16PtrFromString(name)
	ev, err := windows.CreateEvent(nil, 0, 0, p)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(ev)
	held, err := holdNamed(name)
	if held {
		t.Fatal("the lock was won over a foreign object")
	}
	if err == nil {
		t.Fatal("no reason given: it would look like a plain busy lock")
	}
	if tryHoldNamed(name) {
		t.Fatal("tryHoldNamed won")
	}
	// A real mutex of that name held elsewhere in the process is just "held".
	m := name + ".mutex"
	if held, err := holdNamed(m); !held || err != nil {
		t.Fatalf("first: %v %v", held, err)
	}
	if held, err := holdNamed(m); held || err != nil {
		t.Fatalf("second: %v %v", held, err)
	}
}
