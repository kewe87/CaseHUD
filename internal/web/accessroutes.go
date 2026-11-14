package web

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/kewe87/casehud/internal/access"
)

// accessClientView is a program in the list of the settings.
type accessClientView struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Scope     access.Scope     `json:"scope"`
	System    bool             `json:"system"`
	Interface access.Interface `json:"interface"`
	Remember  bool             `json:"remember"`
	// Waiting: the user allowed the request with "remember", the program has
	// not collected its token yet. Revoking the entry cancels the request.
	Waiting bool `json:"waiting,omitempty"`
	// Builtin marks the token of api.json: it cannot be revoked, only renewed.
	Builtin  bool       `json:"builtin"`
	Created  *time.Time `json:"created"`
	LastUsed *time.Time `json:"lastUsed"`
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// accessView is what the settings show: the paired programs, and the requests
// that wait for an answer.
type accessView struct {
	Clients []accessClientView `json:"clients"`
	Pending []access.Request   `json:"pending"`
	API     bool               `json:"api"`
	MCP     bool               `json:"mcp"`
}

func (s *Server) accessView() accessView {
	v := accessView{Clients: []accessClientView{}, Pending: s.acc().Pending(), API: s.Engine.APIEnabled(), MCP: s.Engine.MCPEnabled()}
	if v.API {
		c := accessClientView{ID: defaultClientID, Name: defaultClientName, Scope: access.Control, System: true, Interface: access.API, Remember: true, Builtin: true}
		if n := s.defaultUse.Load(); n > 0 {
			c.LastUsed = timePtr(time.Unix(0, n))
		}
		v.Clients = append(v.Clients, c)
	}
	for _, c := range s.acc().List() {
		v.Clients = append(v.Clients, accessClientView{ID: c.ID, Name: c.Name, Scope: c.Scope, System: c.System, Interface: c.Interface,
			Remember: c.Remember, Created: timePtr(c.Created), LastUsed: timePtr(c.LastUsed)})
	}
	for _, c := range s.acc().Waiting() {
		v.Clients = append(v.Clients, accessClientView{ID: c.ID, Name: c.Name, Scope: c.Scope, System: c.System, Interface: c.Interface,
			Remember: true, Waiting: true, Created: timePtr(c.Created)})
	}
	return v
}

// accessRoutes are the routes of the window for the paired programs. They
// answer to the token of the window only: a program can never decide about its
// own access.
func (s *Server) accessRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/access", func(w http.ResponseWriter, r *http.Request) {
		s.guiPolled.Store(time.Now().UnixNano())
		writeJSON(w, s.accessView())
	})
	api.HandleFunc("POST /api/access/requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Allow    bool         `json:"allow"`
			Scope    access.Scope `json:"scope"`
			System   bool         `json:"system"`
			Remember bool         `json:"remember"`
		}
		if !decodeGUI(w, r, &body) {
			return
		}
		var name string
		for _, p := range s.acc().Pending() {
			if p.ID == r.PathValue("id") {
				name = p.Name
			}
		}
		if err := s.acc().Decide(r.PathValue("id"), body.Allow, body.Scope, body.System, body.Remember); err != nil {
			accessFail(w, err)
			return
		}
		if body.Allow {
			log.Printf("access: %q allowed with %s access (system: %t, remembered: %t)", name, body.Scope, body.System, body.Remember)
		} else {
			log.Printf("access: %q refused", name)
		}
		writeJSON(w, s.accessView())
	})
	api.HandleFunc("PUT /api/access/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope  access.Scope `json:"scope"`
			System *bool        `json:"system"`
		}
		if !decodeGUI(w, r, &body) {
			return
		}
		if r.PathValue("id") == defaultClientID {
			fail(w, http.StatusBadRequest, errors.New("the standard access always has full rights; renew it instead"))
			return
		}
		if body.Scope == "" && body.System == nil {
			fail(w, http.StatusBadRequest, errors.New("scope or system is required"))
			return
		}
		if body.Scope != "" {
			if err := s.acc().SetScope(r.PathValue("id"), body.Scope); err != nil {
				accessFail(w, err)
				return
			}
		}
		if body.System != nil {
			if err := s.acc().SetSystem(r.PathValue("id"), *body.System); err != nil {
				accessFail(w, err)
				return
			}
		}
		log.Printf("access: rights of %q changed (scope %q, system %v)", s.clientName(r.PathValue("id")), body.Scope, body.System != nil && *body.System)
		writeJSON(w, s.accessView())
	})
	api.HandleFunc("DELETE /api/access/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == defaultClientID {
			if err := s.RenewClientToken(); err != nil {
				fail(w, http.StatusConflict, err)
				return
			}
			log.Printf("access: the standard access was renewed")
			writeJSON(w, s.accessView())
			return
		}
		name := s.clientName(id)
		if err := s.acc().Revoke(id); err != nil {
			accessFail(w, err)
			return
		}
		log.Printf("access: %q revoked", name)
		writeJSON(w, s.accessView())
	})
}

func (s *Server) clientName(id string) string {
	for _, c := range s.acc().List() {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

func accessFail(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	if errors.Is(err, access.ErrNotFound) {
		code = http.StatusNotFound
	}
	fail(w, code, err)
}

// decodeGUI reads a small JSON body of the window's own routes.
func decodeGUI(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, err)
		return false
	}
	return true
}
