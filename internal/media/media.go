// Package media turns images, GIFs and videos into frame sequences sized for
// an LCD panel.
package media

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // register decoders
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Frame is one image with how long it stays on screen.
type Frame struct {
	Image image.Image
	Delay time.Duration
}

// Clip is a looping frame sequence.
type Clip struct {
	Frames []Frame
	total  time.Duration
}

// At returns the frame to show at elapsed time t (looping).
func (c *Clip) At(t time.Duration) image.Image {
	if len(c.Frames) == 1 || c.total <= 0 {
		return c.Frames[0].Image
	}
	t %= c.total
	for _, f := range c.Frames {
		if t < f.Delay {
			return f.Image
		}
		t -= f.Delay
	}
	return c.Frames[len(c.Frames)-1].Image
}

// Static reports whether the clip is a single image.
func (c *Clip) Static() bool { return len(c.Frames) == 1 }

func newClip(frames []Frame) (*Clip, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames")
	}
	c := &Clip{Frames: frames}
	for _, f := range frames {
		c.total += f.Delay
	}
	return c, nil
}

// Kind classifies a file by extension: "image", "gif", "video" or "".
func Kind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return "image"
	case ".gif":
		return "gif"
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v":
		return "video"
	}
	return ""
}

// Load decodes path into a clip of w x h frames (center-cropped to fill).
func Load(path string, w, h int) (*Clip, error) {
	switch Kind(path) {
	case "image":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		// Check the declared size first: a tiny file can claim gigapixels.
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", filepath.Base(path), err)
		}
		if cfg.Width*cfg.Height > maxImagePixels {
			return nil, fmt.Errorf("%s is too large (%dx%d)", filepath.Base(path), cfg.Width, cfg.Height)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", filepath.Base(path), err)
		}
		return newClip([]Frame{{Image: Cover(img, w, h), Delay: time.Second}})
	case "gif":
		return loadGIF(path, w, h)
	case "video":
		return loadVideo(path, w, h)
	}
	return nil, fmt.Errorf("unsupported file type %q", filepath.Ext(path))
}

// Cover scales img to fill w x h, cropping the overflow (CSS object-fit: cover).
func Cover(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	scale := max(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	cw, ch := int(float64(w)/scale), int(float64(h)/scale)
	min := image.Pt(b.Min.X+(b.Dx()-cw)/2, b.Min.Y+(b.Dy()-ch)/2)
	src := image.Rectangle{Min: min, Max: min.Add(image.Pt(cw, ch))}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Src, nil)
	return dst
}

// Limits that keep crafted files from exhausting memory.
const (
	maxGIFFrames   = 600         // frames composed for long animations
	maxImagePixels = 8192 * 8192 // still images
	maxGIFPixels   = 1920 * 1080 // per GIF frame
	maxGIFBytes    = 64 << 20    // GIF file size
)

func loadGIF(path string, w, h int) (*Clip, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// DecodeAll unpacks every frame, so bound the canvas and file size.
	if fi, err := f.Stat(); err == nil && fi.Size() > maxGIFBytes {
		return nil, fmt.Errorf("%s is too large for a GIF (max %d MB)", filepath.Base(path), maxGIFBytes>>20)
	}
	cfg, err := gif.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", filepath.Base(path), err)
	}
	if cfg.Width*cfg.Height > maxGIFPixels {
		return nil, fmt.Errorf("%s is too large (%dx%d)", filepath.Base(path), cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", filepath.Base(path), err)
	}
	// Compose frames on a canvas, honouring disposal methods.
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var frames []Frame
	for i, pal := range g.Image {
		if i >= maxGIFFrames {
			break
		}
		var restore *image.RGBA
		if g.Disposal != nil && g.Disposal[i] == gif.DisposalPrevious {
			restore = image.NewRGBA(canvas.Bounds())
			draw.Draw(restore, canvas.Bounds(), canvas, image.Point{}, draw.Src)
		}
		draw.Draw(canvas, pal.Bounds(), pal, pal.Bounds().Min, draw.Over)
		delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if delay < 20*time.Millisecond {
			delay = 100 * time.Millisecond // browsers treat 0/1 as 100ms too
		}
		frames = append(frames, Frame{Image: Cover(canvas, w, h), Delay: delay})
		if g.Disposal != nil {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, pal.Bounds(), image.Transparent, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				canvas = restore
			}
		}
	}
	return newClip(frames)
}
