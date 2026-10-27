package panel

import (
	"bytes"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

// lockHID simulates a TL LCD fan with its transfer lock: an upload (op 0x48)
// occupies the slot, and while it is occupied the fan ignores every command
// (no answer, stream frames go nowhere) until the version query (op 61).
type lockHID struct {
	fakeHID
	locked    bool
	live      bool // settings selected the live view
	streaming bool // a stream frame arrived in the live view
	dropAck   int  // 0x48 reports acknowledged before the acks stop (0: all)
	refuse    bool // settings are never accepted
	noImage   bool // no stored picture: "stored" view is answered with status 0x0C
	deaf      bool // status queries are not answered
	got48     int
	queue     [][]byte
}

func (l *lockHID) reply(op byte, body ...byte) {
	r := make([]byte, 64)
	r[0], r[1] = tlReport, op
	copy(r[tlHeadSize:], body)
	r[10] = byte(len(body))
	l.queue = append(l.queue, r)
}

func (l *lockHID) Write(b []byte) (int, error) {
	l.fakeHID.Write(b)
	switch tlOp(b[1]) {
	case tlOpVersion:
		l.locked = false
		l.reply(61, 'v', '1')
		l.reply(61, 'd')
	case tlOpStartImage, tlOpRestImage:
		l.locked = true
		l.got48++
		if l.dropAck == 0 || l.got48 <= l.dropAck {
			l.reply(b[1])
		}
	case tlOpSettings:
		if l.refuse {
			break
		}
		if !l.locked {
			l.live = b[tlHeadSize] == byte(tlViewLive)
			l.streaming = l.streaming && l.live
			if b[tlHeadSize] == byte(tlViewStored) && l.noImage {
				l.reply(byte(tlOpSettings), tlStatusNoImage)
			} else {
				l.reply(byte(tlOpSettings))
			}
		}
	case tlOpStream:
		if !l.locked && l.live {
			l.streaming = true
		}
	case tlOpStatus:
		if l.deaf {
			break
		}
		if !l.locked {
			st := byte(0)
			if l.streaming {
				st = tlStatusStreaming
			}
			l.reply(byte(tlOpStatus), st)
		}
	case tlOpIdentity:
		if !l.locked {
			l.reply(byte(tlOpIdentity), tlSerial("LOCK", 0, 0)...)
		}
	}
	return len(b), nil
}

func (l *lockHID) Read(b []byte, _ int) (int, error) {
	if len(l.queue) == 0 {
		return 0, hid.ErrTimeout
	}
	n := copy(b, l.queue[0])
	l.queue = l.queue[1:]
	return n, nil
}

func TestTLStartFreesLockedFan(t *testing.T) {
	l := &lockHID{locked: true}
	p := &tlLCD{dev: l, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	if l.locked || !l.live || !p.held {
		t.Fatalf("fan still locked or not live: locked=%v live=%v held=%v", l.locked, l.live, p.held)
	}
	if ops := opsOf(&l.fakeHID); ops[0] != byte(tlOpVersion) {
		t.Fatalf("first op %v, want the version query", ops)
	}
	if p.version != "v1" {
		t.Fatalf("version %q", p.version)
	}
	if err := p.ShowJPEG([]byte{1, 2, 3}); err != nil || !l.streaming {
		t.Fatalf("stream not following after start: %v", err)
	}
}

func TestTLStartImageThenFreeThenSettingsThenStream(t *testing.T) {
	type upload struct {
		op    tlOp
		write func(*tlLCD, []byte) error
	}
	for _, u := range []upload{{tlOpStartImage, (*tlLCD).WriteStartImage}, {tlOpRestImage, (*tlLCD).WriteRestImage}} {
		testUploadFreesFan(t, u.op, u.write)
	}
}

func testUploadFreesFan(t *testing.T, op tlOp, write func(*tlLCD, []byte) error) {
	for _, drop := range []int{0, 2} { // all acknowledged / acknowledgements stop after two
		l := &lockHID{dropAck: drop}
		p := &tlLCD{dev: l, brightness: 50}
		if err := p.start(); err != nil {
			t.Fatal(err)
		}
		l.fakeHID.writes = nil
		err := write(p, bytes.Repeat([]byte{7}, 1200))
		if (drop != 0) != (err != nil) {
			t.Fatalf("drop %d: err = %v", drop, err)
		}
		ops := opsOf(&l.fakeHID)
		var tail []byte
		for i, o := range ops {
			if o != byte(op) {
				tail = ops[i:]
				break
			}
		}
		if !bytes.Equal(tail, []byte{61, 64}) || l.locked {
			t.Fatalf("drop %d: after the upload ops = %v, locked=%v", drop, tail, l.locked)
		}
		if err := p.ShowJPEG([]byte{1, 2, 3}); err != nil || !l.streaming {
			t.Fatalf("drop %d: stream does not follow (%v)", drop, err)
		}
	}
}

func TestTLHealthCheckRecoversLockedFan(t *testing.T) {
	now := time.Now()
	tlNow = func() time.Time { return now }
	defer func() { tlNow = time.Now }()

	l := &lockHID{}
	p := &tlLCD{dev: l, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	// First frame: the check is due and finds the stream active.
	p.ShowJPEG([]byte{1})
	if !l.streaming || !p.statusSeen {
		t.Fatal("first check did not see the stream")
	}
	// Not due again for 30 s: no status query.
	l.fakeHID.writes = nil
	p.ShowJPEG([]byte{1})
	if ops := opsOf(&l.fakeHID); !bytes.Equal(ops, []byte{byte(tlOpStream)}) {
		t.Fatalf("ops between checks = %v", ops)
	}
	// The fan gets stuck (an upload by another program); the next check frees it.
	l.locked, l.streaming = true, false
	now = now.Add(31 * time.Second)
	l.fakeHID.writes = nil
	if err := p.ShowJPEG([]byte{1}); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if l.locked || !l.streaming {
		t.Fatalf("not recovered: locked=%v streaming=%v", l.locked, l.streaming)
	}
	want := []byte{byte(tlOpStream), byte(tlOpStatus), byte(tlOpVersion), byte(tlOpSettings), byte(tlOpStream), byte(tlOpStatus)}
	if ops := opsOf(&l.fakeHID); !bytes.Equal(ops, want) {
		t.Fatalf("recovery ops = %v, want %v", ops, want)
	}
	// A fan that stays dead is reported, and retried soon.
	l.live = false
	l.locked, l.streaming = true, false
	now = now.Add(31 * time.Second)
	l.refuse = true
	if err := p.ShowJPEG([]byte{1}); err == nil {
		t.Fatal("persistent failure not reported")
	}
}

func TestTLHealthCheckIgnoresFanWithoutStatus(t *testing.T) {
	now := time.Now()
	tlNow = func() time.Time { return now }
	defer func() { tlNow = time.Now }()

	l := &lockHID{}
	l.deaf = true
	p := &tlLCD{dev: l, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := p.ShowJPEG([]byte{1}); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Minute)
	}
	if p.watch {
		t.Fatal("check kept running on a fan that never answers the status query")
	}
}

func TestTLCloseWithoutStoredPictureDoesNothingMore(t *testing.T) {
	l := &lockHID{noImage: true}
	p := &tlLCD{dev: l, brightness: 50, rotation: 90}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	l.fakeHID.writes = nil
	p.Close()
	ws := l.fakeHID.writes
	if len(ws) != 1 || ws[0][1] != byte(tlOpSettings) || ws[0][tlHeadSize] != byte(tlViewStored) || ws[0][tlHeadSize+6] != 1 {
		t.Fatalf("close writes = %v", opsOf(&l.fakeHID))
	}
}

func TestTLHealthCheckGivesUpAfterThreeRestores(t *testing.T) {
	now := time.Now()
	tlNow = func() time.Time { return now }
	defer func() { tlNow = time.Now }()

	l := &lockHID{}
	p := &tlLCD{dev: l, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	p.ShowJPEG([]byte{1})
	l.live = false
	l.locked, l.streaming = true, false
	l.refuse = true // the fan never comes back
	for i := 0; i < tlMaxRestores; i++ {
		now = now.Add(31 * time.Second)
		p.ShowJPEG([]byte{1})
	}
	if !p.watch || p.restores != tlMaxRestores {
		t.Fatalf("watch=%v restores=%d after %d restores", p.watch, p.restores, tlMaxRestores)
	}
	now = now.Add(31 * time.Second)
	l.fakeHID.writes = nil
	p.ShowJPEG([]byte{1})
	if p.watch {
		t.Fatal("check kept restoring")
	}
	if ops := opsOf(&l.fakeHID); !bytes.Equal(ops, []byte{byte(tlOpStatus), byte(tlOpStream)}) && !bytes.Equal(ops, []byte{byte(tlOpStream), byte(tlOpStatus)}) {
		t.Fatalf("ops after giving up = %v", ops)
	}
}

func TestTLRestoreCountsAgainstDailyCap(t *testing.T) {
	now := time.Now()
	tlNow = func() time.Time { return now }
	defer func() { tlNow = time.Now }()

	l := &lockHID{}
	p := &tlLCD{dev: l, brightness: 50}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	p.ShowJPEG([]byte{1})
	l.locked, l.streaming = true, false
	now = now.Add(31 * time.Second)
	before := p.rate.count
	p.ShowJPEG([]byte{1})
	if p.rate.count != before+1 {
		t.Fatalf("restore not counted: %d -> %d", before, p.rate.count)
	}
	// Cap used up: release and frame only, no settings write.
	p.rate.count = tlDailyMax
	l.locked, l.streaming = true, false
	now = now.Add(31 * time.Second)
	l.fakeHID.writes = nil
	p.ShowJPEG([]byte{1})
	for _, op := range opsOf(&l.fakeHID) {
		if op == byte(tlOpSettings) {
			t.Fatal("settings written although the cap is reached")
		}
	}
}
