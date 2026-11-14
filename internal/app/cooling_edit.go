package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kewe87/casehud/internal/cooling"
)

// ErrCoolingBusy means a speed measurement is running: nothing about the
// cooling can be changed meanwhile.
var ErrCoolingBusy = errors.New("a speed measurement is running")

// ErrCoolingOff is returned by edits while the cooling area is switched off.
var ErrCoolingOff = errCoolingOff

// Codes of CoolingEditError.
const (
	EditNotFound     = "not_found"
	EditInvalid      = "invalid_value"
	EditInUse        = "curve_in_use"
	EditUnconfigured = "unconfigured"
)

// CoolingEditError says why a change of one curve or output was refused.
type CoolingEditError struct{ Code, Msg string }

func (e *CoolingEditError) Error() string { return e.Msg }

func editErr(code, format string, a ...any) error {
	return &CoolingEditError{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// CoolingConfigDoc is the stored cooling configuration with its revision.
type CoolingConfigDoc struct {
	Config   coolingConfig `json:"config"`
	Revision int64         `json:"revision"`
}

// CoolingConfigDoc returns the stored configuration and the revision a change
// has to name.
func (e *Engine) CoolingConfigDoc() CoolingConfigDoc {
	rev := e.coolingRevision() // before the snapshot: at worst too old, never ahead
	cfg := e.coolingSnapshot().cfg
	if cfg.Outputs == nil {
		cfg.Outputs = []coolingOutputConfig{}
	}
	if cfg.Curves == nil {
		cfg.Curves = []coolingCurveConfig{}
	}
	if cfg.Mode == "" {
		cfg.Mode = "preview"
	}
	return CoolingConfigDoc{Config: cfg, Revision: rev}
}

// CoolingLiveBlock says why the live mode would send nothing right now: the
// disclaimer is not accepted ("disclaimer"), another fan program runs
// ("program", with its name) or that could not be ruled out ("check").
func (e *Engine) CoolingLiveBlock() (reason, program string) {
	if !e.outputAllowed() {
		return coolingBlockedDisclaimer, ""
	}
	return checkRivals(e.conflictSystem())
}

// editCooling is the read-modify-write of the stored configuration: f changes
// a copy, which then goes through the checks of a saved configuration. rev (0:
// not checked) must be the current revision. It returns the new revision.
func (e *Engine) editCooling(rev int64, f func(cfg *coolingConfig) error) (int64, error) {
	if !e.coolingOn() {
		return 0, errCoolingOff
	}
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	if rev != 0 && rev != e.coolingRevision() {
		return 0, ErrCoolingRevision
	}
	if e.coolingMeasuring() {
		return 0, ErrCoolingBusy
	}
	c := e.coolingSnapshot()
	if c.errMsg != "" {
		return 0, editErr(EditUnconfigured, "%s is not usable: %s", coolingFile, c.errMsg)
	}
	cfg := c.cfg
	cfg.Outputs = slices.Clone(cfg.Outputs) // elements are replaced, never changed in place
	cfg.Curves = slices.Clone(cfg.Curves)
	if cfg.Outputs == nil {
		cfg.Outputs = []coolingOutputConfig{}
	}
	if err := f(&cfg); err != nil {
		return 0, err
	}
	cfg.liftUnusedSafeDuty()
	if err := cfg.check(); err != nil {
		return 0, editErr(EditInvalid, "%s", strings.TrimPrefix(err.Error(), coolingFile+": "))
	}
	for _, o := range cfg.Outputs {
		if o.ControlKey != "" && e.channelMissing(o.ControlKey) {
			return 0, editErr(EditInvalid, "%q: the board has no control channel %s", o.ID, o.ControlKey)
		}
	}
	if err := e.storeCooling(cfg); err != nil {
		return 0, err
	}
	return e.coolingRevision(), nil
}

// CoolingManualValue is a fixed value for an output, see CoolingOutputPatch.
type CoolingManualValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"` // "percent" (default) or "rpm"
}

// CoolingOutputPatch changes the fields of one output that are present. Curve
// and Manual exclude each other; Curve "" takes the output off its curve.
type CoolingOutputPatch struct {
	Name      *string             `json:"name"`
	Curve     *string             `json:"curve"`
	Manual    *CoolingManualValue `json:"manual"`
	MinDuty   *int                `json:"min_duty"`
	StopAt    *int                `json:"stop_at"`
	StartDuty *int                `json:"start_duty"`
	StepUp    *int                `json:"step_up"`
	StepDown  *int                `json:"step_down"`
	SafeDuty  *int                `json:"safe_duty"`
	Offset    *int                `json:"offset"`
}

const coolingMaxOutputName = 60

func inRange(name string, p *int, lo, hi int) error {
	if p != nil && (*p < lo || *p > hi) {
		return editErr(EditInvalid, "%s must be between %d and %d", name, lo, hi)
	}
	return nil
}

// PatchCoolingOutput changes one output of the stored configuration. rev is the
// revision the client worked on (0: not checked). A pump never goes below its
// floor: its minimum and a manual value of less than 30 % are refused.
func (e *Engine) PatchCoolingOutput(id string, p CoolingOutputPatch, rev int64) (int64, error) {
	if p.Curve != nil && p.Manual != nil {
		return 0, editErr(EditInvalid, "a curve or a manual value, not both")
	}
	for _, c := range []error{
		inRange("min_duty", p.MinDuty, 0, 100), inRange("stop_at", p.StopAt, 0, 100), inRange("start_duty", p.StartDuty, 0, 100),
		inRange("step_up", p.StepUp, 1, 100), inRange("step_down", p.StepDown, 1, 100), inRange("safe_duty", p.SafeDuty, 0, 100),
		inRange("offset", p.Offset, -100, 100),
	} {
		if c != nil {
			return 0, c
		}
	}
	if p.Name != nil && utf8.RuneCountInString(*p.Name) > coolingMaxOutputName {
		return 0, editErr(EditInvalid, "name is longer than %d characters", coolingMaxOutputName)
	}
	if p.Manual != nil && p.Manual.Unit != "" && p.Manual.Unit != "percent" && p.Manual.Unit != coolingUnitRPM {
		return 0, editErr(EditInvalid, "unknown unit %q", p.Manual.Unit)
	}
	return e.editCooling(rev, func(cfg *coolingConfig) error {
		i := slices.IndexFunc(cfg.Outputs, func(o coolingOutputConfig) bool { return o.ID == id })
		if i < 0 {
			return editErr(EditNotFound, "no output %q", id)
		}
		o := cfg.Outputs[i]
		if p.Name != nil {
			o.Name = strings.TrimSpace(*p.Name)
		}
		switch {
		case p.Curve != nil:
			if *p.Curve != "" && !slices.ContainsFunc(cfg.Curves, func(c coolingCurveConfig) bool { return c.ID == *p.Curve }) {
				return editErr(EditNotFound, "no curve %q", *p.Curve)
			}
			o.Curve, o.Manual = *p.Curve, nil
		case p.Manual != nil:
			o.Curve = ""
			o.Manual = &coolingManual{Value: p.Manual.Value, Unit: p.Manual.Unit}
		}
		if p.MinDuty != nil {
			if o.Pump && *p.MinDuty < cooling.LowestPumpDuty {
				return editErr(EditInvalid, "%q: a pump needs at least %d %%", id, cooling.LowestPumpDuty)
			}
			o.MinDuty = *p.MinDuty
		}
		if p.StopAt != nil {
			o.StopAt = *p.StopAt
		}
		if p.StartDuty != nil {
			o.StartDuty = *p.StartDuty
		}
		if p.StepUp != nil {
			o.StepUp = *p.StepUp
		}
		if p.StepDown != nil {
			o.StepDown = *p.StepDown
		}
		if p.SafeDuty != nil {
			o.SafeDuty = *p.SafeDuty
		}
		if p.Offset != nil {
			o.Offset = *p.Offset
		}
		cfg.Outputs[i] = o
		return nil
	})
}

// PutCoolingCurve creates or replaces the curve id with the one in body (a curve
// as in the configuration; its "id" may be left out). It reports whether the
// curve is new.
func (e *Engine) PutCoolingCurve(id string, body []byte, rev int64) (created bool, newRev int64, err error) {
	var cv coolingCurveConfig
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cv); err != nil {
		return false, 0, editErr(EditInvalid, "curve: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return false, 0, editErr(EditInvalid, "curve: unexpected data after the curve")
	}
	if cv.ID != "" && cv.ID != id {
		return false, 0, editErr(EditInvalid, "the id in the body differs from the one in the path")
	}
	cv.ID = id
	if cv.Name == "" {
		cv.Name = id
	}
	newRev, err = e.editCooling(rev, func(cfg *coolingConfig) error {
		if i := slices.IndexFunc(cfg.Curves, func(c coolingCurveConfig) bool { return c.ID == id }); i >= 0 {
			cfg.Curves[i] = cv
			return nil
		}
		created = true
		cfg.Curves = append(cfg.Curves, cv)
		return nil
	})
	return created && err == nil, newRev, err
}

// DeleteCoolingCurve removes a curve that nothing uses.
func (e *Engine) DeleteCoolingCurve(id string, rev int64) (int64, error) {
	return e.editCooling(rev, func(cfg *coolingConfig) error {
		i := slices.IndexFunc(cfg.Curves, func(c coolingCurveConfig) bool { return c.ID == id })
		if i < 0 {
			return editErr(EditNotFound, "no curve %q", id)
		}
		for _, o := range cfg.Outputs {
			if o.Curve == id {
				return editErr(EditInUse, "the curve is used by %q", firstOf(o.Name, o.ID))
			}
		}
		for _, c := range cfg.Curves {
			if c.ID != id && (c.Reference == id || slices.Contains(c.Inputs, id)) {
				return editErr(EditInUse, "the curve is used by the curve %q", firstOf(c.Name, c.ID))
			}
		}
		cfg.Curves = slices.Delete(cfg.Curves, i, i+1)
		return nil
	})
}

// The types of the stored configuration, named for the API description.
type (
	CoolingConfig = coolingConfig
	CoolingCurve  = coolingCurveConfig
)
