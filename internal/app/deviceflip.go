package app

import (
	"fmt"
	"log"

	"github.com/kewe87/casehud/internal/panel"
)

// The device-side rotation (experimental) lets a bar turn the picture by 180
// degrees itself instead of CaseHUD mirroring it. The setting is kept per
// display in the configuration; it is handed to the panel when the panel is
// opened (before its first frame) and when the user changes it. The panel
// writes the rotation only when the value differs from what the device holds,
// so neither a rescan nor an unchanged setting writes anything.

// SetDeviceFlip stores the device-side rotation setting of a bar display and
// applies it at once.
func (e *Engine) SetDeviceFlip(id string, on bool) error {
	e.mu.Lock()
	w, ok := e.workers[id]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrNoPanel, id)
	}
	f, can := w.p.(panel.DeviceFlipper)
	if !can || !f.CanDeviceFlip() {
		e.mu.Unlock()
		return fmt.Errorf("%q has no device-side rotation", id)
	}
	if e.cfg.DeviceFlip == nil {
		e.cfg.DeviceFlip = map[string]bool{}
	}
	if on {
		e.cfg.DeviceFlip[id] = true
	} else {
		delete(e.cfg.DeviceFlip, id)
	}
	e.mu.Unlock()
	if err := f.SetDeviceFlip(on); err != nil {
		log.Printf("%s: device-side rotation: %v", w.p.Name(), err)
		w.setErr(ErrDevice, err)
	}
	select { // draw the next frame now, in the new orientation
	case w.wake <- struct{}{}:
	default:
	}
	return e.persist()
}

// applyDeviceFlip hands the stored setting to the panels that were just opened.
// The panel only changes its mode (the rotation goes out with the worker's
// first device update, and only if the device does not hold it already).
// Called without e.mu: the panel may talk to the device.
func (e *Engine) applyDeviceFlip(panels []panel.Panel) {
	for _, p := range panels {
		f, ok := p.(panel.DeviceFlipper)
		if !ok || !f.CanDeviceFlip() {
			continue
		}
		e.mu.Lock()
		on := e.cfg.DeviceFlip[p.ID()]
		e.mu.Unlock()
		if on {
			f.SetDeviceFlip(true)
		}
	}
}
