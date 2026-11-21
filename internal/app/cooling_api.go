package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Limits of the public cooling answer.
const (
	coolingAPIMaxName = 60
	coolingAPIMaxList = 64
)

// States of the cooling as the API tells them.
const (
	CoolingStateRunning      = "running"
	CoolingStateTest         = "test"
	CoolingStateBlocked      = "blocked"
	CoolingStateOff          = "off"
	CoolingStateUnconfigured = "unconfigured"
)

// CoolingAPI is the cooling as GET /api/v1/cooling tells it.
type CoolingAPI struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
	// Mode is the stored mode, "preview" (test) or "live"; Revision is the
	// revision of the stored configuration, see GET /cooling/config.
	Mode      string             `json:"mode"`
	Revision  int64              `json:"revision"`
	BlockedBy string             `json:"blockedBy"`
	Preset    *string            `json:"preset"`
	Presets   []CoolingPresetAPI `json:"presets"`
	Alarm     *CoolingAPIAlarm   `json:"alarm"`
	// SensorsDisplayOnly is true while the sensor readings come from a source
	// other than the CaseHUD sensor service (SensorSource "lhm" or "custom"):
	// they are shown, but the cooling does not use them and holds the safe
	// speeds. SensorSource is "" while the service or nothing delivers.
	SensorsDisplayOnly bool               `json:"sensorsDisplayOnly"`
	SensorSource       string             `json:"sensorSource"`
	Outputs            []CoolingAPIOutput `json:"outputs"`
	Temps              []CoolingAPITemp   `json:"temperatures"`
}

// CoolingPresetAPI is a preset in the public answer.
type CoolingPresetAPI struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CoolingAPIAlarm is the worst thing wrong, as a code and the name of the output.
type CoolingAPIAlarm struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// CoolingAPIOutput is one fan or pump the control looks after.
type CoolingAPIOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	RPM  *int   `json:"rpm"`
	// Percent is the duty CaseHUD sends (0-100) and is null exactly when that
	// is not what the hardware gets: unregulated (mode "none"), a board pump
	// CaseHUD does not control yet (notReleased, notMatched, noSource, or not
	// proven yet by the match check), or an output in calibration with no
	// measured duty yet. While calibrating it is the duty the measurement
	// sends. Mode "curve" or "manual" otherwise always has a number.
	Percent *int `json:"percent"`
	// Mode is "curve", "manual" or "none" (not regulated, a pump that is only watched).
	Mode string `json:"mode"`
	// Manual is the fixed value of a manual output.
	Manual *CoolingManualView `json:"manual,omitempty"`
	// Unregulated is true exactly for mode "none": nothing drives the output,
	// the board decides its duty.
	Unregulated bool `json:"unregulated"`
	// Calibrating: a running calibration or measurement (fan or pump) holds
	// the output; percent is then the duty the measurement sends.
	Calibrating bool `json:"calibrating"`
	// Unproven: a board pump whose speed sensor and control pairing is not
	// proven yet (check pending or running). The board keeps the pump, CaseHUD
	// writes nothing and percent is null. A failed check is notMatched.
	Unproven bool `json:"unproven"`
	// NotReleased: a pump on a mainboard header that is not released yet (done
	// in the window only). NotMatched: its speed sensor did not react, see
	// POST /cooling/recheck-pump. NoSource: the check cannot run, the settings
	// have no temperature source.
	NotReleased bool     `json:"notReleased"`
	NotMatched  bool     `json:"notMatched"`
	NoSource    bool     `json:"noSource"`
	Temp        *float64 `json:"temp"`
	Problem     string   `json:"problem"`
}

// CoolingAPITemp is a temperature the cooling uses or names.
type CoolingAPITemp struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// CoolingAPIView builds the public answer from one snapshot of the control.
func (e *Engine) CoolingAPIView() CoolingAPI {
	out := CoolingAPI{State: CoolingStateOff, Mode: "preview", Presets: []CoolingPresetAPI{}, Outputs: []CoolingAPIOutput{}, Temps: []CoolingAPITemp{}}
	if !e.coolingOn() {
		return out
	}
	rev := e.coolingRevision() // before the snapshot: at worst too old
	c := e.coolingSnapshot()
	out.Enabled, out.Mode, out.Revision = true, modeName(c.cfg.Mode), rev
	list, keys, perr := e.Store.readCoolingPresets() // unreadable: no presets, the GUI says why
	for _, p := range list {
		out.Presets = append(out.Presets, CoolingPresetAPI{ID: p.ID, Name: clipName(p.Name, coolingAPIMaxName)})
	}
	v := e.coolingViewOf(c)
	switch {
	case !c.usable():
		out.State = CoolingStateUnconfigured
	case v.Blocked != "":
		out.State = CoolingStateBlocked
		if v.Blocked == coolingBlockedProgram {
			out.BlockedBy = clipName(v.Program, coolingAPIMaxName)
		}
	case v.Live:
		out.State = CoolingStateRunning
	default:
		out.State = CoolingStateTest
	}
	if perr == nil {
		if id := e.activeOf(c, list, keys); id != "" {
			out.Preset = &id
		}
	}
	out.Alarm = apiAlarm(v)
	out.SensorsDisplayOnly, out.SensorSource = v.SensorsDisplayOnly, v.SensorSource
	if out.State == CoolingStateUnconfigured {
		return out
	}
	regulated := c.cfg.regulated()
	for i, o := range v.Outputs {
		if i >= len(regulated) || len(out.Outputs) >= coolingAPIMaxList {
			break
		}
		cfg := regulated[i]
		ao := CoolingAPIOutput{ID: o.ID, Name: clipName(firstOf(cfg.Name, cfg.ID), coolingAPIMaxName), Kind: "fan",
			Mode: cfg.mode(), Manual: o.Manual, Problem: outputProblem(o),
			Unregulated: o.Unregulated, NotReleased: o.NotReleased, NotMatched: o.NotMatched, NoSource: o.NoSource, Calibrating: o.Calibrating, Unproven: o.Unproven}
		ao.Percent = apiPercent(o, v.Measure)
		switch {
		case cfg.Pump:
			ao.Kind = "pump"
		case cfg.Role == coolingRoleCase:
			ao.Kind = "case"
		}
		ao.RPM = o.RPM
		if o.Temp != nil && finite(*o.Temp) {
			t := math.Round(*o.Temp*10) / 10
			ao.Temp = &t
		}
		out.Outputs = append(out.Outputs, ao)
	}
	out.Temps = apiTemps(c.cfg, v.Sensors, e.sensorNames())
	return out
}

