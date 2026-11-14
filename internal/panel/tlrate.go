package panel

import (
	"log"
	"sync"
	"time"
)

// Brightness and rotation of the TL LCD live in the fan's flash. Every change
// is therefore rate-limited per device: at most one write per tlMinInterval,
// changes in between are coalesced to the last value and applied when the
// interval is over, and a device gets at most tlDailyMax such writes per day;
// after that nothing is written until CaseHUD restarts.
const tlDailyMax = 200

var (
	tlMinInterval = 2 * time.Second
	tlNow         = time.Now
)

// The day count, the cap and the time of the last write belong to the fan, not
// to the object that talks to it: a rescan or a re-plug creates a new object
// for the same fan and must not give it a fresh allowance. They are kept per
// SettingsCache key for the life of the process.
type tlRateSnap struct {
	last   time.Time
	day    string
	count  int
	capped bool
}

var (
	tlRateMu  sync.Mutex
	tlRateMem = map[string]tlRateSnap{}
)

// attach binds the limiter to the fan's key and takes over what an earlier
// object for the same fan had used.
func (r *tlRate) attach(key string) {
	r.key = key
	if key == "" {
		return
	}
	tlRateMu.Lock()
	s := tlRateMem[key]
	tlRateMu.Unlock()
	r.last, r.day, r.count, r.capped = s.last, s.day, s.count, s.capped
}

// remember stores the persistent part for the next object of this fan.
func (r *tlRate) remember() {
	if r.key == "" {
		return
	}
	tlRateMu.Lock()
	tlRateMem[r.key] = tlRateSnap{last: r.last, day: r.day, count: r.count, capped: r.capped}
	tlRateMu.Unlock()
}

// tlRate is the state of the limiter of one fan (guarded by tlChain).
type tlRate struct {
	last     time.Time // of the last write made for a change
	pendB    *int      // coalesced values waiting for the interval
	pendR    *int
	timer    *time.Timer
	key      string // SettingsCache key of the fan, "" until known
	day      string // date the count belongs to
	count    int
	capped   bool
	capKnown bool // the cap was logged
}

// changeSettings asks for a new brightness and/or rotation (nil = unchanged).
// The caller holds the chain. The write happens now, or later when the
// interval since the last one has not passed; the result of a postponed write
// is only logged.
func (p *tlLCD) changeSettings(b, r *int) error {
	if b != nil {
		v := *b
		if p.held && v == p.brightness {
			p.rate.pendB = nil
		} else {
			p.rate.pendB = &v
		}
	}
	if r != nil {
		v := *r
		if p.held && v == p.rotation {
			p.rate.pendR = nil
		} else {
			p.rate.pendR = &v
		}
	}
	if p.rate.pendB == nil && p.rate.pendR == nil {
		return nil
	}
	if wait := tlMinInterval - tlNow().Sub(p.rate.last); wait > 0 {
		if p.rate.timer == nil {
			p.rate.timer = time.AfterFunc(wait, p.applyPending)
		}
		return nil
	}
	return p.flushPending()
}

// applyPending is the timer: it writes the coalesced values.
func (p *tlLCD) applyPending() {
	tlChain.enter()
	defer tlChain.leave()
	p.rate.timer = nil
	if p.closed {
		return
	}
	if wait := tlMinInterval - tlNow().Sub(p.rate.last); wait > 0 {
		p.rate.timer = time.AfterFunc(wait, p.applyPending)
		return
	}
	if err := p.flushPending(); err != nil {
		log.Println("TL LCD: postponed settings write failed:", err)
	}
}

// flushPending writes what is waiting, if the daily cap allows. The caller
// holds the chain.
func (p *tlLCD) flushPending() error {
	b, r := p.rate.pendB, p.rate.pendR
	p.rate.pendB, p.rate.pendR = nil, nil
	if b == nil && r == nil {
		return nil
	}
	now := tlNow()
	if !p.spendWrite(now) {
		return nil
	}
	p.rate.last = now
	p.rate.remember()
	if b != nil {
		p.brightness = *b
	}
	if r != nil {
		p.rotation = *r
	}
	return p.push(tlViewLive, true)
}

// spendWrite books one settings write against the daily cap and reports whether
// it may be made. Every write of brightness/rotation goes through it, also the
// restore after a stalled stream. The caller holds the chain.
func (p *tlLCD) spendWrite(now time.Time) bool {
	if day := now.Format("2006-01-02"); day != p.rate.day {
		p.rate.day, p.rate.count = day, 0
	}
	if p.rate.capped || p.rate.count >= tlDailyMax {
		p.rate.capped = true
		p.rate.remember()
		if !p.rate.capKnown {
			p.rate.capKnown = true
			log.Printf("TL LCD %s: %d brightness/rotation writes today: no more are made until CaseHUD restarts (they wear the display's flash)", p.ID(), tlDailyMax)
		}
		return false
	}
	p.rate.count++
	p.rate.remember()
	return true
}

// settleSettings is called by Close (chain held, closed set): the values still
// waiting go into the closing write, which is one write either way; the cap is
// respected.
func (p *tlLCD) settleSettings() {
	if p.rate.timer != nil {
		p.rate.timer.Stop()
		p.rate.timer = nil
	}
	if !p.rate.capped && p.rate.count < tlDailyMax {
		if p.rate.pendB != nil {
			p.brightness = *p.rate.pendB
		}
		if p.rate.pendR != nil {
			p.rotation = *p.rate.pendR
		}
	}
	p.rate.pendB, p.rate.pendR = nil, nil
}
