package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/community"
	"github.com/kewe87/casehud/internal/lcd"
)

// APIVersion is the version of the stable external API under /api/v1/.
// Everything under /api/ without v1 is internal to the GUI and may change.
const APIVersion = 1

// apiError is the body of every v1 error.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func apiFail(w http.ResponseWriter, status int, code, msg string) {
	var e apiError
	e.Error.Code, e.Error.Message = code, msg
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(e)
}

func badRequest(w http.ResponseWriter, format string, a ...any) {
	apiFail(w, http.StatusBadRequest, "bad_request", fmt.Sprintf(format, a...))
}

func notFound(w http.ResponseWriter, format string, a ...any) {
	apiFail(w, http.StatusNotFound, "not_found", fmt.Sprintf(format, a...))
}

// internalError answers 500 without the text of the error: it may name files
// and folders. The text goes to the log.
func internalError(w http.ResponseWriter, err error) {
	log.Printf("api: internal error: %v", err)
	apiFail(w, http.StatusInternalServerError, "internal", "internal error; the details are in the CaseHUD log")
}

func badGateway(w http.ResponseWriter, err error) {
	apiFail(w, http.StatusBadGateway, "bad_gateway", err.Error())
}

// port is the port the server listens on (0 before Listen).
func (s *Server) port() int {
	if s.ln == nil {
		return 0
	}
	if a, ok := s.ln.Addr().(*net.TCPAddr); ok {
		return a.Port
	}
	return 0
}

// hostOK accepts only 127.0.0.1 and localhost with the server's port, so a
// web page that rebinds its DNS name to 127.0.0.1 cannot talk to the API.
func (s *Server) hostOK(host string) bool {
	h, p, err := net.SplitHostPort(host)
	h = strings.ToLower(h)
	if err != nil || (h != "127.0.0.1" && h != "localhost") {
		return false
	}
	return s.ln == nil || p == strconv.Itoa(s.port())
}

// decodeBody reads a JSON object into v, rejecting unknown fields so typos
// don't pass silently.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	ok, _ := decodeBodyOpt(w, r, v)
	return ok
}

// maxBodyBytes limits JSON request bodies.
const maxBodyBytes = 1 << 20

// decodeBodyOpt is decodeBody that also reports an empty body (ok, empty).
// A body above 1 MiB is answered with 413 payload_too_large.
func decodeBodyOpt(w http.ResponseWriter, r *http.Request, v any) (ok, empty bool) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			apiFail(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is larger than 1 MiB")
		case errors.Is(err, io.EOF):
			return true, true
		default:
			badRequest(w, "invalid JSON body: %v", err)
		}
		return false, false
	}
	return true, false
}

// apiV1 builds the handler of /api/v1/ from the route table: the scope check
// first, then the check of the areas, then the route.
func (s *Server) apiV1() http.Handler {
	m := http.NewServeMux()
	scopes := map[string]string{}
	for _, rt := range s.v1Routes() {
		m.HandleFunc(rt.Pattern, rt.Handler)
		scopes[rt.Pattern] = rt.Scope
	}
	m.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		// A known path with another method is 405, anything else 404.
		var allow []string
		for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE"} {
			r2 := r.Clone(r.Context())
			r2.Method = method
			if _, pattern := m.Handler(r2); pattern != "" && pattern != "/api/v1/" {
				allow = append(allow, method)
			}
		}
		if len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
			apiFail(w, http.StatusMethodNotAllowed, "method_not_allowed", fmt.Sprintf("%s is not allowed here; use %s", r.Method, strings.Join(allow, ", ")))
			return
		}
		notFound(w, "no such endpoint: %s %s", r.Method, r.URL.Path)
	})
	return s.scopeGate(m, scopes, s.areaGate(m))
}

// --- status and panels ---

type v1Panel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Model      string `json:"model"`
	W          int    `json:"w"`
	H          int    `json:"h"`
	Connected  bool   `json:"connected"`
	Mode       string `json:"mode"`
	Design     string `json:"design"`
	Theme      string `json:"theme"`
	Brightness int    `json:"brightness"`
	Rotation   int    `json:"rotation"`
	FanNo      int    `json:"fanNo"`
	// Firmware is the version the display reported ("" if unknown); read only.
	Firmware string `json:"firmware"`
	Error    string `json:"error"`
	// Grid is the id of the field layout of a bar display ("" = one design on
	// the whole screen); Tiles are its fields in cell order.
	Grid  string   `json:"grid"`
	Tiles []v1Tile `json:"tiles"`
	// Layout is the id of the stored or ready-made layout the display uses
	// ("" = an own composition or a single design); see GET /api/v1/layouts.
	Layout string `json:"layout"`
}

