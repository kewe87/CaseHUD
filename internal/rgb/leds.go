// Package leds drives addressable RGB hardware from a shared 2D canvas.
//
// An Effect renders into a fixed-size canvas. Every LED of every device has a
// position on that canvas (derived from the zone layout in Config); the engine
// samples the canvas at those positions and sends one frame per device.
package leds

import (
	"context"
	"image"
	"image/color"
	"time"
)

// Canvas size effects render into. LED positions are normalized (0..1), so the
// size only affects effect resolution, not layouts.
const (
	CanvasWidth  = 320
	CanvasHeight = 200
)

// Point is a position in normalized canvas coordinates (0..1 on both axes).
type Point struct{ X, Y float64 }

// Zone is one independently addressed group of LEDs on a device, e.g. an ARGB
// header or the onboard LEDs of a mainboard.
type Zone struct {
	ID   string
	Name string
	// Count is the number of LEDs the device reports. Zero for headers whose
	// length is unknown; the user configures it.
	Count int
	// Max is the largest LED count the zone accepts (equal to Count for fixed zones).
	Max int
	// Resizable zones take their LED count from the configuration.
	Resizable bool
	// Order is the wire color order the hardware expects by default ("RGB", "GRB", ...).
	Order string
}

// Device is one physical controller.
//
// A device returned by Discover must not have changed the hardware state yet:
// it initializes lazily on the first SetFrame, and Close on a device that never
// received a frame must leave the hardware alone. Discovery can therefore run
// repeatedly (hot-plug) without disturbing devices already in use.
//
// SetFrame is called from one goroutine per device (never concurrently for the
// same device), with one color slice per zone ID (already brightness-scaled and
// in wire color order). A slow device only delays itself: while it is busy,
// newer frames replace the waiting one. Implementations
// must push everything in one go so that zones of a controller never tear.
type Device interface {
	ID() string // stable across restarts, e.g. "asusaura:0b05:1aa6"
	Name() string
	Zones() []Zone
	SetFrame(frame map[string][]color.RGBA) error
	// Conflicts lists process names (e.g. "ArmouryCrate.exe") that drive the
	// same hardware. While one is running the engine does not touch the device.
	Conflicts() []string
	// Close releases handles. The engine sends the shutdown color as a last
	// frame before, so devices need not change colors here.
	Close() error
}

// Configurable is implemented by devices with device-specific options (e.g. a
// hardware effect instead of the canvas). Configure is called from the engine
// goroutine before the first frame and whenever the device configuration
// changes, with every declared key present (defaults filled in).
type Configurable interface {
	Settings() []Param
	Configure(p Params) error
}

// FirmwareReporter is implemented by devices that know their firmware
// version. The value is read during discovery (read-only queries, nothing is
// written to the device); "" means unknown.
type FirmwareReporter interface {
	Firmware() string
}

// Backend discovers devices of one kind (ASUS Aura USB, ENE RAM, ...).
type Backend interface {
	Name() string
	Discover(ctx context.Context) ([]Device, error)
}

// Sensors maps CaseHUD sensor paths (as used in the design JSONs) to values.
type Sensors map[string]float64

// Frame is the input of one effect render.
type Frame struct {
	T       time.Duration // time since the effect started
	Dt      time.Duration // time since the previous frame
	N       uint64        // frame index since the effect started
	Sensors Sensors
	Accent  color.RGBA
}

// Effect renders into the canvas. Render is only ever called from the engine
// goroutine, so effects may keep state without locking. The canvas is reused
// between frames and is not cleared by the engine.
//
// Effects that hold resources (screen capture, decoders, audio) implement
// io.Closer; the engine closes them when they are replaced or the engine
// stops. Render must not block: such effects produce frames in the background
// and Render only copies the latest one.
type Effect interface {
	Render(f Frame, c *image.RGBA)
}

// ParamType is the kind of an effect parameter, used to build the GUI.
type ParamType string

const (
	ParamNumber ParamType = "number"
	ParamColor  ParamType = "color" // "#rrggbb"
	ParamBool   ParamType = "bool"
	ParamEnum   ParamType = "enum"
	ParamText   ParamType = "text" // free text, e.g. a file path
)

// Param describes one effect parameter.
type Param struct {
	Key     string    `json:"key"`
	Label   string    `json:"label"`
	Type    ParamType `json:"type"`
	Min     float64   `json:"min,omitempty"`
	Max     float64   `json:"max,omitempty"`
	Default any       `json:"default"`
	Values  []string  `json:"values,omitempty"` // for ParamEnum
}

// EffectInfo describes an effect and creates instances of it.
type EffectInfo struct {
	Name   string  `json:"name"`
	Label  string  `json:"label"`
	Params []Param `json:"params"`
	// New creates an effect. params contains every declared key (defaults filled in).
	New func(params Params) (Effect, error) `json:"-"`
}

// DeviceState is the coarse status of a device for the UI.
type DeviceState string

const (
	StateOK       DeviceState = "ok"
	StateConflict DeviceState = "conflict" // another program drives this hardware
	StateDisabled DeviceState = "disabled" // switched off in the configuration
	StateError    DeviceState = "error"
)

// DeviceStatus is what Engine.Devices reports per device.
type DeviceStatus struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Backend string      `json:"backend"`
	State   DeviceState `json:"state"`
	Message string      `json:"message,omitempty"`
	Zones   []Zone      `json:"zones"`
	// Firmware is the device's firmware version (FirmwareReporter), "" if unknown.
	Firmware string `json:"firmware,omitempty"`
	// Settings describes the device-specific options (Configurable devices).
	Settings []Param `json:"settings,omitempty"`
}
