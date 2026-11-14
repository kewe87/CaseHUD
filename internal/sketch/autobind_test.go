package sketch

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// autoCatalog adds the parts of a typical build to the suggest catalog: TL
// fans with LEDs on a top and a bottom edge, a memory module, a CPU block,
// a GPU block and a distribution plate.
func autoCatalog(t *testing.T) Catalog {
	t.Helper()
	c := suggestCatalog(t)
	add := func(id, cat string, w, h, d float64, pos []string) {
		p, err := ParsePart([]byte(fmt.Sprintf(`{"format":1,"id":%q,"category":%q,"dimensions_mm":{"w":%g,"h":%g,"d":%g},
 "outline":{"front":[{"type":"rect","x":0,"y":0,"w":%g,"h":%g}]},"leds":{"count":%d,"positions_mm":[%s]}}`,
			id, cat, w, h, d, w, h, len(pos), strings.Join(pos, ","))))
		if err != nil {
			t.Fatal(err)
		}
		delete(c.Parts, id)
		if err := c.AddPart(p); err != nil {
			t.Fatal(err)
		}
	}
	var ring []string // 13 LEDs along the bottom edge, then 13 along the top edge
	for i := 0; i < 13; i++ {
		ring = append(ring, fmt.Sprintf("[%g,117.2]", 7.8+8.7*float64(i)))
	}
	for i := 0; i < 13; i++ {
		ring = append(ring, fmt.Sprintf("[%g,6.8]", 7.8+8.7*float64(i)))
	}
	add("uni-fan-tl-reverse-120", "fan", 120, 124, 28, ring)
	add("uni-fan-tl-lcd-120", "fan-lcd", 120, 124, 28, ring)
	add("uni-fan-tl-lcd-reverse-120", "fan-lcd", 120, 124, 28, ring)
	row := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, fmt.Sprintf("[%d,5]", 5+3*i))
		}
		return out
	}
	add("test-ram", "ram", 133, 40, 8, row(8))
	add("test-cpu", "cpu-block", 80, 80, 30, row(8))
	add("test-gpu", "gpu-block", 260, 120, 30, row(11))
	add("test-distro", "distro-plate", 120, 300, 40, row(18))
	return c
}

// kevinSketch is the build the feature was made for: three TL fans on top,
// one TL fan with a display at the rear, three with a display at the glass
// side, a mainboard with a CPU block and two memory modules, a GPU block and
// a distribution plate (positions as drawn in the editor).
func kevinSketch(glassRight bool, labels bool) Sketch {
	k := Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 290, H: 471, D: 478}}
	if glassRight {
		k.Orientation = Orientation{GlassSide: OrientRight}
	}
	pl := func(id, part, facing string, x, y, z float64, label string) Placement {
		if !labels {
			label = ""
		}
		return Placement{ID: id, Part: part, Facing: facing, Pos: Vec3{X: x, Y: y, Z: z}, Label: label}
	}
	return Sketch{Version: Version, Case: k, Parts: []Placement{
		pl("p7", "test-board", FacingSide, 260, 192, 146, "Mainboard"),
		pl("p9", "test-cpu", FacingSide, 214, 125, 144, "CPU"),
		pl("p2", "test-ram", FacingRear, 208, 122, 206, "RAM 1"),
		pl("p10", "test-ram", FacingRear, 208, 122, 224, "RAM 2"),
		pl("p11", "test-gpu", FacingTop, 75, 199, 148, "GPU"),
		pl("p12", "test-distro", FacingTop, 70, 300, 239, "Distro"),
		pl("p13", "uni-fan-tl-lcd-120", FacingRear, 60, 90, 18, "Rear"),
		pl("p3", "uni-fan-tl-reverse-120", FacingTop, 150, 14, 381, "Top 1"),
		pl("p5", "uni-fan-tl-reverse-120", FacingTop, 145, 14, 237, "Top 2"),
		pl("p6", "uni-fan-tl-reverse-120", FacingTop, 145, 14, 113, "Top 3"),
		pl("p1", "uni-fan-tl-lcd-reverse-120", FacingSide, 14, 112, 400, "Side 1"),
		pl("p4", "uni-fan-tl-lcd-reverse-120", FacingSide, 14, 236, 400, "Side 2"),
		pl("p8", "uni-fan-tl-lcd-reverse-120", FacingSide, 14, 360, 400, "Side 3"),
	}}
}

