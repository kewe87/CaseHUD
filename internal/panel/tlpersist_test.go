package panel

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// fanHID answers like a fan: every report is echoed with its op (the
// acknowledgement), the identity answer carries the current serial, and a
// serial write (op 0x3F) is stored unless the fan ignores it.
type fanHID struct {
	scriptHID
	ignoreSerial bool
	ackOp        int // >= 0: acknowledge with this op instead of echoing
	acks         int
	dropSettings int // swallow the answers to this many settings writes
}

func newFanHID(serial []byte) *fanHID {
	return &fanHID{scriptHID: scriptHID{serial: serial}, ackOp: -1}
}

func (f *fanHID) Write(b []byte) (int, error) {
	if tlOp(b[1]) == tlOpSetSerial {
		f.fakeHID.Write(b)
		if !f.ignoreSerial {
			n := int(b[9])<<8 | int(b[10])
			f.serial = tlSerial(string(b[tlHeadSize:tlHeadSize+n]), 0, 1)
		}
		return len(b), nil
	}
	if tlOp(b[1]) == tlOpSettings && f.dropSettings > 0 {
		f.dropSettings--
		f.fakeHID.Write(b)
		return len(b), nil
	}
	if f.ackOp >= 0 && tlOp(b[1]) == tlOpStartImage {
		f.fakeHID.Write(b)
		resp := make([]byte, 64)
		resp[0], resp[1] = tlReport, byte(f.ackOp)
		f.queue = append(f.queue, resp)
		f.acks++
		return len(b), nil
	}
	if tlOp(b[1]) == tlOpStartImage {
		f.acks++
	}
	return f.scriptHID.Write(b)
}

func opsOf(f *fakeHID) []byte {
	var ops []byte
	for _, w := range f.writes {
		ops = append(ops, w[1])
	}
	return ops
}

func TestTLStartImagePacketsAndAckPerReport(t *testing.T) {
	h := newFanHID(tlSerial("S", 0, 0))
	p := &tlLCD{dev: h, brightness: 50}
	if err := p.WriteStartImage(bytes.Repeat([]byte{7}, 1200)); err != nil { // 501 + 501 + 198
		t.Fatal(err)
	}
	// 3 reports of op 0x48, then the normal settings (op 64, live view).
	if !bytes.Equal(opsOf(&h.fakeHID), []byte{0x48, 0x48, 0x48, 61, 64}) {
		t.Fatalf("ops = %v", opsOf(&h.fakeHID))
	}
	wantHead := [][]byte{
		{2, 0x48, 0, 0, 0x04, 0xB0, 0, 0, 0, 0x01, 0xF5},
		{2, 0x48, 0, 0, 0x04, 0xB0, 0, 0, 1, 0x01, 0xF5},
		{2, 0x48, 0, 0, 0x04, 0xB0, 0, 0, 2, 0x00, 0xC6},
	}
	for i, w := range h.writes[:3] {
		if len(w) != 512 || !bytes.Equal(w[:11], wantHead[i]) {
			t.Fatalf("report %d header = % x", i, w[:11])
		}
	}
	if h.writes[4][tlHeadSize] != byte(tlViewLive) {
		t.Fatal("streaming settings not restored")
	}
	if len(h.queue) != 0 {
		t.Fatalf("%d acknowledgements left unread", len(h.queue))
	}
}

func TestTLStartImageAbortsOnMissingOrWrongAck(t *testing.T) {
	// No acknowledgement at all: stop after the first report.
	h := newFanHID(tlSerial("S", 0, 0))
	h.noAnswers = true
	p := &tlLCD{dev: h, brightness: 50}
	if err := p.WriteStartImage(bytes.Repeat([]byte{7}, 1200)); err == nil {
		t.Fatal("missing acknowledgement not reported")
	}
	n := 0
	for _, op := range opsOf(&h.fakeHID) {
		if op == 0x48 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d reports sent after a missing acknowledgement, want 1", n)
	}

	// Wrong op in the acknowledgement: stop as well.
	w := newFanHID(tlSerial("S", 0, 0))
	w.ackOp = int(tlOpStream)
	p = &tlLCD{dev: w, brightness: 50}
	if err := p.WriteStartImage(bytes.Repeat([]byte{7}, 1200)); err == nil || !strings.Contains(err.Error(), "acknowledgement") {
		t.Fatalf("wrong acknowledgement: %v", err)
	}
	if w.acks != 1 {
		t.Fatalf("%d reports sent after a wrong acknowledgement, want 1", w.acks)
	}
	// The panel keeps working afterwards: live settings were restored.
	ops := opsOf(&w.fakeHID)
	if ops[len(ops)-1] != byte(tlOpSettings) {
		t.Fatalf("ops = %v", ops)
	}
}

func TestTLStartImageBounds(t *testing.T) {
	p := &tlLCD{dev: newFanHID(nil)}
	if p.WriteStartImage(nil) == nil || p.WriteStartImage(make([]byte, 65536)) == nil {
		t.Fatal("size limits not enforced")
	}
	p.closed = true
	if err := p.WriteStartImage([]byte{1}); err != ErrClosed {
		t.Fatalf("closed panel: %v", err)
	}
}

func resetSerialTries() {
	tlSerialMu.Lock()
	tlSerialDone = map[string]bool{}
	tlSerialMu.Unlock()
}

func serialWrites(f *fakeHID) [][]byte {
	var out [][]byte
	for _, w := range f.writes {
		if w[1] == byte(tlOpSetSerial) {
			out = append(out, w)
		}
	}
	return out
}

