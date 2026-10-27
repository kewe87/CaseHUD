package app

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/awtrix"
	"github.com/kewe87/casehud/internal/metrics"
)

// AwtrixDevice is an AWTRIX 3 pixel clock that shows CaseHUD apps.
type AwtrixDevice struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Apps     []string `json:"apps"`
	Disabled bool     `json:"disabled,omitempty"`
}

// AwtrixAppInfo describes an app CaseHUD can put on a clock.
type AwtrixAppInfo struct {
	ID string `json:"id"`
}

// AwtrixApps lists the apps in the order they appear in the GUI. Next to an
// 8×8 icon about five characters fit on a 32×8 matrix, so the texts are just
// the reading; the icon (and on the TC002 its label) says what it is.
var AwtrixApps = []AwtrixAppInfo{{"cpu"}, {"gpu"}, {"load"}, {"coolant"}, {"fps"}, {"power"}}

const (
	awtrixInterval = 2 * time.Second
	awtrixLifetime = 10 * time.Second // until a clock drops an app CaseHUD no longer updates
	awtrixWarn     = "FAB005"
	awtrixCrit     = "F43F5E"
	awtrixLabel    = "FFFFFF"
)

// decimalSep swaps the first decimal point for a comma (German display text).
func decimalSep(s string, comma bool) string {
	if comma {
		return strings.Replace(s, ".", ",", 1)
	}
	return s
}

// awtrixApp builds the card for one app id from the current readings.
func awtrixApp(id string, p metrics.Provider, fahrenheit, comma bool) (awtrix.Card, bool) {
	val := func(src string) (float64, bool) {
		r, ok := metrics.Eval(p, src)
		return r.Value, ok
	}
	// Thresholds are in °C; only the shown number converts.
	tnum := func(v float64, ok bool, decimals int) string {
		if fahrenheit {
			v = v*9/5 + 32
		}
		if !ok {
			return "--"
		}
		return decimalSep(fmt.Sprintf("%.*f°", decimals, v), comma)
	}
	num := func(v float64, ok bool, decimals int, unit string) string {
		if !ok {
			return "--"
		}
		return decimalSep(fmt.Sprintf("%.*f%s", decimals, v, unit), comma)
	}
	colour := func(v float64, ok bool, base string, warn, crit float64, invert bool) string {
		switch {
		case !ok:
			return "8A94A8"
		case !invert && crit > 0 && v >= crit, invert && v > 0 && v < crit:
			return awtrixCrit
		case !invert && warn > 0 && v >= warn, invert && v > 0 && v < warn:
			return awtrixWarn
		}
		return base
	}
	pct := func(v float64, ok bool, max float64) int {
		if !ok {
			return -1
		}
		return int(math.Round(math.Max(0, math.Min(1, v/max)) * 100))
	}
	a := awtrix.Card{Icon: id, Progress: -1, Lifetime: awtrixLifetime}
	tempLoad := func(label, tempSrc, loadSrc, base string, warn, crit float64) {
		t, tok := val(tempSrc)
		l, lok := val(loadSrc)
		a.Label = label
		a.Text = []awtrix.Fragment{{T: tnum(t, tok, 0), C: colour(t, tok, base, warn, crit, false)}}
		a.Progress = pct(l, lok, 100)
		a.ProgressC = colour(l, lok, base, 85, 97, false)
		a.ProgressBC = "202020"
	}
	switch id {
	case "cpu":
		tempLoad("CPU", srcCPUTemp, srcCPULoad, "22D3EE", 85, 95)
	case "gpu":
		tempLoad("GPU", srcGPUTemp, srcGPULoad, "76E04A", 80, 88)
	case "load":
		c, cok := val(srcCPULoad)
		g, gok := val(srcGPULoad)
		a.Label = "C/G"
		a.Text = []awtrix.Fragment{
			{T: num(c, cok, 0, ""), C: colour(c, cok, "22D3EE", 85, 97, false)},
			{T: " ", C: awtrixLabel},
			{T: num(g, gok, 0, ""), C: colour(g, gok, "76E04A", 85, 97, false)},
		}
	case "coolant":
		v, ok := val(srcCoolant)
		a.Label = "H2O"
		a.Text = []awtrix.Fragment{{T: tnum(v, ok, 1), C: colour(v, ok, "3B82F6", 38, 42, false)}}
	case "fps":
		v, ok := val(srcFPS)
		a.Label = "FPS"
		a.Text = []awtrix.Fragment{{T: num(v, ok, 0, ""), C: colour(v, ok, "A78BFA", 60, 30, true)}}
		a.Progress = pct(v, ok, 144)
		a.ProgressC = colour(v, ok, "A78BFA", 60, 30, true)
		a.ProgressBC = "202020"
	case "power":
		v, ok := val(srcPowerSum)
		a.Label = "PC"
		a.Text = []awtrix.Fragment{{T: num(v, ok, 0, "W"), C: "F59E0B"}}
	default:
		return a, false
	}
	return a, true
}