// kevinHW is the hardware of that build: port 1 three fans, port 2 one fan
// with a display, port 3 three fans with a display.
func kevinHW() AutoBindInput {
	var fans []DetectedFan
	for i := 0; i < 3; i++ {
		fans = append(fans, DetectedFan{Port: 1, Index: i}, DetectedFan{Port: 3, Index: i, LCD: true})
	}
	fans = append(fans, DetectedFan{Port: 2, Index: 0, LCD: true})
	return AutoBindInput{
		Devices: []DetectedDevice{
			{ID: "lianlitl:0416:7372", Backend: "lianlitl", Zones: []DetectedZone{
				{ID: "port1", Count: 6, Max: 6}, {ID: "port2", Count: 2, Max: 2}, {ID: "port3", Count: 6, Max: 6}}},
			{ID: "asusaura:0b05:19af", Backend: "asusaura", Zones: []DetectedZone{
				{ID: "onboard", Count: 9, Max: 9}, {ID: "argb1", Max: 120}, {ID: "argb2", Max: 120}}},
			{ID: "enram:0b00:70", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 8, Max: 8}}},
			{ID: "enram:0b00:72", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 8, Max: 8}}},
			{ID: "demo:keyboard", Backend: "fake", Zones: []DetectedZone{{ID: "keys", Count: 20, Max: 20}}},
		},
		Fans: fans,
	}
}

// binds renders the bindings as "part=device/zone#order", sorted.
func binds(r AutoBindResult) []string {
	out := make([]string, 0, len(r.Binds))
	for _, b := range r.Binds {
		dev := b.Device
		if i := strings.IndexByte(dev, ':'); i > 0 {
			dev = dev[:i]
		}
		out = append(out, fmt.Sprintf("%s=%s/%s#%d", b.Part, dev, b.Zone, b.Order))
	}
	sort.Strings(out)
	return out
}

func zonesOf(r AutoBindResult) []string {
	var out []string
	for _, z := range r.Zones {
		out = append(out, z.Zone)
	}
	return out
}

func notesOf(r AutoBindResult) []string {
	var out []string
	for _, n := range r.Notes {
		out = append(out, n.Code)
	}
	return out
}

