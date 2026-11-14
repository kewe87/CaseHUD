// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/kewe87/casehud/internal/sysperm"
)

const (
	sensorsFile     = "sensors.json"
	sensorsMaxBytes = 64 * 1024 // the service ignores a larger file
	coolingKey      = "coolingHeaders"
)

// allowCoolingHeader releases one mainboard fan header in the settings file of
// the sensor service (dir\sensors.json). It runs elevated. Other keys of the
// file stay as they are; a file that cannot be read is never replaced. The
// directory must already exist as a plain directory (the setup creates and
// locks it down): the new file inherits its permissions, and fixOwner, if given,
// makes an administrator group its owner, which is what the service demands.
func allowCoolingHeader(dir, key string, fixOwner func(path string) error) (err error) {
	if !sysperm.ValidCoolingHeader(key) {
		return fmt.Errorf("not a fan header of the mainboard: %q", key)
	}
	if fi, err := os.Lstat(dir); err != nil {
		return fmt.Errorf("the settings folder %s is missing; install or repair CaseHUD first: %w", dir, err)
	} else if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || isReparse(fi) {
		return fmt.Errorf("%s is not an ordinary folder", dir)
	}
	path := filepath.Join(dir, sensorsFile)

	doc := map[string]json.RawMessage{}
	switch fi, err := os.Lstat(path); {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	case !fi.Mode().IsRegular() || isReparse(fi):
		return fmt.Errorf("%s is not an ordinary file", path)
	default:
		b, err := readLimited(path)
		if err != nil {
			return fmt.Errorf("cannot read %s, leaving it as it is: %w", path, err)
		}
		if err := json.Unmarshal(b, &doc); err != nil || doc == nil {
			return fmt.Errorf("%s is not valid settings JSON, leaving it as it is", path)
		}
	}

	var keys []string
	if raw, ok := doc[coolingKey]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := json.Unmarshal(raw, &keys); err != nil {
			return fmt.Errorf("%s in %s is not a list of strings, leaving it as it is", coolingKey, path)
		}
	}
	if !slices.Contains(keys, key) {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	enc, _ := json.Marshal(keys)
	doc[coolingKey] = enc

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if len(out) > sensorsMaxBytes {
		return fmt.Errorf("%s would become larger than %d bytes", path, sensorsMaxBytes)
	}

	tmp, err := os.CreateTemp(dir, "sensors-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(out); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if fixOwner != nil {
		if err = fixOwner(tmp.Name()); err != nil {
			return fmt.Errorf("cannot make an administrator the owner of the new file: %w", err)
		}
	}
	return renameRetrying(tmp.Name(), path, os.Rename, isSharingError, renameTries, renameWait)
}

const (
	renameTries = 20
	renameWait  = 100 * time.Millisecond
)

// renameRetrying renames from to to. The service holds the file open for a moment
// whenever it reads it, and a rename onto an open file fails then; that passes, so
// it is tried again for a short while. Any other failure is returned at once.
func renameRetrying(from, to string, rename func(from, to string) error, retryable func(error) bool, tries int, wait time.Duration) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = rename(from, to); err == nil || !retryable(err) {
			return err
		}
		time.Sleep(wait)
	}
	return fmt.Errorf("the file %s is in use, try again in a moment: %w", filepath.Base(to), err)
}

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, sensorsMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > sensorsMaxBytes {
		return nil, errors.New("file too large")
	}
	return b, nil
}
