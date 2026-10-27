package panel

// UNI FAN TL LCD (HID 04FC:7393): every LCD fan shows up as its own HID
// device. All traffic uses 512-byte reports with report ID 0x02 and the
// framing described by tlHeader. Pictures are 400x400 JPEGs of at most 65535
// bytes, sent as a numbered series of frames.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"log"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/hid"
	"github.com/kewe87/casehud/internal/winusb"
)

const (
	tlReport    = 0x02 // report ID of every frame
	tlFrameSize = 512  // bytes per report, including the report ID
	tlHeadSize  = 11   // report ID + tlHeader fields
	tlBodyMax   = tlFrameSize - tlHeadSize
	tlImageMax  = 65535 // largest JPEG the fan accepts

	tlFrameRate = 25 // frames per second: limit offered to the engine and value programmed into the fan
	tlPortMax   = 3  // device ports are numbered 0..3
	tlSlotMax   = 2  // up to three fans per port

	tlFast = 200 * time.Millisecond // wait for the answer to a settings write
	tlSlow = 500 * time.Millisecond // wait for answers while identifying

	tlStatusStreaming = 4          // status byte while the live stream is active
	tlStatusByte      = tlHeadSize // index of the status byte in the answer
	tlStatusNoImage   = 0x0C       // answer to "show the stored picture" when none is stored
	tlCloseAck        = 60 * time.Millisecond
	tlCheckEvery      = 30 * time.Second // distance of the stream checks
	tlCheckRetry      = 5 * time.Second  // distance after a failed check
	tlMaxRestores     = 3                // restores in a row before the check gives up
)

// tlOp is the operation number in a frame header.
type tlOp byte

const (
	tlOpStatus   tlOp = 60   // read-only status; byte 11 of the answer is 4 while the stream is active
	tlOpVersion  tlOp = 61   // firmware version query (two answers); also ends an open transfer, see release
	tlOpIdentity tlOp = 62   // serial number, port and slot
	tlOpSettings tlOp = 64   // view, brightness, frame rate, rotation
	tlOpStream   tlOp = 0x46 // one slice of a picture, never answered
)

// tlView selects what the fan shows.
type tlView byte

const (
	tlViewStored tlView = 1 // the picture kept in the fan's own memory
	tlViewLive   tlView = 5 // pictures streamed by the host
)

var tlUSB = hid.ID{VendorID: 0x04FC, ProductID: 0x7393}

// tlHeader is the framing in front of every payload. Multi-byte fields are
// big endian: report ID, op, total (4 bytes), seq (3 bytes), length (2 bytes).
type tlHeader struct {
	Op     tlOp
	Total  uint32 // size of the whole transfer this frame belongs to
	Seq    uint32 // 24 bit frame number within the transfer
	Length uint16 // payload bytes in this frame
}

// marshal returns the full 512-byte report for h followed by body.
func (h tlHeader) marshal(body []byte) []byte {
	out := make([]byte, tlFrameSize)
	out[0] = tlReport
	out[1] = byte(h.Op)
	binary.BigEndian.PutUint32(out[2:6], h.Total)
	out[6], out[7], out[8] = byte(h.Seq>>16), byte(h.Seq>>8), byte(h.Seq)
	binary.BigEndian.PutUint16(out[9:11], h.Length)
	copy(out[tlHeadSize:], body)
	return out
}

// tlFrame builds a single frame; the body is cut to tlBodyMax.
func tlFrame(op tlOp, total, seq int, body []byte) []byte {
	body = body[:min(len(body), tlBodyMax)]
	return tlHeader{Op: op, Total: uint32(total), Seq: uint32(seq), Length: uint16(len(body))}.marshal(body)
}

// tlBody extracts the payload an answer announces (bounded by what arrived).
func tlBody(report []byte) []byte {
	if len(report) < tlHeadSize {
		return nil
	}
	n := int(binary.BigEndian.Uint16(report[9:11]))
	return report[tlHeadSize:min(len(report), tlHeadSize+n)]
}

// tlSettings is the 11-byte body of tlOpSettings.
type tlSettings struct {
	View       tlView
	Brightness byte // 0..100
	Rate       byte // frames per second
	Turns      byte // rotation in quarter turns, 0..3
}

const tlSettingsLen = 11

func (s tlSettings) bytes() []byte {
	b := make([]byte, tlSettingsLen)
	b[0] = byte(s.View)
	b[4] = s.Brightness
	b[5] = s.Rate
	b[6] = s.Turns
	return b
}

