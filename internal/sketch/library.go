package sketch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Library case limits (the case file schema of the design library).
const (
	maxShapes      = 256
	maxPolyPoints  = 256
	minPolyPoints  = 3
	maxCoordMM     = 2000.0
	maxMounts      = 64
	maxAreas       = 32
	maxMountLayout = 8
	maxSnap        = 8
	maxGPUSlots    = 16
	maxLibText     = 2000
	maxLibName     = 160
)

// CaseView is a drawn view of a library case: its size and outline shapes.
type CaseView struct {
	Name    string  `json:"name,omitempty"`
	Size    ViewDim `json:"size_mm"`
	Outline []Shape `json:"outline"`
}

// ViewDim is a view size in millimetres.
type ViewDim struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Mount is a fan or radiator mounting position of a case.
type Mount struct {
	ID        string        `json:"id"`
	Face      string        `json:"face"`
	View      string        `json:"view"`
	Region    Shape         `json:"region"`
	Layouts   []MountLayout `json:"layouts"`
	Radiators []int         `json:"radiator_mm,omitempty"`
	MaxThick  float64       `json:"max_thickness_mm,omitempty"`
	Slots     bool          `json:"slots,omitempty"`
	Notes     string        `json:"notes,omitempty"`
}

// MountLayout is a fan arrangement of a mount: fan centres in mounting order.
type MountLayout struct {
	FanMM int          `json:"fan_mm"`
	Count int          `json:"count"`
	Snap  [][2]float64 `json:"snap_mm"`
}

// Area is a marked region of a case (motherboard tray, PSU, ...).
type Area struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	View  string `json:"view"`
	Shape Shape  `json:"shape"`
}

// GPUSlot is an optional graphics card position: a region in a view.
type GPUSlot struct {
	ID     string `json:"id"`
	View   string `json:"view"`
	Region Shape  `json:"region"`
}

