package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// status is the part of GET /api/v1/status the tools use.
type status struct {
	Areas *struct {
		Display bool `json:"display"`
	} `json:"areas"`
	Panels json.RawMessage `json:"panels"`
}

// NewCaseHUD returns the MCP server exposing CaseHUD's tools. Every tool goes
// through the public API (/api/v1/) with the token the user granted; the MCP
// switch of the settings is enforced by CaseHUD itself.
func NewCaseHUD(api *API, version string) *Server {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	designArg := map[string]any{"type": "object", "description": "A CaseHUD layout (see the design_guide tool)."}
	s := &Server{Name: "casehud", Version: version, Instructions: instructions, OnClient: api.SetClient}
	s.Tools = []Tool{
		{
			Name: "design_guide", Title: "Design guide",
			Description: "Explains CaseHUD's design (layout) JSON format, widget types, sensor sources and the rules that keep designs readable on tiny fan LCDs. Read this before creating a design.",
			InputSchema: schema(map[string]any{}),
			Handler:     func(json.RawMessage) (ToolResult, error) { return Text(designGuide), nil },
		},
		{
			Name: "list_displays", Title: "List displays",
			Description: "Lists the connected LCDs (id, name, resolution, round or square) and what each one currently shows.",
			InputSchema: schema(map[string]any{}),
			Handler: func(json.RawMessage) (ToolResult, error) {
				var st status
				if _, err := api.call("GET", "/api/v1/status", nil, &st); err != nil {
					return ToolResult{}, err
				}
				if st.Areas != nil && !st.Areas.Display {
					return Text("area_off: the display area is switched off in the CaseHUD settings; no displays are active."), nil
				}
				var panels any
				if err := json.Unmarshal(st.Panels, &panels); err != nil {
					return ToolResult{}, err
				}
				return Text(pretty(panels)), nil
			},
		},
		{
			Name: "list_sensors", Title: "List sensors",
			Description: "Lists live sensor readings (path, value, unit). Paths or their suffixes (e.g. \"Temperatures/GPU Core\") are used as widget sources.",
			InputSchema: schema(map[string]any{"filter": str("Optional case-insensitive text to filter paths, e.g. \"temperature\" or \"gpu\".")}),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct{ Filter string }
				if err := ParseArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				var rs []struct {
					Path  string  `json:"path"`
					Value float64 `json:"value"`
					Unit  string  `json:"unit"`
				}
				if _, err := api.call("GET", "/api/v1/sensors", nil, &rs); err != nil {
					return ToolResult{}, err
				}
				var b strings.Builder
				f := strings.ToLower(p.Filter)
				for _, r := range rs {
					if f == "" || strings.Contains(strings.ToLower(r.Path), f) {
						fmt.Fprintf(&b, "%s = %.4g %s\n", r.Path, r.Value, r.Unit)
					}
				}
				if b.Len() == 0 {
					return Text("no matching sensors"), nil
				}
				return Text(b.String()), nil
			},
		},
		{
			Name: "list_designs", Title: "List designs",
			Description: "Lists built-in and user designs with their ids.",
			InputSchema: schema(map[string]any{}),
			Handler: func(json.RawMessage) (ToolResult, error) {
				b, err := api.call("GET", "/api/v1/designs", nil, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return Text(string(b)), nil
			},
		},
		{
			Name: "get_design", Title: "Get design",
			Description: "Returns a design's layout JSON, e.g. to use a built-in design as a starting point.",
			InputSchema: schema(map[string]any{"id": str("Design id from list_designs.")}, "id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct{ ID string }
				if err := ParseArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				b, err := api.call("GET", "/api/v1/designs/"+url.PathEscape(p.ID), nil, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return Text(string(b)), nil
			},
		},
		{
			Name: "render_design", Title: "Preview design",
			Description: "Renders a layout with live sensor values and returns the image, so you can check readability before saving. Look at the result critically: it will be shown on a screen about 2-6 cm wide.",
			InputSchema: schema(map[string]any{
				"design": designArg,
				"theme":  map[string]any{"type": "string", "enum": []string{"dark", "light"}, "description": "Optional, default dark."},
			}, "design"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					Design json.RawMessage
					Theme  string
				}
				if err := ParseArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				q := "?w=400"
				if p.Theme == "light" {
					q += "&theme=light"
				}
				b, err := api.call("POST", "/api/v1/render"+q, p.Design, nil)
				if err != nil {
					return ToolResult{}, err
				}
				return ToolResult{Content: []Content{{Type: "image", Data: base64.StdEncoding.EncodeToString(b), MimeType: "image/png"}}}, nil
			},
		},
		{
			Name: "save_design", Title: "Save design",
			Description: "Saves a layout as a user design. It then appears in CaseHUD's gallery and can be put on displays.",
			InputSchema: schema(map[string]any{
				"id":     str("Lowercase id: a-z, 0-9 and '-', e.g. \"ai-coolant-ring\". Saving a built-in id overrides it."),
				"design": designArg,
			}, "id", "design"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					ID     string
					Design json.RawMessage
				}
				if err := ParseArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				if _, err := api.call("PUT", "/api/v1/designs/"+url.PathEscape(p.ID), p.Design, nil); err != nil {
					return ToolResult{}, err
				}
				return Text("saved as " + p.ID), nil
			},
		},
		{
			Name: "apply_design", Title: "Show design on a display",
			Description: "Puts a saved design on a display. Only do this when the user asked for it.",
			InputSchema: schema(map[string]any{
				"display_id": str("Display id from list_displays."),
				"design_id":  str("Design id."),
				"theme":      map[string]any{"type": "string", "enum": []string{"dark", "light"}, "description": "Optional; keeps the display's theme when omitted."},
			}, "display_id", "design_id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct {
					DisplayID string `json:"display_id"`
					DesignID  string `json:"design_id"`
					Theme     string `json:"theme"`
				}
				if err := ParseArgs(args, &p); err != nil {
					return ToolResult{}, err
				}
				patch := map[string]any{"mode": "hud", "design": p.DesignID}
				if p.Theme != "" {
					patch["theme"] = p.Theme
				}
				var panel struct {
					Name string `json:"name"`
				}
				if _, err := api.call("PATCH", "/api/v1/panels/"+url.PathEscape(p.DisplayID), patch, &panel); err != nil {
					if strings.Contains(err.Error(), "not_found") {
						return ToolResult{}, fmt.Errorf("no display with id %q, see list_displays", p.DisplayID)
					}
					return ToolResult{}, err
				}
				return Text(fmt.Sprintf("%s now shows %s", panel.Name, p.DesignID)), nil
			},
		},
	}
	s.Tools = append(s.Tools, curatedTools(api)...)
	return s
}

