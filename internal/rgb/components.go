package leds

import (
	"fmt"
	"sort"
	"sync"
)

// Component is the LED layout of a piece of hardware (fan, strip, case panel):
// LED positions on a Width×Height grid, in wire order.
type Component struct {
	Name   string  `json:"name"` // ID used in Segment.Component, e.g. "fan-tl-26"
	Label  string  `json:"label"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	LEDs   []Point `json:"leds"` // grid cells (0..Width-1, 0..Height-1)
	// SizeMM is the outer size of the part in millimetres (0 = unknown); the
	// grid spans exactly this size, so LED positions convert with LEDsMM.
	SizeMM Point `json:"sizeMm"`
	// Outline is the drawn shape: "rounded-square" (fan), "circle", "bar"
	// (strip), "rect".
	Outline string `json:"outline,omitempty"`
	// PositionsMM marks LEDs given in millimetres from the top left corner
	// (library parts) instead of grid cells; SizeMM is then required.
	PositionsMM bool `json:"positionsMm,omitempty"`
	// Zones optionally split the LEDs (contiguous, in wire order).
	Zones []ComponentZone `json:"zones,omitempty"`
}

// ComponentZone is a named run of LEDs inside a part.
type ComponentZone struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	First int    `json:"first"`
	Count int    `json:"count"`
}

// unit maps an LED to 0..1 within the part.
func (c Component) unit(p Point) (float64, float64) {
	if c.PositionsMM && c.SizeMM.X > 0 && c.SizeMM.Y > 0 {
		return p.X / c.SizeMM.X, p.Y / c.SizeMM.Y
	}
	return (p.X + 0.5) / float64(c.Width), (p.Y + 0.5) / float64(c.Height)
}

// LEDsMM returns the LED centres in millimetres from the top left corner of
// the part (nil if the size is unknown).
func (c Component) LEDsMM() []Point {
	if c.SizeMM.X <= 0 || c.SizeMM.Y <= 0 {
		return nil
	}
	if c.PositionsMM {
		return append([]Point(nil), c.LEDs...)
	}
	if c.Width <= 0 || c.Height <= 0 {
		return nil
	}
	out := make([]Point, len(c.LEDs))
	for i, p := range c.LEDs {
		x, y := c.unit(p)
		out[i] = Point{X: x * c.SizeMM.X, Y: y * c.SizeMM.Y}
	}
	return out
}

var (
	componentsMu sync.RWMutex
	components   = map[string]Component{}
)

func init() {
	for _, c := range builtinComponents() {
		components[c.Name] = c
	}
}

// LookupComponent returns a registered component.
func LookupComponent(name string) (Component, bool) {
	componentsMu.RLock()
	defer componentsMu.RUnlock()
	c, ok := components[name]
	return c, ok
}

// Components lists the registered components sorted by name.
func Components() []Component {
	componentsMu.RLock()
	defer componentsMu.RUnlock()
	out := make([]Component, 0, len(components))
	for _, c := range components {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RegisterComponent adds or replaces a component. Layouts using it pick it up
// on the next layout rebuild.
func RegisterComponent(c Component) error {
	sized := (c.PositionsMM && c.SizeMM.X > 0 && c.SizeMM.Y > 0) || (c.Width > 0 && c.Height > 0)
	if c.Name == "" || !sized || len(c.LEDs) == 0 {
		return fmt.Errorf("leds: invalid component %q", c.Name)
	}
	componentsMu.Lock()
	defer componentsMu.Unlock()
	components[c.Name] = c
	return nil
}

// builtinComponents covers the hardware of the reference setup.
func builtinComponents() []Component {
	return []Component{tlFan(), lineStrip("strip-47", "Front strip (47 LEDs)", 47)}
}

// tlFan models a 120 mm fan with two light bars, one along the bottom and one
// along the top edge of the frame, 13 LEDs each. Measured wiring: the bottom
// bar first, left to right, then the top bar, left to right. Positions are on
// a 100x100 grid of the fan front.
func tlFan() Component {
	const perBar = 13
	// 120 x 120 mm frame (manufacturer size of the 120 mm model)
	c := Component{Name: "fan-tl-26", Label: "Fan with top and bottom light bar (26 LEDs)", Width: 100, Height: 100,
		SizeMM: Point{X: 120, Y: 120}, Outline: "rounded-square"}
	for _, y := range []float64{94, 5} { // bottom bar, then top bar
		for i := 0; i < perBar; i++ {
			x := 6 + 87*float64(i)/(perBar-1)
			c.LEDs = append(c.LEDs, Point{X: x, Y: y})
		}
	}
	return c
}

// lineStrip is a straight strip of n LEDs.
func lineStrip(name, label string, n int) Component {
	c := Component{Name: name, Label: label, Width: n, Height: 1, Outline: "bar"}
	for i := 0; i < n; i++ {
		c.LEDs = append(c.LEDs, Point{X: float64(i), Y: 0})
	}
	return c
}
