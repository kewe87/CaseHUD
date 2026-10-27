package panel

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

type fakeUSB struct {
	writes [][]byte
	reply  []byte
}

func (f *fakeUSB) Write(ep byte, buf []byte, _ int) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), buf...))
	return len(buf), nil
}

func (f *fakeUSB) Read(ep byte, buf []byte, _ int) (int, error) {
	if f.reply == nil {
		return 0, winusb.ErrTimeout
	}
	n := copy(buf, f.reply)
	f.reply = nil
	return n, nil
}

func (f *fakeUSB) ClearHalt(byte) error           { return nil }
func (f *fakeUSB) ZeroLengthTerminate(byte) error { return nil }
func (f *fakeUSB) Close() error                   { return nil }

// decrypt undoes the header's DES-CBC layer and returns the 504-byte plaintext.
func usOpen(t *testing.T, hdr []byte) []byte {
	t.Helper()
	blk, _ := des.NewCipher(usKey)
	out := make([]byte, 504)
	cipher.NewCBCDecrypter(blk, usKey).CryptBlocks(out, hdr[:504])
	return out
}

func TestUSCommandBlock(t *testing.T) {
	// backlight (0x0E), stamp 0x0A0B0C0D, one argument byte 0x21
	blk := usCommand{Op: usOpBacklight, Stamp: 0x0A0B0C0D, Args: []byte{0x21}}.seal()
	if len(blk) != 512 || blk[510] != 0xA1 || blk[511] != 0x1A || !bytes.Equal(blk[504:510], make([]byte, 6)) {
		t.Fatalf("envelope = % x", blk[500:])
	}
	plain := usOpen(t, blk)
	want := []byte{0x0E, 0, 0x1A, 0x6D, 0x0D, 0x0C, 0x0B, 0x0A, 0x21}
	if !bytes.Equal(plain[:9], want) {
		t.Fatalf("plain = % x", plain[:9])
	}
	if !bytes.Equal(plain[9:500], make([]byte, 491)) || !bytes.Equal(plain[500:], []byte{4, 4, 4, 4}) {
		t.Fatal("argument area or padding wrong")
	}
	// Arguments beyond 492 bytes never spill into the padding.
	long := usCommand{Op: usOpPicture, Args: bytes.Repeat([]byte{9}, 600)}.seal()
	if p := usOpen(t, long); !bytes.Equal(p[500:], []byte{4, 4, 4, 4}) || p[499] != 9 {
		t.Fatalf("long args: % x", p[495:])
	}
}

func TestUSClockIsStrictlyIncreasing(t *testing.T) {
	var c usClock
	prev := c.next()
	for i := 0; i < 50; i++ { // far faster than one per millisecond
		n := c.next()
		if n <= prev {
			t.Fatalf("stamp %d after %d", n, prev)
		}
		prev = n
	}
	c = usClock{origin: time.Now(), last: 7000}
	if n := c.next(); n != 7001 {
		t.Fatalf("stamp %d, want 7001", n)
	}
}

func TestUS88PictureTransfer(t *testing.T) {
	p := &universal88{dev: &fakeUSB{}, brightness: 100}
	jpg := bytes.Repeat([]byte{0xab}, 1000)
	fr := p.picture(jpg)
	if len(fr) != 512+1000 || !bytes.Equal(fr[512:], jpg) {
		t.Fatal("transfer layout")
	}
	plain := usOpen(t, fr)
	if plain[0] != byte(usOpPicture) || !bytes.Equal(plain[8:12], []byte{0, 0, 0x03, 0xE8}) {
		t.Fatalf("block = % x", plain[:12])
	}
}

func us88Landscape() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, us88Height, us88Width))
	// Red along the top edge, green along the left edge.
	for x := 0; x < us88Height; x++ {
		img.Set(x, 0, color.RGBA{255, 0, 0, 255})
	}
	for y := 0; y < us88Width; y++ {
		img.Set(0, y, color.RGBA{0, 255, 0, 255})
	}
	return img
}

