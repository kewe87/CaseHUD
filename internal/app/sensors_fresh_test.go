package app

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// clock is a time the test moves by hand.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClock() *clock { return &clock{t: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)} }

// snapshot is a machine's readings: a steady coolant temperature next to n-1 values
// that move with the tick number, as loads and clocks do.
func snapshot(n, tick int) []metrics.Reading {
	rs := []metrics.Reading{{Key: "/water", Value: 28.4, Unit: "°C"}}
	for i := 1; i < n; i++ {
		rs = append(rs, metrics.Reading{Key: fmt.Sprintf("/load/%d", i), Value: float64((tick*7 + i*13) % 100), Unit: "%"})
	}
	return rs
}

func freshKeys(rs []metrics.Reading) map[string]bool {
	out := map[string]bool{}
	for _, r := range rs {
		out[r.Key] = true
	}
	return out
}

// A coolant temperature that does not change for hours is normal and must never be
// taken for a frozen one while the machine around it is alive.
func TestASteadyWaterTemperatureIsNeverFrozen(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	for tick := range 3 * 3600 {
		rs := snapshot(40, tick)
		w.observe(rs, nil)
		c.advance(time.Second)
		if got := w.filter(rs); len(got) != len(rs) {
			t.Fatalf("after %d s: %d of %d readings", tick, len(got), len(rs))
		}
	}
	if w.degraded {
		t.Fatal("marked as degraded")
	}
}

func TestAMachineWhoseEveryValueStandsStillIsFrozenAfterAMinute(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	for tick := range 20 { // alive first
		w.observe(snapshot(40, tick), nil)
		c.advance(time.Second)
	}
	frozen := snapshot(40, 20)
	var lastOK, firstBad time.Duration
	for s := time.Duration(0); s <= 2*time.Minute; s += time.Second {
		w.observe(frozen, nil) // the same numbers again and again
		c.advance(time.Second)
		if len(w.filter(frozen)) == 0 {
			firstBad = s
			break
		}
		lastOK = s
	}
	if firstBad < 55*time.Second || firstBad > 65*time.Second {
		t.Fatalf("last good at %v, first frozen at %v", lastOK, firstBad)
	}
	if !w.degraded {
		t.Fatal("not marked as degraded")
	}
	// One number moves: everything is believed again.
	w.observe(snapshot(40, 21), nil)
	if got := w.filter(snapshot(40, 21)); len(got) != 40 || w.degraded {
		t.Fatalf("%d readings, degraded %v", len(got), w.degraded)
	}
}

func TestAFewReadingsNeedLongerToCountAsFrozen(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	few := snapshot(sensorRichCount-1, 0)
	w.observe(few, nil)
	c.advance(9 * time.Minute)
	w.observe(few, nil)
	if got := w.filter(few); len(got) != len(few) {
		t.Fatal("frozen after nine minutes with this few readings")
	}
	c.advance(2 * time.Minute)
	w.observe(few, nil)
	if got := w.filter(few); len(got) != 0 {
		t.Fatal("not frozen after eleven")
	}
}

// With a real age from the source, equal numbers prove nothing.
func TestNumbersThatStandStillAreBelievedWhileTheSourceSaysTheyAreCurrent(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	rs := snapshot(40, 0)
	for i := range rs {
		rs[i].Age, rs[i].AgeKnown = 400*time.Millisecond, true
	}
	for range 4 * 3600 {
		w.observe(rs, nil)
		c.advance(time.Second)
		if len(w.filter(rs)) != len(rs) {
			t.Fatal("dropped readings the service says are current")
		}
	}
}

func TestOldReadingsAreLeftOutAndOthersStay(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	rs := []metrics.Reading{
		{Key: "/cpu", Value: 50, Unit: "°C", Age: time.Second, AgeKnown: true},
		{Key: "/gpu", Value: 60, Unit: "°C", Age: 45 * time.Second, AgeKnown: true},
		{Key: "/other", Value: 1, Unit: "%"}, // no age known
	}
	w.observe(rs, nil)
	got := freshKeys(w.filter(rs))
	if !got["/cpu"] || got["/gpu"] || !got["/other"] {
		t.Fatalf("%v", got)
	}
	if !w.degraded {
		t.Fatal("not degraded")
	}
}

// The age is that at the last answer; the time since adds to it.
func TestTheTimeSinceTheAnswerCountsToo(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	rs := []metrics.Reading{{Key: "/cpu", Value: 50, Unit: "°C", Age: 5 * time.Second, AgeKnown: true}}
	w.observe(rs, nil)
	c.advance(10 * time.Second)
	if len(w.filter(rs)) != 1 {
		t.Fatal("dropped at 15 s")
	}
	c.advance(10 * time.Second)
	if len(w.filter(rs)) != 0 {
		t.Fatal("kept at 25 s: the provider has not been refreshed")
	}
	w.observe(rs, nil) // a new answer with an age again 5 s
	if len(w.filter(rs)) != 1 {
		t.Fatal("not current after a new answer")
	}
}

func TestAFailedRefreshStartsOver(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	rs := snapshot(40, 0)
	w.observe(rs, nil)
	c.advance(50 * time.Second)
	w.observe(nil, fmt.Errorf("no answer")) // the provider drops its readings
	c.advance(50 * time.Second)
	w.observe(rs, nil) // back, and by chance the same numbers
	for range 5 {      // refreshed every 10 s with the same numbers since
		c.advance(10 * time.Second)
		w.observe(rs, nil)
	}
	if len(w.filter(rs)) != len(rs) {
		t.Fatal("the time before the outage counted")
	}
}

