package app

import (
	"context"
	"errors"
	"log"
	"maps"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/conflicts"
	"github.com/kewe87/casehud/internal/coolguard"
	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

const (
	coolingReopenAfter = 5 * time.Second  // pause before opening a lost controller again
	coolingSensorWait  = 30 * time.Second // how long the sensors get after the start
	coolingStopWait    = 5 * time.Second  // how long Stop waits for the safe duties
)

// Reasons why the live control leaves the controller alone (CoolingView.Blocked).
const (
	coolingBlockedDisclaimer = "disclaimer"
	coolingBlockedCheck      = "check"
	coolingBlockedProgram    = "program"
)

var errCoolingBlocked = errors.New("cooling: another program is in charge")

// leftSafeError is errCoolingBlocked for the case that the fans were left at a safe
// duty before: the control then has nothing to put right when it ends.
type leftSafeError struct{}

func (leftSafeError) Error() string { return errCoolingBlocked.Error() }
func (leftSafeError) Is(target error) bool {
	return target == errCoolingBlocked || target == cooling.ErrLeftSafe
}

// tlOutputID is the name of a fan on the TL controller.
var tlOutputID = regexp.MustCompile(`^port[1-4]-fan[1-9][0-9]?$`)

// coolingGate says whether the live control may touch the controller. The
// returned reason is empty if it may, and program names the program in the way.
type coolingGate func() (reason, program string)

// rivalGate is the gate of a real engine.
func (e *Engine) rivalGate() coolingGate {
	sys := e.conflictSystem()
	var at time.Time
	var seen coolingBlock
	var known bool
	return func() (string, string) {
		if !e.outputAllowed() {
			return coolingBlockedDisclaimer, ""
		}
		if time.Since(at) < conflictInterval {
			return seen.reason, seen.program
		}
		at = time.Now()
		seen, known = lookForRivals(sys, seen, known)
		return seen.reason, seen.program
	}
}

// lookForRivals is one look at the running programs. If the look fails the last
// answer stands: a failed list says nothing about the programs, and letting go of
// a working control for it would leave the fans where they happen to be. Only
// what was never known counts as blocked.
func lookForRivals(sys conflicts.System, last coolingBlock, known bool) (coolingBlock, bool) {
	reason, program := checkRivals(sys)
	if reason == coolingBlockedCheck && known {
		return last, true
	}
	return coolingBlock{reason, program}, reason != coolingBlockedCheck
}

func checkRivals(sys conflicts.System) (reason, program string) {
	// Two regulators on one controller overwrite each other: the catalog's fan
	// programs lock the cooling, and ignoring them in the notice changes nothing.
	procs, err := sys.Processes(conflicts.IsFanProgram)
	if err != nil {
		return coolingBlockedCheck, ""
	}
	if len(procs) > 0 {
		return coolingBlockedProgram, strings.TrimSuffix(procs[0].Exe, ".exe")
	}
	return "", ""
}

// coolingBlock is what the view shows about the gate.
type coolingBlock struct{ reason, program string }

// tlFans is the fan controller as the cooling sees it. It opens the controller
// itself and again after losing it, and lets go of it while the gate is closed.
type tlFans struct {
	ctx  context.Context
	open func(context.Context) (lianlitl.FanDevice, error)
	gate coolingGate
	now  func() time.Time

	block atomic.Pointer[coolingBlock]

	mu      sync.Mutex
	dev     lianlitl.FanDevice
	retryAt time.Time
	lastErr error
	safe    map[string]int // the safe duty of every fan
	last    map[string]int // the duty last written to a fan
	// leftSafe: the fans were set to their safe duty when the controller was let go, and
	// it has not been opened or lost since.
	leftSafe bool

	// What the guard process is told, kept apart from mu so that it can be read
	// while a write is in progress.
	gmu      sync.Mutex
	gsafe    map[string]int
	ghigh    map[string]int // the most a fan has been given or is being given
	onChange func()         // called when ghigh changed (may be nil)
}

// setSafe says what the fans are left at when another program takes over.
func (t *tlFans) setSafe(safe map[string]int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.safe = safe
	t.gmu.Lock()
	t.gsafe = maps.Clone(safe)
	t.gmu.Unlock()
}

// handOver lets go of the controller for the program that closed the gate. Fans
// keep their last duty, and whether that program really drives them is not known:
// it may hang, or only be its background service. So they stay at a safe duty, or
// above it if they ran faster.
func (t *tlFans) handOver() {
	if t.dev == nil {
		return
	}
	ok := true
	for id, safe := range t.safe {
		if err := t.dev.SetDuty(id, max(safe, t.last[id])); err != nil {
			log.Printf("cooling: leaving %s at a safe speed: %v", id, err)
			ok = false
		}
	}
	t.leftSafe = ok
	t.drop()
}

func (t *tlFans) drop() {
	if t.dev != nil {
		t.dev.Close()
		t.dev = nil
	}
}

func (t *tlFans) lose(err error) {
	t.leftSafe = false
	t.drop()
	t.lastErr = err
	t.retryAt = t.now().Add(coolingReopenAfter)
}

// Speeds implements cooling.Hardware. An error here means nothing is written:
// that is how the gate and a lost controller stop the control.
func (t *tlFans) Speeds() (map[string]int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	reason, program := t.gate()
	if reason != "" {
		t.block.Store(&coolingBlock{reason, program})
		t.handOver()
		if t.leftSafe {
			return nil, leftSafeError{}
		}
		return nil, errCoolingBlocked
	}
	t.block.Store(nil)
	if t.dev == nil {
		if t.now().Before(t.retryAt) {
			return nil, t.lastErr
		}
		dev, err := t.open(t.ctx)
		if err != nil {
			t.lose(err)
			return nil, err
		}
		t.leftSafe = false
		t.dev = dev
	}
	speeds, err := t.dev.Speeds()
	if err != nil {
		t.lose(err)
		return nil, err
	}
	// The controller starts a stopped fan again by itself after some seconds, unless
	// it is told again. Any other duty stays as it is without being repeated.
	for id, duty := range t.last {
		if duty != 0 {
			continue
		}
		if err := t.dev.SetDuty(id, 0); err != nil {
			t.lose(err)
			return nil, err
		}
	}
	return speeds, nil
}

// LowestDuty is the lowest duty above 0 that makes a difference on the
// controller; a calibration does not measure below it.
func (t *tlFans) LowestDuty() int { return lianlitl.LowestDuty }

// TakeControl implements cooling.Hardware.
func (t *tlFans) TakeControl(id string) error {
	return t.use(func(d lianlitl.FanDevice) error { return d.TakeControl(id) })
}

// SetDuty implements cooling.Hardware.
func (t *tlFans) SetDuty(id string, percent int) error {
	return t.use(func(d lianlitl.FanDevice) error {
		t.noteDuty(id, percent, false)
		err := d.SetDuty(id, percent)
		if err == nil {
			if t.last == nil {
				t.last = map[string]int{}
			}
			t.last[id] = percent
			t.noteDuty(id, percent, true)
		}
		return err
	})
}

func (t *tlFans) use(f func(lianlitl.FanDevice) error) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dev == nil {
		return errors.New("cooling: controller not open")
	}
	return f(t.dev)
}

