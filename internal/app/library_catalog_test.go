package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/sketch"
)

const catPart = `{"format":1,"id":"my-fan","vendor":"V","model":"Fan","category":"fan","dimensions_mm":{"w":120,"h":120,"d":25},
 "leds":{"count":2,"positions_mm":[[10,10],[100,100]]}}`

const catCase = `{"format":1,"id":"my-case","vendor":"V","model":"M","dimensions_mm":{"w":285,"h":459,"d":465},
 "views":{"side":{"size_mm":{"w":465,"h":459},"outline":[{"type":"rect","x":0,"y":0,"w":465,"h":459}]}}}`

func writeCache(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLibraryCatalogLoads(t *testing.T) {
	dir := writeCache(t, map[string]string{
		"components.json": `{"version":1,"components":[
			{"id":"my-fan","file":"components/my-fan.json"},
			{"id":"ctl","file":"components/ctl.json"},
			{"id":"generic-fan-120","file":"components/my-fan.json"},
			{"id":"sketch.x","file":"components/my-fan.json"},
			{"id":"evil","file":"../x.json"},
			{"id":"missing","file":"components/missing.json"},
			{"id":"wrong","file":"components/my-fan.json"}]}`,
		"components/my-fan.json": catPart,
		"components/ctl.json":    `{"format":1,"id":"ctl","category":"controller","dimensions_mm":{"w":50,"h":50},"leds":{"count":0}}`,
		"cases.json":             `{"version":1,"cases":[{"id":"my-case","file":"cases/my-case.json"},{"id":"bad","file":"cases/bad.json"}]}`,
		"cases/my-case.json":     catCase,
		"cases/bad.json":         `{"format":1,"id":"bad","model":"M","dimensions_mm":{"w":1,"h":1,"d":1},"views":{"side":{"size_mm":{"w":1,"h":1},"outline":[{"type":"polygon","points":[[0,0]]}]}}}`,
	})
	cat, skipped := loadLibraryCatalog(dir)
	if _, ok := cat.Parts["my-fan"]; !ok {
		t.Fatal("library part missing")
	}
	if _, ok := cat.Parts["generic-fan-120"]; !ok {
		t.Fatal("built-in part lost")
	}
	if _, ok := cat.Parts["ctl"]; ok {
		t.Error("part without LEDs in the catalog")
	}
	if c, ok := cat.Cases["my-case"]; !ok || len(c.Views["side"].Outline) != 1 {
		t.Fatalf("case missing: %+v", cat.Cases)
	}
	if _, ok := cat.Cases["bad"]; ok {
		t.Error("bad case accepted")
	}
	if _, ok := cat.Cases[sketch.CustomCaseID]; !ok {
		t.Error("custom box lost")
	}
	// built-in duplicate, reserved prefix, traversal, missing file, wrong id, bad case
	if len(skipped) != 6 {
		t.Fatalf("skipped %d: %v", len(skipped), skipped)
	}
	for id := range cat.Parts {
		if strings.HasPrefix(id, sketch.ReservedPrefix) {
			t.Errorf("reserved id %q in the catalog", id)
		}
	}
	// a sketch with a library part validates against it
	s := sketch.Sketch{Version: 1, Case: sketch.Case{Source: "library", ID: "my-case", Dims: sketch.Dims{W: 285, H: 459, D: 465}},
		Parts: []sketch.Placement{{ID: "a", Part: "my-fan", View: "side"}}}
	if err := sketch.Validate(s, cat); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryCatalogPartialCache(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"empty":      {},
		"only cases": {"cases.json": `{"version":1,"cases":[{"id":"my-case","file":"cases/my-case.json"}]}`, "cases/my-case.json": catCase},
		"only parts": {"components.json": `{"version":1,"components":[{"id":"my-fan","file":"components/my-fan.json"}]}`, "components/my-fan.json": catPart},
		"bad index":  {"components.json": `{nope`, "cases.json": `{"version":9}`},
	} {
		cat, _ := loadLibraryCatalog(writeCache(t, files))
		if _, ok := cat.Parts["generic-fan-120"]; !ok {
			t.Errorf("%s: built-ins missing", name)
		}
		if _, ok := cat.Cases[sketch.CustomCaseID]; !ok {
			t.Errorf("%s: custom box missing", name)
		}
	}
	cat, _ := loadLibraryCatalog(writeCache(t, map[string]string{"cases.json": `{"version":1,"cases":[{"id":"my-case","file":"cases/my-case.json"}]}`, "cases/my-case.json": catCase}))
	if _, ok := cat.Cases["my-case"]; !ok {
		t.Error("case lost without a component index")
	}
	cat, _ = loadLibraryCatalog(filepath.Join(t.TempDir(), "nonexistent"))
	if len(cat.Parts) == 0 {
		t.Error("no built-ins")
	}
}

func TestLibraryCatalogBudget(t *testing.T) {
	if b := defaultCatalogBudget; b.parts > 500 || b.cases > 100 || b.partBytes+b.caseBytes > 16<<20 {
		t.Fatalf("default budget too large: %+v", b)
	}
	partFile := func(id string) string { return strings.Replace(catPart, `"my-fan"`, `"`+id+`"`, 1) }
	caseFile := func(id string) string { return strings.Replace(catCase, `"my-case"`, `"`+id+`"`, 1) }
	files := map[string]string{}
	var pidx, cidx []string
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		files["components/"+id+".json"] = partFile(id)
		pidx = append(pidx, `{"id":"`+id+`","file":"components/`+id+`.json"}`)
	}
	for _, id := range []string{"c1", "c2", "c3", "c4"} {
		files["cases/"+id+".json"] = caseFile(id)
		cidx = append(cidx, `{"id":"`+id+`","file":"cases/`+id+`.json"}`)
	}
	files["components.json"] = `{"version":1,"components":[` + strings.Join(pidx, ",") + `]}`
	files["cases.json"] = `{"version":1,"cases":[` + strings.Join(cidx, ",") + `]}`
	dir := writeCache(t, files)
	big := 1 << 20

	// entry counts
	cat, skipped := loadLibraryCatalogWithin(dir, catalogBudget{parts: 3, cases: 2, partBytes: big, caseBytes: big})
	have := func(m []string) string { return strings.Join(m, ",") }
	var ps, cs []string
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		if _, ok := cat.Parts[id]; ok {
			ps = append(ps, id)
		}
	}
	for _, id := range []string{"c1", "c2", "c3", "c4"} {
		if _, ok := cat.Cases[id]; ok {
			cs = append(cs, id)
		}
	}
	if have(ps) != "p1,p2,p3" || have(cs) != "c1,c2" {
		t.Fatalf("loaded parts %v cases %v", ps, cs)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped %v", skipped)
	}

	// byte budget: two part files fit, the third (and everything behind it) does not
	cat, skipped = loadLibraryCatalogWithin(dir, catalogBudget{parts: 500, cases: 100, partBytes: 2*len(catPart) + 5, caseBytes: len(catCase) + 5})
	ps, cs = nil, nil
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		if _, ok := cat.Parts[id]; ok {
			ps = append(ps, id)
		}
	}
	for _, id := range []string{"c1", "c2", "c3", "c4"} {
		if _, ok := cat.Cases[id]; ok {
			cs = append(cs, id)
		}
	}
	if have(ps) != "p1,p2" || have(cs) != "c1" {
		t.Fatalf("loaded parts %v cases %v (skipped %v)", ps, cs, skipped)
	}
	if len(skipped) != 2 || !strings.Contains(skipped[0].Error(), "budget") || !strings.Contains(skipped[1].Error(), "budget") {
		t.Fatalf("skipped %v", skipped)
	}
	// the built-ins are never cut
	if _, ok := cat.Parts["generic-fan-120"]; !ok {
		t.Error("built-in part lost")
	}

	// the real default handles the same cache completely
	cat, skipped = loadLibraryCatalog(dir)
	if len(skipped) != 0 || len(cat.Parts) < 5 {
		t.Fatalf("default budget: %d parts, skipped %v", len(cat.Parts), skipped)
	}
}
