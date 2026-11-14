//go:build windows

package main

import (
	"log"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Explorer sends the registered message "TaskbarCreated" to every top-level
// window when it creates the taskbar: after a restart, or when a session
// switches from a custom shell to the desktop. A message-only window gets no
// broadcasts, so the receiver is an ordinary hidden popup window. It lives on
// the UI thread, whose message loop dispatches to it.
//
// The tray library re-adds the icon on its own once it runs. This window
// matters before that: it starts the tray when the taskbar shows up after a
// start with --no-tray (or without any taskbar).

var (
	// taskbarClass and taskbarMessage are variables so a test uses names of its
	// own and never reaches the window of an installed instance.
	taskbarClass   = "CaseHUDTaskbarWatch"
	taskbarMessage = "TaskbarCreated"

	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	trayStarted                atomic.Bool // the tray library has been started (or is about to be)
)

// startTrayOnTaskbar says whether a "TaskbarCreated" has to start the tray.
// Once it runs the library handles the message itself.
func startTrayOnTaskbar(trayAlreadyStarted bool) bool { return !trayAlreadyStarted }

// watchTaskbar creates the hidden window on the calling thread and calls
// created for every taskbar message (on that thread, inside its message loop).
// It returns the window, or 0 if it could not be created.
func watchTaskbar(created func()) uintptr {
	n, _ := windows.UTF16PtrFromString(taskbarMessage)
	msgID, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(n)))
	if msgID == 0 {
		log.Println("taskbar watch: register message failed")
		return 0
	}
	proc := syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
		if msg == msgID {
			created()
			return 0
		}
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	})
	inst, _, _ := procGetModuleHandleW.Call(0)
	name, _ := windows.UTF16PtrFromString(taskbarClass)
	wc := wndClassEx{wndProc: proc, instance: windows.Handle(inst), className: name}
	wc.size = uint32(unsafe.Sizeof(wc))
	if a, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); a == 0 {
		log.Println("taskbar watch: register class:", err)
		return 0
	}
	h, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)),
		wsPopup, 0, 0, 0, 0, 0, 0, inst, 0)
	if h == 0 {
		log.Println("taskbar watch: create window:", err)
	}
	return h
}
