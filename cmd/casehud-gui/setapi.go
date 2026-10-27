// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/app"
)

// parseOnOff reads the value of --set-api ("" = flag not given).
func parseOnOff(v string) (on, given bool, err error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return false, false, nil
	case "on", "1", "true":
		return true, true, nil
	case "off", "0", "false":
		return false, true, nil
	}
	return false, true, fmt.Errorf("--set-api expects on or off, not %q", v)
}

// setAPISetting switches the "API" setting without starting the app (used by
// the installer: /API=1). A running instance gets the change through its own
// settings endpoint, so it also writes or removes api.json; otherwise only
// config.json is changed.
func setAPISetting(dir string, on bool) error {
	if b, err := os.ReadFile(filepath.Join(dir, "instance.url")); err == nil {
		if applyToRunning(strings.TrimSpace(string(b)), on) {
			return nil
		}
	}
	st := &app.Store{Dir: dir}
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	cfg.API = on
	return st.SaveConfig(cfg)
}

// applyToRunning changes the setting of the instance behind guiURL; false if
// none answers.
func applyToRunning(guiURL string, on bool) bool {
	i := strings.Index(guiURL, "/?t=")
	if i < 0 {
		return false
	}
	base, tok := guiURL[:i], guiURL[i+4:]
	c := http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", base+"/api/state", nil)
	req.Header.Set("X-Token", tok)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var state struct {
		Settings app.Settings `json:"settings"`
	}
	if resp.StatusCode >= 300 || json.NewDecoder(resp.Body).Decode(&state) != nil {
		return false
	}
	state.Settings.API = on
	state.Settings.FFmpeg = false // answer-only field
	b, _ := json.Marshal(state.Settings)
	req, _ = http.NewRequest("PUT", base+"/api/settings/advanced", bytes.NewReader(b))
	req.Header.Set("X-Token", tok)
	r2, err := c.Do(req)
	if err != nil {
		return false
	}
	r2.Body.Close()
	return r2.StatusCode < 300
}

// serviceSIDs are SYSTEM, LocalService and NetworkService: their profile is
// not the user's, so --set-api must not write there.
var serviceSIDs = map[string]bool{"S-1-5-18": true, "S-1-5-19": true, "S-1-5-20": true}

// refuseService returns an error when the process runs as a service account.
// A failed SID lookup refuses too.
func refuseService(sid func() (string, error)) error {
	s, err := sid()
	if err != nil {
		return fmt.Errorf("--set-api: cannot tell which user this is: %w", err)
	}
	if serviceSIDs[s] {
		return fmt.Errorf("--set-api: refused, running as a service account (%s); run it as the user", s)
	}
	return nil
}
