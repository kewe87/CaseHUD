//go:build windows

package idle

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSystemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

const spiGetScreenSaverRunning = 0x0072

func screensaverRunning() bool {
	var running uint32
	r, _, _ := procSystemParametersInfo.Call(spiGetScreenSaverRunning, 0, uintptr(unsafe.Pointer(&running)), 0)
	return r != 0 && running != 0
}
