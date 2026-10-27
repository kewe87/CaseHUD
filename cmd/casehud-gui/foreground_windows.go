//go:build windows

package main

import "golang.org/x/sys/windows"

var procAllowSetFg = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// allowForeground lets process pid take the foreground.
func allowForeground(pid int) { procAllowSetFg.Call(uintptr(pid)) }
