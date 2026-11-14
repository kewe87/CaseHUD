package coolguard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	copyDir    = "guard"
	copyPrefix = "CaseHUD-Guard-"
	// InstalledName is the file name (before the extension) under which the setup
	// puts a second copy of the program beside it, for the guard to run from.
	InstalledName = "CaseHUD-Guard"
)

// InstalledGuard returns the second copy of exe that the setup placed beside it, if
// there is one and it is the very same program. The folder of an installation can
// be written by administrators only, so nothing has to be copied to a place the
// user can write, and the setup replaces the file together with the program.
// Without one (portable use, development) the caller falls back to GuardExe.
func InstalledGuard(exe string) (string, bool) {
	path := filepath.Join(filepath.Dir(exe), InstalledName+filepath.Ext(exe))
	if strings.EqualFold(path, exe) {
		return "", false
	}
	want, size, err := hashFile(exe)
	if err != nil {
		return "", false
	}
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() || st.Size() != size {
		return "", false
	}
	got, _, err := hashFile(path)
	if err != nil || got != want {
		return "", false
	}
	return path, true
}

// RemoveCopies deletes the copies GuardExe made in earlier runs, when the guard
// runs from the installation now. A copy that a guard still runs from stays.
func RemoveCopies(dataDir string) {
	pruneCopies(filepath.Join(dataDir, copyDir), "")
}

// GuardExe returns the program the guard runs: a copy of exe, under a name of its
// own, in the "guard" folder of dataDir. Ending CaseHUD by its name (Stop-Process
// -Name CaseHUD, taskkill /IM CaseHUD.exe) then does not end the guard with it, and the
// guard does not keep the installed program from being replaced. The name carries a
// hash of the content, so a copy is made once per version; copies of other versions
// are removed if no guard runs from them.
func GuardExe(dataDir, exe string) (string, error) {
	sum, size, err := hashFile(exe)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(dataDir, copyDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := copyPrefix + hex.EncodeToString(sum[:6]) + filepath.Ext(exe)
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && st.Size() == size {
		if got, _, err := hashFile(path); err == nil && got == sum {
			pruneCopies(dir, name)
			return path, nil
		}
	}
	tmp, err := copyTo(dir, exe)
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		// A guard of this very version may be running from it right now.
		if st, serr := os.Stat(path); serr == nil && st.Size() == size {
			return path, nil
		}
		return "", err
	}
	pruneCopies(dir, name)
	return path, nil
}

func hashFile(path string) (sum [sha256.Size]byte, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return sum, 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err = io.Copy(h, f)
	if err != nil {
		return sum, 0, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, size, nil
}

// copyTo copies src into a new temporary file in dir and returns its path.
func copyTo(dir, src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, "copy-*.tmp")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(out.Name())
		return "", fmt.Errorf("copying the program for the guard: %w", err)
	}
	return out.Name(), nil
}

// pruneCopies removes the copies other than keep. One that is running cannot be
// removed, and stays.
func pruneCopies(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if n != keep && !e.IsDir() && (strings.HasPrefix(n, copyPrefix) || (strings.HasPrefix(n, "copy-") && strings.HasSuffix(n, ".tmp"))) {
			os.Remove(filepath.Join(dir, n))
		}
	}
}
