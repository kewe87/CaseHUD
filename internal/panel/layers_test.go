package panel

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

// lyUSB is a scripted device that is safe to use from the video goroutine.
type lyUSB struct {
	mu     sync.Mutex
	writes [][]byte
	at     []time.Time
	queue  [][]byte
	script func(op byte, plain []byte) []byte
}

func (s *lyUSB) Write(ep byte, buf []byte, _ int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, append([]byte(nil), buf...))
	s.at = append(s.at, time.Now())
	if len(buf) >= 512 && s.script != nil {
		plain := decryptBlock(buf[:504])
		if r := s.script(plain[0], plain); r != nil {
			s.queue = append(s.queue, r)
		}
	}
	return len(buf), nil
}

func (s *lyUSB) Read(ep byte, buf []byte, _ int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return 0, winusb.ErrTimeout
	}
	n := copy(buf, s.queue[0])
	s.queue = s.queue[1:]
	return n, nil
}

func (s *lyUSB) ClearHalt(byte) error           { return nil }
func (s *lyUSB) ZeroLengthTerminate(byte) error { return nil }
func (s *lyUSB) Close() error                   { return nil }

func (s *lyUSB) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

// take returns and forgets the writes so far.
func (s *lyUSB) take() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, a := s.writes, s.at
	_ = a
	s.writes, s.at = nil, nil
	return w
}

func lyOps(ws [][]byte) []byte {
	var ops []byte
	for _, w := range ws {
		ops = append(ops, decryptBlock(w[:504])[0])
	}
	return ops
}

func fastLayers(t *testing.T) {
	t.Helper()
	oldGap, oldKeep, oldV, oldB, oldP := usLayerGap, usLayerKeepalive, usVideoGap, usVideoBusyWait, usVideoPollGap
	usLayerGap, usLayerKeepalive = 0, time.Hour
	usVideoGap, usVideoBusyWait, usVideoPollGap = time.Millisecond, time.Millisecond, time.Millisecond
	t.Cleanup(func() {
		usLayerGap, usLayerKeepalive, usVideoGap, usVideoBusyWait, usVideoPollGap = oldGap, oldKeep, oldV, oldB, oldP
	})
}

// barOn builds a connected bar of model m on a scripted device and clears its
// start-up traffic.
func barOn(t *testing.T, m *usModel, script func(byte, []byte) []byte) (*universal88, *lyUSB) {
	t.Helper()
	dev := &lyUSB{script: script}
	old := us88Open
	us88Open = func(winusb.Info) (winusb.Device, error) { return dev, nil }
	t.Cleanup(func() { us88Open = old })
	p := &universal88{m: m, id: m.idPrefix + "-0", name: m.name, brightness: 100}
	if err := p.connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	dev.take()
	return p, dev
}

func ack0(op byte, _ []byte) []byte { return reply(op, 0) }

func newCache(t *testing.T) *SettingsCache {
	t.Helper()
	c := NewSettingsCache(nil)
	old := Cache()
	SetSettingsCache(c)
	t.Cleanup(func() { SetSettingsCache(old) })
	return c
}

func TestClockIsSetOnConnect(t *testing.T) {
	old := usNow
	usNow = func() time.Time { return time.Date(2026, 10, 21, 14, 30, 45, 0, time.Local) }
	defer func() { usNow = old }()
	for _, m := range []*usModel{&usModel88, &usModel92, &usModel207} {
		dev := &lyUSB{script: ack0}
		p := &universal88{dev: dev, m: m, brightness: 100}
		if err := p.wake(); err != nil {
			t.Fatal(err)
		}
		var found bool
		for i, w := range dev.writes {
			pl := decryptBlock(w[:504])
			if pl[0] != byte(usOpClock) {
				continue
			}
			found = true
			if want := []byte{0x07, 0xEA, 10, 21, 14, 30, 45, 2}; !bytes.Equal(pl[8:16], want) {
				t.Fatalf("%s: clock args % x, want % x", m.name, pl[8:16], want)
			}
			if i != 1 {
				t.Fatalf("%s: clock is command %d, want right after the halt", m.name, i)
			}
		}
		if !found {
			t.Fatalf("%s: time not set", m.name)
		}
	}
	// Devices without the clock command never get it.
	dev := &lyUSB{script: ack0}
	p := &universal88{dev: dev, m: &usModelTLFlex, brightness: 100}
	p.wake()
	for _, op := range lyOps(dev.writes) {
		if op == byte(usOpClock) {
			t.Fatal("flex got the clock command")
		}
	}
}

