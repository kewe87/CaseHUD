package sketch

import "strings"

// RingZone reports whether a zone takes one colour per ring of a fan rather
// than one per LED: the ports of the TL fan controller report two zone LEDs
// per fan, the top ring first, then the bottom ring.
func RingZone(backend, zone string) bool {
	return backend == backendTL && strings.HasPrefix(zone, "port")
}

// RingZones says which (device, zone) pairs are ring zones (nil: none).
type RingZones func(device, zone string) bool

// RingGroups assigns every LED of a placement to a ring of the part: 0 for
// the top ring, 1 for the bottom ring (by the LED's place in the part's
// front view: above or below the middle). A part whose LEDs all sit on one
// side has one ring. Parametric parts are split in halves of their LED
// order. rings is the number of rings (0: no LEDs).
func RingGroups(part Part, p Placement) (groups []int, rings int) {
	n := PartLEDCount(part, p)
	if n <= 0 {
		return nil, 0
	}
	groups = make([]int, n)
	pos := part.LEDs.Positions
	if part.LEDs.Generator != nil || len(pos) != n {
		if n == 1 {
			return groups, 1
		}
		for i := n / 2; i < n; i++ {
			groups[i] = 1
		}
		return groups, 2
	}
	mid := part.Dimensions.H / 2
	top, bottom := false, false
	for i, q := range pos {
		if q[1] < mid {
			top = true
		} else {
			groups[i] = 1
			bottom = true
		}
	}
	if top && bottom {
		return groups, 2
	}
	clear(groups)
	return groups, 1
}

// ringCentres are the centres of the rings of a placement: the mean of its
// projected LEDs per ring, in ring order.
func ringCentres(part Part, p Placement, pts []Point) []Point {
	groups, rings := RingGroups(part, p)
	if rings == 0 || len(groups) != len(pts) {
		return pts
	}
	sum := make([]Point, rings)
	cnt := make([]int, rings)
	for i, q := range pts {
		g := groups[i]
		sum[g].X += q.X
		sum[g].Y += q.Y
		cnt[g]++
	}
	out := make([]Point, rings)
	for g := range out {
		out[g] = Point{X: sum[g].X / float64(cnt[g]), Y: sum[g].Y / float64(cnt[g])}
	}
	return out
}
