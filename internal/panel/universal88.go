package panel

// Universal Screen 8.8" (WinUSB 1CBE:A088), a 1920x480 USB bar display. The
// device takes its picture in portrait (480x1920), so landscape frames are
// turned a quarter clockwise before encoding; the rotation setting
// (0/90/180/270) is applied in software too. Frames go over bulk endpoint
// 0x01 (replies on 0x81) as JPEGs behind a command block (see us88wire.go).
//
// Only display commands are sent: no startup image (it writes to flash),
// reboot or file operation. The only mode-like command is the Flex screen's
// hand-over on Close (nothing is stored). The 8.8" can be given a picture for
// Close (SetExitImage): it goes out as an ordinary frame, so the bar shows the
// logo after the app quit and nothing is stored. The LED ring (0416:8050) is
// a separate device.
//
// The same driver code also serves the other screens that speak this
// protocol (see usmodels.go): the 9.2" and 207 bars, the Flex fan screens and
// the SL V3 / TL V2 LCD, whose block framing differs (usFrameSL). The
// differences live in usModel.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

const (
	us88VID     = 0x1CBE
	us88PID     = 0xA088
	us88EPOut   = 0x01
	us88EPIn    = 0x81
	us88MaxJPEG = 512000
	us88FPS     = 20   // animation limit offered to the engine
	us88Timeout = 1000 // write timeout, ms
	us88Width   = 480  // short side; the device frame is portrait, 480x1920
	us88Height  = 1920
	us88ReadTO  = 400 // ms to wait for an acknowledgement

	us88BufPolls = 40                    // most queue polls while the device is busy
	us88BufGap   = 25 * time.Millisecond // between queue polls
	us88CloseGap = 50 * time.Millisecond // after the halt command on close
)

// us88Open opens the device; replaced in tests.
var us88Open = winusb.Open

// us88DevBrightness maps a percentage (0-100) to the 8.8"'s 0..50 range.
func us88DevBrightness(percent int) byte { return usScale(percent, usMaxBacklight) }

