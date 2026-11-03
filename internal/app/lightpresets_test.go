// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNoPresetsShipped(t *testing.T) {
	e := startDemo(t)
	if list := e.LightingPresetList(); len(list) != 0 {
		t.Fatalf("a fresh install lists presets: %+v", list)
	}
	if got := e.ActiveLightingPreset(); got != "" {
		t.Fatalf("active = %q", got)
	}
	if _, _, err := e.ApplyLightingPreset("neon-wave"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("unknown preset: %v", err)
	}
}

func TestUserPresetsSaveReplaceDelete(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("wave"), Params: map[string]any{"color": "#123456"}}); err != nil {
		t.Fatal(err)
	}
	p, err := e.SaveLightingPreset("  Mein Blau  ")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "my-mein-blau" || p.Source != PresetUser || p.Effect != "wave" || p.Params["color"] != "#123456" || p.Name["de"] != "Mein Blau" {
		t.Fatalf("saved %+v", p)
	}
	if len(p.Palette) == 0 || p.Palette[0] != "#123456" {
		t.Fatalf("palette %v", p.Palette)
	}
	list := e.LightingPresetList()
	if list[0].ID != p.ID || !list[0].Active {
		t.Fatalf("own preset not first/active: %+v", list[0])
	}
	if got := e.ActiveLightingPreset(); got != p.ID {
		t.Fatalf("active = %q", got)
	}
	if lv := e.Lighting(); lv.Preset != p.ID {
		t.Fatalf("view preset = %q", lv.Preset)
	}
	// Same name: replaced, same ID. Other name with the same slug: new ID.
	if _, err := e.PatchLighting(LightingPatch{Params: map[string]any{"color": "#00ff00"}}); err != nil {
		t.Fatal(err)
	}
	q, err := e.SaveLightingPreset("mein blau")
	if err != nil || q.ID != p.ID || q.Params["color"] != "#00ff00" {
		t.Fatalf("replace: %v %+v", err, q)
	}
	r, err := e.SaveLightingPreset("Mein-Blau!")
	if err != nil || r.ID != "my-mein-blau-2" {
		t.Fatalf("second: %v %+v", err, r)
	}
	// Survives a restart (file in the data folder).
	e2 := &Engine{Store: e.Store, Demo: true}
	n := 0
	for _, x := range e2.LightingPresetList() {
		if x.Source == PresetUser {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d own presets after reload", n)
	}
	if err := e.DeleteLightingPreset(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.presetByID(p.ID); ok {
		t.Fatal("deleted preset still listed")
	}
	if err := e.DeleteLightingPreset("mete"); !errors.Is(err, ErrLighting) {
		t.Fatalf("library preset deleted: %v", err)
	}
	if err := e.DeleteLightingPreset("my-nothing"); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("missing: %v", err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 41), "a\nb"} {
		if _, err := e.SaveLightingPreset(bad); !errors.Is(err, ErrLighting) {
			t.Errorf("name %q: %v", bad, err)
		}
	}
}

func TestUserPresetFileIsChecked(t *testing.T) {
	e := startDemo(t)
	// A hand-edited file: entries outside the user namespace or broken ones are skipped.
	doc := `{"version":1,"presets":[
	 {"format":1,"id":"neon-wave","name":{"de":"x","en":"x"},"effect":"solid","params":{}},
	 {"format":1,"id":"my-ok","name":{"de":"Ok","en":"Ok"},"effect":"solid","params":{"color":"#ff0000"}},
	 {"format":1,"id":"my-bad","name":{"de":"Bad","en":"Bad"},"effect":"solid","params":{"color":{"x":1}}}]}`
	if err := os.WriteFile(e.Store.path(userPresetFile), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	e.invalidatePresets()
	var own []string
	for _, p := range e.LightingPresetList() {
		if p.Source == PresetUser {
			own = append(own, p.ID)
		}
	}
	if len(own) != 1 || own[0] != "my-ok" {
		t.Fatalf("own presets %v", own)
	}
}
