// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/coolguard"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

const (
	guardLogFile    = "cooling-guard.log"
	guardLogMax     = 64 << 10
	guardSingleWait = 30 * time.Second
)

// runCoolingGuard is the whole program of "CaseHUD.exe --cooling-guard=<pid>": it
// waits for the end of the CaseHUD process pid and sets the fans to their safe
// duty if that end was not announced. It starts no window and no engine, and opens
// the fan controller only when it acts. It returns the exit code.
func runCoolingGuard(pid int, dataDir string) int {
	open := func(ctx context.Context) (coolguard.Fans, error) {
		d, err := lianlitl.OpenFans(ctx)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	return runGuard(pid, dataDir, open, app.CoolingRivalRunning)
}

// runGuard is runCoolingGuard with the controller and the check for other fan
// programs given, so that a test can run the real process without real fans.
func runGuard(pid int, dataDir string, open func(context.Context) (coolguard.Fans, error), rival func() (bool, error)) int {
	// The mutexes of this process belong to its thread.
	runtime.LockOSThread()
	if f, err := openGuardLog(filepath.Join(dataDir, guardLogFile)); err == nil {
		defer f.Close()
		log.SetOutput(f)
	}
	logf := func(format string, args ...any) { log.Printf(format, args...) }
	exe, _ := os.Executable()
	logf("guard of process %d started, running from %s", pid, exe)

	if !coolguard.AcquireSingle(guardSingleWait) {
		logf("another guard is still at work, this one ends")
		return 1
	}
	gone, err := coolguard.WatchParent(pid)
	if err != nil {
		logf("cannot watch process %d: %v", pid, err)
		return 1
	}
	out := coolguard.Run(context.Background(), coolguard.Options{
		In:         os.Stdin,
		ParentGone: gone,
		Open:       open,
		Rival:      rival,
		Acting:     coolguard.BeginAct,
		Log:        logf,
	})
	logf("guard ends: %v", out)
	if out == coolguard.Failed {
		return 1
	}
	return 0
}

// openGuardLog opens the log of the guard; it starts anew once it is large.
func openGuardLog(path string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(path); err == nil && st.Size() > guardLogMax {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("guard log: %w", err)
	}
	return f, nil
}
