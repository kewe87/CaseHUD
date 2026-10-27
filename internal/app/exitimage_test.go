// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"image"
	"testing"
)

func TestExitImageOrientation(t *testing.T) {
	for _, sz := range []image.Point{{1920, 480}, {480, 1920}} {
		img, err := exitImagePixels(sz)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Size() != sz {
			t.Fatalf("size %v, want %v", img.Bounds().Size(), sz)
		}
		lit := func(x0, x1 int) bool {
			for y := 0; y < sz.Y; y++ {
				for x := x0; x < x1; x++ {
					if c := img.RGBAAt(x, y); c.R > 0xc0 {
						return true
					}
				}
			}
			return false
		}
		// The name (near white) is there in landscape only.
		if got := lit(0, sz.X); got != (sz.X > sz.Y) {
			t.Fatalf("%v: wordmark present = %v", sz, got)
		}
		// The ring of the symbol lies below its centre; the symbol is centred
		// on a tall display and left of the name on a wide one.
		if sz.X < sz.Y {
			if c := img.RGBAAt(sz.X/2, sz.Y/2+120); c.B < 0x80 {
				t.Fatalf("%v: no symbol at the centre: %v", sz, c)
			}
		}
	}
}
