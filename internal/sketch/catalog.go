package sketch

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Part is a catalog part in the library component format (docs/components.md
// of the design library). Fields the sketch does not use are ignored.
// Omitted shape numbers are 0.
type Part struct {
	Format     int         `json:"format"`
	ID         string      `json:"id"`
	Vendor     string      `json:"vendor"`
	Model      string      `json:"model"`
	Category   string      `json:"category"`
	Dimensions PartDims    `json:"dimensions_mm"`
	Outline    Outline     `json:"outline"`
	LEDs       PartLEDs    `json:"leds"`
	Notes      string      `json:"notes,omitempty"`
	Connection *Connection `json:"connection,omitempty"`
}

// Connection is the library's connection block.
type Connection struct {
	Type  string `json:"type"`
	Notes string `json:"notes,omitempty"`
}

// PartDims is the outer size: w and h of the front view, depth d.
type PartDims struct {
	W           float64 `json:"w"`
	H           float64 `json:"h"`
	D           float64 `json:"d"`
	Approximate bool    `json:"approximate,omitempty"`
}

// Outline holds the drawn shapes per view of the part itself:
//
//	front  x 0..w, y 0..h (the LED side)
//	side   library profile: x 0..h, y 0..d
//	top    x 0..w, y 0..d (optional, not in the library format)
//
// A missing side or top outline is derived from the dimensions (Shapes).
type Outline struct {
	Front       []Shape `json:"front"`
	Side        []Shape `json:"side,omitempty"`
	Top         []Shape `json:"top,omitempty"`
	Approximate bool    `json:"approximate,omitempty"`
}

// Shape is a library shape primitive: rect, circle, ring or polygon.
type Shape struct {
	Type   string       `json:"type"`
	X      float64      `json:"x,omitempty"`
	Y      float64      `json:"y,omitempty"`
	W      float64      `json:"w,omitempty"`
	H      float64      `json:"h,omitempty"`
	R      float64      `json:"r,omitempty"`
	CX     float64      `json:"cx,omitempty"`
	CY     float64      `json:"cy,omitempty"`
	ROuter float64      `json:"r_outer,omitempty"`
	RInner float64      `json:"r_inner,omitempty"`
	Points [][2]float64 `json:"points,omitempty"`
	Role   string       `json:"role,omitempty"`
	Label  string       `json:"label,omitempty"`
}

// PartLEDs is the library leds block plus an optional generator.
type PartLEDs struct {
	Count       int          `json:"count"`
	Positions   [][2]float64 `json:"positions_mm"`
	Zones       []PartZone   `json:"zones,omitempty"`
	Direction   string       `json:"direction,omitempty"`
	Approximate bool         `json:"approximate,omitempty"`
	// Generator makes the LED count (and length) adjustable: positions are
	// computed instead of read from Positions. Positions then hold the
	// default layout.
	Generator *Generator `json:"generator,omitempty"`
}

// PartZone is a named run of LEDs.
type PartZone struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	First int    `json:"first"`
	Count int    `json:"count"`
}

// Generator describes a parametric LED layout.
//
//	ring  Count LEDs on a circle of Radius around the centre, first at the top, clockwise
//	line  Count LEDs evenly from Inset to the other end along Axis ("x", default, or "y"),
//	      at Offset across (0 = centre)
//	rect  Count LEDs evenly around the rectangle inset by Inset, clockwise from the top left corner
//
// With LengthMax > 0 the part's width is adjustable (LengthMin..LengthMax)
// and the outline becomes a single bar; the default length is the width.
type Generator struct {
	Kind      string  `json:"kind"`
	Axis      string  `json:"axis,omitempty"`
	Min       int     `json:"min"`
	Max       int     `json:"max"`
	Default   int     `json:"default"`
	Radius    float64 `json:"radius,omitempty"`
	Inset     float64 `json:"inset,omitempty"`
	Offset    float64 `json:"offset,omitempty"`
	LengthMin float64 `json:"lengthMin,omitempty"`
	LengthMax float64 `json:"lengthMax,omitempty"`
}