// AwtrixPreview renders app id with demo readings as a clock of the given
// model would show it (PNG).
func (e *Engine) AwtrixPreview(id, model string) ([]byte, bool) {
	e.mu.Lock()
	fahrenheit := e.cfg.Fahrenheit
	e.mu.Unlock()
	card, ok := awtrixApp(id, metrics.NewMock(), fahrenheit, e.Language() == "de")
	if !ok {
		return nil, false
	}
	w, h := 32, 8
	if model == "tc002" {
		w, h = 52, 16
	}
	return awtrix.Preview(card, w, h, 5), true
}

func awtrixAppName(id string) string { return "casehud_" + id }

// awtrixState is what the push loop remembers between rounds.
type awtrixState struct {
	mu     sync.Mutex
	status map[string]string            // device id -> last error ("" = ok)
	info   map[string]awtrixInfo        // device id -> what the clock is
	pushed map[string]map[string]pushed // device id -> app id -> where it went
}

// awtrixInfo caches a probe; icons is set once CaseHUD's icons are on an
// AWTRIX 3 clock.
type awtrixInfo struct {
	host  string
	info  awtrix.Info
	icons bool
}

type pushed struct {
	host string
	fw   awtrix.Firmware
}

// AwtrixDevices returns the configured clocks, their last push status and
// what each clock was detected as.
func (e *Engine) AwtrixDevices() ([]AwtrixDevice, map[string]string, map[string]awtrix.Info) {
	e.mu.Lock()
	ds := slices.Clone(e.cfg.Awtrix)
	e.mu.Unlock()
	e.aw.mu.Lock()
	defer e.aw.mu.Unlock()
	st := map[string]string{}
	infos := map[string]awtrix.Info{}
	for _, d := range ds {
		if s, ok := e.aw.status[d.ID]; ok {
			st[d.ID] = s
		}
		if i, ok := e.aw.info[d.ID]; ok && i.host == d.Host {
			infos[d.ID] = i.info
		}
	}
	return ds, st, infos
}

// SetAwtrixDevices replaces the clock list. Apps on removed or disabled
// clocks are taken off by the next push round.
func (e *Engine) SetAwtrixDevices(ds []AwtrixDevice) error {
	e.awCfgMu.Lock()
	defer e.awCfgMu.Unlock()
	return e.setAwtrixDevices(ds)
}

func (e *Engine) setAwtrixDevices(ds []AwtrixDevice) error {
	seen := map[string]bool{}
	for i, d := range ds {
		if !ValidID(d.ID) || seen[d.ID] {
			return fmt.Errorf("invalid or duplicate clock id %q", d.ID)
		}
		seen[d.ID] = true
		if !awtrix.ValidHost(d.Host) {
			return fmt.Errorf("invalid address %q", d.Host)
		}
		apps := []string{}
		for _, a := range d.Apps {
			if _, ok := awtrixApp(a, metrics.NewMock(), false, false); ok && !slices.Contains(apps, a) {
				apps = append(apps, a)
			}
		}
		ds[i].Apps = apps
	}
	e.mu.Lock()
	e.cfg.Awtrix = ds
	e.mu.Unlock()
	e.wakeAwtrix()
	return e.persist()
}

// TestAwtrix checks that host is an AWTRIX clock and tells which.
func (e *Engine) TestAwtrix(ctx context.Context, host string) (awtrix.Info, error) {
	if !e.outputAllowed() {
		return awtrix.Info{}, ErrAckRequired
	}
	return awtrix.New(host).Probe(ctx)
}

// DiscoverAwtrix searches the local network for clocks.
func (e *Engine) DiscoverAwtrix(ctx context.Context) []awtrix.Found {
	if !e.outputAllowed() {
		return nil
	}
	return awtrix.Discover(ctx, 5*time.Second)
}

func (e *Engine) wakeAwtrix() {
	select {
	case e.awWake <- struct{}{}:
	default:
	}
}

