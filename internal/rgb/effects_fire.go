package leds

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

func fireEffect() EffectInfo {
	return EffectInfo{
		Name: "fire", Label: "Fire",
		Params: []Param{
			{Key: "speed", Label: "Speed", Type: ParamNumber, Min: 1, Max: 100, Default: 50.0},
			{Key: "height", Label: "Flame height", Type: ParamNumber, Min: 10, Max: 100, Default: 60.0},
			{Key: "color", Label: "Flame color", Type: ParamColor, Default: "#ff3c00"},
			{Key: "tip", Label: "Tip color", Type: ParamColor, Default: "#ffd040"},
		},
		New: func(p Params) (Effect, error) {
			return &fire{speed: p.Float("speed"), height: p.Float("height"), flame: p.Color("color"), tip: p.Color("tip"),
				rnd: rand.New(rand.NewPCG(1, 2))}, nil
		},
	}
}

// fire is a heat field on a coarse grid: the bottom row is fed with random
// heat, every step each cell takes the heat of the cell below (drifting
// sideways) minus a random cooling. Heat maps to black, flame color, tip
// color and white; the grid is stretched over the canvas, flames rise
// upwards (towards y = 0).
type fire struct {
	speed, height float64
	flame, tip    color.RGBA
	rnd           *rand.Rand
	heat          []float64
	gw, gh        int
	acc           float64 // simulation time not yet stepped (seconds)
}

const (
	fireCols = 48
	fireRows = 30
)

func (fi *fire) step() {
	w, h := fi.gw, fi.gh
	for x := 0; x < w; x++ { // fuel
		fi.heat[(h-1)*w+x] = 0.6 + 0.4*fi.rnd.Float64()
	}
	// cooling per row so that flames reach about height% of the grid
	cool := 1.0 / (float64(h) * math.Max(fi.height, 10) / 100)
	for y := 0; y < h-1; y++ {
		for x := 0; x < w; x++ {
			src := x + fi.rnd.IntN(3) - 1
			src = min(max(src, 0), w-1)
			v := fi.heat[(y+1)*w+src] - cool*(0.4+1.2*fi.rnd.Float64())
			fi.heat[y*w+x] = math.Max(v, 0)
		}
	}
}

func (fi *fire) color(v float64) color.RGBA {
	switch {
	case v < 0.35:
		return lerp(color.RGBA{A: 255}, fi.flame, v/0.35)
	case v < 0.75:
		return lerp(fi.flame, fi.tip, (v-0.35)/0.4)
	default:
		return lerp(fi.tip, color.RGBA{255, 255, 255, 255}, (v-0.75)/0.25*0.6)
	}
}

func (fi *fire) Render(f Frame, c *image.RGBA) {
	if fi.heat == nil {
		fi.gw, fi.gh = fireCols, fireRows
		fi.heat = make([]float64, fi.gw*fi.gh)
	}
	rate := 10 + fi.speed*0.5 // steps per second
	fi.acc += f.Dt.Seconds()
	for n := 0; fi.acc*rate >= 1 && n < 8; n++ { // at most 8 steps per frame
		fi.step()
		fi.acc -= 1 / rate
	}
	fi.acc = math.Min(fi.acc, 1/rate)
	w, h := c.Rect.Dx(), c.Rect.Dy()
	for y := 0; y < h; y++ {
		gy := y * fi.gh / h
		for x := 0; x < w; x++ {
			set(c, x, y, fi.color(fi.heat[gy*fi.gw+x*fi.gw/w]))
		}
	}
}
