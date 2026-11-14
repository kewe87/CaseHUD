package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/access"
)

// Who a request of the public API comes from.
const (
	kindGUI     = "gui"     // the token of the window
	kindDefault = "default" // the token of api.json
	kindPaired  = "paired"  // a program the user let in
)

// defaultClientID and defaultClientName stand for the token of api.json in the
// list of programs: it was there before pairing existed, so programs that read
// api.json keep working.
const (
	defaultClientID   = "default"
	defaultClientName = "Standard-Zugang"
)

// v1Identity is the caller of a v1 request.
type v1Identity struct {
	ID        string
	Name      string
	Scope     access.Scope
	System    bool // the extra right to quit and update CaseHUD
	Kind      string
	Interface access.Interface
}

type identityKey struct{}

func identityOf(r *http.Request) (v1Identity, bool) {
	id, ok := r.Context().Value(identityKey{}).(v1Identity)
	return id, ok
}

// acc returns the list of paired programs, loaded on first use.
func (s *Server) acc() *access.Manager {
	s.accOnce.Do(func() {
		if s.AccessFile != "" {
			old, _ := filepath.Glob(s.AccessFile + ".*.tmp")
			for _, f := range old {
				os.Remove(f) // left behind by a crashed write
			}
		}
		m, err := access.Open(s.AccessFile, WriteSecretFile)
		if err != nil {
			log.Println("access:", err)
		}
		s.accMgr = m
	})
	return s.accMgr
}

// FlushAccess writes what is still unsaved of the paired programs (call it
// when the app ends).
func (s *Server) FlushAccess() { s.acc().Flush() }

// isPublicV1 names the requests that need no token: the pairing itself. They
// still need the right Host header and no Origin header.
func isPublicV1(method, path string) bool {
	const p = "/api/v1/pair"
	if method == http.MethodPost && path == p {
		return true
	}
	rest, ok := strings.CutPrefix(path, p+"/")
	return method == http.MethodGet && ok && rest != "" && !strings.Contains(rest, "/")
}

// authenticate finds who a token belongs to. ok is false for an unknown token.
// code and msg say why a known token is refused (the switch of its interface
// is off). end is closed when the access ends while a request is still open.
func (s *Server) authenticate(tok string) (id v1Identity, end []<-chan struct{}, ok bool, code, msg string) {
	if tok == "" {
		return id, nil, false, "", ""
	}
	if subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) == 1 {
		return v1Identity{ID: "gui", Name: "GUI", Scope: access.Control, System: true, Kind: kindGUI}, nil, true, "", ""
	}
	if gen, isClient := s.clientAuth(tok); isClient {
		s.defaultUse.Store(time.Now().UnixNano())
		return v1Identity{ID: defaultClientID, Name: defaultClientName, Scope: access.Control, System: true, Kind: kindDefault, Interface: access.API},
			[]<-chan struct{}{gen}, true, "", ""
	}
	c, found := s.acc().Authenticate(tok)
	if !found {
		if s.acc().IsRevoked(tok) {
			return id, nil, false, "revoked", "this access was taken away in CaseHUD; the program has to ask the user again (restart it)"
		}
		return id, nil, false, "", ""
	}
	id = v1Identity{ID: c.ID, Name: c.Name, Scope: c.Scope, System: c.System, Kind: kindPaired, Interface: c.Interface}
	switch c.Interface {
	case access.MCP:
		if !s.Engine.MCPEnabled() {
			return id, nil, false, "mcp_disabled", "the MCP interface is switched off in the CaseHUD settings (Settings > Interfaces)"
		}
		end = append(end, s.mcpOff())
	default:
		if !s.Engine.APIEnabled() {
			return id, nil, false, "api_disabled", "the API is switched off in the CaseHUD settings (Settings > Interfaces)"
		}
		end = append(end, s.apiOff())
	}
	return id, append(end, c.Gone), true, "", ""
}

