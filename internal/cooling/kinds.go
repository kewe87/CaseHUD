package cooling

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// Kind says how a curve gets its value.
type Kind string

const (
	// KindDrawn follows Points and holds small temperature changes back.
	KindDrawn Kind = "drawn"
	// KindStraight is a drawn curve of exactly two points.
	KindStraight Kind = "straight"
	// KindGap is a drawn curve over the distance between the hottest source and
	// Reference, for instance coolant above the air in the room. The value stays
	// sensible whatever the room's temperature is.
	KindGap Kind = "gap"
	// KindFixed always asks for Value.
	KindFixed Kind = "fixed"
	// KindSwitch switches between two values: up to LoadValue when the
	// temperature reaches LoadTemp, back to IdleValue when it falls to IdleTemp.
	KindSwitch Kind = "switch"
	// KindThermostat searches for the lowest value that holds LoadTemp, see thermostatCurve.
	KindThermostat Kind = "thermostat"
	// KindCombined combines other curves with a CombineFunc.
	KindCombined Kind = "combined"
	// KindFollow follows what another output does, plus Offset.
	KindFollow Kind = "follow"
)

// CombineFunc says how a KindCombined curve combines its inputs.
type CombineFunc string

// The ways to combine curves. CombineSubtract takes the others from the first input.
const (
	CombineMax      CombineFunc = "max"
	CombineMin      CombineFunc = "min"
	CombineSum      CombineFunc = "sum"
	CombineAverage  CombineFunc = "average"
	CombineSubtract CombineFunc = "subtract"
)

// CurveSpec describes one curve. Any number of outputs can hang on it and share
// its state. Which fields count depends on Kind; the others are ignored, so a
// curve keeps them while someone tries out another kind.
type CurveSpec struct {
	ID string
	// Kind defaults to KindDrawn.
	Kind Kind
	// RPM means the values are speeds, otherwise percent.
	RPM bool
	// Sources are the sensors, for all kinds that follow a temperature. The
	// hottest counts, and a missing one makes the curve's value unusable.
	Sources []string
	// Reference is the sensor a KindGap curve measures from. Without it, or with
	// a bad reading, the curve has no value.
	Reference string

	// Points and the rest of this block are for KindDrawn, KindStraight and KindGap, see
	// the fields of the same name on Fan. DistanceAtEnds makes the distance
	// count at the ends of the curve as well.
	Points         Curve
	RiseBy, FallBy float64
	DistanceAtEnds bool
	LeadTicks      int

	// RiseAfter and FallAfter are for KindDrawn, KindStraight, KindGap and KindSwitch.
	RiseAfter, FallAfter int

	// Value is the value of a KindFixed curve.
	Value float64

	// IdleTemp and LoadTemp are for KindSwitch and KindThermostat. IdleValue and
	// LoadValue are the two values of a switch curve, and the limits of a thermostat curve.
	IdleTemp, LoadTemp   float64
	IdleValue, LoadValue float64

	// Step, Deadband and ReactAfter are for KindThermostat, see thermostatCurve.
	Step, Deadband float64
	ReactAfter     int

	// Inputs and Combine are for KindCombined; Inputs holds curve IDs.
	Inputs  []string
	Combine CombineFunc

	// Follow is the ID of the output a KindFollow curve follows. Offset is added
	// to its value, or with Proportional taken as a share in percent of it.
	Follow       string
	Offset       float64
	Proportional bool

	// Guard are sensors watched only for the emergency rule, for a curve that
	// reads none itself, such as a fixed value that stands for a manual setting.
	// The value of the curve does not depend on them.
	Guard []string
}

func (s CurveSpec) kind() Kind {
	if s.Kind == "" {
		return KindDrawn
	}
	return s.Kind
}

// followsTemperature tells the kinds that read sensors themselves.
func (s CurveSpec) followsTemperature() bool {
	switch s.kind() {
	case KindDrawn, KindStraight, KindGap, KindSwitch, KindThermostat:
		return true
	}
	return false
}