func TestUS88Rotate(t *testing.T) {
	l := us88Landscape()
	f := us88Rotate(l, false)
	if f.Bounds().Dx() != 480 || f.Bounds().Dy() != 1920 {
		t.Fatalf("frame = %v", f.Bounds())
	}
	// frame(x, y) = L(y, 479-x) for every pixel.
	for _, c := range [][2]int{{0, 0}, {479, 0}, {0, 1919}, {479, 1919}, {123, 456}, {300, 1000}} {
		x, y := c[0], c[1]
		if f.RGBAAt(x, y) != l.RGBAAt(y, 479-x) {
			t.Fatalf("pixel (%d,%d) mismatch", x, y)
		}
	}
	// Landscape top edge (red) ends up on the frame's left... of the
	// device orientation: frame x = 479 column.
	if f.RGBAAt(479, 700) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatal("top edge not at frame x=479")
	}
	if f.RGBAAt(100, 0) != (color.RGBA{0, 255, 0, 255}) {
		t.Fatal("left edge not at frame y=0")
	}
	// Flip: landscape turned by 180 degrees first.
	g := us88Rotate(l, true)
	for _, c := range [][2]int{{0, 0}, {10, 20}, {479, 1919}, {200, 900}} {
		x, y := c[0], c[1]
		if g.RGBAAt(x, y) != l.RGBAAt(1919-y, x) {
			t.Fatalf("flipped pixel (%d,%d) mismatch", x, y)
		}
	}
}

