package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	// verifyService tells whether what answers on ServiceURL is the sensor
	// service of this machine (see serviceTrusted). Tests replace it.
	verifyService func() bool
	serviceURL    string // the endpoint verifyService speaks for (ServiceURL; tests replace it)
	displayOnly   bool   // the last snapshot was marked DisplayOnly (logged on change)
}

// NewLHM returns a provider for the given data.json URL ("" = auto).
func NewLHM(url string) *LHM {
	urls := []string{ServiceURL, DefaultLHMURL}
	if url != "" {
		urls = []string{url}
	}
	return &LHM{URLs: urls, verifyService: serviceTrusted, serviceURL: ServiceURL, client: &http.Client{
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
	Text     string `json:"Text"`
	Value    string `json:"Value"`
	SensorID string `json:"SensorId"`
	// AgeMs, on a hardware node of the CaseHUD sensor service: how long before the
	// snapshot was made its values were read from the device.
	AgeMs    *int64    `json:"AgeMs"`
	Children []lhmNode `json:"Children"`
}

// Refresh implements Provider. The endpoints are tried in order on every call,
// so the sensor service is used again as soon as it answers. Only its readings
// may drive the cooling (see Reading.DisplayOnly); a fallback endpoint is for the
// display.
func (l *LHM) Refresh() error {
	var firstErr error
	for idx := range l.URLs {
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
	var out []Reading
	if age, ok := snapshotAge(resp.Header); ok {
		out = flattenLHMAged(root, &age)
	} else {
		out = flattenLHM(root)
	}
	l.markTrust(url, out)
	return out, nil
}

// markTrust sets Reading.DisplayOnly on a snapshot unless it comes from the
// sensor service: any local process can listen on another port, and a source
// that cannot be told from the service must not move fans or the pump.
func (l *LHM) markTrust(url string, rs []Reading) {
	trusted := url == l.serviceURL && l.verifyService != nil && l.verifyService()
	for i := range rs {
		rs[i].DisplayOnly = !trusted
	}
	if !trusted != l.displayOnly {
		l.displayOnly = !trusted
		if l.displayOnly {
			log.Printf("sensors: %s is not the CaseHUD sensor service of this machine, its readings are for display only and do not drive the cooling", url)
		} else {
			log.Print("sensors: the readings come from the sensor service again")
		}
	}
}

// SnapshotAgeHeader is set by the CaseHUD sensor service on every answer: how
// many milliseconds ago it last finished reading the hardware. Other sources do
// not send it, and their readings have no known age.
const SnapshotAgeHeader = "X-Snapshot-Age-Ms"

// snapshotAge reads SnapshotAgeHeader.
func snapshotAge(h http.Header) (time.Duration, bool) {
	ms, err := strconv.ParseInt(strings.TrimSpace(h.Get(SnapshotAgeHeader)), 10, 64)
	if err != nil || ms < 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

// flattenLHM turns the sensor tree into readings. Paths skip the two top
// levels ("Sensor" and the computer name) so they start at the hardware.
func flattenLHM(root lhmNode) []Reading { return flattenLHMAged(root, nil) }

// flattenLHMAged is flattenLHM with the age of the snapshot, if the source told
// it. A reading is then as old as the snapshot plus the time its hardware was
// read before it.
func flattenLHMAged(root lhmNode, snapshot *time.Duration) []Reading {
	var out []Reading
	var walk func(n lhmNode, path []string, hw time.Duration)
	walk = func(n lhmNode, path []string, hw time.Duration) {
		if len(path) == 2 && n.AgeMs != nil && *n.AgeMs >= 0 { // a hardware node
			hw = time.Duration(*n.AgeMs) * time.Millisecond
		}
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
			r := Reading{Key: key, Path: p, Value: v, Unit: unit}
			if snapshot != nil {
				r.Age, r.AgeKnown = *snapshot+hw, true
			}
			out = append(out, r)
			return
		}
		next := append(append([]string{}, path...), n.Text)
		for _, c := range n.Children {
			walk(c, next, hw)
		}
	}
	walk(root, nil, 0)
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
