package app

import (
	"context"
	"encoding/json"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
	"math"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func zoneOf(v LightingView, dev, zone string) LightingZoneView {
	d, _ := demoDevice(v, dev)
	for _, z := range d.Zones {
		if z.ID == zone {
			return z
		}
	}
	return LightingZoneView{}
}

func TestChainCount(t *testing.T) {
	if chainCount([]int{26, 26, 26}, false) != 78 || chainCount([]int{18, 11, 8}, true) != 18 || chainCount([]int{18, 11, 8}, false) != 37 || chainCount(nil, true) != 0 {
		t.Fatal("chain count")
	}
}

func TestLightingSetupNeedsLighting(t *testing.T) {
	e := startDemo(t) // lighting stays off
	if err := e.LightingAssist("demo:board", "argb1", 5); err == nil {
		t.Fatal("assistant while lighting is off")
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 5, nil, false); err == nil {
		t.Fatal("zone setup while lighting is off")
	}
	if _, err := e.ApplyLightingTemplate("asus-x870e-o11-dynamic-evo", "de"); err == nil {
		t.Fatal("template while lighting is off")
	}
	if len(e.LightingTemplates()) != 0 || e.Lighting().Calibrating != nil {
		t.Fatal("templates or marker while off")
	}
	e.saveLighting()
	if cfg, _ := e.Store.LoadConfig(); cfg.Lighting.Engine != nil && len(cfg.Lighting.Engine.Devices) != 0 {
		t.Fatalf("configuration written: %+v", cfg.Lighting.Engine.Devices)
	}
}

func TestLightingAssistStateMachine(t *testing.T) {
	e := enableDemoLighting(t)
	if err := e.LightingAssist("demo:board", "argb1", 7); err != nil {
		t.Fatal(err)
	}
	c := e.Lighting().Calibrating
	if c == nil || !c.Range || c.Pos != 7 || c.Zone != "argb1" {
		t.Fatalf("calibrating: %+v", c)
	}
	for _, bad := range []struct {
		dev, zone string
		n         int
	}{{"demo:board", "argb1", 121}, {"demo:board", "argb1", 0}, {"demo:board", "onboard", 3}, {"demo:board", "nozone", 1}, {"../x", "argb1", 1}} {
		if err := e.LightingAssist(bad.dev, bad.zone, bad.n); err == nil {
			t.Errorf("assist %+v accepted", bad)
		}
	}
	// nothing was stored while the assistant ran
	e.saveLighting()
	if z := zoneOf(e.Lighting(), "demo:board", "argb1"); z.LEDs != 0 || z.Placed {
		t.Fatalf("assistant changed the zone: %+v", z)
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 12, nil, false); err != nil {
		t.Fatal(err)
	}
	v := e.Lighting()
	if v.Calibrating != nil {
		t.Fatal("accepting must end the marker")
	}
	z := zoneOf(v, "demo:board", "argb1")
	if z.Count != 12 || z.LEDs != 12 || len(z.Segments) != 1 || z.Segments[0].Shape != "line" {
		t.Fatalf("count not reflected: %+v", z)
	}
	// raising the count keeps the line geometry
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 20, nil, false); err != nil {
		t.Fatal(err)
	}
	if z := zoneOf(e.Lighting(), "demo:board", "argb1"); z.Count != 20 || z.Segments[0].X != 0.05 {
		t.Fatalf("%+v", z)
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 121, nil, false); err == nil {
		t.Fatal("count over the hardware limit")
	}
	if _, err := e.LightingZoneSetup("demo:board", "onboard", 3, nil, false); err == nil {
		t.Fatal("fixed zone")
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 0, nil, false); err != nil {
		t.Fatal(err)
	}
	if z := zoneOf(e.Lighting(), "demo:board", "argb1"); z.Count != 0 || z.Placed {
		t.Fatalf("back to zero: %+v", z)
	}
}

