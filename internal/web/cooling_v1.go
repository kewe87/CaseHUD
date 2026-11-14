package web

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kewe87/casehud/internal/app"
)

// coolingRoutesV1 lists the cooling routes of the public API. Everything goes
// through the engine's own functions, which check what a saved configuration
// is checked for; nothing here writes cooling.json.
func (s *Server) coolingRoutesV1() []v1Route {
	return []v1Route{
		{"GET /api/v1/cooling", scopeRead, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, s.Engine.CoolingAPIView())
		}},
		{"POST /api/v1/cooling/preset", scopeControl, s.v1CoolingPreset},
		{"GET /api/v1/cooling/config", scopeRead, s.v1CoolingConfigGet},
		{"PUT /api/v1/cooling/config", scopeControl, s.v1CoolingConfigPut},
		{"PATCH /api/v1/cooling/outputs/{id}", scopeControl, s.v1CoolingOutputPatch},
		{"PUT /api/v1/cooling/curves/{id}", scopeControl, s.v1CoolingCurvePut},
		{"DELETE /api/v1/cooling/curves/{id}", scopeControl, s.v1CoolingCurveDelete},
		{"GET /api/v1/cooling/presets", scopeRead, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, s.Engine.CoolingPresets())
		}},
		{"POST /api/v1/cooling/presets", scopeControl, s.v1CoolingPresetSave},
		{"PUT /api/v1/cooling/presets/{id}", scopeControl, s.v1CoolingPresetRename},
		{"POST /api/v1/cooling/presets/{id}/overwrite", scopeControl, func(w http.ResponseWriter, r *http.Request) {
			s.v1PresetDone(w, s.Engine.OverwriteCoolingPreset(r.PathValue("id")))
		}},
		{"DELETE /api/v1/cooling/presets/{id}", scopeControl, func(w http.ResponseWriter, r *http.Request) {
			s.v1PresetDone(w, s.Engine.DeleteCoolingPreset(r.PathValue("id")))
		}},
		{"PUT /api/v1/cooling/mode", scopeControl, s.v1CoolingMode},
		{"GET /api/v1/cooling/measure", scopeRead, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, s.Engine.CoolingMeasure())
		}},
		{"POST /api/v1/cooling/measure", scopeControl, s.v1CoolingMeasure},
		{"POST /api/v1/cooling/measure-pump", scopeControl, s.v1CoolingMeasurePump},
		{"DELETE /api/v1/cooling/measure", scopeControl, s.v1CoolingMeasureStop},
		{"POST /api/v1/cooling/recheck-pump", scopeControl, s.v1CoolingRecheck},
	}
}

func (s *Server) v1CoolingPreset(w http.ResponseWriter, r *http.Request) {
	var req v1IDRequest
	ok, empty := decodeBodyOpt(w, r, &req)
	if !ok {
		return
	}
	if empty || req.ID == "" {
		badRequest(w, "id is required")
		return
	}
	if err := s.Engine.ActivateCoolingPreset(req.ID); err != nil {
		v1PresetError(w, err)
		return
	}
	writeJSON(w, s.Engine.CoolingAPIView())
}

// v1PresetError answers a refused preset request in the format of the API.
func v1PresetError(w http.ResponseWriter, err error) {
	var pe *app.CoolingPresetError
	if !errors.As(err, &pe) {
		internalError(w, err)
		return
	}
	switch pe.Code {
	case app.PresetNotFound:
		notFound(w, "%s", pe.Msg)
	case app.PresetBadRequest:
		badRequest(w, "%s", pe.Msg)
	case app.PresetStorage:
		internalError(w, err)
	default:
		apiFail(w, http.StatusConflict, pe.Code, pe.Msg)
	}
}

