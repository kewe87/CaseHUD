package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/coolguard"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// guardChild is a guard process that ends when its pipe is closed.
type guardChild struct {
	mu     sync.Mutex
	lines  []map[string]any
	closed chan struct{}
	once   sync.Once
}

func (g *guardChild) Write(b []byte) (int, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return 0, err
	}
	g.mu.Lock()
	g.lines = append(g.lines, m)
	g.mu.Unlock()
	return len(b), nil
}
func (g *guardChild) Close() error { g.once.Do(func() { close(g.closed) }); return nil }
func (g *guardChild) Wait() error  { <-g.closed; return nil }
func (g *guardChild) Kill() error  { return g.Close() }

func (g *guardChild) said(op string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, m := range g.lines {
		if m["op"] == op {
			return true
		}
	}
	return false
}

func (g *guardChild) lastState() (map[string]any, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.lines) - 1; i >= 0; i-- {
		if g.lines[i]["op"] == "state" {
			return g.lines[i], true
		}
	}
	return nil, false
}

func (g *guardChild) ended() bool {
	select {
	case <-g.closed:
		return true
	default:
		return false
	}
}

type guardFarm struct {
	mu       sync.Mutex
	children []*guardChild
}

func (f *guardFarm) spawn() (coolguard.Child, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := &guardChild{closed: make(chan struct{})}
	f.children = append(f.children, c)
	return c, nil
}

func (f *guardFarm) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.children)
}