// v1Tile is one field of a grid: a small display of its own.
type v1Tile struct {
	Mode       string `json:"mode"` // hud, media or off
	Design     string `json:"design"`
	Media      string `json:"media"`
	Theme      string `json:"theme"` // dark, light or "" (the panel's theme)
	Background string `json:"background"`
	BgStrength int    `json:"bgStrength"`
	BgBloom    bool   `json:"bgBloom"`
}

func toV1Tiles(ts []app.Tile) []v1Tile {
	out := make([]v1Tile, len(ts))
	for i, t := range ts {
		out[i] = v1Tile{Mode: v1Mode(t.Mode), Design: t.Design, Media: t.Media, Theme: t.Theme,
			Background: t.Background, BgStrength: t.BgStrength, BgBloom: t.BgBloom}
		if out[i].Mode == "" {
			out[i].Mode = "hud"
		}
	}
	return out
}

func (t v1Tile) tile() app.Tile {
	m := t.Mode
	if m == "hud" || m == "" {
		m = "design"
	}
	return app.Tile{Mode: m, Design: t.Design, Media: t.Media, Theme: t.Theme,
		Background: t.Background, BgStrength: t.BgStrength, BgBloom: t.BgBloom}
}

// v1Mode maps the engine's mode names to the API's.
func v1Mode(m string) string {
	if m == "design" {
		return "hud"
	}
	return m
}

// panelModel is the product name without the port/fan suffix.
func panelModel(name string) string {
	if i := strings.Index(name, " ("); i > 0 {
		return name[:i]
	}
	return name
}

func toV1Panel(p app.PanelState, fanNo int) v1Panel {
	name := p.Config.Name
	if name == "" {
		name = p.Name
	}
	theme := p.Config.Theme
	if theme == "" {
		theme = "dark"
	}
	code := ""
	if p.Error != "" {
		code = p.ErrorCode
		if code == "" {
			code = app.ErrDevice
		}
	}
	return v1Panel{ID: p.ID, Name: name, Model: panelModel(p.Name), W: p.Width, H: p.Height, Connected: true,
		Mode: v1Mode(p.Config.Mode), Design: p.Config.Design, Theme: theme, Brightness: p.Config.Brightness,
		Rotation: p.Config.Rotation, FanNo: fanNo, Firmware: p.Firmware, Error: code,
		Grid: p.Config.Grid, Tiles: toV1Tiles(p.Config.Tiles), Layout: p.Config.Layout}
}

func (s *Server) v1Panels() []v1Panel {
	ps := s.Engine.Panels()
	out := make([]v1Panel, len(ps))
	for i, p := range ps {
		out[i] = toV1Panel(p, p.Number)
	}
	return out
}

func (s *Server) panelExists(id string) bool {
	return slices.ContainsFunc(s.Engine.Panels(), func(p app.PanelState) bool { return p.ID == id })
}

