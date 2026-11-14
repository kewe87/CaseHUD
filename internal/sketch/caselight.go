package sketch

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Lighting built into a case (the integrated_lighting block of a library
// case) becomes placements of the sketch that belong to the case: they carry
// the id of their template entry (Placement.CaseLight) and a path through the
// case box (Placement.Path) instead of a position and a facing. They cannot
// be moved or deleted in the editor; they can be bound, ordered, labelled and
// their LED order can be reversed (Placement.Mirror).
//
// A path runs along the walls of the case, possibly over several faces (an
// L-shaped strip along the front and the glass side). Its LEDs are spread
// evenly along the path in wire order; every LED belongs to the face of the
// wall its path segment runs on, so on the case net the strip continues
// across the shared edge of two faces as one strip.

// Case lighting limits.
const (
	maxCaseLights  = 16
	maxCaseLegs    = 8
	MaxPathPoints  = 128
	caseLightIDMax = 52 // "case-" + id stays a placement id
	caseLightPfx   = "case-"
	minStripWidth  = 2.0
	pathJoinTolMM  = 1.0
)

// CaseLight is one light built into a library case, in model coordinates.
type CaseLight struct {
	// ID names the entry within its case (the template id, its zone or
	// "light-<n>").
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Component is the library part of the light, Zone an optional LED zone
	// of that part (the light then has the zone's LEDs only).
	Component string `json:"component"`
	Zone      string `json:"zone,omitempty"`
	// Path is the polyline the LEDs run along, in wire order.
	Path []Vec3 `json:"path"`
}

// libLight is an integrated_lighting entry as it is written in a case file.
// A path is given per view; legs continue it in further views.
type libLight struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Component string       `json:"component"`
	Zone      string       `json:"zone"`
	View      string       `json:"view"`
	Path      [][2]float64 `json:"path_mm"`
	Legs      []struct {
		View string       `json:"view"`
		Path [][2]float64 `json:"path_mm"`
	} `json:"legs"`
}

// caseLights converts the integrated_lighting entries of a case file to
// model coordinates. Entries the sketch cannot place (no path, an unknown
// view, coordinates out of range) are skipped: a case file stays usable
// with lighting the sketch does not understand.
func caseLights(in []libLight, d Dims, o Orientation) []CaseLight {
	var out []CaseLight
	used := map[string]bool{}
	k := Case{Dims: d, Orientation: o}
	for i, e := range in {
		if len(out) >= maxCaseLights {
			break
		}
		if !libraryCaseIDRe.MatchString(e.Component) || (e.Zone != "" && !printable(e.Zone, 64)) ||
			!printable(e.Name, MaxLabel) || len(e.Legs) > maxCaseLegs {
			continue
		}
		type leg struct {
			view string
			pts  [][2]float64
		}
		legs := []leg{{e.View, e.Path}}
		for _, l := range e.Legs {
			legs = append(legs, leg{l.View, l.Path})
		}
		var path []Vec3
		ok := true
		for _, l := range legs {
			if (l.view != ViewFront && l.view != ViewSide) || len(l.pts) < 2 || len(path)+len(l.pts) > MaxPathPoints {
				ok = false
				break
			}
			for _, q := range l.pts {
				if !coordOK(q[0]) || !coordOK(q[1]) {
					ok = false
					break
				}
				var p Vec3
				if l.view == ViewSide {
					u := q[0]
					if o.FrontOnSideLeft() {
						u = d.D - u
					}
					p = Vec3{0, q[1], u}
				} else {
					p, _, _ = FromView(k, ViewFront, q[0], q[1])
					p.Z = d.D
				}
				p = roundVec(p)
				if n := len(path); n > 0 && dist3(path[n-1], p) <= pathJoinTolMM {
					continue // a leg starts where the previous one ends
				}
				path = append(path, p)
			}
		}
		if !ok || len(path) < 2 {
			continue
		}
		id := e.ID
		if id == "" {
			id = e.Zone
		}
		if !libraryCaseIDRe.MatchString(id) {
			id = "light-" + strconv.Itoa(i+1)
		}
		if len(id) > caseLightIDMax {
			id = id[:caseLightIDMax]
		}
		for base, n := id, 2; used[id]; n++ {
			suffix := "-" + strconv.Itoa(n)
			id = base[:min(len(base), caseLightIDMax-len(suffix))] + suffix
		}
		used[id] = true
		out = append(out, CaseLight{ID: id, Name: e.Name, Component: e.Component, Zone: e.Zone, Path: path})
	}
	return out
}

func dist3(a, b Vec3) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
}

// IsCaseLight reports whether a placement belongs to the case.
func (p Placement) IsCaseLight() bool { return p.CaseLight != "" }

