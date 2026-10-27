//go:build !windows

package leds

import "runtime"

func osVersion() string { return runtime.GOOS }
