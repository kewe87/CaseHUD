package cooling

import (
	"errors"
	"testing"
	"time"
)

func joinFan(id string) Fan {
	f := Fan{
		ID: id, Sources: []string{"t"},
		Curve:     Curve{{20, 30}, {60, 80}},
		RiseAfter: 1, FallAfter: 1, StepUp: 100, StepDown: 100, SafeDuty: 70,
	}
	return f
}

func TestJoinKeepsTheOtherDeviceWorking(t *testing.T) {
	fans, pump := newBench(), newBench()
	fans.rpm["f"], pump.rpm["p"] = 900, 3000
	c, err := New([]Fan{joinFan("f"), joinFan("p")},
		Join(Route{fans, []string{"f"}}, Route{pump, []string{"p"}}),
		func() map[string]float64 { return map[string]float64{"t": 40} })
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	fansWritten := len(fans.writes["f"])

	fans.readErr = errors.New("gone")
	err = c.Tick()
	var part *PartlyLost
	if !errors.As(err, &part) || len(part.IDs) != 1 || part.IDs[0] != "f" {
		t.Fatalf("error: %v", err)
	}
	st := c.Status()
	if !st[0].DeviceLost || st[1].DeviceLost {
		t.Fatalf("status: %+v", st)
	}
	if !st[1].HasRPM || st[1].RPM != 3000 {
		t.Fatalf("the working device lost its speed: %+v", st[1])
	}
	if len(fans.writes["f"]) != fansWritten {
		t.Fatal("wrote to a device that cannot be read")
	}

	// Back again: control is taken and the duty sent anew, only there.
	fans.readErr = nil
	takenFans, takenPump := fans.taken["f"], pump.taken["p"]
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	if fans.taken["f"] != takenFans+1 || len(fans.writes["f"]) != fansWritten+1 {
		t.Fatalf("not taken again: %v %v", fans.taken, fans.writes)
	}
	if pump.taken["p"] != takenPump {
		t.Fatal("the working device was taken again")
	}
}

func TestJoinRejectsUnknownOutput(t *testing.T) {
	hw := Join(Route{newBench(), []string{"a"}})
	if err := hw.SetDuty("b", 50); err == nil {
		t.Fatal("no error")
	}
	if err := hw.TakeControl("b"); err == nil {
		t.Fatal("no error")
	}
}

func TestJoinShowsOnlyItsOwnOutputs(t *testing.T) {
	b := newBench()
	b.rpm["a"], b.rpm["other"] = 100, 200
	got, err := Join(Route{b, []string{"a"}}).Speeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["a"] != 100 {
		t.Fatalf("%v", got)
	}
}

// slowBench blocks in Speeds until it is released.
type slowBench struct {
	*bench
	entered, release chan struct{}
}

func (s slowBench) Speeds() (map[string]int, error) {
	close(s.entered)
	<-s.release
	return s.bench.Speeds()
}

func TestStatusDoesNotWaitForTheDevice(t *testing.T) {
	s := slowBench{newBench(), make(chan struct{}), make(chan struct{})}
	c, err := New([]Fan{joinFan("f")}, s, func() map[string]float64 { return nil })
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Tick()
	}()
	<-s.entered
	got := make(chan []Status, 1)
	go func() { got <- c.Status() }()
	select {
	case st := <-got:
		if len(st) != 1 || st[0].ID != "f" {
			t.Fatalf("%+v", st)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Status waits for the device")
	}
	c.Reacquire() // must not block either
	close(s.release)
	<-done
}

func TestPacer(t *testing.T) {
	t0 := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	p := newPacer(t0)
	if slept, resend := p.beat(t0.Add(time.Second)); slept || resend {
		t.Fatal("a normal tick does nothing")
	}
	if slept, resend := p.beat(t0.Add(resendEvery + time.Second)); !slept || resend {
		t.Fatal("a long pause counts as sleep")
	}
	// Ticks every second from here on: resent after one minute.
	now := t0.Add(resendEvery + time.Second)
	sent := 0
	for range 61 {
		now = now.Add(time.Second)
		if _, resend := p.beat(now); resend {
			sent++
		}
	}
	if sent != 1 {
		t.Fatalf("resent %d times in a minute", sent)
	}
}

func TestReacquireTakesEffectOnTheNextTick(t *testing.T) {
	b := newBench()
	b.rpm["f"] = 900
	c, err := New([]Fan{joinFan("f")}, b, func() map[string]float64 { return map[string]float64{"t": 40} })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	c.Reacquire()
	before := len(b.writes["f"])
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	if b.taken["f"] != 2 || len(b.writes["f"]) != before+1 {
		t.Fatalf("taken %d writes %v", b.taken["f"], b.writes["f"])
	}
}

func TestThePeriodicResendTakesControlAgain(t *testing.T) {
	r := newRig(t, plain("a"))
	r.tick(2)
	if r.hw.taken["a"] != 1 {
		t.Fatalf("taken %d", r.hw.taken["a"])
	}
	before := len(r.hw.writes["a"])
	r.c.resend.Store(true)
	r.tick(1)
	if r.hw.taken["a"] != 2 || len(r.hw.writes["a"]) != before+1 {
		t.Fatalf("taken %d, writes %d -> %d", r.hw.taken["a"], before, len(r.hw.writes["a"]))
	}
}
