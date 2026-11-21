package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/rgb/backend/enram"
	"github.com/kewe87/casehud/internal/sysperm"
)

// Pump on a mainboard header. Only the sensor service may set it, over a pipe of
// its own that takes nothing else. The service holds the duty as long as this
// connection lives and gives the header back to the board when it ends or stays
// silent, so a crash of CaseHUD leaves the pump under the board's regulation.

// The sensor service refuses a duty below cooling.LowestPumpDuty too.
const coolingPipe = enram.CoolingPipe

var pumpSensorKey = regexp.MustCompile(`^(/lpc/[a-z0-9]{1,32}/[0-9]{1,2})/fan/([0-9]{1,2})$`)

// pumpControlKey names the header that drives the pump behind a speed sensor. The
// mainboard numbers both the same way.
func pumpControlKey(rpmSensor string) (string, bool) {
	m := pumpSensorKey.FindStringSubmatch(rpmSensor)
	if m == nil {
		return "", false
	}
	return m[1] + "/control/" + m[2], true
}

// pumpControl names the header that drives pump o: the channel chosen for it, or
// else the one derived from its speed sensor.
func pumpControl(o coolingOutputConfig) (string, bool) {
	if o.ControlKey == "" {
		return pumpControlKey(o.RPMSensor)
	}
	if !controlOnChipOf(o.RPMSensor, o.ControlKey) {
		return "", false
	}
	return o.ControlKey, true
}

// controlOnChipOf tells whether key is a control channel of the Super-IO chip the
// speed sensor belongs to.
func controlOnChipOf(rpmSensor, key string) bool {
	m := pumpSensorKey.FindStringSubmatch(rpmSensor)
	return m != nil && sysperm.ValidCoolingHeader(key) && strings.HasPrefix(key, m[1]+"/control/")
}

// checkControlKeys rejects a channel that two pumps would write, whether it was
// chosen or derived from the speed sensor: two pumps on one speed sensor collide too.
func checkControlKeys(outputs []coolingOutputConfig) error {
	seen := map[string]string{} // channel -> output id
	for _, o := range outputs {
		if !o.Pump || tlOutputID.MatchString(o.ID) {
			continue
		}
		key, ok := pumpControl(o)
		if !ok {
			continue
		}
		if prev, dup := seen[key]; dup {
			return fmt.Errorf("%q: the control channel is used by %q already", o.ID, prev)
		}
		seen[key] = o.ID
	}
	return nil
}

// pumpLine is the connection to the service.
type pumpLine interface {
	Hold(key string, percent int) error
	Close() error
}

// coolingOpenPumpLine connects to the service on first use (tests replace it).
var coolingOpenPumpLine = func() pumpLine {
	return servicePumpLine{enram.NewPipeTransport(coolingPipe)}
}

type servicePumpLine struct{ *enram.PipeTransport }

func (l servicePumpLine) Hold(key string, percent int) error {
	req, err := json.Marshal(struct {
		V       int    `json:"v"`
		Type    string `json:"type"`
		Key     string `json:"key"`
		Percent int    `json:"percent"`
	}{1, "hold", key, percent})
	if err != nil {
		return err
	}
	return l.Repeatable(req)
}

// boardPumps implements cooling.Hardware for pumps on mainboard headers. A broken
// line never hides the speed: the pump's standstill must count whatever happens to
// the way of setting it, and without a line the board simply regulates on.
type boardPumps struct {
	gate    coolingGate
	keys    map[string]string // control key by output
	sensors map[string]string // speed sensor by output
	rpm     func() map[string]int
	now     func() time.Time

	// For the view, which must not wait for a write in progress.
	block  atomic.Pointer[coolingBlock]
	broken atomic.Bool
	// Pumps whose header the service does not let CaseHUD set (not released by
	// an administrator). The board keeps regulating them; this is not a failure.
	notReleased sync.Map // output id -> struct{}
	// How often the service refused each pump, to tell a new refusal from an old one.
	denyMu sync.Mutex
	denied map[string]int

	// Whether each pump is proven to sit on its header, see cooling_match.go.
	pmu    sync.Mutex
	states map[string]pumpState
	wake   chan struct{} // asks the matching to look again

	mu      sync.Mutex
	line    pumpLine
	keep    bool // Close leaves the line open: a successor takes it over, see takeOver
	blocked bool
	last    map[string]int // what the control wants, resent every tick
	retryAt time.Time
	lastErr string
	// Since when the service has answered "too many requests" without a break.
	busySince time.Time
}

