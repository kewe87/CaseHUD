// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"log"

	"github.com/kewe87/casehud/internal/panel"
)

// The 8.8" bar shows a boot picture while it starts. With the start image
// setting on, CaseHUD writes its logo there once per device (a permanent
// write) under the same rules as the fan start images: the record holds the
// hash of what was written, a device gets at most one attempt per app run,
// nothing happens before the disclaimer was accepted. A wide bar gets the
// logo with the name, an upright one the symbol alone; turning the bar
// changes the picture, so it is written again with the next device search.

// bootImageVersion is part of the hash: raise it when the drawing changes.
const bootImageVersion = "1"

// bootImage draws the boot picture for a canvas of size sz and returns it
// with its hash (pixels, size and rotation, not the encoder's output).
func bootImage(sz image.Point, rotation int) (*image.RGBA, string, error) {
	img, err := exitImagePixels(sz)
	if err != nil {
		return nil, "", err
	}
	if img.Bounds().Size() != sz {
		return nil, "", fmt.Errorf("boot picture has size %v", img.Bounds().Size())
	}
	h := sha256.New()
	fmt.Fprintf(h, "casehud-boot-image-%s\n%dx%d r%d\n", bootImageVersion, sz.X, sz.Y, rotation)
	h.Write(img.Pix)
	return img, hex.EncodeToString(h.Sum(nil))[:32], nil
}

// writeBootImage writes the boot picture to bi unless it already has it.
// Called from writeStartImages (which holds the write lock and has checked
// the setting and the disclaimer).
func (e *Engine) writeBootImage(bi panel.BootImager, force bool) {
	if !bi.HasBootImage() {
		return
	}
	e.mu.Lock()
	w := e.workers[bi.ID()]
	rotation := e.cfg.Panels[bi.ID()].Rotation
	e.mu.Unlock()
	if w == nil {
		return
	}
	img, hash, err := bootImage(w.sizeFor(rotation), rotation)
	if err != nil {
		log.Println("boot picture:", err)
		return
	}
	e.writeOne(bi, bi.SettingsKey(), hash, nil, force, "boot picture", "boot",
		func([]byte) error { return bi.WriteBootImage(img, rotation) })
}