// pointWallDist is the distance of a point to the wall on side f.
func pointWallDist(d Dims, p Vec3, f string) float64 {
	switch f {
	case FacingSide:
		return math.Abs(p.X)
	case FacingTray:
		return math.Abs(d.W - p.X)
	case FacingTop:
		return math.Abs(p.Y)
	case FacingBottom:
		return math.Abs(d.H - p.Y)
	case FacingRear:
		return math.Abs(p.Z)
	case FacingFront:
		return math.Abs(d.D - p.Z)
	}
	return math.Inf(1)
}

// wallFaces is the order in which equally near walls are preferred: the
// glass side first, it is the face the case is looked at.
var wallFaces = []string{FacingSide, FacingFront, FacingRear, FacingTop, FacingBottom, FacingTray}

// SegmentFace is the face a path segment runs on: the wall both ends are
// nearest to.
func SegmentFace(d Dims, a, b Vec3) string {
	best, bestD := FacingSide, math.Inf(1)
	for _, f := range wallFaces {
		if v := math.Max(pointWallDist(d, a, f), pointWallDist(d, b, f)); v < bestD-1e-9 {
			best, bestD = f, v
		}
	}
	return best
}

// PathLED is one LED of a path placement.
type PathLED struct {
	Pos  Vec3
	Face string
}