// CaseTemplate is a case the sketch can start from. The built-in custom box
// has only the first four fields; library cases add the outlines (ParseCase).
type CaseTemplate struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Dims     Dims      `json:"dims"`
	Chambers []Chamber `json:"chambers,omitempty"`
	// Library data (all optional).
	Vendor      string              `json:"vendor,omitempty"`
	Approximate bool                `json:"approximate,omitempty"`
	Views       map[string]CaseView `json:"views,omitempty"` // "front", "side"
	Mounts      []Mount             `json:"mounts,omitempty"`
	Areas       []Area              `json:"areas,omitempty"`
	GPUSlots    []GPUSlot           `json:"gpuSlots,omitempty"`
}

// CustomCaseID is the built-in case.
const CustomCaseID = "custom-box"

// Catalog is the set of parts and cases a sketch may use.
type Catalog struct {
	Parts map[string]Part         `json:"parts"`
	Cases map[string]CaseTemplate `json:"cases"`
}

//go:embed generic/*.json
var genericFS embed.FS

var (
	builtinOnce sync.Once
	builtinCat  Catalog
)

// Builtin returns a catalog with the embedded generic parts and the custom
// box case. Every call returns independent maps.
func Builtin() Catalog {
	builtinOnce.Do(func() {
		builtinCat = Catalog{Parts: map[string]Part{}, Cases: map[string]CaseTemplate{
			CustomCaseID: {ID: CustomCaseID, Name: "Custom box", Dims: Dims{W: 230, H: 480, D: 450}},
		}}
		entries, err := genericFS.ReadDir("generic")
		if err != nil {
			panic(err)
		}
		for _, e := range entries {
			data, err := genericFS.ReadFile("generic/" + e.Name())
			if err != nil {
				panic(err)
			}
			p, err := ParsePart(data)
			if err != nil {
				panic(fmt.Sprintf("sketch: embedded part %s: %v", e.Name(), err))
			}
			builtinCat.Parts[p.ID] = p
		}
	})
	out := Catalog{Parts: make(map[string]Part, len(builtinCat.Parts)), Cases: make(map[string]CaseTemplate, len(builtinCat.Cases))}
	for k, v := range builtinCat.Parts {
		out.Parts[k] = v
	}
	for k, v := range builtinCat.Cases {
		out.Cases[k] = v
	}
	return out
}

// AddPart adds a part. It refuses a duplicate id (generic parts cannot be
// replaced) and ids in the reserved "sketch." namespace.
func (c *Catalog) AddPart(p Part) error {
	if strings.HasPrefix(p.ID, ReservedPrefix) {
		return fmt.Errorf("sketch: part id %q uses the reserved prefix", p.ID)
	}
	if err := p.check(); err != nil {
		return err
	}
	if _, dup := c.Parts[p.ID]; dup {
		return fmt.Errorf("sketch: part %q already in the catalog", p.ID)
	}
	if c.Parts == nil {
		c.Parts = map[string]Part{}
	}
	c.Parts[p.ID] = p
	return nil
}

