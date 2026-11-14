package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// The curated tools beyond the display ones: a one-call summary, the cooling,
// the lighting, scenes and displays, and a generic call of any /api/v1/ route
// for what no tool covers. Every one of them goes through the public API with
// the token the user granted; the rights are enforced by CaseHUD, not here.

// maxText cuts what a tool returns, so one answer cannot flood the model.
const maxText = 100_000

// strictArgs decodes the arguments of a tool call and refuses fields the tool
// does not know, so a misspelled field is an error instead of being ignored.
func strictArgs(args json.RawMessage, v any) error {
	if len(args) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalidParamsError{err}
	}
	return nil
}

// jsonText makes an answer of the API readable: indented JSON, "OK" for none.
func jsonText(b []byte) ToolResult {
	if len(bytes.TrimSpace(b)) == 0 {
		return Text("OK")
	}
	var out bytes.Buffer
	if json.Indent(&out, b, "", "  ") == nil {
		b = out.Bytes()
	}
	return Text(clip(string(b)))
}

func clip(s string) string {
	if len(s) <= maxText {
		return s
	}
	return s[:maxText] + "\n… (cut, " + strconv.Itoa(len(s)-maxText) + " more bytes; ask for less, e.g. a single item)"
}

// revisionHeader is the If-Match header for a cooling change ("" = none).
func revisionHeader(rev int64) map[string]string {
	if rev <= 0 {
		return nil
	}
	return map[string]string{"If-Match": strconv.FormatInt(rev, 10)}
}

func obj(props map[string]any, required ...string) map[string]any { return schema(props, required...) }

var (
	tString = func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	tBool   = func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	tInt    = func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	tObject = func(desc string) map[string]any { return map[string]any{"type": "object", "description": desc} }
	tEnum   = func(desc string, vals ...string) map[string]any {
		return map[string]any{"type": "string", "enum": vals, "description": desc}
	}
	revision = tInt("Optional. The revision from cooling_get_config. If the settings changed since, the change is refused (revision_mismatch) instead of overwriting the newer state.")
)

