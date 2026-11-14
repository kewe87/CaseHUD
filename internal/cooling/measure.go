package cooling

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Measuring. Fans can take a long time to reach a new speed, and how long
// differs between models. So no step has a fixed duration: each one lasts until
// the speed has stopped moving, up to a limit. A step that hits the limit is
// reported as not settled and does not enter the speed map.
const (
	measurePoll      = 3 * time.Second   // time between two readings
	measureMinWait   = 15 * time.Second  // no judgement before this
	measureStartWait = 30 * time.Second  // a standstill is not believed before this
	measureWindow    = 6                 // readings that must agree
	measureTolerance = 60                // rpm; speeds are reported in such steps
	measureMaxWait   = 120 * time.Second // limit per step
	measureMaxRise   = 10.0              // °C above the start that ends the run
	measureMaxTemp   = 80.0              // °C that end the run or keep it from starting
	measureTopDuty   = 90                // a map must reach at least this duty
)

// MeasureMaxTemp is the temperature at which no fan is measured and a run ends.
const MeasureMaxTemp = measureMaxTemp

// Limits of the guard while a pump is measured. A pump is the only supply of
// coolant flow, so its guard watches the hottest of all temperatures and ends
// a little earlier than the one of a fan.
const (
	PumpMeasureMaxRise = 10.0 // °C above the start that ends a pump measurement
	PumpMeasureMaxTemp = 75.0 // °C that end a pump measurement or keep it from starting
)

// MeasureSteps are the duties measured, from fast to slow, so the fan runs
// when the slow steps come.
var MeasureSteps = []int{100, 90, 80, 70, 60, 50, 40, 30, 20, 15}

// ErrNoSpeed is returned when an output never reported a speed.
var ErrNoSpeed = errors.New("cooling: the fan reports no speed")

// MeasuredStep is the result of one duty.
type MeasuredStep struct {
	Duty    int  `json:"duty"`
	RPM     int  `json:"rpm"`
	Settled bool `json:"settled"`
}

// MeasureDevice is what measuring needs from the hardware.
type MeasureDevice interface {
	TakeControl(id string) error
	SetDuty(id string, percent int) error
	Speeds() (map[string]int, error)
}

// Measurement describes one run.
type Measurement struct {
	Device MeasureDevice
	ID     string
	Steps  []int
	// Wait sleeps and returns the context's error if it ends first.
	Wait func(context.Context, time.Duration) error
	// Check may end the run, for example when the parts get too hot.
	Check func() error
	// Progress is told about every reading; may be nil.
	Progress func(step, duty, rpm int)
	// Update is told about every reading of a calibration, with its phase; may be nil.
	Update func(CalibrationProgress)
}

// Run measures every step. The caller leaves the fan at a safe duty afterwards;
// the output must not be driven by anything else meanwhile (Controller.Hold).
// On an error the steps measured so far are returned with it.
func (m Measurement) Run(ctx context.Context) ([]MeasuredStep, error) {
	var out []MeasuredStep
	if err := m.Device.TakeControl(m.ID); err != nil {
		return out, fmt.Errorf("cooling: taking over %s: %w", m.ID, err)
	}
	for i, duty := range m.Steps {
		step, err := m.step(ctx, i, len(m.Steps), duty)
		if err != nil {
			return out, err
		}
		out = append(out, step)
	}
	if !slices.ContainsFunc(out, func(s MeasuredStep) bool { return s.RPM > 0 }) {
		return out, ErrNoSpeed
	}
	return out, nil
}

