//go:build windows

package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestStartTrayOnTaskbar(t *testing.T) {
	if !startTrayOnTaskbar(false) {
		t.Error("no tray yet: the taskbar message must start it")
	}
	if startTrayOnTaskbar(true) {
		t.Error("tray running: the library re-adds the icon itself")
	}
}

func TestTaskbarMessageReachesOwnWindow(t *testing.T) {
	// Names of their own, so nothing of an installed instance is addressed, and
	// the message goes to this window only, never as a broadcast.
	id := fmt.Sprintf("%d.%d", os.Getpid(), rand.Int63())
	taskbarClass = "CaseHUDTaskbarTest." + id
	taskbarMessage = "CaseHUDTaskbarCreatedTest." + id
	var got atomic.Int32
	hwndCh := make(chan uintptr, 1)
	stop := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		hwndCh <- watchTaskbar(func() { got.Add(1) })
		var msg winMsg
		for {
			select {
			case <-stop:
				return
			default:
			}
			r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
			if r == 0 {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	defer close(stop)
	hwnd := <-hwndCh
	if hwnd == 0 {
		t.Fatal("window not created")
	}
	n, _ := windows.UTF16PtrFromString(taskbarMessage)
	id2, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(n)))
	if id2 == 0 {
		t.Fatal("register message")
	}
	procSendMessageW.Call(hwnd, 0x0400, 0, 0) // some other message: ignored
	if got.Load() != 0 {
		t.Fatal("reacted to another message")
	}
	procSendMessageW.Call(hwnd, id2, 0, 0)
	if got.Load() != 1 {
		t.Fatalf("callback count %d, want 1", got.Load())
	}
}
