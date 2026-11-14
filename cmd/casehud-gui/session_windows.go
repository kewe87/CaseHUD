//go:build windows

package main

import (
	"log"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Logging off or shutting down ends the process without a word unless it has a
// top-level window that answers WM_ENDSESSION. watchSessionEnd gives it a hidden
// one: when the session really ends, onEnd runs and may take until the system
// has had enough, which lets the cooling leave the fans at a safe duty and tell
// its guard goodbye.
//
// Any program of the session can send that message to a window it finds, so
// the message alone proves nothing. onEnd runs only if the system says that the
// session is shutting down (GetSystemMetrics, SM_SHUTTINGDOWN, which covers
// logging off as well as shutting down). A real end that is not recognised
// stays safe: the system ends the process, the guard sees that it did so
// without a goodbye and sets the fans, and the sensor service gives a pump
// header back to the board when the connection ends.

const (
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wsPopup           = 0x80000000
	smShuttingDown    = 0x2000

	// A refused message is logged at most this often.
	sessionIgnoreLogEvery = time.Minute
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetModuleHandleW = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

// sessionShuttingDown asks the system whether the session is shutting down. A
// test replaces it before the window starts.
var sessionShuttingDown = func() bool {
	r, _, _ := procGetSystemMetrics.Call(smShuttingDown)
	return r != 0
}

var (
	sessionNow       = time.Now
	ignoredLogAt     atomic.Int64 // unix nanoseconds of the last log line about a refused message
	ignoredSinceLast atomic.Int64
)

// endSessionMessage handles WM_ENDSESSION. It reports whether onEnd was called.
func endSessionMessage(wParam uintptr) bool {
	if wParam == 0 {
		return false // another program vetoed: the session goes on
	}
	if !sessionShuttingDown() {
		ignoredSinceLast.Add(1)
		now := sessionNow().UnixNano()
		last := ignoredLogAt.Load()
		if (last == 0 || now-last >= int64(sessionIgnoreLogEvery)) && ignoredLogAt.CompareAndSwap(last, now) {
			log.Printf("session watch: end-of-session message ignored, the system is not shutting down (%d since the last note)", ignoredSinceLast.Swap(0))
		}
		return false
	}
	log.Println("the session is ending")
	(*onEnd.Load())()
	return true
}

// sessionClass is the class name of the hidden window; a test sets its own so
// it never addresses the window of an installed instance.
var sessionClass = "CaseHUDSessionEnd"

var (
	onEnd   atomic.Pointer[func()]
	started atomic.Bool
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

// watchSessionEnd starts the hidden window; onEnd is called once, when the session
// is ending for good (not when another program only asked and then vetoed).
func watchSessionEnd(end func()) {
	onEnd.Store(&end)
	if !started.CompareAndSwap(false, true) {
		return
	}
	go func() {
		runtime.LockOSThread()
		proc := syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
			switch msg {
			case wmQueryEndSession:
				return 1 // never the one to veto
			case wmEndSession:
				endSessionMessage(wParam)
				return 0
			}
			r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
			return r
		})
		inst, _, _ := procGetModuleHandleW.Call(0)
		name, _ := windows.UTF16PtrFromString(sessionClass)
		wc := wndClassEx{wndProc: proc, instance: windows.Handle(inst), className: name}
		wc.size = uint32(unsafe.Sizeof(wc))
		if a, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); a == 0 {
			log.Println("session watch: register class:", err)
			return
		}
		h, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)),
			wsPopup, 0, 0, 0, 0, 0, 0, inst, 0)
		if h == 0 {
			log.Println("session watch: create window:", err)
			return
		}
		var msg winMsg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(ret) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}