// PartIDs lists the part ids sorted.
func (c Catalog) PartIDs() []string {
	ids := make([]string, 0, len(c.Parts))
	for id := range c.Parts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

var partIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ParsePart decodes and checks one library-format part file.
func ParsePart(data []byte) (Part, error) {
	if len(data) > MaxBodyBytes {
		return Part{}, errors.New("part file too large")
	}
	var p Part
	if err := json.Unmarshal(data, &p); err != nil {
		return Part{}, err
	}
	if err := p.check(); err != nil {
		return Part{}, err
	}
	return p, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (p Part) check() error {
	if p.Format != 1 {
		return fmt.Errorf("unsupported format %d", p.Format)
	}
	if !partIDRe.MatchString(p.ID) {
		return fmt.Errorf("invalid part id %q", p.ID)
	}
	d := p.Dimensions
	for _, v := range []float64{d.W, d.H, d.D} {
		if !finite(v) || v <= 0 || v > MaxDimMM {
			return errors.New("dimensions missing or out of range")
		}
	}
	for name, ss := range map[string][]Shape{"front": p.Outline.Front, "side": p.Outline.Side, "top": p.Outline.Top} {
		if err := checkShapes(ss); err != nil {
			return fmt.Errorf("outline %s: %w", name, err)
		}
	}
	if !printable(p.Vendor, maxLibName) || !printable(p.Model, maxLibName) || !printable(p.Category, 40) || !printable(p.Notes, maxLibText) {
		return errors.New("vendor, model, category or notes not printable text")
	}
	if c := p.Connection; c != nil && (!printable(c.Type, maxLibName) || !printable(c.Notes, maxLibText)) {
		return errors.New("connection not printable text")
	}
	if !printable(p.LEDs.Direction, maxLibName) {
		return errors.New("LED direction not printable text")
	}
	if len(p.LEDs.Zones) > 64 {
		return errors.New("too many LED zones")
	}
	for _, z := range p.LEDs.Zones {
		if !libraryCaseIDRe.MatchString(z.ID) || !printable(z.Name, maxLibName) {
			return errors.New("LED zone id or name not allowed")
		}
	}
	g := p.LEDs.Generator
	if g == nil {
		n := len(p.LEDs.Positions)
		if n == 0 || n > MaxLEDs || (p.LEDs.Count != 0 && p.LEDs.Count != n) {
			return fmt.Errorf("%d LED positions for %d LEDs", n, p.LEDs.Count)
		}
		return checkPositions(p.LEDs.Positions, d.W, d.H)
	}
	switch g.Kind {
	case "ring", "line", "rect":
	default:
		return fmt.Errorf("unknown generator %q", g.Kind)
	}
	if g.Axis != "" && g.Axis != "x" && g.Axis != "y" {
		return errors.New("generator axis must be x or y")
	}
	if g.Min < 1 || g.Max < g.Min || g.Max > MaxLEDs || g.Default < g.Min || g.Default > g.Max {
		return errors.New("generator LED range invalid")
	}
	for _, v := range []float64{g.Radius, g.Inset, g.Offset, g.LengthMin, g.LengthMax} {
		if !finite(v) || v < 0 || v > MaxDimMM {
			return errors.New("generator value out of range")
		}
	}
	if g.LengthMax > 0 && (g.LengthMin <= 0 || g.LengthMax < g.LengthMin || d.W < g.LengthMin || d.W > g.LengthMax) {
		return errors.New("generator length range invalid")
	}
	r, err := p.resolved(g.Default, 0)
	if err != nil {
		return err
	}
	return checkPositions(r.LEDs.Positions, r.Dimensions.W, r.Dimensions.H)
}

func checkPositions(pos [][2]float64, w, h float64) error {
	for i, q := range pos {
		if !finite(q[0]) || !finite(q[1]) || q[0] < 0 || q[0] > w || q[1] < 0 || q[1] > h {
			return fmt.Errorf("LED %d outside the part", i)
		}
	}
	return nil
}

func isFan(p Part) bool { return p.Category == "fan" || p.Category == "fan-lcd" }

// Resolve returns the concrete part for an LED count and length: for a
// parametric part the positions (and for a length parameter the width and
// outline) are computed; other parts are returned as they are. leds 0 and
// length 0 mean the defaults. A count or length on a part that has no such
// parameter is an error.
func Resolve(p Part, leds int, length float64) (Part, error) {
	if p.LEDs.Generator == nil {
		if leds != 0 && leds != len(p.LEDs.Positions) {
			return Part{}, fmt.Errorf("part %s has a fixed LED count of %d", p.ID, len(p.LEDs.Positions))
		}
		if length != 0 {
			return Part{}, fmt.Errorf("part %s has no length parameter", p.ID)
		}
		return p, nil
	}
	return p.resolved(leds, length)
}

func (p Part) resolved(leds int, length float64) (Part, error) {
	g := p.LEDs.Generator
	if leds == 0 {
		leds = g.Default
	}
	if leds < g.Min || leds > g.Max {
		return Part{}, fmt.Errorf("part %s: LED count %d outside %d..%d", p.ID, leds, g.Min, g.Max)
	}
	out := p
	if length != 0 {
		if g.LengthMax == 0 {
			return Part{}, fmt.Errorf("part %s has no length parameter", p.ID)
		}
		if !finite(length) || length < g.LengthMin || length > g.LengthMax {
			return Part{}, fmt.Errorf("part %s: length %g outside %g..%g", p.ID, length, g.LengthMin, g.LengthMax)
		}
		out.Dimensions.W = length
		r := math.Min(out.Dimensions.H/2, 4)
		out.Outline.Front = []Shape{{Type: "rect", W: length, H: out.Dimensions.H, R: r, Role: "body"}}
	}
	w, h := out.Dimensions.W, out.Dimensions.H
	pos := make([][2]float64, 0, leds)
	switch g.Kind {
	case "ring":
		for i := 0; i < leds; i++ {
			a := 2 * math.Pi * float64(i) / float64(leds)
			pos = append(pos, [2]float64{w/2 + g.Radius*math.Sin(a), h/2 - g.Radius*math.Cos(a)})
		}
	case "line":
		for i := 0; i < leds; i++ {
			t := 0.5
			if leds > 1 {
				t = float64(i) / float64(leds-1)
			}
			if g.Axis == "y" {
				x := w / 2
				if g.Offset > 0 {
					x = g.Offset
				}
				pos = append(pos, [2]float64{x, g.Inset + (h-2*g.Inset)*t})
			} else {
				y := h / 2
				if g.Offset > 0 {
					y = g.Offset
				}
				pos = append(pos, [2]float64{g.Inset + (w-2*g.Inset)*t, y})
			}
		}
	case "rect":
		x0, y0, x1, y1 := g.Inset, g.Inset, w-g.Inset, h-g.Inset
		rw, rh := x1-x0, y1-y0
		if rw <= 0 || rh <= 0 {
			return Part{}, fmt.Errorf("part %s: inset larger than the part", p.ID)
		}
		per := 2 * (rw + rh)
		for i := 0; i < leds; i++ {
			d := per * float64(i) / float64(leds)
			switch {
			case d <= rw:
				pos = append(pos, [2]float64{x0 + d, y0})
			case d <= rw+rh:
				pos = append(pos, [2]float64{x1, y0 + d - rw})
			case d <= 2*rw+rh:
				pos = append(pos, [2]float64{x1 - (d - rw - rh), y1})
			default:
				pos = append(pos, [2]float64{x0, y1 - (d - 2*rw - rh)})
			}
		}
	}
	for i := range pos {
		pos[i][0] = round3(clamp(pos[i][0], 0, w))
		pos[i][1] = round3(clamp(pos[i][1], 0, h))
	}
	out.LEDs.Positions = pos
	out.LEDs.Count = len(pos)
	out.LEDs.Zones = nil
	return out, nil
}

func round3(v float64) float64 { return math.Round(v*1000)/1000 + 0 }
func round6(v float64) float64 { return math.Round(v*1e6)/1e6 + 0 }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// Shapes returns the part's outline for a view of the part itself ("front",
// "side", "top"). A missing side or top outline is a rectangle derived from
// the dimensions: side h x d, top w x d.
func (p Part) Shapes(view string) []Shape {
	d := p.Dimensions
	switch view {
	case ViewSide:
		if len(p.Outline.Side) > 0 {
			return p.Outline.Side
		}
		return []Shape{{Type: "rect", W: d.H, H: d.D, Role: "body"}}
	case ViewTop:
		if len(p.Outline.Top) > 0 {
			return p.Outline.Top
		}
		return []Shape{{Type: "rect", W: d.W, H: d.D, Role: "body"}}
	}
	return p.Outline.Front
}
