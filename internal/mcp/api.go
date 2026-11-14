package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/fileacl"
)

// TokenFileFor is the token file of an app CaseHUD set up (Settings > AI
// connection): the window writes the token there, so the app needs no pairing
// dialog. id is one of the fixed app ids (lower case letters, digits, "-").
func TokenFileFor(id string) (string, bool) {
	if id == "" || len(id) > 32 {
		return "", false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return "", false
		}
	}
	return "mcp-token-" + id + ".json", true
}

// TokenFileName is the file in the data folder that holds the token CaseHUD
// gave this MCP server. It is separate from api.json.
const TokenFileName = "mcp-token.json" //nolint:gosec // a file name, not a credential

// userError is a text for the model and the user, written as a sentence.
type userError string

func (e userError) Error() string { return string(e) }

// Texts of the errors a model is told. The first one is what the user has to do.
const (
	msgConfirm = "Bitte in CaseHUD den Zugriff bestätigen (Fenster „Zugriff erlauben“, Einstellungen > Zugriffe). Danach das Werkzeug erneut aufrufen. / Ask the user to confirm access in CaseHUD (window \"Allow access\", Settings > Access), then call the tool again."
	msgDenied  = "Der Zugriff wurde in CaseHUD abgelehnt. Der Benutzer kann ihn beim nächsten Aufruf erneut erlauben. / Access was denied in CaseHUD. The user can allow it again at the next call."
	msgRevoked = "Der Zugriff wurde in CaseHUD entzogen. Dieser MCP-Server fragt nicht von selbst neu an: den Benutzer bitten, die KI-App bzw. den MCP-Server neu zu starten, dann erscheint die Anfrage erneut. / Access was taken away in CaseHUD. This MCP server does not ask again by itself: ask the user to restart the AI app or MCP server, which asks for access again."
	msgMCPOff  = "The MCP interface is turned off in CaseHUD. Ask the user to enable it under Settings (expert mode) > Interfaces > \"MCP interface for AI assistants\"."
)

// Texts for a missing right. They name the place where the user grants it.
const (
	msgNoControl = "insufficient_scope: Dieser MCP-Zugang darf nur lesen. Den Benutzer bitten, in CaseHUD unter Einstellungen > Zugriffe bei diesem Programm „Rechte ändern“ und „Steuern“ zu erlauben. / " +
		"This MCP connection is read-only. Ask the user to allow Control for it in CaseHUD > Settings > Access (Change rights)."
	msgNoSystem = "insufficient_scope: Dafür fehlt das Recht „System“ (Beenden, Updates). Es ist über MCP nicht vorgesehen; der Benutzer kann es in CaseHUD unter Einstellungen > Zugriffe erteilen. / " +
		"This needs the System right (quit, update), which MCP does not use. The user can grant it in CaseHUD > Settings > Access."
)

// ErrAreaOff is the answer of a tool that would change the displays while that
// area is switched off.
const ErrAreaOff = "area_off: the display area is switched off in the CaseHUD settings, so no display can be changed. Ask the user to switch it on under Settings > Areas."

// API is the running CaseHUD app's public API (/api/v1/), used with a token of
// its own. The token comes from the user: on the first call without one the
// server asks CaseHUD for access (POST /pair) and waits for the answer.
type API struct {
	Base string
	HTTP *http.Client
	// TokenFile holds the token between runs ("" = only in memory).
	TokenFile string
	// Write stores the token file; the default is private to the user.
	Write func(path string, data []byte) error
	// PairWait is how long a tool call waits for the user to answer (default 25 s).
	PairWait time.Duration
	// PollEvery is the pause between two asks (default 1 s).
	PollEvery time.Duration

	mu     sync.Mutex
	client string // the name the MCP client gave
	token  string
	pairID string
	loaded bool
	// revoked: the user took the access away. No new request until the MCP
	// server is started again, so a revoked connection does not ask at once.
	revoked bool
}

