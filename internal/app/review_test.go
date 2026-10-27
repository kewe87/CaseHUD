package app

import (
	"image"
	"strings"
	"testing"
	"time"
)

func TestApplySetOnUprightBarUsesWideDesign(t *testing.T) {
	t.Setenv("CASEHUD_DEMO_BAR", "1")
	e := startDemo(t)
	if _, err := e.UpdatePanelConfig("us88-0", func(c *PanelConfig) error {
		c.Rotation, c.Grid, c.Tiles, c.Design = 90, "", nil, "water-wide"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ApplySet("neon", "dark", []string{"us88-0"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range e.Panels() {
		if p.ID == "us88-0" && p.Config.Design != "neon-wide" {
			t.Fatalf("upright bar got %q, want neon-wide", p.Config.Design)
		}
	}
}

func TestMediaPathRules(t *testing.T) {
	st := &Store{Dir: t.TempDir()}
	ok := []string{"a.png", "My Photo (1).jpg", "Bild äöü [x].mp4", "clip-1_final.GIF"}
	for _, n := range ok {
		if _, err := st.MediaPath(n); err != nil {
			t.Errorf("%q rejected: %v", n, err)
		}
	}
	bad := []string{"", ".hidden.png", "../x.png", `a\b.png`, "a/b.png", "a.png:stream", "C:x.png", "CON.png", "nul.gif", "Com1.mp4",
		"LPT9.png", "aux", "a?.png", "a*.png", "a|b.png", "a<b.png", "x.png ", "x.png.", "a\x01.png", "readme.txt", strings.Repeat("a", 300) + ".png"}
	for _, n := range bad {
		if _, err := st.MediaPath(n); err == nil {
			t.Errorf("%q accepted", n)
		}
	}
	if !ReservedMediaName("com1.txt") || ReservedMediaName("com10.png") || ReservedMediaName("console.png") {
		t.Error("ReservedMediaName")
	}
}

func TestDesignForRejectsBadIDs(t *testing.T) {
	st := &Store{Dir: t.TempDir()}
	for _, id := range []string{"../config", `..\x`, "a/b", "", "Upper"} {
		if _, err := st.DesignFor(id, image.Pt(480, 1920)); err == nil {
			t.Errorf("%q accepted", id)
		}
	}
	if _, err := st.DesignFor("water-wide", image.Pt(480, 1920)); err != nil {
		t.Error(err)
	}
}

func TestCleanPanelConfigRejectsBadDesignIDs(t *testing.T) {
	e := startDemo(t)
	c := PanelConfig{Mode: "design", Design: "../../x"}
	if _, err := e.cleanPanelConfig(c); err == nil {
		t.Error("panel design path accepted")
	}
	c = PanelConfig{Mode: "design", Grid: "4", Tiles: []Tile{{Mode: "design", Design: "a/b"}, {}, {}, {}}}
	if _, err := e.cleanPanelConfig(c); err == nil {
		t.Error("tile design path accepted")
	}
	cfg := Config{Panels: map[string]PanelConfig{"p": {Mode: "design", Grid: "4", Tiles: []Tile{{Mode: "design", Design: "../x"}, {}, {}, {}}}}}
	sanitizeLoadedConfig(&cfg)
	if tl := cfg.Panels["p"].Tiles[0]; tl.Design != "" || tl.Mode != "off" {
		t.Errorf("loaded tile kept: %+v", tl)
	}
}

func TestSceneLimits(t *testing.T) {
	e := startDemo(t)
	if err := e.SaveScene(strings.Repeat("x", 65)); err == nil {
		t.Error("long scene name accepted")
	}
	if err := e.SaveScene("A\x00\x07 B\n"); err != nil {
		t.Fatal(err)
	}
	if sc := e.Scenes(); len(sc) != 1 || sc[0].Name != "A B" {
		t.Fatalf("control characters not stripped: %+v", sc)
	}
	if err := e.SetAutomation(AutomationConfig{Rules: []AutomationRule{{When: "game", Scene: strings.Repeat("y", 65)}}}); err == nil {
		t.Error("long rule scene accepted")
	}
	for i := 0; i < maxScenes-1; i++ {
		if err := e.SaveScene(strings.Repeat("s", 1) + string(rune('a'+i%26)) + strings.Repeat("n", i/26+1)); err != nil {
			t.Fatalf("scene %d: %v", i, err)
		}
	}
	if err := e.SaveScene("one too many"); err == nil {
		t.Error("more than the maximum number of scenes accepted")
	}
}

func TestAutomationStatusFollowsReorder(t *testing.T) {
	e, f := autoEngine(t)
	rules := []AutomationRule{{When: "game:other.exe", Scene: "Elden"}, {When: "game", Scene: "Gaming"}}
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: rules}); err != nil {
		t.Fatal(err)
	}
	f.set(true, "x.exe")
	e.automate(f.advance(0))
	e.automate(f.advance(2 * time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 1 {
		t.Fatalf("%+v", st)
	}
	applied := func() int { e.mu.Lock(); defer e.mu.Unlock(); return e.auto.applied }
	base := applied()
	// the rules swap places: same active rule, new index, no re-apply
	if err := e.SetAutomation(AutomationConfig{Enabled: true, EnterS: 1, Rules: []AutomationRule{rules[1], rules[0]}}); err != nil {
		t.Fatal(err)
	}
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 0 || st.Scene != "Gaming" {
		t.Fatalf("status after reorder: %+v", st)
	}
	if applied() != base {
		t.Error("reorder re-applied the displays")
	}
	// renaming the shown scene does not re-apply either
	if err := e.RenameScene("Gaming", "Zocken"); err != nil {
		t.Fatal(err)
	}
	e.automate(f.advance(time.Second))
	if st := e.AutomationStatus(); st.ActiveRule != 0 || st.Scene != "Zocken" {
		t.Fatalf("status after rename: %+v", st)
	}
	if applied() != base {
		t.Error("rename re-applied the displays")
	}
}
