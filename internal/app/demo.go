package app

import (
	"fmt"
	"image"
	"os"

	"github.com/kewe87/casehud/internal/panel"
)

// demoPanel is a simulated LCD for trying the GUI without hardware.
type demoPanel struct {
	id, name string
	size     image.Point
	round    bool
}

func (d *demoPanel) ID() string              { return d.id }
func (d *demoPanel) Name() string            { return d.name }
func (d *demoPanel) Size() image.Point       { return d.size }
func (d *demoPanel) MaxJPEG() int            { return 1 << 20 }
func (d *demoPanel) ShowJPEG([]byte) error   { return nil }
func (d *demoPanel) SetBrightness(int) error { return nil }
func (d *demoPanel) SetRotation(int) error   { return nil }
func (d *demoPanel) Close() error            { return nil }

func demoPanels() []panel.Panel {
	var out []panel.Panel
	// One fan on the second port and three on the third (device ports 1 and 2).
	for _, pos := range [][2]int{{1, 0}, {2, 0}, {2, 1}, {2, 2}} {
		out = append(out, &demoPanel{id: fmt.Sprintf("tllcd-p%d-i%d", pos[0], pos[1]),
			name: fmt.Sprintf("UNI FAN TL LCD (port %d, fan %d)", panel.TLPhysicalPort(pos[0]), pos[1]+1), size: image.Pt(400, 400), round: true})
	}
	out = append(out, &demoPanel{id: "aio-7398-0", name: "HydroShift LCD", size: image.Pt(480, 480)})
	if os.Getenv("CASEHUD_DEMO_BAR") != "" { // a 1920x480 bar for trying grids without hardware
		out = append(out, &demoBar{demoPanel{id: "us88-0", name: "Universal Screen 8.8\"", size: image.Pt(1920, 480)}})
	}
	return out
}

// demoBar is a simulated 1920x480 bar that can be mounted upright.
type demoBar struct{ demoPanel }

func (b *demoBar) SoftQuarterTurns() bool { return true }

// MaxFPS mirrors the real bar.
func (b *demoBar) MaxFPS() int { return 20 }

// Round reports whether the simulated screen is circular.
func (d *demoPanel) Round() bool { return d.round }

// MaxFPS mirrors the real panels: 25 for TL LCD fans, 24 for pump heads.
func (d *demoPanel) MaxFPS() int {
	if d.size.X == 400 {
		return 25
	}
	return 24
}
