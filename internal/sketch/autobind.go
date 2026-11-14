package sketch

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// AutoBindInput is the hardware the program found.
type AutoBindInput struct {
	Devices []DetectedDevice
	Fans    []DetectedFan // fans on the TL controller (empty: list not read)
	// SyncSource is the mainboard header ("device/zone") the user confirmed
	// for a fan controller in TLModeSync that has no sync source set.
	SyncSource string
}

// AutoBinding is one binding AutoBind found.
type AutoBinding struct {
	Part   string `json:"part"`
	Device string `json:"device"`
	Zone   string `json:"zone"`
	Order  int    `json:"order"`
	// Kind is what made it certain: "fans" (TL port), "sync" (TL fan on the
	// mainboard header the controller follows), "ram", "board", "ring".
	Kind string `json:"kind"`
	// Parallel: wired in parallel with the previous part of the chain (a TL
	// fan of another port on the sync header).
	Parallel bool `json:"parallel,omitempty"`
}

// AutoZone is a free zone that can be lit to find the parts on it.
type AutoZone struct {
	Device  string `json:"device"`
	Zone    string `json:"zone"`
	Backend string `json:"backend"`
	// Port is the TL controller port (1-based) of a fan zone, 0 otherwise.
	Port int `json:"port,omitempty"`
}

// AutoNote tells why parts were left unbound. Code is one of "tl-same"
// (several ports carry the same fans), "tl-count" (no group of fans fits a
// port), "tl-devices" (more than one fan controller), "leds" (the part has
// more LEDs than the zone reports: bound anyway, the rest stays dark),
// "leds-max" (the part has more LEDs than the zone can drive: left open),
// "ram-count" and "board-count" (the number of parts and zones differs),
// "sync-source" (the controller follows a mainboard header, but which one is
// not set and no single header fits; N: the LEDs it needs) and "sync-used"
// (that header already has parts).
type AutoNote struct {
	Code   string   `json:"code"`
	Device string   `json:"device,omitempty"`
	Zone   string   `json:"zone,omitempty"`
	Parts  []string `json:"parts,omitempty"`
	N      int      `json:"n,omitempty"` // "leds": LEDs of the part
	M      int      `json:"m,omitempty"` // "leds": LEDs of the zone
}

// AutoBindResult is what AutoBind found: the certain bindings, the parts it
// left open and the free zones a guided search can light one by one.
type AutoBindResult struct {
	Binds []AutoBinding `json:"binds"`
	Open  []string      `json:"open"`
	Zones []AutoZone    `json:"zones"`
	Notes []AutoNote    `json:"notes"`
	// Sync is set when the TL fans were laid out on the mainboard header the
	// fan controller follows (or are proposed for it).
	Sync *AutoSync `json:"sync,omitempty"`
}

// tlPartPrefix marks the fans of the TL controller in the catalog.
const tlPartPrefix = "uni-fan-tl"

type autoBinder struct {
	s       Sketch
	c       Catalog
	in      AutoBindInput
	net     Net
	faces   []string
	out     AutoBindResult
	taken   map[string]bool    // part ids bound (before or now)
	used    map[[2]string]bool // zones with a part (before or now)
	backend map[string]string  // device id -> backend
}

