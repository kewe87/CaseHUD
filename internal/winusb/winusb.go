// Package winusb is a small bulk-transfer transport for USB displays
// that ship Microsoft OS descriptors, so Windows binds the in-box WinUSB
// driver on its own (no Zadig/libusb driver swap). It uses winusb.dll without
// cgo; other platforms get a stub.
package winusb

import "errors"

// ErrTimeout is returned by Read and Write when the transfer didn't finish in time.
var ErrTimeout = errors.New("winusb: transfer timeout")

// Info describes an enumerated WinUSB device.
type Info struct {
	Path      string
	VendorID  uint16
	ProductID uint16
	Serial    string // from the instance ID, "" if unknown
	// Location is the first LocationPaths entry of the device, e.g.
	// "PCIROOT(0)#PCI(0801)#USBROOT(0)#USB(3)#USB(1)"; "" if unknown.
	Location string
}

// Device is an open handle to a WinUSB device (first interface).
type Device interface {
	// Write sends buf to a bulk OUT endpoint, waiting at most timeoutMS.
	Write(ep byte, buf []byte, timeoutMS int) (int, error)
	// Read reads from a bulk IN endpoint, waiting at most timeoutMS. Returns
	// ErrTimeout if nothing arrived; an early (0, nil) is a zero-length packet.
	Read(ep byte, buf []byte, timeoutMS int) (int, error)
	// ZeroLengthTerminate makes writes to ep end with a zero-length packet
	// when their size is a multiple of the endpoint's packet size, so the
	// device sees the end of the transfer.
	ZeroLengthTerminate(ep byte) error
	// ClearHalt clears a stalled endpoint.
	ClearHalt(ep byte) error
	Close() error
}

// ID is a USB vendor/product pair.
type ID struct{ VendorID, ProductID uint16 }

// Enumerate returns every present WinUSB device matching one of ids.
func Enumerate(ids ...ID) ([]Info, error) { return enumerate(ids) }

// Open opens the device at info.Path.
func Open(info Info) (Device, error) { return open(info) }
