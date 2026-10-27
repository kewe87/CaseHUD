package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/panel"
	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/asusaura"
	"github.com/kewe87/casehud/internal/rgb/backend/enram"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
	"github.com/kewe87/casehud/internal/rgb/backend/lianli"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

// Timing of the lighting integration.
const (
	lightRescan       = 30 * time.Second // hot-plug and recovery of failed devices
	lightSaveDelay    = 750 * time.Millisecond
	lightCalibTimeout = 60 * time.Second // a forgotten calibration marker ends by itself
	lightGap          = 10 * time.Second // a pause this long between two ticks means the PC slept
	lightResume       = 3 * time.Second  // wait after wake-up before talking to the devices
	lightTopoTTL      = 60 * time.Second
	maxSensorChoices  = 400
)

// LightingConfig is the lighting part of config.json. Nothing talks to any
// device unless Enabled is set; TL fans additionally need TL.
type LightingConfig struct {
	Enabled bool `json:"enabled,omitempty"`
	// TL turns on the TL fan controller backend (experimental).
	TL bool `json:"tl,omitempty"`
	// Engine is the configuration of the LED engine.
	Engine *leds.Config `json:"engine,omitempty"`
	// Fans is the fan list last read from the controller (arrangement view);
	// it is shown without asking the controller again.
	// Labels are the names the user gave to zones: device ID, zone ID, name.
	// The LED engine knows nothing about them.
	Labels map[string]map[string]string `json:"labels,omitempty"`
	Fans   []FanPos                     `json:"fans,omitempty"`
	FansAt string                       `json:"fansAt,omitempty"` // RFC 3339
}

// lightingState is the runtime side of the lighting integration. The LED
// engine exists only while lighting is enabled (an engine that was stopped
// does not restart cleanly); l.cfg is the master copy while it does not.
//
// Locking: lifeMu orders the slow operations (rescan, blink, calibration take
// it shared; starting and stopping the engine takes it exclusively). mu only
// guards the fields below and is never held across device I/O, so the state
// requests of the GUI never wait for a device. Order: lifeMu, then mu.
type lightingState struct {
	lifeMu  sync.RWMutex
	scanMu  sync.Mutex // one discovery at a time
	blinkMu sync.Mutex // one command at a time to the controller
	mu      sync.Mutex // cfg, eng, tap, calib, err
	eng     *leds.Engine
	tap     *tlTap
	cfg     leds.Config
	err     string // last start/discovery problem
	calib   *LightingCalib
	timer   *time.Timer
	loaded  bool
	stopped bool

	engPtr atomic.Pointer[leds.Engine] // eng for SetPaused without taking mu
	paused atomic.Bool

	saveCh chan struct{}
	done   chan struct{}

	badBackup string // backup of an unusable stored configuration (mu)

	ramNotes ramNotes     // status of the RAM lighting (SMBus bridge)
	bridgeUp atomic.Bool  // the bridge pipe existed at the last probe
	bridgeAt atomic.Int64 // time of that probe (unix nanoseconds)

	cfgErrMu sync.Mutex
	cfgErrs  map[string]error // devices that refused their settings

	traceMu    sync.Mutex // diagnostics recording (diagnostics.go)
	traceUntil time.Time
	traceTimer *time.Timer

	topoMu   sync.Mutex
	topoAt   time.Time // last time the controller was read (zero: never)
	topoBusy bool
	topo     []FanPos
}

// LightingCalib is the zone that shows the calibration marker.
type LightingCalib struct {
	Device string `json:"device"`
	Zone   string `json:"zone"`
	Pos    int    `json:"pos"`
	// Range: the assistant for the LED count lights LEDs 1..Pos instead of one marker.
	Range bool `json:"range,omitempty"`
}

// startLighting loads the stored configuration and, if lighting is enabled,
// starts the engine. It always starts the helper goroutine.
func (e *Engine) startLighting() {
	e.loadLibraryComponents() // stored layouts may name library parts
	l := &e.light
	e.mu.Lock()
	e.cfg.Lighting.Labels = cleanStoredLabels(e.cfg.Lighting.Labels)
	lc := e.cfg.Lighting
	e.mu.Unlock()

	l.lifeMu.Lock()
	l.mu.Lock()
	l.cfg = leds.DefaultConfig()
	if lc.Engine != nil {
		c, err := cleanLightingConfig(*lc.Engine, nil)
		if err != nil {
			log.Printf("lighting: stored configuration ignored: %v", err)
			e.keepBadLightingConfig(*lc.Engine)
			lc.Enabled = false // do not drive devices from a configuration we cannot trust
			e.mu.Lock()
			e.cfg.Lighting.Enabled = false
			e.mu.Unlock()
		} else {
			l.cfg = c
		}
	}
	l.loaded = true
	l.saveCh = make(chan struct{}, 1)
	l.done = make(chan struct{})
	l.paused.Store(e.asleepSnapshot())
	if !e.Demo && e.Topology == nil {
		e.Topology = lightTopology{e}
	}
	l.loadFans(lc)
	cfg := l.cfg
	l.mu.Unlock()
	if lc.Enabled && e.outputAllowed() && e.Acks().AckLighting {
		e.startLightEngine(cfg, lc.TL)
	}
	l.lifeMu.Unlock()
	go e.runLighting(e.ctx, l.saveCh, l.done)
}

// keepBadLightingConfig saves the unusable stored configuration next to
// config.json before the defaults replace it (one file, overwritten). l.mu is held.
func (e *Engine) keepBadLightingConfig(bad leds.Config) {
	l := &e.light
	path := e.Store.path("config.lighting.bad.json")
	b, err := json.MarshalIndent(bad, "", "  ")
	if err == nil {
		err = writeFileAtomic(path, append(b, byte(10)))
	}
	if err != nil {
		log.Printf("lighting: could not back up the stored configuration: %v", err)
		return
	}
	l.badBackup = path
	log.Printf("lighting: the stored configuration was reset, backup: %s", path)
}

func (e *Engine) asleepSnapshot() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.idleOff || e.powerOff
}

// stopLighting stops the engine (last frame: the shutdown colour) and writes
// the configuration. Safe to call more than once.
// resumeLighting starts the engine that startLighting held back until the
// disclaimer was accepted.
func (e *Engine) resumeLighting() {
	l := &e.light
	e.mu.Lock()
	lc := e.cfg.Lighting
	e.mu.Unlock()
	l.lifeMu.Lock()
	defer l.lifeMu.Unlock()
	l.mu.Lock()
	if !lc.Enabled || !l.loaded || l.stopped || l.eng != nil || !e.Acks().AckLighting {
		l.mu.Unlock()
		return
	}
	cfg := l.cfg
	l.mu.Unlock()
	e.startLightEngine(cfg, lc.TL)
}

