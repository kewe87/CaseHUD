package panel

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/binary"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

func init() {
	usSettle = 0
	usCheckWait = 0
	usFlexHandOverGap = 0
}

// usForbiddenOps are commands that write to flash, reboot, delete files or
// switch the display mode. No driver of this family may ever send them.
var usForbiddenOps = map[byte]bool{
	0x0B: true, // reboot
	0x26: true, // create file
	0x27: true, // large upload
	0x28: true, // write file (firmware, boot logo)
	0x2A: true, // delete file
	// 0x33 (clock) is allowed with flag 2 (set the time) only; the flag 1/0 that
	// stores the offline clock is sent by SetOfflineClock alone (see layers_test.go).
	0x62: true, // file size
	0x63: true, // directory query
	0x96: true, // display-mode switch
	0x97: true, // test
	0xFC: true, // test
	0xFD: true, // factory test
}

// cmdUSB records transfers and answers commands by operation.
type cmdUSB struct {
	writes [][]byte
	queue  [][]byte
	script func(op byte, plain []byte) []byte
	failAt int // fail the n-th write (1-based), 0 = never
}

func (s *cmdUSB) Write(ep byte, buf []byte, _ int) (int, error) {
	s.writes = append(s.writes, append([]byte(nil), buf...))
	if s.failAt > 0 && len(s.writes) == s.failAt {
		return 0, winusb.ErrTimeout
	}
	if len(buf) >= 512 && s.script != nil {
		plain := decryptBlock(buf[:504])
		if r := s.script(plain[0], plain); r != nil {
			s.queue = append(s.queue, r)
		}
	}
	return len(buf), nil
}

func (s *cmdUSB) Read(ep byte, buf []byte, _ int) (int, error) {
	if len(s.queue) == 0 {
		return 0, winusb.ErrTimeout
	}
	n := copy(buf, s.queue[0])
	s.queue = s.queue[1:]
	return n, nil
}

func (s *cmdUSB) ClearHalt(byte) error           { return nil }
func (s *cmdUSB) ZeroLengthTerminate(byte) error { return nil }
func (s *cmdUSB) Close() error                   { return nil }

// ops lists the operation byte of every command block written so far.
func (s *cmdUSB) ops() []byte {
	var out []byte
	for _, w := range s.writes {
		if len(w) >= 512 {
			out = append(out, decryptBlock(w[:504])[0])
		}
	}
	return out
}

// plainOf returns the decrypted header of the i-th write.
func (s *cmdUSB) plainOf(i int) []byte { return decryptBlock(s.writes[i][:504]) }

func decryptBlock(b []byte) []byte {
	blk, _ := des.NewCipher(usKey)
	out := make([]byte, len(b))
	cipher.NewCBCDecrypter(blk, usKey).CryptBlocks(out, b)
	return out
}

func reply(op byte, b8 byte) []byte {
	r := make([]byte, 512)
	r[0], r[8] = op, b8
	return r
}

// newModelPanel builds a connected panel for m on a scripted device.
func newModelPanel(t *testing.T, m *usModel, script func(byte, []byte) []byte) (*universal88, *cmdUSB) {
	t.Helper()
	dev := &cmdUSB{script: script}
	old := us88Open
	us88Open = func(winusb.Info) (winusb.Device, error) { return dev, nil }
	t.Cleanup(func() { us88Open = old })
	p := &universal88{m: m, id: m.idPrefix + "-0", name: m.name, brightness: 100}
	if err := p.connect(); err != nil {
		t.Fatal(err)
	}
	return p, dev
}

func TestSLBlockFraming(t *testing.T) {
	blk := usCommand{Op: usOpBacklight, Stamp: 0x01020304, Args: []byte{0x2A}}.sealSL()
	if len(blk) != 512 {
		t.Fatalf("len %d", len(blk))
	}
	plain := decryptBlock(blk)
	if !bytes.Equal(plain[:9], []byte{0x0E, 0, 0x1A, 0x6D, 4, 3, 2, 1, 0x2A}) {
		t.Fatalf("header % x", plain[:9])
	}
	if !bytes.Equal(plain[504:], bytes.Repeat([]byte{8}, 8)) {
		t.Fatalf("padding % x", plain[500:])
	}
	// the arguments never spill into the padding
	long := usCommand{Op: usOpPicture, Args: bytes.Repeat([]byte{9}, 700)}.sealSL()
	if p := decryptBlock(long); p[503] != 9 || !bytes.Equal(p[504:], bytes.Repeat([]byte{8}, 8)) {
		t.Fatal("long args overrun")
	}
}