func TestOfflineClockOnWakeKeepsTheDeviceClock(t *testing.T) {
	for _, keep := range []bool{false, true} {
		dev := &lyUSB{script: ack0}
		p := &universal88{dev: dev, brightness: 100, offlineClock: keep}
		if err := p.wake(); err != nil {
			t.Fatal(err)
		}
		hidden := false
		for _, op := range lyOps(dev.writes) {
			hidden = hidden || op == byte(usOpClockOff)
		}
		if hidden == keep {
			t.Fatalf("offline clock %v: clock hidden = %v", keep, hidden)
		}
	}
}

func TestOfflineClockWrittenOnlyOnToggle(t *testing.T) {
	cache := newCache(t)
	p, dev := barOn(t, &usModel88, ack0)
	flags := func() []byte {
		var f []byte
		for _, w := range dev.take() {
			if pl := decryptBlock(w[:504]); pl[0] == byte(usOpClock) {
				f = append(f, pl[15])
			}
		}
		return f
	}
	// Never decided: off writes nothing, however often it is asked.
	for i := 0; i < 3; i++ {
		if err := p.SetOfflineClock(false); err != nil {
			t.Fatal(err)
		}
	}
	if len(dev.take()) != 0 {
		t.Fatal("off written although never switched on")
	}
	// On: one write of flag 1, then the clock is hidden so host frames show.
	if err := p.SetOfflineClock(true); err != nil {
		t.Fatal(err)
	}
	ws := dev.take()
	if ops := lyOps(ws); !bytes.Equal(ops, []byte{byte(usOpClock), byte(usOpClockOff)}) {
		t.Fatalf("ops on switching on: % x", ops)
	}
	if pl := decryptBlock(ws[0][:504]); pl[15] != 1 {
		t.Fatalf("flag %d, want 1", pl[15])
	}
	if w := cache.Get(p.SettingsKey()); w.OfflineClock == nil || !*w.OfflineClock {
		t.Fatal("write not recorded")
	}
	// Same value again, also from a new panel object: nothing is written.
	for i := 0; i < 3; i++ {
		p.SetOfflineClock(true)
	}
	q, dev2 := barOn(t, &usModel88, ack0)
	q.SetOfflineClock(true)
	if dev.count() != 0 || dev2.count() != 0 {
		t.Fatal("unchanged offline clock written again")
	}
	// Off after on: exactly one write of flag 0.
	if err := p.SetOfflineClock(false); err != nil {
		t.Fatal(err)
	}
	if f := flags(); !bytes.Equal(f, []byte{0}) {
		t.Fatalf("flags written when switching off: %v", f)
	}
	p.SetOfflineClock(false)
	if dev.count() != 0 {
		t.Fatal("off written twice")
	}
	if p.OfflineClock() {
		t.Fatal("state not off")
	}
	// A device that is not connected reports it and keeps its old state.
	p.disconnect()
	if err := p.SetOfflineClock(true); err == nil {
		t.Fatal("no error without a device")
	}
	// Only bars have the setting.
	f := &universal88{dev: &lyUSB{}, m: &usModelTLFlex}
	if f.SetOfflineClock(true) == nil {
		t.Fatal("flex accepted the offline clock")
	}
}

func bigCanvas(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 480))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func overlayWith(x, y int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 480))
	img.SetRGBA(x, y, c)
	return img
}

