package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
)

const pumpControlReadingKey = "/lpc/chip/0/control/6"

// simBoard is a mainboard with one pump: its speed follows the duty of the header
// it sits on. The line to the service and the readings are both views of it.
type simBoard struct {
	mu      sync.Mutex
	board   int  // duty the board has on the header
	follows bool // the speed sensor belongs to this header
	inverse bool
	noRPM   bool
	denied  bool
	shape   func(n, rpm int) int // distorts the speed, n counts the waits
	// controlKey is the channel the board lists; empty means the one of the pump.
	controlKey string
	// followKey is the only channel the pump sits on; empty means any.
	followKey string
	keys      []string // channel of every hold
	waits     int
	cur       int // duty held through the line, 0 = none
	holds     []int
	closed    int
	lowest    int  // lowest duty ever sent
	collapse  int  // under this duty the pump stands
	dead      bool // and it stays standing, whatever is set afterwards
	broke     bool
	hot       float64 // temperature of /t, 0 = 30
	noTemp    bool    // /t is not listed
}

func newSimBoard(board int) *simBoard { return &simBoard{board: board, follows: true, lowest: 1000} }

func (s *simBoard) Hold(key string, percent int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.denied {
		return enram.ErrDenied
	}
	s.keys = append(s.keys, key)
	if s.followKey == "" || key == s.followKey {
		s.cur = percent
	}
	s.holds = append(s.holds, percent)
	if s.collapse > 0 && percent < s.collapse {
		s.broke = true
	}
	s.lowest = min(s.lowest, percent)
	return nil
}

func (s *simBoard) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur = 0
	s.closed++
	return nil
}

func (s *simBoard) duty() int {
	if s.cur > 0 {
		return s.cur
	}
	return s.board
}

func (s *simBoard) rpm() int {
	d := s.duty()
	switch {
	case s.broke && s.dead, s.collapse > 0 && d < s.collapse:
		return 0
	case !s.follows:
		return 2400
	case s.inverse:
		return 7000 - d*60
	}
	return d * 60
}

// All implements metrics.Provider.
func (s *simBoard) All() []metrics.Reading {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []metrics.Reading{
		{Key: firstOf(s.controlKey, pumpControlReadingKey), Value: float64(s.duty()), Unit: "%"},
	}
	if !s.noTemp {
		out = append(out, metrics.Reading{Key: "/t", Value: firstTemp(s.hot, 30), Unit: "°C"})
	}
	if !s.noRPM {
		out = append(out, metrics.Reading{Key: pumpRPMKey, Value: float64(s.rpm()), Unit: "RPM"})
	}
	return out
}
func (s *simBoard) Refresh() error { return nil }
func (s *simBoard) Get(string) (metrics.Reading, bool) {
	return metrics.Reading{}, false
}

// wait is the pause of the check: the speed moves, nothing really sleeps.
func (s *simBoard) wait(ctx context.Context, _ time.Duration) error {
	s.mu.Lock()
	s.waits++
	n := s.waits
	if s.shape != nil {
		s.shape(n, 0)
	}
	s.mu.Unlock()
	return ctx.Err()
}