// usScale maps a percentage (0-100) onto 0..max, rounding to nearest.
func usScale(percent, max int) byte {
	percent = min(100, max0(percent))
	return byte((percent*max + 50) / 100)
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

type universal88 struct {
	mu         sync.Mutex
	dev        winusb.Device // nil while disconnected
	info       winusb.Info
	curPath    atomic.Pointer[string] // info.Path for DevicePath, readable without p.mu
	m          *usModel               // nil: the 8.8" (see mdl)
	id, name   string
	fw         string       // firmware string reported by the device, "" if unknown
	fwText     firmwareText // fw for readers that must not wait for p.mu
	clock      usClock
	brightness int
	rotation   int // 0, 90, 180 or 270: software for bars, device-side for fan screens
	failures   int
	flipOn     bool // device-side 180 degree rotation wanted (SetDeviceFlip)
	sentTurn   int  // last device rotation value written plus one, 0 = none (see sendRotation)
	sentBright int  // last backlight value written plus one, 0 = none (see setBacklight)
	// offlineClock: the device keeps its own clock flag on (see SetOfflineClock);
	// start-up then leaves its clock alone.
	offlineClock bool
	timeSetAt    time.Time  // last 0x33 time-only write
	lay          layerState // picture/overlay layers and device video (us88layers.go)
	nextTry      time.Time
	lastIO       time.Time     // last write, for the SL V3 heartbeat
	dark         bool          // brightness 0 was sent (display asleep)
	exit         image.Image   // shown by Close (8.8" only, see SetExitImage)
	closed       bool          // Close ran
	stop         chan struct{} // ends the heartbeat
}

// mdl is the model of the panel; panels built without one are 8.8" bars.
func (p *universal88) mdl() *usModel {
	if p.m == nil {
		return &usModel88
	}
	return p.m
}

func discoverUniversal88() ([]Panel, error) { return discoverUSB(&usModel88) }

// discoverUSB opens every present device of the given models.
func discoverUSB(models ...*usModel) ([]Panel, error) {
	var ids []winusb.ID
	for _, m := range models {
		ids = append(ids, winusb.ID{VendorID: us88VID, ProductID: m.pid})
	}
	infos, err := winusb.Enumerate(ids...)
	if err != nil {
		return nil, err
	}
	var out []Panel
	var errs []error
	taken := map[string]bool{}
	count := map[*usModel]int{}
	for _, info := range infos {
		var m *usModel
		for _, c := range models {
			if c.pid == info.ProductID {
				m = c
			}
		}
		if m == nil {
			continue
		}
		n := count[m]
		count[m]++
		p := &universal88{m: m, brightness: m.defBright}
		p.setInfo(info)
		p.id, p.name = m.identify(info, n, infos, taken)
		if p.brightness == 0 {
			p.brightness = 100
		}
		// Start from what the device was last given, so the start-up writes
		// nothing it already holds.
		if w := Cache().Get(p.SettingsKey()); w.Brightness != nil {
			p.brightness = max(0, min(100, *w.Brightness))
			p.sentBright = us88DevHeld(m, p.brightness)
		}
		if w := Cache().Get(p.SettingsKey()); w.Rotation != nil && *w.Rotation >= 0 && *w.Rotation < 255 {
			p.sentTurn = *w.Rotation + 1
		}
		if w := Cache().Get(p.SettingsKey()); w.OfflineClock != nil {
			p.offlineClock = *w.OfflineClock
		}
		if err := p.connect(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.name, err))
			continue
		}
		if m.heartbeat {
			p.stop = make(chan struct{})
			go p.heartbeatLoop()
		}
		out = append(out, p)
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

// setInfo records the device the panel talks to.
func (p *universal88) setInfo(i winusb.Info) {
	p.info = i
	s := i.Path
	p.curPath.Store(&s)
}

// DevicePath implements DevicePather.
func (p *universal88) DevicePath() string {
	if s := p.curPath.Load(); s != nil {
		return *s
	}
	return ""
}

// connect (re)opens the device and runs the init sequence.
func (p *universal88) connect() error {
	dev, err := us88Open(p.info)
	if err != nil {
		return err
	}
	p.dev = dev
	p.lay.reset() // what the device shows is unknown now: resend picture and layer
	// A frame whose size is a multiple of 512 bytes must end with a
	// zero-length packet or the device keeps waiting for more data.
	if p.mdl().zlp {
		dev.ZeroLengthTerminate(us88EPOut)
	}
	if err := p.wake(); err != nil {
		p.disconnect()
		return err
	}
	return nil
}

func (p *universal88) disconnect() {
	if p.dev != nil {
		p.dev.Close()
		p.dev = nil
	}
}

// backoff schedules the next reconnect attempt: 0.5 s, 1 s, ... up to 8 s.
func (p *universal88) backoff() {
	p.failures++
	p.nextTry = time.Now().Add(min(8*time.Second, 250*time.Millisecond<<min(p.failures, 5)))
}

// recover drops the handle after a timeout, which can leave the device
// waiting for the rest of a frame, and reopens it.
func (p *universal88) recover() {
	p.disconnect()
	p.backoff()
}

// reopen reconnects after a recover once the back-off has elapsed.
func (p *universal88) reopen() error {
	if p.dev != nil {
		return nil
	}
	if time.Now().Before(p.nextTry) {
		return errors.New("reconnecting")
	}
	// The path normally survives a replug; look it up again in case it didn't.
	if infos, err := winusb.Enumerate(winusb.ID{VendorID: us88VID, ProductID: p.mdl().pid}); err == nil {
		for _, i := range infos {
			if i.Serial == p.info.Serial {
				p.setInfo(i)
			}
		}
	}
	if err := p.connect(); err != nil {
		p.backoff()
		return err
	}
	return nil
}

// seal builds the next command block, stamped by the panel's clock.
func (p *universal88) seal(op usOp, args []byte) []byte {
	c := usCommand{Op: op, Stamp: p.clock.next(), Args: args}
	if p.mdl().framing == usFrameSL {
		return c.sealSL()
	}
	return c.seal()
}

// seal88 builds a block in the 8.8" framing whatever the model's own framing is.
func (p *universal88) seal88(op usOp, args []byte) []byte {
	return usCommand{Op: op, Stamp: p.clock.next(), Args: args}.seal()
}

func (p *universal88) write(data []byte) error {
	p.lastIO = time.Now()
	for len(data) > 0 {
		n, err := p.dev.Write(us88EPOut, data, us88Timeout)
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("%s: USB write made no progress", p.name)
		}
		data = data[n:]
	}
	return nil
}

