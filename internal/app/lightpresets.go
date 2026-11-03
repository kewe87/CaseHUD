// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/community"
	leds "github.com/kewe87/casehud/internal/rgb"
)

// Lighting presets come from two places. CaseHUD ships none: a fresh install
// knows nothing about anyone's hardware or taste.
//
//   - library: the cached copy of the community library, downloaded only when
//     the user asks for it;
//   - user: presets the user saved from the current lighting
//     (lighting-presets.json in the data folder, IDs start with "my-").
//
// A preset sets the effect and its parameters only; devices, layouts and
// brightness stay as they are.

const (
	userPresetPrefix = "my-"
	maxUserPresets   = 50
	userPresetFile   = "lighting-presets.json"
)

// Preset sources.
const (
	PresetLibrary = "library"
	PresetUser    = "user"
)

// ErrPresetNotFound is returned for a preset ID that does not exist.
var ErrPresetNotFound = errors.New("preset not found")

// LightingPresetView is one preset as the GUI and API v1 list it.
type LightingPresetView struct {
	community.Preset
	Source string `json:"source"` // library or user
	// Active: the current effect and parameters are exactly those of the preset.
	Active bool `json:"active"`
}

// errUserPresetsCorrupt is wrapped by userPresets when the file is not valid
// JSON. Saving moves such a file aside and starts a new one.
var errUserPresetsCorrupt = errors.New("lighting presets file is corrupt")

