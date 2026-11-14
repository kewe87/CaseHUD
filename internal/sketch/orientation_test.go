package sketch

import (
	"fmt"
	"reflect"
	"testing"
)

// With the glass on the right a part looks the same in the views that show
// x as it does in a glass-left case, placed at the mirrored x (and facing the
// glass where it faced the tray): its rotation, mirror and LED order are not
// reversed by the mirrored view.
func TestGlassRightKeepsThePictureOfAPart(t *testing.T) {
	cat := Builtin()
	left, right := box(), box()
	right.Orientation = Orientation{GlassSide: OrientRight}
	dims := left.Dims
	for _, id := range []string{"generic-fan-120", "generic-ram-dimm", "generic-gpu-box"} {
		part := cat.Parts[id]
		for _, facing := range allFacings {
			for _, spin := range []float64{0, 90, 37, -120} {
				for _, mirror := range []bool{false, true} {
					pl := Placement{ID: "a", Part: id, Facing: facing, Pos: Vec3{60, 200, 150}, Spin: spin, Mirror: mirror, Flow: "exhaust"}
					mir := pl
					mir.Pos.X = dims.W - pl.Pos.X
					switch facing {
					case FacingSide:
						mir.Facing = FacingTray
					case FacingTray:
						mir.Facing = FacingSide
					}
					for _, view := range []string{ViewFront, ViewRear, ViewTop, ViewBottom} {
						want, err := ProjectView(part, pl, left, view)
						if err != nil {
							t.Fatal(err)
						}
						got, err := ProjectView(part, mir, right, view)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("%s %s spin %g mirror %v %s: glass right %+v, want %+v", id, facing, spin, mirror, view, got, want)
						}
					}
				}
			}
		}
	}
}

// The default orientation, spelled out or empty, projects exactly like the
// plain functions: existing sketches do not change.
func TestDefaultOrientationIsUnchanged(t *testing.T) {
	cat := Builtin()
	plain, spelled := box(), box()
	spelled.Orientation = Orientation{SideViewLeft: OrientRear, GlassSide: OrientLeft}
	for _, f := range allFacings {
		a, _ := FrameOf(f)
		b, _ := FrameIn(f, Orientation{})
		c, _ := FrameIn(f, spelled.Orientation)
		if a != b || a != c {
			t.Errorf("%s: frame differs: %v %v %v", f, a, b, c)
		}
	}
	for _, id := range cat.PartIDs() {
		part := cat.Parts[id]
		for _, f := range allFacings {
			pl := Placement{ID: "a", Part: id, Facing: f, Pos: Vec3{50, 90, 120}, Spin: 37, Mirror: true}
			p1, err1 := Project3D(part, pl)
			p2, err2 := Project3DIn(part, pl, spelled.Orientation)
			if (err1 == nil) != (err2 == nil) || !reflect.DeepEqual(p1, p2) {
				t.Fatalf("%s %s: %+v vs %+v", id, f, p1, p2)
			}
			for _, v := range allViews {
				v1, _ := ProjectView(part, pl, plain, v)
				v2, _ := ProjectView(part, pl, spelled, v)
				if !reflect.DeepEqual(v1, v2) {
					t.Fatalf("%s %s %s: %+v vs %+v", id, f, v, v1, v2)
				}
			}
		}
	}
}

// Parts of a version 1 sketch keep their picture on a case whose template is
// mirrored: x views as drawn, the side view of a front-left template as the
// mirror image of the drawing (the case outline is mirrored the same way).
func TestUpgradeKeepsThePictureOnMirroredCases(t *testing.T) {
	cat := Builtin()
	pairs := [][2]string{
		{"front", "front"}, {"front", "side"}, {"front", "top"},
		{"side", "side"}, {"side", "front"}, {"side", "top"},
		{"top", "top"}, {"top", "front"}, {"top", "side"},
	}
	orients := []Orientation{{GlassSide: OrientRight}, {SideViewLeft: OrientFront}, {SideViewLeft: OrientFront, GlassSide: OrientRight}}
	n := 0
	for _, o := range orients {
		cs := box()
		cs.Orientation = o
		for _, id := range []string{"generic-fan-120", "generic-ram-dimm", "generic-gpu-box", "generic-strip", "generic-pump-cyl"} {
			part := cat.Parts[id]
			for _, pair := range pairs {
				for _, rot := range []float64{0, 90, 180, 270, 37, -45} {
					for _, mirror := range []bool{false, true} {
						for _, flow := range []string{"", "exhaust"} {
							old := PlacementV1{ID: "a", Part: id, View: pair[0], Face: pair[1], X: 40.5, Y: 60.25, Rot: rot, Mirror: mirror, Flow: flow}
							want, err := projectV1(part, old)
							if err != nil {
								t.Fatal(err)
							}
							up, notes, err := Upgrade(SketchV1{Version: 1, Case: cs, Parts: []PlacementV1{old}}, cat)
							if err != nil || len(notes) != 0 {
								t.Fatalf("%+v %+v: %v %v", o, old, err, notes)
							}
							if err := Validate(up, cat); err != nil {
								t.Fatalf("%+v %+v: invalid: %v", o, old, err)
							}
							got, err := ProjectView(part, up.Parts[0], up.Case, pair[0])
							if err != nil {
								t.Fatal(err)
							}
							label := fmt.Sprintf("%+v %s %s/%s rot %g mirror %v flow %q -> %s spin %g mirror %v", o, id, pair[0], pair[1], rot, mirror, flow,
								up.Parts[0].Facing, up.Parts[0].Spin, up.Parts[0].Mirror)
							wb := want.Box
							flipU := pair[0] == ViewSide && o.FrontOnSideLeft()
							if flipU {
								wb.X = cs.Dims.D - wb.X - wb.W
							}
							if pair[0] == ViewTop {
								wb.Y = cs.Dims.D - wb.Y - wb.H
							}
							if got.EdgeOn != want.EdgeOn || got.Away || !close6(got.Box.X, wb.X) || !close6(got.Box.Y, wb.Y) || !close6(got.Box.W, wb.W) || !close6(got.Box.H, wb.H) {
								t.Fatalf("%s: box %+v edge %v away %v, want %+v edge %v", label, got.Box, got.EdgeOn, got.Away, wb, want.EdgeOn)
							}
							for i := range got.LEDs {
								wx, wy := want.LEDs[i].X, want.LEDs[i].Y
								if flipU {
									wx = cs.Dims.D - wx
								}
								if pair[0] == ViewTop {
									wy = cs.Dims.D - wy
								}
								if !close6(got.LEDs[i].X, wx) || !close6(got.LEDs[i].Y, wy) {
									t.Fatalf("%s: LED %d at %v, want (%g, %g)", label, i, got.LEDs[i], wx, wy)
								}
							}
							n++
						}
					}
				}
			}
		}
	}
	if n < 1500 {
		t.Fatalf("only %d combinations checked", n)
	}
}