func TestSLPictureIsFixedLengthTransfer(t *testing.T) {
	p, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	dev.writes = nil
	jpg := bytes.Repeat([]byte{0xAB}, 5000)
	if err := p.write(p.picture(jpg)); err != nil {
		t.Fatal(err)
	}
	w := dev.writes[0]
	if len(w) != slTransferLen {
		t.Fatalf("transfer is %d bytes, want %d", len(w), slTransferLen)
	}
	pl := dev.plainOf(0)
	if pl[0] != 0x65 || binary.BigEndian.Uint32(pl[8:12]) != 5000 {
		t.Fatalf("header % x", pl[:12])
	}
	if !bytes.Equal(w[512:512+5000], jpg) || !bytes.Equal(w[512+5000:], make([]byte, slTransferLen-512-5000)) {
		t.Fatal("data or padding wrong")
	}
	// commands without data are exactly one block
	dev.writes = nil
	p.issue(usOpInfo, nil)
	if len(dev.writes[0]) != 512 {
		t.Fatalf("plain command is %d bytes", len(dev.writes[0]))
	}
	// pictures above the limit are refused before anything is sent
	dev.writes = nil
	if err := p.ShowJPEG(make([]byte, slMaxJPEG+1)); err == nil {
		t.Fatal("oversize picture accepted")
	}
	if len(dev.writes) != 0 {
		t.Fatal("something was sent")
	}
}

func TestSLInitSequence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply func(byte, []byte) []byte
		want  []byte
	}{
		{"new LCD", func(op byte, _ []byte) []byte { return reply(op, 2) }, []byte{0x80, 0x0A, 0x0D, 0x0E}},
		{"old LCD gets the frame rate", func(op byte, _ []byte) []byte { return reply(op, 0) }, []byte{0x80, 0x0F, 0x0A, 0x0D, 0x0E}},
		{"no answer is tolerated", func(byte, []byte) []byte { return nil }, []byte{0x80, 0x0A, 0x0D, 0x0E}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, dev := newModelPanel(t, &usModelSL, tc.reply)
			if !bytes.Equal(dev.ops(), tc.want) {
				t.Fatalf("ops % x, want % x", dev.ops(), tc.want)
			}
			// the new-LCD query uses the 8.8" framing (trailer), the rest the SL framing
			if w := dev.writes[0]; w[510] != 0xA1 || w[511] != 0x1A {
				t.Fatal("0x80 lacks the 8.8 trailer")
			}
			if w := dev.writes[len(dev.writes)-1]; len(w) != 512 {
				t.Fatalf("last block %d bytes", len(w))
			}
		})
	}
	// the old-LCD frame rate is always 120
	_, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 0) })
	if pl := dev.plainOf(1); pl[0] != 0x0F || pl[8] != 120 {
		t.Fatalf("frame rate command % x", pl[:9])
	}
}

func TestSLRotationAndBrightness(t *testing.T) {
	p, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	// init sent UI 0 degrees as wire value 1
	if pl := dev.plainOf(2); pl[0] != 0x0D || pl[8] != 1 {
		t.Fatalf("initial rotation % x", pl[:9])
	}
	for _, tc := range [][2]int{{90, 2}, {180, 3}, {270, 0}, {0, 1}} {
		dev.writes = nil
		if err := p.SetRotation(tc[0]); err != nil {
			t.Fatal(err)
		}
		if pl := dev.plainOf(0); pl[0] != 0x0D || int(pl[8]) != tc[1] {
			t.Fatalf("rotation %d -> wire %d, want %d", tc[0], pl[8], tc[1])
		}
		// the same value is not written twice (the device may store it)
		dev.writes = nil
		p.SetRotation(tc[0])
		if len(dev.writes) != 0 {
			t.Fatalf("rotation %d written again", tc[0])
		}
	}
	// the backlight follows the curve of the UI steps
	for pct, want := range map[int]byte{0: 0, 25: 10, 50: 30, 75: 40, 100: 100, 12: 5, 90: 76} {
		if got := usModelSL.devBrightness(pct); got != want {
			t.Errorf("brightness %d%% -> %d, want %d", pct, got, want)
		}
	}
	if usModelSL.devBrightness(-5) != 0 || usModelSL.devBrightness(500) != 100 {
		t.Error("brightness is not clamped")
	}
}

