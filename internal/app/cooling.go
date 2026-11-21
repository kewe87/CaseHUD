package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/cooling"
	"github.com/kewe87/casehud/internal/rgb/backend/lianlitl"
)

const (
	coolingFile = "cooling.json"
	coolingTick = time.Second

	// Far above any real machine. The file is read during startup.
	coolingMaxBytes   = 1 << 20
	coolingMaxOutputs = 64
	coolingMaxCurves  = 64
	coolingMaxErrText = 300
	coolingMaxNames   = 512 // names of sensors in the settings
	coolingMaxNameLen = 256 // bytes of a sensor key or name
	coolingRoleCase   = "case"

	coolingModeLive = "live" // anything sent to the hardware needs this in the file
	coolingUnitRPM  = "rpm"

	// What drives an output.
	coolingModeCurve  = "curve"
	coolingModeManual = "manual"
	coolingModeNone   = "none"
)

// coolingCurveConfig is one curve in cooling.json: what it follows and what it
// asks for. Any number of outputs can hang on it. Points are [temperature,
// value], with values in percent, or in rpm with Unit "rpm". Which fields count
// depends on Kind, see cooling.CurveSpec for their meaning.
type coolingCurveConfig struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Kind           string      `json:"kind,omitempty"`
	Sources        []string    `json:"sources"`
	Unit           string      `json:"unit,omitempty"`
	Points         [][]float64 `json:"points"`
	RiseBy         float64     `json:"rise_by"`
	FallBy         float64     `json:"fall_by"`
	RiseAfter      int         `json:"rise_after"`
	FallAfter      int         `json:"fall_after"`
	DistanceAtEnds bool        `json:"distance_at_ends,omitempty"`
	LeadTicks      int         `json:"lead_ticks,omitempty"`
	Value          float64     `json:"value,omitempty"`
	IdleTemp       float64     `json:"idle_temp,omitempty"`
	LoadTemp       float64     `json:"load_temp,omitempty"`
	IdleValue      float64     `json:"idle_value,omitempty"`
	LoadValue      float64     `json:"load_value,omitempty"`
	Step           float64     `json:"step,omitempty"`
	Deadband       float64     `json:"deadband,omitempty"`
	ReactAfter     int         `json:"react_after,omitempty"`
	Reference      string      `json:"reference,omitempty"`
	Inputs         []string    `json:"inputs,omitempty"`
	Combine        string      `json:"combine,omitempty"`
	Follow         string      `json:"follow,omitempty"`
	Offset         float64     `json:"offset,omitempty"`
	Proportional   bool        `json:"proportional,omitempty"`
}

// coolingManual is the fixed value of an output that follows no curve. Unit is
// "percent" (default) or "rpm", which the output's speed table turns into a duty.
type coolingManual struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// coolingManualPrefix starts the ID of the curve a manual value stands for; user
// curves may not use it.
const coolingManualPrefix = "@manual:"

// driven tells an output that has something to follow: a curve or a manual value.
func (o coolingOutputConfig) driven() bool { return o.Curve != "" || o.Manual != nil }

// curveRef is the curve the controller attaches the output to.
func (o coolingOutputConfig) curveRef() string {
	if o.Manual != nil {
		return coolingManualPrefix + o.ID
	}
	return o.Curve
}

// coolingOutputMode names what drives an output.
func (o coolingOutputConfig) mode() string {
	switch {
	case o.Manual != nil:
		return coolingModeManual
	case o.Curve != "":
		return coolingModeCurve
	}
	return coolingModeNone
}

// coolingOutputConfig is one fan or pump in cooling.json: how it carries out what
// its curve asks for. Without a curve it is left alone and only keeps its
// settings and measurement. Speed points are [duty, rpm].
type coolingOutputConfig struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Curve string `json:"curve,omitempty"`
	// Manual is a fixed value instead of a curve. An output has one or the other.
	Manual          *coolingManual `json:"manual,omitempty"`
	RPMSensor       string         `json:"rpm_sensor,omitempty"`
	Speeds          [][]int        `json:"speeds,omitempty"`
	StepUp          int            `json:"step_up"`
	StepDown        int            `json:"step_down"`
	MinDuty         int            `json:"min_duty,omitempty"`
	StopAt          int            `json:"stop_at,omitempty"`
	StartDuty       int            `json:"start_duty,omitempty"`
	Offset          int            `json:"offset,omitempty"`
	SafeDuty        int            `json:"safe_duty"`
	Pump            bool           `json:"pump,omitempty"`
	StoppedBelowRPM int            `json:"stopped_below_rpm,omitempty"`
	// Role "case" is a case fan: it always runs, whatever its curve says.
	Role string `json:"role,omitempty"`
	// ControlKey is the control channel the user chose for a pump on a mainboard
	// header. Empty: it is derived from the speed sensor, see pumpControl.
	ControlKey string `json:"control_key,omitempty"`
	// Calibration is what the last calibration found; it belongs to the fan like Speeds.
	Calibration *coolingCalibration `json:"calibration,omitempty"`
}

