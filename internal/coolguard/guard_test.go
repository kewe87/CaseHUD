package coolguard

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeFans records what the guard writes.
type fakeFans struct {
	mu      sync.Mutex
	taken   []string
	duty    map[string]int
	writes  int
	failIDs map[string]bool
	closed  bool
}

func newFakeFans() *fakeFans { return &fakeFans{duty: map[string]int{}, failIDs: map[string]bool{}} }

func (f *fakeFans) TakeControl(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.taken = append(f.taken, id)
	return nil
}

func (f *fakeFans) SetDuty(id string, p int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failIDs[id] {
		return errors.New("write failed")
	}
	f.duty[id] = p
	f.writes++
	return nil
}

func (f *fakeFans) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// harness wires a guard to a pipe and a parent that can end.
type harness struct {
	t      *testing.T
	pw     *io.PipeWriter
	gone   chan struct{}
	fans   *fakeFans
	opens  int
	rival  bool
	rivErr error
	failN  int // opening fails this many times first
	opts   Options
	mu     sync.Mutex
}

func newHarness(t *testing.T) *harness {
	pr, pw := io.Pipe()
	h := &harness{t: t, pw: pw, gone: make(chan struct{}), fans: newFakeFans()}
	h.opts = Options{
		In: pr, ParentGone: h.gone,
		Open: func(context.Context) (Fans, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.opens++
			if h.opens <= h.failN {
				return nil, errors.New("no controller")
			}
			return h.fans, nil
		},
		Rival:    func() (bool, error) { return h.rival, h.rivErr },
		EOFGrace: 200 * time.Millisecond, OpenEvery: time.Millisecond,
	}
	return h
}

func (h *harness) send(m message) {
	h.t.Helper()
	if _, err := h.pw.Write(encode(m)); err != nil {
		h.t.Error(err)
	}
}

func (h *harness) state(active bool, safe, last map[string]int) {
	h.t.Helper()
	h.send(message{Op: opState, State: State{Active: active, Safe: safe, Last: last}})
}

func (h *harness) run() Outcome {
	h.t.Helper()
	done := make(chan Outcome, 1)
	go func() { done <- Run(context.Background(), h.opts) }()
	select {
	case o := <-done:
		return o
	case <-time.After(5 * time.Second):
		h.t.Fatal("the guard did not end")
		return Stopped
	}
}

func TestCleanEndTouchesNothing(t *testing.T) {
	h := newHarness(t)
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, map[string]int{"port1-fan1": 20})
		h.send(message{Op: opBye})
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Clean {
		t.Fatalf("outcome %v", o)
	}
	if h.opens != 0 || h.fans.writes != 0 {
		t.Fatalf("opened %d, wrote %d", h.opens, h.fans.writes)
	}
}

// The goodbye is sent and the process ends at once: the end of the process may be
// seen before the goodbye has been read.
func TestGoodbyeStillInThePipeWhenTheProcessEnds(t *testing.T) {
	for range 50 {
		h := newHarness(t)
		go func() {
			h.state(true, map[string]int{"port1-fan1": 70}, nil)
			close(h.gone) // seen first
			time.Sleep(time.Millisecond)
			h.send(message{Op: opBye})
			h.pw.Close()
		}()
		if o := h.run(); o != Clean || h.opens != 0 {
			t.Fatalf("outcome %v, opened %d", o, h.opens)
		}
	}
}

func TestHardEndSetsEveryFanToItsSafeDutyAndNeverLowersOne(t *testing.T) {
	h := newHarness(t)
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70, "port1-fan2": 70, "port2-fan1": 100}, map[string]int{"port1-fan1": 20, "port1-fan2": 90, "port2-fan1": 0})
		time.Sleep(10 * time.Millisecond)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Acted {
		t.Fatalf("outcome %v", o)
	}
	want := map[string]int{"port1-fan1": 70, "port1-fan2": 90, "port2-fan1": 100}
	for id, d := range want {
		if h.fans.duty[id] != d {
			t.Errorf("%s: %d, want %d", id, h.fans.duty[id], d)
		}
	}
	if len(h.fans.taken) != 3 || !h.fans.closed {
		t.Fatalf("taken %v closed %v", h.fans.taken, h.fans.closed)
	}
}

