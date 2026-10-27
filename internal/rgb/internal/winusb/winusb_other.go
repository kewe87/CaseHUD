//go:build !windows

package winusb

import "time"

// Paths is not implemented on this platform.
func Paths(guid GUID, vid, pid uint16) ([]string, error) { return nil, ErrUnsupported }

// Device is an open WinUSB device.
type Device struct{ MaxPacket int }

// Open is not implemented on this platform.
func Open(path string, timeout time.Duration) (*Device, error) { return nil, ErrUnsupported }

// Write is not implemented on this platform.
func (d *Device) Write(buf []byte) error { return ErrUnsupported }

// Close is a no-op.
func (d *Device) Close() error { return nil }
