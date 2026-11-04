package lianlitl

import (
	"errors"
	"image/color"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// lightWrites counts the A3 and B0 commands the controller received.
func lightWrites(c *controller) (a3, b0 int) {
	for _, w := range c.writes {
		switch w[offCmd] {
		case cmdFanLight:
			a3++
		case cmdGroupLite:
			b0++
		}
	}
	return
}

func TestOffWritesOnceAndRestoresOnce(t *testing.T) {
	d, c := newTest(t)
	now := time.Unix(1000, 0)
	d.now = func() time.Time { return now }
	configure(t, d, leds.Params{"mode": "hardware", "effect": "wave", "colors": 2.0, "speed": 3.0, "direction": "counter-clockwise"})
	cols := make([]color.RGBA, 14)
	for i := range cols {
		cols[i] = color.RGBA{9, 9, 9, 255}
	}
	for i := 0; i < 5; i++ {
		d.SetFrame(frame(cols...))
	}
	c.writes = nil

	// off: one A3 per fan (7), one B0 per group (3), nothing else
	if err := d.SetOff(true, false); err != nil {
		t.Fatal(err)
	}
	if a3, b0 := lightWrites(c); a3 != 7 || b0 != 3 || len(c.writes) != 10 {
		t.Fatalf("off: A3 %d, B0 %d, all %d", a3, b0, len(c.writes))
	}
	for _, w := range c.writes {
		p := w[offPayload:]
		if p[3] != 0 || p[18] != 1 || p[17] >= 4 || p[2] != 21 || p[4] != 3 || p[19] != 2 {
			t.Fatalf("off payload % x", p[:lightLen])
		}
		if w[offCmd] == cmdFanLight && p[0]&1 != 0 {
			t.Fatalf("sync bit set % x", p[:lightLen])
		}
	}
	// repeated calls and frames while off: silent
	d.SetOff(true, false)
	for i := 0; i < 5; i++ {
		d.SetFrame(frame(cols...))
	}
	if len(c.writes) != 10 {
		t.Fatalf("writes while off: %d", len(c.writes)-10)
	}

	// app restart / resume / rescan: a new handle, state taken over, no write
	c.writes = nil
	d2, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	c.writes = nil
	configure(t, d2, leds.Params{"mode": "hardware", "effect": "wave"})
	d2.SetOff(true, true)
	d2.SetOff(true, false) // already off: nothing
	for i := 0; i < 5; i++ {
		d2.SetFrame(frame(cols...))
		configure(t, d2, leds.Params{"mode": "hardware", "effect": "wave"})
	}
	if len(c.writes) != 0 {
		t.Fatalf("restart wrote %d packets", len(c.writes))
	}

	// back on: too soon is refused without a write, later one set of commands with d[18]=0
	d2.now = func() time.Time { return now }
	d2.offAt = now
	if err := d2.SetOff(false, false); !errors.Is(err, ErrOffTooSoon) || len(c.writes) != 0 {
		t.Fatalf("too soon: %v, %d writes", err, len(c.writes))
	}
	now = now.Add(time.Minute)
	if err := d2.SetOff(false, false); err != nil {
		t.Fatal(err)
	}
	if a3, b0 := lightWrites(c); a3 != 7 || b0 != 3 || len(c.writes) != 10 {
		t.Fatalf("on: A3 %d, B0 %d, all %d", a3, b0, len(c.writes))
	}
	for _, w := range c.writes {
		if p := w[offPayload:]; p[18] != 0 || p[3] != maxLevel {
			t.Fatalf("on payload % x", p[:lightLen])
		}
	}
	// the effect was written by the on command: frames add nothing
	c.writes = nil
	for i := 0; i < 5; i++ {
		d2.SetFrame(frame(cols...))
	}
	if len(c.writes) != 0 {
		t.Fatalf("frames after on wrote %d packets", len(c.writes))
	}
}

// Motherboard-sync mode: the firmware may reset its RAM state after a short
// reconnect, so an off device gets the off set once per takeover, never on
// resume; hardware mode never re-applies.
func TestOffMotherboardReappliedOnTakeoverOnly(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "motherboard"})
	if err := d.SetOff(true, true); err != nil { // app start, configured off
		t.Fatal(err)
	}
	if a3, b0 := lightWrites(c); a3 != 7 || b0 != 3 || len(c.writes) != 10 {
		t.Fatalf("start: A3 %d B0 %d all %d", a3, b0, len(c.writes))
	}
	c.writes = nil
	for i := 0; i < 5; i++ { // resume / rescan without reconnect
		d.SetFrame(frame(make([]color.RGBA, 14)...))
		configure(t, d, leds.Params{"mode": "motherboard"})
	}
	d.SetOff(true, false)
	if len(c.writes) != 0 {
		t.Fatalf("resume wrote %d", len(c.writes))
	}
	// reconnect: a new handle is taken over again
	d2, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	configure(t, d2, leds.Params{"mode": "motherboard"})
	c.writes = nil
	d2.SetOff(true, true)
	if a3, b0 := lightWrites(c); a3 != 7 || b0 != 3 || len(c.writes) != 10 {
		t.Fatalf("reconnect: A3 %d B0 %d all %d", a3, b0, len(c.writes))
	}
}

func TestOffHardwareNotReappliedOnTakeover(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "hardware"})
	d.SetOff(true, true)
	if len(c.writes) != 0 {
		t.Fatalf("hardware start wrote %d", len(c.writes))
	}
}

