package icon

import (
	"encoding/binary"
	"testing"
)

func TestICOHeader(t *testing.T) {
	b := ICO(16, 32)
	if binary.LittleEndian.Uint16(b[2:]) != 1 || binary.LittleEndian.Uint16(b[4:]) != 2 {
		t.Fatalf("bad ICONDIR % x", b[:6])
	}
	size := binary.LittleEndian.Uint32(b[6+8:])
	off := binary.LittleEndian.Uint32(b[6+12:])
	if off != 6+32 || size != 40+16*16*4+4*16 {
		t.Fatalf("entry size=%d off=%d", size, off)
	}
}
