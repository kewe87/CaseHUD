package cooling

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Matching. A pump on a mainboard header is found through two names, its speed
// sensor and the control channel of the header. Nothing proves that both belong
// to the same connector, so before the pump is regulated its channel is raised
// once and the sensor must follow. The duty only ever goes up from where the
// board had it, and the run ends by handing the header back. A pump the board
// already runs at full speed cannot be raised: it is lowered briefly instead,
// but only while the machine is cool and never below its floor, and the start
// duty is put back at once whenever the run ends.
const (
	matchRise  = 20                   // points the duty is raised by
	matchHigh  = 80                   // from this duty on the step goes straight to 100
	matchLeast = 10                   // the smallest step that proves anything
	matchNeed  = 2 * measureTolerance // rpm the speed must move to count as a reaction
	matchAgree = 2                    // readings in a row that must show it
	matchMiss  = 3                    // missing readings in a row that end the run
)

// MatchVerdict is how a run ended.
type MatchVerdict int

const (
	// MatchUnclear: nothing was proven either way; the run is to be repeated later.
	MatchUnclear MatchVerdict = iota
	// MatchProven: the speed followed the channel up and back.
	MatchProven
	// MatchFailed: the speed did not follow, or went the other way.
	MatchFailed
)

// MatchResult is the outcome of a run.
type MatchResult struct {
	Verdict MatchVerdict
	Reason  string
}

// Match describes one run.
type Match struct {
	// Device is written with SetDuty and read with Speeds.
	Device MeasureDevice
	ID     string
	// Floor is the lowest duty the pump may be given (its minimum, never below 30).
	Floor int
	// Current returns the duty the header has now, if that is known. Without it
	// the step goes straight to 100.
	Current func() (int, bool)
	// Release hands the header back to the board.
	Release func() error
	Wait    func(context.Context, time.Duration) error
	// Check may end the run, for example when something else takes over.
	Check func() error
	// Hottest reads the hottest source of the curves. Without it a pump that
	// cannot be raised is never lowered.
	Hottest func() (float64, bool)
	// StoppedBelow is the speed under which the pump counts as stopped (0: only
	// a speed of zero does).
	StoppedBelow int
}

// MatchLowTarget is the duty a run lowers a pump to when the start duty leaves no
// room to raise it. ok is false if the room above the floor is too small.
func MatchLowTarget(start, floor int) (int, bool) {
	start = min(100, max(start, floor, LowestPumpDuty))
	if start <= 100-matchLeast {
		return 0, false
	}
	low := max(start-matchRise, floor, LowestPumpDuty)
	if start-low < matchLeast {
		return 0, false
	}
	return low, true
}

// Run raises the duty and watches the speed. The caller releases the header
// afterwards whatever happened. An error means the run was cut short; the verdict
// is then unclear.
func (m Match) Run(ctx context.Context) (res MatchResult, err error) {
	w := &watcher{m: m}
	base, r, err := w.baseline(ctx)
	if err != nil || r != nil {
		return orUnclear(r), err
	}
	cur, known := 0, false
	if m.Current != nil {
		cur, known = m.Current()
	}
	hold := 0
	target := 100
	if known {
		hold = min(100, max(cur, m.Floor, LowestPumpDuty))
		if hold > 100-matchLeast {
			return m.lowered(ctx, w, base, hold)
		}
		if hold < matchHigh {
			target = hold + matchRise
		}
	}
	if err := m.Device.SetDuty(m.ID, target); err != nil {
		return MatchResult{}, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
	}
	back := false
	defer func() {
		if back {
			return
		}
		// Cut short while raised: the start duty goes back, or the board takes the
		// header again, whichever way the run ends.
		var e error
		if known {
			e = m.Device.SetDuty(m.ID, hold)
		} else if m.Release != nil {
			e = m.Release()
		}
		if e != nil && err == nil {
			res, err = MatchResult{}, fmt.Errorf("cooling: restoring %s: %w", m.ID, e)
		}
	}()
	peak, r, err := w.rise(ctx, base)
	if err != nil || r != nil {
		return orUnclear(r), err
	}
	// Back: the speed must fall again, or it was not this channel that moved it.
	back = true
	if known {
		if err := m.Device.SetDuty(m.ID, hold); err != nil {
			return MatchResult{}, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
		}
	} else if m.Release != nil {
		if err := m.Release(); err != nil {
			return MatchResult{}, err
		}
	}
	return w.fall(ctx, base, peak)
}

