package metrics

import "net/url"

// Multi merges several providers into one sensor list. The first provider is
// the primary one (LibreHardwareMonitor): only its errors are reported, since
// optional sources such as RTSS are often simply not running.
type Multi struct {
	Providers []Provider
	all       []Reading
}

// NewDefault returns the standard provider: LibreHardwareMonitor, plus FPS
// from PresentMon (preferred: frame generation and latency) and RivaTuner
// Statistics Server as a fallback.
func NewDefault(lhmURL string) *Multi {
	return New(Options{SensorURLs: urlsFor("", lhmURL), PresentMon: true, RTSS: true})
}

// Options selects which sources a Multi provider uses.
type Options struct {
	SensorURLs []string // data.json endpoints, tried in order
	PresentMon bool
	RTSS       bool
	// Extra providers are appended after the standard ones.
	Extra []Provider
}

// New builds a provider from options.
func New(o Options) *Multi {
	lhm := NewLHM("")
	if len(o.SensorURLs) > 0 {
		lhm.URLs = o.SensorURLs
	}
	ps := []Provider{lhm}
	if o.PresentMon {
		ps = append(ps, NewPresentMon(""))
	}
	if o.RTSS {
		ps = append(ps, NewRTSS())
	}
	ps = append(ps, o.Extra...)
	return &Multi{Providers: ps}
}

// SensorURLsFor maps a source choice ("auto", "service", "lhm", "custom")
// to the endpoints to query.
func SensorURLsFor(source, customURL string) []string { return urlsFor(source, customURL) }

func urlsFor(source, custom string) []string {
	switch source {
	case "service":
		return []string{ServiceURL}
	case "lhm":
		return []string{DefaultLHMURL}
	case "custom":
		if validSensorURL(custom) {
			return []string{custom}
		}
	}
	if validSensorURL(custom) && source == "" {
		return []string{custom} // legacy lhm_url setting
	}
	return []string{ServiceURL, DefaultLHMURL}
}

// PresentMonStatus reports the PresentMon state if it is part of m.
func (m *Multi) PresentMonStatus() string {
	for _, p := range m.Providers {
		if pm, ok := p.(*PresentMon); ok {
			return pm.Status()
		}
	}
	return "unsupported"
}

// SetGameIgnore tells PresentMon which executables are not games.
func (m *Multi) SetGameIgnore(names []string) {
	for _, p := range m.Providers {
		if pm, ok := p.(*PresentMon); ok {
			pm.SetIgnore(names)
		}
	}
}

// GameName is the name of the game PresentMon measures ("" if unknown).
func (m *Multi) GameName() string {
	for _, p := range m.Providers {
		if pm, ok := p.(*PresentMon); ok {
			return pm.GameName()
		}
	}
	return ""
}

// Refresh implements Provider.
func (m *Multi) Refresh() error {
	var first error
	m.all = m.all[:0]
	for i, p := range m.Providers {
		if err := p.Refresh(); err != nil && i == 0 {
			first = err
		}
		m.all = append(m.all, p.All()...)
	}
	return first
}

// Get implements Provider.
func (m *Multi) Get(source string) (Reading, bool) { return match(m.all, source) }

// All implements Provider.
func (m *Multi) All() []Reading { return m.all }

// Source is the sensor endpoint of the primary provider.
func (m *Multi) Source() string {
	if l, ok := m.Providers[0].(*LHM); ok {
		return l.Source()
	}
	return ""
}

// validSensorURL accepts only http(s) addresses with a host; anything else
// falls back to the default endpoints.
func validSensorURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
