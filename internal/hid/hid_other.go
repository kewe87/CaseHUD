//go:build !windows

package hid

import "errors"

var errUnsupported = errors.New("hid: not supported on this platform (Windows only)")

func enumerate([]ID) ([]Info, error) { return nil, errUnsupported }
func open(Info) (Device, error)      { return nil, errUnsupported }