func (e *Engine) stopLighting() {
	l := &e.light
	l.lifeMu.Lock()
	l.mu.Lock()
	if !l.loaded || l.stopped {
		l.mu.Unlock()
		l.lifeMu.Unlock()
		return
	}
	l.stopped = true
	l.stopTrace()
	eng := e.detachLightEngineLocked()
	l.mu.Unlock()
	stopLightEngine(eng)
	l.lifeMu.Unlock()
	<-l.done // the helper ends with the engine context
	e.saveLighting()
}

// lightBackends lists the device backends that are allowed right now.
func (e *Engine) lightBackends(tl bool) ([]leds.Backend, *tlTap) {
	l := &e.light
	if e.Demo {
		return demoLightBackend(l.setCfgErr), nil
	}
	// The mainboard and the LED frame of the 8.8" display are on whenever
	// lighting is; RAM only through the SMBus bridge (named pipe, fixed name,
	// never the environment override); TL fans need their own switch.
	list := []leds.Backend{
		&cleanNames{inner: asusaura.New()},
		&ringBackend{inner: lianli.New()},
		newRAMBackend(enram.New(smbusBridgePipe), smbusBridgeProbe, &l.ramNotes),
	}
	if tl {
		t := &tlTap{inner: lianlitl.New(), live: map[*tapDevice]struct{}{}, report: l.setCfgErr}
		return append(list, t), t
	}
	return list, nil
}

// startLightEngine builds and starts the engine from cfg and makes it the
// current one. Discovery talks to the devices, so no l.mu is held; the caller
// holds lifeMu exclusively.
func (e *Engine) startLightEngine(cfg leds.Config, tl bool) {
	l := &e.light
	backends, tap := e.lightBackends(tl)
	if len(backends) == 0 {
		l.mu.Lock()
		l.err = "no lighting hardware enabled"
		l.mu.Unlock()
		return
	}
	opt := leds.Options{
		Backends:       backends,
		Effects:        lightingEffects,
		Sensors:        e.lightSensors(),
		RescanInterval: lightRescan,
		Logf:           func(f string, a ...any) { log.Printf(f, a...) },
	}
	if e.Demo {
		opt.ProcessRunning = demoProcessRunning
	}
	eng := leds.New(cfg, opt)
	eng.OnConfigChange(func(leds.Config) { l.markDirty() })
	eng.SetPaused(l.paused.Load())
	errText := ""
	if err := eng.Start(e.ctx); err != nil {
		// Discovery problems are not fatal: the engine runs and rescans.
		errText = softLightErr(err)
		log.Printf("lighting: start: %v", err)
	}
	l.mu.Lock()
	l.err = errText
	l.eng, l.tap = eng, tap
	l.engPtr.Store(eng)
	l.mu.Unlock()
	e.applyDefaultLayouts(eng)
	eng.SetPaused(l.paused.Load()) // a sleep request that came in meanwhile
}

// detachLightEngineLocked takes the engine out of use and keeps its
// configuration; the caller stops it with stopLightEngine after releasing
// l.mu. l.mu is held.
func (e *Engine) detachLightEngineLocked() *leds.Engine {
	l := &e.light
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	l.calib = nil
	eng := l.eng
	if eng == nil {
		return nil
	}
	l.engPtr.Store(nil)
	l.cfg = eng.Config()
	l.eng, l.tap = nil, nil
	return eng
}

// stopLightEngine switches the devices off and releases them (device I/O).
func stopLightEngine(eng *leds.Engine) {
	if eng == nil {
		return
	}
	if err := eng.Stop(); err != nil {
		log.Printf("lighting: stop: %v", err)
	}
}

func (l *lightingState) markDirty() {
	select {
	case l.saveCh <- struct{}{}:
	default:
	}
}

// setLightingSleep pauses the LEDs together with the displays (screen saver,
// power switch) and resumes them with it. It never blocks.
func (e *Engine) setLightingSleep(asleep bool) {
	l := &e.light
	l.paused.Store(asleep)
	if eng := l.engPtr.Load(); eng != nil {
		eng.SetPaused(asleep)
		if asleep {
			eng.EndCalibration() // no marker while the displays sleep
			if l.mu.TryLock() {
				l.calib = nil
				l.mu.Unlock()
			} else {
				go func() { // never block here, but do not leave a stale marker
					l.mu.Lock()
					l.calib = nil
					l.mu.Unlock()
				}()
			}
		}
	}
}

// runLighting saves the configuration (debounced) and notices wake-up from
// standby. save and done belong to this run.
func (e *Engine) runLighting(ctx context.Context, save <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	last := time.Now()
	var resumeAt time.Time
	var saveAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-save:
			saveAt = time.Now().Add(lightSaveDelay)
		case now := <-tick.C:
			if now.Sub(last) >= lightGap {
				resumeAt = now.Add(lightResume)
			}
			last = now
			if !resumeAt.IsZero() && now.After(resumeAt) {
				resumeAt = time.Time{}
				// Device I/O: in its own goroutine, so saving and the tick go on.
				go func() {
					if err := e.rescanLighting(ctx); err != nil && !errors.Is(err, ErrLighting) {
						log.Printf("lighting: rescan after wake-up: %v", err)
					}
				}()
			}
			if !saveAt.IsZero() && now.After(saveAt) {
				saveAt = time.Time{}
				e.saveLighting()
			}
		}
		if !saveAt.IsZero() && time.Now().After(saveAt) {
			saveAt = time.Time{}
			e.saveLighting()
		}
	}
}

// saveLighting writes the engine configuration into config.json.
func (e *Engine) saveLighting() {
	l := &e.light
	l.mu.Lock()
	var c leds.Config
	if l.eng != nil {
		c = l.eng.Config()
	} else {
		c = l.cfg
	}
	l.mu.Unlock()
	e.mu.Lock()
	e.cfg.Lighting.Engine = &c
	e.mu.Unlock()
	if err := e.persist(); err != nil {
		log.Printf("lighting: save: %v", err)
	}
}

// lightSensors serves the sensor values to the effects, rebuilt once a second.
func (e *Engine) lightSensors() func() leds.Sensors {
	var mu sync.Mutex
	var at time.Time
	var cur leds.Sensors
	return func() leds.Sensors {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(at) < time.Second && cur != nil {
			return cur
		}
		at = time.Now()
		m := leds.Sensors{}
		if e.prov != nil {
			for _, r := range e.prov.All() {
				m[r.Key] = r.Value
				m[r.Path] = r.Value
			}
		}
		cur = m
		return m
	}
}

// ---------- API for the GUI and HTTP ----------