func TestUS88ShowJPEG(t *testing.T) {
	f := &fakeUSB{reply: make([]byte, 512)}
	p := &universal88{dev: f, id: "us88-0", name: "Universal Screen 8.8\""}
	var b bytes.Buffer
	jpeg.Encode(&b, us88Landscape(), nil)
	if err := p.ShowJPEG(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 || len(f.writes[0]) < 600 {
		t.Fatalf("writes = %d", len(f.writes))
	}
	plain := usOpen(t, f.writes[0])
	if plain[0] != byte(usOpPicture) || int(binary.BigEndian.Uint32(plain[8:12])) != len(f.writes[0])-512 {
		t.Fatalf("header = % x", plain[:12])
	}
	sent, err := jpeg.Decode(bytes.NewReader(f.writes[0][512:]))
	if err != nil || sent.Bounds().Dx() != 480 || sent.Bounds().Dy() != 1920 {
		t.Fatalf("sent frame: %v %v", sent, err)
	}
	if err := p.ShowJPEG(nil); err == nil {
		t.Fatal("empty frame accepted")
	}
	if p.Size() != image.Pt(1920, 480) {
		t.Fatalf("size = %v", p.Size())
	}
}

// failUSB times out every transfer.
type failUSB struct{ closed bool }

func (f *failUSB) Write(byte, []byte, int) (int, error) { return 0, winusb.ErrTimeout }
func (f *failUSB) Read(byte, []byte, int) (int, error)  { return 0, winusb.ErrTimeout }
func (f *failUSB) ClearHalt(byte) error                 { return nil }
func (f *failUSB) ZeroLengthTerminate(byte) error       { return nil }
func (f *failUSB) Close() error                         { f.closed = true; return nil }

func TestUS88Recovery(t *testing.T) {
	f := &failUSB{}
	p := &universal88{dev: f, name: "x"}
	var b bytes.Buffer
	jpeg.Encode(&b, us88Landscape(), nil)
	if err := p.ShowJPEG(b.Bytes()); err == nil {
		t.Fatal("timeout not reported")
	}
	if !f.closed || p.dev != nil || p.failures != 1 || !p.nextTry.After(time.Now()) {
		t.Fatalf("no recovery: closed=%v dev=%v failures=%d", f.closed, p.dev, p.failures)
	}
	// Within the back-off window the frame is dropped without touching USB.
	if err := p.ShowJPEG(b.Bytes()); err == nil || p.failures != 1 {
		t.Fatalf("back-off ignored: %v %d", err, p.failures)
	}
}

func TestUS88Brightness(t *testing.T) {
	f := &fakeUSB{}
	p := &universal88{dev: f}
	if err := p.SetBrightness(250); err != nil {
		t.Fatal(err)
	}
	plain := usOpen(t, f.writes[0])
	if plain[0] != byte(usOpBacklight) || plain[8] != 50 {
		t.Fatalf("brightness cmd = % x", plain[:9])
	}
	if p.SetRotation(45) == nil || p.SetRotation(180) != nil || p.rotation != 180 {
		t.Fatal("rotation handling")
	}
	if len(f.writes) != 1 {
		t.Fatal("rotation must stay in software")
	}
}

// viewerSees turns a device frame into what a person looking at the bar sees
// when it is mounted with the given rotation: 0 and 180 show the landscape
// picture (180 upside down), 90 has the right edge of the landscape bar up
// (bar turned counter-clockwise), 270 the left edge up (turned clockwise).
func viewerSees(d *image.RGBA, rotation int) *image.RGBA {
	// Landscape view of the device frame: D(x,y) = Lview(y, 479-x).
	lv := image.NewRGBA(image.Rect(0, 0, us88Height, us88Width))
	for y := 0; y < us88Height; y++ {
		for x := 0; x < us88Width; x++ {
			lv.SetRGBA(y, us88Width-1-x, d.RGBAAt(x, y))
		}
	}
	if rotation == 0 || rotation == 180 {
		return lv
	}
	v := image.NewRGBA(image.Rect(0, 0, us88Width, us88Height))
	for ly := 0; ly < us88Width; ly++ {
		for lx := 0; lx < us88Height; lx++ {
			if rotation == 90 {
				v.SetRGBA(ly, us88Height-1-lx, lv.RGBAAt(lx, ly))
			} else {
				v.SetRGBA(us88Width-1-ly, lx, lv.RGBAAt(lx, ly))
			}
		}
	}
	return v
}

// An upright canvas (red band on top, green band on the left, blue pixel
// near the top right) must look the same to the viewer for 90 and 270.
func TestUS88UprightRotation(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, us88Width, us88Height))
	for y := 0; y < us88Height; y++ {
		for x := 0; x < us88Width; x++ {
			c := color.RGBA{30, 30, 30, 255}
			switch {
			case y < 20:
				c = color.RGBA{255, 0, 0, 255}
			case x < 20:
				c = color.RGBA{0, 255, 0, 255}
			case x > 440 && y > 40 && y < 60:
				c = color.RGBA{0, 0, 255, 255}
			}
			canvas.SetRGBA(x, y, c)
		}
	}
	for _, rot := range []int{90, 270} {
		d := us88Upright(canvas, rot)
		got := viewerSees(d, rot)
		for y := 0; y < us88Height; y++ {
			for x := 0; x < us88Width; x++ {
				if got.RGBAAt(x, y) != canvas.RGBAAt(x, y) {
					t.Fatalf("rotation %d: pixel (%d,%d) = %v, want %v", rot, x, y, got.RGBAAt(x, y), canvas.RGBAAt(x, y))
				}
			}
		}
	}
	// A portrait frame sent through ShowJPEG at 90 is accepted.
	f := &fakeUSB{reply: make([]byte, 512)}
	p := &universal88{dev: f}
	p.SetRotation(90)
	var b bytes.Buffer
	jpeg.Encode(&b, canvas, nil)
	if err := p.ShowJPEG(b.Bytes()); err != nil {
		t.Fatal(err)
	}
}

// stallUSB accepts zero bytes without an error.
type stallUSB struct{ fakeUSB }

func (f *stallUSB) Write(byte, []byte, int) (int, error) { return 0, nil }

func TestUS88ShortWriteDoesNotSpin(t *testing.T) {
	p := &universal88{dev: &stallUSB{}, id: "us88-0", name: "us88"}
	done := make(chan error, 1)
	go func() { done <- p.write([]byte{1, 2, 3}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("zero-byte write reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write loops forever on a zero-byte write")
	}
}

func TestUS88RejectsWrongSizeBeforeDecoding(t *testing.T) {
	f := &fakeUSB{reply: make([]byte, 512)}
	p := &universal88{dev: f, id: "us88-0", name: "us88"}
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil)
	if err := p.ShowJPEG(b.Bytes()); err == nil {
		t.Fatal("wrong-sized frame accepted")
	}
	if len(f.writes) != 0 {
		t.Fatal("wrong-sized frame was sent")
	}
}