// tlChainGate lets one goroutine at a time use the shared fan chain; reports
// of different fans must not interleave. It is a one-slot channel.
type tlChainGate chan struct{}

var tlChain = make(tlChainGate, 1)

func (g tlChainGate) enter() { g <- struct{}{} }
func (g tlChainGate) leave() { <-g }

type tlLCD struct {
	dev        hid.Device
	path       string
	port, slot int
	posKnown   bool   // port and slot were read and are in range
	id, name   string // set by assignTLIDs
	serial     string
	version    string
	fw         firmwareText // version and build date for display (filled by release)
	brightness int
	rotation   int
	key        string // SettingsCache key, "" until the identity is read
	held       bool   // brightness and rotation are what the fan holds (last write was acknowledged)
	closed     bool   // guarded by tlChain
	rate       tlRate // limiter of brightness/rotation writes (tlrate.go)
	serialNew  bool   // the serial was written by CaseHUD this run; IDs keep ignoring it (see tlpersist.go)

	// Stream check (guarded by tlChain): watch is set while the fan should be
	// streaming, checkAt is when the status is due (zero: at the next frame),
	// statusSeen is set once the fan has answered a status query.
	watch      bool
	restores   int // restores in a row that did not bring the stream back
	checkAt    time.Time
	statusSeen bool
}

// tlKeyFor is the SettingsCache key: the serial number, else the chain position.
func tlKeyFor(serial string, posKnown bool, port, slot int) string {
	switch {
	case serial != "":
		return "tllcd-s" + tlSafe(serial)
	case posKnown:
		return tlPosID(port, slot)
	}
	return ""
}

// SettingsKey identifies the fan in the SettingsCache: its serial number.
func (p *tlLCD) SettingsKey() string { return p.key }

// SleepByFrame: screen-off shows black frames, brightness is never set to 0.
func (p *tlLCD) SleepByFrame() bool { return true }

