package panel

// Pump-head screens of the water-cooling families (HID, 480x480): HydroShift
// LCD (0416:7398/7399/739A), Galahad II LCD (0416:7391) and Galahad II LCD V2
// (0416:7395). Experimental: not confirmed on real hardware.
//
// Three report layouts share one handle (byte 0 is the report ID, multi-byte
// fields are big endian):
//
//	report 1  64 bytes   [1] cmd, [3:5] packet number, [5] length, data from 6
//	report 2  1024 bytes [1] cmd, [2:6] total, [6:9] packet number, [9:11] length, data from 11
//	report 3  512 bytes  same header as report 2
//
// Pictures go out as JPEG (command 0x0E) in report 2 or 3 chunks. Firmware at
// or above a model-specific version takes report 3 and gets a watchdog; older
// firmware stays on report 2. The screen is turned in software. Only LCD
// commands and read-only queries are sent: pump, fan and lighting commands,
// uploads and firmware commands are never written (the tests check this).
// H.264 streaming is not implemented (see docs/DRIVERS.md).

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/hid"
)

const (
	hsVID = 0x0416

	hsReport1 = 0x01
	hsReport2 = 0x02
	hsReport3 = 0x03

	hsSize1      = 64
	hsSize2      = 1024
	hsSize3      = 512
	hsChunk1     = hsSize1 - 6
	hsChunk2     = hsSize2 - 11 // 1013
	hsChunk3     = hsSize3 - 11 // 501
	hsMaxJPEG    = 153600
	hsFPS        = 24
	hsPacketSize = hsSize2 // kept for the collection filter

	hsCmdLCDControl = 0x0C // mode, brightness, rotation, test pattern, frame rate
	hsCmdJPEG       = 0x0E
	hsCmdStatus     = 0x81 // pump/fan/coolant status
	hsCmdVersion    = 0x86 // firmware information (two answers)
	hsCmdAvailable  = 0x17 // is the LCD alive
	hsCmdReset      = 0x8E // only as the watchdog's last resort

	hsModeSetting = 4 // brightness change; the stream needs no mode switch

	hsDefaultBrightness = 50
	hsWatchEvery        = 2 * time.Second
	hsStatusEvery       = time.Second
	hsStatusMaxAge      = 5 * time.Second
	hsMaxResets         = 3 // a device that stays "unavailable" is left alone after this
)

// hsModel describes one product.
type hsModel struct {
	name       string
	fwMajor    int // firmware from which report 3 and the watchdog are used
	fwMinor    int
	startDelay time.Duration // wait after enumeration before the first command
}

var hsModels = map[uint16]hsModel{
	0x7398: {"HydroShift LCD", 1, 2, 10 * time.Second},
	0x7399: {"HydroShift LCD RGB", 1, 2, 10 * time.Second},
	0x739A: {"HydroShift LCD TL", 1, 2, 10 * time.Second},
	0x7391: {"Galahad II LCD", 1, 5, 0},
	0x7395: {"Galahad II LCD V2", 1, 5, 0},
}

// hsForbidden lists commands that must never be written (by any report):
// firmware, uploads, serial number, pump/fan control, lighting. The reset
// 0x8E is allowed only through the watchdog (see hydroShift.reset).
var hsForbidden = map[byte]bool{
	0x0A: true, 0x0B: true, 0x0F: true, 0x12: true, 0x13: true, 0x14: true, 0x15: true, 0x16: true,
	0x82: true, 0x83: true, 0x85: true, 0x89: true, 0x8A: true, 0x8B: true,
}

// hsAllowed are the only commands write sends (the reset has its own path).
var hsAllowed = map[byte]bool{hsCmdLCDControl: true, hsCmdJPEG: true, hsCmdAvailable: true, hsCmdStatus: true, hsCmdVersion: true}

// hsStatus is the content of an 0x81 answer.
type hsStatus struct {
	FanRPM, PumpRPM int
	Coolant         float64
	CoolantOK       bool
}

