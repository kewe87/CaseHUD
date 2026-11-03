// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"errors"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// DisclaimerVersion is the version of the disclaimer text (installer/disclaimer-*.txt,
// README, first-start dialog). Raise it when the text changes in substance:
// everyone is asked again and CaseHUD sends nothing to a device until they agree.
const DisclaimerVersion = 3

// ErrAckRequired is returned when a switch needs a one-time confirmation first.
var ErrAckRequired = errors.New("confirmation required")

// DisclaimerAck records that the disclaimer was accepted.
type DisclaimerAck struct {
	Version int    `json:"version"`
	At      string `json:"at,omitempty"` // RFC 3339
}

// Acks tells the GUI which confirmations are on record.
type Acks struct {
	Version         int    `json:"version"` // current disclaimer version
	Accepted        bool   `json:"accepted"`
	AcceptedAt      string `json:"acceptedAt,omitempty"`
	AckExperimental bool   `json:"ackExperimental"`
	AckLighting     bool   `json:"ackLighting"`
	// AckCapture: the user confirmed that the screen and audio effects may read
	// the screen content and the audio output.
	AckCapture bool `json:"ackCapture"`
}

// Acks returns the confirmations on record.
func (e *Engine) Acks() Acks {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Acks{Version: DisclaimerVersion, Accepted: e.cfg.Disclaimer.Version >= DisclaimerVersion,
		AcceptedAt: e.cfg.Disclaimer.At, AckExperimental: e.cfg.AckExperimental, AckLighting: e.cfg.AckLighting, AckCapture: e.cfg.AckCapture}
}

// DisclaimerAccepted reports whether the current disclaimer was accepted.
func (e *Engine) DisclaimerAccepted() bool { return e.Acks().Accepted }

// outputAllowed is the gate for everything that talks to a device: panels,
// read-only sensor devices, lighting and AWTRIX clocks. The demo has no
// devices, so it is never held back.
func (e *Engine) outputAllowed() bool {
	return e.Demo || e.DisclaimerAccepted()
}

// AcceptDisclaimer records the acceptance of version v (the current one) and
// starts the device output that was held back.
func (e *Engine) AcceptDisclaimer(v int) error {
	if v != DisclaimerVersion {
		return errors.New("this is not the current version of the disclaimer")
	}
	e.mu.Lock()
	was := e.cfg.Disclaimer.Version >= DisclaimerVersion
	if !was {
		e.cfg.Disclaimer = DisclaimerAck{Version: DisclaimerVersion, At: e.Clock().UTC().Format(time.RFC3339)}
	}
	e.mu.Unlock()
	if was {
		return nil
	}
	// Output starts even when saving fails (the answer is still given); the
	// error is reported so the GUI can warn that it may be asked again.
	perr := e.persist()
	e.startOutput()
	return perr
}

// Acknowledge records a one-time confirmation: "experimental", "lighting" or
// "capture" (screen and audio effects).
func (e *Engine) Acknowledge(what string) error {
	e.mu.Lock()
	switch what {
	case "experimental":
		e.cfg.AckExperimental = true
	case "lighting":
		e.cfg.AckLighting = true
	case "capture":
		e.cfg.AckCapture = true
	default:
		e.mu.Unlock()
		return errors.New("unknown confirmation")
	}
	e.mu.Unlock()
	return e.persist()
}

// RevokeCapture withdraws the confirmation for the screen and audio effects:
// a running capture effect is replaced by the default effect (which stops the
// capture) and the withdrawal is saved. Selecting a capture effect needs the
// confirmation again.
func (e *Engine) RevokeCapture() error {
	e.mu.Lock()
	e.cfg.AckCapture = false
	e.mu.Unlock()
	if fx, _ := e.currentLighting(); isCaptureEffect(fx) {
		def := leds.DefaultConfig().Effect
		if _, err := e.PatchLighting(LightingPatch{Effect: &def}); err != nil {
			return err
		}
	}
	return e.persist()
}

// startOutput starts what Start held back until the disclaimer is accepted.
// It runs once.
func (e *Engine) startOutput() {
	e.mu.Lock()
	if e.outputStarted || e.ctx == nil || e.ctx.Err() != nil {
		e.mu.Unlock()
		return
	}
	e.outputStarted = true
	e.mu.Unlock()
	e.Rescan()
	if !e.Demo {
		go e.runWatch(e.ctx)
	}
	e.resumeLighting()
	select {
	case e.awWake <- struct{}{}:
	default:
	}
}