type coolingConfig struct {
	// Mode is "live" to drive the fans. Empty or "preview" only computes.
	Mode    string                `json:"mode,omitempty"`
	Curves  []coolingCurveConfig  `json:"curves,omitempty"`
	Outputs []coolingOutputConfig `json:"outputs"`
	// SensorNames holds the names the user gave to sensors, by sensor key.
	SensorNames map[string]string `json:"sensor_names,omitempty"`
}

func (c coolingCurveConfig) spec() cooling.CurveSpec {
	s := cooling.CurveSpec{
		ID: c.ID, Kind: cooling.Kind(c.Kind), RPM: c.Unit == coolingUnitRPM, Sources: c.Sources, Reference: c.Reference,
		RiseBy: c.RiseBy, FallBy: c.FallBy, RiseAfter: c.RiseAfter, FallAfter: c.FallAfter,
		DistanceAtEnds: c.DistanceAtEnds, LeadTicks: c.LeadTicks, Value: c.Value,
		IdleTemp: c.IdleTemp, LoadTemp: c.LoadTemp, IdleValue: c.IdleValue, LoadValue: c.LoadValue,
		Step: c.Step, Deadband: c.Deadband, ReactAfter: c.ReactAfter,
		Inputs: c.Inputs, Combine: cooling.CombineFunc(c.Combine), Follow: c.Follow, Offset: c.Offset, Proportional: c.Proportional,
	}
	for _, p := range c.Points {
		s.Points = append(s.Points, cooling.Point{Temp: p[0], Value: p[1]})
	}
	return s
}

// specs returns all curves for the controller. A manual value is a fixed curve
// of its own: it needs no sensor and shares everything else with a curve. The
// sensors of all curves guard it, so that it still goes to full duty when a part
// gets too hot.
func (c coolingConfig) specs() []cooling.CurveSpec {
	out := make([]cooling.CurveSpec, len(c.Curves), len(c.Curves)+len(c.Outputs))
	for i, x := range c.Curves {
		out[i] = x.spec()
	}
	var guard []string
	for _, o := range c.Outputs {
		if o.Manual != nil {
			if guard == nil {
				guard = c.measureSources()
			}
			out = append(out, cooling.CurveSpec{ID: o.curveRef(), Kind: cooling.KindFixed, RPM: o.Manual.Unit == coolingUnitRPM, Value: o.Manual.Value, Guard: guard})
		}
	}
	return out
}

// fan is the output as the controller needs it. Its curve comes separately.
func (o coolingOutputConfig) fan() cooling.Fan {
	f := cooling.Fan{
		ID: o.ID, CurveID: o.curveRef(), Offset: o.Offset, StepUp: o.StepUp, StepDown: o.StepDown, MinDuty: o.MinDuty, StopAt: o.StopAt,
		StartDuty: o.StartDuty, SafeDuty: o.SafeDuty, Pump: o.Pump, StoppedBelowRPM: o.StoppedBelowRPM,
	}
	if o.Manual != nil {
		f.Offset = 0 // the value is what the output is set to
	}
	for _, p := range o.Speeds {
		f.Speeds = append(f.Speeds, cooling.SpeedPoint{Duty: p[0], RPM: p[1]})
	}
	return f
}

// sensors lists the sensors a curve depends on, also through other curves.
func (c coolingConfig) sensors(id string, seen map[string]bool) []string {
	if seen[id] {
		return nil
	}
	seen[id] = true
	x, _ := c.curve(id)
	// Only what the kind uses counts: a curve keeps the rest while another kind is tried.
	var out []string
	switch cooling.Kind(x.Kind) {
	case cooling.KindFixed:
	case cooling.KindCombined:
		for _, in := range x.Inputs {
			out = append(out, c.sensors(in, seen)...)
		}
	case cooling.KindFollow:
		for _, o := range c.Outputs {
			if o.ID == x.Follow {
				out = append(out, c.sensors(o.Curve, seen)...)
			}
		}
	case cooling.KindGap:
		out = append(out, x.Sources...)
		if x.Reference != "" {
			out = append(out, x.Reference)
		}
	default:
		out = append(out, x.Sources...)
	}
	return out
}

func (c coolingConfig) curve(id string) (coolingCurveConfig, bool) {
	for _, x := range c.Curves {
		if x.ID == id {
			return x, true
		}
	}
	return coolingCurveConfig{}, false
}

// regulated returns the outputs the loop looks after: those on a curve or with a manual value, and every
// pump, because a pump that stands still is broken with or without a curve.
func (c coolingConfig) regulated() []coolingOutputConfig {
	var out []coolingOutputConfig
	for _, o := range c.Outputs {
		if o.driven() || o.Pump {
			out = append(out, o)
		}
	}
	return out
}

