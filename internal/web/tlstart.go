package web

import (
	"errors"
	"net/http"

	"github.com/kewe87/casehud/internal/app"
)

// tlStartRoutes registers the switch for the start image of the TL LCD fans
// and the button that writes it again (GUI only, like all /api/ routes).
func (s *Server) tlStartRoutes(api *http.ServeMux) {
	state := func() map[string]any {
		return map[string]any{"on": s.Engine.TLStartImage(), "style": s.Engine.TLStartImageStyle()}
	}
	api.HandleFunc("GET /api/tl-start-image", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, state())
	})
	api.HandleFunc("PUT /api/tl-start-image", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On    *bool   `json:"on"`
			Style *string `json:"style"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Style != nil {
			if err := s.Engine.SetTLStartImageStyle(*req.Style); err != nil {
				fail(w, http.StatusBadRequest, err)
				return
			}
		}
		if req.On != nil {
			if err := s.Engine.SetTLStartImage(*req.On); err != nil {
				fail(w, http.StatusInternalServerError, err)
				return
			}
		}
		writeJSON(w, state())
	})
	api.HandleFunc("GET /api/boot-logo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]int{"recorded": s.Engine.BootLogoRecorded()})
	})
	api.HandleFunc("POST /api/boot-logo/remove", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Confirm bool `json:"confirm"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		n, err := s.Engine.RemoveBootLogos(req.Confirm)
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, app.ErrAckRequired) {
				code = http.StatusConflict
			}
			fail(w, code, err)
			return
		}
		writeJSON(w, map[string]int{"removed": n})
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