// AutoBind binds the unbound parts of a sketch whose zone is certain, and
// only those:
//
//   - TL fans to the ports of the fan controller. A port's fans (how many,
//     how many with a display) must match exactly one group of fan parts on
//     one face of the case, and that group no other port. The fans of a port
//     are ordered by the numbers their labels end in when each has its own,
//     else as the group runs on its face (left to right, or top to bottom
//     when the group is taller than wide).
//   - In TLModeSync the TL fans to the mainboard header the controller
//     follows instead, one port chain in order, the others in parallel
//     (syncFans).
//   - Memory modules to the memory zones, left to right as seen through the
//     glass, in the order the modules report (slot address), when there are
//     as many modules as zones.
//   - The mainboard to the onboard zone, when there is one of each.
//   - The LED ring of the display to the ring zone, one of each.
//
// Parts already bound and zones that already have a part are left alone.
// Everything else is listed as open, with the free zones that a guided
// search can light one at a time. It is pure: nothing is stored or sent.
//
// When these rules learn new cases, raise AUTO_RULES in lighting.js so the
// page offers parts it looked at before once more.
func AutoBind(s Sketch, c Catalog, in AutoBindInput) AutoBindResult {
	a := &autoBinder{s: s, c: c, in: in, taken: map[string]bool{}, used: map[[2]string]bool{}, backend: map[string]string{}}
	a.out = AutoBindResult{Binds: []AutoBinding{}, Open: []string{}, Zones: []AutoZone{}, Notes: []AutoNote{}}
	a.faces, a.net = SketchNet(s, c)
	for _, d := range in.Devices {
		a.backend[d.ID] = d.Backend
	}
	for _, p := range s.Parts {
		if b := p.Bind; b != nil && b.Device != "" {
			a.taken[p.ID] = true
			a.used[[2]string{b.Device, b.Zone}] = true
		}
	}
	devs := append([]DetectedDevice(nil), in.Devices...)
	sort.SliceStable(devs, func(i, j int) bool { return devs[i].ID < devs[j].ID })
	a.fans(devs)
	a.ram(devs)
	a.single(devs, backendBoar, "onboard", "mainboard", "board", "board-count")
	a.single(devs, backendRing, "", "display", "ring", "")
	for _, p := range s.Parts {
		if a.taken[p.ID] {
			continue
		}
		if part, ok := c.Parts[p.Part]; ok && PartLEDCount(part, p) > 0 {
			a.out.Open = append(a.out.Open, p.ID)
		}
	}
	if len(a.out.Open) > 0 {
		for _, d := range devs {
			switch d.Backend {
			case backendBoar, backendRAM, backendTL, backendRing:
			default:
				continue
			}
			for _, z := range d.Zones {
				if a.used[[2]string{d.ID, z.ID}] || (z.Count == 0 && z.Max == 0) { // nothing can hang on an empty port
					continue
				}
				az := AutoZone{Device: d.ID, Zone: z.ID, Backend: d.Backend}
				if d.Backend == backendTL {
					az.Port = portOf(z.ID)
				}
				a.out.Zones = append(a.out.Zones, az)
			}
		}
	}
	return a.out
}

// Only keeps the bindings of the given parts; the other parts AutoBind
// would have bound are listed as open again (nil: all are kept).
func (r AutoBindResult) Only(parts []string) AutoBindResult {
	if parts == nil {
		return r
	}
	keep := make(map[string]bool, len(parts))
	for _, id := range parts {
		keep[id] = true
	}
	// the sync chain is kept as a whole: a parallel fan without the fan
	// before it would mirror the wrong one
	syncAll := true
	for _, b := range r.Binds {
		if b.Kind == "sync" && !keep[b.Part] {
			syncAll = false
		}
	}
	out := r
	out.Binds = []AutoBinding{}
	out.Open = append([]string(nil), r.Open...)
	for _, b := range r.Binds {
		if keep[b.Part] && (b.Kind != "sync" || syncAll) {
			out.Binds = append(out.Binds, b)
		} else {
			out.Open = append(out.Open, b.Part)
		}
	}
	if !syncAll && out.Sync != nil && !out.Sync.Derived {
		out.Sync = nil
	}
	return out
}

// ApplyAutoBind returns s with the bindings of r set.
func ApplyAutoBind(s Sketch, r AutoBindResult) Sketch {
	by := make(map[string]AutoBinding, len(r.Binds))
	for _, b := range r.Binds {
		by[b.Part] = b
	}
	out := s
	out.Parts = make([]Placement, len(s.Parts))
	for i, p := range s.Parts {
		if b, ok := by[p.ID]; ok && p.Bind == nil {
			p.Bind = &Binding{Device: b.Device, Zone: b.Zone, Order: b.Order, Parallel: b.Parallel}
		}
		out.Parts[i] = p
	}
	return out
}