func TestOnlyReportedFansAreTouchedAndNeverBelowTheFloor(t *testing.T) {
	h := newHarness(t)
	go func() {
		// A stopped fan with a safe duty that lets it stay stopped is still started.
		h.state(true, map[string]int{"port1-fan1": 0, "port1-fan2": 5}, map[string]int{"port1-fan1": 0, "port3-fan1": 80})
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Acted {
		t.Fatalf("outcome %v", o)
	}
	if h.fans.duty["port1-fan1"] != 30 || h.fans.duty["port1-fan2"] != 30 {
		t.Fatalf("%v", h.fans.duty)
	}
	if _, ok := h.fans.duty["port3-fan1"]; ok {
		t.Fatal("a fan that is not regulated was written")
	}
}

func TestNothingToDoWithoutAStateOrWithoutFans(t *testing.T) {
	for name, send := range map[string]func(*harness){
		"no state":    func(h *harness) {},
		"no fans":     func(h *harness) { h.state(true, nil, nil) },
		"handed over": func(h *harness) { h.state(false, map[string]int{"port1-fan1": 70}, nil) },
	} {
		h := newHarness(t)
		go func() {
			send(h)
			close(h.gone)
			h.pw.Close()
		}()
		if o := h.run(); o != Idle || h.opens != 0 {
			t.Errorf("%s: outcome %v, opened %d", name, o, h.opens)
		}
	}
}

func TestAnotherFanProgramIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.rival = true
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Idle || h.opens != 0 {
		t.Fatalf("outcome %v, opened %d", o, h.opens)
	}

	// If it cannot be told whether one runs, the controller is left alone too.
	h = newHarness(t)
	h.rivErr = errors.New("no process list")
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Idle || h.opens != 0 {
		t.Fatalf("outcome %v, opened %d", o, h.opens)
	}
}

// The pipe closes but CaseHUD lives: not a hard end, nothing is written.
func TestClosedPipeWhileCaseHUDLivesDoesNothing(t *testing.T) {
	h := newHarness(t)
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		h.pw.Close()
	}()
	if o := h.run(); o != Stopped || h.opens != 0 {
		t.Fatalf("outcome %v, opened %d", o, h.opens)
	}
}

// The pipe closes first and the process follows within the grace time.
func TestClosedPipeThenTheEndOfTheProcessActs(t *testing.T) {
	h := newHarness(t)
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		h.pw.Close()
		time.Sleep(20 * time.Millisecond)
		close(h.gone)
	}()
	if o := h.run(); o != Acted || h.fans.duty["port1-fan1"] != 70 {
		t.Fatalf("outcome %v %v", o, h.fans.duty)
	}
}

func TestTheControllerIsTriedAgainWhileItIsBusy(t *testing.T) {
	h := newHarness(t)
	h.failN = 3
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Acted || h.opens != 4 {
		t.Fatalf("outcome %v after %d opens", o, h.opens)
	}

	h = newHarness(t)
	h.failN = 1000
	h.opts.OpenTries = 3
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Failed || h.opens != 3 {
		t.Fatalf("outcome %v after %d opens", o, h.opens)
	}
}

func TestOneFanThatFailsDoesNotStopTheOthers(t *testing.T) {
	h := newHarness(t)
	h.fans.failIDs["port1-fan1"] = true
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70, "port1-fan2": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Acted || h.fans.duty["port1-fan2"] != 70 {
		t.Fatalf("outcome %v %v", o, h.fans.duty)
	}
}

func TestActingIsMarkedAndReleased(t *testing.T) {
	h := newHarness(t)
	var began, ended int
	h.opts.Acting = func() func() { began++; return func() { ended++ } }
	go func() {
		h.state(true, map[string]int{"port1-fan1": 70}, nil)
		close(h.gone)
		h.pw.Close()
	}()
	h.run()
	if began != 1 || ended != 1 {
		t.Fatalf("%d %d", began, ended)
	}
	// A clean end is not an act.
	h = newHarness(t)
	began, ended = 0, 0
	h.opts.Acting = func() func() { began++; return func() { ended++ } }
	go func() {
		h.send(message{Op: opBye})
		close(h.gone)
		h.pw.Close()
	}()
	h.run()
	if began != 0 {
		t.Fatal("marked without acting")
	}
}

func TestOnlyTheNewestStateCountsAndJunkIsIgnored(t *testing.T) {
	h := newHarness(t)
	go func() {
		h.pw.Write([]byte("not json\n"))
		h.pw.Write([]byte(`{"v":2,"op":"state","active":true,"safe":{"port9-fan9":100}}` + "\n"))
		h.state(true, map[string]int{"port1-fan1": 40}, nil)
		h.state(true, map[string]int{"port1-fan1": 80}, nil)
		h.pw.Write([]byte(strings.Repeat("x", 3*1024) + "\n"))
		close(h.gone)
		h.pw.Close()
	}()
	if o := h.run(); o != Acted || h.fans.duty["port1-fan1"] != 80 || len(h.fans.duty) != 1 {
		t.Fatalf("outcome %v %v", o, h.fans.duty)
	}
}

func TestCancelEndsTheWait(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- Run(ctx, h.opts) }()
	cancel()
	select {
	case o := <-done:
		if o != Stopped {
			t.Fatalf("outcome %v", o)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("still waiting")
	}
}
