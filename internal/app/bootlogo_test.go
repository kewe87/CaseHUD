// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

// eraseBar is an 8.8" stand-in with a file size and a delete counter.
type eraseBar struct {
	bootBar
	size      uint32
	deletes   int
	keep      bool // the file survives the delete
	sizeErr   error
	deleteErr error
}

func (b *eraseBar) BootFileSize() (uint32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size, b.sizeErr
}

func (b *eraseBar) DeleteBootFile() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes++
	if b.deleteErr == nil && !b.keep {
		b.size = 0
	}
	return b.deleteErr
}

func addBar(e *Engine, b *eraseBar, record string) {
	e.mu.Lock()
	e.workers[b.ID()] = &worker{e: e, p: b}
	e.order = append(e.order, b.ID())
	if record != "" {
		if e.cfg.BootImages == nil {
			e.cfg.BootImages = map[string]string{}
		}
		e.cfg.BootImages[b.SettingsKey()] = record
	}
	e.mu.Unlock()
}

func TestRemoveBootLogoNeedsConfirmAndRecord(t *testing.T) {
	e := startEngine(t, true)
	b := &eraseBar{size: 1234}
	addBar(e, b, "")
	if _, err := e.RemoveBootLogos(false); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("without confirm: %v", err)
	}
	if n, err := e.RemoveBootLogos(true); n != 0 || err != nil || b.deletes != 0 {
		t.Fatalf("no record: n=%d err=%v deletes=%d", n, err, b.deletes)
	}
	if e.BootLogoRecorded() != 0 {
		t.Fatal("recorded without a record")
	}
	e.mu.Lock()
	e.cfg.BootImages = map[string]string{b.SettingsKey(): bootPendingPrefix + "x"}
	e.mu.Unlock()
	if n, _ := e.RemoveBootLogos(true); n != 0 || b.deletes != 0 {
		t.Fatalf("pending marker: n=%d deletes=%d", n, b.deletes)
	}
}

func TestRemoveBootLogoNeedsDisclaimer(t *testing.T) {
	e := startEngine(t, true)
	b := &eraseBar{size: 1}
	addBar(e, b, "abc")
	e.mu.Lock()
	e.cfg.Disclaimer.Version = 0
	e.mu.Unlock()
	if _, err := e.RemoveBootLogos(true); !errors.Is(err, ErrAckRequired) || b.deletes != 0 {
		t.Fatalf("err %v deletes %d", err, b.deletes)
	}
}

func TestRemoveBootLogoVerifiesThenDropsRecord(t *testing.T) {
	e := startEngine(t, true)
	b := &eraseBar{size: 1234}
	addBar(e, b, "abc")
	if e.BootLogoRecorded() != 1 {
		t.Fatal("record not seen")
	}
	n, err := e.RemoveBootLogos(true)
	if err != nil || n != 1 || b.deletes != 1 {
		t.Fatalf("n=%d err=%v deletes=%d", n, err, b.deletes)
	}
	if _, ok := e.cfg.BootImages[b.SettingsKey()]; ok {
		t.Fatal("record kept after a verified delete")
	}
	// A second run does nothing: no record, no delete.
	if n, _ := e.RemoveBootLogos(true); n != 0 || b.deletes != 1 {
		t.Fatalf("second run: n=%d deletes=%d", n, b.deletes)
	}
}

func TestRemoveBootLogoFileAlreadyGone(t *testing.T) {
	e := startEngine(t, true)
	b := &eraseBar{size: 0}
	addBar(e, b, "abc")
	if n, err := e.RemoveBootLogos(true); err != nil || n != 1 || b.deletes != 0 {
		t.Fatalf("n=%d err=%v deletes=%d", n, err, b.deletes)
	}
	if len(e.cfg.BootImages) != 0 {
		t.Fatal("record kept")
	}
}

func TestRemoveBootLogoFailuresKeepRecord(t *testing.T) {
	for name, b := range map[string]*eraseBar{
		"survives":     {size: 10, keep: true},
		"delete error": {size: 10, deleteErr: errors.New("usb")},
		"size error":   {size: 10, sizeErr: errors.New("usb")},
	} {
		e := startEngine(t, true)
		addBar(e, b, "abc")
		n, err := e.RemoveBootLogos(true)
		if err == nil || n != 0 {
			t.Fatalf("%s: n=%d err=%v", name, n, err)
		}
		if e.cfg.BootImages[b.SettingsKey()] != "abc" {
			t.Fatalf("%s: record dropped", name)
		}
		if name == "size error" && b.deletes != 0 {
			t.Fatal("deleted without knowing the size")
		}
	}
}

func TestRemoveBootLogoDemoSendsNothing(t *testing.T) {
	countDiscovery(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"disclaimer":{"version":3},"panels":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	b := &eraseBar{size: 10}
	addBar(e, b, "abc")
	if n, err := e.RemoveBootLogos(true); n != 0 || err != nil || b.deletes != 0 || e.BootLogoRecorded() != 0 {
		t.Fatalf("demo: n=%d err=%v deletes=%d", n, err, b.deletes)
	}
}

func TestNeutralStartImage(t *testing.T) {
	logo, hl, err := StartImageFor(TLStyleLogo)
	if err != nil {
		t.Fatal(err)
	}
	neu, hn, err := StartImageFor(TLStyleNeutral)
	if err != nil {
		t.Fatal(err)
	}
	neu2, hn2, _ := StartImageFor(TLStyleNeutral)
	if hl == hn || hn != hn2 || len(neu) != len(neu2) || len(neu) == 0 || len(neu) > 65535 {
		t.Fatalf("hashes %q %q %q, sizes %d %d", hl, hn, hn2, len(neu), len(logo))
	}
	if _, h, _ := StartImage(); h != hl {
		t.Fatal("the default picture changed: existing fans would be written again")
	}
}

func TestStartImageStyleSwitchWritesOnce(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "k"}
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 1 {
		t.Fatalf("writes %d", f.count())
	}
	if err := e.SetTLStartImageStyle("bogus"); err == nil {
		t.Fatal("unknown style accepted")
	}
	if err := e.SetTLStartImageStyle(TLStyleNeutral); err != nil {
		t.Fatal(err)
	}
	if e.TLStartImageStyle() != TLStyleNeutral {
		t.Fatal("style not set")
	}
	e.writeStartImages([]panel.Panel{f}, false)
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 2 {
		t.Fatalf("writes %d, want 2 (logo once, neutral once)", f.count())
	}
	_, hn, _ := StartImageFor(TLStyleNeutral)
	if e.cfg.TLStartImages["k"] != hn {
		t.Fatal("record does not hold the neutral hash")
	}
	// Switch off: a style change writes nothing.
	if err := e.SetTLStartImage(false); err != nil {
		t.Fatal(err)
	}
	if err := e.SetTLStartImageStyle(TLStyleLogo); err != nil {
		t.Fatal(err)
	}
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 2 {
		t.Fatalf("written with the setting off (%d)", f.count())
	}
}
