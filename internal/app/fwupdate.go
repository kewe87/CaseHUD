package app

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/panel"
	"github.com/kewe87/casehud/internal/rgb/fwcheck"
)

// Optional firmware check (off by default): asks the manufacturer's download
// page once per start which firmware is newest and compares it with what the
// devices reported. Nothing is downloaded and nothing goes to a device.

// fwFetch and fwDelay are replaced by tests.
var (
	fwFetch = fwcheck.Latest
	fwDelay = 20 * time.Second
)

// Product IDs of the devices the check knows (see fwcheck).
const (
	fwProdScreen88 = "1cbe:a088"
	fwProdTLLCD    = "04fc:7393"
	fwProdTLCtrl   = "0416:7372"
)

// Statuses of FirmwareNote.
const (
	FwUpdate  = "update"  // a newer version is offered
	FwCurrent = "current" // the device has the newest
	FwNewest  = "newest"  // newest known, no comparison possible
)

// FirmwareNote tells the GUI what the manufacturer offers for one device.
type FirmwareNote struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"` // newest version, if the file name has one
	Date    string `json:"date,omitempty"`    // build date of the newest file (YYYY-MM-DD)
	URL     string `json:"url"`               // page to open (the update is done with the manufacturer's software)
}

type fwState struct {
	mu      sync.Mutex
	running bool
	rel     map[string]fwcheck.Release
}

// FirmwareCheck reports whether the firmware check is switched on.
func (e *Engine) FirmwareCheck() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.FWUpdateCheck
}

// startFirmwareCheck runs the once-per-start check in the background after a
// short delay, if the setting is on. Never in the demo.
func (e *Engine) startFirmwareCheck() {
	if e.Demo || !e.FirmwareCheck() {
		return
	}
	ctx := e.ctx
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(fwDelay):
		}
		e.runFirmwareCheck(ctx)
	}()
}

// runFirmwareCheck fetches once (never two at the same time) and keeps the
// result in memory. Errors go to the log only.
func (e *Engine) runFirmwareCheck(ctx context.Context) {
	if e.Demo || !e.FirmwareCheck() {
		return
	}
	f := &e.fw
	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return
	}
	f.running = true
	f.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rels, err := fwFetch(cctx, nil)
	on := e.FirmwareCheck() // not under f.mu: e.mu is taken first elsewhere
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	if err != nil {
		log.Printf("firmware check: %v", err)
		return
	}
	if !on { // switched off while the request was running
		return
	}
	m := make(map[string]fwcheck.Release, len(rels))
	for _, r := range rels {
		m[r.Product] = r
	}
	f.rel = m
}

// firmwareSwitched is called with the new value after the setting changed:
// off forgets the result, a switch-on starts one check now.
func (e *Engine) firmwareSwitched(was, on bool) {
	if !on {
		e.fw.mu.Lock()
		e.fw.rel = nil
		e.fw.mu.Unlock()
		return
	}
	if !was && !e.Demo && e.ctx != nil {
		go e.runFirmwareCheck(e.ctx)
	}
}

// firmwareNote compares the installed text of a device with the newest
// release for the product. nil when there is nothing to show.
func (e *Engine) firmwareNote(product, installed string) *FirmwareNote {
	if product == "" {
		return nil
	}
	e.fw.mu.Lock()
	r, ok := e.fw.rel[product]
	e.fw.mu.Unlock()
	if !ok {
		return nil
	}
	return buildFirmwareNote(product, r, installed)
}

func buildFirmwareNote(product string, r fwcheck.Release, installed string) *FirmwareNote {
	n := &FirmwareNote{Version: panel.CleanFirmware(r.Version), Date: panel.CleanFirmware(r.Date), URL: fwcheck.PageURL}
	if n.Version == "" {
		if n.Date == "" {
			return nil
		}
		n.Status = FwNewest
		return n
	}
	cur := installedVersion(product, installed)
	switch {
	case cur == "":
		n.Status = FwNewest
	case fwcheck.Newer(n.Version, cur):
		n.Status = FwUpdate
	default:
		n.Status = FwCurrent
	}
	return n
}

var (
	reFw88   = regexp.MustCompile(`lianli88_\d{4}_(\d{4})`)
	reFwDots = regexp.MustCompile(`^[Vv]?(\d+(?:\.\d+)*)$`)
)

// installedVersion turns the text a device reported into the version form
// fwcheck uses ("" if it cannot be read).
func installedVersion(product, s string) string {
	s = strings.TrimSpace(s)
	switch product {
	case fwProdScreen88:
		if m := reFw88.FindStringSubmatch(s); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n < 18 {
				return ""
			}
			return strconv.Itoa((n-8)/10) + "." + strconv.Itoa((n-8)%10)
		}
	case fwProdTLCtrl:
	default:
		return ""
	}
	if m := reFwDots.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// panelFirmwareProduct maps a panel to a product the check knows ("" if none).
func panelFirmwareProduct(p panel.Panel) string {
	d, ok := p.(interface{ DriverID() string })
	if !ok {
		return ""
	}
	switch d.DriverID() {
	case "us88":
		return fwProdScreen88
	case "tllcd":
		return fwProdTLLCD
	}
	return ""
}