// LightingZoneView is one zone as the GUI draws it.
type LightingZoneView struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Count     int            `json:"count"` // LEDs the device reports; resizable zones: the configured number
	Max       int            `json:"max"`
	Resizable bool           `json:"resizable"`
	Order     string         `json:"order"` // effective wire colour order
	LEDs      int            `json:"leds"`  // LEDs with the current layout
	Segments  []leds.Segment `json:"segments"`
	Placed    bool           `json:"placed"` // has a layout of its own (otherwise the default line)
	// Label is the name the user gave the zone ("" = none); Name stays the technical name.
	Label  string       `json:"label,omitempty"`
	Points [][2]float64 `json:"points"` // LED positions on the canvas (0..1)
	// Echo: positions of parallel parts (splitter) that show LEDs already in Points.
	Echo [][2]float64 `json:"echo,omitempty"`
	// Mirrored: the zone shows what a mainboard header sends (TL ports in
	// motherboard mode); MirroredFrom names that header when it is known.
	Mirrored     bool   `json:"mirrored,omitempty"`
	MirroredFrom string `json:"mirroredFrom,omitempty"`
}

// LightingDeviceView is one device as the GUI shows it.
type LightingDeviceView struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Backend         string           `json:"backend"`
	Firmware        string           `json:"firmware,omitempty"`       // as the device reported it, cleaned
	FirmwareLatest  *FirmwareNote    `json:"firmwareLatest,omitempty"` // what the manufacturer offers (firmware check on)
	State           leds.DeviceState `json:"state"`
	Message         string           `json:"message,omitempty"`
	Disabled        bool             `json:"disabled"`
	IgnoreConflicts bool             `json:"ignoreConflicts"`
	Settings        []leds.Param     `json:"settings,omitempty"`
	SettingValues   leds.Params      `json:"settingValues,omitempty"`
	// Warning: the device refused its settings (for example the flash-writing
	// TL modes without consent); Unsupported: found, but never written to.
	Warning     string             `json:"warning,omitempty"`
	Unsupported bool               `json:"unsupported,omitempty"`
	Sync        *LightingSync      `json:"sync,omitempty"` // TL controller: follow a mainboard header
	Zones       []LightingZoneView `json:"zones"`
}

// LightingView is the lighting state as the GUI and the API see it.
type LightingView struct {
	Enabled bool `json:"enabled"`
	// Active: the engine runs (enabled and at least one backend is allowed).
	Active bool `json:"active"`
	TL     bool `json:"tl"`
	// TLAvailable: TL fans can be driven (Windows build, not demo).
	TLAvailable bool `json:"tlAvailable"`
	// RAM: RAM lighting works only through the SMBus bridge of the sensor service.
	RAM        LightingRAM `json:"ram"`
	Paused     bool        `json:"paused"`
	Error      string      `json:"error,omitempty"`
	Brightness int         `json:"brightness"`
	Effect     string      `json:"effect"`
	// Shutdown: what happens to the LEDs on exit: "off", "keep", "device"
	// ("color" when an older configuration holds a fixed colour).
	Shutdown    string               `json:"shutdown"`
	Params      leds.Params          `json:"params"`
	Effects     []LightingEffectView `json:"effects"`
	Devices     []LightingDeviceView `json:"devices"`
	Calibrating *LightingCalib       `json:"calibrating,omitempty"`
	Canvas      [2]int               `json:"canvas"`
	Conflicts   int                  `json:"conflicts"`
	Errors      int                  `json:"errors"`
	// ConfigReset: the stored settings were unusable and have been replaced by
	// the defaults; ConfigBackup is the file that keeps the old ones.
	ConfigReset  bool   `json:"configReset,omitempty"`
	ConfigBackup string `json:"configBackup,omitempty"`
}

// LightingBrief is the short form in the status of the GUI.
type LightingBrief struct {
	Enabled   bool `json:"enabled"`
	Active    bool `json:"active"`
	Conflicts int  `json:"conflicts"`
	Errors    int  `json:"errors"`
	TLActive  bool `json:"tlActive"` // blink buttons of the arrangement view
}

func (e *Engine) lightingFlags() (enabled, tl bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.Lighting.Enabled, e.cfg.Lighting.TL
}

// LightingBrief reports lighting in a few numbers.
func (e *Engine) LightingBrief() LightingBrief {
	enabled, _ := e.lightingFlags()
	l := &e.light
	l.mu.Lock()
	defer l.mu.Unlock()
	b := LightingBrief{Enabled: enabled, Active: l.eng != nil, TLActive: l.eng != nil && l.tap != nil}
	if l.eng != nil {
		for _, d := range l.eng.Devices() {
			switch d.State {
			case leds.StateConflict:
				b.Conflicts++
			case leds.StateError:
				b.Errors++
			}
		}
	}
	return b
}

// Lighting returns the current state.
func (e *Engine) Lighting() LightingView {
	e.refreshBridgeProbe() // before l.mu: it lists the pipe directory
	enabled, tl := e.lightingFlags()
	l := &e.light
	l.mu.Lock()
	defer l.mu.Unlock()
	return e.lightingViewLocked(enabled, tl)
}

func (e *Engine) lightingViewLocked(enabled, tl bool) LightingView {
	l := &e.light
	cfg := l.cfg
	if l.eng != nil {
		cfg = l.eng.Config()
	}
	v := LightingView{Enabled: enabled, Active: l.eng != nil, TL: tl, TLAvailable: !e.Demo, RAM: e.ramView(),
		Paused: l.paused.Load(), Error: l.err, Brightness: cfg.Brightness, Effect: cfg.Effect, Shutdown: shutdownName(cfg.ShutdownColor),
		ConfigReset: l.badBackup != "", ConfigBackup: l.badBackup,
		Calibrating: l.calib, Canvas: [2]int{leds.CanvasWidth, leds.CanvasHeight}, Devices: []LightingDeviceView{}}
	if info, ok := lightingEffect(cfg.Effect); ok {
		v.Params = paramsWithDefaults(info.Params, cfg.Params[cfg.Effect])
	}
	v.Effects = e.lightingEffectList()
	if l.eng == nil {
		return v
	}
	all := l.eng.Devices()
	e.mu.Lock()
	labels := e.cfg.Lighting.Labels // replaced as a whole on change, never modified in place
	e.mu.Unlock()
	for _, d := range all {
		dc := cfg.Devices[d.ID]
		dv := LightingDeviceView{ID: d.ID, Name: d.Name, Backend: d.Backend, Firmware: panel.CleanFirmware(d.Firmware), State: d.State, Message: d.Message,
			Disabled: dc.Disabled, IgnoreConflicts: dc.IgnoreConflicts, Settings: d.Settings, Zones: []LightingZoneView{}}
		if len(d.Settings) > 0 {
			dv.SettingValues = paramsWithDefaults(d.Settings, dc.Settings)
		}
		if d.Backend == "lianlitl" {
			dv.FirmwareLatest = e.firmwareNote(fwProdTLCtrl, dv.Firmware)
		}
		dv.Warning = l.deviceWarning(d.ID)
		dv.Unsupported = strings.HasPrefix(d.ID, unsupportedPref)
		if dv.Unsupported {
			dv.Message = "Nicht unterstützt – wird nicht beschrieben"
		}
		mirror, mirrorFrom := false, ""
		if hasSetting(d.Settings, "syncSource") {
			src, _ := dv.SettingValues["syncSource"].(string)
			mirror = dv.SettingValues.String("mode") == "motherboard"
			mirrorFrom = zoneLabel(all, src)
			dv.Sync = &LightingSync{Source: src, Fans: tlFanCount(d), Choices: syncChoices(all)}
		}
		for _, z := range d.Zones {
			zc := dc.Zones[z.ID]
			zv := LightingZoneView{ID: z.ID, Name: z.Name, Label: labels[d.ID][z.ID], Count: z.Count, Max: z.Max, Resizable: z.Resizable,
				Order: zc.Order, LEDs: leds.LEDCount(z, zc), Segments: append([]leds.Segment{}, zc.Segments...), Placed: len(zc.Segments) > 0}
			if zv.Order == "" {
				zv.Order = z.Order
			}
			if z.Resizable {
				zv.Count = zv.LEDs // the configured number: the header itself reports none
			}
			zv.Mirrored, zv.MirroredFrom = mirror, mirrorFrom
			pts := leds.Positions(z, zc)
			zv.Echo = echoPoints(zc.Segments)
			zv.Points = make([][2]float64, 0, min(len(pts), 512))
			step := 1
			if len(pts) > 512 {
				step = (len(pts) + 511) / 512
			}
			for i := 0; i < len(pts); i += step {
				zv.Points = append(zv.Points, [2]float64{round3(pts[i].X), round3(pts[i].Y)})
			}
			dv.Zones = append(dv.Zones, zv)
		}
		switch d.State {
		case leds.StateConflict:
			v.Conflicts++
		case leds.StateError:
			v.Errors++
		}
		v.Devices = append(v.Devices, dv)
	}
	return v
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }

