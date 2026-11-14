// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"

	"github.com/kewe87/casehud/internal/aiconnect"
	"github.com/kewe87/casehud/internal/web"
)

// aiEnv says where the AI apps of this user are. A demo looks only into its
// own data folder (demo-apps), never into the real folders of the user.
func aiEnv(demo bool, dataDir string) *aiconnect.Env {
	if demo {
		root := filepath.Join(dataDir, "demo-apps")
		return &aiconnect.Env{AppData: filepath.Join(root, "appdata"), Home: filepath.Join(root, "home"),
			Exe:      filepath.Join(root, "bin", "casehud-cli.exe"),
			LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	}
	exe := web.MCPCommand()
	if _, err := os.Stat(exe); err != nil {
		exe = "" // casehud-cli is not installed next to the app: nothing can be set up
	}
	appData, _ := os.UserConfigDir()
	home, _ := os.UserHomeDir()
	return &aiconnect.Env{AppData: appData, Home: home, Exe: exe}
}
