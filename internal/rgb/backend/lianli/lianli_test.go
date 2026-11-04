package lianli

import (
	"bytes"
	"errors"
	"image/color"
	"testing"
	"time"
)

// ringIn is a ring that also answers the version request.
type ringIn struct {
	ring
	answer []byte
}

func (r *ringIn) Read(buf []byte, _ time.Duration) (int, error) {
	if r.answer == nil {
		return 0, errors.New("timeout")
	}
	return copy(buf, r.answer), nil
}

func TestFirmwareVersion(t *testing.T) {
	r := &ringIn{answer: []byte{0x10, 1, 3}}
	d := &device{leds: ring88LEDs, open: func() (transport, error) { return r, nil }}
	if d.Firmware() != "" {
		t.Fatal("version before the ring was opened")
	}
	if err := d.SetFrame(map[string][]color.RGBA{}); err != nil {
		t.Fatal(err)
	}
	req := r.packets[0]
	if len(req) != packetLen || req[0] != 0x10 || !bytes.Equal(req[1:], make([]byte, packetLen-1)) {
		t.Fatalf("request % x", req)
	}
	if d.Firmware() != "1_3" {
		t.Fatalf("version %q", d.Firmware())
	}
	// only the version request and the three LED packets were sent
	if len(r.packets) != 4 {
		t.Fatalf("%d packets", len(r.packets))
	}
}

func TestFirmwareVersionFailuresAreSilent(t *testing.T) {
	for name, tr := range map[string]transport{
		"no IN pipe": &ring{},
		"timeout":    &ringIn{},
		"major 0":    &ringIn{answer: []byte{0x10, 0, 3}},
		"wrong cmd":  &ringIn{answer: []byte{0x11, 1, 3}},
		"short":      &ringIn{answer: []byte{0x10, 1}},
	} {
		d := &device{leds: ring88LEDs, open: func() (transport, error) { return tr, nil }}
		if err := d.SetFrame(map[string][]color.RGBA{}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d.Firmware() != "" {
			t.Fatalf("%s: version %q", name, d.Firmware())
		}
	}
}

type ring struct {
	packets [][]byte
	fail    bool
	closed  int
}

func (r *ring) Write(b []byte) error {
	if r.fail {
		return errors.New("gone")
	}
	r.packets = append(r.packets, append([]byte(nil), b...))
	return nil
}

func (r *ring) Close() error { r.closed++; return nil }

func TestFramePackets(t *testing.T) {
	r := &ring{}
	opens := 0
	d := &device{leds: ring88LEDs, open: func() (transport, error) { opens++; return r, nil }}
	if opens != 0 {
		t.Fatal("opened before the first frame")
	}
	cols := make([]color.RGBA, ring88LEDs)
	cols[0] = color.RGBA{1, 2, 3, 255}
	cols[21] = color.RGBA{4, 5, 6, 255}
	cols[59] = color.RGBA{7, 8, 9, 255}
	if err := d.SetFrame(map[string][]color.RGBA{"ring": cols}); err != nil {
		t.Fatal(err)
	}
	if len(r.packets) != 3 || opens != 1 {
		t.Fatalf("%d packets, %d opens", len(r.packets), opens)
	}
	for i, p := range r.packets {
		if len(p) != packetLen || p[0] != 0x11 || p[1] != byte(20*i) || p[2] != 0 || p[3] != 0 {
			t.Fatalf("packet %d header % x", i, p[:4])
		}
	}
	if !bytes.Equal(r.packets[0][4:7], []byte{1, 2, 3}) || !bytes.Equal(r.packets[1][7:10], []byte{4, 5, 6}) ||
		!bytes.Equal(r.packets[2][61:64], []byte{7, 8, 9}) {
		t.Fatal("colors misplaced")
	}

	// write error: handle dropped, reopened on the next frame
	r.fail = true
	if d.SetFrame(map[string][]color.RGBA{"ring": cols}) == nil || r.closed != 1 {
		t.Fatal("error not handled")
	}
	r.fail = false
	d.SetFrame(map[string][]color.RGBA{"ring": cols})
	if opens != 2 {
		t.Fatalf("opens %d", opens)
	}
}