// loadCoolingConfig reads cooling.json. It returns fs.ErrNotExist if there is
// none, which is the normal case: then the feature stays out of the way.
func (s *Store) loadCoolingConfig() (coolingConfig, error) {
	var c coolingConfig
	f, err := os.Open(s.path(coolingFile))
	if err != nil {
		return c, withoutPath(err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, coolingMaxBytes+1))
	if err != nil {
		return c, withoutPath(err)
	}
	if len(b) > coolingMaxBytes {
		return c, fmt.Errorf("%s: larger than %d bytes", coolingFile, coolingMaxBytes)
	}
	return parseCoolingConfig(b)
}

// parseCoolingConfig reads the settings strictly: a typo must not silently turn
// into a default for a fan.
func parseCoolingConfig(b []byte) (coolingConfig, error) {
	var c coolingConfig
	if len(b) > coolingMaxBytes {
		return c, fmt.Errorf("%s: larger than %d bytes", coolingFile, coolingMaxBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", coolingFile, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("%s: unexpected data after the settings", coolingFile)
	}
	c.liftUnusedSafeDuty()
	if err := c.check(); err != nil {
		return c, fmt.Errorf("%s: %w", coolingFile, err)
	}
	return c, nil
}

// withoutPath keeps the location out of messages that reach the GUI and log.
func withoutPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.ErrNotExist
		}
		return fmt.Errorf("%s: %w", coolingFile, pe.Err)
	}
	return err
}

// hasFanController tells whether a regulated output sits on the TL fan controller,
// the only device a guard process has to watch.
func (c coolingConfig) hasFanController() bool {
	return slices.ContainsFunc(c.regulated(), func(o coolingOutputConfig) bool { return tlOutputID.MatchString(o.ID) })
}

// isRegulated tells an output the loop sets: one on a curve, or any pump.
func (o coolingOutputConfig) isRegulated() bool { return o.driven() || o.Pump }

// liftUnusedSafeDuty raises the safe duty of an output nothing regulates to the
// floor. Such an output is left alone and loaded as before, but it must not carry
// a value that would leave a fan standing once a curve is put on it.
func (c *coolingConfig) liftUnusedSafeDuty() {
	for i := range c.Outputs {
		if o := &c.Outputs[i]; !o.isRegulated() {
			o.SafeDuty = min(max(o.SafeDuty, cooling.LowestSafeDuty), 100)
		}
	}
}

