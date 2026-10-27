//go:build !windows

package web

// restrictFile is a no-op: the file is already created 0600.
func restrictFile(string) error { return nil }
