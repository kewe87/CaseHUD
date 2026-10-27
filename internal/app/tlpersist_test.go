// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

// startFan is a TL LCD stand-in that records start image writes.
type startFan struct {
	mu     sync.Mutex
	key    string
	writes int
	last   []byte
	err    error
}

func (f *startFan) ID() string              { return "fan-" + f.key }
func (f *startFan) Name() string            { return "fan " + f.key }
func (f *startFan) Size() image.Point       { return image.Pt(400, 400) }
func (f *startFan) ShowJPEG([]byte) error   { return nil }
func (f *startFan) MaxJPEG() int            { return 65535 }
func (f *startFan) SetBrightness(int) error { return nil }
func (f *startFan) SetRotation(int) error   { return nil }
func (f *startFan) Close() error            { return nil }
func (f *startFan) SettingsKey() string     { return f.key }
func (f *startFan) count() int              { f.mu.Lock(); defer f.mu.Unlock(); return f.writes }
func (f *startFan) WriteStartImage(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	f.last = b
	return f.err
}

func startEngine(t *testing.T, accepted bool) *Engine {
	t.Helper()
	countDiscovery(t)
	dir := t.TempDir()
	cfg := `{"panels":{}}`
	if accepted {
		cfg = `{"disclaimer":{"version":2},"tl_serial_migrated":true,"panels":{}}`
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e
}

func TestStartImageIsSmallAndStable(t *testing.T) {
	a, ha, err := StartImage()
	if err != nil {
		t.Fatal(err)
	}
	b, hb, _ := StartImage()
	if len(a) == 0 || len(a) > 65535 || ha != hb || len(b) != len(a) {
		t.Fatalf("size %d, hashes %q %q", len(a), ha, hb)
	}
}

func TestStartImageWrittenOncePerFanAndImage(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "tllcd-sAAA"}
	ps := []panel.Panel{f}
	e.writeStartImages(ps, false)
	if f.count() != 1 {
		t.Fatalf("writes = %d, want 1", f.count())
	}
	e.writeStartImages(ps, false) // same app run
	// A new run (nothing tried yet) with the hash on record: still no write.
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if f.count() != 1 {
		t.Fatalf("writes after repeat = %d, want 1", f.count())
	}
	// Changed image (record differs): written once more.
	e.cfg.TLStartImages[f.key] = "old"
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if f.count() != 2 {
		t.Fatalf("writes after changed image = %d, want 2", f.count())
	}
	// A new fan appears (hot-plug): only that one is written.
	g := &startFan{key: "tllcd-sBBB"}
	e.writeStartImages([]panel.Panel{f, g}, false)
	if f.count() != 2 || g.count() != 1 {
		t.Fatalf("f=%d g=%d", f.count(), g.count())
	}
	// The record survives in the config file.
	b, _ := os.ReadFile(filepath.Join(e.Store.Dir, "config.json"))
	if !contains(string(b), "tl_start_images") || !contains(string(b), "tllcd-sBBB") {
		t.Fatalf("record not persisted: %s", b)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestStartImageFailureIsNotRetriedInTheSameRun(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "k", err: errors.New("no ack")}
	e.writeStartImages([]panel.Panel{f}, false)
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 1 {
		t.Fatalf("writes = %d, want 1", f.count())
	}
	if len(e.cfg.TLStartImages) != 0 {
		t.Fatal("a failed write was recorded")
	}
	// A panel closed by a rescan before anything was sent does not use up the attempt.
	g := &startFan{key: "g", err: panel.ErrClosed}
	e.writeStartImages([]panel.Panel{g}, false)
	g.err = nil
	e.writeStartImages([]panel.Panel{g}, false)
	if g.count() != 2 {
		t.Fatalf("writes = %d, want 2", g.count())
	}
}

func TestStartImageSettingOffAndDisclaimer(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "k"}
	if err := e.SetTLStartImage(false); err != nil {
		t.Fatal(err)
	}
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 0 || e.TLStartImage() {
		t.Fatalf("written with the setting off (%d)", f.count())
	}
	if err := e.SetTLStartImage(true); err != nil { // nothing connected: no write, no error
		t.Fatal(err)
	}

	// Disclaimer not accepted: nothing is written, not even when forced.
	e2 := startEngine(t, false)
	h := &startFan{key: "h"}
	e2.writeStartImages([]panel.Panel{h}, false)
	e2.writeStartImages([]panel.Panel{h}, true)
	if h.count() != 0 {
		t.Fatalf("written before the disclaimer (%d)", h.count())
	}
	if _, err := e2.RewriteStartImages(true); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("rewrite before disclaimer: %v", err)
	}
}

func TestStartImageForcedRewrite(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "k"}
	e.writeStartImages([]panel.Panel{f}, false)
	e.writeStartImages([]panel.Panel{f}, true)
	if f.count() != 2 {
		t.Fatalf("writes = %d, want 2", f.count())
	}
}

// After a scan the write happens by itself, in the background.
func TestStartImageAfterRescan(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "scan"}
	old := discoverPanels
	discoverPanels = func() ([]panel.Panel, error) { return []panel.Panel{f}, nil }
	t.Cleanup(func() { discoverPanels = old })
	e.Rescan()
	deadline := time.Now().Add(5 * time.Second)
	for f.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.count() != 1 {
		t.Fatalf("writes = %d, want 1", f.count())
	}
}

