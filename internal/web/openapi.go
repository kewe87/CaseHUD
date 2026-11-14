package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The OpenAPI 3.1 description of /api/v1/. It is written down in openapi_spec.go
// as Go data: the operations with their scope, parameters, bodies and answers.
// Schemas of bodies and answers are read from the Go types the handlers use, so
// they cannot drift from the code. openapi_test.go checks that every registered
// route is described, that nothing else is, and that the scope of both agree.

// specOp describes one operation.
type specOp struct {
	Method, Path string
	Scope        string
	Tag          string
	Summary      string
	Description  string
	Params       []specParam
	Body         any    // a value of the Go type of the JSON body, nil without
	BodyDesc     string // what the body means
	Optional     bool   // the body may be left out
	RawBody      string // a body that is not the Go type: the schema as JSON
	Responses    []specResp
}

type specParam struct {
	In, Name, Desc string
	Required       bool
	Type           string // "string", "integer", "boolean"
	Enum           []string
}

type specResp struct {
	Status  int
	Desc    string
	Body    any    // Go type, nil for no body
	Media   string // default application/json
	Raw     string // schema as JSON, when Body is not a Go type
	Headers map[string]string
}

// An option of op.
type specOpt func(*specOp)

func op(method, path, scope, tag, summary string, opts ...specOpt) specOp {
	o := specOp{Method: method, Path: path, Scope: scope, Tag: tag, Summary: summary}
	for _, f := range opts {
		f(&o)
	}
	return o
}

func opDesc(s string) specOpt { return func(o *specOp) { o.Description = s } }
func opBody(v any, d string) specOpt {
	return func(o *specOp) { o.Body, o.BodyDesc = v, d }
}
func opOptBody(v any, d string) specOpt {
	return func(o *specOp) { o.Body, o.BodyDesc, o.Optional = v, d, true }
}
func opQuery(name, d, typ string, required bool, enum ...string) specOpt {
	return func(o *specOp) {
		o.Params = append(o.Params, specParam{In: "query", Name: name, Desc: d, Required: required, Type: typ, Enum: enum})
	}
}
func opHeader(name, d string, required bool) specOpt {
	return func(o *specOp) {
		o.Params = append(o.Params, specParam{In: "header", Name: name, Desc: d, Required: required, Type: "string"})
	}
}
func opOK(status int, d string, v any) specOpt {
	return func(o *specOp) { o.Responses = append(o.Responses, specResp{Status: status, Desc: d, Body: v}) }
}
func opOKHdr(status int, d string, v any, hdr map[string]string) specOpt {
	return func(o *specOp) {
		o.Responses = append(o.Responses, specResp{Status: status, Desc: d, Body: v, Headers: hdr})
	}
}
func opNone(status int, d string) specOpt {
	return func(o *specOp) { o.Responses = append(o.Responses, specResp{Status: status, Desc: d}) }
}
func opPNG(d string) specOpt {
	return func(o *specOp) {
		o.Responses = append(o.Responses, specResp{Status: 200, Desc: d, Media: "image/png", Raw: `{"type":"string","format":"binary"}`})
	}
}
func opErrs(codes ...int) specOpt {
	return func(o *specOp) {
		for _, c := range codes {
			o.Responses = append(o.Responses, specResp{Status: c, Desc: errorDescriptions[c], Body: apiError{}})
		}
	}
}

var errorDescriptions = map[int]string{
	400: "bad_request, invalid_value or confirmation_required: invalid body, unknown field or value out of range",
	403: "unauthorized, revoked (the token was taken away), insufficient_scope, not_for_mcp (a route a token of the MCP interface never reaches), forbidden, api_disabled or mcp_disabled",
	404: "not_found",
	409: "the request cannot be done in the current state (see error.code)",
	413: "payload_too_large: the body is larger than 1 MiB",
	415: "unsupported_media_type: send Content-Type: application/json",
	428: "precondition_required: send the revision in If-Match",
	429: "rate_limited or too_many_pending",
	502: "bad_gateway: the community gallery could not be reached",
	503: "unavailable, busy or too_many_clients",
}

// --- schema reflection ---

type schemaBuilder struct {
	components map[string]any
	owner      map[string]reflect.Type
	err        error
}

var (
	timeType      = reflect.TypeOf(time.Time{})
	rawType       = reflect.TypeOf(json.RawMessage{})
	marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
)

