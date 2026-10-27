package lcd

import (
	"image"
	"image/color"
	"math"
	"time"
)

// Background effects animate subtly behind a design. They are tinted with
// the design's accent colour and kept dim so values stay readable; on the
// LCDs they also hide the backlight glow that makes plain black look grey.
//
// Effects are cheap enough for 20-30 fps: per-pixel angle and radius are
// computed once, and each frame only rebuilds small lookup tables.

// BackgroundEffects lists the built-in effects in GUI order.
var BackgroundEffects = []string{"spin", "breathe", "radar", "orbit", "aurora", "grid", "rain", "crt"}

// SetBackgrounds suggests an effect per design set.
var SetBackgrounds = map[string]string{"clean": "aurora", "neon": "grid", "retro": "crt", "cyberpunk": "rain"}

// Background renders one effect at a fixed size.
type Background struct {
	effect   string
	w, h     int
	bg, tint color.RGBA
	strength float64 // 0..1, peak share of the tint

	ang []uint16 // angle per pixel, 0..65535 = 0..360° (clockwise from 3 o'clock)
	rad []uint16 // distance from the centre, 0..65535 = 0..half diagonal
	pix []uint8  // reused output buffer
}

const lutBits = 10 // angle lookup resolution (1024 steps)

// NewBackground prepares effect at w x h with the design's background and
// accent colours. strength 0..1 scales how visible it is (0.35 is subtle).
func NewBackground(effect string, w, h int, bg, tint string, strength float64) *Background {
	b := &Background{effect: effect, w: w, h: h, strength: math.Max(0.05, math.Min(1, strength)),
		bg:   parseHex(bg, color.RGBA{0, 0, 0, 255}),
		tint: parseHex(tint, color.RGBA{34, 211, 238, 255})}
	b.ang = make([]uint16, w*h)
	b.rad = make([]uint16, w*h)
	cx, cy := float64(w)/2, float64(h)/2
	maxR := math.Hypot(cx, cy)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			a := math.Atan2(dy, dx)
			if a < 0 {
				a += 2 * math.Pi
			}
			b.ang[y*w+x] = uint16(a / (2 * math.Pi) * 65535)
			b.rad[y*w+x] = uint16(math.Min(1, math.Hypot(dx, dy)/maxR) * 65535)
		}
	}
	return b
}

// Frame draws the effect at time t into a new image.
func (b *Background) Frame(t time.Duration) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, b.w, b.h))
	s := t.Seconds()
	var angLUT [1 << lutBits]float64
	var radLUT [256]float64
	for i := range radLUT {
		radLUT[i] = 1
	}
	angOnly := false
	switch b.effect {
	case "spin":
		// Soft blades turning slowly, like the fan behind the screen.
		phase := s / 12 * 2 * math.Pi
		for i := range angLUT {
			a := float64(i) / float64(len(angLUT)) * 2 * math.Pi
			v := (math.Sin(5*(a-phase)) + 1) / 2
			angLUT[i] = v * v * v
		}
		for i := range radLUT {
			r := float64(i) / 255
			radLUT[i] = smoothstep(0.08, 0.35, r) * (1 - smoothstep(0.55, 0.75, r))
		}
	case "radar":
		// A sweep with a fading trail.
		head := math.Mod(s/5, 1)
		for i := range angLUT {
			d := math.Mod(head-float64(i)/float64(len(angLUT))+1, 1) // 0 at the sweep line
			angLUT[i] = math.Exp(-d * 9)
		}
		for i := range radLUT {
			r := float64(i) / 255
			radLUT[i] = 1 - smoothstep(0.62, 0.72, r)
			if ring := math.Abs(math.Mod(r, 0.18) - 0.09); ring > 0.083 {
				radLUT[i] = math.Max(radLUT[i]*0.35, 0.35*(1-smoothstep(0.62, 0.72, r)))
			}
		}
	case "orbit":
		// Dots on two rings travelling in opposite directions.
		for i := range angLUT {
			angLUT[i] = 1
		}
		angOnly = true
	case "breathe":
		// A glow from the centre that slowly brightens and fades.
		k := 0.55 + 0.45*(math.Sin(s/4.5*2*math.Pi)+1)/2
		for i := range radLUT {
			r := float64(i) / 255
			radLUT[i] = k * math.Exp(-r*r*7)
		}
		for i := range angLUT {
			angLUT[i] = 1
		}
	case "aurora":
		return b.aurora(img, s)
	case "grid":
		return b.grid(img, s)
	case "rain":
		return b.rain(img, s)
	case "crt":
		return b.crt(img, s)
	default:
		b.fill(img, func(int) uint8 { return 0 })
		return img
	}
	if angOnly {
		return b.orbit(img, s)
	}
	var ang [1 << lutBits]uint16
	var rad [256]uint16
	for i, v := range angLUT {
		ang[i] = uint16(math.Max(0, math.Min(1, v)) * 65535)
	}
	for i, v := range radLUT {
		rad[i] = uint16(math.Max(0, math.Min(1, v)) * 65535)
	}
	b.fillLUT(img, &ang, &rad)
	return img
}

