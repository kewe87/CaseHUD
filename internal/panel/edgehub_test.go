package panel

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

// ehReply builds an answer report for cmd with the given data.
func ehReply(cmd byte, data []byte) []byte {
	r := make([]byte, ehSize)
	r[0], r[1] = ehReport, cmd
	r[9], r[10] = byte(len(data)>>8), byte(len(data))
	copy(r[ehHead:], data)
	return r
}

// temps: GPU2 61.5, GPU1 58.2, ambient 27.9, flags all clear.
var ehGood = []byte{61, 5, 58, 2, 27, 9, 0, 0, 0}

func TestEdgeHubRequestOnlyReadCommands(t *testing.T) {
	r, err := ehRequest(ehCmdTemps)
	if err != nil || len(r) != 65 || r[0] != 1 || r[1] != 1 {
		t.Fatalf("temps request % x %v", r[:12], err)
	}
	for _, b := range r[2:] {
		if b != 0 {
			t.Fatal("request has non-zero fields besides the command")
		}
	}
	for cmd := 0; cmd < 256; cmd++ {
		_, err := ehRequest(byte(cmd))
		allowed := cmd == 1 || cmd == 2 || cmd == 5
		if allowed != (err == nil) {
			t.Errorf("command %#x: err %v", cmd, err)
		}
	}
	if _, err := ehRequest(0x03); err == nil {
		t.Fatal("the update command is reachable")
	}
}

func TestEdgeHubParse(t *testing.T) {
	d, err := ehData(ehReply(1, ehGood), 1)
	if err != nil {
		t.Fatal(err)
	}
	tt, err := ehParseTemps(d)
	if err != nil {
		t.Fatal(err)
	}
	if !tt.GPU2OK || !tt.GPU1OK || !tt.AmbientOK ||
		math.Abs(tt.GPU2-61.5) > 1e-9 || math.Abs(tt.GPU1-58.2) > 1e-9 || math.Abs(tt.Ambient-27.9) > 1e-9 {
		t.Fatalf("%+v", tt)
	}
	// each flag drops exactly its own sensor
	for i, want := range [][3]bool{{false, true, true}, {true, false, true}, {true, true, false}} {
		bad := append([]byte(nil), ehGood...)
		bad[6+i] = 1
		tt, _ := ehParseTemps(bad)
		if got := [3]bool{tt.GPU2OK, tt.GPU1OK, tt.AmbientOK}; got != want {
			t.Errorf("flag %d: %v want %v", i, got, want)
		}
	}
	// a tenths byte above 9 is not a temperature
	odd := append([]byte(nil), ehGood...)
	odd[1] = 10
	if tt, _ := ehParseTemps(odd); tt.GPU2OK {
		t.Error("tenth 10 accepted")
	}
	// too short, wrong command, wrong report ID, oversized or overlong length
	if _, err := ehParseTemps(ehGood[:8]); err == nil {
		t.Error("8 data bytes accepted")
	}
	if _, err := ehData(ehReply(2, ehGood), 1); err == nil {
		t.Error("answer to another command accepted")
	}
	bad := ehReply(1, ehGood)
	bad[0] = 2
	if _, err := ehData(bad, 1); err == nil {
		t.Error("wrong report ID accepted")
	}
	long := ehReply(1, ehGood)
	long[10] = 55
	if _, err := ehData(long, 1); err == nil {
		t.Error("length 55 accepted")
	}
	short := ehReply(1, ehGood)[:15]
	if _, err := ehData(short, 1); err == nil {
		t.Error("length beyond the report accepted")
	}
	if _, err := ehData(nil, 1); err == nil {
		t.Error("empty report accepted")
	}
}

// ehFake answers temperature requests from a script.
type ehFake struct {
	script   []func() ([]byte, error) // one entry per request; the last repeats
	writes   [][]byte
	closed   bool
	requests int
}

func (f *ehFake) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), b...))
	return len(b), nil
}

func (f *ehFake) Read(b []byte, ms int) (int, error) {
	i := min(f.requests, len(f.script)-1)
	f.requests++
	r, err := f.script[i]()
	if err != nil {
		return 0, err
	}
	return copy(b, r), nil
}

func (f *ehFake) Close() error { f.closed = true; return nil }

