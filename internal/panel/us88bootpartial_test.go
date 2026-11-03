// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package panel

import (
	"errors"
	"image"
	"testing"

	"github.com/kewe87/casehud/internal/winusb"
)

// partialUSB reports a timeout after part of a large write went out and
// records the timeout of every write and the Close calls.
type partialUSB struct {
	cmdUSB
	timeouts []int
	partial  int  // bytes accepted before the timeout, for writes over 8 KB
	whole    bool // accept large writes completely
	closes   int
}

func (s *partialUSB) Write(ep byte, buf []byte, ms int) (int, error) {
	s.timeouts = append(s.timeouts, ms)
	if len(buf) > 8192 && !s.whole {
		s.writes = append(s.writes, append([]byte(nil), buf[:s.partial]...))
		return s.partial, winusb.ErrTimeout
	}
	return s.cmdUSB.Write(ep, buf, ms)
}

func (s *partialUSB) Close() error { s.closes++; return nil }

func TestUS88BootPartialWrite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, us88Height, us88Width))
	for _, partial := range []int{0, 4096} {
		s := &partialUSB{partial: partial}
		p := &universal88{dev: s, name: "us88", brightness: 100}
		err := p.WriteBootImage(img, 0)
		if !errors.Is(err, ErrBootPartial) {
			t.Fatalf("partial %d: %v, want ErrBootPartial", partial, err)
		}
		if p.dev == nil || s.closes != 0 {
			t.Fatalf("partial %d: device was dropped or reopened (closes %d)", partial, s.closes)
		}
		if p.failures != 0 || !p.nextTry.IsZero() {
			t.Fatal("recover/back-off ran after a partial write")
		}
		big := 0
		for _, ms := range s.timeouts {
			if ms >= 10000 {
				big++
			}
		}
		if big != 1 {
			t.Fatalf("timeouts %v: want exactly one boot write attempt with >= 10 s", s.timeouts)
		}
	}
}

func TestUS88BootKeepsNormalTimeout(t *testing.T) {
	s := &partialUSB{whole: true}
	s.script = func(op byte, _ []byte) []byte {
		if op == byte(usOpWriteFile) {
			return []byte{0x28, 0xC8}
		}
		return nil
	}
	p := &universal88{dev: s, name: "us88", brightness: 100}
	if err := p.WriteBootImage(image.NewRGBA(image.Rect(0, 0, us88Height, us88Width)), 0); err != nil {
		t.Fatal(err)
	}
	if err := p.write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	n := len(s.timeouts)
	if s.timeouts[n-1] != us88Timeout {
		t.Fatalf("timeout after the boot transfer is %d, want %d", s.timeouts[n-1], us88Timeout)
	}
	for _, ms := range s.timeouts[:n-1] {
		if ms != us88Timeout && ms < 10000 {
			t.Fatalf("unexpected timeout %d", ms)
		}
	}
}
