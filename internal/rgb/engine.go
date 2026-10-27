package leds

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/rgb/internal/diag"
	"github.com/kewe87/casehud/internal/rgb/internal/proc"
)

// conflictCheckInterval is how often running processes are checked against
// Device.Conflicts.
const conflictCheckInterval = 2 * time.Second

// Options configure an Engine.
type Options struct {
	Backends []Backend
	// Effects available to SetEffect; nil means BuiltinEffects().
	Effects []EffectInfo
	// Sensors is polled once per frame; nil means no sensor data.
	Sensors func() Sensors
	// RescanInterval enables periodic discovery for hot-plug; 0 = only Rescan().
	RescanInterval time.Duration
	// ProcessRunning reports whether a process with that executable name runs;
	// nil uses the operating system process list.
	ProcessRunning func(names []string) []string
	// Logf receives diagnostic messages; nil discards them.
	Logf func(format string, args ...any)
}

// Engine renders the active effect and drives all devices. All methods are
// safe for concurrent use.
type Engine struct {
	opt     Options
	effects map[string]EffectInfo

	mu          sync.Mutex
	cfg         Config
	onChange    func(Config)
	paused      bool
	accent      color.RGBA
	effectDirty bool
	layoutDirty bool
	calib       *calibration
	devices     []*deviceEntry
	running     bool
	cancel      context.CancelFunc
	done        chan struct{}

	discovered chan discovery
}

type discovery struct {
	devices []backendDevice
	errs    []error
	failed  map[string]bool // backends whose discovery failed: keep their devices
	applied chan struct{}
}

type backendDevice struct {
	backend string
	dev     Device
}

type deviceEntry struct {
	dev     Device
	backend string
	state   DeviceState
	msg     string

	// owned by the engine goroutine
	zones  []zoneRuntime
	frame  map[string][]color.RGBA
	lit    bool // received at least one frame since the last pause
	worker *deviceWorker
}

type zoneRuntime struct {
	id    string
	pts   []Point
	order string
	buf   []color.RGBA
}

// New creates an engine. Call Start to begin driving hardware.
func New(cfg Config, opt Options) *Engine {
	cfg = cfg.clone()
	cfg.normalize()
	if opt.Effects == nil {
		opt.Effects = BuiltinEffects()
	}
	if opt.ProcessRunning == nil {
		opt.ProcessRunning = proc.Running
	}
	userLog := opt.Logf
	opt.Logf = func(format string, args ...any) {
		diag.Logf("engine", format, args...)
		if userLog != nil {
			userLog(format, args...)
		}
	}
	e := &Engine{
		opt:         opt,
		effects:     make(map[string]EffectInfo, len(opt.Effects)),
		cfg:         cfg,
		accent:      color.RGBA{0, 155, 222, 255},
		effectDirty: true,
		layoutDirty: true,
		discovered:  make(chan discovery),
	}
	for _, info := range opt.Effects {
		e.effects[info.Name] = info
	}
	if _, ok := e.effects[e.cfg.Effect]; !ok && len(opt.Effects) > 0 {
		e.cfg.Effect = opt.Effects[0].Name
	}
	return e
}

// Start discovers devices and starts the render loop.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("leds: engine already running")
	}
	e.running = true
	ctx, e.cancel = context.WithCancel(ctx)
	e.done = make(chan struct{})
	e.mu.Unlock()

	d := e.discover(ctx)
	e.apply(d)
	go e.loop(ctx)
	return errors.Join(d.errs...)
}

// Stop ends the render loop, switches all devices off and releases them.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	cancel, done := e.cancel, e.done
	e.mu.Unlock()

	cancel()
	<-done

	e.mu.Lock()
	devices := e.devices
	e.devices = nil
	e.running = false
	off, _ := ParseHexColor(e.cfg.ShutdownColor) // invalid or empty: black
	keep := e.cfg.ShutdownColor == ShutdownKeep
	toDevice := e.cfg.ShutdownColor == ShutdownDevice
	e.mu.Unlock()

	// devices shut down in parallel, every step bounded, so a hanging device
	// cannot block the application from exiting
	var (
		wg   sync.WaitGroup
		emu  sync.Mutex
		errs []error
	)
	fail := func(d *deviceEntry, err error) {
		emu.Lock()
		errs = append(errs, fmt.Errorf("%s: %w", d.dev.ID(), err))
		emu.Unlock()
		e.opt.Logf("leds: stop %s: %v", d.dev.ID(), err)
	}
	for _, d := range devices {
		wg.Add(1)
		go func(d *deviceEntry) {
			defer wg.Done()
			restored := false
			busy := !d.worker.stopWithin(stopWait)
			if r, ok := d.dev.(DeviceEffectRestorer); ok && toDevice && !busy && d.state == StateOK {
				err := within(stopWait, r.RestoreDeviceEffect)
				restored = err == nil
				if err != nil && !errors.Is(err, ErrNoDeviceEffect) {
					fail(d, err)
				}
			}
			if busy {
				fail(d, errStopTimeout) // still busy: skip the last frame
			} else if d.lit && d.state == StateOK && !keep && !restored {
				for _, z := range d.zones {
					for i := range z.buf {
						z.buf[i] = off
					}
					reorder(z.buf, z.order)
				}
				if err := within(stopWait, func() error { return d.dev.SetFrame(d.frame) }); err != nil {
					fail(d, err)
				}
			}
			if err := within(stopWait, d.dev.Close); err != nil {
				fail(d, err)
			}
		}(d)
	}
	wg.Wait()
	return errors.Join(errs...)
}

