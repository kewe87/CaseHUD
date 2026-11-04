// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

// bootBar is an 8.8" stand-in that records boot picture writes.
type bootBar struct {
	mu     sync.Mutex
	writes int
}

func (b *bootBar) ID() string              { return "us88-0" }
func (b *bootBar) Name() string            { return "bar" }
func (b *bootBar) Size() image.Point       { return image.Pt(1920, 480) }
func (b *bootBar) ShowJPEG([]byte) error   { return nil }
func (b *bootBar) MaxJPEG() int            { return 450000 }
func (b *bootBar) SetBrightness(int) error { return nil }
func (b *bootBar) SetRotation(int) error   { return nil }
func (b *bootBar) Close() error            { return nil }
func (b *bootBar) SettingsKey() string     { return "us-us88-0" }
func (b *bootBar) HasBootImage() bool      { return true }
func (b *bootBar) count() int              { b.mu.Lock(); defer b.mu.Unlock(); return b.writes }
func (b *bootBar) WriteBootImage(image.Image, int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writes++
	return nil
}

var _ panel.BootImager = (*bootBar)(nil)

// The 8.8" boot picture is no longer written, automatically or on request.
func TestBootImageIsNeverWritten(t *testing.T) {
	b := &bootBar{}
	e := startEngine(t, true)
	if err := e.SetTLStartImage(true); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.workers[b.ID()] = &worker{e: e, p: b}
	e.order = append(e.order, b.ID())
	e.mu.Unlock()
	e.writeStartImages([]panel.Panel{b}, false)
	e.writeStartImages([]panel.Panel{b}, true)
	if b.count() != 0 {
		t.Fatalf("%d boot writes, want 0", b.count())
	}
	if len(e.cfg.BootImages) != 0 {
		t.Fatalf("boot record changed: %v", e.cfg.BootImages)
	}
	if n, err := e.RewriteStartImages(true); err != nil || n != 0 {
		t.Fatalf("rewrite counted the 8.8\": n=%d err=%v", n, err)
	}
}

// Pending markers of earlier versions are dropped at load; confirmed records stay.
func TestLoadDropsBootPendingMarkers(t *testing.T) {
	dir := t.TempDir()
	raw := `{"panels":{},"boot_images":{"a":"pending:x","b":"abc"}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := (&Store{Dir: dir}).LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.BootImages["a"]; ok || c.BootImages["b"] != "abc" {
		t.Fatalf("boot record after load: %v", c.BootImages)
	}
}