func paramsWithDefaults(decl []leds.Param, p leds.Params) leds.Params {
	out := leds.Params{}
	for _, d := range decl {
		out[d.Key] = d.Default
	}
	for k, v := range p {
		out[k] = v
	}
	return out
}

// lightingFeatured is the one place that says which effects the GUI offers
// (in this order), with their names in both GUI languages. Every other effect
// stays valid for the API, stored configurations and validation; the GUI shows
// it as "more" when it is in use.
var lightingFeatured = []struct{ name, de, en string }{
	{"solid", "Einfarbig", "Solid color"},
	{"breathing", "Atmen", "Breathing"},
	{"rainbow", "Regenbogen", "Rainbow"},
	{"cycle", "Farbwechsel", "Color cycle"},
	{"wave", "Farbwelle", "Color wave"},
	{"sensor", "Temperatur", "Temperature"},
	{"meteor", "Meteor", "Meteor"},
}

func lightingIsFeatured(name string) bool {
	for _, f := range lightingFeatured {
		if f.name == name {
			return true
		}
	}
	return false
}

// LightingEffectView is one effect as the GUI and API v1 list it. Label is the
// English GUI name for featured effects; LabelDE the German one.
type LightingEffectView struct {
	leds.EffectInfo
	Featured bool   `json:"featured"`
	LabelDE  string `json:"labelDe,omitempty"`
	// Params shadows EffectInfo.Params: the descriptors plus labelDe and step.
	Params []LightingParamView `json:"params"`
}

// LightingParamView is a parameter descriptor with the additive fields of API
// v1: LabelDE (German GUI name, featured effects only) and Step (number
// parameters: increment for sliders and spin boxes).
type LightingParamView struct {
	leds.Param
	LabelDE string  `json:"labelDe,omitempty"`
	Step    float64 `json:"step,omitempty"`
}

// lightingParamDE holds the German names of the parameters of the featured
// effects: "effect.key" first, then "key".
var lightingParamDE = map[string]string{
	"color": "Farbe", "useAccent": "Akzentfarbe des Designs", "background": "Hintergrund",
	"speed": "Tempo", "width": "Breite", "period": "Dauer (s)", "scale": "Breite", "vertical": "Senkrecht",
	"sensor": "Sensor", "sensor.min": "Wert für untere Farbe", "sensor.max": "Wert für obere Farbe",
	"sensor.low": "Untere Farbe", "sensor.high": "Obere Farbe", "smoothing": "Glättung",
	"colors": "Genutzte Farben", "path": "Verlauf", "mirror": "Zur Mitte spiegeln", "reverse": "Richtung umkehren",
	"tail":   "Schweiflänge",
	"color1": "Farbe 1", "color2": "Farbe 2", "color3": "Farbe 3", "color4": "Farbe 4", "color5": "Farbe 5", "color6": "Farbe 6",
}

// lightingParamViews wraps the descriptors of one effect.
func lightingParamViews(effect string, ps []leds.Param, featured bool) []LightingParamView {
	out := make([]LightingParamView, len(ps))
	for i, p := range ps {
		v := LightingParamView{Param: p}
		if featured {
			if de, ok := lightingParamDE[effect+"."+p.Key]; ok {
				v.LabelDE = de
			} else {
				v.LabelDE = lightingParamDE[p.Key]
			}
		}
		if p.Type == leds.ParamNumber {
			v.Step = 1
			whole := func(f float64) bool { return f == math.Trunc(f) }
			if d, ok := p.Default.(float64); !whole(p.Min) || !whole(p.Max) || (ok && !whole(d)) {
				v.Step = 0.1
			}
		}
		out[i] = v
	}
	return out
}

// cpuTempPath finds the CPU temperature sensor for the sensor effect's
// preset ("" when there is none).
func (e *Engine) cpuTempPath() string {
	if e.prov == nil {
		return ""
	}
	for _, alt := range strings.Split(srcCPUTemp, "|") {
		for _, r := range e.prov.All() {
			if r.Unit == "°C" && (r.Path == alt || strings.HasSuffix(r.Path, "/"+alt)) {
				return r.Path
			}
		}
	}
	return ""
}

// lightingEffectList lists the effects; the sensor effect gets the sensor
// names as choices (display only, any text is accepted) and the CPU
// temperature as its default sensor.
func (e *Engine) lightingEffectList() []LightingEffectView {
	out := make([]LightingEffectView, len(lightingEffects))
	for i, info := range lightingEffects {
		out[i] = LightingEffectView{EffectInfo: info}
		out[i].Params = lightingParamViews(info.Name, info.Params, lightingIsFeatured(info.Name))
		for _, f := range lightingFeatured {
			if f.name == info.Name {
				out[i].Featured, out[i].Label, out[i].LabelDE = true, f.en, f.de
			}
		}
		if info.Name != "sensor" {
			continue
		}
		ps := append([]leds.Param(nil), info.Params...)
		cpu := e.cpuTempPath()
		for j, p := range ps {
			if p.Key != "sensor" || e.prov == nil {
				continue
			}
			seen := map[string]bool{}
			for _, r := range e.prov.All() {
				if len(ps[j].Values) >= maxSensorChoices {
					break
				}
				if r.Path != "" && !seen[r.Path] && (r.Unit == "°C" || r.Unit == "%" || r.Unit == "RPM" || r.Unit == "W") {
					seen[r.Path] = true
					ps[j].Values = append(ps[j].Values, r.Path)
				}
			}
			if cpu != "" {
				ps[j].Default = cpu
				if !seen[cpu] {
					ps[j].Values = append(ps[j].Values, cpu)
				}
			}
		}
		out[i].Params = lightingParamViews(info.Name, ps, true)
	}
	rank := func(n string) int {
		for i, f := range lightingFeatured {
			if f.name == n {
				return i
			}
		}
		return len(lightingFeatured)
	}
	sort.SliceStable(out, func(a, b int) bool { return rank(out[a].Name) < rank(out[b].Name) })
	return out
}