func (s *simBoard) set(f func(*simBoard)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

func useSimBoard(t *testing.T, s *simBoard) {
	t.Helper()
	line := coolingOpenPumpLine
	wait := pumpMatchWait
	coolingOpenPumpLine = func() pumpLine { return s }
	pumpMatchWait = s.wait
	t.Cleanup(func() { coolingOpenPumpLine, pumpMatchWait = line, wait })
}

type matchRig struct {
	e     *Engine
	s     *simBoard
	cfg   coolingConfig
	pumps *boardPumps
	ctrl  *cooling.Controller
	gate  string
}

func newMatchRig(t *testing.T, s *simBoard) *matchRig {
	t.Helper()
	useSimBoard(t, s)
	r := &matchRig{s: s}
	r.e = coolingEngine(t, pumpExample, s)
	r.cfg, _ = r.e.Store.loadCoolingConfig()
	r.start(t)
	return r
}

// start builds the control the way launchCooling does, from the stored results.
func (r *matchRig) start(t *testing.T) {
	t.Helper()
	r.pumps = newBoardPumps(r.cfg, r.e.prov, func() (string, string) { return r.gate, "" })
	r.e.applyMatches(r.pumps)
	var err error
	if r.ctrl, err = newCoolingLive(r.cfg, r.e.prov, nil, r.pumps); err != nil {
		t.Fatal(err)
	}
	for _, id := range r.pumps.unproven() {
		r.ctrl.Hold(id)
	}
	r.e.cooling = coolingState{ctrl: r.ctrl, cfg: r.cfg, fans: &tlFans{}, pumps: r.pumps}
	if err := r.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
}

func (r *matchRig) check() time.Duration {
	return r.e.matchPump(context.Background(), r.ctrl, r.pumps, r.cfg, "pump")
}

func (r *matchRig) record(t *testing.T) (matchRecord, bool) {
	t.Helper()
	rec, ok := r.e.loadMatches()["pump"]
	return rec, ok
}

// steps is what was sent, without the repeats of the line.
func (r *matchRig) steps() []int { return slices.Compact(r.sent()) }

func (r *matchRig) sent() []int {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return slices.Clone(r.s.holds)
}

func TestAPumpThatFollowsIsProvenStoredAndThenRegulated(t *testing.T) {
	r := newMatchRig(t, newSimBoard(60))
	// Before the proof nothing is written, whatever the curve says.
	for range 3 {
		r.ctrl.Tick()
	}
	if len(r.sent()) != 0 {
		t.Fatalf("written before the proof: %v", r.sent())
	}
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	rec, ok := r.record(t)
	if !ok || rec.Result != matchResultOK || rec.Control != pumpControlReadingKey || rec.Sensor != pumpRPMKey {
		t.Fatalf("%+v %v", rec, ok)
	}
	if h := r.sent(); h[0] != 80 || h[len(h)-1] != 60 {
		t.Fatalf("holds %v", h)
	}
	if r.s.cur != 0 || r.s.closed == 0 {
		t.Fatal("the header was not given back")
	}
	r.sent()
	n := len(r.sent())
	r.ctrl.Tick()
	if h := r.sent(); len(h) == n || h[len(h)-1] != 70 {
		t.Fatalf("not regulated after the proof: %v", h)
	}
	if v := r.e.CoolingView().Outputs[0]; v.NotMatched || v.Matching {
		t.Fatalf("%+v", v)
	}
}

func TestAPumpIsNeverGivenLessThanTheBoardOrItsMinimum(t *testing.T) {
	for _, board := range []int{0, 20, 50, 65, 85, 90} {
		for _, follows := range []bool{true, false} {
			s := newSimBoard(board)
			s.follows = follows
			r := newMatchRig(t, s)
			r.check()
			// The pump's minimum is 50.
			if low := max(board, 50); s.lowest < low {
				t.Errorf("board %d follows %v: sent %d, %v", board, follows, s.lowest, r.sent())
			}
		}
	}
}

func TestAPumpThatDoesNotReactIsLeftToTheBoard(t *testing.T) {
	for name, set := range map[string]func(*simBoard){
		"no reaction": func(s *simBoard) { s.follows = false },
		"wrong way":   func(s *simBoard) { s.inverse = true },
	} {
		s := newSimBoard(60)
		set(s)
		r := newMatchRig(t, s)
		r.check()
		if r.pumps.state("pump") != pumpNotMatched {
			t.Fatalf("%s: state %v", name, r.pumps.state("pump"))
		}
		if rec, ok := r.record(t); !ok || rec.Result != matchResultFailed {
			t.Fatalf("%s: %+v", name, rec)
		}
		sent := len(r.sent())
		for range 3 {
			r.ctrl.Tick()
		}
		if len(r.sent()) != sent {
			t.Fatalf("%s: the pump is written although the check failed", name)
		}
		if s.cur != 0 {
			t.Fatalf("%s: header still held", name)
		}
		v := r.e.CoolingView()
		if o := v.Outputs[0]; !o.NotMatched || o.PumpDown || o.RPM == nil || v.Alarm != nil {
			t.Fatalf("%s: %+v alarm %+v", name, o, v.Alarm)
		}
		if outputProblem(r.e.CoolingView().Outputs[0]) != "not_matched" {
			t.Fatalf("%s: problem %q", name, outputProblem(r.e.CoolingView().Outputs[0]))
		}
	}
}

func TestMatchedStateSurvivesARestartAndAChangedPairChecksAgain(t *testing.T) {
	r := newMatchRig(t, newSimBoard(60))
	r.check()
	n := len(r.sent())

	r.start(t) // restart with the same pair
	if r.pumps.state("pump") != pumpMatched {
		t.Fatal("the proof was lost")
	}
	r.ctrl.Tick()
	if len(r.sent()) == n {
		t.Fatal("the proven pump waited for another check")
	}

	other := *r
	other.cfg.Outputs = slices.Clone(r.cfg.Outputs)
	other.cfg.Outputs[0].RPMSensor = "/lpc/chip/0/fan/7"
	other.start(t)
	if other.pumps.state("pump") != pumpPending {
		t.Fatal("a changed pair counted as proven")
	}
	before := len(r.sent())
	other.ctrl.Tick()
	if len(r.sent()) != before {
		t.Fatal("written for an unproven pair")
	}
}

func TestAFailedVerdictStaysUntilItIsAskedForAgain(t *testing.T) {
	s := newSimBoard(60)
	s.follows = false
	r := newMatchRig(t, s)
	r.check()
	r.start(t)
	if r.pumps.state("pump") != pumpNotMatched {
		t.Fatal("the failed verdict was forgotten at a restart")
	}
	if err := r.e.RecheckCoolingPump("nope"); err == nil {
		t.Fatal("unknown pump accepted")
	}
	if err := r.e.RecheckCoolingPump("pump"); err != nil {
		t.Fatal(err)
	}
	if r.pumps.state("pump") != pumpPending {
		t.Fatal("not asked again")
	}
	if _, ok := r.record(t); ok {
		t.Fatal("old verdict kept")
	}
	select {
	case <-r.pumps.wake:
	default:
		t.Fatal("matching was not woken")
	}
	s.set(func(s *simBoard) { s.follows = true })
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatal("not proven on the second check")
	}
	if err := r.e.RecheckCoolingPump("pump"); err == nil {
		t.Fatal("a proven pump was sent to a new check")
	}
}