// componentRenames names the types whose plain name is taken twice.
var componentRenames = map[string]string{
	"web.v1Layout": "DisplayLayout",
	"lcd.Layout":   "DesignLayout",
}

func componentName(t reflect.Type) string {
	if n, ok := componentRenames[t.String()]; ok {
		return n
	}
	n := t.Name()
	n = strings.TrimPrefix(n, "v1")
	if n == "" {
		return ""
	}
	return strings.ToUpper(n[:1]) + n[1:]
}

// schema returns the JSON schema of t.
func (b *schemaBuilder) schema(t reflect.Type) map[string]any {
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t == rawType:
		return map[string]any{}
	}
	if t.Kind() != reflect.Pointer && t.Kind() != reflect.Interface && reflect.PointerTo(t).Implements(marshalerType) && t.Kind() != reflect.Struct {
		return map[string]any{}
	}
	switch t.Kind() {
	case reflect.Pointer:
		inner := b.schema(t.Elem())
		if ref, ok := inner["$ref"]; ok {
			return map[string]any{"oneOf": []any{map[string]any{"$ref": ref}, map[string]any{"type": "null"}}}
		}
		if typ, ok := inner["type"].(string); ok && len(inner) == 1 {
			return map[string]any{"type": []any{typ, "null"}}
		}
		return map[string]any{"oneOf": []any{inner, map[string]any{"type": "null"}}}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": b.schema(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": b.schema(t.Elem())}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Struct:
		if t.Name() == "" {
			return b.object(t)
		}
		name := componentName(t)
		if owner, taken := b.owner[name]; taken && owner != t {
			b.err = fmt.Errorf("two types are named %s: %v and %v", name, owner, t)
		}
		if _, done := b.owner[name]; !done {
			b.owner[name] = t
			b.components[name] = map[string]any{} // placeholder against recursion
			b.components[name] = b.object(t)
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	}
	return map[string]any{}
}

func (b *schemaBuilder) object(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name, opts, _ := strings.Cut(tag, ",")
			if f.Anonymous && name == "" {
				ft := f.Type
				if ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					walk(ft)
					continue
				}
			}
			if !f.IsExported() {
				continue
			}
			if name == "" {
				name = f.Name
			}
			props[name] = b.schema(f.Type)
			if !strings.Contains(opts, "omitempty") && !strings.Contains(opts, "omitzero") && f.Type.Kind() != reflect.Pointer {
				required = append(required, name)
			}
		}
	}
	walk(t)
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		out["required"] = required
	}
	return out
}

// --- the document ---

var pathParamRe = regexp.MustCompile(`\{([A-Za-z_]+)\}`)

// OpenAPI builds the description of the API.
func (s *Server) OpenAPI() (map[string]any, error) {
	return buildOpenAPI(v1Spec(), s.Version)
}

