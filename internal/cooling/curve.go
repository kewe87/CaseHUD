package cooling

import (
	"errors"
	"fmt"
	"math"
)

// Point maps a temperature to a value on a Curve.
type Point struct {
	Temp  float64 // °C
	Value float64 // percent or rpm, see Fan.Speeds
}

// Curve is a piecewise linear function of temperature. Outside its points it
// keeps the first or last value.
type Curve []Point

func (c Curve) validate() error {
	if len(c) == 0 {
		return errors.New("curve has no points")
	}
	for i, p := range c {
		if math.IsNaN(p.Temp) || math.IsInf(p.Temp, 0) || math.IsNaN(p.Value) || math.IsInf(p.Value, 0) || p.Value < 0 {
			return fmt.Errorf("curve point %d is not a valid number", i)
		}
		if p.Temp < lowestPlausibleTemp || p.Temp > highestPlausibleTemp {
			return fmt.Errorf("curve point %d: temperature out of range", i)
		}
		if i > 0 && p.Temp <= c[i-1].Temp {
			return fmt.Errorf("curve point %d: temperatures must rise", i)
		}
		if i > 0 && p.Value < c[i-1].Value {
			return fmt.Errorf("curve point %d: a hotter point must not ask for less", i)
		}
	}
	return nil
}

func (c Curve) at(temp float64) float64 {
	if temp <= c[0].Temp {
		return c[0].Value
	}
	for i := 1; i < len(c); i++ {
		if temp <= c[i].Temp {
			a, b := c[i-1], c[i]
			return a.Value + (b.Value-a.Value)*(temp-a.Temp)/(b.Temp-a.Temp)
		}
	}
	return c[len(c)-1].Value
}

// SpeedPoint is one measured duty/rpm pair of a running fan.
type SpeedPoint struct {
	Duty int // 1..100
	RPM  int // > 0
}

// SpeedMap holds the measured speeds of one fan, in rising duty order. Two
// fans of the same model can differ, so it is kept per fan. The rpm may stop
// rising at the top of the range: the lowest duty reaching a speed is used.
type SpeedMap []SpeedPoint

func (m SpeedMap) validate() error {
	if len(m) < 2 {
		return errors.New("speed map needs at least two points")
	}
	for i, p := range m {
		if p.Duty < 1 || p.Duty > 100 || p.RPM <= 0 || p.RPM > highestPlausibleRPM {
			return fmt.Errorf("speed map point %d out of range", i)
		}
		if i > 0 && (p.Duty <= m[i-1].Duty || p.RPM < m[i-1].RPM) {
			return fmt.Errorf("speed map point %d: duty and rpm must rise", i)
		}
	}
	return nil
}

// dutyFor returns the duty that reaches at least rpm. A speed above the map
// asks for full duty, a positive speed below it for the slowest running duty,
// and only 0 stops the fan.
func (m SpeedMap) dutyFor(rpm float64) int {
	if rpm <= 0 {
		return 0
	}
	if rpm <= float64(m[0].RPM) {
		return m[0].Duty
	}
	for i := 1; i < len(m); i++ {
		if rpm <= float64(m[i].RPM) {
			a, b := m[i-1], m[i]
			share := (rpm - float64(a.RPM)) / float64(b.RPM-a.RPM)
			return a.Duty + int(math.Ceil(share*float64(b.Duty-a.Duty)))
		}
	}
	return 100
}

// DutyFor is dutyFor for callers outside the package; the map must be valid.
func (m SpeedMap) DutyFor(rpm float64) int { return m.dutyFor(rpm) }

// HighestRPM is the highest speed a configuration may name.
const HighestRPM = highestPlausibleRPM

// rpmFor returns the speed a duty should give: 0 for a stopped fan, otherwise
// interpolated and limited to the ends of the map.
func (m SpeedMap) rpmFor(duty int) int {
	if duty <= 0 {
		return 0
	}
	if duty <= m[0].Duty {
		return m[0].RPM
	}
	for i := 1; i < len(m); i++ {
		if duty <= m[i].Duty {
			a, b := m[i-1], m[i]
			return a.RPM + (b.RPM-a.RPM)*(duty-a.Duty)/(b.Duty-a.Duty)
		}
	}
	return m[len(m)-1].RPM
}
