package enram

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"
)

// chip emulates ENE controllers on a bus and records every access.
type chip struct {
	sticks   map[byte]*eneState
	touched  map[byte]bool // every address the driver tried to access
	badIdent byte          // identification command that answers wrong
	locked   bool
}

type eneState struct {
	ptr  uint16
	regs map[uint16]byte
}

func newENE(name string, leds byte) *eneState {
	s := &eneState{regs: map[uint16]byte{}}
	for i, c := range []byte(name) {
		s.regs[regDeviceName+uint16(i)] = c
	}
	s.regs[regConfigTable+2] = leds
	return s
}

func (c *chip) at(addr byte) (*eneState, bool) {
	c.touched[addr] = true
	s, ok := c.sticks[addr]
	return s, ok
}

type nack struct{}

func (nack) Error() string { return "nack" }

func (c *chip) ReadByteData(addr, cmd byte) (byte, error) {
	s, ok := c.at(addr)
	if !ok {
		return 0, nack{}
	}
	switch {
	case cmd >= 0xA0 && cmd <= 0xAF:
		if cmd == c.badIdent {
			return 0xFF, nil
		}
		return cmd - 0xA0, nil
	case cmd == cmdRead:
		v := s.regs[s.ptr]
		s.ptr++
		return v, nil
	}
	return 0, nil
}

func (c *chip) WriteByteData(addr, cmd, v byte) error {
	s, ok := c.at(addr)
	if !ok {
		return nack{}
	}
	if cmd == cmdWrite {
		s.regs[s.ptr] = v
	}
	return nil
}

func (c *chip) WriteWordData(addr, cmd byte, v uint16) error {
	s, ok := c.at(addr)
	if !ok {
		return nack{}
	}
	if cmd == cmdRegister {
		s.ptr = v<<8 | v>>8
	}
	return nil
}

func (c *chip) WriteBlockData(addr, cmd byte, data []byte) error {
	s, ok := c.at(addr)
	if !ok {
		return nack{}
	}
	for i, b := range data {
		s.regs[s.ptr+uint16(i)] = b
	}
	return nil
}

func (c *chip) Lock(uint32) error { c.locked = true; return nil }
func (c *chip) Unlock()           { c.locked = false }

func TestDiscoverAndFrame(t *testing.T) {
	c := &chip{
		sticks: map[byte]*eneState{
			0x71: newENE("AUDA0-E6K5-0101", 8),
			0x73: newENE("AUDA0-E6K5-0101", 8),
			0x52: newENE("AUDA0-E6K5-0101", 8), // looks like ENE but is in the SPD range
		},
		touched: map[byte]bool{},
	}
	b := NewWithTransport(NewBusTransport(c))
	devs, err := b.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 2 || devs[0].ID() != "enram:0b00:71" || devs[1].ID() != "enram:0b00:73" {
		t.Fatalf("devices: %v", devs)
	}
	for addr := range c.touched {
		if addr < 0x70 || addr > 0x77 {
			t.Fatalf("driver touched address %#02x", addr)
		}
	}
	if z := devs[0].Zones(); len(z) != 1 || z[0].Count != 8 || z[0].Order != "RBG" {
		t.Fatalf("zones %+v", z)
	}

	cols := make([]color.RGBA, 8)
	for i := range cols {
		cols[i] = color.RGBA{byte(i), 0x10, 0x20, 255}
	}
	if err := devs[0].SetFrame(map[string][]color.RGBA{"leds": cols}); err != nil {
		t.Fatal(err)
	}
	s := c.sticks[0x71]
	if s.regs[regDirectMode] != 1 || s.regs[regApply] != 1 {
		t.Fatalf("direct %d apply %d", s.regs[regDirectMode], s.regs[regApply])
	}
	for i := 0; i < 8; i++ {
		if s.regs[0x8100+uint16(3*i)] != byte(i) || s.regs[0x8100+uint16(3*i+2)] != 0x20 {
			t.Fatalf("color %d not written", i)
		}
	}
	if c.locked {
		t.Fatal("mutex not released")
	}

	// unchanged frame: no bus traffic
	s.regs[regApply] = 0
	devs[0].SetFrame(map[string][]color.RGBA{"leds": cols})
	if s.regs[regApply] != 0 {
		t.Fatal("unchanged frame was written")
	}
}

func TestUnknownDeviceIgnored(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x70: newENE("SOMETHING-ELSE", 8)}, touched: map[byte]bool{}}
	devs, _ := NewWithTransport(NewBusTransport(c)).Discover(context.Background())
	if len(devs) != 0 {
		t.Fatalf("unknown device accepted: %v", devs)
	}
}

func TestGuardRefusesSPD(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x50: newENE("AUDA0-E6K5-0101", 8)}, touched: map[byte]bool{}}
	g := guarded{c}
	for _, err := range []error{
		g.WriteByteData(0x50, cmdWrite, 1),
		g.WriteWordData(0x57, cmdRegister, 0),
		g.WriteBlockData(0x50, cmdBlock, []byte{1}),
	} {
		if err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Fatalf("expected refusal, got %v", err)
		}
	}
	if _, err := g.ReadByteData(0x6F, 0); err == nil {
		t.Fatal("0x6f accepted")
	}
	if len(c.touched) != 0 {
		t.Fatalf("bus reached: %v", c.touched)
	}
}

func TestUnknownFamilyNotDriven(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x72: newENE("AUMA0-E8K4-0101", 8)}, touched: map[byte]bool{}}
	devs, err := NewWithTransport(NewBusTransport(c)).Discover(context.Background())
	if len(devs) != 0 {
		t.Fatalf("unverified family accepted: %v", devs)
	}
	var unknown ErrUnknownStick
	if !errors.As(err, &unknown) || unknown.Addr != 0x72 || unknown.Name != "AUMA0-E8K4-0101" {
		t.Fatalf("error %v", err)
	}
	for reg, v := range c.sticks[0x72].regs {
		if reg >= 0x8000 && v != 0 {
			t.Fatalf("wrote register %#04x", reg)
		}
	}
}

func TestIdentificationMustBeComplete(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x71: newENE("AUDA0-E6K5-0101", 8)}, touched: map[byte]bool{}, badIdent: 0xA7}
	devs, _ := NewWithTransport(NewBusTransport(c)).Discover(context.Background())
	if len(devs) != 0 {
		t.Fatal("15 of 16 identification answers accepted")
	}
}
