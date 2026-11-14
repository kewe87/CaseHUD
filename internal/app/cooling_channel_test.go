package app

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
)

const (
	chanDerived = "/lpc/chip/0/control/6"
	chanChosen  = "/lpc/chip/0/control/2"
)

// chanConfig is a mainboard pump on the speed sensor fan/6 with an optional chosen channel.
func chanConfig(control string, extra ...string) string {
	c := ""
	if control != "" {
		c = `,"control_key":"` + control + `"`
	}
	return `{"mode":"live","curves":[
 {"id":"water","name":"Water","sources":["/t"],"points":[[20,60],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[
 {"id":"pump","name":"Pumpe","pump":true,"rpm_sensor":"` + pumpRPMKey + `","curve":"water","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100` + c + `}` +
		strings.Join(extra, "") + `]}`
}

func TestAControlChannelIsOnlyAcceptedOnTheChipOfTheSensorAndNotTwice(t *testing.T) {
	// The second pump derives /lpc/chip/0/control/3.
	second := func(control string) string {
		c := ""
		if control != "" {
			c = `,"control_key":"` + control + `"`
		}
		return `,{"id":"pump2","name":"P2","pump":true,"rpm_sensor":"/lpc/chip/0/fan/3","curve":"water","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100` + c + `}`
	}
	// The same speed sensor, so the same derived channel.
	sameSensor := `,{"id":"pump2","name":"P2","pump":true,"rpm_sensor":"` + pumpRPMKey + `","curve":"water","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}`
	for name, tc := range map[string]struct {
		cfg string
		ok  bool
	}{
		"none (derived)":          {chanConfig(""), true},
		"other channel":           {chanConfig(chanChosen), true},
		"its own derived one":     {chanConfig(chanDerived), true},
		"other chip":              {chanConfig("/lpc/other/0/control/2"), false},
		"other chip instance":     {chanConfig("/lpc/chip/1/control/2"), false},
		"not a control":           {chanConfig("/lpc/chip/0/fan/2"), false},
		"free text":               {chanConfig("control 2"), false},
		"path trick":              {chanConfig("/lpc/chip/0/control/2/../../x"), false},
		"two pumps, no choice":    {chanConfig("", second("")), true},
		"two pumps, one sensor":   {chanConfig("", sameSensor), false},
		"one sensor, one chosen":  {chanConfig(chanChosen, sameSensor), true},
		"taken by a derived one":  {chanConfig("/lpc/chip/0/control/3", second("")), false},
		"taken the other way":     {chanConfig("", second(chanDerived)), false},
		"both explicit, equal":    {chanConfig("/lpc/chip/0/control/5", second("/lpc/chip/0/control/5")), false},
		"both explicit, distinct": {chanConfig("/lpc/chip/0/control/5", second("/lpc/chip/0/control/4")), true},
	} {
		_, err := parseCoolingConfig([]byte(tc.cfg))
		if (err == nil) != tc.ok {
			t.Errorf("%s: err %v, want ok=%v", name, err, tc.ok)
		}
	}
	// Not a pump on a mainboard header.
	fan := `{"mode":"live","curves":[{"id":"a","sources":["/t"],"points":[[20,30],[40,70]],"rise_after":1,"fall_after":1}],"outputs":[
 {"id":"port1-fan1","curve":"a","step_up":100,"step_down":100,"safe_duty":70,"control_key":"` + chanChosen + `"}]}`
	if _, err := parseCoolingConfig([]byte(fan)); err == nil {
		t.Error("a fan took a control channel")
	}
}

func TestTheControlResolutionFollowsTheChoice(t *testing.T) {
	o := coolingOutputConfig{ID: "pump", Pump: true, RPMSensor: pumpRPMKey}
	if k, ok := pumpControl(o); !ok || k != chanDerived {
		t.Fatalf("%q %v", k, ok)
	}
	o.ControlKey = chanChosen
	if k, ok := pumpControl(o); !ok || k != chanChosen {
		t.Fatalf("%q %v", k, ok)
	}
	o.ControlKey = "/lpc/other/0/control/2"
	if _, ok := pumpControl(o); ok {
		t.Fatal("a channel of another chip resolved")
	}
	cfg, err := parseCoolingConfig([]byte(chanConfig(chanChosen)))
	if err != nil {
		t.Fatal(err)
	}
	if b := newBoardPumps(cfg, NewSafeProvider(&coolingReadings{}), nil); b == nil || b.keys["pump"] != chanChosen {
		t.Fatalf("%+v", b)
	}
}

