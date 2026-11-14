package coolguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExe(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTheGuardRunsFromACopyUnderAnotherName(t *testing.T) {
	src := writeExe(t, t.TempDir(), "CaseHUD.exe", "program one")
	data := t.TempDir()
	got, err := GuardExe(data, src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) == "CaseHUD.exe" || !strings.HasPrefix(filepath.Base(got), copyPrefix) || filepath.Ext(got) != ".exe" {
		t.Fatalf("name %s", got)
	}
	if b, _ := os.ReadFile(got); string(b) != "program one" {
		t.Fatalf("content %q", b)
	}
	if b, _ := os.ReadFile(src); string(b) != "program one" {
		t.Fatal("the program itself was touched")
	}
	// The same version is not copied again.
	st1, _ := os.Stat(got)
	again, err := GuardExe(data, src)
	if err != nil || again != got {
		t.Fatalf("%s %v", again, err)
	}
	if st2, _ := os.Stat(got); !st2.ModTime().Equal(st1.ModTime()) {
		t.Fatal("copied again")
	}
}

func TestANewVersionGetsANewCopyAndTheOldOneGoes(t *testing.T) {
	dir := t.TempDir()
	src := writeExe(t, dir, "CaseHUD.exe", "program one")
	data := t.TempDir()
	first, err := GuardExe(data, src)
	if err != nil {
		t.Fatal(err)
	}
	writeExe(t, dir, "CaseHUD.exe", "program two, larger")
	second, err := GuardExe(data, src)
	if err != nil || second == first {
		t.Fatalf("%s %s %v", first, second, err)
	}
	if _, err := os.Stat(first); err == nil {
		t.Fatal("the old copy stayed")
	}
	left, _ := filepath.Glob(filepath.Join(data, copyDir, "*"))
	if len(left) != 1 {
		t.Fatalf("%v", left)
	}
}

func TestACopyThatWasChangedIsReplaced(t *testing.T) {
	src := writeExe(t, t.TempDir(), "CaseHUD.exe", "program one")
	data := t.TempDir()
	got, _ := GuardExe(data, src)
	os.WriteFile(got, []byte("program 1!!"), 0o755) // same size, other content
	again, err := GuardExe(data, src)
	if err != nil || again != got {
		t.Fatalf("%s %v", again, err)
	}
	if b, _ := os.ReadFile(again); string(b) != "program one" {
		t.Fatalf("content %q", b)
	}
}

func TestWithoutACopyThereIsAnError(t *testing.T) {
	data := t.TempDir()
	if _, err := GuardExe(data, filepath.Join(data, "missing.exe")); err == nil {
		t.Fatal("no error for a missing program")
	}
	// A file where the folder should be.
	src := writeExe(t, t.TempDir(), "CaseHUD.exe", "x")
	blocked := writeExe(t, t.TempDir(), "data", "i am a file")
	if _, err := GuardExe(blocked, src); err == nil {
		t.Fatal("no error without a folder")
	}
}

func TestTheInstalledGuardIsUsedWhenItIsTheSameProgram(t *testing.T) {
	dir := t.TempDir()
	exe := writeExe(t, dir, "CaseHUD.exe", "program one")
	if _, ok := InstalledGuard(exe); ok {
		t.Fatal("found a guard that is not there")
	}
	want := writeExe(t, dir, InstalledName+".exe", "program one")
	got, ok := InstalledGuard(exe)
	if !ok || got != want {
		t.Fatalf("%q %v", got, ok)
	}
	// A guard of another version, even of the same size, is not used.
	writeExe(t, dir, InstalledName+".exe", "program 1!!")
	if _, ok := InstalledGuard(exe); ok {
		t.Fatal("used a guard of another program")
	}
	writeExe(t, dir, InstalledName+".exe", "program one, larger")
	if _, ok := InstalledGuard(exe); ok {
		t.Fatal("used a guard of another size")
	}
	// The program itself is never its own guard.
	if _, ok := InstalledGuard(filepath.Join(dir, InstalledName+".exe")); ok {
		t.Fatal("a guard for the guard")
	}
}

func TestOldCopiesGoOnceTheInstalledGuardIsUsed(t *testing.T) {
	src := writeExe(t, t.TempDir(), "CaseHUD.exe", "program one")
	data := t.TempDir()
	if _, err := GuardExe(data, src); err != nil {
		t.Fatal(err)
	}
	RemoveCopies(data)
	if left, _ := filepath.Glob(filepath.Join(data, copyDir, "*")); len(left) != 0 {
		t.Fatalf("%v", left)
	}
}