// After the display was dark (sleep) the rotation is sent again.
func TestSLRotationResentAfterWake(t *testing.T) {
	p, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	p.SetRotation(90)
	dev.writes = nil
	p.SetBrightness(0)
	if got := dev.ops(); !bytes.Equal(got, []byte{0x0E}) {
		t.Fatalf("going dark sent % x", got)
	}
	dev.writes = nil
	p.SetBrightness(60)
	if got := dev.ops(); !bytes.Equal(got, []byte{0x0E, 0x0D}) {
		t.Fatalf("waking sent % x, want backlight then rotation", got)
	}
	if pl := dev.plainOf(1); pl[8] != 2 {
		t.Fatalf("rotation after wake = %d", pl[8])
	}
	dev.writes = nil
	p.SetBrightness(70) // not a wake: no rotation
	if got := dev.ops(); !bytes.Equal(got, []byte{0x0E}) {
		t.Fatalf("ordinary change sent % x", got)
	}
}

func TestSLHeartbeat(t *testing.T) {
	p, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	dev.writes = nil
	p.heartbeat(p.lastIO.Add(500 * time.Millisecond))
	if len(dev.writes) != 0 {
		t.Fatal("heartbeat sent although the line was busy a moment ago")
	}
	p.heartbeat(p.lastIO.Add(1100 * time.Millisecond))
	if got := dev.ops(); !bytes.Equal(got, []byte{0x0A}) {
		t.Fatalf("heartbeat sent % x", got)
	}
	// a failing write drops the handle
	dev.failAt = len(dev.writes) + 1
	p.heartbeat(p.lastIO.Add(2 * time.Second))
	if p.dev != nil {
		t.Fatal("device kept after a failed heartbeat")
	}
}

func TestSLFramesAreNotAcknowledged(t *testing.T) {
	p, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	dev.writes = nil
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	if err := p.ShowImage(img); err != nil {
		t.Fatal(err)
	}
	if len(dev.writes) != 1 || len(dev.writes[0]) != slTransferLen {
		t.Fatalf("%d writes, first %d bytes", len(dev.writes), len(dev.writes[0]))
	}
	if p.ShowImage(image.NewRGBA(image.Rect(0, 0, 480, 480))) == nil {
		t.Fatal("wrong size accepted")
	}
	if p.MaxJPEG() != slMaxJPEG || !p.Round() || p.Size() != image.Pt(400, 400) {
		t.Fatal("model data wrong")
	}
}

func TestBarModels(t *testing.T) {
	for _, tc := range []struct {
		m        *usModel
		w, h     int
		turn     bool
		brightIn int
		brightEx byte
	}{
		{&usModel92, 1920, 464, false, 70, 70},
		{&usModel207, 1472, 720, false, 70, 70},
		{&usModel88, 1920, 480, true, 70, 35},
	} {
		t.Run(tc.m.name, func(t *testing.T) {
			p, dev := newModelPanel(t, tc.m, func(op byte, _ []byte) []byte { return reply(op, 0) })
			want := []byte{0x7B, 0x33, 0x34, 0x0A, 0x66, 0x0E}
			if tc.turn {
				want = []byte{0x7B, 0x33, 0x34, 0x0D, 0x0A, 0x66, 0x0E}
			}
			if !bytes.Equal(dev.ops(), want) {
				t.Fatalf("init ops % x, want % x", dev.ops(), want)
			}
			if p.Size() != image.Pt(tc.w, tc.h) {
				t.Fatalf("size %v", p.Size())
			}
			// overlay is a full-size transparent PNG of the native portrait frame
			ov := dev.writes[len(dev.writes)-2]
			if n := binary.BigEndian.Uint32(decryptBlock(ov[:504])[8:12]); int(n) != len(ov)-512 {
				t.Fatalf("overlay length field %d, data %d", n, len(ov)-512)
			}
			dev.writes = nil
			p.SetBrightness(tc.brightIn)
			if pl := dev.plainOf(0); pl[0] != 0x0E || pl[8] != tc.brightEx {
				t.Fatalf("brightness byte %d, want %d", pl[8], tc.brightEx)
			}
			// a landscape picture reaches the device as native portrait JPEG
			dev.writes = nil
			if err := p.ShowImage(image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))); err != nil {
				t.Fatal(err)
			}
			w := dev.writes[0]
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(w[512:]))
			if err != nil || cfg.Width != tc.h || cfg.Height != tc.w {
				t.Fatalf("device JPEG %+v %v, want %dx%d", cfg, err, tc.h, tc.w)
			}
			if p.ShowImage(image.NewRGBA(image.Rect(0, 0, 1920, 480+1))) == nil && tc.m != &usModel88 {
				t.Fatal("wrong size accepted")
			}
		})
	}
}