func TestAnOldConfigurationStaysAsItIs(t *testing.T) {
	cfg, err := parseCoolingConfig([]byte(chanConfig("")))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Outputs[0].ControlKey != "" {
		t.Fatal("a channel appeared")
	}
	b, _ := json.Marshal(cfg)
	if strings.Contains(string(b), "control_key") {
		t.Fatalf("written although not chosen: %s", b)
	}
}

func TestPresetsCompareTheChosenChannel(t *testing.T) {
	a, _ := parseCoolingConfig([]byte(chanConfig("")))
	b, _ := parseCoolingConfig([]byte(chanConfig(chanChosen)))
	c, _ := parseCoolingConfig([]byte(chanConfig(chanChosen)))
	if presetKey(a) == presetKey(b) {
		t.Fatal("the channel is not part of the comparison")
	}
	if presetKey(b) != presetKey(c) {
		t.Fatal("equal configurations differ")
	}
	if presetConfigOf(b).Outputs[0].ControlKey != chanChosen {
		t.Fatal("the preset lost the channel")
	}
}

func chanRig(t *testing.T, s *simBoard, control string) *matchRig {
	t.Helper()
	useSimBoard(t, s)
	r := &matchRig{s: s}
	r.e = coolingEngine(t, chanConfig(control), s)
	var err error
	if r.cfg, err = r.e.Store.loadCoolingConfig(); err != nil {
		t.Fatal(err)
	}
	r.start(t)
	return r
}

func TestTheCheckReachesTheChosenChannelAndNeverTheDerivedOne(t *testing.T) {
	s := newSimBoard(60)
	s.controlKey, s.followKey = chanChosen, chanChosen
	r := chanRig(t, s, chanChosen)
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	if rec, ok := r.record(t); !ok || rec.Control != chanChosen || rec.Result != matchResultOK {
		t.Fatalf("%+v %v", rec, ok)
	}
	r.ctrl.Tick()
	if len(s.keys) == 0 || slices.ContainsFunc(s.keys, func(k string) bool { return k != chanChosen }) {
		t.Fatalf("written to %v", s.keys)
	}
	if v := r.e.CoolingView().Outputs[0]; v.NotMatched {
		t.Fatalf("%+v", v)
	}
}

