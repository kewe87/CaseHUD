package leds

import "image/color"

// submitFrame hands a frame to the device's worker and keeps a copy for
// Snapshot.
func (e *Engine) submitFrame(d *deviceEntry, frame map[string][]color.RGBA) {
	d.worker.submit(frame)
	id := d.dev.ID()
	e.mu.Lock()
	if e.live == nil {
		e.live = map[string]map[string][]color.RGBA{}
	}
	zones := e.live[id]
	if zones == nil {
		zones = make(map[string][]color.RGBA, len(frame))
		e.live[id] = zones
	}
	for zid, cols := range frame {
		zones[zid] = append(zones[zid][:0], cols...)
	}
	e.mu.Unlock()
}

// Snapshot returns a copy of the last frame sent to each device, per zone, in
// wire order and after brightness (what the LEDs show). Devices that are gone
// are left out. The result belongs to the caller.
func (e *Engine) Snapshot() map[string]map[string][]color.RGBA {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]map[string][]color.RGBA, len(e.live))
	known := make(map[string]bool, len(e.devices))
	for _, d := range e.devices {
		known[d.dev.ID()] = true
	}
	for id, zones := range e.live {
		if !known[id] {
			delete(e.live, id)
			continue
		}
		cp := make(map[string][]color.RGBA, len(zones))
		for zid, cols := range zones {
			cp[zid] = append([]color.RGBA(nil), cols...)
		}
		out[id] = cp
	}
	return out
}
