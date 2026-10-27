package app

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits for cached library parts. The files come from the network once and
// are read from disk on every start, so they are checked here before the LED
// module parses them.
const (
	libMaxIndexBytes = 512 << 10
	libMaxPartBytes  = 256 << 10
	libMaxParts      = 2000
	libMaxLEDs       = 2048
	libMaxZones      = 64
	libMaxSizeMM     = 5000.0
	libMaxText       = 80
)

var libIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

func libPrintable(s string, allowEmpty bool) bool {
	if (s == "" && !allowEmpty) || len(s) > libMaxText || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// checkLibraryPart validates one part file beyond what the LED module checks:
// size, finite and bounded coordinates, printable names, zone ids.
func checkLibraryPart(id string, data []byte) error {
	if len(data) > libMaxPartBytes {
		return fmt.Errorf("file too large (%d bytes)", len(data))
	}
	var f struct {
		Vendor     string `json:"vendor"`
		Model      string `json:"model"`
		Dimensions struct {
			W, H float64
		} `json:"dimensions_mm"`
		LEDs struct {
			Positions [][2]float64 `json:"positions_mm"`
			Zones     []struct {
				ID, Name string
			} `json:"zones"`
		} `json:"leds"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	w, h := f.Dimensions.W, f.Dimensions.H
	if math.IsNaN(w) || math.IsNaN(h) || w <= 0 || h <= 0 || w > libMaxSizeMM || h > libMaxSizeMM {
		return fmt.Errorf("size out of range")
	}
	if len(f.LEDs.Positions) > libMaxLEDs {
		return fmt.Errorf("%d LEDs, at most %d", len(f.LEDs.Positions), libMaxLEDs)
	}
	for _, p := range f.LEDs.Positions {
		for i, v := range p {
			lim := w
			if i == 1 {
				lim = h
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > lim {
				return fmt.Errorf("LED position outside the part")
			}
		}
	}
	if len(f.LEDs.Zones) > libMaxZones {
		return fmt.Errorf("too many zones")
	}
	if !libPrintable(f.Vendor, true) || !libPrintable(f.Model, true) {
		return fmt.Errorf("name is not printable text")
	}
	for _, z := range f.LEDs.Zones {
		if !libIDRe.MatchString(z.ID) || !libPrintable(z.Name, true) {
			return fmt.Errorf("zone id or name not allowed")
		}
	}
	return nil
}

// stageLibrary copies the valid parts of the cached library into a temporary
// folder with its own components.json and returns it. Entries that fail the
// checks are skipped and reported in the error; the caller removes the folder.
func stageLibrary(dir string) (stage string, skipped []error, err error) {
	raw, err := readCapped(filepath.Join(dir, "components.json"), libMaxIndexBytes)
	if err != nil {
		return "", nil, err
	}
	var index struct {
		Version    int `json:"version"`
		Components []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		return "", nil, fmt.Errorf("components.json: %w", err)
	}
	if len(index.Components) > libMaxParts {
		return "", nil, fmt.Errorf("components.json: %d parts, at most %d", len(index.Components), libMaxParts)
	}
	stage, err = os.MkdirTemp("", "casehud-lib-")
	if err != nil {
		return "", nil, err
	}
	type entry struct {
		ID   string `json:"id"`
		File string `json:"file"`
	}
	var kept []entry
	for i, c := range index.Components {
		skip := func(why string) { skipped = append(skipped, fmt.Errorf("%q: %s", c.ID, why)) }
		switch {
		case !libIDRe.MatchString(c.ID):
			skip("id not allowed")
			continue
		case builtinComponentNames[c.ID]:
			skip("id is reserved")
			continue
		case c.File == "" || strings.ContainsAny(c.File, `\:`) || strings.Contains(c.File, "..") || strings.HasPrefix(c.File, "/") || len(c.File) > 120:
			skip("file name not allowed")
			continue
		}
		data, rerr := readCapped(filepath.Join(dir, filepath.FromSlash(c.File)), libMaxPartBytes)
		if rerr != nil {
			skip(rerr.Error())
			continue
		}
		if cerr := checkLibraryPart(c.ID, data); cerr != nil {
			skip(cerr.Error())
			continue
		}
		name := fmt.Sprintf("part-%d.json", i)
		if werr := os.WriteFile(filepath.Join(stage, name), data, 0o600); werr != nil {
			os.RemoveAll(stage)
			return "", nil, werr
		}
		kept = append(kept, entry{c.ID, name})
	}
	idx, _ := json.Marshal(map[string]any{"version": index.Version, "components": kept})
	if err := os.WriteFile(filepath.Join(stage, "components.json"), idx, 0o600); err != nil {
		os.RemoveAll(stage)
		return "", nil, err
	}
	return stage, skipped, nil
}

// readCapped reads a file of at most max bytes.
func readCapped(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if st.Size() > max {
		return nil, fmt.Errorf("file too large (%d bytes)", st.Size())
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("file too large")
	}
	return b, nil
}
