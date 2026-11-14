package access

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func writeFile(path string, b []byte) error { return os.WriteFile(path, b, 0o600) }

func newTest(t *testing.T, path string) (*Manager, *clock) {
	t.Helper()
	m, err := Open(path, writeFile)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	m.now = c.now
	m.Logf = t.Logf
	return m, c
}

func TestScopes(t *testing.T) {
	if !Control.Allows(Read) || !Control.Allows(Control) || !Read.Allows(Read) || Read.Allows(Control) {
		t.Fatal("scope rules")
	}
	if Scope("").Allows(Read) || Scope("x").Allows(Read) || Control.Allows("x") {
		t.Fatal("unknown scopes must allow nothing")
	}
}

func TestPairApproveOnce(t *testing.T) {
	m, _ := newTest(t, "")
	req, err := m.Pair("  Couch\x00 \n HUD  ", Control, API, false)
	if err != nil {
		t.Fatal(err)
	}
	if req.Name != "Couch HUD" || req.Scope != Control {
		t.Fatalf("request: %+v", req)
	}
	if r, _ := m.Poll(req.ID); r.Status != StatusPending || r.Token != "" {
		t.Fatalf("poll before decision: %+v", r)
	}
	if len(m.Pending()) != 1 {
		t.Fatal("pending list")
	}
	if err := m.Decide(req.ID, true, Read, false, false); err != nil {
		t.Fatal(err)
	}
	if len(m.Pending()) != 0 {
		t.Fatal("decided request still pending")
	}
	r, err := m.Poll(req.ID)
	if err != nil || r.Status != StatusApproved || len(r.Token) != 64 || r.Scope != Read || r.Remember {
		t.Fatalf("approved: %+v %v", r, err)
	}
	if _, err := m.Poll(req.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second poll must not show the token again: %v", err)
	}
	c, ok := m.Authenticate(r.Token)
	if !ok || c.Name != "Couch HUD" || c.Scope != Read || c.Interface != API {
		t.Fatalf("authenticate: %+v %v", c, ok)
	}
	for _, bad := range []string{"", "x", r.Token + "0", strings.ToUpper(r.Token)} {
		if _, ok := m.Authenticate(bad); ok {
			t.Fatalf("token %q accepted", bad)
		}
	}
}

func TestDeny(t *testing.T) {
	m, c := newTest(t, "")
	req, _ := m.Pair("x", Read, API, false)
	if err := m.Decide(req.ID, false, "", false, false); err != nil {
		t.Fatal(err)
	}
	if r, _ := m.Poll(req.ID); r.Status != StatusDenied || r.Token != "" {
		t.Fatalf("%+v", r)
	}
	if err := m.Decide(req.ID, true, Read, false, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("a decided request cannot be decided again")
	}
	if err := m.Decide("nope", true, Read, false, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown id")
	}
	c.t = c.t.Add(PairInterval)
	req, _ = m.Pair("y", Read, API, false)
	if err := m.Decide(req.ID, true, "root", false, false); !errors.Is(err, ErrBadScope) {
		t.Fatalf("bad scope: %v", err)
	}
}

