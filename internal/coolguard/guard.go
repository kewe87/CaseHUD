// Package coolguard keeps the fans of the TL controller from being left low when
// CaseHUD ends without cleaning up.
//
// The controller keeps the last duty it was given, so a CaseHUD that is killed or
// crashes leaves the fans wherever they were. CaseHUD therefore starts a second
// process of its own executable while it regulates (see Link). That process only
// waits: for the end of CaseHUD, and for CaseHUD to say that it ended cleanly. It
// opens the controller only if CaseHUD ended without saying so, writes the safe
// duty of every fan (never below what the fan ran at), and ends.
package coolguard

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
)

const (
	protocolVersion = 1
	// maxLine bounds one message; a state of 64 fans is far below it.
	maxLine = 64 << 10

	opState = "state"
	opBye   = "bye"
)

// State is what CaseHUD tells the guard about the fans, as often as it changes.
type State struct {
	// Active is false while CaseHUD has let go of the controller on purpose, for
	// another program that is in charge. The guard then leaves it alone.
	Active bool `json:"active"`
	// Safe is the duty every regulated fan is left at, by fan name.
	Safe map[string]int `json:"safe"`
	// Last is the highest duty a fan has been given or is being given.
	Last map[string]int `json:"last"`
}

func (s State) equal(o State) bool {
	return s.Active == o.Active && mapsEqual(s.Safe, o.Safe) && mapsEqual(s.Last, o.Last)
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

type message struct {
	V  int    `json:"v"`
	Op string `json:"op"`
	State
}

func encode(m message) []byte {
	m.V = protocolVersion
	b, _ := json.Marshal(m)
	return append(b, '\n')
}

// Fans is the part of the fan controller the guard needs.
type Fans interface {
	TakeControl(id string) error
	SetDuty(id string, percent int) error
	Close() error
}

// Outcome says what a run of the guard did.
type Outcome int

const (
	// Clean: CaseHUD said goodbye. Nothing was touched.
	Clean Outcome = iota
	// Idle: CaseHUD ended without goodbye, but there was nothing the guard may do.
	Idle
	// Acted: the fans were set to their safe duty.
	Acted
	// Failed: the fans could not be reached.
	Failed
	// Stopped: the run was cancelled, or CaseHUD closed the pipe while it lived.
	Stopped
)

func (o Outcome) String() string {
	return [...]string{"clean", "idle", "acted", "failed", "stopped"}[o]
}

// Options wire the guard to its surroundings; tests replace all of them.
type Options struct {
	// In carries the messages of CaseHUD.
	In io.Reader
	// ParentGone is closed when the CaseHUD process has ended.
	ParentGone <-chan struct{}
	// Open opens the fan controller. It is called only when the guard acts.
	Open func(ctx context.Context) (Fans, error)
	// Rival tells whether another fan program runs; the guard then stays away.
	Rival func() (bool, error)
	// Acting marks the time the guard works on the controller, so that a CaseHUD
	// starting meanwhile waits for it. It returns the release. May be nil.
	Acting func() (release func())
	// Log receives what the guard did. May be nil.
	Log func(format string, args ...any)

	// EOFGrace is how long the guard waits for the end of CaseHUD after its pipe
	// closed without a goodbye. Default 3 s.
	EOFGrace time.Duration
	// OpenTries and OpenEvery bound the attempts to reach the controller.
	// Defaults: 10 times, 2 s apart.
	OpenTries int
	OpenEvery time.Duration
}

func (o *Options) logf(format string, args ...any) {
	if o.Log != nil {
		o.Log(format, args...)
	}
}

// Run waits for the end of CaseHUD and acts if it ended without saying goodbye.
func Run(ctx context.Context, o Options) Outcome {
	if o.EOFGrace <= 0 {
		o.EOFGrace = 3 * time.Second
	}
	if o.OpenTries <= 0 {
		o.OpenTries = 10
	}
	if o.OpenEvery <= 0 {
		o.OpenEvery = 2 * time.Second
	}
	in := newInbox(o.In, o.logf)
	// The goodbye may still be in the pipe when the process has ended: its write
	// end closes with the process, so the reader reaches the end of the stream
	// shortly after.
	ended := func() Outcome {
		select {
		case <-in.eof:
		case <-time.After(o.EOFGrace):
		case <-ctx.Done():
			return Stopped
		}
		if in.saidBye() {
			o.logf("CaseHUD ended cleanly, nothing to do")
			return Clean
		}
		o.logf("CaseHUD ended without saying goodbye")
		st, ok := in.last()
		return act(ctx, &o, st, ok)
	}
	select {
	case <-ctx.Done():
		return Stopped
	case <-in.bye:
		o.logf("CaseHUD ended cleanly, nothing to do")
		return Clean
	case <-o.ParentGone:
		return ended()
	case <-in.eof:
		// The pipe closed while CaseHUD may still live: give the end of the
		// process a moment, and do nothing if it does not come.
		select {
		case <-o.ParentGone:
			return ended()
		case <-time.After(o.EOFGrace):
			if in.saidBye() {
				return Clean
			}
			o.logf("the pipe to CaseHUD closed but CaseHUD still runs; the guard ends")
			return Stopped
		case <-ctx.Done():
			return Stopped
		}
	}
}

// act sets the fans. The duty of a fan is its safe duty, or what it ran at if that
// was more, and never less than the floor of the control.
func act(ctx context.Context, o *Options, st State, ok bool) Outcome {
	switch {
	case !ok || len(st.Safe) == 0:
		o.logf("no fans were reported, nothing to do")
		return Idle
	case !st.Active:
		o.logf("CaseHUD had handed the controller over, leaving it alone")
		return Idle
	}
	if o.Rival != nil {
		if rival, err := o.Rival(); err != nil {
			o.logf("could not check for another fan program (%v), leaving the controller alone", err)
			return Idle
		} else if rival {
			o.logf("another fan program runs, leaving the controller alone")
			return Idle
		}
	}
	if o.Acting != nil {
		defer o.Acting()()
	}
	var dev Fans
	var err error
	for try := 1; ; try++ {
		dev, err = o.Open(ctx)
		if err == nil {
			break
		}
		o.logf("opening the fan controller (try %d of %d): %v", try, o.OpenTries, err)
		if try >= o.OpenTries {
			return Failed
		}
		select {
		case <-time.After(o.OpenEvery):
		case <-ctx.Done():
			return Stopped
		}
	}
	defer dev.Close()
	ids := make([]string, 0, len(st.Safe))
	for id := range st.Safe {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	failed := 0
	for _, id := range ids {
		duty := min(max(st.Safe[id], st.Last[id], cooling.LowestSafeDuty), 100)
		if err := dev.TakeControl(id); err != nil {
			o.logf("%s: taking control: %v", id, err)
		}
		if err := dev.SetDuty(id, duty); err != nil {
			o.logf("%s: setting %d %%: %v", id, duty, err)
			failed++
			continue
		}
		o.logf("%s set to %d %%", id, duty)
	}
	if failed == len(ids) {
		return Failed
	}
	return Acted
}

// inbox reads the messages of CaseHUD in the background and keeps the newest state.
type inbox struct {
	bye chan struct{} // closed on the goodbye
	eof chan struct{} // closed when the stream ends

	mu     sync.Mutex
	state  State
	have   bool
	goodby bool
	logf   func(format string, args ...any)
}

func newInbox(r io.Reader, logf func(string, ...any)) *inbox {
	in := &inbox{bye: make(chan struct{}), eof: make(chan struct{}), logf: logf}
	go in.read(r)
	return in
}

func (in *inbox) read(r io.Reader) {
	defer close(in.eof)
	if r == nil {
		return
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), maxLine)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m.V != protocolVersion {
			continue
		}
		switch m.Op {
		case opState:
			in.mu.Lock()
			first := !in.have
			in.state, in.have = State{Active: m.Active, Safe: maps.Clone(m.Safe), Last: maps.Clone(m.Last)}, true
			in.mu.Unlock()
			if first {
				in.logf("armed: %d fans, in charge %v", len(m.Safe), m.Active)
			}
		case opBye:
			in.mu.Lock()
			already := in.goodby
			in.goodby = true
			in.mu.Unlock()
			if !already {
				close(in.bye)
			}
		}
	}
}

func (in *inbox) saidBye() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.goodby
}

func (in *inbox) last() (State, bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.state, in.have
}
