package sketch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"unicode"
	"unicode/utf8"
)

// PlacementIDRe is the shape of a placement id. It is at most 57 characters
// so that "sketch.<id>" stays within 64.
var PlacementIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,56}$`)

var libraryCaseIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ComponentName is the name of the derived component of a placement.
func ComponentName(placementID string) string { return ReservedPrefix + placementID }

func printable(s string, maxRunes int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxRunes {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func validView(v string) bool { return v == ViewFront || v == ViewSide || v == ViewTop }

func dimOK(v float64) bool { return finite(v) && v > 0 && v <= MaxDimMM }

// Validate checks a sketch against the catalog: version, case, view, at most
// 64 parts with unique ids, known parts, valid enums, finite coordinates
// within the view box plus a margin, LED count and length bounds, printable
// short labels and bindings, at most 16 parts per (device, zone). It never
// panics on any input.
func Validate(s Sketch, c Catalog) error {
	if s.Version != Version {
		return fmt.Errorf("unsupported sketch version %d", s.Version)
	}
	if err := validateCase(s.Case); err != nil {
		return err
	}
	if s.CanvasView != "" && !validView(s.CanvasView) {
		return fmt.Errorf("invalid canvasView %q", s.CanvasView)
	}
	if len(s.Parts) > MaxParts {
		return fmt.Errorf("more than %d parts", MaxParts)
	}
	seen := make(map[string]bool, len(s.Parts))
	total := 0
	chains := map[[2]string]int{}
	for i, p := range s.Parts {
		if err := validatePlacement(p, s.Case.Dims, c); err != nil {
			return fmt.Errorf("part %d: %w", i, err)
		}
		if seen[p.ID] {
			return fmt.Errorf("part %d: duplicate id %q", i, p.ID)
		}
		seen[p.ID] = true
		if b := p.Bind; b != nil {
			key := [2]string{b.Device, b.Zone}
			if chains[key]++; chains[key] > MaxZoneSegments {
				return fmt.Errorf("part %d: more than %d parts in zone %s/%s", i, MaxZoneSegments, b.Device, b.Zone)
			}
		}
		part := c.Parts[p.Part]
		n := p.LEDs
		if n == 0 {
			if g := part.LEDs.Generator; g != nil {
				n = g.Default
			} else {
				n = len(part.LEDs.Positions)
			}
		}
		total += n
		if total > MaxTotalLEDs {
			return fmt.Errorf("more than %d LEDs in total", MaxTotalLEDs)
		}
	}
	return nil
}

func validateCase(k Case) error {
	switch k.Source {
	case "custom":
		if k.ID != "" && k.ID != CustomCaseID {
			return fmt.Errorf("invalid custom case id %q", k.ID)
		}
	case "library":
		if !libraryCaseIDRe.MatchString(k.ID) {
			return fmt.Errorf("invalid case id %q", k.ID)
		}
	default:
		return fmt.Errorf("invalid case source %q", k.Source)
	}
	if !dimOK(k.Dims.W) || !dimOK(k.Dims.H) || !dimOK(k.Dims.D) {
		return errors.New("case dimensions out of range")
	}
	if len(k.Chambers) > MaxChambers {
		return fmt.Errorf("more than %d chambers", MaxChambers)
	}
	for i, ch := range k.Chambers {
		if !finite(ch.X) || !finite(ch.W) || ch.X < 0 || ch.W <= 0 || ch.X+ch.W > k.Dims.W+1e-9 {
			return fmt.Errorf("chamber %d outside the case", i)
		}
	}
	return nil
}

func validatePlacement(p Placement, dims Dims, c Catalog) error {
	if !PlacementIDRe.MatchString(p.ID) {
		return fmt.Errorf("invalid id %q", p.ID)
	}
	part, ok := c.Parts[p.Part]
	if !ok {
		return fmt.Errorf("unknown part %q", p.Part)
	}
	if p.View != "" && !validView(p.View) {
		return fmt.Errorf("invalid view %q", p.View)
	}
	if p.Face != "" && !validView(p.Face) {
		return fmt.Errorf("invalid face %q", p.Face)
	}
	if p.Flow != "" && p.Flow != "intake" && p.Flow != "exhaust" {
		return fmt.Errorf("invalid flow %q", p.Flow)
	}
	if !finite(p.X) || !finite(p.Y) || !finite(p.Rot) || !finite(p.Len) {
		return errors.New("numbers must be finite")
	}
	if p.Rot < -360 || p.Rot > 360 {
		return errors.New("rot outside -360..360")
	}
	vw, vh, _ := ViewSize(dims, p.ViewOf())
	if p.X < -MarginMM || p.X > vw+MarginMM || p.Y < -MarginMM || p.Y > vh+MarginMM {
		return errors.New("position outside the view")
	}
	if _, _, _, ok := faceNormal(p.ViewOf(), p.FaceOf()); !ok {
		return errors.New("invalid view/face combination")
	}
	if p.LEDs < 0 || p.LEDs > MaxLEDs {
		return fmt.Errorf("LED count %d out of range", p.LEDs)
	}
	if _, err := Resolve(part, p.LEDs, p.Len); err != nil {
		return err
	}
	if !printable(p.Label, MaxLabel) {
		return errors.New("label not printable text of at most 40 characters")
	}
	if b := p.Bind; b != nil {
		if b.Device == "" || b.Zone == "" || !printable(b.Device, maxText) || !printable(b.Zone, maxText) {
			return errors.New("binding needs a printable device and zone")
		}
		if b.Order < 0 || b.Order > maxOrder {
			return errors.New("binding order out of range")
		}
	}
	return nil
}

// Usable splits a stored sketch into the placements that are still valid
// against the catalog and the indexes of those that are not (a library part
// was removed, the cache is incomplete, a limit changed). The returned sketch
// validates; if the sketch as a whole cannot be used (case, version, totals)
// every placement is reported as skipped. The input is not changed.
func Usable(s Sketch, c Catalog) (ok Sketch, skipped []int) {
	ok = s
	ok.Parts = nil
	whole := func() (Sketch, []int) {
		all := make([]int, len(s.Parts))
		for i := range all {
			all[i] = i
		}
		out := s
		out.Parts = nil
		return out, all
	}
	if s.Version != Version || validateCase(s.Case) != nil || (s.CanvasView != "" && !validView(s.CanvasView)) {
		return whole()
	}
	seen := map[string]bool{}
	for i, p := range s.Parts {
		if len(ok.Parts) >= MaxParts || seen[p.ID] || validatePlacement(p, s.Case.Dims, c) != nil {
			skipped = append(skipped, i)
			continue
		}
		seen[p.ID] = true
		ok.Parts = append(ok.Parts, p)
	}
	if Validate(ok, c) != nil { // totals and chain lengths
		return whole()
	}
	return ok, skipped
}

// Decode reads a sketch from JSON: at most 1 MiB, unknown fields and
// trailing data are rejected. It does not validate; call Validate.
func Decode(data []byte) (Sketch, error) {
	if len(data) > MaxBodyBytes {
		return Sketch{}, errors.New("sketch too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Sketch
	if err := dec.Decode(&s); err != nil {
		return Sketch{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Sketch{}, errors.New("trailing data after the sketch")
	}
	return s, nil
}
