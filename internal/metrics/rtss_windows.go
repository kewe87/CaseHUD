//go:build windows

package metrics

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RTSS shared memory layout (RTSSSharedMemory.h, version 2.x): a header of
// DWORDs followed by an array of app entries.
const (
	rtssSignature  = 0x52545353 // "RTSS"
	rtssMinVersion = 0x00020000
	rtssHeaderLen  = 9 * 4
	// Offsets inside an app entry: pid, char name[MAX_PATH], flags, time0,
	// time1, frames, frametime.
	rtssOffTime0 = 4 + 260 + 4
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procOpenFileMappingW  = kernel32.NewProc("OpenFileMappingW")
	procRtlMoveMemory     = kernel32.NewProc("RtlMoveMemory")
	procGetTickCount      = kernel32.NewProc("GetTickCount")
	user32                = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWnd  = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPI = user32.NewProc("GetWindowThreadProcessId")
)

// copyMem copies n bytes from a mapped address into a Go slice without
// converting the uintptr to a Go pointer.
func copyMem(addr uintptr, n int) []byte {
	b := make([]byte, n)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&b[0])), addr, uintptr(n))
	return b
}

func readRTSS() (fps, frametime float64, ok bool) {
	name, _ := windows.UTF16PtrFromString("RTSSSharedMemoryV2")
	h, _, _ := procOpenFileMappingW.Call(windows.FILE_MAP_READ, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return 0, 0, false // RTSS not running
	}
	defer windows.CloseHandle(windows.Handle(h))
	addr, err := windows.MapViewOfFile(windows.Handle(h), windows.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		return 0, 0, false
	}
	defer windows.UnmapViewOfFile(addr)

	hdr := copyMem(addr, rtssHeaderLen)
	le := binary.LittleEndian
	sig, ver := le.Uint32(hdr[0:]), le.Uint32(hdr[4:])
	entrySize, arrOff, arrSize := le.Uint32(hdr[8:]), le.Uint32(hdr[12:]), le.Uint32(hdr[16:])
	if sig != rtssSignature || ver < rtssMinVersion || entrySize < rtssOffTime0+16 || arrSize == 0 || arrSize > 512 {
		return 0, 0, false
	}
	raw := copyMem(addr+uintptr(arrOff), int(entrySize*arrSize))
	apps := make([]rtssApp, arrSize)
	for i := range apps {
		e := raw[uint32(i)*entrySize:]
		apps[i] = rtssApp{
			pid:    le.Uint32(e[0:]),
			time0:  le.Uint32(e[rtssOffTime0:]),
			time1:  le.Uint32(e[rtssOffTime0+4:]),
			frames: le.Uint32(e[rtssOffTime0+8:]),
			ftime:  le.Uint32(e[rtssOffTime0+12:]),
		}
	}
	now, _, _ := procGetTickCount.Call()
	return pickRTSSApp(apps, foregroundPID(), uint32(now))
}
