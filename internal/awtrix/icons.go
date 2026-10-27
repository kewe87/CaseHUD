package awtrix

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"strings"
)

// Icons are drawn here instead of shipped as files: 8×8 for 32×8 clocks
// (TC001) and 16×16 for 52×16 clocks (TC002), where a small label under the
// symbol says what the number is.

var (
	cCyan   = rgb(0x22, 0xD3, 0xEE)
	cCyanD  = rgb(0x0B, 0x4F, 0x5C)
	cGreen  = rgb(0x76, 0xE0, 0x4A)
	cGreenD = rgb(0x24, 0x52, 0x18)
	cBlue   = rgb(0x3B, 0x82, 0xF6)
	cBlueL  = rgb(0x93, 0xC5, 0xFD)
	cViolet = rgb(0xA7, 0x8B, 0xFA)
	cAmber  = rgb(0xF5, 0x9E, 0x0B)
	cRed    = rgb(0xF4, 0x3F, 0x5E)
	cGold   = rgb(0xC8, 0x9B, 0x3C)
	cGrey   = rgb(0x5A, 0x64, 0x78)
	cTrack  = rgb(0x2A, 0x33, 0x45)
	cWhite  = rgb(0xE8, 0xEC, 0xF4)
	cBlack  = rgb(0, 0, 0)
)

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

// canvas is a tiny pixel surface; drawing outside is ignored.
type canvas struct{ *image.RGBA }

func newCanvas(w, h int) canvas {
	c := canvas{image.NewRGBA(image.Rect(0, 0, w, h))}
	draw.Draw(c.RGBA, c.Bounds(), image.NewUniform(cBlack), image.Point{}, draw.Src)
	return c
}

func (c canvas) px(x, y int, col color.RGBA) {
	if image.Pt(x, y).In(c.Bounds()) {
		c.SetRGBA(x, y, col)
	}
}

func (c canvas) fill(x, y, w, h int, col color.RGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.px(i, j, col)
		}
	}
}

func (c canvas) rect(x, y, w, h int, col color.RGBA) {
	for i := x; i < x+w; i++ {
		c.px(i, y, col)
		c.px(i, y+h-1, col)
	}
	for j := y; j < y+h; j++ {
		c.px(x, j, col)
		c.px(x+w-1, j, col)
	}
}

// grid paints rows of characters, each looked up in pal ('.' = nothing).
func (c canvas) grid(x, y int, rows []string, pal map[byte]color.RGBA) {
	for j, row := range rows {
		for i := 0; i < len(row); i++ {
			if col, ok := pal[row[i]]; ok {
				c.px(x+i, y+j, col)
			}
		}
	}
}

// font3x5 holds the glyphs CaseHUD needs, close to AWTRIX's own small font.
var font3x5 = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", ".##", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", ".#.", ".#.", ".#."},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'A': {".#.", "#.#", "###", "#.#", "#.#"},
	'C': {".##", "#..", "#..", "#..", ".##"},
	'D': {"##.", "#.#", "#.#", "#.#", "##."},
	'F': {"###", "#..", "##.", "#..", "#.."},
	'G': {".##", "#..", "#.#", "#.#", ".##"},
	'H': {"#.#", "#.#", "###", "#.#", "#.#"},
	'L': {"#..", "#..", "#..", "#..", "###"},
	'O': {".#.", "#.#", "#.#", "#.#", ".#."},
	'P': {"##.", "#.#", "##.", "#..", "#.."},
	'S': {".##", "#..", ".#.", "..#", "##."},
	'U': {"#.#", "#.#", "#.#", "#.#", "###"},
	'W': {"#.#", "#.#", "#.#", "###", "#.#"},
	'-': {"...", "...", "###", "...", "..."},
	'%': {"#.#", "..#", ".#.", "#..", "#.#"},
	'°': {"##.", "##.", "...", "...", "..."},
	'.': {"...", "...", "...", "...", ".#."},
	' ': {"...", "...", "...", "...", "..."},
}

