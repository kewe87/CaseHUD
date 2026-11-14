package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
)

// A pump is only regulated after it is proven that its speed sensor and the control
// channel of its header belong together: the channel is raised once and the sensor
// must follow (cooling.Match). The verdict is kept with the pair of names, so it
// is asked for once, and again when either name changes.

const coolingMatchFile = "cooling_match.json"

// pumpState is what is known about a pump's header.
type pumpState int

const (
	pumpPending    pumpState = iota // not proven yet: the board keeps the pump
	pumpChecking                    // the check is running
	pumpMatched                     // proven: the control may set the pump
	pumpNotMatched                  // the check failed: the board keeps the pump
)

const (
	matchResultOK     = "ok"
	matchResultFailed = "failed"
)

// Pauses of the matching (tests replace them).
var (
	pumpMatchWait   = cooling.WaitContext
	pumpMatchPoll   = 5 * time.Second  // look again whether a check may start
	pumpMatchDenied = 30 * time.Second // after the header turned out not to be released
	pumpMatchLater  = 10 * time.Minute // after a run that proved nothing
)

var (
	errMatchBlocked = errors.New("cooling: another program is in charge")
	errMatchBusy    = errors.New("cooling: a measurement is running")
)

// matchRecord is a stored verdict with the names it is about. Board says which
// mainboard it was found on (see pumpBoardID): the same names on another board
// prove nothing. A record without it comes from before that and is checked again.
type matchRecord struct {
	Control string `json:"control"`
	Sensor  string `json:"sensor"`
	Result  string `json:"result"`
	Board   string `json:"board,omitempty"`
	At      string `json:"at,omitempty"`
}

var pumpChipOf = regexp.MustCompile(`^/lpc/([a-z0-9]{1,32})/`)

// pumpBoardID names the mainboard a pump sits on: the chip of its channel and,
// where the sensor source names it, the board model (the hardware node above the
// speed sensor). It is empty if not even the chip is known.
func (e *Engine) pumpBoardID(control, sensor string) string {
	m := pumpChipOf.FindStringSubmatch(control)
	if m == nil {
		return ""
	}
	if e.prov != nil {
		for _, r := range e.prov.All() {
			if r.Key != sensor {
				continue
			}
			if model, _, ok := strings.Cut(r.Path, "/"); ok && strings.TrimSpace(model) != "" && !strings.Contains(model, "|") {
				return m[1] + "|" + strings.TrimSpace(model)
			}
			break
		}
	}
	return m[1]
}

// sameBoard tells whether a stored identity is the one of this machine. A record
// with none, or a board that cannot be named, never agrees. If only the model
// cannot be read at the moment, the chip has to agree.
func sameBoard(stored, now string) bool {
	if stored == "" || now == "" {
		return false
	}
	if stored == now {
		return true
	}
	chip, _, _ := strings.Cut(stored, "|")
	return !strings.Contains(now, "|") && chip == now
}

type matchFile struct {
	V     int                    `json:"v"`
	Pumps map[string]matchRecord `json:"pumps"`
}

// loadMatches reads the stored verdicts. A missing or damaged file means none, so
// the pumps are checked again, which is the safe direction.
func (e *Engine) loadMatches() map[string]matchRecord {
	b, _, err := readFileCapped(e.Store.path(coolingMatchFile), 1<<16)
	out := map[string]matchRecord{}
	if err != nil || b == nil {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("cooling: pump check results unreadable: %v", withoutPath(err))
		}
		return out
	}
	var f matchFile
	if json.Unmarshal(b, &f) != nil || f.V != 1 {
		log.Printf("cooling: pump check results unreadable")
		return out
	}
	for id, r := range f.Pumps {
		if r.Result == matchResultOK || r.Result == matchResultFailed {
			out[id] = r
		}
	}
	return out
}

// storeMatch keeps (or, with an empty result, drops) the verdict of one pump.
func (e *Engine) storeMatch(id string, rec *matchRecord) {
	e.matchMu.Lock()
	defer e.matchMu.Unlock()
	m := e.loadMatches()
	if rec == nil {
		if _, ok := m[id]; !ok {
			return
		}
		delete(m, id)
	} else {
		m[id] = *rec
	}
	data, err := json.MarshalIndent(matchFile{V: 1, Pumps: m}, "", "  ")
	if err == nil {
		err = writeFileAtomic(e.Store.path(coolingMatchFile), data)
	}
	if err != nil {
		log.Printf("cooling: keeping the pump check result: %v", withoutPath(err))
	}
}

// applyMatches sets the state of every pump from the stored verdicts.
func (e *Engine) applyMatches(b *boardPumps) {
	if b == nil {
		return
	}
	e.matchMu.Lock()
	m := e.loadMatches()
	e.matchMu.Unlock()
	for _, id := range b.ids() {
		st := pumpPending
		if r, ok := m[id]; ok && r.Control == b.keys[id] && r.Sensor == b.sensors[id] &&
			sameBoard(r.Board, e.pumpBoardID(b.keys[id], b.sensors[id])) {
			st = pumpMatched
			if r.Result == matchResultFailed {
				st = pumpNotMatched
			}
		}
		b.setState(id, st)
	}
}

