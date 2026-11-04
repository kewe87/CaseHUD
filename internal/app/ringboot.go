package app

import (
	"errors"
	"log"
	"sync"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
)

// Boot state of the 8.8" LED ring: what the ring shows after Windows starts,
// before CaseHUD runs. The sensor service keeps it; the app only reports
// changes (debounced, in the background, never from the engine's path).

// ringBootWait is the least time between two reports; the latest state wins.
const ringBootWait = 5 * time.Second

// ringBootSend reports a state to the sensor service (replaced in tests).
var ringBootSend = enram.SetRingBootState

type ringBootState struct {
	mode    string
	r, g, b uint8
}

// ringBootFor derives the boot state from the lighting configuration:
// disabled lighting or device: none; Off: off; a single static color: that
// color (as the ring shows it, brightness applied); anything else: none.
func ringBootFor(enabled bool, cfg leds.Config) ringBootState {
	none := ringBootState{mode: enram.RingBootNone}
	dc := cfg.Devices[ringDeviceID]
	if !enabled || dc.Disabled {
		return none
	}
	if dc.Off {
		return ringBootState{mode: enram.RingBootOff}
	}
	if cfg.Effect != "solid" {
		return none
	}
	p := cfg.Params["solid"]
	if p.Bool("useAccent") {
		return none
	}
	c, err := leds.ParseHexColor(p.String("color"))
	if err != nil {
		return none
	}
	k := max(0, min(100, cfg.Brightness))
	scale := func(v uint8) uint8 { return uint8(int(v) * k / 100) }
	return ringBootState{mode: enram.RingBootColor, r: scale(c.R), g: scale(c.G), b: scale(c.B)}
}

// ringBootNotifier sends the state at most once per wait, trailing: a change
// that comes in meanwhile is picked up when the timer fires, so the latest
// state wins. poke never blocks.
type ringBootNotifier struct {
	state func() ringBootState
	send  func(mode string, r, g, b uint8) error
	wait  time.Duration

	mu     sync.Mutex
	timer  *time.Timer
	sentAt time.Time
	last   *ringBootState
	warned bool
}

func (n *ringBootNotifier) poke() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.timer != nil {
		return
	}
	delay := time.Duration(0)
	if !n.sentAt.IsZero() {
		delay = max(0, n.wait-time.Since(n.sentAt))
	}
	n.timer = time.AfterFunc(delay, n.flush)
}

func (n *ringBootNotifier) flush() {
	n.mu.Lock()
	n.timer = nil
	n.mu.Unlock()
	s := n.state()
	n.mu.Lock()
	if n.last != nil && *n.last == s {
		n.mu.Unlock()
		return
	}
	n.last = &s
	n.sentAt = time.Now()
	n.mu.Unlock()
	err := n.send(s.mode, s.r, s.g, s.b)
	if err == nil || errors.Is(err, enram.ErrUnavailable) {
		return // not installed: nothing to tell
	}
	n.mu.Lock()
	first := !n.warned
	n.warned = true
	n.mu.Unlock()
	if first {
		log.Printf("lighting: ring boot state not stored: %v", err)
	}
}

// ringBootPoke reports that the state of the ring may have changed.
func (e *Engine) ringBootPoke() {
	if e.Demo || !e.outputAllowed() {
		return
	}
	l := &e.light
	l.rbOnce.Do(func() {
		l.rb = &ringBootNotifier{state: e.currentRingBoot, send: ringBootSend, wait: ringBootWait}
	})
	l.rb.poke()
}

// currentRingBoot reads the configuration (locks are taken here, in the
// notifier's own goroutine).
func (e *Engine) currentRingBoot() ringBootState {
	enabled, _ := e.lightingFlags()
	l := &e.light
	l.mu.Lock()
	cfg := l.cfg
	if l.eng != nil {
		cfg = l.eng.Config()
	}
	l.mu.Unlock()
	return ringBootFor(enabled, cfg)
}
