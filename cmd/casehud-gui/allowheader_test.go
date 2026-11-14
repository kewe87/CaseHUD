package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hdrKey = "/lpc/nct6701d/0/control/6"

func readSettings(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "sensors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func headersOf(t *testing.T, m map[string]json.RawMessage) []string {
	t.Helper()
	var l []string
	if err := json.Unmarshal(m["coolingHeaders"], &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func onlyFile(t *testing.T, dir string) {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if e.Name() != "sensors.json" {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}

func TestAllowCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := allowCoolingHeader(dir, hdrKey, nil); err != nil {
		t.Fatal(err)
	}
	if l := headersOf(t, readSettings(t, dir)); len(l) != 1 || l[0] != hdrKey {
		t.Fatalf("%v", l)
	}
	onlyFile(t, dir)
}

func TestAllowKeepsOtherKeysAndIsRepeatable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sensors.json")
	os.WriteFile(p, []byte(`{"smbusBridge": true, "other": {"a": [1,2]}, "coolingHeaders": ["/lpc/nct6701d/0/control/2"]}`), 0o600)
	for range 2 {
		if err := allowCoolingHeader(dir, hdrKey, nil); err != nil {
			t.Fatal(err)
		}
	}
	m := readSettings(t, dir)
	if string(m["smbusBridge"]) != "true" || !strings.Contains(string(m["other"]), `"a"`) {
		t.Fatalf("lost a key: %s", m)
	}
	if l := headersOf(t, m); len(l) != 2 || l[0] != "/lpc/nct6701d/0/control/2" || l[1] != hdrKey {
		t.Fatalf("%v", l)
	}
	onlyFile(t, dir)
}

func TestAllowNeverReplacesWhatItCannotRead(t *testing.T) {
	for name, content := range map[string]string{
		"broken":      `{"smbusBridge": tru`,
		"empty":       ``,
		"null":        `null`,
		"array":       `[1]`,
		"not a list":  `{"coolingHeaders": "x"}`,
		"wrong items": `{"coolingHeaders": [1]}`,
		"too large":   `{"x":"` + strings.Repeat("a", 70000) + `"}`,
	} {
		dir := t.TempDir()
		p := filepath.Join(dir, "sensors.json")
		os.WriteFile(p, []byte(content), 0o600)
		if err := allowCoolingHeader(dir, hdrKey, nil); err == nil {
			t.Errorf("%s: no error", name)
		}
		if b, _ := os.ReadFile(p); string(b) != content {
			t.Errorf("%s: file changed", name)
		}
		onlyFile(t, dir)
	}
}

func TestAllowRefusesBadKeys(t *testing.T) {
	for _, k := range []string{"", "/gpu-nvidia/0/control/0", hdrKey + "\n", "../x"} {
		dir := t.TempDir()
		if err := allowCoolingHeader(dir, k, nil); err == nil {
			t.Errorf("%q accepted", k)
		}
		if _, err := os.Stat(filepath.Join(dir, "sensors.json")); err == nil {
			t.Errorf("%q wrote a file", k)
		}
	}
}

func TestAllowNeedsAnExistingOrdinaryFolder(t *testing.T) {
	root := t.TempDir()
	if err := allowCoolingHeader(filepath.Join(root, "missing"), hdrKey, nil); err == nil {
		t.Error("missing folder accepted")
	}
	file := filepath.Join(root, "afile")
	os.WriteFile(file, nil, 0o600)
	if err := allowCoolingHeader(file, hdrKey, nil); err == nil {
		t.Error("a file as folder accepted")
	}
	real := filepath.Join(root, "real")
	os.Mkdir(real, 0o700)
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := allowCoolingHeader(link, hdrKey, nil); err == nil {
		t.Error("linked folder accepted")
	}
}

func TestAllowRefusesALinkedOrIrregularFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sensors.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := allowCoolingHeader(dir, hdrKey, nil); err == nil {
		t.Error("a folder as file accepted")
	}
	dir = t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	os.WriteFile(target, []byte(`{}`), 0o600)
	if err := os.Symlink(target, filepath.Join(dir, "sensors.json")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := allowCoolingHeader(dir, hdrKey, nil); err == nil {
		t.Error("a linked file accepted")
	}
	if b, _ := os.ReadFile(target); string(b) != `{}` {
		t.Error("the link target changed")
	}
}

func TestAllowWritesNothingWhenTheOwnerCannotBeSet(t *testing.T) {
	dir := t.TempDir()
	err := allowCoolingHeader(dir, hdrKey, func(string) error { return errors.New("no") })
	if err == nil {
		t.Fatal("no error")
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 0 {
		t.Fatalf("left behind: %v", ents)
	}
}

func TestAllowHandsTheTempFileToTheOwnerStep(t *testing.T) {
	dir := t.TempDir()
	var seen string
	err := allowCoolingHeader(dir, hdrKey, func(p string) error { seen = p; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(seen) != dir || filepath.Base(seen) == "sensors.json" {
		t.Fatalf("owner step got %q", seen)
	}
}

func TestTheRenameWaitsForAFileThatIsOpenForAMoment(t *testing.T) {
	busy := errors.New("in use")
	retry := func(err error) bool { return errors.Is(err, busy) }
	calls := 0
	rename := func(a, b string) error {
		calls++
		if calls < 4 {
			return &os.LinkError{Op: "rename", Old: a, New: b, Err: busy}
		}
		return nil
	}
	if err := renameRetrying("a", "b", rename, retry, 10, time.Millisecond); err != nil || calls != 4 {
		t.Fatalf("%v after %d calls", err, calls)
	}
	// It does not wait for ever.
	calls = 0
	always := func(a, b string) error { calls++; return busy }
	err := renameRetrying("a", "b", always, retry, 5, time.Millisecond)
	if !errors.Is(err, busy) || calls != 5 {
		t.Fatalf("%v after %d calls", err, calls)
	}
	// And not for another kind of failure.
	calls = 0
	other := errors.New("no such folder")
	if err := renameRetrying("a", "b", func(a, b string) error { calls++; return other }, retry, 5, time.Millisecond); !errors.Is(err, other) || calls != 1 {
		t.Fatalf("%v after %d calls", err, calls)
	}
}
