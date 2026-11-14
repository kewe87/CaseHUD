//go:build windows

package coolguard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestWaitIdleWaitsForAGuardThatIsActing(t *testing.T) {
	UseNamespace(fmt.Sprintf(".test%d", os.Getpid()))

	// Nobody acts: it returns at once.
	t0 := time.Now()
	WaitIdle(context.Background(), 5*time.Second)
	if d := time.Since(t0); d > time.Second {
		t.Fatalf("waited %v with nobody acting", d)
	}

	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		end := BeginAct()
		close(started)
		<-release
		end()
		close(done)
	}()
	<-started
	waited := make(chan time.Duration, 1)
	go func() {
		t := time.Now()
		WaitIdle(context.Background(), 10*time.Second)
		waited <- time.Since(t)
	}()
	select {
	case d := <-waited:
		t.Fatalf("did not wait for the guard that acts (%v)", d)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	select {
	case d := <-waited:
		if d < 250*time.Millisecond {
			t.Fatalf("waited only %v", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting after the guard was done")
	}
	<-done
}

func TestWaitIdleGivesUpOnAGuardThatNeverEnds(t *testing.T) {
	UseNamespace(fmt.Sprintf(".testlong%d", os.Getpid()))
	started, release := make(chan struct{}), make(chan struct{})
	go func() {
		end := BeginAct()
		close(started)
		<-release
		end()
	}()
	<-started
	defer close(release)
	t0 := time.Now()
	WaitIdle(context.Background(), 400*time.Millisecond)
	if d := time.Since(t0); d < 350*time.Millisecond || d > 3*time.Second {
		t.Fatalf("waited %v", d)
	}
}

const holderEnv = "COOLGUARD_TEST_HOLDER"

// TestHolderProcess is not a test of its own: TestOnlyOneGuardAtATime starts it as
// the process of an older guard. It holds the lock until it is killed.
func TestHolderProcess(t *testing.T) {
	ns := os.Getenv(holderEnv)
	if ns == "" {
		t.Skip("helper of TestOnlyOneGuardAtATime")
	}
	UseNamespace(ns)
	runtime.LockOSThread()
	if !AcquireSingle(time.Second) {
		fmt.Println("busy")
		os.Exit(1)
	}
	fmt.Println("held")
	io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func TestOnlyOneGuardAtATime(t *testing.T) {
	// A name of its own per run, as the lock of an earlier run may still be held.
	ns := fmt.Sprintf(".testone%d.%d", os.Getpid(), time.Now().UnixNano())
	single, acting := singleName, actingName
	defer func() { singleName, actingName = single, acting }()
	singleName, actingName = `Local\CaseHUD.CoolingGuard`, `Local\CaseHUD.CoolingGuard.Acting` // the helper process starts from these
	UseNamespace(ns)

	// The older guard is a process of its own, as in real life: the lock is
	// released by the end of that process, not by a goroutine.
	cmd := exec.Command(os.Args[0], "-test.run=^TestHolderProcess$")
	cmd.Env = append(os.Environ(), holderEnv+"="+ns)
	in, err := cmd.StdinPipe() // stays open: the helper lives until it is killed
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if sc.Text() == "held" || sc.Text() == "busy" {
			break
		}
	}
	if sc.Text() != "held" {
		t.Fatalf("the first guard did not get it (%q)", sc.Text())
	}

	second := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		second <- AcquireSingle(150 * time.Millisecond)
	}()
	if <-second {
		t.Fatal("a second guard at the same time")
	}

	// The first one ends: the next takes over at once.
	cmd.Process.Kill()
	cmd.Wait()
	third := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		third <- AcquireSingle(10 * time.Second)
	}()
	select {
	case ok := <-third:
		if !ok {
			t.Fatal("not taken over after the first ended")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("blocked")
	}
}
