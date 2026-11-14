package sketch

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

const (
	kevinTL    = "lianlitl:0416:7372"
	kevinBoard = "asusaura:0b05:1aa6"
)

// kevinSyncHW is Kevin's hardware with the fan controller in sync mode:
// port 1 three fans, port 2 one fan with a display, port 3 three fans with
// a display; the mainboard headers carry 78, 47 and 18 LEDs.
func kevinSyncHW(source string) AutoBindInput {
	in := kevinHW()
	in.Devices[0].Mode, in.Devices[0].SyncSource = TLModeSync, source
	in.Devices[1] = DetectedDevice{ID: kevinBoard, Backend: "asusaura", Zones: []DetectedZone{
		{ID: "onboard", Count: 9, Max: 9}, {ID: "argb1", Count: 78, Max: 120}, {ID: "argb2", Count: 47, Max: 120}, {ID: "argb3", Count: 18, Max: 120}}}
	return in
}

// syncBinds renders the bindings as "part=zone#order" (+"|" when parallel), in order.
func syncBinds(bs []AutoBinding) []string {
	bs = append([]AutoBinding(nil), bs...)
	sort.SliceStable(bs, func(i, j int) bool {
		return bs[i].Zone < bs[j].Zone || bs[i].Zone == bs[j].Zone && bs[i].Order < bs[j].Order
	})
	var out []string
	for _, b := range bs {
		if b.Kind != "sync" {
			continue
		}
		s := fmt.Sprintf("%s=%s#%d", b.Part, b.Zone, b.Order)
		if b.Parallel {
			s += "|"
		}
		out = append(out, s)
	}
	return out
}

// Kevin's build: the side fans (glass) are the main chain, top fan k and the
// rear fan (one fan: parallel to fan 1) mirror side fan k.
var kevinSync = []string{"p1=argb1#0", "p3=argb1#1|", "p13=argb1#2|", "p4=argb1#3", "p5=argb1#4|", "p8=argb1#5", "p6=argb1#6|"}