func TestAWronglyChosenChannelOnlySeesTheBriefRaiseAndIsNotMatched(t *testing.T) {
	s := newSimBoard(60)
	s.controlKey, s.followKey = chanChosen, chanDerived // the pump really sits on another channel
	r := chanRig(t, s, chanChosen)
	r.check()
	if r.pumps.state("pump") != pumpNotMatched {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	if s.lowest < 60 || s.cur != 0 {
		t.Fatalf("lowest %d held %d", s.lowest, s.cur)
	}
	n := len(s.keys)
	for range 3 {
		r.ctrl.Tick()
	}
	if len(s.keys) != n {
		t.Fatal("written after the failed check")
	}
	if outputProblem(r.e.CoolingView().Outputs[0]) != "not_matched" {
		t.Fatal("not told")
	}
}

func TestAChangedChannelIsANewPairHeldByTheBoardUntilProvenAndReleased(t *testing.T) {
	s := newSimBoard(60)
	r := newMatchRig(t, s) // derived channel
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatal("not proven first")
	}
	before := len(s.keys)

	cfg := r.cfg
	cfg.Outputs = slices.Clone(cfg.Outputs)
	cfg.Outputs[0].ControlKey = chanChosen
	r.e.dropChangedPairs(r.cfg, cfg)
	if _, ok := r.record(t); ok {
		t.Fatal("the result of the old pair stays")
	}
	r.cfg = cfg
	s.controlKey, s.followKey, s.denied = chanChosen, chanChosen, true
	r.start(t)
	if r.pumps.state("pump") != pumpPending {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	for range 3 {
		r.ctrl.Tick()
	}
	if len(s.keys) != before {
		t.Fatalf("written for the new pair: %v", s.keys[before:])
	}
	// Not released: the service refuses, nothing is set.
	if d := r.check(); d != pumpMatchDenied {
		t.Fatalf("delay %v", d)
	}
	if len(s.keys) != before || r.pumps.state("pump") != pumpPending {
		t.Fatalf("keys %v state %v", s.keys[before:], r.pumps.state("pump"))
	}
	r.pumps.notReleased.Store("pump", struct{}{})
	if o := r.e.CoolingView().Outputs[0]; !o.NotReleased || outputProblem(o) != "not_released" {
		t.Fatalf("%+v", o)
	}
	// Released: the check runs on the new channel.
	s.set(func(s *simBoard) { s.denied = false })
	r.check()
	if r.pumps.state("pump") != pumpMatched {
		t.Fatalf("state %v", r.pumps.state("pump"))
	}
	if slices.Contains(s.keys[before:], chanDerived) {
		t.Fatalf("derived channel written: %v", s.keys[before:])
	}
}

func TestReleasingAChosenChannelAsksForThatHeader(t *testing.T) {
	r := newLiveRig(t, false)
	cfg := strings.Replace(coolingLiveExample, `"min_duty":50`, `"control_key":"`+chanChosen+`","min_duty":50`, 1)
	if err := os.WriteFile(r.file, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	r.e.startCooling()
	stopWith(t, r.e)
	var asked []string
	before := coolingAllowHeader
	coolingAllowHeader = func(k string) error { asked = append(asked, k); return nil }
	t.Cleanup(func() { coolingAllowHeader = before })
	r.e.coolingSnapshot().pumps.notReleased.Store("pump", struct{}{})
	if err := r.e.ReleaseCoolingPump("pump"); err != nil || !slices.Equal(asked, []string{chanChosen}) {
		t.Fatalf("%v %v", err, asked)
	}
	// The stored proof is for the derived channel: it does not apply.
	if r.e.coolingSnapshot().pumps.state("pump") == pumpMatched {
		t.Fatal("the proof of the old pair applied")
	}
}

func TestSavingAnotherChannelDropsTheResultAndWritesNothing(t *testing.T) {
	r := newLiveRig(t, false)
	r.e.prov = NewSafeProvider(&coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: pumpRPMKey, Value: 3000, Unit: "RPM"},
		{Key: chanDerived, Value: 60, Unit: "%"},
		{Key: chanChosen, Value: 40, Unit: "%"},
	}})
	r.run(t)
	stopWith(t, r.e)
	if r.e.coolingSnapshot().pumps.state("pump") != pumpMatched {
		t.Fatal("not proven at the start")
	}
	with := func(key string) []byte {
		return []byte(strings.Replace(coolingLiveExample, `"min_duty":50`, `"control_key":"`+key+`","min_duty":50`, 1))
	}
	if err := r.e.SetCoolingConfigJSON(with("/lpc/chip/0/control/9")); err == nil {
		t.Fatal("a channel the board does not list was accepted")
	}
	if err := r.e.SetCoolingConfigJSON(with("/lpc/other/0/control/2")); err == nil {
		t.Fatal("a channel of another chip was accepted")
	}
	r.line.err = enram.ErrDenied // the new header is not released
	n := len(r.line.keys)
	if err := r.e.SetCoolingConfigJSON(with(chanChosen)); err != nil {
		t.Fatal(err)
	}
	stopWith(t, r.e)
	c := r.e.coolingSnapshot()
	if c.pumps == nil || c.pumps.keys["pump"] != chanChosen || c.pumps.state("pump") != pumpPending {
		t.Fatalf("%+v", c.pumps)
	}
	if _, ok := r.e.loadMatches()["pump"]; ok {
		t.Fatal("the result of the old pair stays")
	}
	if len(r.line.keys) != n {
		t.Fatalf("written: %v", r.line.keys[n:])
	}
}

func TestTheSetupOffersOnlyRealControlChannelsWithTheirDuty(t *testing.T) {
	e := coolingEngine(t, "", &coolingReadings{readings: []metrics.Reading{
		{Key: "/t", Value: 30, Unit: "°C"},
		{Key: chanChosen, Path: "Board/Control #2", Value: 40.4, Unit: "%"},
		{Key: chanDerived, Path: "Board/Control #6", Value: 101, Unit: "%"},
		{Key: "/lpc/chip/0/load/1", Value: 5, Unit: "%"},
		{Key: "/lpc/chip/0/fan/6", Value: 5, Unit: "%"},
		{Key: "/lpc/chip/0/control/x", Value: 5, Unit: "%"},
	}})
	got := e.CoolingSetup().Controls
	if len(got) != 2 || got[0].Key != chanChosen || got[0].Duty == nil || *got[0].Duty != 40 || got[1].Key != chanDerived || got[1].Duty != nil {
		t.Fatalf("%+v", got)
	}
}
