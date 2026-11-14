package web

import (
	"errors"
	"net/http"
	"slices"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/autostart"
)

// The routes of /api/v1/ that mirror what the window reads and changes of the
// rotation, the automation, the AWTRIX clocks, designs, layouts, scenes and a
// whitelist of the settings. They call the same engine functions as the
// window's own routes, so the same validation applies.

// autostartSet, autostartEnabled and autostartSupported are the access to the
// autostart entry; tests replace them.
var (
	autostartSet       = autostart.Set
	autostartEnabled   = autostart.Enabled
	autostartSupported = autostart.Supported
)

// moreRoutesV1 lists those routes.
func (s *Server) moreRoutesV1() []v1Route {
	return []v1Route{
		{"GET /api/v1/rotation", scopeRead, s.v1GetRotation},
		{"GET /api/v1/automation", scopeRead, s.v1GetAutomation},
		{"GET /api/v1/awtrix", scopeRead, s.v1GetAwtrix},
		{"DELETE /api/v1/designs/{id}", scopeControl, s.v1DeleteDesign},
		{"GET /api/v1/layouts/{id}", scopeRead, s.v1GetLayout},
		{"PUT /api/v1/layouts/{id}", scopeControl, s.v1PutLayout},
		{"DELETE /api/v1/layouts/{id}", scopeControl, s.v1DeleteLayout},
		{"POST /api/v1/scenes", scopeControl, s.v1SaveScene},
		{"PATCH /api/v1/scenes/{id}", scopeControl, s.v1RenameScene},
		{"GET /api/v1/settings", scopeRead, s.v1GetSettings},
		{"PATCH /api/v1/settings", scopeControl, s.v1PatchSettings},
	}
}

// --- rotation, automation, awtrix (read) ---

// v1Rotation is the scene rotation: its settings and what it does now. The
// scenes themselves are listed by GET /scenes.
type v1Rotation struct {
	Enabled bool `json:"enabled"`
	// Mode is scenes (cycle the saved scenes) or conveyor (designs move down
	// through the displays).
	Mode            string   `json:"mode"`
	IntervalSeconds int      `json:"intervalSeconds"`
	Effect          string   `json:"effect"`
	Cascade         bool     `json:"cascade"`
	Conveyor        []string `json:"conveyor"` // design ids
	Order           []string `json:"order"`    // display ids top to bottom ("" = window order)
	// Active: the rotation currently decides what the displays show; Scene is
	// the scene on screen.
	Active bool   `json:"active"`
	Scene  string `json:"scene"`
	// Gaming: a game runs and holds the rotation.
	Gaming bool `json:"gaming"`
}

func (s *Server) v1GetRotation(w http.ResponseWriter, r *http.Request) {
	c, st := s.Engine.Rotation(), s.Engine.RotationStatus()
	writeJSON(w, v1Rotation{Enabled: c.Enabled, Mode: c.Mode, IntervalSeconds: c.IntervalS, Effect: c.Effect, Cascade: c.Cascade,
		Conveyor: nonNil(c.Conveyor), Order: nonNil(c.Order), Active: st.Active, Scene: st.Scene, Gaming: st.Gaming})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return slices.Clone(s)
}

// v1AutomationRule is one "when ... show scene ..." line.
type v1AutomationRule struct {
	When  string `json:"when"` // game, or game:<exe>
	Scene string `json:"scene"`
	Off   bool   `json:"off"`
}

// v1AutomationState is what the automation does now.
type v1AutomationState struct {
	// ActiveRule is the index of the rule in charge, -1 for none; Pending the
	// rule about to take over (-1 none, -2 about to let go).
	ActiveRule int    `json:"activeRule"`
	Pending    int    `json:"pending"`
	When       string `json:"when"`
	Scene      string `json:"scene"`
	Game       string `json:"game"`
}

// v1Automation is the automation: its rules and what it does now.
type v1Automation struct {
	Enabled      bool               `json:"enabled"`
	Rules        []v1AutomationRule `json:"rules"`
	EnterSeconds int                `json:"enterSeconds"`
	LeaveSeconds int                `json:"leaveSeconds"`
	IgnoreGames  []string           `json:"ignoreGames"`
	State        v1AutomationState  `json:"state"`
	Gaming       v1GamingState      `json:"gaming"`
}

