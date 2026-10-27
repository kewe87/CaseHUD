package panel

// Bars only: the device clock, the picture-plus-overlay mode and playback of
// an H.264 file on the device. All of it uses commands of the same family as
// the JPEG stream (see us88wire.go); none of it writes anything the device
// keeps, except the offline-clock flag, which is written only when the user
// changes the setting.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/maphash"
	"image"
	"io"
	"os"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

// Device clock (0x33): year (u16 BE), month, day, hour, minute, second, flag.
const (
	usClockSetOnly = 2 // set the time, nothing else
	usClockOn      = 1 // keep the clock as the offline display (stored)
	usClockOffFlag = 0 // no offline clock (stored)
)

// usNow is the clock used for the device time (replaced in tests).
var usNow = time.Now

// usTimeArgs are the arguments of the clock command for local time t.
func usTimeArgs(t time.Time, flag byte) []byte {
	y := min(max(t.Year(), 0), 65535)
	return []byte{byte(y >> 8), byte(y), byte(t.Month()), byte(t.Day()),
		byte(t.Hour()), byte(t.Minute()), byte(t.Second()), flag}
}

// SetOfflineClock switches the device's offline clock (the clock it shows
// when no host feeds it). The flag is kept by the device, so it is written
// only when it differs from what the device was last given (SettingsCache);
// a value that was never written counts as off. The current time is sent
// with it. A device whose clock was on and now shows host frames again needs
// the clock hidden, which is done right after.
func (p *universal88) SetOfflineClock(on bool) error {
	if !p.mdl().clock {
		return fmt.Errorf("%s: no offline clock", p.name)
	}
	key := p.SettingsKey()
	p.mu.Lock()
	defer p.mu.Unlock()
	if w := Cache().Get(key); (w.OfflineClock != nil && *w.OfflineClock == on) || (w.OfflineClock == nil && !on) {
		p.offlineClock = on
		return nil
	}
	if p.dev == nil || p.closed {
		return fmt.Errorf("%s: not connected", p.name)
	}
	flag := byte(usClockOffFlag)
	if on {
		flag = usClockOn
	}
	if _, err := p.issue(usOpClock, usTimeArgs(usNow(), flag)); err != nil {
		p.recover()
		return err
	}
	p.offlineClock = on
	Cache().PutOfflineClock(key, on)
	if on {
		// Show host frames again; the flag stays.
		if _, err := p.issue(usOpClockOff, []byte{0}); err != nil {
			p.recover()
			return err
		}
	}
	return nil
}

// OfflineClock reports the flag the device was last given.
func (p *universal88) OfflineClock() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.offlineClock
}

// --- device-side rotation ------------------------------------------------

// Device-side rotation: the bar turns the picture by 180 degrees itself (0x0D 2)
// instead of the host mirroring the frame. The device only knows 0 and 180
// degrees. The 8.8" always does this (fixedFlip); on other bars it is an
// experimental per-device switch (SetDeviceFlip), off by default, because it
// has not been checked on their hardware. With it on, rotation 180 and 90
// write 0x0D 2, 0 and 270 write 0x0D 0; the host only lays the picture on its
// side (and 90 is drawn like 270, the device turns it).

// SetDeviceFlip implements DeviceFlipper. It changes what the next frames look
// like (the host stops or starts mirroring) and sends the turn right away when
// the rotation is 180; an unchanged value is not written again.
func (p *universal88) SetDeviceFlip(on bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.mdl().turn180 {
		return fmt.Errorf("%s: no device-side rotation", p.name)
	}
	if p.flipOn == on {
		return nil
	}
	p.flipOn = on
	p.lay.invalidate() // pictures, layers and video are drawn for the other orientation
	if p.dev != nil && !p.closed {
		return p.sendBarTurn()
	}
	return nil
}

func (p *universal88) devFlip() bool {
	m := p.mdl()
	return m.fixedFlip || p.flipOn && m.turn180
}

// barTurn is the device rotation value of a bar: 2 for a device-side flip, else 0.
func (p *universal88) barTurn() byte {
	if p.devFlip() && (p.rotation == 180 || p.rotation == 90) {
		return 2
	}
	return 0
}