func (b *boardPumps) state(id string) pumpState {
	b.pmu.Lock()
	defer b.pmu.Unlock()
	return b.states[id]
}

func (b *boardPumps) setState(id string, s pumpState) {
	b.pmu.Lock()
	defer b.pmu.Unlock()
	b.states[id] = s
}

func (b *boardPumps) isState(id string, s pumpState) bool {
	if b == nil {
		return false
	}
	return b.state(id) == s
}

// holdsUnproven lists the pumps the control must not write yet.
func (b *boardPumps) unproven() []string {
	if b == nil {
		return nil
	}
	var out []string
	for _, id := range b.ids() {
		if b.state(id) != pumpMatched {
			out = append(out, id)
		}
	}
	return out
}

// unprovenFor tells whether id is a board pump whose pairing is not proven yet:
// pending or being checked. A failed check is NotMatched, with its own flag.
func (b *boardPumps) unprovenFor(id string) bool {
	if b == nil {
		return false
	}
	_, mine := b.keys[id]
	s := b.state(id)
	return mine && (s == pumpPending || s == pumpChecking)
}

// pumpMatchDevice is the pump as the check sees it. It refuses to write a duty
// below the floor or below what the header had when the check began.
type pumpMatchDevice struct {
	b     *boardPumps
	id    string
	floor int
	start int
	known bool
	// low is the one duty below the start that this run may send (0: none).
	low int
}

func (d *pumpMatchDevice) TakeControl(string) error { return nil }

func (d *pumpMatchDevice) SetDuty(_ string, percent int) error {
	if percent < max(d.floor, cooling.LowestPumpDuty) || (d.known && percent < d.start && percent != d.low) {
		return fmt.Errorf("cooling: refusing to lower %s to %d %%", d.id, percent)
	}
	return d.b.write(d.id, percent)
}

func (d *pumpMatchDevice) Speeds() (map[string]int, error) { return d.b.Speeds() }

// channelMissing tells that the sensor source lists control channels, but not the
// one named: the name was derived from the speed sensor and the board has no such
// channel.
func (e *Engine) channelMissing(key string) bool {
	listed := false
	for _, r := range e.prov.All() {
		if r.Key == key {
			return false
		}
		listed = listed || strings.Contains(r.Key, "/control/")
	}
	return listed
}

// headerDuty reads the duty the board has set on the control channel.
func (e *Engine) headerDuty(key string) (int, bool) {
	for _, r := range e.prov.Fresh() {
		if r.Key == key && r.Unit == "%" && finite(r.Value) && r.Value >= 0 && r.Value <= 100 {
			return int(math.Round(r.Value)), true
		}
	}
	return 0, false
}

// mayCheck says whether a check of the pump may start now.
func (e *Engine) mayCheck(ctrl *cooling.Controller, b *boardPumps, id string) bool {
	if b.block.Load() != nil || e.coolingMeasuring() {
		return false
	}
	// A preset switch or a save is under way; it replaces this loop anyway.
	if !e.coolOps.TryLock() {
		return false
	}
	e.coolOps.Unlock()
	if ctrl.LastTick().IsZero() {
		return false
	}
	manual := false
	for _, o := range e.coolingSnapshot().cfg.Outputs {
		manual = manual || (o.ID == id && o.Manual != nil)
	}
	for _, s := range ctrl.Status() {
		if s.PumpDown {
			return false
		}
		if s.ID != id {
			continue
		}
		// A manual value follows no temperature, so none is needed to start.
		if (!s.HasTemp && !manual) || s.SensorLost || !s.HasRPM || s.DeviceLost {
			return false
		}
	}
	return true
}

// checkOngoing ends a check as soon as the pump is no longer free to be tested.
// A pump that was refused before stays reported as not released while a new
// attempt runs; the attempt only ends on a refusal of its own (denied is the
// count at its start).
func (e *Engine) checkOngoing(ctrl *cooling.Controller, b *boardPumps, id string, denied int) func() error {
	return func() error {
		switch {
		case b.block.Load() != nil:
			return errMatchBlocked
		case b.deniedSince(id, denied):
			return cooling.ErrNotReleased
		case b.broken.Load():
			return errors.New("cooling: no line to the sensor service")
		case e.coolingMeasuring():
			return errMatchBusy
		}
		for _, s := range ctrl.Status() {
			if s.PumpDown {
				return cooling.ErrPumpStopped
			}
		}
		return nil
	}
}

// noSourceFor tells a pump that cannot be checked because the configuration has no
// temperature source at all.
func (b *boardPumps) noSourceFor(cfg coolingConfig, id string) bool {
	if b == nil {
		return false
	}
	_, mine := b.keys[id]
	return mine && b.state(id) != pumpMatched && len(cfg.measureSources()) == 0
}

