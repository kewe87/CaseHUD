package sketch

import (
	"fmt"
	"sort"
	"strings"
)

// DetectedZone is an LED zone of a detected device.
type DetectedZone struct {
	ID    string `json:"id"`
	Count int    `json:"count"` // LEDs the zone has now (0: unknown)
	Max   int    `json:"max"`   // most LEDs it accepts (0: no limit known)
}

// DetectedDevice is a detected LED device.
type DetectedDevice struct {
	ID      string         `json:"id"`
	Backend string         `json:"backend"`
	Zones   []DetectedZone `json:"zones"`
	// Mode is the lighting mode of a fan controller ("motherboard": it
	// shows the chain of a mainboard ARGB header on all ports; "": not known).
	Mode string `json:"mode,omitempty"`
	// SyncSource is the ARGB header ("device/zone") a fan controller's sync
	// input is connected to, as the user set it ("" : not set).
	SyncSource string `json:"syncSource,omitempty"`
}

// DetectedFan is a fan found on the fan controller.
type DetectedFan struct {
	Port  int  `json:"port"` // 1-based
	Index int  `json:"index"`
	LCD   bool `json:"lcd"`
}

// SuggestInput describes what the program found. Existing is the sketch the
// suggestions are meant for (zero value: none yet).
type SuggestInput struct {
	Devices  []DetectedDevice
	Fans     []DetectedFan
	Existing Sketch
}

// SuggestNote tells why something was not suggested.
type SuggestNote struct {
	Device string `json:"device,omitempty"`
	Msg    string `json:"msg"`
}

// Suggestion is a start for the sketch: placements, bound where the LED zone
// takes the part, at positions meant as a first guess only.
type Suggestion struct {
	Parts []Placement   `json:"parts"`
	Notes []SuggestNote `json:"notes"`
}

// Backend names Suggest knows: the prefix of the ids the LED backends report.
const (
	backendTL   = "lianlitl"
	backendRAM  = "enram"
	backendBoar = "asusaura"
	backendRing = "lianli"
)

type suggester struct {
	c      Catalog
	k      Case
	in     SuggestInput
	out    Suggestion
	ids    map[string]bool
	chains map[[2]string]int // parts already in a (device, zone) chain
	board  string            // id of a mainboard placement (existing or suggested)
}

// Suggest proposes placements for detected hardware: TL fans (and their
// display variant), memory modules, the mainboard's onboard LEDs and the LED
// ring of the 8.8 inch screen. Zones that the existing sketch already binds
// are left alone. It is pure: nothing is stored or sent.
func Suggest(c Catalog, k Case, in SuggestInput) Suggestion {
	s := &suggester{c: c, k: k, in: in, ids: map[string]bool{}, chains: map[[2]string]int{}}
	s.out.Parts, s.out.Notes = []Placement{}, []SuggestNote{}
	for _, p := range in.Existing.Parts {
		s.ids[p.ID] = true
		if part, ok := c.Parts[p.Part]; ok && part.Category == "mainboard" && s.board == "" {
			s.board = p.ID
		}
		if b := p.Bind; b != nil {
			s.chains[[2]string{b.Device, b.Zone}]++
		}
	}
	devs := append([]DetectedDevice(nil), in.Devices...)
	sort.Slice(devs, func(i, j int) bool { return devs[i].ID < devs[j].ID })
	for _, d := range devs {
		if d.Backend == backendBoar {
			s.mainboard(d)
		}
	}
	for _, d := range devs {
		switch d.Backend {
		case backendRAM:
			s.ram(d)
		case backendRing:
			s.ring(d)
		case backendTL:
			s.fans(d)
		}
	}
	return s.out
}

func (s *suggester) note(dev, format string, a ...any) {
	s.out.Notes = append(s.out.Notes, SuggestNote{Device: dev, Msg: fmt.Sprintf(format, a...)})
}

func (s *suggester) newID(prefix string) string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s-%d", prefix, n)
		if !s.ids[id] {
			s.ids[id] = true
			return id
		}
	}
}

// bound reports whether the existing sketch already binds the zone.
func (s *suggester) bound(dev, zone string) bool {
	for _, p := range s.in.Existing.Parts {
		if p.Bind != nil && p.Bind.Device == dev && p.Bind.Zone == zone {
			return true
		}
	}
	return false
}

