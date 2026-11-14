package cooling

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// safetyHW is a device that does what it is told and says so.
type safetyHW struct {
	duty     map[string]int
	rpm      map[string]int
	writeErr error
}

func newSafetyHW() *safetyHW { return &safetyHW{duty: map[string]int{}, rpm: map[string]int{}} }

func (h *safetyHW) TakeControl(string) error { return nil }
func (h *safetyHW) SetDuty(id string, p int) error {
	if h.writeErr != nil {
		return h.writeErr
	}
	h.duty[id] = p
	return nil
}
func (h *safetyHW) Speeds() (map[string]int, error) { return h.rpm, nil }

func safetyFan(id, curve string) Fan {
	return Fan{ID: id, CurveID: curve, StepUp: 100, StepDown: 100, SafeDuty: 30}
}

func safetyCurve(id string, sources ...string) CurveSpec {
	return CurveSpec{ID: id, Sources: sources, Points: Curve{{20, 30}, {60, 100}}, RiseAfter: 1, FallAfter: 1}
}

func TestShutdownNeverLowersAFan(t *testing.T) {
	temp := map[string]float64{"t": 60}
	h := newSafetyHW()
	h.rpm["a"] = 1000
	c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{safetyFan("a", "c")}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Tick(); err != nil {
		t.Fatal(err)
	}
	if h.duty["a"] != 100 {
		t.Fatalf("under load %d", h.duty["a"])
	}
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if h.duty["a"] != 100 {
		t.Fatalf("left at %d", h.duty["a"])
	}
}

func TestShutdownRaisesAQuietFanToSafe(t *testing.T) {
	temp := map[string]float64{"t": 21}
	h := newSafetyHW()
	h.rpm["a"] = 1000
	f := safetyFan("a", "c")
	f.SafeDuty = 70
	c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{f}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	c.Shutdown()
	if h.duty["a"] != 70 {
		t.Fatalf("left at %d", h.duty["a"])
	}
}

func TestEmergencyHoldsWhileANeighbourSensorIsGone(t *testing.T) {
	two := safetyCurve("c", "hot", "gone")
	other := safetyCurve("e", "gone")
	first := safetyCurve("d", "hot")
	combined := CurveSpec{ID: "m", Kind: KindCombined, Combine: CombineMax, Inputs: []string{"d", "e"}}
	gap := safetyCurve("g", "hot")
	gap.Kind, gap.Reference = KindGap, "gone"
	for name, curves := range map[string][]CurveSpec{
		"two sources": {two},
		"combined":    {first, other, combined},
		"gap":         {gap},
	} {
		h := newSafetyHW()
		h.rpm["a"] = 1000
		id := curves[len(curves)-1].ID
		c, err := NewShared(curves, []Fan{safetyFan("a", id)}, h, func() map[string]float64 { return map[string]float64{"hot": 110} })
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for range emergencyTicks + sensorGraceTicks {
			c.Tick()
		}
		if h.duty["a"] != 100 {
			t.Errorf("%s: 110 °C and duty %d", name, h.duty["a"])
		}
		if s := c.Status()[0]; !s.SensorLost || !s.HasTemp || s.Temp != 110 {
			t.Errorf("%s: %+v", name, s)
		}
	}
}