// LightingPatch changes lighting; only fields that are set change anything.
type LightingPatch struct {
	Enabled    *bool   `json:"enabled"`
	TL         *bool   `json:"tl"`
	Brightness *int    `json:"brightness"`
	Effect     *string `json:"effect"`
	// Shutdown: "off", "keep" or "device".
	Shutdown *string                        `json:"shutdown"`
	Params   map[string]any                 `json:"params"`
	Devices  map[string]LightingDevicePatch `json:"devices"`
}

// LightingDevicePatch changes one device.
type LightingDevicePatch struct {
	Disabled        *bool                        `json:"disabled"`
	IgnoreConflicts *bool                        `json:"ignoreConflicts"`
	Settings        map[string]any               `json:"settings"`
	Zones           map[string]LightingZonePatch `json:"zones"`
}

// LightingZonePatch changes one zone: Count (resizable zones, one straight
// line), or Order, or the full Segments list.
type LightingZonePatch struct {
	// Label names the zone ("" removes the name); up to 40 printable characters.
	Label    *string         `json:"label"`
	Order    *string         `json:"order"`
	Count    *int            `json:"count"`
	Segments *[]leds.Segment `json:"segments"`
}

// ErrLighting marks a rejected request (bad input), as opposed to a failure.
var ErrLighting = errors.New("lighting")

func lightErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrLighting, fmt.Sprintf(format, a...))
}

// PatchLighting validates the whole request first and then applies it; a
// rejected request changes nothing.
func (e *Engine) PatchLighting(p LightingPatch) (LightingView, error) {
	l := &e.light
	if len(p.Devices) > maxLightDevices || len(p.Params) > 64 {
		return LightingView{}, lightErr("request too large")
	}
	if (p.Enabled != nil && *p.Enabled || p.TL != nil && *p.TL) && !e.outputAllowed() {
		return LightingView{}, fmt.Errorf("%w: accept the disclaimer in the CaseHUD window first", ErrAckRequired)
	}
	if p.Enabled != nil && *p.Enabled && !e.Acks().AckLighting {
		return LightingView{}, fmt.Errorf("%w: confirm the lighting notice in the CaseHUD window first", ErrAckRequired)
	}
	if p.TL != nil && e.Demo && *p.TL {
		return LightingView{}, lightErr("TL fans are not available in the demo")
	}

	// Changes that only touch the configuration run next to a rescan; turning
	// lighting on or off or switching the backend needs the engine to itself.
	exclusive := false
	l.lifeMu.RLock()
	for {
		enabled, tl := e.lightingFlags()
		wantEnabled, wantTL := enabled, tl
		if p.Enabled != nil {
			wantEnabled = *p.Enabled
		}
		if p.TL != nil {
			wantTL = *p.TL
		}
		l.mu.Lock()
		running := l.eng != nil
		need := (running && (!wantEnabled || wantTL != tl)) || (wantEnabled && !running)
		if need && !exclusive {
			l.mu.Unlock()
			l.lifeMu.RUnlock()
			l.lifeMu.Lock()
			exclusive = true
			continue
		}
		v, err := e.patchLightingLocked(p, wantEnabled, wantTL, need)
		if exclusive {
			l.lifeMu.Unlock()
		} else {
			l.lifeMu.RUnlock()
		}
		return v, err
	}
}