func TestLimits(t *testing.T) {
	m, c := newTest(t, "")
	if _, err := m.Pair("", Read, API, false); !errors.Is(err, ErrBadName) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := m.Pair("a", "admin", API, false); !errors.Is(err, ErrBadScope) {
		t.Fatalf("scope: %v", err)
	}
	if _, err := m.Pair("a", Read, "x", false); !errors.Is(err, ErrBadScope) {
		t.Fatalf("interface: %v", err)
	}
	long := strings.Repeat("ä", 500)
	r, err := m.Pair(long, Read, API, false)
	if err != nil || len([]rune(r.Name)) != MaxNameRunes {
		t.Fatalf("name cap: %v %d", err, len([]rune(r.Name)))
	}
	var rate *RateError
	if _, err := m.Pair("b", Read, API, false); !errors.As(err, &rate) {
		t.Fatalf("second request at once must be rate limited: %v", err)
	}
	for i := 0; i < MaxPending-1; i++ {
		c.t = c.t.Add(PairInterval)
		if _, err := m.Pair("p", Read, API, false); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	c.t = c.t.Add(PairInterval)
	if _, err := m.Pair("p", Read, API, false); !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("too many pending: %v", err)
	}
	// Requests expire after two minutes and make room.
	c.t = c.t.Add(PendingTTL + time.Second)
	if len(m.Pending()) != 0 {
		t.Fatal("expired requests still listed")
	}
	if _, err := m.Pair("p", Read, API, false); err != nil {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestExpiredApprovalIsLost(t *testing.T) {
	m, c := newTest(t, "")
	req, _ := m.Pair("x", Read, API, false)
	if err := m.Decide(req.ID, true, Read, false, true); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(PendingTTL + time.Second)
	if _, err := m.Poll(req.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a token nobody collected in time must be gone: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatal("client registered without collecting the token")
	}
}

func pairAs(t *testing.T, m *Manager, c *clock, name string, scope Scope, remember bool) (string, string) {
	t.Helper()
	c.t = c.t.Add(PairInterval)
	req, err := m.Pair(name, scope, MCP, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Decide(req.ID, true, scope, false, remember); err != nil {
		t.Fatal(err)
	}
	r, err := m.Poll(req.ID)
	if err != nil {
		t.Fatal(err)
	}
	return req.ID, r.Token
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	m, c := newTest(t, path)
	_, kept := pairAs(t, m, c, "kept", Control, true)
	_, temp := pairAs(t, m, c, "temp", Read, false)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), kept) || strings.Contains(string(b), temp) || strings.Contains(string(b), "temp") {
		t.Fatalf("file holds a secret or a client that was not remembered:\n%s", b)
	}
	if !strings.Contains(string(b), `"hash"`) {
		t.Fatalf("no hash in file:\n%s", b)
	}

	m2, err := Open(path, writeFile)
	if err != nil {
		t.Fatal(err)
	}
	if cl, ok := m2.Authenticate(kept); !ok || cl.Name != "kept" || cl.Scope != Control || cl.Interface != MCP || !cl.Remember {
		t.Fatalf("remembered client after restart: %+v %v", cl, ok)
	}
	if _, ok := m2.Authenticate(temp); ok {
		t.Fatal("a token that was not remembered must end with the program")
	}
}

func TestScopeChangeAndRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	m, c := newTest(t, path)
	id, tok := pairAs(t, m, c, "a", Read, true)
	if err := m.SetScope(id, Control); err != nil {
		t.Fatal(err)
	}
	if cl, _ := m.Authenticate(tok); cl.Scope != Control {
		t.Fatal("scope change not applied")
	}
	if err := m.SetScope(id, "x"); !errors.Is(err, ErrBadScope) {
		t.Fatal(err)
	}
	if err := m.SetScope("nope", Read); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	m2, _ := Open(path, writeFile)
	if cl, _ := m2.Authenticate(tok); cl.Scope != Control {
		t.Fatal("scope change not stored")
	}
	if err := m.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Authenticate(tok); ok {
		t.Fatal("revoked token still works")
	}
	m3, _ := Open(path, writeFile)
	if _, ok := m3.Authenticate(tok); ok {
		t.Fatal("revoked token works after restart")
	}
	if err := m.Revoke(id); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestLastUsedIsWrittenRarely(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	writes := 0
	m, err := Open(path, func(p string, b []byte) error { writes++; return writeFile(p, b) })
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	m.now = c.now
	_, tok := pairAs(t, m, c, "a", Read, true)
	before := writes
	for i := 0; i < 50; i++ {
		c.t = c.t.Add(time.Second)
		m.Authenticate(tok)
	}
	if writes != before {
		t.Fatalf("%d writes within a minute", writes-before)
	}
	c.t = c.t.Add(flushEvery)
	m.Authenticate(tok)
	if writes != before+1 {
		t.Fatalf("lastUsed not flushed after %v", flushEvery)
	}
	m2, _ := Open(path, writeFile)
	if cl := m2.List(); len(cl) != 1 || cl[0].LastUsed.IsZero() {
		t.Fatalf("%+v", cl)
	}
}

func TestDamagedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	os.WriteFile(path, []byte("{nope"), 0o600)
	m, err := Open(path, writeFile)
	if err == nil || m == nil || len(m.List()) != 0 {
		t.Fatalf("damaged file: %v", err)
	}
	// Entries with a bad hash or scope are skipped.
	os.WriteFile(path, []byte(`{"version":1,"clients":[{"id":"a","name":"x","scope":"root","interface":"api","hash":"00"}]}`), 0o600)
	m, _ = Open(path, writeFile)
	if len(m.List()) != 0 {
		t.Fatal("invalid entry loaded")
	}
}

