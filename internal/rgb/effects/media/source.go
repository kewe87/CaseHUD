package media

import (
	"image"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Source supplies frames from the host application (its own video decoder,
// renderer or screen capture), so no second decoder has to run.
//
// Frame is called from the engine goroutine once per tick and must not
// block: it copies the newest frame into dst (canvas size, RGBA) and reports
// whether a frame was available. The host scales its image to the canvas.
type Source interface {
	Frame(dst *image.RGBA) bool
}

// SourceEffect returns an effect that shows the frames of a host source.
// open is called when the effect is selected; if the returned source also
// implements io.Closer it is closed when the effect is replaced.
func SourceEffect(name, label string, open func(leds.Params) (Source, error)) leds.EffectInfo {
	return leds.EffectInfo{
		Name: name, Label: label,
		Params: []leds.Param{
			{Key: "saturation", Label: "Saturation (%)", Type: leds.ParamNumber, Min: 0, Max: 300, Default: 110.0},
			{Key: "smoothing", Label: "Smoothing", Type: leds.ParamNumber, Min: 0, Max: 95, Default: 0.0},
		},
		New: func(p leds.Params) (leds.Effect, error) {
			src, err := open(p)
			if err != nil {
				return nil, err
			}
			return &sourceEffect{src: src, frame: newCanvas(),
				saturation: p.Float("saturation") / 100, smoothing: p.Float("smoothing") / 100}, nil
		},
	}
}

type sourceEffect struct {
	src        Source
	frame      *image.RGBA
	saturation float64
	smoothing  float64
}

func (s *sourceEffect) Render(f leds.Frame, c *image.RGBA) {
	s.src.Frame(s.frame) // keeps the previous frame if nothing new arrived
	grade(c, s.frame, s.smoothing, s.saturation)
}

func (s *sourceEffect) Close() error {
	if c, ok := s.src.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
