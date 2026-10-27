package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/community"
)

// libraryRoutes adds the library and diagnostics endpoints of the GUI (GUI
// token only, not part of API v1). The network is used only by
// POST /api/library/refresh, which the user starts with a button.
func (s *Server) libraryRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/library", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.libraryView(community.Summary{}, false))
	})
	api.HandleFunc("POST /api/library/refresh", s.libraryRefresh)
	api.HandleFunc("POST /api/library/presets/{id}/apply", s.libraryApplyPreset)
	api.HandleFunc("GET /api/diagnostics/trace", func(w http.ResponseWriter, r *http.Request) {
		active, until := s.Engine.LightTraceStatus()
		writeJSON(w, map[string]any{"active": active, "seconds": secondsLeft(until)})
	})
	api.HandleFunc("POST /api/diagnostics/trace", func(w http.ResponseWriter, r *http.Request) {
		until, err := s.Engine.StartLightTrace()
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, app.ErrTrace) {
				status = http.StatusConflict
			}
			fail(w, status, err)
			return
		}
		writeJSON(w, map[string]any{"active": true, "seconds": secondsLeft(until)})
	})
	api.HandleFunc("POST /api/diagnostics/export", s.diagnosticsExport)
}

func secondsLeft(until time.Time) int {
	if until.IsZero() {
		return 0
	}
	return max(0, int(time.Until(until).Round(time.Second)/time.Second))
}

func (s *Server) libraryView(sum community.Summary, fresh bool) map[string]any {
	st := s.Engine.LibraryStatus()
	if fresh {
		st.Warnings = sum.Warnings
	}
	return map[string]any{"status": st, "presets": s.Engine.LightingPresets()}
}

func (s *Server) libraryRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()
	sum, err := s.Engine.RefreshLibrary(ctx)
	if err != nil {
		// The cached copy stays usable; tell the GUI what it still has.
		v := s.libraryView(sum, false)
		v["error"] = err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(v)
		return
	}
	writeJSON(w, s.libraryView(sum, true))
}

func (s *Server) libraryApplyPreset(w http.ResponseWriter, r *http.Request) {
	v, dropped, err := s.Engine.ApplyLightingPreset(r.PathValue("id"))
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"lighting": v, "ignored": dropped})
}

func (s *Server) diagnosticsExport(w http.ResponseWriter, r *http.Request) {
	packets := r.URL.Query().Get("packets") == "1"
	var buf bytes.Buffer
	if err := s.Engine.WriteDiagnostics(r.Context(), &buf, s.Version, app.DiagnosticsOptions{Packets: packets}); err != nil {
		fail(w, http.StatusInternalServerError, fmt.Errorf("diagnostics: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="casehud-diagnostics-`+time.Now().Format("20060102-1504")+`.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(buf.Bytes())
}
