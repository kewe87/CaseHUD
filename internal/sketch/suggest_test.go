package sketch

import (
	"fmt"
	"strings"
	"testing"
)

func suggestCatalog(t *testing.T) Catalog {
	t.Helper()
	c := Builtin()
	add := func(id, cat string, w, h, d float64, n int) {
		var pos []string
		for i := 0; i < n; i++ {
			pos = append(pos, fmt.Sprintf("[%d,%d]", 5+i%10, 5+i/10))
		}
		p, err := ParsePart([]byte(fmt.Sprintf(`{"format":1,"id":%q,"category":%q,"dimensions_mm":{"w":%g,"h":%g,"d":%g},
 "outline":{"front":[{"type":"rect","x":0,"y":0,"w":%g,"h":%g}]},"leds":{"count":%d,"positions_mm":[%s]}}`,
			id, cat, w, h, d, w, h, n, strings.Join(pos, ","))))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddPart(p); err != nil {
			t.Fatal(err)
		}
	}
	add("uni-fan-tl-reverse-120", "fan", 120, 124, 28, 26)
	add("uni-fan-tl-lcd-reverse-120", "fan-lcd", 120, 124, 28, 26)
	add("test-board", "mainboard", 244, 305, 60, 9)
	add("test-screen", "display", 237, 73, 13, 60)
	return c
}

func TestSuggest(t *testing.T) {
	c := suggestCatalog(t)
	k := Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 230, H: 480, D: 450}}
	in := SuggestInput{
		Devices: []DetectedDevice{
			{ID: "asusaura:0b05:19af", Backend: "asusaura", Zones: []DetectedZone{{ID: "onboard", Count: 9, Max: 9}, {ID: "argb1"}}},
			{ID: "enram:0b00:70", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 8, Max: 8}}},
			{ID: "enram:0b00:72", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 8, Max: 8}}},
			{ID: "lianli:0416:8050", Backend: "lianli", Zones: []DetectedZone{{ID: "ring", Count: 60, Max: 60}}},
			{ID: "lianlitl:0416:7372", Backend: "lianlitl", Zones: []DetectedZone{{ID: "port1", Count: 4, Max: 4}}},
		},
		Fans: []DetectedFan{{Port: 1, Index: 1}, {Port: 1, Index: 0, LCD: true}},
	}
	got := Suggest(c, k, in)
	byPart := map[string]int{}
	for _, p := range got.Parts {
		byPart[p.Part]++
	}
	want := map[string]int{"test-board": 1, "generic-ram-dimm": 2, "test-screen": 1, "uni-fan-tl-lcd-reverse-120": 1, "uni-fan-tl-reverse-120": 1}
	for part, n := range want {
		if byPart[part] != n {
			t.Errorf("%s: %d, want %d (%v)", part, byPart[part], n, got.Parts)
		}
	}
	if err := Validate(Sketch{Version: Version, Case: k, Parts: got.Parts}, c); err != nil {
		t.Fatalf("suggestion does not validate: %v", err)
	}
	var board string
	for _, p := range got.Parts {
		switch {
		case p.Part == "test-board":
			board = p.ID
			if p.Bind == nil || p.Bind.Zone != "onboard" {
				t.Errorf("board binding: %+v", p.Bind)
			}
		case strings.HasPrefix(p.ID, "fan-"):
			if p.Bind != nil {
				t.Errorf("fan bound to a zone that is too small: %+v", p.Bind)
			}
		}
	}
	for _, p := range got.Parts {
		if strings.HasPrefix(p.ID, "ram-") && (p.On != board || p.Bind == nil) {
			t.Errorf("ram %+v not mounted on the board / bound", p)
		}
	}
	// The LCD fan comes first (port 1, index 0) and carries the position label.
	for _, p := range got.Parts {
		if p.Part == "uni-fan-tl-lcd-reverse-120" && p.Label != "P1.1" {
			t.Errorf("label %q", p.Label)
		}
	}
	if len(got.Notes) == 0 {
		t.Error("no note about the unbound fans")
	}

	// Nothing is suggested twice: zones the sketch already binds are skipped,
	// ids stay unique, and the chain order continues.
	again := Suggest(c, k, SuggestInput{Devices: in.Devices, Fans: in.Fans, Existing: Sketch{Version: Version, Case: k, Parts: got.Parts}})
	for _, p := range again.Parts {
		if p.Bind != nil {
			t.Errorf("zone bound twice: %+v", p)
		}
		for _, q := range got.Parts {
			if q.ID == p.ID {
				t.Errorf("id %q reused", p.ID)
			}
		}
	}
}

func TestSuggestEmptyAndUnknown(t *testing.T) {
	c := Builtin()
	k := Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 230, H: 480, D: 450}}
	got := Suggest(c, k, SuggestInput{})
	if got.Parts == nil || got.Notes == nil || len(got.Parts) != 0 {
		t.Fatalf("%+v", got)
	}
	got = Suggest(c, k, SuggestInput{Devices: []DetectedDevice{{ID: "x:1", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 77}}}}})
	if len(got.Parts) != 0 || len(got.Notes) != 1 {
		t.Fatalf("%+v", got)
	}
}
