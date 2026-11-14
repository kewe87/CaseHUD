// Package access manages which programs may use the public API: a program asks
// for access (pairing), the user decides in the CaseHUD window, and the program
// gets a token of its own with the scope the user granted. Tokens are only
// stored as SHA-256 hashes; the secret is shown to the program once.
package access

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Scope is what a client may do.
type Scope string

const (
	// Read allows everything that only reads.
	Read Scope = "read"
	// Control also allows the requests that change something.
	Control Scope = "control"
)

// SystemRight is the name of the third right, "system": ending CaseHUD and
// updating it. It is separate from Scope and not implied by Control: a client
// has a Scope and, on top, may have System.
const SystemRight = "system"

// Valid tells a known scope.
func (s Scope) Valid() bool { return s == Read || s == Control }

// Allows tells whether a client with scope s may do what needs scope need.
func (s Scope) Allows(need Scope) bool {
	return s == Control && need.Valid() || s == Read && need == Read
}

// Interface is the door a client came through. The API and MCP switches of the
// settings each close their own.
type Interface string

const (
	API Interface = "api"
	MCP Interface = "mcp"
)

// Valid tells a known interface.
func (i Interface) Valid() bool { return i == API || i == MCP }

// Limits.
const (
	MaxPending   = 3                // open requests at once
	PairInterval = 10 * time.Second // between two requests of one source
	PairAnyEvery = time.Second      // between two requests of any source
	PendingTTL   = 2 * time.Minute  // an open request expires after this
	MaxClients   = 32
	MaxNameRunes = 40
	maxRevoked   = 64              // revoked tokens remembered, to tell them from unknown ones
	flushEvery   = 5 * time.Minute // how often a changed lastUsed is written
)

// Errors of the pairing.
var (
	ErrTooManyPending = errors.New("too many open access requests")
	ErrFull           = errors.New("too many paired programs")
	ErrNotFound       = errors.New("no such request or program")
	ErrBadName        = errors.New("the name is empty")
	ErrBadScope       = errors.New("unknown scope")
	ErrReservedName   = errors.New("this name is reserved for the AI connection")
)

// reservedName tells a name that looks like an MCP connection set up by
// CaseHUD ("MCP (Claude Desktop)"). Only the MCP interface and the AI
// connection of the settings may use it, so a program of the API cannot pass
// for one in the window.
func reservedName(name string) bool {
	n := strings.ToLower(strings.Join(strings.Fields(name), ""))
	return strings.HasPrefix(n, "mcp(")
}

// RateError is returned when requests come too fast.
type RateError struct{ RetryAfter time.Duration }

func (e *RateError) Error() string {
	return fmt.Sprintf("access requests are limited, try again in %d s", int(e.RetryAfter.Seconds())+1)
}

// Client is a paired program, as shown to the user.
type Client struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Scope     Scope     `json:"scope"`
	Interface Interface `json:"interface"`
	// System: the program may also quit and update CaseHUD (off by default).
	System bool `json:"system"`
	// App is the id of the AI app CaseHUD set this token up for ("" = none).
	App string `json:"app,omitempty"`
	// Remember: the token survives a restart of CaseHUD.
	Remember bool      `json:"remember"`
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"lastUsed"`
	// Gone is closed when the program is revoked, so its open streams can end.
	Gone <-chan struct{} `json:"-"`
}

// Request is an open request of a program.
type Request struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Scope     Scope     `json:"scope"` // what the program asks for
	System    bool      `json:"system"`
	Interface Interface `json:"interface"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires"`
}

// Status of a request as the program sees it.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
)

// Result is what a program gets when it asks for its request.
type Result struct {
	Status   string `json:"status"`
	Token    string `json:"token,omitempty"` // only with approved, only once
	Scope    Scope  `json:"scope,omitempty"`
	System   bool   `json:"system,omitempty"`
	Remember bool   `json:"remembered,omitempty"`
}

type entry struct {
	Client
	hash    [32]byte
	flushed time.Time
	gone    chan struct{}
}

type pending struct {
	Request
	decided  bool
	allowed  bool
	scope    Scope
	system   bool
	remember bool
	token    string
}

// WriteFunc stores data in path atomically and readable by the user only.
type WriteFunc func(path string, data []byte) error