// curatedTools returns the tools described above.
func curatedTools(api *API) []Tool {
	get := func(path string) func(json.RawMessage) (ToolResult, error) {
		return func(json.RawMessage) (ToolResult, error) {
			rep, err := api.callFull("GET", path, nil, nil)
			if err != nil {
				return ToolResult{}, err
			}
			return jsonText(rep.Body), nil
		}
	}
	none := obj(map[string]any{})
	return []Tool{
		{
			Name: "status", Title: "CaseHUD status",
			Description: "One-call summary of CaseHUD: which areas (displays, lighting, cooling) are on, the connected displays and what they show, the cooling state and alarm, whether lighting is on, scene/rotation/automation, running conflicts and an available update. Call this first to see what exists before changing anything.",
			InputSchema: none,
			Handler:     func(json.RawMessage) (ToolResult, error) { return statusSummary(api) },
		},

		// --- cooling ---
		{
			Name: "cooling_status", Title: "Cooling status",
			Description: "The cooling as CaseHUD sees it: state (off, unconfigured, test, blocked, running), mode (preview = test, live), alarm, every output (fan or pump) with its speed in percent and rpm, its mode (curve, manual, none) and problems (problem code such as emergency or unguarded; notReleased, notMatched, noSource), the temperatures in use and the preset names. Read-only.",
			InputSchema: none, Handler: get("/api/v1/cooling"),
		},
		{
			Name: "cooling_get_config", Title: "Read cooling configuration",
			Description: "The stored cooling configuration (outputs with their limits, curves, sensor names) and its revision. Always read this before changing an output or a curve: it shows each output's kind (fan or pump), its curve and the ids of the curves and temperature sources you may use. Pass the revision to the change tools so a concurrent change is not overwritten.",
			InputSchema: none, Handler: get("/api/v1/cooling/config"),
		},
		{
			Name: "cooling_set_output", Title: "Set a fan or pump",
			Description: "Changes how ONE output is driven: either give it a curve (curve_id, an id from cooling_get_config) OR a fixed manual value (manual {value, unit}), never both. " +
				"Read cooling_get_config first. Rules enforced by CaseHUD: a PUMP is never allowed below 30 % (and never 0), so a pump with a lower value is refused; unit \"rpm\" only works for fans whose speed CaseHUD has measured, otherwise use \"percent\" (the default). " +
				"Only change an output when the user asked for it; a fan set too low can overheat the PC. Applies at once and is stored.",
			InputSchema: obj(map[string]any{
				"output_id": tString("Output id from cooling_get_config."),
				"curve_id":  tString("Id of an existing curve. Exclusive with manual."),
				"manual": obj(map[string]any{
					"value": map[string]any{"type": "number", "description": "Percent 0-100 (pump: 30-100), or rpm."},
					"unit":  tEnum("percent (default) or rpm (measured fans only).", "percent", "rpm"),
				}, "value"),
				"revision": revision,
			}, "output_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					OutputID string  `json:"output_id"`
					CurveID  *string `json:"curve_id"`
					Manual   *struct {
						Value *float64 `json:"value"`
						Unit  string   `json:"unit"`
					} `json:"manual"`
					Revision int64 `json:"revision"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.OutputID == "" {
					return ToolResult{}, errors.New("output_id is required (see cooling_get_config)")
				}
				if (p.CurveID == nil) == (p.Manual == nil) {
					return ToolResult{}, errors.New("give exactly one of curve_id or manual")
				}
				patch := map[string]any{}
				switch {
				case p.CurveID != nil:
					if *p.CurveID == "" {
						return ToolResult{}, errors.New("curve_id must name a curve; taking an output off its curve is not offered here")
					}
					patch["curve"] = *p.CurveID
				default:
					if p.Manual.Value == nil {
						return ToolResult{}, errors.New("manual.value is required")
					}
					unit := p.Manual.Unit
					if unit == "" {
						unit = "percent"
					}
					if unit != "percent" && unit != "rpm" {
						return ToolResult{}, errors.New("manual.unit must be percent or rpm")
					}
					if *p.Manual.Value <= 0 || (unit == "percent" && *p.Manual.Value > 100) {
						return ToolResult{}, errors.New("manual.value must be above 0 and, for percent, at most 100 (a pump needs at least 30)")
					}
					patch["manual"] = map[string]any{"value": *p.Manual.Value, "unit": unit}
				}
				rep, err := api.callFull("PATCH", "/api/v1/cooling/outputs/"+url.PathEscape(p.OutputID), patch, revisionHeader(p.Revision))
				if err != nil {
					return ToolResult{}, err
				}
				return Text("Done. " + string(bytes.TrimSpace(rep.Body)) + " Check cooling_status for the effect."), nil
			},
		},
		{
			Name: "cooling_set_curve", Title: "Create or replace a cooling curve",
			Description: "Creates or replaces a curve by id (the id inside the curve). A curve maps a temperature to a fan speed; outputs refer to it by id. " +
				"Fields: id (a-z, 0-9, -), name, kind (default \"drawn\": follows points; others: straight, gap, fixed, switch, thermostat, combined, follow), " +
				"sources (temperature sensor ids from cooling_status temperatures; the hottest counts), unit \"percent\" (default) or \"rpm\", " +
				"points: [[temperature in °C, value]] sorted by temperature, e.g. [[30,30],[50,50],[70,100]] (values in percent; pumps are never driven below 30 %). " +
				"Read cooling_get_config first and copy an existing curve as the starting point; unknown fields are refused. Replacing a curve changes every output that uses it. Only do this when the user asked.",
			InputSchema: obj(map[string]any{
				"curve":    tObject("The curve as in config.curves of cooling_get_config, with its id."),
				"revision": revision,
			}, "curve"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Curve    json.RawMessage `json:"curve"`
					Revision int64           `json:"revision"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				var head struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(p.Curve, &head) != nil || head.ID == "" {
					return ToolResult{}, errors.New("curve.id is required")
				}
				rep, err := api.callFull("PUT", "/api/v1/cooling/curves/"+url.PathEscape(head.ID), p.Curve, revisionHeader(p.Revision))
				if err != nil {
					return ToolResult{}, err
				}
				verb := "replaced"
				if rep.Code == 201 {
					verb = "created"
				}
				return Text("Curve " + head.ID + " " + verb + ". " + string(bytes.TrimSpace(rep.Body))), nil
			},
		},
		{
			Name: "cooling_delete_curve", Title: "Delete a cooling curve",
			Description: "Deletes a curve. A curve that an output or another curve uses is refused (curve_in_use): give those outputs another curve first. Only when the user asked.",
			InputSchema: obj(map[string]any{"curve_id": tString("Curve id."), "revision": revision}, "curve_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					CurveID  string `json:"curve_id"`
					Revision int64  `json:"revision"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.CurveID == "" {
					return ToolResult{}, errors.New("curve_id is required")
				}
				rep, err := api.callFull("DELETE", "/api/v1/cooling/curves/"+url.PathEscape(p.CurveID), nil, revisionHeader(p.Revision))
				if err != nil {
					return ToolResult{}, err
				}
				return Text("Deleted. " + string(bytes.TrimSpace(rep.Body))), nil
			},
		},
		{
			Name: "cooling_presets", Title: "List cooling presets",
			Description: "Lists the saved cooling presets (id, name) and which one is active. A preset is a complete set of curves and output settings.",
			InputSchema: none, Handler: get("/api/v1/cooling/presets"),
		},
		{
			Name: "cooling_preset_activate", Title: "Activate a cooling preset",
			Description: "Switches the cooling to a saved preset (completely or not at all). It never changes between test and live mode. Fans keep running. Only when the user asked; names and ids come from cooling_presets.",
			InputSchema: obj(map[string]any{"preset_id": tString("Preset id from cooling_presets.")}, "preset_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					PresetID string `json:"preset_id"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.PresetID == "" {
					return ToolResult{}, errors.New("preset_id is required")
				}
				rep, err := api.callFull("POST", "/api/v1/cooling/preset", map[string]any{"id": p.PresetID}, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},
		{
			Name: "cooling_preset_save", Title: "Save the cooling as a preset",
			Description: "Saves the cooling settings that run now as a new preset with this name (at most 60 characters, at most 32 presets). Does not change the running settings.",
			InputSchema: obj(map[string]any{"name": tString("Name of the preset.")}, "name"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Name string `json:"name"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if strings.TrimSpace(p.Name) == "" {
					return ToolResult{}, errors.New("name is required")
				}
				rep, err := api.callFull("POST", "/api/v1/cooling/presets", map[string]any{"name": p.Name}, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},
		{
			Name: "cooling_calibrate", Title: "Measure fan or pump speed",
			Description: "STARTS A SPEED MEASUREMENT (calibration). Ask the user first and only call with confirm=true after they agreed. " +
				"For fans (target one or all) CaseHUD drives each fan through its whole range and STOPS it for a while to find its stop and start points, so the airflow drops for several minutes: do not run it under load. " +
				"target pump measures the lowest stable duty of a pump and holds it at 30 % at the lowest, never off. " +
				"It needs CaseHUD to control the fans (live mode, no other fan program) and runs in the background: poll cooling_calibrate_status; stop with cooling_calibrate_stop. Cooling changes are refused while it runs.",
			InputSchema: obj(map[string]any{
				"target":    tEnum("one = one fan (needs output_id), all = every fan one after another, pump = a pump (needs output_id).", "one", "all", "pump"),
				"output_id": tString("Output id from cooling_get_config, for target one or pump."),
				"confirm":   tBool("Must be true. Set it only after the user explicitly agreed to the measurement."),
			}, "target", "confirm"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Target   string `json:"target"`
					OutputID string `json:"output_id"`
					Confirm  bool   `json:"confirm"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if !p.Confirm {
					return ToolResult{}, errors.New("confirm must be true. Explain to the user that the fans will stop for a while (a pump, if there is one, goes to 30 %), and call again only after they agreed")
				}
				var path string
				var body map[string]any
				switch p.Target {
				case "all":
					path, body = "/api/v1/cooling/measure", map[string]any{"all": true, "confirm": true}
				case "one", "pump":
					if p.OutputID == "" {
						return ToolResult{}, errors.New("output_id is required for target " + p.Target)
					}
					path, body = "/api/v1/cooling/measure", map[string]any{"id": p.OutputID, "confirm": true}
					if p.Target == "pump" {
						path = "/api/v1/cooling/measure-pump"
					}
				default:
					return ToolResult{}, errors.New("target must be one, all or pump")
				}
				rep, err := api.callFull("POST", path, body, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},
		{
			Name: "cooling_calibrate_status", Title: "Measurement status",
			Description: "The state of the speed measurement: running, step, results, error. Poll it while a measurement runs.",
			InputSchema: none, Handler: get("/api/v1/cooling/measure"),
		},
		{
			Name: "cooling_calibrate_stop", Title: "Stop the measurement",
			Description: "Ends a running measurement; the output goes back to its safe duty. By default the run is discarded; keep_finished stores the fans that were already finished.",
			InputSchema: obj(map[string]any{"keep_finished": tBool("Store the fans that finished before the stop (default: discard).")}),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					KeepFinished bool `json:"keep_finished"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				path := "/api/v1/cooling/measure"
				if p.KeepFinished {
					path += "?keep=finished"
				}
				if _, err := api.callFull("DELETE", path, nil, nil); err != nil {
					return ToolResult{}, err
				}
				return Text("Measurement ended."), nil
			},
		},

		// --- lighting ---
		{
			Name: "lighting_status", Title: "Lighting status",
			Description: "The lighting: on/off, brightness, the active effect and its parameters, the list of effects with their parameters, the devices and their zones, and the active preset. Read-only.",
			InputSchema: none, Handler: get("/api/v1/lighting"),
		},
		{
			Name: "lighting_set", Title: "Change the lighting",
			Description: "Changes the lighting; only the fields you give change. enabled switches CaseHUD's lighting control on or off, brightness is 0-100, effect is an effect id from lighting_status, params are the parameters of that effect (see lighting_status). " +
				"Per-device settings, the fan controller's own options and anything that writes to a device's flash are not available here: they are only possible in the CaseHUD window (the call is refused with gui_only). Only change the lighting when the user asked.",
			InputSchema: obj(map[string]any{
				"enabled":    tBool("Lighting control on or off."),
				"brightness": tInt("0-100."),
				"effect":     tString("Effect id from lighting_status."),
				"params":     tObject("Parameters of the effect, as listed for it in lighting_status."),
			}),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Enabled    *bool          `json:"enabled"`
					Brightness *int           `json:"brightness"`
					Effect     *string        `json:"effect"`
					Params     map[string]any `json:"params"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.Brightness != nil && (*p.Brightness < 0 || *p.Brightness > 100) {
					return ToolResult{}, errors.New("brightness must be 0-100")
				}
				patch := map[string]any{}
				if p.Enabled != nil {
					patch["enabled"] = *p.Enabled
				}
				if p.Brightness != nil {
					patch["brightness"] = *p.Brightness
				}
				if p.Effect != nil {
					patch["effect"] = *p.Effect
				}
				if p.Params != nil {
					patch["params"] = p.Params
				}
				if len(patch) == 0 {
					return ToolResult{}, errors.New("nothing to change: give enabled, brightness, effect or params")
				}
				rep, err := api.callFull("PATCH", "/api/v1/lighting", patch, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},
		{
			Name: "lighting_presets", Title: "List lighting presets",
			Description: "Lists the lighting presets (library and the user's own) with their ids and which one is active.",
			InputSchema: none, Handler: get("/api/v1/lighting/presets"),
		},
		{
			Name: "lighting_preset_apply", Title: "Apply a lighting preset",
			Description: "Applies a lighting preset (effect and parameters). Only when the user asked; ids come from lighting_presets. The answer says what could not be applied.",
			InputSchema: obj(map[string]any{"preset_id": tString("Preset id from lighting_presets.")}, "preset_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					PresetID string `json:"preset_id"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.PresetID == "" {
					return ToolResult{}, errors.New("preset_id is required")
				}
				rep, err := api.callFull("POST", "/api/v1/lighting/presets/"+url.PathEscape(p.PresetID)+"/apply", nil, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},

		// --- scenes and displays ---
		{
			Name: "list_scenes", Title: "List scenes",
			Description: "Lists the saved scenes (a scene is a stored setup of all displays) and which one is on screen.",
			InputSchema: none, Handler: get("/api/v1/scenes"),
		},
		{
			Name: "apply_scene", Title: "Show a scene",
			Description: "Shows a saved scene on the displays. By default it is only shown for now; persist=true stores it as the displays' configuration. Only when the user asked.",
			InputSchema: obj(map[string]any{
				"name":    tString("Scene name from list_scenes."),
				"persist": tBool("Store the scene as the displays' configuration (default false)."),
			}, "name"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Name    string `json:"name"`
					Persist bool   `json:"persist"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.Name == "" {
					return ToolResult{}, errors.New("name is required (see list_scenes)")
				}
				if _, err := api.callFull("POST", "/api/v1/scenes/"+url.PathEscape(p.Name)+"/activate", map[string]any{"persist": p.Persist}, nil); err != nil {
					return ToolResult{}, err
				}
				return Text("Scene " + p.Name + " is shown."), nil
			},
		},
		{
			Name: "set_display", Title: "Change a display",
			Description: "Changes one display; only the fields you give change. design is a design id (list_designs), mode is hud (shows the design), media or off, theme dark or light, brightness 0-100, rotation 0, 90, 180 or 270, layout a layout id for bar displays (list them with api_call GET /api/v1/layouts). " +
				"A single design replaces a grid. Only when the user asked. Returns the display after the change.",
			InputSchema: obj(map[string]any{
				"display_id": tString("Display id from list_displays or status."),
				"design":     tString("Design id."),
				"mode":       tEnum("hud, media or off.", "hud", "media", "off"),
				"theme":      tEnum("dark or light.", "dark", "light"),
				"brightness": tInt("0-100."),
				"rotation":   tInt("0, 90, 180 or 270."),
				"layout":     tString("Layout id (bar displays)."),
			}, "display_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					DisplayID  string  `json:"display_id"`
					Design     *string `json:"design"`
					Mode       *string `json:"mode"`
					Theme      *string `json:"theme"`
					Brightness *int    `json:"brightness"`
					Rotation   *int    `json:"rotation"`
					Layout     *string `json:"layout"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if p.DisplayID == "" {
					return ToolResult{}, errors.New("display_id is required (see list_displays)")
				}
				patch := map[string]any{}
				if p.Design != nil {
					patch["design"] = *p.Design
				}
				if p.Mode != nil {
					patch["mode"] = *p.Mode
				}
				if p.Theme != nil {
					patch["theme"] = *p.Theme
				}
				if p.Brightness != nil {
					patch["brightness"] = *p.Brightness
				}
				if p.Rotation != nil {
					patch["rotation"] = *p.Rotation
				}
				if p.Layout != nil {
					patch["layout"] = *p.Layout
				}
				if len(patch) == 0 {
					return ToolResult{}, errors.New("nothing to change")
				}
				rep, err := api.callFull("PATCH", "/api/v1/panels/"+url.PathEscape(p.DisplayID), patch, nil)
				if err != nil {
					if strings.Contains(err.Error(), "not_found") {
						return ToolResult{}, fmt.Errorf("no display with id %q, see list_displays", p.DisplayID)
					}
					return ToolResult{}, err
				}
				return jsonText(rep.Body), nil
			},
		},

		// --- the rest of the API ---
		{
			Name: "api_spec", Title: "CaseHUD API reference",
			Description: "The reference of CaseHUD's HTTP API that api_call uses. Without arguments: a compact list of every route (method, path, needed right, one-line summary). With path (and optionally method): the full description of that route with its parameters, body and answers. Use it to find a route and to learn the exact body before calling api_call.",
			InputSchema: obj(map[string]any{
				"path":   tString("A path from the list, e.g. /api/v1/scenes/{id}/activate (the /api/v1 prefix may be left out)."),
				"method": tEnum("Only this method of the path.", "GET", "POST", "PUT", "PATCH", "DELETE"),
			}),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Path   string `json:"path"`
					Method string `json:"method"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				var doc map[string]any
				if _, err := api.call("GET", "/api/v1/openapi.json", nil, &doc); err != nil {
					return ToolResult{}, err
				}
				return Text(clip(specView(doc, p.Path, p.Method))), nil
			},
		},
		{
			Name: "api_call", Title: "Call any CaseHUD API route",
			Description: "Calls any route of CaseHUD's HTTP API (/api/v1/...) with this connection's rights, for everything the other tools do not cover (rotation, automation, settings, layouts, designs, scenes, AWTRIX clocks, power, ...). " +
				"First look the route up with api_spec (route list, then the details of the one you need) and send exactly the body it describes. " +
				"method is GET, POST, PUT, PATCH or DELETE; path must start with /api/v1/ and may carry a simple query string (?size=200). " +
				"Reading needs no special care. Everything else changes the user's setup: only do it when asked, and prefer the dedicated tool. " +
				"Not available through MCP: quitting, updating, opening the window, pairing and event streams. Switches for the API/MCP interfaces, the areas, the sensor source and similar are only in the CaseHUD window. " +
				"A 403 insufficient_scope means this connection lacks the right: tell the user to grant it in CaseHUD > Settings > Access. Images are returned as images.",
			InputSchema: obj(map[string]any{
				"method": tEnum("HTTP method.", "GET", "POST", "PUT", "PATCH", "DELETE"),
				"path":   tString("Path starting with /api/v1/, e.g. /api/v1/rotation."),
				"body":   map[string]any{"description": "JSON body for POST, PUT and PATCH, as the route describes it."},
			}, "method", "path"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Method string          `json:"method"`
					Path   string          `json:"path"`
					Body   json.RawMessage `json:"body"`
				}
				if err := strictArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				method, path, err := CheckAPICall(p.Method, p.Path)
				if err != nil {
					return ToolResult{}, err
				}
				var body any
				if len(p.Body) > 0 && string(p.Body) != "null" {
					if method == "GET" || method == "DELETE" {
						return ToolResult{}, errors.New(method + " takes no body")
					}
					if len(p.Body) > 1<<20 {
						return ToolResult{}, errors.New("body is larger than 1 MiB")
					}
					body = p.Body
				} else if method == "POST" || method == "PUT" || method == "PATCH" {
					body = json.RawMessage("{}")
				}
				rep, err := api.callFull(method, path, body, nil)
				if err != nil {
					return ToolResult{}, err
				}
				if ct := rep.Header.Get("Content-Type"); strings.HasPrefix(ct, "image/") {
					return ToolResult{Content: []Content{{Type: "image", Data: base64.StdEncoding.EncodeToString(rep.Body), MimeType: ct}}}, nil
				}
				if len(bytes.TrimSpace(rep.Body)) == 0 {
					return Text("OK (HTTP " + strconv.Itoa(rep.Code) + ")"), nil
				}
				return jsonText(rep.Body), nil
			},
		},
	}
}

// --- api_call: what may be called ---

var (
	queryKeyRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`)
	allowedVerb = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
)

// blockedAPI lists what api_call never reaches, whatever rights the token has:
// ending or updating CaseHUD and opening its window (the "system" right), the
// pairing, revoking the token, and the endless event stream.
var blockedAPI = map[string]string{
	"/api/v1/quit":           "quitting CaseHUD is not available through MCP",
	"/api/v1/update/install": "updating CaseHUD is not available through MCP",
	"/api/v1/update/check":   "updating CaseHUD is not available through MCP",
	"/api/v1/gui/open":       "opening the CaseHUD window is not available through MCP",
	"/api/v1/events":         "the event stream cannot be read through a tool call; poll status instead",
}

// CheckAPICall validates what api_call was asked to do and returns the cleaned
// method and the request path (with a normalised query). Only a plain path
// under /api/v1/ on the CaseHUD server passes: no scheme or host, no dot
// segments (also not encoded), no backslash, no control characters, no
// fragment, and a query of simple keys with plain values.
func CheckAPICall(method, rawPath string) (string, string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !allowedVerb[method] {
		return "", "", errors.New("method must be GET, POST, PUT, PATCH or DELETE")
	}
	if rawPath == "" || len(rawPath) > 2000 {
		return "", "", errors.New("path is required (at most 2000 characters)")
	}
	for _, r := range rawPath {
		if unicode.IsControl(r) || r == '\\' || r == '#' || r == ' ' {
			return "", "", errors.New("path contains a character that is not allowed")
		}
	}
	if !strings.HasPrefix(rawPath, "/api/v1/") {
		return "", "", errors.New("path must start with /api/v1/ (a relative path on CaseHUD, no host, no other path)")
	}
	pathPart, query, hasQuery := strings.Cut(rawPath, "?")
	u, err := url.Parse(pathPart)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", errors.New("path is not a plain path")
	}
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		return "", "", errors.New("path has an invalid % escape")
	}
	if !strings.HasPrefix(decoded, "/api/v1/") || strings.Contains(decoded, "\\") {
		return "", "", errors.New("path must stay under /api/v1/")
	}
	for _, seg := range strings.Split(decoded, "/")[1:] {
		if seg == ".." || seg == "." {
			return "", "", errors.New("path must not contain . or .. segments")
		}
	}
	for _, r := range decoded {
		if unicode.IsControl(r) {
			return "", "", errors.New("path contains a character that is not allowed")
		}
	}
	// Only the path counts for the block list, and only the path of the route
	// itself: a trailing slash is another (unknown) route, not a way around.
	clean := strings.TrimRight(decoded, "/")
	if why, blocked := blockedAPI[clean]; blocked {
		return "", "", errors.New(why)
	}
	if clean == "/api/v1/token" && method != "GET" {
		return "", "", errors.New("the token of this connection is not managed through MCP (GET shows what it may do)")
	}
	if clean == "/api/v1/pair" || strings.HasPrefix(clean, "/api/v1/pair/") {
		return "", "", errors.New("pairing is done by the MCP server itself, not through a tool call")
	}
	out := pathPart
	if hasQuery {
		vals, err := url.ParseQuery(query)
		if err != nil {
			return "", "", errors.New("query string is not valid")
		}
		keys := make([]string, 0, len(vals))
		for k, vs := range vals {
			if !queryKeyRe.MatchString(k) || len(vs) > 4 {
				return "", "", errors.New("query keys must be plain names")
			}
			for _, v := range vs {
				if len(v) > 200 {
					return "", "", errors.New("a query value is too long")
				}
				for _, r := range v {
					if unicode.IsControl(r) {
						return "", "", errors.New("a query value contains a character that is not allowed")
					}
				}
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			out += "?" + vals.Encode()
		}
	}
	return method, out, nil
}

// --- api_spec ---

// specView renders the OpenAPI document for the model: a compact list, or the
// operations of one path with the schemas written out.
func specView(doc map[string]any, path, method string) string {
	paths, _ := doc["paths"].(map[string]any)
	if path == "" {
		var b strings.Builder
		info, _ := doc["info"].(map[string]any)
		fmt.Fprintf(&b, "CaseHUD API %v. Call routes with api_call; rights: read, control, system (system routes are not available through MCP).\n", info["version"])
		b.WriteString("Ask for the details of a route with api_spec {path, method}.\n\n")
		keys := make([]string, 0, len(paths))
		for p := range paths {
			keys = append(keys, p)
		}
		sort.Strings(keys)
		for _, p := range keys {
			item, _ := paths[p].(map[string]any)
			for _, m := range []string{"get", "put", "post", "patch", "delete"} {
				op, ok := item[m].(map[string]any)
				if !ok {
					continue
				}
				fmt.Fprintf(&b, "%-6s %s  [%v]  %v\n", strings.ToUpper(m), p, op["x-scope"], op["summary"])
			}
		}
		return b.String()
	}
	if !strings.HasPrefix(path, "/api/v1") {
		path = "/api/v1/" + strings.TrimPrefix(path, "/")
	}
	item, ok := paths[path].(map[string]any)
	if !ok {
		return "No such path. Call api_spec without arguments for the list of routes."
	}
	comps, _ := doc["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)
	out := map[string]any{}
	for m, op := range item {
		if method != "" && !strings.EqualFold(m, method) {
			continue
		}
		out[strings.ToUpper(m)] = inlineRefs(op, schemas, 0)
	}
	if len(out) == 0 {
		return "That path has no " + method + " route. Call api_spec with the path only to see its methods."
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return path + "\n" + string(b)
}

// inlineRefs replaces {"$ref": "#/components/schemas/X"} by the schema, to a
// limited depth so recursive types end.
func inlineRefs(v any, schemas map[string]any, depth int) any {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if depth >= 6 {
				return map[string]any{"$ref": name, "note": "see api_spec for the type or the route that returns it"}
			}
			if s, ok := schemas[name]; ok {
				return inlineRefs(s, schemas, depth+1)
			}
			return x
		}
		out := make(map[string]any, len(x))
		for k, val := range x {
			if k == "security" || k == "operationId" || k == "tags" {
				continue
			}
			out[k] = inlineRefs(val, schemas, depth)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = inlineRefs(val, schemas, depth)
		}
		return out
	}
	return v
}

// --- status ---

// statusSummary builds the one-call summary from /status and /cooling.
func statusSummary(api *API) (ToolResult, error) {
	var st map[string]any
	if _, err := api.call("GET", "/api/v1/status", nil, &st); err != nil {
		return ToolResult{}, err
	}
	pick := func(m map[string]any, keys ...string) map[string]any {
		out := map[string]any{}
		for _, k := range keys {
			if v, ok := m[k]; ok {
				out[k] = v
			}
		}
		return out
	}
	sub := func(k string) map[string]any { m, _ := st[k].(map[string]any); return m }
	out := map[string]any{}
	for _, k := range []string{"version", "areas", "power", "scene", "disclaimerAccepted", "conflicts", "updateAvailable"} {
		if v, ok := st[k]; ok {
			out[k] = v
		}
	}
	out["gaming"] = pick(sub("gaming"), "active", "game")
	out["rotation"] = pick(sub("rotation"), "enabled", "scene")
	out["automation"] = pick(sub("automation"), "enabled", "scene")
	out["awtrix"] = pick(sub("awtrix"), "enabled")
	out["lighting"] = pick(sub("lighting"), "enabled", "active", "preset")
	var displays []any
	if ps, ok := st["panels"].([]any); ok {
		for _, p := range ps {
			if m, ok := p.(map[string]any); ok {
				displays = append(displays, pick(m, "id", "name", "model", "mode", "design", "theme", "brightness", "layout", "error"))
			}
		}
	}
	if displays == nil {
		displays = []any{}
	}
	out["displays"] = displays
	areas := sub("areas")
	if on, _ := areas["cooling"].(bool); on {
		var c map[string]any
		if _, err := api.call("GET", "/api/v1/cooling", nil, &c); err != nil {
			out["cooling"] = map[string]any{"error": err.Error()}
		} else {
			cool := pick(c, "state", "mode", "blockedBy", "preset", "alarm", "revision")
			var outputs []any
			if os, ok := c["outputs"].([]any); ok {
				for _, o := range os {
					if m, ok := o.(map[string]any); ok {
						outputs = append(outputs, pick(m, "id", "name", "kind", "percent", "rpm", "mode", "problem", "notReleased", "notMatched"))
					}
				}
			}
			cool["outputs"] = outputs
			out["cooling"] = cool
		}
	} else {
		out["cooling"] = map[string]any{"state": "off", "note": "the cooling area is switched off in the CaseHUD settings"}
	}
	return Text(pretty(out)), nil
}
