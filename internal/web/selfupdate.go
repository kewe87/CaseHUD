package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/autostart"
	"github.com/kewe87/casehud/internal/update"
)

// Self-update: on request (button in the window or POST /api/v1/update/install)
// CaseHUD downloads the setup of the newer release, verifies it against the
// signed SHA256SUMS.txt and starts it. The setup asks for administrator rights
// (UAC), closes CaseHUD cleanly and starts it again afterwards.

// Phases of the self-update.
const (
	updIdle        = "idle"
	updDownloading = "downloading"
	updStarting    = "starting" // setup started, waiting for UAC and the install
	updError       = "error"
)

// How CaseHUD comes back after the update.
const (
	RelaunchWindow = "window" // with its window
	RelaunchTray   = "tray"   // in the tray (--minimized)
	RelaunchHidden = "hidden" // without window and tray icon (--background --no-tray)
)

// Limits: one download at a time, and a new attempt (another UAC prompt) at
// most every two minutes, whoever asks.
const (
	updRetryGap   = 2 * time.Minute
	updCheckGap   = time.Minute
	updDownloadTO = 30 * time.Minute
)

type selfUpdate struct {
	mu        sync.Mutex
	phase     string
	version   string
	done      int64
	total     int64
	err       string
	lastStart time.Time
	lastCheck time.Time

	instOnce sync.Once
	inst     bool
}

// UpdateStatus is the self-update state for the window and API v1.
type UpdateStatus struct {
	Current     string          `json:"current"`
	Available   *update.Release `json:"available"`   // null while none is known
	Installable bool            `json:"installable"` // installed with the setup (not portable, not a dev build)
	State       string          `json:"state"`       // idle | downloading | starting | error
	Version     string          `json:"version,omitempty"`
	Done        int64           `json:"done"`
	Total       int64           `json:"total"` // -1 while unknown
	Error       string          `json:"error,omitempty"`
}

var (
	errNoUpdate       = errors.New("no newer version is known; check for updates first")
	errNotInstallable = errors.New("this copy cannot update itself (portable or development build); download the setup from the releases page")
	errUpdateBusy     = errors.New("an update is already running")
	errUpdateTooSoon  = errors.New("an update was started less than two minutes ago")
)

// installable reports whether this copy was installed by the setup
// (looked up once; the state is polled often).
func (s *Server) installable() bool {
	s.selfUpd.instOnce.Do(func() {
		if update.IsDev(s.Version) {
			return
		}
		exe, err := os.Executable()
		if err != nil {
			return
		}
		if p, err := filepath.EvalSymlinks(exe); err == nil {
			exe = p
		}
		s.selfUpd.inst = update.Installed(exe)
	})
	return s.selfUpd.inst
}

func (s *Server) updateStatus() UpdateStatus {
	st := UpdateStatus{Current: s.Version, Available: s.availableUpdate(), Installable: s.installable()}
	u := &s.selfUpd
	u.mu.Lock()
	defer u.mu.Unlock()
	st.State, st.Version, st.Done, st.Total, st.Error = u.phase, u.version, u.done, u.total, u.err
	if st.State == "" {
		st.State = updIdle
	}
	return st
}

// updatesDir holds downloaded setups (per user, emptied before each download
// and at start).
func updatesDir() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "CaseHUD", "updates"), nil
}

// CleanUpdates removes setups left from earlier updates.
func CleanUpdates() {
	if d, err := updatesDir(); err == nil {
		os.RemoveAll(d)
	}
}

// startSelfUpdate begins downloading and installing the known newer release.
// relaunch says how CaseHUD starts again afterwards.
func (s *Server) startSelfUpdate(relaunch string) error {
	if !s.installable() {
		return errNotInstallable
	}
	rel := s.availableUpdate()
	if rel == nil || !update.ValidVersion(rel.Version) || !update.Newer(rel.Version, s.Version) {
		return errNoUpdate
	}
	u := &s.selfUpd
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.phase == updDownloading || u.phase == updStarting {
		return errUpdateBusy
	}
	if !u.lastStart.IsZero() && time.Since(u.lastStart) < updRetryGap {
		return errUpdateTooSoon
	}
	u.lastStart = time.Now()
	u.phase, u.version, u.done, u.total, u.err = updDownloading, rel.Version, 0, -1, ""
	go s.runSelfUpdate(rel.Version, relaunch)
	return nil
}

