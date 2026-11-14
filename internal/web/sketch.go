package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/sketch"
)

// sketchRoutes adds the case sketch endpoints of the GUI (GUI token only,
// not part of API v1).
func (s *Server) sketchRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/lighting/catalog", s.sketchCatalog)
	api.HandleFunc("GET /api/lighting/sketch", s.sketchGet)
	api.HandleFunc("PUT /api/lighting/sketch", s.sketchPut)
	api.HandleFunc("DELETE /api/lighting/sketch", s.sketchDelete)
	api.HandleFunc("POST /api/lighting/sketch/upgrade", s.sketchUpgrade)
	api.HandleFunc("GET /api/lighting/sketch/suggest", s.sketchSuggest)
	api.HandleFunc("GET /api/lighting/sketch/autobind", s.sketchAutoBind)
	api.HandleFunc("POST /api/lighting/sketch/autobind", s.sketchAutoBind)
	api.HandleFunc("GET /api/lighting/live", s.lightingLive)
	api.HandleFunc("POST /api/lighting/sketch/pattern", s.sketchPattern)
	api.HandleFunc("POST /api/lighting/sketch/orient", s.sketchOrient)
}

// sketchLimits are the bounds the editor needs to stay within what the
// server accepts.
type sketchLimits struct {
	Version      int     `json:"version"`
	MaxParts     int     `json:"maxParts"`
	MaxLabel     int     `json:"maxLabel"`
	MaxLEDs      int     `json:"maxLeds"`
	MaxTotalLEDs int     `json:"maxTotalLeds"`
	MaxDimMM     float64 `json:"maxDimMm"`
	MaxChambers  int     `json:"maxChambers"`
	MarginMM     float64 `json:"marginMm"`
	MaxSegments  int     `json:"maxSegments"` // parts in one (device, zone) chain
	MaxLayer     int     `json:"maxLayer"`    // layer is -maxLayer..maxLayer
	MaxSpin      float64 `json:"maxSpin"`     // spin is -maxSpin..maxSpin degrees
	OverlapTolMM float64 `json:"overlapTolMm"`
	MaxBody      int     `json:"maxBody"` // bytes of a sketch document
}

// The catalog sent to the GUI carries only what the editor draws and offers:
// the descriptive text blocks, the outlines of other views, LED zones and
// directions, GPU slots and the like stay on the server.
type partView struct {
	ID         string          `json:"id"`
	Vendor     string          `json:"vendor,omitempty"`
	Model      string          `json:"model,omitempty"`
	Category   string          `json:"category"`
	Dimensions sketch.PartDims `json:"dimensions_mm"`
	Outline    struct {
		Front       []sketch.Shape `json:"front"`
		Approximate bool           `json:"approximate,omitempty"`
	} `json:"outline"`
	LEDs struct {
		Count       int               `json:"count"`
		Positions   [][2]float64      `json:"positions_mm"`
		Approximate bool              `json:"approximate,omitempty"`
		Generator   *sketch.Generator `json:"generator,omitempty"`
	} `json:"leds"`
}

type mountView struct {
	ID      string               `json:"id"`
	View    string               `json:"view"`
	Region  sketch.Shape         `json:"region"`
	Layouts []sketch.MountLayout `json:"layouts"`
	// Face is the case face the mount sits on (fans point their LED side to
	// the inside); MaxThick is the thickest part it takes, 0 = not given.
	Face     string  `json:"face,omitempty"`
	MaxThick float64 `json:"max_thickness_mm,omitempty"`
}

type caseView struct {
	ID          string                     `json:"id"`
	Name        string                     `json:"name"`
	Vendor      string                     `json:"vendor,omitempty"`
	Approximate bool                       `json:"approximate,omitempty"`
	Dims        sketch.Dims                `json:"dims"`
	Chambers    []sketch.Chamber           `json:"chambers,omitempty"`
	Views       map[string]sketch.CaseView `json:"views,omitempty"`
	Mounts      []mountView                `json:"mounts,omitempty"`
	Areas       []sketch.Area              `json:"areas,omitempty"`
	// Orientation is empty when the case has the default one (side view starts
	// at the rear, glass on the left).
	Orientation sketch.Orientation `json:"orientation,omitzero"`
	// Lighting is the lighting built into the case, as the placements a
	// sketch on this case gets (unbound).
	Lighting []caseLightView `json:"lighting,omitempty"`
}