// stopWait bounds every step of shutting a device down.
const stopWait = 500 * time.Millisecond

var errStopTimeout = errors.New("timed out")

// within runs f and gives up waiting after d (f keeps running in the
// background then).
func within(d time.Duration, f func() error) error {
	ch := make(chan error, 1)
	go func() { ch <- f() }()
	select {
	case err := <-ch:
		return err
	case <-time.After(d):
		return errStopTimeout
	}
}

// Rescan runs discovery on all backends now and waits until the result is in use.
func (e *Engine) Rescan(ctx context.Context) error {
	d := e.discover(ctx)
	e.mu.Lock()
	running := e.running
	e.mu.Unlock()
	if !running {
		e.apply(d)
		return errors.Join(d.errs...)
	}
	d.applied = make(chan struct{})
	e.mu.Lock()
	done := e.done
	e.mu.Unlock()
	select {
	case e.discovered <- d:
		<-d.applied
	case <-done: // engine stopped meanwhile
		closeAll(d.devices)
	case <-ctx.Done():
		closeAll(d.devices)
		return ctx.Err()
	}
	return errors.Join(d.errs...)
}

// SetEffect switches the active effect. params are merged into the stored
// parameters of that effect; nil keeps them.
func (e *Engine) SetEffect(name string, params Params) error {
	info, ok := e.effects[name]
	if !ok {
		return fmt.Errorf("leds: unknown effect %q", name)
	}
	merged := withDefaults(info, e.Config().Params[name])
	for k, v := range params {
		merged[k] = v
	}
	// validate the parameters; the render loop creates its own instance
	probe, err := info.New(merged)
	if err != nil {
		return err
	}
	closeEffect(probe)
	e.update(func(c *Config) {
		c.Effect = name
		c.Params[name] = merged
	}, func() { e.effectDirty = true })
	return nil
}

// Effects lists the available effects.
func (e *Engine) Effects() []EffectInfo {
	out := make([]EffectInfo, 0, len(e.effects))
	for _, info := range e.opt.Effects {
		out = append(out, info)
	}
	return out
}

// SetBrightness sets the global brightness (0..100).
func (e *Engine) SetBrightness(v int) {
	e.update(func(c *Config) { c.Brightness = clampInt(v, 0, 100) }, nil)
}

// SetShutdown sets Config.ShutdownColor (see there) for the next Stop.
func (e *Engine) SetShutdown(v string) {
	e.update(func(c *Config) { c.ShutdownColor = v }, nil)
}

// SetPaused switches all LEDs off (true) or resumes the effect (false).
// It is not persisted; CaseHUD uses it for the screen saver.
func (e *Engine) SetPaused(p bool) {
	e.mu.Lock()
	e.paused = p
	e.mu.Unlock()
}

// SetAccent sets the accent color effects can use (e.g. from the active design).
func (e *Engine) SetAccent(c color.RGBA) {
	c.A = 255
	e.mu.Lock()
	e.accent = c
	e.mu.Unlock()
}

// SetDeviceConfig replaces the configuration of one device (layout, enable, ...).
func (e *Engine) SetDeviceConfig(id string, dc DeviceConfig) {
	e.update(func(c *Config) { c.Devices[id] = dc }, func() { e.layoutDirty = true })
}

// Config returns a copy of the current configuration.
func (e *Engine) Config() Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.clone()
}

// OnConfigChange registers a callback that receives the configuration after
// every change made through the engine, so the host can persist it. It runs on
// the goroutine that made the change.
func (e *Engine) OnConfigChange(fn func(Config)) {
	e.mu.Lock()
	e.onChange = fn
	e.mu.Unlock()
}