func (s *Server) v1GetAutomation(w http.ResponseWriter, r *http.Request) {
	c, st := s.Engine.Automation(), s.Engine.AutomationStatus()
	g := s.Engine.Gaming(s.Engine.Clock())
	out := v1Automation{Enabled: c.Enabled, Rules: []v1AutomationRule{}, EnterSeconds: c.EnterS, LeaveSeconds: c.LeaveS,
		IgnoreGames: nonNil(c.IgnoreGames),
		State:       v1AutomationState{ActiveRule: st.ActiveRule, Pending: st.Pending, When: st.When, Scene: st.Scene, Game: st.Game},
		Gaming:      v1GamingState{Active: g.Active, Source: g.Source, Game: g.Game}}
	for _, ru := range c.Rules {
		out.Rules = append(out.Rules, v1AutomationRule{When: ru.When, Scene: ru.Scene, Off: ru.Off})
	}
	writeJSON(w, out)
}

// v1AwtrixClock is a configured AWTRIX clock.
type v1AwtrixClock struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Apps     []string `json:"apps"`
	Disabled bool     `json:"disabled"`
	Model    string   `json:"model"` // tc001, tc002 or "" when unknown
	OK       bool     `json:"ok"`
	// Status is the last problem pushing to the clock, "" if none.
	Status string `json:"status"`
}

// v1Awtrix is the AWTRIX clocks: the list and the apps CaseHUD can show.
type v1Awtrix struct {
	Enabled bool            `json:"enabled"`
	Clocks  []v1AwtrixClock `json:"clocks"`
	Apps    []string        `json:"apps"`
}

func (s *Server) v1GetAwtrix(w http.ResponseWriter, r *http.Request) {
	ds, stat, infos := s.Engine.AwtrixDevices()
	out := v1Awtrix{Clocks: []v1AwtrixClock{}, Apps: []string{}}
	for _, a := range app.AwtrixApps {
		out.Apps = append(out.Apps, a.ID)
	}
	for _, d := range ds {
		if !d.Disabled {
			out.Enabled = true
		}
		msg, seen := stat[d.ID]
		name := d.Name
		if name == "" {
			name = d.ID
		}
		out.Clocks = append(out.Clocks, v1AwtrixClock{ID: d.ID, Name: name, Host: d.Host, Apps: nonNil(d.Apps), Disabled: d.Disabled,
			Model: infos[d.ID].Model, OK: !d.Disabled && seen && msg == "", Status: msg})
	}
	out.Enabled = out.Enabled && s.Engine.Modules().Display
	writeJSON(w, out)
}

// --- designs and layouts ---

// designInUse names where a design is on screen or stored for later, "" if
// nowhere.
func (s *Server) designInUse(id string) string {
	usesTiles := func(ts []app.Tile) bool {
		return slices.ContainsFunc(ts, func(t app.Tile) bool { return t.Design == id })
	}
	for _, p := range s.Engine.Panels() {
		if p.Config.Design == id || usesTiles(p.Config.Tiles) {
			return "a display"
		}
	}
	rc := s.Engine.Rotation()
	if slices.Contains(rc.Conveyor, id) {
		return "the rotation"
	}
	for _, sc := range rc.Scenes {
		for _, c := range sc.Panels {
			if c.Design == id || usesTiles(c.Tiles) {
				return "the scene " + sc.Name
			}
		}
	}
	for _, l := range s.Engine.Layouts() {
		if usesTiles(l.Tiles) {
			return "the layout " + l.ID
		}
	}
	return ""
}

// v1DeleteDesign deletes a design of the user. The designs that come with
// CaseHUD cannot be deleted (a copy saved over one is removed this way, which
// brings the original back), and a design that a display, scene, layout or the
// rotation uses stays.
func (s *Server) v1DeleteDesign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var info *app.DesignInfo
	for _, d := range s.Engine.Store.Designs() {
		if d.ID == id {
			info = &d
			break
		}
	}
	switch {
	case info == nil:
		notFound(w, "design %q not found", id)
		return
	case info.Builtin:
		apiFail(w, http.StatusConflict, "builtin", "designs that come with CaseHUD cannot be deleted")
		return
	}
	if !slices.Contains(app.BuiltinOrder, id) {
		if where := s.designInUse(id); where != "" {
			apiFail(w, http.StatusConflict, "in_use", "the design is used by "+where+"; put another design there first")
			return
		}
	}
	if err := s.Engine.Store.DeleteDesign(id); err != nil {
		badRequest(w, "%v", err)
		return
	}
	s.Engine.DesignChanged(id)
	w.WriteHeader(http.StatusNoContent)
}

// v1LayoutBody is the body of PUT /layouts/{id}.
type v1LayoutBody struct {
	Name  string   `json:"name"`
	Grid  string   `json:"grid"`
	Tiles []v1Tile `json:"tiles"`
}

