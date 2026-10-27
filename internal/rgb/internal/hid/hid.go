// Package hid is a minimal HID transport (Windows hid.dll; stub elsewhere).
package hid

import (
	"errors"
	"strconv"
	"strings"
)

// ErrUnsupported is returned on platforms without an implementation.
var ErrUnsupported = errors.New("hid: not supported on this platform")

// ErrTimeout is returned when a read or write does not complete in time.
var ErrTimeout = errors.New("hid: timeout")

// DeviceInfo describes one HID top-level collection.
type DeviceInfo struct {
	Path       string
	VendorID   uint16
	ProductID  uint16
	UsagePage  uint16
	Usage      uint16
	Interface  int // USB interface number, -1 if unknown
	InputLen   int // input report length including the report ID byte
	OutputLen  int // output report length including the report ID byte
	FeatureLen int
	Product    string
}

// interfaceFromPath extracts the USB interface number ("&mi_02") from a device path.
func interfaceFromPath(path string) int {
	p := strings.ToLower(path)
	i := strings.Index(p, "&mi_")
	if i < 0 || len(p) < i+6 {
		return -1
	}
	n, err := strconv.ParseUint(p[i+4:i+6], 16, 8)
	if err != nil {
		return -1
	}
	return int(n)
}
