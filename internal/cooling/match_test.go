package cooling

import (
	"context"
	"errors"
	"testing"
	"time"
)

// matchPump is a pump whose speed follows the duty of its own channel. board is
// the duty the board has; held is the duty a hold sets, 0 when none is.
type matchPump struct {
	now       time.Duration
	board     int
	held      int
	perDuty   int // rpm per point of duty on this pump
	follows   bool
	inverse   bool
	takes     time.Duration
	changedAt time.Duration
	from      int
	writes    []int
	noRPM     bool
	noise     func(now time.Duration) int
	stopBelow int // under this duty the pump stands
}

func (p *matchPump) duty() int {
	if p.held > 0 {
		return p.held
	}
	return p.board
}

func (p *matchPump) target() int {
	if p.stopBelow > 0 && p.duty() < p.stopBelow {
		return 0
	}
	if !p.follows {
		return 2000
	}
	if p.inverse {
		return 6000 - p.duty()*p.perDuty
	}
	return p.duty() * p.perDuty
}

func (p *matchPump) rpmNow() int {
	to := p.target()
	var rpm int
	if p.now-p.changedAt >= p.takes {
		rpm = to
	} else {
		share := float64(p.now-p.changedAt) / float64(p.takes)
		rpm = p.from + int(float64(to-p.from)*share)
	}
	if p.noise != nil {
		rpm += p.noise(p.now)
	}
	return rpm / 60 * 60
}

func (p *matchPump) change(f func()) {
	p.from = p.rpmNow()
	p.changedAt = p.now
	f()
}

func (p *matchPump) TakeControl(string) error { return nil }
func (p *matchPump) SetDuty(_ string, d int) error {
	p.change(func() { p.held = d })
	p.writes = append(p.writes, d)
	return nil
}
func (p *matchPump) Speeds() (map[string]int, error) {
	if p.noRPM {
		return map[string]int{}, nil
	}
	return map[string]int{"p": p.rpmNow()}, nil
}
func (p *matchPump) wait(ctx context.Context, d time.Duration) error {
	p.now += d
	return ctx.Err()
}
func (p *matchPump) release() error {
	p.change(func() { p.held = 0 })
	return nil
}

func (p *matchPump) match(floor int, known bool) Match {
	return Match{Device: p, ID: "p", Floor: floor, Wait: p.wait, Release: p.release,
		Current: func() (int, bool) { return p.board, known }}
}

func newMatchPump(board int) *matchPump {
	return &matchPump{board: board, perDuty: 30, follows: true, takes: 10 * time.Second, from: board * 30}
}

func lowest(v []int) int {
	m := 1000
	for _, x := range v {
		m = min(m, x)
	}
	return m
}

func TestMatchProvesAPumpThatFollows(t *testing.T) {
	p := newMatchPump(50)
	res, err := p.match(40, true).Run(context.Background())
	if err != nil || res.Verdict != MatchProven {
		t.Fatalf("%+v %v", res, err)
	}
	if len(p.writes) != 2 || p.writes[0] != 70 || p.writes[1] != 50 {
		t.Fatalf("writes %v", p.writes)
	}
}

