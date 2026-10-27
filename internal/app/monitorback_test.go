package app

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/panel"
)

type fwFan struct{ startFan }

func (f *fwFan) Version() string { return "v9 (2025-01-02)" }

func TestPanelsReportFirmware(t *testing.T) {
	e := startEngine(t, true)
	old := discoverPanels
	discoverPanels = func() ([]panel.Panel, error) {
		return []panel.Panel{&fwFan{startFan{key: "a"}}, &startFan{key: "b"}}, nil
	}
	t.Cleanup(func() { discoverPanels = old })
	e.Rescan()
	got := map[string]string{}
	for _, p := range e.Panels() {
		got[p.ID] = p.Firmware
	}
	if got["fan-a"] != "v9 (2025-01-02)" || got["fan-b"] != "" {
		t.Fatalf("firmware = %v", got)
	}
}

func monitorRig(t *testing.T, accepted bool) (*Engine, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	e := startEngine(t, accepted)
	var finds, sends atomic.Int32
	of, ob, or := findMonitorMode, bringBackMonitor, monitorRescan
	findMonitorMode = func() []panel.MonitorModeDevice {
		finds.Add(1)
		return []panel.MonitorModeDevice{{Path: "p1"}}
	}
	bringBackMonitor = func(string) error { sends.Add(1); return nil }
	monitorRescan = time.Millisecond
	t.Cleanup(func() { findMonitorMode, bringBackMonitor, monitorRescan = of, ob, or })
	return e, &finds, &sends
}

func TestMonitorModeHeldBeforeDisclaimer(t *testing.T) {
	e, finds, sends := monitorRig(t, false)
	if len(e.MonitorModeDevices()) != 0 || finds.Load() != 0 {
		t.Fatal("enumerated before the disclaimer")
	}
	if err := e.BringBackMonitorMode("p1"); !errors.Is(err, ErrOutputHeld) || sends.Load() != 0 {
		t.Fatalf("send before the disclaimer: %v", err)
	}
}

func TestMonitorModeListingIsCached(t *testing.T) {
	e, finds, _ := monitorRig(t, true)
	for i := 0; i < 3; i++ {
		if got := e.MonitorModeDevices(); len(got) != 1 || got[0].Path != "p1" {
			t.Fatalf("list %v", got)
		}
	}
	if finds.Load() != 1 {
		t.Fatalf("%d enumerations", finds.Load())
	}
}

func TestMonitorModeBringBackIsRateLimitedAndRescans(t *testing.T) {
	e, _, sends := monitorRig(t, true)
	var scans atomic.Int32
	old := discoverPanels
	discoverPanels = func() ([]panel.Panel, error) { scans.Add(1); return nil, nil }
	t.Cleanup(func() { discoverPanels = old })
	if err := e.BringBackMonitorMode("p1"); err != nil {
		t.Fatal(err)
	}
	if err := e.BringBackMonitorMode("p1"); !errors.Is(err, ErrMonitorBackWait) {
		t.Fatalf("second attempt: %v", err)
	}
	if sends.Load() != 1 {
		t.Fatalf("%d reports", sends.Load())
	}
	deadline := time.Now().Add(3 * time.Second)
	for scans.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if scans.Load() == 0 {
		t.Fatal("no rescan after the report")
	}
}
