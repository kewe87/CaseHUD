package app

import (
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

type fakeSensorDev struct {
	closed  bool
	readout []panel.Sensor
}

func (f *fakeSensorDev) Sensors() []panel.Sensor { return f.readout }
func (f *fakeSensorDev) Name() string            { return "fake" }
func (f *fakeSensorDev) Close() error            { f.closed = true; return nil }

func TestDeviceReadingsAndSwap(t *testing.T) {
	e := &Engine{}
	a := &fakeSensorDev{readout: []panel.Sensor{{Key: "k", Path: "Dev/Temp", Value: 41, Unit: "°C"}}}
	e.setSensorSources(nil, []panel.SensorDevice{a})
	got := e.deviceReadings()
	if len(got) != 1 || got[0].Path != "Dev/Temp" || got[0].Value != 41 {
		t.Fatalf("%+v", got)
	}
	b := &fakeSensorDev{}
	e.setSensorSources(nil, []panel.SensorDevice{b})
	if !a.closed || b.closed {
		t.Fatal("old device not closed / new one closed")
	}
	if len(e.deviceReadings()) != 0 {
		t.Fatal("readings of a replaced device")
	}
	e.setSensorSources(nil, nil)
	if !b.closed {
		t.Fatal("device not closed on shutdown")
	}
}