// check rejects what the output settings cannot express: malformed points, and
// a pump whose standstill could never be noticed.
func (c coolingConfig) check() error {
	if len(c.Outputs) > coolingMaxOutputs {
		return fmt.Errorf("more than %d outputs", coolingMaxOutputs)
	}
	if c.Mode != "" && c.Mode != "preview" && c.Mode != coolingModeLive {
		return fmt.Errorf("unknown mode %q", c.Mode)
	}
	if len(c.Curves) > coolingMaxCurves {
		return fmt.Errorf("more than %d curves", coolingMaxCurves)
	}
	curves := map[string]bool{}
	for _, x := range c.Curves {
		if x.ID == "" || curves[x.ID] {
			return fmt.Errorf("curve %q: the ID is empty or used twice", x.ID)
		}
		curves[x.ID] = true
		if strings.HasPrefix(x.ID, coolingManualPrefix) {
			return fmt.Errorf("curve %q: the ID is reserved", x.ID)
		}
		if x.Unit != "" && x.Unit != "percent" && x.Unit != coolingUnitRPM {
			return fmt.Errorf("curve %q: unknown unit %q", x.ID, x.Unit)
		}
		for _, p := range x.Points {
			if len(p) != 2 {
				return fmt.Errorf("curve %q: a point needs temperature and value", x.ID)
			}
		}
	}
	outputs := map[string]bool{}
	for _, o := range c.Outputs {
		if o.ID == "" || outputs[o.ID] {
			return fmt.Errorf("output %q: the ID is empty or used twice", o.ID)
		}
		outputs[o.ID] = true
		for _, p := range o.Speeds {
			if len(p) != 2 {
				return fmt.Errorf("%q: a speed point needs duty and rpm", o.ID)
			}
		}
		if o.Curve != "" && !curves[o.Curve] {
			return fmt.Errorf("%q: no curve %q", o.ID, o.Curve)
		}
		if err := o.checkManual(); err != nil {
			return err
		}
		// Whatever ends the control leaves the output at this duty or above, so it
		// must keep a fan turning. A fan may stand still in operation (stop_at),
		// but never after the program ended or lost its sensors.
		if o.isRegulated() && (o.SafeDuty < cooling.LowestSafeDuty || o.SafeDuty > 100) {
			return fmt.Errorf("%q: safe_duty must be between %d and 100", o.ID, cooling.LowestSafeDuty)
		}
		switch o.Role {
		case "":
		case coolingRoleCase:
			if o.Pump {
				return fmt.Errorf("%q: a pump is not a case fan", o.ID)
			}
			if o.MinDuty < 1 || o.StopAt > 0 {
				return fmt.Errorf("%q: a case fan always runs, it needs a minimum and no stop", o.ID)
			}
		default:
			return fmt.Errorf("%q: unknown role %q", o.ID, o.Role)
		}
		if err := o.Calibration.check(); err != nil {
			return fmt.Errorf("%q: %w", o.ID, err)
		}
		onController := tlOutputID.MatchString(o.ID)
		if o.ControlKey != "" {
			if !o.Pump || onController {
				return fmt.Errorf("%q: only a pump on a mainboard header has a control channel", o.ID)
			}
			if !controlOnChipOf(o.RPMSensor, o.ControlKey) {
				return fmt.Errorf("%q: the control channel is not on the chip of the speed sensor", o.ID)
			}
		}
		if o.Pump && !onController && o.RPMSensor == "" {
			return fmt.Errorf("%q: a pump needs rpm_sensor", o.ID)
		}
		if c.Mode == coolingModeLive {
			switch {
			case !o.Pump && !onController:
				return fmt.Errorf("%q: not a fan of the TL controller", o.ID)
			case o.Pump && onController && !o.driven():
				return fmt.Errorf("%q: a pump on the fan controller needs a curve or a manual value", o.ID)
			case o.Pump && onController && o.MinDuty < cooling.LowestPumpDuty:
				return fmt.Errorf("%q: a pump needs at least %d %%", o.ID, cooling.LowestPumpDuty)
			case o.Pump && !onController && o.driven():
				if _, ok := pumpControl(o); !ok {
					return fmt.Errorf("%q: this pump cannot be set, it is not on a mainboard header", o.ID)
				}
				if o.MinDuty < cooling.LowestPumpDuty {
					return fmt.Errorf("%q: a pump needs at least %d %%", o.ID, cooling.LowestPumpDuty)
				}
			}
		}
	}
	if err := checkControlKeys(c.Outputs); err != nil {
		return err
	}
	if len(c.SensorNames) > coolingMaxNames {
		return fmt.Errorf("more than %d sensor names", coolingMaxNames)
	}
	for key, name := range c.SensorNames {
		if len(key) > coolingMaxNameLen || len(name) > coolingMaxNameLen {
			return errors.New("a sensor name is too long")
		}
	}
	curveOf := make(map[string]string, len(c.Outputs))
	for _, o := range c.Outputs {
		curveOf[o.ID] = o.curveRef()
	}
	return cooling.CheckCurves(c.specs(), curveOf)
}

// manualDuty is the duty a manual value stands for, and false if it cannot be
// told: an rpm value needs the output's speed table.
func (o coolingOutputConfig) manualDuty() (int, bool) {
	m := o.Manual
	if m == nil {
		return 0, false
	}
	if m.Unit != coolingUnitRPM {
		return int(math.Ceil(m.Value)), true
	}
	var sm cooling.SpeedMap
	for _, p := range o.Speeds {
		sm = append(sm, cooling.SpeedPoint{Duty: p[0], RPM: p[1]})
	}
	if len(sm) < 2 {
		return 0, false
	}
	return sm.DutyFor(m.Value), true
}

// checkManual refuses a manual value that the output could not keep: the limits
// are those of a curve, and a refused value is never changed quietly.
func (o coolingOutputConfig) checkManual() error {
	m := o.Manual
	if m == nil {
		return nil
	}
	if o.Curve != "" {
		return fmt.Errorf("%q: a curve or a manual value, not both", o.ID)
	}
	switch m.Unit {
	case "", "percent":
		if !finite(m.Value) || m.Value <= 0 || m.Value > 100 {
			return fmt.Errorf("%q: manual value must be between 1 and 100 %%", o.ID)
		}
	case coolingUnitRPM:
		if !finite(m.Value) || m.Value <= 0 || m.Value > cooling.HighestRPM {
			return fmt.Errorf("%q: manual value out of range", o.ID)
		}
		if len(o.Speeds) < 2 {
			return fmt.Errorf("%q: rpm needs a measured speed table", o.ID)
		}
	default:
		return fmt.Errorf("%q: unknown unit %q", o.ID, m.Unit)
	}
	duty, _ := o.manualDuty()
	switch {
	case o.Pump && duty < cooling.LowestPumpDuty:
		return fmt.Errorf("%q: a pump needs at least %d %%", o.ID, cooling.LowestPumpDuty)
	case duty < o.MinDuty:
		return fmt.Errorf("%q: below the minimum of %d %%", o.ID, o.MinDuty)
	case tlOutputID.MatchString(o.ID) && !o.Pump && duty < lianlitl.LowestDuty:
		return fmt.Errorf("%q: the fan controller needs at least %d %%", o.ID, lianlitl.LowestDuty)
	}
	return nil
}