// lowered runs the check the other way round for a pump that is already at
// (nearly) full speed: the duty goes down, the speed must follow, and the start
// duty is put back, whichever way the run ends.
func (m Match) lowered(ctx context.Context, w *watcher, base, hold int) (res MatchResult, err error) {
	low, ok := MatchLowTarget(hold, m.Floor)
	if !ok {
		return unclear("no room to raise or lower the duty"), nil
	}
	if m.Hottest == nil {
		return unclear("the temperature is unknown, the pump is not slowed down"), nil
	}
	guard := RiseGuardAt(m.Hottest, PumpMeasureMaxRise, PumpMeasureMaxTemp)
	if err := guard(); err != nil {
		if errors.Is(err, ErrSensorLost) {
			return unclear("a temperature sensor of the curves cannot be read, the pump is not slowed down"), nil
		}
		return unclear("too hot to slow the pump down"), nil
	}
	if err := m.Device.SetDuty(m.ID, low); err != nil {
		return MatchResult{}, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
	}
	restored := false
	defer func() {
		if restored {
			return
		}
		// Cut short: the start duty goes back at once.
		if e := m.Device.SetDuty(m.ID, hold); e != nil && err == nil {
			err = fmt.Errorf("cooling: setting %s: %w", m.ID, e)
		}
	}()
	w.guard = guard
	w.stopped = m.StoppedBelow
	trough, r, err := w.lowerFall(ctx, base)
	if err != nil && w.hot {
		return unclear("too hot to slow the pump down"), nil
	}
	if err != nil || r != nil {
		return orUnclear(r), err
	}
	restored = true
	if err := m.Device.SetDuty(m.ID, hold); err != nil {
		return MatchResult{}, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
	}
	w.guard = nil
	return w.back(ctx, base, trough)
}

func unclear(why string) MatchResult { return MatchResult{MatchUnclear, why} }

func orUnclear(r *MatchResult) MatchResult {
	if r == nil {
		return unclear("")
	}
	return *r
}

type watcher struct {
	m      Match
	seen   []int
	waited time.Duration
	miss   int
	// guard, if set, ends the run when it fails (a slowed pump and the heat).
	guard   func() error
	hot     bool
	stopped int
}

func (w *watcher) reset() { w.seen, w.waited, w.miss = nil, 0, 0 }

// next waits one interval and reads the speed. ok is false for a missing reading.
func (w *watcher) next(ctx context.Context) (rpm int, ok bool, err error) {
	if err := w.m.Wait(ctx, measurePoll); err != nil {
		return 0, false, err
	}
	w.waited += measurePoll
	if w.m.Check != nil {
		if err := w.m.Check(); err != nil {
			return 0, false, err
		}
	}
	if w.guard != nil {
		if err := w.guard(); err != nil {
			w.hot = true
			return 0, false, err
		}
	}
	speeds, err := w.m.Device.Speeds()
	if err != nil {
		return 0, false, fmt.Errorf("cooling: reading speeds: %w", err)
	}
	rpm, ok = speeds[w.m.ID]
	if !ok {
		w.miss++
		return 0, false, nil
	}
	w.miss = 0
	w.seen = append(w.seen, rpm)
	if len(w.seen) > measureWindow {
		w.seen = w.seen[1:]
	}
	return rpm, true, nil
}

func (w *watcher) settled() bool {
	return len(w.seen) == measureWindow && spread(w.seen) <= measureTolerance
}

func (w *watcher) lost() *MatchResult {
	if w.miss >= matchMiss {
		r := unclear("no speed reading")
		return &r
	}
	return nil
}

// baseline waits until the speed has stopped moving.
func (w *watcher) baseline(ctx context.Context) (int, *MatchResult, error) {
	w.reset()
	for {
		// The baseline is the median of the window, not the last reading.
		_, ok, err := w.next(ctx)
		if err != nil {
			return 0, nil, err
		}
		if !ok {
			if r := w.lost(); r != nil {
				return 0, r, nil
			}
			continue
		}
		if w.settled() && w.waited >= measureMinWait {
			rpm := median(w.seen)
			if rpm <= 0 {
				r := unclear("the pump reports a standstill")
				return 0, &r, nil
			}
			return rpm, nil, nil
		}
		if w.waited >= measureMaxWait {
			r := unclear("the speed does not settle")
			return 0, &r, nil
		}
	}
}

