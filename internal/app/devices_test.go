package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

func TestExperimentalDevicesSwitch(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	e.Acknowledge("experimental")
	t.Cleanup(func() { panel.SetOptions(panel.Options{}) })

	// default: everything experimental is off, the list names only experimental drivers
	d := e.Devices()
	if d.Experimental || len(d.Drivers) == 0 || d.ReportURL == "" {
		t.Fatalf("default state %+v", d)
	}
	for _, x := range d.Drivers {
		if !x.Experimental || x.Enabled {
			t.Fatalf("driver %+v should be listed as experimental and off", x)
		}
		if x.ID == "tllcd" || x.ID == "us88" {
			t.Fatalf("stable driver %s is listed as switchable", x.ID)
		}
	}
	if panel.CurrentOptions().Experimental {
		t.Fatal("panel options on by default")
	}

	// stable and unknown drivers cannot be named
	for _, bad := range []string{"tllcd", "us88", "nope", ""} {
		if err := e.SetDevices(true, []string{bad}); err == nil {
			t.Fatalf("driver %q accepted", bad)
		}
	}
	if panel.CurrentOptions().Experimental || e.cfg.ExperimentalDevices {
		t.Fatal("a rejected request changed the setting")
	}

	if err := e.SetDevices(true, []string{"flex", "flex", "aio"}); err != nil {
		t.Fatal(err)
	}
	o := panel.CurrentOptions()
	if !o.Experimental || !o.Off["flex"] || !o.Off["aio"] || o.Off["slv3"] {
		t.Fatalf("options %+v", o)
	}
	for _, x := range e.Devices().Drivers {
		want := x.ID != "flex" && x.ID != "aio"
		if x.Enabled != want {
			t.Errorf("%s enabled = %v, want %v", x.ID, x.Enabled, want)
		}
	}

	// persisted, sorted, and restored on the next start
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if !c.ExperimentalDevices || len(c.ExperimentalOff) != 2 || c.ExperimentalOff[0] != "aio" {
		t.Fatalf("stored %+v", c)
	}
	panel.SetOptions(panel.Options{})
	e2 := &Engine{Store: &Store{Dir: dir}, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e2.Stop()
	if o := panel.CurrentOptions(); !o.Experimental || !o.Off["aio"] {
		t.Fatalf("options after restart %+v", o)
	}

	// switching off again
	if err := e2.SetDevices(false, nil); err != nil {
		t.Fatal(err)
	}
	if panel.CurrentOptions().Experimental {
		t.Fatal("still on")
	}
}

// The ordinary settings path cannot turn experimental devices on.
func TestApplySettingsLeavesExperimentalAlone(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	e.Acknowledge("experimental")
	t.Cleanup(func() { panel.SetOptions(panel.Options{}) })
	if err := e.SetDevices(true, nil); err != nil {
		t.Fatal(err)
	}
	s := e.Settings()
	if err := e.ApplySettings(s); err != nil {
		t.Fatal(err)
	}
	if !e.Devices().Experimental {
		t.Fatal("ApplySettings changed the experimental switch")
	}
}
