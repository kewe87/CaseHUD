package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// A sketch of the old format with one library part (2 LEDs, 120 mm) and one
// built-in part, both bound.
const v1WithLibraryPart = `{"version":1,"case":{"source":"custom","id":"custom-box","dims":{"w":230,"h":480,"d":450}},"canvasView":"side",
 "parts":[
 {"id":"a","part":"my-fan","view":"side","x":10,"y":40,"rot":0,"face":"","mirror":false,"flow":"intake","leds":0,"label":"Lib",
  "bind":{"device":"demo:board","zone":"argb1","order":0,"parallel":false}},
 {"id":"b","part":"generic-fan-120","view":"top","x":50,"y":25,"rot":90,"face":"","mirror":true,"flow":"","leds":0,"label":"",
  "bind":{"device":"demo:board","zone":"argb1","order":1,"parallel":false}}]}`

func writeTestLibrary(t *testing.T, dataDir string) {
	t.Helper()
	files := map[string]string{
		"components.json":        `{"version":1,"components":[{"id":"my-fan","file":"components/my-fan.json"}]}`,
		"components/my-fan.json": catPart,
	}
	for name, body := range files {
		p := filepath.Join(dataDir, "library", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func startIn(t *testing.T, dir string) *Engine {
	t.Helper()
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	e.Acknowledge("lighting")
	e.Acknowledge("experimental")
	return e
}

// replaceStoredSketch rewrites config.json so that lighting.sketch holds raw.
func replaceStoredSketch(t *testing.T, dir, raw string) {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	var light map[string]json.RawMessage
	if err := json.Unmarshal(cfg["lighting"], &light); err != nil {
		t.Fatal(err)
	}
	light["sketch"] = json.RawMessage(raw)
	delete(light, "sketch_v1")
	cfg["lighting"], _ = json.Marshal(light)
	out, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A stored sketch of the old format is upgraded with the full catalog at
// start: the result, the component and the segments are the same as from a
// PUT of the same document, and library parts are not approximated.
func TestStoredV1SketchGivesTheSameAsPut(t *testing.T) {
	dropSketchComponents()
	t.Cleanup(dropSketchComponents)

	// Reference: the old document sent to the editor endpoint.
	ref := t.TempDir()
	writeTestLibrary(t, ref)
	e1 := startIn(t, ref)
	if _, err := e1.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	put, notes, err := sketch.DecodeWithCatalog([]byte(v1WithLibraryPart), e1.SketchCatalog())
	if err != nil || len(notes) != 0 {
		t.Fatalf("%v %v", err, notes)
	}
	v, err := e1.SetSketch(put)
	if err != nil {
		t.Fatal(err)
	}
	wantSegs := zoneSegs(t, v.Lighting, "demo:board", "argb1")
	wantComp, ok := leds.LookupComponent("sketch.a")
	if !ok || len(wantSegs) != 2 || len(wantComp.LEDs) != 2 {
		t.Fatalf("reference: %v %+v %+v", ok, wantSegs, wantComp)
	}
	e1.Stop()
	dropSketchComponents()

	// The old document is what is stored.
	dir := t.TempDir()
	writeTestLibrary(t, dir)
	e2 := startIn(t, dir)
	if _, err := e2.PatchLighting(LightingPatch{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	e2.Stop()
	dropSketchComponents()
	replaceStoredSketch(t, dir, v1WithLibraryPart)

	e3 := startIn(t, dir)
	got := e3.GetSketch()
	if got == nil || got.Version != sketch.Version {
		t.Fatalf("stored sketch not upgraded: %+v", got)
	}
	if !reflect.DeepEqual(got, e1.cfg.Lighting.Sketch) {
		t.Fatalf("stored v1 gives\n%+v\nPUT gives\n%+v", got, e1.cfg.Lighting.Sketch)
	}
	// With the built-in catalog alone the library part would only be guessed.
	old, _ := sketch.DecodeV1([]byte(v1WithLibraryPart))
	approx, notes, _ := sketch.Upgrade(old, sketch.Builtin())
	if len(notes) == 0 || reflect.DeepEqual(approx.Parts[0], got.Parts[0]) {
		t.Fatalf("test does not tell the catalogs apart: %v", notes)
	}
	comp, ok := leds.LookupComponent("sketch.a")
	if !ok || !reflect.DeepEqual(comp, wantComp) {
		t.Fatalf("component after start: %v %+v want %+v", ok, comp, wantComp)
	}
	// The raw document stays as the backup, also when the configuration is written.
	e3.mu.Lock()
	hasBackup := len(e3.cfg.Lighting.SketchV1) > 0
	e3.mu.Unlock()
	if !hasBackup {
		t.Fatal("backup of the old sketch missing")
	}
	if err := e3.persist(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !strings.Contains(string(b), `"sketch_v1"`) || !strings.Contains(string(b), `"version": 2`) {
		t.Fatalf("config.json after persist:\n%.600s", b)
	}
	// The editor's first save derives the same segments and drops the backup.
	v, err = e3.SetSketch(*got)
	if err != nil {
		t.Fatal(err)
	}
	if segs := zoneSegs(t, v.Lighting, "demo:board", "argb1"); !reflect.DeepEqual(segs, wantSegs) {
		t.Fatalf("segments %+v want %+v", segs, wantSegs)
	}
	e3.mu.Lock()
	hasBackup = len(e3.cfg.Lighting.SketchV1) > 0
	e3.mu.Unlock()
	if hasBackup {
		t.Fatal("backup survived the first save")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "config.json"))
	if strings.Contains(string(b), `"sketch_v1"`) {
		t.Fatal("backup still in config.json")
	}
	// A restart now reads the saved version 2 as it is.
	e3.Stop()
	dropSketchComponents()
	e4 := startIn(t, dir)
	if !reflect.DeepEqual(e4.GetSketch(), got) {
		t.Fatalf("restart changed the sketch: %+v", e4.GetSketch())
	}
}

// Deleting the sketch also deletes the backup: it must not come back.
func TestDeleteSketchDropsBackup(t *testing.T) {
	dropSketchComponents()
	t.Cleanup(dropSketchComponents)
	dir := t.TempDir()
	e := startIn(t, dir)
	e.Stop()
	replaceStoredSketch(t, dir, v1WithLibraryPart)
	e = startIn(t, dir)
	if e.GetSketch() == nil {
		t.Fatal("not loaded")
	}
	if _, err := e.DeleteSketch(); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	dropSketchComponents()
	if e2 := startIn(t, dir); e2.GetSketch() != nil {
		t.Fatal("deleted sketch came back from the backup")
	}
}

func TestLightingLiveAndAreaOff(t *testing.T) {
	e := startLit(t)
	var live LightingLive
	deadline := time.Now().Add(5 * time.Second)
	for {
		var err error
		if live, err = e.LightingLive(false); err != nil {
			t.Fatal(err)
		}
		if len(live.Devices["demo:board"]["onboard"]) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	raw, err := base64.StdEncoding.DecodeString(live.Devices["demo:board"]["onboard"])
	if err != nil || len(raw) == 0 || len(raw)%3 != 0 || !live.Active {
		t.Fatalf("live: active=%v %d bytes err=%v all=%v", live.Active, len(raw), err, live.Devices)
	}
	for _, d := range e.Lighting().Devices {
		if d.ID != "demo:board" {
			continue
		}
		for _, z := range d.Zones {
			if z.ID == "onboard" && len(raw) != z.Count*3 {
				t.Errorf("onboard: %d bytes for %d LEDs", len(raw), z.Count)
			}
		}
	}

	// The lighting area off: both reads refuse; the sketch itself stays editable.
	if err := e.SetModules(Modules{Display: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.LightingLive(false); !errors.Is(err, ErrAreaOff) {
		t.Errorf("live: %v", err)
	}
	if _, err := e.SketchSuggest(); !errors.Is(err, ErrAreaOff) {
		t.Errorf("suggest: %v", err)
	}
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, nil))); err != nil {
		t.Errorf("sketch edit with the area off: %v", err)
	}
	if err := e.SetModules(Modules{Display: true, Lighting: true, Cooling: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.LightingLive(false); err != nil {
		t.Errorf("live after switching on: %v", err)
	}
}

func TestSketchSuggestDemo(t *testing.T) {
	e := startLit(t)
	s, err := e.SketchSuggest()
	if err != nil {
		t.Fatal(err)
	}
	if s.Parts == nil || s.Notes == nil {
		t.Fatal("lists must not be null")
	}
	t.Logf("%d parts, notes %+v", len(s.Parts), s.Notes)
	sk := testSketch(s.Parts...)
	if err := sketch.Validate(sk, e.SketchCatalog()); err != nil {
		t.Fatalf("suggestion does not validate: %v\n%+v", err, s)
	}
	if _, err := e.SetSketch(sk); err != nil {
		t.Fatalf("suggestion refused: %v", err)
	}
}

// The demo RAM: two modules drawn left to right go to stick 1 and stick 2;
// a second run finds nothing more to do.
func TestSketchAutoBindDemo(t *testing.T) {
	e := startLit(t)
	ram := func(id string, z float64) sketch.Placement {
		return sketch.Placement{ID: id, Part: "generic-ram-dimm", Facing: sketch.FacingSide, Spin: 90,
			Pos: sketch.Vec3{X: 220, Y: 100, Z: z}}
	}
	if _, err := e.SetSketch(testSketch(ram("right", 240), ram("left", 200))); err != nil {
		t.Fatal(err)
	}
	if r, v, err := e.SketchAutoBind(true, []string{"nothing"}, ""); err != nil || v != nil || len(r.Binds) != 0 || len(r.Open) != 2 {
		t.Fatalf("limited to other parts: %+v %v %v", r, v, err)
	}
	preview, v, err := e.SketchAutoBind(false, nil, "")
	if err != nil || v != nil {
		t.Fatalf("preview: %v %v", v, err)
	}
	if len(preview.Binds) != 2 || e.GetSketch().Parts[0].Bind != nil {
		t.Fatalf("preview stored or wrong: %+v", preview)
	}
	r, v, err := e.SketchAutoBind(true, nil, "")
	if err != nil || v == nil || v.Sketch == nil {
		t.Fatalf("apply: %v %v", v, err)
	}
	got := map[string]string{}
	for _, p := range e.GetSketch().Parts {
		if p.Bind != nil {
			got[p.ID] = p.Bind.Device + "/" + p.Bind.Zone
		}
	}
	if got["left"] != "demo:ram/stick1" || got["right"] != "demo:ram/stick2" {
		t.Fatalf("bindings %v (%+v)", got, r)
	}
	if again, v, err := e.SketchAutoBind(true, nil, ""); err != nil || v != nil || len(again.Binds) != 0 {
		t.Fatalf("second run: %+v %v %v", again, v, err)
	}
}

// A fan bound to a port of the fan controller takes one LED per ring, so
// the port's two LEDs per fan carry the whole fan.
func TestSketchRingZoneDemo(t *testing.T) {
	e := startLit(t)
	fan := sketch.Placement{ID: "f1", Part: "generic-fan-120", Facing: sketch.FacingSide,
		Pos: sketch.Vec3{X: 12.5, Y: 100, Z: 100}, Bind: &sketch.Binding{Device: demoFansID, Zone: "port2"}}
	if _, err := e.SetSketch(testSketch(fan)); err != nil {
		t.Fatal(err)
	}
	for _, d := range e.Lighting().Devices {
		for _, z := range d.Zones {
			if d.ID == demoFansID && z.ID == "port2" && z.LEDs != 2 {
				t.Fatalf("port2 drives %d LEDs, want 2 (%+v)", z.LEDs, z.Segments)
			}
		}
	}
	if !sketchRings("lianlitl:0416:7372", "port1") || sketchRings("demo:board", "port1") || sketchRings(demoFansID, "ring") {
		t.Fatal("sketchRings")
	}
}

// Identify lights one header as a whole; the view says so, and ending the
// calibration returns to the effect.
func TestLightingIdentifyDemo(t *testing.T) {
	e := startLit(t)
	if err := e.LightingIdentify("demo:board", "nope"); err == nil {
		t.Fatal("unknown zone accepted")
	}
	if err := e.LightingIdentify("demo:board", "argb1"); err != nil {
		t.Fatal(err)
	}
	c := e.Lighting().Calibrating
	if c == nil || !c.Identify || c.Device != "demo:board" || c.Zone != "argb1" {
		t.Fatalf("calibrating %+v", c)
	}
	e.LightingEndCalibration()
	if e.Lighting().Calibrating != nil {
		t.Fatal("still identifying")
	}
}