type v1Status struct {
	Version         string `json:"version"`
	APIVersion      int    `json:"apiVersion"`
	UpdateAvailable string `json:"updateAvailable"`
	Sensors         struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	} `json:"sensors"`
	Power bool `json:"power"`
	// DisclaimerAccepted is false until the user accepted the disclaimer in the
	// CaseHUD window; until then CaseHUD sends nothing to any device.
	DisclaimerAccepted bool `json:"disclaimerAccepted"`
	// Scene is the saved scene on screen now, however it got there (rotation,
	// automation, or activated by hand without saving), else "".
	Scene  string `json:"scene"`
	Gaming struct {
		Active bool   `json:"active"`
		Source string `json:"source"`
		Game   string `json:"game"`
	} `json:"gaming"`
	Rotation struct {
		Enabled bool   `json:"enabled"`
		Scene   string `json:"scene"`
	} `json:"rotation"`
	Automation struct {
		Enabled    bool   `json:"enabled"`
		ActiveRule int    `json:"activeRule"`
		When       string `json:"when"`
		Scene      string `json:"scene"`
	} `json:"automation"`
	Awtrix struct {
		Enabled bool           `json:"enabled"`
		Devices []v1AwtrixInfo `json:"devices"`
	} `json:"awtrix"`
	Lighting struct {
		Enabled bool   `json:"enabled"`
		Active  bool   `json:"active"`
		Preset  string `json:"preset"` // ID of the matching lighting preset, "" when none
	} `json:"lighting"`
	// Areas tells which core areas are switched on in the settings. What an area
	// that is off would report (displays, lighting) is reported as off.
	Areas app.Modules `json:"areas"`
	// Conflicts names the programs of a fixed list that run and may use the
	// same devices (image names only; closing them is GUI-only).
	Conflicts []string  `json:"conflicts"`
	Panels    []v1Panel `json:"panels"`
}

type v1AwtrixInfo struct {
	Name  string `json:"name"`
	Host  string `json:"host"`
	Model string `json:"model"`
	OK    bool   `json:"ok"`
}

func (s *Server) v1Status() v1Status {
	e := s.Engine
	var st v1Status
	st.Version, st.APIVersion = s.Version, APIVersion
	if u := s.availableUpdate(); u != nil {
		st.UpdateAvailable = u.Version
	}
	st.Sensors.Error = e.Provider().Err()
	st.Sensors.OK = st.Sensors.Error == ""
	st.Power = e.Power()
	st.DisclaimerAccepted = e.DisclaimerAccepted()
	for _, sc := range e.Scenes() {
		if sc.Active {
			st.Scene = sc.Name
			break
		}
	}
	g := e.Gaming(e.Clock())
	st.Gaming.Active, st.Gaming.Source, st.Gaming.Game = g.Active, g.Source, g.Game
	st.Rotation.Enabled = e.Rotation().Enabled
	if rs := e.RotationStatus(); rs.Active {
		st.Rotation.Scene = rs.Scene
	}
	as := e.AutomationStatus()
	st.Automation.Enabled, st.Automation.ActiveRule, st.Automation.When, st.Automation.Scene = as.Enabled, as.ActiveRule, as.When, as.Scene
	ds, stat, infos := e.AwtrixDevices()
	st.Awtrix.Devices = []v1AwtrixInfo{}
	for _, d := range ds {
		if !d.Disabled {
			st.Awtrix.Enabled = true
		}
		msg, seen := stat[d.ID]
		name := d.Name
		if name == "" {
			name = d.ID
		}
		st.Awtrix.Devices = append(st.Awtrix.Devices, v1AwtrixInfo{Name: name, Host: d.Host,
			Model: infos[d.ID].Model, OK: !d.Disabled && seen && msg == ""})
	}
	st.Areas = e.Modules()
	st.Awtrix.Enabled = st.Awtrix.Enabled && st.Areas.Display
	lb := e.LightingBrief()
	st.Lighting.Enabled, st.Lighting.Active = lb.Enabled && st.Areas.Lighting, lb.Active
	st.Lighting.Preset = e.ActiveLightingPreset()
	st.Conflicts = e.ConflictsRunning()
	st.Panels = s.v1Panels()
	return st
}

type v1PanelPatch struct {
	Design     *string `json:"design"`
	Mode       *string `json:"mode"`
	Theme      *string `json:"theme"`
	Brightness *int    `json:"brightness"`
	Rotation   *int    `json:"rotation"`
	// Grid ("" clears it) and Tiles set the field layout of a bar display.
	Grid  *string   `json:"grid"`
	Tiles *[]v1Tile `json:"tiles"`
	// Layout puts a layout on the display (its grid and fields come with it);
	// "" keeps the current fields as an own composition.
	Layout *string `json:"layout"`
}

