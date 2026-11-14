package app

import (
	"hash/fnv"
	"log"
	"math"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// A regulation that follows a frozen reading is dangerous: the part gets hot and
// the fans keep their speed. The cooling therefore only sees readings that are
// fresh (SafeProvider.Fresh).
//
// A reading is stale if its source says so: the sensor service tells how long ago
// it read the hardware. Where the source says nothing, a reading counts as frozen
// only when the whole snapshot has not changed in any number for a long time. A
// single steady value proves nothing: coolant can sit at one temperature for an
// hour. A machine whose every load, clock, voltage and speed is exactly the same
// for a minute is not measured any more.
const (
	sensorMaxAge = 20 * time.Second
	// With sensorRichCount readings or more, a minute without any change is a
	// freeze. With fewer there is little that could move, so it takes longer.
	sensorFrozenRich   = time.Minute
	sensorFrozenSparse = 10 * time.Minute
	sensorRichCount    = 8
)

// freshWatch follows the snapshots of one provider.
type freshWatch struct {
	mu      sync.Mutex
	now     func() time.Time // time.Now, replaced in tests
	sig     uint64
	since   time.Time // when the snapshot last differed from the one before
	fetched time.Time // when the last good snapshot came in
	count   int
	aged    bool // the snapshot carries its own age

	frozenSeen bool // the transitions below are logged once
	staleSeen  int
	degraded   bool // the last filter left readings out because they are old
}

func (w *freshWatch) clock() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

// signature identifies a snapshot by every key and value in it.
func signature(rs []metrics.Reading) uint64 {
	h := fnv.New64a()
	var b [8]byte
	for _, r := range rs {
		h.Write([]byte(r.Key))
		bits := math.Float64bits(r.Value)
		for i := range b {
			b[i] = byte(bits >> (8 * i))
		}
		h.Write(b[:])
	}
	return h.Sum64()
}

// observe takes a snapshot that was just fetched. After a failed fetch it takes
// err: there is nothing to judge then, and the provider has dropped its readings.
func (w *freshWatch) observe(rs []metrics.Reading, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock()
	if err != nil || len(rs) == 0 {
		w.sig, w.since, w.count, w.aged = 0, now, 0, false
		return
	}
	if sig := signature(rs); sig != w.sig || w.since.IsZero() {
		w.sig, w.since = sig, now
	}
	w.fetched, w.count = now, len(rs)
	w.aged = false
	for _, r := range rs {
		if r.AgeKnown {
			w.aged = true
			break
		}
	}
}

// frozen tells whether the snapshot has not changed for too long to be believed.
func (w *freshWatch) frozen(now time.Time) bool {
	if w.aged || w.count == 0 || w.since.IsZero() {
		return false
	}
	limit := sensorFrozenRich
	if w.count < sensorRichCount {
		limit = sensorFrozenSparse
	}
	return now.Sub(w.since) >= limit
}

// filter returns the readings the cooling may use.
func (w *freshWatch) filter(rs []metrics.Reading) []metrics.Reading {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock()
	if fz := w.frozen(now); fz != w.frozenSeen {
		w.frozenSeen = fz
		if fz {
			log.Printf("cooling: no sensor value has changed for %v, the readings count as missing", now.Sub(w.since).Round(time.Second))
		} else {
			log.Print("cooling: the sensor values change again")
		}
	}
	if w.frozenSeen {
		w.degraded = true
		return nil
	}
	// The service tells the age at its last answer; the time since then adds to it.
	var wait time.Duration
	if !w.fetched.IsZero() {
		wait = max(now.Sub(w.fetched), 0)
	}
	// A refresh that has not delivered for this long makes every reading old,
	// whether or not the source tells an age.
	overdue := !w.fetched.IsZero() && wait > sensorMaxAge
	out := make([]metrics.Reading, 0, len(rs))
	stale := 0
	for _, r := range rs {
		if overdue || r.AgeKnown && r.Age+wait > sensorMaxAge {
			stale++
			continue
		}
		out = append(out, r)
	}
	if stale != w.staleSeen {
		if stale > 0 {
			log.Printf("cooling: %d sensor values are older than %v at the sensor service and count as missing", stale, sensorMaxAge)
		} else {
			log.Print("cooling: the sensor values are current again")
		}
		w.staleSeen = stale
	}
	w.degraded = stale > 0
	return out
}

// Fresh returns the readings that are current, without waiting for a slow refresh. The cooling uses these and not
// All: a frozen value looks like a valid one.
func (s *SafeProvider) Fresh() []metrics.Reading {
	var rs []metrics.Reading
	if s.mu.TryRLock() {
		rs = append(rs, s.p.All()...)
		s.mu.RUnlock()
	} else {
		// A refresh is in progress and may take long: the cooling must not wait for it.
		// What it has is the snapshot before, which turns old by itself.
		s.smu.Lock()
		rs = s.snap
		s.smu.Unlock()
	}
	// Only the sensor service may drive the cooling: values from any other
	// endpoint count as missing, as when the sensors fail.
	trusted := rs[:0:0]
	for _, r := range rs {
		if !r.DisplayOnly {
			trusted = append(trusted, r)
		}
	}
	return s.fresh.filter(trusted)
}

// Stale tells whether the last call of Fresh left readings out because they were
// old or frozen, as opposed to missing.
func (s *SafeProvider) Stale() bool {
	s.fresh.mu.Lock()
	defer s.fresh.mu.Unlock()
	return s.fresh.degraded
}