func (s *Server) v1PresetDone(w http.ResponseWriter, err error) {
	if err != nil {
		v1PresetError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) v1CoolingPresetSave(w http.ResponseWriter, r *http.Request) {
	var req v1NameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	info, err := s.Engine.SaveCoolingPreset(req.Name)
	if err != nil {
		v1PresetError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSONBody(w, info)
}

func (s *Server) v1CoolingPresetRename(w http.ResponseWriter, r *http.Request) {
	var req v1NameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	s.v1PresetDone(w, s.Engine.RenameCoolingPreset(r.PathValue("id"), req.Name))
}

// ifMatch reads the revision a change names. With required it must be there:
// 428 precondition_required says so. Without it, a missing header is 0 (not checked).
func ifMatch(w http.ResponseWriter, r *http.Request, required bool) (int64, bool) {
	h := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	if h == "" {
		if required {
			apiFail(w, http.StatusPreconditionRequired, "precondition_required", "send the revision you read in the If-Match header")
			return 0, false
		}
		return 0, true
	}
	n, err := strconv.ParseInt(h, 10, 64)
	if err != nil || n <= 0 {
		badRequest(w, "If-Match must be the revision (a number) from GET /api/v1/cooling/config")
		return 0, false
	}
	return n, true
}

// v1CoolingFail answers a refused change of the cooling.
func v1CoolingFail(w http.ResponseWriter, err error) {
	var ce *app.CoolingEditError
	switch {
	case errors.Is(err, app.ErrCoolingRevision):
		apiFail(w, http.StatusConflict, "revision_mismatch", "the cooling settings changed since they were read; read them again")
	case errors.Is(err, app.ErrCoolingBusy):
		apiFail(w, http.StatusConflict, "busy", "a speed measurement is running")
	case errors.Is(err, app.ErrCoolingOff):
		apiFail(w, http.StatusConflict, "cooling_off", "the cooling area is switched off in the CaseHUD settings")
	case errors.As(err, &ce):
		switch ce.Code {
		case app.EditNotFound:
			notFound(w, "%s", ce.Msg)
		case app.EditInvalid:
			apiFail(w, http.StatusBadRequest, "invalid_value", ce.Msg)
		default: // curve_in_use, unconfigured
			apiFail(w, http.StatusConflict, ce.Code, ce.Msg)
		}
	default:
		apiFail(w, http.StatusBadRequest, "invalid_value", err.Error())
	}
}

// coolingRevisionDone answers a change with the new revision.
func coolingRevisionDone(w http.ResponseWriter, status int, rev int64, out any) {
	w.Header().Set("ETag", strconv.FormatInt(rev, 10))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if out == nil {
		out = v1Revision{Revision: rev}
	}
	writeJSONBody(w, out)
}

func (s *Server) v1CoolingConfigGet(w http.ResponseWriter, r *http.Request) {
	doc := s.Engine.CoolingConfigDoc()
	w.Header().Set("ETag", strconv.FormatInt(doc.Revision, 10))
	writeJSON(w, doc)
}

// readLimited reads a request body that was limited by limitBody; a body that is
// too large is answered with 413.
func readLimited(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is larger than 1 MiB")
		} else {
			badRequest(w, "%v", err)
		}
		return nil, false
	}
	return b, true
}

func (s *Server) v1CoolingConfigPut(w http.ResponseWriter, r *http.Request) {
	rev, ok := ifMatch(w, r, true)
	if !ok {
		return
	}
	b, ok := readLimited(w, r)
	if !ok {
		return
	}
	newRev, err := s.Engine.SetCoolingConfigRev(b, rev)
	if err != nil {
		v1CoolingFail(w, err)
		return
	}
	coolingRevisionDone(w, http.StatusOK, newRev, nil)
}

func (s *Server) v1CoolingOutputPatch(w http.ResponseWriter, r *http.Request) {
	rev, ok := ifMatch(w, r, false)
	if !ok {
		return
	}
	var p app.CoolingOutputPatch
	if !decodeBody(w, r, &p) {
		return
	}
	newRev, err := s.Engine.PatchCoolingOutput(r.PathValue("id"), p, rev)
	if err != nil {
		v1CoolingFail(w, err)
		return
	}
	coolingRevisionDone(w, http.StatusOK, newRev, nil)
}

func (s *Server) v1CoolingCurvePut(w http.ResponseWriter, r *http.Request) {
	rev, ok := ifMatch(w, r, false)
	if !ok {
		return
	}
	b, ok := readLimited(w, r)
	if !ok {
		return
	}
	created, newRev, err := s.Engine.PutCoolingCurve(r.PathValue("id"), b, rev)
	if err != nil {
		v1CoolingFail(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	coolingRevisionDone(w, status, newRev, nil)
}

func (s *Server) v1CoolingCurveDelete(w http.ResponseWriter, r *http.Request) {
	rev, ok := ifMatch(w, r, false)
	if !ok {
		return
	}
	newRev, err := s.Engine.DeleteCoolingCurve(r.PathValue("id"), rev)
	if err != nil {
		v1CoolingFail(w, err)
		return
	}
	coolingRevisionDone(w, http.StatusOK, newRev, nil)
}

// v1CoolingMode switches between test and live mode. Live is refused while it
// would send nothing: the disclaimer is not accepted, or another fan program runs.
func (s *Server) v1CoolingMode(w http.ResponseWriter, r *http.Request) {
	var req v1ModeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Mode != "preview" && req.Mode != "live" {
		apiFail(w, http.StatusBadRequest, "invalid_value", "mode must be preview or live")
		return
	}
	if req.Mode == "live" {
		switch reason, program := s.Engine.CoolingLiveBlock(); reason {
		case "":
		case "disclaimer":
			apiFail(w, http.StatusConflict, "blocked", "the disclaimer is not accepted yet (in the CaseHUD window)")
			return
		case "program":
			apiFail(w, http.StatusConflict, "blocked", "another fan program is in charge: "+program)
			return
		default:
			apiFail(w, http.StatusConflict, "blocked", "CaseHUD could not rule out that another fan program is running")
			return
		}
	}
	if err := s.Engine.SetCoolingMode(req.Mode); err != nil {
		switch {
		case errors.Is(err, app.ErrCoolingBusy), errors.Is(err, app.ErrCoolingOff):
			v1CoolingFail(w, err)
		default:
			apiFail(w, http.StatusConflict, "conflict", err.Error())
		}
		return
	}
	rev := s.Engine.CoolingRevision()
	coolingRevisionDone(w, http.StatusOK, rev, v1ModeChanged{Revision: rev, Mode: req.Mode})
}

// v1MeasureFail answers a measurement that could not start.
func v1MeasureFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrCoolingBusy), errors.Is(err, app.ErrCoolingOff):
		v1CoolingFail(w, err)
	default:
		apiFail(w, http.StatusConflict, "conflict", err.Error())
	}
}