// mixLUT maps an intensity 0..255 to the bg/tint mix at this strength.
func (b *Background) mixLUT() (lut [256][3]uint8) {
	for i := range lut {
		k := float64(i) / 255 * b.strength
		lut[i] = [3]uint8{mix8(b.bg.R, b.tint.R, k), mix8(b.bg.G, b.tint.G, k), mix8(b.bg.B, b.tint.B, k)}
	}
	return lut
}

// fill writes every pixel from an intensity 0..255.
func (b *Background) fill(img *image.RGBA, v func(i int) uint8) {
	lut := b.mixLUT()
	p := img.Pix
	for i, o := 0, 0; i < b.w*b.h; i, o = i+1, o+4 {
		c := &lut[v(i)]
		p[o], p[o+1], p[o+2], p[o+3] = c[0], c[1], c[2], 0xff
	}
}

// fillLUT is fill for effects that are a product of an angle and a radius
// table, the hot path for most effects: integer maths only.
func (b *Background) fillLUT(img *image.RGBA, ang *[1 << lutBits]uint16, rad *[256]uint16) {
	lut := b.mixLUT()
	p := img.Pix
	for i, o := 0, 0; i < b.w*b.h; i, o = i+1, o+4 {
		v := uint32(ang[b.ang[i]>>(16-lutBits)]) * uint32(rad[b.rad[i]>>8]) >> 24 // 16+16 bit -> 8 bit
		c := &lut[v]
		p[o], p[o+1], p[o+2], p[o+3] = c[0], c[1], c[2], 0xff
	}
}

func (b *Background) orbit(img *image.RGBA, s float64) *image.RGBA {
	b.fill(img, func(int) uint8 { return 0 })
	cx, cy := float64(b.w)/2, float64(b.h)/2
	size := math.Min(float64(b.w), float64(b.h))
	for ring, spec := range []struct {
		r, speed float64
		n        int
	}{{0.47, 1.0 / 30, 36}, {0.40, -1.0 / 45, 28}} {
		for i := 0; i < spec.n; i++ {
			a := (float64(i)/float64(spec.n) + s*spec.speed) * 2 * math.Pi
			x, y := cx+spec.r*size*math.Cos(a), cy+spec.r*size*math.Sin(a)
			// Brightness wave running round the ring.
			k := 0.35 + 0.65*math.Pow((math.Sin(a*2-s*1.3+float64(ring))+1)/2, 3)
			b.dot(img, x, y, size*0.008, k)
		}
	}
	return img
}

func (b *Background) dot(img *image.RGBA, x, y, r, k float64) {
	x0, y0 := int(x-r-1), int(y-r-1)
	for py := y0; py <= int(y+r+1); py++ {
		for px := x0; px <= int(x+r+1); px++ {
			if px < 0 || py < 0 || px >= b.w || py >= b.h {
				continue
			}
			c := math.Max(0, math.Min(1, r+0.5-math.Hypot(float64(px)+0.5-x, float64(py)+0.5-y)))
			if c == 0 {
				continue
			}
			o := (py*b.w + px) * 4
			kk := c * k * b.strength * 1.6
			img.Pix[o] = mix8(img.Pix[o], b.tint.R, kk)
			img.Pix[o+1] = mix8(img.Pix[o+1], b.tint.G, kk)
			img.Pix[o+2] = mix8(img.Pix[o+2], b.tint.B, kk)
		}
	}
}

