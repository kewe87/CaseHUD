package lianlitl

import (
	"fmt"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Off switch.
//
// A3 (per fan) and B0 (per group) carry an off flag: payload byte 18, bit 0,
// and the brightness index in byte 3. The controller keeps both in its flash,
// so the off state survives restarts of the host and of the controller. That
// is why it is written once per change and never repeated on a restart, a
// resume or a rescan (SetOff with held set only takes the state over).
//
// The off commands keep mode, speed, colors and direction of the current
// setting; only brightness (0) and the flag change. The direction byte must be
// below 4, a larger one is sent as 0.
//
// Exception, motherboard-sync mode only: the firmware may reset the RAM state
// of all fans (rainbow, 100 %, off flag cleared) when it is configured within
// a short time after a USB reconnect. The flash still holds off. Because that
// cannot be detected, an off device in this mode gets the off set once when it
// is taken over (app start, reconnect), not on resume or rescan.

const (
	offFlagByte = 18
	// minOffInterval separates two changes of the off state: every change
	// is a set of flash writes.
	minOffInterval = minInterval
)

// ErrOffTooSoon is returned by SetOff when the controller was written a moment
// ago. Like ErrPassive and ErrFlashWrites it wraps leds.ErrOffBlocked: nothing
// was written and the engine tries again later.
var ErrOffTooSoon = fmt.Errorf("%w: the controller was written a moment ago, waiting before writing it again", leds.ErrOffBlocked)

// ErrPassive is returned by SetOff in passive mode, which never writes.
var ErrPassive = fmt.Errorf("%w: passive mode never writes to the controller", leds.ErrOffBlocked)

// offGate says whether the controller may be written now: not in passive
// mode, only with allowFlashWrites (every off or on command is stored in the
// flash; also refused while the last Configure failed, because the settings
// are then stale), and not within minOffInterval of the last attempt.
func (d *device) offGate() error {
	if d.set.passive {
		return ErrPassive
	}
	if !d.flashOK {
		return fmt.Errorf("%w: %w", leds.ErrOffBlocked, ErrFlashWrites)
	}
	if !d.offAt.IsZero() && d.now().Sub(d.offAt) < minOffInterval {
		return ErrOffTooSoon
	}
	return nil
}

// SetOff implements leds.PersistentOff. held means the controller already is
// in that state: it is remembered, and in motherboard-sync mode written once
// more (see the note above). Every attempt that sends starts the rate limit,
// also when it fails half way.
func (d *device) SetOff(off, held bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if held {
		d.off, d.offClobbered = off, false
		if off && d.set.motherboard && d.offGate() == nil {
			d.offAt = d.now()
			if err := d.writeLights(true); err != nil {
				return err
			}
			d.synced = false
		}
		return nil
	}
	if d.off == off {
		return nil
	}
	if err := d.offGate(); err != nil {
		return err
	}
	s := d.set
	d.offAt = d.now()
	if err := d.writeLights(off); err != nil {
		return err
	}
	d.off, d.offClobbered = off, false
	d.synced = false // the fans follow their own light now, not the ARGB input
	switch {
	case off:
		d.pending = d.pending[:0]
	case s.motherboard:
		d.dirty = true // the next frame connects the ports to the ARGB input again
	case s.hardware:
		d.dirty = false // the effect was just written to the groups
		d.pending = d.pending[:0]
	default:
		clear(d.sent) // canvas: the rings show the canvas again
		d.dirty = false
		d.pending = d.pending[:0]
	}
	return nil
}

// writeLights sends the light of every fan (A3) and every group (B0) with the
// current setting and the off flag as given.
func (d *device) writeLights(off bool) error {
	s := d.set
	level, flag := s.brightness, byte(0)
	if off {
		level, flag = 0, 1
	}
	dir := s.direction
	if dir >= 4 {
		dir = 0
	}
	fill := func(p []byte) {
		p[2], p[3], p[4] = s.mode, level, s.speed
		n := min(len(s.colors), maxColors)
		for i := 0; i < n; i++ {
			p[5+3*i], p[6+3*i], p[7+3*i] = s.colors[i].R, s.colors[i].G, s.colors[i].B
		}
		p[17], p[offFlagByte], p[19] = dir, flag, byte(n)
	}
	for _, f := range d.fans {
		p := make([]byte, lightLen)
		p[0], p[1] = byte(f.port<<4), byte(f.port<<4|f.index&0x0F)
		fill(p)
		if err := d.send(cmdFanLight, p); err != nil {
			return err
		}
	}
	for _, g := range d.offGroups() {
		p := make([]byte, lightLen)
		p[1] = g
		fill(p)
		if err := d.send(cmdGroupLite, p); err != nil {
			return err
		}
	}
	return nil
}

// offGroups lists the groups that exist: the merged group, the canvas rings of
// a port or its default group.
func (d *device) offGroups() []byte {
	if d.set.hardware && d.set.merge {
		return []byte{mergedGroup}
	}
	var out []byte
	seen := map[byte]bool{}
	add := func(g byte) {
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	for _, port := range d.ports {
		if d.ringGrouped[port] {
			for _, r := range d.rings {
				if r.port == port {
					add(r.group)
				}
			}
			continue
		}
		add(group(port))
	}
	return out
}

// settleOff runs after the calls that replace the stored lights of the fans
// outside the frame path (the sync switch, regrouping, per-fan and per-ring
// colors, test lights). A device that is off then gets its off state once
// more, if the gates allow it; if not, the flag stays and the next call after
// the rate limit tries again. The mutex is held.
func (d *device) settleOff() {
	if !d.off || !d.offClobbered || d.offGate() != nil {
		return
	}
	d.offAt = d.now()
	if err := d.writeLights(true); err != nil {
		return // the flag stays; no loop, the next call decides
	}
	d.offClobbered = false
	d.synced = false
}

// touchOff records that a command which replaces the fan lights is about to
// be sent; the caller defers settleOff.
func (d *device) touchOff() { d.offClobbered = d.offClobbered || d.off }