// CoolingManualView is the manual value of an output as the GUI shows it.
type CoolingManualView struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// CoolingOutputView is one output as the GUI shows it. Pointers are nil while
// a value is unknown.
type CoolingOutputView struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Pump bool     `json:"pump,omitempty"`
	Temp *float64 `json:"temp"`
	RPM  *int     `json:"rpm"`
	Duty int      `json:"duty"`
	// Unregulated: nothing drives this output, a pump that is only watched. The
	// board decides its duty, so Duty is 0 and means nothing.
	Unregulated bool `json:"unregulated,omitempty"`
	// Manual is the fixed value of an output without a curve.
	Manual     *CoolingManualView `json:"manual,omitempty"`
	TargetRPM  int                `json:"targetRpm,omitempty"`
	SensorLost bool               `json:"sensorLost,omitempty"`
	PumpDown   bool               `json:"pumpDown,omitempty"`
	DeviceLost bool               `json:"deviceLost,omitempty"`
	Stalled    bool               `json:"stalled,omitempty"`
	WriteFail  bool               `json:"writeFailed,omitempty"`
	// Emergency: a sensor stayed at the emergency temperature, the output runs at
	// full duty whatever its curve or manual value says.
	Emergency bool `json:"emergency,omitempty"`
	// Held: a calibration drives this output. Duty is then still what the curve
	// decided, not what is set; the set duty and the speed are in Measure.
	Held bool `json:"held,omitempty"`
	// Calibrating: Held by a running measurement (not by the wait for the pump
	// check, which also holds). Measuring is then the duty it sends.
	Calibrating bool `json:"calibrating,omitempty"`
	// Unproven: a board pump the match check has not proven yet (pending or
	// checking; a failed check is NotMatched): the board keeps it and CaseHUD
	// writes nothing.
	Unproven bool `json:"unproven,omitempty"`
	// NotReleased: a pump on a mainboard header that no administrator released yet.
	NotReleased bool `json:"notReleased,omitempty"`
	// NotMatched: the check found that the pump's speed sensor does not belong to its
	// header, so the board keeps the pump. Matching: that check is running.
	NotMatched bool `json:"notMatched,omitempty"`
	Matching   bool `json:"matching,omitempty"`
	// NoSource: the check cannot run, the configuration has no temperature source.
	NoSource bool `json:"noSource,omitempty"`
	// Unguarded: a manual output that no temperature guards, because no curve of
	// the configuration has a sensor. It is still regulated.
	Unguarded bool `json:"unguarded,omitempty"`
	// MeasureBlock says why this output cannot be measured now (empty: it can), see measureBlockOf.
	MeasureBlock string `json:"measureBlock,omitempty"`
}

// CoolingView is the state of the cooling. Without Live it only computes.
type CoolingView struct {
	Configured bool `json:"configured"`
	// Mode is the stored mode, "live" or "preview"; Disabled: the area is off.
	Mode     string `json:"mode"`
	Disabled bool   `json:"disabled,omitempty"`
	// Available: a controller is attached, so there is something to set up.
	Available bool `json:"available,omitempty"`
	// Live: fans follow the decisions, unless Blocked names a reason.
	Live    bool                `json:"live,omitempty"`
	Blocked string              `json:"blocked,omitempty"`
	Program string              `json:"program,omitempty"` // with Blocked "program"
	Error   string              `json:"error,omitempty"`
	Outputs []CoolingOutputView `json:"outputs,omitempty"`
	Measure CoolingMeasureView  `json:"measure"`
	// Sensors holds the current temperature of every sensor.
	Sensors map[string]float64 `json:"sensors,omitempty"`
	// Alarm is what is wrong right now and must not wait for anybody to look.
	Alarm *CoolingAlarm `json:"alarm,omitempty"`
	// Revision changes with every stored configuration; Preset is the ID of the
	// preset the running configuration equals ("" when none does).
	Revision int64  `json:"revision"`
	Preset   string `json:"preset,omitempty"`
	// Restored: cooling.json could not be read at the start and its safety copy is
	// in use; RestoredFile is the name under which the damaged file was kept.
	Restored     bool   `json:"restored,omitempty"`
	RestoredFile string `json:"restoredFile,omitempty"`
	// SensorsDown: the sensor source (the service) does not answer.
	SensorsDown bool `json:"sensorsDown,omitempty"`
	// SensorsStale: readings are left out because they are old or frozen.
	SensorsStale bool `json:"sensorsStale,omitempty"`
	// SensorsDisplayOnly: the readings come from a source other than the sensor
	// service ("lhm" or "custom" in SensorSource). They are shown but never drive
	// the cooling, so the outputs stay at their safe speed.
	SensorsDisplayOnly bool   `json:"sensorsDisplayOnly,omitempty"`
	SensorSource       string `json:"sensorSource,omitempty"`
}