type hydroShift struct {
	mu       sync.Mutex
	dev      hid.Device
	id, name string
	model    hsModel
	pid      uint16

	brightness int
	bSet       bool // SetBrightness was called (otherwise the device keeps its own)
	rotation   int

	report   byte // 2 or 3: layout of LCD commands
	fw       string
	fwText   firmwareText // fw for readers that must not wait for p.mu
	ready    bool
	paused   bool
	closed   bool
	resets   int
	smu      sync.Mutex // guards status and statusAt; never held across I/O
	status   hsStatus
	statusAt time.Time
	askedAt  time.Time

	stop  chan struct{}
	sleep func(time.Duration) bool // false when the driver was closed meanwhile
}

func discoverHydroShift() ([]Panel, error) {
	var ids []hid.ID
	for pid := range hsModels {
		ids = append(ids, hid.ID{VendorID: hsVID, ProductID: pid})
	}
	infos, err := hid.Enumerate(ids...)
	if err != nil {
		return nil, err
	}
	// On Windows a device with several top-level collections shows up once per
	// collection; LCD frames need the one whose output reports are 1024 bytes.
	// Linux (hidraw) reports no length, so any node will do.
	var out []Panel
	var errs []error
	count := map[uint16]int{}
	for _, info := range infos {
		if info.OutputReportLen != 0 && info.OutputReportLen < hsPacketSize {
			continue
		}
		dev, err := hid.Open(info)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		n := count[info.ProductID]
		count[info.ProductID]++
		p := newHydroShift(dev, info.ProductID, n)
		go p.run()
		out = append(out, p)
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

func newHydroShift(dev hid.Device, pid uint16, n int) *hydroShift {
	m := hsModels[pid]
	p := &hydroShift{
		dev: dev, pid: pid, model: m,
		id:         fmt.Sprintf("aio-%04x-%d", pid, n),
		name:       m.name,
		brightness: hsDefaultBrightness,
		report:     hsReport2,
		stop:       make(chan struct{}),
	}
	if n > 0 {
		p.name = fmt.Sprintf("%s #%d", p.name, n+1)
	}
	p.sleep = p.wait
	return p
}

// wait sleeps for d; false if the panel was closed meanwhile.
func (p *hydroShift) wait(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.stop:
		return false
	case <-t.C:
		return true
	}
}

// --- framing -------------------------------------------------------------

// hsPackets splits data into report-2 packets.
func hsPackets(cmd byte, data []byte) [][]byte { return hsLCDPackets(hsReport2, cmd, data) }

// hsLCDPackets splits data into report 2 or 3 packets (11-byte header).
func hsLCDPackets(report byte, cmd byte, data []byte) [][]byte {
	size, chunk := hsSize2, hsChunk2
	if report == hsReport3 {
		size, chunk = hsSize3, hsChunk3
	}
	var out [][]byte
	for off, num := 0, 0; ; num++ {
		end := min(off+chunk, len(data))
		pkt := make([]byte, size)
		pkt[0], pkt[1] = report, cmd
		binary.BigEndian.PutUint32(pkt[2:6], uint32(len(data)))
		pkt[6], pkt[7], pkt[8] = byte(num>>16), byte(num>>8), byte(num)
		binary.BigEndian.PutUint16(pkt[9:11], uint16(end-off))
		copy(pkt[11:], data[off:end])
		out = append(out, pkt)
		off = end
		if off >= len(data) {
			return out
		}
	}
}

// hsPackets1 splits data into report-1 packets (64 bytes, 6-byte header).
func hsPackets1(cmd byte, data []byte) [][]byte {
	var out [][]byte
	for off, num := 0, 0; ; num++ {
		end := min(off+hsChunk1, len(data))
		pkt := make([]byte, hsSize1)
		pkt[0], pkt[1] = hsReport1, cmd
		pkt[3], pkt[4] = byte(num>>8), byte(num)
		pkt[5] = byte(end - off)
		copy(pkt[6:], data[off:end])
		out = append(out, pkt)
		off = end
		if off >= len(data) {
			return out
		}
	}
}

// hsBody returns the data of an answer in report layout 1, 2 or 3, bounded
// by what actually arrived; ok is false for anything else.
func hsBody(buf []byte) (cmd byte, data []byte, ok bool) {
	if len(buf) < 6 {
		return 0, nil, false
	}
	switch buf[0] {
	case hsReport1:
		n := int(buf[5])
		return buf[1], buf[6:min(len(buf), 6+min(n, hsChunk1))], true
	case hsReport2, hsReport3:
		if len(buf) < 11 {
			return 0, nil, false
		}
		n := int(binary.BigEndian.Uint16(buf[9:11]))
		return buf[1], buf[11:min(len(buf), 11+n)], true
	}
	return 0, nil, false
}

// --- I/O (caller holds mu) -------------------------------------------------

// write sends all packets back to back. Forbidden commands are refused here,
// whatever the caller asked for.
func (p *hydroShift) write(pkts [][]byte) error {
	for _, pkt := range pkts {
		if hsForbidden[pkt[1]] || !hsAllowed[pkt[1]] {
			return fmt.Errorf("%s: command %#x is not allowed", p.name, pkt[1])
		}
		if _, err := p.dev.Write(pkt); err != nil {
			return err
		}
	}
	return nil
}

// read waits up to timeout for an answer to cmd. Other reports are handled
// on the way: status reports update the sensors, the rest is dropped. At most
// 8 reports are looked at.
func (p *hydroShift) read(cmd byte, timeout time.Duration) ([]byte, bool) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, hsSize2)
	for i := 0; i < 8; i++ {
		left := time.Until(deadline)
		if left <= 0 {
			break
		}
		n, err := p.dev.Read(buf, int(left.Milliseconds())+1)
		if err != nil || n <= 0 {
			break
		}
		c, data, ok := hsBody(buf[:n])
		if !ok {
			continue
		}
		if c == hsCmdStatus {
			p.takeStatus(data)
		}
		if c == cmd {
			return append([]byte(nil), data...), true
		}
	}
	return nil, false
}

