//go:build windows

package update

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The setup's uninstall entry (Inno Setup AppId + "_is1").
const uninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{6F3A9C2E-5B71-4C0D-9E8A-CA5E0D1A2B30}_is1`

// Installed reports whether exe is the CaseHUD.exe of a setup installation.
// The portable ZIP is not: it has no setup to update it.
func Installed(exe string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return false
	}
	defer k.Close()
	loc, _, err := k.GetStringValue("InstallLocation")
	if err != nil || loc == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(filepath.Dir(exe)), filepath.Clean(loc))
}

// Start runs the setup at path with args. Before that it opens the file so
// that nobody can change, rename or delete it while it runs (read sharing
// only) and checks through that handle that its content still has the
// digest want. The setup asks for administrator rights itself. wait blocks
// until the setup ends and returns its exit code; it also releases the file.
func Start(path string, want [32]byte, args []string) (wait func() (int, error), err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		f.Close()
		return nil, err
	}
	var got [32]byte
	copy(got[:], hash.Sum(nil))
	if got != want {
		f.Close()
		return nil, errors.New("the setup changed after the download")
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, err
	}
	return func() (int, error) {
		defer f.Close()
		err := cmd.Wait()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		if err != nil {
			return -1, err
		}
		return 0, nil
	}, nil
}
