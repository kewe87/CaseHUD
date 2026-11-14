package sketch

import (
	"sort"
	"strings"
)

// TLModeSync is the fan controller mode in which it shows the chain of a
// mainboard ARGB header on all its ports in parallel: every port shows fan
// 1, fan 2, ... of that chain, TLSyncLEDs LEDs each.
const TLModeSync = "motherboard"

// TLSyncLEDs is the number of LEDs per fan on the header in TLModeSync.
const TLSyncLEDs = 26

// AutoSync is how AutoBind laid the TL fans out on the mainboard header the
// fan controller follows (TLModeSync).
type AutoSync struct {
	Device string `json:"device"` // the fan controller
	Board  string `json:"board"`  // the header: device ...
	Zone   string `json:"zone"`   // ... and zone
	// Fans is the number of fans of the longest port: the header carries
	// Fans x TLSyncLEDs LEDs.
	Fans int `json:"fans"`
	// Main is the chain the header's LEDs are laid out on, in order; the
	// fans of the other ports are wired in parallel to its fans.
	Main []string `json:"main"`
	// Derived: the controller has no header set, this one is the only one
	// with the right number of LEDs. The bindings are only proposed
	// (Proposed) until the user confirms the header (AutoBindInput.SyncSource).
	Derived  bool          `json:"derived,omitempty"`
	Proposed []AutoBinding `json:"proposed,omitempty"`
}

// faceRank orders faces by how well they are seen: the glass side first.
var faceRank = map[string]int{FacingSide: 0, FacingFront: 1, FacingTop: 2, FacingRear: 3, FacingBottom: 4, FacingTray: 5}

// syncFans binds the TL fans to the mainboard header the controller
// follows. That header's chain is shown on every port at once, so one port
// chain (the main chain) is laid out on the header in order and every other
// port's fan k is wired in parallel to fan k of the main chain: it shows the
// same colours, which is what the hardware does.
//
// The main chain is the one with the most fans: the header must carry all
// its LEDs, and a shorter chain would leave fans of the longest port without
// a place on the canvas. Ties go to the chain on the best seen face (glass
// side, front, top, rear, bottom, tray), then to the lower port: its LED
// positions are the ones the effect is sampled at, so the fans the user
// looks at show the effect as drawn.
//
// The header is the controller's sync source; when that is not set, the
// header the user confirmed (AutoBindInput.SyncSource); else the only free
// mainboard ARGB header with Fans x TLSyncLEDs LEDs, as a proposal only.
// Nothing is written to the controller.
func (a *autoBinder) syncFans(dev DetectedDevice, devs []DetectedDevice) {
	ports := a.tlPorts(dev)
	chains, rest, left, _ := a.matchPorts(dev)
	fans := 0
	for _, p := range ports {
		fans = max(fans, p.sig.plain+max(p.sig.lcd, 0))
	}
	// groups whose port is not known (the fan list was not read, or ports
	// look alike): every port shows the same chain, so a group only needs a
	// port of its own and no more fans than the longest port
	var over []string
	for _, g := range rest {
		if left > 0 && len(g.idx) <= fans {
			chains = append(chains, tlChain{idx: a.chainOrder(g.idx)})
			left--
			continue
		}
		for _, i := range g.idx {
			over = append(over, a.s.Parts[i].ID)
		}
	}
	if len(over) > 0 {
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "tl-count", Device: dev.ID, Parts: over})
	}
	if len(chains) == 0 {
		return
	}
	var fit []tlChain
	for _, ch := range chains {
		if len(ch.idx) > fans {
			continue // cannot happen: a matched port holds the chain
		}
		fit = append(fit, ch)
	}
	if len(fit) == 0 {
		return
	}
	sort.SliceStable(fit, func(i, j int) bool {
		p, q := fit[i], fit[j]
		if len(p.idx) != len(q.idx) {
			return len(p.idx) > len(q.idx)
		}
		if fp, fq := a.chainRank(p), a.chainRank(q); fp != fq {
			return fp < fq
		}
		if (p.port.n == 0) != (q.port.n == 0) {
			return q.port.n == 0
		}
		return p.port.n < q.port.n
	})
	var all []string
	for _, ch := range fit {
		for _, i := range ch.idx {
			all = append(all, a.s.Parts[i].ID)
		}
	}
	src, derived := dev.SyncSource, false
	if src == "" {
		src = a.in.SyncSource
	}
	if src == "" {
		var cand []string
		for _, d := range devs {
			if d.Backend != backendBoar {
				continue
			}
			for _, z := range d.Zones {
				if strings.HasPrefix(z.ID, "argb") && z.Count == fans*TLSyncLEDs && !a.used[[2]string{d.ID, z.ID}] {
					cand = append(cand, d.ID+"/"+z.ID)
				}
			}
		}
		if len(cand) != 1 {
			a.out.Notes = append(a.out.Notes, AutoNote{Code: "sync-source", Device: dev.ID, Parts: all, N: fans * TLSyncLEDs})
			return
		}
		src, derived = cand[0], true
	}
	board, zone, _ := strings.Cut(src, "/")
	var hz *DetectedZone
	for _, d := range devs {
		if d.ID != board || d.Backend != backendBoar {
			continue
		}
		for k := range d.Zones {
			if d.Zones[k].ID == zone && strings.HasPrefix(zone, "argb") {
				hz = &d.Zones[k]
			}
		}
	}
	switch {
	case hz == nil:
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "sync-source", Device: dev.ID, Parts: all, N: fans * TLSyncLEDs})
		return
	case a.used[[2]string{board, zone}]:
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "sync-used", Device: board, Zone: zone, Parts: all})
		return
	case hz.Max > 0 && fans*TLSyncLEDs > hz.Max:
		a.out.Notes = append(a.out.Notes, AutoNote{Code: "leds-max", Device: board, Zone: zone, Parts: all[:1], N: fans * TLSyncLEDs, M: hz.Max})
		return
	}
	main := fit[0]
	var list []AutoBinding
	order := 0
	add := func(i int, parallel bool) {
		list = append(list, AutoBinding{Part: a.s.Parts[i].ID, Device: board, Zone: zone, Order: order, Kind: "sync", Parallel: parallel})
		order++
	}
	sync := &AutoSync{Device: dev.ID, Board: board, Zone: zone, Fans: fans, Derived: derived}
	for k, i := range main.idx {
		sync.Main = append(sync.Main, a.s.Parts[i].ID)
		add(i, false)
		for _, ch := range fit[1:] {
			if k < len(ch.idx) {
				add(ch.idx[k], true)
			}
		}
	}
	a.out.Sync = sync
	if derived {
		sync.Proposed = list
		return
	}
	for _, b := range list {
		a.out.Binds = append(a.out.Binds, b)
		a.taken[b.Part] = true
	}
	a.used[[2]string{board, zone}] = true
}

// chainRank is how well the face of a chain is seen (faceRank of its first fan).
func (a *autoBinder) chainRank(ch tlChain) int {
	if r, ok := faceRank[a.faces[ch.idx[0]]]; ok {
		return r
	}
	return len(faceRank)
}
