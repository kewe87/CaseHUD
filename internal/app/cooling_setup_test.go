package app

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// No test in this package reaches a real fan controller, whatever it configures.
func init() {
	refuse := errors.New("no real controller in tests")
	coolingOpenFans = func(context.Context) (lianlitl.FanDevice, error) { return nil, refuse }
	coolingListFans = func(context.Context) ([]string, error) { return nil, refuse }
	coolingOpenPumpLine = func() pumpLine { return refusedPumpLine{refuse} }
}

type refusedPumpLine struct{ err error }

func (l refusedPumpLine) Hold(string, int) error { return l.err }
func (refusedPumpLine) Close() error             { return nil }

func setupEngine(t *testing.T, file string) *Engine {
	t.Helper()
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Path: "Board/T", Value: 30, Unit: "°C"},
		{Key: "/a", Path: "Aaa/T", Value: 40, Unit: "°C"},
		{Key: "/pump", Path: "Board/Pump", Value: 3000, Unit: "RPM"},
		{Key: "/load", Path: "Board/Load", Value: 3, Unit: "%"},
	}}
	e := coolingEngine(t, file, p)
	e.Demo = true // no device, no live loop
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.startCooling()
	t.Cleanup(func() {
		cancel()
		e.waitCooling()
	})
	return e
}

func TestCoolingSetupOffersSensorsByUnit(t *testing.T) {
	e := setupEngine(t, "")
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	set := e.CoolingSetup()
	if len(set.Temps) != 2 || set.Temps[0].Key != "/a" || set.Temps[1].Name != "Board/T" {
		t.Fatalf("temps: %+v", set.Temps)
	}
	if len(set.RPMs) != 1 || set.RPMs[0].Key != "/pump" {
		t.Fatalf("rpms: %+v", set.RPMs)
	}
	if len(set.Fans) == 0 || set.Config.Outputs == nil {
		t.Fatalf("%+v", set)
	}
}

func TestSetCoolingConfigStoresAndApplies(t *testing.T) {
	e := setupEngine(t, "")
	if e.CoolingView().Configured {
		t.Fatal("configured without a file")
	}
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.Store.Dir, coolingFile)); err != nil {
		t.Fatal(err)
	}
	v := e.CoolingView()
	if !v.Configured || len(v.Outputs) != 2 {
		t.Fatalf("%+v", v)
	}
}

