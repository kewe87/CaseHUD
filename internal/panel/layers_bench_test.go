package panel

import (
	"image"
	"testing"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/winusb"
)

// discardUSB takes every write and answers every read, without recording.
type discardUSB struct{}

func (discardUSB) Write(_ byte, b []byte, _ int) (int, error) { return len(b), nil }
func (discardUSB) Read(_ byte, b []byte, _ int) (int, error) {
	for i := range b[:min(len(b), 512)] {
		b[i] = 0
	}
	return min(len(b), 512), nil
}
func (discardUSB) ClearHalt(byte) error           { return nil }
func (discardUSB) ZeroLengthTerminate(byte) error { return nil }
func (discardUSB) Close() error                   { return nil }

var _ winusb.Device = discardUSB{}

// benchHUD renders the dashboard of a 1920x480 bar (four tiles with a ring and
// a number) n times with changing readings, transparent or opaque.
func benchHUD(b testing.TB, n int, transparent bool) []*image.RGBA {
	b.Helper()
	l := lcd.DefaultLayout()
	l.Width, l.Height = 1920, 480
	l.Widgets = nil
	for i, src := range []string{"Load/CPU Total", "Load/GPU Core|Load/D3D 3D", "Temperatures/CPU Package", "Powers/GPU Package|Powers/GPU Power"} {
		x := 12 + i*480
		l.Widgets = append(l.Widgets,
			lcd.Widget{Type: "gauge", X: x, Y: 12, W: 456, H: 456, Source: src, Max: 100, Warn: 85, Crit: 97, Color: "#22d3ee", RingOnly: true},
			lcd.Widget{Type: "value", Align: "center", X: x + 80, Y: 120, W: 300, H: 240, Label: "SRC", Source: src, Unit: "%", Color: "#22d3ee"})
	}
	r, err := lcd.NewRenderer(l)
	if err != nil {
		b.Fatal(err)
	}
	r.Transparent = transparent
	prov := metrics.NewMock()
	var out []*image.RGBA
	for i := 0; i < n; i++ {
		prov.Refresh()
		out = append(out, r.Render(prov))
	}
	return out
}

func benchBar(b *testing.B) *universal88 {
	b.Helper()
	p := &universal88{dev: discardUSB{}, m: &usModel88, id: "us88-0", name: "bar", brightness: 100}
	b.Cleanup(func() { p.Close() })
	usLayerGap, usLayerKeepalive = 0, 0
	b.Cleanup(func() { usLayerGap, usLayerKeepalive = 200000000, 1000000000 })
	return p
}

// One display update through the picture path: rotate, JPEG, write.
func BenchmarkBarUpdateJPEG(b *testing.B) {
	p := benchBar(b)
	frames := benchHUD(b, 8, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := p.ShowImage(frames[i%len(frames)]); err != nil {
			b.Fatal(err)
		}
	}
}

// One display update through the layer mode with a changed overlay: rotate,
// convert, PNG, write (the picture was sent before).
func BenchmarkBarUpdateOverlayChanged(b *testing.B) {
	p := benchBar(b)
	frames := benchHUD(b, 8, true)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, 1920, 480)) }}
	if err := p.ShowLayers(bg, frames[0]); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := p.ShowLayers(bg, frames[(i+1)%len(frames)]); err != nil {
			b.Fatal(err)
		}
	}
}

// The keep-alive of an unchanged overlay: the cached PNG goes out again.
func BenchmarkBarKeepaliveOverlay(b *testing.B) {
	p := benchBar(b)
	frames := benchHUD(b, 1, true)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, 1920, 480)) }}
	p.ShowLayers(bg, frames[0])
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := p.ShowLayers(bg, frames[0]); err != nil {
			b.Fatal(err)
		}
	}
}

// An unchanged overlay inside the keep-alive time: nothing is sent, the
// pixels are only hashed.
func BenchmarkBarUnchangedOverlay(b *testing.B) {
	p := benchBar(b)
	frames := benchHUD(b, 1, true)
	bg := Backdrop{Key: 1, Image: func() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, 1920, 480)) }}
	p.ShowLayers(bg, frames[0])
	usLayerKeepalive = 1 << 40
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ShowLayers(bg, frames[0])
	}
}
