package sketch

// OverlapTolMM is how far two boxes must interpenetrate (on every axis) to
// count as overlapping; touching and rounding errors are ignored.
const OverlapTolMM = 0.5

// Overlap is a pair of placements whose boxes interpenetrate.
type Overlap struct {
	A, B string
}

// exemptCategory parts may overlap anything (LED strips, case accessories).
func exemptCategory(cat string) bool { return cat == "strip" || cat == "case-accessory" }

func boxesOverlap(a, b Projection3D) bool {
	return a.Min.X < b.Max.X-OverlapTolMM && b.Min.X < a.Max.X-OverlapTolMM &&
		a.Min.Y < b.Max.Y-OverlapTolMM && b.Min.Y < a.Max.Y-OverlapTolMM &&
		a.Min.Z < b.Max.Z-OverlapTolMM && b.Min.Z < a.Max.Z-OverlapTolMM
}

// Overlaps reports the pairs of placements whose 3D boxes interpenetrate by
// more than OverlapTolMM on every axis. A part and the part it is mounted on
// (On) never overlap, nor do strips and case accessories. Placements that
// cannot be projected are ignored. Overlaps are only reported: neither the
// server nor Validate rejects them, the editor soft-blocks them.
func Overlaps(s Sketch, c Catalog) []Overlap {
	boxes := make([]Projection3D, len(s.Parts))
	live := make([]bool, len(s.Parts))
	for i, p := range s.Parts {
		part, ok := c.Parts[p.Part]
		if !ok || exemptCategory(part.Category) || len(p.Path) > 0 {
			continue
		}
		if pr, err := Project3D(part, p); err == nil {
			boxes[i], live[i] = pr, true
		}
	}
	var out []Overlap
	for i := range s.Parts {
		for j := i + 1; j < len(s.Parts); j++ {
			if !live[i] || !live[j] || s.Parts[i].On == s.Parts[j].ID || s.Parts[j].On == s.Parts[i].ID {
				continue
			}
			if boxesOverlap(boxes[i], boxes[j]) {
				out = append(out, Overlap{A: s.Parts[i].ID, B: s.Parts[j].ID})
			}
		}
	}
	return out
}

// OverlapsWith returns the ids of the placements of the sketch that pl would
// overlap (same rules as Overlaps). pl need not be part of the sketch; a
// placement with the same id is skipped, so a moved copy can be tested.
func OverlapsWith(s Sketch, c Catalog, pl Placement) []string {
	part, ok := c.Parts[pl.Part]
	if !ok || exemptCategory(part.Category) || len(pl.Path) > 0 {
		return nil
	}
	mine, err := Project3D(part, pl)
	if err != nil {
		return nil
	}
	var out []string
	for _, q := range s.Parts {
		if q.ID == pl.ID || pl.On == q.ID || q.On == pl.ID {
			continue
		}
		other, ok := c.Parts[q.Part]
		if !ok || exemptCategory(other.Category) || len(q.Path) > 0 {
			continue
		}
		if pr, err := Project3D(other, q); err == nil && boxesOverlap(mine, pr) {
			out = append(out, q.ID)
		}
	}
	return out
}
