package panel

import (
	"log"
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

// tlRate is the state of the limiter of one fan (guarded by tlChain).
type tlRate struct {
	last     time.Time // of the last write made for a change
	pendB    *int      // coalesced values waiting for the interval
	pendR    *int
	timer    *time.Timer
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
		if !p.rate.capKnown {
			p.rate.capKnown = true
			log.Printf("TL LCD %s: %d brightness/rotation writes today: no more are made until CaseHUD restarts (they wear the display's flash)", p.ID(), tlDailyMax)
		}
		return false
	}
	p.rate.count++
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
