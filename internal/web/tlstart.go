package web

import (
	"errors"
	"net/http"

	"github.com/kewe87/casehud/internal/app"
)

// tlStartRoutes registers the switch for the start image of the TL LCD fans
// and the button that writes it again (GUI only, like all /api/ routes).
func (s *Server) tlStartRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/tl-start-image", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"on": s.Engine.TLStartImage()})
	})
	api.HandleFunc("PUT /api/tl-start-image", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if err := s.Engine.SetTLStartImage(req.On); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]bool{"on": s.Engine.TLStartImage()})
	})
	api.HandleFunc("GET /api/tl-serial", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"on": s.Engine.TLSerialAssign()})
	})
	api.HandleFunc("PUT /api/tl-serial", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if err := s.Engine.SetTLSerialAssign(req.On); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]bool{"on": s.Engine.TLSerialAssign()})
	})
	api.HandleFunc("POST /api/tl-start-image/rewrite", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Confirm bool `json:"confirm"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		n, err := s.Engine.RewriteStartImages(req.Confirm)
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, app.ErrAckRequired) {
				code = http.StatusConflict
			}
			fail(w, code, err)
			return
		}
		writeJSON(w, map[string]int{"fans": n})
	})
}
