// Package enram drives ENE-based RGB RAM on the AMD PIIX4 SMBus.
//
// Safety rules (SMBus writes to the wrong device can damage hardware):
//   - Only addresses 0x70..0x77 are ever accessed; anything else (in particular
//     the SPD range 0x50..0x57) is rejected by Validate and by the bus guard.
//   - No address remapping: only sticks that already answer on an allowed
//     address with valid ENE registers and a known device name are used.
//   - Every batch runs under the system SMBus mutex shared with other
//     hardware tools; if it is not free within a few
//     milliseconds the frame is dropped.
//   - The PIIX4 port is never switched; only port 0 (base 0x0B00) is used.
//
// Access goes through a Transport: the CaseHUD SMBus bridge (named pipe to the
// SYSTEM service). CaseHUD vendors only this path; there is no in-process bus
// access.
//
// ENE register access: write the register number (byte swapped) as SMBus word
// to command 0x00, then read a byte with command 0x81, write a byte with 0x01
// or a block with 0x03.
package enram

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"sync"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/internal/conflicts"
)

const (
	addrMin = 0x70
	addrMax = 0x77

	regDeviceName  = 0x1000
	regConfigTable = 0x1C00
	regDirectMode  = 0x8020
	regApply       = 0x802F

	cmdRegister = 0x00
	cmdWrite    = 0x01
	cmdBlock    = 0x03
	cmdRead     = 0x81

	blockBytes = 15 // colors per block write (5 LEDs)

	mutexName  = `Global\Access_SMBUS.HTP.Method`
	lockMs     = 20
	scanLockMs = 50
	piix4Base  = 0x0B00
)

// family is a group of controllers recognised by a part of their firmware
// name, with the register block that takes their direct colors (3 bytes per
// LED). Only families verified on real hardware are listed; other controllers
// are reported but never written to.
type family struct {
	marker string
	colors uint16
	size   int // bytes in the color block
}

var families = []family{
	// one DDR5 RGB family, firmware name AUDA0-E6K5-0101: colors at 0x8100,
	// block 0x8100..0x813F (verified: write test through the SMBus bridge)
	{marker: "-E6K5-", colors: 0x8100, size: 0x40},
}

func familyOf(name string) (family, bool) {
	for _, f := range families {
		if strings.Contains(name, f.marker) {
			return f, true
		}
	}
	return family{}, false
}

// ErrUnknownStick reports an ENE controller of an unverified family.
type ErrUnknownStick struct {
	Addr byte
	Name string
}

func (e ErrUnknownStick) Error() string {
	return fmt.Sprintf("enram: RGB controller %q at %#02x not verified, left alone", e.Name, e.Addr)
}

// ErrBusy is returned when the SMBus mutex is held by someone else.
var ErrBusy = errors.New("enram: SMBus busy")

// Bus is the SMBus subset ENE needs, used by NewBusTransport.
type Bus interface {
	ReadByteData(addr, cmd byte) (byte, error)
	WriteByteData(addr, cmd, v byte) error
	WriteWordData(addr, cmd byte, v uint16) error
	WriteBlockData(addr, cmd byte, data []byte) error
	// Lock acquires exclusive bus access, waiting at most ms milliseconds.
	Lock(ms uint32) error
	Unlock()
}

// guarded enforces the address window in front of any Bus.
type guarded struct{ Bus }

func checkAddr(addr byte) error {
	if addr < addrMin || addr > addrMax {
		return fmt.Errorf("enram: refusing SMBus address %#02x (allowed %#02x..%#02x)", addr, addrMin, addrMax)
	}
	return nil
}

func (g guarded) ReadByteData(addr, cmd byte) (byte, error) {
	if err := checkAddr(addr); err != nil {
		return 0, err
	}
	return g.Bus.ReadByteData(addr, cmd)
}

func (g guarded) WriteByteData(addr, cmd, v byte) error {
	if err := checkAddr(addr); err != nil {
		return err
	}
	return g.Bus.WriteByteData(addr, cmd, v)
}

func (g guarded) WriteWordData(addr, cmd byte, v uint16) error {
	if err := checkAddr(addr); err != nil {
		return err
	}
	return g.Bus.WriteWordData(addr, cmd, v)
}

func (g guarded) WriteBlockData(addr, cmd byte, data []byte) error {
	if err := checkAddr(addr); err != nil {
		return err
	}
	return g.Bus.WriteBlockData(addr, cmd, data)
}

// Backend discovers ENE RAM sticks.
type Backend struct {
	mu sync.Mutex
	t  Transport
}

// New uses the CaseHUD SMBus bridge. pipe is the named pipe; empty means
// $CASEHUD_SMBUS_PIPE or BridgePipe.
func New(pipe string) *Backend {
	return NewWithTransport(NewPipeTransport(pipe))
}

// NewWithTransport uses t for all bus access.
func NewWithTransport(t Transport) *Backend { return &Backend{t: t} }

// Name implements leds.Backend.
func (*Backend) Name() string { return "enram" }

func (b *Backend) transport() (Transport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.t, nil
}

