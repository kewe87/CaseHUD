package cooling

import (
	"errors"
	"fmt"
	"math"
)

const (
	sensorGraceTicks = 3  // missing readings tolerated before going to SafeDuty
	spinUpTicks      = 5  // how long a starting fan may take to report a speed
	stallStep        = 3  // extra duty per tick while a fan does not turn
	trendWindow      = 60 // readings the temperature trend is fitted over
	trendMinimum     = 20 // readings needed before a trend is trusted
	maxLeadDegrees   = 3  // the furthest a trend may move a reading, °C

	// Limits of the configuration. Beyond them an output would look regulated
	// but no longer follow its temperature.
	longestDelayTicks = 3600
	longestRiseTicks  = 30   // a rising temperature must not wait longer
	widestDistance    = 20.0 // °C

	// LowestSafeDuty is the least a fan may be left at when the control ends,
	// is lost or loses its sensors. Configurations below it are refused.
	LowestSafeDuty = 30

	// LowestPumpDuty is the least a pump is ever given: its MinDuty must not be
	// lower, and no write, measurement or check goes below it.
	LowestPumpDuty = LowestSafeDuty

	// emergencyTemp sends an output to full duty after emergencyTicks in a row,
	// whatever its curve, filter and steps say. Parts that normally run near
	// 95 °C under load must not set it off.
	emergencyTemp  = 100.0 // °C
	emergencyTicks = 3
)

// Fan is the configuration of one output. An output without Pump set is a
// plain fan; nothing else is needed for a system without water cooling.
type Fan struct {
	ID string
	// CurveID names a curve of the list given to NewShared. Without it, Sources,
	// Curve and the settings around them form a curve for this output alone.
	CurveID string
	// Sources are the sensors this output follows; the hottest one counts.
	// If any of them has no reading, the output goes to SafeDuty.
	Sources []string
	Curve   Curve
	// RPMCurve means the curve's values are speeds, which Speeds turns into
	// duties. Otherwise they are percent.
	RPMCurve bool
	// Speeds is what this fan was measured to do. It is what lets outputs share
	// one rpm curve although each fan needs its own duty for the same speed.
	Speeds SpeedMap

	// A new temperature only counts once it differs from the last accepted one
	// by RiseBy or FallBy for RiseAfter or FallAfter ticks in a row. The ends
	// of the curve can be reached without the distance.
	RiseBy, FallBy       float64
	RiseAfter, FallAfter int

	// Offset is added to what the curve asks for, before MinDuty counts.
	Offset int

	// StepUp and StepDown limit the change of duty per tick.
	StepUp, StepDown int
	// MinDuty is never undercut, so a value above 0 keeps the fan turning.
	MinDuty int
	// StopAt stops the fan when its duty falls to this value.
	StopAt int
	// StartDuty is held for a few ticks when a stopped or standing fan has to
	// start, and from there the duty moves on by its steps. Never less than what
	// it takes to stay above StopAt.
	StartDuty int
	// SafeDuty is used without a valid temperature and when the loop ends.
	SafeDuty int

	// LeadTicks reacts to a rising temperature ahead of time, for slow sources
	// such as coolant: the reading is moved by the trend over this many ticks,
	// by at most 3 °C. 0 turns it off. A falling trend is ignored.
	LeadTicks int

	// Pump marks a coolant pump. It never stops, needs MinDuty, and has no
	// start phase. If it stands still, it is broken and the system is told
	// (ErrPumpStopped). Without a curve and sensors it is only watched.
	Pump bool
	// StoppedBelowRPM counts a pump as stopped below this speed; 0 means only
	// at standstill.
	StoppedBelowRPM int
}

// ownCurve is the curve this output describes itself.
func (f Fan) ownCurve() CurveSpec {
	return CurveSpec{
		ID: f.ID, RPM: f.RPMCurve, Sources: f.Sources, Points: f.Curve, RiseBy: f.RiseBy, FallBy: f.FallBy,
		RiseAfter: f.RiseAfter, FallAfter: f.FallAfter, LeadTicks: f.LeadTicks,
	}
}

