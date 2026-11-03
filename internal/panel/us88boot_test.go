// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package panel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"testing"
)

// bootWrites returns the file-write transfers among s.writes.
func bootWrites(s *cmdUSB) [][]byte {
	var out [][]byte
	for _, w := range s.writes {
		if len(w) >= 512 && decryptBlock(w[:504])[0] == byte(usOpWriteFile) {
			out = append(out, w)
		}
	}
	return out
}

func TestUS88BootImageTransfer(t *testing.T) {
	for _, tc := range []struct {
		rotation int
		canvas   image.Point
	}{{0, image.Pt(us88Height, us88Width)}, {180, image.Pt(us88Height, us88Width)}, {90, image.Pt(us88Width, us88Height)}} {
		s := &cmdUSB{script: func(op byte, _ []byte) []byte {
			if op == byte(usOpWriteFile) {
				return []byte{0x28, 0xC8}
			}
			return nil
		}}
		p := &universal88{dev: s, name: "us88", brightness: 100}
		if !p.HasBootImage() {
			t.Fatal("8.8\" must keep a boot picture")
		}
		if err := p.WriteBootImage(image.NewRGBA(image.Rectangle{Max: tc.canvas}), tc.rotation); err != nil {
			t.Fatalf("rotation %d: %v", tc.rotation, err)
		}
		ws := bootWrites(s)
		if len(ws) != 1 {
			t.Fatalf("rotation %d: %d file writes, want exactly one transfer", tc.rotation, len(ws))
		}
		w := ws[0]
		plain := decryptBlock(w[:504])
		pathLen := binary.BigEndian.Uint32(plain[8:12])
		size := binary.BigEndian.Uint32(plain[12:16])
		if string(plain[16:16+pathLen]) != "/usr/data/boot.jpg" {
			t.Fatalf("path %q", plain[16:16+pathLen])
		}
		if plain[16+pathLen] != 0 {
			t.Fatal("path must not be followed by other bytes")
		}
		if w[510] != 0xA1 || w[511] != 0x1A {
			t.Fatal("8.8\" block trailer missing")
		}
		data := w[512:]
		if int(size) != len(data) || size > us88BootMax {
			t.Fatalf("announced %d bytes, sent %d (max %d)", size, len(data), us88BootMax)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != us88Width || cfg.Height != us88Height {
			t.Fatalf("rotation %d: boot jpeg %dx%d (%v), want the portrait device frame", tc.rotation, cfg.Width, cfg.Height, err)
		}
	}
}

func TestUS88BootImageAnswers(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, us88Height, us88Width))
	// No answer: the data went out completely, it counts as written.
	s := &cmdUSB{}
	p := &universal88{dev: s, name: "us88", brightness: 100}
	if err := p.WriteBootImage(img, 0); !errors.Is(err, ErrImageWritten) {
		t.Fatalf("no answer: %v, want ErrImageWritten", err)
	}
	// A wrong answer is an error.
	s = &cmdUSB{script: func(op byte, _ []byte) []byte {
		if op == byte(usOpWriteFile) {
			return []byte{0x28, 0x00}
		}
		return nil
	}}
	p = &universal88{dev: s, name: "us88", brightness: 100}
	if err := p.WriteBootImage(img, 0); err == nil || errors.Is(err, ErrImageWritten) {
		t.Fatalf("wrong answer: %v", err)
	}
	// A failed write is an error and not counted as written.
	s = &cmdUSB{failAt: 1}
	p = &universal88{dev: s, name: "us88", brightness: 100}
	if err := p.WriteBootImage(img, 0); err == nil || errors.Is(err, ErrImageWritten) {
		t.Fatalf("failed write: %v", err)
	}
}

func TestUS88BootImageOnly88(t *testing.T) {
	for _, m := range []*usModel{&usModel92, &usModelSL, &usModelTLFlex} {
		s := &cmdUSB{}
		p := &universal88{dev: s, m: m, name: m.name}
		if p.HasBootImage() {
			t.Fatalf("%s must not keep a boot picture", m.name)
		}
		if err := p.WriteBootImage(image.NewRGBA(image.Rect(0, 0, m.long, m.short)), 0); err == nil || len(s.writes) != 0 {
			t.Fatalf("%s: err %v, %d writes", m.name, err, len(s.writes))
		}
	}
}

func TestUS88BootTransferLimits(t *testing.T) {
	p := &universal88{name: "us88"}
	if _, err := p.bootTransfer(nil); err == nil {
		t.Fatal("empty file accepted")
	}
	big := make([]byte, us88BootMax+1)
	big[0], big[1], big[len(big)-2], big[len(big)-1] = 0xFF, 0xD8, 0xFF, 0xD9
	if _, err := p.bootTransfer(big); err == nil {
		t.Fatal("oversized file accepted")
	}
	if _, err := p.bootTransfer([]byte{1, 2, 3, 4, 5}); err == nil {
		t.Fatal("non-JPEG accepted")
	}
}
