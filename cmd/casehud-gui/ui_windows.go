//go:build windows

package main

import (
	"errors"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"fyne.io/systray"
	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/icon"
)

// The tray's message loop must stay on the main OS thread.
func init() { runtime.LockOSThread() }

var (
	winMu     sync.Mutex
	win       webview2.WebView
	opening   bool         // a window is being created (guards against two at once)
	fallbacks atomic.Int64 // times the browser was opened instead of a window
)

// openWindow shows the GUI window, or focuses it if it's already open.
func openWindow(guiURL string) {
	winMu.Lock()
	if win != nil {
		w := win
		winMu.Unlock()
		w.Dispatch(func() {
			hwnd := windows.HWND(uintptr(w.Window()))
			showWindow(hwnd)
		})
		return
	}
	if opening {
		winMu.Unlock()
		return
	}
	opening = true
	winMu.Unlock()

	go func() {
		runtime.LockOSThread() // WebView2 needs its own UI thread
		rect, maximized, _ := initialGeometry()
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			AutoFocus: true,
			DataPath:  filepath.Join(app.DefaultDir(), "webview"),
			WindowOptions: webview2.WindowOptions{
				Title: "CaseHUD", Width: uint(rect.w()), Height: uint(rect.h()),
			},
		})
		if w == nil {
			// No WebView2 runtime: fall back to the default browser.
			log.Println("WebView2 unavailable, opening browser")
			exec.Command("rundll32", "url.dll,FileProtocolHandler", guiURL).Start()
			fallbacks.Add(1)
			winMu.Lock()
			opening = false
			winMu.Unlock()
			return
		}
		winMu.Lock()
		win = w
		opening = false
		winMu.Unlock()
		hwnd := windows.HWND(uintptr(w.Window()))
		setWindowIcon(hwnd)
		placeWindow(hwnd, rect, maximized)
		showWindow(hwnd)
		// HintMin only stores the minimum for WM_GETMINMAXINFO; it does not
		// resize the window. It must run on the UI thread.
		setMin := func(mon uintptr, dpi int) {
			mw, mh := minWindowSize(mon, dpi)
			w.SetSize(mw, mh, webview2.HintMin)
		}
		setMin(windowMonitor(hwnd))
		stop := make(chan struct{})
		go trackWindowState(hwnd, windowState{int(rect.Left), int(rect.Top), rect.w(), rect.h(), maximized},
			func(mon uintptr, dpi int) { w.Dispatch(func() { setMin(mon, dpi) }) }, stop)
		w.Navigate(guiURL)
		w.Run() // returns when the window is closed; streaming keeps running
		close(stop)
		winMu.Lock()
		win = nil
		winMu.Unlock()
		w.Destroy()
	}()
}

