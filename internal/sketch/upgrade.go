package sketch

import (
	"fmt"
	"math"
)

// UpgradeNote reports something Upgrade could not carry over exactly.
type UpgradeNote struct {
	// ID is the placement the note is about.
	ID  string `json:"id"`
	Msg string `json:"msg"`
}

// legacyRule maps a version 1 (view, face) pair to the facing a part gets in
// the model and how its rotation and mirror change so that the part looks the
// same in the view it was drawn in.
//
// The version 1 views were drawn as: front u = x, v = y; side u = z (from the
// rear, which is where the library outlines start), v = y; top u = x, v =
// depth from the front, front at the top. Version 2 draws the top view with
// the front at the bottom (ToView), so top-view parts are flipped vertically.
// A flat part flipped vertically is the part mirrored and turned by 180
// degrees (spin = 180 - rot). An edge-on part keeps one coordinate of its LEDs
// along the drawn axis, which fixes the spin up to a multiple of 90 degrees:
//
//	front/front  front  rot        side/side  side  rot        top/top    top    180-rot, mirror toggled
//	front/side   tray   rot        side/front rear  rot        top/front  front  rot
//	front/top    top    rot        side/top   top   rot+90     top/side   tray   rot-90
type legacyRule struct {
	facing string
	spin   func(rot float64) float64
	toggle bool
}

func legacyRuleOf(view, face string) (legacyRule, bool) {
	same := func(r float64) float64 { return r }
	switch view + "/" + face {
	case "front/front":
		return legacyRule{FacingFront, same, false}, true
	case "front/side":
		return legacyRule{FacingTray, same, false}, true
	case "front/top":
		return legacyRule{FacingTop, same, false}, true
	case "side/side":
		return legacyRule{FacingSide, same, false}, true
	case "side/front":
		return legacyRule{FacingRear, same, false}, true
	case "side/top":
		return legacyRule{FacingTop, func(r float64) float64 { return r + 90 }, false}, true
	case "top/top":
		return legacyRule{FacingTop, func(r float64) float64 { return 180 - r }, true}, true
	case "top/front":
		return legacyRule{FacingFront, same, false}, true
	case "top/side":
		return legacyRule{FacingTray, func(r float64) float64 { return r - 90 }, false}, true
	}
	return legacyRule{}, false
}

// normSpin brings an angle into -360..360 (same turn).
func normSpin(a float64) float64 { return math.Mod(a, 360) + 0 }

// partIsMainboard and partIsOnBoard classify catalog categories for the
// placement of version 1 side-view parts along the depth they did not have.
func partIsMainboard(cat string) bool { return cat == "mainboard" }

func partIsOnBoard(cat string) bool {
	switch cat {
	case "ram", "cpu-block", "pump":
		return true
	}
	return false
}

type upItem struct {
	pl         Placement
	view       string
	cx, cy     float64 // centre of the drawn box (version 1 view coordinates)
	bw, bh     float64 // drawn box size
	hx, hy, hz float64 // half size of the part's box in the model
	cat        string
	pr         Projection // the version 1 picture
	known      bool
}

