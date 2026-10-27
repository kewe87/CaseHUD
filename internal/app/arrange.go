package app

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/kewe87/casehud/internal/panel"
)

// Limits of the fan controller, shown as hints in the arrangement view.
const (
	MaxLCDPerPort   = 3  // LCD fans in one port's chain
	MaxLCDTotal     = 13 // LCD fans on one controller
	MaxFansPerPort  = 10 // fans of any kind in one port's chain
	ControllerPorts = 4
	// FirstLCDPort is the first physical port that takes LCD fans; port 1 is
	// for fans without a display.
	FirstLCDPort = 2
)

// FanPos is one fan on the controller as a topology source reports it.
// Port is the number printed on the housing (1-4), Index the 0-based place
// in that port's chain.
type FanPos struct {
	Port   int    `json:"port"`
	Index  int    `json:"index"`
	HasLCD bool   `json:"lcd"`
	ID     string `json:"id,omitempty"` // panel ID when the fan has a display
}

// TopologySource reports fans the engine does not drive as panels, such as
// fans without a display that another module finds on the controller. The
// engine merges them with the LCD fans it knows from its panels; when both
// claim the same place in a chain, the LCD fan wins.
type TopologySource interface {
	Fans() []FanPos
}

// ArrangeScan describes where the fan list of the controller comes from. The
// list is only read from the controller on request (or, with lighting and the
// TL backend on, through the engine's own connection).
type ArrangeScan struct {
	Available bool   `json:"available"`    // a read can be requested
	Live      bool   `json:"live"`         // lighting with TL runs and keeps the list current
	Busy      bool   `json:"busy"`         // a read is running
	At        string `json:"at,omitempty"` // when the list was read (RFC 3339), empty: never
}

// ArrangeFan is a fan in the arrangement view.
type ArrangeFan struct {
	FanPos
	Number int `json:"number,omitempty"` // the user's logical number (LCD fans)
}

// ArrangePort is one controller port with its chain.
type ArrangePort struct {
	Port int          `json:"port"`
	LCD  bool         `json:"lcdAllowed"`
	Fans []ArrangeFan `json:"fans"`
}

// ArrangeWarning is a problem found in the setup; the GUI words it from Code.
type ArrangeWarning struct {
	Code string `json:"code"` // lcdPort1, portLCD, totalLCD, portFans, loose
	Port int    `json:"port,omitempty"`
	N    int    `json:"n,omitempty"`
}

// Arrangement is the controller schematic.
type Arrangement struct {
	Ports    []ArrangePort    `json:"ports"`
	Loose    []ArrangeFan     `json:"loose,omitempty"` // LCD fans whose place is unknown
	Warnings []ArrangeWarning `json:"warnings,omitempty"`
	Limits   map[string]int   `json:"limits"`
	Scan     *ArrangeScan     `json:"scan,omitempty"`
}

// isFanID reports whether a panel is a UNI FAN TL LCD.
func isFanID(id string) bool { return strings.HasPrefix(id, "tllcd-") }

// assignFanNumbers gives every fan a unique number from 1 up. ids must be in
// natural order. A stored number is kept when it is valid and not already
// taken; the others take the lowest free numbers in order, so without any
// stored numbers the fans are simply numbered in chain order.
func assignFanNumbers(ids []string, stored map[string]int) map[string]int {
	out := make(map[string]int, len(ids))
	used := map[int]bool{}
	for _, id := range ids {
		if n := stored[id]; n >= 1 && !used[n] {
			out[id], used[n] = n, true
		}
	}
	next := 1
	for _, id := range ids {
		if _, ok := out[id]; ok {
			continue
		}
		for used[next] {
			next++
		}
		out[id], used[next] = next, true
	}
	return out
}

// sortFansLocked orders the displays naturally and then puts the fans into
// their own slots by logical number; other displays keep their place.
// Caller holds e.mu.
func (e *Engine) sortFansLocked() {
	sort.SliceStable(e.order, func(i, j int) bool { return naturalLess(e.order[i], e.order[j]) })
	var fans []string
	var slots []int
	for i, id := range e.order {
		if isFanID(id) {
			fans = append(fans, id)
			slots = append(slots, i)
		}
	}
	nums := assignFanNumbers(fans, e.cfg.FanNumbers)
	sort.SliceStable(fans, func(i, j int) bool { return nums[fans[i]] < nums[fans[j]] })
	for k, i := range slots {
		e.order[i] = fans[k]
	}
}

// numbersLocked is the number of every connected display: the logical number
// for fans, and for the others the count continues behind the highest fan.
func (e *Engine) numbersLocked() map[string]int {
	var fans []string
	for _, id := range e.order {
		if isFanID(id) {
			fans = append(fans, id)
		}
	}
	slices.SortFunc(fans, func(a, b string) int {
		switch {
		case naturalLess(a, b):
			return -1
		case naturalLess(b, a):
			return 1
		}
		return 0
	})
	nums := assignFanNumbers(fans, e.cfg.FanNumbers)
	top := 0
	for _, n := range nums {
		top = max(top, n)
	}
	for _, id := range e.order {
		if !isFanID(id) {
			top++
			nums[id] = top
		}
	}
	return nums
}

// FanNumbers returns the number of every connected display by panel ID.
func (e *Engine) FanNumbers() map[string]int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.numbersLocked()
}

