package enram

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Ring boot state: what the 8.8" LED ring shows right after Windows starts,
// before the GUI runs. The sensor service stores it (ProgramData, admin-only
// file) and applies it once at its own start.
//
// Intended use (wired by the app, not here): whenever the user's ring state
// changes, call SetRingBootState:
//
//	ring off              -> SetRingBootState(RingBootOff, 0, 0, 0)
//	static color          -> SetRingBootState(RingBootColor, r, g, b)
//	any effect / unknown  -> SetRingBootState(RingBootNone, 0, 0, 0)
//
// "none" leaves the ring alone at boot (the firmware rainbow keeps running).
// The call uses its own always-on pipe (RingPipe), independent of the smbusBridge
// setting; the service accepts nothing but this request there. The server is
// verified like the SMBus bridge (owned by SYSTEM/Administrators, image is the
// service of this installation). It fails with ErrUnavailable if the service is
// not running or not trusted.
const RingPipe = `\\.\pipe\CaseHUD.Ring`

const (
	RingBootNone  = "none"
	RingBootOff   = "off"
	RingBootColor = "color"
)

// ringBootRequest is the wire format: nothing but the mode and the color.
type ringBootRequest struct {
	V    int    `json:"v"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	R    uint8  `json:"r"`
	G    uint8  `json:"g"`
	B    uint8  `json:"b"`
}

// SetRingBootState sends the boot state to the sensor service over the bridge
// pipe. For "off" and "none" the color is ignored (sent as zeros).
func SetRingBootState(mode string, r, g, b uint8) error {
	pt := NewPipeTransport(RingPipe)
	defer pt.Close()
	return pt.SetRingBootState(mode, r, g, b)
}

// SetRingBootState sends the boot state over this transport.
func (p *PipeTransport) SetRingBootState(mode string, r, g, b uint8) error {
	switch mode {
	case RingBootColor:
	case RingBootOff, RingBootNone:
		r, g, b = 0, 0, 0
	default:
		return fmt.Errorf("enram: ring boot mode %q (none, off, color)", mode)
	}
	req, err := json.Marshal(ringBootRequest{V: 1, Type: "setRingBoot", Mode: mode, R: r, G: g, B: b})
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	var resp *bridgeResponse
	for attempt := 0; ; attempt++ {
		reused := p.conn != nil
		if !reused {
			if p.conn, err = p.dial(); err != nil {
				p.conn = nil
				return fmt.Errorf("enram: bridge: %w: %w", ErrUnavailable, err)
			}
		}
		resp, _, err = p.roundTripBounded(req)
		if err == nil {
			break
		}
		p.conn.Close()
		p.conn = nil
		// Setting the same state twice is harmless, so after a dropped
		// (e.g. idle-closed) connection one retry is fine.
		if !reused || attempt > 0 {
			return fmt.Errorf("enram: bridge: %w", err)
		}
	}
	if resp.OK {
		return nil
	}
	switch resp.Error {
	case "timeout":
		return ErrBusy
	case "denied":
		return ErrDenied
	default:
		return &IOError{errors.New(resp.Error)}
	}
}
