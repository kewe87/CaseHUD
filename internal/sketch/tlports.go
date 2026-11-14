package sketch

import (
	"math"
	"sort"
)

// TL fan controller ports in the hardware mode: every port runs a firmware
// effect of its own, with a direction the firmware understands (clockwise,
// up, spread, ...). To make the fans of a port run the way the CaseHUD effect
// runs over the case, the sketch says which fans sit on which port and where
// their LEDs are on the effect canvas; TLDirection picks the firmware
// direction whose LED order matches the effect best.

// TLPortFans are the fan parts of one controller port, in chain order.
type TLPortFans struct {
	Port   int      `json:"port"` // 1-based
	Parts  []string `json:"parts"`
	Labels []string `json:"labels"`
	// Direct: the parts are bound to the port in the sketch; otherwise the
	// port was matched like AutoBind does (number of fans, with and without
	// display, per face of the case).
	Direct bool `json:"direct,omitempty"`
	// Fans: per part its LEDs in wire order.
	Fans []TLFanLEDs `json:"-"`
}

// TLFanLEDs are the LEDs of one fan: where the effect samples them (canvas,
// normalised 0..1) and where they are on the part itself (front view of the
// template, millimetres, y down), with the template centre.
type TLFanLEDs struct {
	Canvas []Point
	Local  []Point
	Centre Point
}

// TLPortLayout finds the fan parts of every port of a TL controller. Parts
// bound to a port of dev count first; the ports left are matched with the
// remaining TL fan parts the way AutoBind matches them, whatever those parts
// are bound to now (in the mainboard sync mode they are bound to the
// mainboard header, but each still hangs on a port). The sketch must have its
// faces resolved (ResolveFaces). Ports without a match are left out.
func TLPortLayout(s Sketch, c Catalog, dev DetectedDevice, fans []DetectedFan) []TLPortFans {
	faces, net := SketchNet(s, c)
	ox, oy, sx, sy := Letterbox(net.W, net.H)
	byPort := map[int][]int{}
	direct := map[int]bool{}
	m := s
	m.Parts = append([]Placement(nil), s.Parts...)
	for i, p := range s.Parts {
		part, ok := c.Parts[p.Part]
		if !ok || !isTLFan(part) {
			continue
		}
		if b := p.Bind; b != nil && b.Device == dev.ID && portOf(b.Zone) > 0 {
			byPort[portOf(b.Zone)] = append(byPort[portOf(b.Zone)], i)
			direct[portOf(b.Zone)] = true
			continue
		}
		m.Parts[i].Bind = nil // matched below, whatever it is bound to now
	}
	for port, idx := range byPort {
		sort.SliceStable(idx, func(a, b int) bool { return s.Parts[idx[a]].Bind.Order < s.Parts[idx[b]].Bind.Order })
		byPort[port] = idx
	}

	a := &autoBinder{s: m, c: c, in: AutoBindInput{Devices: []DetectedDevice{dev}, Fans: fans},
		taken: map[string]bool{}, used: map[[2]string]bool{}, backend: map[string]string{dev.ID: dev.Backend}}
	a.faces, a.net = faces, net
	for _, p := range m.Parts {
		if b := p.Bind; b != nil && b.Device != "" {
			a.taken[p.ID] = true
			a.used[[2]string{b.Device, b.Zone}] = true
		}
	}
	chains, _, _, _ := a.matchPorts(dev)
	for _, ch := range chains {
		if ch.port.n > 0 && len(byPort[ch.port.n]) == 0 {
			byPort[ch.port.n] = ch.idx
		}
	}

	ports := make([]int, 0, len(byPort))
	for port := range byPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	out := make([]TLPortFans, 0, len(ports))
	for _, port := range ports {
		pf := TLPortFans{Port: port, Direct: direct[port], Parts: []string{}, Labels: []string{}}
		for _, i := range byPort[port] {
			p := s.Parts[i]
			label := p.Label
			if label == "" {
				label = p.ID
			}
			pf.Parts, pf.Labels = append(pf.Parts, p.ID), append(pf.Labels, label)
			part := c.Parts[p.Part]
			res, err := Resolve(part, p.LEDs, p.Len)
			if err != nil {
				continue
			}
			pr, err := ProjectNet(part, p, s.Case, net, faces[i])
			if err != nil || len(pr.LEDs) != len(res.LEDs.Positions) {
				continue
			}
			f := TLFanLEDs{Centre: Point{X: res.Dimensions.W / 2, Y: res.Dimensions.H / 2}}
			for k, q := range pr.LEDs {
				f.Canvas = append(f.Canvas, Point{X: ox + q.X*sx, Y: oy + q.Y*sy})
				lp := res.LEDs.Positions[k]
				f.Local = append(f.Local, Point{X: lp[0], Y: lp[1]})
			}
			pf.Fans = append(pf.Fans, f)
		}
		out = append(out, pf)
	}
	return out
}

