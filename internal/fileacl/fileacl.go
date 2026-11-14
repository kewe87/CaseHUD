// Package fileacl writes files that hold secrets: private to the current user.
package fileacl

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to path atomically and private to the user: into a
// temporary file of the same folder that is restricted to the user before
// anything goes in, then renamed over path.
func WriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp") // created 0600
	if err != nil {
		return err
	}
	name := tmp.Name()
	// Restrict the (still empty) file before the secret is written into it.
	if err := Restrict(name); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	if err := Restrict(name); err != nil { // Windows: explicit DACL
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
