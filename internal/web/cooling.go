package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kewe87/casehud/internal/app"
)

// presetStatus is the HTTP status for a refused preset request.
func presetStatus(err error) int {
	var pe *app.CoolingPresetError
	if !errors.As(err, &pe) {
		return http.StatusInternalServerError
	}
	switch pe.Code {
	case app.PresetNotFound:
		return http.StatusNotFound
	case app.PresetBadRequest:
		return http.StatusBadRequest
	case app.PresetStorage:
		return http.StatusInternalServerError
	}
	return http.StatusConflict
}

// coolingPresetRoutes are the routes of the GUI for presets. The public API
// can only activate one.
func (s *Server) coolingPresetRoutes(api *http.ServeMux) {
	answer := func(w http.ResponseWriter, err error) {
		if err != nil {
			fail(w, presetStatus(err), err)
			return
		}
		writeJSON(w, s.Engine.CoolingPresets())
	}
	name := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return "", false
		}
		return body.Name, true
	}
	api.HandleFunc("GET /api/cooling/presets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.CoolingPresets())
	})
	api.HandleFunc("POST /api/cooling/presets", func(w http.ResponseWriter, r *http.Request) {
		n, ok := name(w, r)
		if !ok {
			return
		}
		_, err := s.Engine.SaveCoolingPreset(n)
		answer(w, err)
	})
	api.HandleFunc("PUT /api/cooling/presets/{id}", func(w http.ResponseWriter, r *http.Request) {
		n, ok := name(w, r)
		if !ok {
			return
		}
		answer(w, s.Engine.RenameCoolingPreset(r.PathValue("id"), n))
	})
	api.HandleFunc("POST /api/cooling/presets/{id}/overwrite", func(w http.ResponseWriter, r *http.Request) {
		answer(w, s.Engine.OverwriteCoolingPreset(r.PathValue("id")))
	})
	api.HandleFunc("POST /api/cooling/presets/{id}/activate", func(w http.ResponseWriter, r *http.Request) {
		answer(w, s.Engine.ActivateCoolingPreset(r.PathValue("id")))
	})
	api.HandleFunc("DELETE /api/cooling/presets/{id}", func(w http.ResponseWriter, r *http.Request) {
		answer(w, s.Engine.DeleteCoolingPreset(r.PathValue("id")))
	})
}

// requestRevision is the revision a page sends with a save (0: none).
func requestRevision(r *http.Request) int64 {
	n, _ := strconv.ParseInt(strings.Trim(r.Header.Get("If-Match"), `" `), 10, 64)
	return n
}

func (s *Server) coolingRoutes(api *http.ServeMux) {
	s.coolingPresetRoutes(api)
	api.HandleFunc("GET /api/cooling", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.CoolingView())
	})
	api.HandleFunc("GET /api/cooling/setup", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.CoolingSetup())
	})
	api.HandleFunc("PUT /api/cooling/config", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body) // limited by limitBody
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetCoolingConfigAt(b, requestRevision(r)); err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, app.ErrCoolingRevision) {
				code = http.StatusConflict
			}
			fail(w, code, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/cooling/measure", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.StartCoolingMeasure(body.ID); err != nil {
			fail(w, http.StatusConflict, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/cooling/measure-all", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.StartCoolingMeasureAll(); err != nil {
			fail(w, http.StatusConflict, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/cooling/measure-pump", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.StartCoolingPumpMeasure(body.ID); err != nil {
			fail(w, http.StatusConflict, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/cooling/release-pump", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		// One UAC prompt; the answer only says the elevated start was launched.
		if err := s.Engine.ReleaseCoolingPump(body.ID); err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, app.ErrReleaseWait) {
				code = http.StatusTooManyRequests
			}
			fail(w, code, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	api.HandleFunc("POST /api/cooling/recheck-pump", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.RecheckCoolingPump(body.ID); err != nil {
			fail(w, http.StatusConflict, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("DELETE /api/cooling/measure", func(w http.ResponseWriter, r *http.Request) {
		// Without a parameter the run is thrown away. keep=finished stores the
		// fans of a run over all fans that were already finished.
		switch r.URL.Query().Get("keep") {
		case "":
			s.Engine.StopCoolingMeasure()
		case "finished":
			s.Engine.StopCoolingMeasureKeep()
		default:
			fail(w, http.StatusBadRequest, errors.New("keep must be finished"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