// Manager keeps the pending requests and the paired programs.
type Manager struct {
	// Logf logs problems; the default is log.Printf. It never gets a token.
	Logf func(format string, a ...any)

	mu       sync.Mutex
	path     string
	write    WriteFunc
	now      func() time.Time
	clients  map[string]*entry
	byHash   map[[32]byte]*entry
	pending  map[string]*pending
	revoked  [][32]byte           // hashes of revoked tokens, oldest first
	lastPair time.Time            // of any source
	lastBy   map[string]time.Time // per source: interface and name
}

type fileClient struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Scope     Scope     `json:"scope"`
	Interface Interface `json:"interface"`
	System    bool      `json:"system,omitempty"`
	App       string    `json:"app,omitempty"`
	Hash      string    `json:"hash"` // hex SHA-256 of the token
	Created   time.Time `json:"created"`
	LastUsed  time.Time `json:"lastUsed"`
}

type file struct {
	Version int          `json:"version"`
	Clients []fileClient `json:"clients"`
	// Revoked are hashes of tokens the user took away, so a program that still
	// holds one is told so instead of asking for access again.
	Revoked []string `json:"revoked,omitempty"`
}

// Open loads the remembered programs from path ("" keeps everything in memory).
// An unreadable file is reported and ignored: no program is paired then.
func Open(path string, write WriteFunc) (*Manager, error) {
	m := &Manager{Logf: log.Printf, path: path, write: write, now: time.Now,
		clients: map[string]*entry{}, byHash: map[[32]byte]*entry{}, pending: map[string]*pending{}, lastBy: map[string]time.Time{}}
	if path == "" {
		return m, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return m, fmt.Errorf("the list of paired programs is damaged: %w", err)
	}
	for _, h := range f.Revoked {
		if raw, err := hex.DecodeString(h); err == nil && len(raw) == sha256.Size {
			m.revoked = append(m.revoked, [32]byte(raw))
		}
	}
	if len(m.revoked) > maxRevoked {
		m.revoked = m.revoked[len(m.revoked)-maxRevoked:]
	}
	for _, c := range f.Clients {
		raw, err := hex.DecodeString(c.Hash)
		if err != nil || len(raw) != sha256.Size || !c.Scope.Valid() || !c.Interface.Valid() || c.ID == "" {
			continue
		}
		c.System = c.System && c.Scope == Control
		e := &entry{Client: Client{ID: c.ID, Name: cleanName(c.Name), Scope: c.Scope, Interface: c.Interface, System: c.System, App: c.App,
			Remember: true, Created: c.Created, LastUsed: c.LastUsed}, flushed: c.LastUsed, gone: make(chan struct{})}
		e.Gone = e.gone
		copy(e.hash[:], raw)
		m.clients[e.ID] = e
		m.byHash[e.hash] = e
	}
	return m, nil
}

func newSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// cleanName drops control characters, folds blanks and cuts to MaxNameRunes.
func cleanName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r):
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(out); len(r) > MaxNameRunes {
		out = string(r[:MaxNameRunes])
	}
	return strings.TrimSpace(out)
}

// gc drops expired requests. Needs mu.
func (m *Manager) gc() {
	now := m.now()
	for id, p := range m.pending {
		if now.After(p.Expires) {
			delete(m.pending, id)
		}
	}
}

