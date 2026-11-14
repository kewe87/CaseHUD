package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/metrics"
)

// A pump that stands is told in a preview too.
func TestAStandingPumpIsAnAlarmInAPreviewToo(t *testing.T) {
	v := CoolingView{Outputs: []CoolingOutputView{{ID: "pump", Name: "Pumpe", Pump: true, PumpDown: true}}}
	a := coolingAlarmOf(v, time.Now(), time.Now())
	if a == nil || a.Kind != coolingAlarmPump || a.Name != "Pumpe" {
		t.Fatalf("%+v", a)
	}
}

// The floor of a pump is the cooling package's own rule, in every mode.
func TestAPumpBelowTheFloorIsRefusedInAPreview(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}, {Key: pumpRPMKey, Value: 3000, Unit: "RPM"}}}
	e := coolingEngine(t, strings.Replace(strings.Replace(pumpExample, `"live"`, `"preview"`, 1), `"min_duty":50`, `"min_duty":20`, 1), p)
	cfg, err := e.Store.loadCoolingConfig()
	if err == nil {
		_, err = newCoolingPreview(cfg, e.prov)
	}
	if err == nil || !strings.Contains(err.Error(), "pump") {
		t.Fatalf("a pump at 20 %% was accepted: %v", err)
	}
	if cooling.LowestPumpDuty != 30 {
		t.Fatalf("floor %d", cooling.LowestPumpDuty)
	}
}

func TestBoardIdentityOfAPumpVerdict(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: pumpRPMKey, Path: "Board A/Chip/Fans/Fan #6", Value: 3000, Unit: "RPM"},
	}}
	e := coolingEngine(t, pumpExample, p)
	cfg, _ := e.Store.loadCoolingConfig()
	write := func(board string) {
		rec := `{"v":1,"pumps":{"pump":{"control":"/lpc/chip/0/control/6","sensor":"` + pumpRPMKey + `","result":"ok"` + board + `}}}`
		if err := os.WriteFile(filepath.Join(e.Store.Dir, coolingMatchFile), []byte(rec), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	state := func() pumpState {
		b := newBoardPumps(cfg, e.prov, func() (string, string) { return "", "" })
		e.applyMatches(b)
		return b.state("pump")
	}
	if id := e.pumpBoardID("/lpc/chip/0/control/6", pumpRPMKey); id != "chip|Board A" {
		t.Fatalf("identity %q", id)
	}
	write(``) // a record from before: checked again
	if s := state(); s != pumpPending {
		t.Fatalf("old record: %v", s)
	}
	write(`,"board":"chip|Board B"`)
	if s := state(); s != pumpPending {
		t.Fatalf("other board: %v", s)
	}
	write(`,"board":"other|Board A"`)
	if s := state(); s != pumpPending {
		t.Fatalf("other chip: %v", s)
	}
	write(`,"board":"chip|Board A"`)
	if s := state(); s != pumpMatched {
		t.Fatalf("same board: %v", s)
	}
}

func TestSameBoard(t *testing.T) {
	for _, c := range []struct {
		stored, now string
		want        bool
	}{
		{"", "chip", false}, {"chip", "", false}, {"chip|A", "chip|A", true}, {"chip|A", "chip|B", false},
		{"chip|A", "chip", true}, {"chip", "chip|A", false}, {"a|A", "b", false},
	} {
		if got := sameBoard(c.stored, c.now); got != c.want {
			t.Errorf("%q vs %q: %v", c.stored, c.now, got)
		}
	}
}

// A handover is no unclean end; a failed one leaves the guard in place.
func TestAHandoverLeavesNoDirtyGuard(t *testing.T) {
	e := &Engine{}
	e.crash.link = nil
	e.guardLoopEnded(cooling.ErrDetached) // no link: nothing to do, must not panic
	e.guardHandoverFailed(errors.New("x"))
	if e.crash.dirty {
		t.Fatal("dirty without a guard")
	}
}

// The program is ending: a save only writes the file.
func TestNoLoopStartsOnceTheProgramEnds(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{{Key: "/t", Value: 30, Unit: "°C"}}}
	e := coolingEngine(t, coolingExample, p)
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	cancel()
	cfg, _ := e.Store.loadCoolingConfig()
	if err := e.launchCooling(cfg, 0); err != nil {
		t.Fatal(err)
	}
	if c := e.coolingSnapshot(); c.ctrl != nil || c.done != nil {
		t.Fatalf("a loop was started: %+v", c)
	}
}

// A fan on a manual value has no temperature of its own: the measurement is
// guarded by the hottest source of the configuration instead of stopping at once.
func TestAManualFanCanBeMeasured(t *testing.T) {
	curve := cooling.CurveSpec{ID: "m", Kind: cooling.KindFixed, Value: 50}
	fan := cooling.Fan{ID: "port1-fan1", CurveID: "m", StepUp: 100, StepDown: 100, SafeDuty: 50}
	ctrl, err := cooling.NewShared([]cooling.CurveSpec{curve}, []cooling.Fan{fan}, cooling.NoOutput{}, func() map[string]float64 { return nil })
	if err != nil {
		t.Fatal(err)
	}
	ctrl.Tick()
	temp := 40.0
	check := coolingMeasureCheck(ctrl, "port1-fan1", false, func() (float64, bool) { return temp, true })
	if err := check(); err != nil {
		t.Fatalf("stopped at once: %v", err)
	}
	gone := coolingMeasureCheck(ctrl, "port1-fan1", false, func() (float64, bool) { return 0, false })
	if err := gone(); !errors.Is(err, cooling.ErrSensorLost) {
		t.Fatalf("with no source at all it must stop: %v", err)
	}
}
