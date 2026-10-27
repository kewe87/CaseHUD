package leds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLibrary(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "components", "x"), 0o755)
	os.WriteFile(filepath.Join(dir, "components.json"), []byte(`{"version":1,"components":[
		{"id":"bar-3","file":"components/x/bar-3.json"},{"id":"bad","file":"../evil.json"}]}`), 0o644)
	os.WriteFile(filepath.Join(dir, "components", "x", "bar-3.json"), []byte(`{"format":1,"id":"bar-3","vendor":"V","model":"Bar",
		"category":"strip","dimensions_mm":{"w":30,"h":10},
		"leds":{"count":3,"positions_mm":[[5,5],[15,5],[25,5]],"zones":[{"id":"a","first":0,"count":1},{"id":"b","first":1,"count":2}]}}`), 0o644)
	n, err := LoadLibrary(dir)
	if n != 1 || err == nil {
		t.Fatalf("loaded %d, err %v (the escaping path must be refused)", n, err)
	}
	c, ok := LookupComponent("bar-3")
	if !ok || c.Outline != "bar" || len(c.Zones) != 2 || c.LEDsMM()[2].X != 25 {
		t.Fatalf("component %+v", c)
	}
	// placed into a 0.3 x 0.1 box: LED 0 at x 0.05, LED 1 at 0.15
	p := Positions(Zone{ID: "z", Count: 3, Max: 3}, ZoneConfig{Segments: []Segment{{Shape: "component", Component: "bar-3", W: 0.3, H: 0.1}}})
	if len(p) != 3 || p[0].X < 0.049 || p[0].X > 0.051 || p[1].X < 0.149 || p[1].X > 0.151 {
		t.Fatalf("positions %v", p)
	}
}

func TestParseLibraryComponentRejects(t *testing.T) {
	part := func(id, model, dims, pos, zones string) []byte {
		return []byte(`{"format":1,"id":"` + id + `","vendor":"V","model":"` + model + `","category":"strip",
			"dimensions_mm":` + dims + `,"leds":{"count":2,"positions_mm":` + pos + `,"zones":` + zones + `}}`)
	}
	ok := part("p-1", "M", `{"w":20,"h":10}`, `[[5,5],[15,5]]`, `[{"id":"a","first":0,"count":2}]`)
	if _, err := ParseLibraryComponent(ok); err != nil {
		t.Fatalf("valid part: %v", err)
	}
	for name, data := range map[string][]byte{
		"id":         part("../x", "M", `{"w":20,"h":10}`, `[[5,5],[15,5]]`, `[]`),
		"label":      part("p-1", `M\u0007`, `{"w":20,"h":10}`, `[[5,5],[15,5]]`, `[]`),
		"huge":       part("p-1", "M", `{"w":1e9,"h":10}`, `[[5,5],[15,5]]`, `[]`),
		"outside":    part("p-1", "M", `{"w":20,"h":10}`, `[[5,5],[25,5]]`, `[]`),
		"negative":   part("p-1", "M", `{"w":20,"h":10}`, `[[-1,5],[15,5]]`, `[]`),
		"zone id":    part("p-1", "M", `{"w":20,"h":10}`, `[[5,5],[15,5]]`, `[{"id":"A B","first":0,"count":2}]`),
		"zone count": part("p-1", "M", `{"w":20,"h":10}`, `[[5,5],[15,5]]`, `[{"id":"a","first":0,"count":3}]`),
	} {
		if _, err := ParseLibraryComponent(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLoadLibrarySizeLimit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "components.json"), make([]byte, maxLibraryFile+1), 0o644)
	if _, err := LoadLibrary(dir); err == nil {
		t.Fatal("oversized index accepted")
	}
}

// The real library, if checked out next to this repository.
func TestLoadRealLibrary(t *testing.T) {
	dir := filepath.Join("..", "casehud-designs")
	if _, err := os.Stat(filepath.Join(dir, "components.json")); err != nil {
		t.Skip("library not present")
	}
	n, err := LoadLibrary(dir)
	if n == 0 {
		t.Fatalf("nothing loaded: %v", err)
	}
	t.Logf("loaded %d parts (%v)", n, err)
}
