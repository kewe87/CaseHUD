package sketch

import (
	"math"
	"sort"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// ZoneSegments are the component segments of one (device, zone), in wire
// order, ready for ZoneConfig.Segments.
type ZoneSegments struct {
	Device   string         `json:"device"`
	Zone     string         `json:"zone"`
	Segments []leds.Segment `json:"segments"`
	// LEDs is the number of LEDs the chain drives.
	LEDs int `json:"leds"`
}

// Derived is the LED engine view of a sketch.
type Derived struct {
	// Components has one component "sketch.<placementID>" per placement,
	// in placement order: LED positions in mm from the top left of the
	// placement's bounding box in the canvas view (Component.PositionsMM).
	Components []leds.Component
	// Zones has the segments of every bound (device, zone), sorted by
	// device and zone.
	Zones []ZoneSegments
}

// Letterbox fits a box of bw x bh mm into the LED canvas keeping the aspect
// ratio and centring it. It returns the normalised (0..1) canvas offset of
// the box origin and the normalised size of one millimetre on each axis.
func Letterbox(bw, bh float64) (ox, oy, sx, sy float64) {
	if !(bw > 0) || !(bh > 0) {
		return 0, 0, 0, 0
	}
	cw, ch := float64(leds.CanvasWidth), float64(leds.CanvasHeight)
	scale := math.Min(cw/bw, ch/bh) // pixels per mm
	ox = (cw - bw*scale) / 2 / cw
	oy = (ch - bh*scale) / 2 / ch
	return ox, oy, scale / cw, scale / ch
}

// minExtentMM is the smallest size a component gets along an axis it is
// collapsed on (the engine needs a positive size).
const minExtentMM = 0.01

// Derive validates the sketch and builds components and zone segments.
//
// Every placement becomes a component. A bound placement also becomes one
// segment (shape "component", normalised canvas box) in the chain of its
// (device, zone). The chain is sorted by Order (ties keep the sketch order);
// each segment has an explicit start index: behind the previous part, or, for
// Parallel, the same start as the previous part (an LED driven twice takes
// its canvas position from the first).
//
// The canvas shows the sketch's canvasView, letterboxed. Parts placed in
// another view are mapped into it through the 3D coordinates of the case
// (MapToView), so parts of different views end up where they really are
// relative to each other. A component's LED positions are millimetres from
// the top left of its bounding box in the canvas view. Parts up to MarginMM
// outside the case, or larger than it, would leave the canvas: the segment
// box is clamped to it.
func Derive(s Sketch, c Catalog) (Derived, error) {
	if err := Validate(s, c); err != nil {
		return Derived{}, err
	}
	type bound struct {
		idx int
		seg leds.Segment
		n   int
	}
	var out Derived
	chains := map[[2]string][]bound{}
	cv := s.CanvasViewOf()
	cw, ch, _ := ViewSize(s.Case.Dims, cv)
	ox, oy, sx, sy := Letterbox(cw, ch)
	for i, p := range s.Parts {
		part := c.Parts[p.Part]
		pr, err := Project(part, p)
		if err != nil {
			return Derived{}, err
		}
		label := p.Label
		if label == "" {
			label = p.ID
		}
		view := p.ViewOf()
		toCanvas := func(x, y float64) Point {
			u, v := MapToView(s.Case.Dims, view, cv, x, y)
			return Point{X: u, Y: v}
		}
		a, b := toCanvas(pr.Box.X, pr.Box.Y), toCanvas(pr.Box.X+pr.Box.W, pr.Box.Y+pr.Box.H)
		box := Rect{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y), W: math.Abs(a.X - b.X), H: math.Abs(a.Y - b.Y)}
		bw, bh := math.Max(box.W, minExtentMM), math.Max(box.H, minExtentMM)
		comp := leds.Component{
			Name: ComponentName(p.ID), Label: label,
			Width: int(math.Ceil(bw)), Height: int(math.Ceil(bh)),
			SizeMM: leds.Point{X: bw, Y: bh}, Outline: "rect", PositionsMM: true,
		}
		for _, q := range pr.LEDs {
			m := toCanvas(q.X, q.Y)
			comp.LEDs = append(comp.LEDs, leds.Point{
				X: clamp(round6(m.X-box.X), 0, bw), Y: clamp(round6(m.Y-box.Y), 0, bh)})
		}
		out.Components = append(out.Components, comp)
		if p.Bind == nil {
			continue
		}
		seg := leds.Segment{
			Name: label, Shape: "component", Component: comp.Name,
			X: clamp(round6(ox+box.X*sx), 0, 1), Y: clamp(round6(oy+box.Y*sy), 0, 1),
			W: clamp(round6(bw*sx), 0, 1), H: clamp(round6(bh*sy), 0, 1),
		}
		key := [2]string{p.Bind.Device, p.Bind.Zone}
		chains[key] = append(chains[key], bound{idx: i, seg: seg, n: len(pr.LEDs)})
	}
	keys := make([][2]string, 0, len(chains))
	for k := range chains {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a][0] != keys[b][0] {
			return keys[a][0] < keys[b][0]
		}
		return keys[a][1] < keys[b][1]
	})
	for _, k := range keys {
		chain := chains[k]
		sort.SliceStable(chain, func(a, b int) bool {
			return s.Parts[chain[a].idx].Bind.Order < s.Parts[chain[b].idx].Bind.Order
		})
		zs := ZoneSegments{Device: k[0], Zone: k[1]}
		next, prevStart := 0, 0
		for _, b := range chain {
			start := next
			if s.Parts[b.idx].Bind.Parallel && len(zs.Segments) > 0 {
				start = prevStart
			}
			st := start
			b.seg.Start = &st
			zs.Segments = append(zs.Segments, b.seg)
			prevStart = start
			next = max(next, start+b.n)
		}
		zs.LEDs = next
		out.Zones = append(out.Zones, zs)
	}
	return out, nil
}