func discoverTLLCD() ([]Panel, error) {
	infos, err := hid.Enumerate(tlUSB)
	if err != nil {
		return nil, err
	}
	var fans []*tlLCD
	var errs []error
	for _, info := range infos {
		dev, err := hid.Open(info)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		p := &tlLCD{dev: dev, path: info.Path, brightness: 50}
		if err := p.start(); err != nil {
			dev.Close()
			errs = append(errs, fmt.Errorf("TL LCD %s: %w", info.Path, err))
			continue
		}
		fans = append(fans, p)
	}
	assignTLIDs(fans)
	out := make([]Panel, 0, len(fans))
	for _, p := range fans {
		out = append(out, p)
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

func tlPosID(port, slot int) string { return fmt.Sprintf("tllcd-p%d-i%d", port, slot) }

// assignTLIDs gives every panel a unique ID. A fan with a valid, unique chain
// position is named after it, which keeps stored configurations valid. All
// others (position unreadable, out of range or claimed twice) are named after
// their serial number, plus a hash of the USB path when that is missing or
// repeated too, so no fan can replace another.
func assignTLIDs(ps []*tlLCD) {
	claims := map[string]int{}
	for _, p := range ps {
		if p.posKnown {
			claims[tlPosID(p.port, p.slot)]++
		}
	}
	stem := make([]string, len(ps))
	uses := map[string]int{}
	for i, p := range ps {
		if id := tlPosID(p.port, p.slot); p.posKnown && claims[id] == 1 {
			p.id = id
			p.name = fmt.Sprintf("UNI FAN TL LCD (port %d, fan %d)", TLPhysicalPort(p.port), p.slot+1)
			continue
		}
		if p.serial == "" {
			stem[i] = "tllcd-u"
		} else {
			stem[i] = "tllcd-s" + tlSafe(p.serial)
		}
		uses[stem[i]]++
	}
	for i, p := range ps {
		if p.id != "" {
			continue
		}
		p.id = stem[i]
		if uses[stem[i]] > 1 || p.idSerial() == "" {
			h := fnv.New32a()
			h.Write([]byte(p.path))
			p.id = fmt.Sprintf("%s-%08x", stem[i], h.Sum32())
		}
		p.name = "UNI FAN TL LCD (" + strings.TrimPrefix(p.id, "tllcd-") + ")"
	}
}

// TLPhysicalPort is the port number printed on the controller: the device
// counts its ports from 0, the housing from 1. IDs keep the device value.
func TLPhysicalPort(devPort int) int { return devPort + 1 }

// TLPosition reads the device port and chain index out of a panel ID of the
// form "tllcd-p<port>-i<fan>"; ok is false for other IDs (serial-based ones).
func TLPosition(id string) (port, idx int, ok bool) {
	n, err := fmt.Sscanf(id, "tllcd-p%d-i%d", &port, &idx)
	if err != nil || n != 2 || port < 0 || idx < 0 || id != tlPosID(port, idx) {
		return 0, 0, false
	}
	return port, idx, true
}

// tlSafe keeps letters, digits and dashes (at most 24) so a serial number is
// safe inside an ID.
func tlSafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		}
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

// flush throws away reports queued by earlier traffic so they cannot be taken
// for the answer to the next request.
func (p *tlLCD) flush() {
	buf := make([]byte, tlFrameSize)
	for i := 0; i < 32; i++ {
		if n, err := p.dev.Read(buf, 1); err != nil || n == 0 {
			return
		}
	}
}

// answer waits for the report that answers op. Reports for other operations
// (late answers to older requests) are skipped, at most three of them.
func (p *tlLCD) answer(op tlOp, wait time.Duration) ([]byte, error) {
	buf := make([]byte, tlFrameSize)
	for tries := 0; tries < 4; tries++ {
		n, err := p.dev.Read(buf, int(wait/time.Millisecond))
		if err != nil {
			return nil, fmt.Errorf("op %d: %w", op, err)
		}
		if n >= tlHeadSize && buf[0] == tlReport && tlOp(buf[1]) == op {
			return buf[:n], nil
		}
	}
	return nil, fmt.Errorf("op %d: no matching answer", op)
}

// request sends a one-frame request and returns its answer.
func (p *tlLCD) request(op tlOp, body []byte, wait time.Duration) ([]byte, error) {
	p.flush()
	if _, err := p.dev.Write(tlFrame(op, len(body), 0, body)); err != nil {
		return nil, err
	}
	return p.answer(op, wait)
}

// start identifies the fan, reads its version and switches it to live view.
// Identity and version are informational; start only fails when the fan
// answers neither the identity request nor the settings write.
func (p *tlLCD) start() error {
	tlChain.enter()
	defer tlChain.leave()

	// A fan left inside an aborted upload ignores everything until the
	// version query; this one also yields the version.
	p.version = p.release()

	ans, idErr := p.request(tlOpIdentity, nil, tlSlow)
	if idErr == nil {
		// Use the raw bytes: the announced length may stop before port and slot.
		raw := ans[tlHeadSize:]
		p.serial = strings.TrimRight(string(raw[:min(32, len(raw))]), "\x00")
		if len(raw) > 33 {
			p.port, p.slot = int(raw[32]), int(raw[33])
			p.posKnown = p.port <= tlPortMax && p.slot <= tlSlotMax
		}
	}

	oldKey := tlKeyFor(p.serial, p.posKnown, p.port, p.slot)
	if idErr == nil {
		p.assignSerialIfEmpty(ans[tlHeadSize:]) // only a fan without any serial; see tlpersist.go
	}
	p.key = tlKeyFor(p.serial, p.posKnown, p.port, p.slot)
	// (oldKey: a serial written just now changes the key, the record is under the old one.)
	// Start from what the fan was last given: the settings write below then
	// carries values it already holds, so nothing in its flash changes.
	if w := Cache().Get(oldKey); oldKey != "" {
		if w.Brightness != nil {
			p.brightness = max(0, min(100, *w.Brightness))
		}
		if w.Rotation != nil {
			p.rotation = *w.Rotation
		}
	}

	err := p.push(tlViewLive, true)
	if err != nil && idErr != nil {
		return idErr
	}
	p.watch, p.checkAt = true, time.Time{}
	return nil
}

// release ends a transfer that may still be open. Uploads (pictures, start
// image) occupy a transfer slot, and while it is occupied the fan ignores every
// other command, including settings and the live stream. Only the version
// query frees it, so it is sent first in every session and after every upload,
// whether that succeeded or not. It returns the version text ("" when the fan
// did not answer). The caller holds the chain.
func (p *tlLCD) release() string {
	p.flush()
	if _, err := p.dev.Write(tlFrame(tlOpVersion, 0, 0, nil)); err != nil {
		return ""
	}
	first, err := p.answer(tlOpVersion, tlSlow)
	if err != nil {
		return ""
	}
	var built string
	if second, err := p.answer(tlOpVersion, tlSlow); err == nil { // second answer: build date
		built = strings.TrimRight(string(tlBody(second)), "\x00")
	}
	ver := strings.TrimRight(string(tlBody(first)), "\x00")
	p.noteFirmware(ver, built)
	return ver
}

// streamActive asks the fan whether it shows the live stream (a read, nothing
// is stored). known is false when it did not answer. The caller holds the chain.
func (p *tlLCD) streamActive() (active, known bool) {
	ans, err := p.request(tlOpStatus, nil, tlFast)
	if err != nil || len(ans) <= tlStatusByte {
		return false, false
	}
	return ans[tlStatusByte] == tlStatusStreaming, true
}

// checkStream runs after a frame went out and finds a fan that silently stopped
// following the stream (stuck transfer): it frees it, restores the settings and
// sends the frame again. Checks are 30 s apart. A fan that never answers the
// status query is left alone. The caller holds the chain.
func (p *tlLCD) checkStream(frame []byte) error {
	now := tlNow()
	if !p.watch || now.Before(p.checkAt) {
		return nil
	}
	p.checkAt = now.Add(tlCheckEvery)
	active, known := p.streamActive()
	if active {
		p.statusSeen = true
		p.restores = 0
		return nil
	}
	if !known && !p.statusSeen {
		p.watch = false // this firmware does not answer; nothing to compare with
		return nil
	}
	if p.restores >= tlMaxRestores {
		p.watch = false
		log.Printf("TL LCD %s: still not showing the stream after %d restores, the check is off for this session", p.ID(), p.restores)
		return nil
	}
	p.restores++
	log.Printf("TL LCD %s: not showing the stream, freeing it", p.ID())
	p.release()
	// The settings write wears the flash like any other: it counts against the
	// daily cap and is skipped when that is used up.
	if p.spendWrite(now) {
		if err := p.push(tlViewLive, true); err != nil {
			p.checkAt = now.Add(tlCheckRetry)
			return fmt.Errorf("TL LCD: stream stopped and settings not accepted: %w", err)
		}
	}
	if err := p.sendFrame(frame); err != nil {
		return err
	}
	if active, _ := p.streamActive(); !active {
		p.checkAt = now.Add(tlCheckRetry)
		return errors.New("TL LCD: the display does not show the stream")
	}
	p.restores = 0
	return nil
}

// sendFrame writes one picture as stream reports. The caller holds the chain.
func (p *tlLCD) sendFrame(data []byte) error {
	for seq, off := 0, 0; off < len(data); seq, off = seq+1, off+tlBodyMax {
		if _, err := p.dev.Write(tlFrame(tlOpStream, len(data), seq, data[off:min(off+tlBodyMax, len(data))])); err != nil {
			return fmt.Errorf("TL LCD: %w", err)
		}
	}
	return nil
}

// push writes the current settings in the given view. With wait set the
// fan's answer is awaited. The caller holds the chain.
func (p *tlLCD) push(view tlView, wait bool) error {
	body := tlSettings{
		View:       view,
		Brightness: byte(p.brightness),
		Rate:       tlFrameRate,
		Turns:      byte(p.rotation / 90),
	}.bytes()
	var err error
	if wait {
		_, err = p.request(tlOpSettings, body, tlFast)
	} else {
		_, err = p.dev.Write(tlFrame(tlOpSettings, len(body), 0, body))
	}
	p.held = err == nil
	if err == nil && p.key != "" {
		c := Cache()
		c.PutBrightness(p.key, p.brightness)
		c.PutRotation(p.key, p.rotation)
	}
	return err
}

func (p *tlLCD) ID() string {
	if p.id == "" {
		return tlPosID(p.port, p.slot)
	}
	return p.id
}

func (p *tlLCD) Name() string {
	if p.name == "" {
		return fmt.Sprintf("UNI FAN TL LCD (port %d, fan %d)", TLPhysicalPort(p.port), p.slot+1)
	}
	return p.name
}

func (p *tlLCD) Size() image.Point { return image.Pt(400, 400) }

// MaxFPS is the frame rate limit: the frames of all fans share one USB link.
func (p *tlLCD) MaxFPS() int { return tlFrameRate }

// Round: the display sits in a round fan hub.
func (p *tlLCD) Round() bool  { return true }
func (p *tlLCD) MaxJPEG() int { return tlImageMax }

var errTLClosed = errors.New("TL LCD: closed")

// ShowJPEG streams one picture. The fan only keeps showing it while new
// pictures keep arriving; Close hands the display back to the stored image.
func (p *tlLCD) ShowJPEG(data []byte) error {
	if len(data) == 0 || len(data) > tlImageMax {
		return fmt.Errorf("TL LCD: frame size %d out of range", len(data))
	}
	tlChain.enter()
	defer tlChain.leave()
	if p.closed {
		return errTLClosed
	}
	if err := p.sendFrame(data); err != nil {
		return err
	}
	return p.checkStream(data)
}

func (p *tlLCD) SetBrightness(percent int) error {
	tlChain.enter()
	defer tlChain.leave()
	if p.closed {
		return errTLClosed
	}
	percent = max(0, min(100, percent))
	// A value the fan already stores is never written twice; others are
	// rate-limited and coalesced (tlrate.go).
	return p.changeSettings(&percent, nil)
}

func (p *tlLCD) SetRotation(degrees int) error {
	tlChain.enter()
	defer tlChain.leave()
	if p.closed {
		return errTLClosed
	}
	switch degrees {
	case 0, 90, 180, 270:
	default:
		return errors.New("rotation must be 0, 90, 180 or 270")
	}
	return p.changeSettings(nil, &degrees)
}

// tlCloseWait bounds how long Close waits for the chain, so quitting never
// hangs on an unplugged or stuck fan.
const tlCloseWait = 300 * time.Millisecond

// Close switches the fan back to its stored picture (slot 1, see WriteRestImage) (settings with view
// tlViewStored, the current brightness and rotation, the normal rate) and
// releases the handle. Brightness and rotation are the values the fan holds,
// so closing does not change what it stores. Without
// this the fan stays in live view and goes dark once nothing feeds it.
// Nothing is written afterwards.
func (p *tlLCD) Close() error {
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tlChain.enter()
		defer tlChain.leave()
		if p.closed {
			return
		}
		p.closed = true
		p.watch = false
		p.settleSettings()
		// Brightness and rotation stay as they are (a rotation of 0 here
		// would change the stored one). Only a brightness of 0, which would
		// leave the stored image dark, is replaced.
		br := p.brightness
		if br <= 0 {
			br = 50
		}
		body := tlSettings{View: tlViewStored, Brightness: byte(min(br, 100)), Rate: tlFrameRate, Turns: byte(p.rotation / 90)}.bytes()
		p.flush()
		if _, err := p.dev.Write(tlFrame(tlOpSettings, len(body), 0, body)); err == nil {
			if p.key != "" {
				Cache().PutBrightness(p.key, min(br, 100))
			}
			// Without a stored picture (slot 1) the fan answers with status
			// 0x0C and does not switch; nothing more is done about it.
			if ans, err := p.answer(tlOpSettings, tlCloseAck); err == nil && len(ans) > tlStatusByte && ans[tlStatusByte] == tlStatusNoImage {
				log.Printf("TL LCD %s: no stored picture on the fan, it keeps its current view", p.ID())
			}
		}
		p.dev.Close()
	}()
	select {
	case <-finished:
	case <-time.After(tlCloseWait):
	}
	return nil
}

