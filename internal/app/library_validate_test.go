package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func vlPart(w, h float64, pos string, extra string) string {
	return `{"format":1,"id":"x","vendor":"V","model":"M","category":"fan","dimensions_mm":{"w":` + vlFtoa(w) + `,"h":` + vlFtoa(h) +
		`},"leds":{"count":1,"positions_mm":[` + pos + `]` + extra + `}}`
}

func vlFtoa(f float64) string { b, _ := json.Marshal(f); return string(b) }

func TestStageLibraryFiltersBadParts(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) { os.WriteFile(filepath.Join(dir, name), []byte(s), 0o600) }
	good := strings.Replace(vlPart(100, 100, "[10,10]", ""), `"id":"x"`, `"id":"vl-good"`, 1)
	write("good.json", good)
	write("far.json", strings.Replace(vlPart(100, 100, "[1e9,10]", ""), `"id":"x"`, `"id":"vl-far"`, 1))
	write("neg.json", strings.Replace(vlPart(100, 100, "[-5,10]", ""), `"id":"x"`, `"id":"vl-neg"`, 1))
	write("huge.json", strings.Replace(vlPart(100000, 100, "[10,10]", ""), `"id":"x"`, `"id":"vl-huge"`, 1))
	write("name.json", strings.Replace(strings.Replace(vlPart(100, 100, "[10,10]", ""), `"id":"x"`, `"id":"vl-name"`, 1), `"V"`, `"V\u0007bell"`, 1))
	write("zone.json", strings.Replace(vlPart(100, 100, "[10,10]", `,"zones":[{"id":"Bad Zone!","name":"z","first":0,"count":1}]`), `"id":"x"`, `"id":"vl-zone"`, 1))
	write("big.json", good+strings.Repeat(" ", libMaxPartBytes))
	write("components.json", `{"version":1,"components":[
	 {"id":"vl-good","file":"good.json"},{"id":"vl-far","file":"far.json"},{"id":"vl-neg","file":"neg.json"},
	 {"id":"vl-huge","file":"huge.json"},{"id":"vl-name","file":"name.json"},{"id":"vl-zone","file":"zone.json"},
	 {"id":"vl-big","file":"big.json"},{"id":"vl-esc","file":"../good.json"},{"id":"Bad ID","file":"good.json"},
	 {"id":"fan-tl-26","file":"good.json"}]}`)
	stage, skipped, err := stageLibrary(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	if len(skipped) != 9 {
		t.Fatalf("skipped %d: %v", len(skipped), skipped)
	}
	n, err := leds.LoadLibrary(stage)
	if n != 1 || err != nil {
		t.Fatalf("loaded %d, err %v", n, err)
	}
}
