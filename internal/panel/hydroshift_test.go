package panel

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

// A 50000-byte JPEG needs 50 packets of at most 1013 bytes.
func TestHydroShiftPackets(t *testing.T) {
	pkts := hsPackets(hsCmdJPEG, bytes.Repeat([]byte{7}, 50000))
	if len(pkts) != 50 {
		t.Fatalf("%d packets, want 50", len(pkts))
	}
	if !bytes.Equal(pkts[0][:11], []byte{0x02, 0x0E, 0, 0, 0xC3, 0x50, 0, 0, 0, 0x03, 0xF5}) {
		t.Fatalf("first header = % x", pkts[0][:11])
	}
	if !bytes.Equal(pkts[49][:11], []byte{0x02, 0x0E, 0, 0, 0xC3, 0x50, 0, 0, 0x31, 0x01, 0x6B}) {
		t.Fatalf("last header = % x", pkts[49][:11])
	}
	for _, p := range pkts {
		if len(p) != 1024 {
			t.Fatal("packets must be 1024 bytes")
		}
	}
}

func TestHydroShiftReport3AndReport1Packets(t *testing.T) {
	pkts := hsLCDPackets(hsReport3, hsCmdJPEG, bytes.Repeat([]byte{7}, 1003)) // 501 + 501 + 1
	if len(pkts) != 3 {
		t.Fatalf("%d packets, want 3", len(pkts))
	}
	if !bytes.Equal(pkts[0][:11], []byte{0x03, 0x0E, 0, 0, 0x03, 0xEB, 0, 0, 0, 0x01, 0xF5}) ||
		!bytes.Equal(pkts[2][:11], []byte{0x03, 0x0E, 0, 0, 0x03, 0xEB, 0, 0, 2, 0x00, 0x01}) {
		t.Fatalf("headers % x / % x", pkts[0][:11], pkts[2][:11])
	}
	for _, p := range pkts {
		if len(p) != 512 {
			t.Fatal("report 3 packets must be 512 bytes")
		}
	}
	// an empty request is one packet with length 0
	one := hsPackets1(hsCmdStatus, nil)
	if len(one) != 1 || len(one[0]) != 64 || !bytes.Equal(one[0][:6], []byte{1, 0x81, 0, 0, 0, 0}) {
		t.Fatalf("report 1 request % x", one[0][:6])
	}
	// 100 bytes need two packets of at most 58
	two := hsPackets1(0x86, make([]byte, 100))
	if len(two) != 2 || two[0][5] != 58 || two[1][5] != 42 || two[1][4] != 1 {
		t.Fatalf("report 1 split: %d packets, lens %d/%d", len(two), two[0][5], two[1][5])
	}
}

func TestHydroShiftBodyBounds(t *testing.T) {
	// lengths that point past the report must not panic or over-read
	for _, b := range [][]byte{
		nil, {1}, {1, 0x81, 0, 0, 0, 200, 1, 2, 3},
		{2, 0x17, 0, 0, 0, 1, 0, 0, 0, 0xFF, 0xFF, 5},
		{3, 0x17, 0, 0, 0, 1, 0, 0, 0},
		{9, 0x17, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		_, d, ok := hsBody(b)
		if ok && len(d) > len(b) {
			t.Fatalf("over-read for % x", b)
		}
	}
	if _, d, ok := hsBody([]byte{2, 0x17, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}); !ok || !bytes.Equal(d, []byte{0}) {
		t.Fatalf("valid report: %v % x", ok, d)
	}
	// a status answer with too few bytes is ignored
	p := &hydroShift{}
	p.takeStatus([]byte{1, 2, 3})
	if !p.statusAt.IsZero() {
		t.Fatal("short status accepted")
	}
}

// hsFake plays the pump head: it answers requests and records everything
// written.
type hsFake struct {
	writes   [][]byte
	queue    [][]byte
	version  string // 7th CSV field; "" = no answer to the version query
	avail    byte   // value of the 0x17 answer
	noAvail  bool
	resetOK  bool
	pump     int
	coolant  byte
	frac     byte
	hasTemp  byte
	closed   bool
	reportOf byte // layout used for 0x17 answers
}

func (f *hsFake) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), b...))
	cmd := b[1]
	r1 := func(cmd byte, data []byte) []byte {
		r := make([]byte, 64)
		r[0], r[1], r[5] = 1, cmd, byte(len(data))
		copy(r[6:], data)
		return r
	}
	r23 := func(rep, cmd byte, data []byte) []byte {
		r := make([]byte, 512)
		r[0], r[1] = rep, cmd
		r[9], r[10] = byte(len(data)>>8), byte(len(data))
		copy(r[11:], data)
		return r
	}
	switch {
	case b[0] == 1 && cmd == hsCmdVersion && f.version != "":
		f.queue = append(f.queue, r1(cmd, []byte("1,2,3,4,5,6,"+f.version)), r1(cmd, []byte("2024-01-01 00:00:00")))
	case b[0] == 1 && cmd == hsCmdStatus:
		f.queue = append(f.queue, r1(cmd, []byte{0x03, 0xE8, byte(f.pump >> 8), byte(f.pump), f.hasTemp, f.coolant, f.frac}))
	case cmd == hsCmdAvailable && !f.noAvail:
		f.queue = append(f.queue, r23(b[0], cmd, []byte{f.avail}))
	case b[0] == 1 && cmd == hsCmdReset:
		d := byte(0)
		if f.resetOK {
			d = 1
		}
		f.queue = append(f.queue, r1(cmd, []byte{d}))
	case cmd == hsCmdLCDControl:
		f.queue = append(f.queue, r23(b[0], cmd, nil))
	}
	return len(b), nil
}

