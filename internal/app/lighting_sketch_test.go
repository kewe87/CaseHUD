package app

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

func testSketch(parts ...sketch.Placement) sketch.Sketch {
	return sketch.Sketch{Version: sketch.Version, Case: sketch.Case{Source: "custom", ID: sketch.CustomCaseID,
		Dims: sketch.Dims{W: 230, H: 480, D: 450}}, CanvasView: "side", Parts: parts}
}

func fanAt(id string, x float64, bind *sketch.Binding) sketch.Placement {
	return sketch.Placement{ID: id, Part: "generic-fan-120", View: "side", X: x, Y: 40, Bind: bind}
}

func bindTo(zone string, order int) *sketch.Binding {
	return &sketch.Binding{Device: "demo:board", Zone: zone, Order: order}
}

func zoneSegs(t *testing.T, v LightingView, dev, zone string) []leds.Segment {
	t.Helper()
	for _, d := range v.Devices {
		if d.ID != dev {
			continue
		}
		for _, z := range d.Zones {
			if z.ID == zone {
				return z.Segments
			}
		}
	}
	t.Fatalf("zone %s/%s not in the view", dev, zone)
	return nil
}

func dropSketchComponents() {
	for name := range sketchComponentNames() {
		leds.UnregisterComponent(name)
	}
}

func startLit(t *testing.T) *Engine {
	t.Helper()
	dropSketchComponents()
	t.Cleanup(dropSketchComponents)
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSketchRoundTripAndDelete(t *testing.T) {
	e := startLit(t)
	if e.GetSketch() != nil {
		t.Fatal("sketch before any PUT")
	}
	// A zone the sketch does not bind keeps its own layout.
	other := "argb2"
	if _, err := e.PatchLighting(patchDev("demo:board", other, LightingZonePatch{Count: ptr(12)})); err != nil {
		t.Fatal(err)
	}
	s := testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1)), fanAt("free", 10, nil))
	v, err := e.SetSketch(s)
	if err != nil {
		t.Fatal(err)
	}
	if v.Sketch == nil || len(v.Sketch.Parts) != 3 {
		t.Fatalf("sketch %+v", v.Sketch)
	}
	for _, n := range []string{"sketch.a", "sketch.b", "sketch.free"} {
		if _, ok := leds.LookupComponent(n); !ok {
			t.Errorf("component %s not registered", n)
		}
	}
	segs := zoneSegs(t, v.Lighting, "demo:board", "argb1")
	if len(segs) != 2 || segs[0].Component != "sketch.a" || segs[1].Component != "sketch.b" || segs[1].Start == nil || *segs[1].Start != 16 {
		t.Fatalf("segments %+v", segs)
	}
	if got := zoneSegs(t, v.Lighting, "demo:board", other); len(got) != 1 || got[0].Shape != "line" {
		t.Fatalf("unbound zone changed: %+v", got)
	}

	// Remove a placement: its component goes, the chain shrinks.
	v, err = e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0))))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := leds.LookupComponent("sketch.b"); ok {
		t.Error("component of the removed placement still registered")
	}
	if segs = zoneSegs(t, v.Lighting, "demo:board", "argb1"); len(segs) != 1 {
		t.Fatalf("segments after removal %+v", segs)
	}
	// Unbind the remaining placement: the zone falls back to its default layout.
	v, err = e.SetSketch(testSketch(fanAt("a", 10, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if segs = zoneSegs(t, v.Lighting, "demo:board", "argb1"); len(segs) != 0 {
		t.Fatalf("unbound zone keeps sketch segments: %+v", segs)
	}

	// Delete.
	if _, err = e.SetSketch(s); err != nil {
		t.Fatal(err)
	}
	dv, err := e.DeleteSketch()
	if err != nil {
		t.Fatal(err)
	}
	if dv.Sketch != nil || e.GetSketch() != nil || len(sketchComponentNames()) != 0 {
		t.Fatalf("sketch left: %+v %v", dv.Sketch, sketchComponentNames())
	}
	if segs = zoneSegs(t, dv.Lighting, "demo:board", "argb1"); len(segs) != 0 {
		t.Fatalf("zone keeps dangling segments: %+v", segs)
	}
	if got := zoneSegs(t, dv.Lighting, "demo:board", other); len(got) != 1 {
		t.Fatalf("unbound zone lost its layout: %+v", got)
	}
	if _, err := e.DeleteSketch(); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestSketchRejectedChangesNothing(t *testing.T) {
	e := startLit(t)
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]sketch.Sketch{
		"unknown part":   testSketch(sketch.Placement{ID: "x", Part: "nope", View: "side"}),
		"unknown device": testSketch(fanAt("a", 10, &sketch.Binding{Device: "nope", Zone: "argb1"}), fanAt("b", 100, nil)),
		"unknown zone":   testSketch(fanAt("a", 10, &sketch.Binding{Device: "demo:board", Zone: "nope"}), fanAt("b", 100, nil)),
		"nan":            testSketch(sketch.Placement{ID: "x", Part: "generic-fan-120", X: math.NaN()}),
	} {
		_, err := e.SetSketch(s)
		if err == nil || !strings.Contains(err.Error(), "sketch") {
			t.Errorf("%s: %v", name, err)
		}
		cur := e.GetSketch()
		if cur == nil || len(cur.Parts) != 1 || cur.Parts[0].ID != "a" {
			t.Fatalf("%s: stored sketch changed: %+v", name, cur)
		}
		if _, ok := leds.LookupComponent("sketch.a"); !ok {
			t.Fatalf("%s: component lost", name)
		}
		if _, ok := leds.LookupComponent("sketch.b"); ok {
			t.Fatalf("%s: component of the rejected sketch registered", name)
		}
		if segs := zoneSegs(t, e.Lighting(), "demo:board", "argb1"); len(segs) != 1 || segs[0].Component != "sketch.a" {
			t.Fatalf("%s: zone changed: %+v", name, segs)
		}
	}
	// Too many parts for one zone (at most 16 segments).
	var many []sketch.Placement
	for i := 0; i < 17; i++ {
		many = append(many, fanAt("p"+strings.Repeat("x", i), 10, bindTo("argb1", i)))
	}
	if _, err := e.SetSketch(testSketch(many...)); err == nil {
		t.Error("17 segments in one zone accepted")
	}
	if cur := e.GetSketch(); cur == nil || len(cur.Parts) != 1 {
		t.Fatal("stored sketch changed by the refused one")
	}
	if _, ok := leds.LookupComponent("sketch.p"); ok {
		t.Error("component of the refused sketch registered")
	}
}

// A restart keeps the component segments: the derived components exist
// before the stored layout is cleaned.
func TestSketchSurvivesRestart(t *testing.T) {
	e := startLit(t)
	dir := e.Store.Dir
	s := testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1)))
	if _, err := e.SetSketch(s); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	dropSketchComponents() // a new process starts without them

	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e2.Stop)
	if sk := e2.GetSketch(); sk == nil || len(sk.Parts) != 2 {
		t.Fatalf("sketch lost: %+v", sk)
	}
	for _, n := range []string{"sketch.a", "sketch.b"} {
		if _, ok := leds.LookupComponent(n); !ok {
			t.Errorf("component %s not registered after start", n)
		}
	}
	v := e2.Lighting()
	segs := zoneSegs(t, v, "demo:board", "argb1")
	if len(segs) != 2 || segs[0].Shape != "component" || segs[0].Component != "sketch.a" || segs[1].Component != "sketch.b" {
		t.Fatalf("segments after restart: %+v", segs)
	}
	if !v.Enabled || v.ConfigReset {
		t.Fatalf("lighting state after restart: enabled=%v reset=%v", v.Enabled, v.ConfigReset)
	}
}