// glyphWidth is the advance of r in pixels at scale 1 ('.' and '°' are narrow).
func glyphWidth(r rune) int {
	switch r {
	case '.':
		return 2
	case '°':
		return 3
	case ' ':
		return 2
	}
	return 4
}

func textWidth(s string, scale int) int {
	w := 0
	for _, r := range s {
		w += glyphWidth(r) * scale
	}
	if w > 0 {
		w -= scale // no gap after the last glyph
	}
	return w
}

// text draws s with its top left at x, y and returns the x after it.
func (c canvas) text(x, y int, s string, col color.RGBA, scale int) int {
	for _, r := range strings.ToUpper(s) {
		g, ok := font3x5[r]
		if !ok {
			g = font3x5['-']
		}
		off := 0
		if r == '.' {
			off = -1 // the dot sits in the middle column
		}
		for j, row := range g {
			for i := 0; i < 3; i++ {
				if row[i] == '#' {
					c.fill(x+(i+off)*scale, y+j*scale, scale, scale, col)
				}
			}
		}
		x += glyphWidth(r) * scale
	}
	return x
}

// ring paints the pixels between radius r0 and r1 around cx, cy.
func (c canvas) ring(cx, cy, r0, r1 float64, col color.RGBA) {
	for y := 0; y < c.Bounds().Dy(); y++ {
		for x := 0; x < c.Bounds().Dx(); x++ {
			if d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy); d >= r0 && d <= r1 {
				c.px(x, y, col)
			}
		}
	}
}