// apiPercent is the duty CaseHUD sends to the output, or nil where that is not
// what the hardware gets.
func apiPercent(o CoolingOutputView, m CoolingMeasureView) *int {
	var duty int
	switch {
	case o.Calibrating:
		if m.Phase == "" {
			return nil // the measurement has not sent anything yet
		}
		duty = m.Duty
	case o.Unregulated, o.NotReleased, o.NotMatched, o.NoSource, o.Unproven:
		return nil
	default:
		duty = o.Duty
	}
	p := min(100, max(0, duty))
	return &p
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// outputProblem is the most serious problem of an output, or "".
func outputProblem(o CoolingOutputView) string {
	switch {
	case o.PumpDown:
		return "pump_down"
	case o.DeviceLost:
		return "device_lost"
	case o.WriteFail:
		return "write_failed"
	case o.Emergency:
		return "emergency"
	case o.Stalled:
		return "stalled"
	case o.SensorLost:
		return "sensor_lost"
	case o.NotMatched:
		return "not_matched"
	case o.NotReleased:
		return "not_released"
	case o.Unguarded:
		return "unguarded"
	}
	return ""
}

// apiAlarm maps the alarm of the GUI to the codes of the API. A block by
// another program is not an alarm here: the state says it.
func apiAlarm(v CoolingView) *CoolingAPIAlarm {
	a := v.Alarm
	if a == nil {
		return nil
	}
	code := a.Kind
	switch code {
	case coolingAlarmPumpUnknown:
		code = "pump_unknown"
	case coolingAlarmConfigCopy:
		// The name is the damaged file kept in the data folder: nothing of the
		// file system leaves the program.
		return &CoolingAPIAlarm{Code: "config_backup"}
	case coolingAlarmPump, coolingAlarmStale, coolingAlarmConfig, coolingAlarmDevice, coolingAlarmWrite, coolingAlarmSensor, coolingAlarmStalled, coolingAlarmService, coolingAlarmHalted, coolingAlarmEmergency:
	default:
		return nil
	}
	// The GUI falls back to the ID where an output has no name; the API gives
	// only a name the user chose.
	name := a.Name
	for _, o := range v.Outputs {
		if o.ID == name && o.Name == "" {
			name = ""
		}
	}
	return &CoolingAPIAlarm{Code: code, Name: clipName(name, coolingAPIMaxName)}
}

// sensorNames returns the names of all sensors by key.
func (e *Engine) sensorNames() map[string]string {
	m := map[string]string{}
	if e.prov == nil {
		return m
	}
	for _, r := range e.prov.All() {
		m[r.Key] = r.Path
	}
	return m
}

// apiTemps lists the temperatures the cooling uses: the sources of its curves
// and the sensors it has a name for. A sensor without a value is left out.
func apiTemps(cfg coolingConfig, temps map[string]float64, paths map[string]string) []CoolingAPITemp {
	keys := map[string]bool{}
	seen := map[string]bool{}
	for _, c := range cfg.Curves {
		for _, k := range cfg.sensors(c.ID, seen) {
			keys[k] = true
		}
	}
	for k := range cfg.SensorNames {
		keys[k] = true
	}
	out := []CoolingAPITemp{}
	for k := range keys {
		v, ok := temps[k]
		if !ok || !finite(v) {
			continue
		}
		name := strings.TrimSpace(cfg.SensorNames[k])
		if name == "" {
			name = firstOf(paths[k], k)
		}
		out = append(out, CoolingAPITemp{ID: k, Name: clipName(name, coolingAPIMaxName), Value: math.Round(v*10) / 10})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > coolingAPIMaxList {
		out = out[:coolingAPIMaxList]
	}
	return out
}

// EventKey is what the cooling event watches: the preset, the state, the alarm
// and the problem of each output, but not the measured values.
func (v CoolingAPI) EventKey() string {
	var b strings.Builder
	preset := ""
	if v.Preset != nil {
		preset = *v.Preset
	}
	fmt.Fprintf(&b, "%t|%s|%s|%s|%t", v.Enabled, v.State, v.BlockedBy, preset, v.SensorsDisplayOnly)
	if v.Alarm != nil {
		fmt.Fprintf(&b, "|%s|%s", v.Alarm.Code, v.Alarm.Name)
	}
	for _, o := range v.Outputs {
		fmt.Fprintf(&b, "|%s=%s", o.ID, o.Problem)
	}
	return b.String()
}
