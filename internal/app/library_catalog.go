package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kewe87/casehud/internal/sketch"
)

// The case sketch uses the built-in generic parts and cases plus whatever
// the cached library offers. The cache is read from disk only; every file is
// checked again (it may have been edited by hand), a missing or broken file
// only drops that entry, and a cache with only some of the files works.

const libMaxCaseBytes = 512 << 10

// catalogBudget bounds what the sketch catalog takes from the cached library
// in total: the cache is edited by hand or filled by the network, and the
// catalog is held in memory and sent to the GUI. Entries beyond a bound are
// skipped, not loaded.
type catalogBudget struct {
	parts, cases         int // entries read
	partBytes, caseBytes int // file bytes read
}

// defaultCatalogBudget: 500 parts and 100 cases, 16 MiB of files in total.
var defaultCatalogBudget = catalogBudget{parts: 500, cases: 100, partBytes: 12 << 20, caseBytes: 4 << 20}

// safeCacheFile reports whether an index entry names a plain relative file.
func safeCacheFile(f string) bool {
	return f != "" && len(f) <= 120 && !strings.ContainsAny(f, `\:`) && !strings.Contains(f, "..") && !strings.HasPrefix(f, "/")
}

type cacheIndexEntry struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// readCacheIndex reads components.json or cases.json of the cache; a missing
// or unusable index is no entries.
func readCacheIndex(dir, file, key string) ([]cacheIndexEntry, error) {
	raw, err := readCapped(filepath.Join(dir, file), libMaxIndexBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var version int
	_ = json.Unmarshal(generic["version"], &version)
	if version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d", file, version)
	}
	var es []cacheIndexEntry
	if err := json.Unmarshal(generic[key], &es); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return es, nil
}

// loadLibraryCatalog builds the sketch catalog from the built-in parts and
// the cached library in dir. skipped lists the entries that were refused.
func loadLibraryCatalog(dir string) (cat sketch.Catalog, skipped []error) {
	return loadLibraryCatalogWithin(dir, defaultCatalogBudget)
}

// loadLibraryCatalogWithin is loadLibraryCatalog with explicit bounds.
func loadLibraryCatalogWithin(dir string, budget catalogBudget) (cat sketch.Catalog, skipped []error) {
	cat = sketch.Builtin()
	skip := func(kind, id string, err error) {
		skipped = append(skipped, fmt.Errorf("%s %q: %w", kind, id, err))
	}
	parts, err := readCacheIndex(dir, "components.json", "components")
	if err != nil {
		skipped = append(skipped, err)
	}
	if len(parts) > budget.parts {
		skipped = append(skipped, fmt.Errorf("components.json: %d parts, only the first %d are used", len(parts), budget.parts))
		parts = parts[:budget.parts]
	}
	partBytes := 0
	for _, c := range parts {
		switch {
		case !libIDRe.MatchString(c.ID):
			skip("part", c.ID, errors.New("id not allowed"))
			continue
		case !safeCacheFile(c.File):
			skip("part", c.ID, errors.New("file name not allowed"))
			continue
		}
		data, err := readCapped(filepath.Join(dir, filepath.FromSlash(c.File)), libMaxPartBytes)
		if err != nil {
			skip("part", c.ID, err)
			continue
		}
		if partBytes += len(data); partBytes > budget.partBytes {
			skip("part", c.ID, errors.New("part files exceed the size budget, the rest is not used"))
			break
		}
		if err := checkLibraryPart(c.ID, data); err != nil {
			skip("part", c.ID, err)
			continue
		}
		var probe struct {
			LEDs struct {
				Positions [][2]float64 `json:"positions_mm"`
				Generator *struct{}    `json:"generator"`
			} `json:"leds"`
		}
		if json.Unmarshal(data, &probe) == nil && len(probe.LEDs.Positions) == 0 && probe.LEDs.Generator == nil {
			continue // controllers and sensors: nothing to place
		}
		p, err := sketch.ParsePart(data)
		if err == nil && p.ID != c.ID {
			err = fmt.Errorf("file declares id %q", p.ID)
		}
		if err == nil {
			err = cat.AddPart(p)
		}
		if err != nil {
			skip("part", c.ID, err)
		}
	}
	cases, err := readCacheIndex(dir, "cases.json", "cases")
	if err != nil {
		skipped = append(skipped, err)
	}
	if len(cases) > budget.cases {
		skipped = append(skipped, fmt.Errorf("cases.json: %d cases, only the first %d are used", len(cases), budget.cases))
		cases = cases[:budget.cases]
	}
	caseBytes := 0
	for _, c := range cases {
		switch {
		case !libIDRe.MatchString(c.ID):
			skip("case", c.ID, errors.New("id not allowed"))
			continue
		case !safeCacheFile(c.File):
			skip("case", c.ID, errors.New("file name not allowed"))
			continue
		}
		data, err := readCapped(filepath.Join(dir, filepath.FromSlash(c.File)), libMaxCaseBytes)
		if err != nil {
			skip("case", c.ID, err)
			continue
		}
		if caseBytes += len(data); caseBytes > budget.caseBytes {
			skip("case", c.ID, errors.New("case files exceed the size budget, the rest is not used"))
			break
		}
		t, err := sketch.ParseCase(data)
		if err == nil && t.ID != c.ID {
			err = fmt.Errorf("file declares id %q", t.ID)
		}
		if err == nil {
			err = cat.AddCase(t)
		}
		if err != nil {
			skip("case", c.ID, err)
		}
	}
	return cat, skipped
}

func (e *Engine) invalidateSketchCatalog() {
	s := &e.library
	s.catMu.Lock()
	s.cat = nil
	s.catMu.Unlock()
}

// SketchCatalog returns the parts and cases the sketch may use: the generic
// built-ins and the cached library (no network). The maps are the caller's.
func (e *Engine) SketchCatalog() sketch.Catalog {
	s := &e.library
	s.catMu.Lock()
	defer s.catMu.Unlock()
	if s.cat == nil {
		cat := sketch.Builtin()
		if lib, err := e.Library(); err == nil {
			var skipped []error
			cat, skipped = loadLibraryCatalog(lib.Dir)
			if len(skipped) > 0 {
				log.Printf("library: %d sketch parts or cases refused: %v", len(skipped), errors.Join(skipped...))
			}
		}
		s.cat = &cat
	}
	out := sketch.Catalog{Parts: make(map[string]sketch.Part, len(s.cat.Parts)), Cases: make(map[string]sketch.CaseTemplate, len(s.cat.Cases))}
	for k, v := range s.cat.Parts {
		out.Parts[k] = v
	}
	for k, v := range s.cat.Cases {
		out.Cases[k] = v
	}
	return out
}
