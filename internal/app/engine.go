package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/maphash"
	"image"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/idle"
	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/media"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/panel"
)

// Engine owns all panels and streams to them in the background.
type Engine struct {
	Store *Store
	Demo  bool // use simulated panels and sensors instead of hardware
	// Topology optionally reports fans without a display for the arrangement view.
	Topology TopologySource

	rescanMu      sync.Mutex // serializes Rescan (discovery runs outside mu)
	mu            sync.Mutex
	cfg           Config
	prov          *SafeProvider
	workers       map[string]*worker
	order         []string
	clips         map[string]*media.Clip // cache: path@WxH
	scanErr       string
	manualRescans atomic.Int64 // manual searches, re-arm the watcher
	watchGaveUp   atomic.Bool  // automatic rescans stopped after repeated failures
	cancelFn      context.CancelFunc
	ctx           context.Context
	aw            awtrixState
	awWake        chan struct{}
	rot           rotationState
	auto          autoState     // under mu
	saveMu        sync.Mutex    // serialises config writes
	setMu         sync.Mutex    // serialises SetPanelConfig end to end; taken before mu, never while holding mu or a worker lock
	idleOff       bool          // screens are off for the screen saver (under mu)
	outputStarted bool          // device output was started (under mu; see disclaimer.go)
	tlp           tlpState      // start image of the TL LCD fans (see tlpersist.go)
	dv            devVideoState // videos encoded for the displays (devvideo.go)
	conf          confState     // other programs on the same devices (conflicts.go)

	// Now is the clock for leases and the like; nil = time.Now (tests set it).
	Now        func() time.Time
	powerOff   bool                         // all displays off by API request, not persisted (under mu)
	asleepNow  bool                         // what the workers were last told: idleOff || powerOff (under mu)
	sleepMu    sync.Mutex                   // held across computing and telling the workers, so they end up matching asleepNow; taken before mu
	rotMu      sync.Mutex                   // serialises read-modify-write of the rotation settings
	awCfgMu    sync.Mutex                   // same for the AWTRIX device list
	gamingFn   func(time.Time) GamingStatus // tests: replaces Gaming for the automation
	gamingHint gamingHint                   // external "a game is running" report (under mu)
	light      lightingState                // LED engine (lighting.go)
	library    libraryState                 // component/preset library (library.go)
	monitor    monitorModeState             // bar displays in monitor mode (monitorback.go)
	fw         fwState                      // optional firmware check (fwupdate.go)
	devMu      sync.Mutex                   // guards the next two (devices.go); taken last
	sensorDevs []panel.SensorDevice         // read-only devices
	devSrcs    []panel.SensorSource         // everything that reports sensors
}

// PanelState is a panel as the GUI sees it.
type PanelState struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Round  bool   `json:"round"`
	// Experimental marks a display whose driver is not confirmed on hardware.
	Experimental bool        `json:"experimental,omitempty"`
	Config       PanelConfig `json:"config"`
	// Number is the logical number shown by Identify: the user's fan number
	// (see Arrangement), for other displays the count continues behind it.
	Number int `json:"number"`
	// OfflineClock is set for bar displays that can show their own clock when
	// CaseHUD is not running (a flag the display keeps): whether it is on.
	OfflineClock *bool `json:"offlineClock,omitempty"`
	// DeviceFlip is set for bar displays that can turn the picture by 180
	// degrees themselves (experimental): whether it is on.
	DeviceFlip *bool `json:"deviceFlip,omitempty"`
	// Grids are the layouts offered for a bar display (nil for other panels).
	Grids []lcd.GridPreset `json:"grids,omitempty"`
	// Firmware is the version the device reported (read only, "" if unknown).
	Firmware string `json:"firmware,omitempty"`
	// FirmwareLatest is what the manufacturer offers (only with the firmware check on).
	FirmwareLatest *FirmwareNote `json:"firmwareLatest,omitempty"`
	Error          string        `json:"error,omitempty"`
	// ErrorCode classifies Error (see the Err* constants).
	ErrorCode string `json:"errorCode,omitempty"`
}

// Start loads the config, connects to the sensor backend and the panels.
func (e *Engine) Start(ctx context.Context) error {
	cfg, err := e.Store.LoadConfig()
	if err != nil {
		return err
	}
	e.ctx, e.cancelFn = context.WithCancel(ctx)
	e.cfg = cfg
	panel.SetOptions(panelOptions(cfg))
	e.startSettingsCache(cfg)
	e.clips = map[string]*media.Clip{}
	var p metrics.Provider = e.providerFor(cfg)
	if e.Demo {
		p = metrics.NewMock()
	}
	e.prov = NewSafeProvider(p)
	e.prov.SetGameIgnore(cfg.Automation.IgnoreGames)
	go e.prov.Run(e.ctx, time.Second)
	e.aw = awtrixState{status: map[string]string{}, info: map[string]awtrixInfo{}, pushed: map[string]map[string]pushed{}}
	e.awWake = make(chan struct{}, 1)
	go e.runAwtrix(e.ctx)
	e.rot = rotationState{skip: make(chan struct{}, 1)}
	e.auto = autoState{}
	migrateLegacyGaming(&e.cfg)
	sanitizeLoadedConfig(&e.cfg)
	go e.runRotation(e.ctx)
	go e.runIdle(e.ctx)
	e.startConflicts()
	e.startLighting()
	e.startFirmwareCheck()
	if e.outputAllowed() {
		e.startOutput() // otherwise it waits for AcceptDisclaimer
	}
	return nil
}

// providerFor is the standard provider plus the readings of the devices
// CaseHUD talks to itself (pump heads, temperature hub).
func (e *Engine) providerFor(c Config) metrics.Provider {
	return providerWith(c, &metrics.Func{Fn: e.deviceReadings})
}

