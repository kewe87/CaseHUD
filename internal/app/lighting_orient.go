package app

import (
	"errors"
	"fmt"
	"image/color"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// The orientation check: a still pattern on one bound part (sketch.Pattern)
// through the same path as the calibration marker, for lightCalibTimeout
// at most. Nothing is stored and nothing is written to a device's memory;
// DELETE /api/lighting/calibrate (LightingEndCalibration) ends it earlier.
// What the user reports (turned, mirrored, reversed) corrects the part's
// spin, mirror or direction in the sketch (sketch.Orient), and the pattern
// is shown again.

// orientErr makes a refused check a request error, with its code in front.
func orientErr(err error) error {
	var oe *sketch.OrientError
	if errors.As(err, &oe) {
		return fmt.Errorf("%w: %s: %s", ErrLighting, oe.Code, oe.Msg)
	}
	return lightErr("%v", err)
}

// resolvedForOrient is the stored sketch as Derive sees it (nil: none).
func (e *Engine) resolvedForOrient() (*sketch.Sketch, sketch.Catalog) {
	sk := e.GetSketch()
	if sk == nil {
		return nil, sketch.Catalog{}
	}
	cat := e.SketchCatalog()
	s := sketch.ResolveFaces(sketch.SyncCaseLights(*sk, cat), cat)
	return &s, cat
}

// showPattern puts the pattern on its zone. A part in parallel shows the
// LEDs of another part: it gets nothing of its own.
func (e *Engine) showPattern(p sketch.PartPattern) error {
	if p.Parallel {
		return nil
	}
	cols := make([]color.RGBA, p.Start+len(p.Colors))
	for i := range cols {
		cols[i] = color.RGBA{A: 255}
	}
	for k, h := range p.Colors {
		c, err := leds.ParseHexColor(h)
		if err != nil {
			return lightErr("pattern: %v", err)
		}
		cols[p.Start+k] = c
	}
	c := LightingCalib{Device: p.Device, Zone: p.Zone, Part: p.Part, Start: p.Start, Pattern: p.Colors}
	return e.lightingMarker(p.Device, p.Zone, c, lightCalibTimeout,
		func(eng *leds.Engine) error { return eng.Pattern(p.Device, p.Zone, cols) })
}

// LightingPartPattern shows the check pattern on a bound part of the stored
// sketch. A part that is not connected has none; a part in parallel is
// reported (Parallel, Model) but shows nothing of its own.
func (e *Engine) LightingPartPattern(id string) (sketch.PartPattern, error) {
	if !e.lightingOn() {
		return sketch.PartPattern{}, ErrAreaOff
	}
	l := &e.light
	l.sketchMu.Lock()
	defer l.sketchMu.Unlock()
	s, cat := e.resolvedForOrient()
	if s == nil {
		return sketch.PartPattern{}, lightErr("no sketch")
	}
	p, err := sketch.Pattern(*s, cat, sketchRings, id)
	if err != nil {
		return sketch.PartPattern{}, orientErr(err)
	}
	return p, e.showPattern(p)
}

// LightingOrient corrects a bound part from what its pattern showed (fix:
// sketch.FixRot90 and so on), stores the sketch and shows the pattern again.
func (e *Engine) LightingOrient(id, fix string) (SketchView, sketch.PartPattern, error) {
	if !e.lightingOn() {
		return SketchView{}, sketch.PartPattern{}, ErrAreaOff
	}
	l := &e.light
	l.sketchMu.Lock()
	defer l.sketchMu.Unlock()
	s, cat := e.resolvedForOrient()
	if s == nil {
		return SketchView{}, sketch.PartPattern{}, lightErr("no sketch")
	}
	o, err := sketch.Orient(*s, cat, sketchRings, id, fix)
	if err != nil {
		return SketchView{}, sketch.PartPattern{}, orientErr(err)
	}
	v, err := e.setSketchLocked(o)
	if err != nil {
		return SketchView{}, sketch.PartPattern{}, err
	}
	s, cat = e.resolvedForOrient()
	p, err := sketch.Pattern(*s, cat, sketchRings, id)
	if err != nil {
		return v, sketch.PartPattern{}, orientErr(err)
	}
	if err := e.showPattern(p); err != nil {
		return v, p, err
	}
	v.Lighting = e.Lighting()
	return v, p, nil
}