// v1PatchPanel changes only the fields present in the body.
func (s *Server) v1PatchPanel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.panelExists(id) {
		notFound(w, "panel %q not found", id)
		return
	}
	var p v1PanelPatch
	if !decodeBody(w, r, &p) {
		return
	}
	if p.Mode != nil && !slices.Contains([]string{"hud", "media", "off"}, *p.Mode) {
		badRequest(w, "mode must be hud, media or off")
		return
	}
	if p.Theme != nil && *p.Theme != "dark" && *p.Theme != "light" {
		badRequest(w, "theme must be dark or light")
		return
	}
	if p.Brightness != nil && (*p.Brightness < 0 || *p.Brightness > 100) {
		badRequest(w, "brightness must be 0-100")
		return
	}
	if p.Rotation != nil && !slices.Contains([]int{0, 90, 180, 270}, *p.Rotation) {
		badRequest(w, "rotation must be 0, 90, 180 or 270")
		return
	}
	if p.Layout != nil && *p.Layout != "" {
		if _, ok := s.Engine.Store.Layout(*p.Layout); !ok {
			badRequest(w, "layout %q not found", *p.Layout)
			return
		}
		p.Grid, p.Tiles = nil, nil // the layout brings its own
	}
	if p.Tiles != nil && p.Grid == nil {
		// Tiles go with the grid already set on the display.
		for _, ps := range s.Engine.Panels() {
			if ps.ID == id && ps.Config.Grid == "" {
				badRequest(w, "tiles need a grid: send \"grid\" too")
				return
			}
		}
	}
	if p.Grid != nil && *p.Grid != "" && p.Tiles == nil {
		for _, ps := range s.Engine.Panels() {
			if ps.ID == id && ps.Config.Grid != *p.Grid {
				badRequest(w, "a new grid needs its tiles")
				return
			}
		}
	}
	if p.Design != nil {
		if _, err := s.Engine.Store.Design(*p.Design); err != nil {
			badRequest(w, "%v", err)
			return
		}
	}
	_, err := s.Engine.UpdatePanelConfig(id, func(c *app.PanelConfig) error {
		if p.Mode != nil {
			c.Mode = *p.Mode
			if c.Mode == "hud" {
				c.Mode = "design"
			}
		}
		if p.Design != nil {
			c.Design = *p.Design
			if p.Grid == nil && p.Tiles == nil && p.Layout == nil {
				c.Grid, c.Tiles, c.Layout = "", nil, "" // a single design replaces the grid
			}
		}
		if p.Layout != nil {
			c.Layout = *p.Layout
		} else if p.Grid != nil || p.Tiles != nil {
			c.Layout = "" // edited by hand: no longer the stored composition
		}
		if p.Grid != nil {
			c.Grid = *p.Grid
			if c.Grid == "" {
				c.Tiles = nil
			}
		}
		if p.Tiles != nil {
			c.Tiles = make([]app.Tile, len(*p.Tiles))
			for i, t := range *p.Tiles {
				c.Tiles[i] = t.tile()
			}
		}
		if p.Theme != nil {
			c.Theme = ""
			if *p.Theme == "light" {
				c.Theme = "light"
			}
		}
		if p.Brightness != nil {
			c.Brightness = *p.Brightness
		}
		if p.Rotation != nil {
			c.Rotation = *p.Rotation
		}
		if p.Mode != nil && c.Mode == "media" && c.Media == "" {
			return errors.New("no media file is chosen for this display yet")
		}
		return nil
	})
	if err != nil {
		badRequest(w, "%v", err)
		return
	}
	for _, ps := range s.Engine.Panels() {
		if ps.ID == id {
			writeJSON(w, toV1Panel(ps, ps.Number))
			return
		}
	}
	notFound(w, "panel %q not found", id)
}

// jpegToPNG converts the panel's last frame for API clients.
func jpegToPNG(b []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = png.Encode(&out, img)
	return out.Bytes(), err
}

func (s *Server) v1PanelPreview(w http.ResponseWriter, r *http.Request) {
	b := s.Engine.Preview(r.PathValue("id"))
	if b == nil {
		notFound(w, "no frame for panel %q yet", r.PathValue("id"))
		return
	}
	out, err := jpegToPNG(b)
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(out)
}

// --- designs and sets ---

type v1Design struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Set     string `json:"set"`
	Theme   string `json:"theme"`
	Builtin bool   `json:"builtin"`
	W       int    `json:"w"`
	H       int    `json:"h"`
}

