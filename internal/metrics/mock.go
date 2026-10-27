package metrics

import (
	"hash/fnv"
	"math"
	"strings"
	"time"
)

// Mock fabricates plausible, slowly changing values for any source, so layouts
// can be previewed without LibreHardwareMonitor or the LCD attached.
type Mock struct {
	start time.Time
	now   time.Time
}

// NewMock returns a mock provider.
func NewMock() *Mock { return &Mock{start: time.Now(), now: time.Now()} }

// Refresh implements Provider.
func (m *Mock) Refresh() error {
	m.now = time.Now()
	return nil
}

// SetTime pins the mock clock, e.g. to backfill graph history for a preview.
func (m *Mock) SetTime(t time.Time) { m.now = t }

// Get implements Provider. The value range is guessed from the sensor group
// in the source ("Temperatures", "Load", "Fans", ...).
func (m *Mock) Get(source string) (Reading, bool) {
	alt, _, _ := strings.Cut(source, "|")
	h := fnv.New32a()
	h.Write([]byte(alt))
	phase := float64(h.Sum32()%1000) / 1000 * 2 * math.Pi
	t := m.now.Sub(m.start).Seconds()
	w := (math.Sin(t/7+phase) + math.Sin(t/2.3+phase*2)*0.3 + 1.3) / 2.6 // ~0..1

	lo, hi, unit := 0.0, 100.0, "%"
	switch l := strings.ToLower(alt); {
	case strings.Contains(l, "frametime"):
		// the reciprocal of the frame rate, with the jitter of a real game
		fps, _ := m.Get("FPS/Framerate")
		jit := 1 + 0.12*math.Sin(t*11+phase) + 0.06*math.Sin(t*29)
		return Reading{Key: alt, Path: alt, Value: 1000 / fps.Value * jit, Unit: "ms"}, true
	case strings.Contains(l, "coolant"), strings.Contains(l, "water"), strings.Contains(l, "liquid"), strings.Contains(l, "t sensor"):
		lo, hi, unit = 27, 36, "°C"
	case strings.Contains(l, "hot spot"):
		lo, hi, unit = 45, 86, "°C"
	case strings.Contains(l, "memory used"):
		lo, hi, unit = 3, 13, "GB"
	case strings.Contains(l, "memory total"):
		lo, hi, unit = 16, 16, "GB"
	case strings.Contains(l, "temperature"):
		lo, hi, unit = 35, 78, "°C"
	case strings.Contains(l, "fan"), strings.Contains(l, "pump"):
		lo, hi, unit = 900, 2400, "RPM"
	case strings.Contains(l, "flow"):
		lo, hi, unit = 140, 190, "L/h"
	case strings.Contains(l, "latency"):
		lo, hi, unit = 18, 34, "ms"
	case strings.Contains(l, "framerate"), strings.Contains(l, "fps"):
		lo, hi, unit = 90, 160, "FPS"
	case strings.Contains(l, "power"):
		lo, hi, unit = 60, 320, "W"
	case strings.Contains(l, "clock"):
		lo, hi, unit = 1800, 2800, "MHz"
	}
	if unit == "FPS" {
		w += 0.04 * math.Sin(t*11+phase) // frame rate wobble
	}
	return Reading{Key: alt, Path: alt, Value: lo + (hi-lo)*w, Unit: unit}, true
}

// All implements Provider.
func (m *Mock) All() []Reading { return nil }
