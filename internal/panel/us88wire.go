package panel

import (
	"crypto/cipher"
	"crypto/des"
	"encoding/binary"
	"time"
)

// Wire format of the Universal Screen 8.8" (WinUSB 1CBE:A088).
//
// Every command is a 512-byte block, optionally followed directly by data in
// the same transfer:
//
//	plaintext (504 bytes)   [0] op, [1] 0, [2:4] 0x1A 0x6D, [4:8] stamp (u32 LE),
//	                        [8:500] arguments, [500:504] PKCS#7 padding (4 x 0x04)
//	block                   DES-CBC(plaintext), 6 zero bytes, 0xA1 0x1A
//
// The key and the IV are the same 8 ASCII bytes.

// usOp is the operation byte of a command.
type usOp byte

const (
	usOpInfo      usOp = 0x0A // firmware version query
	usOpFPS       usOp = 0x0F // frame-rate hint (SL V3 LCD only)
	usOpTurn      usOp = 0x0D // device-side rotation (we only ever send 0)
	usOpBacklight usOp = 0x0E // backlight 0..usMaxBacklight
	usOpClock     usOp = 0x33 // set the device time (flag 2) or switch the offline clock (flag 1/0)
	usOpClockOff  usOp = 0x34 // hide the built-in clock
	usOpPicture   usOp = 0x65 // JPEG picture, length (u32 BE) as argument
	usOpOverlay   usOp = 0x66 // PNG layer drawn above the picture
	usOpVideo     usOp = 0x79 // block of an H.264 stream, length (u32 BE) as argument
	usOpQueue     usOp = 0x7A // report how many pictures are queued
	usOpHalt      usOp = 0x7B // stop video playback
	usOpHandOver  usOp = 0x7F // give the display back to the firmware's own theme (Flex LCD)
	usOpCheck     usOp = 0x80 // "new LCD" query (SL V3 and Flex LCD)
)

const (
	usBlockLen     = 512
	usPlainLen     = 504
	usArgsLen      = 492 // plaintext bytes 8..500
	usMaxBacklight = 50  // the device's backlight range is 0..50
)

var usKey = []byte("slv3tuzx")

// SL V3 / TL V2 framing: the whole 504-byte plaintext is block-padded, which
// adds a full block, so the ciphertext is 512 bytes and there is no trailer.
// A command with data always goes out as one transfer of slTransferLen bytes.
const (
	slArgsLen     = 496 // plaintext bytes 8..504
	slTransferLen = 102400
	slMaxJPEG     = 100000
)

// sealSL produces the 512-byte block of the SL V3 framing for c.
func (c usCommand) sealSL() []byte {
	plain := make([]byte, usBlockLen)
	plain[0] = byte(c.Op)
	plain[2], plain[3] = 0x1A, 0x6D
	binary.LittleEndian.PutUint32(plain[4:8], c.Stamp)
	copy(plain[8:8+slArgsLen], c.Args)
	for i := usPlainLen; i < usBlockLen; i++ {
		plain[i] = 8
	}
	blk, _ := des.NewCipher(usKey)
	cipher.NewCBCEncrypter(blk, usKey).CryptBlocks(plain, plain)
	return plain
}

// usCommand is one command before it is sealed.
type usCommand struct {
	Op    usOp
	Stamp uint32
	Args  []byte
}

// seal produces the 512-byte block for c.
func (c usCommand) seal() []byte {
	plain := make([]byte, usPlainLen)
	plain[0] = byte(c.Op)
	plain[2], plain[3] = 0x1A, 0x6D
	binary.LittleEndian.PutUint32(plain[4:8], c.Stamp)
	copy(plain[8:8+usArgsLen], c.Args)
	for i := usPlainLen - 4; i < usPlainLen; i++ {
		plain[i] = 4
	}
	blk, _ := des.NewCipher(usKey)
	cipher.NewCBCEncrypter(blk, usKey).CryptBlocks(plain, plain)
	out := make([]byte, usBlockLen)
	copy(out, plain)
	out[510], out[511] = 0xA1, 0x1A
	return out
}

// usClock hands out the stamps for commands: milliseconds since the first
// use, and strictly increasing even for commands issued within one
// millisecond (the device ignores a stamp it has already seen).
type usClock struct {
	origin time.Time
	last   uint32
}

func (c *usClock) next() uint32 {
	if c.origin.IsZero() {
		c.origin = time.Now()
	}
	t := uint32(time.Since(c.origin).Milliseconds())
	if t <= c.last {
		t = c.last + 1
	}
	c.last = t
	return t
}

// Timing and thresholds. These are empirical values (tuned on a device), not
// protocol constants.
const (
	// usBusyAbove / usIdleAt: when an acknowledgement reports more than
	// usBusyAbove queued pictures, wait (polling the queue) until at most
	// usIdleAt remain; this keeps the queue short without stalling the stream.
	usBusyAbove = 3
	usIdleAt    = 2
)

// usSettle: pause after each of the first wake-up commands, chosen so the
// device has settled before the next one arrives (a variable so tests can
// skip the waiting).
var usSettle = 150 * time.Millisecond