// NewAPI parses the GUI address CaseHUD writes to instance.url
// ("http://127.0.0.1:PORT/?t=TOKEN") and keeps only where it is: the token of
// the window is never used or stored here.
func NewAPI(instanceURL string) (*API, error) {
	u, err := url.Parse(strings.TrimSpace(instanceURL))
	if err != nil || u.Host == "" || u.Scheme == "" {
		return nil, fmt.Errorf("CaseHUD is not running (no valid instance.url)")
	}
	// The file is written by CaseHUD for this computer; anything else in it is
	// not trusted with the token of this connection.
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		return nil, fmt.Errorf("instance.url does not point to this computer")
	}
	if u.Scheme != "http" || u.Port() == "" {
		return nil, fmt.Errorf("instance.url is not an address CaseHUD writes")
	}
	return &API{Base: u.Scheme + "://" + u.Host, HTTP: newHTTPClient()}, nil
}

// newHTTPClient never follows a redirect: the token would go along to wherever
// it points.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// SetClient remembers the name of the MCP client for the access request.
func (a *API) SetClient(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = strings.TrimSpace(name)
}

// pairName is the name the user sees in the window.
func (a *API) pairName() string {
	if a.client == "" {
		return "MCP"
	}
	return "MCP (" + a.client + ")"
}

type tokenFile struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

func (a *API) loadToken() {
	if a.loaded {
		return
	}
	a.loaded = true
	if a.TokenFile == "" {
		return
	}
	b, err := os.ReadFile(a.TokenFile)
	if err != nil {
		return
	}
	var f tokenFile
	if json.Unmarshal(b, &f) == nil {
		a.token = f.Token
	}
}

func (a *API) saveToken(tok string) {
	a.token = tok
	if a.TokenFile == "" {
		return
	}
	b, _ := json.MarshalIndent(tokenFile{Token: tok, Name: a.pairName()}, "", "  ")
	write := a.Write
	if write == nil {
		write = fileacl.WriteFile
	}
	if err := write(a.TokenFile, append(b, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, "mcp: the token could not be stored:", err) // stderr: stdout is the protocol
	}
}

func (a *API) forgetToken() {
	a.token = ""
	if a.TokenFile != "" {
		os.Remove(a.TokenFile)
	}
}

func (a *API) httpClient() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return defaultHTTP
}

var defaultHTTP = newHTTPClient()

// apiError is the error body of API v1.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// raw does one request. A token is added when there is one.
func (a *API) raw(method, path string, body any, token string) (int, []byte, error) {
	code, _, b, err := a.do(method, path, body, token, nil)
	return code, b, err
}

