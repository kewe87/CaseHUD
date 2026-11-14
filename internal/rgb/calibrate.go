package leds

import (
	"fmt"
	"image/color"
	"time"
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
	identify     bool // the whole zone blinks (Identify)
	// pattern is a still picture on the zone (Pattern): LED i shows
	// pattern[i], LEDs past its end are dark.
	pattern []color.RGBA
}

// IdentifyColor is the colour of a zone that blinks to be found; it is on
// for identifyOn of every identifyPeriod.
var IdentifyColor = color.RGBA{255, 255, 255, 255}

const (
	identifyPeriod = 800 * time.Millisecond
	identifyOn     = 500 * time.Millisecond
)

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

// Identify makes every LED of one zone blink white, up to the zone's
// hardware limit, so the user can see which parts hang on it. Like
// Calibrate, the other zones of the device are dark, other devices keep the
// effect, and EndCalibration returns to the effect.
func (e *Engine) Identify(deviceID, zone string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range e.devices {
		if d.dev.ID() != deviceID {
			continue
		}
		for _, z := range d.dev.Zones() {
			if z.ID == zone {
				if calibrationLimit(z) == 0 {
					return fmt.Errorf("leds: zone %s has no LEDs", zone)
				}
				e.calib = &calibration{device: deviceID, zone: zone, identify: true}
				return nil
			}
		}
		return fmt.Errorf("leds: device %s has no zone %q", deviceID, zone)
	}
	return fmt.Errorf("leds: unknown device %q", deviceID)
}

// Pattern shows a still picture on one zone instead of the effect: LED i
// takes cols[i], the LEDs after them stay dark. Like Calibrate, the other
// zones of the device are dark, other devices keep the effect, nothing is
// stored and EndCalibration returns to the effect.
func (e *Engine) Pattern(deviceID, zone string, cols []color.RGBA) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range e.devices {
		if d.dev.ID() != deviceID {
			continue
		}
		for _, z := range d.dev.Zones() {
			if z.ID == zone {
				if len(cols) == 0 || len(cols) > calibrationLimit(z) {
					return fmt.Errorf("leds: pattern of %d LEDs for zone %s (1..%d)", len(cols), zone, calibrationLimit(z))
				}
				e.calib = &calibration{device: deviceID, zone: zone, pattern: append([]color.RGBA(nil), cols...)}
				return nil
			}
		}
		return fmt.Errorf("leds: device %s has no zone %q", deviceID, zone)
	}
	return fmt.Errorf("leds: unknown device %q", deviceID)
}

// IdentifyFrame returns n LEDs of a blinking zone at time t: all white
// while on, all dark while off.
func IdentifyFrame(n int, t time.Time) []color.RGBA {
	c := color.RGBA{A: 255}
	if time.Duration(t.UnixNano())%identifyPeriod < identifyOn {
		c = IdentifyColor
	}
	cols := make([]color.RGBA, n)
	for i := range cols {
		cols[i] = c
	}
	return cols
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
			switch {
			case c.identify:
				m = IdentifyFrame(calibrationLimit(z), time.Now())
			case c.pattern != nil:
				m = make([]color.RGBA, calibrationLimit(z))
				for i := range m {
					m[i] = color.RGBA{A: 255}
				}
				copy(m, c.pattern)
			}
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