// line draws from x0,y0 to x1,y1 (Bresenham), coloured by x.
func (c canvas) line(x0, y0, x1, y1 int, col func(x int) color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.px(x0, y0, col(x0))
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// IconIDs lists the icons CaseHUD uses; the app id doubles as icon id.
var IconIDs = []string{"cpu", "gpu", "load", "coolant", "fps", "power"}

// Icon draws icon id at size 8 or 16.
func Icon(id string, size int) image.Image {
	if size >= 16 {
		return icon16(id)
	}
	return icon8(id)
}

func icon8(id string) image.Image {
	c := newCanvas(8, 8)
	switch id {
	case "cpu":
		c.grid(0, 0, []string{
			"..p..p..",
			".bbbbbb.",
			"pbddddbp",
			".bdLLdb.",
			".bdLLdb.",
			"pbddddbp",
			".bbbbbb.",
			"..p..p..",
		}, map[byte]color.RGBA{'p': cGrey, 'b': cCyan, 'd': cCyanD, 'L': cWhite})
	case "gpu":
		c.grid(0, 0, []string{
			"........",
			"gggggggg",
			"gdffdddg",
			"gfwwfdkg",
			"gfwwfdkg",
			"gdffdddg",
			"gggggggg",
			"..yyyy..",
		}, map[byte]color.RGBA{'g': cGreen, 'f': cGreen, 'w': cWhite, 'd': cGreenD, 'k': cGreen, 'y': cGold})
	case "load":
		c.grid(0, 0, []string{
			"......rr",
			"......rr",
			"....aarr",
			"....aarr",
			"..ggaarr",
			"..ggaarr",
			"ccggaarr",
			"ccggaarr",
		}, map[byte]color.RGBA{'c': cCyan, 'g': cGreen, 'a': cAmber, 'r': cRed})
	case "coolant":
		c.grid(0, 0, []string{
			"...bb...",
			"...bb...",
			"..bbbb..",
			"..bLbb..",
			".bLbbbb.",
			".bbbbbb.",
			".bbbbbb.",
			"..bbbb..",
		}, map[byte]color.RGBA{'b': cBlue, 'L': cBlueL})
	case "fps":
		c.grid(0, 0, []string{
			"........",
			".vvvvvv.",
			"vvwvvvrv",
			"vwwwvrvr",
			"vvwvvvrv",
			"vvvvvvvv",
			"vv....vv",
			"........",
		}, map[byte]color.RGBA{'v': cViolet, 'w': cWhite, 'r': cRed})
	default: // "power" and the CaseHUD logo
		c.grid(0, 0, []string{
			"..cbbv..",
			".c....v.",
			"c......v",
			"c..w...t",
			"c.w.w.wt",
			"cw...w.t",
			".c....t.",
			"........",
		}, map[byte]color.RGBA{'c': cCyan, 'b': lerp(cCyan, cViolet, 0.5), 'v': cViolet, 'w': cWhite, 't': cTrack})
	}
	return c.RGBA
}

func icon16(id string) image.Image {
	c := newCanvas(16, 16)
	label := map[string]string{"cpu": "CPU", "gpu": "GPU", "load": "LOAD", "coolant": "H2O", "fps": "FPS"}[id]
	labelCol := cWhite
	switch id {
	case "cpu":
		c.fill(4, 1, 8, 8, cCyan)
		c.fill(5, 2, 6, 6, cCyanD)
		c.fill(6, 3, 4, 4, cWhite)
		for _, p := range []int{5, 7, 8, 10} { // pins on all four sides
			c.px(p, 0, cGrey)
			c.px(p, 9, cGrey)
			c.px(3, p-3, cGrey)
			c.px(12, p-3, cGrey)
		}
		labelCol = cCyan
	case "gpu":
		c.fill(0, 0, 1, 10, cGrey) // slot bracket
		c.rect(1, 1, 15, 7, cGreen)
		c.fill(2, 2, 13, 5, cGreenD)
		for _, fx := range []float64{5.5, 11.5} { // two fans with a hub
			c.ring(fx, 4.5, 1.1, 2.3, cGreen)
			c.px(int(fx), 4, cWhite)
		}
		c.fill(4, 8, 8, 2, cGold) // PCIe fingers
		labelCol = cGreen
	case "load":
		bars := []struct {
			h   int
			col color.RGBA
		}{{3, cCyan}, {5, cGreen}, {7, cAmber}, {10, cRed}}
		for i, b := range bars {
			c.fill(1+i*4, 10-b.h, 3, b.h, b.col)
		}
	case "coolant":
		c.grid(4, 0, []string{
			"...bb...",
			"...bb...",
			"..bbbb..",
			"..bbbb..",
			".bLbbbb.",
			".bLbbbb.",
			"bLbbbbbb",
			"bbbbbbbb",
			".bbbbbb.",
			"..bbbb..",
		}, map[byte]color.RGBA{'b': cBlue, 'L': cBlueL})
		labelCol = cBlueL
	case "fps":
		c.grid(1, 2, []string{
			".vvvvvvvvvvvv.",
			"vvvwvvvvvvvrvv",
			"vvwwwvvvvvrvrv",
			"vvvwvvvvvvvrvv",
			"vvvvvvvvvvvvvv",
			"vvvv......vvvv",
			".vv........vv.",
		}, map[byte]color.RGBA{'v': cViolet, 'w': cWhite, 'r': cRed})
		labelCol = cViolet
	default:
		logo16(c)
		return c.RGBA
	}
	c.text((16-textWidth(label, 1)+1)/2, 11, label, labelCol, 1)
	return c.RGBA
}

// logo16 draws the CaseHUD logo: a 270° gauge, cyan to violet, around a
// pulse line.
func logo16(c canvas) {
	const cx, cy, r = 8.0, 8.0, 6.6
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			if d < r-1.4 || d > r+0.6 {
				continue
			}
			// Angle clockwise from the gauge start (bottom left).
			a := math.Mod(math.Atan2(dy, dx)*180/math.Pi-135+720, 360)
			switch {
			case a > 270:
				continue // the open bottom
			case a <= 190:
				t := a / 190
				c.px(x, y, lerp(cCyan, cViolet, t))
			default:
				c.px(x, y, cTrack)
			}
		}
	}
	// The pulse line from the logo, scaled to the matrix.
	pts := [][2]int{{4, 10}, {6, 10}, {7, 7}, {8, 10}, {9, 6}, {10, 9}, {12, 9}}
	for i := 1; i < len(pts); i++ {
		c.line(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], func(x int) color.RGBA {
			return lerp(cCyan, cViolet, float64(x-4)/8)
		})
	}
	c.px(12, 9, cWhite)
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// IconGIF encodes an icon as a GIF, the lossless format both firmwares
// accept for icons.
func IconGIF(id string, size int) []byte {
	src := Icon(id, size)
	p := image.NewPaletted(src.Bounds(), palette.Plan9)
	// Exact colours: build the palette from the icon itself.
	seen := map[color.RGBA]bool{}
	var pal color.Palette
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			col := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)
			if !seen[col] {
				seen[col] = true
				pal = append(pal, col)
			}
		}
	}
	p.Palette = pal
	draw.Draw(p, p.Bounds(), src, image.Point{}, draw.Src)
	var b bytes.Buffer
	gif.Encode(&b, p, nil)
	return b.Bytes()
}

