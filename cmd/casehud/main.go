// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

// Command casehud renders a live sensor dashboard (CPU/GPU load, temperatures,
// coolant, pump, ...) for PC case displays. Sensors come from
// LibreHardwareMonitor's remote web server, so it runs on Windows with NVIDIA
// GPUs as well as on Linux.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/icon"
	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/mcp"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/panel"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `casehud - sensor dashboard for PC case displays

USAGE:
  casehud version                             print the version
  casehud mcp [--data DIR]                    MCP server on stdio for AI assistants
                                              (needs the CaseHUD app running with
                                              the MCP interface enabled)
  casehud devices                             list connected displays
  casehud sensors [--url URL]                 list LibreHardwareMonitor sensors
  casehud init [--layout PATH]                write the default layout file
  casehud preview [--layout PATH] [--mock] [--out FILE]
                                              render one frame to a PNG
  casehud run [--layout PATH] [--mock] [--png FILE] [--brightness N]
                                              render continuously to all LCDs
                                              (or to a PNG file with --png)

Sensors are read from LibreHardwareMonitor (Options -> Remote Web Server -> Run,
default http://localhost:8085/data.json). --mock uses fake values instead.
Default layout path: %s
`

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(usage, lcd.DefaultLayoutPath())
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	layoutPath := fs.String("layout", lcd.DefaultLayoutPath(), "layout JSON file")
	url := fs.String("url", "", "LibreHardwareMonitor data.json URL (overrides layout)")
	mock := fs.Bool("mock", false, "use fake sensor values")
	out := fs.String("out", "preview.png", "preview output file")
	pngOut := fs.String("png", "", "write frames to this PNG file")
	brightness := fs.Int("brightness", -1, "LCD brightness 0-100 (-1 = leave as is)")

	switch cmd {
	case "-h", "--help", "help":
		fmt.Printf(usage, lcd.DefaultLayoutPath())
		return nil
	case "version", "--version":
		fmt.Println("casehud", version)
		return nil
	case "devices":
		return cmdDevices()
	case "mcp":
		mfs := flag.NewFlagSet("mcp", flag.ContinueOnError)
		dir := mfs.String("data", app.DefaultDir(), "CaseHUD data folder")
		if err := mfs.Parse(args); err != nil {
			return err
		}
		return cmdMCP(*dir)
	case "write-icon": // build helper for the installer
		if len(args) != 1 {
			return fmt.Errorf("usage: casehud write-icon FILE.ico")
		}
		return os.WriteFile(args[0], icon.ICO(16, 24, 32, 48, 64, 128, 256), 0o644)
	case "sensors", "init", "preview", "run":
	default:
		return fmt.Errorf("unknown command %q (try 'casehud help')", cmd)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if cmd == "init" {
		if _, err := os.Stat(*layoutPath); err == nil {
			return fmt.Errorf("%s already exists", *layoutPath)
		}
		if err := lcd.SaveLayout(*layoutPath, lcd.DefaultLayout()); err != nil {
			return err
		}
		fmt.Println("wrote", *layoutPath)
		return nil
	}

	layout, err := lcd.LoadLayout(*layoutPath)
	if os.IsNotExist(err) {
		layout = lcd.DefaultLayout()
	} else if err != nil {
		return err
	}
	if *url != "" {
		layout.LHMURL = *url
	}

	var prov metrics.Provider = metrics.NewDefault(layout.LHMURL)
	if *mock {
		prov = metrics.NewMock()
	}

	switch cmd {
	case "sensors":
		return cmdSensors(prov)
	case "preview":
		return cmdPreview(layout, prov, *mock, *out)
	default:
		if *pngOut != "" {
			return cmdRun([]target{{layout: layout, display: lcd.PNGDisplay{Path: *pngOut}}}, prov, layout.IntervalMS)
		}
		panels, err := panel.Discover()
		if err != nil {
			return err
		}
		if len(panels) == 0 {
			return fmt.Errorf("no supported display found (try 'casehud devices', or --png FILE to render to a file)")
		}
		var targets []target
		for _, p := range panels {
			if *brightness >= 0 {
				if err := p.SetBrightness(*brightness); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %s: %v\n", p.Name(), err)
				}
			}
			sz := p.Size()
			fmt.Printf("streaming to %s (%dx%d)\n", p.Name(), sz.X, sz.Y)
			targets = append(targets, target{layout: layout.ScaledTo(sz.X, sz.Y), display: panelDisplay{p}})
		}
		return cmdRun(targets, prov, layout.IntervalMS)
	}
}

