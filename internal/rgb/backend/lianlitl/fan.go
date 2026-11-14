package lianlitl

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	cmdFanSpeed  = 0xAA
	cmdSpeedSync = 0xB1

	speedSyncBit = 0x80
	dutyStop     = 1 // wire value that stops a fan
	dutyLowest   = 12
	dutyHighest  = 100

	speedPause = 100 * time.Millisecond
)

// LowestDuty is the lowest duty above 0 the controller acts on: SetDuty raises
// every smaller value to it, so a duty below it is not a speed of its own.
const LowestDuty = dutyLowest

// FanDevice is the speed control of one controller.
type FanDevice interface {
	FanIDs() []string
	TakeControl(id string) error
	SetDuty(id string, percent int) error
	Speeds() (map[string]int, error)
	Close() error
}

// OpenFans opens the controller for speed control, with its own handle, so it
// does not depend on the lighting being on. Opening only reads. With several
// controllers the fan names would not be unique, which is refused for now.
func OpenFans(ctx context.Context) (FanDevice, error) {
	devs, err := New().Discover(ctx)
	switch len(devs) {
	case 1:
		if err != nil {
			devs[0].Close() // a second one that failed to open still makes the names ambiguous
			return nil, fmt.Errorf("lianlitl: another controller could not be opened: %w", err)
		}
		return devs[0].(*device), nil
	case 0:
		return nil, errors.Join(errors.New("lianlitl: no controller found"), err)
	}
	for _, d := range devs {
		d.Close()
	}
	return nil, errors.New("lianlitl: more than one controller")
}

// fanID is the stable name of a fan: 1-based, as printed on the controller.
func fanID(f fan) string { return fmt.Sprintf("port%d-fan%d", f.port+1, f.index+1) }

// lookupFan finds a connected fan by its ID.
func (d *device) lookupFan(id string) (fan, error) {
	for _, f := range d.fans {
		if fanID(f) == id {
			return f, nil
		}
	}
	return fan{}, fmt.Errorf("lianlitl: no fan %q", id)
}

func fanAddress(f fan) byte { return byte(f.port<<4 | f.index&0x0F) }

// FanIDs lists the connected fans in port and chain order.
func (d *device) FanIDs() []string {
	ids := make([]string, 0, len(d.fans))
	for _, f := range d.fans {
		ids = append(ids, fanID(f))
	}
	return ids
}

// dutyByte turns a percentage into the wire value: 0 stops the fan, the
// controller's lowest running duty is 12.
func dutyByte(percent int) (byte, error) {
	switch {
	case percent < 0 || percent > dutyHighest:
		return 0, fmt.Errorf("lianlitl: duty %d out of range", percent)
	case percent == 0:
		return dutyStop, nil
	default:
		return byte(max(percent, dutyLowest)), nil
	}
}

// SetDuty sets the speed of one fan in percent.
func (d *device) SetDuty(id string, percent int) error {
	wire, err := dutyByte(percent)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	f, err := d.lookupFan(id)
	if err != nil {
		return err
	}
	if err := d.h.Write(packet(cmdFanSpeed, []byte{fanAddress(f), wire}), ioTimeout); err != nil {
		return err
	}
	// AA has no answer, so a lost packet cannot be noticed here. The pause
	// keeps the next command from overwriting this one (see send).
	time.Sleep(speedPause)
	return nil
}

// TakeControl makes one fan follow SetDuty instead of the controller's PWM
// input.
func (d *device) TakeControl(id string) error { return d.setSpeedSync(id, false) }

// ReleaseControl connects one fan to the controller's PWM input again. All
// fans released this way run at the same speed.
func (d *device) ReleaseControl(id string) error { return d.setSpeedSync(id, true) }

func (d *device) setSpeedSync(id string, sync bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	f, err := d.lookupFan(id)
	if err != nil {
		return err
	}
	b := fanAddress(f)
	if sync {
		b |= speedSyncBit
	}
	return d.send(cmdSpeedSync, []byte{b})
}

// Speeds reads the current rpm of every detected fan. It fails while the
// controller reports a firmware update.
func (d *device) Speeds() (map[string]int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fans, err := d.handshake()
	if err != nil {
		return nil, err
	}
	rpm := reportedRPM(d.raw)
	out := make(map[string]int, len(fans))
	for _, f := range fans {
		out[fanID(f)] = rpm[f]
	}
	return out, nil
}

// reportedRPM reads the speed of every detected fan from a raw A1 answer. The
// answer is padded with empty slots, whose address reads as port 0, fan 0.
func reportedRPM(raw []byte) map[fan]int {
	rpm := map[fan]int{}
	if len(raw) <= offLen {
		return rpm
	}
	for i := 0; i < int(raw[offLen])/3 && offPayload+3*i+2 < len(raw); i++ {
		e := raw[offPayload+3*i:]
		if e[0]&flagDetected == 0 {
			continue
		}
		rpm[fan{port: int(e[0]>>4) & 3, index: int(e[0] & 0x0F)}] = int(e[1])<<8 | int(e[2])
	}
	return rpm
}