func (f *guardFarm) first() *guardChild {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.children[0]
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for range 1000 {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", what)
}

func quickGrace(t *testing.T) {
	before := coolingGuardGrace
	coolingGuardGrace = 30 * time.Millisecond
	t.Cleanup(func() { coolingGuardGrace = before })
}

// stateOf reads the safe and last duties a state line carries.
func dutiesOf(m map[string]any, key string) map[string]int {
	out := map[string]int{}
	if raw, ok := m[key].(map[string]any); ok {
		for id, v := range raw {
			out[id] = int(v.(float64))
		}
	}
	return out
}

func TestLiveControlArmsOneGuardWithTheSafeDuties(t *testing.T) {
	r := newLiveRig(t, false)
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.run(t)
	waitUntil(t, "state sent", func() bool {
		return farm.count() == 1 && func() bool { _, ok := farm.first().lastState(); return ok }()
	})
	m, _ := farm.first().lastState()
	if active, _ := m["active"].(bool); !active {
		t.Fatalf("%v", m)
	}
	if safe := dutiesOf(m, "safe"); len(safe) != 1 || safe["port1-fan1"] != 70 {
		t.Fatalf("safe %v: only the fan on the controller, never the pump on the board", safe)
	}
	// The duty the fan was given reaches the guard.
	waitUntil(t, "the fan's duty", func() bool {
		m, _ := farm.first().lastState()
		return dutiesOf(m, "last")["port1-fan1"] == 50
	})
	r.e.stopCoolingModule()
}

func TestNoGuardWithoutALiveControl(t *testing.T) {
	// Demo and preview never reach a fan.
	r := newLiveRig(t, true)
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.e.startCooling()
	time.Sleep(50 * time.Millisecond)
	if farm.count() != 0 {
		t.Fatal("a guard for a demo")
	}
	r.e.stopCoolingModule()

	e := coolingEngine(t, strings.Replace(coolingLiveExample, `"mode":"live"`, `"mode":"preview"`, 1), &coolingReadings{})
	e.GuardSpawn = farm.spawn
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.ctx = ctx
	e.startCooling()
	time.Sleep(50 * time.Millisecond)
	if farm.count() != 0 {
		t.Fatal("a guard for a preview")
	}
	e.stopCoolingModule()
}

func TestCleanEndsSayGoodbye(t *testing.T) {
	quickGrace(t)
	t.Run("area switched off", func(t *testing.T) {
		r := newLiveRig(t, false)
		farm := &guardFarm{}
		r.e.GuardSpawn = farm.spawn
		r.run(t)
		waitUntil(t, "armed", func() bool { return farm.count() == 1 })
		r.e.stopCoolingModule()
		g := farm.first()
		waitUntil(t, "goodbye", g.ended)
		if !g.said("bye") {
			t.Fatal("the guard ended without a goodbye")
		}
	})
	t.Run("program stops", func(t *testing.T) {
		r := newLiveRig(t, false)
		farm := &guardFarm{}
		r.e.GuardSpawn = farm.spawn
		r.run(t)
		waitUntil(t, "armed", func() bool { return farm.count() == 1 })
		// What Stop does with the cooling.
		r.e.cancelFn()
		r.e.waitCooling()
		r.e.guardFinish()
		g := farm.first()
		if !g.ended() || !g.said("bye") {
			t.Fatal("no goodbye when the program stopped")
		}
	})
	t.Run("switched to the test mode", func(t *testing.T) {
		r := newLiveRig(t, false)
		farm := &guardFarm{}
		r.e.GuardSpawn = farm.spawn
		r.run(t)
		waitUntil(t, "armed", func() bool { return farm.count() == 1 })
		if err := r.e.SetCoolingMode("preview"); err != nil {
			t.Fatal(err)
		}
		g := farm.first()
		waitUntil(t, "goodbye", g.ended)
		if !g.said("bye") {
			t.Fatal("no goodbye")
		}
		r.e.stopCoolingModule()
	})
}

// A change of the settings replaces the loop; the guard stays, and no gap opens in
// which a hard end would go unnoticed.
func TestReplacingTheLoopKeepsTheGuard(t *testing.T) {
	quickGrace(t)
	r := newLiveRig(t, false)
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.run(t)
	waitUntil(t, "armed", func() bool { return farm.count() == 1 })
	if err := r.e.SetCoolingConfigJSON([]byte(coolingLiveExample)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if farm.count() != 1 || farm.first().ended() || farm.first().said("bye") {
		t.Fatalf("guards started %d, ended %v", farm.count(), farm.first().ended())
	}
	// The fans stayed where the old loop left them; the guard still knows how high.
	waitUntil(t, "duties inherited", func() bool {
		m, _ := farm.first().lastState()
		return dutiesOf(m, "last")["port1-fan1"] > 0
	})
	// And a clean end of the new loop says goodbye.
	r.e.stopCoolingModule()
	waitUntil(t, "goodbye", farm.first().ended)
	if !farm.first().said("bye") {
		t.Fatal("no goodbye after the replaced loop")
	}
}

// failingTL refuses writes once told to, as a controller that went away does.
type failingTL struct {
	*fakeTL
	fail atomic.Bool
}

func (f *failingTL) SetDuty(id string, p int) error {
	if f.fail.Load() {
		return errors.New("controller gone")
	}
	return f.fakeTL.SetDuty(id, p)
}

// If the loop could not leave the fans safe, the guard must stay: it is what acts
// when this process ends afterwards.
func TestGuardStaysWhenTheFansWereNotLeftSafe(t *testing.T) {
	quickGrace(t)
	r := newLiveRig(t, false)
	dev := &failingTL{fakeTL: r.tl}
	coolingOpenFans = func(context.Context) (lianlitl.FanDevice, error) { return dev, nil }
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.run(t)
	waitUntil(t, "armed", func() bool { return farm.count() == 1 })
	dev.fail.Store(true)
	// The control wanted 50 %, the safe duty is 70 %: ending needs a write that fails.
	r.e.stopCoolingModule()
	time.Sleep(200 * time.Millisecond)
	g := farm.first()
	if g.ended() || g.said("bye") {
		t.Fatal("told the guard goodbye although the fans may be low")
	}
	// Stopping the program does not say goodbye either: the guard acts afterwards.
	r.e.cancelFn()
	r.e.waitCooling()
	r.e.guardFinish()
	if g.ended() || g.said("bye") {
		t.Fatal("goodbye at the end of the program")
	}
	g.Close() // the test's own cleanup
}

// A loop that is still running when the program stops has not left the fans safe.
func TestGuardStaysWhileALoopStillRuns(t *testing.T) {
	r := newLiveRig(t, false)
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.run(t)
	waitUntil(t, "armed", func() bool { return farm.count() == 1 })
	r.e.guardFinish() // no stop was asked for: the loop runs
	if farm.first().ended() || farm.first().said("bye") {
		t.Fatal("goodbye while the fans are regulated")
	}
	r.e.stopCoolingModule()
}

func TestNoGuardSpawnMeansNothingHappens(t *testing.T) {
	r := newLiveRig(t, false)
	r.run(t)
	r.e.stopCoolingModule()
	r.e.guardFinish()
}

// A rival takes over: the fans were left at their safe duty, so ending CaseHUD has
// nothing to put right. If the controller was lost before the rival came, the fans
// may be low and the end says so.
func TestAnEndWhileARivalIsInChargeIsSafeOnlyIfTheFansWereLeftSafe(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900})}
	e, _ := liveEngine(t, g)
	e.cooling.ctrl.Tick()
	g.reason, g.program = coolingBlockedProgram, "Rival"
	e.cooling.ctrl.Tick()
	if err := e.cooling.ctrl.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := e.cooling.ctrl.EndError(); err != nil {
		t.Fatalf("left safe, but the end says %v", err)
	}

	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	g = &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900}), at: now}
	e, _ = liveEngine(t, g)
	e.cooling.ctrl.Tick()
	g.dev.readErr = errors.New("unplugged")
	e.cooling.ctrl.Tick() // lost
	g.reason, g.program = coolingBlockedProgram, "Rival"
	e.cooling.ctrl.Tick()
	e.cooling.ctrl.Shutdown()
	if e.cooling.ctrl.EndError() == nil {
		t.Fatal("a controller lost before the rival came was counted as left safe")
	}
}