func (a *autoBinder) bind(p Placement, dev, zone string, order int, kind string) {
	a.out.Binds = append(a.out.Binds, AutoBinding{Part: p.ID, Device: dev, Zone: zone, Order: order, Kind: kind})
	a.taken[p.ID] = true
	a.used[[2]string{dev, zone}] = true
}

// free lists the unbound parts the filter accepts, in sketch order, with
// their index.
func (a *autoBinder) free(ok func(Part, Placement) bool) []int {
	var out []int
	for i, p := range a.s.Parts {
		if a.taken[p.ID] || p.IsCaseLight() {
			continue
		}
		if part, found := a.c.Parts[p.Part]; found && ok(part, p) {
			out = append(out, i)
		}
	}
	return out
}

// centre is the middle of a placement in case coordinates.
func (a *autoBinder) centre(i int) Vec3 {
	return a.s.Parts[i].Pos
}

// netPos is where a placement sits on the net, on the given face (own face
// when face is ""); ok is false when that face is not in the net.
func (a *autoBinder) netPos(i int, face string) (Point, bool) {
	if face == "" {
		face = a.faces[i]
	}
	return a.net.Point(face, a.centre(i))
}

// runOrder sorts parts as they run on their face: along the axis the group
// spreads most on the net, left to right or top to bottom.
func (a *autoBinder) runOrder(idx []int) []int {
	type pt struct {
		i    int
		x, y float64
	}
	pts := make([]pt, 0, len(idx))
	minX, maxX, minY, maxY := 1e18, -1e18, 1e18, -1e18
	for _, i := range idx {
		q, _ := a.netPos(i, "")
		pts = append(pts, pt{i, q.X, q.Y})
		minX, maxX = min(minX, q.X), max(maxX, q.X)
		minY, maxY = min(minY, q.Y), max(maxY, q.Y)
	}
	byX := maxX-minX >= maxY-minY
	sort.SliceStable(pts, func(j, k int) bool {
		p, q := pts[j], pts[k]
		if byX && p.x != q.x {
			return p.x < q.x
		}
		if p.y != q.y {
			return p.y < q.y
		}
		return p.x < q.x
	})
	out := make([]int, len(pts))
	for j, p := range pts {
		out[j] = p.i
	}
	return out
}

// chainOrder is the order of a port's fans: by the numbers that end their
// labels when every fan has its own ("Top 1", "P1.2"), else as they run on
// their face (runOrder).
func (a *autoBinder) chainOrder(idx []int) []int {
	num := make(map[int]int, len(idx))
	seen := map[int]bool{}
	for _, i := range idx {
		m := trailingNum.FindStringSubmatch(a.s.Parts[i].Label)
		if m == nil {
			return a.runOrder(idx)
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			return a.runOrder(idx)
		}
		seen[n], num[i] = true, n
	}
	out := append([]int(nil), idx...)
	sort.SliceStable(out, func(j, k int) bool { return num[out[j]] < num[out[k]] })
	return out
}

var trailingNum = regexp.MustCompile(`(\d{1,3})\s*$`)

func portOf(zone string) int {
	var n int
	if _, err := fmt.Sscanf(zone, "port%d", &n); err != nil {
		return 0
	}
	return n
}

// fanSig is what a port or a group of fan parts holds: fans without and
// with a display (lcd -1: not known, only the total counts).
type fanSig struct{ plain, lcd int }

func (f fanSig) fits(g fanSig) bool {
	if f.lcd < 0 || g.lcd < 0 {
		return f.plain+max(f.lcd, 0) == g.plain+max(g.lcd, 0)
	}
	return f == g
}

// tlPort is a port of the fan controller with fans on it.
type tlPort struct {
	zone string
	n    int // 1-based port number
	sig  fanSig
}

// tlChain is the fan parts of one port (port 0: not known), in chain order.
type tlChain struct {
	port tlPort
	idx  []int
}