func TestLayersPictureOnceOverlayOnChange(t *testing.T) {
	fastLayers(t)
	p, dev := barOn(t, &usModel88, ack0)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return bigCanvas(color.RGBA{10, 20, 30, 255}) }}
	ov := overlayWith(5, 7, color.RGBA{64, 0, 0, 128}) // premultiplied: straight (128,0,0) at 50 %

	if err := p.ShowLayers(bg, ov); err != nil {
		t.Fatal(err)
	}
	ws := dev.take()
	if ops := lyOps(ws); !bytes.Equal(ops, []byte{byte(usOpPicture), byte(usOpOverlay)}) {
		t.Fatalf("first frame: ops % x", ops)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(ws[0][512:]))
	if err != nil || cfg.Width != 480 || cfg.Height != 1920 {
		t.Fatalf("picture %+v %v", cfg, err)
	}
	if n := binary.BigEndian.Uint32(decryptBlock(ws[1][:504])[8:12]); int(n) != len(ws[1])-512 || n > 1<<20 {
		t.Fatalf("overlay length field %d for %d bytes", n, len(ws[1])-512)
	}
	img, err := png.Decode(bytes.NewReader(ws[1][512:]))
	if err != nil || img.Bounds().Dx() != 480 || img.Bounds().Dy() != 1920 {
		t.Fatalf("overlay %v %v", img, err)
	}
	// frame(x, y) = L(y, 479-x): the pixel (5, 7) lands at x = 479-7, y = 5.
	got := color.NRGBAModel.Convert(img.At(472, 5)).(color.NRGBA)
	if got != (color.NRGBA{128, 0, 0, 128}) {
		t.Fatalf("overlay pixel %+v, want straight alpha {128 0 0 128}", got)
	}
	if _, _, _, a := img.At(100, 100).RGBA(); a != 0 {
		t.Fatal("rest of the overlay not transparent")
	}

	// The same frame again: nothing goes out (keep-alive is an hour away).
	for i := 0; i < 5; i++ {
		if err := p.ShowLayers(bg, ov); err != nil {
			t.Fatal(err)
		}
	}
	if dev.count() != 0 {
		t.Fatalf("unchanged frame wrote %d times", dev.count())
	}
	// A changed overlay: one PNG, no picture.
	if err := p.ShowLayers(bg, overlayWith(9, 9, color.RGBA{255, 255, 255, 255})); err != nil {
		t.Fatal(err)
	}
	if ops := lyOps(dev.take()); !bytes.Equal(ops, []byte{byte(usOpOverlay)}) {
		t.Fatalf("changed overlay: ops % x", ops)
	}
	// A new picture key: the picture goes out again, then the overlay (changed).
	bg.Key = 2
	if err := p.ShowLayers(bg, ov); err != nil {
		t.Fatal(err)
	}
	if ops := lyOps(dev.take()); !bytes.Equal(ops, []byte{byte(usOpPicture), byte(usOpOverlay)}) {
		t.Fatalf("new key: ops % x", ops)
	}
	// A nil overlay is an empty one.
	if err := p.ShowLayers(bg, nil); err != nil {
		t.Fatal(err)
	}
	ws = dev.take()
	if len(ws) != 1 {
		t.Fatalf("nil overlay wrote %d commands", len(ws))
	}
	if im, err := png.Decode(bytes.NewReader(ws[0][512:])); err != nil || im.Bounds().Dy() != 1920 {
		t.Fatal("nil overlay is not a full-frame PNG")
	}
}

func TestLayersKeepAliveAndRateLimit(t *testing.T) {
	fastLayers(t)
	usLayerKeepalive = 60 * time.Millisecond
	p, dev := barOn(t, &usModel88, ack0)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return bigCanvas(color.RGBA{}) }}
	ov := overlayWith(1, 1, color.RGBA{255, 0, 0, 255})
	p.ShowLayers(bg, ov)
	dev.take()
	time.Sleep(80 * time.Millisecond)
	p.ShowLayers(bg, ov) // unchanged, but the keep-alive is due: the cached PNG again
	if ops := lyOps(dev.take()); !bytes.Equal(ops, []byte{byte(usOpOverlay)}) {
		t.Fatalf("keep-alive: ops % x", ops)
	}
	// Different overlays are at least usLayerGap apart (5 Hz).
	usLayerGap = 200 * time.Millisecond
	usLayerKeepalive = time.Hour
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := p.ShowLayers(bg, overlayWith(2+i, 2, color.RGBA{255, 0, 0, 255})); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 380*time.Millisecond {
		t.Fatalf("3 changed overlays in %v, want 5 Hz at most", d)
	}
	if n := len(dev.take()); n != 3 {
		t.Fatalf("%d writes, want 3 overlays", n)
	}
}

