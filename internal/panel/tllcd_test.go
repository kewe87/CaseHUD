package panel

import (
	"bytes"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

// Expected bytes are written out by hand from the documented layout:
// 02 | op | total u32 BE | seq u24 BE | length u16 BE | body.
func TestTLFrameLayout(t *testing.T) {
	// total 70000 = 0x00011170, seq 0x000203 (515), 300 = 0x012C body bytes.
	body := bytes.Repeat([]byte{0x5a}, 300)
	f := tlFrame(tlOpStream, 70000, 0x203, body)
	want := []byte{0x02, 0x46, 0x00, 0x01, 0x11, 0x70, 0x00, 0x02, 0x03, 0x01, 0x2C}
	if !bytes.Equal(f[:11], want) {
		t.Fatalf("header = % x, want % x", f[:11], want)
	}
	if len(f) != 512 || !bytes.Equal(f[11:311], body) || !bytes.Equal(f[311:], make([]byte, 201)) {
		t.Fatal("body or padding wrong")
	}
	// A body longer than one frame is cut to 501 bytes.
	if g := tlFrame(tlOpStream, 600, 0, bytes.Repeat([]byte{1}, 600)); g[9] != 0x01 || g[10] != 0xF5 {
		t.Fatalf("length = % x", g[9:11])
	}
}

func TestTLSettingsBody(t *testing.T) {
	got := tlSettings{View: tlViewLive, Brightness: 80, Rate: 25, Turns: 3}.bytes()
	want := []byte{5, 0, 0, 0, 0x50, 0x19, 3, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("body = % x, want % x", got, want)
	}
	f := tlFrame(tlOpSettings, len(got), 0, got)
	if !bytes.Equal(f[:11], []byte{2, 0x40, 0, 0, 0, 0x0B, 0, 0, 0, 0, 0x0B}) || !bytes.Equal(f[11:22], want) {
		t.Fatalf("frame = % x", f[:22])
	}
}

func TestTLBodyOfAnswer(t *testing.T) {
	ans := append([]byte{2, 61, 0, 0, 0, 3, 0, 0, 0, 0, 3}, 'a', 'b', 'c', 'd', 'e')
	if got := tlBody(ans); string(got) != "abc" {
		t.Fatalf("body %q", got)
	}
	if tlBody(ans[:5]) != nil {
		t.Fatal("short report not rejected")
	}
	// Announced length beyond what arrived is bounded.
	if got := tlBody([]byte{2, 61, 0, 0, 0, 0, 0, 0, 0, 0, 9, 'x'}); string(got) != "x" {
		t.Fatalf("body %q", got)
	}
}

type fakeHID struct{ writes [][]byte }

func (f *fakeHID) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte{}, b...))
	return len(b), nil
}
func (f *fakeHID) Read(b []byte, _ int) (int, error) { return 0, nil }
func (f *fakeHID) Close() error                      { return nil }

func TestTLShowJPEGChunks(t *testing.T) {
	f := &fakeHID{}
	p := &tlLCD{dev: f}
	if err := p.ShowJPEG(bytes.Repeat([]byte{1}, 1200)); err != nil { // 501 + 501 + 198
		t.Fatal(err)
	}
	if len(f.writes) != 3 {
		t.Fatalf("%d frames, want 3", len(f.writes))
	}
	wantHead := [][]byte{
		{2, 0x46, 0, 0, 0x04, 0xB0, 0, 0, 0, 0x01, 0xF5},
		{2, 0x46, 0, 0, 0x04, 0xB0, 0, 0, 1, 0x01, 0xF5},
		{2, 0x46, 0, 0, 0x04, 0xB0, 0, 0, 2, 0x00, 0xC6},
	}
	for i, w := range f.writes {
		if !bytes.Equal(w[:11], wantHead[i]) {
			t.Fatalf("frame %d header = % x", i, w[:11])
		}
	}
	if p.ShowJPEG(nil) == nil || p.ShowJPEG(make([]byte, 65536)) == nil {
		t.Fatal("size limits not enforced")
	}
}