// coolingState is the running control. It is replaced as a whole (coolMu).
type coolingState struct {
	ctrl       *cooling.Controller
	cfg        coolingConfig
	fans       *tlFans            // nil unless live
	pumps      *boardPumps        // nil unless a pump on the mainboard is regulated
	ctx        context.Context    // of this loop
	cancel     context.CancelFunc // ends this loop
	done       chan struct{}      // closed when the loop has ended
	errMsg     string
	configured bool // a file exists, even with no outputs in it
}

func (e *Engine) coolingSnapshot() coolingState {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	return e.cooling
}

func (e *Engine) setCoolingState(c coolingState) {
	e.coolMu.Lock()
	defer e.coolMu.Unlock()
	e.cooling = c
}

// startCooling starts the control if cooling.json exists. A broken file is
// shown in the GUI and starts nothing.
func (e *Engine) startCooling() {
	cfg, err := e.loadOrRestoreCooling()
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil {
		err = e.launchCooling(cfg, coolingSensorWait)
	}
	if err != nil {
		msg := shortCoolingError(err)
		log.Printf("cooling: %s", msg)
		e.setCoolingState(coolingState{errMsg: msg, configured: true})
	}
}

func shortCoolingError(err error) string {
	msg := err.Error()
	if len(msg) > coolingMaxErrText {
		msg = msg[:coolingMaxErrText] + " ..."
	}
	return msg
}

// launchCooling starts a loop for cfg. It gives the sensors up to sensorWait
// before the first tick. Without outputs there is nothing to run.
func (e *Engine) launchCooling(cfg coolingConfig, sensorWait time.Duration) error {
	return e.launchCoolingAfter(cfg, sensorWait, nil, nil, nil)
}

// launchCoolingAfter is launchCooling for a loop that must not write before prev is
// closed, the end of an older loop that was stuck in a slow call when it was replaced.
// from are the pumps of an older loop that has ended and kept its line for this one.
// start are the duties the outputs stand at after that older loop (nil if unknown): the
// new one ramps from them instead of jumping to its first decision.
func (e *Engine) launchCoolingAfter(cfg coolingConfig, sensorWait time.Duration, prev <-chan struct{}, from *boardPumps, start map[string]int) error {
	defer from.endKept() // whatever of the old line is left when this returns
	if e.ctx != nil && e.ctx.Err() != nil {
		// The program is ending: only the file counts, no loop is started behind it.
		e.setCoolingState(coolingState{cfg: cfg, configured: true})
		return nil
	}
	if !e.coolingOn() { // the file stays as it is; switching the area on reads it
		e.setCoolingState(coolingState{cfg: cfg, configured: true})
		return nil
	}
	if len(cfg.regulated()) == 0 {
		e.setCoolingState(coolingState{cfg: cfg, configured: true})
		return nil
	}
	var ctrl *cooling.Controller
	var fans *tlFans
	var pumps *boardPumps
	var err error
	if cfg.Mode == coolingModeLive && !e.Demo {
		fans = &tlFans{open: coolingOpenFans, gate: e.rivalGate(), now: time.Now}
		pumps = newBoardPumps(cfg, e.prov, e.rivalGate())
		ctrl, err = newCoolingLive(cfg, e.prov, fans, pumps)
	} else {
		ctrl, err = newCoolingPreview(cfg, e.prov)
	}
	if err != nil {
		return err
	}
	ctrl.StartAt(start)
	ctx, cancel := context.WithCancel(e.ctx)
	if fans != nil {
		fans.ctx = ctx
		if cfg.hasFanController() {
			e.guardStarted(fans, e.coolingSnapshot().fans)
		}
	}
	done := make(chan struct{})
	var matched chan struct{}
	if pumps != nil {
		// A pump is written only once its header is proven to be the one behind its sensor.
		e.applyMatches(pumps)
		pumps.takeOver(from)
		for _, id := range pumps.unproven() {
			ctrl.Hold(id)
		}
		matched = make(chan struct{})
	}
	e.setCoolingState(coolingState{ctrl: ctrl, cfg: cfg, fans: fans, pumps: pumps, ctx: ctx, cancel: cancel, done: done, configured: true})
	if fans != nil {
		log.Printf("cooling: live, %d outputs", len(cfg.regulated()))
	} else {
		log.Printf("cooling: preview of %d outputs, nothing is sent to the hardware", len(cfg.regulated()))
	}
	if matched != nil {
		go func() {
			waitEnded(ctx, prev)
			e.runPumpMatching(ctx, ctrl, pumps, cfg, matched)
		}()
	}
	go func() {
		waitEnded(ctx, prev)
		e.runCooling(ctx, ctrl, cfg, fans, pumps, done, matched, sensorWait)
	}()
	return nil
}