func providerWith(c Config, extra metrics.Provider) metrics.Provider {
	url := c.SensorURL
	if url == "" {
		url = c.LHMURL
	}
	o := metrics.Options{
		SensorURLs: metrics.SensorURLsFor(c.SensorSource, url),
		PresentMon: !c.NoPresentMon,
		RTSS:       !c.NoRTSS,
	}
	if extra != nil {
		o.Extra = []metrics.Provider{extra}
	}
	return metrics.New(o)
}

// Settings are the user-controllable options shown in the GUI.
type Settings struct {
	SensorSource string `json:"sensorSource"`
	SensorURL    string `json:"sensorUrl"`
	PresentMon   bool   `json:"presentmon"`
	RTSS         bool   `json:"rtss"`
	IntervalMS   int    `json:"intervalMs"`
	UpdateCheck  bool   `json:"updateCheck"`
	// FirmwareCheck asks the manufacturer's download page for new firmware once per start.
	FirmwareCheck bool `json:"firmwareCheck"`
	Animation     bool `json:"animation"`
	MCP           bool `json:"mcp"`
	API           bool `json:"api"`
	// ScreensaverOff turns the LCDs off while the screen saver runs.
	ScreensaverOff bool `json:"screensaverOff"`
	Fahrenheit     bool `json:"fahrenheit"`
	Clock12h       bool `json:"clock12h"`
	// DeviceVideo lets displays that can play video themselves do so (needs
	// ffmpeg, default on); nil in a request leaves it as it is. FFmpeg tells
	// whether ffmpeg was found (answer only).
	DeviceVideo *bool `json:"deviceVideo,omitempty"`
	FFmpeg      bool  `json:"ffmpeg,omitempty"`
}

// Settings returns the current options.
func (e *Engine) Settings() Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	src := e.cfg.SensorSource
	if src == "" {
		src = "auto"
	}
	return Settings{SensorSource: src, SensorURL: e.cfg.SensorURL, PresentMon: !e.cfg.NoPresentMon,
		RTSS: !e.cfg.NoRTSS, IntervalMS: e.cfg.IntervalMS, UpdateCheck: !e.cfg.NoUpdateCheck, FirmwareCheck: e.cfg.FWUpdateCheck,
		Animation: !e.cfg.NoAnimation, MCP: e.cfg.MCP, API: e.cfg.API, ScreensaverOff: e.cfg.ScreensaverOff,
		Fahrenheit: e.cfg.Fahrenheit, Clock12h: e.cfg.Clock12h,
		DeviceVideo: refTo(!e.cfg.NoDeviceVideo), FFmpeg: ffmpegAvailable()}
}

// APIEnabled tells whether the v1 API for other programs is switched on.
func (e *Engine) APIEnabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.API
}

func refTo[T any](v T) *T { return &v }

// renderSettings are the options that change what the workers draw or send.
type renderSettings struct {
	interval                 int
	noAnim, fahrenheit, c12h bool
	noDeviceVideo            bool
}

func (e *Engine) renderSettingsLocked() renderSettings {
	return renderSettings{e.cfg.IntervalMS, e.cfg.NoAnimation, e.cfg.Fahrenheit, e.cfg.Clock12h, e.cfg.NoDeviceVideo}
}

// ApplySettings stores new options, rebuilds the sensor provider and
// restarts the display workers so everything takes effect immediately.
func (e *Engine) ApplySettings(s Settings) error {
	switch s.SensorSource {
	case "auto", "service", "lhm":
	case "custom":
		if s.SensorURL == "" {
			return fmt.Errorf("please enter the sensor address")
		}
	default:
		return fmt.Errorf("unknown sensor source %q", s.SensorSource)
	}
	switch s.IntervalMS {
	case 0, 500, 1000, 2000, 5000:
	default:
		return fmt.Errorf("unsupported refresh interval %d ms", s.IntervalMS)
	}
	e.mu.Lock()
	renderBefore := e.renderSettingsLocked()
	e.cfg.SensorSource, e.cfg.SensorURL = s.SensorSource, s.SensorURL
	if s.SensorSource == "auto" {
		e.cfg.SensorSource = ""
	}
	e.cfg.NoPresentMon, e.cfg.NoRTSS = !s.PresentMon, !s.RTSS
	e.cfg.IntervalMS = s.IntervalMS
	e.cfg.NoUpdateCheck = !s.UpdateCheck
	fwWas := e.cfg.FWUpdateCheck
	e.cfg.FWUpdateCheck = s.FirmwareCheck
	e.cfg.NoAnimation = !s.Animation
	e.cfg.MCP = s.MCP
	e.cfg.API = s.API
	e.cfg.ScreensaverOff = s.ScreensaverOff
	e.cfg.Fahrenheit, e.cfg.Clock12h = s.Fahrenheit, s.Clock12h
	if s.DeviceVideo != nil {
		e.cfg.NoDeviceVideo = !*s.DeviceVideo
	}
	cfg := e.cfg
	var ws []*worker
	if e.renderSettingsLocked() != renderBefore { // other changes need no restart (no flicker)
		for _, w := range e.workers {
			ws = append(ws, w)
		}
	}
	e.mu.Unlock()
	e.firmwareSwitched(fwWas, s.FirmwareCheck)
	if !e.Demo {
		e.prov.Swap(e.providerFor(cfg))
	}
	for _, w := range ws {
		w.reconfigure(w.current()) // keeps a rotation's content on screen
	}
	return e.persist()
}

// CommunityURL is the configured community gallery index ("" = default).
func (e *Engine) CommunityURL() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.CommunityURL
}

