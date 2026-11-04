package app

import (
	"errors"
	"sync"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
)

func TestRingBootFor(t *testing.T) {
	cfg := leds.DefaultConfig()
	cfg.Effect, cfg.Brightness = "solid", 50
	cfg.Params = map[string]leds.Params{"solid": {"color": "#c86432"}}
	cases := []struct {
		name    string
		enabled bool
		mod     func(*leds.Config)
		want    ringBootState
	}{
		{"static color, brightness applied", true, func(*leds.Config) {}, ringBootState{enram.RingBootColor, 100, 50, 25}},
		{"master off", false, func(*leds.Config) {}, ringBootState{mode: enram.RingBootNone}},
		{"device disabled", true, func(c *leds.Config) { c.Devices[ringDeviceID] = leds.DeviceConfig{Disabled: true, Off: true} }, ringBootState{mode: enram.RingBootNone}},
		{"device off", true, func(c *leds.Config) { c.Devices[ringDeviceID] = leds.DeviceConfig{Off: true} }, ringBootState{mode: enram.RingBootOff}},
		{"effect", true, func(c *leds.Config) { c.Effect = "rainbow" }, ringBootState{mode: enram.RingBootNone}},
		{"design accent", true, func(c *leds.Config) { c.Params["solid"] = leds.Params{"color": "#ffffff", "useAccent": true} }, ringBootState{mode: enram.RingBootNone}},
	}
	for _, c := range cases {
		cc := cfg
		cc.Devices = map[string]leds.DeviceConfig{}
		cc.Params = map[string]leds.Params{"solid": {"color": "#c86432"}}
		c.mod(&cc)
		if got := ringBootFor(c.enabled, cc); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestRingBootNotifierDebouncesAndLatestWins(t *testing.T) {
	var mu sync.Mutex
	cur := ringBootState{mode: enram.RingBootOff}
	var sent []ringBootState
	n := &ringBootNotifier{
		wait:  80 * time.Millisecond,
		state: func() ringBootState { mu.Lock(); defer mu.Unlock(); return cur },
		send: func(m string, r, g, b uint8) error {
			mu.Lock()
			sent = append(sent, ringBootState{m, r, g, b})
			mu.Unlock()
			return enram.ErrUnavailable // not installed: silent
		},
	}
	set := func(s ringBootState) { mu.Lock(); cur = s; mu.Unlock(); n.poke() }
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(sent) }
	n.poke() // start
	time.Sleep(30 * time.Millisecond)
	if count() != 1 {
		t.Fatalf("first report: %d", count())
	}
	set(ringBootState{mode: enram.RingBootNone})
	set(ringBootState{enram.RingBootColor, 1, 2, 3})
	set(ringBootState{enram.RingBootColor, 4, 5, 6})
	time.Sleep(30 * time.Millisecond)
	if count() != 1 {
		t.Fatal("sent inside the wait")
	}
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 || sent[1] != (ringBootState{enram.RingBootColor, 4, 5, 6}) {
		t.Fatalf("sent %+v", sent)
	}
}

func TestRingBootNotifierSkipsUnchangedAndLogsOnce(t *testing.T) {
	calls := 0
	n := &ringBootNotifier{
		wait:  time.Millisecond,
		state: func() ringBootState { return ringBootState{mode: enram.RingBootNone} },
		send:  func(string, uint8, uint8, uint8) error { calls++; return errors.New("boom") },
	}
	for i := 0; i < 3; i++ {
		n.poke()
		time.Sleep(20 * time.Millisecond)
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
}