func TestLightingZoneParts(t *testing.T) {
	e := enableDemoLighting(t)
	v, err := e.LightingZoneSetup("demo:board", "argb1", 0, []string{"fan-tl-26", "fan-tl-26", "fan-tl-26"}, false)
	if err != nil {
		t.Fatal(err)
	}
	z := zoneOf(v, "demo:board", "argb1")
	if z.LEDs != 78 || z.Count != 78 || len(z.Segments) != 3 || z.Segments[0].Start != nil {
		t.Fatalf("chain: %+v", z)
	}
	// parallel branches: the longest part counts
	leds.RegisterComponent(leds.Component{Name: "test-short", Label: "Short", Width: 8, Height: 1, LEDs: make([]leds.Point, 8)})
	v, err = e.LightingZoneSetup("demo:board", "argb2", 0, []string{"strip-47", "test-short", "fan-tl-26"}, true)
	if err != nil {
		t.Fatal(err)
	}
	z = zoneOf(v, "demo:board", "argb2")
	if z.LEDs != 47 || len(z.Segments) != 3 || z.Segments[1].Start == nil || len(z.Echo) != 8+26 {
		t.Fatalf("parallel: leds=%d segs=%d echo=%d", z.LEDs, len(z.Segments), len(z.Echo))
	}
	// one part with the parallel flag is just a chain of one
	v, _ = e.LightingZoneSetup("demo:board", "argb2", 0, []string{"strip-47"}, true)
	if z = zoneOf(v, "demo:board", "argb2"); z.LEDs != 47 || z.Segments[0].Start != nil {
		t.Fatalf("single part: %+v", z)
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 0, []string{"nope"}, false); err == nil {
		t.Fatal("unknown part")
	}
	over := []string{"strip-47", "strip-47", "strip-47"} // 141 > 120
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 0, over, false); err == nil {
		t.Fatal("more LEDs than the header takes")
	}
	if _, err := e.LightingZoneSetup("demo:board", "argb1", 0, make([]string, maxChainParts+1), false); err == nil {
		t.Fatal("too many parts")
	}
	if _, err := cleanLightingConfig(e.light.eng.Config(), nil); err != nil {
		t.Fatal(err)
	}
	e.saveLighting()
	cfg, _ := e.Store.LoadConfig()
	if cfg.Lighting.Engine == nil || len(cfg.Lighting.Engine.Devices["demo:board"].Zones["argb1"].Segments) != 3 {
		t.Fatalf("not stored: %+v", cfg.Lighting.Engine)
	}
}

func TestLightingTemplates(t *testing.T) {
	tpls := loadLightingTemplates()
	if len(tpls) == 0 || tpls[0].ID != "asus-x870e-o11-dynamic-evo" {
		t.Fatalf("embedded templates: %+v", tpls)
	}
	byZone := map[string]LightingTemplateZone{}
	for _, z := range tpls[0].Zones {
		byZone[z.Zone] = z
	}
	// the library parts are registered with their LED counts
	for name, n := range map[string]int{"core-distro-plate-o11-dynamic-evo-apex-vpp": 18, "core-geforce-rtx-5080-zotac-solid-amp": 11,
		"apex-1-cpu-block-am5-argb": 8, "o11-dynamic-evo-argb-strip": 47} {
		if _, ok := leds.LookupComponent(name); !ok {
			leds.RegisterComponent(leds.Component{Name: name, Label: name, Width: n, Height: 1, LEDs: make([]leds.Point, n)})
		}
	}
	if byZone["argb1"].leds() != 78 || byZone["argb2"].leds() != 47 || byZone["argb3"].leds() != 18 || byZone["ch4"].leds() != 9 || byZone["ring"].leds() != 60 {
		t.Fatalf("LED counts: %d %d %d %d %d", byZone["argb1"].leds(), byZone["argb2"].leds(), byZone["argb3"].leds(), byZone["ch4"].leds(), byZone["ring"].leds())
	}
	if len(byZone["argb3"].Segments) != 3 {
		t.Fatal("splitter: three separate parts")
	}
	for _, s := range byZone["argb3"].Segments {
		if s.Start == nil || *s.Start != 0 || s.Shape != "component" {
			t.Fatalf("splitter part must be parallel: %+v", s)
		}
	}
	for _, tp := range tpls {
		for _, z := range tp.Zones {
			if len(z.Segments) == 0 {
				continue
			}
			if _, err := cleanSegments(z.Segments); err != nil {
				t.Errorf("%s/%s: %v", tp.ID, z.Zone, err)
			}
		}
	}
}