func (s *Server) v1Designs(w http.ResponseWriter, r *http.Request) {
	out := []v1Design{}
	for _, d := range s.Engine.Store.Designs() {
		set := "custom"
		if d.Builtin && d.Set != "" {
			set = d.Set
		}
		l, _ := s.Engine.Store.Design(d.ID)
		out = append(out, v1Design{ID: d.ID, Name: d.Name, Set: set, Theme: "dark", Builtin: d.Builtin, W: l.Width, H: l.Height})
	}
	writeJSON(w, out)
}

// v1PNG renders layout l about ?size pixels wide (32..480, default 200) as a
// PNG, in ?theme=dark|light.
func (s *Server) v1PNG(w http.ResponseWriter, r *http.Request, l lcd.Layout) {
	size := 200
	if v := r.URL.Query().Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			badRequest(w, "size must be a number")
			return
		}
		size = n
	}
	size = max(32, min(480, size))
	theme := r.URL.Query().Get("theme")
	if theme != "" && theme != "dark" && theme != "light" {
		badRequest(w, "theme must be dark or light")
		return
	}
	l = lcd.ApplyTheme(l, theme)
	if l.Width <= 0 || l.Height <= 0 {
		internalError(w, errors.New("design has no size"))
		return
	}
	h := max(1, min(size*l.Height/l.Width, 2048))
	img, err := s.Engine.RenderDesignWithBackground(l, size, h, "", 0, false)
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func (s *Server) v1DesignPreview(w http.ResponseWriter, r *http.Request) {
	l, err := s.Engine.Store.Design(r.PathValue("id"))
	if err != nil {
		notFound(w, "%v", err)
		return
	}
	s.v1PNG(w, r, l)
}

type v1Set struct {
	Set   string `json:"set"`
	Theme string `json:"theme"`
	Name  string `json:"name"`
}

func (s *Server) v1Sets(w http.ResponseWriter, r *http.Request) {
	out := []v1Set{}
	for _, set := range app.SetOrder {
		for _, th := range []string{"dark", "light"} {
			out = append(out, v1Set{Set: set, Theme: th, Name: strings.ToUpper(set[:1]) + set[1:] + " (" + th + ")"})
		}
	}
	writeJSON(w, out)
}

func (s *Server) v1ApplySet(w http.ResponseWriter, r *http.Request) {
	var req v1ApplySetRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Theme == "" {
		req.Theme = "dark"
	}
	done, err := s.Engine.ApplySet(req.Set, req.Theme, req.Panels)
	switch {
	case errors.Is(err, app.ErrUnknownSet):
		badRequest(w, "unknown set %q", req.Set)
		return
	case err != nil:
		msg := err.Error()
		if len(done) > 0 {
			msg += "; already applied to: " + strings.Join(done, ", ")
		}
		if errors.Is(err, app.ErrNoPanel) {
			notFound(w, "%s", msg)
		} else {
			badRequest(w, "%s", msg)
		}
		return
	}
	if done == nil {
		done = []string{}
	}
	writeJSON(w, v1Applied{Applied: done})
}

// --- scenes, rotation, power, gaming ---

func (s *Server) v1Scenes(w http.ResponseWriter, r *http.Request) {
	out := []v1Scene{}
	for _, sc := range s.Engine.Scenes() {
		out = append(out, v1Scene{sc.Name, sc.Name, sc.Active})
	}
	writeJSON(w, out)
}

func (s *Server) v1ActivateScene(w http.ResponseWriter, r *http.Request) {
	// persist (query ?persist=true or body {"persist":true}) saves the scene as
	// the displays' configuration; by default it is only shown for now.
	var req v1ActivateRequest
	if ok, _ := decodeBodyOpt(w, r, &req); !ok {
		return
	}
	if q := r.URL.Query().Get("persist"); q == "true" || q == "1" {
		req.Persist = true
	}
	switch err := s.Engine.ActivateScene(r.PathValue("id"), req.Persist); {
	case errors.Is(err, app.ErrNoScene):
		notFound(w, "scene %q not found", r.PathValue("id"))
	case errors.Is(err, app.ErrGamingHold):
		apiFail(w, http.StatusConflict, "conflict", err.Error())
	case err != nil:
		internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) v1PatchRotation(w http.ResponseWriter, r *http.Request) {
	var req v1EnabledRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		badRequest(w, "enabled is required")
		return
	}
	if err := s.Engine.SetRotationEnabled(*req.Enabled); err != nil {
		badRequest(w, "%v", err)
		return
	}
	writeJSON(w, v1Enabled{Enabled: s.Engine.Rotation().Enabled})
}