func TestAutoBind(t *testing.T) {
	c := autoCatalog(t)
	kevinBinds := func(top, side []string) []string {
		out := []string{"p13=lianlitl/port2#0", "p7=asusaura/onboard#0", "p2=enram/leds#0", "p10=enram/leds#0"}
		for i, id := range top {
			out = append(out, fmt.Sprintf("%s=lianlitl/port1#%d", id, i))
		}
		for i, id := range side {
			out = append(out, fmt.Sprintf("%s=lianlitl/port3#%d", id, i))
		}
		sort.Strings(out)
		return out
	}
	twoPorts := func(a, b int) AutoBindInput {
		in := AutoBindInput{Devices: []DetectedDevice{{ID: "lianlitl:0416:7372", Backend: "lianlitl",
			Zones: []DetectedZone{{ID: "port1", Count: 2 * a, Max: 2 * a}, {ID: "port2", Count: 2 * b, Max: 2 * b}}}}}
		for i := 0; i < a; i++ {
			in.Fans = append(in.Fans, DetectedFan{Port: 1, Index: i})
		}
		for i := 0; i < b; i++ {
			in.Fans = append(in.Fans, DetectedFan{Port: 2, Index: i})
		}
		return in
	}
	fanRow := func(n int, y float64, facing string) []Placement {
		var out []Placement
		for i := 0; i < n; i++ {
			out = append(out, Placement{ID: fmt.Sprintf("%s%d", facing, i), Part: "uni-fan-tl-reverse-120", Facing: facing,
				Pos: Vec3{X: 145, Y: y, Z: 100 + 130*float64(i)}})
		}
		return out
	}
	box := Case{Source: "custom", ID: CustomCaseID, Dims: Dims{W: 290, H: 471, D: 478}}
	cases := []struct {
		name  string
		s     Sketch
		in    AutoBindInput
		want  []string
		open  []string
		zones []string
		notes []string
	}{
		{
			name: "kevin: by the numbers in the labels",
			s:    kevinSketch(false, true), in: kevinHW(),
			want:  kevinBinds([]string{"p3", "p5", "p6"}, []string{"p1", "p4", "p8"}),
			open:  []string{"p9", "p11", "p12"},
			zones: []string{"argb1", "argb2"},
		},
		{
			name: "kevin without labels: left to right, top to bottom",
			s:    kevinSketch(false, false), in: kevinHW(),
			// glass left: the rear is on the left of the net, so the top row runs from the rear fan
			want:  kevinBinds([]string{"p6", "p5", "p3"}, []string{"p1", "p4", "p8"}),
			open:  []string{"p9", "p11", "p12"},
			zones: []string{"argb1", "argb2"},
		},
		{
			name: "kevin, glass on the right: the net is mirrored, so is the row",
			s:    kevinSketch(true, false), in: kevinHW(),
			want:  kevinBinds([]string{"p3", "p5", "p6"}, []string{"p1", "p4", "p8"}),
			open:  []string{"p9", "p11", "p12"},
			zones: []string{"argb1", "argb2"},
		},
		{
			name: "fan list not read: the zone sizes decide, displays unknown",
			s:    kevinSketch(false, true),
			in:   func() AutoBindInput { in := kevinHW(); in.Fans = nil; return in }(),
			// port1 and port3 both hold three fans: the top and the side group fit either
			want:  []string{"p10=enram/leds#0", "p13=lianlitl/port2#0", "p2=enram/leds#0", "p7=asusaura/onboard#0"},
			open:  []string{"p9", "p11", "p12", "p3", "p5", "p6", "p1", "p4", "p8"},
			zones: []string{"argb1", "argb2", "port1", "port3"},
			notes: []string{"tl-same"},
		},
		{
			name: "two ports with the same fans: nothing guessed",
			s:    Sketch{Version: Version, Case: box, Parts: append(fanRow(3, 14, FacingTop), fanRow(3, 457, FacingBottom)...)},
			in:   twoPorts(3, 3),
			open: []string{"top0", "top1", "top2", "bottom0", "bottom1", "bottom2"}, zones: []string{"port1", "port2"},
			notes: []string{"tl-same"},
		},
		{
			name: "one port left over takes the rest across faces",
			s:    Sketch{Version: Version, Case: box, Parts: append(fanRow(2, 14, FacingTop), fanRow(1, 457, FacingBottom)...)},
			in:   twoPorts(1, 2),
			// the bottom fan alone fits port 1; then port 2 takes the top pair
			want: []string{"bottom0=lianlitl/port1#0", "top0=lianlitl/port2#0", "top1=lianlitl/port2#1"},
		},
		{
			name: "more fans drawn than found: left open",
			s:    Sketch{Version: Version, Case: box, Parts: fanRow(3, 14, FacingTop)},
			in:   twoPorts(2, 0),
			open: []string{"top0", "top1", "top2"}, zones: []string{"port1"},
			notes: []string{"tl-count"},
		},
		{
			name: "memory: more modules than zones",
			s: Sketch{Version: Version, Case: box, Parts: []Placement{
				{ID: "r1", Part: "test-ram", Facing: FacingSide, Pos: Vec3{X: 280, Y: 100, Z: 200}},
				{ID: "r2", Part: "test-ram", Facing: FacingSide, Pos: Vec3{X: 280, Y: 100, Z: 220}},
			}},
			in:   AutoBindInput{Devices: []DetectedDevice{{ID: "enram:0b00:70", Backend: "enram", Zones: []DetectedZone{{ID: "leds", Count: 8, Max: 8}}}}},
			open: []string{"r1", "r2"}, zones: []string{"leds"}, notes: []string{"ram-count"},
		},
		{
			name: "mainboard with more LEDs than the zone reports: bound, noted",
			s:    Sketch{Version: Version, Case: box, Parts: []Placement{{ID: "mb", Part: "test-board", Facing: FacingSide, Pos: Vec3{X: 260, Y: 200, Z: 150}}}},
			in:   AutoBindInput{Devices: []DetectedDevice{{ID: "demo:board", Backend: "asusaura", Zones: []DetectedZone{{ID: "onboard", Count: 8, Max: 20}, {ID: "argb1", Max: 120}}}}},
			want: []string{"mb=demo/onboard#0"}, notes: []string{"leds"},
		},
		{
			name: "mainboard with more LEDs than the zone can drive: open, noted",
			s:    Sketch{Version: Version, Case: box, Parts: []Placement{{ID: "mb", Part: "test-board", Facing: FacingSide, Pos: Vec3{X: 260, Y: 200, Z: 150}}}},
			in:   AutoBindInput{Devices: []DetectedDevice{{ID: "demo:board", Backend: "asusaura", Zones: []DetectedZone{{ID: "onboard", Count: 8, Max: 8}, {ID: "argb1", Max: 120}}}}},
			open: []string{"mb"}, zones: []string{"onboard", "argb1"}, notes: []string{"leds-max"},
		},
		{
			name: "display ring",
			s:    Sketch{Version: Version, Case: box, Parts: []Placement{{ID: "scr", Part: "test-screen", Facing: FacingSide, Pos: Vec3{X: 10, Y: 200, Z: 240}}}},
			in:   AutoBindInput{Devices: []DetectedDevice{{ID: "lianli:0416:8050", Backend: "lianli", Zones: []DetectedZone{{ID: "ring", Count: 60, Max: 60}}}}},
			want: []string{"scr=lianli/ring#0"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := AutoBind(tc.s, c, tc.in)
			if got := binds(r); !reflect.DeepEqual(got, append([]string{}, tc.want...)) {
				t.Errorf("binds\n got %v\nwant %v", got, tc.want)
			}
			if !reflect.DeepEqual(r.Open, append([]string{}, tc.open...)) {
				t.Errorf("open %v, want %v", r.Open, tc.open)
			}
			if got := zonesOf(r); !reflect.DeepEqual(got, tc.zones) {
				t.Errorf("zones %v, want %v", got, tc.zones)
			}
			if got := notesOf(r); !reflect.DeepEqual(got, tc.notes) {
				t.Errorf("notes %v, want %v", got, tc.notes)
			}
			// the result stores: every binding validates
			if err := Validate(ApplyAutoBind(tc.s, r), c); err != nil {
				t.Errorf("result does not validate: %v", err)
			}
		})
	}
}

