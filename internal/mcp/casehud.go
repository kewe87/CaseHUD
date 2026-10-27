package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// API is the running CaseHUD app's local HTTP API.
type API struct {
	Base, Token string
	HTTP        *http.Client
}

// NewAPI parses the GUI address CaseHUD writes to instance.url
// ("http://127.0.0.1:PORT/?t=TOKEN").
func NewAPI(instanceURL string) (*API, error) {
	u, err := url.Parse(strings.TrimSpace(instanceURL))
	if err != nil || u.Host == "" || u.Query().Get("t") == "" {
		return nil, fmt.Errorf("CaseHUD is not running (no valid instance.url)")
	}
	return &API{Base: u.Scheme + "://" + u.Host, Token: u.Query().Get("t"), HTTP: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (a *API) call(method, path string, body any, out any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.Base+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", a.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CaseHUD is not reachable, is it running? (%v)", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("CaseHUD: %s", strings.TrimSpace(string(b)))
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return nil, err
		}
	}
	return b, nil
}

type panelState struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Width  int             `json:"width"`
	Height int             `json:"height"`
	Round  bool            `json:"round"`
	Config json.RawMessage `json:"config"`
}

type state struct {
	Panels   []panelState    `json:"panels"`
	Designs  json.RawMessage `json:"designs"`
	Settings struct {
		MCP bool `json:"mcp"`
	} `json:"settings"`
}

func (a *API) state() (state, error) {
	var s state
	_, err := a.call("GET", "/api/state", nil, &s)
	if err == nil && !s.Settings.MCP {
		err = fmt.Errorf("the MCP interface is turned off in CaseHUD. Ask the user to enable it under Settings (expert mode) > \"MCP interface for AI assistants\"")
	}
	return s, err
}

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

// NewCaseHUD returns the MCP server exposing CaseHUD's tools.
func NewCaseHUD(api *API, version string) *Server {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	designArg := map[string]any{"type": "object", "description": "A CaseHUD layout (see the design_guide tool)."}
	s := &Server{Name: "casehud", Version: version, Instructions: instructions}
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
				st, err := api.state()
				if err != nil {
					return ToolResult{}, err
				}
				return Text(pretty(st.Panels)), nil
			},
		},
		{
			Name: "list_sensors", Title: "List sensors",
			Description: "Lists live sensor readings (path, value, unit). Paths or their suffixes (e.g. \"Temperatures/GPU Core\") are used as widget sources.",
			InputSchema: schema(map[string]any{"filter": str("Optional case-insensitive text to filter paths, e.g. \"temperature\" or \"gpu\".")}),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct{ Filter string }
				json.Unmarshal(args, &p)
				if _, err := api.state(); err != nil {
					return ToolResult{}, err
				}
				var rs []struct {
					Path  string  `json:"Path"`
					Value float64 `json:"Value"`
					Unit  string  `json:"Unit"`
				}
				if _, err := api.call("GET", "/api/sensors", nil, &rs); err != nil {
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
				st, err := api.state()
				if err != nil {
					return ToolResult{}, err
				}
				return Text(string(st.Designs)), nil
			},
		},
		{
			Name: "get_design", Title: "Get design",
			Description: "Returns a design's layout JSON, e.g. to use a built-in design as a starting point.",
			InputSchema: schema(map[string]any{"id": str("Design id from list_designs.")}, "id"),
			Handler: func(args json.RawMessage) (ToolResult, error) {
				var p struct{ ID string }
				json.Unmarshal(args, &p)
				if _, err := api.state(); err != nil {
					return ToolResult{}, err
				}
				b, err := api.call("GET", "/api/designs/"+url.PathEscape(p.ID), nil, nil)
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
				json.Unmarshal(args, &p)
				if _, err := api.state(); err != nil {
					return ToolResult{}, err
				}
				q := "?w=400"
				if p.Theme == "light" {
					q += "&theme=light"
				}
				b, err := api.call("POST", "/api/render"+q, p.Design, nil)
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
				json.Unmarshal(args, &p)
				if _, err := api.state(); err != nil {
					return ToolResult{}, err
				}
				if _, err := api.call("PUT", "/api/designs/"+url.PathEscape(p.ID), p.Design, nil); err != nil {
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
				json.Unmarshal(args, &p)
				st, err := api.state()
				if err != nil {
					return ToolResult{}, err
				}
				for _, ps := range st.Panels {
					if ps.ID != p.DisplayID {
						continue
					}
					var cfg map[string]any
					json.Unmarshal(ps.Config, &cfg)
					if cfg == nil {
						cfg = map[string]any{}
					}
					cfg["mode"], cfg["design"] = "design", p.DesignID
					delete(cfg, "grid") // a single design replaces a grid
					delete(cfg, "tiles")
					if p.Theme != "" {
						cfg["theme"] = p.Theme
					}
					if _, err := api.call("PUT", "/api/panels/"+url.PathEscape(ps.ID), cfg, nil); err != nil {
						return ToolResult{}, err
					}
					return Text(fmt.Sprintf("%s now shows %s", ps.Name, p.DesignID)), nil
				}
				return ToolResult{}, fmt.Errorf("no display with id %q, see list_displays", p.DisplayID)
			},
		},
	}
	return s
}

const instructions = `CaseHUD drives small LCD screens inside PC cases (fan LCDs about 2" round, pump heads, case screens) with sensor dashboards. Use design_guide before creating designs, list_sensors to find real sources, render_design to check your work visually, then save_design. Only change what is on a display (apply_design) when the user asks.`

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
- Coolant "Temperatures/T Sensor|Temperatures/Coolant|Temperatures/Water|Temperatures/Liquid"
- Pump "Fans/Pump|Fans/AIO Pump|Fans/W_PUMP+"
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