// cleanPanelConfig validates c and clamps its ranges.
func (e *Engine) cleanPanelConfig(c PanelConfig) (PanelConfig, error) {
	c = c.Clone()
	switch c.Mode {
	case "design", "media", "off":
	default:
		return c, fmt.Errorf("unknown mode %q", c.Mode)
	}
	if c.Theme != "light" {
		c.Theme = ""
	}
	c.Brightness = max(0, min(100, c.Brightness))
	switch c.Rotation {
	case 0, 90, 180, 270:
	default:
		c.Rotation = 0
	}
	if !e.validBackground(c.Background) {
		return c, fmt.Errorf("unknown background %q", c.Background)
	}
	c.BgStrength = max(0, min(100, c.BgStrength))
	if c.Design != "" && !ValidID(c.Design) {
		return c, fmt.Errorf("invalid design id %q", c.Design)
	}
	c, err := e.resolveLayout(c)
	if err != nil {
		return c, err
	}
	c, err = e.cleanGrid(c)
	if err != nil {
		return c, err
	}
	if c.Grid == "" {
		c.Layout = ""
	}
	return c, nil
}

// persist saves the config. Marshalling happens under e.mu (the config
// holds maps that requests change), and saves are serialised so an older
// snapshot can never overwrite a newer one.
func (e *Engine) persist() error {
	e.saveMu.Lock()
	defer e.saveMu.Unlock()
	e.mu.Lock()
	b, err := json.MarshalIndent(e.cfg, "", "  ")
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return e.Store.saveConfigJSON(b)
}

// localize applies the unit and clock settings to a renderer.
func (e *Engine) localize(r *lcd.Renderer) {
	e.mu.Lock()
	r.Fahrenheit, r.Clock12h = e.cfg.Fahrenheit, e.cfg.Clock12h
	e.mu.Unlock()
}

// animationTau is how quickly shown values follow a new reading: about
// 95 % of the way after three time constants.
const animationTau = 120 * time.Millisecond

func (e *Engine) animationOff() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.NoAnimation
}

// intervalOverride is the global refresh interval, 0 = per design.
func (e *Engine) intervalOverride() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Duration(e.cfg.IntervalMS) * time.Millisecond
}

// Stop blanks nothing (panels keep their last frame) and releases devices.
func (e *Engine) Stop() {
	e.cancelFn()
	// Wait for a running Rescan: it sees the cancelled context and closes
	// what it found instead of installing workers behind our back.
	e.rescanMu.Lock()
	defer e.rescanMu.Unlock()
	e.stopLighting()
	e.setSensorSources(nil, nil)
	e.mu.Lock()
	old := e.workers
	e.workers, e.order = nil, nil
	e.mu.Unlock()
	e.endStartImages()
	for _, w := range old {
		if e.outputAllowed() {
			w.setExitImage()
		}
		w.stop()
	}
	// The settings cache may have changed while the panels closed; its own
	// timer no longer saves once the context is cancelled.
	if err := e.persist(); err != nil {
		log.Println("save config:", err)
	}
}

// Provider exposes the shared sensor provider (for the design editor).
func (e *Engine) Provider() *SafeProvider { return e.prov }

// Discovery seams (tests replace them so no real device is touched).
var (
	discoverPanels  = panel.Discover
	discoverSensors = panel.DiscoverSensors
)

// Rescan closes all panels and discovers them again (hot-plug, recovery).
func (e *Engine) Rescan() {
	// Rescans are serialized; discovery itself runs without e.mu so a slow
	// device init cannot stall the UI/API.
	e.rescanMu.Lock()
	defer e.rescanMu.Unlock()
	e.scanConflicts()
	// Stop old workers without holding e.mu: they may be waiting on it.
	e.mu.Lock()
	old := e.workers
	e.workers, e.order = map[string]*worker{}, nil
	e.mu.Unlock()
	e.endStartImages()
	for _, w := range old {
		w.stop()
	}

	var panels []panel.Panel
	var err error
	var sensors []panel.SensorDevice
	e.setSensorSources(nil, nil) // close the old read-only devices before opening them again
	if !e.outputAllowed() {
		// The disclaimer is not accepted yet: no device is opened.
		e.mu.Lock()
		e.scanErr = ""
		e.mu.Unlock()
		return
	}
	if e.Demo {
		panels = demoPanels()
	} else {
		panels, err = discoverPanels()
	}
	if !e.Demo {
		var serr error
		if sensors, serr = discoverSensors(); serr != nil {
			log.Println("sensor devices:", serr)
		}
	}
	if e.ctx != nil && e.ctx.Err() != nil { // Stop ran meanwhile: nobody would close these later
		for _, p := range panels {
			p.Close()
		}
		for _, s := range sensors {
			s.Close()
		}
		return
	}
	e.setSensorSources(panels, sensors)
	e.applyDeviceFlip(panels)

	e.mu.Lock()
	defer e.mu.Unlock()
	e.asleepNow = e.idleOff || e.powerOff
	e.scanErr = ""
	if err != nil {
		e.scanErr = err.Error()
		log.Println("panel discovery:", err)
	}
	for _, p := range panels {
		round := isRound(p)
		cfg, ok := e.cfg.Panels[p.ID()]
		if !ok {
			cfg = defaultConfigFor(p.Size(), round)
			e.cfg.Panels[p.ID()] = cfg
		}
		w := &worker{e: e, p: p, round: round, wake: make(chan struct{}, 1), switchCh: make(chan switchReq, 1), asleep: e.idleOff || e.powerOff}
		e.workers[p.ID()] = w
		e.order = append(e.order, p.ID())
		w.start(e.ctx, cfg)
	}
	e.sortFansLocked()
	e.startImagesAfterScan(panels)
}