// Parts and zones that are already bound stay as they are.
func TestAutoBindKeepsBindings(t *testing.T) {
	c := autoCatalog(t)
	s := kevinSketch(false, true)
	for i := range s.Parts {
		switch s.Parts[i].ID {
		case "p9": // the CPU block sits on the zone the mainboard would take
			s.Parts[i].Bind = &Binding{Device: "asusaura:0b05:19af", Zone: "onboard"}
		case "p13": // the rear fan was bound by hand to another port
			s.Parts[i].Bind = &Binding{Device: "lianlitl:0416:7372", Zone: "port3", Order: 5}
		}
	}
	r := AutoBind(s, c, kevinHW())
	for _, b := range r.Binds {
		if b.Part == "p9" || b.Part == "p13" || b.Zone == "onboard" || b.Zone == "port3" {
			t.Errorf("touched a binding: %+v", b)
		}
	}
	out := ApplyAutoBind(s, r)
	for i, p := range out.Parts {
		if p.ID == "p9" || p.ID == "p13" {
			if !reflect.DeepEqual(p.Bind, s.Parts[i].Bind) {
				t.Errorf("%s: %+v, was %+v", p.ID, p.Bind, s.Parts[i].Bind)
			}
		}
	}
	// nothing left to do: a second run finds nothing new
	if again := AutoBind(out, c, kevinHW()); len(again.Binds) != 0 {
		t.Errorf("second run binds again: %v", binds(again))
	}
}

