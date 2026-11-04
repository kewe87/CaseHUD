package lianli

import (
	"fmt"
	"time"
)

// Firmware version: a 64-byte request with byte 0 = 0x10 and the rest zero is
// answered with byte 0 = 0x10, byte 1 = major (never 0 in a valid answer) and
// byte 2 = minor. The version is shown as "major_minor". The query reads
// only; it is made once, right after the device was opened.
const (
	cmdVersion     = 0x10
	versionTimeout = 150 * time.Millisecond
)

// reader is implemented by transports that can read the bulk IN pipe.
type reader interface {
	Read(buf []byte, timeout time.Duration) (int, error)
}

// readVersion asks an open transport for its firmware version. Any failure
// (no IN pipe, no answer, odd answer) gives "" without a message.
func readVersion(t transport) string {
	r, ok := t.(reader)
	if !ok {
		return ""
	}
	req := make([]byte, packetLen)
	req[0] = cmdVersion
	if t.Write(req) != nil {
		return ""
	}
	buf := make([]byte, packetLen)
	n, err := r.Read(buf, versionTimeout)
	if err != nil || n < 3 {
		return ""
	}
	return parseVersion(buf[:n])
}

func parseVersion(b []byte) string {
	if len(b) < 3 || b[0] != cmdVersion || b[1] == 0 {
		return ""
	}
	return fmt.Sprintf("%d_%d", b[1], b[2])
}

// Firmware implements leds.FirmwareReporter. It is known once the ring was
// opened (first frame); "" before that or when the ring did not answer.
func (d *device) Firmware() string {
	d.fwMu.Lock()
	defer d.fwMu.Unlock()
	return d.fw
}

func (d *device) setFirmware(v string) {
	d.fwMu.Lock()
	d.fw = v
	d.fwMu.Unlock()
}