const instructions = `CaseHUD controls a PC's case hardware through a Windows app: small LCD screens (fan LCDs about 2" round, pump heads, bar displays), RGB lighting, and the cooling (fans and, if present, a pump), plus scenes, rotation, automation and a few settings.

Start with status: it shows which areas (displays, lighting, cooling) are on, the displays, the cooling state and alarm. An area that is switched off cannot be changed; tell the user to switch it on in CaseHUD > Settings > Areas.

Displays: design_guide before creating designs, list_sensors to find real sources, render_design to check visually, then save_design; apply_design or set_display put a design on a display; list_scenes and apply_scene for stored setups.
Lighting: lighting_status, lighting_set, lighting_presets, lighting_preset_apply.
Cooling: cooling_status shows the state, cooling_get_config the outputs and curves (always read it before changing), cooling_set_output gives one fan or pump a curve or a fixed value, cooling_set_curve and cooling_delete_curve edit curves, cooling_presets, cooling_preset_activate and cooling_preset_save handle presets, cooling_calibrate starts a speed measurement.
Everything else (rotation, automation, settings, layouts, designs, scenes, AWTRIX clocks, power): look the route up with api_spec, then call it with api_call.

Safety rules. Only change something when the user asked for it; reading is always fine. A pump (if there is one) must never be set below 30 % and never off; CaseHUD refuses it, do not try to work around it. A fan set too low can overheat the PC: prefer curves over fixed values and check cooling_status afterwards (alarm, problem, temperatures). A speed measurement stops fans and pump for minutes: explain that and get the user's agreement before cooling_calibrate (confirm=true), and never run it while the PC is under load. Quitting or updating CaseHUD, API/MCP switches, sensor source, areas and flashing devices are not available through MCP; some lighting options are only possible in the CaseHUD window (gui_only): tell the user where to do it.

Access. The first call needs the user to confirm access in the CaseHUD window; if a tool says so, tell the user and call it again afterwards. This connection can change things only if the user allowed Control; a tool that answers insufficient_scope needs the user to grant it in CaseHUD > Settings > Access (Change rights). Errors name the code (area_off, revision_mismatch, busy, blocked ...); read the message and tell the user what is needed instead of retrying blindly.`

