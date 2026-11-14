//go:build windows

package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestSessionEndCallsOnlyForARealEnd(t *testing.T) {
	// A name of its own: the fixed one would also find the window of a running
	// CaseHUD and end its session.
	sessionClass = fmt.Sprintf("CaseHUDSessionEndTest.%d.%d", os.Getpid(), rand.Int63())
	var system atomic.Bool // what the system says about the shutdown
	sessionShuttingDown = system.Load
	t.Cleanup(func() { sessionShuttingDown = realShuttingDown })
	var ended atomic.Int32
	watchSessionEnd(func() { ended.Add(1) })
	name, _ := windows.UTF16PtrFromString(sessionClass)
	var hwnd uintptr
	for i := 0; i < 100 && hwnd == 0; i++ {
		hwnd, _, _ = procFindWindowW.Call(uintptr(unsafe.Pointer(name)), 0)
		time.Sleep(20 * time.Millisecond)
	}
	if hwnd == 0 {
		t.Fatal("the hidden window did not come up")
	}
	if r, _, _ := procSendMessageW.Call(hwnd, wmQueryEndSession, 0, 0); r != 1 {
		t.Fatalf("query answered %d, want 1 (no veto)", r)
	}
	procSendMessageW.Call(hwnd, wmEndSession, 0, 0) // another program vetoed
	if ended.Load() != 0 {
		t.Fatal("acted on a session end that did not happen")
	}
	procSendMessageW.Call(hwnd, wmEndSession, 1, 0) // a program of the session made it up
	if ended.Load() != 0 {
		t.Fatal("acted on a forged message")
	}
	system.Store(true)
	procSendMessageW.Call(hwnd, wmEndSession, 0, 0) // vetoed while the system says it shuts down
	if ended.Load() != 0 {
		t.Fatal("acted on a vetoed end")
	}
	procSendMessageW.Call(hwnd, wmEndSession, 1, 0)
	if ended.Load() != 1 {
		t.Fatal("did not act on the end of the session")
	}
}

var realShuttingDown = sessionShuttingDown

func TestForgedEndMessagesAreLoggedOncePerMinute(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	sessionNow = func() time.Time { return now }
	sessionShuttingDown = func() bool { return false }
	ignoredLogAt.Store(0)
	ignoredSinceLast.Store(0)
	t.Cleanup(func() { sessionNow, sessionShuttingDown = time.Now, realShuttingDown })
	var ended atomic.Int32
	onEnd.Store(&[]func(){func() { ended.Add(1) }}[0])
	lines := func() int { return strings.Count(buf.String(), "ignored") }
	for range 50 {
		if endSessionMessage(1) {
			t.Fatal("acted on a forged message")
		}
	}
	if lines() != 1 {
		t.Fatalf("%d log lines for the first burst", lines())
	}
	now = now.Add(59 * time.Second)
	endSessionMessage(1)
	if lines() != 1 {
		t.Fatal("logged again within the minute")
	}
	now = now.Add(2 * time.Second)
	endSessionMessage(1)
	if lines() != 2 || !strings.Contains(buf.String(), "(51 since the last note)") {
		t.Fatalf("%d lines: %s", lines(), buf.String())
	}
	if ended.Load() != 0 {
		t.Fatal("onEnd ran")
	}
	sessionShuttingDown = func() bool { return true }
	if !endSessionMessage(1) || ended.Load() != 1 {
		t.Fatal("a real end did not run onEnd")
	}
}