func TestLayersShowImageClearsOverlayFirst(t *testing.T) {
	fastLayers(t)
	p, dev := barOn(t, &usModel88, ack0)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return bigCanvas(color.RGBA{}) }}
	p.ShowLayers(bg, overlayWith(1, 1, color.RGBA{255, 0, 0, 255}))
	dev.take()
	if err := p.ShowImage(bigCanvas(color.RGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	ws := dev.take()
	if ops := lyOps(ws); !bytes.Equal(ops, []byte{byte(usOpOverlay), byte(usOpPicture)}) {
		t.Fatalf("ops % x, want overlay then picture", ops)
	}
	if im, err := png.Decode(bytes.NewReader(ws[0][512:])); err != nil {
		t.Fatal(err)
	} else if _, _, _, a := im.At(478, 1).RGBA(); a != 0 {
		t.Fatal("overlay not cleared")
	}
	// Without a visible overlay the JPEG goes out alone.
	p.ShowImage(bigCanvas(color.RGBA{9, 9, 9, 255}))
	if ops := lyOps(dev.take()); !bytes.Equal(ops, []byte{byte(usOpPicture)}) {
		t.Fatalf("ops % x", ops)
	}
	// Going back to layers sends the picture again (the JPEG replaced it).
	p.ShowLayers(bg, nil)
	if ops := lyOps(dev.take()); !bytes.Equal(ops, []byte{byte(usOpPicture), byte(usOpOverlay)}) {
		t.Fatalf("back to layers: ops % x", ops)
	}
}

func TestLayersOversizeOverlayFallsBackToOneJPEG(t *testing.T) {
	fastLayers(t)
	old := usMaxPNG
	usMaxPNG = 300
	defer func() { usMaxPNG = old }()
	p, dev := barOn(t, &usModel88, ack0)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return bigCanvas(color.RGBA{0, 0, 0, 255}) }}
	if err := p.ShowLayers(bg, overlayWith(1, 1, color.RGBA{255, 255, 255, 255})); err != nil {
		t.Fatal(err)
	}
	ws := dev.take()
	// picture (sent first), then the overlay did not fit: the composed JPEG
	if ops := lyOps(ws); !bytes.Equal(ops, []byte{byte(usOpPicture), byte(usOpPicture)}) {
		t.Fatalf("ops % x", ops)
	}
	im, err := jpeg.Decode(bytes.NewReader(ws[1][512:]))
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := im.At(478, 1).RGBA()
	if r>>8 < 200 {
		t.Fatalf("composed frame lacks the overlay pixel (r=%d)", r>>8)
	}
	if p.lay.ovOn || p.lay.bgSent {
		t.Fatal("layer state not reset after the fallback")
	}
}

func TestLayersAreBarsOnly(t *testing.T) {
	for _, m := range []*usModel{&usModelSL, &usModelTL2, &usModelTLFlex, &usModelSLFlex} {
		p := &universal88{dev: &lyUSB{}, m: m}
		if c := p.LayerCaps(); c.Static || c.Video {
			t.Fatalf("%s offers layers", m.name)
		}
		if p.ShowLayers(Backdrop{Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, 400, 400)) }}, nil) == nil {
			t.Fatalf("%s accepted layers", m.name)
		}
	}
	for _, m := range []*usModel{&usModel88, &usModel92, &usModel207} {
		if c := (&universal88{m: m}).LayerCaps(); !c.Static || !c.Video {
			t.Fatalf("%s lacks layers", m.name)
		}
	}
}

func TestVideoTurn(t *testing.T) {
	// 8.8": the device turns by 180 degrees itself, 90 is drawn like 270.
	p := &universal88{m: &usModel88}
	for rot, want := range map[int]int{0: 1, 90: 0, 180: 1, 270: 0} {
		if got := p.VideoTurn(rot); got != want {
			t.Errorf("8.8 rotation %d: turn %d, want %d", rot, got, want)
		}
	}
	// A bar without the device turn does all of it on the host.
	q := &universal88{m: &usModel92}
	for rot, want := range map[int]int{0: 1, 90: 2, 180: 3, 270: 0} {
		if got := q.VideoTurn(rot); got != want {
			t.Errorf("9.2 rotation %d: turn %d, want %d", rot, got, want)
		}
	}
}