// ack reads one 512-byte reply and drops anything queued behind it, so acks
// never pile up. ErrTimeout means the device didn't answer.
func (p *universal88) ack() ([]byte, error) {
	buf := make([]byte, 512)
	var n int
	var err error
	// The device may put zero-length packets in front of a reply; skip them,
	// or every reply would be read one command late.
	for i := 0; i < 4 && n == 0; i++ {
		if n, err = p.dev.Read(us88EPIn, buf, us88ReadTO); err != nil {
			return nil, err
		}
	}
	if n == 0 {
		return nil, winusb.ErrTimeout
	}
	var junk [512]byte
	for i := 0; i < 16; i++ { // bounded: a chatty device must not hold the lock forever
		if m, err := p.dev.Read(us88EPIn, junk[:], 5); err != nil || m == 0 {
			break
		}
	}
	return buf[:n], nil
}

// issue sends one command block without data and returns the device's reply (nil
// if it didn't answer: not every command is acknowledged).
func (p *universal88) issue(op usOp, params []byte) ([]byte, error) {
	if err := p.write(p.seal(op, params)); err != nil {
		return nil, err
	}
	r, _ := p.ack()
	return r, nil
}

// issue88 is issue with a block in the 8.8" framing.
func (p *universal88) issue88(op usOp, params []byte) ([]byte, error) {
	if err := p.write(p.seal88(op, params)); err != nil {
		return nil, err
	}
	r, _ := p.ack()
	return r, nil
}

// usEmptyPNG is a fully transparent w x h PNG. Pushed to the overlay layer it
// removes a picture another program left there, which would otherwise stay on
// top of our JPEGs.
func usEmptyPNG(w, h int) []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

var (
	usOverlayMu    sync.Mutex
	usOverlayCache = map[[2]int][]byte{}
)

// usOverlay returns the cached empty overlay of the given size.
func usOverlay(w, h int) []byte {
	usOverlayMu.Lock()
	defer usOverlayMu.Unlock()
	k := [2]int{w, h}
	if b, ok := usOverlayCache[k]; ok {
		return b
	}
	b := usEmptyPNG(w, h)
	usOverlayCache[k] = b
	return b
}

// wakeSteps are the commands that bring a bar into a state where it shows
// host frames as sent: stop video playback, hide the built-in clock, reset
// the stored rotation (8.8" only; we rotate in software) and query the
// version. Each is followed by usSettle. The frame-rate command is not part
// of it: it only matters for video playback, frames are paced by the host.
var wakeSteps = []struct {
	op   usOp
	args []byte
}{
	{usOpHalt, nil},
	{usOpClock, nil}, // set the time (flag 2); the arguments are built when it is sent
	{usOpClockOff, []byte{0}},
	{usOpTurn, []byte{0}},
	{usOpInfo, nil},
}

// wake brings the device into the state where it shows host frames.
func (p *universal88) wake() error {
	switch p.mdl().init {
	case usInitSL:
		return p.wakeSL()
	case usInitFlex:
		return p.wakeFlex()
	}
	return p.wakeBar()
}