// A stored sketch that cannot be derived any more (library part removed)
// must not take the lighting configuration down with it.
func TestStoredSketchWithMissingPartDoesNotBreakLighting(t *testing.T) {
	e := startLit(t)
	dir := e.Store.Dir
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	dropSketchComponents()
	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), `"part": "generic-fan-120"`, `"part": "gone-part"`, 1)
	if edited == string(b) {
		t.Fatal("test edit did not apply")
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e2.Stop)
	v := e2.Lighting()
	if !v.Enabled || v.ConfigReset {
		t.Fatalf("lighting configuration refused: enabled=%v reset=%v", v.Enabled, v.ConfigReset)
	}
	// The placement keeps a stand-in component: nothing is stripped from the
	// stored layout because the catalog lacks the part.
	if segs := zoneSegs(t, v, "demo:board", "argb1"); len(segs) != 1 || segs[0].Component != "sketch.a" {
		t.Fatalf("segment of the placement lost: %+v", segs)
	}
	if e2.GetSketch() == nil {
		t.Fatal("stored sketch dropped")
	}
	// The user can still replace it.
	if _, err := e2.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatal(err)
	}
}

func TestSketchWhileLightingIsOff(t *testing.T) {
	dropSketchComponents()
	t.Cleanup(dropSketchComponents)
	e := startDemo(t)
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatalf("no engine: bindings are kept unchecked: %v", err)
	}
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, &sketch.Binding{Device: "bad id!", Zone: "z"}))); err == nil {
		t.Error("malformed device id accepted")
	}
	if e.GetSketch() == nil {
		t.Fatal("sketch not stored")
	}
	v, err := e.PatchLighting(LightingPatch{Enabled: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if segs := zoneSegs(t, v, "demo:board", "argb1"); len(segs) != 1 || segs[0].Component != "sketch.a" {
		t.Fatalf("segments after enabling: %+v", segs)
	}
}
