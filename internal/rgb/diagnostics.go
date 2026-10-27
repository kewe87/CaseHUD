package leds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/rgb/internal/conflicts"
	"github.com/kewe87/casehud/internal/rgb/internal/diag"
	"github.com/kewe87/casehud/internal/rgb/internal/hid"
	"github.com/kewe87/casehud/internal/rgb/internal/proc"
)

// Report is a diagnostic snapshot a user can export and send in. It contains
// no serial numbers; device paths are shortened to a hash.
type Report struct {
	Generated time.Time      `json:"generated"`
	Library   string         `json:"library"` // module version / VCS revision
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Engine    bool           `json:"engine"` // taken from a running engine
	Config    *Config        `json:"config,omitempty"`
	Devices   []DeviceReport `json:"devices"`
	Errors    []string       `json:"errors,omitempty"`
	Running   []string       `json:"otherRgbSoftware,omitempty"` // known conflicting programs that run
	HID       []HIDEntry     `json:"hidInventory"`
	Tracing   bool           `json:"tracing"`
	Log       []diag.Entry   `json:"log"`
	Packets   []diag.Entry   `json:"packets,omitempty"`
}

// DeviceReport is one device with its backend-specific raw details.
type DeviceReport struct {
	DeviceStatus
	Details string `json:"details,omitempty"`
}

// HIDEntry describes a HID collection that could be lighting hardware: one
// with a vendor-defined usage page or from a vendor known for RGB devices.
type HIDEntry struct {
	VID        string `json:"vid"`
	PID        string `json:"pid"`
	Interface  int    `json:"interface"`
	UsagePage  string `json:"usagePage"`
	Usage      string `json:"usage"`
	InputLen   int    `json:"inputLen"`
	OutputLen  int    `json:"outputLen"`
	FeatureLen int    `json:"featureLen"`
	Product    string `json:"product,omitempty"`
	PathHash   string `json:"pathHash"` // tells collections apart without exposing the path
}

// rgbVendors are USB vendor IDs of common lighting hardware (for the inventory).
var rgbVendors = map[uint16]bool{
	0x0B05: true, 0x1532: true, 0x0416: true, 0x04FC: true, 0x1CBE: true, 0x1A86: true,
	0x1B1C: true, 0x1038: true, 0x046D: true, 0x0951: true, 0x2516: true, 0x3633: true,
	0x1044: true, 0x048D: true, 0x26CE: true, 0x0CF2: true, 0x1462: true, 0x320F: true,
}

// SetTracing switches the packet record on or off (see Report.Packets).
func SetTracing(on bool) { diag.SetTracing(on) }

// Diagnostics returns a report of the running engine.
func (e *Engine) Diagnostics() Report {
	r := newReport()
	r.Engine = true
	cfg := e.Config()
	r.Config = &cfg
	e.mu.Lock()
	for _, d := range e.devices {
		r.Devices = append(r.Devices, DeviceReport{DeviceStatus: e.statusLocked(d), Details: describe(d.dev)})
	}
	e.mu.Unlock()
	sortDevices(r.Devices)
	r.collectRecords()
	return r
}

// collectRecords adds the log and packet record (last step, so it covers
// what the report itself triggered).
func (r *Report) collectRecords() {
	r.Tracing, r.Log, r.Packets = diag.Tracing(), diag.Logs(), diag.Packets()
}

// Inspect builds a report without an engine: it runs discovery (which does
// not change any lighting), records what it finds and closes the devices.
func Inspect(ctx context.Context, backends []Backend) Report {
	r := newReport()
	for _, b := range backends {
		devs, err := b.Discover(ctx)
		if err != nil {
			r.Errors = append(r.Errors, b.Name()+": "+err.Error())
		}
		for _, d := range devs {
			r.Devices = append(r.Devices, DeviceReport{
				DeviceStatus: DeviceStatus{ID: d.ID(), Name: d.Name(), Backend: b.Name(), State: StateOK, Zones: d.Zones()},
				Details:      describe(d),
			})
			d.Close()
		}
	}
	sortDevices(r.Devices)
	r.collectRecords()
	return r
}

func newReport() Report {
	r := Report{Generated: time.Now(), OS: osVersion(), Arch: runtime.GOARCH}
	if bi, ok := debug.ReadBuildInfo(); ok {
		r.Library = bi.Main.Version
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				r.Library += " " + s.Value
			}
		}
		for _, m := range bi.Deps {
			if m.Path == "github.com/kewe87/casehud/internal/rgb" {
				r.Library = m.Version
			}
		}
	}
	var names []string
	for _, l := range [][]string{conflicts.Mainboard, conflicts.RAM, conflicts.Razer, conflicts.Ring, conflicts.TL, conflicts.AIO, conflicts.Hubs} {
		names = append(names, l...)
	}
	r.Running = dedupe(proc.Running(dedupe(names)))
	r.HID = hidInventory()
	return r
}

func hidInventory() []HIDEntry {
	infos, err := hid.Enumerate(0, 0)
	if err != nil && !errors.Is(err, hid.ErrUnsupported) {
		return nil
	}
	var out []HIDEntry
	for _, i := range infos {
		if i.UsagePage < 0xFF00 && !rgbVendors[i.VendorID] {
			continue
		}
		sum := sha256.Sum256([]byte(strings.ToLower(i.Path)))
		out = append(out, HIDEntry{
			VID: hex4(i.VendorID), PID: hex4(i.ProductID), Interface: i.Interface,
			UsagePage: hex4(i.UsagePage), Usage: hex4(i.Usage),
			InputLen: i.InputLen, OutputLen: i.OutputLen, FeatureLen: i.FeatureLen,
			Product: i.Product, PathHash: hex.EncodeToString(sum[:4]),
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].VID+out[a].PID != out[b].VID+out[b].PID {
			return out[a].VID+out[a].PID < out[b].VID+out[b].PID
		}
		return out[a].PathHash < out[b].PathHash
	})
	return out
}

func describe(d Device) string {
	if v, ok := d.(interface{ Describe() string }); ok {
		return v.Describe()
	}
	return ""
}

func sortDevices(ds []DeviceReport) {
	sort.Slice(ds, func(a, b int) bool { return ds[a].ID < ds[b].ID })
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if k := strings.ToLower(s); !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12&0xF], digits[v>>8&0xF], digits[v>>4&0xF], digits[v&0xF]})
}