func init() {
	Register(Driver{
		ID: "tllcd", Name: "UNI FAN TL LCD", Vendor: "USB 04FC", Kind: KindDisplay, Transport: TransportHID, Order: 10,
		Models: []Model{{VendorID: tlUSB.VendorID, ProductID: tlUSB.ProductID, Name: "UNI FAN TL LCD"}},
		Caps: Capabilities{Width: 400, Height: 400, Round: true, Rotation: RotationDevice,
			Rotations: []int{0, 90, 180, 270}, BrightnessMax: 100},
		Enumerate: func() ([]string, error) { return hidPaths(tlUSB) },
		Discover:  discoverTLLCD,
	})
}

// DriverID names the registry entry of the panel.
func (p *tlLCD) DriverID() string { return "tllcd" }

// hidPaths lists the paths of the present HID devices with the given IDs.
func hidPaths(ids ...hid.ID) ([]string, error) {
	infos, err := hid.Enumerate(ids...)
	var out []string
	for _, i := range infos {
		out = append(out, i.Path)
	}
	return out, err
}

// winusbPaths lists the paths of the present WinUSB devices with the given IDs.
func winusbPaths(ids ...winusb.ID) ([]string, error) {
	infos, err := winusb.Enumerate(ids...)
	var out []string
	for _, i := range infos {
		out = append(out, i.Path)
	}
	return out, err
}

// noteFirmware keeps the version (and build date, if the fan sent one) for the
// display in the window. A fan that answered nothing leaves the old text.
func (p *tlLCD) noteFirmware(ver, built string) {
	if ver == "" {
		return
	}
	if built != "" {
		ver += " (" + built + ")"
	}
	p.fw.set(ver)
}

// Version is the firmware version (and build date) the fan reported ("" if none).
func (p *tlLCD) Version() string { return p.fw.get() }
