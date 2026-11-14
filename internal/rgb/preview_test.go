package leds

import (
	"image"
	"image/color"
	"testing"
	"time"
)

func TestDownscaleAveragesAndDims(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			c := color.RGBA{A: 255}
			if x%2 == 0 {
				c.R = 200
			} else {
				c.B = 100
			}
			src.SetRGBA(x, y, c)
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 2, 1))
	Downscale(src, dst, 256)
	if got := dst.RGBAAt(0, 0); got != (color.RGBA{100, 0, 50, 255}) {
		t.Errorf("full brightness %v", got)
	}
	Downscale(src, dst, 128)
	if got := dst.RGBAAt(1, 0); got != (color.RGBA{50, 0, 25, 255}) {
		t.Errorf("half brightness %v", got)
	}
}

// The engine only prepares previews while they are asked for.
func TestCanvasTapOnlyWhenRequested(t *testing.T) {
	var e Engine
	canvas := image.NewRGBA(image.Rect(0, 0, CanvasWidth, CanvasHeight))
	for i := range canvas.Pix {
		canvas.Pix[i] = 255
	}
	now := time.Now()
	e.tap.take(canvas, 256, now)
	if e.tap.img != nil {
		t.Fatal("preview taken without a request")
	}
	if img, _ := e.CanvasPreview(); img != nil {
		t.Fatal("first request returned a frame")
	}
	e.tap.take(canvas, 256, time.Now())
	img, at := e.CanvasPreview()
	if img == nil || at.IsZero() || img.Bounds() != canvas.Bounds() || img.RGBAAt(5, 5).R != 255 {
		t.Fatalf("preview %v at %v", img, at)
	}
	// after the requests stop, no more frames are prepared
	e.tap.mu.Lock()
	e.tap.wanted = time.Now().Add(-previewKeep - time.Second)
	e.tap.mu.Unlock()
	clear(canvas.Pix)
	e.tap.take(canvas, 256, time.Now().Add(time.Second))
	if e.tap.img.RGBAAt(5, 5).R != 255 {
		t.Error("frame prepared without a request")
	}
}

type countEffect struct {
	builds *int
	closed *int
}

func (c countEffect) Render(f Frame, canvas *image.RGBA) {
	for i := 0; i < len(canvas.Pix); i += 4 {
		canvas.Pix[i] = uint8(f.N * 10)
		canvas.Pix[i+3] = 255
	}
}

func (c countEffect) Close() error { *c.closed++; return nil }

func TestPreviewerRebuildsOnChange(t *testing.T) {
	builds, closed := 0, 0
	info := EffectInfo{Name: "count", New: func(Params) (Effect, error) {
		builds++
		return countEffect{&builds, &closed}, nil
	}}
	var p Previewer
	now := time.Now()
	a, err := p.Frame(info, nil, "a", nil, 256, now)
	if err != nil || a.RGBAAt(0, 0).R != 0 {
		t.Fatalf("first frame %v %v", a.RGBAAt(0, 0), err)
	}
	if b, _ := p.Frame(info, nil, "a", nil, 256, now.Add(10*time.Millisecond)); b.RGBAAt(0, 0).R != 0 {
		t.Error("rendered again within a tenth of a second")
	}
	if b, _ := p.Frame(info, nil, "a", nil, 256, now.Add(200*time.Millisecond)); b.RGBAAt(0, 0).R != 10 || builds != 1 {
		t.Errorf("second frame %v, builds %d", b.RGBAAt(0, 0), builds)
	}
	if _, err := p.Frame(info, nil, "b", nil, 256, now.Add(300*time.Millisecond)); err != nil || builds != 2 || closed != 1 {
		t.Errorf("change: builds %d closed %d %v", builds, closed, err)
	}
	p.Close()
	if closed != 2 {
		t.Errorf("closed %d", closed)
	}
}