func TestSetCoolingConfigRejectsBadInputAndKeepsTheOld(t *testing.T) {
	e := setupEngine(t, coolingExample)
	path := filepath.Join(e.Store.Dir, coolingFile)
	before, _ := os.ReadFile(path)
	for name, bad := range map[string]string{
		"not json":      `{`,
		"unknown field": `{"outputs":[],"x":1}`,
		"bad output":    `{"outputs":[{"id":"a","curve":"none"}]}`,
		"bad curve":     `{"curves":[{"id":"c","points":[[40,50],[20,60]],"rise_after":1,"fall_after":1}],"outputs":[]}`,
		"twice":         `{"curves":[{"id":"c","points":[[20,50]],"rise_after":1,"fall_after":1},{"id":"c","points":[[20,50]],"rise_after":1,"fall_after":1}],"outputs":[]}`,
		"unmeasured":    `{"curves":[{"id":"c","sources":["/t"],"unit":"rpm","points":[[20,500]],"rise_after":1,"fall_after":1}],"outputs":[{"id":"a","curve":"c","step_up":1,"step_down":1,"safe_duty":70}]}`,
		"unknown mode":  `{"mode":"turbo","outputs":[]}`,
		"oversized":     `{"outputs":[]}` + strings.Repeat(" ", coolingMaxBytes),
	} {
		if err := e.SetCoolingConfigJSON([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
	if v := e.CoolingView(); len(v.Outputs) != 2 || v.Error != "" {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingWithoutOutputsRunsNothing(t *testing.T) {
	e := setupEngine(t, coolingExample)
	if err := e.SetCoolingConfigJSON([]byte(`{"outputs":[]}`)); err != nil {
		t.Fatal(err)
	}
	v := e.CoolingView()
	if !v.Configured || len(v.Outputs) != 0 {
		t.Fatalf("%+v", v)
	}
}

func TestMeasureNeedsLiveControl(t *testing.T) {
	e := setupEngine(t, coolingExample)
	if err := e.StartCoolingMeasure("a"); err == nil {
		t.Fatal("measured in preview")
	}
}

func TestAnOutputWithoutACurveIsLeftAlone(t *testing.T) {
	e := setupEngine(t, coolingExample)
	cfg := strings.Replace(coolingExample, `"curve":"air",`, "", 1)
	if err := e.SetCoolingConfigJSON([]byte(cfg)); err != nil {
		t.Fatal(err)
	}
	v := e.CoolingView()
	if len(v.Outputs) != 1 || v.Outputs[0].ID != "pump" {
		t.Fatalf("%+v", v.Outputs)
	}
	if err := e.SetCoolingConfigJSON([]byte(strings.Replace(cfg, `"curve":"water",`, "", 1))); err != nil {
		t.Fatal(err)
	}
	// A pump stays watched without a curve: if it stands still, it is broken.
	if v := e.CoolingView(); !v.Configured || len(v.Outputs) != 1 || v.Outputs[0].ID != "pump" {
		t.Fatalf("%+v", v)
	}
	if got, _ := e.Store.loadCoolingConfig(); len(got.Outputs) != 2 || len(got.Outputs[0].Speeds) != 2 {
		t.Fatalf("the settings of a detached fan are gone: %+v", got)
	}
}

func TestAPumpNeedsItsSpeedSensor(t *testing.T) {
	cfg := `{"outputs":[{"id":"pump","name":"Pump","pump":true,"step_up":1,"step_down":1,"min_duty":50,"safe_duty":100}]}`
	if _, err := parseCoolingConfig([]byte(cfg)); err == nil {
		t.Fatal("a pump nobody can watch was accepted")
	}
}

func TestWatchingOnlyAPumpLeavesTheFanControllerClosed(t *testing.T) {
	e := setupEngine(t, coolingExample)
	cfg := coolingConfig{Mode: coolingModeLive, Outputs: []coolingOutputConfig{
		{ID: "pump", Name: "Pump", Pump: true, RPMSensor: "/rpm", StepUp: 1, StepDown: 1, MinDuty: 50, SafeDuty: 100},
	}}
	ctrl, err := newCoolingLive(cfg, e.prov, nil, nil) // any use of the fan device panics
	if err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Tick(); err != nil {
		t.Fatal(err)
	}
}

func TestWithMeasuredSpeedsReplacesTheMap(t *testing.T) {
	m := cooling.SpeedMap{{Duty: 20, RPM: 400}, {Duty: 100, RPM: 2000}}
	o := withMeasuredSpeeds(coolingOutputConfig{Speeds: [][]int{{10, 1}, {20, 2}}}, m)
	if len(o.Speeds) != 2 || o.Speeds[0][1] != 400 || o.Speeds[1][1] != 2000 {
		t.Fatalf("%v", o.Speeds)
	}
}

// Real run with a fake device: the fan leaves the control, comes back at its
// safe duty and the map lands in the file.
func TestMeasureSavesTheSpeedMap(t *testing.T) {
	savedSteps, savedWait := cooling.MeasureSteps, coolingMeasureWait
	cooling.MeasureSteps = []int{100, 50}
	coolingMeasureWait = func(context.Context, time.Duration) error { return nil }
	defer func() { cooling.MeasureSteps, coolingMeasureWait = savedSteps, savedWait }()

	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 1000})}
	g.dev.rpmPerDuty = 20
	e, _ := liveEngine(t, g)
	e.Demo = true // what follows the measurement must not reach a real device
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		e.waitCooling()
	}()
	e.ctx = ctx
	c := e.coolingSnapshot()
	c.ctx, c.cancel = ctx, cancel
	e.setCoolingState(c)
	if err := c.ctrl.Tick(); err != nil {
		t.Fatal(err)
	}

	if err := e.StartCoolingMeasure("pump"); err == nil {
		t.Fatal("measured a pump as a fan")
	}
	if err := e.StartCoolingMeasure("nope"); err == nil {
		t.Fatal("measured an unknown output")
	}
	if err := e.StartCoolingMeasure("port1-fan1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for e.coolingMeasuring() {
		if time.Now().After(deadline) {
			t.Fatal("did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
	m := e.CoolingMeasure()
	if m.Error != "" || len(m.Results) != 40 || !m.Results[0].Settled || len(m.Outcomes) != 1 {
		t.Fatalf("%+v", m)
	}
	got := g.dev.duty["port1-fan1"]
	if got[len(got)-1] != 70 || got[1] != 100 || got[2] != 50 {
		t.Fatalf("duties %v", got)
	}
	for _, d := range got {
		if d != 0 && d < 12 && d != 50 && d != 70 {
			t.Fatalf("duty %d is below what the controller acts on", d)
		}
	}
	// The standstill was held with a stop sent again every second.
	zeros := 0
	for _, d := range got {
		if d == 0 {
			zeros++
		}
	}
	if zeros < 30 {
		t.Fatalf("%d stops sent", zeros)
	}
	stored, err := e.Store.loadCoolingConfig()
	if err != nil {
		t.Fatal(err)
	}
	o := stored.Outputs[0]
	if n := len(o.Speeds); n != 40 || o.Speeds[n-1][0] != 100 || o.Speeds[n-1][1] != 2000 || o.Speeds[0][0] != 12 {
		t.Fatalf("%+v", o)
	}
	if c := o.Calibration; c == nil || c.Kind != "fan" || c.Stops != "yes" || c.Stop != 12 || !c.Starts || c.Start != 12 {
		t.Fatalf("%+v", o.Calibration)
	}
}

func TestConfigStaysWhileMeasuring(t *testing.T) {
	e := setupEngine(t, coolingExample)
	e.coolMeasure.Running = true
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err == nil {
		t.Fatal("changed the configuration during a measurement")
	}
}

func TestCoolingFansListIsCached(t *testing.T) {
	saved := coolingListFans
	defer func() { coolingListFans = saved }()
	calls := 0
	coolingListFans = func(context.Context) ([]string, error) {
		calls++
		return nil, errors.New("none")
	}
	savedAllowed := coolingFansAllowed
	defer func() { coolingFansAllowed = savedAllowed }()
	coolingFansAllowed = func(*Engine) bool { return true }
	e := &Engine{ctx: context.Background()}
	if got, _ := e.coolingFans(); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	e.coolingFans()
	if calls != 1 {
		t.Fatalf("%d reads", calls)
	}
}

func TestCoolingFansLeaveAHeldControllerAlone(t *testing.T) {
	saved, savedAllowed := coolingListFans, coolingFansAllowed
	defer func() { coolingListFans, coolingFansAllowed = saved, savedAllowed }()
	coolingListFans = func(context.Context) ([]string, error) {
		t.Fatal("read the controller")
		return nil, nil
	}
	coolingFansAllowed = func(*Engine) bool { return false }
	e := &Engine{ctx: context.Background()}
	if got, held := e.coolingFans(); len(got) != 0 || !held {
		t.Fatalf("%v %v", got, held)
	}
}

func TestCoolingTreatsAnUnusableReadingAsMissing(t *testing.T) {
	p := &coolingReadings{readings: []metrics.Reading{
		{Key: "/nan", Path: "Board/NaN", Value: math.NaN(), Unit: "°C"},
		{Key: "/inf", Path: "Board/Inf", Value: math.Inf(1), Unit: "RPM"},
		{Key: "/t", Path: "Board/T", Value: 30, Unit: "°C"},
	}}
	e := coolingEngine(t, "", p)
	e.Demo = true
	e.ctx = context.Background()
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	if temps := coolingTemps(e.prov); len(temps) != 1 || temps["/t"] != 30 {
		t.Fatalf("%v", temps)
	}
	if _, err := json.Marshal(e.CoolingSetup()); err != nil {
		t.Fatal(err)
	}
}