func TestWithoutReadingsThereIsNothingToJudge(t *testing.T) {
	w := &freshWatch{}
	if got := w.filter(nil); len(got) != 0 || w.degraded {
		t.Fatalf("%v %v", got, w.degraded)
	}
	w.observe(nil, nil)
	if w.frozen(time.Now().Add(100 * time.Hour)) {
		t.Fatal("frozen without readings")
	}
}

func TestTheProviderOnlyHandsTheCoolingWhatIsFresh(t *testing.T) {
	c := newClock()
	pump := metrics.Reading{Key: "/pump", Value: 2000, Unit: "RPM"}
	p := &coolingReadings{readings: append(snapshot(40, 0), pump)}
	prov := NewSafeProvider(p)
	prov.fresh.now = c.now
	prov.Refresh()
	if got := coolingTemps(prov); got["/water"] != 28.4 {
		t.Fatalf("%v", got)
	}
	if got := coolingSpeeds(prov, map[string]string{"pump": "/pump"}); got["pump"] != 2000 {
		t.Fatalf("%v", got)
	}
	// Nothing moves for two minutes.
	for range 120 {
		c.advance(time.Second)
		prov.Refresh()
	}
	if got := coolingTemps(prov); len(got) != 0 {
		t.Fatalf("a frozen snapshot reached the cooling: %v", got)
	}
	if got := coolingSpeeds(prov, map[string]string{"pump": "/pump"}); len(got) != 0 {
		t.Fatalf("frozen speeds reached the cooling: %v", got)
	}
	if !prov.Stale() {
		t.Fatal("not stale")
	}
	// The display still shows what there is.
	if len(prov.All()) != 41 {
		t.Fatal("the readings are gone for everybody")
	}
	p.readings = append(snapshot(40, 5), pump)
	prov.Refresh()
	if got := coolingTemps(prov); got["/water"] != 28.4 || prov.Stale() {
		t.Fatalf("%v", got)
	}
}

func TestAnOverdueRefreshMakesEveryReadingOld(t *testing.T) {
	c := newClock()
	w := &freshWatch{now: c.now}
	rs := snapshot(40, 0)
	w.observe(rs, nil)
	c.advance(sensorMaxAge / 2)
	if len(w.filter(rs)) != len(rs) {
		t.Fatal("readings dropped too early")
	}
	c.advance(sensorMaxAge)
	if got := w.filter(rs); len(got) != 0 || !w.degraded {
		t.Fatalf("%d readings kept after a refresh that never came", len(got))
	}
}

// slowProvider blocks in Refresh until released.
type slowProvider struct {
	coolingReadings
	entered, release chan struct{}
}

func (s *slowProvider) Refresh() error {
	close(s.entered)
	<-s.release
	return nil
}

func TestTheCoolingDoesNotWaitForASlowRefresh(t *testing.T) {
	p := &slowProvider{
		coolingReadings: coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 31, Unit: "°C"}}},
		entered:         make(chan struct{}), release: make(chan struct{}),
	}
	prov := NewSafeProvider(p)
	prov.smu.Lock()
	prov.snap = []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}}
	prov.smu.Unlock()
	done := make(chan struct{})
	go func() { prov.Refresh(); close(done) }()
	<-p.entered
	got := make(chan []metrics.Reading, 1)
	go func() { prov.Err(); got <- prov.Fresh() }()
	select {
	case rs := <-got:
		if len(rs) != 1 || rs[0].Value != 30 {
			t.Fatalf("%v", rs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Fresh waited for the refresh")
	}
	close(p.release)
	<-done
}

func TestReadingsOfAnUnverifiedSourceAreForTheDisplayOnly(t *testing.T) {
	pump := metrics.Reading{Key: "/pump", Value: 2000, Unit: "RPM", DisplayOnly: true}
	water := metrics.Reading{Key: "/water", Value: 28.4, Unit: "°C", DisplayOnly: true}
	p := &coolingReadings{readings: []metrics.Reading{water, pump}}
	prov := NewSafeProvider(p)
	prov.Refresh()
	if got := coolingTemps(prov); len(got) != 0 {
		t.Fatalf("an unverified temperature reached the cooling: %v", got)
	}
	if got := coolingSpeeds(prov, map[string]string{"pump": "/pump"}); len(got) != 0 {
		t.Fatalf("an unverified speed reached the cooling: %v", got)
	}
	if len(prov.All()) != 2 {
		t.Fatal("the display lost the readings")
	}
	// Once the service answers, the same readings count.
	p.readings = []metrics.Reading{{Key: "/water", Value: 28.4, Unit: "°C"}, {Key: "/pump", Value: 2000, Unit: "RPM"}}
	prov.Refresh()
	if got := coolingTemps(prov); got["/water"] != 28.4 {
		t.Fatalf("%v", got)
	}
}

func TestTheViewSaysWhenOnlyADisplaySourceDelivers(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/water", Value: 28.4, Unit: "°C", DisplayOnly: true}}}
	e := &Engine{prov: NewSafeProvider(p)}
	e.prov.Refresh()
	var v CoolingView
	e.addCrashInfo(&v)
	if !v.SensorsDisplayOnly || v.SensorSource != "custom" {
		t.Fatalf("%+v", v)
	}
	p.readings = []metrics.Reading{{Key: "/water", Value: 28.4, Unit: "°C"}}
	e.prov.Refresh()
	v = CoolingView{}
	e.addCrashInfo(&v)
	if v.SensorsDisplayOnly || v.SensorSource != "" {
		t.Fatalf("%+v", v)
	}
}