func TestBarRotationStaysInSoftware(t *testing.T) {
	p, dev := newModelPanel(t, &usModel92, func(op byte, _ []byte) []byte { return reply(op, 0) })
	dev.writes = nil
	p.SetRotation(180)
	p.SetRotation(90)
	if len(dev.writes) != 0 {
		t.Fatal("bar rotation reached the device")
	}
	if !p.SoftQuarterTurns() {
		t.Fatal("bars want portrait canvases")
	}
}

func TestFlexSequence(t *testing.T) {
	p, dev := newModelPanel(t, &usModelTLFlex, func(op byte, _ []byte) []byte { return reply(op, 2) })
	if want := []byte{0x80, 0x0A, 0x66, 0x0D, 0x0E}; !bytes.Equal(dev.ops(), want) {
		t.Fatalf("init ops % x, want % x", dev.ops(), want)
	}
	if pl := dev.plainOf(3); pl[8] != 1 {
		t.Fatalf("default rotation wire value %d, want 1", pl[8])
	}
	// 8.8 framing: JPEG with variable length and trailer
	dev.writes = nil
	if err := p.ShowImage(image.NewRGBA(image.Rect(0, 0, 400, 400))); err != nil {
		t.Fatal(err)
	}
	w := dev.writes[0]
	if w[510] != 0xA1 || w[511] != 0x1A || len(w) == slTransferLen {
		t.Fatal("flex pictures use the 8.8 framing")
	}
	// the hand-over on Close is the only 0x7F
	dev.writes = nil
	p.Close()
	if want := []byte{0x66, 0x7F}; !bytes.Equal(dev.ops(), want) {
		t.Fatalf("close ops % x, want % x", dev.ops(), want)
	}
	if p.ShowImage(image.NewRGBA(image.Rect(0, 0, 400, 400))) == nil {
		t.Fatal("frame accepted after Close")
	}
	if p.Round() {
		t.Fatal("flex is square")
	}
}

func TestNoForbiddenOpsAnywhere(t *testing.T) {
	for _, m := range []*usModel{&usModel88, &usModel92, &usModel207, &usModelSL, &usModelTL2, &usModelTLFlex, &usModelSLFlex} {
		t.Run(m.name, func(t *testing.T) {
			p, dev := newModelPanel(t, m, func(op byte, _ []byte) []byte { return reply(op, 2) })
			sz := p.Size()
			for i := 0; i < 3; i++ {
				p.ShowImage(image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y)))
			}
			p.SetBrightness(0)
			p.SetBrightness(80)
			p.SetRotation(270)
			p.SetRotation(0)
			p.heartbeat(time.Now().Add(time.Hour))
			p.Close()
			for i, op := range dev.ops() {
				if op == 0x33 && dev.plainOf(i)[15] != usClockSetOnly {
					t.Fatalf("clock flag %d written without a user request", dev.plainOf(i)[15])
				}
				if usForbiddenOps[op] {
					t.Fatalf("forbidden operation %#x was sent", op)
				}
				if op == 0x7F && m.init != usInitFlex {
					t.Fatal("hand-over sent by a device without one")
				}
			}
			// plain data transfers carry no hidden commands: only the first block of
			// each transfer is a command
			for i, w := range dev.writes {
				if len(w) < 512 {
					t.Fatalf("write %d is only %d bytes", i, len(w))
				}
			}
		})
	}
}