// Devices reports all known devices with their status, sorted by ID.
func (e *Engine) Devices() []DeviceStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]DeviceStatus, 0, len(e.devices))
	for _, d := range e.devices {
		out = append(out, e.statusLocked(d))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (e *Engine) statusLocked(d *deviceEntry) DeviceStatus {
	st := DeviceStatus{
		ID: d.dev.ID(), Name: d.dev.Name(), Backend: d.backend,
		State: d.state, Message: d.msg, Zones: d.dev.Zones(),
	}
	if c, ok := d.dev.(Configurable); ok {
		st.Settings = c.Settings()
	}
	if f, ok := d.dev.(FirmwareReporter); ok {
		st.Firmware = f.Firmware()
	}
	return st
}

func (e *Engine) update(change func(*Config), flags func()) {
	e.mu.Lock()
	change(&e.cfg)
	e.cfg.normalize()
	if flags != nil {
		flags()
	}
	fn, cfg := e.onChange, e.cfg.clone()
	e.mu.Unlock()
	if fn != nil {
		fn(cfg)
	}
}

func (e *Engine) discover(ctx context.Context) discovery {
	d := discovery{failed: map[string]bool{}}
	for _, b := range e.opt.Backends {
		devs, err := b.Discover(ctx)
		if err != nil {
			d.failed[b.Name()] = true
			d.errs = append(d.errs, fmt.Errorf("%s: %w", b.Name(), err))
			e.opt.Logf("leds: discover %s: %v", b.Name(), err)
		}
		for _, dev := range devs {
			d.devices = append(d.devices, backendDevice{b.Name(), dev})
		}
	}
	return d
}

// apply merges a discovery result: known healthy devices keep their instance,
// new ones are added, vanished or failed ones are closed.
func (e *Engine) apply(d discovery) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := make(map[string]*deviceEntry, len(e.devices))
	for _, de := range e.devices {
		old[de.dev.ID()] = de
	}
	var next []*deviceEntry
	seen := map[string]bool{}
	for _, bd := range d.devices {
		id := bd.dev.ID()
		if seen[id] {
			bd.dev.Close()
			continue
		}
		seen[id] = true
		if de, ok := old[id]; ok && de.state != StateError {
			bd.dev.Close() // never initialized, so this does not touch the hardware
			next = append(next, de)
			delete(old, id)
			continue
		}
		next = append(next, &deviceEntry{dev: bd.dev, backend: bd.backend, state: StateOK, worker: startWorker(bd.dev)})
		e.opt.Logf("leds: device %s (%s, %s) ready", id, bd.dev.Name(), bd.backend)
	}
	for id, de := range old {
		if d.failed[de.backend] && de.state != StateError {
			// discovery of its backend failed: no evidence that it is gone
			next = append(next, de)
			delete(old, id)
		}
	}
	for _, de := range old {
		e.opt.Logf("leds: device %s removed", de.dev.ID())
		de.worker.stop()
		de.dev.Close()
	}
	e.devices = next
	e.layoutDirty = true
	e.checkConflictsLocked()
}

func (e *Engine) checkConflictsLocked() {
	var names []string
	for _, d := range e.devices {
		names = append(names, d.dev.Conflicts()...)
	}
	running := map[string]bool{}
	if len(names) > 0 {
		for _, n := range e.opt.ProcessRunning(names) {
			running[strings.ToLower(n)] = true
		}
	}
	for _, d := range e.devices {
		if d.state == StateError {
			continue
		}
		dc := e.cfg.Devices[d.dev.ID()]
		var hit []string
		for _, n := range d.dev.Conflicts() {
			if running[strings.ToLower(n)] {
				hit = append(hit, n)
			}
		}
		before := d.state
		switch {
		case dc.Disabled:
			d.state, d.msg = StateDisabled, ""
		case len(hit) > 0 && !dc.IgnoreConflicts:
			d.state, d.msg = StateConflict, "in use by "+strings.Join(hit, ", ")
		default:
			d.state, d.msg = StateOK, ""
		}
		if d.state != before {
			e.opt.Logf("leds: %s: %s -> %s %s", d.dev.ID(), before, d.state, d.msg)
		}
	}
}

