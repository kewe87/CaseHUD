// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresetReadErrorIsNotCached(t *testing.T) {
	e := startDemo(t)
	// A directory in place of the file: reading fails, not "missing".
	f := e.Store.path(userPresetFile)
	if err := os.Mkdir(f, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := e.LightingPresetList(); len(got) != 0 {
		t.Fatalf("listed %v", got)
	}
	if e.presetCache != nil {
		t.Fatal("catalog cached despite the read error")
	}
	os.Remove(f)
	doc := `{"version":1,"presets":[{"format":1,"id":"my-ok","name":{"de":"Ok","en":"Ok"},"effect":"solid","params":{"color":"#ff0000"}}]}`
	if err := os.WriteFile(f, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := e.LightingPresetList(); len(got) != 1 || got[0].ID != "my-ok" {
		t.Fatalf("after the file became readable: %v", got)
	}
}

func TestInvalidPresetEntriesSurviveSave(t *testing.T) {
	e := startDemo(t)
	bad := `{"format":1,"id":"my-bad","name":{"de":"Bad","en":"Bad"},"effect":"solid","params":{"color":{"x":1}}}`
	other := `{"format":1,"id":"neon-wave","name":{"de":"x","en":"x"},"effect":"solid","params":{}}`
	doc := `{"version":1,"presets":[` + bad + `,` + other + `]}`
	if err := os.WriteFile(e.Store.path(userPresetFile), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveLightingPreset("Neu"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(e.Store.path(userPresetFile))
	if !strings.Contains(string(raw), `"my-bad"`) || !strings.Contains(string(raw), `"neon-wave"`) || !strings.Contains(string(raw), `"my-neu"`) {
		t.Fatalf("file after save: %s", raw)
	}
	if err := e.DeleteLightingPreset("my-neu"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(e.Store.path(userPresetFile))
	if !strings.Contains(string(raw), `"my-bad"`) || strings.Contains(string(raw), `"my-neu"`) {
		t.Fatalf("file after delete: %s", raw)
	}
}

func TestCorruptPresetFileMovedAsideOnSave(t *testing.T) {
	e := startDemo(t)
	if err := os.WriteFile(e.Store.path(userPresetFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := e.SaveLightingPreset("Frisch")
	if err != nil {
		t.Fatalf("save on a corrupt file: %v", err)
	}
	m, _ := filepath.Glob(e.Store.path(userPresetFile) + ".broken-*")
	if len(m) != 1 {
		t.Fatalf("broken copies: %v", m)
	}
	if b, _ := os.ReadFile(m[0]); string(b) != "{not json" {
		t.Fatalf("moved file changed: %q", b)
	}
	if _, ok := e.presetByID(p.ID); !ok {
		t.Fatal("new preset not listed")
	}
}
