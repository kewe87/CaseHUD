// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/kewe87/casehud/internal/panel"
)

// The 8.8" keeps a boot picture that earlier versions wrote (BootImages
// record). Removing it is a permanent delete on the device and happens only on
// an explicit request, only for a device the record names, and only for the
// fixed file the panel package owns; the path never passes through here.

// bootEraser returns the panel's eraser and the record key if CaseHUD wrote a
// boot picture to this device.
func (e *Engine) bootEraser(p panel.Panel) (panel.BootEraser, string, bool) {
	be, ok := p.(panel.BootEraser)
	if !ok || !be.HasBootImage() {
		return nil, "", false
	}
	key := be.SettingsKey()
	if key == "" {
		key = p.ID()
	}
	e.mu.Lock()
	v, rec := e.cfg.BootImages[key]
	e.mu.Unlock()
	if !rec || v == "" || strings.HasPrefix(v, bootPendingPrefix) {
		return nil, "", false
	}
	return be, key, true
}

// BootLogoRecorded returns the number of connected displays on which CaseHUD
// wrote a boot picture earlier.
func (e *Engine) BootLogoRecorded() int {
	if e.Demo {
		return 0
	}
	n := 0
	for _, p := range e.connectedPanels() {
		if _, _, ok := e.bootEraser(p); ok {
			n++
		}
	}
	return n
}

// RemoveBootLogos removes the boot picture CaseHUD wrote from every connected
// display that the record names. The result is checked with a size query; the
// record is dropped only when the file is gone. confirm must be true: the
// caller has shown the user the permanent-write warning. It returns the number
// of displays that are clean afterwards.
func (e *Engine) RemoveBootLogos(confirm bool) (int, error) {
	if !confirm {
		return 0, fmt.Errorf("%w: confirm the permanent change first", ErrAckRequired)
	}
	if !e.outputAllowed() {
		return 0, fmt.Errorf("%w: accept the disclaimer in the CaseHUD window first", ErrAckRequired)
	}
	if e.Demo {
		return 0, nil
	}
	e.bootMu.Lock()
	defer e.bootMu.Unlock()
	done := 0
	var errs []error
	for _, p := range e.connectedPanels() {
		be, key, ok := e.bootEraser(p)
		if !ok {
			continue
		}
		if err := e.removeBootLogo(be, key); err != nil {
			log.Printf("boot picture on %s: %v", p.Name(), err)
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
			continue
		}
		done++
	}
	return done, errors.Join(errs...)
}

func (e *Engine) removeBootLogo(be panel.BootEraser, key string) error {
	size, err := be.BootFileSize()
	if err != nil {
		return err
	}
	if size > 0 {
		if err := be.DeleteBootFile(); err != nil {
			return err
		}
		if size, err = be.BootFileSize(); err != nil {
			return err
		}
		if size != 0 {
			return fmt.Errorf("boot picture still present (%d bytes)", size)
		}
	}
	e.mu.Lock()
	delete(e.cfg.BootImages, key)
	e.mu.Unlock()
	return e.persist()
}