func (f *hsFake) Read(b []byte, _ int) (int, error) {
	if len(f.queue) == 0 {
		return 0, hid.ErrTimeout
	}
	n := copy(b, f.queue[0])
	f.queue = f.queue[1:]
	return n, nil
}

func (f *hsFake) Close() error { f.closed = true; return nil }

// cmds lists (report, command) of everything written.
func (f *hsFake) cmds() [][2]byte {
	var out [][2]byte
	for _, w := range f.writes {
		out = append(out, [2]byte{w[0], w[1]})
	}
	return out
}

func (f *hsFake) count(report, cmd byte) int {
	n := 0
	for _, c := range f.cmds() {
		if c == [2]byte{report, cmd} {
			n++
		}
	}
	return n
}

// startAIO runs the start-up of a panel with the sleeps shortened: the
// watchdog gets `ticks` iterations.
func startAIO(p *hydroShift, ticks int) {
	var slept []time.Duration
	left := ticks + 2 // start delay and the 2 s after the status query
	p.sleep = func(d time.Duration) bool {
		slept = append(slept, d)
		left--
		return left >= 0
	}
	p.run()
}

func TestHydroShiftModeByFirmware(t *testing.T) {
	for _, tc := range []struct {
		pid     uint16
		version string
		report  byte
		watch   bool
	}{
		{0x7398, "1.2", hsReport3, true},
		{0x7398, "1.1", hsReport2, false},
		{0x7391, "1.5", hsReport3, true},
		{0x7391, "1.4", hsReport2, false},
		{0x7395, "2.0", hsReport3, true},
		{0x7395, "", hsReport2, false}, // no answer: old layout, no watchdog
		{0x7395, "garbage", hsReport2, false},
	} {
		f := &hsFake{version: tc.version}
		p := newHydroShift(f, tc.pid, 0)
		startAIO(p, 1)
		if p.report != tc.report || !p.ready {
			t.Errorf("%04x fw %q: report %d ready %v, want %d", tc.pid, tc.version, p.report, p.ready, tc.report)
		}
		if got := f.count(hsReport3, hsCmdAvailable)+f.count(hsReport2, hsCmdAvailable) > 0; got != tc.watch {
			t.Errorf("%04x fw %q: watchdog active = %v, want %v", tc.pid, tc.version, got, tc.watch)
		}
	}
}

func TestHydroShiftStartDelayAndNoModeAtDiscover(t *testing.T) {
	f := &hsFake{version: "1.0"}
	p := newHydroShift(f, 0x7398, 0)
	if p.model.startDelay != 10*time.Second || newHydroShift(f, 0x7391, 0).model.startDelay != 0 {
		t.Fatal("start delay: 10 s for HydroShift, none for Galahad")
	}
	if len(f.writes) != 0 {
		t.Fatal("constructing a panel must not write")
	}
	// frames before the device is ready are dropped, not sent
	img := image.NewRGBA(image.Rect(0, 0, 480, 480))
	if err := p.ShowImage(img); err != nil || len(f.writes) != 0 {
		t.Fatalf("frame before ready: err %v, %d writes", err, len(f.writes))
	}
	if p.brightness != 50 {
		t.Fatalf("default brightness %d, want 50", p.brightness)
	}
	startAIO(p, 0)
	// the default brightness is not pushed to the device; no LCD control yet
	if f.count(hsReport2, hsCmdLCDControl) != 0 {
		t.Fatal("LCD control sent without a brightness change")
	}
	// SetBrightness sends mode 4, the brightness, rotation 0 and 24 fps
	if err := p.SetBrightness(80); err != nil {
		t.Fatal(err)
	}
	var w []byte
	for _, x := range f.writes {
		if x[1] == hsCmdLCDControl {
			w = x
		}
	}
	if w == nil || !bytes.Equal(w[11:19], []byte{4, 80, 0, 0, 0, 0, 0, 24}) || w[0] != hsReport2 {
		t.Fatalf("LCD control = % x", w[:20])
	}
	// the rotation setting never reaches the device
	p.SetRotation(90)
	p.SetBrightness(60)
	for _, x := range f.writes {
		if x[1] == hsCmdLCDControl && x[13] != 0 {
			t.Fatal("hardware rotation byte set")
		}
	}
}

