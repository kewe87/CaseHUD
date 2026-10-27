// Package autostart registers the GUI to start with Windows (HKCU Run key).
package autostart

// Supported reports whether autostart can be configured on this OS.
func Supported() bool { return supported }