// Discover implements leds.Backend. It only reads ENE registers.
func (b *Backend) Discover(ctx context.Context) ([]leds.Device, error) {
	t, err := b.transport()
	if err != nil {
		return nil, err
	}
	var out []leds.Device
	var errs []error
	for addr := byte(addrMin); addr <= addrMax; addr++ {
		if ctx.Err() != nil {
			break
		}
		d, err := probe(t, addr)
		if err != nil {
			if !isIO(err) {
				errs = append(errs, fmt.Errorf("%#02x: %w", addr, err))
			}
			if errors.Is(err, ErrBusy) || errors.Is(err, ErrDenied) || errors.Is(err, ErrUnavailable) || errors.Is(err, ErrUntrusted) {
				break // the next address would meet the same answer
			}
			continue
		}
		if d != nil {
			out = append(out, d)
		}
	}
	return out, errors.Join(errs...)
}

// probe identifies an ENE stick at addr; nil without error means "no stick".
func probe(t Transport, addr byte) (*stick, error) {
	// an ENE controller answers the identification commands 0xA0..0xAF with
	// 0x00..0x0F; all sixteen must match
	var ops []Op
	for i := byte(0); i < 16; i++ {
		ops = append(ops, Op{Op: OpReadCmd, Cmd: identCmdMin + i})
	}
	ident, err := t.Batch(addr, scanLockMs, ops)
	if err != nil {
		if isIO(err) {
			return nil, nil // nothing answers here
		}
		return nil, err
	}
	for i, v := range ident {
		if v != byte(i) {
			return nil, nil
		}
	}
	if len(ident) != 16 {
		return nil, nil
	}

	// firmware name (16 bytes) and the first 16 bytes of the config block;
	// config byte 2 holds the LED count
	ops = ops[:0]
	for i := uint16(0); i < 16; i++ {
		ops = append(ops, Op{Op: OpReadReg, Reg: regDeviceName + i})
	}
	for i := uint16(0); i < 16; i++ {
		ops = append(ops, Op{Op: OpReadReg, Reg: regConfigTable + i})
	}
	r, err := t.Batch(addr, scanLockMs, ops)
	if err != nil {
		return nil, err
	}
	if len(r) != 32 {
		return nil, fmt.Errorf("short read (%d bytes)", len(r))
	}
	name, config := cstr(r[:16]), r[16:32]
	fam, ok := familyOf(name)
	if !ok {
		return nil, ErrUnknownStick{Addr: addr, Name: name}
	}
	count := int(config[2])
	if count < 1 || 3*count > fam.size {
		return nil, fmt.Errorf("%q: implausible LED count %d", name, count)
	}
	return &stick{t: t, addr: addr, model: name, colors: fam.colors, count: count,
		info: fmt.Sprintf("config % x", config)}, nil
}

func cstr(b []byte) string {
	var out []byte
	for _, c := range b {
		if c > 0 {
			out = append(out, c)
		}
	}
	return string(out)
}

type stick struct {
	t      Transport
	addr   byte
	model  string
	colors uint16 // direct color block
	count  int
	info   string

	mu     sync.Mutex
	direct bool
	prev   []byte
}

// Describe returns the raw identification data.
func (s *stick) Describe() string { return s.info }

func (s *stick) ID() string { return fmt.Sprintf("enram:%04x:%02x", piix4Base, s.addr) }

// Firmware is the name the stick reports in its device-name register.
func (s *stick) Firmware() string { return s.model }

func (s *stick) Name() string { return fmt.Sprintf("RGB RAM %s @%#02x", s.model, s.addr) }

func (s *stick) Zones() []leds.Zone {
	// ENE expects red, blue, green
	return []leds.Zone{{ID: "leds", Name: "DIMM", Count: s.count, Max: s.count, Order: "RBG"}}
}

func (s *stick) Conflicts() []string {
	return conflicts.RAM
}

// SetFrame sends direct mode (first frame only), the colors and apply as one batch.
func (s *stick) SetFrame(frame map[string][]color.RGBA) error {
	cols := frame["leds"]
	data := make([]byte, 0, 3*s.count)
	for _, c := range cols[:min(len(cols), s.count)] {
		data = append(data, c.R, c.G, c.B)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.direct && string(data) == string(s.prev) {
		return nil // unchanged: keep the bus free
	}
	var ops []Op
	if !s.direct {
		ops = append(ops, Op{Op: OpWriteReg, Reg: regDirectMode, Val: 0x01})
	}
	base := s.colors
	for off := 0; off < len(data); off += blockBytes {
		ops = append(ops, Op{Op: OpBlock, Reg: base + uint16(off), Data: data[off:min(off+blockBytes, len(data))]})
	}
	ops = append(ops, Op{Op: OpWriteReg, Reg: regApply, Val: 0x01})

	if _, err := s.t.Batch(s.addr, lockMs, ops); err != nil {
		if errors.Is(err, ErrBusy) {
			return nil // drop this frame
		}
		return err
	}
	s.direct = true
	s.prev = data
	return nil
}

// RestoreDeviceEffect implements leds.DeviceEffectRestorer: direct mode off,
// the stick shows its own stored effect again.
func (s *stick) RestoreDeviceEffect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.direct {
		return nil
	}
	if _, err := s.t.Batch(s.addr, lockMs, []Op{
		{Op: OpWriteReg, Reg: regDirectMode, Val: 0x00},
		{Op: OpWriteReg, Reg: regApply, Val: 0x01},
	}); err != nil {
		return err
	}
	s.direct, s.prev = false, nil
	return nil
}

// Close leaves the stick showing the last frame; the transport stays with the backend.
func (s *stick) Close() error { return nil }