// matchPump runs the check of one pump and returns how long to leave it alone.
func (e *Engine) matchPump(ctx context.Context, ctrl *cooling.Controller, b *boardPumps, cfg coolingConfig, id string) time.Duration {
	if !e.mayCheck(ctrl, b, id) {
		return pumpMatchPoll
	}
	i := slices.IndexFunc(cfg.Outputs, func(o coolingOutputConfig) bool { return o.ID == id })
	if i < 0 {
		return pumpMatchLater
	}
	key := b.keys[id]
	if e.channelMissing(key) {
		return e.concludeMatch(ctrl, b, id, cooling.MatchResult{Verdict: cooling.MatchFailed, Reason: "the board has no such channel"})
	}
	dev := &pumpMatchDevice{b: b, id: id, floor: max(cfg.Outputs[i].MinDuty, cooling.LowestPumpDuty)}
	dev.start, dev.known = e.headerDuty(key)
	if dev.known {
		if low, ok := cooling.MatchLowTarget(dev.start, dev.floor); ok {
			dev.low = low
		}
	}
	sources := cfg.measureSources()
	if len(sources) == 0 { // nothing guards the temperature: the page says so, see noSourceFor
		return pumpMatchLater
	}
	// notReleased stays: it only goes once the service accepts a write.
	denied := b.denials(id)
	b.broken.Store(false)
	b.setState(id, pumpChecking)
	res, err := cooling.Match{
		Device: dev, ID: id, Floor: dev.floor,
		Current:      func() (int, bool) { return dev.start, dev.known },
		Release:      func() error { b.giveBack(id); return nil },
		Wait:         pumpMatchWait,
		Check:        e.checkOngoing(ctrl, b, id, denied),
		Hottest:      func() (float64, bool) { return e.hottestTemp(sources) },
		StoppedBelow: cfg.Outputs[i].StoppedBelowRPM,
	}.Run(ctx)
	b.giveBack(id) // whatever happened, the header goes back to the board
	b.setState(id, pumpPending)
	if err != nil {
		switch {
		case errors.Is(err, cooling.ErrNotReleased):
			return pumpMatchDenied
		case ctx.Err() != nil:
			return pumpMatchLater
		case errors.Is(err, errMatchBlocked), errors.Is(err, errMatchBusy):
			return pumpMatchPoll
		}
		log.Printf("cooling: checking the pump on %s ended: %v", key, err)
		return pumpMatchLater
	}
	return e.concludeMatch(ctrl, b, id, res)
}

// concludeMatch keeps the verdict of a run and acts on it; the pause is how long
// the pump is left alone afterwards.
func (e *Engine) concludeMatch(ctrl *cooling.Controller, b *boardPumps, id string, res cooling.MatchResult) time.Duration {
	key := b.keys[id]
	rec := matchRecord{Control: key, Sensor: b.sensors[id], Board: e.pumpBoardID(key, b.sensors[id]), At: time.Now().UTC().Format(time.RFC3339)}
	switch res.Verdict {
	case cooling.MatchProven:
		rec.Result = matchResultOK
		e.storeMatch(id, &rec)
		b.setState(id, pumpMatched)
		ctrl.Release(id, 0) // the board had the header: unknown, so it only comes down by its steps
		log.Printf("cooling: the pump on %s belongs to its speed sensor, it is regulated now", key)
		return 0
	case cooling.MatchFailed:
		rec.Result = matchResultFailed
		e.storeMatch(id, &rec)
		b.setState(id, pumpNotMatched)
		log.Printf("cooling: the pump on %s does not belong to its speed sensor (%s), the board keeps it", key, res.Reason)
		return 0
	}
	log.Printf("cooling: checking the pump on %s proved nothing (%s), trying again later", key, res.Reason)
	return pumpMatchLater
}

// runPumpMatching checks the pumps one after the other until all are decided.
func (e *Engine) runPumpMatching(ctx context.Context, ctrl *cooling.Controller, b *boardPumps, cfg coolingConfig, done chan<- struct{}) {
	defer close(done)
	next := map[string]time.Time{}
	for ctx.Err() == nil {
		for _, id := range b.unproven() {
			if b.state(id) == pumpNotMatched || time.Now().Before(next[id]) {
				continue
			}
			next[id] = time.Now().Add(e.matchPump(ctx, ctrl, b, cfg, id))
		}
		select {
		case <-ctx.Done():
			return
		case <-b.wake:
			clear(next)
		case <-time.After(pumpMatchPoll):
		}
	}
}

// RecheckCoolingPump forgets a failed verdict and has the pump checked again.
func (e *Engine) RecheckCoolingPump(id string) error {
	c := e.coolingSnapshot()
	if c.pumps == nil || !c.pumps.isState(id, pumpNotMatched) {
		return fmt.Errorf("no pump %q to check", id)
	}
	e.storeMatch(id, nil)
	c.pumps.setState(id, pumpPending)
	select {
	case c.pumps.wake <- struct{}{}:
	default:
	}
	return nil
}