// scriptHID answers requests the way a fan does: every request is echoed in
// the answer; serial controls the identity answer.
type scriptHID struct {
	fakeHID
	queue     [][]byte
	serial    []byte // nil: identity request unanswered
	stale     bool   // queue a stale report before each answer
	noAnswers bool
}

func (s *scriptHID) Write(b []byte) (int, error) {
	s.fakeHID.Write(b)
	if s.noAnswers {
		return len(b), nil
	}
	op := b[1]
	resp := make([]byte, 64)
	resp[0], resp[1] = tlReport, op
	switch tlOp(op) {
	case tlOpIdentity:
		if s.serial == nil {
			return len(b), nil
		}
		copy(resp[tlHeadSize:], s.serial)
		resp[10] = byte(len(s.serial))
	case tlOpVersion:
		copy(resp[tlHeadSize:], "v1.0")
		resp[10] = 4
		date := make([]byte, 64)
		date[0], date[1], date[10] = tlReport, op, 10
		copy(date[tlHeadSize:], "2024-05-06")
		s.queue = append(s.queue, append([]byte(nil), resp...), date)
		return len(b), nil
	case tlOpStream:
		return len(b), nil
	}
	if s.stale {
		old := make([]byte, 64)
		old[0], old[1] = tlReport, 99
		s.queue = append(s.queue, old)
	}
	s.queue = append(s.queue, resp)
	return len(b), nil
}

func (s *scriptHID) Read(b []byte, _ int) (int, error) {
	if len(s.queue) == 0 {
		return 0, hid.ErrTimeout
	}
	n := copy(b, s.queue[0])
	s.queue = s.queue[1:]
	return n, nil
}

func tlSerial(serial string, port, slot byte) []byte {
	b := make([]byte, 34)
	copy(b, serial)
	b[32], b[33] = port, slot
	return b
}