type caseLightView struct {
	sketch.Placement
	Name string `json:"name,omitempty"`
	Zone string `json:"zone,omitempty"`
}

type sketchCatalogView struct {
	Parts  map[string]partView `json:"parts"`
	Cases  map[string]caseView `json:"cases"`
	Limits sketchLimits        `json:"limits"`
}

func newPartView(p sketch.Part) partView {
	v := partView{ID: p.ID, Vendor: p.Vendor, Model: p.Model, Category: p.Category, Dimensions: p.Dimensions}
	v.Outline.Front, v.Outline.Approximate = p.Outline.Front, p.Outline.Approximate
	v.LEDs.Count, v.LEDs.Positions, v.LEDs.Approximate, v.LEDs.Generator = p.LEDs.Count, p.LEDs.Positions, p.LEDs.Approximate, p.LEDs.Generator
	return v
}

func newCaseView(c sketch.CaseTemplate, cat sketch.Catalog) caseView {
	v := caseView{ID: c.ID, Name: c.Name, Vendor: c.Vendor, Approximate: c.Approximate, Dims: c.Dims,
		Chambers: c.Chambers, Views: c.Views, Areas: c.Areas, Orientation: c.Orientation}
	for _, m := range c.Mounts {
		v.Mounts = append(v.Mounts, mountView{ID: m.ID, View: m.View, Region: m.Region, Layouts: m.Layouts, Face: m.Face, MaxThick: m.MaxThick})
	}
	names := map[string]sketch.CaseLight{}
	for _, l := range c.Lighting {
		names[l.ID] = l
	}
	for _, p := range sketch.LightsOf(c, cat) {
		l := names[p.CaseLight]
		v.Lighting = append(v.Lighting, caseLightView{Placement: p, Name: l.Name, Zone: l.Zone})
	}
	return v
}

func (s *Server) sketchCatalog(w http.ResponseWriter, r *http.Request) {
	c := s.Engine.SketchCatalog()
	out := sketchCatalogView{Parts: make(map[string]partView, len(c.Parts)), Cases: make(map[string]caseView, len(c.Cases)),
		Limits: sketchLimits{
			Version: sketch.Version, MaxParts: sketch.MaxParts, MaxLabel: sketch.MaxLabel, MaxLEDs: sketch.MaxLEDs,
			MaxTotalLEDs: sketch.MaxTotalLEDs, MaxDimMM: sketch.MaxDimMM, MaxChambers: sketch.MaxChambers, MarginMM: sketch.MarginMM,
			MaxSegments: sketch.MaxZoneSegments, MaxLayer: sketch.MaxLayer, MaxSpin: 360, OverlapTolMM: sketch.OverlapTolMM,
			MaxBody: sketch.MaxBodyBytes,
		}}
	for id, p := range c.Parts {
		out.Parts[id] = newPartView(p)
	}
	for id, t := range c.Cases {
		out.Cases[id] = newCaseView(t, c)
	}
	writeJSON(w, out)
}

func (s *Server) sketchGet(w http.ResponseWriter, r *http.Request) {
	sk, n := s.Engine.ResolvedSketch()
	writeJSON(w, struct {
		Sketch *sketch.Sketch `json:"sketch"`
		Net    *sketch.Net    `json:"net,omitempty"`
	}{sk, n})
}

func (s *Server) sketchPut(w http.ResponseWriter, r *http.Request) {
	body, ok := readSketchBody(w, r)
	if !ok {
		return
	}
	sk, notes, err := sketch.DecodeWithCatalog(body, s.Engine.SketchCatalog()) // version 1 sketches are upgraded
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("invalid sketch: "+err.Error()))
		return
	}
	v, err := s.Engine.SetSketch(sk)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, struct {
		app.SketchView
		Notes []sketch.UpgradeNote `json:"notes,omitempty"`
	}{v, notes})
}

func readSketchBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, sketch.MaxBodyBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, http.StatusRequestEntityTooLarge, errors.New("sketch is larger than 1 MiB"))
		} else {
			fail(w, http.StatusBadRequest, errors.New("request body could not be read"))
		}
		return nil, false
	}
	return body, true
}

