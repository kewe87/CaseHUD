package web

import (
	"errors"
	"log"
	"net/http"

	"github.com/kewe87/casehud/internal/conflicts"
)

// conflictRoutes registers the GUI routes for other programs on the same
// devices. They are not part of the API v1: only the CaseHUD window closes
// programs (the status of API v1 lists the names, read-only).
func (s *Server) conflictRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/conflicts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.ConflictsStatus())
	})
	api.HandleFunc("PUT /api/conflicts/ask", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if err := s.Engine.SetConflictsAsk(req.On); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, s.Engine.ConflictsStatus())
	})
	api.HandleFunc("PUT /api/conflicts/ignore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Exe string `json:"exe"`
			On  bool   `json:"on"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if err := s.Engine.SetConflictIgnore(req.Exe, req.On); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, s.Engine.ConflictsStatus())
	})
	api.HandleFunc("POST /api/conflicts/close", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Exe  string `json:"exe"`
			Hard bool   `json:"hard"`
			// AllowUnusual answers the second warning for a program that
			// runs from an unusual location.
			AllowUnusual bool `json:"allowUnusual"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		res, err := s.Engine.CloseConflict(req.Exe, req.Hard, req.AllowUnusual)
		switch {
		case errors.Is(err, conflicts.ErrUnknown):
			fail(w, http.StatusBadRequest, err)
		case errors.Is(err, conflicts.ErrNeedsSoft), errors.Is(err, conflicts.ErrBusy), errors.Is(err, conflicts.ErrUnusualPath):
			fail(w, http.StatusConflict, err)
		case err != nil:
			fail(w, http.StatusInternalServerError, err)
		default:
			writeJSON(w, res)
		}
	})
	api.HandleFunc("POST /api/conflicts/open-services", func(w http.ResponseWriter, r *http.Request) {
		if s.Engine.Demo {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := conflicts.OpenServices(); err != nil {
			log.Println("conflicts: services list:", err)
			fail(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
