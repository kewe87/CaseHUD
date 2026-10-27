package panel

// EdgeHub Advanced (HID 39B0:0101): a read-only temperature source. The hub
// reports the temperature at both 12V connectors of a graphics card and the
// ambient temperature. Experimental.
//
// One report layout (65 bytes, report ID 1; big endian):
//
//	[0] 1   [1] cmd   [2:6] total   [6:9] packet number   [9:11] length (<= 54)   data from 11
//
// A request has every field but the command zero. The answer must start with
// 1, repeat the command and announce at most 54 data bytes. Only the three
// read commands in ehAllowed can be sent; the firmware update command is not
// reachable from this code.

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

const (
	ehVID = 0x39B0
	ehPID = 0x0101

	ehReport  = 0x01
	ehSize    = 65
	ehHead    = 11
	ehMaxData = 54

	ehCmdTemps  = 0x01
	ehCmdInfo   = 0x02
	ehCmdSerial = 0x05

	ehPoll       = time.Second
	ehTimeout    = 500 * time.Millisecond
	ehMaxFails   = 3 // transport failures in a row before the device is reopened
	ehMaxAge     = 5 * time.Second
	ehTempsBytes = 9
)

// ehAllowed are the only commands the driver can write.
var ehAllowed = map[byte]bool{ehCmdTemps: true, ehCmdInfo: true, ehCmdSerial: true}

var errEHCommand = errors.New("edgehub: command not allowed")

// ehRequest builds the request report for a read command.
func ehRequest(cmd byte) ([]byte, error) {
	if !ehAllowed[cmd] {
		return nil, errEHCommand
	}
	b := make([]byte, ehSize)
	b[0], b[1] = ehReport, cmd
	return b, nil
}

// ehData validates an answer to cmd and returns its data bytes.
func ehData(buf []byte, cmd byte) ([]byte, error) {
	if len(buf) < ehHead {
		return nil, fmt.Errorf("edgehub: short report (%d bytes)", len(buf))
	}
	if buf[0] != ehReport || buf[1] != cmd {
		return nil, fmt.Errorf("edgehub: unexpected answer % x", buf[:2])
	}
	n := int(buf[9])<<8 | int(buf[10])
	if n > ehMaxData || ehHead+n > len(buf) {
		return nil, fmt.Errorf("edgehub: bad length %d", n)
	}
	return buf[ehHead : ehHead+n], nil
}

// ehTemps is the content of a temperature answer. A value is only meaningful
// when its OK flag is set.
type ehTemps struct {
	GPU2, GPU1, Ambient       float64
	GPU2OK, GPU1OK, AmbientOK bool
}

// ehParseTemps reads a temperature answer: degrees and tenths for GPU2, GPU1
// and the ambient sensor, then one flag per sensor (non-zero: sensor missing
// or value abnormal, so the value is dropped).
func ehParseTemps(d []byte) (ehTemps, error) {
	if len(d) < ehTempsBytes {
		return ehTemps{}, fmt.Errorf("edgehub: temperature answer has %d bytes, want %d", len(d), ehTempsBytes)
	}
	val := func(i int) (float64, bool) {
		if d[6+i] != 0 || d[2*i+1] > 9 {
			return 0, false
		}
		return float64(d[2*i]) + float64(d[2*i+1])/10, true
	}
	var t ehTemps
	t.GPU2, t.GPU2OK = val(0)
	t.GPU1, t.GPU1OK = val(1)
	t.Ambient, t.AmbientOK = val(2)
	return t, nil
}

type edgeHub struct {
	infos []hid.Info
	open  func(hid.Info) (hid.Device, error)

	mu    sync.Mutex // guards the readings
	temps ehTemps
	at    time.Time

	dev   hid.Device // used by the poll goroutine only (and Close after it ended)
	fails int
	start int // index into infos to try first

	running bool
	stop    chan struct{}
	done    chan struct{}
}

func discoverEdgeHub() ([]SensorDevice, error) {
	infos, err := hid.Enumerate(hid.ID{VendorID: ehVID, ProductID: ehPID})
	if err != nil || len(infos) == 0 {
		return nil, err
	}
	e := newEdgeHub(infos, hid.Open)
	e.run()
	return []SensorDevice{e}, nil
}

