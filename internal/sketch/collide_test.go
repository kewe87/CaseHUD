package sketch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestOverlaps(t *testing.T) {
	cat := Builtin()
	fan := func(id string, x, y, z float64) Placement {
		return Placement{ID: id, Part: "generic-fan-120", Facing: FacingSide, Pos: Vec3{x, y, z}}
	}
	s := Sketch{Version: Version, Case: box(), Parts: []Placement{
		fan("a", 12.5, 100, 100),
		fan("b", 12.5, 100, 300),   // 200 mm apart: free
		fan("c", 12.5, 100, 219),   // 119 mm from a: 1 mm of overlap
		fan("d", 12.5, 100, 219.7), // 0.3 mm of overlap: within the tolerance
	}}
	got := Overlaps(s, cat)
	has := func(a, b string) bool {
		for _, o := range got {
			if (o.A == a && o.B == b) || (o.A == b && o.B == a) {
				return true
			}
		}
		return false
	}
	if !has("a", "c") || has("a", "b") || has("a", "d") || !has("c", "d") {
		t.Errorf("overlaps %v", got)
	}

	// mounted parts may overlap their parent, other parts may not
	s.Parts = []Placement{fan("mb", 12.5, 100, 100), fan("ram", 12.5, 100, 100), fan("other", 12.5, 100, 100)}
	s.Parts[1].On = "mb"
	got = Overlaps(s, cat)
	if len(got) != 2 || has("mb", "ram") {
		t.Errorf("mounted: %v", got)
	}

	// strips overlap anything
	s.Parts = []Placement{fan("a", 12.5, 100, 100), {ID: "s", Part: "generic-strip", Facing: FacingSide, Pos: Vec3{4, 100, 100}}}
	if got := Overlaps(s, cat); len(got) != 0 {
		t.Errorf("strip: %v", got)
	}

	// touching boxes and unknown parts are no overlap
	s.Parts = []Placement{fan("a", 12.5, 100, 100), fan("t", 12.5, 100, 220), {ID: "x", Part: "gone", Facing: FacingSide}}
	if got := Overlaps(s, cat); len(got) != 0 {
		t.Errorf("touching: %v", got)
	}

	// different depth: a fan in front of a fan does not overlap
	s.Parts = []Placement{fan("a", 12.5, 100, 100), fan("b", 37.5, 100, 100)}
	if got := Overlaps(s, cat); len(got) != 0 {
		t.Errorf("stacked: %v", got)
	}
	s.Parts[1].Pos.X = 30
	if got := Overlaps(s, cat); len(got) != 1 {
		t.Errorf("stacked too close: %v", got)
	}

	// the sketch with overlaps still validates and derives
	if err := Validate(s, cat); err != nil {
		t.Errorf("overlap rejected: %v", err)
	}
}

func TestOverlapsWith(t *testing.T) {
	cat := Builtin()
	fan := func(id string, z float64) Placement {
		return Placement{ID: id, Part: "generic-fan-120", Facing: FacingSide, Pos: Vec3{12.5, 100, z}}
	}
	s := Sketch{Version: Version, Case: box(), Parts: []Placement{fan("a", 100), fan("b", 300)}}
	if got := OverlapsWith(s, cat, fan("new", 300)); len(got) != 1 || got[0] != "b" {
		t.Errorf("new part: %v", got)
	}
	if got := OverlapsWith(s, cat, fan("a", 100)); len(got) != 0 {
		t.Errorf("a part does not overlap itself: %v", got)
	}
	if got := OverlapsWith(s, cat, fan("a", 290)); len(got) != 1 || got[0] != "b" {
		t.Errorf("moved copy: %v", got)
	}
	m := fan("kid", 300)
	m.On = "b"
	if got := OverlapsWith(s, cat, m); len(got) != 0 {
		t.Errorf("mounted: %v", got)
	}
	if got := OverlapsWith(s, cat, Placement{ID: "z", Part: "gone"}); got != nil {
		t.Errorf("unknown part: %v", got)
	}
}

// The golden file records which placements overlap in a set of cases so the
// editor, which checks the same rule in JavaScript, can be tested against it.
// Run `go test ./internal/sketch -run OverlapGolden -update` after an intended
// change of the rule.
type overlapCase struct {
	Name     string      `json:"name"`
	Parts    []Placement `json:"parts"`
	Overlaps [][2]string `json:"overlaps"`
}

func overlapParts() map[string]Part {
	parts := goldenParts()
	acc := Part{Format: 1, ID: "golden-acc", Category: "case-accessory", Dimensions: PartDims{W: 80, H: 80, D: 30},
		LEDs: PartLEDs{Count: 1, Positions: [][2]float64{{40, 40}}}}
	strip := Part{Format: 1, ID: "golden-strip", Category: "strip", Dimensions: PartDims{W: 100, H: 8, D: 4},
		LEDs: PartLEDs{Count: 1, Positions: [][2]float64{{50, 4}}}}
	parts["golden-acc"], parts["golden-strip"] = acc, strip
	return parts
}

