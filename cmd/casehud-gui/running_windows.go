//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// The mutex runningMutexName is held while CaseHUD runs. It is the single-instance
// lock, and the setup waits for it to go away after asking CaseHUD to quit
// (--quit), before it replaces files.

// tryHold takes the single-instance lock; false if it did not get it. Why not,
// if it is not simply held, is noted for the message (noteHoldFailure).
func tryHold() bool {
	held, err := holdNamed(runningMutexName)
	if err != nil {
		noteHoldFailure(err)
	}
	return held
}

// tryHoldNamed is holdNamed without the reason.
func tryHoldNamed(name string) bool {
	held, _ := holdNamed(name)
	return held
}

// holdNamed creates the named mutex. CreateMutex is atomic: exactly one caller
// gets it without ERROR_ALREADY_EXISTS. The error is nil if the mutex is held by
// another (the same or another account, elevated or not) and non-nil if nothing is
// known about that: then the lock is not held either, so no second instance can
// start on a guess. That is the case when the name belongs to an object of another
// type.
func holdNamed(name string) (held bool, err error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false, err
	}
	h, err := windows.CreateMutex(nil, false, p)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return false, nil
	}
	if h == 0 {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			// Another account's or an elevated process owns it.
			return false, nil
		}
		return false, err
	}
	// The handle stays open on purpose: the mutex goes away when the process ends.
	return true, nil
}

// uiGerman reports whether the user's first UI language is German.
func uiGerman() bool {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	return err == nil && len(langs) > 0 && len(langs[0]) >= 2 && (langs[0][:2] == "de" || langs[0][:2] == "DE")
}
