package enram

import (
	"errors"
	"fmt"
)

// Op is one ENE operation inside a batch. The JSON form is the wire format of
// the CaseHUD SMBus bridge.
type Op struct {
	Op   string `json:"op"`             // "rb", "wb", "blk", "rcmd"
	Reg  uint16 `json:"reg,omitempty"`  // ENE register (rb, wb, blk)
	Val  byte   `json:"val,omitempty"`  // wb
	Data []byte `json:"data,omitempty"` // blk (base64 in JSON), at most 32 bytes
	Cmd  byte   `json:"cmd,omitempty"`  // rcmd: raw SMBus read byte, 0xA0..0xAF only
}

// Op kinds.
const (
	OpReadReg  = "rb"   // set register pointer, read byte (command 0x81)
	OpWriteReg = "wb"   // set register pointer, write byte (command 0x01)
	OpBlock    = "blk"  // set register pointer, block write (command 0x03)
	OpReadCmd  = "rcmd" // read byte data with a raw command (ENE identification registers)
)

const (
	maxOps      = 64
	maxBlock    = 32
	maxLockMs   = 50
	identCmdMin = 0xA0
	identCmdMax = 0xAF
)

// Transport executes a batch of operations on one stick atomically (under the
// SMBus mutex) and returns the bytes read by rb/rcmd ops in order.
// It returns ErrBusy if the bus could not be locked within lockMs.
type Transport interface {
	Batch(addr byte, lockMs uint32, ops []Op) ([]byte, error)
}

// Validate checks a batch against the rules every transport (and the CaseHUD
// bridge service) enforces.
func Validate(addr byte, lockMs uint32, ops []Op) error {
	if err := checkAddr(addr); err != nil {
		return err
	}
	if lockMs < 1 || lockMs > maxLockMs {
		return fmt.Errorf("enram: lockMs %d (1..%d)", lockMs, maxLockMs)
	}
	if len(ops) == 0 || len(ops) > maxOps {
		return fmt.Errorf("enram: %d ops (1..%d)", len(ops), maxOps)
	}
	for i, op := range ops {
		switch op.Op {
		case OpReadReg, OpWriteReg:
		case OpBlock:
			if len(op.Data) == 0 || len(op.Data) > maxBlock {
				return fmt.Errorf("enram: op %d: block of %d bytes", i, len(op.Data))
			}
		case OpReadCmd:
			if op.Cmd < identCmdMin || op.Cmd > identCmdMax {
				return fmt.Errorf("enram: op %d: raw command %#02x not allowed", i, op.Cmd)
			}
		default:
			return fmt.Errorf("enram: op %d: unknown op %q", i, op.Op)
		}
	}
	return nil
}

// busTransport runs batches directly on an SMBus (test buses).
type busTransport struct{ bus Bus }

// NewBusTransport executes batches on b, behind the address guard.
func NewBusTransport(b Bus) Transport { return busTransport{guarded{b}} }

func (t busTransport) Batch(addr byte, lockMs uint32, ops []Op) ([]byte, error) {
	if err := Validate(addr, lockMs, ops); err != nil {
		return nil, err
	}
	if err := t.bus.Lock(lockMs); err != nil {
		return nil, err
	}
	defer t.bus.Unlock()

	var reads []byte
	setReg := func(reg uint16) error {
		return t.bus.WriteWordData(addr, cmdRegister, reg<<8|reg>>8)
	}
	for _, op := range ops {
		var err error
		switch op.Op {
		case OpReadCmd:
			var v byte
			if v, err = t.bus.ReadByteData(addr, op.Cmd); err == nil {
				reads = append(reads, v)
			}
		case OpReadReg:
			if err = setReg(op.Reg); err == nil {
				var v byte
				if v, err = t.bus.ReadByteData(addr, cmdRead); err == nil {
					reads = append(reads, v)
				}
			}
		case OpWriteReg:
			if err = setReg(op.Reg); err == nil {
				err = t.bus.WriteByteData(addr, cmdWrite, op.Val)
			}
		case OpBlock:
			if err = setReg(op.Reg); err == nil {
				err = t.bus.WriteBlockData(addr, cmdBlock, op.Data)
			}
		}
		if err != nil {
			return reads, &IOError{err}
		}
	}
	return reads, nil
}

// IOError is an SMBus transfer failure (device did not answer, ...).
type IOError struct{ Err error }

func (e *IOError) Error() string { return "enram: io: " + e.Err.Error() }
func (e *IOError) Unwrap() error { return e.Err }

func isIO(err error) bool {
	var io *IOError
	return errors.As(err, &io)
}
