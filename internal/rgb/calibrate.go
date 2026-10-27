package leds

import (
	"fmt"
	"image/color"
)

// calibrationMaxLEDs bounds zones without a hardware limit during calibration.
const calibrationMaxLEDs = 512

// Marker colors: the red head is the LED at the position, the blue tail shows
// the direction. Once the head is no longer visible the position is past the
// end of the chain.
var (
	MarkerHead = color.RGBA{255, 0, 0, 255}
	MarkerTail = color.RGBA{0, 60, 255, 255}
)

type calibration struct {
	device, zone string
	pos          int
}

// Calibrate shows a marker (red head at pos, blue tail behind it, 0-based) on
// one zone of a device instead of the effect, so the user can find the real
// LED count, the first LED and the direction by moving pos. The other zones
// of that device are dark; other devices keep running the effect. The marker
// is sent in full brightness and up to the zone's hardware limit, regardless
// of the configured LED count.
func (e *Engine) Calibrate(deviceID, zone string, pos int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range e.devices {
		if d.dev.ID() != deviceID {
			continue
		}
		for _, z := range d.dev.Zones() {
			if z.ID == zone {
				if pos < 0 || pos >= calibrationLimit(z) {
					return fmt.Errorf("leds: position %d outside zone %s (0..%d)", pos, zone, calibrationLimit(z)-1)
				}
				e.calib = &calibration{device: deviceID, zone: zone, pos: pos}
				return nil
			}
		}
		return fmt.Errorf("leds: device %s has no zone %q", deviceID, zone)
	}
	return fmt.Errorf("leds: unknown device %q", deviceID)
}

// EndCalibration returns to the effect.
func (e *Engine) EndCalibration() {
	e.mu.Lock()
	e.calib = nil
	e.mu.Unlock()
}

func calibrationLimit(z Zone) int {
	if z.Max > 0 {
		return min(z.Max, calibrationMaxLEDs)
	}
	if z.Resizable {
		return calibrationMaxLEDs
	}
	return z.Count
}

// CalibrationMarker returns n LEDs with the marker at pos: head at pos, tail
// on the two LEDs before it (the second one dimmer).
func CalibrationMarker(n, pos int) []color.RGBA {
	cols := make([]color.RGBA, n)
	for i := range cols {
		cols[i] = color.RGBA{A: 255}
	}
	set := func(i int, c color.RGBA) {
		if i >= 0 && i < n {
			cols[i] = c
		}
	}
	set(pos-2, color.RGBA{MarkerTail.R / 4, MarkerTail.G / 4, MarkerTail.B / 4, 255})
	set(pos-1, MarkerTail)
	set(pos, MarkerHead)
	return cols
}

// calibrationFrame builds the frame for a device under calibration.
func calibrationFrame(d *deviceEntry, c calibration) map[string][]color.RGBA {
	frame := make(map[string][]color.RGBA, len(d.frame))
	for id, buf := range d.frame {
		frame[id] = make([]color.RGBA, len(buf))
		for i := range frame[id] {
			frame[id][i] = color.RGBA{A: 255}
		}
	}
	for _, z := range d.dev.Zones() {
		if z.ID == c.zone {
			m := CalibrationMarker(calibrationLimit(z), c.pos)
			order := z.Order
			for _, zr := range d.zones {
				if zr.id == z.ID {
					order = zr.order
				}
			}
			reorder(m, order)
			frame[z.ID] = m
		}
	}
	return frame
}