func TestHydroShiftBrightnessBeforeReadyIsSentOnce(t *testing.T) {
	f := &hsFake{version: "1.0"}
	p := newHydroShift(f, 0x7391, 0)
	p.SetBrightness(30)
	if len(f.writes) != 0 {
		t.Fatal("brightness written before ready")
	}
	startAIO(p, 0)
	if f.count(hsReport2, hsCmdLCDControl) != 1 {
		t.Fatalf("%d LCD control commands after start, want 1", f.count(hsReport2, hsCmdLCDControl))
	}
}

func TestHydroShiftFrameLayouts(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 480, 480))
	for _, tc := range []struct {
		version string
		report  byte
		size    int
	}{{"1.5", hsReport3, 512}, {"1.0", hsReport2, 1024}} {
		f := &hsFake{version: tc.version}
		p := newHydroShift(f, 0x7391, 0)
		startAIO(p, 0)
		f.writes = nil
		if err := p.ShowImage(img); err != nil {
			t.Fatal(err)
		}
		if len(f.writes) == 0 {
			t.Fatal("nothing sent")
		}
		for _, w := range f.writes {
			if w[0] != tc.report || w[1] != hsCmdJPEG || len(w) != tc.size {
				t.Fatalf("fw %s: report %d cmd %#x len %d", tc.version, w[0], w[1], len(w))
			}
		}
	}
	p := newHydroShift(&hsFake{}, 0x7391, 0)
	if p.ShowImage(image.NewRGBA(image.Rect(0, 0, 400, 400))) == nil || p.ShowJPEG(nil) == nil {
		t.Fatal("bad frames accepted")
	}
}

func TestHydroShiftSoftwareRotation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 480, 480))
	red := color.RGBA{255, 0, 0, 255}
	src.SetRGBA(10, 20, red) // x=10, y=20
	for _, tc := range []struct {
		deg  int
		x, y int
	}{
		{0, 10, 20},
		{90, 479 - 20, 10},
		{180, 479 - 10, 479 - 20},
		{270, 20, 479 - 10},
	} {
		out := hsTurn(src, tc.deg)
		if r, _, _, _ := out.At(tc.x, tc.y).RGBA(); r>>8 != 255 {
			t.Errorf("%d degrees: pixel not at %d,%d", tc.deg, tc.x, tc.y)
		}
	}
}

func TestHydroShiftSensors(t *testing.T) {
	f := &hsFake{version: "1.0", pump: 2750, hasTemp: 1, coolant: 31, frac: 27} // 27 -> tenth 7
	p := newHydroShift(f, 0x7398, 0)
	if p.Sensors() != nil {
		t.Fatal("sensors before start")
	}
	startAIO(p, 0)
	got := map[string]Sensor{}
	for _, s := range p.Sensors() {
		got[s.Path] = s
	}
	if s := got["HydroShift LCD/Coolant"]; math.Abs(s.Value-31.7) > 1e-9 || s.Unit != "°C" {
		t.Errorf("coolant %+v", s)
	}
	if s := got["HydroShift LCD/Pump"]; s.Value != 2750 || s.Unit != "RPM" {
		t.Errorf("pump %+v", s)
	}
	if s := got["HydroShift LCD/Fan"]; s.Value != 1000 {
		t.Errorf("fan %+v", s)
	}
	// no valid temperature: no coolant sensor
	f.hasTemp = 0
	p.askedAt = time.Time{}
	for _, s := range p.Sensors() {
		if strings.HasSuffix(s.Path, "Coolant") {
			t.Error("coolant reported although invalid")
		}
	}
	// stale values disappear
	p.statusAt = time.Now().Add(-time.Minute)
	f.queue = nil
	p.askedAt = time.Now()
	if p.Sensors() != nil {
		t.Error("stale sensors reported")
	}
}