func (s *Server) v1PatchAutomation(w http.ResponseWriter, r *http.Request) {
	var req v1EnabledRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		badRequest(w, "enabled is required")
		return
	}
	a := s.Engine.Automation()
	a.Enabled = *req.Enabled
	if err := s.Engine.SetAutomation(a); err != nil {
		badRequest(w, "%v", err)
		return
	}
	writeJSON(w, v1Enabled{Enabled: s.Engine.Automation().Enabled})
}

func (s *Server) v1Power(w http.ResponseWriter, r *http.Request) {
	var req v1PowerRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.On == nil {
		badRequest(w, "on is required")
		return
	}
	s.Engine.SetPower(*req.On)
	writeJSON(w, v1PowerState{Power: s.Engine.Power()})
}

func (s *Server) v1Gaming(w http.ResponseWriter, r *http.Request) {
	var req v1GamingRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Active == nil {
		badRequest(w, "active is required")
		return
	}
	lease := app.DefaultGamingLease
	if req.LeaseSeconds != nil {
		lease = time.Duration(max(10, min(3600, *req.LeaseSeconds))) * time.Second
	}
	s.Engine.SetGamingHint(*req.Active, req.Game, req.PID, lease)
	g := s.Engine.Gaming(s.Engine.Clock())
	writeJSON(w, v1GamingState{Active: g.Active, Source: g.Source, Game: g.Game})
}

// --- community ---

func (s *Server) v1CommunityList(w http.ResponseWriter, r *http.Request) {
	es, err := s.community().Entries(r.Context())
	if err != nil {
		badGateway(w, err)
		return
	}
	out := []v1CommunityEntry{}
	for _, e := range es { // the gallery index has no set or theme: they stay empty
		out = append(out, v1CommunityEntry{ID: e.ID, Name: e.Name, Author: e.Author})
	}
	writeJSON(w, out)
}

// communityDesign fetches a gallery design, answering the request itself on
// failure.
func (s *Server) communityDesign(w http.ResponseWriter, r *http.Request) (lcd.Layout, string, bool) {
	id := r.PathValue("id")
	es, err := s.community().Entries(r.Context())
	if err != nil {
		badGateway(w, err)
		return lcd.Layout{}, "", false
	}
	if !slices.ContainsFunc(es, func(e community.Entry) bool { return e.ID == id }) {
		notFound(w, "design %q is not in the community gallery", id)
		return lcd.Layout{}, "", false
	}
	l, e, err := s.community().Design(r.Context(), id)
	if err != nil {
		badGateway(w, err)
		return lcd.Layout{}, "", false
	}
	return l, e.ID, true
}

func (s *Server) v1CommunityPreview(w http.ResponseWriter, r *http.Request) {
	if l, _, ok := s.communityDesign(w, r); ok {
		s.v1PNG(w, r, l)
	}
}

func (s *Server) v1CommunityImport(w http.ResponseWriter, r *http.Request) {
	l, eid, ok := s.communityDesign(w, r)
	if !ok {
		return
	}
	id := "community-" + eid
	if err := s.Engine.Store.SaveDesign(id, l); err != nil {
		badRequest(w, "%v", err)
		return
	}
	s.Engine.DesignChanged(id)
	writeJSON(w, v1ID{ID: id})
}

// --- awtrix, quit ---

func (s *Server) v1PatchAwtrix(w http.ResponseWriter, r *http.Request) {
	var req v1EnabledRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		badRequest(w, "enabled is required")
		return
	}
	if err := s.Engine.SetAwtrixEnabled(*req.Enabled); err != nil {
		badRequest(w, "%v", err)
		return
	}
	writeJSON(w, v1Enabled{Enabled: s.v1Status().Awtrix.Enabled})
}

func (s *Server) v1Quit(w http.ResponseWriter, r *http.Request) {
	if s.OnQuit == nil {
		internalError(w, errors.New("quit is not available"))
		return
	}
	s.SetEnding()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	io.WriteString(w, `{"status":"quitting"}`+"\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(200 * time.Millisecond) // let the response leave first
		s.OnQuit()
	}()
}

