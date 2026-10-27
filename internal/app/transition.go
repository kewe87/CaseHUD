package app

import (
	"image"
	"image/draw"
	"math"
	"time"
)

// Transition effects between two contents of a panel.
var Transitions = []string{"fade", "slide-down", "slide-up", "slide-left", "slide-right", "flip", "zoom"}

// transitionDuration is how long a change animates.
const transitionDuration = 600 * time.Millisecond

func validTransition(effect string) bool {
	for _, t := range Transitions {
		if t == effect {
			return true
		}
	}
	return effect == "" || effect == "none"
}

// easeInOut is a cubic ease: slow start, fast middle, soft landing.
func easeInOut(p float64) float64 {
	if p < 0.5 {
		return 4 * p * p * p
	}
	return 1 - math.Pow(-2*p+2, 3)/2
}

// blend draws the state of a transition from old to next at progress p
// (0..1, already eased). Both images have the same size.
func blend(effect string, p float64, old, next *image.RGBA) *image.RGBA {
	b := next.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(b)
	switch effect {
	case "slide-down", "slide-up", "slide-left", "slide-right":
		dx, dy := 0, 0
		switch effect {
		case "slide-down":
			dy = int(math.Round(p * float64(h)))
		case "slide-up":
			dy = -int(math.Round(p * float64(h)))
		case "slide-left":
			dx = -int(math.Round(p * float64(w)))
		case "slide-right":
			dx = int(math.Round(p * float64(w)))
		}
		// The old content moves out, the new one follows it in.
		draw.Draw(out, b.Add(image.Pt(dx, dy)), old, b.Min, draw.Src)
		off := image.Pt(dx, dy)
		switch effect {
		case "slide-down":
			off.Y -= h
		case "slide-up":
			off.Y += h
		case "slide-left":
			off.X += w
		case "slide-right":
			off.X -= w
		}
		draw.Draw(out, b.Add(off), next, b.Min, draw.Src)
	case "flip":
		// The old page folds to a line, then the new one unfolds.
		src, k := old, 1-2*p
		if p >= 0.5 {
			src, k = next, 2*p-1
		}
		squeeze(out, src, k)
	case "zoom":
		// The new content grows from the centre over the old one.
		draw.Draw(out, b, old, b.Min, draw.Src)
		k := math.Max(0.01, p)
		tmp := image.NewRGBA(b)
		scaleCentre(tmp, next, k)
		mask := image.NewUniform(colorAlpha(p))
		draw.DrawMask(out, b, tmp, b.Min, mask, image.Point{}, draw.Over)
	default: // fade
		a := uint32(math.Round(p * 255))
		for i := range out.Pix {
			out.Pix[i] = uint8((uint32(old.Pix[i])*(255-a) + uint32(next.Pix[i])*a + 127) / 255)
		}
	}
	return out
}

// squeeze draws src scaled horizontally by k (0..1) around the centre on a
// black background.
func squeeze(dst, src *image.RGBA, k float64) {
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	for i := 3; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 0xff
	}
	half := float64(w) / 2 * k
	cx := float64(w) / 2
	x0, x1 := int(cx-half), int(math.Ceil(cx+half))
	if x1 <= x0 {
		return
	}
	for x := max(0, x0); x < min(w, x1); x++ {
		sx := int((float64(x) + 0.5 - (cx - half)) / (2 * half) * float64(w))
		sx = max(0, min(w-1, sx))
		// Darken towards the fold for a bit of depth.
		shade := 0.45 + 0.55*k
		for y := 0; y < h; y++ {
			so, do := y*src.Stride+sx*4, y*dst.Stride+x*4
			dst.Pix[do] = uint8(float64(src.Pix[so]) * shade)
			dst.Pix[do+1] = uint8(float64(src.Pix[so+1]) * shade)
			dst.Pix[do+2] = uint8(float64(src.Pix[so+2]) * shade)
			dst.Pix[do+3] = 0xff
		}
	}
}

// scaleCentre draws src scaled by k around the centre (nearest neighbour,
// fine for a 0.6 s effect).
func scaleCentre(dst, src *image.RGBA, k float64) {
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	cx, cy := float64(w)/2, float64(h)/2
	for y := 0; y < h; y++ {
		sy := int(cy + (float64(y)+0.5-cy)/k)
		if sy < 0 || sy >= h {
			continue
		}
		for x := 0; x < w; x++ {
			sx := int(cx + (float64(x)+0.5-cx)/k)
			if sx < 0 || sx >= w {
				continue
			}
			copy(dst.Pix[y*dst.Stride+x*4:y*dst.Stride+x*4+4], src.Pix[sy*src.Stride+sx*4:sy*src.Stride+sx*4+4])
		}
	}
}

type colorAlpha float64

func (a colorAlpha) RGBA() (r, g, b, al uint32) {
	v := uint32(math.Max(0, math.Min(1, float64(a))) * 0xffff)
	return v, v, v, v
}
