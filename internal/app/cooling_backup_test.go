package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func brokenFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, coolingBrokenPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEverySaveLeavesAVerifiedSafetyCopy(t *testing.T) {
	e := setupEngine(t, "")
	dir := e.Store.Dir
	if _, err := os.Stat(filepath.Join(dir, coolingBackupFile)); err == nil {
		t.Fatal("a copy without a save")
	}
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	main, bak := readFile(t, filepath.Join(dir, coolingFile)), readFile(t, filepath.Join(dir, coolingBackupFile))
	if main != bak {
		t.Fatal("the copy differs from the file")
	}
	if _, err := parseCoolingConfig([]byte(bak)); err != nil {
		t.Fatalf("the copy does not parse: %v", err)
	}
	// A second save replaces the copy; no temporary files stay behind.
	changed := strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":85`, 1)
	if err := e.SetCoolingConfigJSON([]byte(changed)); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, coolingBackupFile)); got != readFile(t, filepath.Join(dir, coolingFile)) || !strings.Contains(got, `"safe_duty": 85`) {
		t.Fatalf("the copy is not the last save:\n%s", got)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(left) != 0 || len(brokenFiles(t, dir)) != 0 {
		t.Fatalf("leftovers %v %v", left, brokenFiles(t, dir))
	}
}

func TestAFailedSaveLeavesBothFilesAlone(t *testing.T) {
	e := setupEngine(t, coolingExample)
	dir := e.Store.Dir
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	before, beforeBak := readFile(t, filepath.Join(dir, coolingFile)), readFile(t, filepath.Join(dir, coolingBackupFile))
	for _, bad := range []string{`{`, strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":0`, 1)} {
		if err := e.SetCoolingConfigJSON([]byte(bad)); err == nil {
			t.Fatal("accepted")
		}
	}
	if readFile(t, filepath.Join(dir, coolingFile)) != before || readFile(t, filepath.Join(dir, coolingBackupFile)) != beforeBak {
		t.Fatal("a refused save changed a file")
	}
}

func TestADamagedFileIsReplacedByTheCopyAndKeptUnderItsOwnName(t *testing.T) {
	for name, damaged := range map[string]string{
		"cut off":       coolingExample[:len(coolingExample)/2],
		"empty":         "",
		"garbage":       "\x00\x01 not json",
		"unknown field": `{"outputs":[],"nope":1}`,
		"invalid":       strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":0`, 1),
	} {
		e := setupEngine(t, "")
		dir := e.Store.Dir
		if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		good := readFile(t, filepath.Join(dir, coolingFile))
		e.stopCoolingModule()

		// The file breaks on disk while the program is not running.
		if err := os.WriteFile(filepath.Join(dir, coolingFile), []byte(damaged), 0o600); err != nil {
			t.Fatal(err)
		}
		e2 := coolingEngine(t, "", &coolingReadings{})
		e2.Store = &Store{Dir: dir}
		e2.Demo = true
		e2.ctx = e.ctx
		e2.startCooling()

		v := e2.CoolingView()
		if v.Error != "" || !v.Configured || !v.Restored {
			t.Fatalf("%s: %+v", name, v)
		}
		if v.Alarm == nil || v.Alarm.Kind != coolingAlarmConfigCopy || !strings.HasPrefix(v.Alarm.Name, coolingBrokenPrefix) {
			t.Fatalf("%s: alarm %+v", name, v.Alarm)
		}
		kept := brokenFiles(t, dir)
		if len(kept) != 1 || readFile(t, kept[0]) != damaged {
			t.Fatalf("%s: the damaged file was not kept as it was: %v", name, kept)
		}
		if filepath.Base(kept[0]) != v.RestoredFile {
			t.Fatalf("%s: alarm names %q, kept %q", name, v.RestoredFile, kept[0])
		}
		if readFile(t, filepath.Join(dir, coolingFile)) != good {
			t.Fatalf("%s: cooling.json is not the safety copy", name)
		}
		if got := e2.cooling.cfg; len(got.Outputs) != 2 {
			t.Fatalf("%s: running on %+v", name, got)
		}
		if a := e2.currentCoolingAlarm(); a == nil || a.Kind != coolingAlarmConfigCopy {
			t.Fatalf("%s: current alarm %+v", name, a)
		}
		e2.stopCoolingModule()
	}
}

func TestSavingAgainEndsTheAlarmAndKeepsTheDamagedFile(t *testing.T) {
	e := setupEngine(t, "")
	dir := e.Store.Dir
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	e.stopCoolingModule()
	os.WriteFile(filepath.Join(dir, coolingFile), []byte("{"), 0o600)
	e.crash.clearRestored()
	e.startCooling()
	if !e.CoolingView().Restored {
		t.Fatal("not restored")
	}
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	if v := e.CoolingView(); v.Restored || v.Alarm != nil {
		t.Fatalf("%+v", v)
	}
	if len(brokenFiles(t, dir)) != 1 {
		t.Fatalf("%v", brokenFiles(t, dir))
	}
}