// v1GUIOpen asks the app to show (or focus) its window.
func (s *Server) v1GUIOpen(w http.ResponseWriter, r *http.Request) {
	if s.OnShow == nil {
		apiFail(w, http.StatusServiceUnavailable, "unavailable", "no window available")
		return
	}
	// OnShow returns once the window is visible (or the browser took over).
	if err := s.OnShow(); err != nil {
		apiFail(w, http.StatusServiceUnavailable, "unavailable", err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// --- events ---

const (
	eventsCheck     = 500 * time.Millisecond
	eventsPing      = 15 * time.Second
	maxEventClients = 8
	// maxEventsPerClient is how many of these one program (one token) may hold.
	maxEventsPerClient = 3
)

// statusJSON is the marshalled status, built at most once per check interval
// however many event streams are open.
func (s *Server) statusJSON() ([]byte, error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.statusBuf != nil && time.Since(s.statusAt) < eventsCheck {
		return s.statusBuf, nil
	}
	b, err := json.Marshal(s.v1Status())
	if err != nil {
		return nil, err
	}
	s.statusBuf, s.statusAt = b, time.Now()
	return b, nil
}

// v1Events streams the status as server-sent events: "status" on connect and
// whenever it changes, "cooling" when the preset, state, alarm or a problem of
// the cooling changes, "ping" every 15 s.
func (s *Server) v1Events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		internalError(w, errors.New("streaming unsupported"))
		return
	}
	if s.eventClients.Add(1) > maxEventClients {
		s.eventClients.Add(-1)
		apiFail(w, http.StatusServiceUnavailable, "too_many_clients", "too many event streams are open")
		return
	}
	defer s.eventClients.Add(-1)
	// One program may not take all the slots.
	who, _ := identityOf(r)
	s.eventMu.Lock()
	if s.eventBy[who.ID] >= maxEventsPerClient {
		s.eventMu.Unlock()
		apiFail(w, http.StatusServiceUnavailable, "too_many_clients", "this program already has the most event streams it may open")
		return
	}
	if s.eventBy == nil {
		s.eventBy = map[string]int{}
	}
	s.eventBy[who.ID]++
	s.eventMu.Unlock()
	defer func() {
		s.eventMu.Lock()
		if s.eventBy[who.ID]--; s.eventBy[who.ID] <= 0 {
			delete(s.eventBy, who.ID)
		}
		s.eventMu.Unlock()
	}()
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	check := time.NewTicker(eventsCheck)
	defer check.Stop()
	ping := time.NewTicker(eventsPing)
	defer ping.Stop()
	var last []byte
	lastCooling := s.Engine.CoolingAPIView().EventKey() // the state on connect is read with GET
	for {
		if v := s.Engine.CoolingAPIView(); v.EventKey() != lastCooling {
			lastCooling = v.EventKey()
			if b, err := json.Marshal(v); err == nil {
				if _, err := fmt.Fprintf(w, "event: cooling\ndata: %s\n\n", b); err != nil {
					return
				}
				fl.Flush()
			}
		}
		b, err := s.statusJSON()
		if err == nil && !bytes.Equal(b, last) {
			last = b
			if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", b); err != nil {
				return
			}
			fl.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := fmt.Fprint(w, "event: ping\ndata: {}\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-check.C:
		}
	}
}

// v1Layout is a composition for bar displays that displays can refer to.
type v1Layout struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"` // "" for ready-made ones: see tpl and set
	Builtin bool     `json:"builtin"`
	Tpl     string   `json:"tpl"` // ready-made composition: overview, gaming, watercooling, clock
	Set     string   `json:"set"` // design set of a ready-made layout
	Grid    string   `json:"grid"`
	Tiles   []v1Tile `json:"tiles"`
}

func (s *Server) v1Layouts(w http.ResponseWriter, r *http.Request) {
	ls := s.Engine.Layouts()
	out := make([]v1Layout, len(ls))
	for i, l := range ls {
		out[i] = v1Layout{ID: l.ID, Name: l.Name, Builtin: l.Builtin, Tpl: l.Tpl, Set: l.Set, Grid: l.Grid, Tiles: toV1Tiles(l.Tiles)}
	}
	writeJSON(w, out)
}
