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
	"syscall"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/web"
)

// stateDir is the data folder; the Windows GUI keeps its window geometry there.
var stateDir string

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	demo := flag.Bool("demo", false, "simulate LCDs and sensors (no hardware needed)")
	minimized := flag.Bool("minimized", false, "start in the tray without opening the window")
	background := flag.Bool("background", false, "start in the tray; if CaseHUD already runs, exit silently without showing its window")
	noTray := flag.Bool("no-tray", false, "never create a tray icon (shells without a taskbar); the window opens through the API or a second start")
	port := flag.Int("port", 0, "GUI port on 127.0.0.1 (0 = random)")
	dataDir := flag.String("data", app.DefaultDir(), "data folder (config, designs, media)")
	setAPI := flag.String("set-api", "", "switch the API for other programs on or off in the settings, then exit (used by the installer)")
	flag.Parse()
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
	dataGiven := false
	flag.Visit(func(f *flag.Flag) { dataGiven = dataGiven || f.Name == "data" })

	store := &app.Store{Dir: *dataDir}
	stateDir = store.Dir
	if err := os.MkdirAll(store.Dir, 0o700); err != nil {
		fatal(err)
	}
	logFile, err := os.OpenFile(filepath.Join(store.Dir, "casehud.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err == nil {
		log.SetOutput(logFile)
	}

	if _, err := os.Stat(filepath.Join(store.Dir, "config.json")); err == nil {
		web.RestrictFile(filepath.Join(store.Dir, "config.json"))
	}

	// Single instance: if one is running, ask it to show its window.
	instFile := filepath.Join(store.Dir, "instance.url")
	if b, err := os.ReadFile(instFile); err == nil && !*demo {
		running := strings.TrimSpace(string(b))
		if *background {
			if instanceRunning(running) {
				return
			}
		} else if showExisting(running, store.Dir) {
			return
		}
	}

	engine := &app.Engine{Store: store, Demo: *demo}
	if err := engine.Start(context.Background()); err != nil {
		fatal(err)
	}
	// The markers (instance.url/pid, api.json) stay until the engine has
	// released the devices, so a second start cannot fight over them.
	var markers []func()
	defer func() {
		engine.Stop()
		for _, rm := range markers {
			rm()
		}
	}()

	srv := &web.Server{Engine: engine, Version: version}
	guiURL, err := srv.Listen(*port)
	if err != nil {
		fatal(err)
	}
	// A demo writes the instance files only into a folder given with --data,
	// never into the default one of a real installation.
	if !*demo || dataGiven {
		writeRestricted(instFile, guiURL)
		markers = append(markers, func() { os.Remove(instFile) })
		// The PID lets a second start give this window the foreground (api.json
		// only exists while the API is on).
		pidFile := filepath.Join(store.Dir, "instance.pid")
		writeRestricted(pidFile, strconv.Itoa(os.Getpid()))
		markers = append(markers, func() { os.Remove(pidFile) })
	}
	// Other programs find the v1 API through api.json (never log its token),
	// but only while the "API" setting is on (default off; --background and
	// --no-tray do not change that). A demo run never writes into the default
	// data folder, where a real installation's file would live.
	if !*demo || dataGiven {
		srv.APIFileDir = store.Dir
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
	os.WriteFile(path, []byte(content), 0o600)
}

// showExisting pings a running instance; true if it answered.
func showExisting(guiURL, dataDir string) bool {
	i := strings.Index(guiURL, "/?t=")
	if i < 0 {
		return false
	}
	// Let the running instance take the foreground (errors are ignored).
	if b, err := os.ReadFile(filepath.Join(dataDir, "instance.pid")); err == nil {
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(b))); pid > 0 {
			allowForeground(pid)
		}
	}
	c := http.Client{Timeout: time.Second}
	req, _ := http.NewRequest("POST", guiURL[:i]+"/api/show", nil)
	req.Header.Set("X-Token", guiURL[i+4:])
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 300
}

// instanceRunning pings a running instance without showing its window.
func instanceRunning(guiURL string) bool {
	i := strings.Index(guiURL, "/?t=")
	if i < 0 {
		return false
	}
	c := http.Client{Timeout: time.Second}
	// /api/state exists in every version, so older instances count too: any
	// answer to the GUI token means CaseHUD runs.
	req, _ := http.NewRequest("GET", guiURL[:i]+"/api/state", nil)
	req.Header.Set("X-Token", guiURL[i+4:])
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
