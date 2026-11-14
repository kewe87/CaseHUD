package sketch

import (
	"fmt"
	"math"
	"strings"
)

// The orientation check shows a still pattern on one bound part, so the
// user can compare the part with the preview: a fan gets four quadrants in
// four colours (top, right, bottom, left as the canvas shows the part) and
// a white first LED, a strip runs from green (first LED) to red (last LED).
// If the part shows the pattern turned, mirrored or the other way round,
// Orient corrects its spin, mirror or direction in the sketch.

// Pattern kinds.
const (
	PatternFan   = "fan"   // quadrants, first LED white
	PatternStrip = "strip" // green to red
	PatternRings = "rings" // one colour per ring (zones with one LED per ring)
)

// Quadrant colours (top, right, bottom, left), the first LED and the ends of
// a strip, as "#rrggbb".
var (
	QuadrantColors = [4]string{"#ff0000", "#00c800", "#0050ff", "#ffc800"}
	FirstLEDColor  = "#ffffff"
	StripFirst     = [3]uint8{0, 255, 0}
	StripLast      = [3]uint8{255, 0, 0}
)

// PartPattern is the check pattern of one bound part.
type PartPattern struct {
	Part   string `json:"part"`
	Device string `json:"device"`
	Zone   string `json:"zone"`
	// Start is the zone LED of the part's first LED; Colors are the part's
	// LEDs from there on in wire order.
	Start  int      `json:"start"`
	Colors []string `json:"colors"`
	Kind   string   `json:"kind"`
	// Parallel: the part shows the LEDs of the part before it (Model) and
	// cannot be corrected on its own.
	Parallel bool   `json:"parallel,omitempty"`
	Model    string `json:"model,omitempty"`
}

// OrientError is a check or correction that is not possible; Code says why:
// "unknown", "unbound", "parallel", "fix" (not for this kind of part) or
// "nomatch" (no spin and mirror gives the asked result).
type OrientError struct {
	Code string
	Msg  string
}

func (e *OrientError) Error() string { return e.Msg }

func orientErr(code, format string, a ...any) error {
	return &OrientError{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// Orientation fixes: what the part shows compared with the preview.
const (
	FixRot90   = "rot90"   // turned 90° clockwise
	FixRot180  = "rot180"  // turned 180°
	FixRot270  = "rot270"  // turned 270° clockwise (90° anticlockwise)
	FixMirror  = "mirror"  // mirrored left to right
	FixReverse = "reverse" // strip: starts at the other end
)

// partLEDs returns the placement's LEDs on the net as the engine samples
// them (one point per ring on a ring zone), whether they are ring centres,
// and the centre of the part's box.
func partLEDs(s Sketch, c Catalog, rings RingZones, i int, face string, n Net) ([]Point, bool, Point, error) {
	p := s.Parts[i]
	part := c.Parts[p.Part]
	pr, err := ProjectNet(part, p, s.Case, n, face)
	if err != nil {
		return nil, false, Point{}, err
	}
	pts := pr.LEDs
	ring := false
	if p.Bind != nil && p.Path == nil && rings != nil && rings(p.Bind.Device, p.Bind.Zone) {
		pts, ring = ringCentres(part, p, pts), true
	}
	return pts, ring, Point{X: pr.Box.X + pr.Box.W/2, Y: pr.Box.Y + pr.Box.H/2}, nil
}

// stripLike tells a part that runs in one line: case lighting, strips and
// parts whose LEDs lie on a line.
func stripLike(part Part, p Placement, pts []Point) bool {
	if p.IsCaseLight() || strings.Contains(part.Category, "strip") {
		return true
	}
	if len(pts) < 3 {
		return len(pts) == 2 && !isFan(part)
	}
	// all LEDs on the line through the two LEDs farthest apart
	a, b, best := 0, 0, -1.0
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if d := math.Hypot(pts[i].X-pts[j].X, pts[i].Y-pts[j].Y); d > best {
				a, b, best = i, j, d
			}
		}
	}
	if best <= 0 {
		return false
	}
	for _, q := range pts {
		cross := (pts[b].X-pts[a].X)*(q.Y-pts[a].Y) - (pts[b].Y-pts[a].Y)*(q.X-pts[a].X)
		if math.Abs(cross)/best > 0.5 {
			return false
		}
	}
	return true
}

