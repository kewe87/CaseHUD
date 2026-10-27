package app

import (
	"testing"
	"time"
)

type watchRig struct {
	w       *watcher
	fp      string
	failing time.Time
	scanBad bool
	rescans int
	now     time.Time
}

func newWatchRig() *watchRig {
	r := &watchRig{fp: "a", now: time.Unix(1000, 0)}
	r.w = &watcher{
		fingerprint: func() (string, error) { return r.fp, nil },
		failingFor:  func() time.Time { return r.failing },
		scanFailed:  func() bool { return r.scanBad },
		rescan:      func() { r.rescans++ },
	}
	r.w.step(r.now) // baseline
	return r
}

func (r *watchRig) advance(d time.Duration) bool {
	r.now = r.now.Add(d)
	return r.w.step(r.now)
}

func TestWatchNoChangeNoRescan(t *testing.T) {
	r := newWatchRig()
	for i := 0; i < 20; i++ {
		r.advance(2 * time.Second)
	}
	if r.rescans != 0 {
		t.Fatalf("rescans = %d", r.rescans)
	}
}

func TestWatchArrivalIsDebounced(t *testing.T) {
	r := newWatchRig()
	r.fp = "a\nb"
	if r.advance(2*time.Second) || r.rescans != 0 {
		t.Fatal("rescanned on first sight")
	}
	// The list flaps back before the debounce time: nothing happens.
	r.fp = "a"
	r.advance(time.Second)
	r.fp = "a\nb"
	r.advance(time.Second)
	if r.rescans != 0 {
		t.Fatal("rescanned while flapping")
	}
	r.advance(2 * time.Second)
	if r.rescans != 1 {
		t.Fatalf("rescans = %d, want 1", r.rescans)
	}
	r.advance(2 * time.Second)
	if r.rescans != 1 {
		t.Fatal("rescanned again for the same list")
	}
	// Removal works the same way.
	r.fp = "a"
	r.advance(time.Second)
	r.advance(2 * time.Second)
	if r.rescans != 2 {
		t.Fatalf("removal: rescans = %d", r.rescans)
	}
}

func TestWatchResumeWaitsThenRescans(t *testing.T) {
	r := newWatchRig()
	r.advance(2 * time.Second)
	r.advance(30 * time.Minute) // the loop was frozen: the PC slept
	if r.rescans != 0 {
		t.Fatal("rescanned before the wake-up delay")
	}
	r.advance(2 * time.Second)
	if r.rescans != 0 {
		t.Fatal("rescanned too early after wake-up")
	}
	r.advance(2 * time.Second)
	if r.rescans != 1 {
		t.Fatalf("rescans = %d, want 1", r.rescans)
	}
}

func TestWatchPersistentDeviceErrorsBackOff(t *testing.T) {
	r := newWatchRig()
	r.failing = r.now // failing since the beginning
	for i := 0; i < 3; i++ {
		r.advance(2 * time.Second)
	}
	if r.rescans != 0 {
		t.Fatal("rescanned before the grace period")
	}
	r.advance(4 * time.Second)
	if r.rescans != 1 {
		t.Fatalf("rescans = %d, want 1", r.rescans)
	}
	// Still failing: the next attempts are spaced out (10 s, then 20 s, ...).
	n := 0
	for r.rescans < 3 && n < 100 {
		r.advance(2 * time.Second)
		n++
	}
	if gap := time.Duration(n) * 2 * time.Second; gap < 28*time.Second {
		t.Fatalf("second and third rescan after only %v", gap)
	}
	// Recovery resets the back-off.
	r.failing = time.Time{}
	r.advance(2 * time.Second)
	if r.w.backoff != 0 {
		t.Fatal("back-off not reset")
	}
}

func TestWatchRetriesFailedDiscovery(t *testing.T) {
	r := newWatchRig()
	r.scanBad = true
	r.advance(2 * time.Second)
	if r.rescans != 1 {
		t.Fatalf("rescans = %d, want 1", r.rescans)
	}
	r.fp = "" // nothing plugged in: no point retrying
	r.advance(2 * time.Second)
	r.advance(2 * time.Second)
	before := r.rescans
	for i := 0; i < 10; i++ {
		r.advance(2 * time.Second)
	}
	if r.rescans != before {
		t.Fatal("retried with no device present")
	}
}

func TestSetErrTracksDeviceFailureRun(t *testing.T) {
	w := &worker{}
	w.setErr(ErrDevice, errTest("x"))
	first := w.devErrSince
	if first.IsZero() {
		t.Fatal("no start recorded")
	}
	w.setErr(ErrDevice, errTest("y"))
	if !w.devErrSince.Equal(first) {
		t.Fatal("run start moved while still failing")
	}
	w.setErr(ErrContent, errTest("z"))
	if !w.devErrSince.IsZero() {
		t.Fatal("not cleared by another class")
	}
	w.setErr(ErrDevice, errTest("x"))
	w.setErr("", nil)
	if !w.devErrSince.IsZero() {
		t.Fatal("not cleared by success")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

// next advances in 2 s steps (below the sleep detection) until a rescan.
func (r *watchRig) next() bool {
	for i := 0; i < 100; i++ {
		if r.advance(2 * time.Second) {
			return true
		}
	}
	return false
}

func TestWatcherGivesUpAfterFiveRetries(t *testing.T) {
	r := newWatchRig()
	var manual int64
	var gave bool
	r.w.manual = func() int64 { return manual }
	r.w.onGaveUp = func(v bool) { gave = v }
	r.scanBad = true
	for i := 0; i < watchMaxRetries; i++ {
		if !r.next() {
			t.Fatalf("retry %d did not rescan", i+1)
		}
	}
	if !gave || r.rescans != watchMaxRetries {
		t.Fatalf("gaveUp %v after %d rescans", gave, r.rescans)
	}
	// Given up: only a rare slow try, not the usual back-off rhythm.
	before := r.rescans
	for i := 0; i < int((watchBackoffMax-10*time.Second)/(2*time.Second)); i++ {
		if r.advance(2 * time.Second) {
			t.Fatal("rescanned soon after giving up")
		}
	}
	slow := 0
	for i := 0; i < int(watchSlowRetry/(2*time.Second))*2; i++ {
		if r.advance(2 * time.Second) {
			slow++
		}
	}
	if slow < 1 || slow > 3 || !gave || r.rescans != before+slow {
		t.Fatalf("slow tries = %d (gaveUp %v)", slow, gave)
	}
	manual++ // "Neu suchen"
	if !r.next() || gave {
		t.Fatal("manual search did not re-arm the watcher")
	}
	for !gave {
		r.next()
	}
	r.fp = "b" // a plug event starts afresh
	for i := 0; i < 5; i++ {
		r.advance(2 * time.Second)
	}
	if gave {
		t.Fatal("plug event did not re-arm the watcher")
	}
}