// The offline clock flag lives in universal88.offlineClock (see SetOfflineClock).

// wakeBar runs wakeSteps, clears a leftover overlay picture with a
// transparent one and sets the backlight. Only a failing first step or write
// aborts it; the device does not answer every command.
func (p *universal88) wakeBar() error {
	m := p.mdl()
	for i, st := range wakeSteps {
		args := st.args
		switch st.op {
		case usOpTurn:
			if !m.sendTurn && !p.devFlip() {
				continue // a stored setting: only the 8.8" gets it rewritten
			}
			args = []byte{p.barTurn()}
			if p.sentTurn == int(args[0])+1 {
				continue // the device already holds this turn
			}
		case usOpClock:
			if !m.clock {
				continue
			}
			if !p.timeSetAt.IsZero() && time.Since(p.timeSetAt) < time.Minute {
				continue // a flapping device is not set again and again
			}
			p.timeSetAt = time.Now()
			args = usTimeArgs(usNow(), usClockSetOnly) // time only, stores nothing
		case usOpClockOff:
			if p.offlineClock {
				continue // the user wants the device's own clock left alone
			}
		}
		r, err := p.issue(st.op, args)
		if err != nil && i == 0 {
			return err
		}
		if st.op == usOpInfo {
			p.noteFirmware(r)
		}
		if st.op == usOpTurn && err == nil {
			p.sentTurn = int(args[0]) + 1
			Cache().PutRotation(p.SettingsKey(), int(args[0]))
		}
		time.Sleep(usSettle)
	}
	if err := p.write(p.withData(usOpOverlay, usOverlay(m.short, m.long))); err != nil {
		return err
	}
	p.ack() // acknowledged like a picture; a missing ack is not fatal here
	return p.setBacklight(false)
}

// noteFirmware stores the version string of an 0x0A reply (text from byte 8).
func (p *universal88) noteFirmware(r []byte) {
	if len(r) <= 8 || r[0] != byte(usOpInfo) {
		return
	}
	end := 8
	for end < len(r) && end < 8+32 && r[end] >= 0x20 && r[end] < 0x7F {
		end++
	}
	p.fw = string(r[8:end])
	p.fwText.set(p.fw)
}

// Version is the firmware string the device reported at start ("" if none).
func (p *universal88) Version() string { return p.fwText.get() }

// withData is a command block carrying the data length (u32 BE) plus the data, as one transfer.
func (p *universal88) withData(op usOp, data []byte) []byte {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	out := append(p.seal(op, size[:]), data...)
	if p.mdl().framing == usFrameSL {
		// Always one transfer of fixed length; the rest is padding. Data
		// that doesn't fit is cut: callers check the size first.
		if len(out) > slTransferLen {
			out = out[:slTransferLen]
		}
		out = append(out, make([]byte, slTransferLen-len(out))...)
	}
	return out
}

// picture is a picture command with its JPEG.
func (p *universal88) picture(jpeg []byte) []byte { return p.withData(usOpPicture, jpeg) }

// us88Rotate turns a landscape frame (1920x480) into the 480x1920 portrait
// frame the device expects: frame(x, y) = L(y, h-1-x), a quarter turn
// clockwise. With flip set the landscape frame is turned by 180 degrees first
// (panel mounted upside down). The copy runs in tiles so both the reads and
// the writes stay in cache.
func us88Rotate(src *image.RGBA, flip bool) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy() // landscape size
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	const tile = 32
	for ty := 0; ty < h; ty += tile {
		for tx := 0; tx < w; tx += tile {
			for sy := ty; sy < min(ty+tile, h); sy++ {
				so := sy*src.Stride + tx*4
				for sx := tx; sx < min(tx+tile, w); sx++ {
					dx, dy := h-1-sy, sx // frame(x, y) = L(y, h-1-x)
					if flip {
						dx, dy = sy, w-1-sx
					}
					do := dy*dst.Stride + dx*4
					copy(dst.Pix[do:do+4:do+4], src.Pix[so:so+4:so+4])
					so += 4
				}
			}
		}
	}
	return dst
}