// Without a usable copy the behaviour stays as it was: the error is shown, and the
// damaged file is not touched.
func TestWithoutAUsableCopyTheFileStaysAndTheErrorIsShown(t *testing.T) {
	for name, bak := range map[string]*string{
		"no copy":        nil,
		"copy damaged":   ptr(coolingExample[:30]),
		"copy invalid":   ptr(strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":0`, 1)),
		"copy not valid": ptr(`{"outputs":[],"nope":1}`),
	} {
		e := coolingEngine(t, "{", &coolingReadings{})
		dir := e.Store.Dir
		if bak != nil {
			os.WriteFile(filepath.Join(dir, coolingBackupFile), []byte(*bak), 0o600)
		}
		e.startCooling()
		v := e.CoolingView()
		if v.Error == "" || v.Restored || v.Alarm == nil || v.Alarm.Kind != coolingAlarmConfig {
			t.Fatalf("%s: %+v", name, v)
		}
		if readFile(t, filepath.Join(dir, coolingFile)) != "{" || len(brokenFiles(t, dir)) != 0 {
			t.Fatalf("%s: the file was touched", name)
		}
		if bak != nil && readFile(t, filepath.Join(dir, coolingBackupFile)) != *bak {
			t.Fatalf("%s: the copy was touched", name)
		}
	}
}

// Saving over a damaged file with no copy to restore from must keep what was there.
func TestSavingOverADamagedFileKeepsItFirst(t *testing.T) {
	e := setupEngine(t, "")
	dir := e.Store.Dir
	damaged := `{"curves": [ typo`
	os.WriteFile(filepath.Join(dir, coolingFile), []byte(damaged), 0o600)
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	kept := brokenFiles(t, dir)
	if len(kept) != 1 || readFile(t, kept[0]) != damaged {
		t.Fatalf("not kept: %v", kept)
	}
	if strings.Contains(readFile(t, filepath.Join(dir, coolingFile)), "typo") {
		t.Fatal("still the damaged file")
	}
	// Saving over a valid file keeps nothing.
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err != nil {
		t.Fatal(err)
	}
	if len(brokenFiles(t, dir)) != 1 {
		t.Fatalf("%v", brokenFiles(t, dir))
	}
}

// If the damaged file cannot be kept, it is not overwritten.
func TestNothingIsOverwrittenIfTheDamagedFileCannotBeKept(t *testing.T) {
	e := setupEngine(t, "")
	dir := e.Store.Dir
	os.WriteFile(filepath.Join(dir, coolingFile), []byte("{"), 0o600)
	e.Store.Dir = filepath.Join(dir, "gone")
	if _, err := e.Store.keepBrokenCooling(); err != nil {
		t.Fatalf("a missing file is nothing to keep: %v", err)
	}
	e.Store.Dir = dir
	// A directory where the file should be cannot be read, so it cannot be kept.
	os.Remove(filepath.Join(dir, coolingFile))
	os.Mkdir(filepath.Join(dir, coolingFile), 0o700)
	if err := e.SetCoolingConfigJSON([]byte(coolingExample)); err == nil {
		t.Fatal("overwrote something that could not be kept")
	}
	if st, err := os.Stat(filepath.Join(dir, coolingFile)); err != nil || !st.IsDir() {
		t.Fatal("the file was replaced")
	}
}

// A file saved before the copies existed gets its first copy when it loads fine.
func TestAnOldFileGetsItsFirstCopy(t *testing.T) {
	e := setupEngine(t, coolingExample)
	dir := e.Store.Dir
	if got := readFile(t, filepath.Join(dir, coolingBackupFile)); got != coolingExample {
		t.Fatalf("copy %q", got)
	}
	// An existing copy stays as it is: it may be the only good one.
	os.WriteFile(filepath.Join(dir, coolingBackupFile), []byte(coolingExample+"\n"), 0o600)
	e.stopCoolingModule()
	e.startCooling()
	if got := readFile(t, filepath.Join(dir, coolingBackupFile)); got != coolingExample+"\n" {
		t.Fatal("the copy was replaced")
	}
}

func TestTwoDamagedFilesInOneSecondGetTwoNames(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	a, err := s.keepCopy([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.keepCopy([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b || readFile(t, s.path(a)) != "a" || readFile(t, s.path(b)) != "b" {
		t.Fatalf("%s %s", a, b)
	}
}