func TestMatchProvesASluggishPump(t *testing.T) {
	p := newMatchPump(50)
	p.takes = 60 * time.Second
	res, err := p.match(40, true).Run(context.Background())
	if err != nil || res.Verdict != MatchProven {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestMatchGoesStraightToFullFromAHighDuty(t *testing.T) {
	p := newMatchPump(85)
	res, _ := p.match(40, true).Run(context.Background())
	if res.Verdict != MatchProven || p.writes[0] != 100 || p.writes[1] != 85 {
		t.Fatalf("%+v %v", res, p.writes)
	}
}

func TestMatchNeverWritesBelowTheBoardOrTheFloor(t *testing.T) {
	for _, board := range []int{0, 10, 30, 45, 79, 80, 90} {
		for _, known := range []bool{true, false} {
			p := newMatchPump(board)
			Match{Device: p, ID: "p", Floor: 50, Wait: p.wait, Release: p.release,
				Current: func() (int, bool) { return board, known }}.Run(context.Background())
			for _, w := range p.writes {
				if w < max(board, 50) || w < 30 {
					t.Errorf("board %d known %v: wrote %d (%v)", board, known, w, p.writes)
				}
			}
		}
	}
}

func TestMatchFailsWhenTheSpeedStaysPut(t *testing.T) {
	p := newMatchPump(50)
	p.follows = false
	res, err := p.match(40, true).Run(context.Background())
	if err != nil || res.Verdict != MatchFailed {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestMatchFailsWhenTheSpeedGoesTheOtherWay(t *testing.T) {
	p := newMatchPump(50)
	p.inverse = true
	res, _ := p.match(40, true).Run(context.Background())
	if res.Verdict != MatchFailed {
		t.Fatalf("%+v", res)
	}
}

func TestMatchWithoutASpeedProvesNothing(t *testing.T) {
	p := newMatchPump(50)
	p.noRPM = true
	res, err := p.match(40, true).Run(context.Background())
	if err != nil || res.Verdict != MatchUnclear || len(p.writes) != 0 {
		t.Fatalf("%+v %v %v", res, err, p.writes)
	}
}

func TestMatchWithAChangingLoadIsUnclear(t *testing.T) {
	p := newMatchPump(50)
	p.noise = func(now time.Duration) int { return int(now/(3*time.Second)%2) * 400 }
	res, err := p.match(40, true).Run(context.Background())
	if err != nil || res.Verdict != MatchUnclear {
		t.Fatalf("%+v %v", res, err)
	}
	if len(p.writes) != 0 {
		t.Fatalf("wrote on an unsettled speed: %v", p.writes)
	}
}

// A speed that rises by itself, as a board does under load, must not pass if
// it does not come down when the duty does.
func TestMatchNeedsTheSpeedToComeBack(t *testing.T) {
	p := newMatchPump(50)
	p.follows = false
	p.noise = func(now time.Duration) int {
		if now > 30*time.Second {
			return 600
		}
		return 0
	}
	res, _ := p.match(40, true).Run(context.Background())
	if res.Verdict == MatchProven {
		t.Fatalf("%+v", res)
	}
}

func TestMatchWithoutRoomIsUnclear(t *testing.T) {
	p := newMatchPump(95)
	res, _ := p.match(40, true).Run(context.Background())
	if res.Verdict != MatchUnclear || len(p.writes) != 0 {
		t.Fatalf("%+v %v", res, p.writes)
	}
}

func TestMatchWithAnUnknownDutyGoesToFullAndLetsTheBoardTakeOver(t *testing.T) {
	p := newMatchPump(50)
	res, _ := p.match(40, false).Run(context.Background())
	if res.Verdict != MatchProven || len(p.writes) != 1 || p.writes[0] != 100 || p.held != 0 {
		t.Fatalf("%+v %v held %d", res, p.writes, p.held)
	}
}

func TestMatchEndsWhenChecked(t *testing.T) {
	p := newMatchPump(50)
	m := p.match(40, true)
	calls := 0
	m.Check = func() error {
		if calls++; calls > 8 {
			return errors.New("another program")
		}
		return nil
	}
	res, err := m.Run(context.Background())
	if err == nil || res.Verdict != MatchUnclear {
		t.Fatalf("%+v %v", res, err)
	}
	if lowest(p.writes) < 50 {
		t.Fatalf("%v", p.writes)
	}
}

func TestMatchEndsWithItsContext(t *testing.T) {
	p := newMatchPump(50)
	ctx, cancel := context.WithCancel(context.Background())
	m := p.match(40, true)
	n := 0
	m.Wait = func(c context.Context, d time.Duration) error {
		if n++; n == 8 {
			cancel()
		}
		return p.wait(c, d)
	}
	res, err := m.Run(ctx)
	if err == nil || res.Verdict != MatchUnclear {
		t.Fatalf("%+v %v", res, err)
	}
}

func cool(t float64, ok bool) func() (float64, bool) {
	return func() (float64, bool) { return t, ok }
}

// atFull is a pump the board runs at full speed: the check goes down and back.
func (p *matchPump) atFull(floor int) Match {
	m := p.match(floor, true)
	m.Hottest = cool(30, true)
	return m
}

func TestMatchProvesAPumpAtFullSpeedByLoweringIt(t *testing.T) {
	p := newMatchPump(100)
	res, err := p.atFull(50).Run(context.Background())
	if err != nil || res.Verdict != MatchProven {
		t.Fatalf("%+v %v", res, err)
	}
	if len(p.writes) != 2 || p.writes[0] != 80 || p.writes[1] != 100 {
		t.Fatalf("writes %v", p.writes)
	}
}

func TestMatchLowersASluggishPumpAtFullSpeed(t *testing.T) {
	p := newMatchPump(100)
	p.takes = 60 * time.Second
	res, err := p.atFull(50).Run(context.Background())
	if err != nil || res.Verdict != MatchProven || p.writes[len(p.writes)-1] != 100 {
		t.Fatalf("%+v %v %v", res, err, p.writes)
	}
}

func TestMatchAtFullSpeedFailsForAWrongChannel(t *testing.T) {
	for name, set := range map[string]func(*matchPump){
		"no reaction": func(p *matchPump) { p.follows = false },
		"wrong way":   func(p *matchPump) { p.inverse = true },
	} {
		p := newMatchPump(100)
		set(p)
		res, err := p.atFull(50).Run(context.Background())
		if err != nil || res.Verdict != MatchFailed {
			t.Fatalf("%s: %+v %v", name, res, err)
		}
		if p.writes[len(p.writes)-1] != 100 {
			t.Fatalf("%s: not restored: %v", name, p.writes)
		}
	}
}

func TestMatchLoweringNeverGoesBelowTheFloorOrThirty(t *testing.T) {
	for _, start := range []int{91, 95, 100} {
		for _, floor := range []int{0, 30, 50, 85, 90, 95} {
			p := newMatchPump(start)
			p.atFull(floor).Run(context.Background())
			for _, w := range p.writes {
				if w < max(floor, 30) || w > 100 {
					t.Errorf("start %d floor %d: wrote %d (%v)", start, floor, w, p.writes)
				}
			}
		}
	}
}

func TestMatchWithoutRoomToLowerIsUnclear(t *testing.T) {
	p := newMatchPump(100)
	res, err := p.atFull(95).Run(context.Background())
	if err != nil || res.Verdict != MatchUnclear || len(p.writes) != 0 {
		t.Fatalf("%+v %v %v", res, err, p.writes)
	}
	if _, ok := MatchLowTarget(100, 95); ok {
		t.Fatal("target without room")
	}
	if low, ok := MatchLowTarget(100, 50); !ok || low != 80 {
		t.Fatalf("%d %v", low, ok)
	}
}

func TestMatchDoesNotLowerWhenHotOrBlind(t *testing.T) {
	for name, h := range map[string]func() (float64, bool){
		"hot":     cool(PumpMeasureMaxTemp, true),
		"blind":   cool(0, false),
		"missing": nil,
	} {
		p := newMatchPump(100)
		m := p.atFull(50)
		m.Hottest = h
		res, err := m.Run(context.Background())
		if err != nil || res.Verdict != MatchUnclear || len(p.writes) != 0 {
			t.Fatalf("%s: %+v %v %v", name, res, err, p.writes)
		}
	}
}

func TestMatchRestoresWhenItGetsHotWhileLowered(t *testing.T) {
	p := newMatchPump(100)
	m := p.atFull(50)
	m.Hottest = func() (float64, bool) {
		if len(p.writes) > 0 && p.now > 40*time.Second {
			return PumpMeasureMaxTemp, true
		}
		return 30, true
	}
	p.follows = false // nothing proven before the heat arrives
	res, err := m.Run(context.Background())
	if err != nil || res.Verdict != MatchUnclear || p.writes[len(p.writes)-1] != 100 {
		t.Fatalf("%+v %v %v", res, err, p.writes)
	}
}

func TestMatchRestoresTheStartDutyWhenCutShort(t *testing.T) {
	p := newMatchPump(100)
	m := p.atFull(50)
	n := 0
	stop := errors.New("someone else takes over")
	m.Check = func() error {
		if len(p.writes) > 0 {
			if n++; n > 4 {
				return stop
			}
		}
		return nil
	}
	_, err := m.Run(context.Background())
	if !errors.Is(err, stop) {
		t.Fatalf("%v", err)
	}
	if len(p.writes) < 2 || p.writes[0] != 80 || p.writes[len(p.writes)-1] != 100 || p.held != 100 {
		t.Fatalf("writes %v held %d", p.writes, p.held)
	}
}

func TestMatchRestoresAtOnceWhenTheLoweredPumpStops(t *testing.T) {
	p := newMatchPump(100)
	p.stopBelow = 90
	p.takes = 3 * time.Second
	m := p.atFull(50)
	res, err := m.Run(context.Background())
	if err != nil || res.Verdict != MatchUnclear || p.writes[len(p.writes)-1] != 100 || p.held != 100 {
		t.Fatalf("%+v %v %v", res, err, p.writes)
	}
	// With a stop limit above the lowered speed it is the same.
	p = newMatchPump(100)
	p.takes = 3 * time.Second
	m = p.atFull(50)
	m.StoppedBelow = 3000
	res, _ = m.Run(context.Background())
	if res.Verdict != MatchUnclear || p.writes[len(p.writes)-1] != 100 {
		t.Fatalf("%+v %v", res, p.writes)
	}
}
