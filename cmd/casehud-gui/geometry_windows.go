//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Window geometry. The process is Per-Monitor-DPI aware (release builds get
// the PerMonitorV2 manifest from scripts/package-windows.ps1; dev builds
// without the .syso fall back to the call in init()), so all values below are
// physical pixels. Minimum and default sizes are given in logical pixels and
// scaled by the monitor's DPI.
const (
	defaultW, defaultH = 1280, 860 // smallest default size that does not clip the UI
	minW, minH         = 960, 560  // smallest size the user can resize to (frontend breakpoint: 960 px)
	defaultFraction    = 0.8       // share of the work area on first start
)

var (
	shcore                        = windows.NewLazySystemDLL("shcore.dll")
	procGetDpiForMonitor          = shcore.NewProc("GetDpiForMonitor")
	procSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetCursorPos              = user32.NewProc("GetCursorPos")
	procMonitorFromPoint          = user32.NewProc("MonitorFromPoint")
	procMonitorFromRect           = user32.NewProc("MonitorFromRect")
	procMonitorFromWindow         = user32.NewProc("MonitorFromWindow")
	procGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	procGetMonitorInfoW           = user32.NewProc("GetMonitorInfoW")
	procGetWindowRect             = user32.NewProc("GetWindowRect")
	procIsZoomed                  = user32.NewProc("IsZoomed")
	procIsIconic                  = user32.NewProc("IsIconic")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
)

func init() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4. Must run before any
	// window exists. It only takes effect for dev builds without the manifest;
	// with the manifest the awareness is already set and the call fails
	// harmlessly (as it does on old Windows).
	m := ^uintptr(3)
	procSetProcessDpiAwarenessCtx.Call(m)
}

type winRect struct{ Left, Top, Right, Bottom int32 }

func (r winRect) w() int { return int(r.Right - r.Left) }
func (r winRect) h() int { return int(r.Bottom - r.Top) }

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

// windowState is persisted as window.json in the data folder.
type windowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	W         int  `json:"w"`
	H         int  `json:"h"`
	Maximized bool `json:"maximized"`
}

const monitorDefaultToNull = 0

func monitorWork(mon uintptr) (winRect, bool) {
	mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.Work, r != 0
}

// monitorDPI returns the effective DPI of a monitor (96 = 100 %).
func monitorDPI(mon uintptr) int {
	var x, y uint32
	if r, _, _ := procGetDpiForMonitor.Call(mon, 0 /* MDT_EFFECTIVE_DPI */, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); r == 0 && x > 0 {
		return int(x)
	}
	return 96
}

func loadWindowState() (windowState, bool) {
	var s windowState
	b, err := os.ReadFile(filepath.Join(stateDir, "window.json"))
	if err != nil || json.Unmarshal(b, &s) != nil || s.W <= 0 || s.H <= 0 {
		return s, false
	}
	return s, true
}

// saveWindowState writes window.json atomically (temp file + rename).
func saveWindowState(s windowState) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	path := filepath.Join(stateDir, "window.json")
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) != nil {
		return
	}
	if os.Rename(tmp, path) != nil {
		os.Remove(tmp)
	}
}

// windowMonitor returns the monitor the window is on (nearest if none) and
// its DPI (GetDpiForWindow, falling back to the monitor's effective DPI).
func windowMonitor(hwnd windows.HWND) (mon uintptr, dpi int) {
	const monitorDefaultToNearest = 2
	mon, _, _ = procMonitorFromWindow.Call(uintptr(hwnd), monitorDefaultToNearest)
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(uintptr(hwnd)); d > 0 {
			return mon, int(d)
		}
	}
	return mon, monitorDPI(mon)
}

// minWindowSize is the minimum window size in physical pixels for the monitor
// the window is on: minW x minH logical, but never larger than its work area.
func minWindowSize(mon uintptr, dpi int) (w, h int) {
	w, h = minW*dpi/96, minH*dpi/96
	if work, ok := monitorWork(mon); ok {
		w, h = min(w, work.w()), min(h, work.h())
	}
	return w, h
}

