package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

func withLEDs(p sketch.Placement, n int) sketch.Placement {
	p.LEDs = n
	return p
}

// The sketch limit of parts per zone is the limit the engine configuration
// check enforces: a sketch within it is never refused further down.
func TestSketchSegmentLimitMatchesEngine(t *testing.T) {
	if sketch.MaxZoneSegments != maxLightSegments {
		t.Fatalf("sketch allows %d parts per zone, the lighting configuration %d", sketch.MaxZoneSegments, maxLightSegments)
	}
}

// PUT and DELETE replace and remove components while layouts are rebuilt
// (the engine goroutine, GET /api/lighting, other zone changes). Run with
// -race: no panic, no data race, and a consistent end state.
func TestSketchConcurrentChangesAndLayoutRebuild(t *testing.T) {
	e := startLit(t)
	variants := []sketch.Sketch{
		testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1))),
		testSketch(withLEDs(fanAt("a", 10, bindTo("argb1", 0)), 8)),
		testSketch(fanAt("c", 10, bindTo("argb1", 0)), withLEDs(fanAt("b", 150, bindTo("argb1", 1)), 12)),
		testSketch(fanAt("free", 10, nil)),
	}
	stop := make(chan struct{})
	var readers, writers sync.WaitGroup
	for r := 0; r < 3; r++ {
		readers.Add(1)
		go func(r int) {
			defer readers.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				switch r {
				case 0, 1:
					v := e.Lighting() // Positions of every zone
					for _, d := range v.Devices {
						for _, z := range d.Zones {
							_ = leds.Positions(leds.Zone{ID: z.ID, Resizable: true, Max: 512}, leds.ZoneConfig{Segments: z.Segments})
						}
					}
				default:
					n := 8 + i%5 // an unrelated zone change rebuilds the layout of the device too
					if _, err := e.PatchLighting(patchDev("demo:board", "argb2", LightingZonePatch{Count: ptr(n)})); err != nil {
						t.Errorf("patch: %v", err)
						return
					}
				}
			}
		}(r)
	}
	for w := 0; w < 2; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := 0; i < 40; i++ {
				if i%5 == 4 {
					if _, err := e.DeleteSketch(); err != nil {
						t.Errorf("delete: %v", err)
						return
					}
					continue
				}
				if _, err := e.SetSketch(variants[(i+w)%len(variants)]); err != nil {
					t.Errorf("put: %v", err)
					return
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	// End state: the stored sketch, the registered components and the zone
	// segments agree.
	final, err := e.SetSketch(variants[2])
	if err != nil {
		t.Fatal(err)
	}
	segs := zoneSegs(t, final.Lighting, "demo:board", "argb1")
	if len(segs) != 2 {
		t.Fatalf("segments %+v", segs)
	}
	for _, s := range segs {
		if _, ok := leds.LookupComponent(s.Component); !ok {
			t.Errorf("segment names unregistered component %s", s.Component)
		}
	}
	if got := sketchComponentNames(); len(got) != 2 {
		t.Errorf("registered sketch components %v", got)
	}
}

// A refused sketch (too many parts in a zone) is a clear client error and is
// caught before any component is registered.
func TestSketchMoreThan16SegmentsPerZone(t *testing.T) {
	e := startLit(t)
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatal(err)
	}
	var many []sketch.Placement
	for i := 0; i < sketch.MaxZoneSegments+1; i++ {
		many = append(many, withLEDs(fanAt("p"+strings.Repeat("x", i), 10, bindTo("argb1", i)), 4))
	}
	_, err := e.SetSketch(testSketch(many...))
	if err == nil || !errors.Is(err, ErrLighting) || !strings.Contains(err.Error(), "more than 16 parts in zone demo:board/argb1") {
		t.Fatalf("17 parts: %v", err)
	}
	if got := sketchComponentNames(); len(got) != 1 || got["sketch.a"].Name == "" {
		t.Errorf("components after the refused sketch: %v", got)
	}
	// 16 are fine.
	if _, err := e.SetSketch(testSketch(many[:sketch.MaxZoneSegments]...)); err != nil {
		t.Fatalf("16 parts: %v", err)
	}
}