// writeVideo creates a file of n bytes whose block k starts with the byte k+1.
func writeVideo(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i/usVideoBlock + 1)
	}
	path := filepath.Join(t.TempDir(), "v.h264")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func lyWaitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func TestVideoBlocksFramingLoopAndStop(t *testing.T) {
	fastLayers(t)
	path := writeVideo(t, 2*usVideoBlock+1000)
	p, dev := barOn(t, &usModel88, ack0)
	bg := Backdrop{Video: &DeviceVideo{Path: path, FPS: 30}}
	if err := p.ShowLayers(bg, nil); err != nil {
		t.Fatal(err)
	}
	// frame rate first, then the blocks, then the file starts again
	lyWaitFor(t, "second pass", func() bool {
		n := 0
		dev.mu.Lock()
		defer dev.mu.Unlock()
		for _, w := range dev.writes {
			if decryptBlock(w[:504])[0] == byte(usOpVideo) {
				n++
			}
		}
		return n >= 4
	})
	p.Close() // ends the stream
	p.lay.vid = nil
	ws := dev.take()
	ops := lyOps(ws)
	// halt, frame rate, (blocks ...), overlay with its frame rate, ..., halt on close
	if ops[0] != byte(usOpHalt) || ops[1] != byte(usOpFPS) {
		t.Fatalf("stream start: % x", ops[:min(len(ops), 4)])
	}
	if f := decryptBlock(ws[1][:504]); f[8] != 30 {
		t.Fatalf("frame rate %d", f[8])
	}
	if ops[len(ops)-1] != byte(usOpHalt) {
		t.Fatalf("no stop at the end: % x", ops)
	}
	var blocks [][]byte
	for _, w := range ws {
		if decryptBlock(w[:504])[0] == byte(usOpVideo) {
			blocks = append(blocks, w)
		}
	}
	wantLens := []int{usVideoBlock, usVideoBlock, 1000}
	for i, b := range blocks {
		pl := decryptBlock(b[:504])
		n := int(binary.BigEndian.Uint32(pl[8:12]))
		if n != wantLens[i%3] || len(b) != 512+n {
			t.Fatalf("block %d: length field %d, data %d, want %d", i, n, len(b)-512, wantLens[i%3])
		}
		if n > usVideoBlock {
			t.Fatalf("block of %d bytes", n)
		}
		last := byte(0)
		if i%3 == 2 {
			last = 1
		}
		if pl[12] != last {
			t.Fatalf("block %d: last flag %d, want %d", i, pl[12], last)
		}
		if b[512] != byte(i%3+1) {
			t.Fatalf("block %d carries the wrong data", i)
		}
	}
	if len(blocks) < 4 {
		t.Fatalf("%d blocks", len(blocks))
	}
}

func TestVideoPacingAndQueuePolling(t *testing.T) {
	fastLayers(t)
	usVideoGap = 30 * time.Millisecond
	path := writeVideo(t, 3*usVideoBlock)
	var mu sync.Mutex
	polls, blocks := 0, 0
	script := func(op byte, _ []byte) []byte {
		mu.Lock()
		defer mu.Unlock()
		switch op {
		case byte(usOpVideo):
			blocks++
			if blocks == 1 {
				return reply(op, 5) // above 3: busy
			}
			return reply(op, 0)
		case byte(usOpQueue):
			polls++
			if polls <= 2 {
				return reply(op, 4) // still above 2
			}
			return reply(op, 2)
		}
		return reply(op, 0)
	}
	p, dev := barOn(t, &usModel88, script)
	if err := p.ShowLayers(Backdrop{Video: &DeviceVideo{Path: path, FPS: 24}}, nil); err != nil {
		t.Fatal(err)
	}
	lyWaitFor(t, "blocks", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return blocks >= 3
	})
	p.Close()
	dev.mu.Lock()
	defer dev.mu.Unlock()
	var seq []byte
	var blockAt []time.Time
	for i, w := range dev.writes {
		op := decryptBlock(w[:504])[0]
		if op == byte(usOpVideo) || op == byte(usOpQueue) {
			seq = append(seq, op)
		}
		if op == byte(usOpVideo) {
			blockAt = append(blockAt, dev.at[i])
		}
	}
	// block 1 is busy: exactly 3 polls (4, 4, 2), then the next block
	want := []byte{0x79, 0x7A, 0x7A, 0x7A, 0x79}
	if len(seq) < len(want) || !bytes.Equal(seq[:len(want)], want) {
		t.Fatalf("sequence % x, want prefix % x", seq, want)
	}
	for i := 1; i < len(blockAt) && i < 3; i++ {
		if d := blockAt[i].Sub(blockAt[i-1]); d < 25*time.Millisecond {
			t.Fatalf("blocks %d and %d only %v apart", i-1, i, d)
		}
	}
}