func isTLFan(p Part) bool {
	return isFan(p) && len(p.ID) >= len(tlPartPrefix) && p.ID[:len(tlPartPrefix)] == tlPartPrefix
}

// TLDirections are the firmware directions and what each is assumed to do on
// a fan: the phase of every LED (template wire order) in the direction of
// travel. NOT VERIFIED ON HARDWARE: these are hypotheses. Known from the
// template only: LEDs 0-12 lie on one bar (at the bottom when the fan is not
// spun), 13-25 on the other bar (at the top).
//
//	clockwise          LED index order
//	counter-clockwise  reverse index order
//	up                 template y, bottom to top
//	down               template y, top to bottom
//	spread             distance from the fan centre, outwards
//	gather             distance from the fan centre, inwards
var TLDirections = []struct {
	Name  string
	Phase func(k int, local, centre Point) float64
}{
	{"clockwise", func(k int, _, _ Point) float64 { return float64(k) }},
	{"counter-clockwise", func(k int, _, _ Point) float64 { return -float64(k) }},
	{"up", func(_ int, q, _ Point) float64 { return -q.Y }},
	{"down", func(_ int, q, _ Point) float64 { return q.Y }},
	{"spread", func(_ int, q, c Point) float64 { return math.Hypot(q.X-c.X, q.Y-c.Y) }},
	{"gather", func(_ int, q, c Point) float64 { return -math.Hypot(q.X-c.X, q.Y-c.Y) }},
}

const (
	// TLMinCorr is the correlation a direction needs; below it the effect
	// does not run across the fans of the port in any firmware direction.
	TLMinCorr = 0.5
	// tlMinSpan is the smallest change of the path coordinate across a fan
	// (as a fraction of the whole path) that counts as running across it.
	tlMinSpan = 0.02
)

// TLDirection picks the firmware direction (TLDirections) whose LED phases
// correlate best (Pearson) with the effect's path coordinate u at the fans'
// LEDs. Each fan is taken relative to its own mean (the firmware runs the
// same pattern on every fan of a port), a circular u is unwrapped around the
// fan's centre first. whole is true when no direction reaches TLMinCorr or
// the effect barely changes across the fans: the fans then light as a whole.
func TLDirection(fans []TLFanLEDs, u func(x, y float64) float64, circular bool) (dir string, corr float64, whole bool) {
	var us []float64
	var span float64
	n := 0
	ph := make([][]float64, len(TLDirections))
	for _, f := range fans {
		if len(f.Canvas) < 2 {
			continue
		}
		fu := make([]float64, len(f.Canvas))
		for k, q := range f.Canvas {
			fu[k] = u(q.X, q.Y)
		}
		if circular {
			var cx, cy float64
			for _, q := range f.Canvas {
				cx, cy = cx+q.X, cy+q.Y
			}
			ref := u(cx/float64(len(f.Canvas)), cy/float64(len(f.Canvas)))
			for k := range fu {
				fu[k] -= math.Round(fu[k] - ref)
			}
		}
		mean, lo, hi := 0.0, math.Inf(1), math.Inf(-1)
		for _, v := range fu {
			mean += v
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		mean /= float64(len(fu))
		span += hi - lo
		n++
		for k := range fu {
			us = append(us, fu[k]-mean)
		}
		for d, h := range TLDirections {
			vals := make([]float64, len(fu))
			m := 0.0
			for k := range fu {
				vals[k] = h.Phase(k, f.Local[k], f.Centre)
				m += vals[k]
			}
			m /= float64(len(fu))
			for k := range vals {
				ph[d] = append(ph[d], vals[k]-m)
			}
		}
	}
	if n == 0 {
		return "", 0, true
	}
	best, bestCorr := -1, math.Inf(-1)
	for d := range TLDirections {
		if r := pearson(ph[d], us); r > bestCorr {
			best, bestCorr = d, r
		}
	}
	if best < 0 || bestCorr < TLMinCorr || span/float64(n) < tlMinSpan {
		return "", math.Max(bestCorr, 0), true
	}
	return TLDirections[best].Name, bestCorr, false
}

// pearson is the correlation of two series (0 when one is constant).
func pearson(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 2 {
		return 0
	}
	var ma, mb float64
	for i := range a {
		ma, mb = ma+a[i], mb+b[i]
	}
	ma, mb = ma/float64(len(a)), mb/float64(len(b))
	var sab, saa, sbb float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		sab, saa, sbb = sab+da*db, saa+da*da, sbb+db*db
	}
	if saa <= 1e-12 || sbb <= 1e-12 {
		return 0
	}
	return sab / math.Sqrt(saa*sbb)
}
