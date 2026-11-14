package app

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// A second scheme for the example: the pump follows a steeper curve.
var coolingEvening = strings.Replace(coolingExample, `"points":[[20,60],[40,60]]`, `"points":[[20,50],[40,90]]`, 1)

func presetEngine(t *testing.T) *Engine {
	t.Helper()
	e := setupEngine(t, coolingExample)
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	return e
}

func wantPresetCode(t *testing.T, err error, code string) {
	t.Helper()
	var pe *CoolingPresetError
	if !errors.As(err, &pe) || pe.Code != code {
		t.Fatalf("got %v, want code %q", err, code)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCoolingPresetsSwitchAndAreDerived(t *testing.T) {
	e := presetEngine(t)
	day, err := e.SaveCoolingPreset("  Tag ")
	if err != nil || day.Name != "Tag" {
		t.Fatalf("%+v %v", day, err)
	}
	if !regexp.MustCompile(`^tag-[a-z0-9]{4}$`).MatchString(day.ID) {
		t.Fatalf("id %q", day.ID)
	}
	if v := e.CoolingPresets(); v.Active != day.ID || len(v.Presets) != 1 {
		t.Fatalf("%+v", v)
	}
	if err := e.SetCoolingConfigJSON([]byte(coolingEvening)); err != nil {
		t.Fatal(err)
	}
	// Changed by hand: no preset is active any more.
	if v := e.CoolingPresets(); v.Active != "" {
		t.Fatalf("active after a manual change: %+v", v)
	}
	if v := e.CoolingView(); v.Preset != "" {
		t.Fatalf("view: %q", v.Preset)
	}
	evening, err := e.SaveCoolingPreset("Abend")
	if err != nil {
		t.Fatal(err)
	}
	if e.CoolingPresets().Active != evening.ID {
		t.Fatal("the saved one is not active")
	}
	if err := e.ActivateCoolingPreset(day.ID); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingPresets(); v.Active != day.ID {
		t.Fatalf("%+v", v)
	}
	cfg, err := e.Store.loadCoolingConfig()
	if err != nil || presetKey(cfg) != presetKey(e.coolingSnapshot().cfg) {
		t.Fatalf("file and running differ: %v", err)
	}
	if c, _ := cfg.curve("water"); c.Points[0][1] != 60 {
		t.Fatalf("the preset's curve is not in the file: %+v", c.Points)
	}
}

func TestCoolingPresetKeepsModeAndSpeeds(t *testing.T) {
	e := presetEngine(t)
	a, err := e.SaveCoolingPreset("A")
	if err != nil {
		t.Fatal(err)
	}
	evening := strings.Replace(coolingEvening, `"curves"`, `"mode":"preview","curves"`, 1)
	evening = strings.Replace(evening, `"speeds":[[20,500],[100,1500]]`, `"speeds":[[20,600],[100,1700]]`, 1)
	if err := e.SetCoolingConfigJSON([]byte(evening)); err != nil {
		t.Fatal(err)
	}
	// A new measurement does not make the preset another one.
	b, _ := e.SaveCoolingPreset("B")
	if e.CoolingPresets().Active != b.ID {
		t.Fatal("B should be active")
	}
	if err := e.ActivateCoolingPreset(a.ID); err != nil {
		t.Fatal(err)
	}
	c := e.coolingSnapshot().cfg
	if c.Mode != "preview" {
		t.Fatalf("mode %q", c.Mode)
	}
	if got := c.Outputs[0].Speeds; len(got) != 2 || got[0][1] != 600 || got[1][1] != 1700 {
		t.Fatalf("the measurement of the fan was replaced: %v", got)
	}
	if e.CoolingPresets().Active != a.ID {
		t.Fatal("A should be active")
	}
}

func TestCoolingPresetActiveOneHasNoEffect(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	rev := e.coolingRevision()
	file := readFile(t, filepath.Join(e.Store.Dir, coolingFile))
	if err := e.ActivateCoolingPreset(a.ID); err != nil {
		t.Fatal(err)
	}
	if e.coolingRevision() != rev || readFile(t, filepath.Join(e.Store.Dir, coolingFile)) != file {
		t.Fatal("an active preset was applied again")
	}
}

func TestCoolingPresetIDStaysAndNamesAreChecked(t *testing.T) {
	e := presetEngine(t)
	p, err := e.SaveCoolingPreset("Größe Ünder / 日本")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^groesse-uender-[a-z0-9]{4}$`).MatchString(p.ID) {
		t.Fatalf("id %q", p.ID)
	}
	if err := e.RenameCoolingPreset(p.ID, "Neu"); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingPresets(); len(v.Presets) != 1 || v.Presets[0].ID != p.ID || v.Presets[0].Name != "Neu" {
		t.Fatalf("%+v", v)
	}
	if _, err := e.SaveCoolingPreset("neu"); err == nil {
		t.Fatal("the same name twice")
	} else {
		wantPresetCode(t, err, PresetBadRequest)
	}
	wantPresetCode(t, e.RenameCoolingPreset("nope", "X"), PresetNotFound)
	for _, bad := range []string{"", "   ", strings.Repeat("a", 61), "a\x00b", "a\nb", "\xff"} {
		if _, err := e.SaveCoolingPreset(bad); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	if _, err := e.SaveCoolingPreset(strings.Repeat("ä", 60)); err != nil {
		t.Fatalf("60 characters: %v", err)
	}
	if _, err := e.SaveCoolingPreset(""); err == nil {
		t.Fatal("empty accepted")
	}
	// Only a plain slug is made of a name without letters.
	q, err := e.SaveCoolingPreset("日本")
	if err != nil || !strings.HasPrefix(q.ID, "preset-") {
		t.Fatalf("%+v %v", q, err)
	}
}

func TestCoolingPresetsAreLimited(t *testing.T) {
	e := presetEngine(t)
	for i := range coolingMaxPresets {
		if _, err := e.SaveCoolingPreset("P" + strings.Repeat("x", i)); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	_, err := e.SaveCoolingPreset("one more")
	wantPresetCode(t, err, PresetBadRequest)
	if v := e.CoolingPresets(); len(v.Presets) != coolingMaxPresets {
		t.Fatalf("%d presets", len(v.Presets))
	}
	ids := map[string]bool{}
	for _, p := range e.CoolingPresets().Presets {
		if !coolingPresetID.MatchString(p.ID) || ids[p.ID] {
			t.Fatalf("id %q", p.ID)
		}
		ids[p.ID] = true
	}
	// Deleting makes room again.
	if err := e.DeleteCoolingPreset(e.CoolingPresets().Presets[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveCoolingPreset("one more"); err != nil {
		t.Fatal(err)
	}
}

func TestCoolingPresetOverwriteAndDelete(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	if err := e.SetCoolingConfigJSON([]byte(coolingEvening)); err != nil {
		t.Fatal(err)
	}
	if e.CoolingPresets().Active != "" {
		t.Fatal("still active")
	}
	if err := e.OverwriteCoolingPreset(a.ID); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingPresets(); v.Active != a.ID || v.Presets[0].Name != "A" {
		t.Fatalf("%+v", v)
	}
	wantPresetCode(t, e.OverwriteCoolingPreset("nope"), PresetNotFound)
	wantPresetCode(t, e.DeleteCoolingPreset("nope"), PresetNotFound)
	if err := e.DeleteCoolingPreset(a.ID); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingPresets(); len(v.Presets) != 0 || v.Active != "" {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingPresetRefusals(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	if err := e.SetCoolingConfigJSON([]byte(coolingEvening)); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, filepath.Join(e.Store.Dir, coolingFile))
	rev := e.coolingRevision()
	same := func(what string) {
		t.Helper()
		if readFile(t, filepath.Join(e.Store.Dir, coolingFile)) != before || e.coolingRevision() != rev {
			t.Fatalf("%s: the configuration changed", what)
		}
	}

	wantPresetCode(t, e.ActivateCoolingPreset("nope"), PresetNotFound)
	same("not found")

	e.coolMu.Lock()
	e.coolMeasure.Running = true
	e.coolMu.Unlock()
	wantPresetCode(t, e.ActivateCoolingPreset(a.ID), PresetBusy)
	same("busy")
	e.coolMu.Lock()
	e.coolMeasure.Running = false
	e.coolMu.Unlock()

	// A preset that no longer fits: another fan, a sensor that is gone, a rule broken.
	for name, bad := range map[string]string{
		"other fan":   strings.Replace(coolingExample, `"id":"fan","name":"Top","curve":"air","speeds":[[20,500],[100,1500]],`, `"id":"port4-fan1","name":"Top","curve":"air","speeds":[[20,500],[100,1500]],`, 1),
		"gone sensor": strings.ReplaceAll(coolingExample, `"/t"`, `"/gone"`),
		"gone pump":   strings.Replace(coolingExample, `"/pump"`, `"/gone"`, 1),
		"pump no rpm": strings.Replace(coolingExample, `"rpm_sensor":"/pump",`, ``, 1),
	} {
		cfg, err := parseCoolingConfig([]byte(bad))
		if err != nil && name != "pump no rpm" {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "pump no rpm" { // not storable through the checks, but a file can be edited
			cfg, _ = parseCoolingConfig([]byte(coolingExample))
			cfg.Outputs[1].RPMSensor = ""
		}
		list, _ := e.Store.loadCoolingPresets()
		list = append(list, coolingPreset{ID: "bad", Name: "Bad", Config: cfg})
		if err := e.Store.saveCoolingPresets(list); err != nil {
			t.Fatal(err)
		}
		err = e.ActivateCoolingPreset("bad")
		wantPresetCode(t, err, PresetInvalid)
		if want := map[string]string{"other fan": "port4-fan1", "gone sensor": "/gone", "gone pump": "/gone", "pump no rpm": "rpm_sensor"}[name]; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: refused for another reason: %v", name, err)
		}
		same(name)
		if err := e.DeleteCoolingPreset("bad"); err != nil {
			t.Fatal(err)
		}
	}
	if v := e.CoolingView(); len(v.Outputs) != 2 || v.Error != "" {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingPresetNeedsTheAreaAndASetup(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	e.mu.Lock()
	e.cfg.NoCooling = true
	e.mu.Unlock()
	wantPresetCode(t, e.ActivateCoolingPreset(a.ID), PresetCoolingOff)
	if _, err := e.SaveCoolingPreset("B"); err == nil {
		t.Fatal("saved with the area off")
	}
	if v := e.CoolingPresets(); v.Active != "" {
		t.Fatalf("active while off: %+v", v)
	}
	if api := e.CoolingAPIView(); api.Enabled || api.State != CoolingStateOff || api.Preset != nil || len(api.Presets) != 0 || len(api.Outputs) != 0 {
		t.Fatalf("%+v", api)
	}

	// Nothing set up.
	u := setupEngine(t, "")
	list := []coolingPreset{{ID: "a", Name: "A", Config: func() coolingConfig { c, _ := parseCoolingConfig([]byte(coolingExample)); return c }()}}
	if err := u.Store.saveCoolingPresets(list); err != nil {
		t.Fatal(err)
	}
	wantPresetCode(t, u.ActivateCoolingPreset("a"), PresetUnconfigured)
	api := u.CoolingAPIView()
	if api.State != CoolingStateUnconfigured || len(api.Presets) != 1 || len(api.Outputs) != 0 || len(api.Temps) != 0 || api.Preset != nil {
		t.Fatalf("%+v", api)
	}
	if _, err := u.SaveCoolingPreset("B"); err == nil {
		t.Fatal("saved nothing")
	}
}

func TestCoolingPresetBlockedByAnotherProgram(t *testing.T) {
	g := &openGate{dev: newFakeTL(map[string]int{"port1-fan1": 900}), reason: coolingBlockedProgram, program: "Rival"}
	e, _ := liveEngine(t, g)
	e.cooling.configured = true
	for range 2 {
		e.cooling.ctrl.Tick()
	}
	cfg := e.cooling.cfg
	if err := e.Store.saveCoolingPresets([]coolingPreset{{ID: "a", Name: "A", Config: presetConfigOf(cfg)}}); err != nil {
		t.Fatal(err)
	}
	// The running one is not that preset: change the pump curve in the preset.
	list, _ := e.Store.loadCoolingPresets()
	list[0].Config.Curves[1].Points = [][]float64{{20, 55}, {40, 85}}
	if err := e.Store.saveCoolingPresets(list); err != nil {
		t.Fatal(err)
	}
	wantPresetCode(t, e.ActivateCoolingPreset("a"), PresetBlocked)
	api := e.CoolingAPIView()
	if api.State != CoolingStateBlocked || api.BlockedBy != "Rival" {
		t.Fatalf("%+v", api)
	}
	if api.Alarm != nil {
		t.Fatalf("a block by a program is the state, not an alarm: %+v", api.Alarm)
	}
}

func TestCoolingPresetWriteFailureChangesNothing(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	if err := e.SetCoolingConfigJSON([]byte(coolingEvening)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(e.Store.Dir, coolingFile)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil { // writing the file now fails
		t.Fatal(err)
	}
	rev := e.coolingRevision()
	before := e.coolingSnapshot()
	wantPresetCode(t, e.ActivateCoolingPreset(a.ID), PresetStorage)
	after := e.coolingSnapshot()
	if e.coolingRevision() != rev || presetKey(before.cfg) != presetKey(after.cfg) || after.ctrl != before.ctrl {
		t.Fatal("the running configuration changed although saving failed")
	}
	if v := e.CoolingPresets(); v.Active != "" || len(v.Presets) != 1 {
		t.Fatalf("%+v", v)
	}
}

func TestCoolingPresetsFileThatCannotBeReadIsLeftAlone(t *testing.T) {
	e := presetEngine(t)
	path := filepath.Join(e.Store.Dir, coolingPresetsFile)
	for name, bad := range map[string]string{
		"syntax":    `{"presets":[`,
		"unknown":   `{"presets":[],"x":1}`,
		"bad id":    `{"presets":[{"id":"A B","name":"x","config":{"outputs":[]}}]}`,
		"twice":     `{"presets":[{"id":"a","name":"x","config":{"outputs":[]}},{"id":"a","name":"y","config":{"outputs":[]}}]}`,
		"long name": `{"presets":[{"id":"a","name":"` + strings.Repeat("x", 61) + `","config":{"outputs":[]}}]}`,
		"trailing":  `{"presets":[]} []`,
	} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := e.SaveCoolingPreset("New"); err == nil || !strings.Contains(err.Error(), coolingPresetsFile) {
			t.Errorf("%s: save: %v", name, err)
		}
		wantPresetCode(t, e.ActivateCoolingPreset("a"), PresetStorage)
		wantPresetCode(t, e.DeleteCoolingPreset("a"), PresetStorage)
		wantPresetCode(t, e.RenameCoolingPreset("a", "z"), PresetStorage)
		if v := e.CoolingPresets(); v.Error == "" || strings.Contains(v.Error, e.Store.Dir) {
			t.Errorf("%s: %+v", name, v)
		}
		if readFile(t, path) != bad {
			t.Errorf("%s: the file was overwritten", name)
		}
		if api := e.CoolingAPIView(); len(api.Presets) != 0 || api.Preset != nil {
			t.Errorf("%s: %+v", name, api)
		}
	}
}

func TestCoolingRevisionStopsAStaleSave(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("A")
	rev := e.CoolingSetup().Revision
	if rev != e.coolingRevision() {
		t.Fatal("the setup does not carry the revision")
	}
	// The page edits the evening scheme while a preset is switched.
	if err := e.SetCoolingConfigJSON([]byte(coolingEvening)); err != nil {
		t.Fatal(err)
	}
	b, _ := e.SaveCoolingPreset("B")
	if err := e.ActivateCoolingPreset(a.ID); err != nil {
		t.Fatal(err)
	}
	file := readFile(t, filepath.Join(e.Store.Dir, coolingFile))
	if err := e.SetCoolingConfigAt([]byte(coolingEvening), rev); !errors.Is(err, ErrCoolingRevision) {
		t.Fatalf("stale save: %v", err)
	}
	if readFile(t, filepath.Join(e.Store.Dir, coolingFile)) != file {
		t.Fatal("a stale save was written")
	}
	// A save with the current revision goes through, and moves the revision on.
	cur := e.coolingRevision()
	if err := e.SetCoolingConfigAt([]byte(coolingEvening), cur); err != nil {
		t.Fatal(err)
	}
	if e.coolingRevision() == cur || e.CoolingPresets().Active != b.ID {
		t.Fatal("revision or active preset did not follow")
	}
	// A mode switch is a change too: an old mode must not come back.
	cur = e.coolingRevision()
	if err := e.SetCoolingMode("preview"); err != nil {
		t.Fatal(err)
	}
	if e.coolingRevision() == cur {
		t.Fatal("the mode switch kept the revision")
	}
	if err := e.SetCoolingConfigAt([]byte(coolingEvening), cur); !errors.Is(err, ErrCoolingRevision) {
		t.Fatalf("%v", err)
	}
	// Without a revision nothing is checked.
	if err := e.SetCoolingConfigAt([]byte(coolingEvening), 0); err != nil {
		t.Fatal(err)
	}
}

func TestCoolingAPIView(t *testing.T) {
	e := presetEngine(t)
	a, _ := e.SaveCoolingPreset("Tag")
	e.coolingSnapshot().ctrl.Tick()
	api := e.CoolingAPIView()
	if !api.Enabled || api.State != CoolingStateTest || api.BlockedBy != "" || api.Preset == nil || *api.Preset != a.ID {
		t.Fatalf("%+v", api)
	}
	if len(api.Outputs) != 2 {
		t.Fatalf("%+v", api.Outputs)
	}
	fan, pump := api.Outputs[0], api.Outputs[1]
	if fan.Kind != "fan" || fan.Name != "Top" || pump.Kind != "pump" || fan.Problem != "" {
		t.Fatalf("%+v %+v", fan, pump)
	}
	if fan.Percent == nil || *fan.Percent < 0 || *fan.Percent > 100 || pump.RPM == nil || *pump.RPM != 3000 {
		t.Fatalf("%+v %+v", fan, pump)
	}
	// Only the sensor a curve reads, plus a sensor with a name of its own.
	if len(api.Temps) != 1 || api.Temps[0].ID != "/t" || api.Temps[0].Name != "Board/T" || api.Temps[0].Value != 30 {
		t.Fatalf("%+v", api.Temps)
	}
	named := strings.Replace(coolingExample, `"outputs"`, `"sensor_names":{"/a":"Raum","/pump":"Pumpe","/gone":"Weg"},"outputs"`, 1)
	if err := e.SetCoolingConfigJSON([]byte(named)); err != nil {
		t.Fatal(err)
	}
	api = e.CoolingAPIView()
	if api.Preset != nil {
		t.Fatalf("a hand-made change keeps a preset: %q", *api.Preset)
	}
	// "/pump" is an rpm sensor and "/gone" has no value: neither is a temperature.
	if len(api.Temps) != 2 || api.Temps[0].Name != "Board/T" && api.Temps[0].Name != "Raum" {
		t.Fatalf("%+v", api.Temps)
	}
	names := []string{api.Temps[0].Name, api.Temps[1].Name}
	if !(names[0] == "Board/T" && names[1] == "Raum") && !(names[0] == "Raum" && names[1] == "Board/T") {
		t.Fatalf("%v", names)
	}
}

func TestCoolingAPIKindsAlarmNamesAndLimits(t *testing.T) {
	cfg := `{"curves":[{"id":"c","name":"C","sources":["/t"],"points":[[20,60],[40,80]],"rise_after":1,"fall_after":1}],"outputs":[
	 {"id":"port1-fan1","name":"` + strings.Repeat("ü", 90) + `","curve":"c","step_up":100,"step_down":100,"safe_duty":70},
	 {"id":"port1-fan2","role":"case","curve":"c","step_up":100,"step_down":100,"min_duty":20,"safe_duty":70},
	 {"id":"pump","name":"P","pump":true,"rpm_sensor":"/pump","curve":"c","step_up":100,"step_down":100,"min_duty":50,"safe_duty":100}]}`
	e := setupEngine(t, cfg)
	if err := e.prov.Refresh(); err != nil {
		t.Skip(err)
	}
	api := e.CoolingAPIView()
	if len(api.Outputs) != 3 || api.Outputs[0].Kind != "fan" || api.Outputs[1].Kind != "case" || api.Outputs[2].Kind != "pump" {
		t.Fatalf("%+v", api.Outputs)
	}
	if n := []rune(api.Outputs[0].Name); len(n) != 60 {
		t.Fatalf("%d characters in a name", len(n))
	}
	if api.Outputs[1].Name != "port1-fan2" {
		t.Fatalf("name without one: %q", api.Outputs[1].Name)
	}
	// The alarm names the output as the user did; none was given for the case fan.
	if a := apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: coolingAlarmStalled, Name: "port1-fan2"}, Outputs: []CoolingOutputView{{ID: "port1-fan2"}}}); a == nil || a.Name != "" || a.Code != "stalled" {
		t.Fatalf("%+v", a)
	}
	if a := apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: coolingAlarmStalled, Name: "Top"}, Outputs: []CoolingOutputView{{ID: "x", Name: "Top"}}}); a == nil || a.Name != "Top" {
		t.Fatalf("%+v", a)
	}
	for kind, want := range map[string]string{
		coolingAlarmPump: "pump", coolingAlarmPumpUnknown: "pump_unknown", coolingAlarmStale: "stale", coolingAlarmConfig: "config",
		coolingAlarmDevice: "device", coolingAlarmWrite: "write", coolingAlarmSensor: "sensor", coolingAlarmStalled: "stalled",
		coolingAlarmService: "service", coolingAlarmConfigCopy: "config_backup", coolingAlarmEmergency: "emergency",
	} {
		if a := apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: kind}}); a == nil || a.Code != want {
			t.Errorf("%s: %+v", kind, a)
		}
	}
	if apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: coolingBlockedProgram, Program: "X"}}) != nil || apiAlarm(CoolingView{}) != nil {
		t.Error("a block or nothing is no alarm")
	}
}

func TestCoolingAPIProblemOrder(t *testing.T) {
	all := CoolingOutputView{PumpDown: true, DeviceLost: true, WriteFail: true, Emergency: true, Stalled: true, SensorLost: true, NotReleased: true, Unguarded: true}
	var got []string
	for range 9 {
		p := outputProblem(all)
		got = append(got, p)
		switch p {
		case "pump_down":
			all.PumpDown = false
		case "device_lost":
			all.DeviceLost = false
		case "write_failed":
			all.WriteFail = false
		case "emergency":
			all.Emergency = false
		case "stalled":
			all.Stalled = false
		case "sensor_lost":
			all.SensorLost = false
		case "not_released":
			all.NotReleased = false
		case "unguarded":
			all.Unguarded = false
		}
	}
	if strings.Join(got, ",") != "pump_down,device_lost,write_failed,emergency,stalled,sensor_lost,not_released,unguarded," {
		t.Fatalf("%v", got)
	}
}

func strPtr(s string) *string { return &s }

func TestCoolingAPIEventKeyIgnoresMeasurements(t *testing.T) {
	rpm := func(n int) *int { return &n }
	temp := func(f float64) *float64 { return &f }
	a := CoolingAPI{Enabled: true, State: "running", Outputs: []CoolingAPIOutput{{ID: "f", RPM: rpm(600), Percent: intPtr(40), Temp: temp(27.5)}}}
	b := a
	b.Outputs = []CoolingAPIOutput{{ID: "f", RPM: rpm(900), Percent: intPtr(80), Temp: temp(31)}}
	b.Temps = []CoolingAPITemp{{ID: "t", Value: 30}}
	if a.EventKey() != b.EventKey() {
		t.Fatal("a measured value is no event")
	}
	for name, c := range map[string]CoolingAPI{
		"problem": {Enabled: true, State: "running", Outputs: []CoolingAPIOutput{{ID: "f", Problem: "stalled"}}},
		"state":   {Enabled: true, State: "test", Outputs: a.Outputs},
		"alarm":   {Enabled: true, State: "running", Outputs: a.Outputs, Alarm: &CoolingAPIAlarm{Code: "stale"}},
		"preset":  {Enabled: true, State: "running", Outputs: a.Outputs, Preset: strPtr("x")},
		"blocked": {Enabled: true, State: "running", Outputs: a.Outputs, BlockedBy: "X"},
		"off":     {State: "running", Outputs: a.Outputs},
	} {
		if c.EventKey() == a.EventKey() {
			t.Errorf("%s: no change in the key", name)
		}
	}
}

func TestCoolingAPIViewJSONShape(t *testing.T) {
	// Lists are never null, the unknown values are.
	api := (&Engine{Store: &Store{Dir: t.TempDir()}, prov: NewSafeProvider(&coolingReadings{readings: []metrics.Reading{}})}).CoolingAPIView()
	if api.Presets == nil || api.Outputs == nil || api.Temps == nil {
		t.Fatalf("%+v", api)
	}
}

func TestThePresetFileIsNotReadAgainWhileItStaysAsItIs(t *testing.T) {
	e := presetEngine(t)
	day, err := e.SaveCoolingPreset("Tag")
	if err != nil {
		t.Fatal(err)
	}
	list1, keys1, err := e.Store.readCoolingPresets()
	if err != nil || len(list1) != 1 || len(keys1) != 1 || keys1[0] != presetKey(list1[0].Config) {
		t.Fatalf("%v %v %v", list1, keys1, err)
	}
	list2, _, _ := e.Store.readCoolingPresets()
	if &list1[0] != &list2[0] {
		t.Fatal("read from the disk again")
	}
	// The views still tell the active preset from memory.
	if got := e.CoolingPresets().Active; got != day.ID {
		t.Fatalf("active %q", got)
	}
	// Saving through the program changes what is read.
	if _, err := e.SaveCoolingPreset("Abend"); err != nil {
		t.Fatal(err)
	}
	if list3, keys3, _ := e.Store.readCoolingPresets(); len(list3) != 2 || len(keys3) != 2 {
		t.Fatalf("%d presets, %d keys", len(list3), len(keys3))
	}
	// So does a change of the file from outside, even to the same size.
	path := e.Store.path(coolingPresetsFile)
	b := readFile(t, path)
	if err := os.WriteFile(path, []byte(strings.Replace(b, "Abend", "Abeng", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	list4, _, _ := e.Store.readCoolingPresets()
	if len(list4) != 2 || list4[1].Name != "Abeng" {
		t.Fatalf("%+v", list4)
	}
	// A file that cannot be read is told every time, and nothing stale is kept.
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.Store.readCoolingPresets(); err == nil {
		t.Fatal("a broken file was read")
	}
	if v := e.CoolingPresets(); v.Error == "" || len(v.Presets) != 0 {
		t.Fatalf("%+v", v)
	}
}
