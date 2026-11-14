package web

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func specKeys(t *testing.T, doc map[string]any) map[string]string {
	t.Helper()
	out := map[string]string{} // "METHOD path" -> scope
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		for method, o := range item.(map[string]any) {
			scope, _ := o.(map[string]any)["x-scope"].(string)
			out[strings.ToUpper(method)+" "+path] = scope
		}
	}
	return out
}

// Every registered route is described, nothing else is, and the scope in the
// description is the scope that is enforced.
func TestOpenAPIMatchesTheRoutes(t *testing.T) {
	s, _ := testServer(t)
	doc, err := s.OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	spec := specKeys(t, doc)
	routes := map[string]string{}
	for _, r := range s.v1Routes() {
		routes[r.Pattern] = r.Scope
	}
	var problems []string
	for p, scope := range routes {
		sc, ok := spec[p]
		switch {
		case !ok:
			problems = append(problems, "route without description: "+p)
		case sc != scope:
			problems = append(problems, "scope differs for "+p+": route "+scope+", description "+sc)
		}
	}
	for p := range spec {
		if _, ok := routes[p]; !ok {
			problems = append(problems, "description of a route that does not exist: "+p)
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
	if len(routes) != len(s.v1Routes()) {
		t.Fatal("a route is registered twice")
	}
}

// The same route twice in the spec would silently overwrite the first.
func TestOpenAPIHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range v1Spec() {
		k := o.Method + " " + o.Path
		if seen[k] {
			t.Errorf("%s is described twice", k)
		}
		seen[k] = true
		if o.Summary == "" || o.Tag == "" || len(o.Responses) == 0 {
			t.Errorf("%s: summary, tag and an answer are required", k)
		}
	}
}

func TestOpenAPIIsWellFormed(t *testing.T) {
	s, _ := testServer(t)
	doc, err := s.OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("version %v", doc["openapi"])
	}
	// Every $ref points at a schema that exists.
	comps := doc["components"].(map[string]any)["schemas"].(map[string]any)
	for _, m := range regexp.MustCompile(`"\$ref":"#/components/schemas/([A-Za-z0-9_]+)"`).FindAllStringSubmatch(string(b), -1) {
		if _, ok := comps[m[1]]; !ok {
			t.Errorf("dangling $ref %s", m[1])
		}
	}
	// Path parameters in the path are declared; operation ids are unique.
	ids := map[string]string{}
	for path, item := range doc["paths"].(map[string]any) {
		for method, o := range item.(map[string]any) {
			op := o.(map[string]any)
			id := op["operationId"].(string)
			if prev, dup := ids[id]; dup {
				t.Errorf("operationId %s used by %s and %s %s", id, prev, method, path)
			}
			ids[id] = method + " " + path
			declared := map[string]bool{}
			params, _ := op["parameters"].([]any)
			for _, p := range params {
				pm := p.(map[string]any)
				if pm["in"] == "path" {
					declared[pm["name"].(string)] = true
				}
			}
			for _, m := range pathParamRe.FindAllStringSubmatch(path, -1) {
				if !declared[m[1]] {
					t.Errorf("%s %s: parameter %s is not declared", method, path, m[1])
				}
			}
			if _, ok := op["responses"].(map[string]any); !ok {
				t.Errorf("%s %s: no responses", method, path)
			}
			switch op["x-scope"] {
			case "read", "control", "system", "public":
			default:
				t.Errorf("%s %s: scope %v", method, path, op["x-scope"])
			}
		}
	}
	// The schemas are read from the Go types.
	for _, name := range []string{"Panel", "Status", "CoolingAPI", "CoolingConfig", "CoolingOutputPatch", "DesignLayout", "DisplayLayout", "ApiError"} {
		if _, ok := comps[name]; !ok {
			t.Errorf("schema %s is missing", name)
		}
	}
}

// The route table and the check of public routes agree: exactly the pairing
// routes need no token.
func TestPublicRoutesAreOnlyThePairingOnes(t *testing.T) {
	s, _ := testServer(t)
	for _, r := range s.v1Routes() {
		method, path, _ := strings.Cut(r.Pattern, " ")
		path = strings.ReplaceAll(path, "{id}", "abc")
		if got := isPublicV1(method, path); got != (r.Scope == scopePublic) {
			t.Errorf("%s: isPublicV1 = %t, scope %s", r.Pattern, got, r.Scope)
		}
	}
}

// The scope of a route follows what it does: every route that is not a GET and
// not one of the listed read-only POSTs needs control.
func TestScopesFollowTheMethod(t *testing.T) {
	s, _ := testServer(t)
	readOnlyPost := map[string]bool{"POST /api/v1/render": true}
	for _, r := range s.v1Routes() {
		method, _, _ := strings.Cut(r.Pattern, " ")
		switch {
		case r.Scope == scopePublic:
		case method == "GET" && r.Scope != scopeRead:
			t.Errorf("%s needs %s but only reads", r.Pattern, r.Scope)
		case method != "GET" && r.Scope == scopeRead && !readOnlyPost[r.Pattern] && r.Pattern != "DELETE /api/v1/token":
			t.Errorf("%s changes state but needs only read", r.Pattern)
		}
	}
}
