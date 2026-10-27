package web

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/kewe87/casehud/internal/app"
)

// lightingRoutes adds the lighting endpoints of the GUI (GUI token).
func (s *Server) lightingRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/lighting", s.lightingGet)
	api.HandleFunc("PATCH /api/lighting", s.lightingPatch)
	api.HandleFunc("POST /api/lighting/rescan", s.lightingRescan)
	api.HandleFunc("POST /api/lighting/calibrate", s.lightingCalibrate)
	api.HandleFunc("DELETE /api/lighting/calibrate", func(w http.ResponseWriter, r *http.Request) {
		s.Engine.LightingEndCalibration()
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/lighting/blink", s.lightingBlink)
	api.HandleFunc("POST /api/lighting/sync", s.lightingSync)
	// LED count of ARGB headers (GUI only): assistant, parts, templates.
	api.HandleFunc("POST /api/lighting/assist", s.lightingAssist)
	api.HandleFunc("POST /api/lighting/zone-setup", s.lightingZoneSetup)
	api.HandleFunc("GET /api/lighting/parts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.LightingParts())
	})
	api.HandleFunc("GET /api/lighting/templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Engine.LightingTemplates())
	})
	api.HandleFunc("POST /api/lighting/templates/apply", s.lightingTemplateApply)
	// One read of the fan list from the controller, on the user's request.
	api.HandleFunc("POST /api/arrangement/scan", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.ScanFans(); err != nil {
			lightingFail(w, r, err)
			return
		}
		writeJSON(w, s.Engine.Arrangement())
	})
}

// lightingRoutesV1 adds GET/PATCH /api/v1/lighting.
func (s *Server) lightingRoutesV1(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/lighting", s.lightingGet)
	m.HandleFunc("PATCH /api/v1/lighting", s.lightingPatchV1)
}

// lightingFail answers in the format of the path: the v1 error object under
// /api/v1/, plain text for the GUI.
func lightingFail(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusInternalServerError, "internal"
	if errors.Is(err, app.ErrLighting) {
		status, code = http.StatusBadRequest, "bad_request"
	}
	if errors.Is(err, app.ErrAckRequired) {
		status, code = http.StatusConflict, "ack_required"
	}
	if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/api/v1/" {
		if status == http.StatusInternalServerError {
			log.Printf("lighting: %v", err)
			err = errors.New("device I/O error")
		}
		apiFail(w, status, code, err.Error())
		return
	}
	if status == http.StatusInternalServerError {
		log.Printf("lighting: %v", err) // device paths stay out of the answer
		err = errors.New("device I/O error")
	}
	fail(w, status, err)
}

func (s *Server) lightingGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Engine.Lighting())
}

// lightingPatchV1 is the API variant: settings that make CaseHUD write to
// the fan controller or share a device with another program (TL backend,
// device settings, ignoring conflicts) are only available in the GUI.
func (s *Server) lightingPatchV1(w http.ResponseWriter, r *http.Request) {
	s.lightingPatchWith(w, r, true)
}

func (s *Server) lightingPatch(w http.ResponseWriter, r *http.Request) {
	s.lightingPatchWith(w, r, false)
}

func (s *Server) lightingPatchWith(w http.ResponseWriter, r *http.Request, api bool) {
	var p app.LightingPatch
	if !decodeBody(w, r, &p) {
		return
	}
	if api {
		guiOnly := p.TL != nil
		for _, d := range p.Devices {
			guiOnly = guiOnly || len(d.Settings) > 0 || d.IgnoreConflicts != nil
		}
		if guiOnly {
			apiFail(w, http.StatusForbidden, "gui_only", "tl, devices.*.settings and devices.*.ignoreConflicts can only be changed in the CaseHUD window")
			return
		}
	}
	v, err := s.Engine.PatchLighting(p)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) lightingRescan(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.RescanLighting(); err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, s.Engine.Lighting())
}

func (s *Server) lightingCalibrate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"`
		Zone   string `json:"zone"`
		Pos    int    `json:"pos"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.Engine.LightingCalibrate(req.Device, req.Zone, req.Pos); err != nil {
		lightingFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) lightingBlink(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.URL.Query().Get("port"))
	if err == nil {
		err = s.Engine.LightingBlink(port)
	} else {
		err = errors.Join(app.ErrLighting, errors.New("port is required"))
	}
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// lightingSync is the helper for TL fans that follow a mainboard header (GUI
// only): it sets the controller's syncSource and lays out that header.
func (s *Server) lightingSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"`
		Source string `json:"source"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	v, err := s.Engine.LightingSetSync(req.Device, req.Source)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) lightingAssist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"`
		Zone   string `json:"zone"`
		N      int    `json:"n"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.Engine.LightingAssist(req.Device, req.Zone, req.N); err != nil {
		lightingFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// lightingZoneSetup sets the LED count of a header: a number, or library parts.
func (s *Server) lightingZoneSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device   string   `json:"device"`
		Zone     string   `json:"zone"`
		Count    int      `json:"count"`
		Parts    []string `json:"parts"`
		Parallel bool     `json:"parallel"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	v, err := s.Engine.LightingZoneSetup(req.Device, req.Zone, req.Count, req.Parts, req.Parallel)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) lightingTemplateApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Lang string `json:"lang"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	res, err := s.Engine.ApplyLightingTemplate(req.ID, req.Lang)
	if err != nil {
		lightingFail(w, r, err)
		return
	}
	writeJSON(w, res)
}
