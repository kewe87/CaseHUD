package lianlitl

import "github.com/kewe87/casehud/internal/rgb/internal/hid"

// Present reports whether the TL controller is attached. It only reads the
// HID device list and the IDs in the paths: nothing is opened.
func Present() bool { return hid.Listed(vendorID, productTL) }
