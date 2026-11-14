// Package sketch is the pure model behind the case sketch: a simplified 3D
// model of a PC case (outer box, optional chamber divider) with parts (fans,
// memory, strips, ...), each bound to an LED zone of a device.
//
// The package has no GUI, API or storage code. It defines the sketch and part
// catalog types, validates a sketch, projects a part's LEDs into one of five
// views or onto the unfolded case net in millimetres, reports overlaps and derives LED engine components
// and zone segments from it.
//
// Case box, millimetres, outer size W x H x D (width, height, depth):
//
//	x  across the width, 0 at the glass side
//	y  down the height, 0 at the top
//	z  into the depth, 0 at the rear, D at the front
//
// The model is glass-relative: Case.Orientation says on which physical side
// of the case the glass is and which end the side view shows on its left, so
// the views can be labelled and drawn like the real case.
//
// A part is described like a library component: the LED side (w x h) with LED
// positions in wire order, and a depth d. A placement puts the part's centre
// at Pos, turns the LED side towards a Facing and spins it clockwise (seen
// from the facing side) around its centre.
//
// Sketch format version 1 (three flat views) is read and upgraded to this
// model by Upgrade.
package sketch

// Version is the sketch format version.
const Version = 2

// Views. Seite looks at the glass side, Vorne at the front, Hinten at the
// rear, Oben down on the top and Unten up at the bottom of the case. The
// version 1 names front, side and top keep their meaning.
const (
	ViewFront  = "front"
	ViewRear   = "rear"
	ViewSide   = "side"
	ViewTop    = "top"
	ViewBottom = "bottom"
)

// Facings: the case face a part's LED side points to. Side is the glass side
// (-x), tray the opposite side (+x).
const (
	FacingFront  = "front"
	FacingRear   = "rear"
	FacingSide   = "side"
	FacingTray   = "tray"
	FacingTop    = "top"
	FacingBottom = "bottom"
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
	MaxLayer        = 100
)

// Sketch is the stored case sketch.
type Sketch struct {
	Version int  `json:"version"`
	Case    Case `json:"case"`
	// CanvasView is the canvas mapping: "net" (the unfolded case, see
	// NetLayout). Older sketches store a view name or ""; every value maps to
	// the net, so they keep working unchanged.
	CanvasView string      `json:"canvasView"`
	Parts      []Placement `json:"parts"`
}

// Case is the case box the parts are placed in.
type Case struct {
	Source   string    `json:"source"` // "custom" or "library"
	ID       string    `json:"id"`
	Dims     Dims      `json:"dims"`
	Chambers []Chamber `json:"chambers,omitempty"`
	// Orientation tells which way round the case is built; the zero value
	// means the default (see Orientation).
	Orientation Orientation `json:"orientation,omitzero"`
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

// Vec3 is a point or vector in the case box, millimetres.
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Placement is one part placed in the case.
type Placement struct {
	ID   string `json:"id"`
	Part string `json:"part"` // catalog part id
	// Pos is the centre of the part's box.
	Pos Vec3 `json:"pos"`
	// Facing is the case face the part's LED side points to.
	Facing string `json:"facing"`
	// Face is the face of the case net the part is shown on (a facing name);
	// "" means the default (DefaultFace). Every part belongs to one face.
	Face string `json:"face,omitempty"`
	// Spin is the clockwise rotation in degrees (-360..360) around the
	// centre, seen from the facing side.
	Spin float64 `json:"spin"`
	// Mirror flips the part horizontally (before the spin).
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
	// On is the id of the placement this part is mounted on (one level; the
	// pair may overlap, see Overlaps). A moved parent takes its children along.
	On string `json:"on,omitempty"`
	// Group welds placements together (editor): ^[a-z0-9-]{1,16}$.
	Group string `json:"group,omitempty"`
	// Layer orders drawing (-100..100, higher is drawn later).
	Layer int `json:"layer,omitempty"`
	// Hidden hides the part in the editor only; it still counts everywhere else.
	Hidden bool `json:"hidden,omitempty"`
	// CaseLight marks lighting built into the case: the id of its entry in
	// the case template (see SyncCaseLights). Such a placement has a Path
	// instead of a position; Mirror reverses its LED order.
	CaseLight string `json:"caseLight,omitempty"`
	// Path is the polyline of a case light in the case box, in wire order
	// (only with CaseLight). Pos and Facing then follow its first point.
	Path []Vec3 `json:"path,omitempty"`
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