// errMeasureNotRestored is what ends a live loop whose measurement did not give its fan back in time.
var errMeasureNotRestored = errors.New("cooling: a measured fan was not restored before the end")

// waitEnded returns when prev is closed or ctx ends. A nil prev does not wait.
func waitEnded(ctx context.Context, prev <-chan struct{}) {
	if prev == nil {
		return
	}
	select {
	case <-prev:
	case <-ctx.Done():
	}
}

// runCooling runs the loop. The sensors get time first, see waitForSensors. A
// measurement on this loop ends with a write of its own, so the controller stays
// open until it is through.
func (e *Engine) runCooling(ctx context.Context, ctrl *cooling.Controller, cfg coolingConfig, fans *tlFans, pumps *boardPumps, done chan struct{}, matched <-chan struct{}, sensorWait time.Duration) {
	defer close(done)
	// However the loop ends, the board takes its pump back, after a check in
	// progress has handed its header back.
	defer pumps.Close()
	if matched != nil {
		defer func() { <-matched }()
	}
	waitForSensors(ctx, cfg, func() map[string]float64 { return coolingTemps(e.prov) }, sensorWait)
	ctrl.Run(ctx, coolingTick, coolingReporter())
	if fans != nil {
		restored := e.waitCoolingMeasure()
		fans.Close()
		end := ctrl.EndError()
		if !restored {
			// The measured fan may not be back at its safe duty: the guard stays.
			end = errors.Join(end, errMeasureNotRestored)
		}
		if cfg.hasFanController() { // the guard watches fans of the TL controller only
			e.guardLoopEnded(end)
		}
	}
}

// coolingReporter logs what goes wrong once per change, not once per tick.
func coolingReporter() func(error) {
	var last string
	return func(err error) {
		msg := err.Error()
		if msg == last {
			return
		}
		last = msg
		if len(msg) > coolingMaxErrText {
			msg = msg[:coolingMaxErrText] + " ..."
		}
		log.Printf("cooling: %s", msg)
	}
}

// newCoolingLive builds a controller that drives what can be driven: everything on
// the TL controller, and a pump on a mainboard header if it has a curve. A pump
// without a curve is only watched and stays with the board.
func newCoolingLive(cfg coolingConfig, prov *SafeProvider, fans cooling.Hardware, pumps *boardPumps) (*cooling.Controller, error) {
	outputs := cfg.regulated()
	list := make([]cooling.Fan, len(outputs))
	var fanIDs, drivenIDs, watchedIDs []string
	rpmSensors := map[string]string{}
	safe := map[string]int{}
	for i, o := range outputs {
		list[i] = o.fan()
		switch {
		case tlOutputID.MatchString(o.ID):
			fanIDs = append(fanIDs, o.ID)
			safe[o.ID] = max(o.SafeDuty, cooling.LowestSafeDuty)
		case o.driven():
			drivenIDs = append(drivenIDs, o.ID)
		default:
			watchedIDs = append(watchedIDs, o.ID)
			rpmSensors[o.ID] = o.RPMSensor
		}
	}
	readRPM := func() map[string]int { return coolingSpeeds(prov, rpmSensors) }
	routes := []cooling.Route{{Device: cooling.NoOutput{RPM: readRPM}, IDs: watchedIDs}}
	if len(fanIDs) > 0 { // only a pump to watch: the fan controller stays closed
		routes = append(routes, cooling.Route{Device: fans, IDs: fanIDs})
	}
	if len(drivenIDs) > 0 {
		routes = append(routes, cooling.Route{Device: pumps, IDs: drivenIDs})
	}
	if t, ok := fans.(*tlFans); ok {
		t.setSafe(safe)
	}
	hw := cooling.Join(routes...)
	readTemps := func() map[string]float64 { return coolingTemps(prov) }
	return cooling.NewShared(cfg.specs(), list, hw, readTemps)
}

// newCoolingPreview builds a controller whose hardware is cooling.NoOutput.
func newCoolingPreview(cfg coolingConfig, prov *SafeProvider) (*cooling.Controller, error) {
	outputs := cfg.regulated()
	fans := make([]cooling.Fan, len(outputs))
	rpmSensors := map[string]string{} // sensor key by output
	for i, o := range outputs {
		fans[i] = o.fan()
		if o.RPMSensor != "" {
			rpmSensors[o.ID] = o.RPMSensor
		}
	}
	readRPM := func() map[string]int { return coolingSpeeds(prov, rpmSensors) }
	readTemps := func() map[string]float64 { return coolingTemps(prov) }
	return cooling.NewShared(cfg.specs(), fans, cooling.NoOutput{RPM: readRPM}, readTemps)
}