func TestTLStartReadsIdentityAndValidatesAnswers(t *testing.T) {
	h := &scriptHID{serial: tlSerial("ABC123", 2, 1), stale: true}
	h.queue = [][]byte{make([]byte, 64)} // queued before start: must be flushed
	p := &tlLCD{dev: h, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	if p.serial != "ABC123" || p.port != 2 || p.slot != 1 || !p.posKnown || p.version != "v1.0" {
		t.Fatalf("identity = %+v", p)
	}
	if got := p.Version(); got != "v1.0 (2024-05-06)" {
		t.Fatalf("Version() = %q", got)
	}
	// Order on the wire: version (frees an open transfer), identity, settings.
	var ops []byte
	for _, w := range h.writes {
		ops = append(ops, w[1])
	}
	if !bytes.Equal(ops, []byte{61, 62, 64}) {
		t.Fatalf("ops = %v", ops)
	}
	assignTLIDs([]*tlLCD{p})
	if p.ID() != "tllcd-p2-i1" || p.Name() != "UNI FAN TL LCD (port 3, fan 2)" {
		t.Fatalf("id %q name %q", p.ID(), p.Name())
	}
	// Stale reports (operation 99) never count as the answer.
	if _, err := p.request(tlOpSettings, make([]byte, 11), 10); err != nil {
		t.Fatal(err)
	}
	h.noAnswers = true
	if _, err := p.request(tlOpSettings, make([]byte, 11), 10); err == nil {
		t.Fatal("missing answer not reported")
	}
	if p.MaxFPS() != 25 {
		t.Fatalf("fps = %d", p.MaxFPS())
	}
}

func TestTLStartToleratesMissingIdentity(t *testing.T) {
	h := &scriptHID{} // identity unanswered, the rest works
	p := &tlLCD{dev: h, path: "p1", brightness: 50}
	if err := p.start(); err != nil {
		t.Fatalf("start failed without identity: %v", err)
	}
	if p.posKnown {
		t.Fatal("position trusted without an identity answer")
	}
	// A fan that answers nothing at all is not a panel.
	if err := (&tlLCD{dev: &scriptHID{noAnswers: true}}).start(); err == nil {
		t.Fatal("silent device accepted")
	}
}

func TestTLUniqueIDs(t *testing.T) {
	mk := func(path, serial string, port, slot int, ok bool) *tlLCD {
		return &tlLCD{path: path, serial: serial, port: port, slot: slot, posKnown: ok}
	}
	a, b := mk("a", "S1", 0, 0, true), mk("b", "S2", 0, 1, true)
	assignTLIDs([]*tlLCD{a, b})
	if a.ID() != "tllcd-p0-i0" || b.ID() != "tllcd-p0-i1" {
		t.Fatalf("ids %q %q", a.ID(), b.ID())
	}
	// Two fans claim one position, a third is fine, a fourth out of range, a fifth unreadable.
	d1, d2 := mk("d1", "S1", 1, 0, true), mk("d2", "S2", 1, 0, true)
	ok := mk("ok", "S3", 3, 2, true)
	bad := mk("bad", "S4", 9, 0, false)
	none := mk("none", "", 0, 0, false)
	all := []*tlLCD{d1, d2, ok, bad, none}
	assignTLIDs(all)
	seen := map[string]bool{}
	for _, p := range all {
		if seen[p.ID()] {
			t.Fatalf("duplicate id %q", p.ID())
		}
		seen[p.ID()] = true
	}
	if ok.ID() != "tllcd-p3-i2" {
		t.Fatalf("valid unique fan lost its id: %q", ok.ID())
	}
	if d1.ID() != "tllcd-sS1" || d2.ID() != "tllcd-sS2" {
		t.Fatalf("serial ids %q %q", d1.ID(), d2.ID())
	}
	// Same serial and position: the USB path separates them, deterministically.
	e1, e2 := mk("pathA", "X", 0, 0, true), mk("pathB", "X", 0, 0, true)
	assignTLIDs([]*tlLCD{e1, e2})
	if e1.ID() == e2.ID() {
		t.Fatalf("identical ids %q", e1.ID())
	}
	e3 := mk("pathA", "X", 0, 0, true)
	assignTLIDs([]*tlLCD{e3, mk("pathB", "X", 0, 0, true)})
	if e3.ID() != e1.ID() {
		t.Fatal("ids not stable")
	}
}

func TestTLCloseShowsStoredImage(t *testing.T) {
	f := &fakeHID{}
	p := &tlLCD{dev: f, brightness: 80, rotation: 90}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(f.writes))
	}
	w := f.writes[0]
	// view 1, brightness 80, rate, the current rotation (90 = one quarter turn)
	want := append([]byte{2, 0x40, 0, 0, 0, 0x0B, 0, 0, 0, 0, 0x0B, 1, 0, 0, 0, 80, tlFrameRate, 1}, make([]byte, 4)...)
	if !bytes.Equal(w[:22], want) {
		t.Fatalf("report = % x", w[:22])
	}
	// Unknown/zero brightness falls back to 50 so the stored image is not dark.
	f0 := &fakeHID{}
	(&tlLCD{dev: f0}).Close()
	if len(f0.writes) != 1 || f0.writes[0][11+4] != 50 || f0.writes[0][11+6] != 0 {
		t.Fatalf("default close report = % x", f0.writes[0][:22])
	}
	// Nothing is written afterwards, and a second Close is a no-op.
	if err := p.ShowJPEG([]byte{1, 2, 3}); err == nil {
		t.Fatal("ShowJPEG after Close succeeded")
	}
	p.SetBrightness(50)
	p.Close()
	if len(f.writes) != 1 {
		t.Fatalf("writes after Close = %d", len(f.writes))
	}
}

