package app

import (
	"context"
	"sort"
	"testing"
)

func TestDemoPanelsRoundDefaults(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	for _, p := range e.Panels() {
		wantRound := p.Width == 400 // demo TL LCD fans
		if p.Round != wantRound {
			t.Errorf("%s: round=%v, want %v", p.ID, p.Round, wantRound)
		}
		if wantRound && p.Config.Design != "cpu-gpu" {
			t.Errorf("%s: default design %q, want cpu-gpu for round screens", p.ID, p.Config.Design)
		}
	}
}

func TestSetPanelConfigValidatesContent(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	id := e.Panels()[0].ID
	base := e.Panels()[0].Config

	c := base
	c.Mode, c.Design = "design", "nonexistent"
	if err := e.SetPanelConfig(id, c); err == nil {
		t.Error("unknown design accepted")
	}
	for _, name := range []string{"../../etc/x", "a/b.gif", ".hidden.gif", "x.txt"} {
		c = base
		c.Mode, c.Media = "media", name
		if err := e.SetPanelConfig(id, c); err == nil {
			t.Errorf("media name %q accepted", name)
		}
	}
	c = base
	c.Mode, c.Media = "media", "missing.gif"
	if err := e.SetPanelConfig(id, c); err == nil {
		t.Error("missing media file accepted")
	}

	// A user design deleted while in use must not lock the panel: other
	// settings stay editable as long as the reference is unchanged.
	if err := e.Store.SaveDesign("tmp-design", builtinDesigns()["gaming"]); err != nil {
		t.Fatal(err)
	}
	c = base
	c.Mode, c.Design = "design", "tmp-design"
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Fatal(err)
	}
	if err := e.Store.DeleteDesign("tmp-design"); err != nil {
		t.Fatal(err)
	}
	c.Brightness = 40
	if err := e.SetPanelConfig(id, c); err != nil {
		t.Errorf("unchanged stale design rejected: %v", err)
	}
	c.Design = "tmp-design-2"
	if err := e.SetPanelConfig(id, c); err == nil {
		t.Error("changed to unknown design accepted")
	}
}

func TestNaturalOrder(t *testing.T) {
	ids := []string{"tllcd-p10-i0", "tllcd-p2-i1", "aio-7398-0", "tllcd-p2-i0", "tllcd-p1-i2"}
	sort.Slice(ids, func(i, j int) bool { return naturalLess(ids[i], ids[j]) })
	want := []string{"aio-7398-0", "tllcd-p1-i2", "tllcd-p2-i0", "tllcd-p2-i1", "tllcd-p10-i0"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order %v, want %v", ids, want)
		}
	}
}