// runAwtrix pushes the selected apps to every enabled clock until ctx ends,
// then removes them again.
func (e *Engine) runAwtrix(ctx context.Context) {
	t := time.NewTicker(awtrixInterval)
	defer t.Stop()
	for {
		if e.outputAllowed() {
			e.pushAwtrix(ctx)
		}
		select {
		case <-ctx.Done():
			if !e.outputAllowed() {
				return // nothing was ever sent
			}
			// Best effort: clocks would drop the apps after their lifetime anyway.
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			e.removeAwtrix(cleanup, nil)
			cancel()
			return
		case <-t.C:
		case <-e.awWake:
		}
	}
}

// clockInfo returns what the clock d is, probing it (and putting CaseHUD's
// icons on an AWTRIX 3 clock) when not known yet.
func (e *Engine) clockInfo(ctx context.Context, c *awtrix.Client, d AwtrixDevice) (awtrixInfo, error) {
	e.aw.mu.Lock()
	ci, ok := e.aw.info[d.ID]
	e.aw.mu.Unlock()
	if ok && ci.host == d.Host {
		return ci, nil
	}
	info, err := c.Probe(ctx)
	if err != nil {
		return ci, err
	}
	ci = awtrixInfo{host: d.Host, info: info}
	if info.Firmware == awtrix.Awtrix3 {
		// Without icons the apps fall back to text labels.
		ci.icons = c.UploadIcons(ctx) == nil
	}
	e.aw.mu.Lock()
	e.aw.info[d.ID] = ci
	e.aw.mu.Unlock()
	return ci, nil
}

func (e *Engine) pushAwtrix(ctx context.Context) {
	e.mu.Lock()
	ds := slices.Clone(e.cfg.Awtrix)
	fahrenheit := e.cfg.Fahrenheit
	e.mu.Unlock()
	comma := e.Language() == "de"
	keep := map[string]AwtrixDevice{}
	for _, d := range ds {
		if !d.Disabled && len(d.Apps) > 0 {
			keep[d.ID] = d
		}
	}
	e.removeAwtrix(ctx, keep)

	var wg sync.WaitGroup
	for _, d := range keep {
		wg.Add(1)
		go func(d AwtrixDevice) {
			defer wg.Done()
			c := awtrix.New(d.Host)
			ci, err := e.clockInfo(ctx, c, d)
			for _, id := range d.Apps {
				if err != nil {
					break
				}
				a, ok := awtrixApp(id, e.prov, fahrenheit, comma)
				if !ok {
					continue
				}
				if err = c.Push(ctx, awtrixAppName(id), a, ci.info, ci.icons); err != nil {
					// Unreachable, or the firmware changed: probe again next round.
					e.aw.mu.Lock()
					delete(e.aw.info, d.ID)
					e.aw.mu.Unlock()
					break
				}
				e.aw.mu.Lock()
				if e.aw.pushed[d.ID] == nil {
					e.aw.pushed[d.ID] = map[string]pushed{}
				}
				e.aw.pushed[d.ID][id] = pushed{d.Host, ci.info.Firmware}
				e.aw.mu.Unlock()
			}
			e.aw.mu.Lock()
			e.aw.status[d.ID] = ""
			if err != nil {
				e.aw.status[d.ID] = err.Error()
			}
			e.aw.mu.Unlock()
		}(d)
	}
	wg.Wait()
}

// removeAwtrix takes apps off clocks that are no longer configured for them
// (keep nil = remove everything CaseHUD pushed).
func (e *Engine) removeAwtrix(ctx context.Context, keep map[string]AwtrixDevice) {
	e.aw.mu.Lock()
	type job struct {
		dev, app string
		to       pushed
	}
	var jobs []job
	for dev, apps := range e.aw.pushed {
		d, ok := keep[dev]
		for app, to := range apps {
			if !ok || d.Host != to.host || !slices.Contains(d.Apps, app) {
				jobs = append(jobs, job{dev, app, to})
			}
		}
	}
	e.aw.mu.Unlock()
	for _, j := range jobs {
		awtrix.New(j.to.host).Remove(ctx, awtrixAppName(j.app), j.to.fw)
		e.aw.mu.Lock()
		delete(e.aw.pushed[j.dev], j.app)
		if len(e.aw.pushed[j.dev]) == 0 {
			delete(e.aw.pushed, j.dev)
			if _, ok := keep[j.dev]; !ok {
				delete(e.aw.status, j.dev)
				delete(e.aw.info, j.dev)
			}
		}
		e.aw.mu.Unlock()
	}
}
