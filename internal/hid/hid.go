// Package hid gives CaseHUD access to USB HID devices through the Windows
// HID class driver (hid.dll), so no driver has to be replaced. Other platforms
// get a stub that reports "not supported". No cgo is involved.
package hid

import "errors"

// ErrTimeout is returned by Read when no report arrived in time.
var ErrTimeout = errors.New("hid: read timeout")

// Info describes one enumerated HID device (a top-level collection).
type Info struct {
	Path      string
	VendorID  uint16
	ProductID uint16
	UsagePage uint16 // 0 if unknown
	Product   string // best-effort product string
	// OutputReportLen is the output report length including the report-ID
	// byte, 0 if unknown.
	OutputReportLen int
}

// Device is an open HID handle.
type Device interface {
	// Write sends one output report. buf[0] is the report ID. The buffer is
	// zero-padded to the device's output report length.
	Write(buf []byte) (int, error)
	// Read reads one input report (buf[0] is the report ID), waiting at most
	// timeoutMS milliseconds. Returns ErrTimeout if nothing arrived.
	Read(buf []byte, timeoutMS int) (int, error)
	Close() error
}

// ID is a USB vendor/product pair.
type ID struct{ VendorID, ProductID uint16 }

// Enumerate returns every present HID device matching one of ids.
func Enumerate(ids ...ID) ([]Info, error) { return enumerate(ids) }

// Open opens the device at info.Path.
func Open(info Info) (Device, error) { return open(info) }

func matches(ids []ID, vid, pid uint16) bool {
	for _, id := range ids {
		if id.VendorID == vid && id.ProductID == pid {
			return true
		}
	}
	return false
}
