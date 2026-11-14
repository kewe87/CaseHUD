package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/kewe87/casehud/internal/lcd"
)

// What a route needs from the caller. Reading needs "read", anything that
// changes state (or drives hardware) needs "control". Ending or updating
// CaseHUD and opening its window need "system", a right of its own that
// "control" does not include. The pairing routes need no token.
const (
	scopeRead    = "read"
	scopeControl = "control"
	scopeSystem  = "system"
	scopePublic  = "public"
)

// v1Route is one route of the public API with the scope it needs. This table is
// the only place routes are registered; openapi_test.go checks it against the
// OpenAPI description.
type v1Route struct {
	Pattern string // "METHOD /api/v1/path"
	Scope   string
	Handler http.HandlerFunc
}

// v1Routes lists every route of /api/v1/.
func (s *Server) v1Routes() []v1Route {
	rs := []v1Route{
		{"POST /api/v1/pair", scopePublic, s.v1Pair},
		{"GET /api/v1/pair/{id}", scopePublic, s.v1PairPoll},
		{"GET /api/v1/token", scopeRead, s.v1Token},
		{"DELETE /api/v1/token", scopeRead, s.v1RevokeToken},
		{"GET /api/v1/openapi.json", scopeRead, s.v1OpenAPI},

		{"GET /api/v1/status", scopeRead, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.v1Status()) }},
		{"GET /api/v1/events", scopeRead, s.v1Events},
		{"GET /api/v1/panels", scopeRead, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.v1Panels()) }},
		{"PATCH /api/v1/panels/{id}", scopeControl, s.v1PatchPanel},
		{"GET /api/v1/panels/{id}/preview.png", scopeRead, s.v1PanelPreview},
		{"POST /api/v1/panels/{id}/identify", scopeControl, func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if !s.panelExists(id) {
				notFound(w, "panel %q not found", id)
				return
			}
			if err := s.Engine.Identify(id); err != nil {
				notFound(w, "%v", err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}},
		{"POST /api/v1/identify", scopeControl, func(w http.ResponseWriter, r *http.Request) {
			if err := s.Engine.Identify(""); err != nil {
				notFound(w, "no displays connected")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}},
		{"GET /api/v1/sensors", scopeRead, s.v1Sensors},
		{"GET /api/v1/designs", scopeRead, s.v1Designs},
		{"GET /api/v1/designs/{id}", scopeRead, s.v1GetDesign},
		{"PUT /api/v1/designs/{id}", scopeControl, s.v1PutDesign},
		{"GET /api/v1/designs/{id}/preview.png", scopeRead, s.v1DesignPreview},
		{"POST /api/v1/render", scopeRead, s.v1Render},
		{"GET /api/v1/sets", scopeRead, s.v1Sets},
		{"GET /api/v1/layouts", scopeRead, s.v1Layouts},
		{"POST /api/v1/sets/apply", scopeControl, s.v1ApplySet},
		{"GET /api/v1/scenes", scopeRead, s.v1Scenes},
		{"POST /api/v1/scenes/{id}/activate", scopeControl, s.v1ActivateScene},
		{"PATCH /api/v1/rotation", scopeControl, s.v1PatchRotation},
		{"PATCH /api/v1/automation", scopeControl, s.v1PatchAutomation},
		{"POST /api/v1/rotation/next", scopeControl, func(w http.ResponseWriter, r *http.Request) {
			s.Engine.NextRotation()
			w.WriteHeader(http.StatusNoContent)
		}},
		{"POST /api/v1/power", scopeControl, s.v1Power},
		{"POST /api/v1/gaming", scopeControl, s.v1Gaming},
		{"GET /api/v1/community", scopeRead, s.v1CommunityList},
		{"GET /api/v1/community/{id}/preview.png", scopeRead, s.v1CommunityPreview},
		{"POST /api/v1/community/{id}/import", scopeControl, s.v1CommunityImport},
		{"PATCH /api/v1/awtrix", scopeControl, s.v1PatchAwtrix},
	}
	rs = append(rs, s.lightingRoutesV1()...)
	rs = append(rs, s.coolingRoutesV1()...)
	rs = append(rs, s.moreRoutesV1()...)
	return append(rs, []v1Route{
		{"GET /api/v1/update", scopeRead, s.v1Update},
		{"POST /api/v1/update/check", scopeSystem, s.v1UpdateCheck},
		{"POST /api/v1/update/install", scopeSystem, s.v1UpdateInstall},
		{"POST /api/v1/quit", scopeSystem, s.v1Quit},
		{"POST /api/v1/gui/open", scopeSystem, s.v1GUIOpen},
	}...)
}

// writeJSONBody encodes v after the header was written.
func writeJSONBody(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }

// v1Sensor is one reading.
type v1Sensor struct {
	Path  string  `json:"path"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// v1Sensors lists the live readings; a path (or its suffix) is a widget source.
func (s *Server) v1Sensors(w http.ResponseWriter, r *http.Request) {
	rs := s.Engine.Provider().All()
	out := make([]v1Sensor, 0, len(rs))
	for _, x := range rs {
		out = append(out, v1Sensor{Path: x.Path, Value: x.Value, Unit: x.Unit})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	writeJSON(w, out)
}

// v1GetDesign returns the layout JSON of a design.
func (s *Server) v1GetDesign(w http.ResponseWriter, r *http.Request) {
	l, err := s.Engine.Store.Design(r.PathValue("id"))
	if err != nil {
		notFound(w, "design %q not found", r.PathValue("id"))
		return
	}
	writeJSON(w, l)
}

// readLayoutV1 reads a layout from the body, in the format of the API's errors.
func readLayoutV1(w http.ResponseWriter, r *http.Request) (lcd.Layout, bool) {
	l, err := readLayout(r)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is larger than 1 MiB")
		} else {
			badRequest(w, "%v", err)
		}
		return l, false
	}
	return l, true
}

// v1PutDesign saves a layout as a user design (an id of a built-in design
// overrides it).
func (s *Server) v1PutDesign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	l, ok := readLayoutV1(w, r)
	if !ok {
		return
	}
	if err := s.Engine.Store.SaveDesign(id, l); err != nil {
		badRequest(w, "%v", err)
		return
	}
	s.Engine.DesignChanged(id)
	w.WriteHeader(http.StatusNoContent)
}

// renderSlots limits the renders of POST /render that run at once: a layout of
// 2048 pixels costs a lot of memory.
var renderSlots = make(chan struct{}, 2)

// v1Render draws a layout with live values and answers a PNG; nothing is stored.
// A background from the media library is refused: decoding a clip is far too
// heavy for a read token.
func (s *Server) v1Render(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Query().Get("bg"), "media:") {
		badRequest(w, "a media background is not available here")
		return
	}
	select {
	case renderSlots <- struct{}{}:
		defer func() { <-renderSlots }()
	default:
		w.Header().Set("Retry-After", "1")
		apiFail(w, http.StatusServiceUnavailable, "busy", "other renders are running, try again in a moment")
		return
	}
	l, ok := readLayoutV1(w, r)
	if !ok {
		return
	}
	s.writePNG(w, l, r)
}
