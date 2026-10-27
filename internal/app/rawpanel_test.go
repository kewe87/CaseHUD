package app

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

// rawFake is a panel that takes unencoded pictures.
type rawFake struct {
	jpegs, images int
}

func (r *rawFake) ID() string                  { return "raw" }
func (r *rawFake) Name() string                { return "raw" }
func (r *rawFake) Size() image.Point           { return image.Pt(64, 32) }
func (r *rawFake) ShowJPEG([]byte) error       { r.jpegs++; return nil }
func (r *rawFake) ShowImage(image.Image) error { r.images++; return nil }
func (r *rawFake) MaxJPEG() int                { return 1 << 20 }
func (r *rawFake) SetBrightness(int) error     { return nil }
func (r *rawFake) SetRotation(int) error       { return nil }
func (r *rawFake) Close() error                { return nil }

func TestWorkerRawPanelEncodesPreviewOnDemand(t *testing.T) {
	p := &rawFake{}
	w := &worker{p: p}
	w.show(image.NewRGBA(image.Rect(0, 0, 64, 32)))
	if p.jpegs != 0 || p.images != 1 {
		t.Fatalf("jpegs=%d images=%d, want 0/1", p.jpegs, p.images)
	}
	if w.last != nil {
		t.Fatal("preview JPEG made before anybody asked")
	}
	b := w.preview()
	if _, err := jpeg.Decode(bytes.NewReader(b)); err != nil {
		t.Fatalf("preview: %v", err)
	}
	b2, seq := w.snapshot()
	if &b2[0] != &b[0] || seq != 1 {
		t.Fatal("preview not cached per frame")
	}
	w.show(image.NewRGBA(image.Rect(0, 0, 64, 32)))
	if _, seq := w.snapshot(); seq != 2 || w.err() != "" {
		t.Fatalf("seq=%d err=%q", seq, w.err())
	}
}

// panelOnlyJPEG is a panel that only takes encoded frames.
type panelOnlyJPEG struct{ n int }

func (p *panelOnlyJPEG) ID() string              { return "j" }
func (p *panelOnlyJPEG) Name() string            { return "j" }
func (p *panelOnlyJPEG) Size() image.Point       { return image.Pt(64, 32) }
func (p *panelOnlyJPEG) ShowJPEG([]byte) error   { p.n++; return nil }
func (p *panelOnlyJPEG) MaxJPEG() int            { return 1 << 20 }
func (p *panelOnlyJPEG) SetBrightness(int) error { return nil }
func (p *panelOnlyJPEG) SetRotation(int) error   { return nil }
func (p *panelOnlyJPEG) Close() error            { return nil }

func TestWorkerCachesStaticFrames(t *testing.T) {
	p := &panelOnlyJPEG{}
	w := &worker{p: p}
	a := image.NewRGBA(image.Rect(0, 0, 64, 32))
	w.show(a)
	first := w.preview()
	if p.n != 1 {
		t.Fatalf("shows = %d", p.n)
	}
	w.show(image.NewRGBA(image.Rect(0, 0, 64, 32))) // same content, new buffer
	if p.n != 2 || &w.preview()[0] != &first[0] {
		t.Fatal("identical frame was encoded again")
	}
	a.Pix[0] = 200
	w.show(a)
	if &w.preview()[0] == &first[0] {
		t.Fatal("changed frame served from cache")
	}
}
