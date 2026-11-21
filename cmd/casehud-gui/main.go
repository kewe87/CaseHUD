// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

// Command casehud-gui is the desktop app: it streams to all supported displays in
// the background and offers a GUI (tray icon + window on Windows, browser
// elsewhere) to pick designs, images, GIFs and videos per screen.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/coolguard"
	"github.com/kewe87/casehud/internal/web"
)

// stateDir is the data folder; the Windows GUI keeps its window geometry there.
var stateDir string

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	demo := flag.Bool("demo", false, "simulate LCDs and sensors (no hardware needed)")
	demoSetup := flag.Bool("demo-setup", false, "with --demo: show the first-run setup dialog")
	minimized := flag.Bool("minimized", false, "start in the tray without opening the window")
	background := flag.Bool("background", false, "start in the tray; if CaseHUD already runs, exit silently without showing its window")
	noTray := flag.Bool("no-tray", false, "no tray icon while there is no taskbar (added when the taskbar is created later); the window opens through the API or a second start")
	port := flag.Int("port", 0, "GUI port on 127.0.0.1 (0 = random)")
	dataDir := flag.String("data", app.DefaultDir(), "data folder (config, designs, media)")
	setAPI := flag.String("set-api", "", "switch the API for other programs on or off in the settings, then exit (used by the installer)")
	allowHeader := flag.String("allow-cooling-header", "", "release one mainboard fan header for CaseHUD in the service settings, then exit (needs administrator rights; started by CaseHUD itself)")
	quit := flag.Bool("quit", false, "ask a running CaseHUD to quit cleanly, then exit (used by the installer)")
	guard := flag.Int("cooling-guard", 0, "run as the guard of the CaseHUD process with this id: set the fans to a safe speed if it ends without notice (started by CaseHUD itself)")
	flag.Parse()
	if *quit {
		quitRunning(*dataDir)
		return
	}
	if v, given, err := parseOnOff(*setAPI); err != nil {
		quitErr(err)
	} else if given {
		// No message box: the installer runs this hidden and must not hang.
		if err := refuseService(processSID); err != nil {
			quitErr(err)
		}
		if err := os.MkdirAll(*dataDir, 0o700); err != nil {
			quitErr(err)
		}
		if err := setAPISetting(*dataDir, v); err != nil {
			quitErr(err)
		}
		return
	}
	if *allowHeader != "" {
		// Started elevated by CaseHUD after the user asked for it, so a failure is shown.
		dir, err := sensorsDir()
		if err == nil {
			err = allowCoolingHeader(dir, *allowHeader, makeAdminOwner)
		}
		if err != nil {
			showError(err.Error())
			os.Exit(1)
		}
		return
	}
	if *guard > 0 {
		os.Exit(runCoolingGuard(*guard, *dataDir))
	}
	dataGiven := false
	flag.Visit(func(f *flag.Flag) { dataGiven = dataGiven || f.Name == "data" })

	store := &app.Store{Dir: *dataDir}
	stateDir = store.Dir
	if err := os.MkdirAll(store.Dir, 0o700); err != nil {
		fatal(err)
	}
	instFile := filepath.Join(store.Dir, "instance.url")
	// Single instance, first of all: nothing that touches devices, the log or
	// the markers may run before this start owns the lock. A demo is exempt
	// (test runs start several).
	if !*demo {
		quietStart := *background || *minimized
		res := acquireInstance(tryHold, func() bool {
			b, err := os.ReadFile(instFile)
			if err != nil {
				return false
			}
			running := strings.TrimSpace(string(b))
			// A start meant for the tray (autostart) never brings the window up.
			if quietStart {
				return instanceRunning(running)
			}
			return showExisting(running, store.Dir)
		}, func() bool { return lastHoldFailure() != "" }, lockWait, lockPoll)
		switch res {
		case lockHandedOver:
			return
		case lockTimeout:
			failure := lastHoldFailure()
			if f, err := os.OpenFile(filepath.Join(store.Dir, "casehud-start.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
				what := "the lock " + runningMutexName + " is held and nobody answered within " + lockWait.String()
				if failure != "" {
					what = "the lock " + runningMutexName + " could not be created (" + failure + ")"
				}
				fmt.Fprintf(f, "%s CaseHUD %s: %s; this start (pid %d) ended\n", time.Now().Format(time.RFC3339), version, what, os.Getpid())
				f.Close()
			}
			// A start meant for the tray (autostart, --minimized) only logs: a
			// dialog nobody asked for at logon helps no one.
			if !quietStart {
				showError(lockTimeoutText(uiGerman(), failure))
			}
			os.Exit(1)
		}
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "config.json")); err == nil {
		_ = web.RestrictFile(filepath.Join(store.Dir, "config.json"))
	}

	// Opened only now: a second start that hands over to the running
	// instance must not empty that instance's log.
	logFile, err := os.OpenFile(filepath.Join(store.Dir, "casehud.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.Printf("CaseHUD %s, started with %q", version, os.Args[1:])

	engine := &app.Engine{Store: store, Demo: *demo, DemoSetup: *demoSetup, OnCoolingAlarm: showCoolingAlarm}
	if !*demo {
		engine.GuardSpawn = func() (coolguard.Child, error) { return coolguard.Spawn(store.Dir) }
	}
	if err := engine.Start(context.Background()); err != nil {
		fatal(err)
	}
	srv := &web.Server{Engine: engine, Version: version, NoTray: *noTray}
	// The markers (instance.url/pid, api.json) stay until the engine has
	// released the devices, so a second start cannot fight over them.
	var markers []func()
	var stopOnce sync.Once
	stopEngine := func() { stopOnce.Do(engine.Stop) }
	defer func() {
		// From here on this instance no longer takes over a second start: the
		// stop can take seconds, and that start has to wait for the lock instead.
		srv.SetEnding()
		stopEngine()
		srv.FlushAccess()
		for _, rm := range markers {
			rm()
		}
	}()

	if !*demo {
		// Logging off or shutting down: leave the fans safe before the system ends the process.
		watchSessionEnd(func() {
			done := make(chan struct{})
			// Not a planned stop: if the process outlives it, the alarm says so.
			srv.SetEnding()
			go func() { stopOnce.Do(engine.StopUnplanned); close(done) }()
			select {
			case <-done:
			case <-time.After(4 * time.Second):
			}
		})
	}
	guiURL, err := srv.Listen(*port)
	if err != nil {
		fatal(err)
	}
	// A demo writes the instance files only into a folder given with --data,
	// never into the default one of a real installation.
	if !*demo || dataGiven {
		writeRestricted(instFile, guiURL)
		markers = append(markers, func() { removeMarker(instFile, guiURL) })
		// The PID lets a second start give this window the foreground (api.json
		// only exists while the API is on).
		pidFile := filepath.Join(store.Dir, "instance.pid")
		writeRestricted(pidFile, pidString())
		markers = append(markers, func() { removeMarker(pidFile, pidString()) })
	}
	// Other programs find the v1 API through api.json (never log its token),
	// but only while the "API" setting is on (default off; --background and
	// --no-tray do not change that). A demo run never writes into the default
	// data folder, where a real installation's file would live.
	if !*demo || dataGiven {
		srv.APIFileDir = store.Dir
		srv.AccessFile = filepath.Join(store.Dir, "access.json")
		srv.DataDir = store.Dir
		srv.AI = aiEnv(*demo, store.Dir)
		if !engine.APIEnabled() {
			os.Remove(filepath.Join(store.Dir, web.APIFileName)) // left behind by a crash
		}
		markers = append(markers, func() { web.RemoveAPIFile(store.Dir, os.Getpid()) })
	}
	srv.SyncAPI()
	srv.OnQuit = quitApp
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		srv.SetEnding()
		quitApp()
	}()
	if i := strings.Index(guiURL, "?"); i > 0 {
		log.Println("GUI at", guiURL[:i]) // never log the session token
	}

	srv.OnShow = func() error { return showWindowWait(guiURL) }
	go srv.RunUpdateChecks(context.Background())
	go func() {
		if err := srv.Serve(); err != nil {
			fatal(err)
		}
	}()
	runUI(guiURL, !*minimized && !*background, *noTray)
}

