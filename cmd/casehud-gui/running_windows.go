//go:build windows

package main

import "golang.org/x/sys/windows"

// runningMutex is held while CaseHUD runs. The setup waits for it to go
// away after asking CaseHUD to quit (--quit), before it replaces files.
const runningMutex = "CaseHUD.Running"

// holdRunning creates the mutex; it is released when the process ends.
func holdRunning() {
	if name, err := windows.UTF16PtrFromString(runningMutex); err == nil {
		windows.CreateMutex(nil, false, name)
	}
}