// coolingTemps returns the temperatures by exact sensor key. The provider drops
// its readings when a refresh fails, and Fresh leaves out those that are stale or
// frozen, so nothing old is passed on.
func coolingTemps(prov *SafeProvider) map[string]float64 {
	out := map[string]float64{}
	for _, r := range prov.Fresh() {
		if r.Unit == "°C" && finite(r.Value) {
			out[r.Key] = r.Value
		}
	}
	return out
}

// finite tells a usable reading from NaN and infinity, which the sensor service
// may report and which cannot be calculated with or sent as JSON.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func coolingSpeeds(prov *SafeProvider, sensors map[string]string) map[string]int {
	if len(sensors) == 0 {
		return nil
	}
	rpm := map[string]float64{}
	for _, r := range prov.Fresh() {
		if r.Unit == "RPM" && finite(r.Value) {
			rpm[r.Key] = r.Value
		}
	}
	out := map[string]int{}
	for id, key := range sensors {
		if v, ok := rpm[key]; ok {
			out[id] = int(v + 0.5)
		}
	}
	return out
}

// CoolingView returns the current state of the cooling.
// coolingReadings returns the current temperatures, for the plan to show.
func (e *Engine) coolingReadings() map[string]float64 {
	if e.prov == nil {
		return nil
	}
	return coolingTemps(e.prov)
}

func (e *Engine) CoolingView() CoolingView {
	rev := e.coolingRevision()
	c := e.coolingSnapshot()
	v := e.coolingViewOf(c)
	v.Revision, v.Preset = rev, e.activeCoolingPreset(c)
	return v
}

// coolingViewOf describes the control c.
func (e *Engine) coolingViewOf(c coolingState) CoolingView {
	if !e.coolingOn() {
		return CoolingView{Mode: modeName(c.cfg.Mode), Disabled: true}
	}
	if c.ctrl == nil {
		v := CoolingView{
			Configured: c.configured || c.errMsg != "", Error: c.errMsg, Mode: modeName(c.cfg.Mode),
			// Live with nothing on a curve: no loop runs, but the page must not
			// say another program is in charge.
			Live:      c.cfg.Mode == coolingModeLive && !e.Demo && c.errMsg == "",
			Available: e.Demo || lianlitl.Present(), Measure: e.CoolingMeasure(),
			Sensors: e.coolingReadings(),
		}
		e.addCrashInfo(&v)
		v.Alarm = coolingAlarmOf(v, time.Time{}, time.Now())
		return v
	}
	statuses := c.ctrl.Status()
	outputs := c.cfg.regulated()
	v := CoolingView{Configured: true, Available: true, Mode: modeName(c.cfg.Mode), Measure: e.CoolingMeasure(), Outputs: make([]CoolingOutputView, len(statuses)),
		Sensors: e.coolingReadings()}
	if c.fans != nil {
		v.Live = true
		v.Blocked, v.Program = c.blockedNow()
	}
	var guard []string // the sensors guarding the manual outputs, see specs
	for i, s := range statuses {
		o := CoolingOutputView{
			ID: s.ID, Name: outputs[i].Name, Pump: outputs[i].Pump,
			Duty: s.Duty, TargetRPM: s.TargetRPM, SensorLost: s.SensorLost, PumpDown: s.PumpDown, Emergency: s.Emergency,
			DeviceLost: s.DeviceLost, Stalled: s.Stalled, Held: s.Held, WriteFail: s.WriteFailed || c.pumps.failing(s.ID),
			NotReleased: c.pumps.notReleasedFor(s.ID),
			NotMatched:  c.pumps.isState(s.ID, pumpNotMatched), Matching: c.pumps.isState(s.ID, pumpChecking),
			NoSource:     c.pumps.noSourceFor(c.cfg, s.ID),
			MeasureBlock: e.measureBlockOf(c, outputs[i]),
		}
		o.Unproven = c.pumps.unprovenFor(s.ID)
		o.Calibrating = s.Held && v.Measure.Running && v.Measure.ID == s.ID
		if !outputs[i].driven() {
			o.Unregulated, o.Duty, o.TargetRPM = true, 0, 0
		}
		if m := outputs[i].Manual; m != nil {
			o.Manual = &CoolingManualView{Value: m.Value, Unit: firstOf(m.Unit, "percent")}
			if guard == nil {
				guard = c.cfg.measureSources()
			}
			o.Unguarded = len(guard) == 0 && !o.NoSource
		}
		if s.HasTemp {
			o.Temp = &s.Temp
		}
		if s.HasRPM {
			o.RPM = &s.RPM
		}
		v.Outputs[i] = o
	}
	e.addCrashInfo(&v)
	v.Alarm = coolingAlarmOf(v, c.ctrl.LastTick(), time.Now())
	if v.Live && e.halted.Load() {
		v.Alarm = &CoolingAlarm{Kind: coolingAlarmHalted} // nothing regulates any more: first
	}
	return v
}