func (p *universal88) ID() string        { return p.id }
func (p *universal88) Name() string      { return p.name }
func (p *universal88) Size() image.Point { return p.mdl().landscape() }
func (p *universal88) MaxJPEG() int      { return p.mdl().jpegMax() }
func (p *universal88) MaxFPS() int       { return p.mdl().fpsMax() }
func (p *universal88) Round() bool       { return p.mdl().round }

// DriverID names the registry entry of the panel.
func (p *universal88) DriverID() string { return p.mdl().driver }

// ShowJPEG takes a landscape JPEG, turns it into the device's portrait
// orientation and streams it.
func (p *universal88) ShowJPEG(data []byte) error {
	if len(data) == 0 || len(data) > p.MaxJPEG() {
		return fmt.Errorf("%s: frame size %d out of range", p.name, len(data))
	}
	// Check the size from the header first: no decoding of a wrong-sized picture.
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	if err := p.checkSize(image.Pt(cfg.Width, cfg.Height)); err != nil {
		return err
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	return p.present(toRGBA(img))
}

// ShowImage takes a landscape (or upright portrait) picture without going
// through an encoded JPEG first: it is turned and encoded once.
func (p *universal88) ShowImage(img image.Image) error {
	if err := p.checkSize(img.Bounds().Size()); err != nil {
		return err
	}
	return p.present(toRGBA(img))
}

func (p *universal88) checkSize(sz image.Point) error {
	m := p.mdl()
	if m.square {
		if sz != image.Pt(m.short, m.short) {
			return fmt.Errorf("%s: frame is %v, want %dx%d", p.name, sz, m.short, m.short)
		}
		return nil
	}
	if sz != image.Pt(m.long, m.short) && sz != image.Pt(m.short, m.long) {
		return fmt.Errorf("%s: frame is %v, want %dx%d or %dx%d", p.name, sz, m.long, m.short, m.short, m.long)
	}
	return nil
}

// toRGBA returns img as an RGBA picture with its origin at (0,0), without
// copying when it already is one.
func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) {
		return r
	}
	b := img.Bounds()
	r := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(r, r.Bounds(), img, b.Min, draw.Src)
	return r
}

// present orients a decoded frame for the device and sends it.
func (p *universal88) present(rgba *image.RGBA) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.send(p.orient(rgba))
}

// orient turns a canvas into the device's portrait frame (see present).
func (p *universal88) orient(rgba *image.RGBA) *image.RGBA {
	if p.mdl().square {
		return rgba // fan screens turn the picture themselves
	}
	if rgba.Rect.Dx() == p.mdl().short {
		return us88Upright(rgba, p.rotation)
	}
	return us88Rotate(rgba, p.rotation == 180 && !p.devFlip())
}

// query sends a command block and waits for its answer; unlike issue a
// missing answer is an error.
func (p *universal88) query(op usOp) ([]byte, error) {
	if err := p.write(p.seal(op, nil)); err != nil {
		return nil, err
	}
	return p.ack()
}

// transfer writes one JPEG frame and reads its acknowledgement. err is a write
// failure; a frame that went out without an acknowledgement has acked == false.
func (p *universal88) transfer(jpg []byte) (r []byte, acked bool, err error) {
	return p.transferOp(usOpPicture, jpg)
}

// transferOp is transfer for any command that carries data (picture, overlay).
func (p *universal88) transferOp(op usOp, data []byte) (r []byte, acked bool, err error) {
	if err := p.write(p.withData(op, data)); err != nil {
		return nil, false, err
	}
	if !p.mdl().ackFrames {
		// This device answers asynchronously: just empty the receive queue.
		p.drain()
		return nil, true, nil
	}
	r, rerr := p.ack()
	return r, rerr == nil, nil
}