func TestCurvesThatWouldStopAFanAreRefused(t *testing.T) {
	d := safetyCurve("d", "t")
	zero := CurveSpec{ID: "z", Kind: KindFixed, Value: 0}
	hundred := CurveSpec{ID: "h", Kind: KindFixed, Value: 100}
	low := safetyFan("a", "c")
	low.Offset = -100
	onH := low
	onH.CurveID = "h"
	const never = "never asks"
	for name, tc := range map[string]struct {
		curves []CurveSpec
		fans   []Fan
		want   string
	}{
		"offset takes it all": {[]CurveSpec{func() CurveSpec { x := d; x.ID = "c"; return x }()}, []Fan{low}, never},
		"fixed minus offset":  {[]CurveSpec{hundred}, []Fan{onH}, never},
		"lowest with zero":    {[]CurveSpec{d, zero, {ID: "c", Kind: KindCombined, Combine: CombineMin, Inputs: []string{"d", "z"}}}, []Fan{safetyFan("a", "c")}, never},
		"same input twice":    {[]CurveSpec{d, {ID: "c", Kind: KindCombined, Combine: CombineSubtract, Inputs: []string{"d", "d"}}}, []Fan{safetyFan("a", "c")}, "twice"},
		"follow scaled to 0":  {[]CurveSpec{d, {ID: "c", Kind: KindFollow, Follow: "b", Proportional: true, Offset: -100}}, []Fan{safetyFan("b", "d"), safetyFan("a", "c")}, never},
		"follow offset to 0":  {[]CurveSpec{d, {ID: "c", Kind: KindFollow, Follow: "b", Offset: -100}}, []Fan{safetyFan("b", "d"), safetyFan("a", "c")}, never},
		"subtract from zero":  {[]CurveSpec{d, zero, {ID: "c", Kind: KindCombined, Combine: CombineSubtract, Inputs: []string{"z", "d"}}}, []Fan{safetyFan("a", "c")}, never},
		"drawn at zero":       {[]CurveSpec{{ID: "c", Sources: []string{"t"}, Points: Curve{{20, 0}, {60, 0}}, RiseAfter: 1, FallAfter: 1}}, []Fan{safetyFan("a", "c")}, never},
	} {
		_, err := NewShared(tc.curves, tc.fans, newSafetyHW(), func() map[string]float64 { return nil })
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestStillAcceptedCurves(t *testing.T) {
	d := safetyCurve("d", "t")
	stoppable := safetyCurve("c", "t")
	stoppable.Points = Curve{{20, 0}, {60, 100}}
	for name, curves := range map[string][]CurveSpec{
		"reaches zero when cool": {stoppable},
		"difference":             {d, {ID: "e", Sources: []string{"t"}, Points: Curve{{20, 10}, {60, 20}}, RiseAfter: 1, FallAfter: 1}, {ID: "c", Kind: KindCombined, Combine: CombineSubtract, Inputs: []string{"d", "e"}}},
	} {
		if _, err := NewShared(curves, []Fan{safetyFan("a", "c")}, newSafetyHW(), func() map[string]float64 { return nil }); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A chain where each curve takes the two before it doubles a naive walk nearly
// every level. It must not hang the start.
func TestLongChainsAreNoWork(t *testing.T) {
	curves := []CurveSpec{safetyCurve("c0", "t"), safetyCurve("c1", "t")}
	for i := 2; i < 64; i++ {
		curves = append(curves, CurveSpec{ID: fmt.Sprintf("c%d", i), Kind: KindCombined, Combine: CombineMax,
			Inputs: []string{fmt.Sprintf("c%d", i-1), fmt.Sprintf("c%d", i-2)}})
	}
	done := make(chan error, 1)
	go func() {
		_, err := NewShared(curves, []Fan{safetyFan("a", "c63")}, newSafetyHW(), func() map[string]float64 { return nil })
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hangs")
	}
}

func TestStatusSaysWhatTheFanDoes(t *testing.T) {
	temp := map[string]float64{"t": 40}
	h := newSafetyHW()
	h.rpm["a"] = 0
	c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{safetyFan("a", "c")}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	if !c.LastTick().IsZero() {
		t.Fatal("ticked before the first tick")
	}
	for range spinUpTicks + 2 {
		c.Tick()
	}
	if c.LastTick().IsZero() {
		t.Fatal("no tick noted")
	}
	if s := c.Status()[0]; !s.Stalled || s.WriteFailed {
		t.Fatalf("%+v", s)
	}
	h.rpm["a"] = 1000
	h.writeErr = errors.New("refused")
	c.Reacquire()
	if err := c.Tick(); err == nil {
		t.Fatal("no error")
	}
	if s := c.Status()[0]; s.Stalled || !s.WriteFailed {
		t.Fatalf("%+v", s)
	}
}

func TestAStandingFanIsNotCopied(t *testing.T) {
	temp := map[string]float64{"t": 40}
	h := newSafetyHW()
	h.rpm["lead"], h.rpm["b"] = 800, 800
	lead := CurveSpec{ID: "d", RPM: true, Kind: KindFixed, Value: 800}
	follow := CurveSpec{ID: "f", RPM: true, Kind: KindFollow, Follow: "lead"}
	a, b := safetyFan("lead", "d"), safetyFan("b", "f")
	a.Speeds = SpeedMap{{Duty: 20, RPM: 400}, {Duty: 100, RPM: 1600}}
	b.Speeds = a.Speeds
	c, err := NewShared([]CurveSpec{lead, follow}, []Fan{a, b}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	h.rpm["lead"] = 0
	for range sensorGraceTicks + 2 {
		c.Tick()
	}
	if s := c.Status()[1]; !s.SensorLost {
		t.Fatalf("%+v", s)
	}
	if h.duty["b"] < 30 {
		t.Fatalf("follower dragged down to %d", h.duty["b"])
	}
}

// A safe duty below the floor is refused on construction; should one get past
// anyway, the shutdown still must not leave a stopped fan behind.
func TestShutdownNeverLeavesAStoppedFan(t *testing.T) {
	temp := map[string]float64{"t": 30}
	h := newSafetyHW()
	f := safetyFan("a", "c")
	f.SafeDuty = 0
	if _, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{f}, h, func() map[string]float64 { return temp }); err == nil {
		t.Fatal("a safe duty of 0 was accepted")
	}
	f.SafeDuty = 30
	c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{f}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.outputs[0].owned, c.outputs[0].written = true, 0
	c.outputs[0].reg.cfg.SafeDuty = 0
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if h.duty["a"] != LowestSafeDuty {
		t.Fatalf("left at %d", h.duty["a"])
	}
}

func TestEndErrorTellsWhetherTheFansWereLeftSafe(t *testing.T) {
	build := func(h Hardware) *Controller {
		c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{safetyFan("a", "c")}, h, func() map[string]float64 { return map[string]float64{"t": 40} })
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	c := build(newSafetyHW())
	c.Tick()
	if err := c.EndError(); err != nil {
		t.Fatalf("before the end: %v", err)
	}
	c.Shutdown()
	if err := c.EndError(); err != nil {
		t.Fatalf("clean end: %v", err)
	}

	c = build(newSafetyHW())
	c.Tick()
	c.Detach()
	if err := c.EndError(); !errors.Is(err, ErrDetached) {
		t.Fatalf("detached: %v", err)
	}
	c.Shutdown() // after a detach it must not claim a safe end
	if err := c.EndError(); !errors.Is(err, ErrDetached) {
		t.Fatalf("shutdown after detach: %v", err)
	}

	h := newSafetyHW()
	c = build(h)
	c.outputs[0].reg.cfg.SafeDuty = 90 // above the 65 % the curve asks for, so a write is due
	c.Tick()
	h.writeErr = errors.New("gone")
	c.Shutdown()
	if c.EndError() == nil {
		t.Fatal("a failed write counted as a safe end")
	}

	// A device that was lost after the fans had been set: they may still be low.
	h = newSafetyHW()
	c = build(h)
	c.Tick()
	c.forgetHardware(false)
	c.Shutdown()
	if c.EndError() == nil {
		t.Fatal("lost hardware counted as a safe end")
	}

	// Nothing was ever written: nothing can be low.
	c = build(newSafetyHW())
	c.Shutdown()
	if err := c.EndError(); err != nil {
		t.Fatalf("untouched: %v", err)
	}
}

// A device that was let go on purpose, with its outputs at a safe duty, leaves
// nothing to put right when the control ends. One that was lost does.
func TestAnEndAfterLettingGoOnPurposeIsSafe(t *testing.T) {
	leave := &letGoHW{safetyHW: newSafetyHW()}
	build := func(h Hardware) *Controller {
		c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{safetyFan("a", "c")}, h, func() map[string]float64 { return map[string]float64{"t": 40} })
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	for name, h := range map[string]Hardware{
		"direct": leave,
		"joined": Join(Route{Device: leave, IDs: []string{"a"}}),
	} {
		leave.letGo = false
		c := build(h)
		c.Tick()
		leave.letGo = true
		c.Tick()
		c.Shutdown()
		if err := c.EndError(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// The same with a device that was lost.
	leave.letGo, leave.lostErr = false, errors.New("unplugged")
	for name, h := range map[string]Hardware{
		"direct": leave,
		"joined": Join(Route{Device: leave, IDs: []string{"a"}}),
	} {
		leave.letGo = false
		leave.lostErr = nil
		c := build(h)
		c.Tick()
		leave.lostErr = errors.New("unplugged")
		c.Tick()
		c.Shutdown()
		if c.EndError() == nil {
			t.Errorf("%s: a lost device counted as left safe", name)
		}
	}
}

type letGoHW struct {
	*safetyHW
	letGo   bool
	lostErr error
}

func (h *letGoHW) Speeds() (map[string]int, error) {
	switch {
	case h.letGo:
		return nil, ErrLeftSafe
	case h.lostErr != nil:
		return nil, h.lostErr
	}
	return h.safetyHW.Speeds()
}

// A fan that a calibration stops on purpose is no stalled fan; a pump that
// stands is always a fault, held or not. Once the fan is given back, the count
// starts anew.
func TestAHeldFanIsNotStalledButAHeldPumpIs(t *testing.T) {
	temp := map[string]float64{"t": 40}
	h := newSafetyHW()
	h.rpm["a"], h.rpm["p"] = 1000, 1000
	pump := safetyFan("p", "c")
	pump.Pump, pump.SafeDuty, pump.MinDuty = true, 100, 60
	c, err := NewShared([]CurveSpec{safetyCurve("c", "t")}, []Fan{safetyFan("a", "c"), pump}, h, func() map[string]float64 { return temp })
	if err != nil {
		t.Fatal(err)
	}
	c.Tick()
	c.Hold("a")
	c.Hold("p")
	h.rpm["a"], h.rpm["p"] = 0, 0
	for range 3 * spinUpTicks {
		c.Tick()
	}
	st := c.Status()
	if st[0].Stalled || !st[0].Held {
		t.Fatalf("held fan: %+v", st[0])
	}
	if !st[1].PumpDown {
		t.Fatalf("held pump: %+v", st[1])
	}
	c.Release("a", 50)
	for range spinUpTicks - 1 {
		c.Tick()
	}
	if c.Status()[0].Stalled {
		t.Fatal("stalled right after the release")
	}
	// Still standing a few ticks after the release: now it is a real alarm.
	for range 3 {
		c.Tick()
	}
	if s := c.Status()[0]; !s.Stalled {
		t.Fatalf("a fan that stays down after the release: %+v", s)
	}
}

// A fixed value follows no sensor; its guard sends it to full duty like a curve.
func TestGuardSendsAFixedValueToFullDuty(t *testing.T) {
	for _, pump := range []bool{false, true} {
		temps := map[string]float64{"cpu": 50, "water": 30}
		h := newSafetyHW()
		h.rpm["a"] = 1000
		f := safetyFan("a", "m")
		f.StepDown = 10
		if pump {
			f.Pump, f.MinDuty = true, LowestPumpDuty
		}
		fixed := CurveSpec{ID: "m", Kind: KindFixed, Value: 40, Guard: []string{"cpu", "water"}}
		c, err := NewShared([]CurveSpec{fixed}, []Fan{f}, h, func() map[string]float64 { return temps })
		if err != nil {
			t.Fatal(err)
		}
		tick := func() Status {
			t.Helper()
			_ = c.Tick()
			return c.Status()[0]
		}
		for range 5 {
			if s := tick(); s.Duty != 40 || s.Emergency || s.HasTemp || s.SensorLost {
				t.Fatalf("pump %v, cold: %+v", pump, s)
			}
		}
		temps["cpu"] = emergencyTemp
		for i := range emergencyTicks {
			s := tick()
			if want := i == emergencyTicks-1; s.Emergency != want || (s.Duty == 100) != want {
				t.Fatalf("pump %v, hot tick %d: %+v", pump, i+1, s)
			}
		}
		// The hot sensor is gone: what was known to be too hot stays too hot.
		delete(temps, "cpu")
		for range sensorGraceTicks + 2 {
			if s := tick(); s.Duty != 100 || !s.Emergency {
				t.Fatalf("pump %v, lost while hot: %+v", pump, s)
			}
		}
		if h.duty["a"] != 100 {
			t.Fatalf("pump %v: sent %d", pump, h.duty["a"])
		}
		// Back and cool: down by the steps to the fixed value.
		temps["cpu"] = 60
		if s := tick(); s.Emergency || s.Duty != 90 {
			t.Fatalf("pump %v, cool again: %+v", pump, s)
		}
		for range 10 {
			tick()
		}
		if s := c.Status()[0]; s.Duty != 40 || h.duty["a"] != 40 {
			t.Fatalf("pump %v, back: %+v sent %d", pump, s, h.duty["a"])
		}
		// A cool sensor that goes missing changes nothing.
		delete(temps, "water")
		if s := tick(); s.Duty != 40 || s.Emergency {
			t.Fatalf("pump %v, cool sensor lost: %+v", pump, s)
		}
	}
}
