// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"os"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

func TestBootPendingMarker(t *testing.T) {
	b := &bootBar{}
	e := bootEngine(t, b, 0)
	ps := []panel.Panel{b}
	key := b.SettingsKey()
	// The marker is on record (and on disk) before the write goes out.
	var during string
	var onDisk bool
	b.onWrite = func() {
		e.mu.Lock()
		during = e.cfg.BootImages[key]
		e.mu.Unlock()
		raw, _ := os.ReadFile(e.Store.path("config.json"))
		onDisk = strings.Contains(string(raw), bootPendingPrefix)
	}
	b.err = panel.ErrBootPartial
	e.writeStartImages(ps, false)
	if !strings.HasPrefix(during, bootPendingPrefix) || !onDisk {
		t.Fatalf("marker during write %q, on disk %v", during, onDisk)
	}
	if got := e.cfg.BootImages[key]; got != during {
		t.Fatalf("failure dropped the marker: %q", got)
	}
	// Next run, no force: nothing is written.
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if b.count() != 1 {
		t.Fatalf("%d writes, the pending marker must block the automatic path", b.count())
	}
	// A different picture (turned bar) is blocked as well.
	e.mu.Lock()
	c := e.cfg.Panels[b.ID()]
	c.Rotation = 90
	e.cfg.Panels[b.ID()] = c
	e.mu.Unlock()
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if b.count() != 1 {
		t.Fatal("a new picture was written onto a possibly hung device")
	}
	// The explicit rewrite goes through; success replaces the marker.
	b.err = nil
	e.writeStartImages(ps, true)
	if b.count() != 2 {
		t.Fatalf("forced rewrite: %d writes", b.count())
	}
	if got := e.cfg.BootImages[key]; got == "" || strings.HasPrefix(got, bootPendingPrefix) {
		t.Fatalf("record after success %q", got)
	}
	// ErrImageWritten also counts as written.
	b2 := &bootBar{err: panel.ErrImageWritten}
	e2 := bootEngine(t, b2, 0)
	e2.writeStartImages([]panel.Panel{b2}, false)
	if got := e2.cfg.BootImages[key]; got == "" || strings.HasPrefix(got, bootPendingPrefix) {
		t.Fatalf("record after ErrImageWritten %q", got)
	}
}

func TestBootClosedLeavesNoMarker(t *testing.T) {
	b := &bootBar{err: panel.ErrClosed}
	e := bootEngine(t, b, 0)
	e.writeStartImages([]panel.Panel{b}, false)
	if got, ok := e.cfg.BootImages[b.SettingsKey()]; ok {
		t.Fatalf("marker %q left although nothing was sent", got)
	}
}