// clampSize bounds a physical size to [min, work] (work wins if it is smaller).
func clampSize(v, lo, hi int) int {
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// initialGeometry decides where the window starts: the saved rect if it is
// still visible on a connected monitor, otherwise 80 % of the work area of the
// monitor under the cursor (primary monitor if there is none), centered.
func initialGeometry() (r winRect, maximized bool, dpi int) {
	if s, ok := loadWindowState(); ok {
		// Probe the title-bar strip: the window must be reachable by the user.
		cx := int32(s.X + s.W/2)
		probe := winRect{cx - 50, int32(s.Y), cx + 50, int32(s.Y) + 30}
		if mon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&probe)), monitorDefaultToNull); mon != 0 {
			if work, ok := monitorWork(mon); ok {
				dpi = monitorDPI(mon)
				w := clampSize(s.W, minW*dpi/96, work.w())
				h := clampSize(s.H, minH*dpi/96, work.h())
				x, y := s.X, s.Y
				if x+w > int(work.Right) {
					x = int(work.Right) - w
				}
				if y+h > int(work.Bottom) {
					y = int(work.Bottom) - h
				}
				if x < int(work.Left) {
					x = int(work.Left)
				}
				if y < int(work.Top) {
					y = int(work.Top)
				}
				return winRect{int32(x), int32(y), int32(x + w), int32(y + h)}, s.Maximized, dpi
			}
		}
	}

	// MonitorFromPoint(cursor, MONITOR_DEFAULTTOPRIMARY = 1); a zero point
	// resolves to the primary monitor if GetCursorPos fails.
	var pt struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	mon, _, _ := procMonitorFromPoint.Call(uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32, 1)
	work, ok := monitorWork(mon)
	if !ok {
		return winRect{0, 0, defaultW, defaultH}, false, 96
	}
	dpi = monitorDPI(mon)
	w := clampSize(int(float64(work.w())*defaultFraction), defaultW*dpi/96, work.w())
	h := clampSize(int(float64(work.h())*defaultFraction), defaultH*dpi/96, work.h())
	x := int(work.Left) + (work.w()-w)/2
	y := int(work.Top) + (work.h()-h)/2
	return winRect{int32(x), int32(y), int32(x + w), int32(y + h)}, false, dpi
}

// placeWindow moves/sizes a freshly created window and applies the minimum
// size (via the library's WM_GETMINMAXINFO handling, see ui_windows.go).
func placeWindow(hwnd windows.HWND, r winRect, maximized bool) {
	const swpNoZOrder, swpNoActivate = 0x0004, 0x0010
	procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.w()), uintptr(r.h()), swpNoZOrder|swpNoActivate)
	if maximized {
		const swMaximize = 3
		procShowWindow.Call(uintptr(hwnd), swMaximize)
	}
}

// trackWindowState remembers the window geometry until stop is closed. The
// window's message loop belongs to the library, so instead of hooking
// WM_CLOSE the state is sampled and written once it has been stable for a
// tick (and re-applies the minimum size when the monitor or DPI changes,
// via setMin, which must hop to the UI thread); this also survives a quit from the tray or a crash.
func trackWindowState(hwnd windows.HWND, initial windowState, setMin func(mon uintptr, dpi int), stop <-chan struct{}) {
	last, prev := initial, windowState{}
	lastMon, lastDPI := windowMonitor(hwnd)
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		if z, _, _ := procIsIconic.Call(uintptr(hwnd)); z != 0 {
			continue
		}
		// Monitor or DPI changed: the minimum size depends on both.
		if mon, dpi := windowMonitor(hwnd); mon != 0 && (mon != lastMon || dpi != lastDPI) {
			lastMon, lastDPI = mon, dpi
			setMin(mon, dpi)
		}
		cur := last
		if z, _, _ := procIsZoomed.Call(uintptr(hwnd)); z != 0 {
			cur.Maximized = true // keep the last normal rect for restoring
		} else {
			var r winRect
			if ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 || r.w() <= 0 || r.h() <= 0 {
				continue
			}
			cur = windowState{X: int(r.Left), Y: int(r.Top), W: r.w(), H: r.h()}
		}
		if cur == prev && cur != last {
			saveWindowState(cur)
			last = cur
		}
		prev = cur
	}
}