func TestVideoOverlayAndSwitching(t *testing.T) {
	fastLayers(t)
	path := writeVideo(t, 10000)
	p, dev := barOn(t, &usModel88, ack0)
	vb := Backdrop{Video: &DeviceVideo{Path: path, FPS: 30}}
	ov := overlayWith(3, 3, color.RGBA{255, 0, 0, 255})
	if err := p.ShowLayers(vb, ov); err != nil {
		t.Fatal(err)
	}
	// the same video again must not restart it
	if err := p.ShowLayers(vb, ov); err != nil {
		t.Fatal(err)
	}
	lyWaitFor(t, "a block", func() bool { return dev.count() >= 4 })
	// A JPEG frame stops the stream (halt) and clears the overlay.
	if err := p.ShowImage(bigCanvas(color.RGBA{0, 0, 0, 255})); err != nil {
		t.Fatal(err)
	}
	ws := dev.take()
	ops := lyOps(ws)
	count := func(op usOp) (n int) {
		for _, o := range ops {
			if o == byte(op) {
				n++
			}
		}
		return
	}
	if count(usOpHalt) != 2 { // start and the switch to a picture
		t.Fatalf("halts: %d in % x", count(usOpHalt), ops)
	}
	if count(usOpFPS) < 2 { // at the start and before the overlay
		t.Fatalf("frame-rate commands: %d", count(usOpFPS))
	}
	if ops[len(ops)-1] != byte(usOpPicture) || ops[len(ops)-2] != byte(usOpOverlay) {
		t.Fatalf("switch to a picture: % x", ops[len(ops)-4:])
	}
	// no block after the halt that ended the stream
	haltAt := -1
	for i, o := range ops {
		if o == byte(usOpHalt) {
			haltAt = i
		}
	}
	for _, o := range ops[haltAt+1:] {
		if o == byte(usOpVideo) {
			t.Fatal("video block after the stop")
		}
	}
	time.Sleep(20 * time.Millisecond)
	if dev.count() != 0 {
		t.Fatalf("stream kept writing after the stop (%d)", dev.count())
	}
}