// ParseCase decodes and checks one library-format case file. Only the parts
// the sketch uses are read; the other fields are ignored. Sizes, counts and
// text are bounded, numbers must be finite.
func ParseCase(data []byte) (CaseTemplate, error) {
	if len(data) > MaxBodyBytes {
		return CaseTemplate{}, errors.New("case file too large")
	}
	var f struct {
		Format      int                 `json:"format"`
		ID          string              `json:"id"`
		Vendor      string              `json:"vendor"`
		Model       string              `json:"model"`
		Dims        PartDims            `json:"dimensions_mm"`
		Views       map[string]CaseView `json:"views"`
		Mounts      []Mount             `json:"mounts"`
		Areas       []Area              `json:"areas"`
		GPUSlots    []GPUSlot           `json:"gpu_slots"`
		Approximate bool                `json:"approximate"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return CaseTemplate{}, err
	}
	if f.Format != 1 {
		return CaseTemplate{}, fmt.Errorf("unsupported format %d", f.Format)
	}
	if !libraryCaseIDRe.MatchString(f.ID) || strings.HasPrefix(f.ID, ReservedPrefix) {
		return CaseTemplate{}, fmt.Errorf("invalid case id %q", f.ID)
	}
	if !dimOK(f.Dims.W) || !dimOK(f.Dims.H) || !dimOK(f.Dims.D) {
		return CaseTemplate{}, errors.New("case dimensions out of range")
	}
	if !printable(f.Vendor, maxLibName) || !printable(f.Model, maxLibName) || f.Model == "" {
		return CaseTemplate{}, errors.New("vendor or model not printable text")
	}
	if len(f.Views) > 2 {
		return CaseTemplate{}, errors.New("too many views")
	}
	for name, v := range f.Views {
		if name != ViewFront && name != ViewSide {
			return CaseTemplate{}, fmt.Errorf("invalid view %q", name)
		}
		if !dimOK(v.Size.W) || !dimOK(v.Size.H) {
			return CaseTemplate{}, fmt.Errorf("view %s: size out of range", name)
		}
		if !printable(v.Name, maxLibName) {
			return CaseTemplate{}, fmt.Errorf("view %s: name not printable", name)
		}
		if err := checkShapes(v.Outline); err != nil {
			return CaseTemplate{}, fmt.Errorf("view %s: %w", name, err)
		}
	}
	if len(f.Mounts) > maxMounts || len(f.Areas) > maxAreas || len(f.GPUSlots) > maxGPUSlots {
		return CaseTemplate{}, errors.New("too many mounts, areas or gpu slots")
	}
	libView := func(v string) bool { return v == ViewFront || v == ViewSide }
	for i, m := range f.Mounts {
		if !libraryCaseIDRe.MatchString(m.ID) || !libView(m.View) {
			return CaseTemplate{}, fmt.Errorf("mount %d: invalid id or view", i)
		}
		switch m.Face {
		case "top", "bottom", "front", "rear", "side", "other":
		default:
			return CaseTemplate{}, fmt.Errorf("mount %d: invalid face %q", i, m.Face)
		}
		if err := checkShape(m.Region); err != nil {
			return CaseTemplate{}, fmt.Errorf("mount %d: %w", i, err)
		}
		if len(m.Layouts) == 0 || len(m.Layouts) > maxMountLayout || len(m.Radiators) > 6 {
			return CaseTemplate{}, fmt.Errorf("mount %d: invalid layouts", i)
		}
		for _, r := range m.Radiators {
			if r <= 0 || r > MaxDimMM {
				return CaseTemplate{}, fmt.Errorf("mount %d: radiator size out of range", i)
			}
		}
		if !finite(m.MaxThick) || m.MaxThick < 0 || m.MaxThick > MaxDimMM || !printable(m.Notes, maxLibText) {
			return CaseTemplate{}, fmt.Errorf("mount %d: invalid thickness or notes", i)
		}
		for _, l := range m.Layouts {
			if l.FanMM <= 0 || l.FanMM > 400 || l.Count < 1 || l.Count > maxSnap || len(l.Snap) > maxSnap {
				return CaseTemplate{}, fmt.Errorf("mount %d: invalid layout", i)
			}
			for _, q := range l.Snap {
				if !coordOK(q[0]) || !coordOK(q[1]) {
					return CaseTemplate{}, fmt.Errorf("mount %d: snap point out of range", i)
				}
			}
		}
	}
	for i, a := range f.Areas {
		switch a.Kind {
		case "motherboard", "psu", "psu-shroud", "drive", "gpu-bay", "cable-area", "chamber":
		default:
			return CaseTemplate{}, fmt.Errorf("area %d: invalid kind %q", i, a.Kind)
		}
		if !libraryCaseIDRe.MatchString(a.ID) || !libView(a.View) || !printable(a.Name, maxLibName) {
			return CaseTemplate{}, fmt.Errorf("area %d: invalid id, view or name", i)
		}
		if err := checkShape(a.Shape); err != nil {
			return CaseTemplate{}, fmt.Errorf("area %d: %w", i, err)
		}
	}
	for i, g := range f.GPUSlots {
		if !libraryCaseIDRe.MatchString(g.ID) || !libView(g.View) {
			return CaseTemplate{}, fmt.Errorf("gpu slot %d: invalid id or view", i)
		}
		if err := checkShape(g.Region); err != nil {
			return CaseTemplate{}, fmt.Errorf("gpu slot %d: %w", i, err)
		}
	}
	name := f.Model
	if f.Vendor != "" {
		name = f.Vendor + " " + f.Model
	}
	return CaseTemplate{
		ID: f.ID, Name: name, Dims: Dims{W: f.Dims.W, H: f.Dims.H, D: f.Dims.D},
		Vendor: f.Vendor, Approximate: f.Approximate, Views: f.Views,
		Mounts: f.Mounts, Areas: f.Areas, GPUSlots: f.GPUSlots,
	}, nil
}

// AddCase adds a library case. It refuses duplicates (the built-in custom box
// cannot be replaced) and the reserved prefix.
func (c *Catalog) AddCase(t CaseTemplate) error {
	if strings.HasPrefix(t.ID, ReservedPrefix) || !libraryCaseIDRe.MatchString(t.ID) {
		return fmt.Errorf("sketch: case id %q not allowed", t.ID)
	}
	if _, dup := c.Cases[t.ID]; dup {
		return fmt.Errorf("sketch: case %q already in the catalog", t.ID)
	}
	if c.Cases == nil {
		c.Cases = map[string]CaseTemplate{}
	}
	c.Cases[t.ID] = t
	return nil
}

func coordOK(v float64) bool { return finite(v) && v >= -maxCoordMM && v <= maxCoordMM }

func checkShapes(ss []Shape) error {
	if len(ss) > maxShapes {
		return fmt.Errorf("more than %d shapes", maxShapes)
	}
	for i, s := range ss {
		if err := checkShape(s); err != nil {
			return fmt.Errorf("shape %d: %w", i, err)
		}
	}
	return nil
}

var shapeRoles = map[string]bool{"": true, "body": true, "frame": true, "glass": true, "display": true,
	"lit": true, "cutout": true, "label": true, "zone": true}

// checkShape validates one shape: known type, finite numbers within +-2000
// mm, positive sizes, polygons of 3..256 points, short printable label.
func checkShape(s Shape) error {
	if !shapeRoles[s.Role] {
		return fmt.Errorf("invalid role %q", s.Role)
	}
	if !printable(s.Label, 80) {
		return errors.New("label not printable text")
	}
	size := func(v float64) bool { return finite(v) && v > 0 && v <= MaxDimMM }
	switch s.Type {
	case "rect":
		if !coordOK(s.X) || !coordOK(s.Y) || !size(s.W) || !size(s.H) || !finite(s.R) || s.R < 0 || s.R > 1000 {
			return errors.New("rect out of range")
		}
	case "circle":
		if !coordOK(s.CX) || !coordOK(s.CY) || !size(s.R) {
			return errors.New("circle out of range")
		}
	case "ring":
		if !coordOK(s.CX) || !coordOK(s.CY) || !size(s.ROuter) || !finite(s.RInner) || s.RInner < 0 || s.RInner > MaxDimMM {
			return errors.New("ring out of range")
		}
	case "polygon":
		if len(s.Points) < minPolyPoints || len(s.Points) > maxPolyPoints {
			return fmt.Errorf("polygon needs %d..%d points", minPolyPoints, maxPolyPoints)
		}
		for _, q := range s.Points {
			if !coordOK(q[0]) || !coordOK(q[1]) {
				return errors.New("polygon point out of range")
			}
		}
	default:
		return fmt.Errorf("unknown shape type %q", s.Type)
	}
	return nil
}