// aurora draws drifting soft blobs at quarter resolution and scales up.
func (b *Background) aurora(img *image.RGBA, s float64) *image.RGBA {
	const q = 4
	sw, sh := (b.w+q-1)/q, (b.h+q-1)/q
	small := make([]float64, sw*sh)
	blobs := [3][4]float64{{0.30, 0.35, 0.33, 17}, {0.68, 0.60, 0.29, 23}, {0.45, 0.75, 0.25, 29}}
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			fx, fy := (float64(x)+0.5)/float64(sw), (float64(y)+0.5)/float64(sh)
			v := 0.0
			for i, bl := range blobs {
				ph := s / bl[3] * 2 * math.Pi
				bx := bl[0] + 0.16*math.Sin(ph+float64(i))
				by := bl[1] + 0.16*math.Cos(ph*0.8+float64(i)*2)
				d := (fx-bx)*(fx-bx) + (fy-by)*(fy-by)
				v += math.Exp(-d / (bl[2] * bl[2] * 0.5))
			}
			small[y*sw+x] = math.Min(1, v*0.6)
		}
	}
	// Bilinear upscale in 8.8 fixed point.
	sm := make([]uint32, sw*sh)
	for i, v := range small {
		sm[i] = uint32(v * 255)
	}
	lut := b.mixLUT()
	p := img.Pix
	for y := 0; y < b.h; y++ {
		gy := max(0, (y*2+1)*128/q-128) // (y+0.5)/q-0.5 in 1/256 units
		y0 := min(gy>>8, sh-1)
		y1 := min(y0+1, sh-1)
		ty := uint32(gy & 0xff)
		for x := 0; x < b.w; x++ {
			gx := max(0, (x*2+1)*128/q-128)
			x0 := min(gx>>8, sw-1)
			x1 := min(x0+1, sw-1)
			tx := uint32(gx & 0xff)
			top := sm[y0*sw+x0]*(256-tx) + sm[y0*sw+x1]*tx
			bot := sm[y1*sw+x0]*(256-tx) + sm[y1*sw+x1]*tx
			v := (top*(256-ty) + bot*ty) >> 16
			c := &lut[min(v, 255)]
			o := (y*b.w + x) * 4
			p[o], p[o+1], p[o+2], p[o+3] = c[0], c[1], c[2], 0xff
		}
	}
	return img
}

func mix8(a, b uint8, k float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*math.Max(0, math.Min(1, k)) + 0.5)
}

func smoothstep(e0, e1, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-e0)/(e1-e0)))
	return t * t * (3 - 2*t)
}

// grid is a synthwave floor: perspective lines rushing towards the viewer
// below a glowing horizon.
func (b *Background) grid(img *image.RGBA, s float64) *image.RGBA {
	lut := b.mixLUT()
	horizon := float64(b.h) * 0.68 // low, so the floor stays under the numbers
	cx := float64(b.w) / 2
	scroll := math.Mod(s*0.6, 1)
	p := img.Pix
	for y := 0; y < b.h; y++ {
		fy := float64(y) + 0.5
		rowV := 0.0
		var z float64
		if fy > horizon {
			z = float64(b.h) * 0.35 / (fy - horizon) // depth of this row
			// Horizontal lines: thin where near, fading with distance.
			f := math.Mod(z+scroll, 1)
			d := math.Min(f, 1-f) / (0.04 * z)
			fade := math.Max(0, math.Min(1, 1.8/z)-0.12)
			rowV = math.Max(0, 1-d) * fade * fade
			if fy-horizon < 1.5 {
				rowV = 0.8 // the horizon line
			}
		} else {
			// Sky: glow just above the horizon.
			rowV = 0.5 * math.Exp(-(horizon-fy)/(float64(b.h)*0.08))
		}
		for x := 0; x < b.w; x++ {
			v := rowV
			if fy > horizon {
				// Vertical lines converge in the vanishing point.
				gx := (float64(x) + 0.5 - cx) * z / float64(b.w) * 9
				f := gx - math.Floor(gx)
				d := math.Min(f, 1-f) / (0.03 * z)
				fade := math.Max(0, math.Min(1, 1.8/z)-0.12)
				v = math.Max(v, math.Max(0, 1-d)*fade*fade)
			}
			c := &lut[uint8(math.Min(1, v)*255)]
			o := (y*b.w + x) * 4
			p[o], p[o+1], p[o+2], p[o+3] = c[0], c[1], c[2], 0xff
		}
	}
	return img
}