// sendBarTurn writes the bar's rotation when it differs from what the device holds.
func (p *universal88) sendBarTurn() error {
	v := p.barTurn()
	if p.sentTurn == int(v)+1 {
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

// --- layers ----------------------------------------------------------------

// usMaxPNG is the largest overlay the device takes (a variable for tests).
var usMaxPNG = 1 << 20

const (
	usVideoBlock   = 202752 // largest block of a video stream
	usMaxVideoFile = 128 << 20
)

// Pacing of the layer mode and the video stream (variables so tests can speed
// them up).
var (
	// usLayerGap is the shortest time between two different overlays (5 Hz).
	usLayerGap = 200 * time.Millisecond
	// usLayerKeepalive repeats an unchanged overlay now and then, like a
	// frame, so the device keeps showing host content.
	usLayerKeepalive = time.Second

	usVideoGap      = 30 * time.Millisecond // after every block
	usVideoBusyWait = 50 * time.Millisecond // before the first queue poll
	usVideoPollGap  = 50 * time.Millisecond // between queue polls
	usVideoPolls    = 200                   // most polls before the stream gives up
)

var layerSeed = maphash.MakeSeed()

// layerState is what the panel knows about the picture and the overlay on the
// device (under universal88.mu).
type layerState struct {
	bgSent bool
	bgKey  uint64

	ovSent bool      // the overlay below is on the device
	ovOn   bool      // the overlay layer may show something
	ovHash uint64    // hash of the last overlay's pixels
	ovAt   time.Time // when it went out
	png    []byte    // its encoding, for the keep-alive

	vid *videoRun
}

// invalidate forgets what the device shows (the host picture changed, the
// orientation changed ...), so the next frame sends everything again.
func (l *layerState) invalidate() { l.bgSent, l.ovSent, l.png = false, false, nil }

// reset is invalidate for a device whose state is unknown (new connection).
func (l *layerState) reset() {
	l.invalidate()
	l.ovOn = false
	l.stopVideo()
}

// stopVideo ends the stream's goroutine; it does not talk to the device.
func (l *layerState) stopVideo() {
	if l.vid != nil {
		close(l.vid.cancel)
		l.vid = nil
	}
}

// LayerCaps implements LayeredPanel.
func (p *universal88) LayerCaps() LayerCaps {
	m := p.mdl()
	return LayerCaps{Static: m.layers, Video: m.video}
}

// VideoTurn implements VideoOrienter: the quarter turns (clockwise) from the
// canvas for the given rotation to the device frame (see orient).
func (p *universal88) VideoTurn(rotation int) int {
	switch rotation {
	case 90:
		if p.devFlip() {
			return 0 // drawn like 270, the device turns it
		}
		return 2
	case 180:
		if p.devFlip() {
			return 1
		}
		return 3
	case 270:
		return 0
	}
	return 1 // landscape canvas: a quarter turn clockwise
}

// leaveLayers is called before a plain JPEG frame goes out: video playback
// ends and a visible overlay is cleared, or it would stay on top of the picture.
func (p *universal88) leaveLayers() error {
	if p.lay.vid != nil {
		p.haltVideo()
	}
	if p.lay.ovOn {
		m := p.mdl()
		if err := p.write(p.withData(usOpOverlay, usOverlay(m.short, m.long))); err != nil {
			p.recover() // the handle is stale: reopen with the next frame
			return fmt.Errorf("%s: %w", p.name, err)
		}
		p.ack()
		p.lay.ovOn = false
	}
	p.lay.invalidate()
	return nil
}

// haltVideo stops the stream and the playback on the device.
func (p *universal88) haltVideo() {
	p.lay.stopVideo()
	if p.dev != nil && p.write(p.seal(usOpHalt, nil)) == nil {
		time.Sleep(us88CloseGap)
	}
}

// ShowLayers implements LayeredPanel.
func (p *universal88) ShowLayers(b Backdrop, overlay *image.RGBA) error {
	m := p.mdl()
	if !m.layers {
		return fmt.Errorf("%s: no layers", p.name)
	}
	if overlay != nil {
		if err := p.checkSize(overlay.Bounds().Size()); err != nil {
			return err
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("%s: closed", p.name)
	}
	if err := p.reopen(); err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	if b.Video != nil {
		if !m.video {
			return fmt.Errorf("%s: no video", p.name)
		}
		if err := p.playVideo(b.Video); err != nil {
			return err
		}
		return p.sendOverlay(overlay, nil)
	}
	if p.lay.vid != nil {
		p.haltVideo()
		p.lay.bgSent = false
	}
	if b.Image == nil {
		return fmt.Errorf("%s: no picture", p.name)
	}
	if !p.lay.bgSent || p.lay.bgKey != b.Key {
		img := b.Image()
		if err := p.checkSize(img.Bounds().Size()); err != nil {
			return err
		}
		jpg, err := p.encodeJPEG(p.orient(img))
		if err != nil {
			return err
		}
		if err := p.sendData(usOpPicture, jpg); err != nil {
			return err
		}
		p.lay.bgSent, p.lay.bgKey = true, b.Key
	}
	return p.sendOverlay(overlay, b.Image)
}

// overlayPNG orients ov and encodes it as a PNG with straight alpha.
func (p *universal88) overlayPNG(ov *image.RGBA) ([]byte, error) {
	m := p.mdl()
	if ov == nil {
		return usOverlay(m.short, m.long), nil
	}
	o := p.orient(ov)
	n := image.NewNRGBA(o.Rect)
	for y := 0; y < o.Rect.Dy(); y++ {
		src := o.Pix[y*o.Stride : y*o.Stride+o.Rect.Dx()*4]
		dst := n.Pix[y*n.Stride : y*n.Stride+o.Rect.Dx()*4]
		for i := 0; i+3 < len(src); i += 4 {
			a := uint32(src[i+3])
			switch a {
			case 0:
			case 0xff:
				copy(dst[i:i+4], src[i:i+4])
			default:
				dst[i] = uint8(min(255, (uint32(src[i])*255+a/2)/a))
				dst[i+1] = uint8(min(255, (uint32(src[i+1])*255+a/2)/a))
				dst[i+2] = uint8(min(255, (uint32(src[i+2])*255+a/2)/a))
				dst[i+3] = uint8(a)
			}
		}
	}
	return encodePNGFast(n)
}

// sendOverlay puts ov on the overlay layer. An unchanged overlay is only
// repeated once per usLayerKeepalive, a changed one waits until usLayerGap has
// passed since the last. A overlay the device would not take (too large)
// falls back to one composed JPEG, when the picture is known.
func (p *universal88) sendOverlay(ov *image.RGBA, bg func() *image.RGBA) error {
	var h uint64
	if ov != nil {
		h = maphash.Bytes(layerSeed, ov.Pix) | 1 // never 0, which stands for "nothing"
	}
	now := time.Now()
	same := p.lay.ovSent && h == p.lay.ovHash
	if same && now.Sub(p.lay.ovAt) < usLayerKeepalive {
		return nil
	}
	if !same && p.lay.ovSent {
		if wait := usLayerGap - now.Sub(p.lay.ovAt); wait > 0 {
			time.Sleep(wait)
		}
	}
	data := p.lay.png
	if !same || data == nil {
		var err error
		if data, err = p.overlayPNG(ov); err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
	}
	if len(data) > usMaxPNG {
		return p.composeFallback(ov, bg)
	}
	if p.lay.vid != nil && p.lay.vid.fps > 0 {
		// The device expects the video frame rate to be sent again before every overlay.
		if p.write(p.seal(usOpFPS, []byte{byte(p.lay.vid.fps)})) == nil {
			p.drain()
		}
	}
	if err := p.sendData(usOpOverlay, data); err != nil {
		return err
	}
	p.lay.ovSent, p.lay.ovHash, p.lay.ovAt, p.lay.png = true, h, time.Now(), data
	p.lay.ovOn = ov != nil
	return nil
}

// composeFallback sends picture and overlay as one JPEG frame.
func (p *universal88) composeFallback(ov *image.RGBA, bg func() *image.RGBA) error {
	if bg == nil {
		return fmt.Errorf("%s: overlay too large", p.name)
	}
	base := bg()
	img := image.NewRGBA(base.Rect)
	copy(img.Pix, base.Pix)
	usOver(img, ov)
	return p.send(p.orient(img)) // clears the overlay layer and invalidates the layers
}

// usOver composites the premultiplied src onto the opaque dst of the same size.
func usOver(dst, src *image.RGBA) {
	d, s := dst.Pix, src.Pix
	for i := 0; i+3 < len(s) && i+3 < len(d); i += 4 {
		a := uint32(s[i+3])
		switch a {
		case 0:
		case 0xff:
			d[i], d[i+1], d[i+2] = s[i], s[i+1], s[i+2]
		default:
			k := 255 - a
			d[i] = uint8(uint32(s[i]) + (uint32(d[i])*k+127)/255)
			d[i+1] = uint8(uint32(s[i+1]) + (uint32(d[i+1])*k+127)/255)
			d[i+2] = uint8(uint32(s[i+2]) + (uint32(d[i+2])*k+127)/255)
		}
	}
}

// --- video -------------------------------------------------------------------

// videoRun is one running stream.
type videoRun struct {
	path   string
	fps    int
	cancel chan struct{}
	done   chan struct{}
	err    error // why the stream ended by itself (written before done closes)
}

func (r *videoRun) cancelled() bool {
	select {
	case <-r.cancel:
		return true
	default:
		return false
	}
}

func (r *videoRun) finished() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// sleep waits d or until the stream is cancelled; false means cancelled.
func (r *videoRun) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-r.cancel:
		return false
	case <-t.C:
		return true
	}
}

// playVideo makes sure v is playing: it starts the stream, or restarts it when
// another file is wanted or the stream ended (device lost).
func (p *universal88) playVideo(v *DeviceVideo) error {
	if r := p.lay.vid; r != nil {
		if r.path == v.Path && !r.finished() {
			return nil
		}
		p.haltVideo()
	}
	fps := v.FPS
	if fps < 1 || fps > 119 {
		return fmt.Errorf("%s: video frame rate %d out of range", p.name, fps)
	}
	f, err := os.Open(v.Path)
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 || st.Size() > usMaxVideoFile {
		f.Close()
		return fmt.Errorf("%s: video file unusable", p.name)
	}
	// A picture the device holds is replaced by the video.
	p.lay.bgSent = false
	if err := p.write(p.seal(usOpHalt, nil)); err != nil {
		f.Close()
		p.recover()
		return fmt.Errorf("%s: %w", p.name, err)
	}
	time.Sleep(us88CloseGap)
	if err := p.write(p.seal(usOpFPS, []byte{byte(fps)})); err != nil {
		f.Close()
		p.recover()
		return fmt.Errorf("%s: %w", p.name, err)
	}
	p.drain()
	r := &videoRun{path: v.Path, fps: fps, cancel: make(chan struct{}), done: make(chan struct{})}
	p.lay.vid = r
	go p.videoLoop(r, f, st.Size())
	return nil
}

// videoLoop sends the file in blocks, over and over, until cancelled. The
// panel lock is held for one block at a time.
func (p *universal88) videoLoop(r *videoRun, f *os.File, size int64) {
	defer close(r.done)
	defer f.Close()
	buf := make([]byte, usVideoBlock)
	var off int64
	for {
		n, err := io.ReadFull(f, buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			r.err = err
			return
		}
		if n == 0 { // the file shrank under us
			r.err = io.ErrUnexpectedEOF
			return
		}
		off += int64(n)
		last := off >= size || err != nil
		p.mu.Lock()
		if r.cancelled() || p.dev == nil || p.closed {
			p.mu.Unlock()
			return
		}
		busy, werr := p.videoBlock(buf[:n], last)
		if werr != nil {
			r.err = werr
			p.recover()
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
		if !r.sleep(usVideoGap) {
			return
		}
		if busy && !p.videoDrain(r) {
			return
		}
		if last {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				r.err = err
				return
			}
			off = 0
		}
	}
}

// videoBlock writes one block (caller holds p.mu) and reports whether the
// device's queue is above the busy mark.
func (p *universal88) videoBlock(data []byte, last bool) (busy bool, err error) {
	if len(data) == 0 || len(data) > usVideoBlock {
		return false, fmt.Errorf("%s: video block of %d bytes", p.name, len(data))
	}
	var args [5]byte
	binary.BigEndian.PutUint32(args[:4], uint32(len(data)))
	if last {
		args[4] = 1
	}
	if err := p.write(append(p.seal(usOpVideo, args[:]), data...)); err != nil {
		return false, err
	}
	reply, aerr := p.ack()
	if aerr != nil && !errors.Is(aerr, winusb.ErrTimeout) {
		return false, aerr
	}
	return aerr == nil && len(reply) > 8 && reply[8] > usBusyAbove, nil
}

// videoDrain waits for the device's queue to go down: it polls every 50 ms
// until at most usIdleAt blocks remain. False means cancelled or given up.
func (p *universal88) videoDrain(r *videoRun) bool {
	if !r.sleep(usVideoBusyWait) {
		return false
	}
	for i := 0; i < usVideoPolls; i++ {
		p.mu.Lock()
		if r.cancelled() || p.dev == nil || p.closed {
			p.mu.Unlock()
			return false
		}
		q, err := p.query(usOpQueue)
		if err != nil && !errors.Is(err, winusb.ErrTimeout) {
			r.err = err
			p.recover()
			p.mu.Unlock()
			return false
		}
		p.mu.Unlock()
		if err != nil || len(q) <= 8 || q[8] <= usIdleAt {
			return true
		}
		if !r.sleep(usVideoPollGap) {
			return false
		}
	}
	r.err = errors.New("video queue stayed full")
	p.mu.Lock()
	if !r.cancelled() && !p.closed {
		p.recover()
	}
	p.mu.Unlock()
	return false
}

var (
	_ LayeredPanel   = (*universal88)(nil)
	_ VideoOrienter  = (*universal88)(nil)
	_ OfflineClocker = (*universal88)(nil)
)

// CanDeviceFlip implements DeviceFlipper.
func (p *universal88) CanDeviceFlip() bool { return p.mdl().turn180 }
