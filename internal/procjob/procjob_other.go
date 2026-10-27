//go:build !windows

// Package procjob ties child processes to the lifetime of CaseHUD (Windows only).
package procjob

import "os"

// Assign is a no-op outside Windows.
func Assign(*os.Process) {}