// ScanError is the last discovery error, if any.
func (e *Engine) ScanError() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.scanErr
}

// Panels lists the connected panels.
func (e *Engine) Panels() []PanelState {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]PanelState, 0, len(e.order)) // [] not null in JSON
	nums := e.numbersLocked()
	for _, id := range e.order {
		w := e.workers[id]
		cfg := e.cfg.Panels[id].Clone()
		sz := w.sizeFor(cfg.Rotation)
		out = append(out, PanelState{ID: id, Name: w.p.Name(), Width: sz.X, Height: sz.Y,
			Round: w.round, Experimental: panel.IsExperimental(w.p), Config: cfg, Number: nums[id], Error: w.err(), ErrorCode: w.errClass(), Grids: lcd.GridPresets(sz.X, sz.Y)})
		if v, ok := w.p.(panel.Versioned); ok {
			out[len(out)-1].Firmware = v.Version()
			out[len(out)-1].FirmwareLatest = e.firmwareNote(panelFirmwareProduct(w.p), v.Version())
		}
		if _, ok := w.p.(panel.OfflineClocker); ok {
			out[len(out)-1].OfflineClock = refTo(e.cfg.OfflineClock[id])
		}
		if f, ok := w.p.(panel.DeviceFlipper); ok && f.CanDeviceFlip() {
			out[len(out)-1].DeviceFlip = refTo(e.cfg.DeviceFlip[id])
		}
	}
	return out
}

// SetPanelConfig applies and persists a panel's configuration. The config
// replaces the old one completely (clients send the full object).
//
// A new design or media file must exist; an unchanged reference is accepted
// even if the file was deleted since, so other settings of such a panel stay
// editable (the panel shows an error until a valid one is chosen).
func (e *Engine) SetPanelConfig(id string, c PanelConfig) error {
	// One call at a time: otherwise two callers could leave the worker with
	// one config and e.cfg (and the file) with the other.
	e.setMu.Lock()
	defer e.setMu.Unlock()
	return e.setPanelConfigLocked(id, c)
}

// UpdatePanelConfig applies change to the panel's current configuration and
// stores the result with the same validation as SetPanelConfig. Reading and
// writing happen under one lock, so concurrent patches don't lose fields.
func (e *Engine) UpdatePanelConfig(id string, change func(*PanelConfig) error) (PanelConfig, error) {
	e.setMu.Lock()
	defer e.setMu.Unlock()
	e.mu.Lock()
	_, ok := e.workers[id]
	c := e.cfg.Panels[id].Clone()
	e.mu.Unlock()
	if !ok {
		return c, fmt.Errorf("%w: %q", ErrNoPanel, id)
	}
	if err := change(&c); err != nil {
		return c, err
	}
	if err := e.setPanelConfigLocked(id, c); err != nil {
		return c, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.Panels[id].Clone(), nil
}

// setPanelConfigLocked is SetPanelConfig with e.setMu already held.
func (e *Engine) setPanelConfigLocked(id string, c PanelConfig) error {
	c, err := e.cleanPanelConfig(c)
	if err != nil {
		return err
	}
	e.mu.Lock()
	if _, ok := e.workers[id]; !ok {
		e.mu.Unlock()
		return fmt.Errorf("panel %q not connected", id)
	}
	old := e.cfg.Panels[id].Clone()
	wk := e.workers[id]
	e.mu.Unlock()
	if c.Grid != "" && c.Mode == "design" && old.Grid == c.Grid {
		// Turning the bar upright (or back) turns the wide fields with it.
		was, now := wk.sizeFor(old.Rotation), wk.sizeFor(c.Rotation)
		if was != now && slices.Equal(old.Tiles, c.Tiles) {
			reorientTiles(&c, now.Y > now.X)
		}
	}
	if c.Layout != "" && c.Mode == "design" && wk.sizeFor(c.Rotation).Y > wk.sizeFor(c.Rotation).X {
		reorientTiles(&c, true) // layouts are made for the landscape bar
	}
	if c.Grid != "" && c.Mode == "design" {
		if sz := wk.sizeFor(c.Rotation); lcd.GridPresets(sz.X, sz.Y) == nil {
			return fmt.Errorf("grids are only available on bar displays (this panel is %dx%d)", sz.X, sz.Y)
		}
		if _, ok := lcd.GridPresetByID(wk.sizeFor(c.Rotation).X, wk.sizeFor(c.Rotation).Y, c.Grid); !ok {
			return fmt.Errorf("unknown grid %q", c.Grid)
		}
	}
	if err := e.checkContent(c, old); err != nil {
		return err
	}
	e.mu.Lock()
	// The panel may have disconnected (Rescan) during the I/O above.
	w, ok := e.workers[id]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("panel %q not connected", id)
	}
	e.cfg.Panels[id] = c
	e.rot.transient = "" // the user changed something: a one-off scene is over
	e.mu.Unlock()
	w.reconfigure(c)
	return e.persist()
}

// checkContent rejects a design or media reference that is malformed (via
// Store.MediaPath) or, when it differs from old, points to nothing.
func (e *Engine) checkContent(c, old PanelConfig) error {
	switch c.Mode {
	case "design":
		if c.Grid != "" {
			return e.checkTiles(c, old)
		}
		if c.Design != "" && c.Design != old.Design {
			if _, err := e.Store.Design(c.Design); err != nil {
				return err
			}
		}
	case "media":
		if c.Media == "" {
			return nil
		}
		p, err := e.Store.MediaPath(c.Media)
		if err != nil {
			return err
		}
		if c.Media != old.Media {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("media file %q not found", c.Media)
			}
		}
	}
	return nil
}

