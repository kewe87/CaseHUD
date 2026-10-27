//go:build windows

package metrics

import "unsafe"

var (
	procGetWindowRect     = user32.NewProc("GetWindowRect")
	procMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW   = user32.NewProc("GetMonitorInfoW")
	procGetClassNameW     = user32.NewProc("GetClassNameW")
	procGetWindowLongW    = user32.NewProc("GetWindowLongW")
	procIsZoomed          = user32.NewProc("IsZoomed")
)

type winRect = pmRect

type winMonitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

// foregroundInfo describes the focused window: its process and whether it
// covers its whole monitor (exclusive fullscreen or borderless).
func foregroundInfo() pmFg {
	hwnd, _, _ := procGetForegroundWnd.Call()
	if hwnd == 0 {
		return pmFg{}
	}
	var fg pmFg
	procGetWindowThreadPI.Call(hwnd, uintptr(unsafe.Pointer(&fg.PID)))
	// The desktop itself covers the monitor but is no game.
	var cls [64]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&cls[0])), uintptr(len(cls)))
	switch name := string(utf16ToRunes(cls[:n])); name {
	case "Progman", "WorkerW", "Shell_TrayWnd":
		return fg
	}
	var r winRect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return fg
	}
	mon, _, _ := procMonitorFromWindow.Call(hwnd, 2) // MONITOR_DEFAULTTONEAREST
	if mon == 0 {
		return fg
	}
	mi := winMonitorInfo{Size: uint32(unsafe.Sizeof(winMonitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return fg
	}
	m := mi.Monitor
	style, _, _ := procGetWindowLongW.Call(hwnd, ^uintptr(15)) // GWL_STYLE = -16
	zoomed, _, _ := procIsZoomed.Call(hwnd)
	fg.Fullscreen = coversMonitor(r, m, uint32(style), zoomed != 0)
	return fg
}

func utf16ToRunes(u []uint16) []rune {
	out := make([]rune, len(u))
	for i, c := range u {
		out[i] = rune(c)
	}
	return out
}