// patchLightingLocked runs with l.mu held (and lifeMu in the right mode) and
// releases l.mu before it talks to a device. need: the engine is started,
// stopped or restarted.
func (e *Engine) patchLightingLocked(p LightingPatch, wantEnabled, wantTL, need bool) (LightingView, error) {
	l := &e.light
	locked := true
	defer func() {
		if locked {
			l.mu.Unlock()
		}
	}()

	cur := l.cfg
	var devs []leds.DeviceStatus
	if l.eng != nil {
		cur = l.eng.Config()
		devs = l.eng.Devices()
	}
	next := cloneLightCfg(cur)
	var newLabels map[string]map[string]string // set when a label changes
	if p.Brightness != nil {
		if *p.Brightness < 0 || *p.Brightness > 100 {
			return LightingView{}, lightErr("brightness must be 0..100")
		}
		next.Brightness = *p.Brightness
	}
	if p.Shutdown != nil {
		switch *p.Shutdown {
		case "off":
			next.ShutdownColor = ""
		case "keep":
			next.ShutdownColor = leds.ShutdownKeep
		case "device":
			next.ShutdownColor = leds.ShutdownDevice
		default:
			return LightingView{}, lightErr("shutdown must be off, keep or device")
		}
	}
	effect := next.Effect
	if p.Effect != nil {
		if _, ok := lightingEffect(*p.Effect); !ok {
			return LightingView{}, lightErr("unknown effect %q", *p.Effect)
		}
		effect = *p.Effect
		next.Effect = effect
	}
	if len(p.Params) > 0 {
		info, _ := lightingEffect(effect)
		cp, err := cleanParams(info.Params, p.Params)
		if err != nil {
			return LightingView{}, lightErr("%v", err)
		}
		merged := paramsWithDefaults(info.Params, next.Params[effect])
		for k, v := range cp {
			merged[k] = v
		}
		next.Params[effect] = merged
	}
	settingsOf := map[string]leds.DeviceStatus{}
	for _, d := range devs {
		settingsOf[d.ID] = d
	}
	for id, dp := range p.Devices {
		ds, known := settingsOf[id]
		if !known {
			return LightingView{}, lightErr("device %q not found", id)
		}
		dc := next.Devices[id]
		if dp.Disabled != nil {
			dc.Disabled = *dp.Disabled
		}
		if dp.IgnoreConflicts != nil {
			dc.IgnoreConflicts = *dp.IgnoreConflicts
		}
		if len(dp.Settings) > 0 {
			cp, err := cleanParams(ds.Settings, dp.Settings)
			if err != nil {
				return LightingView{}, lightErr("%s: %v", id, err)
			}
			merged := leds.Params{}
			for k, v := range dc.Settings {
				merged[k] = v
			}
			for k, v := range cp {
				merged[k] = v
			}
			dc.Settings = merged
		}
		zones := map[string]leds.ZoneConfig{}
		for k, v := range dc.Zones {
			zones[k] = v
		}
		for zid, zp := range dp.Zones {
			var zone *leds.Zone
			for i := range ds.Zones {
				if ds.Zones[i].ID == zid {
					zone = &ds.Zones[i]
				}
			}
			if zone == nil {
				return LightingView{}, lightErr("%s: zone %q not found", id, zid)
			}
			zc := zones[zid]
			if zp.Label != nil {
				lab, err := cleanLabel(*zp.Label)
				if err != nil {
					return LightingView{}, lightErr("%s/%s: %v", id, zid, err)
				}
				if newLabels == nil {
					e.mu.Lock()
					newLabels = copyLabels(e.cfg.Lighting.Labels)
					e.mu.Unlock()
				}
				if newLabels[id] == nil {
					newLabels[id] = map[string]string{}
				}
				if lab == "" {
					delete(newLabels[id], zid)
				} else {
					newLabels[id][zid] = lab
				}
				if len(newLabels[id]) == 0 {
					delete(newLabels, id)
				}
				if countLabels(newLabels) > maxLightDevices*maxLightZones {
					return LightingView{}, lightErr("too many names")
				}
			}
			if zp.Order != nil {
				zc.Order = *zp.Order
			}
			if zp.Count != nil {
				if !zone.Resizable {
					return LightingView{}, lightErr("%s/%s: the LED count of this zone is fixed", id, zid)
				}
				if len(zc.Segments) > 1 || (len(zc.Segments) == 1 && zc.Segments[0].Shape != "line" && zc.Segments[0].Shape != "") {
					return LightingView{}, lightErr("%s/%s: the zone has a custom layout; send segments instead", id, zid)
				}
				limit := maxSegmentLEDs
				if zone.Max > 0 {
					limit = min(limit, zone.Max)
				}
				if *zp.Count < 0 || *zp.Count > limit {
					return LightingView{}, lightErr("%s/%s: count must be 0..%d", id, zid, limit)
				}
				if *zp.Count == 0 {
					zc.Segments = nil
				} else {
					zc.Segments = []leds.Segment{{Name: zone.Name, Count: *zp.Count, Shape: "line", X: 0.05, Y: 0.5, W: 0.9}}
				}
			}
			if zp.Segments != nil {
				zc.Segments = *zp.Segments
			}
			zones[zid] = zc
		}
		if len(zones) > 0 {
			dc.Zones = zones
		}
		next.Devices[id] = dc
	}
	descr := map[string][]leds.Param{}
	for _, d := range devs {
		if len(d.Settings) > 0 {
			descr[d.ID] = d.Settings
		}
	}
	clean, err := cleanLightingConfig(next, descr)
	if err != nil {
		return LightingView{}, lightErr("%v", err)
	}
	if p.Devices != nil {
		// A zone may not hold more LEDs than the hardware takes.
		for id := range p.Devices {
			for _, z := range settingsOf[id].Zones {
				zc := clean.Devices[id].Zones[z.ID]
				if z.Max > 0 && len(zc.Segments) > 0 && leds.LEDCount(z, zc) > z.Max {
					return LightingView{}, lightErr("%s/%s: at most %d LEDs", id, z.ID, z.Max)
				}
			}
		}
	}

	// Apply: first the configuration, then the on/off transitions.
	if l.eng != nil {
		if err := l.eng.SetEffect(clean.Effect, clean.Params[clean.Effect]); err != nil {
			return LightingView{}, lightErr("%v", err)
		}
		l.eng.SetBrightness(clean.Brightness)
		if p.Shutdown != nil {
			l.eng.SetShutdown(clean.ShutdownColor) // read by the engine when it stops
		}
		// A device is only touched when its configuration changed: a
		// controller stores every command in flash.
		running := l.eng.Config().Devices
		for id := range p.Devices {
			if cur, ok := running[id]; ok && reflect.DeepEqual(cur, clean.Devices[id]) {
				continue
			}
			l.eng.SetDeviceConfig(id, clean.Devices[id])
		}
		clean = l.eng.Config()
	}
	l.cfg = clean
	e.mu.Lock()
	e.cfg.Lighting.Enabled, e.cfg.Lighting.TL = wantEnabled, wantTL
	if newLabels != nil {
		e.cfg.Lighting.Labels = newLabels
	}
	e.mu.Unlock()
	if need {
		old := e.detachLightEngineLocked()
		cfg := l.cfg
		l.mu.Unlock()
		locked = false
		stopLightEngine(old) // device I/O outside l.mu
		if wantEnabled {
			e.startLightEngine(cfg, wantTL)
		}
		l.mu.Lock()
		locked = true
	}
	l.markDirty()
	return e.lightingViewLocked(wantEnabled, wantTL), nil
}

// shutdownName is the API name of a stored shutdown setting.
func shutdownName(v string) string {
	switch v {
	case "":
		return "off"
	case leds.ShutdownKeep, leds.ShutdownDevice:
		return v
	}
	return "color"
}

func cloneLightCfg(c leds.Config) leds.Config {
	out := c
	out.Params = map[string]leds.Params{}
	for k, v := range c.Params {
		p := leds.Params{}
		for pk, pv := range v {
			p[pk] = pv
		}
		out.Params[k] = p
	}
	out.Devices = map[string]leds.DeviceConfig{}
	for k, v := range c.Devices {
		d := v
		d.Settings = nil
		if v.Settings != nil {
			d.Settings = leds.Params{}
			for sk, sv := range v.Settings {
				d.Settings[sk] = sv
			}
		}
		d.Zones = map[string]leds.ZoneConfig{}
		for zk, zv := range v.Zones {
			z := zv
			z.Segments = append([]leds.Segment(nil), zv.Segments...)
			d.Zones[zk] = z
		}
		out.Devices[k] = d
	}
	return out
}

// RescanLighting looks for lighting hardware again.
func (e *Engine) RescanLighting() error {
	return e.rescanLighting(e.ctx)
}

// rescanLighting runs discovery without holding l.mu, so the GUI keeps its
// state while the devices answer.
func (e *Engine) rescanLighting(parent context.Context) error {
	l := &e.light
	l.lifeMu.RLock()
	defer l.lifeMu.RUnlock()
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return lightErr("lighting is off")
	}
	l.scanMu.Lock()
	defer l.scanMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	err := eng.Rescan(ctx)
	l.mu.Lock()
	if l.eng == eng {
		l.err = ""
		if err != nil {
			l.err = softLightErr(err)
		}
	}
	l.mu.Unlock()
	e.applyDefaultLayouts(eng)
	return nil
}

// LightingCalibrate shows the calibration marker at LED position pos of a
// zone (device, zone and position are checked by the engine).
func (e *Engine) LightingCalibrate(device, zone string, pos int) error {
	return e.lightingMarker(device, zone, LightingCalib{Device: device, Zone: zone, Pos: pos}, lightCalibTimeout,
		func(eng *leds.Engine) error { return eng.Calibrate(device, zone, pos) })
}