func TestTLPositionAndPhysicalPort(t *testing.T) {
	if port, idx, ok := TLPosition("tllcd-p2-i1"); !ok || port != 2 || idx != 1 || TLPhysicalPort(port) != 3 {
		t.Fatalf("got %d %d %v", port, idx, ok)
	}
	for _, id := range []string{"tllcd-sABC", "tllcd-p1", "tllcd-p1-i0-ff", "aio-1", "tllcd-p-1-i0", "tllcd-p01-i0"} {
		if _, _, ok := TLPosition(id); ok {
			t.Errorf("%q parsed", id)
		}
	}
}

// echoHID answers every write with the same report, like a fan that
// acknowledges a command, and counts the settings writes.
type echoHID struct {
	fakeHID
	pending []byte
}

func (f *echoHID) Write(b []byte) (int, error) {
	f.pending = append([]byte{}, b...)
	return f.fakeHID.Write(b)
}
func (f *echoHID) Read(b []byte, _ int) (int, error) {
	if f.pending == nil {
		return 0, nil
	}
	n := copy(b, f.pending)
	f.pending = nil
	return n, nil
}

// settingsWrites returns the settings reports (op 64) written so far.
func (f *echoHID) settingsWrites() [][]byte {
	var out [][]byte
	for _, w := range f.writes {
		if tlOp(w[1]) == tlOpSettings {
			out = append(out, w)
		}
	}
	return out
}

func TestTLSettingsWearProtection(t *testing.T) {
	cache := NewSettingsCache(nil)
	SetSettingsCache(cache)
	defer SetSettingsCache(nil)

	// First start, nothing known: the settings are written once.
	f := &echoHID{}
	p := &tlLCD{dev: f, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	if n := len(f.settingsWrites()); n != 1 {
		t.Fatalf("first start wrote settings %d times, want 1", n)
	}
	// Slider-like rapid changes: equal values never reach the device.
	tlMinInterval = 30 * time.Millisecond
	defer func() { tlMinInterval = 2 * time.Second }()
	for i := 0; i < 20; i++ {
		p.SetBrightness(70)
		p.SetRotation(90)
	}
	time.Sleep(150 * time.Millisecond) // the coalesced rotation is applied
	tlChain.enter()
	n := len(f.settingsWrites())
	tlChain.leave()
	if n != 3 { // start + brightness 70 + rotation 90
		t.Fatalf("after repeated identical requests: %d settings writes, want 3", n)
	}
	key := p.SettingsKey()
	if w := cache.Get(key); w.Brightness == nil || *w.Brightness != 70 || w.Rotation == nil || *w.Rotation != 90 {
		t.Fatalf("cache = %+v", w)
	}
	p.Close()
	// Close keeps rotation and brightness (the view is the only change).
	ws := f.settingsWrites()
	last := ws[len(ws)-1]
	if last[11] != byte(tlViewStored) || last[11+4] != 70 || last[11+6] != 1 {
		t.Fatalf("close report = % x", last[:22])
	}

	// Restart (also a rescan): the fan starts from the recorded values, and a
	// request for the same values writes nothing more.
	f2 := &echoHID{}
	p2 := &tlLCD{dev: f2, brightness: 50}
	if err := p2.start(); err != nil {
		t.Fatal(err)
	}
	ws = f2.settingsWrites()
	if len(ws) != 1 || ws[0][11+4] != 70 || ws[0][11+6] != 1 {
		t.Fatalf("restart settings = %v", ws)
	}
	p2.SetBrightness(70)
	p2.SetRotation(90)
	if n := len(f2.settingsWrites()); n != 1 {
		t.Fatalf("same values after restart wrote %d times", n)
	}
	p2.SetBrightness(40)
	if n := len(f2.settingsWrites()); n != 2 {
		t.Fatalf("changed brightness: %d settings writes, want 2", n)
	}
}
