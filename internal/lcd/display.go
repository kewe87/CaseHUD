package lcd

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Display is a sink for rendered frames: an LCD panel driver, or a PNG file.
type Display interface {
	Show(img image.Image) error
	Close() error
}

// PNGDisplay writes every frame to a PNG file (atomically, so an image viewer
// with auto-reload never sees a half-written file). Handy for designing
// layouts before the panel driver is wired up.
type PNGDisplay struct {
	Path string
}

// Show implements Display.
func (p PNGDisplay) Show(img image.Image) error {
	tmp, err := os.CreateTemp(filepath.Dir(p.Path), ".casehud-*.png")
	if err != nil {
		return err
	}
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p.Path)
}

// Close implements Display.
func (p PNGDisplay) Close() error { return nil }