func (s CurveSpec) highest() float64 {
	if s.RPM {
		return highestPlausibleRPM
	}
	return 100
}

// Check checks what belongs to the curve itself. A curve may exist without
// sensors until an output uses it, and what it refers to is checked by
// CheckCurves.
func (s CurveSpec) Check() error {
	var errs []error
	check := func(ok bool, what string) {
		if !ok {
			errs = append(errs, errors.New(what))
		}
	}
	number := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	value := func(v float64) bool { return number(v) && v >= 0 && v <= s.highest() }
	temperature := func(v float64) bool {
		return number(v) && v > lowestPlausibleTemp && v <= highestPlausibleTemp
	}
	delays := func() {
		check(s.RiseAfter >= 1 && s.FallAfter >= 1 && s.RiseAfter <= longestRiseTicks && s.FallAfter <= longestDelayTicks,
			"delays out of range")
	}
	switch s.kind() {
	case KindDrawn, KindStraight, KindGap:
		check(!slices.Contains(s.Sources, s.Reference), "the reference is also a source")
		check(s.RiseBy >= 0 && s.FallBy >= 0 && s.RiseBy <= widestDistance && s.FallBy <= widestDistance,
			"temperature distance out of range")
		delays()
		check(s.LeadTicks >= 0, "negative LeadTicks")
		if err := s.Points.validate(); err != nil {
			errs = append(errs, err)
		} else {
			check(s.Points[len(s.Points)-1].Value <= s.highest(), "curve value out of range")
		}
		check(s.kind() != KindStraight || len(s.Points) == 2, "a straight curve has two points")
	case KindFixed:
		check(value(s.Value), "value out of range")
	case KindSwitch:
		delays()
		check(temperature(s.IdleTemp) && temperature(s.LoadTemp) && s.IdleTemp < s.LoadTemp, "the idle temperature must be below the load temperature")
		check(value(s.IdleValue) && value(s.LoadValue) && s.IdleValue <= s.LoadValue, "values out of range")
	case KindThermostat:
		check(temperature(s.IdleTemp) && temperature(s.LoadTemp) && s.IdleTemp < s.LoadTemp, "the idle temperature must be below the load temperature")
		check(value(s.IdleValue) && value(s.LoadValue) && s.IdleValue < s.LoadValue, "values out of range")
		check(number(s.Step) && s.Step > 0 && s.Step <= s.highest(), "step out of range")
		check(number(s.Deadband) && s.Deadband >= 0 && s.Deadband < s.LoadTemp-s.IdleTemp, "deadband out of range")
		check(s.ReactAfter >= 1 && s.ReactAfter <= longestRiseTicks, "delays out of range")
	case KindCombined:
		check(len(s.Inputs) > 0, "a combined curve needs inputs")
		sorted := slices.Sorted(slices.Values(s.Inputs))
		check(len(slices.Compact(sorted)) == len(s.Inputs), "an input is connected twice")
		switch s.Combine {
		case CombineMax, CombineMin, CombineSum, CombineAverage, CombineSubtract:
		default:
			errs = append(errs, fmt.Errorf("unknown combine function %q", s.Combine))
		}
	case KindFollow:
		check(s.Follow != "", "nothing to follow")
		check(number(s.Offset) && math.Abs(s.Offset) <= s.highest(), "offset out of range")
	default:
		errs = append(errs, fmt.Errorf("unknown kind %q", s.Kind))
	}
	return errors.Join(errs...)
}

