package lianlitl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	cmdProductInfo = 0xA6
	infoTimeout    = 200 * time.Millisecond
)

// FanInfo describes one fan as the controller reports it.
type FanInfo struct {
	Port    int    `json:"port"`  // 1..4
	Index   int    `json:"index"` // position in the chain, 1..
	RPM     int    `json:"rpm"`
	Model   string `json:"model,omitempty"` // model code from the fan firmware, "" if unknown
	LCD     bool   `json:"lcd"`             // TL LCD fan (has a screen)
	Reverse bool   `json:"reverse"`         // reverse-blade variant
	SizeMM  int    `json:"sizeMm,omitempty"`
}

// Topology is the wiring of one TL controller.
type Topology struct {
	DeviceID string    `json:"deviceId"`
	Fans     []FanInfo `json:"fans"` // sorted by port, index
}

// Port limits of the controller (manufacturer FAQ): port 1 takes no TL LCD
// fans; a port takes at most 10 TL or 3 TL LCD fans.
const (
	MaxFansPerPort    = 10
	MaxLCDFansPerPort = 3
	NoLCDPort         = 1
)

// QueryTopology reads the wiring of every TL controller without an engine:
// the A1 status query plus one product-info query (A6) per fan. Both are
// read-only; the lighting and fan speeds are not touched.
func QueryTopology(ctx context.Context) ([]Topology, error) {
	devs, err := New().Discover(ctx)
	if err != nil && len(devs) == 0 {
		return nil, err
	}
	var out []Topology
	var errs []error
	for _, dv := range devs {
		d := dv.(*device)
		t, err := d.Topology()
		if err != nil {
			errs = append(errs, err)
		} else {
			out = append(out, t)
		}
		d.Close()
	}
	return out, errors.Join(errs...)
}

// Topology returns the wiring of this controller (see QueryTopology).
func (d *device) Topology() (Topology, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fans, err := d.handshake() // fresh RPM
	if err != nil {
		return Topology{}, err
	}
	rpm := map[fan]int{}
	if len(d.raw) > offLen {
		for i := 0; i < int(d.raw[offLen])/3 && offPayload+3*i+2 < len(d.raw); i++ {
			e := d.raw[offPayload+3*i:]
			rpm[fan{port: int(e[0]>>4) & 3, index: int(e[0] & 0x0F)}] = int(e[1])<<8 | int(e[2])
		}
	}
	t := Topology{DeviceID: d.ID()}
	for _, f := range fans {
		fi := FanInfo{Port: f.port + 1, Index: f.index + 1, RPM: rpm[f]}
		if model, err := d.productModel(f); err == nil {
			fi.Model = model
			fi.LCD, fi.Reverse, fi.SizeMM = classify(model)
		}
		t.Fans = append(t.Fans, fi)
	}
	return t, nil
}

// productModel asks a fan for its firmware info and returns modelB.
func (d *device) productModel(f fan) (string, error) {
	info, err := d.productInfo(1, byte(f.port<<4|f.index&0x0F))
	if err != nil {
		return "", fmt.Errorf("lianlitl: product info %d/%d: %w", f.port+1, f.index+1, err)
	}
	return info[4], nil
}

// Firmware is the controller's firmware version in the form the manufacturer
// software shows ("0.62").
func (d *device) Firmware() string { return d.fw }

// firmwareVersion turns the version field "V1.00.62" into "0.62" (second and
// third number); other forms are returned without the leading "V".
func firmwareVersion(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "V")
	parts := strings.Split(s, ".")
	if len(parts) == 3 {
		minor, err1 := strconv.Atoi(parts[1])
		build, err2 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil {
			return fmt.Sprintf("%d.%d", minor, build)
		}
	}
	return s
}

// productInfo sends the read-only product-info query A6 [target, addr]
// (target 0 = the controller, 1 = the fan at addr = port<<4|fan). The answer
// comes in two reports; the first holds "chip,boot,type,modelA,modelB,
// version", which is returned split and trimmed.
func (d *device) productInfo(target, addr byte) ([]string, error) {
	buf := make([]byte, d.inLen)
	for i := 0; i < 16; i++ { // drain answers of other commands
		if _, err := d.h.Read(buf, 5*time.Millisecond); err != nil {
			break
		}
	}
	if err := d.h.Write(packet(cmdProductInfo, []byte{target, addr}), ioTimeout); err != nil {
		return nil, err
	}
	for tries := 0; tries < 6; tries++ {
		n, err := d.h.Read(buf, infoTimeout)
		if err != nil {
			return nil, err
		}
		p := buf[:n]
		if n <= offPayload+3 || p[0] != reportID || p[offCmd] != cmdProductInfo ||
			p[offPayload] != target || p[offPayload+1] != addr || p[offPayload+2] != 1 {
			continue // second report (date) or a foreign answer
		}
		s := string(p[offPayload+3:])
		if i := strings.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		parts := strings.Split(strings.TrimSpace(s), ",")
		if len(parts) < 6 {
			return nil, fmt.Errorf("lianlitl: product info %q", s)
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts, nil
	}
	return nil, errors.New("lianlitl: no product info answer")
}

// classify derives the fan type from the model code: TLFC = LCD reverse,
// TLF = reverse, TLC = LCD, TL = standard; the two digits after the prefix are
// the size (12 = 120 mm, 14 = 140 mm).
func classify(model string) (lcd, reverse bool, size int) {
	var rest string
	switch {
	case strings.HasPrefix(model, "TLFC"):
		lcd, reverse, rest = true, true, model[4:]
	case strings.HasPrefix(model, "TLF"):
		reverse, rest = true, model[3:]
	case strings.HasPrefix(model, "TLC"):
		lcd, rest = true, model[3:]
	case strings.HasPrefix(model, "TL"):
		rest = model[2:]
	default:
		return false, false, 0
	}
	switch {
	case strings.HasPrefix(rest, "12"):
		size = 120
	case strings.HasPrefix(rest, "14"):
		size = 140
	}
	return lcd, reverse, size
}

// Warnings checks a topology against the port limits.
func (t Topology) Warnings() []string {
	var w []string
	count, lcd := map[int]int{}, map[int]int{}
	for _, f := range t.Fans {
		count[f.Port]++
		if f.LCD {
			lcd[f.Port]++
		}
	}
	for port := 1; port <= 4; port++ {
		if port == NoLCDPort && lcd[port] > 0 {
			w = append(w, fmt.Sprintf("port %d does not support TL LCD fans (%d connected)", port, lcd[port]))
		}
		if lcd[port] > MaxLCDFansPerPort {
			w = append(w, fmt.Sprintf("port %d: %d TL LCD fans, at most %d", port, lcd[port], MaxLCDFansPerPort))
		}
		if count[port] > MaxFansPerPort {
			w = append(w, fmt.Sprintf("port %d: %d fans, at most %d", port, count[port], MaxFansPerPort))
		}
	}
	return w
}
