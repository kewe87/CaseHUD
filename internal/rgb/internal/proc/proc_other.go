//go:build !windows

// Package proc reports running processes by executable name.
package proc

// Running is not implemented outside Windows and reports nothing.
func Running(names []string) []string { return nil }
