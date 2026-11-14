package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Safety copy of cooling.json. After every successful save a verified copy of what
// was written goes next to it. A file that cannot be read at the next start is
// kept under a name of its own, and the copy takes its place. Nothing is ever
// overwritten without a copy of what was there.

const (
	coolingBackupFile   = "cooling.json.bak"
	coolingBrokenPrefix = "cooling.json.broken-"
	// A damaged file larger than this is not copied, and then not replaced either.
	coolingKeepMaxBytes = 64 << 20
)

// readFileCapped reads a file of at most max bytes; larger is reported by big.
func readFileCapped(path string, max int64) (b []byte, big bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	b, err = io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > max {
		return nil, true, nil
	}
	return b, false, nil
}

// writeVerified puts data at path through a temporary file that is read back and
// parsed first, so a bad copy never replaces a good one.
func writeVerified(path string, data []byte, parse func([]byte) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // after a successful rename it is gone already
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	got, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, data) {
		return errors.New("the copy differs from what was written")
	}
	if err := parse(got); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func parseCoolingBytes(b []byte) error {
	_, err := parseCoolingConfig(b)
	return err
}

// backupCooling stores the verified copy of data. A failure is reported, never
// fatal: the save itself went through.
func (s *Store) backupCooling(data []byte) {
	if err := writeVerified(s.path(coolingBackupFile), data, parseCoolingBytes); err != nil {
		log.Printf("cooling: no safety copy of %s: %v", coolingFile, withoutPath(err))
	}
}

// keepBrokenCooling copies cooling.json to a name of its own if it cannot be
// parsed. It returns that name; "" means there was nothing to keep (no file, or a
// valid one). An error means the file could not be kept: the caller must then
// leave it alone.
func (s *Store) keepBrokenCooling() (string, error) {
	b, big, err := readFileCapped(s.path(coolingFile), coolingKeepMaxBytes)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("%s cannot be read: %w", coolingFile, withoutPath(err))
	case big:
		return "", fmt.Errorf("%s is larger than %d bytes and is not copied", coolingFile, coolingKeepMaxBytes)
	}
	if _, err := parseCoolingConfig(b); err == nil {
		return "", nil
	}
	return s.keepCopy(b)
}

// keepCopy writes b under a new name, never over an existing file.
func (s *Store) keepCopy(b []byte) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	for n := 0; n < 100; n++ {
		name := coolingBrokenPrefix + stamp
		if n > 0 {
			name = fmt.Sprintf("%s-%d", name, n)
		}
		f, err := os.OpenFile(s.path(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("keeping the damaged %s: %w", coolingFile, withoutPath(err))
		}
		_, werr := f.Write(b)
		if werr == nil {
			werr = f.Sync()
		}
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(s.path(name))
			return "", fmt.Errorf("keeping the damaged %s: %w", coolingFile, withoutPath(werr))
		}
		if got, err := os.ReadFile(s.path(name)); err != nil || !bytes.Equal(got, b) {
			os.Remove(s.path(name))
			return "", fmt.Errorf("keeping the damaged %s: the copy differs", coolingFile)
		}
		return name, nil
	}
	return "", errors.New("no free name for the damaged " + coolingFile)
}

// restoreCooling replaces an unreadable cooling.json by the safety copy. ok is
// false, and nothing changes, without a copy that parses. kept is the name under
// which the damaged file was kept. It is copied first; if that fails, the copy is
// used for this run only and the file stays where it is, so kept is its own name.
func (s *Store) restoreCooling() (cfg coolingConfig, kept string, ok bool) {
	b, _, err := readFileCapped(s.path(coolingBackupFile), coolingMaxBytes)
	if err != nil || b == nil {
		return cfg, "", false
	}
	cfg, err = parseCoolingConfig(b)
	if err != nil {
		log.Printf("cooling: the safety copy of %s is not usable either: %v", coolingFile, err)
		return coolingConfig{}, "", false
	}
	kept, err = s.keepBrokenCooling()
	if err != nil {
		log.Printf("cooling: %v; %s stays as it is, the safety copy is used for this run", err, coolingFile)
		return cfg, coolingFile, true
	}
	if kept == "" {
		// The file reads fine now (a transient error): it counts, not the copy.
		return coolingConfig{}, "", false
	}
	if err := writeFileAtomic(s.path(coolingFile), b); err != nil {
		log.Printf("cooling: restoring %s from the safety copy: %v", coolingFile, withoutPath(err))
	}
	return cfg, kept, true
}

// ensureCoolingBackup makes the first copy for a file that was saved before the
// copies existed. An existing copy is left alone.
func (s *Store) ensureCoolingBackup(data []byte) {
	if _, err := os.Stat(s.path(coolingBackupFile)); errors.Is(err, fs.ErrNotExist) {
		s.backupCooling(data)
	}
}

// saveCoolingFile writes cooling.json: a damaged file that is there gets a copy
// of its own first, and the new content gets its safety copy after it.
func (e *Engine) saveCoolingFile(data []byte) error {
	if _, err := e.Store.keepBrokenCooling(); err != nil {
		return err
	}
	if err := writeFileAtomic(e.Store.path(coolingFile), data); err != nil {
		return err
	}
	e.Store.backupCooling(data)
	e.crash.clearRestored()
	return nil
}

// loadOrRestoreCooling reads cooling.json at the start. If it cannot be read,
// the safety copy takes over and the engine remembers the name under which the
// damaged file was kept, for the alarm.
func (e *Engine) loadOrRestoreCooling() (coolingConfig, error) {
	cfg, err := e.Store.loadCoolingConfig()
	if err == nil {
		if b, _, rerr := readFileCapped(e.Store.path(coolingFile), coolingMaxBytes); rerr == nil && b != nil {
			e.Store.ensureCoolingBackup(b)
		}
		return cfg, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}
	restored, kept, ok := e.Store.restoreCooling()
	if !ok {
		return cfg, err
	}
	log.Printf("cooling: %s was not usable (%s); running on the safety copy, the damaged file is kept as %q", coolingFile, shortCoolingError(err), kept)
	e.crash.markRestored(kept)
	return restored, nil
}