// drain reads and drops whatever the device has queued, without waiting long.
func (p *universal88) drain() {
	var junk [512]byte
	for i := 0; i < 4; i++ {
		if m, err := p.dev.Read(us88EPIn, junk[:], 2); err != nil || m == 0 {
			return
		}
	}
}

// send encodes a portrait frame and streams it. Failures are handled in
// steps: a write error reopens the device once right away and resends; a
// missing acknowledgement is first checked with a buffer level query; only if
// that stays unanswered too the device is closed and a reconnect scheduled
// (the frame is dropped).
func (p *universal88) send(img image.Image) error {
	if p.closed {
		return fmt.Errorf("%s: closed", p.name)
	}
	if err := p.reopen(); err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	if err := p.leaveLayers(); err != nil {
		return err
	}
	jpg, err := p.encodeJPEG(img)
	if err != nil {
		return err
	}
	return p.sendData(usOpPicture, jpg)
}

// encodeJPEG encodes img, lowering the quality until it fits the device.
func (p *universal88) encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	for q := 90; ; q -= 10 {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, err
		}
		if buf.Len() <= p.MaxJPEG() {
			return buf.Bytes(), nil
		}
		if q <= 30 {
			return nil, fmt.Errorf("%s: frame too large (%d bytes)", p.name, buf.Len())
		}
	}
}

// sendData streams one picture or overlay command with its data and handles
// failures in steps (see send).
func (p *universal88) sendData(op usOp, data []byte) error {
	r, acked, err := p.transferOp(op, data)
	if err != nil {
		// The handle may have gone stale (standby, brief unplug): reopen and
		// initialise once, then try again.
		p.disconnect()
		if cerr := p.connect(); cerr != nil {
			p.backoff()
			return fmt.Errorf("%s: %w (reopen: %v)", p.name, err, cerr)
		}
		if r, acked, err = p.transferOp(op, data); err != nil {
			p.recover()
			return fmt.Errorf("%s: %w", p.name, err)
		}
	}
	if !acked {
		// The frame may well have arrived; ask the device before giving up.
		if r, err = p.query(usOpQueue); err != nil {
			p.recover()
			return fmt.Errorf("%s: no acknowledgement: %w", p.name, err)
		}
	}
	p.failures = 0
	// The ack carries the device's frame buffer level; wait while it is full.
	if len(r) > 8 && r[8] > usBusyAbove {
		for i := 0; i < us88BufPolls; i++ {
			q, err := p.query(usOpQueue)
			if err != nil && !errors.Is(err, winusb.ErrTimeout) {
				p.recover()
				return fmt.Errorf("%s: %w", p.name, err)
			}
			if err != nil || len(q) <= 8 || q[8] <= usIdleAt {
				break
			}
			time.Sleep(us88BufGap)
		}
	}
	return nil
}

func (p *universal88) SetBrightness(percent int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.brightness = max(0, min(100, percent))
	if p.dev == nil || p.closed {
		return nil // sent by the next connect
	}
	woke := p.dark && p.brightness > 0
	if err := p.setBacklight(false); err != nil {
		p.recover()
		return err
	}
	p.dark = p.brightness == 0
	if woke && p.mdl().hwTurn {
		// The rotation does not reliably survive the display being off.
		return p.sendRotation(true)
	}
	return nil
}

