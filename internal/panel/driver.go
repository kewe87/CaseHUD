package panel

// Driver registry. Every device family is described by one Driver value:
// static metadata plus a few entry points. The host only talks to drivers
// through this description and the Panel / SensorDevice interfaces, so a
// family can later be moved out of the program without touching the rest
// (see docs/DRIVERS.md).

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Transport is how a driver reaches its device.
type Transport string

const (
	TransportHID    Transport = "hid"    // HID class driver
	TransportWinUSB Transport = "winusb" // bulk transfers through WinUSB
)

// Kind tells what a driver produces.
type Kind string

const (
	KindDisplay Kind = "display" // Panels
	KindSensor  Kind = "sensor"  // SensorDevices (read-only)
)

// RotationMode tells where a panel turns its picture.
type RotationMode string

const (
	RotationNone     RotationMode = "none"     // not offered
	RotationDevice   RotationMode = "device"   // the device turns it
	RotationSoftware RotationMode = "software" // the driver (or host) turns the bitmap
)

// Model is one USB product a driver handles.
type Model struct {
	VendorID, ProductID uint16
	Name                string
}

// Capabilities describe what the driver implements (not what the hardware
// might offer).
type Capabilities struct {
	Width, Height int // native frame size as Panel.Size reports it
	Round         bool
	Rotation      RotationMode
	Rotations     []int // accepted angles for SetRotation
	// BrightnessMax is the largest raw value the device understands; the
	// driver scales the 0..100 percent of Panel.SetBrightness onto it.
	BrightnessMax int
	Overlay       bool // can draw a separate transparent layer
	Video         bool // can play a device-side video stream
}

// Driver is the registration record of one device family.
type Driver struct {
	ID        string // stable, lower case, e.g. "tllcd"
	Name      string // display name for lists
	Vendor    string // neutral maker label (USB vendor id)
	Kind      Kind
	Models    []Model
	Transport Transport
	// Experimental drivers are untested on real hardware. They are only
	// discovered when the user enabled them (see Options).
	Experimental bool
	Caps         Capabilities
	// Order sorts discovery (lower first); ties are broken by ID.
	Order int
	// TracksPaths: every panel of the driver is a DevicePather, so a present
	// device without an open panel can be noticed (see UnopenedDisplays).
	TracksPaths bool

	// Enumerate lists the system paths of the present devices without
	// opening them or sending anything. Used to notice plug and unplug.
	Enumerate func() ([]string, error)
	// Discover opens and initialises every present device (KindDisplay).
	Discover func() ([]Panel, error)
	// DiscoverSensors opens every present device (KindSensor).
	DiscoverSensors func() ([]SensorDevice, error)
}

// Sensor is one reading of a SensorSource.
type Sensor struct {
	Key   string // stable id, e.g. "edgehub/ambient"
	Path  string // human path, e.g. "EdgeHub/Ambient"
	Value float64
	Unit  string // "°C", "RPM"
}

// SensorSource is implemented by panels and sensor devices that can report
// readings. Sensors must not block on device I/O for long; it returns the
// latest known values (possibly none).
type SensorSource interface {
	Sensors() []Sensor
}

// SensorDevice is a read-only device (KindSensor).
type SensorDevice interface {
	SensorSource
	Name() string
	Close() error
}

// Options is the user's choice about experimental drivers.
type Options struct {
	// Experimental is the global switch; off by default.
	Experimental bool
	// Off lists experimental driver IDs the user switched off individually.
	Off map[string]bool
	// NoSerial switches off assigning a serial number to displays that have
	// none (the TL LCD driver); the zero value assigns.
	NoSerial bool
}

var (
	regMu   sync.RWMutex
	drivers []Driver
	opts    Options
)

// Register adds a driver. It is meant for init functions and panics on an
// invalid or duplicate record (a programming error).
func Register(d Driver) {
	if d.ID == "" || d.Name == "" || len(d.Models) == 0 || d.Enumerate == nil {
		panic("panel: incomplete driver " + d.ID)
	}
	switch d.Kind {
	case KindDisplay:
		if d.Discover == nil {
			panic("panel: display driver without Discover: " + d.ID)
		}
	case KindSensor:
		if d.DiscoverSensors == nil {
			panic("panel: sensor driver without DiscoverSensors: " + d.ID)
		}
	default:
		panic("panel: unknown driver kind " + string(d.Kind))
	}
	regMu.Lock()
	defer regMu.Unlock()
	for _, x := range drivers {
		if x.ID == d.ID {
			panic("panel: duplicate driver " + d.ID)
		}
	}
	drivers = append(drivers, d)
	sort.SliceStable(drivers, func(i, j int) bool {
		if drivers[i].Order != drivers[j].Order {
			return drivers[i].Order < drivers[j].Order
		}
		return drivers[i].ID < drivers[j].ID
	})
}

