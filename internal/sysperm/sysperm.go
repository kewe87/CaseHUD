// Package sysperm grants the Windows permissions CaseHUD's optional features
// need without running CaseHUD itself as administrator.
package sysperm

// Supported reports whether the setup can run on this OS.
func Supported() bool { return supported }
