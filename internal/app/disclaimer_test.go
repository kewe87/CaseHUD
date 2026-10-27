// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

// countDiscovery replaces the device discovery so no real hardware is touched
// and counts the calls.
func countDiscovery(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	op, os_ := discoverPanels, discoverSensors
	discoverPanels = func() ([]panel.Panel, error) { n.Add(1); return nil, nil }
	discoverSensors = func() ([]panel.SensorDevice, error) { n.Add(1); return nil, nil }
	t.Cleanup(func() { discoverPanels, discoverSensors = op, os_ })
	return &n
}

func TestNoDeviceOutputBeforeDisclaimer(t *testing.T) {
	n := countDiscovery(t)
	dir := t.TempDir()
	// lighting and an AWTRIX clock are configured, as after an update
	cfg := `{"lighting":{"enabled":true},"ack_lighting":true,"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if e.DisclaimerAccepted() {
		t.Fatal("a fresh config must not count as accepted")
	}
	e.Rescan()
	e.ManualRescan()
	if n.Load() != 0 {
		t.Fatalf("devices were looked for before the disclaimer was accepted (%d)", n.Load())
	}
	if v := e.Lighting(); v.Active {
		t.Fatal("lighting started before the disclaimer was accepted")
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("lighting enable before acceptance: %v", err)
	}
	if err := e.AcceptDisclaimer(DisclaimerVersion + 1); err == nil {
		t.Fatal("a wrong version must be refused")
	}
	if n.Load() != 0 {
		t.Fatal("a refused acceptance started the output")
	}

	if err := e.AcceptDisclaimer(DisclaimerVersion); err != nil {
		t.Fatal(err)
	}
	if n.Load() == 0 {
		t.Fatal("accepting did not start the device search")
	}
	a := e.Acks()
	if !a.Accepted || a.AcceptedAt == "" || a.Version != DisclaimerVersion {
		t.Fatalf("acks: %+v", a)
	}
	got := n.Load()
	if err := e.AcceptDisclaimer(DisclaimerVersion); err != nil || n.Load() != got {
		t.Fatalf("accepting twice must do nothing (err %v)", err)
	}

	// it is stored, and an older version asks again
	c, err := e.Store.LoadConfig()
	if err != nil || c.Disclaimer.Version != DisclaimerVersion || c.Disclaimer.At == "" {
		t.Fatalf("stored: %+v %v", c.Disclaimer, err)
	}
	c.Disclaimer.Version = DisclaimerVersion - 1
	if err := e.Store.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	e2 := &Engine{Store: e.Store}
	before := n.Load()
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e2.Stop()
	if e2.DisclaimerAccepted() || n.Load() != before {
		t.Fatal("an outdated acceptance must not release the devices")
	}
}

func TestAcceptedStartOpensDevices(t *testing.T) {
	n := countDiscovery(t)
	dir := t.TempDir()
	cfg := `{"disclaimer":{"version":2,"at":"2026-01-01T00:00:00Z"},"panels":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Store: &Store{Dir: dir}}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if n.Load() == 0 {
		t.Fatal("an accepted start must look for devices")
	}
}

func TestOneTimeConfirmations(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	t.Cleanup(func() { panel.SetOptions(panel.Options{}) })
	if err := e.SetDevices(true, nil); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("experimental without confirmation: %v", err)
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("lighting without confirmation: %v", err)
	}
	if err := e.Acknowledge("nonsense"); err == nil {
		t.Fatal("unknown confirmation accepted")
	}
	if err := e.Acknowledge("experimental"); err != nil {
		t.Fatal(err)
	}
	if err := e.Acknowledge("lighting"); err != nil {
		t.Fatal(err)
	}
	if err := e.SetDevices(true, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	c, _ := e.Store.LoadConfig()
	if !c.AckExperimental || !c.AckLighting {
		t.Fatalf("not stored: %+v", c)
	}
}
