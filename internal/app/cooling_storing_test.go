package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The API never gives a file name: the damaged file is named in the GUI only.
func TestAPIAlarmWithoutFileName(t *testing.T) {
	a := apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: coolingAlarmConfigCopy, Name: "cooling.json.broken-20260101"}})
	if a == nil || a.Code != "config_backup" || a.Name != "" {
		t.Fatalf("%+v", a)
	}
	if a := apiAlarm(CoolingView{Alarm: &CoolingAlarm{Kind: coolingAlarmService}}); a == nil || a.Code != "service" || a.Name != "" {
		t.Fatalf("%+v", a)
	}
}

// Changing the mode and activating a preset go through the same storing as a
// saved configuration: the safety copy follows, and the guard stays.
func TestModeAndPresetKeepTheSafetyCopyAndTheGuard(t *testing.T) {
	quickGrace(t)
	r := newLiveRig(t, false)
	farm := &guardFarm{}
	r.e.GuardSpawn = farm.spawn
	r.run(t)
	waitUntil(t, "armed", func() bool { return farm.count() == 1 })
	dir := r.e.Store.Dir
	same := func(what string) string {
		t.Helper()
		main, bak := readFile(t, filepath.Join(dir, coolingFile)), readFile(t, filepath.Join(dir, coolingBackupFile))
		if main != bak {
			t.Fatalf("%s: the copy differs from the file", what)
		}
		return main
	}
	p, err := r.e.SaveCoolingPreset("A")
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(coolingLiveExample, `"safe_duty":70`, `"safe_duty":65`, 1)
	if err := r.e.SetCoolingConfigJSON([]byte(changed)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(same("changed"), `"safe_duty": 65`) {
		t.Fatal("not the changed settings")
	}
	if err := r.e.ActivateCoolingPreset(p.ID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(same("preset"), `"safe_duty": 70`) {
		t.Fatal("the copy does not hold the preset")
	}
	time.Sleep(150 * time.Millisecond)
	if farm.count() != 1 || farm.first().ended() || farm.first().said("bye") {
		t.Fatalf("a preset change let the guard go: guards %d, ended %v", farm.count(), farm.first().ended())
	}
	if err := r.e.SetCoolingMode("preview"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(same("mode"), `"mode": "preview"`) {
		t.Fatal("the copy does not hold the mode")
	}
}
