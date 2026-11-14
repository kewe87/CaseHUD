// Package metrics provides live system readings (CPU/GPU load, temperatures,
// pump/fan RPM, coolant temperature, ...) for the LCD dashboard.
//
// The primary backend is LibreHardwareMonitor's built-in remote web server
// (Options -> Remote Web Server), which exposes every sensor it knows as JSON.
// On Windows this is the only practical way to read CPU temperatures without
// shipping a kernel driver, and it also covers NVIDIA GPUs, motherboard fan
// headers, coolant sensors and Aquacomputer devices.
package metrics

import (
	"strings"
	"time"
)

// Reading is one sensor value.
type Reading struct {
	Key   string  // stable id, e.g. "/gpu-nvidia/0/temperature/0"
	Path  string  // human path, e.g. "NVIDIA GeForce RTX 4080/Temperatures/GPU Core"
	Value float64 // numeric value
	Unit  string  // "°C", "%", "RPM", ...
	// Age is how long before it was delivered the value was read from the hardware,
	// if the source says so (AgeKnown). Most sources do not.
	Age      time.Duration `json:"-"`
	AgeKnown bool          `json:"-"`
	// DisplayOnly marks a value from a source that is not known to be the sensor
	// service of this machine: it may be shown, but must not drive the cooling.
	DisplayOnly bool `json:"-"`
}

// Provider is a source of sensor readings.
type Provider interface {
	// Refresh pulls a fresh snapshot from the backend.
	Refresh() error
	// Get resolves a source expression to a reading. A source may list
	// alternatives separated by "|"; the first one that resolves wins. Each
	// alternative matches a sensor id exactly, a full path exactly
	// (case-insensitive), or a path suffix on a "/" boundary, e.g.
	// "Temperatures/GPU Core".
	Get(source string) (Reading, bool)
	// All returns every reading of the last snapshot.
	All() []Reading
}

// Match resolves source against a list of readings with the rules documented
// on Provider.Get.
func Match(readings []Reading, source string) (Reading, bool) { return match(readings, source) }

// match resolves source against readings using the rules documented on
// Provider.Get.
func match(readings []Reading, source string) (Reading, bool) {
	for _, alt := range strings.Split(source, "|") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		la := strings.ToLower(alt)
		for _, r := range readings {
			if (r.Key == alt || strings.ToLower(r.Path) == la) && plausible(la, r) {
				return r, true
			}
		}
		// Suffix matches can hit several devices (e.g. "Load/GPU Core" on
		// both a dedicated card and the CPU's integrated graphics). Prefer
		// devices that aren't integrated GPUs.
		var fallback *Reading
		for i, r := range readings {
			if strings.HasSuffix(strings.ToLower(r.Path), "/"+la) && plausible(la, r) {
				if !isIntegratedGPU(r.Path) {
					return r, true
				}
				if fallback == nil {
					fallback = &readings[i]
				}
			}
		}
		if fallback != nil {
			return *fallback, true
		}
	}
	return Reading{}, false
}

// plausible rejects the reading of a coolant probe header ("T Sensor") with
// nothing connected: zero, below zero or implausibly hot. The source then falls
// through to the next alternative, or shows nothing. A pump header is not
// filtered: a pump that stands must stay visible as 0 rpm.
func plausible(alt string, r Reading) bool {
	switch {
	case strings.HasSuffix(alt, "/t sensor"):
		return r.Value > 0 && r.Value < 120
	}
	return true
}

// integratedGPUs are name fragments of CPU-integrated graphics in
// LibreHardwareMonitor's hardware names.
var integratedGPUs = []string{"radeon(tm) graphics", "radeon graphics", "uhd graphics", "iris", "intel(r) graphics", "intel graphics"}

func isIntegratedGPU(path string) bool {
	hw, _, _ := strings.Cut(strings.ToLower(path), "/")
	for _, s := range integratedGPUs {
		if strings.Contains(hw, s) {
			return true
		}
	}
	return false
}

// History keeps a fixed-size ring of recent values per source, for graphs.
type History struct {
	size int
	data map[string][]float64
}

// NewHistory returns a history holding up to size samples per source.
func NewHistory(size int) *History {
	return &History{size: size, data: map[string][]float64{}}
}

// Push appends a sample for source, dropping the oldest when full.
func (h *History) Push(source string, v float64) {
	s := append(h.data[source], v)
	if len(s) > h.size {
		s = s[len(s)-h.size:]
	}
	h.data[source] = s
}

// Get returns the samples for source, oldest first.
func (h *History) Get(source string) []float64 {
	return h.data[source]
}