// CheckCurves checks a set of curves and what they refer to. curveOf gives the
// curve of every output, empty for an output on none.
func CheckCurves(curves []CurveSpec, curveOf map[string]string) error {
	byID := make(map[string]CurveSpec, len(curves))
	for _, s := range curves {
		if s.ID == "" {
			return errors.New("cooling: a curve has no ID")
		}
		if _, twice := byID[s.ID]; twice {
			return fmt.Errorf("cooling: curve %q configured twice", s.ID)
		}
		byID[s.ID] = s
		if err := s.Check(); err != nil {
			return fmt.Errorf("cooling: curve %q: %w", s.ID, err)
		}
	}
	for _, s := range curves {
		switch s.kind() {
		case KindCombined:
			for _, in := range s.Inputs {
				input, ok := byID[in]
				if !ok {
					return fmt.Errorf("cooling: curve %q: no curve %q", s.ID, in)
				}
				if input.RPM != s.RPM {
					return fmt.Errorf("cooling: curve %q: %q is in another unit", s.ID, in)
				}
			}
		case KindFollow:
			on, known := curveOf[s.Follow]
			if !known {
				return fmt.Errorf("cooling: curve %q: no output %q", s.ID, s.Follow)
			}
			if on == "" {
				return fmt.Errorf("cooling: curve %q: output %q is on no curve", s.ID, s.Follow)
			}
		}
	}
	_, err := curveOrder(curves, curveOf)
	return err
}

