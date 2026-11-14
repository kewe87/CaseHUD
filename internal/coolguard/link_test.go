package coolguard

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeChild is a guard process that ends when its pipe is closed or it is killed.
type fakeChild struct {
	mu     sync.Mutex
	lines  []message
	closed chan struct{}
	once   sync.Once
	killed atomic.Bool
	hang   bool // does not end on Close
	failW  bool
}

func newChild() *fakeChild { return &fakeChild{closed: make(chan struct{})} }

func (c *fakeChild) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failW {
		return 0, errors.New("pipe broken")
	}
	var m message
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &m); err != nil {
		return 0, err
	}
	c.lines = append(c.lines, m)
	return len(b), nil
}

func (c *fakeChild) Close() error {
	if !c.hang {
		c.once.Do(func() { close(c.closed) })
	}
	return nil
}
func (c *fakeChild) Wait() error { <-c.closed; return nil }
func (c *fakeChild) Kill() error {
	c.killed.Store(true)
	c.once.Do(func() { close(c.closed) })
	return nil
}
func (c *fakeChild) die() { c.once.Do(func() { close(c.closed) }) }

func (c *fakeChild) ops() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, m := range c.lines {
		out = append(out, m.Op)
	}
	return out
}

func (c *fakeChild) hasState() bool { _, ok := c.last(); return ok }

func (c *fakeChild) last() (message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.lines) == 0 {
		return message{}, false
	}
	return c.lines[len(c.lines)-1], true
}

type spawner struct {
	mu       sync.Mutex
	children []*fakeChild
	failFor  int // the first spawns fail
	count    int
}

func (s *spawner) spawn() (Child, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	if s.count <= s.failFor {
		return nil, errors.New("cannot start")
	}
	c := newChild()
	s.children = append(s.children, c)
	return c, nil
}

func (s *spawner) all() []*fakeChild {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*fakeChild(nil), s.children...)
}

func (s *spawner) started() int { return len(s.all()) }

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for range 1000 {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", what)
}

func testLink(s *spawner, src Source) *Link {
	return &Link{
		Spawn: s.spawn, Source: src,
		Poll: 5 * time.Millisecond, Every: time.Hour,
		RetryFrom: 5 * time.Millisecond, RetryUntil: 20 * time.Millisecond, ByeWait: 200 * time.Millisecond,
	}
}

var steady = State{Active: true, Safe: map[string]int{"port1-fan1": 70}, Last: map[string]int{"port1-fan1": 30}}

func TestArmStartsOneGuardWhateverTheCallsAndTellsItTheState(t *testing.T) {
	s := &spawner{}
	l := testLink(s, func() (State, bool) { return steady, true })
	for range 5 {
		l.Arm()
	}
	eventually(t, "state sent", func() bool { return len(s.all()) == 1 && s.all()[0].hasState() })
	time.Sleep(30 * time.Millisecond)
	if s.started() != 1 {
		t.Fatalf("%d guards", s.started())
	}
	m, _ := s.all()[0].last()
	if m.Op != opState || !m.Active || m.Safe["port1-fan1"] != 70 || m.Last["port1-fan1"] != 30 {
		t.Fatalf("%+v", m)
	}
	// Unchanged state is not repeated.
	n := len(s.all()[0].ops())
	time.Sleep(50 * time.Millisecond)
	if got := len(s.all()[0].ops()); got != n {
		t.Fatalf("repeated: %d -> %d", n, got)
	}
	l.Disarm()
}

func TestDisarmSaysGoodbyeAndTheGuardEnds(t *testing.T) {
	s := &spawner{}
	l := testLink(s, func() (State, bool) { return steady, true })
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	l.Disarm()
	c := s.all()[0]
	ops := c.ops()
	if ops[len(ops)-1] != opBye {
		t.Fatalf("ops %v", ops)
	}
	select {
	case <-c.closed:
	default:
		t.Fatal("the guard was not released")
	}
	if c.killed.Load() {
		t.Fatal("a guard that ended by itself was killed")
	}
	if l.Running() {
		t.Fatal("still running")
	}
	l.Disarm() // twice is fine
}