// rise waits for the speed to climb by the needed amount.
func (w *watcher) rise(ctx context.Context, base int) (int, *MatchResult, error) {
	w.reset()
	up, zero := 0, 0
	for {
		rpm, ok, err := w.next(ctx)
		if err != nil {
			return 0, nil, err
		}
		if !ok {
			if r := w.lost(); r != nil {
				return 0, r, nil
			}
			continue
		}
		if rpm == 0 {
			if zero++; zero >= matchAgree {
				r := unclear("the pump stood still")
				return 0, &r, nil
			}
		} else {
			zero = 0
		}
		if rpm >= base+matchNeed {
			if up++; up >= matchAgree {
				return rpm, nil, nil
			}
		} else {
			up = 0
		}
		if w.settled() && w.waited >= measureStartWait {
			if med := median(w.seen); med <= base-matchNeed {
				r := MatchResult{MatchFailed, "the speed fell when the duty rose"}
				return 0, &r, nil
			}
			r := MatchResult{MatchFailed, "the speed did not follow the duty"}
			return 0, &r, nil
		}
		if w.waited >= measureMaxWait {
			if med := median(w.seen); med <= base-matchNeed {
				r := MatchResult{MatchFailed, "the speed fell when the duty rose"}
				return 0, &r, nil
			}
			r := unclear("the speed does not settle")
			return 0, &r, nil
		}
	}
}

// fall waits for the speed to come back down after the duty did.
func (w *watcher) fall(ctx context.Context, base, peak int) (MatchResult, error) {
	w.reset()
	limit := base + (peak-base)/2
	down := 0
	for {
		rpm, ok, err := w.next(ctx)
		if err != nil {
			return unclear(""), err
		}
		if !ok {
			if r := w.lost(); r != nil {
				return *r, nil
			}
			continue
		}
		if rpm <= limit {
			if down++; down >= matchAgree {
				return MatchResult{MatchProven, ""}, nil
			}
		} else {
			down = 0
		}
		if w.waited >= measureMaxWait {
			return unclear("the speed did not come back"), nil
		}
	}
}

// lowerFall waits for the speed to drop by the needed amount after the duty did.
func (w *watcher) lowerFall(ctx context.Context, base int) (int, *MatchResult, error) {
	w.reset()
	down, zero := 0, 0
	for {
		rpm, ok, err := w.next(ctx)
		if err != nil {
			return 0, nil, err
		}
		if !ok {
			if r := w.lost(); r != nil {
				return 0, r, nil
			}
			continue
		}
		if rpm == 0 || (w.stopped > 0 && rpm < w.stopped) {
			if zero++; zero >= matchAgree {
				r := unclear("the pump stood still")
				return 0, &r, nil
			}
		} else {
			zero = 0
		}
		if rpm <= base-matchNeed {
			if down++; down >= matchAgree {
				return rpm, nil, nil
			}
		} else {
			down = 0
		}
		if w.settled() && w.waited >= measureStartWait {
			if med := median(w.seen); med >= base+matchNeed {
				r := MatchResult{MatchFailed, "the speed rose when the duty fell"}
				return 0, &r, nil
			}
			r := MatchResult{MatchFailed, "the speed did not follow the duty"}
			return 0, &r, nil
		}
		if w.waited >= measureMaxWait {
			if med := median(w.seen); med >= base+matchNeed {
				r := MatchResult{MatchFailed, "the speed rose when the duty fell"}
				return 0, &r, nil
			}
			r := unclear("the speed does not settle")
			return 0, &r, nil
		}
	}
}

// back waits for the speed to climb again after the duty went back up.
func (w *watcher) back(ctx context.Context, base, trough int) (MatchResult, error) {
	w.reset()
	limit := trough + (base-trough)/2
	up := 0
	for {
		rpm, ok, err := w.next(ctx)
		if err != nil {
			return unclear(""), err
		}
		if !ok {
			if r := w.lost(); r != nil {
				return *r, nil
			}
			continue
		}
		if rpm >= limit {
			if up++; up >= matchAgree {
				return MatchResult{MatchProven, ""}, nil
			}
		} else {
			up = 0
		}
		if w.waited >= measureMaxWait {
			return unclear("the speed did not come back"), nil
		}
	}
}

// ErrNotReleased ends a run when the service refuses the header.
var ErrNotReleased = errors.New("cooling: the header is not released")
