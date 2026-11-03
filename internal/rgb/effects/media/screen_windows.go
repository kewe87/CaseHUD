package media

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"unsafe"

	leds "github.com/kewe87/casehud/internal/rgb"
	"golang.org/x/sys/windows"
)

var (
	modUser32                          = windows.NewLazySystemDLL("user32.dll")
	modGdi32                           = windows.NewLazySystemDLL("gdi32.dll")
	procGetDC                          = modUser32.NewProc("GetDC")
	procReleaseDC                      = modUser32.NewProc("ReleaseDC")
	procEnumDisplayMonitors            = modUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW                = modUser32.NewProc("GetMonitorInfoW")
	procSetThreadDpiAwarenessCtx       = modUser32.NewProc("SetThreadDpiAwarenessContext")
	procCreateCompatibleDC             = modGdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection               = modGdi32.NewProc("CreateDIBSection")
	procSelectObject                   = modGdi32.NewProc("SelectObject")
	procDeleteObject                   = modGdi32.NewProc("DeleteObject")
	procDeleteDC                       = modGdi32.NewProc("DeleteDC")
	procStretchBlt                     = modGdi32.NewProc("StretchBlt")
	procSetStretchBltMode              = modGdi32.NewProc("SetStretchBltMode")
	procSetBrushOrgEx                  = modGdi32.NewProc("SetBrushOrgEx")
	procGdiFlush                       = modGdi32.NewProc("GdiFlush")
	dpiAwarenessPerMonitorV2           = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	monitorInfoPrimary           int32 = 1
)

const (
	halftone = 4
	srcCopy  = 0x00CC0020
)

type monitorInfo struct {
	Size    uint32
	Monitor windows.Rect
	Work    windows.Rect
	Flags   uint32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

var (
	monitorMu            sync.Mutex
	monitorList          []windows.Rect
	monitorPrimary       int
	monitorCallback      uintptr
	monitorCallbackSetup sync.Once
)

// monitors returns the monitor rectangles, primary first. The coordinates
// depend on the DPI awareness of the calling thread.
func monitors() []windows.Rect {
	// NewCallback slots are limited per process: create the callback once.
	monitorCallbackSetup.Do(func() {
		monitorCallback = windows.NewCallback(func(h, hdc, rect, data uintptr) uintptr {
			mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
			if r, _, _ := procGetMonitorInfoW.Call(h, uintptr(unsafe.Pointer(&mi))); r != 0 {
				if int32(mi.Flags)&monitorInfoPrimary != 0 {
					monitorPrimary = len(monitorList)
				}
				monitorList = append(monitorList, mi.Monitor)
			}
			return 1
		})
	})
	monitorMu.Lock()
	defer monitorMu.Unlock()
	monitorList, monitorPrimary = nil, 0
	procEnumDisplayMonitors.Call(0, 0, monitorCallback, 0)
	out := append([]windows.Rect(nil), monitorList...)
	if monitorPrimary > 0 {
		out[0], out[monitorPrimary] = out[monitorPrimary], out[0]
	}
	return out
}

// gdiScreen captures with StretchBlt (HALFTONE) into a canvas-sized DIB.
// All GDI objects are created and used on the capture goroutine's thread
// (it is locked to its OS thread); Close releases them, a later Grab sets up again.
type gdiScreen struct {
	display  int
	ready    bool
	rect     windows.Rect
	screenDC uintptr
	memDC    uintptr
	bmp      uintptr
	oldBmp   uintptr // the bitmap that was selected into memDC before ours
	bits     unsafe.Pointer
}

func openScreen(display int) (grabber, error) {
	if n := len(monitors()); display < 0 || display >= n {
		return nil, fmt.Errorf("media: display %d not found (%d connected)", display, n)
	}
	return &gdiScreen{display: display}, nil
}

func (g *gdiScreen) setup() error {
	procSetThreadDpiAwarenessCtx.Call(dpiAwarenessPerMonitorV2)
	mons := monitors()
	if g.display >= len(mons) {
		return errors.New("media: display disconnected")
	}
	g.rect = mons[g.display]
	g.screenDC, _, _ = procGetDC.Call(0)
	if g.screenDC == 0 {
		return errors.New("media: GetDC failed")
	}
	g.memDC, _, _ = procCreateCompatibleDC.Call(g.screenDC)
	bi := bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: leds.CanvasWidth,
		Height: -leds.CanvasHeight, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	g.bmp, _, _ = procCreateDIBSection.Call(g.memDC, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if g.memDC == 0 || g.bmp == 0 || bits == nil {
		g.release()
		return errors.New("media: cannot create capture bitmap")
	}
	g.bits = bits
	g.oldBmp, _, _ = procSelectObject.Call(g.memDC, g.bmp)
	procSetStretchBltMode.Call(g.memDC, halftone)
	procSetBrushOrgEx.Call(g.memDC, 0, 0, 0)
	g.ready = true
	return nil
}

func (g *gdiScreen) Grab(dst *image.RGBA) error {
	if !g.ready {
		if err := g.setup(); err != nil {
			return err
		}
	}
	if g.screenDC == 0 { // dropped after a failed blit: the bitmap stays, the screen DC is taken again
		mons := monitors()
		if g.display >= len(mons) {
			return errors.New("media: display disconnected")
		}
		g.rect = mons[g.display]
		if g.screenDC, _, _ = procGetDC.Call(0); g.screenDC == 0 {
			return errors.New("media: GetDC failed")
		}
	}
	r := g.rect
	ok, _, _ := procStretchBlt.Call(g.memDC, 0, 0, leds.CanvasWidth, leds.CanvasHeight,
		g.screenDC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), srcCopy)
	procGdiFlush.Call()
	if ok == 0 {
		// desktop switch (UAC, lock screen): take a new screen DC next time,
		// the bitmap is still good
		procReleaseDC.Call(0, g.screenDC)
		g.screenDC = 0
		return errors.New("media: screen capture failed")
	}
	src := unsafe.Slice((*byte)(g.bits), leds.CanvasWidth*leds.CanvasHeight*4)
	for i := 0; i+3 < len(src) && i+3 < len(dst.Pix); i += 4 {
		dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = src[i+2], src[i+1], src[i], 255
	}
	return nil
}

func (g *gdiScreen) release() {
	// A bitmap that is selected into a DC cannot be deleted: select the old
	// one back first, then delete the DIB and the DC.
	if g.memDC != 0 && g.oldBmp != 0 {
		procSelectObject.Call(g.memDC, g.oldBmp)
	}
	if g.bmp != 0 {
		procDeleteObject.Call(g.bmp)
	}
	if g.memDC != 0 {
		procDeleteDC.Call(g.memDC)
	}
	if g.screenDC != 0 {
		procReleaseDC.Call(0, g.screenDC)
	}
	g.bmp, g.oldBmp, g.memDC, g.screenDC, g.bits, g.ready = 0, 0, 0, 0, nil, false
}

func (g *gdiScreen) Close() error {
	g.release()
	return nil
}