func buildOpenAPI(ops []specOp, version string) (map[string]any, error) {
	b := &schemaBuilder{components: map[string]any{}, owner: map[string]reflect.Type{}}
	paths := map[string]any{}
	jsonSchema := func(v any, raw string) any {
		if raw != "" {
			var m any
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				b.err = fmt.Errorf("bad raw schema %q: %w", raw, err)
			}
			return m
		}
		return b.schema(reflect.TypeOf(v))
	}
	for _, o := range ops {
		item, _ := paths[o.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[o.Path] = item
		}
		out := map[string]any{
			"operationId": operationID(o),
			"summary":     o.Summary,
			"tags":        []string{o.Tag},
			"x-scope":     o.Scope,
		}
		if o.Description != "" {
			out["description"] = o.Description
		}
		if o.Scope == scopePublic {
			out["security"] = []any{}
		} else {
			out["security"] = []any{map[string]any{"ApiToken": []any{}}}
		}
		var params []any
		for _, m := range pathParamRe.FindAllStringSubmatch(o.Path, -1) {
			params = append(params, map[string]any{"name": m[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		for _, p := range o.Params {
			sc := map[string]any{"type": p.Type}
			if len(p.Enum) > 0 {
				sc["enum"] = p.Enum
			}
			params = append(params, map[string]any{"name": p.Name, "in": p.In, "description": p.Desc, "required": p.Required, "schema": sc})
		}
		if len(params) > 0 {
			out["parameters"] = params
		}
		if o.Body != nil || o.RawBody != "" {
			out["requestBody"] = map[string]any{
				"description": o.BodyDesc, "required": !o.Optional,
				"content": map[string]any{"application/json": map[string]any{"schema": jsonSchema(o.Body, o.RawBody)}},
			}
		}
		resps := map[string]any{}
		for _, r := range o.Responses {
			rm := map[string]any{"description": r.Desc}
			if r.Body != nil || r.Raw != "" {
				media := r.Media
				if media == "" {
					media = "application/json"
				}
				rm["content"] = map[string]any{media: map[string]any{"schema": jsonSchema(r.Body, r.Raw)}}
			}
			if len(r.Headers) > 0 {
				h := map[string]any{}
				for k, d := range r.Headers {
					h[k] = map[string]any{"description": d, "schema": map[string]any{"type": "string"}}
				}
				rm["headers"] = h
			}
			resps[strconv.Itoa(r.Status)] = rm
		}
		out["responses"] = resps
		item[strings.ToLower(o.Method)] = out
	}
	if b.err != nil {
		return nil, b.err
	}
	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "CaseHUD API",
			"version": strconv.Itoa(APIVersion) + " (CaseHUD " + version + ")",
			"description": "The stable HTTP API of CaseHUD for other programs, on 127.0.0.1 only. " +
				"Every request needs an X-Token header; a program gets its token through POST /pair, " +
				"which the user answers in the CaseHUD window. Routes with scope read only read; " +
				"routes with scope control change state; routes with scope system (quit, update, open the window) need that right as well, which is separate from control and only exists together with it. " +
				"A token of the MCP interface never reaches the system routes, the event stream or DELETE /token (403 not_for_mcp), whatever its rights. " +
				"Errors are {\"error\":{\"code\",\"message\"}}. " +
				"The routes of the CaseHUD window (/api/...) answer to the window's token only and are not part of this API; two contracts are listed under x-window-routes.",
			"x-window-routes": map[string]any{
				"GET /api/access": "The list of programs for the settings. Besides the paired programs it lists requests the user allowed with \"remember\" whose program has not collected its token yet, as entries with waiting: true (id, name, scope, system as for the others). DELETE /api/access/clients/{id} on such an entry cancels the pairing.",
				"/api/ai/...":     "Errors of connecting or disconnecting an AI app are JSON {code, message, error}. code is one of invalid_json, duplicate_keys, symlink, not_found, write_failed, cli_failed, unsupported, conflict; message names no file or folder. The HTTP status is 409 for conflict, 404 for not_found, 422 when a file was refused (invalid_json, duplicate_keys, symlink, some unsupported and write_failed), 502 otherwise.",
			},
		},
		"servers": []any{map[string]any{
			"url": "http://127.0.0.1:{port}", "description": "the port is in api.json (the data folder), or a program pairs through the port it was told",
			"variables": map[string]any{"port": map[string]any{"default": "0"}},
		}},
		"tags":  tagList(ops),
		"paths": paths,
		"components": map[string]any{
			"schemas": b.components,
			"securitySchemes": map[string]any{"ApiToken": map[string]any{
				"type": "apiKey", "in": "header", "name": "X-Token",
				"description": "The token of a paired program (POST /pair), the token of api.json, or the window's own token. GET .../preview.png also accepts ?t=.",
			}},
		},
	}
	return doc, nil
}

func tagList(ops []specOp) []any {
	seen := map[string]bool{}
	var out []any
	for _, o := range ops {
		if !seen[o.Tag] {
			seen[o.Tag] = true
			out = append(out, map[string]any{"name": o.Tag})
		}
	}
	return out
}

// operationID is the method and path in camel case: GET /cooling/measure is getCoolingMeasure.
func operationID(o specOp) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(o.Method))
	for _, part := range strings.FieldsFunc(strings.TrimPrefix(o.Path, "/api/v1/"), func(r rune) bool {
		return r == '/' || r == '-' || r == '.' || r == '{' || r == '}'
	}) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// v1OpenAPI serves the description. It needs the read scope like everything
// else: it says what the API offers on this machine, nothing secret, but there
// is no reason to answer a program that was not let in.
func (s *Server) v1OpenAPI(w http.ResponseWriter, r *http.Request) {
	doc, err := s.OpenAPI()
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, doc)
}
