package sketch

import (
	"fmt"
	"strings"
	"testing"
)

const goodCase = `{"format":1,"id":"my-case","vendor":"V","model":"M","dimensions_mm":{"w":285,"h":459,"d":465},"approximate":true,
 "views":{"side":{"name":"Glass","size_mm":{"w":465,"h":459},"outline":[
   {"type":"rect","x":0,"y":0,"w":465,"h":459,"r":10,"role":"body"},
   {"type":"polygon","points":[[0,0],[10,0],[10,5]],"role":"frame"}]}},
 "mounts":[{"id":"top","face":"top","view":"side","region":{"type":"rect","x":20,"y":4,"w":360,"h":87.5},
   "layouts":[{"fan_mm":120,"count":2,"snap_mm":[[80,16.5],[200,16.5]]}],"radiator_mm":[360],"slots":true}],
 "areas":[{"id":"gpu","kind":"gpu-bay","view":"side","shape":{"type":"circle","cx":5,"cy":5,"r":3}}],
 "gpu_slots":[{"id":"s1","view":"side","region":{"type":"rect","x":1,"y":1,"w":5,"h":5}}]}`

func TestParseCase(t *testing.T) {
	c, err := ParseCase([]byte(goodCase))
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "my-case" || c.Name != "V M" || c.Dims.D != 465 || len(c.Views["side"].Outline) != 2 ||
		len(c.Mounts) != 1 || len(c.Areas) != 1 || len(c.GPUSlots) != 1 || !c.Approximate {
		t.Fatalf("%+v", c)
	}
	cat := Builtin()
	if err := cat.AddCase(c); err != nil {
		t.Fatal(err)
	}
	if err := cat.AddCase(c); err == nil {
		t.Error("duplicate case accepted")
	}
	if err := cat.AddCase(CaseTemplate{ID: CustomCaseID}); err == nil {
		t.Error("custom box replaced")
	}
	if err := cat.AddCase(CaseTemplate{ID: "sketch.x"}); err == nil {
		t.Error("reserved prefix accepted")
	}
}

