// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"image"
	"sync"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

// bootBar is an 8.8" stand-in that records boot picture writes.
type bootBar struct {
	mu      sync.Mutex
	writes  []image.Point
	rots    []int
	none    bool // HasBootImage false (other bar models)
	err     error
	onWrite func()
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
func (b *bootBar) SoftQuarterTurns() bool  { return true }
func (b *bootBar) HasBootImage() bool      { return !b.none }
func (b *bootBar) count() int              { b.mu.Lock(); defer b.mu.Unlock(); return len(b.writes) }
func (b *bootBar) WriteBootImage(img image.Image, rotation int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writes = append(b.writes, img.Bounds().Size())
	b.rots = append(b.rots, rotation)
	if b.onWrite != nil {
		b.onWrite()
	}
	return b.err
}

var _ panel.BootImager = (*bootBar)(nil)

func bootEngine(t *testing.T, b *bootBar, rotation int) *Engine {
	e := startEngine(t, true)
	e.mu.Lock()
	e.workers[b.ID()] = &worker{e: e, p: b}
	c := e.cfg.Panels[b.ID()]
	c.Rotation = rotation
	e.cfg.Panels[b.ID()] = c
	e.mu.Unlock()
	return e
}

func TestBootImageWrittenOncePerDeviceAndPicture(t *testing.T) {
	b := &bootBar{}
	e := bootEngine(t, b, 0)
	ps := []panel.Panel{b}
	e.writeStartImages(ps, false)
	e.writeStartImages(ps, false) // same run
	e.tlp.tried = nil
	e.writeStartImages(ps, false) // next run, hash on record
	if b.count() != 1 || b.writes[0] != image.Pt(1920, 480) {
		t.Fatalf("writes %v, want one wide picture", b.writes)
	}
	if e.cfg.BootImages["us-us88-0"] == "" {
		t.Fatal("boot picture not recorded")
	}
	// Turned upright: a new picture (symbol only), written with the next run.
	e.mu.Lock()
	c := e.cfg.Panels[b.ID()]
	c.Rotation = 90
	e.cfg.Panels[b.ID()] = c
	e.mu.Unlock()
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if b.count() != 2 || b.writes[1] != image.Pt(480, 1920) || b.rots[1] != 90 {
		t.Fatalf("after turning: writes %v rots %v", b.writes, b.rots)
	}
}

func TestBootImageFollowsSettingAndModel(t *testing.T) {
	b := &bootBar{}
	e := bootEngine(t, b, 0)
	e.cfg.TLStartImageOff = true
	e.writeStartImages([]panel.Panel{b}, false)
	if b.count() != 0 {
		t.Fatal("written although the setting is off")
	}
	// Other bar models have no boot picture: nothing is written, nothing counted.
	o := &bootBar{none: true}
	e2 := bootEngine(t, o, 0)
	e2.writeStartImages([]panel.Panel{o}, true)
	if o.count() != 0 {
		t.Fatal("written to a model without boot picture")
	}
}

func TestForcedRewriteSkipsUnchangedBootPicture(t *testing.T) {
	b := &bootBar{}
	e := bootEngine(t, b, 0)
	ps := []panel.Panel{b}
	e.writeStartImages(ps, false)
	e.writeStartImages(ps, true) // user rewrite, record equals the hash
	if b.count() != 1 {
		t.Fatalf("writes %d, want 1: forced rewrite must not repeat an unchanged boot picture", b.count())
	}
	// Changed picture: a forced rewrite writes it.
	e.mu.Lock()
	c := e.cfg.Panels[b.ID()]
	c.Rotation = 90
	e.cfg.Panels[b.ID()] = c
	e.mu.Unlock()
	e.writeStartImages(ps, true)
	if b.count() != 2 {
		t.Fatalf("writes %d, want 2 after the picture changed", b.count())
	}
	// A pending marker is rewritten on force (explicit user recovery).
	e.mu.Lock()
	e.cfg.BootImages["us-us88-0"] = bootPendingPrefix + "x"
	e.mu.Unlock()
	e.writeStartImages(ps, true)
	if b.count() != 3 {
		t.Fatalf("writes %d, want 3 after pending marker", b.count())
	}
}