func ok(data []byte) func() ([]byte, error) {
	return func() ([]byte, error) { return ehReply(1, data), nil }
}
func timeout() ([]byte, error) { return nil, hid.ErrTimeout }

func TestEdgeHubSensors(t *testing.T) {
	f := &ehFake{script: []func() ([]byte, error){ok(ehGood)}}
	e := newEdgeHub([]hid.Info{{Path: "a"}}, func(hid.Info) (hid.Device, error) { return f, nil })
	if e.Sensors() != nil {
		t.Fatal("readings before the first poll")
	}
	e.poll()
	got := map[string]Sensor{}
	for _, s := range e.Sensors() {
		got[s.Path] = s
	}
	if len(got) != 3 || math.Abs(got["EdgeHub/Ambient"].Value-27.9) > 1e-9 || got["EdgeHub/GPU1"].Unit != "°C" {
		t.Fatalf("%+v", got)
	}
	// only temperature requests were written, and nothing else
	for _, w := range f.writes {
		if len(w) != 65 || w[0] != 1 || w[1] != 1 {
			t.Fatalf("unexpected write % x", w[:12])
		}
	}
	// a missing sensor (flag set) is dropped
	f.script = []func() ([]byte, error){ok([]byte{61, 5, 0, 0, 27, 9, 0, 1, 0})}
	f.requests = 0
	e.poll()
	got = map[string]Sensor{}
	for _, s := range e.Sensors() {
		got[s.Path] = s
	}
	if _, has := got["EdgeHub/GPU1"]; has || len(got) != 2 {
		t.Fatalf("flagged sensor reported: %+v", got)
	}
	// stale readings disappear
	e.mu.Lock()
	e.at = time.Now().Add(-time.Minute)
	e.mu.Unlock()
	if e.Sensors() != nil {
		t.Fatal("stale readings reported")
	}
}

func TestEdgeHubReopensAfterThreeFailures(t *testing.T) {
	opens := 0
	var devs []*ehFake
	e := newEdgeHub([]hid.Info{{Path: "a"}, {Path: "b"}}, func(info hid.Info) (hid.Device, error) {
		opens++
		f := &ehFake{script: []func() ([]byte, error){ok(ehGood)}}
		if info.Path == "a" {
			f.script = []func() ([]byte, error){ok(ehGood), timeout} // answers once, then nothing
		}
		devs = append(devs, f)
		return f, nil
	})
	e.poll() // opens a, reads fine
	if opens != 1 || len(e.Sensors()) != 3 {
		t.Fatalf("opens %d sensors %d", opens, len(e.Sensors()))
	}
	e.poll()
	e.poll()
	if devs[0].closed {
		t.Fatal("reopened after two failures")
	}
	e.poll() // third failure in a row
	if !devs[0].closed || e.dev != nil {
		t.Fatal("device not dropped after three failures")
	}
	e.poll() // reopen: tries the next collection first
	if opens < 2 || len(e.Sensors()) != 3 {
		t.Fatalf("not reopened: opens %d", opens)
	}
	e.Close() // loop was never started: Close must still be safe
}

func TestEdgeHubCloseStopsLoop(t *testing.T) {
	f := &ehFake{script: []func() ([]byte, error){ok(ehGood)}}
	e := newEdgeHub([]hid.Info{{Path: "a"}}, func(hid.Info) (hid.Device, error) { return f, nil })
	e.run()
	deadline := time.Now().Add(2 * time.Second)
	for e.Sensors() == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if !f.closed {
		t.Fatal("handle not closed")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEdgeHubOpenErrors(t *testing.T) {
	e := newEdgeHub([]hid.Info{{Path: "a"}}, func(hid.Info) (hid.Device, error) { return nil, errors.New("busy") })
	for i := 0; i < 5; i++ {
		e.poll() // must neither panic nor report values
	}
	if e.Sensors() != nil {
		t.Fatal("readings without a device")
	}
	// garbage answers count as failures too
	g := &ehFake{script: []func() ([]byte, error){func() ([]byte, error) { return []byte{1, 1, 0}, nil }}}
	e2 := newEdgeHub([]hid.Info{{Path: "a"}}, func(hid.Info) (hid.Device, error) { return g, nil })
	e2.poll()
	if e2.Sensors() != nil {
		t.Fatal("garbage taken for a reading")
	}
}