// do is raw with extra request headers; it also returns the response headers.
func (a *API) do(method, path string, body any, token string, hdr map[string]string) (int, http.Header, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := bodyBytes(body)
		if err != nil {
			return 0, nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.Base+path, rd)
	if err != nil {
		return 0, nil, nil, err
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("CaseHUD is not reachable, is it running? (%w)", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return resp.StatusCode, resp.Header, b, nil
}

func bodyBytes(body any) ([]byte, error) {
	if raw, ok := body.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(body)
}

// ensureToken makes sure a token is there; without one it asks CaseHUD and
// waits for the answer of the user. Needs mu.
func (a *API) ensureToken() error {
	if a.revoked {
		return userError(msgRevoked)
	}
	a.loadToken()
	if a.token != "" {
		return nil
	}
	if a.pairID == "" {
		b, _ := json.Marshal(map[string]any{"name": a.pairName(), "scopes": []string{"control"}, "interface": "mcp"})
		code, resp, err := a.raw("POST", "/api/v1/pair", json.RawMessage(b), "")
		if err != nil {
			return err
		}
		if code != http.StatusAccepted {
			return a.refused(code, resp)
		}
		var r struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(resp, &r) != nil || r.ID == "" {
			return errors.New("CaseHUD answered the access request in a way that is not understood")
		}
		a.pairID = r.ID
	}
	wait, every := a.PairWait, a.PollEvery
	if wait <= 0 {
		wait = 25 * time.Second
	}
	if every <= 0 {
		every = time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		code, resp, err := a.raw("GET", "/api/v1/pair/"+a.pairID, nil, "")
		if err != nil {
			return err
		}
		if code == http.StatusNotFound { // expired, or collected elsewhere
			a.pairID = ""
			return userError(msgConfirm + " (Die Anfrage war abgelaufen. / The request had expired.)")
		}
		if code != http.StatusOK {
			return a.refused(code, resp)
		}
		var r struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}
		if json.Unmarshal(resp, &r) != nil {
			return errors.New("CaseHUD answered the access request in a way that is not understood")
		}
		switch r.Status {
		case "approved":
			a.pairID = ""
			a.saveToken(r.Token)
			return nil
		case "denied":
			a.pairID = ""
			return userError(msgDenied)
		}
		if time.Now().Add(every).After(deadline) {
			return userError(msgConfirm)
		}
		time.Sleep(every)
	}
}

// refused turns an error answer into the text a model is told.
func (a *API) refused(code int, body []byte) error {
	var e apiError
	_ = json.Unmarshal(body, &e)
	switch e.Error.Code {
	case "mcp_disabled":
		return userError(msgMCPOff)
	case "area_off":
		return userError(ErrAreaOff)
	case "revoked":
		return userError(msgRevoked)
	case "not_for_mcp":
		return userError("not_for_mcp: " + e.Error.Message + ". This is only possible in the CaseHUD window. / Das ist nur im CaseHUD-Fenster möglich.")
	case "insufficient_scope":
		if strings.Contains(e.Error.Message, "system right") {
			return userError(msgNoSystem)
		}
		return userError(msgNoControl)
	case "gui_only":
		return userError("gui_only: this change can only be made in the CaseHUD window, not through MCP. Tell the user what to change and where. / Diese Änderung ist nur im CaseHUD-Fenster möglich: dem Benutzer sagen, was er dort ändern soll.")
	case "rate_limited", "too_many_pending":
		return fmt.Errorf("%s: %s. Wait a moment and call the tool again", e.Error.Code, e.Error.Message)
	case "":
		return fmt.Errorf("CaseHUD: %s", strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("CaseHUD: %s: %s", e.Error.Code, e.Error.Message)
}

// call sends a request with the token, getting one first if needed. A token
// CaseHUD no longer knows (revoked) is dropped and asked for again once.
func (a *API) call(method, path string, body any, out any) ([]byte, error) {
	rep, err := a.callFull(method, path, body, nil)
	if err != nil {
		return nil, err
	}
	if out != nil && len(rep.Body) > 0 {
		if err := json.Unmarshal(rep.Body, out); err != nil {
			return nil, err
		}
	}
	return rep.Body, nil
}

// reply is a successful answer of CaseHUD.
type reply struct {
	Code   int
	Header http.Header
	Body   []byte
}

// callFull is call with extra request headers; it returns status and headers
// too. Every non-2xx answer becomes an error that tells what to do.
func (a *API) callFull(method, path string, body any, hdr map[string]string) (*reply, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for attempt := 0; ; attempt++ {
		if err := a.ensureToken(); err != nil {
			return nil, err
		}
		code, h, b, err := a.do(method, path, body, a.token, hdr)
		if err != nil {
			return nil, err
		}
		var e apiError
		if code == http.StatusForbidden && json.Unmarshal(b, &e) == nil && e.Error.Code == "unauthorized" && attempt == 0 {
			a.forgetToken()
			continue
		}
		if code == http.StatusForbidden && e.Error.Code == "revoked" {
			a.forgetToken() // also the file, so the next start asks again
			a.revoked = true
		}
		if code/100 != 2 {
			return nil, a.refused(code, b)
		}
		return &reply{Code: code, Header: h, Body: b}, nil
	}
}