// Close lets go of the controller.
func (t *tlFans) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.drop()
}

// blocked returns the reason shown in the view.
func (t *tlFans) blocked() coolingBlock {
	if b := t.block.Load(); b != nil {
		return *b
	}
	return coolingBlock{}
}

// waitForSensors holds the start back until every source has a reading, so a
// slow sensor service does not send all fans to their safe speed on each start.
// Afterwards the failure rules of the control take over.
func waitForSensors(ctx context.Context, cfg coolingConfig, temps func() map[string]float64, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if sourcesPresent(cfg, temps()) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func sourcesPresent(cfg coolingConfig, temps map[string]float64) bool {
	for _, o := range cfg.regulated() {
		for _, s := range cfg.sensors(o.Curve, map[string]bool{}) {
			if _, ok := temps[s]; !ok {
				return false
			}
		}
	}
	return true
}

// waitCooling waits for the loop to leave the outputs at their safe duty. It
// must run while the devices are still open.
func (e *Engine) waitCooling() {
	done := e.coolingSnapshot().done
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(coolingStopWait):
	}
}

// noteDuty records what the guard process must know before and after a write: the
// higher of the old and the new duty while it is in progress, the new one after.
func (t *tlFans) noteDuty(id string, percent int, done bool) {
	t.gmu.Lock()
	if t.ghigh == nil {
		t.ghigh = map[string]int{}
	}
	if done {
		t.ghigh[id] = percent
	} else {
		t.ghigh[id] = max(t.ghigh[id], percent)
	}
	t.gmu.Unlock()
	if t.onChange != nil {
		t.onChange()
	}
}

// guardState is what the guard process is told about the fans.
func (t *tlFans) guardState() coolguard.State {
	t.gmu.Lock()
	defer t.gmu.Unlock()
	return coolguard.State{Active: t.block.Load() == nil, Safe: maps.Clone(t.gsafe), Last: maps.Clone(t.ghigh)}
}

// inheritDuties takes over what the previous control knew about the fans, which
// stay where it left them when a new control takes over at once.
func (t *tlFans) inheritDuties(from *tlFans) {
	if from == nil {
		return
	}
	from.gmu.Lock()
	high := maps.Clone(from.ghigh)
	from.gmu.Unlock()
	t.gmu.Lock()
	t.ghigh = high
	t.gmu.Unlock()
}
