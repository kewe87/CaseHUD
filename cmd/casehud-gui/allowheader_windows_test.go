// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestTheSettingsFolderComesFromTheSystemNotTheEnvironment(t *testing.T) {
	want, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ProgramData", `C:\Users\somebody\planted`)
	got, err := sensorsDir()
	if err != nil || got != filepath.Join(want, "CaseHUD") {
		t.Fatalf("%q %v, want %q", got, err, filepath.Join(want, "CaseHUD"))
	}
	t.Setenv("ProgramData", "")
	if got, err := sensorsDir(); err != nil || got != filepath.Join(want, "CaseHUD") {
		t.Fatalf("%q %v", got, err)
	}
}
