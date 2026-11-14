package web

import (
	"net/http"
	"strings"

	"github.com/kewe87/casehud/internal/access"
	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/lcd"
)

// v1Spec lists every operation of /api/v1/ for the OpenAPI description. Add the
// description of a new route here: the test fails until it is there.
func v1Spec() []specOp {
	const (
		R, C, S, P = scopeRead, scopeControl, scopeSystem, scopePublic
	)
	etag := map[string]string{"ETag": "the new revision"}
	ifMatchHint := "Optional: the revision from GET /cooling/config. A change made on an older revision is answered with 409 revision_mismatch."
	ops := []specOp{
		// --- pairing and the token ---
		op("POST", "/api/v1/pair", P, "Access", "Ask for access",
			opDesc("Needs no token, only Content-Type: application/json and no Origin header (no web pages). The user answers in the CaseHUD window: read only, control, optionally also system (quit and update CaseHUD; never implied by control, off unless the user ticks it), or refuse, remembered or until CaseHUD quits. Poll GET /pair/{id} for the answer. At most 3 requests wait at once, one new request per 10 s from the same source (interface and name; 1 s between any two requests), a request expires after 2 minutes. interface mcp is for the MCP server and needs the MCP switch on; api (default) needs the API switch on. A name that starts with \"MCP (\" is reserved for the MCP interface and the AI connection of the window: with interface api it is refused with 400. The user can grant system only together with control."),
			opBody(v1PairRequest{}, "name (at most 40 characters) and the scopes wanted: read, control, system"),
			opOK(202, "the request waits for the user", v1PairAccepted{}),
			opErrs(400, 403, 409, 415, 429)),
		op("GET", "/api/v1/pair/{id}", P, "Access", "Collect the answer to an access request",
			opDesc("pending until the user decided. approved carries the token exactly once; later calls answer 404. denied is reported once."),
			opOK(200, "the state of the request", access.Result{}), opErrs(404)),
		op("GET", "/api/v1/token", R, "Access", "What the token in use may do",
			opDesc("scope is read or control; system tells whether the extra right to quit and update CaseHUD was granted (it only exists together with control). interface is api or mcp."),
			opOK(200, "the caller", v1TokenInfo{})),
		op("DELETE", "/api/v1/token", R, "Access", "Give the access up",
			opDesc("Revokes the token of a paired program. The token of api.json and the window's token cannot be revoked this way. Not available to a token of the MCP interface (403 not_for_mcp): such a connection is managed in the window. A revoked token is answered with 403 revoked afterwards, so a program can tell it from a token CaseHUD never knew."),
			opNone(204, "revoked"), opErrs(400, 403)),
		op("GET", "/api/v1/openapi.json", R, "Access", "This description",
			opDesc("OpenAPI 3.1. It needs a token with at least the read scope."),
			opOK(200, "the OpenAPI document", map[string]any{})),

		// --- status ---
		op("GET", "/api/v1/status", R, "Status", "State of CaseHUD", opOK(200, "the state", v1Status{})),
		op("GET", "/api/v1/events", R, "Status", "Server-sent events",
			opDesc("A stream of status, cooling and update events (text/event-stream). At most 8 streams, at most 3 per token. It ends when the API is switched off, the token is renewed or the program is revoked. Not available to a token of the MCP interface (403 not_for_mcp)."),
			specOpt(func(o *specOp) {
				o.Responses = append(o.Responses, specResp{Status: 200, Desc: "the event stream", Media: "text/event-stream", Raw: `{"type":"string"}`})
			}), opErrs(403, 503)),

		// --- displays ---
		op("GET", "/api/v1/panels", R, "Displays", "List the connected displays", opOK(200, "the displays", []v1Panel{})),
		op("PATCH", "/api/v1/panels/{id}", C, "Displays", "Change a display",
			opDesc("Only the fields present change. A single design replaces a grid."),
			opBody(v1PanelPatch{}, "the fields to change"), opOK(200, "the display after the change", v1Panel{}), opErrs(400, 404)),
		op("GET", "/api/v1/panels/{id}/preview.png", R, "Displays", "The last frame of a display", opPNG("a PNG"),
			opQuery("t", "the token, for use in an img tag", "string", false), opErrs(404)),
		op("POST", "/api/v1/panels/{id}/identify", C, "Displays", "Show the number on a display", opNone(204, "done"), opErrs(404)),
		op("POST", "/api/v1/identify", C, "Displays", "Show the numbers on all displays", opNone(204, "done"), opErrs(404)),
		op("GET", "/api/v1/sensors", R, "Designs", "Live sensor readings",
			opDesc("The path (or a suffix of it) is a widget source in a design."), opOK(200, "the readings", []v1Sensor{})),
		op("GET", "/api/v1/sets", R, "Displays", "Design sets", opOK(200, "the sets", []v1Set{})),
		op("GET", "/api/v1/layouts", R, "Displays", "Layouts for wide displays", opOK(200, "the layouts", []v1Layout{})),
		op("GET", "/api/v1/layouts/{id}", R, "Displays", "A layout", opOK(200, "the layout", v1Layout{}), opErrs(404)),
		op("PUT", "/api/v1/layouts/{id}", C, "Displays", "Save a layout of your own",
			opDesc("Creates or replaces a layout; the displays and scenes that use it follow at once. The id is a-z, 0-9 and -. The ready-made layouts cannot be replaced (409 builtin). Refused with 409 area_off while the display area is off."),
			opBody(v1LayoutBody{}, "name, grid and tiles"), opNone(204, "saved"), opErrs(400, 409)),
		op("DELETE", "/api/v1/layouts/{id}", C, "Displays", "Delete a layout of your own",
			opDesc("Displays and scenes that used it keep what it showed as their own composition. Ready-made layouts: 409 builtin."),
			opNone(204, "deleted"), opErrs(404, 409)),
		op("POST", "/api/v1/sets/apply", C, "Displays", "Put a design set on displays",
			opBody(v1ApplySetRequest{}, "the set, theme and displays (all if empty)"), opOK(200, "the displays it was applied to", v1Applied{}), opErrs(400, 404)),
		op("GET", "/api/v1/scenes", R, "Displays", "Scenes", opOK(200, "the scenes", []v1Scene{})),
		op("POST", "/api/v1/scenes/{id}/activate", C, "Displays", "Activate a scene",
			opDesc("By default the scene is only shown for now; persist (body or ?persist=true) stores it as the displays' configuration."),
			opOptBody(v1ActivateRequest{}, "persist"), opQuery("persist", "same as persist in the body", "boolean", false),
			opNone(204, "done"), opErrs(400, 404)),
		op("PATCH", "/api/v1/rotation", C, "Displays", "Switch the scene rotation on or off",
			opBody(v1EnabledRequest{}, "enabled"), opOK(200, "the state", v1Enabled{}), opErrs(400)),
		op("POST", "/api/v1/rotation/next", C, "Displays", "Next scene of the rotation", opNone(204, "done")),
		op("PATCH", "/api/v1/automation", C, "Displays", "Switch the automation on or off",
			opBody(v1EnabledRequest{}, "enabled"), opOK(200, "the state", v1Enabled{}), opErrs(400)),
		op("POST", "/api/v1/power", C, "Displays", "Displays on or off",
			opBody(v1PowerRequest{}, "on"), opOK(200, "the state", v1PowerState{}), opErrs(400)),
		op("POST", "/api/v1/gaming", C, "Displays", "Tell CaseHUD that a game runs",
			opDesc("A lease: the hint ends after leaseSeconds (10 to 3600, default 60) unless it is renewed."),
			opBody(v1GamingRequest{}, "active, game, pid, leaseSeconds"), opOK(200, "the gaming state", v1GamingState{}), opErrs(400)),
		op("PATCH", "/api/v1/awtrix", C, "Displays", "Switch the AWTRIX clocks on or off",
			opBody(v1EnabledRequest{}, "enabled"), opOK(200, "the state", v1Enabled{}), opErrs(400)),
		op("GET", "/api/v1/rotation", R, "Displays", "The scene rotation",
			opDesc("Settings and what the rotation does now. The scenes are listed by GET /scenes; switch it with PATCH /rotation."),
			opOK(200, "the rotation", v1Rotation{})),
		op("GET", "/api/v1/automation", R, "Displays", "The automation",
			opDesc("The rules (when a game runs, show a scene) and what the automation does now. Switch it with PATCH /automation."),
			opOK(200, "the automation", v1Automation{})),
		op("GET", "/api/v1/awtrix", R, "Displays", "The AWTRIX clocks",
			opDesc("The configured clocks, whether they answer, and the apps CaseHUD can show on them. Switch them with PATCH /awtrix."),
			opOK(200, "the clocks", v1Awtrix{})),
		op("POST", "/api/v1/scenes", C, "Displays", "Save what the displays show as a scene",
			opDesc("Stores the current configuration of the connected displays under the name (at most 64 characters). A scene of that name is only replaced with overwrite: true, otherwise 409 exists. At most 50 scenes."),
			opBody(v1SceneSave{}, "name, overwrite, noRotation"), opOK(201, "the scene", v1Scene{}), opErrs(400, 409)),
		op("PATCH", "/api/v1/scenes/{id}", C, "Displays", "Rename a scene",
			opDesc("The id of a scene is its name. Automation rules that point to it follow."),
			opBody(v1SceneRename{}, "the new name"), opOK(200, "the scene", v1Scene{}), opErrs(400, 404, 409)),

		// --- designs ---
		op("GET", "/api/v1/designs", R, "Designs", "List designs", opOK(200, "the designs", []v1Design{})),
		op("GET", "/api/v1/designs/{id}", R, "Designs", "A design as layout JSON", opOK(200, "the layout", lcd.Layout{}), opErrs(404)),
		op("PUT", "/api/v1/designs/{id}", C, "Designs", "Save a layout as a user design",
			opDesc("The id is a-z, 0-9 and -. An id of a built-in design overrides it."),
			opBody(lcd.Layout{}, "the layout"), opNone(204, "saved"), opErrs(400)),
		op("DELETE", "/api/v1/designs/{id}", C, "Designs", "Delete a design of the user",
			opDesc("Designs that come with CaseHUD cannot be deleted (409 builtin). A design that a display, scene, layout or the rotation uses is refused with 409 in_use. Deleting a copy saved over a built-in design brings the original back."),
			opNone(204, "deleted"), opErrs(404, 409)),
		op("GET", "/api/v1/designs/{id}/preview.png", R, "Designs", "Preview of a design", opPNG("a PNG"),
			opQuery("size", "width in pixels, 32 to 480", "integer", false), opQuery("theme", "dark or light", "string", false, "dark", "light"),
			opQuery("t", "the token, for use in an img tag", "string", false), opErrs(400, 404)),
		op("POST", "/api/v1/render", R, "Designs", "Render a layout with live values",
			opDesc("Nothing is stored. The scope is read although the method is POST. w is limited to 2048 pixels; a background from the media library (bg=media:...) is refused with 400; at most 2 renders run at once, further ones get 503 busy with Retry-After."),
			opBody(lcd.Layout{}, "the layout"), opPNG("a PNG"),
			opQuery("w", "width in pixels, up to 2048", "integer", false), opQuery("theme", "dark or light", "string", false, "dark", "light"),
			opErrs(400, 503)),
		op("GET", "/api/v1/community", R, "Designs", "The community gallery", opOK(200, "the entries", []v1CommunityEntry{}), opErrs(502)),
		op("GET", "/api/v1/community/{id}/preview.png", R, "Designs", "Preview of a gallery design", opPNG("a PNG"),
			opQuery("size", "width in pixels", "integer", false), opQuery("theme", "dark or light", "string", false, "dark", "light"),
			opQuery("t", "the token, for use in an img tag", "string", false), opErrs(404, 502)),
		op("POST", "/api/v1/community/{id}/import", C, "Designs", "Import a gallery design", opOK(200, "the id of the new design", v1ID{}), opErrs(404, 502)),

		// --- lighting ---
		op("GET", "/api/v1/lighting", R, "Lighting", "The lighting", opOK(200, "the lighting", app.LightingView{})),
		op("PATCH", "/api/v1/lighting", C, "Lighting", "Change the lighting",
			opDesc("tl, devices.*.settings, devices.*.ignoreConflicts and off for the fan controller are for the CaseHUD window only and are answered with 403 gui_only."),
			opBody(app.LightingPatch{}, "the fields to change"), opOK(200, "the lighting after the change", app.LightingView{}), opErrs(400, 409)),
		op("GET", "/api/v1/lighting/presets", R, "Lighting", "Lighting presets", opOK(200, "the presets", []app.LightingPresetView{})),
		op("POST", "/api/v1/lighting/presets", C, "Lighting", "Save the lighting as a preset",
			opBody(v1NameRequest{}, "name"), opOK(200, "the preset", app.LightingPresetView{}), opErrs(400)),
		op("POST", "/api/v1/lighting/presets/{id}/apply", C, "Lighting", "Apply a preset",
			opOK(200, "the lighting, and what could not be applied", map[string]any{}), opErrs(404, 409)),
		op("DELETE", "/api/v1/lighting/presets/{id}", C, "Lighting", "Delete a preset of the user", opNone(204, "deleted"), opErrs(400, 404)),

		// --- cooling ---
		op("GET", "/api/v1/cooling", R, "Cooling", "The cooling as CaseHUD sees it",
			opDesc("state, mode, revision, alarm and the outputs. Outputs say mode (curve, manual, none), manual, unregulated, calibrating, unproven, notReleased, notMatched, noSource and problem, so a client can tell the user what to do. Releasing a pump stays in the window."),
			opOK(200, "the cooling", app.CoolingAPI{})),
		op("POST", "/api/v1/cooling/preset", C, "Cooling", "Activate a cooling preset",
			opBody(v1IDRequest{}, "the id of the preset"), opOK(200, "the cooling after the change", app.CoolingAPI{}), opErrs(400, 404, 409)),
		op("GET", "/api/v1/cooling/config", R, "Cooling", "The stored cooling configuration with its revision",
			opOKHdr(200, "the configuration", app.CoolingConfigDoc{}, etag)),
		op("PUT", "/api/v1/cooling/config", C, "Cooling", "Replace the cooling configuration",
			opDesc("The whole configuration as in the config field of GET /cooling/config. The revision of that answer is required in If-Match (428 without, 409 revision_mismatch when it is old). It goes through the checks of a saved configuration; a pump never goes below 30 %. Refused with 409 busy while a measurement runs."),
			opHeader("If-Match", "the revision from GET /cooling/config", true),
			specOpt(func(o *specOp) { o.Body, o.BodyDesc = app.CoolingConfig{}, "the configuration" }),
			opOKHdr(200, "stored", v1Revision{}, etag), opErrs(400, 409, 428)),
		op("PATCH", "/api/v1/cooling/outputs/{id}", C, "Cooling", "Change one output",
			opDesc("A curve or a manual value, not both; curve \"\" takes the output off its curve. A pump below 30 % (minimum or manual value) is refused with 400 invalid_value. "+ifMatchHint),
			opHeader("If-Match", "the revision from GET /cooling/config", false),
			opBody(app.CoolingOutputPatch{}, "the fields to change"), opOKHdr(200, "stored", v1Revision{}, etag), opErrs(400, 404, 409)),
		op("PUT", "/api/v1/cooling/curves/{id}", C, "Cooling", "Create or replace a curve",
			opDesc("201 for a new curve, 200 for a replaced one. "+ifMatchHint),
			opHeader("If-Match", "the revision from GET /cooling/config", false),
			opBody(app.CoolingCurve{}, "the curve; id may be left out"), opOKHdr(200, "replaced", v1Revision{}, etag),
			opOKHdr(201, "created", v1Revision{}, etag), opErrs(400, 409)),
		op("DELETE", "/api/v1/cooling/curves/{id}", C, "Cooling", "Delete a curve",
			opDesc("A curve an output or another curve uses is refused with 409 curve_in_use. "+ifMatchHint),
			opHeader("If-Match", "the revision from GET /cooling/config", false),
			opOKHdr(200, "deleted", v1Revision{}, etag), opErrs(404, 409)),
		op("GET", "/api/v1/cooling/presets", R, "Cooling", "Cooling presets", opOK(200, "the presets", app.CoolingPresetsView{})),
		op("POST", "/api/v1/cooling/presets", C, "Cooling", "Save the running settings as a preset",
			opBody(v1NameRequest{}, "name"), opOK(201, "the preset", app.CoolingPresetInfo{}), opErrs(400, 409)),
		op("PUT", "/api/v1/cooling/presets/{id}", C, "Cooling", "Rename a preset",
			opBody(v1NameRequest{}, "name"), opNone(204, "renamed"), opErrs(400, 404, 409)),
		op("POST", "/api/v1/cooling/presets/{id}/overwrite", C, "Cooling", "Overwrite a preset with the running settings",
			opNone(204, "done"), opErrs(404, 409)),
		op("DELETE", "/api/v1/cooling/presets/{id}", C, "Cooling", "Delete a preset", opNone(204, "deleted"), opErrs(404, 409)),
		op("PUT", "/api/v1/cooling/mode", C, "Cooling", "Switch between test and live mode",
			opDesc("Live is refused with 409 blocked while the disclaimer is not accepted or another fan program runs."),
			opBody(v1ModeRequest{}, "preview or live"), opOKHdr(200, "switched", v1ModeChanged{}, etag), opErrs(400, 409)),
		op("GET", "/api/v1/cooling/measure", R, "Cooling", "State of the speed measurement", opOK(200, "the measurement", app.CoolingMeasureView{})),
		op("POST", "/api/v1/cooling/measure", C, "Cooling", "Measure the speed of one fan or all fans",
			opDesc("Drives the fans through their range. id or all, and confirm: true are required."),
			opBody(v1MeasureRequest{}, "id or all, and confirm"), opOK(202, "started", app.CoolingMeasureView{}), opErrs(400, 409)),
		op("POST", "/api/v1/cooling/measure-pump", C, "Cooling", "Measure the lowest stable duty of the pump",
			opDesc("confirm: true is required. The pump never goes below 30 %."),
			opBody(v1MeasurePumpRequest{}, "id and confirm"), opOK(202, "started", app.CoolingMeasureView{}), opErrs(400, 409)),
		op("DELETE", "/api/v1/cooling/measure", C, "Cooling", "End a running measurement",
			opQuery("keep", "finished stores the fans of a run over all fans that were already finished", "string", false, "finished"),
			opNone(204, "ended"), opErrs(400)),
		op("POST", "/api/v1/cooling/recheck-pump", C, "Cooling", "Check a pump again",
			opDesc("For an output with notMatched. Releasing a pump (administrator rights) is not offered here."),
			opBody(v1IDRequest{}, "id of the pump"), opNone(204, "started"), opErrs(400, 409)),

		// --- settings ---
		op("GET", "/api/v1/settings", R, "Settings", "The settings a program may change",
			opDesc("Only a whitelist: autostart, language, updateCheck, fahrenheit, clock12h, animation, deviceVideo, screensaverOff and intervalMs. Everything else (the API and MCP switches, the sensor source, the areas, the disclaimer) is for the CaseHUD window only."),
			opOK(200, "the settings", v1Settings{})),
		op("PATCH", "/api/v1/settings", C, "Settings", "Change settings",
			opDesc("Only the fields present change; any other field is refused with 400. language is de or en, intervalMs 0 (automatic), 500, 1000, 2000 or 5000. autostart adds or removes CaseHUD from the programs started at sign-in."),
			opBody(v1SettingsPatch{}, "the fields to change"), opOK(200, "the settings after the change", v1Settings{}), opErrs(400)),

		// --- system ---
		op("GET", "/api/v1/update", R, "System", "State of the self-update", opOK(200, "the state", UpdateStatus{})),
		op("POST", "/api/v1/update/check", S, "System", "Check for a new release now", opOK(200, "the state", UpdateStatus{}), opErrs(409)),
		op("POST", "/api/v1/update/install", S, "System", "Install the new release", opNone(202, "started"), opErrs(409)),
		op("POST", "/api/v1/quit", S, "System", "Quit CaseHUD", opNone(202, "quitting"), opErrs(503)),
		op("POST", "/api/v1/gui/open", S, "System", "Open the CaseHUD window", opNone(202, "opening"), opErrs(503)),
	}
	return withStandardErrors(ops)
}

// withStandardErrors adds the answers every operation can give: 403 for a
// missing token or scope, 400 and 413 for a body, 404 for a path with an id,
// 409 for a change of an area that can be switched off.
func withStandardErrors(ops []specOp) []specOp {
	has := func(o specOp, status int) bool {
		for _, r := range o.Responses {
			if r.Status == status {
				return true
			}
		}
		return false
	}
	add := func(o *specOp, status int) {
		if !has(*o, status) {
			opErrs(status)(o)
		}
	}
	for i := range ops {
		o := &ops[i]
		if o.Scope != scopePublic {
			add(o, 403)
		}
		if o.Body != nil || o.RawBody != "" {
			add(o, 400)
			add(o, 413)
		}
		if strings.Contains(o.Path, "{") && o.Scope != scopePublic {
			add(o, 404)
		}
		if a := areaOf(o.Method, o.Path); a == "display" || a == "lighting" {
			add(o, 409)
		}
		if strings.HasPrefix(o.Path, "/api/v1/cooling/") && o.Method != http.MethodGet {
			add(o, 409)
		}
	}
	return ops
}