// Identify shows each panel's number for a few seconds; id "" identifies
// all panels.
func (e *Engine) Identify(id string) error {
	if id == "" {
		return e.identify(nil)
	}
	return e.identify([]string{id})
}

// IdentifyIDs shows the numbers on the given panels.
func (e *Engine) IdentifyIDs(ids []string) error {
	if len(ids) == 0 {
		return errors.New("no panels given")
	}
	return e.identify(ids)
}

// identify shows the numbers on ids, or on every panel when ids is nil.
func (e *Engine) identify(ids []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	nums := e.numbersLocked()
	found := false
	for _, pid := range e.order {
		if ids != nil && !slices.Contains(ids, pid) {
			continue
		}
		w := e.workers[pid]
		name := e.cfg.Panels[pid].Name
		if name == "" {
			name = w.p.Name()
		}
		w.identify(nums[pid], name)
		found = true
	}
	if !found {
		return fmt.Errorf("panel %q not connected", strings.Join(ids, ","))
	}
	return nil
}

// Preview returns the last JPEG sent to a panel.
func (e *Engine) Preview(id string) []byte {
	e.mu.Lock()
	w, ok := e.workers[id]
	e.mu.Unlock()
	if !ok {
		return nil
	}
	return w.preview()
}

// Frame is the latest JPEG of a panel and its sequence number.
type Frame struct {
	ID  string
	Seq uint64
	JPG []byte
}

// Frames returns the latest frame of every panel whose sequence number
// differs from seen[id].
func (e *Engine) Frames(seen map[string]uint64) []Frame {
	e.mu.Lock()
	ws := make(map[string]*worker, len(e.workers))
	for id, w := range e.workers {
		ws[id] = w
	}
	e.mu.Unlock()
	var out []Frame
	for id, w := range ws {
		w.mu.Lock()
		seq := w.lastSeq
		w.mu.Unlock()
		if seq == seen[id] {
			continue // nothing new: don't encode
		}
		if b, seq := w.snapshot(); b != nil {
			out = append(out, Frame{id, seq, b})
		}
	}
	return out
}

// RenderDesign renders a layout at w x h with live sensor values.
func (e *Engine) RenderDesign(l lcd.Layout, w, h int) (*image.RGBA, error) {
	l.Lang = e.Language()
	r, err := lcd.NewRenderer(l.FitTo(w, h))
	if err != nil {
		return nil, err
	}
	e.localize(r)
	r.Sample(e.prov)
	r.Sample(e.prov) // two samples so graphs show a line
	return r.Render(e.prov), nil
}

// UpdateCheck reports whether the release check is enabled.
func (e *Engine) UpdateCheck() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.cfg.NoUpdateCheck
}

// SetUpdateCheck enables or disables the release check.
func (e *Engine) SetUpdateCheck(on bool) error {
	e.mu.Lock()
	e.cfg.NoUpdateCheck = !on
	e.mu.Unlock()
	return e.persist()
}

// Language is the UI language, used for clock day/month names.
func (e *Engine) Language() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cfg.Language == "" {
		return "de"
	}
	return e.cfg.Language
}

// SetLanguage stores the UI language and refreshes panels showing designs.
func (e *Engine) SetLanguage(lang string) error {
	if lang != "de" && lang != "en" {
		return fmt.Errorf("unsupported language %q", lang)
	}
	e.mu.Lock()
	e.cfg.Language = lang
	var ws []*worker
	for _, w := range e.workers {
		ws = append(ws, w)
	}
	e.mu.Unlock()
	for _, w := range ws {
		if c := w.current(); c.Mode == "design" {
			w.reconfigure(c)
		}
	}
	return e.persist()
}

// DesignChanged restarts panels using design id so edits show up live.
func (e *Engine) DesignChanged(id string) {
	e.mu.Lock()
	var ws []*worker
	for _, w := range e.workers {
		ws = append(ws, w)
	}
	e.mu.Unlock()
	for _, w := range ws {
		if c := w.current(); c.usesDesign(id) {
			w.reconfigure(c)
		}
	}
}

// MediaChanged drops cached clips for a file and restarts panels showing it.
func (e *Engine) MediaChanged(name string) {
	e.mu.Lock()
	for k := range e.clips {
		delete(e.clips, k)
	}
	var ws []*worker
	var cs []PanelConfig
	for pid, w := range e.workers {
		if c := e.cfg.Panels[pid]; c.usesMedia(name) {
			ws, cs = append(ws, w), append(cs, c.Clone())
		}
	}
	e.mu.Unlock()
	for i, w := range ws {
		w.reconfigure(cs[i])
	}
}

func (e *Engine) clip(name string, w, h int) (*media.Clip, error) {
	path, err := e.Store.MediaPath(name)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s@%dx%d", path, w, h)
	e.mu.Lock()
	c, ok := e.clips[key]
	e.mu.Unlock()
	if ok {
		return c, nil
	}
	c, err = media.Load(path, w, h)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.clips[key] = c
	e.mu.Unlock()
	return c, nil
}

// maxFPS is the panel's frame rate limit for animations (default 24).
func maxFPS(p panel.Panel) int {
	if f, ok := p.(interface{ MaxFPS() int }); ok && f.MaxFPS() > 0 {
		return f.MaxFPS()
	}
	return 24
}

// isRound reports whether a panel's visible area is circular.
func isRound(p panel.Panel) bool {
	if r, ok := p.(interface{ Round() bool }); ok {
		return r.Round()
	}
	return false
}