func confirmed(w http.ResponseWriter, ok bool) bool {
	if !ok {
		apiFail(w, http.StatusBadRequest, "confirmation_required", `a measurement drives the fans through their whole range; send "confirm": true`)
	}
	return ok
}

func (s *Server) v1CoolingMeasure(w http.ResponseWriter, r *http.Request) {
	var req v1MeasureRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if (req.ID == "") == !req.All {
		badRequest(w, "send either id or all")
		return
	}
	if !confirmed(w, req.Confirm) {
		return
	}
	var err error
	if req.All {
		err = s.Engine.StartCoolingMeasureAll()
	} else {
		err = s.Engine.StartCoolingMeasure(req.ID)
	}
	if err != nil {
		v1MeasureFail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSONBody(w, s.Engine.CoolingMeasure())
}

func (s *Server) v1CoolingMeasurePump(w http.ResponseWriter, r *http.Request) {
	var req v1MeasurePumpRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		badRequest(w, "id is required")
		return
	}
	if !confirmed(w, req.Confirm) {
		return
	}
	if err := s.Engine.StartCoolingPumpMeasure(req.ID); err != nil {
		v1MeasureFail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSONBody(w, s.Engine.CoolingMeasure())
}

func (s *Server) v1CoolingMeasureStop(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("keep") {
	case "":
		s.Engine.StopCoolingMeasure()
	case "finished":
		s.Engine.StopCoolingMeasureKeep()
	default:
		badRequest(w, "keep must be finished")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) v1CoolingRecheck(w http.ResponseWriter, r *http.Request) {
	var req v1IDRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		badRequest(w, "id is required")
		return
	}
	if err := s.Engine.RecheckCoolingPump(req.ID); err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// areaOf names the core area a v1 request would change, "" for a request that
// only reads or changes none. Saving and deleting presets and designs only
// touch stored data, so they are not listed.
func areaOf(method, path string) string {
	p := strings.TrimPrefix(path, "/api/v1/")
	switch {
	case method == http.MethodGet || p == "render":
		return ""
	case strings.HasPrefix(p, "cooling/"):
		return "cooling"
	case p == "lighting" && method == http.MethodPatch,
		strings.HasPrefix(p, "lighting/presets/") && strings.HasSuffix(p, "/apply"):
		return "lighting"
	case strings.HasPrefix(p, "panels/") && (method == http.MethodPatch || strings.HasSuffix(p, "/identify")),
		strings.HasPrefix(p, "layouts/"), // they refresh the displays that use them
		p == "identify", p == "sets/apply", p == "power", p == "gaming", p == "awtrix",
		p == "rotation" || p == "rotation/next", p == "automation",
		strings.HasPrefix(p, "scenes/") && strings.HasSuffix(p, "/activate"):
		return "display"
	}
	return ""
}

// areaGate answers requests that would change a core area that is switched off
// with 409 area_off. Reading stays possible. POST /cooling/preset has its own
// code, cooling_off, because the cooling contract names it; the cooling edits
// answer cooling_off themselves.
func (s *Server) areaGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if area := areaOf(r.Method, r.URL.Path); area != "" && area != "cooling" {
			m := s.Engine.Modules()
			if (area == "display" && !m.Display) || (area == "lighting" && !m.Lighting) {
				apiFail(w, http.StatusConflict, "area_off", "the "+area+" area is switched off in the CaseHUD settings")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
