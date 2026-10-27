//go:build !windows

package hid

import "time"

// Enumerate is not implemented on this platform.
func Enumerate(vid, pid uint16) ([]DeviceInfo, error) { return nil, ErrUnsupported }

// Device is an open HID collection.
type Device struct{ Info DeviceInfo }

// Open is not implemented on this platform.
func Open(info DeviceInfo) (*Device, error) { return nil, ErrUnsupported }

// Close is a no-op.
func (d *Device) Close() error { return nil }

// Write is not implemented on this platform.
func (d *Device) Write(report []byte, timeout time.Duration) error { return ErrUnsupported }

// Read is not implemented on this platform.
func (d *Device) Read(buf []byte, timeout time.Duration) (int, error) { return 0, ErrUnsupported }

// SetFeature is not implemented on this platform.
func (d *Device) SetFeature(report []byte) error { return ErrUnsupported }

// GetFeature is not implemented on this platform.
func (d *Device) GetFeature(buf []byte) error { return ErrUnsupported }

// GetInput is not implemented on this platform.
func (d *Device) GetInput(buf []byte) error { return ErrUnsupported }
