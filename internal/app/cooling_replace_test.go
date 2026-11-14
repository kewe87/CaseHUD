package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// countingReadings counts how often the cooling looked at the readings.
type countingReadings struct {
	coolingReadings
	reads atomic.Int32
}

func (c *countingReadings) All() []metrics.Reading {
	c.reads.Add(1)
	return c.coolingReadings.All()
}

func TestANewLoopWaitsForAnOldOneThatIsStillInACall(t *testing.T) {
	p := &countingReadings{coolingReadings: coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}}}}
	e := coolingEngine(t, coolingExample, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.ctx = ctx
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	old := make(chan struct{})
	if err := e.launchCoolingAfter(cfg, 0, old, nil, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // several ticks if it did not wait
	if n := p.reads.Load(); n != 0 {
		t.Fatalf("looked at the readings %d times while the old loop was still running", n)
	}
	close(old)
	deadline := time.Now().Add(5 * time.Second)
	for p.reads.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("never started after the old loop ended")
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.coolingSnapshot().cancel()
	<-e.coolingSnapshot().done
}

func TestANewLoopThatIsEndedWhileWaitingEndsAtOnce(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}}}
	e := coolingEngine(t, coolingExample, p)
	e.ctx = context.Background()
	cfg, _ := e.Store.loadCoolingConfig()
	if err := e.launchCoolingAfter(cfg, 0, make(chan struct{}), nil, nil); err != nil {
		t.Fatal(err)
	}
	c := e.coolingSnapshot()
	c.cancel()
	select {
	case <-c.done:
	case <-time.After(3 * time.Second):
		t.Fatal("did not end")
	}
}
