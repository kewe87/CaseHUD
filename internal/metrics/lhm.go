package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sensor endpoints tried in order when no URL is configured: CaseHUD's own
// sensor service (LibreHardwareMonitorLib bundled, installed by the setup),
// then a running LibreHardwareMonitor with its remote web server enabled.
const (
	ServiceURL    = "http://127.0.0.1:8086/data.json"
	DefaultLHMURL = "http://localhost:8085/data.json"
)

// LHM reads sensors in LibreHardwareMonitor's data.json format.
type LHM struct {
	URLs     []string
	client   *http.Client
	readings []Reading
	active   int // index of the URL that answered last
}

// NewLHM returns a provider for the given data.json URL ("" = auto).
func NewLHM(url string) *LHM {
	urls := []string{ServiceURL, DefaultLHMURL}
	if url != "" {
		urls = []string{url}
	}
	return &LHM{URLs: urls, client: &http.Client{
		Timeout: 1500 * time.Millisecond,
		// A sensor endpoint answers itself; following redirects would let it
		// point the app at other addresses.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// Source is the endpoint currently delivering data.
func (l *LHM) Source() string { return l.URLs[l.active] }

// lhmNode is one node of LibreHardwareMonitor's sensor tree:
// Sensor -> <computer> -> <hardware> [-> <sub-hardware>] -> <group> -> <sensor>.
type lhmNode struct {
	Text     string    `json:"Text"`
	Value    string    `json:"Value"`
	SensorID string    `json:"SensorId"`
	Children []lhmNode `json:"Children"`
}

// Refresh implements Provider. It sticks with the endpoint that worked last
// and only probes the others when that one fails.
func (l *LHM) Refresh() error {
	var firstErr error
	for i := range l.URLs {
		idx := (l.active + i) % len(l.URLs)
		rs, err := l.fetch(l.URLs[idx])
		if err == nil {
			l.readings, l.active = rs, idx
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	l.readings = nil
	if len(l.URLs) > 1 {
		return fmt.Errorf("no sensor source reachable: install the CaseHUD sensor service (setup) or run LibreHardwareMonitor with its remote web server")
	}
	return firstErr
}

func (l *LHM) fetch(url string) ([]Reading, error) {
	resp, err := l.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("sensors not reachable at %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sensors at %s: HTTP %d", url, resp.StatusCode)
	}
	var root lhmNode
	// A local process squatting on the port must not be able to stream
	// unbounded data into memory.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&root); err != nil {
		return nil, fmt.Errorf("decoding sensor JSON from %s: %w", url, err)
	}
	return flattenLHM(root), nil
}

// flattenLHM turns the sensor tree into readings. Paths skip the two top
// levels ("Sensor" and the computer name) so they start at the hardware.
func flattenLHM(root lhmNode) []Reading {
	var out []Reading
	var walk func(n lhmNode, path []string)
	walk = func(n lhmNode, path []string) {
		if len(n.Children) == 0 {
			v, unit, ok := parseLHMValue(n.Value)
			if !ok || len(path) < 2 {
				return
			}
			p := strings.Join(append(path[2:], n.Text), "/")
			key := n.SensorID
			if key == "" {
				key = p
			}
			out = append(out, Reading{Key: key, Path: p, Value: v, Unit: unit})
			return
		}
		next := append(append([]string{}, path...), n.Text)
		for _, c := range n.Children {
			walk(c, next)
		}
	}
	walk(root, nil)
	return out
}

// parseLHMValue parses strings like "45.3 °C", "45,3 °C" (German locale),
// "1234 RPM" or "12.5 %" into a number and a unit.
func parseLHMValue(s string) (float64, string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, "", false
	}
	numPart, unit, _ := strings.Cut(s, " ")
	numPart = strings.ReplaceAll(numPart, ",", ".")
	v, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, "", false
	}
	return v, strings.TrimSpace(unit), true
}

// Get implements Provider.
func (l *LHM) Get(source string) (Reading, bool) { return match(l.readings, source) }

// All implements Provider.
func (l *LHM) All() []Reading { return l.readings }