// panelDisplay adapts a panel to lcd.Display.
type panelDisplay struct{ p panel.Panel }

func (d panelDisplay) Show(img image.Image) error { return panel.Show(d.p, img) }
func (d panelDisplay) Close() error               { return d.p.Close() }

type target struct {
	layout  lcd.Layout
	display lcd.Display
}

func cmdDevices() error {
	panels, err := panel.Discover()
	if err != nil {
		return err
	}
	if len(panels) == 0 {
		fmt.Println("no supported display found")
		return nil
	}
	for _, p := range panels {
		sz := p.Size()
		fmt.Printf("%-16s %-40s %dx%d\n", p.ID(), p.Name(), sz.X, sz.Y)
		p.Close()
	}
	return nil
}

func cmdSensors(p metrics.Provider) error {
	if err := p.Refresh(); err != nil {
		return err
	}
	rs := p.All()
	sort.Slice(rs, func(i, j int) bool { return rs[i].Path < rs[j].Path })
	for _, r := range rs {
		fmt.Printf("%-60s %10.1f %-4s  %s\n", r.Path, r.Value, r.Unit, r.Key)
	}
	return nil
}

func cmdPreview(l lcd.Layout, p metrics.Provider, mock bool, out string) error {
	r, err := lcd.NewRenderer(l)
	if err != nil {
		return err
	}
	// Fill the graph history so the preview shows a populated curve.
	samples := 1
	if mock {
		samples = l.HistorySize
	}
	base := time.Now()
	for i := 0; i < samples; i++ {
		if m, ok := p.(*metrics.Mock); ok {
			m.SetTime(base.Add(time.Duration(i-samples) * time.Duration(l.IntervalMS) * time.Millisecond))
		} else if err := p.Refresh(); err != nil {
			return err
		}
		r.Sample(p)
	}
	if err := (lcd.PNGDisplay{Path: out}).Show(r.Render(p)); err != nil {
		return err
	}
	fmt.Println("wrote", out)
	return nil
}

func cmdRun(targets []target, p metrics.Provider, intervalMS int) error {
	renderers := make([]*lcd.Renderer, len(targets))
	for i, t := range targets {
		defer t.display.Close()
		r, err := lcd.NewRenderer(t.layout)
		if err != nil {
			return err
		}
		renderers[i] = r
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tick := time.NewTicker(time.Duration(intervalMS) * time.Millisecond)
	defer tick.Stop()
	for {
		if err := p.Refresh(); err != nil {
			// LHM may just be starting up; keep going and show "--".
			fmt.Fprintln(os.Stderr, "warning:", err)
		}
		for i, t := range targets {
			renderers[i].Sample(p)
			if err := t.display.Show(renderers[i].Render(p)); err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// cmdMCP serves MCP on stdin/stdout, forwarding to the running app. The
// address is read per call so the app may be (re)started at any time.
func cmdMCP(dataDir string) error {
	api := &mcp.API{}
	resolve := func() error {
		b, err := os.ReadFile(filepath.Join(dataDir, "instance.url"))
		if err != nil {
			return fmt.Errorf("CaseHUD is not running: start the CaseHUD app first")
		}
		a, err := mcp.NewAPI(string(b))
		if err != nil {
			return err
		}
		*api = *a
		return nil
	}
	srv := mcp.NewCaseHUD(api, version)
	for i := range srv.Tools {
		h := srv.Tools[i].Handler
		srv.Tools[i].Handler = func(args json.RawMessage) (mcp.ToolResult, error) {
			if err := resolve(); err != nil {
				return mcp.ToolResult{}, err
			}
			return h(args)
		}
	}
	return srv.Serve(os.Stdin, os.Stdout)
}
