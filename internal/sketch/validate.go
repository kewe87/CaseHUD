package sketch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"unicode"
	"unicode/utf8"
)

// PlacementIDRe is the shape of a placement id. It is at most 57 characters
// so that "sketch.<id>" stays within 64.
var PlacementIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,56}$`)

var libraryCaseIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

var groupRe = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)

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

func dimOK(v float64) bool { return finite(v) && v > 0 && v <= MaxDimMM }

// Validate checks a sketch against the catalog: version, case, canvas view, at
// most 64 parts with unique ids, known parts, valid enums, finite positions
// within the case box plus a margin, spin, layer, group and mounting parent
// (one level, existing), LED count and length bounds, printable short labels
// and bindings, at most 16 parts per (device, zone). Overlaps are not an
// error (see Overlaps). It never panics on any input.
func Validate(s Sketch, c Catalog) error {
	if s.Version != Version {
		return fmt.Errorf("unsupported sketch version %d", s.Version)
	}
	if err := validateCase(s.Case); err != nil {
		return err
	}
	if !validCanvas(s.CanvasView) {
		return fmt.Errorf("invalid canvasView %q", s.CanvasView)
	}
	if len(s.Parts) > MaxParts {
		return fmt.Errorf("more than %d parts", MaxParts)
	}
	if err := checkMounts(s.Parts); err != nil {
		return err
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
		total += PartLEDCount(c.Parts[p.Part], p)
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
	if err := k.Orientation.validate(); err != nil {
		return err
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
	if !validFacing(p.Facing) {
		return fmt.Errorf("invalid facing %q", p.Facing)
	}
	if !validFace(p.Face) {
		return fmt.Errorf("invalid face %q", p.Face)
	}
	if p.Flow != "" && p.Flow != "intake" && p.Flow != "exhaust" {
		return fmt.Errorf("invalid flow %q", p.Flow)
	}
	if !finite(p.Pos.X) || !finite(p.Pos.Y) || !finite(p.Pos.Z) || !finite(p.Spin) || !finite(p.Len) {
		return errors.New("numbers must be finite")
	}
	if p.Spin < -360 || p.Spin > 360 {
		return errors.New("spin outside -360..360")
	}
	if p.LEDs < 0 || p.LEDs > MaxLEDs {
		return fmt.Errorf("LED count %d out of range", p.LEDs)
	}
	if p.IsCaseLight() || len(p.Path) > 0 {
		// The case defines the LED count of its lighting (a zone of the part).
		if err := validatePath(p, dims); err != nil {
			return err
		}
	} else if err := validateBox(part, p, dims); err != nil {
		return err
	}
	return validateRest(p)
}

func validateBox(part Part, p Placement, dims Dims) error {
	if _, err := Resolve(part, p.LEDs, p.Len); err != nil {
		return err
	}
	// The part's box must reach into the case plus the margin (a long strip
	// may be centred anywhere along its length).
	pr, err := Project3D(part, p)
	if err != nil {
		return err
	}
	if pr.Max.X < -MarginMM || pr.Min.X > dims.W+MarginMM || pr.Max.Y < -MarginMM || pr.Min.Y > dims.H+MarginMM ||
		pr.Max.Z < -MarginMM || pr.Min.Z > dims.D+MarginMM {
		return errors.New("position outside the case")
	}
	return nil
}

func validateRest(p Placement) error {
	if p.Layer < -MaxLayer || p.Layer > MaxLayer {
		return fmt.Errorf("layer outside -%d..%d", MaxLayer, MaxLayer)
	}
	if p.Group != "" && !groupRe.MatchString(p.Group) {
		return fmt.Errorf("invalid group %q", p.Group)
	}
	if p.On != "" && !PlacementIDRe.MatchString(p.On) {
		return fmt.Errorf("invalid on %q", p.On)
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

// checkMounts checks the On references: the parent exists, is not the part
// itself and is not mounted on something else (one level).
func checkMounts(parts []Placement) error {
	byID := make(map[string]Placement, len(parts))
	for _, p := range parts {
		byID[p.ID] = p
	}
	for i, p := range parts {
		if p.On == "" {
			continue
		}
		parent, ok := byID[p.On]
		switch {
		case p.On == p.ID:
			return fmt.Errorf("part %d: mounted on itself", i)
		case !ok:
			return fmt.Errorf("part %d: mounted on unknown part %q", i, p.On)
		case parent.On != "":
			return fmt.Errorf("part %d: mounting parent %q is itself mounted", i, p.On)
		case parent.IsCaseLight():
			return fmt.Errorf("part %d: mounted on a case light", i)
		}
	}
	return nil
}

// Usable splits a stored sketch into the placements that are still valid
// against the catalog and the indexes of those that are not (a library part
// was removed, the cache is incomplete, a limit changed, the part it is
// mounted on is gone). The returned sketch validates; if the sketch as a
// whole cannot be used (case, version, totals) every placement is reported as
// skipped. The input is not changed.
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
	if s.Version != Version || validateCase(s.Case) != nil || !validCanvas(s.CanvasView) {
		return whole()
	}
	seen := map[string]bool{}
	var idx []int // index in s.Parts of every entry of ok.Parts
	for i, p := range s.Parts {
		if len(ok.Parts) >= MaxParts || seen[p.ID] || validatePlacement(p, s.Case.Dims, c) != nil {
			skipped = append(skipped, i)
			continue
		}
		seen[p.ID] = true
		ok.Parts = append(ok.Parts, p)
		idx = append(idx, i)
	}
	// A part mounted on something that is gone (or is itself mounted) goes too.
	for changed := true; changed; {
		changed = false
		byID := make(map[string]Placement, len(ok.Parts))
		for _, p := range ok.Parts {
			byID[p.ID] = p
		}
		var kept []Placement
		var keptIdx []int
		for n, p := range ok.Parts {
			if parent, found := byID[p.On]; p.On != "" && (!found || parent.On != "" || p.On == p.ID) {
				skipped = append(skipped, idx[n])
				changed = true
				continue
			}
			kept = append(kept, p)
			keptIdx = append(keptIdx, idx[n])
		}
		ok.Parts, idx = kept, keptIdx
	}
	sort.Ints(skipped)
	if Validate(ok, c) != nil { // totals and chain lengths
		return whole()
	}
	return ok, skipped
}

// Decode reads a sketch from JSON: at most 1 MiB, unknown fields and
// trailing data are rejected. A version 1 sketch is upgraded (Upgrade) with
// the built-in catalog; use DecodeWithCatalog to upgrade with library parts.
// Decode does not validate; call Validate.
func Decode(data []byte) (Sketch, error) {
	s, _, err := DecodeWithCatalog(data, Builtin())
	return s, err
}

// DecodeWithCatalog is Decode with the catalog a version 1 sketch is upgraded
// against. It also returns the notes of the upgrade (none for version 2).
func DecodeWithCatalog(data []byte, c Catalog) (Sketch, []UpgradeNote, error) {
	if len(data) > MaxBodyBytes {
		return Sketch{}, nil, errors.New("sketch too large")
	}
	if peekVersion(data) == LegacyVersion {
		old, err := DecodeV1(data)
		if err != nil {
			return Sketch{}, nil, err
		}
		return Upgrade(old, c)
	}
	var s plainSketch
	if err := decodeStrict(data, &s); err != nil {
		return Sketch{}, nil, err
	}
	out := Sketch(s)
	out.repairText()
	return out, nil, nil
}

// DecodeV1 reads a version 1 sketch strictly (same rules as Decode).
func DecodeV1(data []byte) (SketchV1, error) {
	if len(data) > MaxBodyBytes {
		return SketchV1{}, errors.New("sketch too large")
	}
	var s SketchV1
	if err := decodeStrict(data, &s); err != nil {
		return SketchV1{}, err
	}
	return s, nil
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after the sketch")
	}
	return nil
}

// plainSketch has the fields of Sketch but not its UnmarshalJSON.
type plainSketch Sketch

// peekVersion returns the "version" field of a sketch document (0 if absent
// or unreadable).
func peekVersion(data []byte) int {
	var v struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(data, &v) != nil {
		return 0
	}
	return v.Version
}

// UnmarshalJSON reads a stored sketch: version 2 as it is, version 1 upgraded
// with the built-in catalog (library parts are then placed approximately; a
// caller that knows the library should keep the raw document and use
// DecodeWithCatalog). Unknown fields are ignored here so that a stored
// document survives later additions.
func (s *Sketch) UnmarshalJSON(data []byte) error {
	if peekVersion(data) == LegacyVersion {
		var old SketchV1
		if err := json.Unmarshal(data, &old); err != nil {
			return err
		}
		up, _, err := Upgrade(old, Builtin())
		if err != nil {
			return err
		}
		*s = up
		return nil
	}
	var p plainSketch
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*s = Sketch(p)
	s.repairText()
	return nil
}
