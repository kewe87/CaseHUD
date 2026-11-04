package lianli

import "github.com/kewe87/casehud/internal/rgb/internal/winusb"

// Present reports whether the LED ring is attached. It only lists the USB
// device interfaces by vendor and product ID: the device is not opened and
// nothing is sent to it.
func Present() bool {
	paths, err := winusb.Paths(interfaceGUID, vendorID, productRing88)
	return err == nil && len(paths) > 0
}