func (a *autoBinder) fans(devs []DetectedDevice) {
	var tl []DetectedDevice
	for _, d := range devs {
		if d.Backend == backendTL {
			tl = append(tl, d)
		}
	}
	if len(tl) == 0 {
		return
	}
	if len(tl) > 1 {
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "tl-devices"})
		return
	}
	dev := tl[0]
	if dev.Mode == TLModeSync {
		a.syncFans(dev, devs)
		return
	}
	chains, _, _, note := a.matchPorts(dev)
	for _, ch := range chains {
		for k, i := range ch.idx {
			a.bind(a.s.Parts[i], dev.ID, ch.port.zone, k, "fans")
		}
	}
	if note != nil {
		a.out.Notes = append(a.out.Notes, *note)
	}
}

// tlPorts are the free ports of the fan controller that have fans.
func (a *autoBinder) tlPorts(dev DetectedDevice) []tlPort {
	var ports []tlPort
	for _, z := range dev.Zones {
		n := portOf(z.ID)
		if n == 0 || a.used[[2]string{dev.ID, z.ID}] {
			continue
		}
		sig := fanSig{lcd: -1, plain: z.Count / 2}
		if len(a.in.Fans) > 0 {
			sig = fanSig{}
			for _, f := range a.in.Fans {
				if f.Port != n {
					continue
				}
				if f.LCD {
					sig.lcd++
				} else {
					sig.plain++
				}
			}
		}
		if sig.plain+max(sig.lcd, 0) > 0 {
			ports = append(ports, tlPort{z.ID, n, sig})
		}
	}
	return ports
}

// fanGroup is the free TL fan parts on one face of the case.
type fanGroup struct {
	face string
	idx  []int
	sig  fanSig
	done bool
}

// matchPorts finds the fan parts of every port: a port's fans (how many,
// how many with a display) must match exactly one group of fan parts on one
// face, and that group no other port; one port left over takes the rest.
// It returns the chains found (fans in chain order), the groups left over,
// the number of ports left without fans and a note when fans stay unmatched.
func (a *autoBinder) matchPorts(dev DetectedDevice) ([]tlChain, []*fanGroup, int, *AutoNote) {
	ports := a.tlPorts(dev)
	idx := a.free(func(part Part, _ Placement) bool { return strings.HasPrefix(part.ID, tlPartPrefix) })
	if len(ports) == 0 || len(idx) == 0 {
		return nil, nil, 0, nil
	}
	var groups []*fanGroup
	byFace := map[string]*fanGroup{}
	for _, i := range idx {
		f := a.faces[i]
		g := byFace[f]
		if g == nil {
			g = &fanGroup{face: f}
			byFace[f] = g
			groups = append(groups, g)
		}
		g.idx = append(g.idx, i)
		if a.c.Parts[a.s.Parts[i].Part].Category == "fan-lcd" {
			g.sig.lcd++
		} else {
			g.sig.plain++
		}
	}
	var chains []tlChain
	done := make([]bool, len(ports))
	for changed := true; changed; {
		changed = false
		for pi, p := range ports {
			if done[pi] {
				continue
			}
			var cand []*fanGroup
			for _, g := range groups {
				if !g.done && p.sig.fits(g.sig) {
					cand = append(cand, g)
				}
			}
			if len(cand) != 1 {
				continue
			}
			rivals := 0 // other open ports the same group would fit
			for qi, q := range ports {
				if qi != pi && !done[qi] && q.sig.fits(cand[0].sig) {
					rivals++
				}
			}
			if rivals > 0 {
				continue
			}
			chains = append(chains, tlChain{p, a.chainOrder(cand[0].idx)})
			cand[0].done, done[pi], changed = true, true, true
		}
	}
	// one port left and the rest of the fans make exactly its set
	var left []int
	for pi := range ports {
		if !done[pi] {
			left = append(left, pi)
		}
	}
	var rest []int
	var restSig fanSig
	var restGroups []*fanGroup
	for _, g := range groups {
		if !g.done {
			rest = append(rest, g.idx...)
			restSig.plain += g.sig.plain
			restSig.lcd += g.sig.lcd
			restGroups = append(restGroups, g)
		}
	}
	if len(left) == 1 && len(rest) > 0 && ports[left[0]].sig.fits(restSig) {
		return append(chains, tlChain{ports[left[0]], a.chainOrder(rest)}), nil, 0, nil
	}
	if len(left) == 0 || len(rest) == 0 {
		return chains, restGroups, len(left), nil
	}
	ids := make([]string, len(rest))
	for k, i := range rest {
		ids[k] = a.s.Parts[i].ID
	}
	code := "tl-count"
	for _, pi := range left {
		for _, qi := range left {
			if pi < qi && ports[pi].sig.fits(ports[qi].sig) {
				code = "tl-same"
			}
		}
	}
	return chains, restGroups, len(left), &AutoNote{Code: code, Device: dev.ID, Parts: ids}
}