// validate checks the output's own settings against the most its curve can ask
// for.
func (f Fan) validate(top float64) error {
	var errs []error
	check := func(ok bool, what string) {
		if !ok {
			errs = append(errs, errors.New(what))
		}
	}
	check(f.ID != "", "no ID")
	watchOnly := f.watchOnly()
	check(f.StepUp >= 1 && f.StepDown >= 1, "steps must be at least 1")
	check(inPercent(f.MinDuty) && inPercent(f.StopAt) && inPercent(f.StartDuty), "duty out of range")
	check(f.Offset >= -100 && f.Offset <= 100, "offset out of range")
	check(f.SafeDuty >= LowestSafeDuty && f.SafeDuty <= 100, fmt.Sprintf("SafeDuty must be %d..100", LowestSafeDuty))
	check(f.SafeDuty >= f.MinDuty, "SafeDuty below MinDuty")
	check(f.MinDuty == 0 || f.StopAt < f.MinDuty, "StopAt would undercut MinDuty")
	if f.Pump {
		check(f.MinDuty >= LowestPumpDuty, fmt.Sprintf("a pump needs MinDuty of at least %d", LowestPumpDuty))
		check(f.StopAt == 0 && f.StartDuty == 0, "a pump has no stop or start phase")
		check(f.StoppedBelowRPM >= 0, "negative StoppedBelowRPM")
	} else {
		check(f.StoppedBelowRPM == 0, "StoppedBelowRPM is for pumps")
	}
	speedsOK := true
	if f.Speeds != nil {
		if err := f.Speeds.validate(); err != nil {
			errs = append(errs, err)
			speedsOK = false
		}
	} else if f.RPMCurve {
		check(false, "an rpm curve needs a measured fan")
		speedsOK = false
	}
	if !watchOnly {
		if speedsOK {
			r := regulator{cfg: f}
			check(r.wantedDuty(top) > 0, "curve never asks for any speed")
			check(f.StopAt == 0 || r.wantedDuty(top) > f.StopAt, "StopAt reaches the top of the curve")
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("cooling: %q: %w", f.ID, err)
	}
	return nil
}

// watchOnly tells a pump without a curve: nothing asks it for a speed, but its
// standstill still counts.
func (f Fan) watchOnly() bool { return f.Pump && len(f.Curve) == 0 && f.CurveID == "" }

func inPercent(v int) bool { return v >= 0 && v <= 100 }

// result is what one output does in one tick.
type result struct {
	duty       int
	sensorLost bool
	pumpDown   bool
	emergency  bool // full duty by the emergency rule
}

// regulator holds the state of one output. It is advanced once per tick and
// does no I/O.
type regulator struct {
	cfg     Fan
	started bool
	duty    int

	missing int
	hot     int // ticks in a row at emergencyTemp
	holding int
	idle    int // ticks without rotation at a running duty
}

// step advances the regulator with what its curve says. rpmOK tells whether the
// speed can be used; an unknown speed is never taken for a stopped fan.
func (r *regulator) step(sig signal, rpm int, rpmOK bool) result {
	r.idle = r.updateIdle(rpm, rpmOK)
	stalled := r.idle >= spinUpTicks
	pumpDown := r.cfg.Pump && stalled
	if r.cfg.watchOnly() {
		r.duty = r.cfg.MinDuty
		if pumpDown {
			r.duty = 100
		}
		return result{duty: r.duty, pumpDown: pumpDown}
	}
	// Before anything else: it also holds while another sensor of the curve is gone.
	if t, ok := sig.watched(); ok && t >= emergencyTemp {
		r.hot++
	} else {
		r.hot = 0
	}
	if !sig.ok {
		res := r.stepWithoutSensor()
		if pumpDown {
			r.duty = 100
			res.duty, res.pumpDown = 100, true
		} else if r.hot >= emergencyTicks {
			r.duty, res.duty, res.emergency = 100, 100, true
		}
		return res
	}
	first := !r.started
	r.started = true
	r.missing = 0

	wanted := r.wantedDuty(sig.value)

	switch {
	case pumpDown:
		r.duty = 100
		return result{duty: r.duty, pumpDown: true}
	case r.hot >= emergencyTicks:
		r.duty, r.holding = 100, 0
		return result{duty: r.duty, emergency: true}
	default:
		r.duty = r.nextDuty(wanted, rpm, rpmOK, stalled, first)
	}
	return result{duty: r.duty}
}

// resumeAt makes the regulator continue from a duty the output really stands at,
// so that the next step is limited by StepUp and StepDown like any other.
func (r *regulator) resumeAt(duty int) {
	r.duty, r.started, r.holding = duty, true, 0
}

// nextDuty is the normal way from one duty to the next: through the start phase
// if the output has one, then by its steps. A fan that stands still is never
// lowered for that: it may just be a bad reading while the part runs hot.
func (r *regulator) nextDuty(wanted, rpm int, rpmOK, stalled, first bool) int {
	stop := r.cfg.StopAt
	phase := stop > 0 || r.cfg.StartDuty > 0
	if !phase {
		if stalled {
			return min(100, r.duty+stallStep)
		}
		if first { // what the fan does now is unknown, a ramp would only delay it
			return wanted
		}
	} else {
		kick := max(r.cfg.StartDuty, stop+1)
		standing := rpmOK && rpm == 0
		if wanted > stop && (r.duty <= stop || standing || (!rpmOK && r.duty == kick)) {
			if r.holding < spinUpTicks {
				r.holding++
				return max(r.duty, kick)
			}
			if standing {
				return min(100, r.duty+stallStep)
			}
		} else {
			r.holding = 0
		}
	}
	duty := r.duty + max(-r.cfg.StepDown, min(r.cfg.StepUp, wanted-r.duty))
	if duty <= stop {
		return 0
	}
	return duty
}

// stepWithoutSensor keeps the last duty over a short gap, then goes to
// SafeDuty. Before the first reading there is nothing to keep.
func (r *regulator) stepWithoutSensor() result {
	r.missing++
	if r.started && r.missing < sensorGraceTicks {
		return result{duty: r.duty}
	}
	r.started = true
	r.duty = max(r.duty, r.cfg.SafeDuty)
	r.holding = 0
	return result{duty: r.duty, sensorLost: true}
}

func (r *regulator) updateIdle(rpm int, rpmOK bool) int {
	// A pump always has to run, so its duty says nothing about a standstill.
	if !rpmOK || (r.duty == 0 && !r.cfg.Pump) || rpm >= max(1, r.cfg.StoppedBelowRPM) {
		return 0
	}
	return r.idle + 1
}

func (r *regulator) wantedDuty(value float64) int {
	var duty int
	if r.cfg.RPMCurve {
		duty = r.cfg.Speeds.dutyFor(value)
	} else {
		duty = int(min(100, math.Ceil(value)))
	}
	return max(r.cfg.MinDuty, min(100, max(0, duty+r.cfg.Offset)))
}

// fittedSlope returns the least-squares slope per sample.
func fittedSlope(y []float64) float64 {
	n := float64(len(y))
	meanX := (n - 1) / 2
	var meanY float64
	for _, v := range y {
		meanY += v
	}
	meanY /= n
	var cov, varX float64
	for i, v := range y {
		dx := float64(i) - meanX
		cov += dx * (v - meanY)
		varX += dx * dx
	}
	return cov / varX
}
