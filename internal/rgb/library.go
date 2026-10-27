package leds

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LoadLibrary registers the parts of a design library (components.json with
// {version, components:[{id, file}]} and one JSON file per part). Each part
// becomes a Component named by its library ID, with its outer size, LED
// positions in millimetres (wire order) and zones. It returns the number of
// parts loaded; parts that do not parse are reported in the error and skipped.
func LoadLibrary(dir string) (int, error) {
	raw, err := readLimited(filepath.Join(dir, "components.json"))
	if err != nil {
		return 0, err
	}
	var index struct {
		Version    int `json:"version"`
		Components []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		return 0, fmt.Errorf("components.json: %w", err)
	}
	if index.Version != 1 {
		return 0, fmt.Errorf("components.json: unsupported version %d", index.Version)
	}
	n := 0
	var errs []error
	for _, e := range index.Components {
		if strings.Contains(e.File, "..") || filepath.IsAbs(e.File) {
			errs = append(errs, fmt.Errorf("%s: file outside the library", e.ID))
			continue
		}
		if !validID(e.ID) {
			errs = append(errs, fmt.Errorf("%q: invalid id", e.ID))
			continue
		}
		data, err := readLimited(filepath.Join(dir, filepath.FromSlash(e.File)))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		c, err := ParseLibraryComponent(data)
		if errors.Is(err, ErrNoLEDs) {
			continue // controllers, sensors: nothing to place
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.ID, err))
			continue
		}
		if c.Name != e.ID {
			errs = append(errs, fmt.Errorf("%s: file declares id %q", e.ID, c.Name))
			continue
		}
		if err := RegisterComponent(c); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// ErrNoLEDs marks a library part without addressable LEDs (skipped).
var ErrNoLEDs = errors.New("part has no LEDs")

// Limits for library files: they come from a download cache and are not
// trusted.
const (
	maxLibraryFile = 1 << 20 // bytes per file
	maxPartMM      = 2000    // outer size of a part
	maxPartLEDs    = 1024
	maxLabel       = 100 // characters of vendor, model, zone name
	maxID          = 64
)

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLibraryFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLibraryFile {
		return nil, fmt.Errorf("%s: larger than %d bytes", filepath.Base(path), maxLibraryFile)
	}
	return data, nil
}

// validID accepts lower-case letters, digits, '-', '_' and '.', starting
// with a letter or digit.
func validID(s string) bool {
	if s == "" || len(s) > maxID {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case i > 0 && (r == '-' || r == '_' || r == '.'):
		default:
			return false
		}
	}
	return true
}

// validLabel accepts printable text without line breaks.
func validLabel(s string) bool {
	if utf8.RuneCountInString(s) > maxLabel || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func finiteIn(v, lo, hi float64) bool {
	return !math.IsNaN(v) && v >= lo && v <= hi
}

// ParseLibraryComponent converts one library part. Parts without LEDs are
// rejected (they cannot be placed on the canvas).
func ParseLibraryComponent(data []byte) (Component, error) {
	var f struct {
		Format     int    `json:"format"`
		ID         string `json:"id"`
		Vendor     string `json:"vendor"`
		Model      string `json:"model"`
		Category   string `json:"category"`
		Dimensions struct {
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"dimensions_mm"`
		LEDs struct {
			Count     int          `json:"count"`
			Positions [][2]float64 `json:"positions_mm"`
			Zones     []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				First int    `json:"first"`
				Count int    `json:"count"`
			} `json:"zones"`
		} `json:"leds"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return Component{}, err
	}
	if f.Format != 1 {
		return Component{}, fmt.Errorf("unsupported format %d", f.Format)
	}
	if !validID(f.ID) {
		return Component{}, fmt.Errorf("invalid id %q", f.ID)
	}
	if !validLabel(f.Vendor) || !validLabel(f.Model) {
		return Component{}, errors.New("vendor or model not printable text")
	}
	w, h := f.Dimensions.W, f.Dimensions.H
	if !finiteIn(w, 0, maxPartMM) || !finiteIn(h, 0, maxPartMM) || w == 0 || h == 0 {
		return Component{}, errors.New("dimensions missing or out of range")
	}
	if f.LEDs.Count == 0 && len(f.LEDs.Positions) == 0 {
		return Component{}, ErrNoLEDs
	}
	if len(f.LEDs.Positions) == 0 || (f.LEDs.Count > 0 && f.LEDs.Count != len(f.LEDs.Positions)) {
		return Component{}, fmt.Errorf("%d LED positions for %d LEDs", len(f.LEDs.Positions), f.LEDs.Count)
	}
	if len(f.LEDs.Positions) > maxPartLEDs {
		return Component{}, fmt.Errorf("more than %d LEDs", maxPartLEDs)
	}
	for i, p := range f.LEDs.Positions {
		if !finiteIn(p[0], 0, w) || !finiteIn(p[1], 0, h) {
			return Component{}, fmt.Errorf("LED %d outside the part", i)
		}
	}
	c := Component{
		Name: f.ID, Label: strings.TrimSpace(f.Vendor + " " + f.Model),
		SizeMM: Point{X: f.Dimensions.W, Y: f.Dimensions.H}, PositionsMM: true, Outline: outlineFor(f.Category),
	}
	for _, p := range f.LEDs.Positions {
		c.LEDs = append(c.LEDs, Point{X: p[0], Y: p[1]})
	}
	next := 0
	for _, z := range f.LEDs.Zones {
		if !validID(z.ID) || !validLabel(z.Name) {
			return Component{}, fmt.Errorf("zone %q: invalid id or name", z.ID)
		}
		if z.First != next || z.Count <= 0 || z.Count > len(f.LEDs.Positions) {
			return Component{}, fmt.Errorf("zone %q not contiguous", z.ID)
		}
		next += z.Count
		c.Zones = append(c.Zones, ComponentZone{ID: z.ID, Name: z.Name, First: z.First, Count: z.Count})
	}
	if len(c.Zones) > 0 && next != len(c.LEDs) {
		return Component{}, errors.New("zones do not cover all LEDs")
	}
	return c, nil
}

func outlineFor(category string) string {
	switch category {
	case "fan", "fan-lcd":
		return "rounded-square"
	case "strip":
		return "bar"
	case "cpu-block", "pump":
		return "circle"
	}
	return "rect"
}