// Drivers returns a copy of all registered drivers, in discovery order.
func Drivers() []Driver {
	regMu.RLock()
	defer regMu.RUnlock()
	return append([]Driver(nil), drivers...)
}

// DriverByID looks a driver up.
func DriverByID(id string) (Driver, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	for _, d := range drivers {
		if d.ID == id {
			return d, true
		}
	}
	return Driver{}, false
}

// SetOptions sets which experimental drivers take part in discovery.
func SetOptions(o Options) {
	cp := Options{Experimental: o.Experimental, NoSerial: o.NoSerial, Off: map[string]bool{}}
	for k, v := range o.Off {
		if v {
			cp.Off[k] = true
		}
	}
	regMu.Lock()
	opts = cp
	regMu.Unlock()
}

// CurrentOptions returns the options in effect.
func CurrentOptions() Options {
	regMu.RLock()
	defer regMu.RUnlock()
	cp := Options{Experimental: opts.Experimental, Off: map[string]bool{}}
	for k, v := range opts.Off {
		cp.Off[k] = v
	}
	return cp
}

// Enabled reports whether discovery includes d: stable drivers always,
// experimental ones only when switched on.
func Enabled(d Driver) bool {
	if !d.Experimental {
		return true
	}
	regMu.RLock()
	defer regMu.RUnlock()
	return opts.Experimental && !opts.Off[d.ID]
}

func enabledDrivers(kind Kind) []Driver {
	var out []Driver
	for _, d := range Drivers() {
		if d.Kind == kind && Enabled(d) {
			out = append(out, d)
		}
	}
	return out
}

// IsExperimental reports whether p comes from an experimental driver
// (regardless of the current options).
func IsExperimental(p Panel) bool {
	if dp, ok := p.(interface{ DriverID() string }); ok {
		d, found := DriverByID(dp.DriverID())
		return found && d.Experimental
	}
	return false
}

// Discover finds and initialises every panel of the enabled drivers. A
// family that fails doesn't hide the others; errors are only returned when
// nothing was found.
func Discover() ([]Panel, error) {
	var out []Panel
	var errs []error
	for _, d := range enabledDrivers(KindDisplay) {
		ps, err := d.Discover()
		out = append(out, ps...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.ID, err))
		}
	}
	if len(out) == 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

// DiscoverSensors opens the read-only devices of the enabled sensor drivers.
func DiscoverSensors() ([]SensorDevice, error) {
	var out []SensorDevice
	var errs []error
	for _, d := range enabledDrivers(KindSensor) {
		ss, err := d.DiscoverSensors()
		out = append(out, ss...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.ID, err))
		}
	}
	if len(out) == 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

// Fingerprint lists the supported devices of the enabled drivers that are
// currently plugged in as one string (their system paths, sorted). It only
// enumerates; no device is opened or spoken to, so it is cheap and safe to
// call every few seconds. A changed fingerprint means a device arrived or
// left.
func Fingerprint() (string, error) {
	var paths []string
	var firstErr error
	for _, d := range Drivers() {
		if !Enabled(d) {
			continue
		}
		ps, err := d.Enumerate()
		if err != nil && firstErr == nil {
			firstErr = err
		}
		paths = append(paths, ps...)
	}
	sort.Strings(paths)
	return strings.ToLower(strings.Join(paths, "\n")), firstErr
}

// DevicePather is implemented by panels that know the system path of their
// device; drivers with TracksPaths set only build such panels.
type DevicePather interface {
	DevicePath() string
}

// enumerateDisplays is the seam for UnopenedDisplays (tests replace it).
var enumerateDisplays = func() []Driver {
	var out []Driver
	for _, d := range Drivers() {
		if d.Kind == KindDisplay && d.TracksPaths && Enabled(d) {
			out = append(out, d)
		}
	}
	return out
}

// UnopenedDisplays lists the present devices of the path-tracking display
// drivers that none of the open panels uses: a display that came back but
// was not (or no longer is) opened. It only enumerates, like Fingerprint.
func UnopenedDisplays(open []Panel) []string {
	have := map[string]bool{}
	for _, p := range open {
		if dp, ok := p.(DevicePather); ok {
			have[strings.ToLower(dp.DevicePath())] = true
		}
	}
	var out []string
	for _, d := range enumerateDisplays() {
		ps, _ := d.Enumerate()
		for _, s := range ps {
			if !have[strings.ToLower(s)] {
				out = append(out, s)
			}
		}
	}
	return out
}
