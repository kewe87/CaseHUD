package asusaura

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
	"time"
)

// board answers queries from a fixed list of replies; nil means "no reply".
type board struct {
	writes  [][]byte
	replies [][]byte
}

func (b *board) Write(p []byte, _ time.Duration) error {
	b.writes = append(b.writes, append([]byte(nil), p...))
	return nil
}

func (b *board) Read(buf []byte, _ time.Duration) (int, error) {
	if len(b.replies) == 0 {
		return 0, errNoReply{}
	}
	r := b.replies[0]
	b.replies = b.replies[1:]
	if r == nil {
		return 0, errNoReply{}
	}
	return copy(buf, r), nil
}

func (b *board) Close() error { return nil }

type errNoReply struct{}

func (errNoReply) Error() string { return "no reply" }

// Layout reply read from a ROG STRIX X870E-E GAMING WIFI (model AULA3-AR60-0112).
var x870eReply = []byte{
	0xec, 0x30, 0x00, 0x00, 0x10, 0x1f, 0x04, 0x01, 0x00, 0x00, 0x78, 0x3c, 0x00, 0x01, 0x00, 0x00,
	0x78, 0x3c, 0x00, 0x01, 0x00, 0x00, 0x78, 0x3c, 0x00, 0x01, 0x01, 0x00, 0x09, 0x3c, 0x00, 0x00,
	0x08, 0x02, 0x01, 0xf4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func identifyReply(model string) []byte {
	return append([]byte{marker, replyIdentify}, []byte(model)...)
}

func TestProbeX870E(t *testing.T) {
	// each query first skips stale replies (one "no reply"), then gets its answer
	b := &board{replies: [][]byte{nil, identifyReply("AULA3-AR60-0112"), nil, x870eReply}}
	d := &device{port: b, pid: 0x1AA6, inSize: reportSize}
	l, err := d.probe()
	if err != nil {
		t.Fatal(err)
	}
	if l.model != "AULA3-AR60-0112" || l.headers != 3 || len(l.fixed) != 1 || l.fixed[0] != 9 || l.onboard != 1 {
		t.Fatalf("layout %+v", l)
	}
	d.lay = l
	var ids []string
	for _, z := range d.Zones() {
		ids = append(ids, z.ID)
	}
	if strings.Join(ids, " ") != "onboard argb1 argb2 argb3 ch4" {
		t.Fatalf("zones %v", ids)
	}
}

func TestParseLayoutPlain(t *testing.T) {
	r := make([]byte, reportSize)
	r[0], r[1] = marker, replyLayout
	r[6] = 2                  // two pixel channels
	r[10], r[16] = 0x78, 0x78 // both addressable headers
	r[31], r[33] = 4, 1       // four onboard LEDs, no 12 V header
	l, err := parseLayout(r)
	if err != nil || l.headers != 2 || len(l.fixed) != 0 || l.onboard != 4 {
		t.Fatalf("layout %+v, %v", l, err)
	}
}

func TestFrameReports(t *testing.T) {
	b := &board{}
	d := &device{port: b, pid: 0x1AA6, inSize: reportSize, lay: layout{headers: 2, onboard: 1}}
	strip := make([]color.RGBA, 26)
	for i := range strip {
		strip[i] = color.RGBA{byte(i), 1, 2, 255}
	}
	if err := d.SetFrame(map[string][]color.RGBA{"argb1": strip, "onboard": {{9, 8, 7, 255}}}); err != nil {
		t.Fatal(err)
	}
	want := [][]byte{
		{marker, opHostMode, 0x53, 0x00, 0x01},
		{marker, opEffectMode, 0, 0, 0, hostOwned},
		{marker, opEffectMode, 1, 0, 0, hostOwned},
		{marker, opEffectMode, 2, 0, 0, hostOwned},
	}
	if len(b.writes) != 7 {
		t.Fatalf("%d reports", len(b.writes))
	}
	for i, w := range want {
		if !bytes.Equal(b.writes[i], w) {
			t.Errorf("report %d: % x", i, b.writes[i])
		}
	}
	// 26 LEDs on pixel channel 0: 20, then 6 with the show flag
	if r := b.writes[4]; !bytes.Equal(r[:5], []byte{marker, opPixels, 0, 0, 20}) || len(r) != 65 {
		t.Errorf("first pixel report % x", r[:6])
	}
	if r := b.writes[5]; !bytes.Equal(r[:5], []byte{marker, opPixels, 0 | showFrame, 20, 6}) || r[5] != 20 || len(r) != 5+18 {
		t.Errorf("second pixel report % x", r[:6])
	}
	if r := b.writes[6]; !bytes.Equal(r, []byte{marker, opPixels, onboardPixels | showFrame, 0, 1, 9, 8, 7}) {
		t.Errorf("onboard report % x", r)
	}
	b.writes = nil
	d.SetFrame(map[string][]color.RGBA{"onboard": {{1, 2, 3, 255}}})
	if len(b.writes) != 1 {
		t.Errorf("took over twice: %d reports", len(b.writes))
	}
}