func TestHydroShiftNeverSendsForbiddenCommands(t *testing.T) {
	for _, avail := range []byte{0, 1} {
		f := &hsFake{version: "1.5", avail: avail, resetOK: true, hasTemp: 1}
		p := newHydroShift(f, 0x7391, 0)
		startAIO(p, 6)
		p.SetBrightness(40)
		p.SetRotation(180)
		p.ShowImage(image.NewRGBA(image.Rect(0, 0, 480, 480)))
		p.Sensors()
		p.Close()
		for _, c := range f.cmds() {
			if hsForbidden[c[1]] {
				t.Fatalf("avail %d: forbidden command %#x (report %d)", avail, c[1], c[0])
			}
		}
		resets := f.count(hsReport1, hsCmdReset)
		if avail == 0 && resets != 0 {
			t.Fatalf("reset sent although the LCD said it is available")
		}
		if avail == 1 && (resets == 0 || resets > hsMaxResets) {
			t.Fatalf("%d resets, want 1..%d", resets, hsMaxResets)
		}
		if f.count(hsReport2, hsCmdReset)+f.count(hsReport3, hsCmdReset) != 0 {
			t.Fatal("reset in the wrong report")
		}
	}
	// the write guard itself refuses forbidden commands
	p := newHydroShift(&hsFake{}, 0x7391, 0)
	for cmd := range hsForbidden {
		if p.write(hsPackets1(cmd, nil)) == nil {
			t.Fatalf("command %#x passed the guard", cmd)
		}
	}
}

func TestHydroShiftWatchdogPausesAndRecovers(t *testing.T) {
	f := &hsFake{version: "1.5", avail: 1, resetOK: true}
	p := newHydroShift(f, 0x7391, 0)
	var pausedDuringReset, ticks int
	left := 0
	p.sleep = func(d time.Duration) bool {
		ticks++
		p.mu.Lock()
		if p.paused {
			pausedDuringReset++
		}
		p.mu.Unlock()
		left++
		return left <= 4 // start delay, 2 s, then two watchdog rounds (each also waits 1 s after a reset)
	}
	p.run()
	if f.count(hsReport1, hsCmdReset) != 1 {
		t.Fatalf("%d resets, want 1", f.count(hsReport1, hsCmdReset))
	}
	if pausedDuringReset == 0 {
		t.Fatal("frames were not paused around the reset")
	}
	if p.paused {
		t.Fatal("stream still paused")
	}
	// when the LCD answers nothing at all there is no reset
	f2 := &hsFake{version: "1.5", noAvail: true}
	p2 := newHydroShift(f2, 0x7391, 0)
	startAIO(p2, 3)
	if f2.count(hsReport1, hsCmdReset) != 0 {
		t.Fatal("reset without an answer")
	}
	// a device stuck on "unavailable" is reset at most hsMaxResets times
	f3 := &hsFake{version: "1.5", avail: 1}
	p3 := newHydroShift(f3, 0x7391, 0)
	startAIO(p3, 20)
	if n := f3.count(hsReport1, hsCmdReset); n != hsMaxResets {
		t.Fatalf("%d resets, want %d", n, hsMaxResets)
	}
}

func TestHydroShiftCloseStops(t *testing.T) {
	f := &hsFake{version: "1.5"}
	p := newHydroShift(f, 0x7391, 0)
	p.Close()
	if !f.closed {
		t.Fatal("handle not closed")
	}
	p.run() // returns at once: the stop channel is closed
	if len(f.writes) != 0 {
		t.Fatal("wrote after Close")
	}
	if p.ShowImage(image.NewRGBA(image.Rect(0, 0, 480, 480))) == nil {
		t.Fatal("frame accepted after Close")
	}
	if p.Close() != nil {
		t.Fatal("second Close failed")
	}
}

func TestHydroShiftVersionParsing(t *testing.T) {
	if got := hsUserVersion([]byte("a,b,c,d,e,f, 1.5\x00 ")); got != "1.5" {
		t.Fatalf("%q", got)
	}
	if hsUserVersion([]byte("a,b,c")) != "" {
		t.Fatal("short CSV accepted")
	}
	for in, want := range map[string][3]int{"1.5": {1, 5, 1}, "1.05": {1, 5, 1}, "2.10": {2, 10, 1}, "x": {0, 0, 0}, "1.": {0, 0, 0}, "-1.2": {0, 0, 0}} {
		a, b, ok := hsParseVersion(in)
		got := [3]int{a, b, 0}
		if ok {
			got[2] = 1
		}
		if !ok && want[2] == 0 {
			continue
		}
		if got != want {
			t.Errorf("%q: %v want %v", in, got, want)
		}
	}
}
