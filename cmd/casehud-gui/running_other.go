//go:build !windows

package main

// tryHold: there is no single-instance lock outside Windows.
func tryHold() bool { return true }

func uiGerman() bool { return false }