func TestNothingIsProvenWithoutAUsableSpeed(t *testing.T) {
	// No speed at all: the check does not even start.
	s := newSimBoard(60)
	s.noRPM = true
	r := newMatchRig(t, s)
	r.check()
	if len(r.sent()) != 0 || r.pumps.state("pump") != pumpPending {
		t.Fatalf("%v %v", r.sent(), r.pumps.state("pump"))
	}

	// The speed vanishes during the run.
	s = newSimBoard(60)
	r = newMatchRig(t, s)
	s.shape = func(n, _ int) int {
		if n == 9 {
			s.noRPM = true
		}
		return 0
	}
	if d := r.check(); d != pumpMatchLater {
		t.Fatalf("delay %v", d)
	}
	if _, ok := r.record(t); ok || r.pumps.state("pump") != pumpPending || s.cur != 0 {
		t.Fatalf("proved on a missing speed: state %v held %d", r.pumps.state("pump"), s.cur)
	}
	if s.lowest < 60 {
		t.Fatalf("sent %d", s.lowest)
	}

	// A speed that keeps moving (changing load) is no proof either way.
	s = newSimBoard(60)
	r = newMatchRig(t, s)
	s.shape = func(n, _ int) int { s.board = 40 + n%2*40; return 0 }
	r.check()
	if _, ok := r.record(t); ok || r.pumps.state("pump") != pumpPending {
		t.Fatalf("decided on an unsettled speed: %v", r.pumps.state("pump"))
	}
}

func TestACheckEndsWhenAnotherProgramTakesOver(t *testing.T) {
	s := newSimBoard(60)
	r := newMatchRig(t, s)
	s.shape = func(n, _ int) int {
		if n == 8 {
			r.gate = coolingBlockedProgram
		}
		return 0
	}
	if d := r.check(); d != pumpMatchPoll {
		t.Fatalf("delay %v", d)
	}
	if r.pumps.state("pump") != pumpPending || s.cur != 0 || s.lowest < 60 {
		t.Fatalf("state %v held %d lowest %d", r.pumps.state("pump"), s.cur, s.lowest)
	}
	if _, ok := r.record(t); ok {
		t.Fatal("recorded")
	}
}