// SetRotation turns the picture in software; the device's own rotate command
// is never sent. 0 and 180 turn the landscape frame. 90 and 270 are for a bar
// mounted upright: the engine then renders a 480x1920 portrait canvas (see
// SoftQuarterTurns) in which every field stands upright, and no picture is
// rotated in the engine. 90 means the content is turned clockwise, i.e. the
// right edge of the landscape bar is up; 270 has the left edge up.
func (p *universal88) SetRotation(degrees int) error {
	switch degrees {
	case 0, 90, 180, 270:
	default:
		return fmt.Errorf("rotation must be 0, 90, 180 or 270")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rotation = degrees
	p.lay.invalidate() // the layers are drawn for the old orientation
	if p.mdl().hwTurn && p.dev != nil && !p.closed {
		return p.sendRotation(false)
	}
	if p.devFlip() && p.dev != nil && !p.closed {
		return p.sendBarTurn()
	}
	return nil
}

// setBacklight writes the backlight unless the device already holds the
// value (the device may store it). With force it always writes.
func (p *universal88) setBacklight(force bool) error {
	v := p.mdl().devBrightness(p.brightness)
	if !force && p.sentBright == int(v)+1 {
		return nil
	}
	if _, err := p.issue(usOpBacklight, []byte{v}); err != nil {
		return err
	}
	p.sentBright = int(v) + 1
	Cache().PutBrightness(p.SettingsKey(), p.brightness)
	return nil
}

// SettingsKey identifies the device in the SettingsCache.
func (p *universal88) SettingsKey() string { return "us-" + p.id }

// us88DevHeld is the sentBright marker for a brightness in percent.
func us88DevHeld(m *usModel, percent int) int { return int(m.devBrightness(percent)) + 1 }

// sendRotation tells the device to turn its picture. The wire value is the
// requested quarter turn plus the model's mounting offset. Unless force is
// set, an unchanged value is not written again (the device may store it).
func (p *universal88) sendRotation(force bool) error {
	v := p.mdl().wireTurn(p.rotation)
	if !force && p.sentTurn == int(v)+1 {
		return nil
	}
	if _, err := p.issue(usOpTurn, []byte{v}); err != nil {
		p.recover()
		return err
	}
	p.sentTurn = int(v) + 1
	Cache().PutRotation(p.SettingsKey(), int(v))
	return nil
}

// SoftQuarterTurns tells the engine that at 90 and 270 degrees this display
// wants a portrait canvas (480x1920) instead of a turned landscape picture.
func (p *universal88) SoftQuarterTurns() bool { return !p.mdl().square }

// us88Upright maps an upright portrait canvas (480x1920) to the device
// frame. The device frame is D(x,y) = L(y, 479-x) for a landscape picture L.
// Rotation 270 (left edge of the bar up) shows the canvas as it is; 90 (right
// edge up) is the same turned by 180 degrees.
func us88Upright(src *image.RGBA, rotation int) *image.RGBA {
	if rotation != 90 {
		return src
	}
	dst := image.NewRGBA(src.Bounds())
	w, h := src.Rect.Dx(), src.Rect.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			copy(dst.Pix[dst.PixOffset(x, y):][:4], src.Pix[src.PixOffset(w-1-x, h-1-y):][:4])
		}
	}
	return dst
}

// Close leaves the device in a defined state and releases the handle: bars
// stop playback; the Flex screen is handed back to the firmware's own theme
// (a mode switch, nothing is stored); the SL V3 needs nothing.
func (p *universal88) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	p.lay.stopVideo()
	if p.stop != nil {
		close(p.stop)
	}
	if p.dev != nil {
		m := p.mdl()
		switch m.init {
		case usInitFlex:
			if p.write(p.withData(usOpOverlay, usOverlay(m.short, m.short))) == nil {
				p.ack()
				time.Sleep(usFlexHandOverGap)
				if p.write(p.seal(usOpHandOver, nil)) == nil {
					time.Sleep(usFlexHandOverGap)
				}
			}
		case usInitSL:
		default:
			if p.write(p.seal(usOpHalt, nil)) == nil {
				time.Sleep(us88CloseGap)
				p.showExit()
			}
		}
	}
	p.disconnect()
	return nil
}

