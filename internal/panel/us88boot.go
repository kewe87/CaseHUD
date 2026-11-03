// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package panel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"time"

	"github.com/kewe87/casehud/internal/winusb"
)

// Boot picture of the Universal Screen 8.8": the device shows the file
// /usr/data/boot.jpg while it starts. It is written with the device's
// file-write command, which is a permanent write and has no checks of its
// own on the device side:
//
//   - the path is a constant here and never built from input;
//   - the announced size must match the data exactly (a short transfer leaves
//     the device waiting until it loses power) and stays far below the
//     device's receive buffer, so it is capped at us88BootMax;
//   - header and data go out as one transfer.
//
// The picture is a baseline JPEG in the device's portrait frame (480x1920).

const (
	usOpWriteFile usOp = 0x28
	us88BootPath       = "/usr/data/boot.jpg"
	us88BootMax        = 204800
)

// ErrBootPartial is returned when the boot transfer broke off after (or
// possibly after) some bytes had gone out. The device then waits for the rest
// of the file and does not react until it loses power.
var ErrBootPartial = errors.New("boot picture transfer interrupted: the display may be waiting for data; switch the display off (unplug USB power) and on again")

// us88BootTimeoutMin and us88BootTimeoutPerKB set the write timeout of the
// boot transfer: far above the 1 s of ordinary frames, because a timeout in
// the middle of the transfer is what leaves the device hanging.
const (
	us88BootTimeoutMin   = 10 * time.Second
	us88BootTimeoutPerKB = 50 * time.Millisecond
)

// us88BootTimeoutMS is the transfer timeout for n bytes, in ms.
func us88BootTimeoutMS(n int) int {
	return int(max(us88BootTimeoutMin, time.Duration(n/1024+1)*us88BootTimeoutPerKB) / time.Millisecond)
}

// BootImager is implemented by panels that keep a boot picture of their own.
type BootImager interface {
	Panel
	// SettingsKey identifies the device.
	SettingsKey() string
	// HasBootImage reports whether this device keeps a boot picture.
	HasBootImage() bool
	// WriteBootImage stores img as the boot picture. img is the canvas for
	// rotation (wide 1920x480 or upright 480x1920), turned into the device
	// frame like every other picture. Permanent write: call it rarely. An
	// error wrapping ErrImageWritten means the data went out completely but
	// the device did not confirm it.
	WriteBootImage(img image.Image, rotation int) error
}

// HasBootImage implements BootImager.
func (p *universal88) HasBootImage() bool { return p.mdl().bootLogo }

// us88BootJPEG encodes the portrait frame as a baseline JPEG of at most
// us88BootMax bytes, starting at quality 95 and stepping down by 5.
func us88BootJPEG(frame image.Image) ([]byte, error) {
	var buf bytes.Buffer
	for q := 95; q >= 30; q -= 5 {
		buf.Reset()
		if err := jpeg.Encode(&buf, frame, &jpeg.Options{Quality: q}); err != nil {
			return nil, err
		}
		if buf.Len() <= us88BootMax {
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("boot picture too large (%d bytes)", buf.Len())
}

// bootTransfer is the complete file-write transfer for data: the command
// block (path length and size, both big endian, then the path) followed
// directly by the data.
func (p *universal88) bootTransfer(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > us88BootMax {
		return nil, fmt.Errorf("boot picture has %d bytes (1..%d allowed)", len(data), us88BootMax)
	}
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 || data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		return nil, errors.New("boot picture is not a complete JPEG")
	}
	args := make([]byte, 8+len(us88BootPath))
	binary.BigEndian.PutUint32(args[0:4], uint32(len(us88BootPath)))
	binary.BigEndian.PutUint32(args[4:8], uint32(len(data)))
	copy(args[8:], us88BootPath)
	return append(p.seal88(usOpWriteFile, args), data...), nil
}

// WriteBootImage implements BootImager (8.8" only).
func (p *universal88) WriteBootImage(img image.Image, rotation int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.mdl().bootLogo {
		return fmt.Errorf("%s: no boot picture on this model", p.name)
	}
	if p.closed {
		return ErrClosed
	}
	if err := p.checkSize(img.Bounds().Size()); err != nil {
		return err
	}
	jpg, err := us88BootJPEG(p.orientFor(toRGBA(img), rotation))
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	tr, err := p.bootTransfer(jpg)
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	if err := p.reopen(); err != nil {
		return fmt.Errorf("%s: %w: %w", p.name, ErrClosed, err)
	}
	// No video or layer may be running while the file goes in, and no old
	// reply may be read as its answer.
	if err := p.leaveLayers(); err != nil {
		return err
	}
	p.drain()
	sent, err := p.writeTO(tr, us88BootTimeoutMS(len(tr)))
	if err != nil {
		if sent > 0 || errors.Is(err, winusb.ErrTimeout) {
			// Possibly a partial file on the device: touch nothing (no
			// recover, reopen or init, no retry); only a power cycle helps.
			log.Printf("%s: boot picture: %d of %d bytes sent: %v", p.name, sent, len(tr), err)
			return fmt.Errorf("%w: %s: %d of %d bytes sent: %w", ErrBootPartial, p.name, sent, len(tr), err)
		}
		p.recover() // nothing went out: an ordinary failure
		return fmt.Errorf("%s: boot picture: %w", p.name, err)
	}
	r, err := p.ack()
	if err != nil || len(r) < 2 {
		log.Printf("warning: %s: boot picture sent completely but the device did not confirm it (%v)", p.name, err)
		return fmt.Errorf("%w: %s: boot picture sent, no confirmation", ErrImageWritten, p.name)
	}
	if r[0] != byte(usOpWriteFile) || r[1] != 0xC8 {
		return fmt.Errorf("%s: boot picture: device answered % x", p.name, r[:2])
	}
	return nil
}
