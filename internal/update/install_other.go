//go:build !windows

package update

import "errors"

// Installed is always false outside Windows: there is no setup.
func Installed(string) bool { return false }

// Start is only supported on Windows.
func Start(string, [32]byte, []string) (func() (int, error), error) {
	return nil, errors.New("updates are only installed on Windows")
}