func (s *Server) runSelfUpdate(version, relaunch string) {
	u := &s.selfUpd
	fail := func(err error) {
		log.Println("update:", err)
		u.mu.Lock()
		u.phase, u.err = updError, err.Error()
		u.mu.Unlock()
	}
	root, err := updatesDir()
	if err != nil {
		fail(err)
		return
	}
	os.RemoveAll(root)
	if err := os.MkdirAll(root, 0o700); err != nil {
		fail(err)
		return
	}
	dir, err := os.MkdirTemp(root, "v"+version+"-")
	if err != nil {
		fail(err)
		return
	}
	log.Printf("update: downloading %s", version)
	ctx, cancel := context.WithTimeout(context.Background(), updDownloadTO)
	path, want, err := update.Download(ctx, version, dir, func(done, total int64) {
		u.mu.Lock()
		u.done, u.total = done, total
		u.mu.Unlock()
	})
	cancel()
	if err != nil {
		fail(err)
		return
	}
	u.mu.Lock()
	u.phase = updStarting
	u.mu.Unlock()
	tasks := "!autostart"
	if autostart.Enabled() {
		tasks = "autostart"
	}
	args := []string{"/SILENT", "/SUPPRESSMSGBOXES", "/NORESTART",
		"/RELAUNCH=" + relaunch, "/MERGETASKS=" + tasks}
	log.Printf("update: signature and checksum OK, starting the setup %q %q", filepath.Base(path), args)
	wait, err := update.Start(path, want, args)
	if err != nil {
		fail(err)
		return
	}
	// On success the setup closes CaseHUD before it ends, so this only
	// returns when the setup gave up (UAC declined, cancelled, failed).
	code, err := wait()
	if err == nil && code != 0 {
		err = fmt.Errorf("the setup ended with code %d (administrator rights declined or setup cancelled?)", code)
	}
	if err != nil {
		fail(err)
		return
	}
	u.mu.Lock()
	u.phase = updIdle
	u.mu.Unlock()
}

// checkUpdateNow runs a release check on request, at most once a minute.
func (s *Server) checkUpdateNow() {
	u := &s.selfUpd
	u.mu.Lock()
	recent := !u.lastCheck.IsZero() && time.Since(u.lastCheck) < updCheckGap
	if !recent {
		u.lastCheck = time.Now()
	}
	u.mu.Unlock()
	if !recent {
		s.checkUpdate()
	}
}

func validRelaunch(v string) bool {
	return v == RelaunchWindow || v == RelaunchTray || v == RelaunchHidden
}

// --- API v1 ---

func (s *Server) v1Update(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.updateStatus())
}

func (s *Server) v1UpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.Engine.UpdateCheck() {
		apiFail(w, http.StatusConflict, "conflict", "the update check is switched off in CaseHUD's settings")
		return
	}
	s.checkUpdateNow()
	writeJSON(w, s.updateStatus())
}

func (s *Server) v1UpdateInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Relaunch string `json:"relaunch"`
	}
	ok, empty := decodeBodyOpt(w, r, &req)
	if !ok {
		return
	}
	if empty || req.Relaunch == "" {
		req.Relaunch = RelaunchTray
		if s.NoTray {
			req.Relaunch = RelaunchHidden
		}
	}
	if !validRelaunch(req.Relaunch) {
		badRequest(w, "relaunch must be window, tray or hidden")
		return
	}
	// Nobody may make a CaseHUD with a tray icon come back invisible.
	if req.Relaunch == RelaunchHidden && !s.NoTray {
		badRequest(w, "relaunch hidden is only possible when CaseHUD runs with --no-tray")
		return
	}
	if err := s.startSelfUpdate(req.Relaunch); err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(s.updateStatus())
}
