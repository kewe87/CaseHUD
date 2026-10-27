package web

import (
	"errors"
	"net/http"

	"github.com/kewe87/casehud/internal/app"
)

// monitorBackRoutes registers the way back for a display in monitor mode. GUI
// token only: the v1 API has no such route.
func (s *Server) monitorBackRoutes(api *http.ServeMux) {
	api.HandleFunc("POST /api/monitor-mode/bring-back", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		listed := false
		for _, d := range s.Engine.MonitorModeDevices() {
			listed = listed || d.Path == req.Path
		}
		if !listed {
			fail(w, http.StatusNotFound, errors.New("display in monitor mode not found"))
			return
		}
		switch err := s.Engine.BringBackMonitorMode(req.Path); {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, app.ErrMonitorBackWait):
			fail(w, http.StatusTooManyRequests, err)
		case errors.Is(err, app.ErrOutputHeld):
			fail(w, http.StatusConflict, err)
		default:
			fail(w, http.StatusBadGateway, err)
		}
	})
}