// curveOrder sorts the curves so that each comes after those it needs in the
// same tick, and refuses curves that end up needing themselves.
func curveOrder(curves []CurveSpec, curveOf map[string]string) ([]CurveSpec, error) {
	byID := make(map[string]CurveSpec, len(curves))
	for _, s := range curves {
		byID[s.ID] = s
	}
	const visiting, done = 1, 2
	state := make(map[string]int, len(curves))
	out := make([]CurveSpec, 0, len(curves))
	var visit func(s CurveSpec) error
	visit = func(s CurveSpec) error {
		switch state[s.ID] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("cooling: curve %q ends up following itself", s.ID)
		}
		state[s.ID] = visiting
		for _, need := range s.needs(curveOf) {
			if err := visit(byID[need]); err != nil {
				return err
			}
		}
		state[s.ID] = done
		out = append(out, s)
		return nil
	}
	for _, s := range curves {
		if err := visit(s); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// needs lists the curves this one takes its value from.
func (s CurveSpec) needs(curveOf map[string]string) []string {
	switch s.kind() {
	case KindCombined:
		return s.Inputs
	case KindFollow:
		if on := curveOf[s.Follow]; on != "" {
			return []string{on}
		}
	}
	return nil
}

// evaluator turns a temperature into a curve's value, once per tick. Without a
// usable value the result is false.
type evaluator interface {
	advance(temp float64, tempOK bool) (float64, bool)
}

func newEvaluator(s CurveSpec) evaluator {
	switch s.kind() {
	case KindFixed:
		return fixedCurve(s.Value)
	case KindSwitch:
		return &switchCurve{spec: s}
	case KindThermostat:
		return newAutoCurve(s)
	case KindCombined, KindFollow:
		return nil // these work on other curves, see curveNode
	}
	return &drawnCurve{
		points: s.Points, lead: leadTrend{ticks: s.LeadTicks},
		gate: tempGate{riseBy: s.RiseBy, fallBy: s.FallBy, riseAfter: s.RiseAfter, fallAfter: s.FallAfter, distanceAtEnds: s.DistanceAtEnds},
	}
}

// fixedCurve needs no sensor, so it cannot lose one.
type fixedCurve float64

func (f fixedCurve) advance(float64, bool) (float64, bool) { return float64(f), true }

// drawnCurve follows its points through a tempGate. The first reading after a
// gap counts at once: there is nothing to hold on to.
type drawnCurve struct {
	points Curve
	gate   tempGate
	lead   leadTrend
	value  float64
	has    bool
}

func (g *drawnCurve) advance(temp float64, tempOK bool) (float64, bool) {
	if !tempOK {
		g.has = false
		g.lead.gap()
		return 0, false
	}
	t := g.lead.apply(temp)
	side := 0
	if t <= g.points[0].Temp {
		side = -1
	} else if t >= g.points[len(g.points)-1].Temp {
		side = 1
	}
	if g.gate.pass(t, side) || !g.has {
		g.value, g.has = g.points.at(t), true
	}
	return g.value, true
}

// tempGate decides when a new temperature counts: once it differs from the last
// accepted one by the distance, for the number of ticks. Beyond an end of the
// curve the distance is not needed, unless distanceAtEnds asks for it: going
// there only takes the ticks of that direction, and moving about out there counts
// at once, since the value stays the same.
type tempGate struct {
	riseBy, fallBy       float64
	riseAfter, fallAfter int
	distanceAtEnds       bool

	seen         bool
	last         float64
	lastSide     int
	rises, falls int
}

// pass takes a reading; side tells which end of the curve it lies beyond, 0 for
// none.
func (g *tempGate) pass(temp float64, side int) bool {
	if !g.seen {
		g.seen, g.last, g.lastSide, g.rises, g.falls = true, temp, side, 0, 0
		return true
	}
	up := temp-g.last >= g.riseBy
	down := g.last-temp >= g.fallBy
	if side != 0 && !g.distanceAtEnds {
		if side == g.lastSide {
			g.last, g.rises, g.falls = temp, 0, 0
			return true
		}
		up, down = side > 0, side < 0
	}
	fire := false
	switch {
	case up:
		g.falls = 0
		if g.rises++; g.rises >= g.riseAfter {
			g.rises, fire = 0, true
		}
	case down:
		g.rises = 0
		if g.falls++; g.falls >= g.fallAfter {
			g.falls, fire = 0, true
		}
	default:
		g.rises, g.falls = 0, 0
	}
	if fire {
		g.last, g.lastSide = temp, side
	}
	return fire
}

// reset forgets the accepted temperature, so the next reading passes.
func (g *tempGate) reset() { g.seen, g.rises, g.falls = false, 0, 0 }

// leadTrend moves a rising temperature by its trend, see Fan.LeadTicks. The trend
// is a line fitted over the last readings, since sensors with whole degrees show
// only steps from one reading to the next.
type leadTrend struct {
	ticks   int
	history []float64
	missing int
}

func (l *leadTrend) gap() {
	if l.missing++; l.missing >= sensorGraceTicks {
		l.history = nil
	}
}

func (l *leadTrend) apply(temp float64) float64 {
	l.missing = 0
	if l.ticks == 0 {
		return temp
	}
	if len(l.history) == trendWindow {
		copy(l.history, l.history[1:])
		l.history = l.history[:trendWindow-1]
	}
	l.history = append(l.history, temp)
	if len(l.history) < trendMinimum {
		return temp
	}
	slope := fittedSlope(l.history)
	if slope <= 0 {
		return temp
	}
	return temp + min(slope*float64(l.ticks), maxLeadDegrees)
}

// switchCurve remembers which of its two values it is on. That survives a
// missing sensor: on the way back it goes on from the same state, so a part that
// got hot meanwhile still switches up.
type switchCurve struct {
	spec         CurveSpec
	known, high  bool
	rises, falls int
}

func (c *switchCurve) advance(temp float64, tempOK bool) (float64, bool) {
	if !tempOK {
		return 0, false
	}
	switch {
	case !c.known:
		c.known, c.high = true, temp > c.spec.LoadTemp
	case temp >= c.spec.LoadTemp && !c.high:
		if c.rises++; c.rises >= c.spec.RiseAfter {
			c.high, c.rises = true, 0
		}
	case temp <= c.spec.IdleTemp && c.high:
		if c.falls++; c.falls >= c.spec.FallAfter {
			c.high, c.falls = false, 0
		}
	default:
		c.rises, c.falls = 0, 0
	}
	if c.high {
		return c.spec.LoadValue, true
	}
	return c.spec.IdleValue, true
}

// Trend directions. A direction needs a window full of readings.
const (
	trendUnknown = iota
	trendDown
	trendFlat
	trendUp
)

const (
	autoTrendTolerance     = 1    // °C the line may move over half its window
	thermostatLoadShare    = 0.75 // where between idle and load the load phase begins
	thermostatShortWindow  = 3    // windows in units of ReactAfter
	thermostatLongWindow   = 5
	thermostatFallFactor   = 2 // falling reacts this much slower than rising
	thermostatIdleDistance = 2 // °C the temperature moves before an idle value changes
)

// slopeTrend tells the direction of a temperature from a line fitted over the
// last readings. A slope too small to count on its own still adds up, so a very
// slow drift is noticed too. It calculates in single precision, as sensors
// report: the limits are exact values that double precision can land beside.
type slopeTrend struct {
	window   []float32
	next     int
	filled   int
	drift    float32
	sumX     float32
	divisor  float32
	halfSize float64
}

func newSlopeTrend(size int) *slopeTrend {
	sumX, sumXX := 0, 0
	for i := range size {
		sumX += i
		sumXX += i * i
	}
	return &slopeTrend{
		window: make([]float32, size), sumX: float32(sumX),
		divisor: float32(size*sumXX - sumX*sumX), halfSize: float64(size) / 2,
	}
}

func (t *slopeTrend) reset() { t.next, t.filled, t.drift = 0, 0, 0 }

// push adds a reading. The first verdict comes with the reading after the one
// that fills the window.
func (t *slopeTrend) push(v float32) int {
	size := len(t.window)
	t.window[t.next] = v
	t.next = (t.next + 1) % size
	if t.filled <= size-1 {
		t.filled++
		return trendUnknown
	}
	slope := t.slope()
	if math.Abs(float64(slope)*t.halfSize) > autoTrendTolerance {
		if slope < 0 {
			return trendDown
		}
		return trendUp
	}
	t.drift = float32(t.drift + slope)
	if math.Abs(float64(t.drift)) > autoTrendTolerance {
		d := trendUp
		if t.drift < 0 {
			d = trendDown
		}
		t.drift = 0
		return d
	}
	return trendFlat
}

func (t *slopeTrend) slope() float32 {
	size := len(t.window)
	var sumY, sumXY float32
	for i := range size {
		v := t.window[(t.next+i)%size]
		sumY = float32(sumY + v)
		sumXY = float32(sumXY + float32(float32(i)*v))
	}
	a := float32(float32(size) * sumXY)
	b := float32(t.sumX * sumY)
	return float32(a-b) / t.divisor
}

// thermostatCurve searches for the lowest value that holds LoadTemp. Below the load
// phase it runs on a straight line from idle to load. In the load phase, which
// begins three quarters of the way up, it goes up by Step while the temperature
// exceeds LoadTemp or climbs towards it, and down by half a Step while it falls
// or rests below LoadTemp minus Deadband. Falling reacts half as fast as rising,
// and once a direction has waited its time it moves every tick. It leaves the
// load phase when the temperature has dropped below it and is no longer rising.
// A gap in the readings keeps the value but forgets the trends: the last line
// before the gap says nothing about the time after it.
type thermostatCurve struct {
	spec        CurveSpec
	gate        tempGate
	long, short *slopeTrend
	value       float64
	has         bool
	target      float64
	underLoad   bool
	rises       int
	falls       int
}

func newAutoCurve(s CurveSpec) *thermostatCurve {
	return &thermostatCurve{
		spec: s,
		gate: tempGate{riseBy: thermostatIdleDistance, fallBy: thermostatIdleDistance, riseAfter: s.ReactAfter, fallAfter: s.ReactAfter, distanceAtEnds: true},
		long: newSlopeTrend(thermostatLongWindow * s.ReactAfter), short: newSlopeTrend(thermostatShortWindow * s.ReactAfter),
	}
}

func (c *thermostatCurve) advance(temp float64, tempOK bool) (float64, bool) {
	if !tempOK {
		c.long.reset()
		c.short.reset()
		return 0, false
	}
	s := c.spec
	if !c.has {
		c.has = true
		c.target = s.IdleValue + thermostatLoadShare*(s.LoadValue-s.IdleValue)
		c.value = s.IdleValue
		if temp > s.LoadTemp {
			c.value = s.LoadValue
		}
		return c.value, true
	}
	long := c.long.push(float32(temp))
	short := c.short.push(float32(temp))
	if temp >= s.IdleTemp+thermostatLoadShare*(s.LoadTemp-s.Deadband-s.IdleTemp) {
		c.underLoad = true
		c.seekLoad(temp, long, short)
		c.value = c.target
		return c.value, true
	}
	if c.underLoad && (long == trendFlat || long == trendDown) && c.fallDue() {
		c.underLoad = false
		c.rises, c.falls = 0, 0
		c.gate.reset()
	}
	if !c.underLoad {
		if temp <= s.IdleTemp {
			c.gate.reset()
			c.value = s.IdleValue
		} else if c.gate.pass(temp, 0) {
			c.value = s.IdleValue + (s.LoadValue-s.IdleValue)/(s.LoadTemp-s.IdleTemp)*(temp-s.IdleTemp)
		}
	}
	return c.value, true
}

func (c *thermostatCurve) seekLoad(temp float64, long, short int) {
	s := c.spec
	border := s.LoadTemp - s.Deadband
	switch {
	case (short == trendUp && temp >= border) || temp > s.LoadTemp:
		c.falls = 0
		if c.rises++; c.rises >= s.ReactAfter {
			c.target = max(s.IdleValue, min(s.LoadValue, c.value+s.Step))
		}
	case long == trendDown || (temp <= border && long == trendFlat):
		c.rises = 0
		if c.fallDue() {
			c.target = max(s.IdleValue, min(s.LoadValue, c.value-s.Step/2))
		}
	default:
		c.rises, c.falls = 0, 0
	}
}

func (c *thermostatCurve) fallDue() bool {
	c.falls++
	return c.falls >= thermostatFallFactor*c.spec.ReactAfter
}

// combine works out a combined curve. Every input counts, see curveNode.
func combine(f CombineFunc, values []float64, highest float64) float64 {
	var v float64
	switch f {
	case CombineMax:
		for _, x := range values {
			v = max(v, x)
		}
	case CombineMin:
		v = highest
		for _, x := range values {
			v = min(v, x)
		}
	case CombineSum, CombineAverage:
		for _, x := range values {
			v += x
		}
		if f == CombineAverage {
			v /= float64(len(values))
		}
	case CombineSubtract:
		v = values[0]
		for _, x := range values[1:] {
			v -= x
		}
	}
	return max(0, min(highest, v))
}

// following works out a KindFollow value.
func following(s CurveSpec, v float64) float64 {
	if s.Proportional {
		v *= 1 + s.Offset/100
	} else {
		v += s.Offset
	}
	return max(0, min(s.highest(), v))
}

// curveNode is one curve at runtime, with the value of the current tick. A combined curve
// has no usable value as soon as one input has none: going by the others could
// lower the fans just because the hot part's sensor is gone.
type curveNode struct {
	spec   CurveSpec
	eval   evaluator
	inputs []*curveNode // what a combined curve combines, or the curve of a followed output
	follow int          // index of the output a follow curve follows
	outs   []int        // outputs hanging on this curve
	sig    signal
	// most and noSensor are worked out once, when the inputs are known: walking
	// the inputs each time doubles the work with every level of a long chain.
	most     float64 // the most this curve can ask for
	noSensor bool    // this curve or one it depends on has no sensor to follow
	// guardHot keeps the last reading of a guard sensor that was at the
	// emergency temperature and is gone now.
	guardHot map[string]float64
}

// signal is what an output gets from its curve.
type signal struct {
	value   float64
	ok      bool
	temp    float64
	hasTemp bool
	// guard is the hottest guard sensor, see CurveSpec.Guard. It only counts for
	// the emergency rule and is not the temperature the output shows.
	guard    float64
	hasGuard bool
}

// watched is the reading the emergency rule goes by: the hottest of the curve's
// temperature and its guard.
func (s signal) watched() (float64, bool) {
	switch {
	case s.hasTemp && s.hasGuard:
		return max(s.temp, s.guard), true
	case s.hasTemp:
		return s.temp, true
	}
	return s.guard, s.hasGuard
}

// advance computes this tick's signal. followed reads the output a follow curve
// follows, which already got its decision in this tick.
func (n *curveNode) advance(temps map[string]float64, followed func(int) (float64, bool)) {
	s := n.spec
	var sig signal
	if s.followsTemperature() {
		temp, ok := hottest(s.Sources, temps)
		// What the output sees stays the real temperature, for the emergency rule.
		sig.temp, sig.hasTemp = warmestPresent(s.Sources, temps)
		if s.kind() == KindGap {
			from, fromOK := hottest([]string{s.Reference}, temps)
			temp, ok = temp-from, ok && fromOK
		}
		sig.value, sig.ok = n.eval.advance(temp, ok)
		n.sig = sig
		return
	}
	// Sensors further down still count for the emergency rule.
	values := make([]float64, 0, len(n.inputs))
	inputsOK := true
	for _, in := range n.inputs {
		values = append(values, in.sig.value)
		inputsOK = inputsOK && in.sig.ok
		if in.sig.hasTemp && (!sig.hasTemp || in.sig.temp > sig.temp) {
			sig.temp, sig.hasTemp = in.sig.temp, true
		}
	}
	switch s.kind() {
	case KindFixed:
		sig.value, sig.ok = n.eval.advance(0, true)
	case KindCombined:
		if sig.ok = inputsOK; inputsOK {
			sig.value = combine(s.Combine, values, s.highest())
		}
	case KindFollow:
		if v, ok := followed(n.follow); ok {
			sig.value, sig.ok = following(s, v), true
		}
	}
	sig.guard, sig.hasGuard = n.guarded(temps)
	n.sig = sig
}

// guarded returns the hottest guard sensor. A sensor that was at the emergency
// temperature keeps that reading while it is gone: a part known to be too hot
// stays too hot, and a lost reading never lowers the output.
func (n *curveNode) guarded(temps map[string]float64) (float64, bool) {
	best, found := 0.0, false
	for _, s := range n.spec.Guard {
		t, ok := warmestPresent([]string{s}, temps)
		switch {
		case ok && t >= emergencyTemp:
			if n.guardHot == nil {
				n.guardHot = map[string]float64{}
			}
			n.guardHot[s] = t
		case ok:
			delete(n.guardHot, s)
		default:
			t, ok = n.guardHot[s]
		}
		if ok {
			best, found = max(best, t), true
		}
	}
	return best, found
}

// settle works out what follows from the settings. The inputs must be settled.
func (n *curveNode) settle() {
	n.most = n.top()
	n.noSensor = n.spec.followsTemperature() && len(n.spec.Sources) == 0 ||
		n.spec.kind() == KindGap && n.spec.Reference == ""
	for _, in := range n.inputs {
		n.noSensor = n.noSensor || in.noSensor
	}
}

// top is the most this curve can ask for, used to refuse settings that could
// never work.
func (n *curveNode) top() float64 {
	s := n.spec
	switch s.kind() {
	case KindFixed:
		return s.Value
	case KindSwitch, KindThermostat:
		return s.LoadValue
	case KindDrawn, KindStraight, KindGap:
		return s.Points[len(s.Points)-1].Value
	case KindCombined:
		if s.Combine == CombineSubtract { // the others can all be at zero
			return n.inputs[0].most
		}
		tops := make([]float64, len(n.inputs))
		for i, in := range n.inputs {
			tops[i] = in.most
		}
		return combine(s.Combine, tops, s.highest())
	case KindFollow:
		return following(s, s.highest())
	}
	return s.highest()
}