func (m Measurement) step(ctx context.Context, index, total, duty int) (MeasuredStep, error) {
	if err := m.Device.SetDuty(m.ID, duty); err != nil {
		return MeasuredStep{}, fmt.Errorf("cooling: setting %s: %w", m.ID, err)
	}
	var seen []int
	for waited := time.Duration(0); ; {
		if err := m.Wait(ctx, measurePoll); err != nil {
			return MeasuredStep{}, err
		}
		waited += measurePoll
		if m.Check != nil {
			if err := m.Check(); err != nil {
				return MeasuredStep{}, err
			}
		}
		speeds, err := m.Device.Speeds()
		if err != nil {
			return MeasuredStep{}, fmt.Errorf("cooling: reading speeds: %w", err)
		}
		rpm, ok := speeds[m.ID]
		if !ok {
			return MeasuredStep{}, fmt.Errorf("cooling: %s is gone", m.ID)
		}
		if m.Progress != nil {
			m.Progress(index, duty, rpm)
		}
		m.update(CalibrationProgress{Phase: PhaseDown, Step: index, Steps: total, Duty: duty, RPM: rpm})
		seen = append(seen, rpm)
		if len(seen) > measureWindow {
			seen = seen[1:]
		}
		settled := len(seen) == measureWindow && spread(seen) <= measureTolerance
		if settled && waited >= measureMinWait && (rpm > 0 || waited >= measureStartWait) {
			return MeasuredStep{Duty: duty, RPM: median(seen), Settled: true}, nil
		}
		if waited >= measureMaxWait {
			return MeasuredStep{Duty: duty, RPM: median(seen), Settled: false}, nil
		}
	}
}

func spread(v []int) int { return slices.Max(v) - slices.Min(v) }

func median(v []int) int {
	s := slices.Sorted(slices.Values(v))
	return s[len(s)/2]
}

// SpeedMapFrom builds a speed map from the settled steps that turned. A step
// slower than a lower duty's is lifted to it, as long as the difference is within
// what the fan's report can tell apart. Curves are turned into duties through
// this map, so a map that misses the fast end, falls or stays flat is refused:
// it would hold the fan at a low duty whatever the temperature.
func SpeedMapFrom(steps []MeasuredStep) (SpeedMap, error) {
	good := slices.DeleteFunc(slices.Clone(steps), func(s MeasuredStep) bool { return !s.Settled || s.RPM <= 0 })
	slices.SortFunc(good, func(a, b MeasuredStep) int { return a.Duty - b.Duty })
	var m SpeedMap
	for _, s := range good {
		rpm := s.RPM
		if len(m) > 0 {
			last := m[len(m)-1]
			if last.RPM-rpm > measureTolerance {
				return nil, fmt.Errorf("cooling: %d rpm at %d %% but %d rpm at %d %%", last.RPM, last.Duty, rpm, s.Duty)
			}
			rpm = max(rpm, last.RPM)
		}
		m = append(m, SpeedPoint{Duty: s.Duty, RPM: rpm})
	}
	m = withoutFlatInterior(m)
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("cooling: too few usable steps: %w", err)
	}
	if top := m[len(m)-1]; top.Duty < measureTopDuty {
		return nil, fmt.Errorf("cooling: no settled speed at %d %% or more", measureTopDuty)
	} else if top.RPM-m[0].RPM <= measureTolerance {
		return nil, errors.New("cooling: the speed does not follow the duty")
	}
	return m, nil
}

// withoutFlatInterior drops the points inside a run of equal speeds, keeping its
// first and last. The map says the same, but a calibration that measures every
// percent at the low end does not fill it with copies.
func withoutFlatInterior(m SpeedMap) SpeedMap {
	var out SpeedMap
	for i, p := range m {
		if i > 0 && i < len(m)-1 && p.RPM == m[i-1].RPM && p.RPM == m[i+1].RPM {
			continue
		}
		out = append(out, p)
	}
	return out
}

// WaitContext is the Wait of a real run.
func WaitContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RiseGuard returns a Check that ends a run when the temperature read by temp
// climbs measureMaxRise above its first value, reaches measureMaxTemp or can no
// longer be read.
func RiseGuard(temp func() (float64, bool)) func() error {
	return RiseGuardAt(temp, measureMaxRise, measureMaxTemp)
}

// RiseGuardAt is RiseGuard with the limits given.
func RiseGuardAt(temp func() (float64, bool), maxRise, maxTemp float64) func() error {
	var start float64
	var started bool
	return func() error {
		t, ok := temp()
		if !ok {
			return ErrSensorLost
		}
		if !started {
			start, started = t, true
		}
		if t >= maxTemp {
			return fmt.Errorf("cooling: %.0f °C is too hot for measuring", t)
		}
		if t-start >= maxRise {
			return fmt.Errorf("cooling: temperature rose from %.0f to %.0f °C", start, t)
		}
		return nil
	}
}