// worker streams to one panel.
type worker struct {
	e     *Engine
	p     panel.Panel
	round bool

	mu          sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
	last        []byte             // JPEG of the latest frame; nil until encoded for a raw panel
	lastImg     image.Image        // latest frame of a raw panel, encoded for the preview on demand
	lastFn      func() image.Image // latest layered frame, composed for the preview on demand
	lastSeq     uint64             // counts frames, so the live view sends only new ones
	lastErr     string
	errCode     string    // machine-readable class of lastErr, see the Err* constants
	devErrSince time.Time // since when the device has been failing without a success in between
	dw          devWrites
	ocMu        sync.Mutex  // guards ocTimer
	ocTimer     *time.Timer // pending write of the offline clock (offlineclock.go)

	// Identify overlay: a big number shown for a few seconds.
	idNum   int
	idName  string
	idUntil time.Time
	idImg   image.Image // rendered once per identify request
	wake    chan struct{}

	switchCh chan switchReq // content changes from rotation
	asleep   bool           // screen off while the screen saver runs (under mu)

	encJPG  []byte // encode cache, see encode
	encHash uint64
	encRect image.Rectangle

	life    sync.Mutex  // serialises start/stop/reconfigure
	stopped bool        // closed by Rescan/Stop: never start again (under life)
	cur     PanelConfig // what the panel shows now, incl. rotation (under mu)
}

// current is what the panel shows right now (a rotation may override the
// configured content).
func (w *worker) current() PanelConfig {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cur
}

func (w *worker) setCurrent(c PanelConfig) {
	w.mu.Lock()
	w.cur = c
	w.mu.Unlock()
}

func (w *worker) start(ctx context.Context, c PanelConfig) {
	w.life.Lock()
	defer w.life.Unlock()
	if !w.stopped {
		w.startLocked(ctx, c)
	}
}

func (w *worker) startLocked(ctx context.Context, c PanelConfig) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.mu.Lock()
	w.cancel, w.done = cancel, done
	w.mu.Unlock()
	go func() {
		defer close(done)
		w.run(ctx, c)
	}()
}

// halt stops the running loop and waits for it (caller holds w.life).
func (w *worker) halt() {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel, w.done = nil, nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (w *worker) stop() {
	w.life.Lock()
	defer w.life.Unlock()
	w.halt()
	if !w.stopped {
		w.stopped = true
		w.ocMu.Lock()
		pending := w.ocTimer != nil && w.ocTimer.Stop()
		w.ocMu.Unlock()
		if pending {
			w.writeOfflineClock(true) // the user's change must not get lost with the worker
		}
		w.dw.close()
		w.p.Close()
	}
}

func (w *worker) reconfigure(c PanelConfig) {
	w.life.Lock()
	defer w.life.Unlock()
	if w.stopped {
		return // the panel was closed by a rescan meanwhile
	}
	w.halt()
	w.startLocked(w.e.ctx, c)
}

// Panel error classes reported by the external API.
const (
	ErrDevice  = "device_error"  // the display rejected a frame or setting
	ErrFrame   = "frame_error"   // a frame could not be encoded for the display
	ErrContent = "content_error" // the design, media file or background cannot be loaded
)

// setErr records the panel's last error with its class; nil clears it.
func (w *worker) setErr(code string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		w.lastErr, w.errCode = "", ""
	} else {
		w.lastErr, w.errCode = err.Error(), code
	}
	switch {
	case err == nil || code != ErrDevice:
		w.devErrSince = time.Time{}
	case w.devErrSince.IsZero():
		w.devErrSince = time.Now()
	}
}

// deviceFailingSince is the start of the longest ongoing run of device errors
// over all panels, or the zero time if every panel works.
func (e *Engine) deviceFailingSince() time.Time {
	e.mu.Lock()
	ws := make([]*worker, 0, len(e.workers))
	for _, w := range e.workers {
		ws = append(ws, w)
	}
	e.mu.Unlock()
	var oldest time.Time
	for _, w := range ws {
		w.mu.Lock()
		t := w.devErrSince
		w.mu.Unlock()
		if !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
			oldest = t
		}
	}
	return oldest
}

func (w *worker) errClass() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.errCode
}

func (w *worker) err() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

func (w *worker) preview() []byte {
	b, _ := w.snapshot()
	return b
}

// snapshot returns the latest frame as JPEG with its sequence number. Raw
// panels are not sent a JPEG, so it is made here, once per frame and only when
// somebody looks (preview, live view).
func (w *worker) snapshot() ([]byte, uint64) {
	w.mu.Lock()
	b, img, seq, fn := w.last, w.lastImg, w.lastSeq, w.lastFn
	w.mu.Unlock()
	if b != nil || (img == nil && fn == nil) {
		return b, seq
	}
	if img == nil {
		img = fn() // a layered frame: put the layers together
	}
	b, err := panel.EncodeJPEG(w.p, img)
	if err != nil {
		return nil, seq
	}
	w.mu.Lock()
	if w.lastSeq == seq && (w.lastImg == img || fn != nil) {
		w.last = b
	}
	w.mu.Unlock()
	return b, seq
}

func (w *worker) show(img image.Image) {
	if rp, ok := w.p.(panel.RawPanel); ok {
		err := rp.ShowImage(img)
		w.mu.Lock()
		w.last, w.lastImg, w.lastFn = nil, img, nil
		w.lastSeq++
		w.mu.Unlock()
		w.setErr(ErrDevice, err)
		return
	}
	code := ErrFrame
	b, err := w.encode(img)
	if err == nil {
		code = ErrDevice
		err = w.p.ShowJPEG(b)
	}
	w.mu.Lock()
	if b != nil {
		w.last, w.lastFn = b, nil
		w.lastSeq++
	}
	w.mu.Unlock()
	w.setErr(code, err)
}

var frameSeed = maphash.MakeSeed()