func (s *Server) layoutV1(id string) (v1Layout, bool) {
	for _, l := range s.Engine.Layouts() {
		if l.ID == id {
			return v1Layout{ID: l.ID, Name: l.Name, Builtin: l.Builtin, Tpl: l.Tpl, Set: l.Set, Grid: l.Grid, Tiles: toV1Tiles(l.Tiles)}, true
		}
	}
	return v1Layout{}, false
}

func (s *Server) v1GetLayout(w http.ResponseWriter, r *http.Request) {
	l, ok := s.layoutV1(r.PathValue("id"))
	if !ok {
		notFound(w, "layout %q not found", r.PathValue("id"))
		return
	}
	writeJSON(w, l)
}

// v1PutLayout stores a layout of the user and updates the displays and scenes
// that use it. The ready-made layouts cannot be replaced.
func (s *Server) v1PutLayout(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body v1LayoutBody
	if !decodeBody(w, r, &body) {
		return
	}
	if l, ok := s.layoutV1(id); ok && l.Builtin {
		apiFail(w, http.StatusConflict, "builtin", "ready-made layouts cannot be changed; save a copy under another id")
		return
	}
	tiles := make([]app.Tile, len(body.Tiles))
	for i, t := range body.Tiles {
		tiles[i] = t.tile()
	}
	if err := s.Engine.SaveLayout(id, app.Layout{Name: body.Name, Grid: body.Grid, Tiles: tiles}); err != nil {
		badRequest(w, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// v1DeleteLayout deletes a layout of the user; displays and scenes that used it
// keep what it showed as their own composition.
func (s *Server) v1DeleteLayout(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	l, ok := s.layoutV1(id)
	switch {
	case !ok:
		notFound(w, "layout %q not found", id)
		return
	case l.Builtin:
		apiFail(w, http.StatusConflict, "builtin", "ready-made layouts cannot be deleted")
		return
	}
	if err := s.Engine.DeleteLayout(id); err != nil {
		badRequest(w, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- scenes ---

// v1SceneSave is the body of POST /scenes.
type v1SceneSave struct {
	Name string `json:"name"`
	// Overwrite replaces a scene of that name; without it that is 409 exists.
	Overwrite bool `json:"overwrite"`
	// NoRotation keeps the scene out of the rotation (left out: a scene that
	// is replaced keeps its flag).
	NoRotation *bool `json:"noRotation"`
}

// v1SceneRename is the body of PATCH /scenes/{id}.
type v1SceneRename struct {
	Name string `json:"name"`
}

func (s *Server) sceneByName(name string) (v1Scene, bool) {
	for _, sc := range s.Engine.Scenes() {
		if sc.Name == name {
			return v1Scene{ID: sc.Name, Name: sc.Name, Active: sc.Active}, true
		}
	}
	return v1Scene{}, false
}

// v1SaveScene stores what the displays are configured to show as a scene.
func (s *Server) v1SaveScene(w http.ResponseWriter, r *http.Request) {
	var req v1SceneSave
	if !decodeBody(w, r, &req) {
		return
	}
	name, err := app.CleanSceneName(req.Name)
	if err != nil {
		badRequest(w, "%v", err)
		return
	}
	if _, exists := s.sceneByName(name); exists && !req.Overwrite {
		apiFail(w, http.StatusConflict, "exists", "a scene named "+name+" exists already; send overwrite: true to replace it")
		return
	}
	if err := s.Engine.SaveSceneWith(name, req.NoRotation); err != nil {
		badRequest(w, "%v", err)
		return
	}
	sc, _ := s.sceneByName(name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSONBody(w, sc)
}

// v1RenameScene renames a scene; the automation rules that point to it follow.
func (s *Server) v1RenameScene(w http.ResponseWriter, r *http.Request) {
	var req v1SceneRename
	if !decodeBody(w, r, &req) {
		return
	}
	from := r.PathValue("id")
	to, err := app.CleanSceneName(req.Name)
	if err != nil {
		badRequest(w, "%v", err)
		return
	}
	if _, ok := s.sceneByName(from); !ok {
		notFound(w, "scene %q not found", from)
		return
	}
	if _, exists := s.sceneByName(to); exists && to != from {
		apiFail(w, http.StatusConflict, "exists", "a scene named "+to+" exists already")
		return
	}
	switch err := s.Engine.RenameScene(from, to); {
	case errors.Is(err, app.ErrNoScene):
		notFound(w, "scene %q not found", from)
		return
	case err != nil:
		badRequest(w, "%v", err)
		return
	}
	sc, _ := s.sceneByName(to)
	writeJSON(w, sc)
}

// --- settings (a whitelist) ---

// v1Settings are the settings a program may read and change. What is left out
// (the API and MCP switches, the sensor source, the areas, the disclaimer) is
// for the CaseHUD window only.
type v1Settings struct {
	Autostart          bool   `json:"autostart"`
	AutostartSupported bool   `json:"autostartSupported"` // read only
	Language           string `json:"language"`           // de or en
	UpdateCheck        bool   `json:"updateCheck"`
	Fahrenheit         bool   `json:"fahrenheit"`
	Clock12h           bool   `json:"clock12h"`
	Animation          bool   `json:"animation"`
	DeviceVideo        bool   `json:"deviceVideo"`
	ScreensaverOff     bool   `json:"screensaverOff"`
	// IntervalMS is how often the displays refresh: 0 (automatic), 500, 1000, 2000 or 5000.
	IntervalMS int `json:"intervalMs"`
}

// v1SettingsPatch is the body of PATCH /settings: only the fields present change.
type v1SettingsPatch struct {
	Autostart      *bool   `json:"autostart"`
	Language       *string `json:"language"`
	UpdateCheck    *bool   `json:"updateCheck"`
	Fahrenheit     *bool   `json:"fahrenheit"`
	Clock12h       *bool   `json:"clock12h"`
	Animation      *bool   `json:"animation"`
	DeviceVideo    *bool   `json:"deviceVideo"`
	ScreensaverOff *bool   `json:"screensaverOff"`
	IntervalMS     *int    `json:"intervalMs"`
}

func (s *Server) settingsV1() v1Settings {
	st := s.Engine.Settings()
	return v1Settings{Autostart: autostartEnabled(), AutostartSupported: autostartSupported(), Language: s.Engine.Language(),
		UpdateCheck: st.UpdateCheck, Fahrenheit: st.Fahrenheit, Clock12h: st.Clock12h, Animation: st.Animation,
		DeviceVideo: st.DeviceVideo == nil || *st.DeviceVideo, ScreensaverOff: st.ScreensaverOff, IntervalMS: st.IntervalMS}
}

func (s *Server) v1GetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.settingsV1())
}

func (s *Server) v1PatchSettings(w http.ResponseWriter, r *http.Request) {
	var p v1SettingsPatch
	if !decodeBody(w, r, &p) {
		return
	}
	if p.Language != nil && *p.Language != "de" && *p.Language != "en" {
		badRequest(w, "language must be de or en")
		return
	}
	if p.IntervalMS != nil && !slices.Contains([]int{0, 500, 1000, 2000, 5000}, *p.IntervalMS) {
		badRequest(w, "intervalMs must be 0, 500, 1000, 2000 or 5000")
		return
	}
	if p.Autostart != nil && !autostartSupported() {
		badRequest(w, "autostart is not supported on this system")
		return
	}
	// Everything is checked; the engine takes the settings in one step.
	patch := func(st *app.Settings) {
		if p.UpdateCheck != nil {
			st.UpdateCheck = *p.UpdateCheck
		}
		if p.Fahrenheit != nil {
			st.Fahrenheit = *p.Fahrenheit
		}
		if p.Clock12h != nil {
			st.Clock12h = *p.Clock12h
		}
		if p.Animation != nil {
			st.Animation = *p.Animation
		}
		if p.DeviceVideo != nil {
			st.DeviceVideo = p.DeviceVideo
		}
		if p.ScreensaverOff != nil {
			st.ScreensaverOff = *p.ScreensaverOff
		}
		if p.IntervalMS != nil {
			st.IntervalMS = *p.IntervalMS
		}
	}
	if p.UpdateCheck != nil || p.Fahrenheit != nil || p.Clock12h != nil || p.Animation != nil || p.DeviceVideo != nil ||
		p.ScreensaverOff != nil || p.IntervalMS != nil {
		if err := s.Engine.PatchSettings(patch); err != nil {
			badRequest(w, "%v", err)
			return
		}
		if p.UpdateCheck != nil && *p.UpdateCheck {
			go s.checkUpdate()
		}
	}
	if p.Language != nil {
		if err := s.Engine.SetLanguage(*p.Language); err != nil {
			badRequest(w, "%v", err)
			return
		}
	}
	if p.Autostart != nil {
		if err := autostartSet(*p.Autostart); err != nil {
			badRequest(w, "%v", err)
			return
		}
	}
	writeJSON(w, s.settingsV1())
}