// byCategory lists the catalog parts of a category, sorted by id.
func (s *suggester) byCategory(cat string) []Part {
	var out []Part
	for _, p := range s.c.Parts {
		if p.Category == cat {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// withLEDs picks the first part with exactly n LEDs: a fixed count, or a
// parametric part that can make n (the second result is then n).
func withLEDs(parts []Part, n int) (Part, int, bool) {
	for _, p := range parts {
		if g := p.LEDs.Generator; g != nil {
			if n >= g.Min && n <= g.Max {
				return p, n, true
			}
			continue
		}
		if len(p.LEDs.Positions) == n {
			return p, 0, true
		}
	}
	return Part{}, 0, false
}

// add validates the placement and keeps it, bound to z when given.
func (s *suggester) add(p Placement, z *DetectedZone, dev string) {
	if len(s.in.Existing.Parts)+len(s.out.Parts) >= MaxParts {
		s.note(dev, "part limit reached, not everything is suggested")
		return
	}
	if z != nil {
		key := [2]string{dev, z.ID}
		if s.chains[key] < MaxZoneSegments {
			p.Bind = &Binding{Device: dev, Zone: z.ID, Order: s.chains[key]}
		}
	}
	if err := validatePlacement(p, s.k.Dims, s.c); err != nil {
		s.note(dev, "%s: %v", p.Part, err)
		return
	}
	if p.Bind != nil {
		s.chains[[2]string{dev, z.ID}]++
	}
	s.out.Parts = append(s.out.Parts, p)
}

func (s *suggester) mainboard(d DetectedDevice) {
	for i := range d.Zones {
		z := &d.Zones[i]
		if z.ID != "onboard" || z.Count == 0 || s.bound(d.ID, z.ID) {
			continue
		}
		part, _, ok := withLEDs(s.byCategory("mainboard"), z.Count)
		if !ok {
			s.note(d.ID, "no mainboard with %d LEDs in the catalog", z.Count)
			continue
		}
		id := s.newID("mb")
		s.board = id
		s.add(Placement{ID: id, Part: part.ID, Facing: FacingSide,
			Pos: Vec3{X: s.k.Dims.W - part.Dimensions.D/2, Y: 20 + part.Dimensions.H/2, Z: 20 + part.Dimensions.W/2}}, z, d.ID)
	}
}

func (s *suggester) ram(d DetectedDevice) {
	for i := range d.Zones {
		z := &d.Zones[i]
		if z.Count == 0 || s.bound(d.ID, z.ID) {
			continue
		}
		part, leds, ok := withLEDs(s.byCategory("ram"), z.Count)
		if !ok {
			s.note(d.ID, "no memory module with %d LEDs in the catalog", z.Count)
			continue
		}
		n := 0
		for _, p := range s.out.Parts {
			if strings.HasPrefix(p.ID, "ram-") {
				n++
			}
		}
		pl := Placement{ID: s.newID("ram"), Part: part.ID, Facing: FacingSide, Spin: 90, LEDs: leds,
			Pos: Vec3{X: s.k.Dims.W - part.Dimensions.D/2, Y: 30 + part.Dimensions.W/2, Z: 180 + 20*float64(n)}}
		if s.board != "" {
			pl.On = s.board
		}
		s.add(pl, z, d.ID)
	}
}

func (s *suggester) ring(d DetectedDevice) {
	for i := range d.Zones {
		z := &d.Zones[i]
		if z.Count == 0 || s.bound(d.ID, z.ID) {
			continue
		}
		part, leds, ok := withLEDs(s.byCategory("display"), z.Count)
		if !ok {
			s.note(d.ID, "no display with a ring of %d LEDs in the catalog", z.Count)
			continue
		}
		dm := s.k.Dims
		s.add(Placement{ID: s.newID("ring"), Part: part.ID, Facing: FacingSide, LEDs: leds,
			Pos: Vec3{X: part.Dimensions.D / 2, Y: dm.H - 40 - part.Dimensions.H/2, Z: dm.D / 2}}, z, d.ID)
	}
}

// fanPart picks the TL fan part: the display variant for a fan with a
// screen, preferring reversed blades, else the generic fan.
func (s *suggester) fanPart(lcd bool) Part {
	cat := "fan"
	if lcd {
		cat = "fan-lcd"
	}
	var best Part
	for _, p := range s.byCategory(cat) {
		if strings.HasPrefix(p.ID, "uni-fan-tl") && (best.ID == "" || strings.Contains(p.ID, "reverse")) {
			best = p
		}
	}
	if best.ID == "" {
		best = s.c.Parts["generic-fan-120"]
	}
	return best
}

func (s *suggester) fans(d DetectedDevice) {
	fans := append([]DetectedFan(nil), s.in.Fans...)
	sort.Slice(fans, func(i, j int) bool {
		if fans[i].Port != fans[j].Port {
			return fans[i].Port < fans[j].Port
		}
		return fans[i].Index < fans[j].Index
	})
	dm := s.k.Dims
	unbound := false
	for k, f := range fans {
		part := s.fanPart(f.LCD)
		if part.ID == "" {
			s.note(d.ID, "no fan part in the catalog")
			return
		}
		// A column against the glass at the front, wrapping towards the rear.
		step := part.Dimensions.W + 5
		perCol := max(1, int((dm.H-40)/step))
		col, row := k/perCol, k%perCol
		pl := Placement{ID: s.newID("fan"), Part: part.ID, Facing: FacingSide, Flow: "intake",
			Label: fmt.Sprintf("P%d.%d", f.Port, f.Index+1),
			Pos: Vec3{X: part.Dimensions.D / 2, Y: 20 + part.Dimensions.W/2 + float64(row)*step,
				Z: dm.D - 20 - part.Dimensions.W/2 - float64(col)*step}}
		// The controller reports two LEDs per fan while the part has more, so a
		// zone is only bound when it takes the whole chain.
		var zone *DetectedZone
		for i := range d.Zones {
			if d.Zones[i].ID == fmt.Sprintf("port%d", f.Port) {
				zone = &d.Zones[i]
			}
		}
		if zone != nil && !s.bound(d.ID, zone.ID) && zone.Max > 0 &&
			zone.Max >= len(part.LEDs.Positions)*(s.chains[[2]string{d.ID, zone.ID}]+1) {
			s.add(pl, zone, d.ID)
		} else {
			s.add(pl, nil, d.ID)
			unbound = true
		}
	}
	if unbound {
		s.note(d.ID, "fans stay unbound where the controller zone holds fewer LEDs than the fan part")
	}
}