// Pair registers a request of a program. want is the scope it asks for, system
// whether it also asks for the system right.
func (m *Manager) Pair(name string, want Scope, via Interface, system bool) (Request, error) {
	name = cleanName(name)
	switch {
	case name == "":
		return Request{}, ErrBadName
	case !want.Valid() || !via.Valid():
		return Request{}, ErrBadScope
	case via != MCP && reservedName(name):
		return Request{}, ErrReservedName
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	now := m.now()
	// One program asking over and over must not keep the others out: the
	// pause of PairInterval counts per source (its interface and name), a
	// shorter one for all sources together.
	src := string(via) + "\x00" + strings.ToLower(name)
	if wait := m.lastBy[src].Add(PairInterval).Sub(now); wait > 0 && !m.lastBy[src].IsZero() {
		return Request{}, &RateError{RetryAfter: wait}
	}
	if wait := m.lastPair.Add(PairAnyEvery).Sub(now); wait > 0 && !m.lastPair.IsZero() {
		return Request{}, &RateError{RetryAfter: wait}
	}
	open := 0
	for _, p := range m.pending {
		if !p.decided {
			open++
		}
	}
	if open >= MaxPending {
		return Request{}, ErrTooManyPending
	}
	if len(m.clients)+len(m.pending) >= MaxClients {
		return Request{}, ErrFull
	}
	id, err := newSecret(16)
	if err != nil {
		return Request{}, err
	}
	p := &pending{Request: Request{ID: id, Name: name, Scope: want, System: system, Interface: via, Created: now, Expires: now.Add(PendingTTL)}}
	m.pending[id] = p
	m.lastPair = now
	for k, t := range m.lastBy { // forget old sources
		if now.Sub(t) > PairInterval {
			delete(m.lastBy, k)
		}
	}
	m.lastBy[src] = now
	return p.Request, nil
}

// Pending lists the requests nobody decided yet, oldest first.
func (m *Manager) Pending() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	out := []Request{}
	for _, p := range m.pending {
		if !p.decided {
			out = append(out, p.Request)
		}
	}
	slices.SortFunc(out, func(a, b Request) int { return a.Created.Compare(b.Created) })
	return out
}

// Decide answers an open request. A refused request needs no scope; an allowed
// one gets the scope the user chose, which may be lower than the one asked for.
func (m *Manager) Decide(id string, allow bool, scope Scope, system, remember bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	p := m.pending[id]
	if p == nil || p.decided {
		return ErrNotFound
	}
	if allow {
		if !scope.Valid() {
			return ErrBadScope
		}
		tok, err := newSecret(32)
		if err != nil {
			return err
		}
		p.token, p.scope, p.system, p.remember = tok, scope, system && scope == Control, remember
	}
	p.decided, p.allowed = true, allow
	return nil
}

// Poll tells a program how its request stands. An approved request hands out
// the token exactly once: the program is then paired and the secret is gone
// from memory. A refused request is reported once.
func (m *Manager) Poll(id string) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	p := m.pending[id]
	if p == nil {
		return Result{}, ErrNotFound
	}
	switch {
	case !p.decided:
		return Result{Status: StatusPending}, nil
	case !p.allowed:
		delete(m.pending, id)
		return Result{Status: StatusDenied}, nil
	}
	now := m.now()
	e := &entry{Client: Client{ID: id, Name: p.Name, Scope: p.scope, System: p.system, Interface: p.Interface, Remember: p.remember, Created: now}, flushed: now, gone: make(chan struct{})}
	e.Gone = e.gone
	e.hash = sha256.Sum256([]byte(p.token))
	res := Result{Status: StatusApproved, Token: p.token, Scope: p.scope, System: p.system, Remember: p.remember}
	delete(m.pending, id)
	m.clients[id] = e
	m.byHash[e.hash] = e
	m.save()
	return res, nil
}

// Mint pairs a program on behalf of the user without a request: the window
// itself hands the token to the program (an AI app it set up, or a token the
// user copies). The token is returned once; only its hash is kept. system is
// only granted together with Control.
func (m *Manager) Mint(name string, scope Scope, system bool, via Interface, remember bool) (id, token string, err error) {
	return m.MintApp("", name, scope, system, via, remember)
}

