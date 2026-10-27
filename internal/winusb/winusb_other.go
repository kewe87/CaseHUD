//go:build !windows

package winusb

import "errors"

var errUnsupported = errors.New("winusb: only Windows is supported")

func enumerate([]ID) ([]Info, error) { return nil, errUnsupported }
func open(Info) (Device, error)      { return nil, errUnsupported }