// lightingMarker shows a calibration marker (single marker or LED range) and
// remembers it for the view. The marker ends by itself after timeout.
func (e *Engine) lightingMarker(device, zone string, c LightingCalib, timeout time.Duration, show func(*leds.Engine) error) error {
	if !lightIDRe.MatchString(device) || !lightIDRe.MatchString(zone) {
		return lightErr("invalid device or zone")
	}
	l := &e.light
	l.lifeMu.RLock()
	defer l.lifeMu.RUnlock()
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng == nil {
		return lightErr("lighting is off")
	}
	for _, d := range eng.Devices() {
		if d.ID == device && d.State != leds.StateOK {
			return lightErr("the device is not active (%s)", d.State)
		}
	}
	if err := show(eng); err != nil {
		return lightErr("%v", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.eng != eng {
		return nil
	}
	l.calib = &c
	if l.timer != nil {
		l.timer.Stop()
	}
	l.timer = time.AfterFunc(timeout, func() {
		l.mu.Lock()
		cur := l.eng == eng
		if cur {
			l.calib = nil
		}
		l.mu.Unlock()
		if cur {
			eng.EndCalibration()
		}
	})
	return nil
}

// LightingEndCalibration returns to the effect.
func (e *Engine) LightingEndCalibration() {
	l := &e.light
	l.mu.Lock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	l.calib = nil
	eng := l.eng
	l.mu.Unlock()
	if eng != nil {
		eng.EndCalibration()
	}
}

// LightingBlink makes the fans of one controller port (1-4) blink. It needs
// lighting and the TL backend switched on and the controller not in use by
// another program.
func (e *Engine) LightingBlink(port int) error {
	if port < 1 || port > ControllerPorts {
		return lightErr("port must be 1..%d", ControllerPorts)
	}
	l := &e.light
	l.lifeMu.RLock()
	defer l.lifeMu.RUnlock()
	l.mu.Lock()
	eng, tap := l.eng, l.tap
	l.mu.Unlock()
	if eng == nil || tap == nil {
		return lightErr("TL fan lighting is off")
	}
	for _, d := range eng.Devices() {
		if d.Backend == "lianlitl" && d.State != leds.StateOK {
			return lightErr("the fan controller is not active (%s)", d.State)
		}
	}
	l.blinkMu.Lock()
	defer l.blinkMu.Unlock()
	return tap.blink(port - 1) // device I/O: no l.mu
}

// ---------- topology ----------

// queryTopology is the one-shot read of the controller's fan list (A1/A6,
// read only). A variable so tests can count the calls.
var queryTopology = lianlitl.QueryTopology

const (
	maxKnownFans   = 64
	maxFanIndex    = 31
	topoQueryLimit = 10 * time.Second
)

// lightTopology reports the fans of the TL controller for the arrangement
// view without talking to the controller on its own: it serves the list that
// was read last (kept in config.json). Only while lighting and the TL backend
// run does it refresh that list through the engine's own handle; otherwise
// the list changes only when the user asks (ScanFans).
type lightTopology struct{ e *Engine }

func (t lightTopology) Fans() []FanPos {
	l := &t.e.light
	l.mu.Lock()
	tap := l.tap
	l.mu.Unlock()
	l.topoMu.Lock()
	defer l.topoMu.Unlock()
	if tap != nil && !l.topoBusy && time.Since(l.topoAt) >= lightTopoTTL {
		l.topoBusy = true
		go t.e.refreshTopology(tap)
	}
	return append([]FanPos(nil), l.topo...)
}

// ScanInfo tells the arrangement view what the list is and whether it can be
// read.
func (t lightTopology) ScanInfo() ArrangeScan {
	l := &t.e.light
	l.mu.Lock()
	live := l.tap != nil
	l.mu.Unlock()
	l.topoMu.Lock()
	defer l.topoMu.Unlock()
	s := ArrangeScan{Available: !t.e.Demo, Live: live, Busy: l.topoBusy}
	if !l.topoAt.IsZero() {
		s.At = l.topoAt.UTC().Format(time.RFC3339)
	}
	return s
}

// loadFans takes the stored fan list from the config. l.mu is held.
func (l *lightingState) loadFans(lc LightingConfig) {
	var fans []FanPos
	for _, f := range lc.Fans {
		if len(fans) >= maxKnownFans {
			break
		}
		if f.Port < 1 || f.Port > ControllerPorts || f.Index < 0 || f.Index > maxFanIndex {
			continue
		}
		fans = append(fans, FanPos{Port: f.Port, Index: f.Index, HasLCD: f.HasLCD})
	}
	var at time.Time
	if t, err := time.Parse(time.RFC3339, lc.FansAt); err == nil && !t.After(time.Now().Add(time.Hour)) {
		at = t
	}
	l.topoMu.Lock()
	l.topo, l.topoAt = fans, at
	l.topoMu.Unlock()
}

// readTopology reads the fan list once: through the engine's handle if it has
// one, else (fallback) with query. It returns the fans of the first controller.
func readTopology(tap *tlTap, query func(context.Context) ([]lianlitl.Topology, error)) (fans []FanPos, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	var topos []lianlitl.Topology
	if tap != nil {
		var t lianlitl.Topology
		if t, err = tap.topology(); err == nil {
			topos = []lianlitl.Topology{t}
		} else if query == nil {
			return nil, err
		}
	}
	if topos == nil && query != nil {
		ctx, cancel := context.WithTimeout(context.Background(), topoQueryLimit)
		defer cancel()
		if topos, err = query(ctx); err != nil {
			return nil, err
		}
	}
	for _, t := range topos {
		for _, f := range t.Fans {
			if len(fans) >= maxKnownFans {
				break
			}
			fans = append(fans, FanPos{Port: f.Port, Index: f.Index - 1, HasLCD: f.LCD})
		}
		break // one controller
	}
	return fans, nil
}

// storeTopology keeps a fresh result in memory and in the config.
func (e *Engine) storeTopology(fans []FanPos) {
	l := &e.light
	now := time.Now()
	l.topoMu.Lock()
	l.topo, l.topoAt = fans, now
	l.topoMu.Unlock()
	e.mu.Lock()
	e.cfg.Lighting.Fans = append([]FanPos(nil), fans...)
	e.cfg.Lighting.FansAt = now.UTC().Format(time.RFC3339)
	e.mu.Unlock()
	if err := e.persist(); err != nil {
		log.Printf("lighting: save fan list: %v", err)
	}
}

// refreshTopology re-reads the list through the engine's own handle (lighting
// and TL are on). It never opens the controller itself.
func (e *Engine) refreshTopology(tap *tlTap) {
	l := &e.light
	fans, err := readTopology(tap, nil)
	if err != nil {
		log.Printf("lighting: topology: %v", err)
		l.topoMu.Lock()
		l.topoAt, l.topoBusy = time.Now(), false // try again after the TTL
		l.topoMu.Unlock()
		return
	}
	e.storeTopology(fans)
	l.topoMu.Lock()
	l.topoBusy = false
	l.topoMu.Unlock()
}

// ScanFans is the user's "look for fans" action: it reads the fan list from
// the controller once (read only, changes nothing) and keeps it.
func (e *Engine) ScanFans() error {
	if e.Demo {
		return lightErr("not available in the demo")
	}
	return e.scanFans(queryTopology)
}

func (e *Engine) scanFans(query func(context.Context) ([]lianlitl.Topology, error)) error {
	l := &e.light
	l.lifeMu.RLock() // the engine is not started or stopped meanwhile
	defer l.lifeMu.RUnlock()
	l.topoMu.Lock()
	if l.topoBusy {
		l.topoMu.Unlock()
		return lightErr("the fan list is being read already")
	}
	l.topoBusy = true
	l.topoMu.Unlock()
	defer func() {
		l.topoMu.Lock()
		l.topoBusy = false
		l.topoMu.Unlock()
	}()
	l.mu.Lock()
	tap := l.tap
	l.mu.Unlock()
	fans, err := readTopology(tap, query)
	if err != nil {
		return err
	}
	e.storeTopology(fans)
	return nil
}

// ---------- TL backend wrapper ----------

// tlTap wraps the TL backend and remembers its open devices, so topology and
// the blink command use the handle the engine already holds instead of
// opening the controller a second time.
type tlTap struct {
	inner  leds.Backend
	mu     sync.Mutex
	live   map[*tapDevice]struct{}
	report func(id string, err error) // why the controller refused its settings
}

func (t *tlTap) Name() string { return t.inner.Name() }

func (t *tlTap) Discover(ctx context.Context) ([]leds.Device, error) {
	devs, err := t.inner.Discover(ctx)
	out := make([]leds.Device, 0, len(devs))
	for _, d := range devs {
		td := &tapDevice{Device: d, tap: t}
		t.mu.Lock()
		t.live[td] = struct{}{}
		t.mu.Unlock()
		out = append(out, td)
	}
	return out, err
}

type tlControls interface {
	Topology() (lianlitl.Topology, error)
	BlinkPort(port int) error
}

func (t *tlTap) first() (*tapDevice, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for d := range t.live {
		return d, true
	}
	return nil, false
}

func (t *tlTap) topology() (lianlitl.Topology, error) {
	d, ok := t.first()
	if !ok {
		return lianlitl.Topology{}, errors.New("no controller open")
	}
	return d.topology()
}

func (t *tlTap) blink(port int) error {
	d, ok := t.first()
	if !ok {
		return lightErr("no fan controller found")
	}
	return d.blink(port)
}

// tapDevice forwards to the TL device and refuses calls after Close, so a
// handle value that Windows may reuse is never touched.
type tapDevice struct {
	leds.Device
	tap    *tlTap
	mu     sync.RWMutex
	closed bool
}

func (d *tapDevice) Settings() []leds.Param {
	if c, ok := d.Device.(leds.Configurable); ok {
		return c.Settings()
	}
	return nil
}

// Configure re-checks the stored settings against the device's own
// descriptors (numbers clamped, enums checked) before the driver sees them, so
// a hand-edited config.json cannot make the driver write faster or more often
// than its limits.
func (d *tapDevice) Configure(p leds.Params) error {
	c, ok := d.Device.(leds.Configurable)
	if !ok {
		return nil
	}
	decl := c.Settings()
	clean, err := cleanParams(decl, migrateLightSettings(p))
	if err == nil {
		err = c.Configure(paramsWithDefaults(decl, clean))
	}
	if d.tap.report != nil {
		d.tap.report(d.Device.ID(), err)
	}
	return err
}

func (d *tapDevice) topology() (lianlitl.Topology, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.Device.(tlControls)
	if d.closed || !ok {
		return lianlitl.Topology{}, errors.New("controller closed")
	}
	return c.Topology()
}

func (d *tapDevice) blink(port int) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.Device.(tlControls)
	if d.closed || !ok {
		return lightErr("the fan controller is closed")
	}
	return c.BlinkPort(port)
}

func (d *tapDevice) Close() error {
	d.mu.Lock()
	already := d.closed
	d.closed = true
	d.mu.Unlock()
	d.tap.mu.Lock()
	delete(d.tap.live, d)
	d.tap.mu.Unlock()
	if already {
		return nil
	}
	return d.Device.Close()
}

// ---------- demo hardware ----------

// demoLightBackend returns simulated lighting hardware: a mainboard with a
// resizable header, RAM, a fan controller and a keyboard that another RGB
// program (simulated) is driving.
func demoLightBackend(report func(id string, err error)) []leds.Backend {
	fans := &fake.Device{DeviceID: "demo:fans", DeviceName: "Demo fan controller", ZoneList: []leds.Zone{
		{ID: "port2", Name: "Port 2", Count: 2, Max: 2, Order: "RGB"},
		{ID: "port3", Name: "Port 3", Count: 6, Max: 6, Order: "RGB"},
	}}
	board := &fake.Backend{Devices: []*fake.Device{
		{DeviceID: "demo:board", DeviceName: "Demo mainboard", ZoneList: []leds.Zone{
			{ID: "onboard", Name: "Onboard", Count: 8, Max: 8, Order: "RGB"},
			{ID: "argb1", Name: "ARGB header 1", Max: 120, Resizable: true, Order: "GRB"},
			{ID: "argb2", Name: "ARGB header 2", Max: 120, Resizable: true, Order: "GRB"},
		}, Conflict: []string{"DemoBoardTool.exe"}}}}
	ram := &fake.Backend{Devices: []*fake.Device{
		{DeviceID: "demo:ram", DeviceName: "Demo RAM", ZoneList: []leds.Zone{
			{ID: "stick1", Name: "Stick 1", Count: 5, Max: 5, Order: "RGB"},
			{ID: "stick2", Name: "Stick 2", Count: 5, Max: 5, Order: "RGB"},
		}}}}
	fanCtl := &fake.Backend{Devices: []*fake.Device{fans}}
	other := &fake.Backend{Devices: []*fake.Device{
		{DeviceID: "demo:keyboard", DeviceName: "Demo keyboard", ZoneList: []leds.Zone{
			{ID: "keys", Name: "Keys", Count: 20, Max: 20, Order: "RGB"},
		}, Conflict: []string{"DemoRGB.exe"}},
	}}
	ctl := &demoController{Device: fans, mode: "motherboard", report: report}
	// The names are those of the real backends, so the view treats the demo
	// devices like the real ones (mainboard headers, fan controller).
	return []leds.Backend{
		&demoBackend{inner: board, name: boardBackend},
		&demoBackend{inner: ram, name: ramBackend},
		&demoBackend{inner: fanCtl, name: "lianlitl", ctl: ctl},
		&demoBackend{inner: other, name: "fake"},
	}
}

// demoProcessRunning simulates one other RGB program for the demo.
func demoProcessRunning(names []string) []string {
	var out []string
	for _, n := range names {
		if strings.EqualFold(n, "DemoRGB.exe") {
			out = append(out, n)
		}
	}
	return out
}
