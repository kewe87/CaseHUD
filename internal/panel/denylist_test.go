package panel

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"image"
	"image/jpeg"
	"testing"
)

// The deny-list test: every display driver is run through start-up, frames,
// brightness, rotation and Close against a recording fake, and no command
// outside the documented allow-list may be written. The forbidden classes are
// firmware update, flash or boot image, serial number write outside tlpersist.go, configuration
// erase, reboot, monitor mode and USB reset: none of them is in any allow-list.

// denyCovered records which display drivers the test exercised.
var denyCovered = map[string]bool{}

func blackJPEG(t *testing.T, size image.Point) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rectangle{Max: size}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func exercise(t *testing.T, p Panel) {
	t.Helper()
	size := p.Size()
	for _, deg := range []int{0, 90, 180, 270} {
		p.SetRotation(deg)
		p.SetBrightness(deg / 4)
		p.ShowJPEG(blackJPEG(t, size))
	}
	p.SetBrightness(100)
	p.Close()
}

// allUSBBlocks decodes the op byte of the first block of every recorded write.
func usOps(t *testing.T, f *fakeUSB) []byte {
	var ops []byte
	blk, _ := des.NewCipher(usKey)
	for _, w := range f.writes {
		if len(w) < 512 {
			t.Fatalf("write of %d bytes without a command block", len(w))
		}
		out := make([]byte, 8)
		cipher.NewCBCDecrypter(blk, usKey).CryptBlocks(out, w[:8])
		ops = append(ops, out[0])
	}
	return ops
}

type denyUSB struct{ fakeUSB }

func (f *denyUSB) Read(ep byte, buf []byte, _ int) (int, error) {
	r := make([]byte, 512)
	r[0], r[8] = byte(usOpCheck), 2
	return copy(buf, r), nil
}

func TestDenyListTL(t *testing.T) {
	f := &fakeHID{}
	p := &tlLCD{dev: f, brightness: 50}
	exercise(t, p)
	denyCovered["tllcd"] = true
	// The start image (0x48) and the serial number (0x3F) are persistent
	// writes: they are not in this list and may only come from their own
	// functions (see TestPersistentOpsOnlyThroughTheirFunctions).
	allowed := map[byte]bool{byte(tlOpVersion): true, byte(tlOpStatus): true, byte(tlOpIdentity): true, byte(tlOpSettings): true, byte(tlOpStream): true}
	if len(f.writes) == 0 {
		t.Fatal("nothing written")
	}
	for _, w := range f.writes {
		if !allowed[w[1]] {
			t.Errorf("tllcd wrote op %#x", w[1])
		}
	}
}

func TestDenyListUniversal(t *testing.T) {
	allowed := map[byte]bool{}
	for _, o := range []usOp{usOpInfo, usOpFPS, usOpTurn, usOpBacklight, usOpClock, usOpClockOff, usOpPicture, usOpOverlay, usOpVideo, usOpQueue, usOpHalt, usOpHandOver, usOpCheck} {
		allowed[byte(o)] = true
	}
	for _, m := range []*usModel{&usModel88, &usModel92, &usModel207, &usModelSL, &usModelTL2, &usModelTLFlex, &usModelSLFlex} {
		f := &denyUSB{}
		p := &universal88{dev: f, m: m, id: m.idPrefix + "-0", name: m.name, brightness: 100, stop: make(chan struct{})}
		if err := p.wake(); err != nil {
			t.Fatalf("%s: wake: %v", m.name, err)
		}
		exercise(t, p)
		denyCovered[m.driver] = true
		ops := usOps(t, &f.fakeUSB)
		if len(ops) == 0 {
			t.Fatalf("%s: nothing written", m.name)
		}
		for i, op := range ops {
			if !allowed[op] {
				t.Errorf("%s wrote op %#x", m.name, op)
			}
			// The clock command may only set the time: the flags that store the
			// offline clock are written on a user request, never by this run.
			if op == byte(usOpClock) && usOpen(t, f.writes[i])[15] != usClockSetOnly {
				t.Errorf("%s wrote a stored clock flag", m.name)
			}
		}
	}
}

func TestDenyListAIO(t *testing.T) {
	for pid := range hsModels {
		for _, version := range []string{"1.0", "1.5", "2.0"} {
			f := &hsFake{version: version, avail: 1, resetOK: true}
			p := newHydroShift(f, pid, 0)
			startAIO(p, 4)
			exercise(t, p)
			denyCovered["aio"] = true
			if len(f.writes) == 0 {
				t.Fatal("nothing written")
			}
			for _, c := range f.cmds() {
				if hsForbidden[c[1]] {
					t.Errorf("%04x fw %s wrote forbidden command %#x", pid, version, c[1])
				}
				if !hsAllowed[c[1]] && !(c[0] == 1 && c[1] == hsCmdReset) {
					t.Errorf("%04x fw %s wrote unlisted command %#x", pid, version, c[1])
				}
			}
		}
	}
}

// TestDenyListCoversAllDrivers fails when a display driver is added without
// being run through the deny-list test.
func TestDenyListCoversAllDrivers(t *testing.T) {
	// The other deny-list tests fill denyCovered; run them first.
	TestDenyListTL(t)
	TestDenyListUniversal(t)
	TestDenyListAIO(t)
	for _, d := range Drivers() {
		if d.Kind == KindDisplay && !denyCovered[d.ID] {
			t.Errorf("display driver %q has no deny-list test", d.ID)
		}
	}
}