func newEdgeHub(infos []hid.Info, open func(hid.Info) (hid.Device, error)) *edgeHub {
	return &edgeHub{infos: infos, open: open, stop: make(chan struct{}), done: make(chan struct{})}
}

// run starts the poll goroutine.
func (e *edgeHub) run() {
	e.running = true
	go e.loop()
}

func (e *edgeHub) Name() string { return "EdgeHub Advanced" }

func (e *edgeHub) loop() {
	defer close(e.done)
	t := time.NewTicker(ehPoll)
	defer t.Stop()
	for {
		e.poll()
		select {
		case <-e.stop:
			return
		case <-t.C:
		}
	}
}

// poll reads the temperatures once. After ehMaxFails failures in a row the
// handle is dropped and opened again (trying the next collection first, since
// a device with several collections shows up more than once).
func (e *edgeHub) poll() {
	var t ehTemps
	var err error
	if e.dev == nil {
		t, err = e.connect()
	} else {
		t, err = e.readTemps()
	}
	if err != nil {
		e.fail()
		return
	}
	e.fails = 0
	e.mu.Lock()
	e.temps, e.at = t, time.Now()
	e.mu.Unlock()
}

func (e *edgeHub) fail() {
	e.fails++
	if e.fails >= ehMaxFails {
		if e.dev != nil {
			e.dev.Close()
			e.dev = nil
		}
		e.start = (e.start + 1) % max(1, len(e.infos))
		e.fails = 0
	}
}

// connect opens the first collection that answers a temperature request and
// returns that first answer.
func (e *edgeHub) connect() (ehTemps, error) {
	var last error = errors.New("edgehub: no device")
	for i := range e.infos {
		info := e.infos[(e.start+i)%len(e.infos)]
		dev, err := e.open(info)
		if err != nil {
			last = err
			continue
		}
		e.dev = dev
		t, err := e.readTemps()
		if err == nil {
			e.start = (e.start + i) % len(e.infos)
			return t, nil
		}
		last = err
		dev.Close()
		e.dev = nil
	}
	return ehTemps{}, last
}

// readTemps sends one request and parses the answer.
func (e *edgeHub) readTemps() (ehTemps, error) {
	req, err := ehRequest(ehCmdTemps)
	if err != nil {
		return ehTemps{}, err
	}
	if _, err := e.dev.Write(req); err != nil {
		return ehTemps{}, err
	}
	buf := make([]byte, ehSize)
	n, err := e.dev.Read(buf, int(ehTimeout/time.Millisecond))
	if err != nil {
		return ehTemps{}, err
	}
	d, err := ehData(buf[:max(0, min(n, len(buf)))], ehCmdTemps)
	if err != nil {
		return ehTemps{}, err
	}
	return ehParseTemps(d)
}

// Sensors returns the latest readings; sensors whose flag is set (missing or
// abnormal) and stale values are left out.
func (e *edgeHub) Sensors() []Sensor {
	e.mu.Lock()
	t, at := e.temps, e.at
	e.mu.Unlock()
	if at.IsZero() || time.Since(at) > ehMaxAge {
		return nil
	}
	var out []Sensor
	add := func(ok bool, key, name string, v float64) {
		if ok {
			out = append(out, Sensor{Key: "edgehub/" + key, Path: "EdgeHub/" + name, Value: v, Unit: "°C"})
		}
	}
	add(t.GPU1OK, "gpu1", "GPU1", t.GPU1)
	add(t.GPU2OK, "gpu2", "GPU2", t.GPU2)
	add(t.AmbientOK, "ambient", "Ambient", t.Ambient)
	return out
}

func (e *edgeHub) Close() error {
	select {
	case <-e.stop:
		return nil
	default:
	}
	close(e.stop)
	if e.running {
		<-e.done
	}
	if e.dev != nil {
		return e.dev.Close()
	}
	return nil
}

func init() {
	id := hid.ID{VendorID: ehVID, ProductID: ehPID}
	Register(Driver{
		ID: "edgehub", Name: "EdgeHub Advanced", Vendor: "USB 39B0", Kind: KindSensor, Transport: TransportHID,
		Experimental: true, Order: 40,
		Models:          []Model{{VendorID: ehVID, ProductID: ehPID, Name: "EdgeHub Advanced"}},
		Enumerate:       func() ([]string, error) { return hidPaths(id) },
		DiscoverSensors: discoverEdgeHub,
	})
}
