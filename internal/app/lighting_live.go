package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// ErrAreaOff marks a request that needs the lighting area while it is
// switched off in the settings.
var ErrAreaOff = errors.New("area_off: the lighting area is switched off in the CaseHUD settings")

// LightingLive is the colour of every LED as last sent to the devices: per
// device and zone the bytes R, G, B of each LED in wire order, base64.
type LightingLive struct {
	// Active: the LED engine runs (otherwise Devices is empty).
	Active  bool                         `json:"active"`
	Devices map[string]map[string]string `json:"devices"`
	// Canvas is the effect picture, only when asked for (canvas=1).
	Canvas *LightingCanvas `json:"canvas,omitempty"`
}

// LightingCanvas is the effect canvas at reduced size, brightness applied.
type LightingCanvas struct {
	W int `json:"w"`
	H int `json:"h"`
	// PNG is the picture (base64); empty when there is none (see Note).
	PNG string `json:"png,omitempty"`
	// Source: "engine" (what the devices get) or "preview" (the effect
	// rendered for the page while the engine does not run).
	Source string `json:"source"`
	// Effect is the effect shown.
	Effect string `json:"effect"`
	// Note says why there is no picture: "capture" (screen and audio effects
	// are not previewed), "starting" (the first picture is being prepared)
	// or "error".
	Note string `json:"note,omitempty"`
}

// lightingCanvas returns the effect picture: the engine's canvas while it
// runs, otherwise a preview of the configured effect. Capture effects are
// never started for a preview.
func (e *Engine) lightingCanvas(eng *leds.Engine) *LightingCanvas {
	l := &e.light
	l.mu.Lock()
	cfg := l.cfg
	effect, bright := cfg.Effect, cfg.Brightness*256/100
	params := cfg.Params[effect]
	sig, _ := json.Marshal(params)
	l.mu.Unlock()
	out := &LightingCanvas{W: leds.PreviewWidth, H: leds.PreviewHeight, Effect: effect}
	var img *image.RGBA
	if eng != nil {
		out.Source = "engine"
		if img, _ = eng.CanvasPreview(); img == nil {
			out.Note = "starting"
		}
	} else {
		out.Source = "preview"
		info, ok := lightingEffect(effect)
		switch {
		case isCaptureEffect(effect):
			l.preview.Close()
			out.Note = "capture"
		case !ok || info.New == nil:
			out.Note = "error"
		default:
			l.previewOnce.Do(func() { l.previewSensors = e.lightSensors() })
			var err error
			img, err = l.preview.Frame(info, params, effect+"|"+string(sig), l.previewSensors(), bright, time.Now())
			if err != nil {
				out.Note = "error"
			}
		}
	}
	if img != nil {
		out.W, out.H = img.Rect.Dx(), img.Rect.Dy() // larger for sharp effects (leds.PreviewWidth)
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if enc.Encode(&buf, img) == nil {
			out.PNG = base64.StdEncoding.EncodeToString(buf.Bytes())
		}
	}
	return out
}

// LightingLive returns the current LED colours (a copy of the engine's last
// frame) and, with canvas, the effect picture (see LightingCanvas). It fails
// with ErrAreaOff while the lighting area is off.
func (e *Engine) LightingLive(canvas bool) (LightingLive, error) {
	if !e.lightingOn() {
		return LightingLive{}, ErrAreaOff
	}
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	out := LightingLive{Devices: map[string]map[string]string{}}
	if canvas {
		out.Canvas = e.lightingCanvas(eng)
	}
	if eng == nil {
		return out, nil
	}
	out.Active = true
	for dev, zones := range eng.Snapshot() {
		m := make(map[string]string, len(zones))
		for zone, cols := range zones {
			buf := make([]byte, 0, len(cols)*3)
			for _, c := range cols {
				buf = append(buf, c.R, c.G, c.B)
			}
			m[zone] = base64.StdEncoding.EncodeToString(buf)
		}
		out.Devices[dev] = m
	}
	return out, nil
}