// rain is digital rain: columns of glyph cells falling with fading trails.
func (b *Background) rain(img *image.RGBA, s float64) *image.RGBA {
	b.fill(img, func(int) uint8 { return 0 })
	lut := b.mixLUT()
	cell := max(8, b.w/26)
	cols, rows := b.w/cell+1, b.h/cell+1
	for c := 0; c < cols; c++ {
		h := hash32(uint32(c)*2654435761 + 7)
		speed := 4 + float64(h%7)                  // cells per second
		offset := float64(h >> 8 % uint32(rows*3)) // start position
		trail := 6 + float64(h>>16%10)
		head := math.Mod(s*speed+offset, float64(rows)+trail)
		for r := 0; r < rows; r++ {
			d := head - float64(r)
			if d < 0 || d > trail {
				continue
			}
			v := math.Pow(1-d/trail, 1.6)
			if d < 1 {
				v = 1 // bright head
			}
			// Keep the middle, where the numbers are, calm.
			mx, my := float64(c*cell)/float64(b.w)-0.5, float64(r*cell)/float64(b.h)-0.5
			v *= 0.25 + 0.75*smoothstep(0.22, 0.42, math.Hypot(mx, my))
			// Glyphs change now and then.
			g := hash32(uint32(c*131+r*977) + uint32(s*3)*7919)
			b.glyph(img, c*cell, r*cell, cell, g, &lut[uint8(v*255)])
		}
	}
	return img
}

// glyph draws a pseudo character: a few lit dots of a 3x5 grid.
func (b *Background) glyph(img *image.RGBA, x0, y0, cell int, bits uint32, c *[3]uint8) {
	dot := max(1, cell/5)
	for gy := 0; gy < 5; gy++ {
		for gx := 0; gx < 3; gx++ {
			if bits>>(gy*3+gx)&1 == 0 {
				continue
			}
			px, py := x0+1+gx*(dot+cell/12), y0+gy*dot
			for yy := py; yy < py+dot && yy < b.h; yy++ {
				for xx := px; xx < px+dot && xx < b.w; xx++ {
					o := (yy*b.w + xx) * 4
					img.Pix[o], img.Pix[o+1], img.Pix[o+2] = c[0], c[1], c[2]
				}
			}
		}
	}
}

// crt is an old monitor: a soft bright band rolling down, faint noise and
// a dark vignette.
func (b *Background) crt(img *image.RGBA, s float64) *image.RGBA {
	lut := b.mixLUT()
	band := math.Mod(s/6, 1.3) - 0.15
	frame := uint32(s * 24)
	p := img.Pix
	for y := 0; y < b.h; y++ {
		fy := (float64(y) + 0.5) / float64(b.h)
		d := (fy - band) / 0.09
		row := 0.10 + 0.55*math.Exp(-d*d)
		for x := 0; x < b.w; x++ {
			i := y*b.w + x
			r := float64(b.rad[i]) / 65535
			centre := 0.3 + 0.7*smoothstep(0.25, 0.45, r) // the band stays dim behind the numbers
			v := (0.10 + (row-0.10)*centre) * (1 - smoothstep(0.45, 0.72, r))
			n := float64(hash32(uint32(i)*31+frame*2246822519)&0xff) / 255
			v = v*0.85 + n*0.12*v + n*0.03
			c := &lut[uint8(math.Min(1, v)*255)]
			o := i * 4
			p[o], p[o+1], p[o+2], p[o+3] = c[0], c[1], c[2], 0xff
		}
	}
	return img
}