// encode is panel.EncodeJPEG with a one-entry cache: static content is sent
// again every second as a keep-alive and is not encoded each time. Only used
// from the worker's own goroutine.
func (w *worker) encode(img image.Image) ([]byte, error) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return panel.EncodeJPEG(w.p, img)
	}
	h := maphash.Bytes(frameSeed, rgba.Pix)
	if w.encJPG != nil && h == w.encHash && rgba.Rect == w.encRect {
		return w.encJPG, nil
	}
	b, err := panel.EncodeJPEG(w.p, img)
	if err == nil {
		w.encJPG, w.encHash, w.encRect = b, h, rgba.Rect
	}
	return b, err
}

// switchReq asks a running worker to change content with a transition.
type switchReq struct {
	c      PanelConfig
	effect string
	at     time.Time
}

// switchTo changes the panel's content at now+delay with a transition
// effect, without restarting the worker (used by rotation). A newer
// request replaces a pending one.
func (w *worker) switchTo(c PanelConfig, effect string, delay time.Duration) {
	req := switchReq{c: c, effect: effect, at: time.Now().Add(delay)}
	for {
		select {
		case w.switchCh <- req:
			return
		default:
			select {
			case <-w.switchCh:
			default:
			}
		}
	}
}

// applyDevice hands brightness and rotation to the device writer (see
// devWrites): coalesced, rate limited and never written twice.
func (w *worker) applyDevice(c PanelConfig) { w.writeDevice(c.Brightness, c.Rotation) }

// sleepBacklight switches the backlight for screen-off and back. Panels that
// store the brightness in flash are not touched: they get a black picture
// instead, so a screen saver does not wear the flash.
func (w *worker) sleepBacklight(v int) {
	if s, ok := w.p.(panel.SleepByFrame); ok && s.SleepByFrame() {
		return
	}
	w.p.SetBrightness(v)
}

func (w *worker) run(ctx context.Context, c PanelConfig) {
	w.setCurrent(c)
	w.applyDevice(c)
	w.syncOfflineClock()
	sz := w.sizeFor(c.Rotation)
	src, err := w.newSource(c)
	if err != nil {
		w.setErr(ErrContent, err)
		src, _ = w.newSource(PanelConfig{Mode: "off", Rotation: c.Rotation})
	}
	transFrame := time.Second / time.Duration(min(maxFPS(w.p), 30))

	start := time.Now()
	var last *image.RGBA
	var trans struct {
		old    *image.RGBA
		effect string
		start  time.Time
	}
	var pending *switchReq
	var lastLayer func() *image.RGBA // the last frame, when it went out in layers (see layers.go)
	defer func() {
		// A restart (settings change) must not drop a switch that was
		// already taken off the channel, e.g. during a cascade delay.
		if pending != nil {
			select {
			case w.switchCh <- *pending:
			default:
			}
		}
	}()
	sleeping := false
	black := image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y))
	for i := 3; i < len(black.Pix); i += 4 {
		black.Pix[i] = 0xff
	}
	for {
		if black.Bounds().Size() != sz {
			black = image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y))
			for i := 3; i < len(black.Pix); i += 4 {
				black.Pix[i] = 0xff
			}
		}
		w.mu.Lock()
		asleep := w.asleep
		w.mu.Unlock()
		if asleep != sleeping {
			sleeping = asleep
			if sleeping {
				w.sleepBacklight(0)
				w.show(black)
			} else {
				w.sleepBacklight(c.Brightness)
			}
		}
		if sleeping {
			select {
			case <-ctx.Done():
				return
			case <-w.wake:
			case req := <-w.switchCh:
				pending = &req
			case <-time.After(5 * time.Second): // keep the panel from falling back to its own demo
				w.show(black)
			}
			continue
		}
		now := time.Now()
		if pending != nil && !now.Before(pending.at) {
			req := *pending
			pending = nil
			if ns, err := w.newSource(req.c); err != nil {
				w.setErr(ErrContent, err)
			} else {
				if last == nil && lastLayer != nil {
					last = lastLayer() // the picture a transition starts from
				}
				lastLayer = nil
				w.applyDevice(req.c)
				sz = w.sizeFor(req.c.Rotation)
				if last != nil && last.Bounds().Size() != sz {
					last = nil // no transition between different canvas sizes
				}
				if last != nil && req.effect != "" && req.effect != "none" {
					trans.old, trans.effect, trans.start = last, req.effect, now
				}
				src, start, c = ns, now, req.c
				w.setCurrent(c)
			}
		}
		var wait time.Duration
		if lw, compose, layered := w.layeredStep(src, now.Sub(start), trans.old != nil); layered {
			wait, last, lastLayer = lw, nil, compose
		} else {
			lastLayer = nil
			var img *image.RGBA
			img, wait = src.frame(now.Sub(start))
			if trans.old != nil {
				if p := float64(now.Sub(trans.start)) / float64(transitionDuration); p >= 1 {
					trans.old = nil
				} else {
					img, wait = blend(trans.effect, easeInOut(p), trans.old, img), transFrame
				}
			}
			last = img
			var out image.Image = img
			if id, ok := w.identifyFrame(sz, c.Theme); ok {
				out, wait = id, 250*time.Millisecond
			}
			t0 := time.Now()
			w.show(out)
			wait = max(0, wait-time.Since(t0))
		}
		if pending != nil {
			wait = min(wait, max(0, time.Until(pending.at)))
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case req := <-w.switchCh:
			pending = &req
		case <-time.After(wait):
		}
	}
}

// identifyDuration is how long the number stays on the display.
const identifyDuration = 4 * time.Second

