// Package icon draws the app icon and encodes it as a Windows .ico, so no
// binary assets need to live in the repo.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

// Image draws the logo (an open accent ring) at size x size.
func Image(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	outer, inner := c-0.5, c*0.55
	accent := color.RGBA{0x22, 0xd3, 0xee, 0xff}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-c, float64(y)+0.5-c
			d := math.Hypot(dx, dy)
			a := math.Atan2(dy, dx) * 180 / math.Pi // gap towards the top right
			if d > outer || d < inner || (a > -80 && a < -10) {
				continue
			}
			cov := math.Min(1, math.Min(outer-d, d-inner))
			img.SetRGBA(x, y, color.RGBA{uint8(float64(accent.R) * cov), uint8(float64(accent.G) * cov), uint8(float64(accent.B) * cov), uint8(255 * cov)})
		}
	}
	return img
}

// ICO encodes the logo as a 32-bit BMP-based .ico with the given sizes.
func ICO(sizes ...int) []byte {
	var dir, data bytes.Buffer
	binary.Write(&dir, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for _, s := range sizes {
		bmp := bmpData(Image(s))
		wb := byte(s)
		if s >= 256 {
			wb = 0
		}
		dir.Write([]byte{wb, wb, 0, 0})
		binary.Write(&dir, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&dir, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&dir, binary.LittleEndian, uint32(len(bmp)))
		binary.Write(&dir, binary.LittleEndian, uint32(offset))
		offset += len(bmp)
		data.Write(bmp)
	}
	return append(dir.Bytes(), data.Bytes()...)
}

func bmpData(img *image.RGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Size                     uint32
		Width, Height            int32
		Planes, BitCount         uint16
		Compression, SizeImage   uint32
		XPels, YPels, Used, Impt int32
	}{40, int32(w), int32(h * 2), 1, 32, 0, 0, 0, 0, 0, 0})
	for y := h - 1; y >= 0; y-- { // bottom-up BGRA
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, ((w+31)/32)*4*h)) // AND mask, unused with alpha
	return b.Bytes()
}