// showWindowWait opens or focuses the window and waits up to 3 s until it is
// visible. The browser fallback (no WebView2 runtime) counts as success.
func showWindowWait(guiURL string) error {
	before := fallbacks.Load()
	openWindow(guiURL)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fallbacks.Load() != before {
			return nil
		}
		winMu.Lock()
		w := win
		winMu.Unlock()
		if w != nil {
			if v, _, _ := procIsWindowVisible.Call(uintptr(w.Window())); v != 0 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("window did not appear")
}

var (
	quitOnce sync.Once
	quitting = make(chan struct{})
)

// quitApp asks the UI loop to end, so main returns and its deferred cleanup
// runs (tray Quit, API quit and signals all end up here). It is safe to call
// repeatedly and before the loop has started: the service goroutine checks
// the closed channel as soon as it runs.
func quitApp() { quitOnce.Do(func() { close(quitting) }) }

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

const (
	wmQuit   = 0x0012
	wmTaskOK = 0x8000 + 1 // WM_APP+1: the taskbar appeared
)

var (
	trayReady atomic.Bool // systray's onReady ran: the icon exists
	mainTID   uint32
)

// runUI runs on the main OS thread until quitApp. The initial open and
// quitting are handled by a service goroutine and showing the window by
// showWindowWait, so none of them depends on the tray.
//
// The tray comes from fyne.io/systray, which creates its hidden window and
// notification icon in Run. Without a taskbar (custom shell, no explorer.exe)
// that fails with "unable to init instance", leaves half-built global state
// and cannot be registered a second time. So the library is started only once
// the taskbar window (Shell_TrayWnd) exists. Until then a plain message loop
// runs and Shell_TrayWnd is polled every 10 s; when explorer shows up the loop
// ends and systray.Run takes the thread. (After that the library re-adds the
// icon itself on "TaskbarCreated", i.e. when explorer restarts.) noTray skips
// the tray entirely.
func runUI(guiURL string, openNow, noTray bool) {
	mainTID = windows.GetCurrentThreadId()
	var msg winMsg
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0) // create this thread's message queue

	go func() {
		if openNow {
			openWindow(guiURL)
		}
		<-quitting
		winMu.Lock()
		if win != nil {
			win.Terminate()
		}
		winMu.Unlock()
		if trayReady.Load() {
			systray.Quit() // removes the icon, ends systray's loop
		}
		procPostThreadMessageW.Call(uintptr(mainTID), wmQuit, 0, 0) // ends whichever loop runs
	}()

	if noTray {
		log.Println("tray disabled (--no-tray)")
		plainLoop(nil)
		return
	}
	if !taskbarPresent() {
		log.Println("no taskbar found, running without tray icon (retrying every 10 s)")
		if !waitForTaskbar() {
			return
		}
	}
	systray.Run(func() {
		systray.SetIcon(icon.ICO(16, 32, 48))
		systray.SetTitle("CaseHUD")
		systray.SetTooltip("CaseHUD")
		mOpen := systray.AddMenuItem("Open CaseHUD", "Show the window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop streaming and exit")
		systray.SetOnTapped(func() { openWindow(guiURL) }) // left click
		trayReady.Store(true)
		for {
			select {
			case <-mOpen.ClickedCh:
				openWindow(guiURL)
			case <-mQuit.ClickedCh:
				quitApp()
			case <-quitting:
				return
			}
		}
	}, func() {})
}

func taskbarPresent() bool {
	c, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(c)), 0)
	return h != 0
}

// waitForTaskbar pumps messages until the taskbar exists (true) or the app is
// quitting (false).
func waitForTaskbar() bool {
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if taskbarPresent() {
					procPostThreadMessageW.Call(uintptr(mainTID), wmTaskOK, 0, 0)
					return
				}
			case <-done:
				return
			}
		}
	}()
	return plainLoop(func(m *winMsg) bool { return m.message == wmTaskOK })
}

// plainLoop is a bare message loop. It returns true when stop says so and
// false on WM_QUIT.
func plainLoop(stop func(*winMsg) bool) bool {
	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return false
		}
		if stop != nil && stop(&msg) {
			return true
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetFgWindow              = user32.NewProc("SetForegroundWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procFindWindowW              = user32.NewProc("FindWindowW")
	procPeekMessageW             = user32.NewProc("PeekMessageW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procPostThreadMessageW       = user32.NewProc("PostThreadMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
)

func showWindow(hwnd windows.HWND) {
	const swShow, swRestore = 5, 9
	// Restore only a minimized window; SW_RESTORE would un-maximize.
	cmd := uintptr(swShow)
	if z, _, _ := procIsIconic.Call(uintptr(hwnd)); z != 0 {
		cmd = swRestore
	}
	procShowWindow.Call(uintptr(hwnd), cmd)
	procSetFgWindow.Call(uintptr(hwnd))
}

// setWindowIcon sets the title bar / taskbar icon from the generated logo.
func setWindowIcon(hwnd windows.HWND) {
	const wmSetIcon, iconSmall, iconBig = 0x0080, 0, 1
	for _, s := range []struct{ size, kind int }{{16, iconSmall}, {48, iconBig}} {
		ico := icon.ICO(s.size)
		bmp := ico[6+16:] // skip ICONDIR + entry: CreateIconFromResourceEx wants the image data
		h, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&bmp[0])), uintptr(len(bmp)), 1, 0x00030000, uintptr(s.size), uintptr(s.size), 0)
		if h != 0 {
			procSendMessageW.Call(uintptr(hwnd), wmSetIcon, uintptr(s.kind), h)
		}
	}
}

func showError(msg string) {
	t, _ := windows.UTF16PtrFromString("CaseHUD")
	m, _ := windows.UTF16PtrFromString(msg)
	const mbIconError = 0x10
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbIconError)
}