// MintApp is Mint for the AI connection of an app: app is its id, by which
// the token is found again (a name can be forged, the id is not).
func (m *Manager) MintApp(app, name string, scope Scope, system bool, via Interface, remember bool) (id, token string, err error) {
	name = cleanName(name)
	switch {
	case name == "":
		return "", "", ErrBadName
	case !scope.Valid() || !via.Valid():
		return "", "", ErrBadScope
	case app == "" && via != MCP && reservedName(name):
		return "", "", ErrReservedName
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	if len(m.clients)+len(m.pending) >= MaxClients {
		return "", "", ErrFull
	}
	if id, err = newSecret(16); err != nil {
		return "", "", err
	}
	if token, err = newSecret(32); err != nil {
		return "", "", err
	}
	now := m.now()
	e := &entry{Client: Client{ID: id, Name: name, Scope: scope, System: system && scope == Control, Interface: via, App: app, Remember: remember, Created: now},
		flushed: now, gone: make(chan struct{})}
	e.Gone = e.gone
	e.hash = sha256.Sum256([]byte(token))
	m.clients[id] = e
	m.byHash[e.hash] = e
	m.save()
	return id, token, nil
}

// Authenticate finds the program a token belongs to.
func (m *Manager) Authenticate(token string) (Client, bool) {
	if token == "" {
		return Client{}, false
	}
	h := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.byHash[h]
	if e == nil {
		return Client{}, false
	}
	now := m.now()
	e.LastUsed = now
	if e.Remember && now.Sub(e.flushed) >= flushEvery {
		m.save()
	}
	return e.Client, true
}

// IsRevoked tells a token the user took away (and that was not seen again as a
// new one).
func (m *Manager) IsRevoked(token string) bool {
	if token == "" {
		return false
	}
	h := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Contains(m.revoked, h)
}

// Waiting lists the requests the user allowed with "remember" whose program has
// not collected its token yet, oldest first. They have the rights that were
// granted. Revoke cancels one.
func (m *Manager) Waiting() []Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	out := []Client{}
	for _, p := range m.pending {
		if p.decided && p.allowed && p.remember {
			out = append(out, Client{ID: p.ID, Name: p.Name, Scope: p.scope, System: p.system, Interface: p.Interface, Remember: true, Created: p.Created})
		}
	}
	slices.SortFunc(out, func(a, b Client) int {
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// List returns the paired programs, oldest first.
func (m *Manager) List() []Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Client, 0, len(m.clients))
	for _, e := range m.clients {
		out = append(out, e.Client)
	}
	slices.SortFunc(out, func(a, b Client) int {
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// SetScope changes what a paired program may do.
func (m *Manager) SetScope(id string, scope Scope) error {
	if !scope.Valid() {
		return ErrBadScope
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.clients[id]
	if e == nil {
		return ErrNotFound
	}
	e.Scope = scope
	if scope != Control {
		e.System = false // the system right needs control
	}
	m.save()
	return nil
}

// SetSystem grants or takes the system right.
func (m *Manager) SetSystem(id string, system bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.clients[id]
	if e == nil {
		return ErrNotFound
	}
	e.System = system && e.Scope == Control // the system right needs control
	m.save()
	return nil
}

// Revoke ends the access of a program at once.
func (m *Manager) Revoke(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.clients[id]
	if e == nil {
		// An allowed request whose token was not collected: cancel it.
		if p := m.pending[id]; p != nil && p.decided && p.allowed {
			h := sha256.Sum256([]byte(p.token))
			delete(m.pending, id)
			m.noteRevoked(h)
			return nil
		}
		return ErrNotFound
	}
	delete(m.clients, id)
	delete(m.byHash, e.hash)
	close(e.gone)
	m.noteRevoked(e.hash)
	return nil
}

// noteRevoked remembers a revoked token and saves. Needs mu.
func (m *Manager) noteRevoked(h [32]byte) {
	m.revoked = append(m.revoked, h)
	if len(m.revoked) > maxRevoked {
		m.revoked = m.revoked[len(m.revoked)-maxRevoked:]
	}
	m.save()
}

// SetClock replaces the clock; tests of the program using the manager move time with it.
func (m *Manager) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// Flush writes pending changes of lastUsed (call it when the app ends).
func (m *Manager) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.save()
}

// save writes the remembered programs. Needs mu.
func (m *Manager) save() {
	if m.path == "" || m.write == nil {
		return
	}
	f := file{Version: 1, Clients: []fileClient{}}
	var keep []*entry
	for _, e := range m.clients {
		if e.Remember {
			keep = append(keep, e)
		}
	}
	slices.SortFunc(keep, func(a, b *entry) int { return strings.Compare(a.ID, b.ID) })
	now := m.now()
	for _, e := range keep {
		e.flushed = now
		f.Clients = append(f.Clients, fileClient{ID: e.ID, Name: e.Name, Scope: e.Scope, Interface: e.Interface, System: e.System, App: e.App,
			Hash: hex.EncodeToString(e.hash[:]), Created: e.Created, LastUsed: e.LastUsed})
	}
	for _, h := range m.revoked {
		f.Revoked = append(f.Revoked, hex.EncodeToString(h[:]))
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err == nil {
		err = m.write(m.path, append(b, '\n'))
	}
	if err != nil && m.Logf != nil {
		m.Logf("access: the list of paired programs could not be saved: %v", err)
	}
}