// takeStatus records an 0x81 answer: fan rpm, pump rpm, validity, coolant
// degrees and tenths.
func (p *hydroShift) takeStatus(d []byte) {
	if len(d) < 7 {
		return
	}
	s := hsStatus{
		FanRPM:  int(binary.BigEndian.Uint16(d[0:2])),
		PumpRPM: int(binary.BigEndian.Uint16(d[2:4])),
	}
	if d[4] != 0 {
		s.CoolantOK = true
		s.Coolant = float64(d[5]) + float64(d[6]%10)/10
	}
	p.smu.Lock()
	p.status, p.statusAt = s, time.Now()
	p.smu.Unlock()
}

// lcdCmd sends a command in the current report layout.
func (p *hydroShift) lcdCmd(cmd byte, data []byte) error {
	return p.write(hsLCDPackets(p.report, cmd, data))
}

// --- start-up ------------------------------------------------------------

// run waits out the start delay, queries the firmware and picks the report
// layout; then it starts the watchdog if the firmware has one.
func (p *hydroShift) run() {
	if !p.sleep(p.model.startDelay) {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.queryVersion()
	p.write(hsPackets1(hsCmdStatus, nil))
	p.read(hsCmdStatus, 3*time.Second)
	p.mu.Unlock()

	if !p.sleep(2 * time.Second) {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	watch := false
	if maj, mi, ok := hsParseVersion(p.fw); ok && (maj > p.model.fwMajor || maj == p.model.fwMajor && mi >= p.model.fwMinor) {
		p.report, watch = hsReport3, true
	}
	if p.bSet {
		p.sendBrightness()
	}
	p.ready = true
	p.mu.Unlock()
	if watch {
		p.watchdog()
	}
}

// queryVersion asks for the firmware information and keeps the user version
// (7th field). The caller holds mu.
func (p *hydroShift) queryVersion() {
	if p.write(hsPackets1(hsCmdVersion, nil)) != nil {
		return
	}
	if first, ok := p.read(hsCmdVersion, 3*time.Second); ok {
		p.fw = hsUserVersion(first)
		p.fwText.set(p.fw)
		p.read(hsCmdVersion, 3*time.Second) // second answer: build date, not used
	}
}

// hsUserVersion extracts the 7th CSV field of the version answer ("" if
// there are fewer than seven fields).
func hsUserVersion(d []byte) string {
	f := strings.Split(string(d), ",")
	if len(f) < 7 {
		return ""
	}
	return strings.Trim(f[6], " \x00\r\n")
}

// hsParseVersion reads "major.minor".
func hsParseVersion(s string) (major, minor int, ok bool) {
	a, b, found := strings.Cut(strings.TrimSpace(s), ".")
	if !found {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(a)
	minor, err2 := strconv.Atoi(b)
	return major, minor, err1 == nil && err2 == nil && major >= 0 && minor >= 0
}

// sendBrightness writes the LCD control command for the current brightness.
// Rotation stays 0 (the picture is turned in software), frame rate 24.
func (p *hydroShift) sendBrightness() error {
	if err := p.lcdCmd(hsCmdLCDControl, []byte{hsModeSetting, byte(p.brightness), 0, 0, 0, 0, 0, hsFPS}); err != nil {
		return err
	}
	p.read(hsCmdLCDControl, time.Second)
	return nil
}

// watchdog asks every two seconds whether the LCD is alive and, if it says
// no, stops the stream, resets the device and carries on. Gives up after
// hsMaxResets resets.
func (p *hydroShift) watchdog() {
	for p.sleep(hsWatchEvery) {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		down := p.unavailable()
		if !down || p.resets >= hsMaxResets {
			p.mu.Unlock()
			continue
		}
		p.paused = true
		p.reset()
		p.mu.Unlock()
		p.sleep(time.Second)
		p.mu.Lock()
		p.paused = false
		p.mu.Unlock()
	}
}

// unavailable reports whether the device answered "not available". No answer
// is not taken for that. The caller holds mu.
func (p *hydroShift) unavailable() bool {
	if p.lcdCmd(hsCmdAvailable, nil) != nil {
		return false
	}
	d, ok := p.read(hsCmdAvailable, time.Second)
	return ok && len(d) == 1 && d[0] == 1 // anything else (garbage, empty) is no reason to reset
}

// reset sends the reset command (report 1, no data). The caller holds mu.
func (p *hydroShift) reset() {
	log.Printf("aio: %s did not answer, sending the reset command (%d)", p.name, p.resets+1)
	p.resets++
	if _, err := p.dev.Write(hsPackets1(hsCmdReset, nil)[0]); err == nil {
		p.read(hsCmdReset, time.Second)
	}
}

// --- Panel -----------------------------------------------------------------

func (p *hydroShift) ID() string        { return p.id }
func (p *hydroShift) Name() string      { return p.name }
func (p *hydroShift) Size() image.Point { return image.Pt(480, 480) }
func (p *hydroShift) MaxJPEG() int      { return hsMaxJPEG }
func (p *hydroShift) MaxFPS() int       { return hsFPS }

// DriverID names the registry entry of the panel.
func (p *hydroShift) DriverID() string { return "aio" }

// Version is the firmware version the device reported ("" if unknown).
func (p *hydroShift) Version() string { return p.fwText.get() }

// ShowJPEG sends an encoded frame. Frames are dropped silently while the
// device is still starting or being reset.
func (p *hydroShift) ShowJPEG(data []byte) error {
	if len(data) == 0 || len(data) > hsMaxJPEG {
		return fmt.Errorf("%s: frame size %d out of range", p.name, len(data))
	}
	p.mu.Lock()
	rot := p.rotation
	p.mu.Unlock()
	if rot != 0 { // turn in software: decode, rotate, encode again
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
		return p.ShowImage(img)
	}
	return p.stream(data)
}

// ShowImage turns img according to the rotation setting, encodes it and
// sends it.
func (p *hydroShift) ShowImage(img image.Image) error {
	if sz := img.Bounds().Size(); sz != image.Pt(480, 480) {
		return fmt.Errorf("%s: frame is %v, want 480x480", p.name, sz)
	}
	p.mu.Lock()
	rot := p.rotation
	p.mu.Unlock()
	img = hsTurn(img, rot)
	var buf bytes.Buffer
	for q := 90; ; q -= 10 {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return err
		}
		if buf.Len() <= hsMaxJPEG {
			break
		}
		if q <= 30 {
			return fmt.Errorf("%s: frame too large (%d bytes)", p.name, buf.Len())
		}
	}
	return p.stream(buf.Bytes())
}

func (p *hydroShift) stream(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("%s: closed", p.name)
	}
	if !p.ready || p.paused {
		return nil
	}
	if err := p.lcdCmd(hsCmdJPEG, data); err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	p.read(hsCmdJPEG, 20*time.Millisecond) // the acknowledgement; a missing one is not an error
	return nil
}

// hsTurn returns img turned clockwise by degrees (0, 90, 180, 270).
func hsTurn(img image.Image, degrees int) image.Image {
	if degrees == 0 {
		return img
	}
	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	n := b.Dx()
	dst := image.NewRGBA(src.Bounds())
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			sx, sy := x, y
			switch degrees {
			case 90:
				sx, sy = y, n-1-x
			case 180:
				sx, sy = n-1-x, n-1-y
			case 270:
				sx, sy = n-1-y, x
			}
			copy(dst.Pix[dst.PixOffset(x, y):][:4], src.Pix[src.PixOffset(sx, sy):][:4])
		}
	}
	return dst
}