// SketchSuggest proposes placements for the detected hardware, for the case
// of the stored sketch (the built-in box if there is none). Nothing is stored.
func (e *Engine) SketchSuggest() (sketch.Suggestion, error) {
	if !e.lightingOn() {
		return sketch.Suggestion{}, ErrAreaOff
	}
	cat := e.SketchCatalog()
	sk := e.GetSketch()
	var existing sketch.Sketch
	k := cat.Cases[sketch.CustomCaseID].Dims
	cs := sketch.Case{Source: "custom", ID: sketch.CustomCaseID, Dims: k}
	if sk != nil {
		existing, cs = *sk, sk.Case
	}
	in := sketch.SuggestInput{Existing: existing}
	in.Devices, in.Fans = e.foundLightingHW()
	sug := sketch.Suggest(cat, cs, in)
	sug.Parts = sketch.ResolveFaces(sketch.Sketch{Case: cs, Parts: sug.Parts}, cat).Parts
	return sug, nil
}

// foundLightingHW is the found lighting hardware as the sketch sees it:
// the LED devices with their zones and the fans of the TL controller.
func (e *Engine) foundLightingHW() ([]sketch.DetectedDevice, []sketch.DetectedFan) {
	var devs []sketch.DetectedDevice
	var fans []sketch.DetectedFan
	for _, d := range e.Lighting().Devices {
		dd := sketch.DetectedDevice{ID: d.ID, Backend: d.Backend}
		if hasSetting(d.Settings, "syncSource") { // the fan controller: whom it follows
			dd.Mode = d.SettingValues.String("mode")
			dd.SyncSource, _ = d.SettingValues["syncSource"].(string)
		}
		for _, z := range d.Zones {
			dd.Zones = append(dd.Zones, sketch.DetectedZone{ID: z.ID, Count: z.Count, Max: z.Max})
		}
		devs = append(devs, dd)
	}
	for _, p := range e.Arrangement().Ports {
		for _, f := range p.Fans {
			fans = append(fans, sketch.DetectedFan{Port: f.Port, Index: f.Index, LCD: f.HasLCD})
		}
	}
	return devs, fans
}

// SketchAutoBind binds the parts of the stored sketch whose LED zone is
// certain (sketch.AutoBind) and reports what it left open. only limits the
// bindings to those parts (nil: all). syncSource is the mainboard header
// ("device/zone") the user confirmed for a fan controller that follows the
// mainboard without a header set ("" : none); the controller's own setting
// is not changed. With apply the bindings are stored (only when there are
// any) and the view of the stored sketch comes along; otherwise nothing
// changes. No sketch: nothing to do.
func (e *Engine) SketchAutoBind(apply bool, only []string, syncSource string) (sketch.AutoBindResult, *SketchView, error) {
	if !e.lightingOn() {
		return sketch.AutoBindResult{}, nil, ErrAreaOff
	}
	l := &e.light
	l.sketchMu.Lock()
	defer l.sketchMu.Unlock()
	sk := e.GetSketch()
	if sk == nil {
		return sketch.AutoBind(sketch.Sketch{}, sketch.Catalog{}, sketch.AutoBindInput{}), nil, nil
	}
	cat := e.SketchCatalog()
	base := sketch.ResolveFaces(sketch.SyncCaseLights(*sk, cat), cat)
	var in sketch.AutoBindInput
	in.Devices, in.Fans = e.foundLightingHW()
	in.SyncSource = syncSource
	r := sketch.AutoBind(base, cat, in).Only(only)
	if !apply || len(r.Binds) == 0 {
		return r, nil, nil
	}
	v, err := e.setSketchLocked(sketch.ApplyAutoBind(base, r))
	if err != nil {
		return sketch.AutoBindResult{}, nil, err
	}
	return r, &v, nil
}