// Upgrade converts a version 1 sketch to the 3D model.
//
// Every part keeps the picture it had in the view it was drawn in (position,
// rotation, mirror, flow, edge-on or flat), see legacyRule. The depth a flat
// view does not have is chosen by meaning: front view parts sit against the
// front panel, top view parts under the top panel; side view parts sit against
// the glass, the motherboard against the tray, and memory, CPU blocks
// and pumps whose drawn centre is over a mainboard are mounted on it (On)
// just in front of it. The canvas view is kept; a top canvas shows the case
// the other way up in the vertical (front at the bottom).
//
// Upgrade only fails for a sketch that is not version 1, or has more than
// MaxParts parts (checked first: every part costs work here). Parts it cannot map
// exactly are listed in the notes: a part missing from the catalog (placed by
// its drawn corner) and an invalid view/face pair (replaced by the flat face).
// Positions are not clamped: a part far outside the case stays invalid and is
// rejected by Validate. Rotations never need rounding.
// A case template with the side view starting at the front (Orientation) has
// its side picture mirrored (z = D - u), a template with the glass on the right
// its x positions. Parts follow: fitPicture picks the facing, spin and mirror
// that show the part as drawn (side view of a front-left case: as drawn, mirrored
// with the case), so rotation, mirror and LED order stay right.
//
// The returned sketch is not validated.
func Upgrade(old SketchV1, c Catalog) (Sketch, []UpgradeNote, error) {
	if old.Version != LegacyVersion {
		return Sketch{}, nil, fmt.Errorf("cannot upgrade sketch version %d", old.Version)
	}
	if len(old.Parts) > MaxParts {
		return Sketch{}, nil, fmt.Errorf("more than %d parts", MaxParts)
	}
	out := Sketch{Version: Version, Case: old.Case, CanvasView: old.CanvasView}
	if out.Case.Source == "library" && out.Case.Orientation == (Orientation{}) {
		if t, ok := c.Cases[out.Case.ID]; ok {
			out.Case.Orientation = t.Orientation
		}
	}
	cs := out.Case
	dims := cs.Dims
	var notes []UpgradeNote
	note := func(id, format string, a ...any) {
		notes = append(notes, UpgradeNote{ID: id, Msg: fmt.Sprintf(format, a...)})
	}
	items := make([]upItem, 0, len(old.Parts))
	for _, p := range old.Parts {
		view, face := p.ViewOf(), p.FaceOf()
		rule, ok := legacyRuleOf(view, face)
		if !ok {
			note(p.ID, "invalid view %q or face %q, the flat face is used", view, face)
			if view != ViewFront && view != ViewTop {
				view = ViewSide
			}
			face = view
			rule, _ = legacyRuleOf(view, face)
		}
		it := upItem{view: view}
		it.pl = Placement{
			ID: p.ID, Part: p.Part, Facing: rule.facing, Spin: normSpin(rule.spin(p.Rot)), Mirror: p.Mirror != rule.toggle,
			Flow: p.Flow, LEDs: p.LEDs, Len: p.Len, Label: p.Label, Bind: p.Bind,
		}
		it.cx, it.cy = p.X, p.Y
		part, known := c.Parts[p.Part]
		var pr Projection
		if known {
			var err error
			pr, err = projectV1(part, PlacementV1{ID: p.ID, View: view, Face: face, X: p.X, Y: p.Y, Rot: p.Rot,
				Mirror: p.Mirror, Flow: p.Flow, LEDs: p.LEDs, Len: p.Len})
			if err != nil {
				note(p.ID, "part cannot be projected (%v), placed by its drawn corner", err)
				known = false
			}
		} else {
			note(p.ID, "part %q is not in the catalog, placed by its drawn corner", p.Part)
		}
		if known {
			it.pr, it.known = pr, true
			it.bw, it.bh = pr.Box.W, pr.Box.H
			it.cx, it.cy = pr.Box.X+pr.Box.W/2, pr.Box.Y+pr.Box.H/2
			it.cat = part.Category
			p3, err := Project3D(part, it.pl)
			if err != nil {
				return Sketch{}, nil, err
			}
			it.hx, it.hy, it.hz = (p3.Max.X-p3.Min.X)/2, (p3.Max.Y-p3.Min.Y)/2, (p3.Max.Z-p3.Min.Z)/2
		}
		items = append(items, it)
	}
	// Motherboards of the side view, to mount parts on.
	var boards []int
	for i, it := range items {
		if it.view == ViewSide && partIsMainboard(it.cat) {
			boards = append(boards, i)
		}
	}
	for i := range items {
		it := &items[i]
		var pos Vec3
		switch it.view {
		case ViewFront:
			pos, _, _ = FromView(cs, ViewFront, it.cx, it.cy)
			pos.Z = dims.D - it.hz
		case ViewTop:
			pos, _, _ = FromView(cs, ViewTop, it.cx, dims.D-it.cy)
			pos.Y = it.hy
		default:
			pos, _, _ = FromView(cs, ViewSide, it.cx, it.cy)
			if cs.Orientation.FrontOnSideLeft() {
				pos.Z = dims.D - it.cx
			}
			pos.X = it.hx
			switch {
			case partIsMainboard(it.cat):
				pos.X = dims.W - it.hx
			case partIsOnBoard(it.cat):
				for _, b := range boards {
					bd := items[b]
					if b != i && math.Abs(it.cx-bd.cx) <= bd.bw/2 && math.Abs(it.cy-bd.cy) <= bd.bh/2 {
						it.pl.On = bd.pl.ID
						pos.X = dims.W - 2*bd.hx - it.hx
						break
					}
				}
			}
		}
		it.pl.Pos = pos
		if it.known && (cs.Orientation.GlassRight() || cs.Orientation.FrontOnSideLeft()) {
			if !fitPicture(&it.pl, c.Parts[it.pl.Part], cs, it.view, it.pr) {
				note(it.pl.ID, "the part could not be mapped exactly onto the mirrored case, its mirror and rotation may differ")
			}
		}
		out.Parts = append(out.Parts, it.pl)
	}
	return out, notes, nil
}