func readStoredSegments(t *testing.T, dir, dev, zone string) []leds.Segment {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Lighting struct {
			Engine *leds.Config `json:"engine"`
		} `json:"lighting"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil || cfg.Lighting.Engine == nil {
		t.Fatalf("config.json: %v", err)
	}
	return cfg.Lighting.Engine.Devices[dev].Zones[zone].Segments
}

// A stored sketch with one part missing from the catalog still derives its
// other placements; the missing one keeps its segment (as a placeholder),
// the neighbours do not move, and saving the lighting configuration does not
// write a stripped layout.
func TestStoredSketchWithOneMissingPartKeepsTheRest(t *testing.T) {
	e := startLit(t)
	dir := e.Store.Dir
	s := testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1)), fanAt("c", 290, bindTo("argb1", 2)))
	if _, err := e.SetSketch(s); err != nil {
		t.Fatal(err)
	}
	want := zoneSegs(t, e.Lighting(), "demo:board", "argb1")
	if len(want) != 3 {
		t.Fatalf("segments %+v", want)
	}
	e.Stop()
	dropSketchComponents()

	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const needle = `"part": "generic-fan-120"`
	first := strings.Index(string(b), needle)
	second := first + len(needle) + strings.Index(string(b)[first+len(needle):], needle)
	if first < 0 || second < first+len(needle) {
		t.Fatal("test edit did not apply")
	}
	edited := string(b)[:second] + `"part": "gone-part"` + string(b)[second+len(needle):]
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e2.Stop)
	t.Cleanup(dropSketchComponents)
	if c, ok := leds.LookupComponent("sketch.a"); !ok || len(c.LEDs) != 16 || c.SizeMM.X != 120 {
		t.Fatalf("component a not derived: %+v", c)
	}
	if _, ok := leds.LookupComponent("sketch.b"); !ok {
		t.Fatal("no stand-in for the placement with the missing part")
	}
	if c, ok := leds.LookupComponent("sketch.c"); !ok || len(c.LEDs) != 16 {
		t.Fatalf("component c not derived: %+v", c)
	}
	v := e2.Lighting()
	if !v.Enabled || v.ConfigReset {
		t.Fatalf("lighting configuration refused: enabled=%v reset=%v", v.Enabled, v.ConfigReset)
	}
	got := zoneSegs(t, v, "demo:board", "argb1")
	if len(got) != len(want) {
		t.Fatalf("segments after restore %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Component != want[i].Component || got[i].X != want[i].X || got[i].Y != want[i].Y ||
			got[i].Start == nil || *got[i].Start != *want[i].Start {
			t.Errorf("segment %d changed: %+v, was %+v", i, got[i], want[i])
		}
	}
	if sk := e2.GetSketch(); sk == nil || len(sk.Parts) != 3 || sk.Parts[1].Part != "gone-part" {
		t.Fatalf("stored sketch changed: %+v", sk)
	}
	// What gets written next still has all three segments.
	if err := e2.saveLightingErr(); err != nil {
		t.Fatal(err)
	}
	stored := readStoredSegments(t, dir, "demo:board", "argb1")
	if len(stored) != 3 || stored[1].Component != "sketch.b" || stored[2].Start == nil || *stored[2].Start != *want[2].Start {
		t.Fatalf("persisted segments %+v", stored)
	}
	// Replacing the sketch with a valid one derives everything again.
	if _, err := e2.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1)))); err != nil {
		t.Fatal(err)
	}
	if c, _ := leds.LookupComponent("sketch.b"); len(c.LEDs) != 16 {
		t.Errorf("placeholder not replaced: %d LEDs", len(c.LEDs))
	}
}

// When the whole catalog is unusable (no placement validates) every
// placement keeps a stand-in, so even then no segment is dropped.
func TestStoredSketchWithoutAnyPartKeepsItsSegments(t *testing.T) {
	e := startLit(t)
	dir := e.Store.Dir
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)), fanAt("b", 150, bindTo("argb1", 1)))); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	dropSketchComponents()
	path := filepath.Join(dir, "config.json")
	b, _ := os.ReadFile(path)
	edited := strings.ReplaceAll(string(b), `"part": "generic-fan-120"`, `"part": "gone-part"`)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e2.Stop)
	t.Cleanup(dropSketchComponents)
	if err := e2.saveLightingErr(); err != nil {
		t.Fatal(err)
	}
	if stored := readStoredSegments(t, dir, "demo:board", "argb1"); len(stored) != 2 {
		t.Fatalf("persisted segments %+v", stored)
	}
}

// If config.json cannot be written the request fails (not a silent 200) and
// the error says so; bad input stays a client error.
func TestSketchPersistFailure(t *testing.T) {
	e := startLit(t)
	path := filepath.Join(e.Store.Dir, "config.json")
	// A non-empty directory in the place of the file: the atomic rename fails.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0))))
	if !errors.Is(err, ErrPersist) || errors.Is(err, ErrLighting) {
		t.Fatalf("put: %v", err)
	}
	_, err = e.DeleteSketch()
	if !errors.Is(err, ErrPersist) || errors.Is(err, ErrLighting) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := e.SetSketch(testSketch(sketch.Placement{ID: "x", Part: "nope"})); !errors.Is(err, ErrLighting) {
		t.Fatalf("bad input: %v", err)
	}
	// Once the disk works again the next change succeeds.
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetSketch(testSketch(fanAt("a", 10, bindTo("argb1", 0)))); err != nil {
		t.Fatalf("after the disk recovered: %v", err)
	}
}

// Parts in the 50 mm margin around the view (the editor allows them) give
// segment boxes outside the canvas; they must be accepted by the lighting
// configuration check, not refused with "x, y, w and h must be between 0 and 1".
func TestSketchPartsInTheMarginAreAccepted(t *testing.T) {
	e := startLit(t)
	dims := sketch.Dims{W: 230, H: 480, D: 450}
	for _, cv := range []string{"front", "side", "top"} {
		for _, view := range []string{"front", "side", "top"} {
			vw, vh, _ := sketch.ViewSize(dims, view)
			for name, pos := range map[string][2]float64{
				"right":        {vw + sketch.MarginMM, 20},
				"bottom":       {20, vh + sketch.MarginMM},
				"top left":     {-sketch.MarginMM, -sketch.MarginMM},
				"bottom/right": {vw + sketch.MarginMM, vh + sketch.MarginMM},
			} {
				p := sketch.Placement{ID: "m", Part: "generic-fan-120", View: view, X: pos[0], Y: pos[1], LEDs: 8, Bind: bindTo("argb1", 0)}
				s := testSketch(p, sketch.Placement{ID: "w", Part: "generic-strip", View: view, X: pos[0], Y: pos[1], LEDs: 8, Len: 1000, Bind: bindTo("argb1", 1)})
				s.CanvasView = cv
				v, err := e.SetSketch(s)
				if err != nil {
					t.Fatalf("canvas %s, view %s, %s: %v", cv, view, name, err)
				}
				for _, sg := range zoneSegs(t, v.Lighting, "demo:board", "argb1") {
					for _, f := range []float64{sg.X, sg.Y, sg.W, sg.H} {
						if f < 0 || f > 1 {
							t.Fatalf("canvas %s, view %s, %s: segment %+v outside 0..1", cv, view, name, sg)
						}
					}
				}
			}
		}
	}
}