// Every SetOff that writes needs allowFlashWrites, in every mode; a failed
// Configure leaves no consent behind.
func TestOffNeedsFlashConsentInEveryMode(t *testing.T) {
	for _, mode := range []string{"motherboard", "hardware", "canvas"} {
		d, c := newTest(t)
		full := leds.Params{}
		for _, s := range d.Settings() {
			full[s.Key] = s.Default
		}
		full["mode"] = mode
		d.Configure(full) // allowFlashWrites false: hardware and canvas are refused
		err := d.SetOff(true, false)
		if !errors.Is(err, ErrFlashWrites) || !errors.Is(err, leds.ErrOffBlocked) || len(c.writes) != 0 {
			t.Fatalf("%s: %v, %d writes", mode, err, len(c.writes))
		}
		// the takeover in motherboard mode needs it as well
		d.SetOff(true, true)
		if len(c.writes) != 0 {
			t.Fatalf("%s: takeover wrote", mode)
		}
	}
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "motherboard"})
	full := leds.Params{"mode": "no-such-mode", "effect": "no-such-effect", "allowFlashWrites": true}
	if d.Configure(full) == nil {
		t.Fatal("bad configuration accepted")
	}
	if err := d.SetOff(true, false); !errors.Is(err, ErrFlashWrites) || len(c.writes) != 0 {
		t.Fatalf("stale settings used: %v", err)
	}
}

// A failed attempt counts for the rate limit, too.
func TestOffRateLimitCoversPartialFailure(t *testing.T) {
	d, c := newTest(t)
	now := time.Unix(1000, 0)
	d.now = func() time.Time { return now }
	configure(t, d, leds.Params{"mode": "motherboard"})
	real := d.h
	d.h = &flaky{transport: real, left: 3}
	if err := d.SetOff(true, false); err == nil {
		t.Fatal("expected a write failure")
	}
	d.h = real
	c.writes = nil
	if err := d.SetOff(true, false); !errors.Is(err, ErrOffTooSoon) || len(c.writes) != 0 {
		t.Fatalf("retry right after a partial failure: %v, %d writes", err, len(c.writes))
	}
	now = now.Add(31 * time.Second)
	if err := d.SetOff(true, false); err != nil || len(c.writes) != 10 {
		t.Fatalf("after the limit: %v, %d writes", err, len(c.writes))
	}
}

// flaky fails every write after the first left ones.
type flaky struct {
	transport
	left int
}

func (f *flaky) Write(r []byte, to time.Duration) error {
	if f.left <= 0 {
		return errors.New("gone")
	}
	f.left--
	return f.transport.Write(r, to)
}

func TestOffPassiveNeverWrites(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "passive"})
	if err := d.SetOff(true, false); !errors.Is(err, ErrPassive) || len(c.writes) != 0 {
		t.Fatalf("passive: %v", err)
	}
}

// Commands that replace the stored fan lights clear the off state; it is
// written once more when the gates allow, otherwise the flag waits. Blink and
// probe commands do not count.
func TestOffReappliedAfterSyncReset(t *testing.T) {
	d, c := newTest(t)
	now := time.Unix(1000, 0)
	d.now = func() time.Time { return now }
	configure(t, d, leds.Params{"mode": "motherboard"})
	if err := d.SetOff(true, false); err != nil {
		t.Fatal(err)
	}
	// blink and probe: nothing replaced, nothing re-applied (even after the limit)
	now = now.Add(time.Minute)
	c.writes = nil
	d.BlinkPort(1)
	d.Probe(0xA1, nil)
	if a3, b0 := lightWrites(c); a3 != 0 || b0 != 0 {
		t.Fatalf("blink/probe re-applied: A3 %d B0 %d", a3, b0)
	}
	// regrouping inside the rate limit: the flag stays, nothing is written twice
	now = now.Add(-50 * time.Second)
	c.writes = nil
	if err := d.SetupGroups(1); err != nil {
		t.Fatal(err)
	}
	if a3, b0 := lightWrites(c); a3 != 0 || b0 != 0 || !d.offClobbered {
		t.Fatalf("limit ignored: A3 %d B0 %d clobbered %v", a3, b0, d.offClobbered)
	}
	// next call after the limit settles it, once
	now = now.Add(time.Minute)
	c.writes = nil
	if err := d.SetupGroups(1); err != nil {
		t.Fatal(err)
	}
	a3, b0 := lightWrites(c)
	if a3 != 7 || b0 != 3 || d.offClobbered {
		t.Fatalf("A3 %d B0 %d clobbered %v", a3, b0, d.offClobbered)
	}
	// without consent it never writes
	now = now.Add(time.Minute)
	d.flashOK = false
	c.writes = nil
	d.mu.Lock()
	d.setSync(true)
	d.settleOff()
	d.mu.Unlock()
	if a3, b0 := lightWrites(c); a3 != 3 || b0 != 0 || !d.offClobbered {
		t.Fatalf("no consent: A3 %d B0 %d clobbered %v", a3, b0, d.offClobbered)
	}
	// frames add nothing while off
	c.writes = nil
	d.SetFrame(frame(make([]color.RGBA, 14)...))
	if len(c.writes) != 0 {
		t.Fatal("frame while off wrote")
	}
}
