package app

import (
	"fmt"
	"log"
	"slices"
	"sort"

	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/panel"
)

// HardwareReportURL opens the issue form for test reports.
const HardwareReportURL = "https://github.com/kewe87/CaseHUD/issues/new?template=hardware_report.yml"

// DriverInfo is one device driver as the settings page lists it.
type DriverInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"` // "display" or "sensor"
	Models       []string `json:"models"`
	Experimental bool     `json:"experimental"`
	// Enabled is whether discovery currently includes the driver.
	Enabled bool `json:"enabled"`
	// Off is the individual switch (only meaningful for experimental drivers).
	Off bool `json:"off,omitempty"`
}

// DevicesState is the experimental-devices section of the settings.
type DevicesState struct {
	Experimental bool         `json:"experimental"`
	Drivers      []DriverInfo `json:"drivers"`
	ReportURL    string       `json:"reportUrl"`
}

// panelOptions turns the stored choice into discovery options.
func panelOptions(c Config) panel.Options {
	o := panel.Options{Experimental: c.ExperimentalDevices, NoSerial: !c.TLSerialOn, Off: map[string]bool{}}
	for _, id := range c.ExperimentalOff {
		o.Off[id] = true
	}
	return o
}

// Devices lists the drivers and the experimental switches.
func (e *Engine) Devices() DevicesState {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	o := panelOptions(cfg)
	st := DevicesState{Experimental: o.Experimental, ReportURL: HardwareReportURL, Drivers: []DriverInfo{}}
	for _, d := range panel.Drivers() {
		if !d.Experimental {
			continue
		}
		in := DriverInfo{ID: d.ID, Name: d.Name, Kind: string(d.Kind), Experimental: true,
			Enabled: o.Experimental && !o.Off[d.ID], Off: o.Off[d.ID], Models: []string{}}
		for _, m := range d.Models {
			if m.Name != "" {
				in.Models = append(in.Models, m.Name)
			}
		}
		st.Drivers = append(st.Drivers, in)
	}
	return st
}

// SetDevices stores the experimental switches (global and per driver, by
// driver ID) and rescans so the change takes effect. Unknown or stable
// driver IDs are rejected: only experimental drivers can be switched.
func (e *Engine) SetDevices(on bool, off []string) error {
	var ids []string
	for _, id := range off {
		d, ok := panel.DriverByID(id)
		if !ok || !d.Experimental {
			return fmt.Errorf("unknown experimental driver %q", id)
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	e.mu.Lock()
	if on && !e.cfg.ExperimentalDevices && !e.cfg.AckExperimental {
		e.mu.Unlock()
		return fmt.Errorf("%w: confirm the notice for experimental devices in the CaseHUD window first", ErrAckRequired)
	}
	changed := e.cfg.ExperimentalDevices != on || !slices.Equal(e.cfg.ExperimentalOff, ids)
	e.cfg.ExperimentalDevices, e.cfg.ExperimentalOff = on, ids
	panel.SetOptions(panelOptions(e.cfg)) // under the lock: concurrent calls cannot cross
	e.mu.Unlock()
	if err := e.persist(); err != nil {
		return err
	}
	if changed && !e.Demo {
		go e.Rescan()
	}
	return nil
}

// deviceReadings gathers the sensors of panels and read-only devices. It
// takes only devMu, never the engine lock: it runs inside the provider's
// refresh, which other code reaches while holding the engine lock.
func (e *Engine) deviceReadings() []metrics.Reading {
	e.devMu.Lock()
	srcs := slices.Clone(e.devSrcs)
	e.devMu.Unlock()
	var out []metrics.Reading
	for _, s := range srcs {
		for _, r := range s.Sensors() {
			out = append(out, metrics.Reading{Key: r.Key, Path: r.Path, Value: r.Value, Unit: r.Unit})
		}
	}
	return out
}

// setSensorSources replaces the readable devices: the panels that report
// sensors and the read-only devices. The old read-only devices are closed.
func (e *Engine) setSensorSources(panels []panel.Panel, devs []panel.SensorDevice) {
	var srcs []panel.SensorSource
	for _, p := range panels {
		if s, ok := p.(panel.SensorSource); ok {
			srcs = append(srcs, s)
		}
	}
	for _, d := range devs {
		srcs = append(srcs, d)
	}
	e.devMu.Lock()
	old := e.sensorDevs
	e.sensorDevs, e.devSrcs = devs, srcs
	e.devMu.Unlock()
	for _, d := range old {
		if err := d.Close(); err != nil {
			log.Println("sensor device:", err)
		}
	}
}
