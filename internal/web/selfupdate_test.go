package web

import (
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/update"
)

func TestV1UpdateEndpoints(t *testing.T) {
	s, ts := testServer(t)
	var st UpdateStatus
	wantStatus(t, v1(t, ts, "GET", "/api/v1/update", "", &st), 200)
	if st.Current != "test" || st.State != "idle" || st.Installable || st.Available != nil || st.Total != 0 {
		t.Fatalf("status %+v", st)
	}
	// A test binary is no setup installation.
	wantError(t, v1(t, ts, "POST", "/api/v1/update/install", "", nil), 409, "conflict")
	wantError(t, v1(t, ts, "POST", "/api/v1/update/install", `{"relaunch":"cmd.exe"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/update/install", `{"relaunch":"tray","x":1}`, nil), 400, "bad_request")
	// Only a CaseHUD without tray icon may come back hidden.
	wantError(t, v1(t, ts, "POST", "/api/v1/update/install", `{"relaunch":"hidden"}`, nil), 400, "bad_request")
	s.NoTray = true
	wantError(t, v1(t, ts, "POST", "/api/v1/update/install", `{"relaunch":"hidden"}`, nil), 409, "conflict")
	s.NoTray = false

	// The check obeys the setting (and never goes online in this test).
	if err := s.Engine.SetUpdateCheck(false); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1(t, ts, "POST", "/api/v1/update/check", "", nil), 409, "conflict")
	wantError(t, v1(t, ts, "GET", "/api/v1/update/install", "", nil), 405, "method_not_allowed")
}

func TestStartSelfUpdateRules(t *testing.T) {
	s, _ := testServer(t)
	s.Version = "1.0.2"
	s.selfUpd.instOnce.Do(func() { s.selfUpd.inst = true })
	if err := s.startSelfUpdate(RelaunchTray); err != errNoUpdate {
		t.Fatalf("without a known update: %v", err)
	}
	for _, v := range []string{"1.0.1", "1.0.2", "1.0.3-beta", "1.0.3/x"} {
		s.update = &update.Release{Version: v}
		if err := s.startSelfUpdate(RelaunchTray); err != errNoUpdate {
			t.Errorf("version %s: %v", v, err)
		}
	}
	s.update = &update.Release{Version: "1.0.3"}
	s.selfUpd.phase = updStarting
	if err := s.startSelfUpdate(RelaunchTray); err != errUpdateBusy {
		t.Fatalf("while running: %v", err)
	}
	s.selfUpd.phase = updError
	s.selfUpd.lastStart = time.Now()
	if err := s.startSelfUpdate(RelaunchTray); err != errUpdateTooSoon {
		t.Fatalf("right after a start: %v", err)
	}
	// Not installed (portable) always wins.
	s2, _ := testServer(t)
	s2.Version = "1.0.2"
	s2.update = &update.Release{Version: "1.0.3"}
	if err := s2.startSelfUpdate(RelaunchTray); err != errNotInstallable {
		t.Fatalf("portable: %v", err)
	}
}
