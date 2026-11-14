package app

import (
	"errors"
	"fmt"
	"log"
	"time"
)

// ErrNoModule is returned when a request would switch off every core area.
var ErrNoModule = errors.New("at least one area must stay on")

// Modules tells which of the three core areas run. They are independent; the
// stored configuration of an area that is off is kept as it is.
type Modules struct {
	Display  bool `json:"display"`
	Lighting bool `json:"lighting"`
	Cooling  bool `json:"cooling"`
}

func modulesOf(c *Config) Modules {
	return Modules{Display: !c.NoDisplay, Lighting: !c.NoLighting, Cooling: !c.NoCooling}
}

// normalizeModules repairs a stored state with every area off: an engine with
// nothing to do is never the result, so everything runs.
func normalizeModules(c *Config) {
	if c.NoDisplay && c.NoLighting && c.NoCooling {
		log.Print("modules: all areas were off in the configuration, starting with all of them on")
		c.NoDisplay, c.NoLighting, c.NoCooling = false, false, false
	}
}

// Modules returns which areas are on.
func (e *Engine) Modules() Modules {
	e.mu.Lock()
	defer e.mu.Unlock()
	return modulesOf(&e.cfg)
}

func (e *Engine) displayOn() bool  { return e.Modules().Display }
func (e *Engine) lightingOn() bool { return e.Modules().Lighting }
func (e *Engine) coolingOn() bool  { return e.Modules().Cooling }

// SetModules switches the areas. A state with every area off is refused before
// anything changes. What turns off is stopped the way a clean exit stops it,
// what turns on starts from the stored configuration.
func (e *Engine) SetModules(m Modules) error {
	if !m.Display && !m.Lighting && !m.Cooling {
		return ErrNoModule
	}
	e.modMu.Lock()
	defer e.modMu.Unlock()
	e.mu.Lock()
	before := modulesOf(&e.cfg)
	e.cfg.NoDisplay, e.cfg.NoLighting, e.cfg.NoCooling = !m.Display, !m.Lighting, !m.Cooling
	e.mu.Unlock()
	if before == m {
		return nil
	}
	if err := e.persist(); err != nil {
		e.mu.Lock()
		e.cfg.NoDisplay, e.cfg.NoLighting, e.cfg.NoCooling = !before.Display, !before.Lighting, !before.Cooling
		e.mu.Unlock()
		return err
	}
	if e.ctx == nil || e.ctx.Err() != nil {
		return nil // not running: the next start reads the file
	}
	// Switching off first, so the shared controller is never held by both.
	if before.Cooling && !m.Cooling {
		e.stopCoolingModule()
	}
	if before.Lighting && !m.Lighting {
		e.suspendLighting()
	}
	if before.Display && !m.Display {
		e.displaysOff()
	}
	if !before.Cooling && m.Cooling {
		e.startCoolingModule()
	}
	if !before.Lighting && m.Lighting && e.outputAllowed() {
		e.resumeLighting()
	}
	if !before.Display && m.Display {
		e.Rescan()
	}
	e.wakeAwtrix()
	return nil
}

// ErrSetupDone is returned when the setup dialog was already answered.
var ErrSetupDone = errors.New("setup already completed")

// SetupPending tells whether the setup dialog is due: only for a new
// configuration, and not in a demo unless it asks for it.
func (e *Engine) SetupPending() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.FirstRun && (!e.Demo || e.DemoSetup)
}

// CompleteSetup ends the setup. With m the areas are switched like in the
// settings (same rule, same error, nothing changes on error and the dialog
// stays due); without it they stay as they are.
func (e *Engine) CompleteSetup(m *Modules) error {
	e.mu.Lock()
	if !e.cfg.FirstRun {
		e.mu.Unlock()
		return ErrSetupDone
	}
	e.cfg.FirstRun = false
	e.mu.Unlock()
	if m != nil {
		if err := e.SetModules(*m); err != nil {
			e.mu.Lock()
			e.cfg.FirstRun = true
			e.mu.Unlock()
			return err
		}
	}
	if err := e.persist(); err != nil {
		e.mu.Lock()
		e.cfg.FirstRun = true
		e.mu.Unlock()
		return err
	}
	return nil
}

// stopCoolingModule ends the control like a clean exit does: pumps go back to
// the board, fans stay at their safe duty or above. The configuration file is
// not touched.
func (e *Engine) stopCoolingModule() {
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	old := e.coolingSnapshot()
	e.setCoolingState(coolingState{cfg: old.cfg, configured: old.configured})
	if old.cancel == nil {
		return
	}
	old.cancel()
	if old.done != nil {
		select {
		case <-old.done:
		case <-time.After(coolingStopWait):
		}
	}
}

func (e *Engine) startCoolingModule() {
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	e.startCooling()
}

// suspendLighting stops the light engine and switches the devices off as a
// clean exit does. The lighting configuration, including its own on/off
// switch, stays as it is.
func (e *Engine) suspendLighting() {
	l := &e.light
	l.lifeMu.Lock()
	defer l.lifeMu.Unlock()
	l.mu.Lock()
	eng := e.detachLightEngineLocked()
	l.mu.Unlock()
	stopLightEngine(eng)
	e.ringBootPoke()
}

// displaysOff hands the exit picture to the displays, as Stop does, and closes
// them. Read-only sensor devices stay open.
func (e *Engine) displaysOff() {
	if e.outputAllowed() {
		e.mu.Lock()
		ws := make([]*worker, 0, len(e.workers))
		for _, w := range e.workers {
			ws = append(ws, w)
		}
		e.mu.Unlock()
		for _, w := range ws {
			w.setExitImage()
		}
	}
	e.Rescan()
}

var errCoolingOff = errors.New("cooling is switched off in the settings")

// modeName is the mode as shown: anything but "live" only computes.
func modeName(m string) string {
	if m == coolingModeLive {
		return coolingModeLive
	}
	return "preview"
}

// SetCoolingMode switches between live control and test mode. Only the mode
// changes: the stored curves and outputs stay as they are, and the checks are
// the ones a saved configuration goes through.
func (e *Engine) SetCoolingMode(mode string) error {
	if mode != coolingModeLive && mode != "preview" {
		return fmt.Errorf("unknown mode %q", mode)
	}
	if !e.coolingOn() {
		return errCoolingOff
	}
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	if e.coolingMeasuring() {
		return ErrCoolingBusy
	}
	c := e.coolingSnapshot()
	if c.errMsg != "" {
		return fmt.Errorf("%s is not usable: %s", coolingFile, c.errMsg)
	}
	cfg := c.cfg
	cfg.Mode = mode
	if cfg.Outputs == nil {
		cfg.Outputs = []coolingOutputConfig{}
	}
	if err := cfg.check(); err != nil {
		return fmt.Errorf("%s: %w", coolingFile, err)
	}
	return e.storeCooling(cfg)
}