// pumpBusyAfter is how long the service may keep refusing the line before that
// counts as a problem: something else uses the pipe at a rate that crowds CaseHUD out.
const pumpBusyAfter = 30 * time.Second

// newBoardPumps returns nil if no pump on the mainboard has a curve.
func newBoardPumps(cfg coolingConfig, prov *SafeProvider, gate coolingGate) *boardPumps {
	keys := map[string]string{}
	sensors := map[string]string{}
	for _, o := range cfg.regulated() {
		if !o.Pump || !o.driven() || tlOutputID.MatchString(o.ID) {
			continue
		}
		if key, ok := pumpControl(o); ok {
			keys[o.ID], sensors[o.ID] = key, o.RPMSensor
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return &boardPumps{
		gate: gate, keys: keys, sensors: sensors, now: time.Now, last: map[string]int{},
		states: map[string]pumpState{}, wake: make(chan struct{}, 1),
		rpm: func() map[string]int { return coolingSpeeds(prov, sensors) },
	}
}

// Speeds implements cooling.Hardware. It is also the sign of life: the service
// only keeps a duty that is repeated.
func (b *boardPumps) Speeds() (map[string]int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	reason, program := b.gate()
	b.blocked = reason != ""
	if b.blocked {
		b.block.Store(&coolingBlock{reason, program})
		b.closeLine()
	} else {
		b.block.Store(nil)
		for id, duty := range b.last {
			_ = b.send(id, duty) // told by failing, and the speed still counts
		}
	}
	return b.rpm(), nil
}

// TakeControl implements cooling.Hardware. The first hold takes control.
func (b *boardPumps) TakeControl(string) error { return nil }

// SetDuty implements cooling.Hardware. A pump whose header is not proven to
// belong to its speed sensor is never written: the board keeps it.
func (b *boardPumps) SetDuty(id string, percent int) error {
	if b.state(id) != pumpMatched {
		return nil
	}
	return b.write(id, percent)
}

// write sends a duty whatever the state of the pump; only the matching and
// SetDuty call it.
func (b *boardPumps) write(id string, percent int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last[id] = percent
	if b.blocked {
		return nil // another program is in charge, which the view says already
	}
	return b.send(id, percent)
}

func (b *boardPumps) send(id string, percent int) error {
	if b.now().Before(b.retryAt) {
		return errors.New("cooling: no line to the sensor service")
	}
	if b.line == nil {
		b.line = coolingOpenPumpLine()
	}
	err := b.line.Hold(b.keys[id], percent)
	if !errors.Is(err, enram.ErrBusy) {
		b.busySince = time.Time{}
	}
	if errors.Is(err, enram.ErrDenied) {
		// The service answered, so the line is fine. Sending goes on, which lets
		// the control take over by itself as soon as the header is released.
		b.denyMu.Lock()
		if b.denied == nil {
			b.denied = map[string]int{}
		}
		b.denied[id]++
		b.denyMu.Unlock()
		if _, seen := b.notReleased.Swap(id, struct{}{}); !seen {
			log.Printf("cooling: the pump on %s is not released for CaseHUD, the board regulates it", b.keys[id])
		}
		return nil
	}
	if err == nil {
		if _, was := b.notReleased.LoadAndDelete(id); was {
			log.Printf("cooling: the pump on %s is released and under control now", b.keys[id])
		}
		if b.broken.Swap(false) {
			log.Printf("cooling: the pump can be set again")
		}
		b.lastErr = ""
		return nil
	}
	if errors.Is(err, enram.ErrBusy) {
		// Too many requests for a moment: the service keeps what it has. Not for
		// long, though: then the pump may be running at a duty CaseHUD did not choose.
		now := b.now()
		if b.busySince.IsZero() {
			b.busySince = now
		}
		if now.Sub(b.busySince) >= pumpBusyAfter && !b.broken.Swap(true) {
			log.Printf("cooling: the sensor service has refused the pump line for %v, another program may be using the pipe; the pump may not follow its curve", pumpBusyAfter)
		}
		return err
	}
	if msg := err.Error(); msg != b.lastErr {
		log.Printf("cooling: the board regulates the pump, setting it failed: %v", err)
		b.lastErr = msg
	}
	b.broken.Store(true)
	b.retryAt = b.now().Add(coolingReopenAfter)
	b.closeLine()
	return err
}

// giveBack stops holding a pump. With nothing else held the line is closed, which
// hands the header to the board at once; otherwise the service does it after its
// silence limit.
func (b *boardPumps) giveBack(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.last, id)
	if len(b.last) == 0 {
		b.closeLine()
	}
}

func (b *boardPumps) ids() []string {
	ids := make([]string, 0, len(b.keys))
	for id := range b.keys {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (b *boardPumps) closeLine() {
	if b.line != nil {
		b.line.Close()
		b.line = nil
	}
}

// failing tells whether the pump named id is not being set as it should.
func (b *boardPumps) failing(id string) bool {
	if b == nil {
		return false
	}
	_, mine := b.keys[id]
	return mine && b.broken.Load() && b.block.Load() == nil
}

// notReleasedFor tells whether the service refuses to let CaseHUD set the pump named id.
func (b *boardPumps) notReleasedFor(id string) bool {
	if b == nil {
		return false
	}
	_, no := b.notReleased.Load(id)
	return no
}

// denials is how often the service has refused the pump named id.
func (b *boardPumps) denials(id string) int {
	b.denyMu.Lock()
	defer b.denyMu.Unlock()
	return b.denied[id]
}

// deniedSince tells whether the service refused the pump after the count was taken.
func (b *boardPumps) deniedSince(id string, count int) bool {
	return b.notReleasedFor(id) && b.denials(id) > count
}

// blockedBy names the program in charge, for a configuration with no fan that would tell.
func (b *boardPumps) blockedBy() coolingBlock {
	if b != nil {
		if s := b.block.Load(); s != nil {
			return *s
		}
	}
	return coolingBlock{}
}

// Close gives the pumps back to the board.
func (b *boardPumps) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.keep {
		b.closeLine()
	}
}

// keepLine makes Close leave the line open, for a control that replaces this one at
// once. The caller then takes the line with takeOver, or ends it with endKept.
func (b *boardPumps) keepLine() {
	if b != nil {
		b.mu.Lock()
		b.keep = true
		b.mu.Unlock()
	}
}

// endKept gives the header back after keepLine, when no successor takes the line.
func (b *boardPumps) endKept() {
	if b != nil {
		b.mu.Lock()
		b.keep = false
		b.closeLine()
		b.mu.Unlock()
	}
}

// takeOver adopts the line of the pumps this control replaces, which ended after
// keepLine, together with what they held: the header stays with CaseHUD instead of
// going back to the board for the time it takes to start again. Only a pump that is
// proven here and sits on the same header as before is carried over.
func (b *boardPumps) takeOver(from *boardPumps) {
	if b == nil || from == nil {
		return
	}
	from.mu.Lock()
	line, last := from.line, maps.Clone(from.last)
	from.line, from.keep = nil, false
	from.mu.Unlock()
	if line == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	released := false // a hold stays on a header this control does not take over
	for id, duty := range last {
		if key, ok := b.keys[id]; ok && key == from.keys[id] && b.state(id) == pumpMatched {
			b.last[id] = duty
		} else {
			released = true
		}
	}
	if len(b.last) == 0 {
		line.Close()
		return
	}
	b.line = line
	if released {
		b.releaseOthers(line)
	}
}

// releaseOthers hands the headers that are no longer held back to the board at
// once. The service has no release for a single header: a hold ends with its
// connection. So the holds that stay are made on a new line first, then the old one
// is closed. If the new line cannot hold them all, the old one stays and the
// service releases the rest after its silence limit.
func (b *boardPumps) releaseOthers(old pumpLine) {
	nl := coolingOpenPumpLine()
	for id, duty := range b.last {
		if err := nl.Hold(b.keys[id], duty); err != nil {
			nl.Close()
			return
		}
	}
	b.line = nl
	old.Close()
}
