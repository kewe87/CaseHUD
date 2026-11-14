// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"io/fs"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func isReparse(fi fs.FileInfo) bool {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return ok && d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// sensorsDir is the folder of the sensor service's settings. It comes from the
// system, not from the environment: this runs elevated, and a user can set
// environment variables for it.
func sensorsDir() (string, error) {
	pd, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", err
	}
	return filepath.Join(pd, "CaseHUD"), nil
}

// isSharingError tells a file that another program has open from other failures.
func isSharingError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// makeAdminOwner makes BUILTIN\Administrators the owner of path: the service only
// honors a file owned by an administrator account, and under some policies a file
// created by an elevated user belongs to that user.
func makeAdminOwner(path string) error {
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, sid, nil, nil, nil)
}