func TestTLSerialWrittenOnlyWhenEmptyAndVerified(t *testing.T) {
	resetSerialTries()
	h := newFanHID(tlSerial("", 1, 2))
	p := &tlLCD{dev: h, path: "dev-a", brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	ws := serialWrites(&h.fakeHID)
	if len(ws) != 1 {
		t.Fatalf("%d serial writes, want 1", len(ws))
	}
	w := ws[0]
	if w[9] != 0 || w[10] != 32 || w[5] != 32 {
		t.Fatalf("header % x", w[:11])
	}
	body := string(w[tlHeadSize : tlHeadSize+32])
	if len(body) != 32 || body != strings.ToLower(body) || body[8] != '-' || body[13] != '-' || body[14] != '4' {
		t.Fatalf("serial %q is not the start of a lower-case UUID", body)
	}
	if p.serial != body || !p.serialNew {
		t.Fatalf("serial %q not adopted", p.serial)
	}
	if p.key != "tllcd-s"+tlSafe(body) {
		t.Fatalf("key %q", p.key)
	}
	// The panel ID does not change because of the new serial.
	assignTLIDs([]*tlLCD{p})
	if p.ID() != "tllcd-p1-i2" {
		t.Fatalf("id %q", p.ID())
	}
	// Order: version, identity, serial, identity (verify), settings.
	if !bytes.Equal(opsOf(&h.fakeHID), []byte{61, 62, 0x3F, 62, 64}) {
		t.Fatalf("ops = %v", opsOf(&h.fakeHID))
	}
}

func TestTLSerialNeverOverwritten(t *testing.T) {
	resetSerialTries()
	for _, s := range []string{"ABC", "x", strings.Repeat("z", 32)} {
		h := newFanHID(tlSerial(s, 0, 0))
		p := &tlLCD{dev: h, path: "dev-b", brightness: 50}
		if err := p.start(); err != nil {
			t.Fatal(err)
		}
		if len(serialWrites(&h.fakeHID)) != 0 || p.serialNew {
			t.Fatalf("serial %q was overwritten", s)
		}
	}
	// A short or missing identity answer is not "empty".
	h := newFanHID(nil)
	p := &tlLCD{dev: h, path: "dev-c", brightness: 50}
	p.start()
	if len(serialWrites(&h.fakeHID)) != 0 {
		t.Fatal("serial written without an identity answer")
	}
	p.assignSerialIfEmpty(make([]byte, 10))
	if len(serialWrites(&h.fakeHID)) != 0 {
		t.Fatal("serial written on a truncated answer")
	}
}

func TestTLSerialNotConfirmedIsNotAdoptedNorRetried(t *testing.T) {
	resetSerialTries()
	h := newFanHID(tlSerial("", 0, 0))
	h.ignoreSerial = true
	p := &tlLCD{dev: h, path: "dev-d", brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	if p.serial != "" || p.serialNew {
		t.Fatal("unconfirmed serial adopted")
	}
	// Same device again in this run (rescan): no second attempt.
	p2 := &tlLCD{dev: h, path: "dev-d", brightness: 50}
	p2.start()
	if n := len(serialWrites(&h.fakeHID)); n != 1 {
		t.Fatalf("%d serial writes, want 1", n)
	}
}

func TestPersistentOpsOnlyThroughTheirFunctions(t *testing.T) {
	resetSerialTries()
	// Everything else the driver does never sends the two persistent ops.
	f := &fakeHID{}
	p := &tlLCD{dev: f, brightness: 50}
	exercise(t, p)
	for _, op := range opsOf(f) {
		if op == byte(tlOpStartImage) || op == byte(tlOpRestImage) || op == byte(tlOpSetSerial) {
			t.Fatalf("op %#x sent outside its dedicated function", op)
		}
	}
	// Through them, exactly these ops appear.
	h := newFanHID(tlSerial("", 0, 0))
	q := &tlLCD{dev: h, path: "dev-e", brightness: 50}
	if err := q.start(); err != nil {
		t.Fatal(err)
	}
	if err := q.WriteRestImage([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if err := q.WriteStartImage([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	allowed := map[byte]bool{62: true, 61: true, 64: true, 0x3F: true, 0x48: true, 0x41: true}
	for _, op := range opsOf(&h.fakeHID) {
		if !allowed[op] {
			t.Errorf("unexpected op %#x", op)
		}
	}
}

func TestTLStartImageSettingsRetriedOnce(t *testing.T) {
	tlImageRetry = time.Millisecond
	img := bytes.Repeat([]byte{7}, 1200)

	h := newFanHID(tlSerial("S", 0, 0))
	h.dropSettings = 1
	p := &tlLCD{dev: h, brightness: 50}
	if err := p.WriteStartImage(img); err != nil {
		t.Fatalf("retry should have succeeded: %v", err)
	}
	if !bytes.Equal(opsOf(&h.fakeHID), []byte{0x48, 0x48, 0x48, 61, 64, 64}) {
		t.Fatalf("ops = %v", opsOf(&h.fakeHID))
	}

	// Both attempts fail: the error says the image is written; it is not sent again.
	h = newFanHID(tlSerial("S", 0, 0))
	h.dropSettings = 2
	p = &tlLCD{dev: h, brightness: 50}
	err := p.WriteStartImage(img)
	if !errors.Is(err, ErrImageWritten) {
		t.Fatalf("got %v", err)
	}
	if !bytes.Equal(opsOf(&h.fakeHID), []byte{0x48, 0x48, 0x48, 61, 64, 64}) {
		t.Fatalf("ops = %v", opsOf(&h.fakeHID))
	}

	// A failed image is not marked as written.
	h = newFanHID(tlSerial("S", 0, 0))
	h.noAnswers = true
	p = &tlLCD{dev: h, brightness: 50}
	if err := p.WriteStartImage(img); err == nil || errors.Is(err, ErrImageWritten) {
		t.Fatalf("got %v", err)
	}
}