// writeRestricted creates path with a protected DACL before the content goes in.
func writeRestricted(path, content string) {
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		f.Close()
	}
	if err := web.RestrictFile(path); err != nil {
		log.Println("restrict", filepath.Base(path)+":", err)
	}
	_ = os.WriteFile(path, []byte(content), 0o600)
}

// showExisting pings a running instance; true if it answered.
func showExisting(guiURL, dataDir string) bool {
	base, token, ok := splitInstanceURL(guiURL)
	if !ok {
		return false
	}
	// Let the running instance take the foreground (errors are ignored).
	if b, err := os.ReadFile(filepath.Join(dataDir, "instance.pid")); err == nil {
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(b))); pid > 0 {
			allowForeground(pid)
		}
	}
	c := http.Client{Timeout: time.Second}
	req, _ := http.NewRequest("POST", base+"/api/show", nil)
	req.Header.Set("X-Token", token)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 300
}

// quitRunning asks the instance of dataDir to quit, the same way as the
// tray's Quit, and returns at once (the setup waits for the process itself).
func quitRunning(dataDir string) {
	b, err := os.ReadFile(filepath.Join(dataDir, "instance.url"))
	if err != nil {
		return
	}
	base, token, ok := splitInstanceURL(strings.TrimSpace(string(b)))
	if !ok {
		return
	}
	c := http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("POST", base+"/api/quit", nil)
	req.Header.Set("X-Token", token)
	if resp, err := c.Do(req); err == nil {
		resp.Body.Close()
	}
}

// instanceRunning pings a running instance without showing its window.
func instanceRunning(guiURL string) bool {
	base, token, ok := splitInstanceURL(guiURL)
	if !ok {
		return false
	}
	c := http.Client{Timeout: time.Second}
	// /api/state exists in every version, so older instances count too: any
	// answer to the GUI token means CaseHUD runs.
	req, _ := http.NewRequest("GET", base+"/api/state", nil)
	req.Header.Set("X-Token", token)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 300
}

// quitErr reports on stderr and exits with 1, without a message box.
func quitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func fatal(err error) {
	log.Println("fatal:", err)
	showError(fmt.Sprint(err))
	os.Exit(1)
}