// fitPicture looks for the facing, spin and mirror of pl whose picture in its
// drawn view equals the version 1 picture want (the side view of a case whose
// template starts at the front is the mirrored picture, the top view is
// flipped vertically). The rule based choice is tried first, so cases that
// need no change keep it. It reports whether a match was found.
func fitPicture(pl *Placement, part Part, cs Case, view string, want Projection) bool {
	d := cs.Dims
	target := make([]Point, len(want.LEDs))
	for i, q := range want.LEDs {
		switch {
		case view == ViewSide && cs.Orientation.FrontOnSideLeft():
			q.X = d.D - q.X
		case view == ViewTop:
			q.Y = d.D - q.Y
		}
		target[i] = q
	}
	wantBox := want.Box
	switch {
	case view == ViewSide && cs.Orientation.FrontOnSideLeft():
		wantBox.X = d.D - wantBox.X - wantBox.W
	case view == ViewTop:
		wantBox.Y = d.D - wantBox.Y - wantBox.H
	}
	const tol = 1e-4
	matches := func(c Placement) bool {
		got, err := ProjectView(part, c, cs, view)
		if err != nil || got.Away || got.EdgeOn != want.EdgeOn || len(got.LEDs) != len(target) {
			return false
		}
		if math.Abs(got.Box.X-wantBox.X) > tol || math.Abs(got.Box.Y-wantBox.Y) > tol ||
			math.Abs(got.Box.W-wantBox.W) > tol || math.Abs(got.Box.H-wantBox.H) > tol {
			return false
		}
		for i, q := range got.LEDs {
			if math.Abs(q.X-target[i].X) > tol || math.Abs(q.Y-target[i].Y) > tol {
				return false
			}
		}
		return true
	}
	facings := []string{pl.Facing}
	switch pl.Facing {
	case FacingFront:
		facings = append(facings, FacingRear)
	case FacingRear:
		facings = append(facings, FacingFront)
	case FacingSide:
		facings = append(facings, FacingTray)
	case FacingTray:
		facings = append(facings, FacingSide)
	}
	for _, f := range facings {
		for _, m := range []bool{pl.Mirror, !pl.Mirror} {
			for _, s := range []float64{pl.Spin, -pl.Spin, 180 - pl.Spin, 180 + pl.Spin} {
				c := *pl
				c.Facing, c.Mirror, c.Spin = f, m, normSpin(s)
				if matches(c) {
					*pl = c
					return true
				}
			}
		}
	}
	return false
}