func TestRewriteNeedsConfirmation(t *testing.T) {
	e := startEngine(t, true)
	if _, err := e.RewriteStartImages(false); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("rewrite without confirmation: %v", err)
	}
	if _, err := e.RewriteStartImages(true); err != nil {
		t.Fatal(err)
	}
}

func TestSerialAssignSwitch(t *testing.T) {
	e := startEngine(t, true)
	t.Cleanup(func() { panel.SetOptions(panel.Options{}) })
	if e.TLSerialAssign() {
		t.Fatal("default must be off")
	}
	if err := e.SetTLSerialAssign(true); err != nil || !e.TLSerialAssign() || panelOptions(e.cfg).NoSerial {
		t.Fatal("switch on not applied")
	}
	if err := e.SetTLSerialAssign(false); err != nil {
		t.Fatal(err)
	}
	if e.TLSerialAssign() || !panelOptions(e.cfg).NoSerial {
		t.Fatal("switch not applied")
	}
}

func TestTLSerialMigration(t *testing.T) {
	dir := t.TempDir()
	st := &Store{Dir: dir}
	c, _ := st.LoadConfig() // new installation: off
	if c.TLSerialOn {
		t.Fatal("new config must start with serial assignment off")
	}
	for _, tc := range []struct {
		name, js string
		want     bool
	}{
		{"accepted, never switched off", `{"disclaimer":{"version":2},"panels":{}}`, true},
		{"accepted, switched off", `{"disclaimer":{"version":2},"tl_serial_off":true,"panels":{}}`, false},
		{"not accepted", `{"panels":{}}`, false},
		{"already migrated, off", `{"disclaimer":{"version":2},"tl_serial_migrated":true,"panels":{}}`, false},
	} {
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(tc.js), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := st.LoadConfig()
		if err != nil || c.TLSerialOn != tc.want {
			t.Fatalf("%s: on=%v err=%v", tc.name, c.TLSerialOn, err)
		}
	}
}

func TestCorruptConfigStartsWithDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{\"panels\": "), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := (&Store{Dir: dir}).LoadConfig()
	if err != nil || c.Panels == nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.bad.json")); err != nil {
		t.Fatal("bad copy missing")
	}
}

func TestStartImageRecordedWhenOnlySettingsAckMissing(t *testing.T) {
	e := startEngine(t, true)
	f := &startFan{key: "s", err: fmt.Errorf("%w: settings: op 64: hid: read timeout", panel.ErrImageWritten)}
	e.writeStartImages([]panel.Panel{f}, false)
	e.mu.Lock()
	rec := e.cfg.TLStartImages["s"]
	e.mu.Unlock()
	if rec == "" {
		t.Fatal("image not recorded as written")
	}
	e.tlp.mu.Lock()
	e.tlp.tried = nil
	e.tlp.mu.Unlock()
	e.writeStartImages([]panel.Panel{f}, false)
	if f.count() != 1 {
		t.Fatalf("written again: %d", f.count())
	}
}

// restFan also keeps a stored picture.
type restFan struct {
	startFan
	rest int
}

func (f *restFan) WriteRestImage([]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rest++
	return f.err
}

func (f *restFan) restCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.rest }

func TestRestImageWrittenOncePerFanAndImage(t *testing.T) {
	e := startEngine(t, true)
	f := &restFan{startFan: startFan{key: "tllcd-sREST"}}
	ps := []panel.Panel{f}
	e.writeStartImages(ps, false)
	if f.count() != 1 || f.restCount() != 1 {
		t.Fatalf("start %d, rest %d, want 1 and 1", f.count(), f.restCount())
	}
	// Restart with the hashes on record: nothing is uploaded again.
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if f.count() != 1 || f.restCount() != 1 {
		t.Fatalf("after restart: start %d, rest %d", f.count(), f.restCount())
	}
	// Only the stored picture is out of date: only that one is written.
	e.cfg.TLRestImages[f.key] = "old"
	e.tlp.tried = nil
	e.writeStartImages(ps, false)
	if f.count() != 1 || f.restCount() != 2 {
		t.Fatalf("changed rest image: start %d, rest %d", f.count(), f.restCount())
	}
	// "Write again" writes both.
	e.writeStartImages(ps, true)
	if f.count() != 2 || f.restCount() != 3 {
		t.Fatalf("forced: start %d, rest %d", f.count(), f.restCount())
	}
	// A failing upload is tried once per run and not recorded.
	g := &restFan{startFan: startFan{key: "tllcd-sFAIL", err: errors.New("no ack")}}
	e.writeStartImages([]panel.Panel{g}, false)
	e.writeStartImages([]panel.Panel{g}, false)
	if g.count() != 1 || g.restCount() != 1 || e.cfg.TLRestImages[g.key] != "" {
		t.Fatalf("failure: start %d, rest %d, record %q", g.count(), g.restCount(), e.cfg.TLRestImages[g.key])
	}
	b, _ := os.ReadFile(filepath.Join(e.Store.Dir, "config.json"))
	if !contains(string(b), "tl_rest_images") {
		t.Fatalf("record not persisted: %s", b)
	}
}