func overlapCases() []overlapCase {
	plate := func(id string, f string, x, y, z, spin float64) Placement {
		return Placement{ID: id, Part: "golden-plate", Facing: f, Pos: Vec3{x, y, z}, Spin: spin}
	}
	fan := func(id string, f string, x, y, z float64) Placement {
		return Placement{ID: id, Part: "golden-fan", Facing: f, Pos: Vec3{x, y, z}}
	}
	with := func(p Placement, on string) Placement { p.On = on; return p }
	return []overlapCase{
		{Name: "apart", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 5, 100, 300, 0)}},
		{Name: "touching", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 5, 100, 160, 0)}},
		{Name: "inside tolerance", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 5, 100, 159.6, 0)}},
		{Name: "just over tolerance", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 5, 100, 159.4, 0)}},
		{Name: "stacked in depth", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 15, 100, 100, 0)}},
		{Name: "stacked too close", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 14, 100, 100, 0)}},
		{Name: "rotated box", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 37), plate("b", FacingSide, 5, 100, 140, 0), plate("c", FacingSide, 5, 100, 190, 0)}},
		{Name: "turned upright against flat", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingBottom, 5, 100, 100, 0)}},
		{Name: "fan through plate", Parts: []Placement{plate("a", FacingFront, 100, 100, 100, 0), fan("b", FacingFront, 110, 100, 100)}},
		{Name: "mounted parent and child", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), with(plate("b", FacingSide, 5, 100, 100, 0), "a"), plate("c", FacingSide, 5, 100, 100, 0)}},
		{Name: "mounted other way round", Parts: []Placement{with(plate("a", FacingSide, 5, 100, 100, 0), "b"), plate("b", FacingSide, 5, 100, 100, 0)}},
		{Name: "siblings on one parent", Parts: []Placement{plate("p", FacingSide, 5, 100, 100, 0), with(plate("k1", FacingSide, 5, 100, 100, 0), "p"), with(plate("k2", FacingSide, 5, 100, 100, 0), "p")}},
		{Name: "strip and accessory are exempt", Parts: []Placement{
			plate("a", FacingSide, 5, 100, 100, 0),
			{ID: "s", Part: "golden-strip", Facing: FacingSide, Pos: Vec3{5, 100, 100}},
			{ID: "x", Part: "golden-acc", Facing: FacingSide, Pos: Vec3{5, 100, 100}}}},
		{Name: "unknown part is ignored", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), {ID: "u", Part: "gone", Facing: FacingSide, Pos: Vec3{5, 100, 100}}}},
		{Name: "chain of three", Parts: []Placement{plate("a", FacingSide, 5, 100, 100, 0), plate("b", FacingSide, 5, 100, 140, 0), plate("c", FacingSide, 5, 100, 180, 0)}},
	}
}

func TestOverlapGolden(t *testing.T) {
	const path = "testdata/overlap_golden.json"
	parts := overlapParts()
	cat := Catalog{Parts: parts}
	cases := overlapCases()
	for i, c := range cases {
		pairs := [][2]string{}
		for _, o := range Overlaps(Sketch{Version: Version, Case: box(), Parts: c.Parts}, cat) {
			pairs = append(pairs, [2]string{o.A, o.B})
		}
		cases[i].Overlaps = pairs
	}
	var buf bytes.Buffer
	for _, k := range []string{"about", "tolerance", "parts"} {
		var v any
		switch k {
		case "about":
			v = "Pairs of placements whose boxes overlap, by the rule of Overlaps. Generated by TestOverlapGolden; the editor must report the same pairs."
		case "tolerance":
			v = OverlapTolMM
		default:
			v = parts
		}
		b, _ := json.Marshal(v)
		if buf.Len() == 0 {
			buf.WriteString("{\n")
		}
		fmt.Fprintf(&buf, " %q: %s,\n", k, b)
	}
	buf.WriteString(` "cases": [` + "\n")
	for i, c := range cases {
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		sep := ","
		if i == len(cases)-1 {
			sep = ""
		}
		fmt.Fprintf(&buf, "  %s%s\n", b, sep)
	}
	buf.WriteString(" ]\n}\n")
	if *update {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Fatal("overlaps differ from testdata/overlap_golden.json; run with -update if the change is intended")
	}
	// the cases that must be free of overlaps / must overlap, independent of the file
	byName := map[string][][2]string{}
	for _, c := range cases {
		byName[c.Name] = c.Overlaps
	}
	for _, n := range []string{"apart", "touching", "inside tolerance", "stacked in depth", "strip and accessory are exempt", "unknown part is ignored"} {
		if got := byName[n]; len(got) != 0 {
			t.Errorf("%s: unexpected overlaps %v", n, got)
		}
	}
	for _, n := range []string{"just over tolerance", "stacked too close", "turned upright against flat", "fan through plate", "siblings on one parent"} {
		if len(byName[n]) != 1 {
			t.Errorf("%s: overlaps %v, want one", n, byName[n])
		}
	}
}
