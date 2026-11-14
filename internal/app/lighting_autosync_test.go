package app

import (
	"fmt"
	"math"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// withTLParts adds a TL fan with a display (26 LEDs: 13 along the bottom
// edge, 13 along the top edge, as the library has it) to the engine's
// sketch catalog.
func withTLParts(t *testing.T, e *Engine) {
	t.Helper()
	var pos []string
	for i := 0; i < 13; i++ {
		pos = append(pos, fmt.Sprintf("[%g,117.2]", 7.8+8.7*float64(i)))
	}
	for i := 0; i < 13; i++ {
		pos = append(pos, fmt.Sprintf("[%g,6.8]", 7.8+8.7*float64(i)))
	}
	p, err := sketch.ParsePart([]byte(`{"format":1,"id":"uni-fan-tl-lcd-reverse-120","category":"fan-lcd","dimensions_mm":{"w":120,"h":124,"d":28},
 "outline":{"front":[{"type":"rect","x":0,"y":0,"w":120,"h":124}]},"leds":{"count":26,"positions_mm":[` + strings.Join(pos, ",") + `]}}`))
	if err != nil {
		t.Fatal(err)
	}
	cat := e.SketchCatalog()
	if err := cat.AddPart(p); err != nil {
		t.Fatal(err)
	}
	e.library.catMu.Lock()
	e.library.cat = &cat
	e.library.catMu.Unlock()
}

func zoneView(t *testing.T, v LightingView, dev, zone string) LightingZoneView {
	t.Helper()
	for _, d := range v.Devices {
		for _, z := range d.Zones {
			if d.ID == dev && z.ID == zone {
				return z
			}
		}
	}
	t.Fatalf("zone %s/%s not in the view", dev, zone)
	return LightingZoneView{}
}

// The demo fan controller follows the mainboard (port 2: one fan, port 3:
// three fans). The side fans are laid out on the header in order, the rear
// fan mirrors fan 1; the controller's setting stays as it is and nothing is
// bound to its ports.
func TestSketchAutoBindSyncDemo(t *testing.T) {
	e := startLit(t)
	withTLParts(t, e)
	fan := func(id, facing, label string, x, y, z float64) sketch.Placement {
		return sketch.Placement{ID: id, Part: "uni-fan-tl-lcd-reverse-120", Facing: facing, Pos: sketch.Vec3{X: x, Y: y, Z: z}, Label: label}
	}
	sk := testSketch(fan("rear", sketch.FacingRear, "Rear", 60, 90, 14),
		fan("s1", sketch.FacingSide, "Seite 1", 14, 112, 300), fan("s2", sketch.FacingSide, "Seite 2", 14, 236, 300), fan("s3", sketch.FacingSide, "Seite 3", 14, 360, 300))
	if _, err := e.SetSketch(sk); err != nil {
		t.Fatal(err)
	}
	tlBinds := func(r sketch.AutoBindResult) (sync, port int) {
		for _, b := range r.Binds {
			switch {
			case b.Kind == "sync":
				sync++
			case strings.HasPrefix(b.Zone, "port"):
				port++
			}
		}
		return
	}
	// no header set and none with 78 LEDs: nothing guessed, the ports stay free
	r, _, err := e.SketchAutoBind(false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if s, p := tlBinds(r); s != 0 || p != 0 || len(r.Notes) == 0 || r.Notes[0].Code != "sync-source" {
		t.Fatalf("without a header: %+v", r)
	}
	// the user confirms the header
	r, v, err := e.SketchAutoBind(true, nil, "demo:board/argb1")
	if err != nil || v == nil {
		t.Fatalf("apply: %v %v", v, err)
	}
	if s, p := tlBinds(r); s != 4 || p != 0 {
		t.Fatalf("bindings %+v", r.Binds)
	}
	tl, _ := demoDevice(v.Lighting, demoFansID)
	if tl.SettingValues["syncSource"] != "" || tl.SettingValues.String("mode") != "motherboard" {
		t.Fatalf("controller changed: %+v", tl.SettingValues)
	}
	z := zoneView(t, v.Lighting, "demo:board", "argb1")
	if z.LEDs != 78 || len(z.Points) != 78 {
		t.Fatalf("argb1: %d LEDs, %d points", z.LEDs, len(z.Points))
	}
	// LED i is LED i%26 of side fan i/26, where that fan is on the canvas
	for f, id := range []string{"s1", "s2", "s3"} {
		var own []leds.Point
		for _, seg := range z.Segments {
			if seg.Component == sketch.ComponentName(id) {
				one := seg
				one.Start = nil
				own = leds.Positions(leds.Zone{Resizable: true}, leds.ZoneConfig{Segments: []leds.Segment{one}})
			}
		}
		if len(own) != 26 {
			t.Fatalf("%s: %d LEDs", id, len(own))
		}
		for k := 0; k < 26; k++ {
			p := z.Points[f*26+k]
			if math.Abs(p[0]-round3(own[k].X)) > 1e-9 || math.Abs(p[1]-round3(own[k].Y)) > 1e-9 {
				t.Fatalf("argb1 LED %d at %v, %s LED %d at %v", f*26+k, p, id, k, own[k])
			}
		}
		// the same columns on every side fan: a wave from left to right
		if f > 0 {
			for k := 0; k < 26; k++ {
				if z.Points[f*26+k][0] != z.Points[k][0] {
					t.Fatalf("%s LED %d in column %v, fan 1 in %v", id, k, z.Points[f*26+k][0], z.Points[k][0])
				}
			}
		}
	}
	for _, seg := range z.Segments {
		if seg.Component == sketch.ComponentName("rear") && (seg.Start == nil || *seg.Start != 0) {
			t.Fatalf("rear fan not parallel to fan 1: %+v", seg)
		}
	}

	// with the header set on the controller it is used without asking
	if _, err := e.LightingSetSync(demoFansID, "demo:board/argb2"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetSketch(sk); err != nil {
		t.Fatal(err)
	}
	r, _, err = e.SketchAutoBind(false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := tlBinds(r); s != 4 || r.Sync == nil || r.Sync.Zone != "argb2" || r.Sync.Derived {
		t.Fatalf("with the controller's header: %+v", r)
	}

	// canvas mode: the ports as before
	if _, err := e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{demoFansID: {Settings: map[string]any{"mode": "canvas", "allowFlashWrites": true}}}}); err != nil {
		t.Fatal(err)
	}
	r, _, err = e.SketchAutoBind(false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if s, p := tlBinds(r); s != 0 || p != 4 || r.Sync != nil {
		t.Fatalf("canvas mode: %+v", r)
	}
}

// The orientation check shows its pattern on the bound part's zone through
// the calibration path; a report corrects the stored part and shows the
// pattern again. A part in parallel shows nothing of its own, a part that
// is not connected has no check.
func TestLightingOrientDemo(t *testing.T) {
	e := startLit(t)
	withTLParts(t, e)
	fan := func(id, facing, label string, x, y, z float64) sketch.Placement {
		return sketch.Placement{ID: id, Part: "uni-fan-tl-lcd-reverse-120", Facing: facing, Pos: sketch.Vec3{X: x, Y: y, Z: z}, Label: label}
	}
	s1 := fan("s1", sketch.FacingSide, "Seite 1", 14, 112, 300)
	s1.Bind = &sketch.Binding{Device: "demo:board", Zone: "argb1"}
	rear := fan("rear", sketch.FacingRear, "Rear", 60, 90, 14)
	rear.Bind = &sketch.Binding{Device: "demo:board", Zone: "argb1", Order: 1, Parallel: true}
	sk := testSketch(s1, rear, fan("s2", sketch.FacingSide, "Seite 2", 14, 236, 300))
	if _, err := e.SetSketch(sk); err != nil {
		t.Fatal(err)
	}
	p, err := e.LightingPartPattern("s1")
	if err != nil {
		t.Fatal(err)
	}
	c := e.Lighting().Calibrating
	if p.Kind != sketch.PatternFan || p.Start != 0 || len(p.Colors) != 26 || c == nil || c.Part != "s1" || c.Zone != "argb1" || len(c.Pattern) != 26 || c.Pattern[0] != sketch.FirstLEDColor {
		t.Fatalf("pattern %+v, calibrating %+v", p, c)
	}
	v, p2, err := e.LightingOrient("s1", sketch.FixRot180)
	if err != nil {
		t.Fatal(err)
	}
	var got sketch.Placement
	for _, q := range v.Sketch.Parts {
		if q.ID == "s1" {
			got = q
		}
	}
	if got.Spin != 180 || got.Mirror || got.Bind == nil || got.Bind.Zone != "argb1" {
		t.Fatalf("corrected part %+v", got)
	}
	if st := e.GetSketch(); st.Parts[0].Spin != 180 {
		t.Fatalf("not stored: %+v", st.Parts[0])
	}
	if c := e.Lighting().Calibrating; c == nil || c.Part != "s1" || p2.Colors[0] != sketch.FirstLEDColor {
		t.Fatalf("pattern not shown again: %+v", c)
	}
	e.LightingEndCalibration()
	// in parallel: reported, nothing shown, not corrected
	p, err = e.LightingPartPattern("rear")
	if err != nil || !p.Parallel || p.Model != "s1" || e.Lighting().Calibrating != nil {
		t.Fatalf("parallel: %+v %v %+v", p, err, e.Lighting().Calibrating)
	}
	if _, _, err := e.LightingOrient("rear", sketch.FixRot90); err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Fatalf("parallel corrected: %v", err)
	}
	if _, err := e.LightingPartPattern("s2"); err == nil || !strings.Contains(err.Error(), "unbound") {
		t.Fatalf("unbound: %v", err)
	}
}