func (p *hydroShift) SetBrightness(percent int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.brightness, p.bSet = max(0, min(100, percent)), true
	if !p.ready || p.closed {
		return nil // sent when the device is ready
	}
	return p.sendBrightness()
}

// SetRotation turns the picture in software; the device's own rotation is
// never used.
func (p *hydroShift) SetRotation(degrees int) error {
	switch degrees {
	case 0, 90, 180, 270:
	default:
		return fmt.Errorf("rotation must be 0, 90, 180 or 270")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rotation = degrees
	return nil
}

// Sensors reports pump speed, fan speed and coolant temperature from the
// device's status answer. It asks for a new status about once a second.
func (p *hydroShift) Sensors() []Sensor {
	// Never wait for the device lock: a frame or the start-up may hold it for
	// long, and the caller is the shared sensor refresh.
	if p.mu.TryLock() {
		if !p.closed && p.ready && !p.paused && time.Since(p.askedAt) >= hsStatusEvery {
			p.askedAt = time.Now()
			if p.write(hsPackets1(hsCmdStatus, nil)) == nil {
				p.read(hsCmdStatus, 20*time.Millisecond)
			}
		}
		p.mu.Unlock()
	}
	p.smu.Lock()
	st, at := p.status, p.statusAt
	p.smu.Unlock()
	if at.IsZero() || time.Since(at) > hsStatusMaxAge {
		return nil
	}
	base := strings.ToLower(strings.ReplaceAll(p.id, " ", "-"))
	out := []Sensor{
		{Key: "aio/" + base + "/pump", Path: p.name + "/Pump", Value: float64(st.PumpRPM), Unit: "RPM"},
		{Key: "aio/" + base + "/fan", Path: p.name + "/Fan", Value: float64(st.FanRPM), Unit: "RPM"},
	}
	if st.CoolantOK {
		out = append(out, Sensor{Key: "aio/" + base + "/coolant", Path: p.name + "/Coolant", Value: st.Coolant, Unit: "°C"})
	}
	return out
}

func (p *hydroShift) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	close(p.stop)
	return p.dev.Close()
}

func init() {
	var models []Model
	var ids []hid.ID
	for pid, m := range hsModels {
		models = append(models, Model{VendorID: hsVID, ProductID: pid, Name: m.name})
		ids = append(ids, hid.ID{VendorID: hsVID, ProductID: pid})
	}
	sortModels(models)
	Register(Driver{
		ID: "aio", Name: "HydroShift LCD / Galahad II LCD", Vendor: "USB 0416", Kind: KindDisplay, Transport: TransportHID,
		Experimental: true, Order: 20, Models: models,
		Caps:      Capabilities{Width: 480, Height: 480, Rotation: RotationSoftware, Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100},
		Enumerate: func() ([]string, error) { return hidPaths(ids...) },
		Discover:  discoverHydroShift,
	})
}

func sortModels(ms []Model) {
	sort.Slice(ms, func(i, j int) bool { return ms[i].ProductID < ms[j].ProductID })
}