func TestParseCaseRejects(t *testing.T) {
	poly := func(n int) string {
		pts := make([]string, n)
		for i := range pts {
			pts[i] = "[1,2]"
		}
		return `{"type":"polygon","points":[` + strings.Join(pts, ",") + `]}`
	}
	withOutline := func(shape string) string {
		return fmt.Sprintf(`{"format":1,"id":"c","model":"M","dimensions_mm":{"w":1,"h":1,"d":1},"views":{"side":{"size_mm":{"w":10,"h":10},"outline":[%s]}}}`, shape)
	}
	bad := map[string]string{
		"format":      strings.Replace(goodCase, `"format":1`, `"format":2`, 1),
		"id upper":    strings.Replace(goodCase, `"id":"my-case"`, `"id":"My Case"`, 1),
		"id prefix":   strings.Replace(goodCase, `"id":"my-case"`, `"id":"sketch.a"`, 1),
		"dims zero":   strings.Replace(goodCase, `"w":285`, `"w":0`, 1),
		"dims huge":   strings.Replace(goodCase, `"h":459,"d"`, `"h":99999,"d"`, 1),
		"view name":   strings.Replace(goodCase, `"side":{"name"`, `"top":{"name"`, 1),
		"view size":   strings.Replace(goodCase, `"size_mm":{"w":465`, `"size_mm":{"w":-1`, 1),
		"face":        strings.Replace(goodCase, `"face":"top"`, `"face":"bogus"`, 1),
		"area kind":   strings.Replace(goodCase, `"gpu-bay"`, `"nope"`, 1),
		"vendor ctrl": strings.Replace(goodCase, `"vendor":"V"`, `"vendor":"V\u0007"`, 1),
		"snap huge":   strings.Replace(goodCase, `[80,16.5]`, `[8000,16.5]`, 1),
		"no layouts":  strings.Replace(goodCase, `"layouts":[{"fan_mm":120,"count":2,"snap_mm":[[80,16.5],[200,16.5]]}]`, `"layouts":[]`, 1),
		"poly 2 pts":  withOutline(poly(2)),
		"poly 257":    withOutline(poly(257)),
		"unknown":     withOutline(`{"type":"star"}`),
		"rect huge":   withOutline(`{"type":"rect","x":0,"y":0,"w":1e9,"h":5}`),
		"role":        withOutline(`{"type":"rect","x":0,"y":0,"w":5,"h":5,"role":"x"}`),
		"not json":    `{`,
	}
	for name, in := range bad {
		if _, err := ParseCase([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ParseCase([]byte(withOutline(poly(256)))); err != nil {
		t.Errorf("256 points refused: %v", err)
	}
	big := make([]byte, MaxBodyBytes+1)
	if _, err := ParseCase(big); err == nil {
		t.Error("oversized case accepted")
	}
}

func TestParsePartOutlineChecks(t *testing.T) {
	base := `{"format":1,"id":"p","vendor":"V","model":"M","category":"fan","dimensions_mm":{"w":120,"h":120,"d":25},
 "outline":{"front":[%s]},"leds":{"count":1,"positions_mm":[[10,10]]}}`
	if _, err := ParsePart([]byte(fmt.Sprintf(base, `{"type":"rect","x":0,"y":0,"w":120,"h":120}`))); err != nil {
		t.Fatal(err)
	}
	for _, sh := range []string{`{"type":"polygon","points":[[0,0],[1,1]]}`, `{"type":"rect","x":0,"y":0,"w":1e12,"h":1}`, `{"type":"blob"}`} {
		if _, err := ParsePart([]byte(fmt.Sprintf(base, sh))); err == nil {
			t.Errorf("accepted %s", sh)
		}
	}
}

func TestParseCaseOrientation(t *testing.T) {
	with := func(block string) string {
		return `{"format":1,"id":"o","model":"M","dimensions_mm":{"w":200,"h":400,"d":500},` + block + `"views":{"side":{"size_mm":{"w":500,"h":400},
 "outline":[{"type":"rect","x":10,"y":20,"w":100,"h":50},{"type":"circle","cx":30,"cy":40,"r":5},{"type":"polygon","points":[[0,0],[100,10],[50,50]]}]}},
 "mounts":[{"id":"front-mount","face":"front","view":"side","region":{"type":"rect","x":0,"y":10,"w":25,"h":300},
   "layouts":[{"fan_mm":120,"count":1,"snap_mm":[[12.5,100]]}]},
  {"id":"fv","face":"front","view":"front","region":{"type":"rect","x":0,"y":10,"w":25,"h":30},"layouts":[{"fan_mm":120,"count":1,"snap_mm":[[12.5,100]]}]}],
 "areas":[{"id":"mb","kind":"motherboard","view":"side","shape":{"type":"rect","x":100,"y":50,"w":300,"h":250}}],
 "gpu_slots":[{"id":"s1","view":"side","region":{"type":"rect","x":120,"y":100,"w":200,"h":60}}]}`
	}
	// no block: the default, nothing converted
	c, err := ParseCase([]byte(with("")))
	if err != nil || c.Orientation != (Orientation{}) || c.Mounts[0].Layouts[0].Snap[0][0] != 12.5 {
		t.Fatalf("no block: %v %+v", err, c.Orientation)
	}
	// explicit defaults are kept and change nothing
	c, err = ParseCase([]byte(with(`"orientation":{"side_view_left":"rear","glass_side":"left"},`)))
	if err != nil || c.Orientation != (Orientation{SideViewLeft: "rear", GlassSide: "left"}) || c.Mounts[0].Layouts[0].Snap[0][0] != 12.5 {
		t.Fatalf("defaults: %v %+v", err, c.Orientation)
	}
	if _, err := ParseCase([]byte(with(`"orientation":{"glass_side":"right"},`))); err != nil {
		t.Fatalf("partial block: %v", err)
	}
	// front on the left: side view data is measured from the rear afterwards (z = D - u)
	c, err = ParseCase([]byte(with(`"orientation":{"side_view_left":"front","glass_side":"right"},`)))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Orientation.FrontOnSideLeft() || !c.Orientation.GlassRight() {
		t.Fatalf("orientation %+v", c.Orientation)
	}
	m := c.Mounts[0]
	if m.Layouts[0].Snap[0] != [2]float64{487.5, 100} || m.Region.X != 475 || m.Region.W != 25 || m.Region.Y != 10 {
		t.Errorf("side mount not converted: %+v", m)
	}
	if c.Mounts[1].Layouts[0].Snap[0] != [2]float64{12.5, 100} || c.Mounts[1].Region.X != 0 {
		t.Errorf("front view mount changed: %+v", c.Mounts[1])
	}
	if a := c.Areas[0].Shape; a.X != 100 || a.W != 300 { // 500 - 100 - 300
		t.Errorf("area: %+v", a)
	}
	if g := c.GPUSlots[0].Region; g.X != 180 || g.W != 200 {
		t.Errorf("gpu slot: %+v", g)
	}
	out := c.Views["side"].Outline
	if out[0].X != 390 || out[1].CX != 470 || out[2].Points[1] != [2]float64{400, 10} {
		t.Errorf("outline: %+v", out)
	}
	// converting twice returns to the start, and the parsed input is not shared
	again, _ := ParseCase([]byte(with(`"orientation":{"side_view_left":"front"},`)))
	again.flipSideView()
	if again.Mounts[0].Layouts[0].Snap[0][0] != 12.5 {
		t.Errorf("not an involution: %v", again.Mounts[0].Layouts[0].Snap)
	}
	for name, block := range map[string]string{
		"side": `"orientation":{"side_view_left":"top"},`, "glass": `"orientation":{"glass_side":"middle"},`,
		"mixed case": `"orientation":{"side_view_left":"Rear"},`,
	} {
		if _, err := ParseCase([]byte(with(block))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