// ram binds memory modules to the memory zones in slot order.
func (a *autoBinder) ram(devs []DetectedDevice) {
	type zr struct {
		dev  string
		zone DetectedZone
	}
	var zones []zr
	for _, d := range devs {
		if d.Backend != backendRAM {
			continue
		}
		for _, z := range d.Zones {
			if !a.used[[2]string{d.ID, z.ID}] {
				zones = append(zones, zr{d.ID, z})
			}
		}
	}
	idx := a.free(func(part Part, _ Placement) bool { return part.Category == "ram" })
	if len(zones) == 0 || len(idx) == 0 {
		return
	}
	if len(zones) != len(idx) {
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "ram-count", N: len(idx), M: len(zones)})
		return
	}
	// left to right as seen through the glass (the side face), else on
	// their own face
	type pt struct {
		i    int
		x, y float64
	}
	pts := make([]pt, len(idx))
	for k, i := range idx {
		q, ok := a.netPos(i, FacingSide)
		if !ok {
			q, _ = a.netPos(i, "")
		}
		pts[k] = pt{i, q.X, q.Y}
	}
	sort.SliceStable(pts, func(j, k int) bool {
		if pts[j].x != pts[k].x {
			return pts[j].x < pts[k].x
		}
		return pts[j].y < pts[k].y
	})
	for k, p := range pts {
		a.bindChecked(p.i, zones[k].dev, zones[k].zone, "ram")
	}
}

// bindChecked binds a part to a fixed zone and notes when the part has
// more LEDs than the zone reports. A part with more LEDs than the zone can
// drive at all stays open: the lighting would refuse the sketch.
func (a *autoBinder) bindChecked(i int, dev string, z DetectedZone, kind string) {
	p := a.s.Parts[i]
	n := PartLEDCount(a.c.Parts[p.Part], p)
	if z.Max > 0 && n > z.Max {
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "leds-max", Device: dev, Zone: z.ID, Parts: []string{p.ID}, N: n, M: z.Max})
		return
	}
	if z.Count > 0 && n > z.Count {
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "leds", Device: dev, Zone: z.ID, Parts: []string{p.ID}, N: n, M: z.Count})
	}
	a.bind(p, dev, z.ID, 0, kind)
}

// single binds the one free part of a category to the one free zone of a
// backend (zone "" : any zone), when there is exactly one of each.
func (a *autoBinder) single(devs []DetectedDevice, backend, zone, category, kind, countNote string) {
	type zr struct {
		dev  string
		zone DetectedZone
	}
	var zones []zr
	for _, d := range devs {
		if d.Backend != backend {
			continue
		}
		for _, z := range d.Zones {
			if (zone == "" || z.ID == zone) && !a.used[[2]string{d.ID, z.ID}] {
				zones = append(zones, zr{d.ID, z})
			}
		}
	}
	idx := a.free(func(part Part, _ Placement) bool { return part.Category == category })
	if len(zones) == 0 || len(idx) == 0 {
		return
	}
	if len(zones) != 1 || len(idx) != 1 {
		if countNote != "" {
			a.out.Notes = append(a.out.Notes, AutoNote{Code: countNote, N: len(idx), M: len(zones)})
		}
		return
	}
	a.bindChecked(idx[0], zones[0].dev, zones[0].zone, kind)
}
