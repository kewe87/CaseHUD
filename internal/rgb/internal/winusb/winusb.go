// Package winusb is a minimal WinUSB bulk-out transport (Windows; stub elsewhere).
package winusb

import (
	"errors"
	"strings"
)

// ErrUnsupported is returned on platforms without an implementation.
var ErrUnsupported = errors.New("winusb: not supported on this platform")

// GUID identifies a device interface class.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// matchIDs reports whether a device interface path belongs to vid:pid.
func matchIDs(path string, vid, pid uint16) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "vid_"+hex4(vid)+"&pid_"+hex4(pid))
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12&0xF], digits[v>>8&0xF], digits[v>>4&0xF], digits[v&0xF]})
}