func TestClientCap(t *testing.T) {
	m, c := newTest(t, "")
	for i := 0; i < MaxClients; i++ {
		pairAs(t, m, c, "c", Read, false)
	}
	c.t = c.t.Add(PairInterval)
	if _, err := m.Pair("one more", Read, API, false); !errors.Is(err, ErrFull) {
		t.Fatalf("%v", err)
	}
}

func TestSystemRightIsSeparateAndStored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	m, c := newTest(t, path)
	req, err := m.Pair("sys", Control, MCP, true)
	if err != nil || !req.System {
		t.Fatalf("%+v %v", req, err)
	}
	if err := m.Decide(req.ID, true, Control, false, true); err != nil { // the user may grant less
		t.Fatal(err)
	}
	r, _ := m.Poll(req.ID)
	if r.System {
		t.Fatal("the user did not grant system")
	}
	cl, _ := m.Authenticate(r.Token)
	if cl.System {
		t.Fatal("system must be off unless granted")
	}
	if err := m.SetSystem(cl.ID, true); err != nil {
		t.Fatal(err)
	}
	if cl, _ = m.Authenticate(r.Token); !cl.System || cl.Scope != Control {
		t.Fatalf("%+v", cl)
	}
	m2, _ := Open(path, writeFile)
	if cl, ok := m2.Authenticate(r.Token); !ok || !cl.System {
		t.Fatal("system not stored")
	}
	if err := m.SetSystem("nope", true); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// An entry written before the right existed has no system.
	_ = c
	id, tok := pairAs(t, m, c, "old", Control, true)
	_ = id
	if cl, _ := m.Authenticate(tok); cl.System {
		t.Fatal("control must not imply system")
	}
}