func TestAGuardThatDoesNotEndAfterGoodbyeIsKilled(t *testing.T) {
	s := &spawner{}
	l := testLink(s, func() (State, bool) { return steady, true })
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	s.all()[0].hang = true
	l.Disarm()
	if !s.all()[0].killed.Load() {
		t.Fatal("left a guard behind")
	}
}

func TestAGuardThatDiesIsStartedAgain(t *testing.T) {
	s := &spawner{}
	l := testLink(s, func() (State, bool) { return steady, true })
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	s.all()[0].die()
	eventually(t, "second guard", func() bool { return s.started() == 2 })
	eventually(t, "state sent to it", func() bool { return s.all()[1].hasState() })
	l.Disarm()
	if ops := s.all()[1].ops(); ops[len(ops)-1] != opBye {
		t.Fatalf("%v", ops)
	}
}

func TestAGuardThatCannotBeStartedIsTriedAgainWithPauses(t *testing.T) {
	s := &spawner{failFor: 3}
	l := testLink(s, func() (State, bool) { return steady, true })
	l.Arm()
	eventually(t, "started after failures", func() bool { return s.started() == 1 })
	l.Disarm()
}

func TestABrokenPipeStartsANewGuardAndNeverLeavesTheOldOne(t *testing.T) {
	s := &spawner{}
	var n atomic.Int32
	l := testLink(s, func() (State, bool) {
		st := steady
		st.Last = map[string]int{"port1-fan1": int(30 + n.Load())}
		return st, true
	})
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	s.all()[0].mu.Lock()
	s.all()[0].failW = true
	s.all()[0].mu.Unlock()
	n.Add(1) // a change is due, and cannot be sent
	eventually(t, "replaced", func() bool { return s.started() == 2 })
	if !s.all()[0].killed.Load() {
		t.Fatal("the guard with the broken pipe was left running")
	}
	l.Disarm()
}

func TestDisarmSoonIsCancelledByArm(t *testing.T) {
	s := &spawner{}
	l := testLink(s, func() (State, bool) { return steady, true })
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	l.DisarmSoon(600 * time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	l.Arm() // a new control takes over
	time.Sleep(900 * time.Millisecond)
	if !l.Running() || s.started() != 1 {
		t.Fatalf("running %v, %d guards", l.Running(), s.started())
	}
	for _, op := range s.all()[0].ops() {
		if op == opBye {
			t.Fatal("said goodbye to a guard that is still needed")
		}
	}
	l.DisarmSoon(20 * time.Millisecond)
	eventually(t, "goodbye", func() bool {
		ops := s.all()[0].ops()
		return !l.Running() && len(ops) > 0 && ops[len(ops)-1] == opBye
	})
}

func TestNothingIsSentWhileTheStateIsUnknown(t *testing.T) {
	s := &spawner{}
	var known atomic.Bool
	l := testLink(s, func() (State, bool) { return steady, known.Load() })
	l.Arm()
	eventually(t, "started", func() bool { return s.started() == 1 })
	time.Sleep(40 * time.Millisecond)
	if _, ok := s.all()[0].last(); ok {
		t.Fatal("sent a state nobody knows")
	}
	known.Store(true)
	eventually(t, "sent", func() bool { return s.all()[0].hasState() })
	l.Disarm()
}

func TestPokeSendsAChangeAtOnce(t *testing.T) {
	s := &spawner{}
	poke := make(chan struct{}, 1)
	var duty atomic.Int32
	duty.Store(30)
	l := testLink(s, func() (State, bool) {
		return State{Active: true, Safe: map[string]int{"a": 70}, Last: map[string]int{"a": int(duty.Load())}}, true
	})
	l.Poke = poke
	l.Poll = time.Hour
	l.Arm()
	eventually(t, "first state", func() bool { return len(s.all()) == 1 && s.all()[0].hasState() })
	duty.Store(90)
	poke <- struct{}{}
	eventually(t, "change sent", func() bool { m, _ := s.all()[0].last(); return m.Last["a"] == 90 })
	l.Disarm()
}
