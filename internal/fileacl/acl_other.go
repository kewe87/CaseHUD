//go:build !windows

package fileacl

import "os"

// Restrict is a no-op: the file is already created 0600.
func Restrict(string) error { return nil }

// CopyDACL gives dst the permission bits of src.
func CopyDACL(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, fi.Mode().Perm())
}