func TestStampsStrictlyIncreasePerPanel(t *testing.T) {
	_, dev := newModelPanel(t, &usModelSL, func(op byte, _ []byte) []byte { return reply(op, 2) })
	var prev uint32
	for i := range dev.writes {
		st := binary.LittleEndian.Uint32(dev.plainOf(i)[4:8])
		if i > 0 && st <= prev {
			t.Fatalf("stamp %d after %d", st, prev)
		}
		prev = st
	}
}

func TestDeviceOfflineAtStart(t *testing.T) {
	old := us88Open
	defer func() { us88Open = old }()
	us88Open = func(winusb.Info) (winusb.Device, error) { return &cmdUSB{}, nil }
	p := &universal88{m: &usModelTLFlex, name: "x", brightness: 100}
	if err := p.connect(); err == nil {
		t.Fatal("a Flex screen that does not answer the query was taken for online")
	}
}

func TestVersionParsing(t *testing.T) {
	for _, tc := range []struct {
		f    func(string) (string, bool)
		in   string
		want string
		ok   bool
	}{
		{usVersion88, "lianli88_0001_0020", "1.2", true},
		{usVersion88, "lianli88_0001_0017", "", false},
		{usVersion207, "lianli207_0001_0023", "2.3", true},
		{usVersion207, "lianli207_0001_0009", "", false},
		{usVersion207, "garbage", "", false},
		{usVersionDigits, "lianli92_2025010203", "2025010203", true},
		{usVersionDigits, "lianli92_20250102xx", "", false},
		{usVersionDigits, "short", "", false},
		{usVersionLastPart, "lianliflex_0001_1.7", "1.7", true},
		{usVersionLastPart, "nounderscore", "", false},
		{usVersionLastPart, "trailing_", "", false},
	} {
		got, ok := tc.f(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q: got %q %v, want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	// the firmware string of an 0x0A reply is kept, bounded and printable
	p := &universal88{}
	r := make([]byte, 512)
	r[0] = 0x0A
	copy(r[8:], "lianli88_0001_0020\x00junk")
	p.noteFirmware(r)
	if p.fw != "lianli88_0001_0020" {
		t.Fatalf("fw %q", p.fw)
	}
	p.noteFirmware([]byte{0x0A, 0, 0}) // truncated reply must not panic
}

func TestUSBChain(t *testing.T) {
	hub, port, ok := usbChain("PCIROOT(0)#PCI(0801)#USBROOT(0)#USB(3)#USB(2)")
	if !ok || port != 1 || hub != "PCIROOT(0)#PCI(0801)#USBROOT(0)#USB(3)" {
		t.Fatalf("%q %d %v", hub, port, ok)
	}
	for _, bad := range []string{"", "PCIROOT(0)", "USB(0)", "USB(x)", "USB(2)#PCI(1)"} {
		if _, _, ok := usbChain(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
	all := []winusb.Info{
		{Serial: "aaa", Location: "ROOT#USB(1)#USB(3)"},
		{Serial: "bbb", Location: "ROOT#USB(1)#USB(1)"},
		{Serial: "ccc", Location: "ROOT#USB(2)#USB(2)"},
	}
	taken := map[string]bool{}
	id, name := usModelSL.identify(all[0], 0, all, taken)
	if id != "slv3-aaa" || name != "SL V3 LCD (group 1, position 3)" {
		t.Fatalf("%q %q", id, name)
	}
	id, name = usModelSL.identify(all[2], 2, all, taken)
	if id != "slv3-ccc" || name != "SL V3 LCD (group 2, position 2)" {
		t.Fatalf("%q %q", id, name)
	}
	// no location, duplicate or missing serial: still unique IDs
	a, _ := usModelSL.identify(winusb.Info{Serial: "aaa"}, 3, all, taken)
	b, _ := usModelSL.identify(winusb.Info{}, 4, all, taken)
	c, _ := usModelSL.identify(winusb.Info{}, 5, all, taken)
	if a == "slv3-aaa" || b == c || a == b {
		t.Fatalf("ids not unique: %q %q %q", a, b, c)
	}
	// bars are numbered
	if id, name := usModel92.identify(winusb.Info{}, 1, nil, nil); id != "us92-1" || name != "Vision-M 9.2\" #2" {
		t.Fatalf("%q %q", id, name)
	}
}