// authV1 guards /api/v1/: right Host header, then the token of the window, the
// token of api.json or the token of a paired program (X-Token header; ?t= only
// for GET .../preview.png so <img> works). Pairing needs no token.
func (s *Server) authV1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostOK(r.Host) {
			apiFail(w, http.StatusForbidden, "forbidden", "unexpected Host header")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if isPublicV1(r.Method, r.URL.Path) {
			if r.Header.Get("Origin") != "" { // a web page, not a program
				apiFail(w, http.StatusForbidden, "forbidden", "requests from web pages are not accepted")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		tok := r.Header.Get("X-Token")
		if tok == "" && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "preview.png") {
			tok = r.URL.Query().Get("t")
		}
		id, end, ok, code, msg := s.authenticate(tok)
		switch {
		case code != "":
			apiFail(w, http.StatusForbidden, code, msg)
			return
		case !ok && !s.Engine.APIEnabled() && !s.Engine.MCPEnabled():
			apiFail(w, http.StatusForbidden, "api_disabled", "the API is switched off in the CaseHUD settings (Settings > Interfaces)")
			return
		case !ok:
			apiFail(w, http.StatusForbidden, "unauthorized", "missing or wrong token")
			return
		}
		ctx := context.WithValue(r.Context(), identityKey{}, id)
		var ends []<-chan struct{}
		for _, c := range end {
			if c != nil {
				ends = append(ends, c)
			}
		}
		if len(ends) > 0 {
			// Switching the API off, renewing the token or revoking the program
			// ends its streams at once.
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			for _, c := range ends {
				go func() {
					select {
					case <-c:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// mcpRefused names the routes a token of the MCP interface never reaches, as
// the MCP server's api_call tool does not offer them either. The server
// enforces it too, so a program that holds an MCP token cannot call them
// directly. The system routes are refused for MCP in scopeGate.
var mcpRefused = map[string]string{
	"GET /api/v1/events":   "the event stream is not available through MCP; poll status instead",
	"DELETE /api/v1/token": "the token of an MCP connection is managed in the CaseHUD window",
}

// scopeGate answers a request whose caller lacks the scope the route needs with
// 403 insufficient_scope, and logs each request that changes something with the
// name of the caller. It runs before the area gate, so a program without the
// right is told so whatever the state of the areas.
func (s *Server) scopeGate(m *http.ServeMux, scopes map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := m.Handler(r)
		need := scopes[pattern]
		if need == "" || need == scopePublic {
			next.ServeHTTP(w, r)
			return
		}
		id, ok := identityOf(r)
		if !ok { // authV1 always sets it
			apiFail(w, http.StatusForbidden, "unauthorized", "missing or wrong token")
			return
		}
		if id.Interface == access.MCP {
			why, refused := mcpRefused[pattern]
			if need == scopeSystem {
				refused, why = true, "quitting, updating and opening the window are not available through MCP"
			}
			if refused {
				apiFail(w, http.StatusForbidden, "not_for_mcp", why)
				return
			}
		}
		switch {
		case need == scopeSystem && (!id.System || id.Scope != access.Control):
			apiFail(w, http.StatusForbidden, "insufficient_scope", "this program lacks the system right (quit, update); the user can grant it in CaseHUD > Settings > Access for "+r.Method+" "+r.URL.Path)
			return
		case need != scopeSystem && !id.Scope.Allows(access.Scope(need)):
			apiFail(w, http.StatusForbidden, "insufficient_scope", "this program has read access only; control access is needed for "+r.Method+" "+r.URL.Path+" (the user can grant it in CaseHUD > Settings > Access)")
			return
		}
		if need == scopeControl || need == scopeSystem {
			log.Printf("api: %s %s by %q (%s)", r.Method, r.URL.EscapedPath(), id.Name, id.Kind)
		}
		next.ServeHTTP(w, r)
	})
}

// pairGuard checks what only a program, not a web page, can do: a JSON body.
func pairGuard(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return true
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		apiFail(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "send Content-Type: application/json")
		return false
	}
	return true
}

// v1Pair registers a request for access. The user decides in the window.
func (s *Server) v1Pair(w http.ResponseWriter, r *http.Request) {
	if !pairGuard(w, r) {
		return
	}
	var req v1PairRequest
	if !decodeBody(w, r, &req) {
		return
	}
	via := access.API
	if req.Interface != "" {
		via = access.Interface(req.Interface)
		if !via.Valid() {
			badRequest(w, "interface must be api or mcp")
			return
		}
	}
	switch {
	case via == access.API && !s.Engine.APIEnabled():
		apiFail(w, http.StatusForbidden, "api_disabled", "the API is switched off in the CaseHUD settings (Settings > Interfaces)")
		return
	case via == access.MCP && !s.Engine.MCPEnabled():
		apiFail(w, http.StatusForbidden, "mcp_disabled", "the MCP interface is switched off in the CaseHUD settings (Settings > Interfaces)")
		return
	}
	want, system := access.Read, false
	for _, sc := range req.Scopes {
		switch sc {
		case string(access.Read):
		case string(access.Control):
			want = access.Control
		case access.SystemRight:
			system = true
		default:
			badRequest(w, "scopes may contain read, control and system")
			return
		}
	}
	pr, err := s.acc().Pair(req.Name, want, via, system)
	var rate *access.RateError
	switch {
	case errors.As(err, &rate):
		w.Header().Set("Retry-After", strconv.Itoa(int(rate.RetryAfter.Seconds())+1))
		apiFail(w, http.StatusTooManyRequests, "rate_limited", err.Error())
		return
	case errors.Is(err, access.ErrTooManyPending):
		apiFail(w, http.StatusTooManyRequests, "too_many_pending", err.Error())
		return
	case errors.Is(err, access.ErrFull):
		apiFail(w, http.StatusConflict, "limit_reached", err.Error())
		return
	case errors.Is(err, access.ErrBadName), errors.Is(err, access.ErrBadScope), errors.Is(err, access.ErrReservedName):
		badRequest(w, "%v", err)
		return
	case err != nil:
		internalError(w, err)
		return
	}
	log.Printf("access: %q asks for %s access (system: %t) via %s", pr.Name, pr.Scope, pr.System, pr.Interface)
	s.bringUpWindow()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSONBody(w, v1PairAccepted{ID: pr.ID, Status: access.StatusPending, ExpiresIn: int(access.PendingTTL.Seconds()), Poll: "/api/v1/pair/" + pr.ID})
}

// v1PairPoll tells a program how its request stands; the token comes once.
func (s *Server) v1PairPoll(w http.ResponseWriter, r *http.Request) {
	res, err := s.acc().Poll(r.PathValue("id"))
	if errors.Is(err, access.ErrNotFound) {
		notFound(w, "no such access request (it expired, or its answer was already collected)")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	if res.Status == access.StatusApproved {
		log.Printf("access: a program was let in with %s access (system: %t)", res.Scope, res.System)
	}
	writeJSON(w, res)
}

// bringUpWindow shows the window for a new request when it is not being watched,
// so the user sees the question even if CaseHUD sits in the tray.
func (s *Server) bringUpWindow() {
	if s.OnShow == nil || time.Since(time.Unix(0, s.guiPolled.Load())) < 15*time.Second {
		return
	}
	go func() {
		if err := s.OnShow(); err != nil {
			log.Println("show window:", err)
		}
	}()
}

// v1Token tells a program what its own token may do.
func (s *Server) v1Token(w http.ResponseWriter, r *http.Request) {
	id, _ := identityOf(r)
	writeJSON(w, v1TokenInfo{ID: id.ID, Name: id.Name, Scope: string(id.Scope), System: id.System, Kind: id.Kind, Interface: string(id.Interface)})
}

// v1RevokeToken lets a program give its access up.
func (s *Server) v1RevokeToken(w http.ResponseWriter, r *http.Request) {
	id, _ := identityOf(r)
	if id.Kind != kindPaired {
		badRequest(w, "only the token of a paired program can be given up here")
		return
	}
	if err := s.acc().Revoke(id.ID); err != nil {
		notFound(w, "%v", err)
		return
	}
	log.Printf("access: %q gave its access up", id.Name)
	w.WriteHeader(http.StatusNoContent)
}
