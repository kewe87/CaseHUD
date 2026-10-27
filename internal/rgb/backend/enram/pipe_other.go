//go:build !windows

package enram

import "os"

func openPipe(name string) (*os.File, error) { return os.OpenFile(name, os.O_RDWR, 0) }

// verifyServer: named pipes of the bridge only exist on Windows.
func verifyServer(*os.File) error { return ErrUntrusted }
