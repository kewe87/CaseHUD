package leds_test

import (
	"context"
	"encoding/json"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

func TestInspectReport(t *testing.T) {
	dev := newDev()
	r := leds.Inspect(context.Background(), []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}})
	if len(r.Devices) != 1 || r.Devices[0].ID != "fake:1" || r.Engine {
		t.Fatalf("report %+v", r)
	}
	if dev.Frames() != 0 {
		t.Fatal("inspect sent a frame")
	}
	if _, err := json.Marshal(r); err != nil {
		t.Fatal(err)
	}
}

func TestEngineDiagnosticsLog(t *testing.T) {
	dev := newDev()
	e := leds.New(leds.DefaultConfig(), leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})
	e.Start(context.Background())
	defer e.Stop()
	r := e.Diagnostics()
	if !r.Engine || r.Config == nil || len(r.Devices) != 1 {
		t.Fatalf("report %+v", r)
	}
	found := false
	for _, l := range r.Log {
		if l.Source == "engine" && l.Kind == "log" {
			found = true
		}
	}
	if !found {
		t.Fatal("engine log missing")
	}
}
