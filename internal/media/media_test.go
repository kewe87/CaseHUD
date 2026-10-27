package media

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoverCropsToFill(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 400)) // wide
	for x := 0; x < 800; x++ {
		for y := 0; y < 400; y++ {
			if x >= 200 && x < 600 {
				src.Set(x, y, color.White) // centre square white
			}
		}
	}
	out := Cover(src, 100, 100)
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 100 {
		t.Fatalf("size %v", out.Bounds())
	}
	if c := out.RGBAAt(5, 50); c.R < 200 {
		t.Fatalf("edge pixel %v: expected the white centre to fill the frame", c)
	}
}

func TestGIFClip(t *testing.T) {
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < 3; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 10, 10), pal)
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10) // 100ms
	}
	p := filepath.Join(t.TempDir(), "a.gif")
	f, _ := os.Create(p)
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
	f.Close()
	c, err := Load(p, 40, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Frames) != 3 || c.Static() {
		t.Fatalf("%d frames", len(c.Frames))
	}
	if c.At(150*time.Millisecond) != c.Frames[1].Image || c.At(350*time.Millisecond) != c.Frames[0].Image {
		t.Fatal("At() does not loop over frame delays")
	}
}
