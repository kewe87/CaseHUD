package cooling

import (
	"context"
	"errors"
	"testing"
	"time"
)

// inertFan approaches its target speed slowly, like a heavy fan. The clock
// advances only through Wait.
type inertFan struct {
	now     time.Duration
	setAt   time.Duration
	from    int
	to      int
	takes   time.Duration
	perDuty int
	readErr error
	duties  []int
	taken   bool
}

func (f *inertFan) rpmNow() int {
	if f.now-f.setAt >= f.takes {
		return f.to
	}
	share := float64(f.now-f.setAt) / float64(f.takes)
	return (f.from + int(float64(f.to-f.from)*share)) / 60 * 60 // 60 rpm raster
}

func (f *inertFan) TakeControl(string) error {
	f.taken = true
	return nil
}

func (f *inertFan) SetDuty(_ string, d int) error {
	if !f.taken {
		return errors.New("not in control")
	}
	f.from, f.to, f.setAt = f.rpmNow(), d*f.perDuty, f.now
	f.duties = append(f.duties, d)
	return nil
}

func (f *inertFan) Speeds() (map[string]int, error) {
	return map[string]int{"f": f.rpmNow()}, f.readErr
}

func (f *inertFan) wait(ctx context.Context, d time.Duration) error {
	f.now += d
	return ctx.Err()
}

func TestMeasureWaitsForASlowFan(t *testing.T) {
	// Far slower than any fixed wait of a few seconds would notice.
	f := &inertFan{takes: 50 * time.Second, perDuty: 20}
	var at []time.Duration
	m := Measurement{Device: f, ID: "f", Steps: []int{100, 50}, Wait: f.wait,
		Progress: func(int, int, int) { at = append(at, f.now) }}
	steps, err := m.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || !steps[0].Settled || !steps[1].Settled {
		t.Fatalf("%+v", steps)
	}
	if steps[0].RPM < 1940 || steps[1].RPM > 1060 {
		t.Fatalf("judged too early: %+v", steps)
	}
	if f.now < 100*time.Second {
		t.Fatalf("took only %v for two 50 s changes", f.now)
	}
}

func TestMeasureTakesAQuickFanQuickly(t *testing.T) {
	f := &inertFan{takes: time.Second, perDuty: 20}
	m := Measurement{Device: f, ID: "f", Steps: []int{100}, Wait: f.wait}
	if _, err := m.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.now > 2*measureMinWait {
		t.Fatalf("took %v", f.now)
	}
}

func TestMeasureMarksAStepThatNeverSettles(t *testing.T) {
	f := &inertFan{takes: 10 * time.Minute, perDuty: 100}
	m := Measurement{Device: f, ID: "f", Steps: []int{100}, Wait: f.wait}
	steps, err := m.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Settled {
		t.Fatalf("%+v", steps)
	}
	if f.now > measureMaxWait+measurePoll {
		t.Fatalf("ran past its limit: %v", f.now)
	}
	if _, err := SpeedMapFrom(steps); err == nil {
		t.Fatal("an unsettled step entered the map")
	}
}

func TestMeasureReportsAFanWithoutSpeed(t *testing.T) {
	f := &inertFan{perDuty: 0}
	m := Measurement{Device: f, ID: "f", Steps: []int{100, 50}, Wait: f.wait}
	if _, err := m.Run(context.Background()); !errors.Is(err, ErrNoSpeed) {
		t.Fatalf("%v", err)
	}
}

func TestMeasureStopsOnCheckAndContext(t *testing.T) {
	f := &inertFan{perDuty: 20}
	boom := errors.New("too hot")
	m := Measurement{Device: f, ID: "f", Steps: []int{100}, Wait: f.wait, Check: func() error { return boom }}
	if _, err := m.Run(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m = Measurement{Device: f, ID: "f", Steps: []int{100}, Wait: f.wait}
	if _, err := m.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	f.readErr = errors.New("gone")
	m = Measurement{Device: f, ID: "f", Steps: []int{100}, Wait: f.wait}
	if _, err := m.Run(context.Background()); !errors.Is(err, f.readErr) {
		t.Fatalf("%v", err)
	}
}

func TestSpeedMapFrom(t *testing.T) {
	m, err := SpeedMapFrom([]MeasuredStep{
		{100, 2040, true}, {90, 2040, true}, {50, 1020, true}, {40, 1080, true},
		{30, 660, false}, {15, 0, true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := SpeedMap{{40, 1080}, {50, 1080}, {90, 2040}, {100, 2040}}
	if len(m) != len(want) {
		t.Fatalf("%v", m)
	}
	for i := range m {
		if m[i] != want[i] {
			t.Fatalf("%v", m)
		}
	}
}

func TestRiseGuard(t *testing.T) {
	temp, ok := 40.0, true
	g := RiseGuard(func() (float64, bool) { return temp, ok })
	if err := g(); err != nil {
		t.Fatal(err)
	}
	temp = 49
	if err := g(); err != nil {
		t.Fatal(err)
	}
	temp = 50
	if err := g(); err == nil {
		t.Fatal("no stop at the limit")
	}
	temp, ok = 40, false
	if err := g(); !errors.Is(err, ErrSensorLost) {
		t.Fatalf("%v", err)
	}
}

func TestHoldKeepsTheControllerOffAnOutput(t *testing.T) {
	b := newBench()
	b.rpm["f"] = 900
	c, err := New([]Fan{joinFan("f")}, b, func() map[string]float64 { return map[string]float64{"t": 40} })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	before := len(b.writes["f"])
	c.Hold("f")
	c.Tick()
	c.Tick()
	if len(b.writes["f"]) != before {
		t.Fatalf("wrote to a held output: %v", b.writes["f"])
	}
	if st := c.Status(); !st[0].Held {
		t.Fatalf("%+v", st[0])
	}
	c.Release("f", 50)
	c.Tick()
	if len(b.writes["f"]) != before+1 {
		t.Fatalf("duty not sent again: %v", b.writes["f"])
	}
}

func TestDetachLeavesTheDuties(t *testing.T) {
	b := newBench()
	b.rpm["f"] = 900
	c, err := New([]Fan{joinFan("f")}, b, func() map[string]float64 { return map[string]float64{"t": 40} })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	writes := len(b.writes["f"])
	c.Detach()
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(b.writes["f"]) != writes {
		t.Fatalf("%v", b.writes["f"])
	}
	if err := c.Tick(); !errors.Is(err, ErrClosed) {
		t.Fatalf("%v", err)
	}
}
