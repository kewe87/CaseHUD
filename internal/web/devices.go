package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kewe87/casehud/internal/app"
)

// devicesRoutes registers the switches for experimental devices. Only these
// routes (and the config file) change what discovery may talk to.
func (s *Server) devicesRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Engine.Devices()) })
	api.HandleFunc("PUT /api/devices", s.putDevices)
	api.HandleFunc("POST /api/disclaimer/accept", s.acceptDisclaimer)
	api.HandleFunc("POST /api/ack", s.ack)
}

// acceptDisclaimer records the first-start acceptance (GUI token only: API
// clients cannot accept on the user's behalf) and releases device output.
func (s *Server) acceptDisclaimer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version int `json:"version"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.Engine.AcceptDisclaimer(req.Version); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, s.Engine.Acks())
}

// ack records a one-time confirmation ("experimental" or "lighting").
func (s *Server) ack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		What string `json:"what"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.Engine.Acknowledge(req.What); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, s.Engine.Acks())
}

func (s *Server) putDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Experimental *bool    `json:"experimental"`
		Off          []string `json:"off"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	cur := s.Engine.Devices()
	on := cur.Experimental
	if req.Experimental != nil {
		on = *req.Experimental
	}
	off := req.Off
	if off == nil { // not sent: keep the individual switches
		for _, d := range cur.Drivers {
			if d.Off {
				off = append(off, d.ID)
			}
		}
	}
	if err := s.Engine.SetDevices(on, off); err != nil {
		if errors.Is(err, app.ErrAckRequired) {
			fail(w, http.StatusConflict, err)
			return
		}
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, s.Engine.Devices())
}