// identify shows number n (and the panel's name) on the display briefly.
func (w *worker) identify(n int, name string) {
	w.mu.Lock()
	w.idNum, w.idName, w.idUntil, w.idImg = n, name, time.Now().Add(identifyDuration), nil
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *worker) identifyFrame(sz image.Point, theme string) (image.Image, bool) {
	w.mu.Lock()
	n, name, until, cached := w.idNum, w.idName, w.idUntil, w.idImg
	w.mu.Unlock()
	if time.Now().After(until) {
		return nil, false
	}
	if cached != nil {
		return cached, true
	}
	// A huge black number on white, readable at a glance through the
	// case glass.
	_ = name
	l := lcd.DefaultLayout()
	l.Round = w.round
	l.Panel = ""
	l.Background = "#ffffff"
	l.Widgets = []lcd.Widget{
		{Type: "text", X: 20, Y: 20, W: 440, H: 440, Text: fmt.Sprint(n), Size: 380, Color: "#000000"},
	}
	r, err := lcd.NewRenderer(l.FitTo(sz.X, sz.Y))
	if err != nil {
		return nil, false
	}
	img := r.Render(w.e.prov)
	w.mu.Lock()
	w.idImg = img
	w.mu.Unlock()
	return img, true
}

// SafeProvider wraps a metrics.Provider for concurrent use and refreshes it
// in the background.
type SafeProvider struct {
	mu      sync.RWMutex
	p       metrics.Provider
	lastErr string
	ignore  []string // user list of programs that are not games
}

// NewSafeProvider wraps p.
func NewSafeProvider(p metrics.Provider) *SafeProvider { return &SafeProvider{p: p} }

// Run refreshes every interval until ctx ends.
func (s *SafeProvider) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		s.Refresh()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Refresh implements metrics.Provider.
func (s *SafeProvider) Refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.p.Refresh()
	s.lastErr = ""
	if err != nil {
		s.lastErr = err.Error()
	}
	return err
}

// Swap replaces the underlying provider (settings changed).
func (s *SafeProvider) Swap(p metrics.Provider) {
	s.mu.Lock()
	s.p = p
	if m, ok := p.(interface{ SetGameIgnore([]string) }); ok {
		m.SetGameIgnore(s.ignore)
	}
	s.mu.Unlock()
	s.Refresh()
}

// PresentMonStatus is the FPS backend state ("running", "missing",
// "no-permission", "stopped", "unsupported").
func (s *SafeProvider) PresentMonStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.p.(interface{ PresentMonStatus() string }); ok {
		return m.PresentMonStatus()
	}
	return "unsupported"
}

// SetGameIgnore passes the user's "not a game" list to the FPS detection.
func (s *SafeProvider) SetGameIgnore(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ignore = slices.Clone(names)
	if m, ok := s.p.(interface{ SetGameIgnore([]string) }); ok {
		m.SetGameIgnore(names)
	}
}

// GameName is the executable behind the FPS readings ("" if unknown).
func (s *SafeProvider) GameName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.p.(interface{ GameName() string }); ok {
		return m.GameName()
	}
	return ""
}

// SensorSource is the endpoint currently delivering sensor data.
func (s *SafeProvider) SensorSource() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.p.(interface{ Source() string }); ok {
		return m.Source()
	}
	return ""
}

// Err is the last refresh error ("" when sensors are fine).
func (s *SafeProvider) Err() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastErr
}

// Get implements metrics.Provider.
func (s *SafeProvider) Get(src string) (metrics.Reading, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.p.Get(src)
}

// All implements metrics.Provider.
func (s *SafeProvider) All() []metrics.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]metrics.Reading(nil), s.p.All()...)
}

// setAsleep turns the panel off (brightness 0, black frame) or back on.
func (w *worker) setAsleep(v bool) {
	w.mu.Lock()
	w.asleep = v
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// runIdle turns the LCDs off while the screen saver runs, if enabled.
func (e *Engine) runIdle(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		running := idle.ScreensaverRunning()
		e.mu.Lock()
		e.idleOff = e.cfg.ScreensaverOff && running
		e.mu.Unlock()
		e.syncSleep()
	}
}

// syncSleep puts the displays to sleep or wakes them when the screen saver
// or the API power switch changed what they should do.
func (e *Engine) syncSleep() {
	e.sleepMu.Lock()
	defer e.sleepMu.Unlock()
	e.mu.Lock()
	want := e.idleOff || e.powerOff
	if want == e.asleepNow {
		e.mu.Unlock()
		return
	}
	e.asleepNow = want
	e.setLightingSleep(want)
	ws := make([]*worker, 0, len(e.workers))
	for _, w := range e.workers {
		ws = append(ws, w)
	}
	e.mu.Unlock()
	for _, w := range ws {
		w.setAsleep(want)
	}
}

// naturalLess orders ids with numbers by value, so "tllcd-p2-i0" comes
// before "tllcd-p10-i0" and displays are numbered port by port, fan by fan.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digitsPrefix(a), digitsPrefix(b)
		if da != "" && db != "" {
			na, _ := strconv.Atoi(da)
			nb, _ := strconv.Atoi(db)
			if na != nb {
				return na < nb
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digitsPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// startSettingsCache installs the record of written device settings and keeps
// it in config.json (saved a moment after the last change).
func (e *Engine) startSettingsCache(cfg Config) {
	c := panel.NewSettingsCache(cfg.DeviceWritten)
	var mu sync.Mutex
	var timer *time.Timer
	c.OnChange(func(m map[string]panel.Written) {
		e.mu.Lock()
		e.cfg.DeviceWritten = m
		e.mu.Unlock()
		mu.Lock()
		defer mu.Unlock()
		if timer == nil {
			timer = time.AfterFunc(2*time.Second, func() {
				mu.Lock()
				timer = nil
				mu.Unlock()
				if e.ctx.Err() == nil {
					e.persist()
				}
			})
		}
	})
	panel.SetSettingsCache(c)
}
