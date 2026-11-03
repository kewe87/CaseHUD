// Package sketch is the pure model behind the case sketch: a simplified
// drawing of a PC case (outer box, optional chamber divider) with parts
// (fans, memory, strips, ...) placed in a front, side or top view, each bound
// to an LED zone of a device.
//
// The package has no GUI, API or storage code. It defines the sketch and part
// catalog types, validates a sketch, projects a part's LEDs into a view in
// millimetres and derives LED engine components and zone segments from it.
//
// Coordinates are millimetres, x to the right, y down, origin at the top left
// of a view. Views of a case with outer size W x H x D (width, height, depth):
//
//	front  W x H   looking at the case front
//	side   D x H   looking at the window side, case front on the left
//	top    W x D   looking down, case front at the top
//
// A part is described like a library component: the front view (w x h, the
// LED side) with LED positions in wire order, and a depth d.
package sketch

// Version is the sketch format version.
const Version = 1

// Views and faces share the names front, side and top.
const (
	ViewFront = "front"
	ViewSide  = "side"
	ViewTop   = "top"
)

// ReservedPrefix starts the names of derived components; catalog parts must
// not use it.
const ReservedPrefix = "sketch."

// Limits.
const (
	MaxParts     = 64
	MaxLabel     = 40
	MaxLEDs      = 512  // per part
	MaxTotalLEDs = 4096 // per sketch
	MaxDimMM     = 2000
	MaxChambers  = 4
	// MaxZoneSegments is the most parts one (device, zone) chain may have: the
	// LED engine accepts at most this many segments per zone.
	MaxZoneSegments = 16
	MarginMM        = 50 // parts may sit this far outside the view box
	MaxBodyBytes    = 1 << 20
	maxOrder        = 1000
	maxText         = 128
)

// Sketch is the stored case sketch.
type Sketch struct {
	Version    int         `json:"version"`
	Case       Case        `json:"case"`
	CanvasView string      `json:"canvasView"` // view mapped to the LED canvas (all parts, see MapToView); "" = side
	Parts      []Placement `json:"parts"`
}

// Case is the case box the parts are placed on.
type Case struct {
	Source   string    `json:"source"` // "custom" or "library"
	ID       string    `json:"id"`
	Dims     Dims      `json:"dims"`
	Chambers []Chamber `json:"chambers,omitempty"`
}

// Dims is the outer size in millimetres: width, height, depth.
type Dims struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
	D float64 `json:"d"`
}

// Chamber is a vertical compartment: x offset and width along the case width.
type Chamber struct {
	X float64 `json:"x"`
	W float64 `json:"w"`
}

// Placement is one part placed in a view.
type Placement struct {
	ID   string `json:"id"`
	Part string `json:"part"` // catalog part id
	// View holds X and Y: the top left corner of the part's bounding box
	// (after rotation) in that view. "" = side.
	View string  `json:"view"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	// Rot is the clockwise rotation in degrees (-360..360) in the view plane.
	Rot float64 `json:"rot"`
	// Face is the case face the part's LED side points to; "" = the view's own
	// name, i.e. the part is seen flat-on. Any other face shows the part edge-on.
	Face string `json:"face"`
	// Mirror flips the part horizontally (before rotation).
	Mirror bool `json:"mirror"`
	// Flow of a fan: "intake" or "exhaust" (or ""). Exhaust is a fan turned
	// around: its LED order runs the other way round (spatial mirror).
	Flow string `json:"flow"`
	// LEDs is the LED count of a parametric part (0 = the part's default).
	LEDs int `json:"leds"`
	// Len is the length in mm of a part with a length parameter (strips, 0 = default).
	Len   float64  `json:"len,omitempty"`
	Label string   `json:"label"`
	Bind  *Binding `json:"bind,omitempty"`
}

// Binding connects a placement to an LED zone of a device.
type Binding struct {
	Device string `json:"device"`
	Zone   string `json:"zone"`
	// Order is the position in the zone's chain (ascending).
	Order int `json:"order"`
	// Parallel wires the part in parallel with the previous part of the chain
	// (same first LED, same colours) instead of behind it.
	Parallel bool `json:"parallel"`
}

// Point is a position in millimetres.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Rect is a rectangle in millimetres.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}
