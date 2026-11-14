package app

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// lockedReadings is a provider the test can change while the loop reads it.
type lockedReadings struct {
	mu sync.Mutex
	rs []metrics.Reading
}

func (l *lockedReadings) set(rs []metrics.Reading) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rs = rs
}

func (l *lockedReadings) Refresh() error { return nil }
func (l *lockedReadings) Get(string) (metrics.Reading, bool) {
	return metrics.Reading{}, false
}
func (l *lockedReadings) All() []metrics.Reading {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]metrics.Reading(nil), l.rs...)
}

// frozenRig is a live loop on fakes whose sensor source can freeze.
type frozenRig struct {
	*liveRig
	p     *lockedReadings
	clock *clock
	tick  int
}

func newFrozenRig(t *testing.T) *frozenRig {
	r := newLiveRig(t, false)
	f := &frozenRig{liveRig: r, clock: newClock(), p: &lockedReadings{}}
	f.p.set(f.readings())
	r.e.prov = NewSafeProvider(f.p)
	r.e.prov.fresh.now = f.clock.now
	r.run(t)
	return f
}

// readings is the machine: a steady coolant temperature, a pump, and values that
// move with every tick.
func (f *frozenRig) readings() []metrics.Reading {
	rs := []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: "/lpc/chip/0/fan/6", Value: 3000, Unit: "RPM"},
	}
	for i := range 12 {
		rs = append(rs, metrics.Reading{Key: fmt.Sprintf("/load/%d", i), Value: float64((f.tick*7 + i*13) % 100), Unit: "%"})
	}
	return rs
}

// step lets one second pass. A living machine changes its numbers.
func (f *frozenRig) step(alive bool) {
	if alive {
		f.tick++
		f.p.set(f.readings())
	}
	f.e.prov.Refresh()
	f.clock.advance(time.Second)
	f.e.coolingSnapshot().ctrl.Tick()
}

func outputByID(v CoolingView, id string) CoolingOutputView {
	for _, o := range v.Outputs {
		if o.ID == id {
			return o
		}
	}
	return CoolingOutputView{}
}

func TestStableTemperaturesNeverRaiseAnAlarm(t *testing.T) {
	f := newFrozenRig(t)
	for range 2 * 3600 { // two hours with a coolant that does not move at all
		f.step(true)
	}
	v := f.e.CoolingView()
	if o := outputByID(v, "port1-fan1"); o.SensorLost || o.Temp == nil || *o.Temp != 30 {
		t.Fatalf("%+v", o)
	}
	if v.Alarm != nil || v.SensorsStale {
		t.Fatalf("alarm %+v, stale %v", v.Alarm, v.SensorsStale)
	}
	r := outputByID(v, "pump")
	if r.SensorLost || r.RPM == nil {
		t.Fatalf("%+v", r)
	}
	f.e.stopCoolingModule()
}

func TestAFrozenSourceSendsTheFansToTheirSafeDutyAndRaisesTheAlarm(t *testing.T) {
	f := newFrozenRig(t)
	for range 600 {
		f.step(true)
	}
	if v := f.e.CoolingView(); v.Alarm != nil {
		t.Fatalf("alarm while alive: %+v", v.Alarm)
	}
	// The service hangs: its answers go on, always the same numbers.
	for range 90 {
		f.step(false)
	}
	v := f.e.CoolingView()
	fan := outputByID(v, "port1-fan1")
	if !fan.SensorLost || fan.Duty < 70 {
		t.Fatalf("fan: %+v", fan)
	}
	if got := f.tl.duty["port1-fan1"]; got[len(got)-1] < 70 {
		t.Fatalf("the fan was left at %d %%", got[len(got)-1])
	}
	pump := outputByID(v, "pump")
	if !pump.SensorLost || pump.Duty < 100 {
		t.Fatalf("pump: %+v", pump)
	}
	if v.Alarm == nil || v.Alarm.Kind != coolingAlarmSensor || v.Alarm.Name != "Top" || !v.SensorsStale {
		t.Fatalf("alarm %+v, stale %v", v.Alarm, v.SensorsStale)
	}

	// It runs again.
	for range 5 {
		f.step(true)
	}
	v = f.e.CoolingView()
	if outputByID(v, "port1-fan1").SensorLost || v.Alarm != nil {
		t.Fatalf("not recovered: %+v", v.Alarm)
	}
	f.e.stopCoolingModule()
}

// An answer that carries its own age is judged by it, however the numbers move.
func TestAnAnswerThatSaysItIsOldIsNotBelieved(t *testing.T) {
	f := newFrozenRig(t)
	age := func(a time.Duration) {
		rs := f.readings()
		for i := range rs {
			rs[i].Age, rs[i].AgeKnown = a, true
		}
		f.p.set(rs)
	}
	for range 10 {
		age(300 * time.Millisecond)
		f.step(false)
	}
	if v := f.e.CoolingView(); outputByID(v, "port1-fan1").SensorLost {
		t.Fatalf("%+v", v.Outputs)
	}
	// The values still move, but the service says it read them half a minute ago.
	for range 10 {
		f.tick++
		age(30 * time.Second)
		f.step(false)
	}
	v := f.e.CoolingView()
	if !outputByID(v, "port1-fan1").SensorLost || v.Alarm == nil || v.Alarm.Kind != coolingAlarmSensor {
		t.Fatalf("alarm %+v, outputs %+v", v.Alarm, v.Outputs)
	}
	f.e.stopCoolingModule()
}