// Quadrant returns the quadrant of a point around a centre on the canvas
// (y down): 0 top, 1 right, 2 bottom, 3 left.
func Quadrant(c, q Point) int {
	a := math.Atan2(q.X-c.X, c.Y-q.Y) * 180 / math.Pi // 0 = up, clockwise
	a = math.Mod(a+45+360, 360)
	return int(a/90) % 4
}

func hex3(r, g, b uint8) string { return fmt.Sprintf("#%02x%02x%02x", r, g, b) }

// StripColors runs n LEDs from green to red.
func StripColors(n int) []string {
	out := make([]string, n)
	for k := range out {
		t := 0.0
		if n > 1 {
			t = float64(k) / float64(n-1)
		}
		mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t)) }
		out[k] = hex3(mix(StripFirst[0], StripLast[0]), mix(StripFirst[1], StripLast[1]), mix(StripFirst[2], StripLast[2]))
	}
	return out
}

func findPart(s Sketch, id string) int {
	for i, p := range s.Parts {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// chainStart is the zone LED a bound part starts at and, for a part in
// parallel, the part whose LEDs it shows (as Derive counts them).
func chainStart(s Sketch, c Catalog, rings RingZones, i int) (start int, model string) {
	b := s.Parts[i].Bind
	type link struct{ idx, n int }
	var chain []link
	for j, p := range s.Parts {
		if p.Bind != nil && p.Bind.Device == b.Device && p.Bind.Zone == b.Zone {
			n := PartLEDCount(c.Parts[p.Part], p)
			if p.Path == nil && rings != nil && rings(b.Device, b.Zone) {
				if _, r := RingGroups(c.Parts[p.Part], p); r > 0 {
					n = r
				}
			}
			chain = append(chain, link{j, n})
		}
	}
	// stable by order, as Derive
	for a := 1; a < len(chain); a++ {
		for k := a; k > 0 && s.Parts[chain[k].idx].Bind.Order < s.Parts[chain[k-1].idx].Bind.Order; k-- {
			chain[k], chain[k-1] = chain[k-1], chain[k]
		}
	}
	next, prev, owner := 0, 0, ""
	for k, l := range chain {
		st := next
		if s.Parts[l.idx].Bind.Parallel && k > 0 {
			st = prev
		} else {
			owner = s.Parts[l.idx].ID
		}
		if l.idx == i {
			if s.Parts[l.idx].Bind.Parallel && k > 0 {
				return st, owner
			}
			return st, ""
		}
		prev = st
		next = max(next, st+l.n)
	}
	return 0, ""
}

// Pattern returns the check pattern of a bound part of a sketch whose faces
// are resolved (ResolveFaces).
func Pattern(s Sketch, c Catalog, rings RingZones, id string) (PartPattern, error) {
	i := findPart(s, id)
	if i < 0 {
		return PartPattern{}, orientErr("unknown", "no part %q", id)
	}
	p := s.Parts[i]
	if p.Bind == nil {
		return PartPattern{}, orientErr("unbound", "part %q is not connected", id)
	}
	part, ok := c.Parts[p.Part]
	if !ok {
		return PartPattern{}, orientErr("unknown", "part %q: unknown part %q", id, p.Part)
	}
	faces, n := SketchNet(s, c)
	pts, ring, mid, err := partLEDs(s, c, rings, i, faces[i], n)
	if err != nil {
		return PartPattern{}, err
	}
	out := PartPattern{Part: id, Device: p.Bind.Device, Zone: p.Bind.Zone}
	out.Start, out.Model = chainStart(s, c, rings, i)
	out.Parallel = out.Model != ""
	switch {
	case !ring && stripLike(part, p, pts):
		out.Kind = PatternStrip
		out.Colors = StripColors(len(pts))
	default:
		out.Kind = PatternFan
		if ring {
			out.Kind = PatternRings
		}
		out.Colors = make([]string, len(pts))
		for k, q := range pts {
			out.Colors[k] = QuadrantColors[Quadrant(mid, q)]
		}
		if !ring && len(pts) > 1 {
			out.Colors[0] = FirstLEDColor
		}
	}
	return out, nil
}

// Orient corrects the placement of a bound part from what it shows (fix):
// the spin and mirror (or, for case lighting, the direction) are chosen so
// that every LED is drawn where the part really has it. The sketch's faces
// must be resolved; the corrected sketch is returned.
func Orient(s Sketch, c Catalog, rings RingZones, id, fix string) (Sketch, error) {
	i := findPart(s, id)
	if i < 0 {
		return s, orientErr("unknown", "no part %q", id)
	}
	p := s.Parts[i]
	if p.Bind == nil {
		return s, orientErr("unbound", "part %q is not connected", id)
	}
	part, ok := c.Parts[p.Part]
	if !ok {
		return s, orientErr("unknown", "part %q: unknown part %q", id, p.Part)
	}
	if _, model := chainStart(s, c, rings, i); model != "" {
		return s, orientErr("parallel", "part %q shows the LEDs of %q", id, model)
	}
	faces, n := SketchNet(s, c)
	pts, ring, mid, err := partLEDs(s, c, rings, i, faces[i], n)
	if err != nil {
		return s, err
	}
	strip := !ring && stripLike(part, p, pts)
	out := s
	out.Parts = append([]Placement(nil), s.Parts...)
	if p.IsCaseLight() {
		// a case light runs along the case: only its direction can be wrong
		if fix != FixReverse && fix != FixMirror {
			return s, orientErr("fix", "case lighting can only be reversed")
		}
		out.Parts[i].Mirror = !p.Mirror
		return out, nil
	}
	target := make([]Point, len(pts))
	switch fix {
	case FixRot90, FixRot180, FixRot270:
		if strip {
			return s, orientErr("fix", "a strip is not turned, it is reversed")
		}
		th := map[string]float64{FixRot90: 90, FixRot180: 180, FixRot270: 270}[fix] * math.Pi / 180
		cs, sn := math.Cos(th), math.Sin(th)
		for k, q := range pts {
			dx, dy := q.X-mid.X, q.Y-mid.Y // clockwise on the canvas (y down)
			target[k] = Point{X: mid.X + dx*cs - dy*sn, Y: mid.Y + dx*sn + dy*cs}
		}
	case FixMirror:
		if strip {
			return s, orientErr("fix", "a strip is not mirrored, it is reversed")
		}
		for k, q := range pts {
			target[k] = Point{X: 2*mid.X - q.X, Y: q.Y}
		}
	case FixReverse:
		if !strip {
			return s, orientErr("fix", "only a strip is reversed")
		}
		for k := range pts {
			target[k] = pts[len(pts)-1-k]
		}
	default:
		return s, orientErr("fix", "unknown fix %q", fix)
	}
	// the spin and mirror that put every LED on its target
	best, bestErr := Placement{}, math.Inf(1)
	for _, m := range []bool{p.Mirror, !p.Mirror} {
		for d := 0.0; d < 360; d += 90 {
			q := p
			q.Mirror = m
			q.Spin = math.Mod(math.Mod(p.Spin+d, 360)+360, 360)
			out.Parts[i] = q
			got, _, _, err := partLEDs(out, c, rings, i, faces[i], n)
			if err != nil || len(got) != len(target) {
				continue
			}
			e := 0.0
			for k := range got {
				e = math.Max(e, math.Hypot(got[k].X-target[k].X, got[k].Y-target[k].Y))
			}
			if e < bestErr-1e-9 {
				best, bestErr = q, e
			}
		}
	}
	out.Parts[i] = p
	if bestErr > 1 {
		return s, orientErr("nomatch", "part %q: no spin or mirror shows the part %s", id, fix)
	}
	out.Parts[i] = best
	return out, nil
}