func TestFileWithoutSystemFieldMeansNoSystem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	tok := "0123456789abcdef"
	h := sha256.Sum256([]byte(tok))
	data := `{"version":1,"clients":[{"id":"a","name":"old","scope":"control","interface":"api","hash":"` + hex.EncodeToString(h[:]) + `","created":"2026-01-01T00:00:00Z","lastUsed":"2026-01-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Open(path, writeFile)
	if err != nil {
		t.Fatal(err)
	}
	cl, ok := m.Authenticate(tok)
	if !ok || cl.Scope != Control || cl.System {
		t.Fatalf("%+v %v", cl, ok)
	}
}

func TestRateLimitIsPerSource(t *testing.T) {
	m, c := newTest(t, "")
	if _, err := m.Pair("one", Read, API, false); err != nil {
		t.Fatal(err)
	}
	var rate *RateError
	c.t = c.t.Add(PairAnyEvery)
	if _, err := m.Pair("one", Read, API, false); !errors.As(err, &rate) {
		t.Fatalf("the same source must wait: %v", err)
	}
	if _, err := m.Pair("two", Read, API, false); err != nil {
		t.Fatalf("another source must not wait for it: %v", err)
	}
	if _, err := m.Pair("three", Read, API, false); !errors.As(err, &rate) {
		t.Fatalf("all sources together are still limited: %v", err)
	}
}

func TestSystemNeedsControl(t *testing.T) {
	m, c := newTest(t, "")
	r, _ := m.Pair("a", Control, API, true)
	if err := m.Decide(r.ID, true, Read, true, false); err != nil {
		t.Fatal(err)
	}
	res, _ := m.Poll(r.ID)
	if res.System {
		t.Fatal("read with system must not keep system")
	}
	id, _, err := m.Mint("b", Read, true, API, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetSystem(id, true); err != nil {
		t.Fatal(err)
	}
	for _, cl := range m.List() {
		if cl.System {
			t.Fatalf("%s has system with read", cl.Name)
		}
	}
	id2, _ := pairAs(t, m, c, "c", Control, false)
	_ = m.SetSystem(id2, true)
	_ = m.SetScope(id2, Read)
	for _, cl := range m.List() {
		if cl.ID == id2 && cl.System {
			t.Fatal("lowering to read must take system away")
		}
	}
}

func TestReservedName(t *testing.T) {
	m, c := newTest(t, "")
	for _, n := range []string{"MCP (Claude Desktop)", "mcp(x)", "  MCP  (x)"} {
		if _, err := m.Pair(n, Read, API, false); !errors.Is(err, ErrReservedName) {
			t.Errorf("%q over the API: %v", n, err)
		}
		if _, _, err := m.Mint(n, Read, false, API, false); !errors.Is(err, ErrReservedName) {
			t.Errorf("%q minted for the API: %v", n, err)
		}
	}
	if _, err := m.Pair("MCP (x)", Read, MCP, false); err != nil {
		t.Errorf("over MCP: %v", err)
	}
	c.t = c.t.Add(PairInterval)
	if _, err := m.Pair("MCPlayer", Read, API, false); err != nil {
		t.Errorf("a name that only starts with MCP: %v", err)
	}
	if _, _, err := m.MintApp("cursor", "MCP (Cursor)", Control, false, API, true); err != nil {
		t.Errorf("AI connection: %v", err)
	}
}

func TestRevokedTokenIsRecognised(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	m, c := newTest(t, path)
	id, tok := pairAs(t, m, c, "a", Read, true)
	if m.IsRevoked(tok) {
		t.Fatal("revoked before revoking")
	}
	if err := m.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if !m.IsRevoked(tok) || m.IsRevoked("other") || m.IsRevoked("") {
		t.Fatal("revoked token not recognised")
	}
	m2, _ := Open(path, writeFile)
	if !m2.IsRevoked(tok) {
		t.Fatal("revocation lost at restart")
	}
}

func TestWaitingRequestsAreListedAndCancelled(t *testing.T) {
	m, _ := newTest(t, "")
	r, _ := m.Pair("kept", Read, API, false)
	_ = m.Decide(r.ID, true, Control, false, true)
	c2 := &clock{t: m.now().Add(PairInterval)}
	m.now = c2.now
	r2, _ := m.Pair("once", Read, API, false)
	_ = m.Decide(r2.ID, true, Read, false, false)
	w := m.Waiting()
	if len(w) != 1 || w[0].ID != r.ID || w[0].Scope != Control || w[0].Name != "kept" {
		t.Fatalf("waiting: %+v", w)
	}
	if err := m.Revoke(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Poll(r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cancelled request still there: %v", err)
	}
	if len(m.Waiting()) != 0 {
		t.Fatal("still waiting")
	}
}

func TestAppIDIsStored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.json")
	m, _ := newTest(t, path)
	if _, _, err := m.MintApp("cursor", "MCP (Cursor)", Control, false, MCP, true); err != nil {
		t.Fatal(err)
	}
	m2, _ := Open(path, writeFile)
	l := m2.List()
	if len(l) != 1 || l[0].App != "cursor" {
		t.Fatalf("%+v", l)
	}
}