const designGuide = `# CaseHUD design format

A design is JSON:

{
  "name": "My design", "round": true, "set": "",
  "width": 480, "height": 480, "interval_ms": 1000, "history_size": 90,
  "background": "#000000", "panel": "", "foreground": "#f1f4fa",
  "muted": "#8a94a8", "accent": "#22d3ee", "scanlines": false,
  "widgets": [ ... ]
}

Design in a 480x480 space: CaseHUD scales to each display (TL LCD fans are
400x400 and round, HydroShift pump heads 480x480). Set "round": true for
round screens; corners are cut off there, keep content inside the circle.
"panel" draws tiles behind widgets (leave "" on round screens). Use true
black backgrounds: LCD greys look washed out. Light themes are derived
automatically, design dark.

## Widgets
Common fields: type, x, y, w, h, label, source, unit ("none" hides it),
decimals, min, max, warn, crit, invert, color.

- gauge: 270° ring. thickness (fraction of size, e.g. 0.055), glow,
  smooth (fade colour towards warn/crit), ring_only (no number, combine
  with a centred value), tint (number in the ring colour), segments (N
  blocks), ticks (N scale marks), sub_source/sub_unit/sub_decimals for a
  second reading under the number.
- value: a number. "align": "center" stacks label, number and an optional
  sub_source reading centred; otherwise label top-left, number below.
- bar: horizontal bar with label and value; "segments": 12 makes an LED
  meter.
- graph: history line of source between min and max.
- text: static "text", size in px.
- clock: "text" is a Go time layout, e.g. "15:04" or "Monday, 02.01.".

warn/crit colour the value amber/red at or above the thresholds (0 = off);
"invert": true means lower is worse (FPS, pump rpm).

## Sources
A source matches a sensor path suffix from list_sensors, e.g.
"Temperatures/GPU Core". Alternatives with "|" pick the first that exists:
"Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)". Sources can add
or subtract: "Powers/Package + Powers/GPU Package". Handy ones:
- CPU load "Load/CPU Total", GPU load "Load/GPU Core|Load/D3D 3D"
- CPU temp "Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)|Temperatures/Core (Tctl)"
- GPU temp "Temperatures/GPU Core", hotspot "Temperatures/GPU Hot Spot"
- Coolant (only if the PC has a coolant sensor; check list_sensors) "Temperatures/Coolant|Temperatures/Water|Temperatures/Liquid"
- Pump (only if present) "Fans/Pump|Fans/AIO Pump|Fans/Water Pump"
- FPS "FPS/Framerate", frametime "FPS/Frametime", latency "FPS/Latency"
- VRAM "Data/GPU Memory Used" (GB), "Load/GPU Memory" (%)

## Readability rules (important)
Fan LCDs are about 2" across and seen from 50-80 cm:
- One big number per screen, at least 120 px tall in the 480 space.
- At most one second value, at least 44 px; labels at least 32 px.
- Two numbers stacked (e.g. CPU over GPU) is the maximum for a fan LCD.
- Short neutral English labels (CPU, GPU, FPS, COOLANT).
- Strong colours on black; colour by meaning (warn/crit) rather than decoration.
Always render_design and look at the image before saving.
`