// Preview draws roughly what a clock of w×h pixels shows for card, as LEDs
// of scale pixels.
func Preview(card Card, w, h, scale int) []byte {
	c := newCanvas(w, h)
	big := h >= 16
	iconSize, fs := 8, 1
	if big {
		iconSize, fs = 16, 2
	}
	x0 := 0
	if card.Icon != "" {
		draw.Draw(c.RGBA, image.Rect(0, (h-iconSize)/2, iconSize, (h-iconSize)/2+iconSize), Icon(card.Icon, iconSize), image.Point{}, draw.Src)
		x0 = iconSize + 1
	}
	frags := card.Text
	if card.Icon == "" && card.Label != "" {
		frags = append([]Fragment{{T: card.Label + " ", C: "FFFFFF"}}, frags...)
	}
	s := ""
	for _, f := range frags {
		s += f.T
	}
	tw := textWidth(s, fs)
	x := x0 + (w-x0-tw)/2
	if x < x0 {
		x = x0
	}
	y := (h - 5*fs) / 2
	if card.Progress >= 0 && !big {
		y = 1
	}
	for _, f := range frags {
		x = c.text(x, y, f.T, hexColour(f.C), fs)
	}
	if card.Progress >= 0 {
		bx := x0 - 1
		if bx < 0 {
			bx = 0
		}
		c.fill(bx, h-1, w-bx, 1, hexColour(card.ProgressBC))
		c.fill(bx, h-1, (w-bx)*card.Progress/100, 1, hexColour(card.ProgressC))
	}
	return leds(c.RGBA, scale)
}

// leds scales img up to round-ish LEDs with dark gaps, like the real matrix.
func leds(img *image.RGBA, scale int) []byte {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	draw.Draw(out, out.Bounds(), image.NewUniform(rgb(8, 9, 12)), image.Point{}, draw.Src)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			col := img.RGBAAt(x, y)
			if col == cBlack {
				col = rgb(22, 24, 30) // an unlit LED is still visible
			}
			for j := 0; j < scale-1; j++ {
				for i := 0; i < scale-1; i++ {
					corner := scale > 3 && (i == 0 || i == scale-2) && (j == 0 || j == scale-2)
					if !corner {
						out.SetRGBA(x*scale+i, y*scale+j, col)
					}
				}
			}
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, out)
	return buf.Bytes()
}

func hexColour(s string) color.RGBA {
	s = strings.TrimPrefix(s, "#")
	var v [3]uint8
	if len(s) != 6 {
		return cGrey
	}
	for i := 0; i < 3; i++ {
		var n uint8
		for _, ch := range s[i*2 : i*2+2] {
			n <<= 4
			switch {
			case ch >= '0' && ch <= '9':
				n |= uint8(ch - '0')
			case ch >= 'a' && ch <= 'f':
				n |= uint8(ch - 'a' + 10)
			case ch >= 'A' && ch <= 'F':
				n |= uint8(ch - 'A' + 10)
			}
		}
		v[i] = n
	}
	return rgb(v[0], v[1], v[2])
}
