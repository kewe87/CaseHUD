// Package conflicts finds programs that drive the same devices as CaseHUD and
// closes them when the user asks. Only programs in the catalog below are ever
// looked at, and only an explicit request from the user closes one.
package conflicts

import (
	"sort"
	"strings"
)

// Category groups the catalog in the settings.
const (
	CatDisplay  = "display"
	CatLighting = "lighting"
)

// Device keys: what a program can block. The GUI turns them into names.
const (
	DevTLLCD    = "tllcd"    // fan LCDs
	DevUS88     = "us88"     // 8.8" screen
	DevAIO      = "aio"      // pump head LCD
	DevFanLED   = "fanled"   // fan controller lighting
	DevRingLED  = "ringled"  // 8.8" screen frame
	DevBoardLED = "boardled" // mainboard lighting
	DevRAMLED   = "ramled"   // memory lighting
)

// LightingDevices are the device keys that only matter while lighting is on.
var LightingDevices = []string{DevFanLED, DevRingLED, DevBoardLED, DevRAMLED}

// Entry is one program of the catalog (data only).
type Entry struct {
	// Exe is the image name, matched case-insensitively and exactly.
	Exe  string `json:"exe"`
	Name string `json:"name"`
	// Category is CatDisplay or CatLighting.
	Category string `json:"category"`
	// Devices lists what the program can block.
	Devices []string `json:"devices"`
	// Service marks a background service: it cannot be closed from the user
	// session and is shown with a hint instead.
	Service bool `json:"service,omitempty"`
}

var (
	displayAll = []string{DevTLLCD, DevUS88, DevAIO, DevFanLED, DevRingLED}
	lightAll   = []string{DevFanLED, DevRingLED, DevBoardLED, DevRAMLED}
	boardRAM   = []string{DevBoardLED, DevRAMLED}
)

// catalog is the complete list. Nothing outside it is touched.
var catalog = []Entry{
	{Exe: "L-Connect 3.exe", Name: "L-Connect 3", Category: CatDisplay, Devices: displayAll},
	{Exe: "L-Connect.exe", Name: "L-Connect", Category: CatDisplay, Devices: displayAll},
	{Exe: "L-Connect-Service.exe", Name: "L-Connect (Dienst)", Category: CatDisplay, Devices: displayAll, Service: true},
	{Exe: "SignalRgb.exe", Name: "SignalRGB", Category: CatLighting, Devices: lightAll},
	{Exe: "OpenRGB.exe", Name: "OpenRGB", Category: CatLighting, Devices: lightAll},
	{Exe: "ArmouryCrate.exe", Name: "Armoury Crate", Category: CatLighting, Devices: boardRAM},
	{Exe: "LightingService.exe", Name: "Armoury Crate (Beleuchtungsdienst)", Category: CatLighting, Devices: boardRAM, Service: true},
}

// Catalog returns a copy of the list, sorted by name.
func Catalog() []Entry {
	out := make([]Entry, len(catalog))
	for i, e := range catalog {
		e.Devices = append([]string(nil), e.Devices...)
		out[i] = e
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Lookup finds a catalog entry by image name (case-insensitive, exact).
func Lookup(exe string) (Entry, bool) {
	for _, e := range catalog {
		if sameName(e.Exe, exe) {
			return e, true
		}
	}
	return Entry{}, false
}

func isListed(exe string) bool {
	_, ok := Lookup(exe)
	return ok
}

// sameName compares image names case-insensitively with ASCII folding only,
// so look-alike characters never match.
func sameName(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
