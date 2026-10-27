// Package panel drives PC case LCD screens: device discovery, init and JPEG
// frame streaming.
package panel

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// Panel is one physical LCD.
type Panel interface {
	// ID is stable across restarts as long as the cabling doesn't change,
	// e.g. "tllcd-p1-i0" (port 1, fan 0). Used as the config key.
	ID() string
	// Name is a human-readable description, e.g. "UNI FAN TL LCD (port 1, fan 1)".
	Name() string
	// Size is the native resolution.
	Size() image.Point
	// ShowJPEG streams one encoded frame.
	ShowJPEG(jpeg []byte) error
	// MaxJPEG is the largest encoded frame the panel accepts.
	MaxJPEG() int
	// SetBrightness sets the backlight, 0-100.
	SetBrightness(percent int) error
	// SetRotation rotates the image on the device, 0/90/180/270.
	SetRotation(degrees int) error
	Close() error
}

// RawPanel is implemented by panels that can take an unencoded picture. They
// do their own (single) encode, so the host skips a JPEG round trip.
type RawPanel interface {
	Panel
	// ShowImage displays img, which must have the panel's native size (or the
	// canvas size the panel asks for). The panel must not keep or modify img.
	ShowImage(img image.Image) error
}

// EncodeJPEG encodes img for p, lowering the quality until the frame fits the
// panel's payload limit.
func EncodeJPEG(p Panel, img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	for q := 90; q >= 30; q -= 10 {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, err
		}
		if buf.Len() <= p.MaxJPEG() {
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("frame too large for %s even at low JPEG quality (%d > %d bytes)", p.Name(), buf.Len(), p.MaxJPEG())
}

// Show encodes and streams img to p.
func Show(p Panel, img image.Image) error {
	if rp, ok := p.(RawPanel); ok {
		return rp.ShowImage(img)
	}
	b, err := EncodeJPEG(p, img)
	if err != nil {
		return err
	}
	return p.ShowJPEG(b)
}