func hash32(x uint32) uint32 {
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return x
}

// Bloom softens an image and lets bright parts glow: the background looks
// washed out and dreamy, which also keeps busy GIFs from fighting the HUD.
// Blurring happens at quarter resolution; the rest is integer maths.
func Bloom(img *image.RGBA) {
	const q = 4
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	sw, sh := (w+q-1)/q, (h+q-1)/q
	small := make([]float32, sw*sh*3)
	sums := make([]uint32, sw*3)
	for sy := 0; sy < sh; sy++ {
		clear(sums)
		rows := 0
		for y := sy * q; y < min(h, sy*q+q); y++ {
			rows++
			row := img.Pix[y*img.Stride:]
			for x := 0; x < w; x++ {
				o, i := x*4, (x/q)*3
				sums[i] += uint32(row[o])
				sums[i+1] += uint32(row[o+1])
				sums[i+2] += uint32(row[o+2])
			}
		}
		for sx := 0; sx < sw; sx++ {
			n := float32(rows * (min(w, sx*q+q) - sx*q))
			i := (sy*sw + sx) * 3
			small[i], small[i+1], small[i+2] = float32(sums[sx*3])/n, float32(sums[sx*3+1])/n, float32(sums[sx*3+2])/n
		}
	}
	for pass := 0; pass < 3; pass++ {
		boxBlur(small, sw, sh, 3)
	}
	glow := make([]uint32, len(small))
	for i, v := range small {
		glow[i] = uint32(min(255, v*1.6)) // brighter than the source: the glow
	}
	// Bilinear upscale in 8.8 fixed point, mixed 40/60 with the original.
	cx0, cx1, ctx := make([]int, w), make([]int, w), make([]uint32, w)
	for x := range cx0 {
		gx := max(0, (x*2+1)*128/q-128)
		cx0[x] = min(gx>>8, sw-1) * 3
		cx1[x] = min(gx>>8+1, sw-1) * 3
		ctx[x] = uint32(gx & 0xff)
	}
	p := img.Pix
	for y := 0; y < h; y++ {
		gy := max(0, (y*2+1)*128/q-128)
		r0 := glow[min(gy>>8, sh-1)*sw*3:]
		r1 := glow[min(gy>>8+1, sh-1)*sw*3:]
		ty := uint32(gy & 0xff)
		row := p[y*img.Stride:]
		for x := 0; x < w; x++ {
			a, b, tx := cx0[x], cx1[x], ctx[x]
			o := x * 4
			for k := 0; k < 3; k++ {
				top := r0[a+k]*(256-tx) + r0[b+k]*tx
				bot := r1[a+k]*(256-tx) + r1[b+k]*tx
				v := ((top*(256-ty)+bot*ty)>>16*180 + uint32(row[o+k])*90) >> 8
				row[o+k] = uint8(min(255, v))
			}
		}
	}
}

// boxBlur blurs an RGB float image in place with radius r (horizontal then
// vertical).
func boxBlur(px []float32, w, h, r int) {
	tmp := make([]float32, len(px))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s [3]float32
			n := float32(0)
			for k := max(0, x-r); k <= min(w-1, x+r); k++ {
				i := (y*w + k) * 3
				s[0], s[1], s[2], n = s[0]+px[i], s[1]+px[i+1], s[2]+px[i+2], n+1
			}
			i := (y*w + x) * 3
			tmp[i], tmp[i+1], tmp[i+2] = s[0]/n, s[1]/n, s[2]/n
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s [3]float32
			n := float32(0)
			for k := max(0, y-r); k <= min(h-1, y+r); k++ {
				i := (k*w + x) * 3
				s[0], s[1], s[2], n = s[0]+tmp[i], s[1]+tmp[i+1], s[2]+tmp[i+2], n+1
			}
			i := (y*w + x) * 3
			px[i], px[i+1], px[i+2] = s[0]/n, s[1]/n, s[2]/n
		}
	}
}