func BenchmarkUS88ShowJPEG(b *testing.B) {
	var buf bytes.Buffer
	jpeg.Encode(&buf, us88Landscape(), &jpeg.Options{Quality: 85})
	data := buf.Bytes()
	for i := 0; i < b.N; i++ {
		f := &fakeUSB{reply: make([]byte, 512)}
		p := &universal88{dev: f, id: "us88-0", name: "us88"}
		if err := p.ShowJPEG(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestUS88DevBrightness(t *testing.T) {
	for in, want := range map[int]byte{-5: 0, 0: 0, 1: 1, 50: 25, 99: 50, 100: 50, 250: 50} {
		if got := us88DevBrightness(in); got != want {
			t.Errorf("brightness %d -> %d, want %d", in, got, want)
		}
	}
}

func TestUS88InitSequence(t *testing.T) {
	f := &fakeUSB{}
	p := &universal88{dev: f, brightness: 100}
	if err := p.wake(); err != nil {
		t.Fatal(err)
	}
	var cmds []byte
	for _, w := range f.writes {
		plain := usOpen(t, w)
		cmds = append(cmds, plain[0])
		switch plain[0] {
		case byte(usOpTurn):
			if plain[8] != 0 {
				t.Fatalf("rotation = %d, want 0", plain[8])
			}
		case byte(usOpBacklight):
			if plain[8] != 50 {
				t.Fatalf("brightness = %d, want 50", plain[8])
			}
		case byte(usOpOverlay):
			n := int(binary.BigEndian.Uint32(plain[8:12]))
			img, err := png.Decode(bytes.NewReader(w[512:]))
			if err != nil || n != len(w)-512 {
				t.Fatalf("overlay: size %d/%d err %v", n, len(w)-512, err)
			}
			if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 1920 {
				t.Fatalf("overlay is %v", img.Bounds())
			}
			if _, _, _, a := img.At(100, 100).RGBA(); a != 0 {
				t.Fatal("overlay not transparent")
			}
		case 0x0F:
			t.Fatal("frame-rate command must not be sent")
		}
	}
	want := []byte{byte(usOpHalt), byte(usOpClock), byte(usOpClockOff), byte(usOpTurn), byte(usOpInfo), byte(usOpOverlay), byte(usOpBacklight)}
	if !bytes.Equal(cmds, want) {
		t.Fatalf("init commands = % x, want % x", cmds, want)
	}
}

func TestUS88ShowImage(t *testing.T) {
	f := &fakeUSB{reply: make([]byte, 512)}
	p := &universal88{dev: f, id: "us88-0", name: "us88"}
	var _ RawPanel = p
	l := us88Landscape()
	for y := 0; y < 32; y++ { // a band that survives JPEG chroma subsampling
		for x := 0; x < us88Height; x++ {
			l.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	before := append([]byte(nil), l.Pix...)
	if err := p.ShowImage(l); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, l.Pix) {
		t.Fatal("ShowImage modified its input")
	}
	if len(f.writes) != 1 {
		t.Fatalf("writes = %d", len(f.writes))
	}
	sent, err := jpeg.Decode(bytes.NewReader(f.writes[0][512:]))
	if err != nil || sent.Bounds().Dx() != 480 || sent.Bounds().Dy() != 1920 {
		t.Fatalf("sent frame: %v %v", sent, err)
	}
	// Top edge of the landscape picture (red) ends up in the frame's column 479.
	if r, g, _, _ := sent.At(470, 700).RGBA(); r>>8 < 200 || g>>8 > 60 {
		t.Fatal("orientation wrong")
	}
	if err := p.ShowImage(image.NewRGBA(image.Rect(0, 0, 64, 64))); err == nil {
		t.Fatal("wrong-sized image accepted")
	}
	// A picture with a non-zero origin is accepted too.
	sub := image.NewRGBA(image.Rect(10, 10, 10+us88Height, 10+us88Width))
	f.reply = make([]byte, 512)
	if err := p.ShowImage(sub); err != nil {
		t.Fatal(err)
	}
}

func us88NoisyLandscape() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, us88Height, us88Width))
	for y := 0; y < us88Width; y++ {
		for x := 0; x < us88Height; x++ {
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = byte(x/8), byte(y/2), byte(x*y>>6), 255
		}
	}
	return img
}

// The engine's old path: encode a JPEG, then ShowJPEG decodes, turns and re-encodes it.
func BenchmarkUS88PipelineViaJPEG(b *testing.B) {
	img := us88NoisyLandscape()
	f := &fakeUSB{}
	p := &universal88{dev: f, name: "us88"}
	for i := 0; i < b.N; i++ {
		f.reply, f.writes = make([]byte, 512), nil
		if err := Show(struct{ Panel }{p}, img); err != nil { // hides RawPanel
			b.Fatal(err)
		}
	}
}

func BenchmarkUS88PipelineRaw(b *testing.B) {
	img := us88NoisyLandscape()
	f := &fakeUSB{}
	p := &universal88{dev: f, name: "us88"}
	for i := 0; i < b.N; i++ {
		f.reply, f.writes = make([]byte, 512), nil
		if err := Show(p, img); err != nil {
			b.Fatal(err)
		}
	}
}

// scriptUSB lets a test decide the outcome of every write and read.
type scriptUSB struct {
	writes  [][]byte
	failW   int      // fail this many writes first (non-timeout error)
	acks    [][]byte // replies for successive reads; nil entry = timeout
	closed  bool
	readIdx int
}

func (s *scriptUSB) Write(ep byte, buf []byte, _ int) (int, error) {
	if s.failW > 0 {
		s.failW--
		return 0, errors.New("device gone")
	}
	s.writes = append(s.writes, append([]byte(nil), buf...))
	return len(buf), nil
}

func (s *scriptUSB) Read(ep byte, buf []byte, _ int) (int, error) {
	if s.readIdx >= len(s.acks) || s.acks[s.readIdx] == nil {
		s.readIdx++
		return 0, winusb.ErrTimeout
	}
	n := copy(buf, s.acks[s.readIdx])
	s.readIdx++
	return n, nil
}
func (s *scriptUSB) ClearHalt(byte) error           { return nil }
func (s *scriptUSB) ZeroLengthTerminate(byte) error { return nil }
func (s *scriptUSB) Close() error                   { s.closed = true; return nil }

func us88Cmds(t *testing.T, ws [][]byte) []byte {
	var c []byte
	for _, w := range ws {
		c = append(c, usOpen(t, w)[0])
	}
	return c
}

func TestUS88MissingAckAsksBufferLevelFirst(t *testing.T) {
	// frame ack missing, buffer level query answered: device stays open.
	s := &scriptUSB{acks: [][]byte{nil, make([]byte, 512), nil}}
	p := &universal88{dev: s, name: "us88"}
	if err := p.ShowImage(us88Landscape()); err != nil {
		t.Fatal(err)
	}
	if p.dev == nil || s.closed || p.failures != 0 {
		t.Fatal("device was dropped although it answered the buffer query")
	}
	if got := us88Cmds(t, s.writes); !bytes.Equal(got, []byte{byte(usOpPicture), byte(usOpQueue)}) {
		t.Fatalf("commands = % x", got)
	}
	// Both unanswered: recover.
	s2 := &scriptUSB{}
	p2 := &universal88{dev: s2, name: "us88"}
	if err := p2.ShowImage(us88Landscape()); err == nil || p2.dev != nil || !s2.closed {
		t.Fatalf("no recovery: %v", err)
	}
}

func TestUS88WriteErrorReopensOnceAndResends(t *testing.T) {
	old := us88Open
	defer func() { us88Open = old }()
	fresh := &scriptUSB{acks: [][]byte{nil, nil, nil, nil, nil, nil, nil, make([]byte, 512)}}
	opens := 0
	us88Open = func(winusb.Info) (winusb.Device, error) { opens++; return fresh, nil }
	stale := &scriptUSB{failW: 1}
	p := &universal88{dev: stale, name: "us88"}
	if err := p.ShowImage(us88Landscape()); err != nil {
		t.Fatal(err)
	}
	if opens != 1 || !stale.closed || p.dev != fresh {
		t.Fatalf("opens=%d staleClosed=%v", opens, stale.closed)
	}
	cmds := us88Cmds(t, fresh.writes)
	if cmds[0] != byte(usOpHalt) || cmds[len(cmds)-2] != byte(usOpPicture) && cmds[len(cmds)-1] != byte(usOpPicture) {
		t.Fatalf("init + resend expected, got % x", cmds)
	}
	// A second failure is not retried again.
	fresh2 := &scriptUSB{failW: 99}
	us88Open = func(winusb.Info) (winusb.Device, error) { return fresh2, nil }
	p = &universal88{dev: &scriptUSB{failW: 1}, name: "us88"}
	if err := p.ShowImage(us88Landscape()); err == nil || p.dev != nil {
		t.Fatalf("expected failure and closed device, err=%v", err)
	}
}

func TestUS88CloseSendsStopPlay(t *testing.T) {
	s := &scriptUSB{}
	p := &universal88{dev: s, name: "us88"}
	p.Close()
	if got := us88Cmds(t, s.writes); !bytes.Equal(got, []byte{byte(usOpHalt)}) || !s.closed || p.dev != nil {
		t.Fatalf("close: cmds % x closed=%v", got, s.closed)
	}
	p.Close() // twice is fine
}

// With an exit image set, Close sends the halt, then the picture and an empty
// overlay as ordinary frames; without one nothing more goes out.
func TestUS88CloseLeavesExitImage(t *testing.T) {
	s := &scriptUSB{}
	p := &universal88{dev: s, name: "us88"}
	p.SetExitImage(us88Landscape())
	p.Close()
	want := []byte{byte(usOpHalt), byte(usOpPicture), byte(usOpOverlay)}
	if got := us88Cmds(t, s.writes); !bytes.Equal(got, want) || !s.closed {
		t.Fatalf("close: cmds % x closed=%v", got, s.closed)
	}

	// A picture of the wrong size is skipped, Close still halts and releases.
	s = &scriptUSB{}
	p = &universal88{dev: s, name: "us88"}
	p.SetExitImage(image.NewRGBA(image.Rect(0, 0, 100, 100)))
	p.Close()
	if got := us88Cmds(t, s.writes); !bytes.Equal(got, []byte{byte(usOpHalt)}) || !s.closed {
		t.Fatalf("wrong size: cmds % x", got)
	}

	// Other models ignore it.
	s = &scriptUSB{}
	p = &universal88{dev: s, name: "us92", m: &usModel92}
	p.SetExitImage(image.NewRGBA(image.Rect(0, 0, usModel92.long, usModel92.short)))
	p.Close()
	if got := us88Cmds(t, s.writes); !bytes.Equal(got, []byte{byte(usOpHalt)}) {
		t.Fatalf("9.2: cmds % x", got)
	}
}

// A device that already holds the brightness and turn gets neither again, on
// start-up or later; a changed value is written once.
func TestUS88SettingsWrittenOnlyWhenChanged(t *testing.T) {
	f := &fakeUSB{}
	p := &universal88{dev: f, brightness: 100}
	p.sentBright, p.sentTurn = us88DevHeld(p.mdl(), 100), 1
	if err := p.wake(); err != nil {
		t.Fatal(err)
	}
	for _, w := range f.writes {
		if op := usOpen(t, w)[0]; op == byte(usOpBacklight) || op == byte(usOpTurn) {
			t.Fatalf("stored setting rewritten on start-up (op %#x)", op)
		}
	}
	n := len(f.writes)
	for i := 0; i < 10; i++ {
		p.SetBrightness(100)
	}
	if len(f.writes) != n {
		t.Fatal("same brightness written again")
	}
	for i := 0; i < 10; i++ {
		p.SetBrightness(60)
	}
	if len(f.writes) != n+1 {
		t.Fatalf("brightness change wrote %d times, want 1", len(f.writes)-n)
	}
}

func TestUniversalVersionFromInfoReply(t *testing.T) {
	var p Versioned = &universal88{}
	if p.Version() != "" {
		t.Fatal("version before any reply")
	}
	r := make([]byte, 64)
	r[0] = byte(usOpInfo)
	copy(r[8:], "V1.2.3\x00junk")
	p.(*universal88).noteFirmware(r)
	if got := p.Version(); got != "V1.2.3" {
		t.Fatalf("Version() = %q", got)
	}
}