func TestApplyTemplateOnDemo(t *testing.T) {
	e := enableDemoLighting(t)
	list := e.LightingTemplates()
	if len(list) != 1 || list[0].ID != "asus-x870e-o11-dynamic-evo" || !list[0].TLSync {
		t.Fatalf("matching templates: %+v", list)
	}
	for _, z := range list[0].Zones {
		if z.Zone == "onboard" || z.Zone == "argb3" {
			t.Errorf("zone %s is not in the demo or fixed without layout", z.Zone)
		}
	}
	if _, err := e.ApplyLightingTemplate("nope", "de"); err == nil {
		t.Fatal("unknown template")
	}
	res, err := e.ApplyLightingTemplate("asus-x870e-o11-dynamic-evo", "de")
	if err != nil {
		t.Fatal(err)
	}
	if z := zoneOf(res.View, "demo:board", "argb1"); z.LEDs != 78 || len(z.Segments) != 3 {
		t.Fatalf("argb1: %+v", z)
	}
	if z := zoneOf(res.View, "demo:board", "argb2"); z.LEDs != 47 {
		t.Fatalf("argb2: %+v", z)
	}
	tl, _ := demoDevice(res.View, "demo:fans")
	if tl.SettingValues["syncSource"] != "demo:board/argb1" {
		t.Fatalf("fan controller not following argb1: %v", tl.SettingValues)
	}
	skipped := strings.Join(res.Skipped, "|")
	if !strings.Contains(skipped, "ARGB 3") || !strings.Contains(skipped, "LED-Ring") {
		t.Fatalf("missing zones must be reported: %v", res.Skipped)
	}
	if _, err := cleanLightingConfig(e.light.eng.Config(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestZoneCountMirrorsConfiguration(t *testing.T) {
	e := enableDemoLighting(t)
	v, err := e.PatchLighting(patchDev("demo:board", "argb2", LightingZonePatch{Count: ptr(47)}))
	if err != nil {
		t.Fatal(err)
	}
	if z := zoneOf(v, "demo:board", "argb2"); z.Count != 47 || z.LEDs != 47 {
		t.Fatalf("count must equal the configured number: %+v", z)
	}
	if z := zoneOf(v, "demo:board", "onboard"); z.Count != 8 {
		t.Fatalf("fixed zone keeps the hardware count: %+v", z)
	}
	e.saveLighting()
	cfg, _ := e.Store.LoadConfig()
	if cfg.Lighting.Engine == nil || len(cfg.Lighting.Engine.Devices["demo:board"].Zones["argb2"].Segments) != 1 {
		t.Fatalf("devices not stored: %+v", cfg.Lighting.Engine)
	}
}

func TestUntrustedBridgeIsTranslated(t *testing.T) {
	inner := &stubBackend{name: "enram", err: enram.ErrUntrusted}
	var notes ramNotes
	g := newRAMBackend(inner, func() bool { return true }, &notes)
	_, err := g.Discover(context.Background())
	if err == nil || !strings.Contains(err.Error(), "vertrauenswürdig") || strings.Contains(err.Error(), "not trusted") {
		t.Fatalf("error: %v", err)
	}
}

func TestZoneLabels(t *testing.T) {
	e := enableDemoLighting(t)
	set := func(l string) (LightingView, error) {
		return e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{
			"demo:board": {Zones: map[string]LightingZonePatch{"argb1": {Label: &l}}}}})
	}
	v, err := set("  Lüfter (Sync) ")
	if err != nil {
		t.Fatal(err)
	}
	z := zoneOf(v, "demo:board", "argb1")
	if z.Label != "Lüfter (Sync)" || z.Name != "ARGB header 1" {
		t.Fatalf("label: %+v", z)
	}
	for _, bad := range []string{strings.Repeat("x", 41), "a\nb", "a\x00b", "\xff\xfe"} {
		if _, err := set(bad); err == nil {
			t.Errorf("label %q accepted", bad)
		}
	}
	if z := zoneOf(e.Lighting(), "demo:board", "argb1"); z.Label != "Lüfter (Sync)" {
		t.Fatal("a rejected request changed the label")
	}
	if _, err := e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{
		"demo:board": {Zones: map[string]LightingZonePatch{"nozone": {Label: ptr("x")}}}}}); err == nil {
		t.Fatal("label for an unknown zone")
	}
	e.saveLighting()
	cfg, _ := e.Store.LoadConfig()
	if cfg.Lighting.Labels["demo:board"]["argb1"] != "Lüfter (Sync)" {
		t.Fatalf("not stored: %+v", cfg.Lighting.Labels)
	}
	// diagnostics never carry the names
	b, _ := json.Marshal(e.sanitizedConfig())
	if strings.Contains(string(b), "Lüfter") {
		t.Fatalf("name in the diagnostics: %s", b)
	}
	v, _ = set("")
	if zoneOf(v, "demo:board", "argb1").Label != "" {
		t.Fatal("label not removed")
	}
	if got := cleanStoredLabels(map[string]map[string]string{"demo:board": {"argb1": "ok", "argb2": "a\nb"}, "../x": {"a": "b"}}); len(got) != 1 || got["demo:board"]["argb1"] != "ok" || len(got["demo:board"]) != 1 {
		t.Fatalf("stored labels: %+v", got)
	}
}

func TestTemplateSetsNames(t *testing.T) {
	e := enableDemoLighting(t)
	res, err := e.ApplyLightingTemplate("asus-x870e-o11-dynamic-evo", "en")
	if err != nil {
		t.Fatal(err)
	}
	if zoneOf(res.View, "demo:board", "argb1").Label != "Fans (sync)" || zoneOf(res.View, "demo:board", "argb2").Label != "Case strip" {
		t.Fatalf("names: %q %q", zoneOf(res.View, "demo:board", "argb1").Label, zoneOf(res.View, "demo:board", "argb2").Label)
	}
}

func TestChainSegmentsClampsLibrarySizes(t *testing.T) {
	inf := math.Inf(1)
	parts := []leds.Component{
		{Name: "a", SizeMM: leds.Point{X: inf, Y: 50}, LEDs: make([]leds.Point, 4)},
		{Name: "b", SizeMM: leds.Point{X: 1e12, Y: 1e12}, LEDs: make([]leds.Point, 4)},
		{Name: "c", SizeMM: leds.Point{X: math.NaN(), Y: 10}, LEDs: make([]leds.Point, 4)},
		{Name: "d", SizeMM: leds.Point{X: 120, Y: 120}, LEDs: make([]leds.Point, 4)},
	}
	for _, s := range chainSegments(parts) {
		for _, v := range []float64{s.X, s.Y, s.W, s.H} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1.01 {
				t.Fatalf("segment %s has %v", s.Name, v)
			}
		}
	}
}