// SetFanNumber gives a fan the logical number n (1 to the number of fans).
// If another fan has n, the two swap numbers. Only the CaseHUD config
// changes; nothing is written to a device.
func (e *Engine) SetFanNumber(id string, n int) error {
	e.mu.Lock()
	var fans []string
	for _, pid := range e.order {
		if isFanID(pid) {
			fans = append(fans, pid)
		}
	}
	if !slices.Contains(fans, id) {
		e.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrNoPanel, id)
	}
	if n < 1 || n > len(fans) {
		e.mu.Unlock()
		return fmt.Errorf("number must be between 1 and %d", len(fans))
	}
	slices.SortFunc(fans, func(a, b string) int {
		if naturalLess(a, b) {
			return -1
		}
		if naturalLess(b, a) {
			return 1
		}
		return 0
	})
	nums := assignFanNumbers(fans, e.cfg.FanNumbers)
	old := nums[id]
	for _, other := range fans {
		if other != id && nums[other] == n {
			nums[other] = old
		}
	}
	nums[id] = n
	if e.cfg.FanNumbers == nil {
		e.cfg.FanNumbers = map[string]int{}
	}
	for fid, v := range nums {
		e.cfg.FanNumbers[fid] = v
	}
	e.sortFansLocked()
	e.mu.Unlock()
	return e.persist()
}

// ResetFanNumbers forgets all logical numbers: fans are numbered in chain order.
func (e *Engine) ResetFanNumbers() error {
	e.mu.Lock()
	e.cfg.FanNumbers = nil
	e.sortFansLocked()
	e.mu.Unlock()
	return e.persist()
}

// Arrangement describes the controller: its ports and the fans on them.
func (e *Engine) Arrangement() Arrangement {
	e.mu.Lock()
	nums := e.numbersLocked()
	var fans []FanPos
	var loose []ArrangeFan
	for _, id := range e.order {
		if !isFanID(id) {
			continue
		}
		dp, idx, ok := panel.TLPosition(id)
		if !ok {
			loose = append(loose, ArrangeFan{FanPos: FanPos{HasLCD: true, ID: id}, Number: nums[id]})
			continue
		}
		fans = append(fans, FanPos{Port: panel.TLPhysicalPort(dp), Index: idx, HasLCD: true, ID: id})
	}
	src := e.Topology
	e.mu.Unlock()
	if src != nil {
		fans = append(fans, src.Fans()...)
	}
	a := buildArrangement(fans, nums)
	if si, ok := src.(interface{ ScanInfo() ArrangeScan }); ok {
		info := si.ScanInfo()
		a.Scan = &info
	}
	a.Loose = loose
	if len(loose) > 0 {
		a.Warnings = append(a.Warnings, ArrangeWarning{Code: "loose", N: len(loose)})
	}
	return a
}

// buildArrangement lays fans out by port and chain place and checks the
// controller's limits. LCD fans win over other fans at the same place.
func buildArrangement(fans []FanPos, nums map[string]int) Arrangement {
	a := Arrangement{Limits: map[string]int{"lcdPerPort": MaxLCDPerPort, "lcdTotal": MaxLCDTotal, "fansPerPort": MaxFansPerPort}}
	byPort := map[int]map[int]ArrangeFan{}
	for _, f := range fans {
		if f.Port < 1 || f.Index < 0 {
			continue
		}
		m := byPort[f.Port]
		if m == nil {
			m = map[int]ArrangeFan{}
			byPort[f.Port] = m
		}
		if cur, taken := m[f.Index]; taken && cur.HasLCD && !f.HasLCD {
			continue
		}
		m[f.Index] = ArrangeFan{FanPos: f, Number: nums[f.ID]}
	}
	highest := ControllerPorts
	for p := range byPort {
		highest = max(highest, p)
	}
	total := 0
	for p := 1; p <= highest; p++ {
		ap := ArrangePort{Port: p, LCD: p >= FirstLCDPort, Fans: []ArrangeFan{}}
		for _, f := range byPort[p] {
			ap.Fans = append(ap.Fans, f)
		}
		sort.Slice(ap.Fans, func(i, j int) bool { return ap.Fans[i].Index < ap.Fans[j].Index })
		lcd := 0
		for _, f := range ap.Fans {
			if f.HasLCD {
				lcd++
			}
		}
		total += lcd
		if lcd > 0 && p < FirstLCDPort {
			a.Warnings = append(a.Warnings, ArrangeWarning{Code: "lcdPort1", Port: p, N: lcd})
		}
		if lcd > MaxLCDPerPort {
			a.Warnings = append(a.Warnings, ArrangeWarning{Code: "portLCD", Port: p, N: lcd})
		}
		if len(ap.Fans) > MaxFansPerPort {
			a.Warnings = append(a.Warnings, ArrangeWarning{Code: "portFans", Port: p, N: len(ap.Fans)})
		}
		a.Ports = append(a.Ports, ap)
	}
	if total > MaxLCDTotal {
		a.Warnings = append(a.Warnings, ArrangeWarning{Code: "totalLCD", N: total})
	}
	return a
}

// IdentifyPort shows the numbers on the LCD fans of one physical port.
func (e *Engine) IdentifyPort(port int) error {
	var ids []string
	for _, p := range e.Arrangement().Ports {
		if p.Port == port {
			for _, f := range p.Fans {
				if f.HasLCD && f.ID != "" {
					ids = append(ids, f.ID)
				}
			}
		}
	}
	if len(ids) == 0 {
		return errors.New("no LCD fans on this port")
	}
	return e.IdentifyIDs(ids)
}
