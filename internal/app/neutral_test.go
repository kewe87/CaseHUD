package app

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

// The tests expect German unless they say otherwise, whatever language the
// machine they run on uses.
func TestMain(m *testing.M) {
	systemLanguage = func() string { return "de" }
	os.Exit(m.Run())
}

func TestLanguageFollowsTheSystemUntilChosen(t *testing.T) {
	old := systemLanguage
	defer func() { systemLanguage = old }()
	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	for _, sys := range []string{"de", "en"} {
		systemLanguage = func() string { return sys }
		if got := e.Language(); got != sys {
			t.Errorf("system %s: language %q", sys, got)
		}
	}
	systemLanguage = func() string { return "en" }
	if err := e.SetLanguage("de"); err != nil {
		t.Fatal(err)
	}
	if got := e.Language(); got != "de" {
		t.Errorf("a stored choice must win over the system language, got %q", got)
	}
}

func TestFanSetsWithoutCoolant(t *testing.T) {
	full, none := FanSetsFor(true), FanSetsFor(false)
	for set, trio := range full {
		if !strings.HasSuffix(trio[2], "loop") {
			t.Errorf("%s: coolant trio %v", set, trio)
		}
		if strings.Contains(none[set][2], "loop") || none[set][2] != squareRoles["power"][set] {
			t.Errorf("%s: trio without coolant %v", set, none[set])
		}
		if !slices.Equal(trio[:2], none[set][:2]) {
			t.Errorf("%s: CPU and GPU must stay: %v / %v", set, trio, none[set])
		}
		for _, id := range none[set] {
			if _, ok := builtinDesigns()[id]; !ok {
				t.Errorf("%s: design %q does not exist", set, id)
			}
		}
		if d, _ := setDesign(set, false, true, 0, false); d != "" {
			t.Errorf("%s: bar without coolant gets %q, want the composition", set, d)
		}
	}
	if d, _ := setDesign("clean", false, false, 0, false); d != "cpu-gpu" {
		t.Errorf("clean square without coolant: %q", d)
	}
}

func TestTemplatesWithoutCoolant(t *testing.T) {
	for _, set := range SetOrder {
		with, without := GridTemplatesFor(set, true), GridTemplatesFor(set, false)
		if len(with) != len(without)+1 {
			t.Fatalf("%s: %d templates with, %d without a coolant sensor", set, len(with), len(without))
		}
		for _, tp := range without {
			if tp.ID == "watercooling" {
				t.Errorf("%s: water cooling offered without a coolant sensor", set)
			}
			for _, tl := range tp.Tiles {
				d := tl.Design
				if strings.Contains(d, "loop") || strings.Contains(d, "coolant") || strings.Contains(d, "water") {
					t.Errorf("%s/%s: %q needs a coolant sensor", set, tp.ID, d)
				}
				if _, ok := (&Store{Dir: t.TempDir()}).Design(d); d == "" || ok != nil {
					t.Errorf("%s/%s: design %q does not exist", set, tp.ID, d)
				}
			}
		}
	}
	s := &Store{Dir: t.TempDir(), hasCoolant: func() bool { return false }}
	for _, l := range s.Layouts() {
		if l.Tpl == "watercooling" {
			t.Errorf("layout %s listed without a coolant sensor", l.ID)
		}
	}
	if _, ok := s.Layout("watercooling-clean"); !ok {
		t.Error("a composition that is asked for by id stays available")
	}
}

func TestHasCoolant(t *testing.T) {
	e := &Engine{prov: NewSafeProvider(metrics.NewMock())}
	if !e.HasCoolant() {
		t.Error("before the first snapshot the coolant counts as present")
	}
	e.prov.snap = []metrics.Reading{{Key: "a", Path: "CPU/Temperatures/Package", Value: 40}}
	if e.HasCoolant() {
		t.Error("no coolant sensor in the snapshot")
	}
	e.prov.snap = append(e.prov.snap, metrics.Reading{Key: "b", Path: "Board/Temperatures/Coolant", Value: 30})
	if !e.HasCoolant() {
		t.Error("a coolant sensor in the snapshot")
	}
}

func TestLightingTextsFollowTheLanguage(t *testing.T) {
	if flashWarnDE == flashWarnEN || !strings.Contains(flashWarnEN, "flash") || strings.Contains(flashWarnEN, "Lüfter") {
		t.Error("flash warning must exist in English")
	}
	if pick("de", "a", "b") != "a" || pick("en", "a", "b") != "b" || pick("", "a", "b") != "b" {
		t.Error("pick")
	}
}