// sketchUpgrade converts a sketch document to the current format without
// storing anything, so that an import in the editor goes through the same
// migration as a stored sketch. The body is a sketch (version 1 or 2) or the
// answer of GET /api/lighting/sketch. "invalid" says why the result would be
// refused by PUT; the sketch is returned either way.
func (s *Server) sketchUpgrade(w http.ResponseWriter, r *http.Request) {
	body, ok := readSketchBody(w, r)
	if !ok {
		return
	}
	var wrap struct {
		Version *int            `json:"version"`
		Sketch  json.RawMessage `json:"sketch"`
	}
	if json.Unmarshal(body, &wrap) == nil && wrap.Version == nil && len(wrap.Sketch) > 0 {
		body = wrap.Sketch
	}
	cat := s.Engine.SketchCatalog()
	sk, notes, err := sketch.DecodeWithCatalog(body, cat)
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("invalid sketch: "+err.Error()))
		return
	}
	sk = sketch.SyncCaseLights(sk, cat)
	out := struct {
		Sketch  sketch.Sketch        `json:"sketch"`
		Notes   []sketch.UpgradeNote `json:"notes"`
		Invalid string               `json:"invalid,omitempty"`
	}{Sketch: sketch.ResolveFaces(sk, cat), Notes: notes}
	if out.Notes == nil {
		out.Notes = []sketch.UpgradeNote{}
	}
	if err := sketch.Validate(sk, cat); err != nil {
		out.Invalid = err.Error()
	}
	writeJSON(w, out)
}

func (s *Server) sketchSuggest(w http.ResponseWriter, r *http.Request) {
	sug, err := s.Engine.SketchSuggest()
	if err != nil {
		areaFail(w, r, err)
		return
	}
	writeJSON(w, sug)
}

// sketchAutoBind finds the bindings that are certain (GET) and stores them
// (POST, optional body {"parts": [...]} to bind only those parts, "sync":
// the mainboard header the user confirmed for TL fans in sync mode). The
// answer is the result, with the stored sketch after a POST that bound
// something ("sketch", "net", "lighting" as in GET /sketch).
func (s *Server) sketchAutoBind(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parts []string `json:"parts"`
		Sync  string   `json:"sync"`
	}
	if r.Method == http.MethodPost {
		if ok, _ := decodeBodyOpt(w, r, &req); !ok {
			return
		}
	}
	res, v, err := s.Engine.SketchAutoBind(r.Method == http.MethodPost, req.Parts, req.Sync)
	if err != nil {
		areaFail(w, r, err)
		return
	}
	out := struct {
		sketch.AutoBindResult
		*app.SketchView
	}{res, v}
	writeJSON(w, out)
}

// sketchPattern shows the orientation check pattern on one bound part
// (DELETE /api/lighting/calibrate ends it).
func (s *Server) sketchPattern(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Part string `json:"part"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	p, err := s.Engine.LightingPartPattern(req.Part)
	if err != nil {
		areaFail(w, r, err)
		return
	}
	writeJSON(w, struct {
		Pattern  sketch.PartPattern `json:"pattern"`
		Lighting app.LightingView   `json:"lighting"`
	}{p, s.Engine.Lighting()})
}

// sketchOrient corrects a part from what its pattern showed and shows the
// pattern again.
func (s *Server) sketchOrient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Part string `json:"part"`
		Fix  string `json:"fix"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	v, p, err := s.Engine.LightingOrient(req.Part, req.Fix)
	if err != nil {
		areaFail(w, r, err)
		return
	}
	writeJSON(w, struct {
		app.SketchView
		Pattern sketch.PartPattern `json:"pattern"`
	}{v, p})
}

func (s *Server) lightingLive(w http.ResponseWriter, r *http.Request) {
	v, err := s.Engine.LightingLive(r.URL.Query().Get("canvas") == "1")
	if err != nil {
		areaFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, v)
}

// areaFail answers 409 while the lighting area is switched off, like the
// other changes to that area.
func areaFail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, app.ErrAreaOff) {
		fail(w, http.StatusConflict, err)
		return
	}
	lightingFail(w, r, err)
}

func (s *Server) sketchDelete(w http.ResponseWriter, r *http.Request) {
	v, err := s.Engine.DeleteSketch()
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}
