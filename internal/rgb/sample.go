package leds

import (
	"image"
	"image/color"
	"strings"
)

// sampleRadius is the half size of the box averaged around each LED position.
const sampleRadius = 2

// Sample reads the canvas color at each point (box average around the pixel)
// into dst, which must have len(pts) elements.
func Sample(c *image.RGBA, pts []Point, dst []color.RGBA) {
	b := c.Bounds()
	w, h := b.Dx(), b.Dy()
	for i, p := range pts {
		px := b.Min.X + int(p.X*float64(w-1)+0.5)
		py := b.Min.Y + int(p.Y*float64(h-1)+0.5)
		var r, g, bl, n int
		for y := max(py-sampleRadius, b.Min.Y); y <= min(py+sampleRadius, b.Max.Y-1); y++ {
			off := c.PixOffset(0, y)
			for x := max(px-sampleRadius, b.Min.X); x <= min(px+sampleRadius, b.Max.X-1); x++ {
				o := off + x*4
				r += int(c.Pix[o])
				g += int(c.Pix[o+1])
				bl += int(c.Pix[o+2])
				n++
			}
		}
		dst[i] = color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255}
	}
}

// scale applies brightness (0..256, 256 = full) in place.
func scale(cols []color.RGBA, f int) {
	if f >= 256 {
		return
	}
	for i, c := range cols {
		cols[i] = color.RGBA{uint8(int(c.R) * f >> 8), uint8(int(c.G) * f >> 8), uint8(int(c.B) * f >> 8), 255}
	}
}

// reorder converts RGB colors into the wire order (e.g. "GRB") in place: the
// R, G, B fields of the result carry the bytes in wire order.
func reorder(cols []color.RGBA, order string) {
	order = strings.ToUpper(order)
	if order == "" || order == "RGB" || len(order) != 3 {
		return
	}
	for i, c := range cols {
		var out [3]uint8
		for j := 0; j < 3; j++ {
			switch order[j] {
			case 'R':
				out[j] = c.R
			case 'G':
				out[j] = c.G
			case 'B':
				out[j] = c.B
			}
		}
		cols[i] = color.RGBA{out[0], out[1], out[2], 255}
	}
}
