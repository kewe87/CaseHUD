package web

import (
	"errors"
	"io"
	"net/http"

	"github.com/kewe87/casehud/internal/sketch"
)

// sketchRoutes adds the case sketch endpoints of the GUI (GUI token only,
// not part of API v1).
func (s *Server) sketchRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/lighting/catalog", s.sketchCatalog)
	api.HandleFunc("GET /api/lighting/sketch", s.sketchGet)
	api.HandleFunc("PUT /api/lighting/sketch", s.sketchPut)
	api.HandleFunc("DELETE /api/lighting/sketch", s.sketchDelete)
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

func newCaseView(c sketch.CaseTemplate) caseView {
	v := caseView{ID: c.ID, Name: c.Name, Vendor: c.Vendor, Approximate: c.Approximate, Dims: c.Dims,
		Chambers: c.Chambers, Views: c.Views, Areas: c.Areas}
	for _, m := range c.Mounts {
		v.Mounts = append(v.Mounts, mountView{ID: m.ID, View: m.View, Region: m.Region, Layouts: m.Layouts})
	}
	return v
}

func (s *Server) sketchCatalog(w http.ResponseWriter, r *http.Request) {
	c := s.Engine.SketchCatalog()
	out := sketchCatalogView{Parts: make(map[string]partView, len(c.Parts)), Cases: make(map[string]caseView, len(c.Cases)),
		Limits: sketchLimits{
			Version: sketch.Version, MaxParts: sketch.MaxParts, MaxLabel: sketch.MaxLabel, MaxLEDs: sketch.MaxLEDs,
			MaxTotalLEDs: sketch.MaxTotalLEDs, MaxDimMM: sketch.MaxDimMM, MaxChambers: sketch.MaxChambers, MarginMM: sketch.MarginMM,
			MaxSegments: sketch.MaxZoneSegments,
		}}
	for id, p := range c.Parts {
		out.Parts[id] = newPartView(p)
	}
	for id, t := range c.Cases {
		out.Cases[id] = newCaseView(t)
	}
	writeJSON(w, out)
}

func (s *Server) sketchGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]*sketch.Sketch{"sketch": s.Engine.GetSketch()})
}

func (s *Server) sketchPut(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, sketch.MaxBodyBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, http.StatusRequestEntityTooLarge, errors.New("sketch is larger than 1 MiB"))
		} else {
			fail(w, http.StatusBadRequest, errors.New("request body could not be read"))
		}
		return
	}
	sk, err := sketch.Decode(body)
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("invalid sketch: "+err.Error()))
		return
	}
	v, err := s.Engine.SetSketch(sk)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) sketchDelete(w http.ResponseWriter, r *http.Request) {
	v, err := s.Engine.DeleteSketch()
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}