// userPresetEntries reads the file as raw entries. A missing file is empty.
func (s *Store) userPresetEntries() ([]json.RawMessage, error) {
	b, err := os.ReadFile(s.path(userPresetFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Presets []json.RawMessage `json:"presets"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", errUserPresetsCorrupt, err)
	}
	return doc.Presets, nil
}

// validUserPreset parses one raw entry with the library checks plus the
// namespace rule.
func validUserPreset(raw json.RawMessage) (community.Preset, bool) {
	p, err := community.ParsePreset(raw)
	if err != nil || !strings.HasPrefix(p.ID, userPresetPrefix) {
		return community.Preset{}, false
	}
	return p, true
}

// userPresets reads the user's presets; entries that do not pass the
// library checks are not listed (but saveUserPresets keeps them in the file).
// A missing file is an empty list.
func (s *Store) userPresets() ([]community.Preset, error) {
	entries, err := s.userPresetEntries()
	if err != nil {
		return nil, err
	}
	out := []community.Preset{}
	for _, raw := range entries {
		if p, ok := validUserPreset(raw); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// saveUserPresets writes ps. Entries of the existing file that userPresets
// does not list (failed validation, e.g. hand-edited or from a newer
// version) are kept as they are, so a save never drops them. A file that is
// not valid JSON is renamed to lighting-presets.json.broken-<timestamp> and
// a new one is started.
func (s *Store) saveUserPresets(ps []community.Preset) error {
	kept, err := s.userPresetEntries()
	if errors.Is(err, errUserPresetsCorrupt) {
		from := s.path(userPresetFile)
		to := from + ".broken-" + time.Now().Format("20060102-150405")
		if rerr := os.Rename(from, to); rerr != nil {
			return fmt.Errorf("%w (could not move it aside: %w)", err, rerr)
		}
		log.Printf("lighting presets: %v; moved to %s, starting a new file", err, to)
		kept = nil
	} else if err != nil {
		return err
	}
	entries := make([]json.RawMessage, 0, len(ps)+len(kept))
	for _, p := range ps {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		entries = append(entries, b)
	}
	for _, raw := range kept {
		if _, ok := validUserPreset(raw); !ok {
			entries = append(entries, raw)
		}
	}
	return writeJSON(s.path(userPresetFile), struct {
		Version int               `json:"version"`
		Presets []json.RawMessage `json:"presets"`
	}{1, entries})
}

// sourcedPreset is a preset with its origin.
type sourcedPreset struct {
	community.Preset
	source string
}

// presetCatalog lists all presets by ID (built in, then library, then user).
// The list is kept until a preset is saved or deleted or the library is
// refreshed (invalidatePresets), so the status checks read no files.
func (e *Engine) presetCatalog() []sourcedPreset {
	e.presetMu.Lock()
	defer e.presetMu.Unlock()
	if e.presetCache == nil {
		cat, cacheable := e.buildPresetCatalog()
		if !cacheable {
			return cat // a transient read error must not hide presets until restart
		}
		e.presetCache = cat
	}
	return e.presetCache
}

// invalidatePresets drops the cached catalog.
func (e *Engine) invalidatePresets() {
	e.presetMu.Lock()
	e.presetCache = nil
	e.presetMu.Unlock()
}

// buildPresetCatalog reads all sources; e.presetMu is held. cacheable is
// false when the user's file could not be read (a corrupt file is stable and
// is cached: saving repairs it).
func (e *Engine) buildPresetCatalog() (out []sourcedPreset, cacheable bool) {
	byID := map[string]sourcedPreset{}
	if lib, err := e.Library(); err == nil {
		for _, p := range lib.Presets() {
			if strings.HasPrefix(p.ID, userPresetPrefix) {
				continue // that namespace belongs to the user
			}
			byID[p.ID] = sourcedPreset{p, PresetLibrary}
		}
	}
	cacheable = true
	user, err := e.Store.userPresets()
	if err != nil {
		log.Println("lighting presets:", err)
		cacheable = errors.Is(err, errUserPresetsCorrupt)
	}
	for _, p := range user {
		byID[p.ID] = sourcedPreset{p, PresetUser}
	}
	out = make([]sourcedPreset, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	// The user's own first, then the rest by ID.
	sort.Slice(out, func(i, j int) bool {
		if (out[i].source == PresetUser) != (out[j].source == PresetUser) {
			return out[i].source == PresetUser
		}
		return out[i].ID < out[j].ID
	})
	return out, cacheable
}

// currentLighting returns the effect and its complete parameters.
func (e *Engine) currentLighting() (string, leds.Params) {
	l := &e.light
	l.mu.Lock()
	defer l.mu.Unlock()
	cfg := l.cfg
	if l.eng != nil {
		cfg = l.eng.Config()
	}
	info, ok := lightingEffect(cfg.Effect)
	if !ok {
		return cfg.Effect, nil
	}
	return cfg.Effect, paramsWithDefaults(info.Params, cfg.Params[cfg.Effect])
}

// presetParams resolves a preset against its effect: every parameter of the
// effect, the preset's value where it has one, else the default. Names the
// effect does not know are returned as dropped.
func presetParams(p community.Preset) (leds.Params, []string, bool) {
	info, ok := lightingEffect(p.Effect)
	if !ok {
		return nil, nil, false
	}
	params := leds.Params{}
	known := map[string]bool{}
	for _, d := range info.Params {
		params[d.Key] = d.Default
		known[d.Key] = true
	}
	var dropped []string
	for k, v := range p.Params {
		if !known[k] {
			dropped = append(dropped, k)
			continue
		}
		params[k] = v
	}
	sort.Strings(dropped)
	return params, dropped, true
}

// sameParam compares two parameter values as they come from JSON (numbers
// as float64, colors in any case).
func sameParam(a, b any) bool {
	if as, ok := a.(string); ok {
		bs, ok := b.(string)
		return ok && strings.EqualFold(as, bs)
	}
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok || bok {
		return aok && bok && af == bf
	}
	return a == b
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

func presetMatches(p community.Preset, effect string, cur leds.Params) bool {
	if p.Effect != effect || cur == nil {
		return false
	}
	want, _, ok := presetParams(p)
	if !ok {
		return false
	}
	for k, v := range want {
		if !sameParam(v, cur[k]) {
			return false
		}
	}
	return true
}

// LightingPresetList lists the presets this build can run (the effects the
// GUI offers), with the one matching the current lighting marked active.
func (e *Engine) LightingPresetList() []LightingPresetView {
	effect, cur := e.currentLighting()
	out := []LightingPresetView{}
	for _, p := range e.presetCatalog() {
		if _, ok := lightingEffect(p.Effect); !ok || !lightingIsFeatured(p.Effect) {
			continue
		}
		out = append(out, LightingPresetView{Preset: p.Preset, Source: p.source, Active: presetMatches(p.Preset, effect, cur)})
	}
	return out
}

// ActiveLightingPreset is the ID of the preset matching the current lighting
// ("" when none does; the user's own win over the others).
func (e *Engine) ActiveLightingPreset() string {
	for _, p := range e.LightingPresetList() {
		if p.Active {
			return p.ID
		}
	}
	return ""
}

func (e *Engine) presetByID(id string) (sourcedPreset, bool) {
	for _, p := range e.presetCatalog() {
		if p.ID == id {
			return p, true
		}
	}
	return sourcedPreset{}, false
}

// ApplyLightingPreset sets the effect and its parameters from a preset.
// Parameters the preset does not name get their defaults; names the effect
// does not know are dropped (and returned). The result goes through the same
// validation as every other change.
func (e *Engine) ApplyLightingPreset(id string) (LightingView, []string, error) {
	p, ok := e.presetByID(id)
	if !ok {
		return LightingView{}, nil, fmt.Errorf("%w: preset %q", ErrPresetNotFound, id)
	}
	params, dropped, ok := presetParams(p.Preset)
	if !ok {
		return LightingView{}, nil, lightErr("preset %q needs the effect %q, which this version does not have", id, p.Effect)
	}
	if len(dropped) > 0 {
		log.Printf("lighting preset %s: parameters not supported by %s: %v", id, p.Effect, dropped)
	}
	effect := p.Effect
	v, err := e.PatchLighting(LightingPatch{Effect: &effect, Params: params})
	if err != nil {
		return LightingView{}, dropped, fmt.Errorf("preset %q: %w", id, err)
	}
	return v, dropped, nil
}

// presetSlug makes the ID part of a user preset from its name.
func presetSlug(name string) string {
	r := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")
	s := strings.ToLower(r.Replace(name))
	var b strings.Builder
	dash := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 36 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "preset"
	}
	return out
}

// SaveLightingPreset stores the current effect and parameters as a preset of
// the user's under name (1-40 printable characters). Saving under the name of
// one of the user's presets replaces it.
func (e *Engine) SaveLightingPreset(name string) (LightingPresetView, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 || !printable(name, 160) {
		return LightingPresetView{}, lightErr("name must be 1-40 printable characters")
	}
	effect, cur := e.currentLighting()
	info, ok := lightingEffect(effect)
	if !ok || cur == nil {
		return LightingPresetView{}, lightErr("the current effect %q cannot be saved", effect)
	}
	p := community.Preset{Format: 1, Name: map[string]string{"de": name, "en": name}, Effect: effect,
		Params: map[string]any{}, Tags: []string{}, Author: "me"}
	for _, d := range info.Params {
		v := cur[d.Key]
		p.Params[d.Key] = v
		if s, ok := v.(string); ok && d.Type == leds.ParamColor && len(p.Palette) < 6 {
			p.Palette = append(p.Palette, strings.ToLower(s))
		}
	}
	e.presetMu.Lock()
	defer e.presetMu.Unlock()
	list, err := e.Store.userPresets()
	if errors.Is(err, errUserPresetsCorrupt) {
		list, err = nil, nil // saveUserPresets moves the broken file aside
	}
	if err != nil {
		return LightingPresetView{}, err
	}
	base := userPresetPrefix + presetSlug(name)
	p.ID = base
	replace := -1
	for i, q := range list {
		if strings.EqualFold(q.Name["de"], name) || strings.EqualFold(q.Name["en"], name) {
			replace, p.ID = i, q.ID
			break
		}
	}
	if replace < 0 {
		if len(list) >= maxUserPresets {
			return LightingPresetView{}, lightErr("at most %d own presets", maxUserPresets)
		}
		taken := map[string]bool{}
		for _, q := range list {
			taken[q.ID] = true
		}
		for n := 2; taken[p.ID]; n++ {
			p.ID = base + "-" + strconv.Itoa(n)
		}
	}
	// The stored form must pass the same checks as a library file.
	b, err := json.Marshal(p)
	if err != nil {
		return LightingPresetView{}, err
	}
	if p, err = community.ParsePreset(b); err != nil {
		return LightingPresetView{}, lightErr("preset: %v", err)
	}
	if replace >= 0 {
		list[replace] = p
	} else {
		list = append(list, p)
	}
	e.presetCache = nil
	if err := e.Store.saveUserPresets(list); err != nil {
		return LightingPresetView{}, err
	}
	return LightingPresetView{Preset: p, Source: PresetUser, Active: true}, nil
}

// DeleteLightingPreset removes one of the user's presets.
func (e *Engine) DeleteLightingPreset(id string) error {
	if !strings.HasPrefix(id, userPresetPrefix) {
		return fmt.Errorf("%w: only your own presets can be deleted", ErrLighting)
	}
	e.presetMu.Lock()
	defer e.presetMu.Unlock()
	list, err := e.Store.userPresets()
	if err != nil {
		return err
	}
	for i, p := range list {
		if p.ID == id {
			e.presetCache = nil
			return e.Store.saveUserPresets(append(list[:i:i], list[i+1:]...))
		}
	}
	return fmt.Errorf("%w: preset %q", ErrPresetNotFound, id)
}