func TestVideoRejectsBadInput(t *testing.T) {
	fastLayers(t)
	p, dev := barOn(t, &usModel88, ack0)
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.h264")
	os.WriteFile(empty, nil, 0o600)
	for name, v := range map[string]*DeviceVideo{
		"missing":  {Path: filepath.Join(dir, "nope.h264"), FPS: 30},
		"empty":    {Path: empty, FPS: 30},
		"dir":      {Path: dir, FPS: 30},
		"fps zero": {Path: writeVideo(t, 100), FPS: 0},
		"fps big":  {Path: writeVideo(t, 100), FPS: 120},
	} {
		if err := p.ShowLayers(Backdrop{Video: v}, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if dev.count() != 0 {
		t.Fatalf("bad input wrote %d commands", dev.count())
	}
	// Models without video refuse it.
	q := &universal88{dev: &lyUSB{}, m: &usModelSL}
	if q.ShowLayers(Backdrop{Video: &DeviceVideo{Path: writeVideo(t, 100), FPS: 30}}, nil) == nil {
		t.Fatal("slv3 accepted video")
	}
}

func TestVideoBlockBounds(t *testing.T) {
	p := &universal88{dev: &lyUSB{}}
	if _, err := p.videoBlock(make([]byte, usVideoBlock+1), false); err == nil {
		t.Fatal("oversize block accepted")
	}
	if _, err := p.videoBlock(nil, false); err == nil {
		t.Fatal("empty block accepted")
	}
}

func TestVideoStreamEndsWhenTheDeviceFails(t *testing.T) {
	fastLayers(t)
	path := writeVideo(t, usVideoBlock)
	dev := &lyFailUSB{lyUSB: lyUSB{script: ack0}}
	old := us88Open
	us88Open = func(winusb.Info) (winusb.Device, error) { return dev, nil }
	defer func() { us88Open = old }()
	p := &universal88{m: &usModel88, id: "us88-0", name: "x", brightness: 100}
	if err := p.connect(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.ShowLayers(Backdrop{Video: &DeviceVideo{Path: path, FPS: 30}}, nil); err != nil {
		t.Fatal(err)
	}
	dev.fail()
	r := func() *videoRun { p.mu.Lock(); defer p.mu.Unlock(); return p.lay.vid }()
	select {
	case <-r.done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream goroutine did not end")
	}
	if r.err == nil {
		t.Fatal("no error recorded")
	}
}

type lyFailUSB struct {
	lyUSB
	bad bool
}

func (f *lyFailUSB) fail() { f.mu.Lock(); f.bad = true; f.mu.Unlock() }
func (f *lyFailUSB) Write(ep byte, buf []byte, to int) (int, error) {
	f.mu.Lock()
	bad := f.bad
	f.mu.Unlock()
	if bad {
		return 0, winusb.ErrTimeout
	}
	return f.lyUSB.Write(ep, buf, to)
}

// The new commands stay inside the allow-list of the bars, whatever they are
// asked to do.
func TestLayerAndVideoPathsStayInTheAllowList(t *testing.T) {
	fastLayers(t)
	newCache(t)
	allowed := map[byte]bool{}
	for _, o := range []usOp{usOpInfo, usOpFPS, usOpTurn, usOpBacklight, usOpClock, usOpClockOff, usOpPicture, usOpOverlay, usOpVideo, usOpQueue, usOpHalt} {
		allowed[byte(o)] = true
	}
	for _, m := range []*usModel{&usModel88, &usModel92, &usModel207} {
		p, dev := barOn(t, m, ack0)
		sz := p.Size()
		bg := Backdrop{Key: 1, Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y)) }}
		for _, rot := range []int{0, 90, 180, 270} {
			p.SetRotation(rot)
			s := sz
			if rot == 90 || rot == 270 {
				s = image.Pt(sz.Y, sz.X)
			}
			b := Backdrop{Key: uint64(rot) + 1, Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, s.X, s.Y)) }}
			if err := p.ShowLayers(b, image.NewRGBA(image.Rect(0, 0, s.X, s.Y))); err != nil {
				t.Fatalf("%s rot %d: %v", m.name, rot, err)
			}
		}
		_ = bg
		p.SetRotation(0)
		p.ShowLayers(Backdrop{Video: &DeviceVideo{Path: writeVideo(t, 300000), FPS: 30}}, nil)
		p.SetOfflineClock(true)
		p.SetOfflineClock(false)
		p.SetBrightness(40)
		p.Close()
		for i, op := range lyOps(dev.take()) {
			if !allowed[op] || usForbiddenOps[op] {
				t.Errorf("%s wrote op %#x (command %d)", m.name, op, i)
			}
		}
	}
}

// The 8.8" always turns by 180 degrees itself: no switch, 180 and 90 write
// 0x0D 2, 0 and 270 write 0, and the host neither mirrors nor turns by 180.
func TestUS88AlwaysTurnsOnDevice(t *testing.T) {
	p, dev := barOn(t, &usModel88, ack0)
	if !p.devFlip() || p.CanDeviceFlip() || p.SetDeviceFlip(false) == nil {
		t.Fatal("8.8 must turn on the device, without a switch")
	}
	dev.take()
	var turns []byte
	l := us88Landscape()
	c := image.NewRGBA(image.Rect(0, 0, us88Width, us88Height))
	c.Pix[0] = 9
	for _, rot := range []int{180, 90, 270, 0} {
		p.SetRotation(rot)
		switch rot {
		case 180:
			if !bytes.Equal(p.orient(l).Pix, us88Rotate(l, false).Pix) {
				t.Fatal("host mirrors at 180")
			}
		case 90:
			if !bytes.Equal(p.orient(c).Pix, us88Upright(c, 270).Pix) {
				t.Fatal("90 not drawn like 270")
			}
		}
		for _, w := range dev.take() {
			if pl := decryptBlock(w[:504]); pl[0] == byte(usOpTurn) {
				turns = append(turns, pl[8])
			}
		}
	}
	if !bytes.Equal(turns, []byte{2, 0}) {
		t.Fatalf("device turns %v, want 2 (180, kept for 90) then 0 (270, kept for 0)", turns)
	}
}

