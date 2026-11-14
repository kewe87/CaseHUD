package app

import (
	"context"
	"hash/crc32"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// The simulated fans of a demo run, so that the calibration can be tried without
// hardware. They behave like the real ones in what matters to it: a fan turns
// down to a duty and stops under it, needs more to start than to keep turning,
// twitches below that, and a stop that is not sent again ends after a few
// seconds. The pump loses its speed under a duty. Time is simulated and runs
// demoSpeedup times faster than the clock, so a run takes seconds.
const demoSpeedup = 120

type demoFan struct {
	pump      bool
	perDuty   int
	stopBelow int // fan: it stops under this duty; pump: its speed collapses under it
	startAt   int
	twitchAt  int
	never     bool          // it keeps turning at 0 %
	byItself  time.Duration // a stop not sent again within this time ends

	on        bool
	duty      int
	lastZero  time.Duration
	twitchEnd time.Duration
}

type demoFans struct {
	mu   sync.Mutex
	now  time.Duration
	fans map[string]*demoFan
	pump map[string]bool
}

// declare tells the rig what kind of output id is, before it is first used.
func (d *demoFans) declare(o coolingOutputConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pump == nil {
		d.pump = map[string]bool{}
	}
	d.pump[o.ID] = o.Pump
}

func (e *Engine) demoRig() *demoFans {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	if e.coolDemo == nil {
		e.coolDemo = &demoFans{fans: map[string]*demoFan{}}
	}
	return e.coolDemo
}

// demoModel gives each fan its own behaviour. The three fans of the first port
// are fixed (one that stops and starts, one that never stops, one that needs a
// lot to start); others get theirs from their name.
func demoModel(id string, pump bool) *demoFan {
	switch id {
	case "port1-fan1":
		return &demoFan{perDuty: 24, stopBelow: 12, startAt: 16, twitchAt: 13, byItself: 8 * time.Second}
	case "port1-fan2":
		return &demoFan{perDuty: 22, never: true, startAt: 1}
	case "port1-fan3":
		return &demoFan{perDuty: 26, stopBelow: 14, startAt: 22, twitchAt: 18, byItself: 8 * time.Second}
	}
	if pump {
		return &demoFan{pump: true, perDuty: 45, stopBelow: 42, on: true}
	}
	h := crc32.ChecksumIEEE([]byte(id))
	stop := 12 + int(h%5)
	return &demoFan{perDuty: 20 + int(h/7%9), stopBelow: stop, startAt: stop + 4 + int(h/11%6), twitchAt: stop + 2, byItself: 8 * time.Second}
}

func (d *demoFans) fan(id string) *demoFan {
	f := d.fans[id]
	if f == nil {
		f = demoModel(id, d.pump[id])
		f.on = true
		f.duty = 100
		d.fans[id] = f
	}
	return f
}

// LowestDuty is the same as on the real controller, so that the calibration
// leaves out the duties below it here too.
func (d *demoFans) LowestDuty() int { return lianlitl.LowestDuty }

func (d *demoFans) TakeControl(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fan(id)
	return nil
}

func (d *demoFans) SetDuty(id string, p int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	f := d.fan(id)
	if p > 0 && p < lianlitl.LowestDuty && !f.pump {
		p = lianlitl.LowestDuty // like the controller: a smaller duty is not a speed of its own
	}
	f.duty = p
	switch {
	case f.pump:
	case p == 0 && !f.never:
		f.on, f.lastZero = false, d.now
	case f.on && p < f.stopBelow && !f.never:
		f.on = false
	case !f.on && p >= f.startAt:
		f.on = true
	case !f.on && f.twitchAt > 0 && p >= f.twitchAt:
		f.twitchEnd = d.now + 3*time.Second
	}
	return nil
}

func (d *demoFans) Speeds() (map[string]int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]int, len(d.fans))
	for id, f := range d.fans {
		out[id] = f.rpm(d.now)
	}
	return out, nil
}

func (f *demoFan) rpm(now time.Duration) int {
	switch {
	case f.pump:
		if f.duty < f.stopBelow {
			return 0
		}
		return f.duty * f.perDuty
	case f.on && f.duty > 0:
		return f.duty * f.perDuty
	case f.on:
		return 12 * f.perDuty
	case f.byItself > 0 && f.duty == 0 && now-f.lastZero >= f.byItself:
		return 12 * f.perDuty
	case now < f.twitchEnd:
		return 300
	}
	return 0
}

// wait advances the simulated time by d and sleeps d/demoSpeedup.
func (d *demoFans) wait(ctx context.Context, dur time.Duration) error {
	d.mu.Lock()
	d.now += dur
	d.mu.Unlock()
	t := time.NewTimer(dur / demoSpeedup)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
