package app

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/conflicts"
	"github.com/kewe87/casehud/internal/coolguard"
	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// What happens when CaseHUD or the sensor service ends without cleaning up.
//
// Fans on the TL controller keep their last duty when CaseHUD dies. While the
// control is live, a guard process (package coolguard) therefore waits for the end
// of this one and sets the fans to their safe duty if it was not a clean end.

// coolingGuardGrace is how long a clean end of the control waits before the guard
// is told goodbye, so that a control replaced by a new one keeps its guard.
var coolingGuardGrace = time.Second

// coolingGuardOpenWait is how long opening the controller waits for a guard that
// is just setting the fans.
const coolingGuardOpenWait = 5 * time.Second

// coolingCrash is the engine's state for the cases above.
type coolingCrash struct {
	mu       sync.Mutex
	restored bool   // cooling.json was replaced by its safety copy
	kept     string // the name under which the damaged file was kept
	link     *coolguard.Link
	poke     chan struct{}
	running  int  // live loops that started and have not ended
	dirty    bool // the last loop ended without leaving the fans safe
}

func (c *coolingCrash) markRestored(kept string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.restored, c.kept = true, kept
}

func (c *coolingCrash) clearRestored() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.restored, c.kept = false, ""
}

// restoredFile reports whether the safety copy of cooling.json is in use, and the
// name of the damaged file that was kept.
func (c *coolingCrash) restoredFile() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kept, c.restored
}

// guardStarted arms the guard for a live loop. Without a way to start one
// (Engine.GuardSpawn) nothing happens.
func (e *Engine) guardStarted(fans *tlFans, previous *tlFans) {
	if e.GuardSpawn == nil {
		return
	}
	c := &e.crash
	c.mu.Lock()
	defer c.mu.Unlock()
	fans.inheritDuties(previous)
	if c.poke == nil {
		c.poke = make(chan struct{}, 1)
	}
	fans.onChange = func() {
		select {
		case c.poke <- struct{}{}:
		default:
		}
	}
	if c.link == nil {
		c.link = &coolguard.Link{
			Spawn: e.GuardSpawn, Source: e.guardState, Poke: c.poke,
			Log: func(format string, args ...any) { log.Printf("cooling guard: "+format, args...) },
		}
	}
	c.running++
	c.link.Arm()
}

// guardLoopEnded is called when a live loop has ended. If it left every fan at its
// safe duty the guard has nothing to watch for any more.
func (e *Engine) guardLoopEnded(ctrlEnd error) {
	c := &e.crash
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.link == nil {
		return
	}
	c.running = max(c.running-1, 0)
	handed := errors.Is(ctrlEnd, cooling.ErrDetached)
	c.dirty = ctrlEnd != nil && !handed
	switch {
	case handed:
		// A new loop takes the fans over at once. If it cannot start, the
		// replacement marks the guard (guardHandoverFailed).
	case c.dirty:
		log.Printf("cooling: the loop did not leave the fans safe (%v); the guard stays", ctrlEnd)
	case c.running == 0:
		c.link.DisarmSoon(coolingGuardGrace)
	}
}

// guardHandoverFailed is called when the loop that was to take over detached fans
// did not start: nothing regulates them, so the guard stays.
func (e *Engine) guardHandoverFailed(err error) {
	c := &e.crash
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.link == nil {
		return
	}
	c.dirty = true
	log.Printf("cooling: the new loop did not start (%v); the guard stays", err)
}

// guardFinish is called by Stop, once the loop has had its chance to end. Only if
// every loop ended cleanly the guard is told goodbye; otherwise it stays, and acts
// when this process ends.
func (e *Engine) guardFinish() {
	c := &e.crash
	c.mu.Lock()
	link, clean := c.link, c.running == 0 && !c.dirty
	c.mu.Unlock()
	if link == nil {
		return
	}
	if !clean {
		log.Printf("cooling: the fans may not be at a safe duty; the guard stays and acts when CaseHUD has ended")
		return
	}
	link.Disarm()
}

// guardState is what the guard is told: the fans of the current live loop.
func (e *Engine) guardState() (coolguard.State, bool) {
	if f := e.coolingSnapshot().fans; f != nil {
		return f.guardState(), true
	}
	return coolguard.State{}, false
}

// CoolingRivalRunning tells whether another fan program is running. The guard
// process asks it before it touches the controller.
func CoolingRivalRunning() (bool, error) {
	reason, _ := checkRivals(conflicts.NewSystem())
	if reason == coolingBlockedCheck {
		return false, errors.New("cannot tell which programs run")
	}
	return reason != "", nil
}

// coolingOpenFans opens the fan controller (tests replace it). It waits for a guard
// process that is setting the fans, so that the two never write at once.
var coolingOpenFans = func(ctx context.Context) (lianlitl.FanDevice, error) {
	coolguard.WaitIdle(ctx, coolingGuardOpenWait)
	return lianlitl.OpenFans(ctx)
}

// addCrashInfo adds to a view what concerns hard ends: that the safety copy of
// cooling.json is in use, and that the sensor source does not answer.
func (e *Engine) addCrashInfo(v *CoolingView) {
	kept, restored := e.crash.restoredFile()
	v.Restored, v.RestoredFile = restored, kept
	v.SensorsDown = e.prov != nil && e.prov.Err() != ""
	v.SensorsStale = e.prov != nil && e.prov.Stale()
	if e.prov != nil {
		v.SensorSource, v.SensorsDisplayOnly = e.prov.DisplayOnlySource()
	}
}
