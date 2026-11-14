package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func startSetup(t *testing.T, dir string, demo, demoSetup bool) *Engine {
	t.Helper()
	e := &Engine{Store: &Store{Dir: dir}, Demo: demo, DemoSetup: demoSetup}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	return e
}

func TestSetupDueOnlyForNewConfiguration(t *testing.T) {
	dir := t.TempDir()
	e := startSetup(t, dir, true, true)
	if !e.SetupPending() {
		t.Fatal("new configuration: setup not due")
	}
	// Saved without an answer (for example a started and closed app): still due.
	if err := e.persist(); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	e = startSetup(t, dir, true, true)
	if !e.SetupPending() {
		t.Fatal("unanswered setup lost after a restart")
	}
}

func TestSetupNotDueForExistingConfiguration(t *testing.T) {
	// A configuration written before the setup existed has no marker.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"language":"en","panels":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e := startSetup(t, dir, true, true)
	if e.SetupPending() {
		t.Fatal("existing installation would be asked")
	}
	if err := e.persist(); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	if e = startSetup(t, dir, true, true); e.SetupPending() {
		t.Fatal("asked after a save")
	}
	// An unreadable file counts as existing, too.
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "config.json"), []byte(`{"lang`), 0o600); err != nil {
		t.Fatal(err)
	}
	if e = startSetup(t, bad, true, true); e.SetupPending() {
		t.Fatal("asked after a damaged file")
	}
}

func TestSetupHiddenInDemoUnlessAsked(t *testing.T) {
	if e := startSetup(t, t.TempDir(), true, false); e.SetupPending() {
		t.Fatal("demo shows the setup by itself")
	}
}

func TestSetupAppliesAreasLikeTheSettings(t *testing.T) {
	dir := t.TempDir()
	e := startSetup(t, dir, true, true)
	if err := e.CompleteSetup(&Modules{}); !errors.Is(err, ErrNoModule) {
		t.Fatalf("all off: %v", err)
	}
	if !e.SetupPending() || e.Modules() != (Modules{true, true, true}) {
		t.Fatalf("refused request changed something: pending %v, %+v", e.SetupPending(), e.Modules())
	}
	if err := e.CompleteSetup(&Modules{Display: true, Lighting: true}); err != nil {
		t.Fatal(err)
	}
	if e.SetupPending() || e.Modules() != (Modules{Display: true, Lighting: true}) {
		t.Fatalf("pending %v, %+v", e.SetupPending(), e.Modules())
	}
	// Cooling was never configured: switching it off writes and stops nothing.
	if _, err := os.Stat(filepath.Join(dir, coolingFile)); !os.IsNotExist(err) {
		t.Fatalf("cooling file created: %v", err)
	}
	if err := e.CompleteSetup(nil); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second answer: %v", err)
	}
	e.Stop()
	e = startSetup(t, dir, true, true)
	if e.SetupPending() || e.Modules() != (Modules{Display: true, Lighting: true}) {
		t.Fatalf("after restart: pending %v, %+v", e.SetupPending(), e.Modules())
	}
}

func TestSetupClosedKeepsAreasAndDoesNotAskAgain(t *testing.T) {
	dir := t.TempDir()
	e := startSetup(t, dir, true, true)
	if err := e.CompleteSetup(nil); err != nil {
		t.Fatal(err)
	}
	if e.Modules() != (Modules{true, true, true}) {
		t.Fatalf("%+v", e.Modules())
	}
	e.Stop()
	if e = startSetup(t, dir, true, true); e.SetupPending() {
		t.Fatal("asked again")
	}
}