func (e *Engine) loop(ctx context.Context) {
	defer close(e.done)

	canvas := image.NewRGBA(image.Rect(0, 0, CanvasWidth, CanvasHeight))
	var (
		effect      Effect
		start, prev = time.Now(), time.Now()
		n           uint64
		fps         int
		ticker      *time.Ticker
		lastCheck   = time.Now()
		lastRescan  = time.Now()
		rescanning  bool
		rescanDone  = make(chan discovery, 1)
	)
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
		closeEffect(effect)
		if rescanning { // its devices were never applied: release them
			d := <-rescanDone
			closeAll(d.devices)
		}
	}()

	for {
		e.mu.Lock()
		if e.cfg.FPS != fps {
			fps = e.cfg.FPS
			if ticker != nil {
				ticker.Stop()
			}
			ticker = time.NewTicker(time.Second / time.Duration(fps))
		}
		e.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case d := <-e.discovered:
			e.apply(d)
			close(d.applied)
			continue
		case d := <-rescanDone:
			rescanning = false
			e.apply(d)
			continue
		case <-ticker.C:
		}

		now := time.Now()
		if e.opt.RescanInterval > 0 && !rescanning && now.Sub(lastRescan) >= e.opt.RescanInterval {
			lastRescan, rescanning = now, true
			go func() { rescanDone <- e.discover(ctx) }()
		}

		e.mu.Lock()
		if now.Sub(lastCheck) >= conflictCheckInterval {
			lastCheck = now
			e.checkConflictsLocked()
		}
		if e.effectDirty {
			e.effectDirty = false
			info := e.effects[e.cfg.Effect]
			if info.New != nil {
				eff, err := info.New(withDefaults(info, e.cfg.Params[info.Name]))
				if err != nil {
					e.opt.Logf("leds: effect %s: %v", info.Name, err)
				} else {
					closeEffect(effect)
					effect, start, n = eff, now, 0
				}
			}
		}
		if e.layoutDirty {
			e.layoutDirty = false
			for _, d := range e.devices {
				e.buildLayoutLocked(d)
			}
		}
		paused, accent := e.paused, e.accent
		var calib calibration
		if e.calib != nil {
			calib = *e.calib
		}
		bright := e.cfg.Brightness * 256 / 100
		devices := append([]*deviceEntry(nil), e.devices...)
		e.mu.Unlock()

		if !paused && effect != nil {
			var s Sensors
			if e.opt.Sensors != nil {
				s = e.opt.Sensors()
			}
			effect.Render(Frame{T: now.Sub(start), Dt: now.Sub(prev), N: n, Sensors: s, Accent: accent}, canvas)
			n++
		}
		prev = now

		for _, d := range devices {
			e.mu.Lock()
			if err := d.worker.takeErr(); err != nil && d.state == StateOK {
				d.state, d.msg = StateError, err.Error()
				e.opt.Logf("leds: %s: %v", d.dev.ID(), err)
			}
			ok := d.state == StateOK
			e.mu.Unlock()
			if !ok {
				continue
			}
			if paused {
				if !d.lit {
					continue
				}
				for _, z := range d.zones {
					for i := range z.buf {
						z.buf[i] = color.RGBA{A: 255}
					}
				}
			} else {
				for _, z := range d.zones {
					Sample(canvas, z.pts, z.buf)
					scale(z.buf, bright)
					reorder(z.buf, z.order)
				}
			}
			frame := d.frame
			if calib.device == d.dev.ID() {
				frame = calibrationFrame(d, calib)
			}
			d.worker.submit(frame)
			d.lit = !paused
		}
	}
}

func (e *Engine) buildLayoutLocked(d *deviceEntry) {
	dc := e.cfg.Devices[d.dev.ID()]
	if c, ok := d.dev.(Configurable); ok {
		if err := c.Configure(paramsWithDefaults(c.Settings(), dc.Settings)); err != nil {
			e.opt.Logf("leds: %s: settings: %v", d.dev.ID(), err)
		}
	}
	d.zones = d.zones[:0]
	d.frame = map[string][]color.RGBA{}
	for _, z := range d.dev.Zones() {
		zc := dc.Zones[z.ID]
		pts := Positions(z, zc)
		order := zc.Order
		if order == "" {
			order = z.Order
		}
		buf := make([]color.RGBA, len(pts))
		d.zones = append(d.zones, zoneRuntime{id: z.ID, pts: pts, order: order, buf: buf})
		d.frame[z.ID] = buf
	}
}

// closeEffect releases effects that hold resources (capture, decoder, ...).
func closeEffect(eff Effect) {
	if c, ok := eff.(io.Closer); ok {
		c.Close()
	}
}

func closeAll(devs []backendDevice) {
	for _, bd := range devs {
		bd.dev.Close()
	}
}