// SetExitImage implements ExitImager: only the 8.8" takes it.
func (p *universal88) SetExitImage(img image.Image) {
	if !p.mdl().exitImage {
		return
	}
	p.mu.Lock()
	p.exit = img
	p.mu.Unlock()
}

// showExit sends the exit image as an ordinary frame (turned like every
// other), then an empty overlay so no old sensor layer stays on top. The
// acknowledgements are read once each; any error ends it (Close goes on).
func (p *universal88) showExit() {
	m := p.mdl()
	if !m.exitImage || p.exit == nil || p.checkSize(p.exit.Bounds().Size()) != nil {
		return
	}
	jpg, err := p.encodeJPEG(p.orient(toRGBA(p.exit)))
	if err != nil || p.write(p.picture(jpg)) != nil {
		return
	}
	p.ack()
	if p.write(p.withData(usOpOverlay, usOverlay(m.short, m.long))) == nil {
		p.ack()
	}
}

// usFlexHandOverGap is the pause around the hand-over on Close.
var usFlexHandOverGap = 300 * time.Millisecond

// usCheckWait is the pause before the answer to the new-LCD query is read.
var usCheckWait = 20 * time.Millisecond

// usCheck asks the device whether it is a "new" LCD. No answer means the
// device is offline.
func (p *universal88) usCheck() (isNew bool, err error) {
	if err := p.write(p.seal88(usOpCheck, nil)); err != nil {
		return false, err
	}
	time.Sleep(usCheckWait)
	r, err := p.ack()
	if err != nil {
		if p.mdl().lenient {
			return true, nil // answers may arrive elsewhere; the write worked
		}
		return false, fmt.Errorf("device did not answer: %w", err)
	}
	return len(r) > 8 && r[0] == byte(usOpCheck) && r[8] == 2, nil
}

// wakeSL initialises an SL V3 / TL V2 LCD: new-LCD query (8.8" framing), the
// fixed frame rate for old LCDs, rotation and backlight.
func (p *universal88) wakeSL() error {
	isNew, err := p.usCheck()
	if err != nil {
		return err
	}
	if !isNew {
		p.issue(usOpFPS, []byte{120})
	}
	r, _ := p.issue(usOpInfo, nil) // informational only
	p.noteFirmware(r)
	if err := p.sendRotation(true); err != nil {
		return err
	}
	err = p.setBacklight(false)
	p.dark = p.brightness == 0
	p.lastIO = time.Now()
	return err
}

// wakeFlex initialises a Flex fan screen: new-LCD query, version, one empty
// overlay, rotation and backlight. The frame-rate command is not sent.
func (p *universal88) wakeFlex() error {
	if _, err := p.usCheck(); err != nil {
		return err
	}
	r, _ := p.issue88(usOpInfo, nil)
	p.noteFirmware(r)
	m := p.mdl()
	if err := p.write(p.withData(usOpOverlay, usOverlay(m.short, m.short))); err != nil {
		return err
	}
	p.ack()
	if err := p.sendRotation(true); err != nil {
		return err
	}
	err := p.setBacklight(false)
	p.dark = p.brightness == 0
	return err
}

// heartbeatEvery is how long the SL V3 may stay silent before it is asked
// for its version.
const heartbeatEvery = time.Second

// heartbeatLoop keeps an SL V3 LCD alive until Close.
func (p *universal88) heartbeatLoop() {
	t := time.NewTicker(heartbeatEvery / 4)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case now := <-t.C:
			p.heartbeat(now)
		}
	}
}

// heartbeat sends the version query when nothing went out for a second. A
// device that does not answer is dropped and reconnected with the next frame.
func (p *universal88) heartbeat(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.dev == nil || now.Sub(p.lastIO) < heartbeatEvery {
		return
	}
	// The answer comes asynchronously (and on this device possibly not on the
	// bulk pipe), so only a failed write counts as lost.
	if err := p.write(p.seal(usOpInfo, nil)); err != nil {
		p.recover()
		return
	}
	p.drain()
}