// Case lights are never matched automatically: their header is found by
// lighting it.
func TestAutoBindCaseLightsStayOpen(t *testing.T) {
	c, tpl := lCatalog(t, lCaseJSON(false, false))
	s := lSketch(tpl, c)
	r := AutoBind(s, c, AutoBindInput{Devices: []DetectedDevice{{ID: "demo:board", Backend: "asusaura",
		Zones: []DetectedZone{{ID: "onboard", Count: 8, Max: 8}, {ID: "argb1", Max: 120}}}}})
	if len(r.Binds) != 0 {
		t.Fatalf("bound %v", binds(r))
	}
	if len(r.Open) == 0 || !strings.HasPrefix(r.Open[0], caseLightPfx) {
		t.Fatalf("open %v", r.Open)
	}
	if got := zonesOf(r); !reflect.DeepEqual(got, []string{"onboard", "argb1"}) {
		t.Fatalf("zones %v", got)
	}
}

func TestAutoBindOnly(t *testing.T) {
	c := autoCatalog(t)
	r := AutoBind(kevinSketch(false, true), c, kevinHW())
	o := r.Only([]string{"p13", "p7"})
	if got := binds(o); !reflect.DeepEqual(got, []string{"p13=lianlitl/port2#0", "p7=asusaura/onboard#0"}) {
		t.Fatalf("binds %v", got)
	}
	if len(o.Open) != len(r.Open)+len(r.Binds)-2 {
		t.Fatalf("open %v", o.Open)
	}
	if all := r.Only(nil); len(all.Binds) != len(r.Binds) {
		t.Fatal("nil must keep all")
	}
}

func TestRingGroups(t *testing.T) {
	c := autoCatalog(t)
	g, n := RingGroups(c.Parts["uni-fan-tl-reverse-120"], Placement{})
	if n != 2 || len(g) != 26 {
		t.Fatalf("rings %d, groups %d", n, len(g))
	}
	for i, v := range g {
		if want := map[bool]int{true: 1, false: 0}[i < 13]; v != want { // the first 13 run along the bottom edge
			t.Fatalf("LED %d in ring %d, want %d", i+1, v, want)
		}
	}
	if _, n := RingGroups(c.Parts["test-ram"], Placement{}); n != 1 {
		t.Fatalf("LEDs on one edge: %d rings", n)
	}
	if !RingZone("lianlitl", "port2") || RingZone("asusaura", "port2") || RingZone("lianlitl", "ring") {
		t.Fatal("RingZone")
	}
}

// A TL fan bound to a port takes two zone LEDs, top ring first, at the
// centres of its rings.
func TestDeriveRingZones(t *testing.T) {
	c := autoCatalog(t)
	s := ApplyAutoBind(kevinSketch(false, true), AutoBind(kevinSketch(false, true), c, kevinHW()))
	rings := func(dev, zone string) bool { return strings.HasPrefix(dev, "lianlitl:") && RingZone("lianlitl", zone) }
	d, err := DeriveWith(s, c, rings)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"port1": 6, "port2": 2, "port3": 6, "onboard": 9, "leds": 8}
	for _, z := range d.Zones {
		if w, ok := want[z.Zone]; ok && z.LEDs != w {
			t.Errorf("%s/%s: %d LEDs, want %d", z.Device, z.Zone, z.LEDs, w)
		}
	}
	for _, comp := range d.Components {
		if comp.Name != ComponentName("p3") {
			continue
		}
		if len(comp.LEDs) != 2 {
			t.Fatalf("fan on a port: %d LEDs", len(comp.LEDs))
		}
	}
	// the top fans face up: the net shows their LED side; the top ring is
	// the one nearer the top edge of the fan's front view, the first LED
	// the controller drives
	plain, err := Derive(s, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range plain.Zones {
		if z.Zone == "port1" && z.LEDs != 78 {
			t.Errorf("without ring zones port1 takes %d LEDs, want 78", z.LEDs)
		}
	}
}
