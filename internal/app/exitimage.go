// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"image"
	"image/draw"

	"github.com/kewe87/casehud/internal/icon"
	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/panel"
)

// When the app quits, the displays that can leave a picture (panel.ExitImager)
// show the logo instead of the last frame. It is an ordinary frame sent while
// closing, nothing is stored on the device. Displays the user switched off
// keep that state.

const exitLogo = 300 // symbol size in px

// exitImagePixels draws the exit picture for a display of size sz (the frame
// as the engine composes it): the logo with the name next to it on a wide
// display, the symbol alone on a tall one.
func exitImagePixels(sz image.Point) (*image.RGBA, error) {
	l := lcd.DefaultLayout()
	l.Width, l.Height = sz.X, sz.Y
	l.Background, l.Panel, l.Round = "#0b0f14", "", false
	l.Widgets = nil
	logo := min(exitLogo, sz.X*3/4, sz.Y*3/4)
	at := image.Pt((sz.X-logo)/2, (sz.Y-logo)/2)
	if sz.X > sz.Y {
		const textW, gap = 440, 70
		at.X = (sz.X - logo - gap - textW) / 2
		l.Widgets = []lcd.Widget{
			{Type: "text", X: at.X + logo + gap, Y: at.Y + logo/2 - 65, W: textW, H: 130,
				Text: "CaseHUD", Size: 120, Align: "left", Color: "#e6edf3"},
		}
	}
	r, err := lcd.NewRenderer(l)
	if err != nil {
		return nil, err
	}
	img := r.Render(&metrics.Func{})
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	draw.Draw(out, image.Rectangle{Min: at, Max: at.Add(image.Pt(logo, logo))}, icon.Image(logo), image.Point{}, draw.Over)
	return out, nil
}

// setExitImage hands the exit picture to the panel, if it takes one and is
// not switched off. Called only when the app quits, before the panel closes.
func (w *worker) setExitImage() {
	ei, ok := w.p.(panel.ExitImager)
	if !ok {
		return
	}
	c := w.current()
	if c.Mode == "off" {
		return
	}
	img, err := exitImagePixels(w.sizeFor(c.Rotation))
	if err != nil {
		return
	}
	ei.SetExitImage(img)
}