// Other bars keep the switch, off by default.
func TestDeviceFlipIsOffByDefaultAndSoftwareMirrors(t *testing.T) {
	p, dev := barOn(t, &usModel92, ack0)
	if p.devFlip() {
		t.Fatal("device flip must be off by default")
	}
	p.SetRotation(180)
	for _, op := range lyOps(dev.take()) {
		if op == byte(usOpTurn) {
			t.Fatal("rotation 180 sent a device turn by default")
		}
	}
	p.SetRotation(0)
	dev.take()
	if err := p.SetDeviceFlip(true); err != nil {
		t.Fatal(err)
	}
	p.SetRotation(0) // switching on wrote 0 (never sent before), this writes nothing
	p.SetRotation(180)
	l := us88Landscape()
	got := p.orient(l) // rotation 180 with the device turning it
	p.SetRotation(0)
	var turns []byte
	for _, w := range dev.take() {
		if pl := decryptBlock(w[:504]); pl[0] == byte(usOpTurn) {
			turns = append(turns, pl[8])
		}
	}
	if !bytes.Equal(turns, []byte{0, 2, 0}) {
		t.Fatalf("device turns %v, want 0 (switch on), 2, then 0", turns)
	}
	// the frame is then not mirrored by the host
	want := us88Rotate(l, false)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("host still mirrors the frame")
	}
	// 207 has no device rotation
	q := &universal88{m: &usModel207}
	if q.devFlip() {
		t.Fatal("207 flips on the device")
	}
}

func TestFastPNGRoundTrip(t *testing.T) {
	n := image.NewNRGBA(image.Rect(0, 0, 37, 19))
	for i := range n.Pix {
		n.Pix[i] = byte(i*7 + i/13)
	}
	b, err := encodePNGFast(n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := got.(*image.NRGBA)
	if !ok || g.Rect != n.Rect || !bytes.Equal(g.Pix, n.Pix) {
		t.Fatalf("round trip differs (%T)", got)
	}
	// 480x1920, mostly transparent, as the device gets it
	big := image.NewNRGBA(image.Rect(0, 0, 480, 1920))
	big.SetNRGBA(100, 100, color.NRGBA{1, 2, 3, 4})
	b, err = encodePNGFast(big)
	if err != nil || len(b) > 20000 {
		t.Fatalf("empty overlay is %d bytes (%v)", len(b), err)
	}
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func TestSetDeviceFlipWhileTurned(t *testing.T) {
	turns := func(dev *lyUSB) []byte {
		var out []byte
		for _, w := range dev.take() {
			if pl := decryptBlock(w[:504]); pl[0] == byte(usOpTurn) {
				out = append(out, pl[8])
			}
		}
		return out
	}
	p, dev := barOn(t, &usModel92, ack0)
	p.SetRotation(180)
	if got := turns(dev); len(got) != 0 {
		t.Fatalf("software flip wrote turns %v", got)
	}
	l := us88Landscape()
	soft := p.orient(l)
	if !bytes.Equal(soft.Pix, us88Rotate(l, true).Pix) {
		t.Fatal("host does not mirror with the switch off")
	}
	p.SetDeviceFlip(true) // already at 180: written at once
	if got := turns(dev); !bytes.Equal(got, []byte{2}) {
		t.Fatalf("switch on at 180 wrote %v, want [2]", got)
	}
	if !bytes.Equal(p.orient(l).Pix, us88Rotate(l, false).Pix) {
		t.Fatal("host still mirrors with the switch on")
	}
	p.SetDeviceFlip(true) // unchanged: nothing
	if got := turns(dev); len(got) != 0 {
		t.Fatalf("unchanged switch wrote %v", got)
	}
	p.SetDeviceFlip(false)
	if got := turns(dev); !bytes.Equal(got, []byte{0}) {
		t.Fatalf("switch off wrote %v, want [0]", got)
	}
	// models without the device rotation refuse it
	q := &universal88{m: &usModel207}
	if q.SetDeviceFlip(true) == nil || q.devFlip() {
		t.Fatal("207 accepted the device flip")
	}
}
