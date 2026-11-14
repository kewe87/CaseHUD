package coolguard

import (
	"context"
	"io"
	"sync"
	"time"
)

// Child is a running guard process as CaseHUD sees it.
type Child interface {
	// Write sends a line of the protocol.
	io.Writer
	// Close ends the pipe to the guard, which then sees the end of the stream.
	Close() error
	// Wait returns when the guard has ended.
	Wait() error
	// Kill ends the guard.
	Kill() error
}

// Source says what the fans are at. ok is false while that is not known.
type Source func() (st State, ok bool)

// Link keeps one guard process running while the fans are regulated, and tells it
// what they are at. It starts a new one if the guard dies, and says goodbye to it
// when the regulation ended cleanly.
type Link struct {
	Spawn  func() (Child, error)
	Source Source
	// Poke, if set, wakes the link when the state has changed.
	Poke <-chan struct{}
	Log  func(format string, args ...any)

	// Intervals; defaults below.
	Every      time.Duration // state is sent at least this often (default 10 s)
	Poll       time.Duration // state is looked at this often (default 1 s)
	RetryFrom  time.Duration // pause after a failed start (default 2 s)
	RetryUntil time.Duration // longest pause (default 1 min)
	ByeWait    time.Duration // how long a guard gets to end after the goodbye (default 3 s)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	timer  *time.Timer
}

func (l *Link) logf(format string, args ...any) {
	if l.Log != nil {
		l.Log(format, args...)
	}
}

func orDefault(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}

// Arm makes sure a guard runs. It is cheap to call again and never starts a
// second guard.
func (l *Link) Arm() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	if l.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel, l.done = cancel, make(chan struct{})
	go l.run(ctx, l.done)
}

// Running tells whether a guard is meant to run.
func (l *Link) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cancel != nil
}

// DisarmSoon says goodbye to the guard after d, unless Arm comes first. A control
// that is replaced by a new one ends and starts within that time and keeps its
// guard.
func (l *Link) DisarmSoon(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel == nil {
		return
	}
	if l.timer != nil {
		l.timer.Stop()
	}
	l.timer = time.AfterFunc(d, l.Disarm)
}

// Disarm says goodbye to the guard and waits for it to end. Call it only when the
// fans were left at their safe duty: the guard then does nothing at all.
func (l *Link) Disarm() {
	l.mu.Lock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	cancel, done := l.cancel, l.done
	l.cancel, l.done = nil, nil
	l.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(orDefault(l.ByeWait, 3*time.Second) + 2*time.Second):
		l.logf("the cooling guard did not end in time")
	}
}

func (l *Link) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	pause := orDefault(l.RetryFrom, 2*time.Second)
	longest := orDefault(l.RetryUntil, time.Minute)
	for ctx.Err() == nil {
		child, err := l.Spawn()
		if err != nil {
			l.logf("starting the cooling guard failed: %v", err)
			if !sleep(ctx, pause) {
				return
			}
			pause = min(pause*2, longest)
			continue
		}
		started := time.Now()
		if l.serve(ctx, child) {
			return
		}
		l.logf("the cooling guard ended on its own, starting it again")
		if time.Since(started) > 2*time.Minute {
			pause = orDefault(l.RetryFrom, 2*time.Second)
		}
		if !sleep(ctx, pause) {
			return
		}
		pause = min(pause*2, longest)
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// serve feeds one guard process. It returns true after a goodbye, false if the
// guard went away by itself.
func (l *Link) serve(ctx context.Context, child Child) (bye bool) {
	exited := make(chan struct{})
	go func() {
		_ = child.Wait()
		close(exited)
	}()
	var sent State
	var sentAt time.Time
	haveSent := false
	send := func(force bool) bool {
		st, ok := l.Source()
		if !ok {
			return true
		}
		if !force && haveSent && st.equal(sent) && time.Since(sentAt) < orDefault(l.Every, 10*time.Second) {
			return true
		}
		if _, err := child.Write(encode(message{Op: opState, State: st})); err != nil {
			return false
		}
		sent, sentAt, haveSent = st, time.Now(), true
		return true
	}
	ticker := time.NewTicker(orDefault(l.Poll, time.Second))
	defer ticker.Stop()
	if !send(true) {
		_ = child.Kill()
		<-exited
		return false
	}
	for {
		select {
		case <-ctx.Done():
			_, _ = child.Write(encode(message{Op: opBye}))
			child.Close()
			select {
			case <-exited:
			case <-time.After(orDefault(l.ByeWait, 3*time.Second)):
				_ = child.Kill()
				<-exited
			}
			return true
		case <-exited:
			child.Close()
			return false
		case <-ticker.C:
		case <-l.Poke:
		}
		if !send(false) {
			_ = child.Kill()
			<-exited
			return false
		}
	}
}
