//go:build !windows

package autostart

import "errors"

const supported = false

// Enabled reports whether autostart is on.
func Enabled() bool { return false }

// Set is unsupported outside Windows (use your desktop's autostart).
func Set(bool) error { return errors.New("autostart is only supported on Windows") }