func TestAutoBindSync(t *testing.T) {
	c := autoCatalog(t)
	board := kevinBoard + "/argb1"
	cases := []struct {
		name    string
		in      AutoBindInput
		want    []string // stored bindings
		propose []string // proposed only
		notes   []string
		fansOn  string // zone the fans are bound to directly ("": none)
	}{
		{name: "sync source set", in: kevinSyncHW(board), want: kevinSync},
		{name: "fan list not read: groups taken as they are", in: func() AutoBindInput { in := kevinSyncHW(board); in.Fans = nil; return in }(), want: kevinSync},
		{name: "no source, one header fits: proposal only", in: kevinSyncHW(""), propose: kevinSync},
		{name: "no source, header confirmed", in: func() AutoBindInput { in := kevinSyncHW(""); in.SyncSource = board; return in }(), want: kevinSync},
		{name: "no source, two headers fit: nothing guessed", in: func() AutoBindInput {
			in := kevinSyncHW("")
			in.Devices[1].Zones[2].Count = 78
			return in
		}(), notes: []string{"sync-source"}},
		{name: "no source, no header fits", in: func() AutoBindInput {
			in := kevinSyncHW("")
			in.Devices[1].Zones[1].Count = 0
			return in
		}(), notes: []string{"sync-source"}},
		{name: "source is no ARGB header", in: kevinSyncHW(kevinBoard + "/onboard"), notes: []string{"sync-source"}},
		{name: "header too short", in: func() AutoBindInput {
			in := kevinSyncHW(board)
			in.Devices[1].Zones[1].Max = 60
			return in
		}(), notes: []string{"leds-max"}},
		{name: "canvas mode: the ports as before", in: func() AutoBindInput {
			in := kevinSyncHW(board)
			in.Devices[0].Mode = "canvas"
			return in
		}(), fansOn: "port"},
		{name: "mode not known: the ports as before", in: func() AutoBindInput {
			in := kevinSyncHW(board)
			in.Devices[0].Mode = ""
			return in
		}(), fansOn: "port"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := kevinSketch(false, true)
			r := AutoBind(s, c, tc.in)
			if got := syncBinds(r.Binds); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("binds\n got %v\nwant %v", got, tc.want)
			}
			var prop []string
			if r.Sync != nil {
				prop = syncBinds(r.Sync.Proposed)
				if r.Sync.Derived != (tc.propose != nil) {
					t.Errorf("derived %v", r.Sync.Derived)
				}
				if r.Sync.Fans != 3 || !reflect.DeepEqual(r.Sync.Main, []string{"p1", "p4", "p8"}) || r.Sync.Board != kevinBoard || r.Sync.Zone != "argb1" {
					t.Errorf("sync %+v", r.Sync)
				}
			} else if tc.want != nil || tc.propose != nil {
				t.Error("no sync info")
			}
			if !reflect.DeepEqual(prop, tc.propose) {
				t.Errorf("proposed\n got %v\nwant %v", prop, tc.propose)
			}
			if got := notesOf(r); !reflect.DeepEqual(got, tc.notes) {
				t.Errorf("notes %v, want %v", got, tc.notes)
			}
			direct := 0
			for _, b := range r.Binds {
				if strings.HasPrefix(b.Zone, "port") {
					direct++
				}
			}
			if (tc.fansOn == "port") != (direct == 7) || (tc.fansOn == "" && direct != 0) {
				t.Errorf("%d fans bound to ports: %v", direct, binds(r))
			}
			if tc.propose != nil {
				for _, id := range []string{"p1", "p3", "p13"} {
					if !contains(r.Open, id) {
						t.Errorf("proposed fan %s not open: %v", id, r.Open)
					}
				}
			}
			// nothing else changes: memory and mainboard as always
			for _, want := range []string{"p2=enram/leds#0", "p10=enram/leds#0"} {
				if !contains(binds(r), want) {
					t.Errorf("missing %s: %v", want, binds(r))
				}
			}
			if err := Validate(ApplyAutoBind(s, r), c); err != nil {
				t.Errorf("result does not validate: %v", err)
			}
		})
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Equal chains: the one on the better seen face leads, whatever the port
// numbers; without a display difference and with the side group drawn on
// port 1 the side group still leads.
func TestAutoBindSyncMainChain(t *testing.T) {
	c := autoCatalog(t)
	s := kevinSketch(false, true)
	for i := range s.Parts { // all fans without a display: top and side alike
		if strings.HasPrefix(s.Parts[i].Part, "uni-fan-tl-lcd-reverse") {
			s.Parts[i].Part = "uni-fan-tl-reverse-120"
		}
	}
	in := kevinSyncHW(kevinBoard + "/argb1")
	in.Fans = nil // port1 and port3 look alike
	r := AutoBind(s, c, in)
	if r.Sync == nil || !reflect.DeepEqual(r.Sync.Main, []string{"p1", "p4", "p8"}) {
		t.Fatalf("main chain %+v (%v)", r.Sync, binds(r))
	}
	// more fans beat the face: four on top, three at the side
	s.Parts = append(s.Parts, Placement{ID: "p20", Part: "uni-fan-tl-reverse-120", Facing: FacingTop, Pos: Vec3{X: 145, Y: 14, Z: 0}, Label: "Top 0"})
	in.Devices[0].Zones[0].Count = 8 // port 1: four fans
	in.Devices[1].Zones[1].Count = 104
	r = AutoBind(s, c, in)
	if r.Sync == nil || len(r.Sync.Main) != 4 || r.Sync.Main[0] != "p20" {
		t.Fatalf("main chain %+v (%v)", r.Sync, binds(r))
	}
}

// Limited to some parts: the sync chain is all or nothing, a parallel fan
// without its model would mirror the wrong fan.
func TestAutoBindSyncOnly(t *testing.T) {
	c := autoCatalog(t)
	r := AutoBind(kevinSketch(false, true), c, kevinSyncHW(kevinBoard+"/argb1"))
	if o := r.Only([]string{"p3", "p13"}); len(syncBinds(o.Binds)) != 0 || o.Sync != nil {
		t.Fatalf("partial chain kept: %v", syncBinds(o.Binds))
	}
	if o := r.Only([]string{"p1", "p3", "p13", "p4", "p5", "p8", "p6"}); !reflect.DeepEqual(syncBinds(o.Binds), kevinSync) {
		t.Fatalf("whole chain: %v", syncBinds(o.Binds))
	}
}

