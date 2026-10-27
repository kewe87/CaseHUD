package panel

import (
	"image"
	"strings"
	"sync/atomic"
)

// Optional capabilities of panels that keep a picture and a transparent
// overlay apart. The picture is sent once, the live values go out as a PNG
// layer when they change, which saves encoding and USB traffic; an H.264
// video can play on the device itself with the values on top.

// LayerCaps tells what a LayeredPanel can do right now.
type LayerCaps struct {
	Static bool // picture once, overlay on change
	Video  bool // plays an H.264 file itself, overlay on top
}

// Backdrop is the picture under the overlay: either a static image (sent once
// per Key) or a video file the device plays.
type Backdrop struct {
	// Key identifies the static image. The panel sends the picture again only
	// when the key changes (or after a reconnect); Image builds it then.
	Key   uint64
	Image func() *image.RGBA
	// Video, when set, replaces the static image.
	Video *DeviceVideo
}

// DeviceVideo is a prepared Annex-B H.264 stream (baseline profile, no
// B-frames) in the orientation of the device frame.
type DeviceVideo struct {
	Path string
	FPS  int // 1..119
}

// LayeredPanel is a RawPanel that can show a backdrop with an overlay.
type LayeredPanel interface {
	RawPanel
	LayerCaps() LayerCaps
	// ShowLayers shows the backdrop with overlay on top. overlay has the
	// canvas size and premultiplied alpha (nil = nothing on top). The panel
	// must not keep or modify the images. It paces itself: an unchanged
	// overlay is only repeated as a keep-alive and changes are limited in rate.
	ShowLayers(b Backdrop, overlay *image.RGBA) error
}

// VideoOrienter tells how a picture of the canvas size must be turned before
// it is encoded as a device video: the number of clockwise quarter turns
// (0..3) that bring it into the orientation of the device frame.
type VideoOrienter interface {
	VideoTurn(rotation int) int
}

// OfflineClocker is implemented by panels whose built-in clock can be left on
// when the host is gone.
type OfflineClocker interface {
	// SetOfflineClock switches the flag the device keeps (a persistent
	// write). It writes only when the value differs from what the device was
	// last given.
	SetOfflineClock(on bool) error
	// OfflineClock is the flag the device was last given (false = unknown).
	OfflineClock() bool
}

// DeviceFlipper is implemented by panels that can turn the picture by 180
// degrees themselves (experimental, off until switched on).
type DeviceFlipper interface {
	// CanDeviceFlip tells whether this model has the device-side rotation.
	CanDeviceFlip() bool
	SetDeviceFlip(on bool) error
}

// Versioned is implemented by panels that report the firmware version of the
// device. It is shown to the user only; CaseHUD never installs firmware.
type Versioned interface {
	// Version is the text the device reported ("" if it reported none).
	Version() string
}

var _ = []Versioned{(*tlLCD)(nil), (*universal88)(nil), (*hydroShift)(nil)}

// firmwareText holds a device-reported version for display. It has its own
// lock-free storage so a reader never waits for a USB transfer, and it keeps
// only printable ASCII, at most firmwareMax bytes.
type firmwareText struct{ v atomic.Pointer[string] }

const firmwareMax = 48

func (f *firmwareText) set(s string) {
	s = CleanFirmware(s)
	f.v.Store(&s)
}

func (f *firmwareText) get() string {
	if s := f.v.Load(); s != nil {
		return *s
	}
	return ""
}

// CleanFirmware drops everything but printable ASCII and caps the length.
func CleanFirmware(s string) string {
	var b strings.Builder
	for i := 0; i < len(s) && b.Len() < firmwareMax; i++ {
		if c := s[i]; c >= 0x20 && c < 0x7F {
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}