func TestNoCheckDuringAMeasurementOrASave(t *testing.T) {
	s := newSimBoard(60)
	r := newMatchRig(t, s)
	r.e.coolMeasure.Running = true
	r.check()
	r.e.coolMeasure.Running = false
	r.e.coolOps.Lock()
	r.check()
	r.e.coolOps.Unlock()
	r.gate = coolingBlockedProgram
	r.ctrl.Tick()
	r.check()
	if len(r.sent()) != 0 || s.waits != 0 {
		t.Fatalf("checked anyway: %v", r.sent())
	}
}

func TestACheckEndsWhenTheMeasurementStarts(t *testing.T) {
	s := newSimBoard(60)
	r := newMatchRig(t, s)
	s.shape = func(n, _ int) int {
		if n == 8 {
			r.e.coolMeasure.Running = true
		}
		return 0
	}
	if d := r.check(); d != pumpMatchPoll || s.cur != 0 || r.pumps.state("pump") != pumpPending {
		t.Fatalf("delay %v held %d", d, s.cur)
	}
	r.e.coolMeasure.Running = false
}

func TestAnUnreleasedHeaderIsCheckedAgainLater(t *testing.T) {
	s := newSimBoard(60)
	s.denied = true
	r := newMatchRig(t, s)
	if d := r.check(); d != pumpMatchDenied {
		t.Fatalf("delay %v", d)
	}
	if r.pumps.state("pump") != pumpPending {
		t.Fatal("decided although not released")
	}
	if _, ok := r.record(t); ok {
		t.Fatal("recorded")
	}
	r.pumps.notReleased.Store("pump", struct{}{})
	if o := r.e.CoolingView().Outputs[0]; !o.NotReleased || o.NotMatched {
		t.Fatalf("%+v", o)
	}
	s.set(func(s *simBoard) { s.denied = false })
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatal("not proven after the release")
	}
}

func TestAChannelTheBoardDoesNotListFailsWithoutWriting(t *testing.T) {
	s := newSimBoard(60)
	s.controlKey = "/lpc/chip/0/control/2" // the board numbers its channels differently
	r := newMatchRig(t, s)
	r.check()
	if r.pumps.state("pump") != pumpNotMatched || len(r.sent()) != 0 || s.waits != 0 {
		t.Fatalf("state %v sent %v", r.pumps.state("pump"), r.sent())
	}
	if rec, ok := r.record(t); !ok || rec.Result != matchResultFailed {
		t.Fatalf("%+v", rec)
	}
}

func TestACancelledCheckGivesTheHeaderBack(t *testing.T) {
	s := newSimBoard(60)
	r := newMatchRig(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	s.shape = func(n, _ int) int {
		if n == 8 {
			cancel()
		}
		return 0
	}
	r.e.matchPump(ctx, r.ctrl, r.pumps, r.cfg, "pump")
	if s.cur != 0 || s.lowest < 60 || r.pumps.state("pump") != pumpPending {
		t.Fatalf("held %d lowest %d", s.cur, s.lowest)
	}
}

func TestTheDeviceRefusesToLowerAPump(t *testing.T) {
	s := newSimBoard(70)
	r := newMatchRig(t, s)
	d := &pumpMatchDevice{b: r.pumps, id: "pump", floor: 50, start: 70, known: true}
	for _, p := range []int{0, 29, 49, 69} {
		if err := d.SetDuty("pump", p); err == nil {
			t.Errorf("%d accepted", p)
		}
	}
	if err := d.SetDuty("pump", 70); err != nil || len(r.sent()) != 1 {
		t.Fatal(err)
	}
}

func TestOutputProblemOrderPutsNotMatchedBeforeNotReleased(t *testing.T) {
	for _, tc := range []struct {
		o    CoolingOutputView
		want string
	}{
		{CoolingOutputView{NotMatched: true, NotReleased: true}, "not_matched"},
		{CoolingOutputView{NotMatched: true}, "not_matched"},
		{CoolingOutputView{NotMatched: true, SensorLost: true}, "sensor_lost"},
		{CoolingOutputView{NotMatched: true, PumpDown: true}, "pump_down"},
		{CoolingOutputView{NotReleased: true}, "not_released"},
	} {
		if got := outputProblem(tc.o); got != tc.want {
			t.Errorf("%+v: %q", tc.o, got)
		}
	}
}

// An installation that regulated the pump before the check existed has no
// verdict. The pump is checked once through the real loop, and only then written.
func TestAnOldInstallationIsCheckedOnceAndThenRegulated(t *testing.T) {
	r := newLiveRig(t, false)
	os.Remove(filepath.Join(r.e.Store.Dir, coolingMatchFile)) // as before the check existed
	s := newSimBoard(60)
	useSimBoard(t, s)
	r.e.prov = NewSafeProvider(s)
	pumpMatchWait = func(ctx context.Context, _ time.Duration) error { // quicker than the loop's second
		time.Sleep(time.Millisecond)
		return s.wait(ctx, 0)
	}
	poll := pumpMatchPoll
	pumpMatchPoll = 20 * time.Millisecond
	t.Cleanup(func() { pumpMatchPoll = poll })
	stopWith(t, r.e)
	r.e.startCooling()
	deadline := time.Now().Add(15 * time.Second)
	c := r.e.coolingSnapshot()
	for c.pumps.state("pump") != pumpMatched && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.pumps.state("pump") != pumpMatched {
		t.Fatal("never proven")
	}
	for len(sentTo(s)) == 0 || sentTo(s)[len(sentTo(s))-1] != 70 {
		if time.Now().After(deadline) {
			t.Fatalf("not regulated after the proof: %v", sentTo(s))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if low := lowestOf(s); low < 60 {
		t.Fatalf("sent %d", low)
	}
	r.e.cancelFn()
	if _, err := os.Stat(filepath.Join(r.e.Store.Dir, coolingMatchFile)); err != nil {
		t.Fatal(err)
	}
}

func sentTo(s *simBoard) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.holds)
}

func lowestOf(s *simBoard) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lowest
}