// After the binding the header samples the parts where they are: LED i of
// argb1 is LED i%26 of side fan i/26 at its place on the canvas, not the
// default line; the top and rear fans start at the same LED as their model.
// A wave running left to right gives every side fan different colours from
// left to right and the same column colours on all three fans.
func TestAutoBindSyncSamplesThePartPositions(t *testing.T) {
	c := autoCatalog(t)
	s := kevinSketch(false, true)
	s = ApplyAutoBind(s, AutoBind(s, c, kevinSyncHW(kevinBoard+"/argb1")))
	d, err := Derive(s, c)
	if err != nil {
		t.Fatal(err)
	}
	var zs ZoneSegments
	for _, z := range d.Zones {
		if z.Device == kevinBoard && z.Zone == "argb1" {
			zs = z
		}
	}
	if zs.LEDs != 78 || len(zs.Segments) != 7 {
		t.Fatalf("argb1: %d LEDs, %d segments", zs.LEDs, len(zs.Segments))
	}
	starts := map[string]int{}
	for _, seg := range zs.Segments {
		starts[strings.TrimPrefix(seg.Component, ReservedPrefix)] = *seg.Start
	}
	if want := map[string]int{"p1": 0, "p3": 0, "p13": 0, "p4": 26, "p5": 26, "p8": 52, "p6": 52}; !reflect.DeepEqual(starts, want) {
		t.Fatalf("starts %v, want %v", starts, want)
	}
	for _, comp := range d.Components {
		cc := comp
		if err := leds.RegisterComponent(cc); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { leds.UnregisterComponent(cc.Name) })
	}
	pts := leds.Positions(leds.Zone{ID: "argb1", Resizable: true, Max: 120}, leds.ZoneConfig{Segments: zs.Segments})
	if len(pts) != 78 {
		t.Fatalf("%d positions", len(pts))
	}
	// the canvas point of each side fan LED, as the fan alone would give it
	own := func(id string) []leds.Point {
		for _, seg := range zs.Segments {
			if seg.Component == ComponentName(id) {
				one := seg
				one.Start = nil
				return leds.Positions(leds.Zone{Resizable: true}, leds.ZoneConfig{Segments: []leds.Segment{one}})
			}
		}
		t.Fatalf("%s not on argb1", id)
		return nil
	}
	near := func(a, b leds.Point) bool { return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 }
	for f, id := range []string{"p1", "p4", "p8"} {
		o := own(id)
		for k := 0; k < 26; k++ {
			if !near(pts[f*26+k], o[k]) {
				t.Fatalf("argb1 LED %d at %v, side fan %s LED %d at %v", f*26+k, pts[f*26+k], id, k, o[k])
			}
		}
	}
	line := leds.Positions(leds.Zone{ID: "argb1", Count: 78}, leds.ZoneConfig{})
	same := 0
	for i := range pts {
		if near(pts[i], line[i]) {
			same++
		}
	}
	if same > 2 {
		t.Fatalf("%d LEDs still on the default line", same)
	}
	// a horizontal wave: colour = x on the canvas
	cols := func(f int) []float64 {
		var xs []float64
		for k := 0; k < 26; k++ {
			xs = append(xs, math.Round(pts[f*26+k].X*1e6)/1e6)
		}
		return xs
	}
	for f := 0; f < 3; f++ {
		xs := cols(f)
		lo, hi := xs[0], xs[0]
		for _, x := range xs {
			lo, hi = min(lo, x), max(hi, x)
		}
		if hi-lo < 0.02 {
			t.Fatalf("side fan %d: one colour from left to right (%v)", f+1, xs)
		}
		if !reflect.DeepEqual(xs, cols(0)) {
			t.Fatalf("side fan %d columns %v, fan 1 %v", f+1, xs, cols(0))
		}
	}
}
