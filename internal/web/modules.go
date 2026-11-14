package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/kewe87/casehud/internal/app"
)

func (s *Server) modulesRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/modules", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.Modules())
	})
	api.HandleFunc("PUT /api/modules", func(w http.ResponseWriter, r *http.Request) {
		var m app.Modules
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&m); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetModules(m); err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, app.ErrNoModule) {
				code = http.StatusBadRequest
			}
			fail(w, code, err)
			return
		}
		writeJSON(w, s.Engine.Modules())
	})
	// The setup dialog: the choice goes through SetModules, so the rule and the
	// error are those of the settings. Closing it keeps the areas as they are.
	api.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		var m app.Modules
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&m); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		s.finishSetup(w, &m)
	})
	api.HandleFunc("POST /api/setup/skip", func(w http.ResponseWriter, r *http.Request) {
		s.finishSetup(w, nil)
	})
	api.HandleFunc("PUT /api/cooling/mode", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetCoolingMode(body.Mode); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		// The page follows the mode itself; the revision tells it that this change was its own.
		w.Header().Set("ETag", strconv.FormatInt(s.Engine.CoolingRevision(), 10))
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) finishSetup(w http.ResponseWriter, m *app.Modules) {
	if err := s.Engine.CompleteSetup(m); err != nil {
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, app.ErrNoModule):
			code = http.StatusBadRequest
		case errors.Is(err, app.ErrSetupDone):
			code = http.StatusConflict
		}
		fail(w, code, err)
		return
	}
	writeJSON(w, s.Engine.Modules())
}
