//go:build windows

package coolguard

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

var (
	singleName = `Local\CaseHUD.CoolingGuard`
	actingName = `Local\CaseHUD.CoolingGuard.Acting`
)

// UseNamespace gives the guard its own mutex names, so that a test never waits for
// the guard of a CaseHUD that really runs on the same machine.
func UseNamespace(suffix string) {
	singleName += suffix
	actingName += suffix
}

func spawnAttr(breakaway bool) *syscall.SysProcAttr {
	flags := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	if breakaway {
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: flags}
}

// WatchParent returns a channel that is closed when the process pid has ended. A
// process that is not there, or that started after this one (its id was reused),
// counts as ended.
func WatchParent(pid int) (<-chan struct{}, error) {
	gone := make(chan struct{})
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			close(gone)
			return gone, nil
		}
		return nil, err
	}
	var c, e, k, u, oc windows.Filetime
	if windows.GetProcessTimes(h, &c, &e, &k, &u) == nil &&
		windows.GetProcessTimes(windows.CurrentProcess(), &oc, &e, &k, &u) == nil &&
		c.Nanoseconds() > oc.Nanoseconds() {
		windows.CloseHandle(h)
		close(gone)
		return gone, nil
	}
	go func() {
		_, _ = windows.WaitForSingleObject(h, windows.INFINITE)
		windows.CloseHandle(h)
		close(gone)
	}()
	return gone, nil
}

// AcquireSingle makes this process the only guard of the session. An older guard
// that still runs, such as one that is just setting the fans, gets timeout to end.
// It must be called on the thread that stays locked for the guard's lifetime.
func AcquireSingle(timeout time.Duration) bool {
	name, _ := windows.UTF16PtrFromString(singleName)
	h, err := windows.CreateMutex(nil, false, name)
	if h == 0 {
		return err == nil
	}
	ev, _ := windows.WaitForSingleObject(h, uint32(timeout.Milliseconds()))
	if ev != windows.WAIT_OBJECT_0 && ev != windows.WAIT_ABANDONED {
		windows.CloseHandle(h)
		return false
	}
	// h is never closed: the process holds the mutex for as long as it lives.
	return true
}

// BeginAct marks that this guard works on the controller. It returns the release.
func BeginAct() (release func()) {
	name, _ := windows.UTF16PtrFromString(actingName)
	h, _ := windows.CreateMutex(nil, false, name)
	if h == 0 {
		return func() {}
	}
	runtime.LockOSThread()
	ev, _ := windows.WaitForSingleObject(h, 5000)
	owned := ev == windows.WAIT_OBJECT_0 || ev == windows.WAIT_ABANDONED
	return func() {
		if owned {
			_ = windows.ReleaseMutex(h)
		}
		windows.CloseHandle(h)
		runtime.UnlockOSThread()
	}
}

// WaitIdle returns once no guard is working on the controller, at the latest
// after timeout. Opening the controller waits for it, so that CaseHUD starting
// again does not write at the same time as a guard.
func WaitIdle(ctx context.Context, timeout time.Duration) {
	name, _ := windows.UTF16PtrFromString(actingName)
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return // no guard is there
	}
	defer windows.CloseHandle(h)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ev, _ := windows.WaitForSingleObject(h, uint32(timeout.Milliseconds()))
	if ev == windows.WAIT_OBJECT_0 || ev == windows.WAIT_ABANDONED {
		_ = windows.ReleaseMutex(h)
	}
}