func TestTestModeAndDemoNeverCheckOrWrite(t *testing.T) {
	for name, demo := range map[string]bool{"test mode": false, "demo": true} {
		r := newLiveRig(t, demo)
		s := newSimBoard(60)
		useSimBoard(t, s)
		stopWith(t, r.e)
		os.Remove(filepath.Join(r.e.Store.Dir, coolingMatchFile))
		r.e.prov = NewSafeProvider(s)
		file := coolingLiveExample
		if !demo {
			file = `{"mode":"preview"` + coolingLiveExample[len(`{"mode":"live"`):]
		}
		if err := os.WriteFile(r.file, []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
		r.e.startCooling()
		c := r.e.coolingSnapshot()
		if c.pumps != nil || c.ctrl == nil {
			t.Fatalf("%s: pumps %v", name, c.pumps)
		}
		c.ctrl.Tick()
		time.Sleep(50 * time.Millisecond)
		r.e.cancelFn()
		if len(sentTo(s)) != 0 || s.waits != 0 {
			t.Fatalf("%s: wrote %v", name, sentTo(s))
		}
		if _, err := os.Stat(filepath.Join(r.e.Store.Dir, coolingMatchFile)); err == nil {
			t.Fatalf("%s: results stored", name)
		}
	}
}

// stopWith ends the loop of e when the test ends, before the test seams are put back.
func stopWith(t *testing.T, e *Engine) {
	t.Cleanup(func() {
		e.cancelFn()
		if done := e.coolingSnapshot().done; done != nil {
			<-done
		}
	})
}

func firstTemp(v, otherwise float64) float64 {
	if v != 0 {
		return v
	}
	return otherwise
}

func TestAPumpAtFullSpeedIsProvenByLoweringIt(t *testing.T) {
	s := newSimBoard(100)
	r := newMatchRig(t, s)
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	if rec, ok := r.record(t); !ok || rec.Result != matchResultOK {
		t.Fatalf("%+v %v", rec, ok)
	}
	if h := r.steps(); len(h) != 2 || h[0] != 80 || h[1] != 100 {
		t.Fatalf("holds %v", h)
	}
	if s.cur != 0 || s.closed == 0 {
		t.Fatal("the header was not given back")
	}
}

func TestAPumpAtFullSpeedOnTheWrongChannelFails(t *testing.T) {
	for name, set := range map[string]func(*simBoard){
		"no reaction": func(s *simBoard) { s.follows = false },
		"wrong way":   func(s *simBoard) { s.inverse = true },
	} {
		s := newSimBoard(100)
		set(s)
		r := newMatchRig(t, s)
		r.check()
		if r.pumps.state("pump") != pumpNotMatched {
			t.Fatalf("%s: state %v", name, r.pumps.state("pump"))
		}
		if rec, ok := r.record(t); !ok || rec.Result != matchResultFailed {
			t.Fatalf("%s: %+v", name, rec)
		}
		if h := r.sent(); h[len(h)-1] != 100 || s.lowest < 80 || s.cur != 0 {
			t.Fatalf("%s: holds %v lowest %d held %d", name, h, s.lowest, s.cur)
		}
	}
}

func TestALoweredPumpNeverGetsLessThanThirtyOrItsMinimum(t *testing.T) {
	for _, board := range []int{91, 95, 100} {
		s := newSimBoard(board)
		r := newMatchRig(t, s)
		r.check()
		// The pump's minimum is 50, the lowered value is 20 points below the start.
		if s.lowest < max(board-20, 50, 30) {
			t.Errorf("board %d: sent %d, %v", board, s.lowest, r.sent())
		}
	}
}

func TestAPumpAtFullSpeedIsNotSlowedDownWhenHotOrBlind(t *testing.T) {
	for name, set := range map[string]func(*simBoard){
		"hot":   func(s *simBoard) { s.hot = cooling.PumpMeasureMaxTemp },
		"blind": func(s *simBoard) { s.noTemp = true },
	} {
		s := newSimBoard(100)
		r := newMatchRig(t, s)
		s.set(set)
		r.e.matchPump(context.Background(), r.ctrl, r.pumps, r.cfg, "pump")
		if h := r.sent(); len(h) != 0 {
			t.Fatalf("%s: written %v", name, h)
		}
		if r.pumps.state("pump") == pumpMatched || r.pumps.state("pump") == pumpNotMatched {
			t.Fatalf("%s: state %v", name, r.pumps.state("pump"))
		}
		if _, ok := r.record(t); ok {
			t.Fatalf("%s: a verdict was kept", name)
		}
	}
	// Hot, but the temperature is readable: the run ends unclear and is repeated later.
	s := newSimBoard(100)
	r := newMatchRig(t, s)
	s.set(func(s *simBoard) { s.hot = cooling.PumpMeasureMaxTemp })
	if d := r.check(); d != pumpMatchLater || len(r.sent()) != 0 {
		t.Fatalf("pause %v holds %v", d, r.sent())
	}
}

func TestACutShortRunPutsTheStartDutyBack(t *testing.T) {
	s := newSimBoard(100)
	r := newMatchRig(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lowered := 0
	s.shape = func(n, _ int) int {
		if s.cur == 80 {
			if lowered++; lowered > 1 {
				cancel()
			}
		}
		return 0
	}
	r.e.matchPump(ctx, r.ctrl, r.pumps, r.cfg, "pump")
	h := r.sent()
	if len(h) < 2 || h[0] != 80 || h[len(h)-1] != 100 {
		t.Fatalf("holds %v", h)
	}
	if s.cur != 0 || r.pumps.state("pump") == pumpMatched {
		t.Fatalf("held %d state %v", s.cur, r.pumps.state("pump"))
	}
}

func TestAPumpThatStopsWhenLoweredGoesBackAtOnce(t *testing.T) {
	s := newSimBoard(100)
	s.collapse = 90
	r := newMatchRig(t, s)
	r.check()
	h := r.sent()
	if h[len(h)-1] != 100 || r.pumps.state("pump") == pumpMatched {
		t.Fatalf("holds %v state %v", h, r.pumps.state("pump"))
	}
}

func TestThePumpCheckRefusesEveryLoweringButItsOwnTarget(t *testing.T) {
	s := newSimBoard(100)
	r := newMatchRig(t, s)
	d := &pumpMatchDevice{b: r.pumps, id: "pump", floor: 50, start: 100, known: true, low: 80}
	for _, p := range []int{79, 70, 50, 30, 0} {
		if d.SetDuty("pump", p) == nil {
			t.Errorf("%d accepted", p)
		}
	}
	for _, p := range []int{80, 100} {
		if err := d.SetDuty("pump", p); err != nil {
			t.Errorf("%d: %v", p, err)
		}
	}
	d.low = 0
	if d.SetDuty("pump", 80) == nil {
		t.Error("lowering accepted without a target")
	}
	d = &pumpMatchDevice{b: r.pumps, id: "pump", floor: 90, start: 100, known: true, low: 80}
	if d.SetDuty("pump", 80) == nil {
		t.Error("below the floor accepted")
	}
}