// PathLEDs spreads n LEDs evenly along the path (LED i at (i + 0.5) / n of
// its length), in wire order; reverse runs them from the end of the path.
// Every LED takes the face of its segment.
func PathLEDs(d Dims, path []Vec3, n int, reverse bool) []PathLED {
	if len(path) == 0 || n <= 0 {
		return nil
	}
	segLen := make([]float64, len(path)-1)
	total := 0.0
	for i := range segLen {
		segLen[i] = dist3(path[i], path[i+1])
		total += segLen[i]
	}
	out := make([]PathLED, n)
	seg, start := 0, 0.0
	for i := 0; i < n; i++ {
		if len(segLen) == 0 || total == 0 {
			out[i] = PathLED{path[0], SegmentFace(d, path[0], path[len(path)-1])}
			continue
		}
		t := (float64(i) + 0.5) * total / float64(n)
		for seg < len(segLen)-1 && (segLen[seg] == 0 || t > start+segLen[seg]) {
			start += segLen[seg]
			seg++
		}
		a, b := path[seg], path[seg+1]
		f := 0.0
		if segLen[seg] > 0 {
			f = clamp((t-start)/segLen[seg], 0, 1)
		}
		p := Vec3{a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f, a.Z + (b.Z-a.Z)*f}
		out[i] = PathLED{roundVec(p), SegmentFace(d, a, b)}
	}
	if reverse {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

// PartLEDCount is the LED count of a placement: its own or the part's
// default.
func PartLEDCount(part Part, p Placement) int {
	if p.LEDs > 0 {
		return p.LEDs
	}
	if g := part.LEDs.Generator; g != nil {
		return g.Default
	}
	return len(part.LEDs.Positions)
}

// placementLEDs are the LEDs of a path placement.
func placementLEDs(part Part, p Placement, d Dims) []PathLED {
	return PathLEDs(d, p.Path, PartLEDCount(part, p), p.Mirror)
}

// pathFaces returns every face a path placement has LEDs on, the face of
// its first LED first.
func pathFaces(part Part, p Placement, d Dims) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range placementLEDs(part, p, d) {
		if !seen[l.Face] {
			seen[l.Face] = true
			out = append(out, l.Face)
		}
	}
	if len(out) == 0 {
		out = []string{FacingSide}
	}
	return out
}

// projectPath projects a path placement onto the net: every LED on its own
// face; the box spans the LEDs and the corners of the path, widened by half
// the strip's width.
func projectPath(part Part, pl Placement, c Case, n Net) (Projection, error) {
	ls := placementLEDs(part, pl, c.Dims)
	out := Projection{LEDs: make([]Point, len(ls))}
	minU, minV, maxU, maxV := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(face string, p Vec3) (Point, error) {
		q, ok := n.Point(face, p)
		if !ok {
			return Point{}, fmt.Errorf("placement %q: face %q not in the net", pl.ID, face)
		}
		minU, maxU = math.Min(minU, q.X), math.Max(maxU, q.X)
		minV, maxV = math.Min(minV, q.Y), math.Max(maxV, q.Y)
		return q, nil
	}
	for i, l := range ls {
		q, err := add(l.Face, l.Pos)
		if err != nil {
			return Projection{}, err
		}
		out.LEDs[i] = Point{round6(q.X), round6(q.Y)}
	}
	for i := 0; i+1 < len(pl.Path); i++ {
		f := SegmentFace(c.Dims, pl.Path[i], pl.Path[i+1])
		if _, ok := n.Face(f); !ok {
			continue
		}
		_, _ = add(f, pl.Path[i])
		_, _ = add(f, pl.Path[i+1])
	}
	if math.IsInf(minU, 1) {
		return Projection{}, errors.New("path without LEDs")
	}
	pad := math.Max(part.Dimensions.H, minStripWidth) / 2
	out.Box = Rect{X: round6(minU - pad), Y: round6(minV - pad), W: round6(maxU - minU + 2*pad), H: round6(maxV - minV + 2*pad)}
	return out, nil
}

// validatePath checks a path placement: it belongs to the case, has 2..128
// finite points within the case plus the margin and no length parameter.
func validatePath(p Placement, dims Dims) error {
	if !libraryCaseIDRe.MatchString(p.CaseLight) || len(p.CaseLight) > caseLightIDMax+8 {
		return fmt.Errorf("invalid caseLight %q", p.CaseLight)
	}
	if len(p.Path) < 2 || len(p.Path) > MaxPathPoints {
		return fmt.Errorf("a case light path needs 2..%d points", MaxPathPoints)
	}
	for _, q := range p.Path {
		if !finite(q.X) || !finite(q.Y) || !finite(q.Z) {
			return errors.New("numbers must be finite")
		}
		if q.X < -MarginMM || q.X > dims.W+MarginMM || q.Y < -MarginMM || q.Y > dims.H+MarginMM ||
			q.Z < -MarginMM || q.Z > dims.D+MarginMM {
			return errors.New("path outside the case")
		}
	}
	if p.Len != 0 {
		return errors.New("a case light has no length parameter")
	}
	if p.On != "" || p.Group != "" {
		return errors.New("a case light cannot be mounted or grouped")
	}
	return nil
}

// LightsOf returns the case lights of a template that the catalog can place,
// as placements (id "case-<entry id>", unbound) with their LED count.
func LightsOf(t CaseTemplate, c Catalog) []Placement {
	var out []Placement
	for _, l := range t.Lighting {
		part, ok := c.Parts[l.Component]
		if !ok {
			continue
		}
		n := PartLEDCount(part, Placement{})
		if l.Zone != "" {
			n = 0
			for _, z := range part.LEDs.Zones {
				if z.ID == l.Zone {
					n = z.Count
				}
			}
		}
		if n <= 0 || n > MaxLEDs {
			continue
		}
		face := SegmentFace(t.Dims, l.Path[0], l.Path[1])
		out = append(out, Placement{
			ID: caseLightPfx + l.ID, Part: l.Component, CaseLight: l.ID, Path: append([]Vec3(nil), l.Path...),
			Pos: l.Path[0], Facing: face, LEDs: n,
		})
	}
	return out
}

// SyncCaseLights brings the case lights of a sketch in line with its case
// template: missing ones are added (at the end, unbound), the geometry of
// existing ones is taken from the template (binding, label, LED direction,
// layer and visibility stay), and lights the template no longer has are
// removed. A sketch on a custom box loses its case lights; a sketch whose
// library case is not in the catalog keeps them as they are. If the result
// would not validate (part or LED limits) the sketch is returned unchanged.
func SyncCaseLights(s Sketch, c Catalog) Sketch {
	var want []Placement
	if s.Case.Source == "library" {
		t, ok := c.Cases[s.Case.ID]
		if !ok {
			return s
		}
		want = LightsOf(t, c)
	}
	has := false
	for _, p := range s.Parts {
		has = has || p.IsCaseLight()
	}
	if len(want) == 0 && !has {
		return s
	}
	byEntry := map[string]Placement{}
	ids := map[string]bool{}
	for _, p := range s.Parts {
		if p.IsCaseLight() {
			byEntry[p.CaseLight] = p
		} else {
			ids[p.ID] = true
		}
	}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w.CaseLight] = true
	}
	out := s
	out.Parts = make([]Placement, 0, len(s.Parts)+len(want))
	for _, p := range s.Parts {
		if p.IsCaseLight() {
			continue // re-added below in template order
		}
		if p.On != "" && !ids[p.On] {
			p.On = "" // was mounted on a case light
		}
		out.Parts = append(out.Parts, p)
	}
	for _, w := range want {
		if old, ok := byEntry[w.CaseLight]; ok {
			w.ID, w.Bind, w.Label, w.Mirror, w.Layer, w.Hidden = old.ID, old.Bind, old.Label, old.Mirror, old.Layer, old.Hidden
		} else {
			for base, n := w.ID, 2; ids[w.ID]; n++ {
				w.ID = base + "-" + strconv.Itoa(n)
			}
		}
		if ids[w.ID] || !PlacementIDRe.MatchString(w.ID) {
			continue
		}
		ids[w.ID] = true
		out.Parts = append(out.Parts, w)
	}
	if Validate(out, c) != nil && Validate(s, c) == nil {
		return s
	}
	return out
}
