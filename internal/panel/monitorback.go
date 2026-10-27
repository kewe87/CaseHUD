package panel

import (
	"errors"

	"github.com/kewe87/casehud/internal/hid"
)

// Universal Screen 8.8" in monitor mode. A bar that was switched to act as a
// plain monitor shows up as a composite device (1A86:AD21) whose HID interface
// takes 64-byte reports. One output report with ID 0 and the ASCII payload
// "5f3759df" makes it restart as the normal display (1CBE:A088). This file is
// the way back only; nothing here switches a display into that mode.

// ErrNotInMonitorMode is returned when the device is gone or was never in monitor mode.
var ErrNotInMonitorMode = errors.New("display in monitor mode not found")

var monitorModeID = hid.ID{VendorID: 0x1A86, ProductID: 0xAD21}

// monitorBackPayload is the whole payload of the one report this file sends.
const monitorBackPayload = "5f3759df"

const monitorReportLen = 64 // payload bytes behind the report ID

// MonitorModeDevice is a display found in monitor mode.
type MonitorModeDevice struct {
	Path string `json:"path"`
}

// Seams for tests.
var (
	mmEnumerate = hid.Enumerate
	mmOpen      = hid.Open
)

// FindMonitorMode lists the displays that are in monitor mode (read only).
func FindMonitorMode() []MonitorModeDevice {
	infos, err := mmEnumerate(monitorModeID)
	if err != nil {
		return nil
	}
	var out []MonitorModeDevice
	for _, in := range infos {
		if in.VendorID == monitorModeID.VendorID && in.ProductID == monitorModeID.ProductID {
			out = append(out, MonitorModeDevice{Path: in.Path})
		}
	}
	return out
}

// BringBackFromMonitorMode sends the restart report to the device at path,
// which must be one FindMonitorMode lists right now. It writes exactly one
// report and nothing else.
func BringBackFromMonitorMode(path string) error {
	infos, err := mmEnumerate(monitorModeID)
	if err != nil {
		return err
	}
	for _, in := range infos {
		if in.Path != path || in.VendorID != monitorModeID.VendorID || in.ProductID != monitorModeID.ProductID {
			continue
		}
		dev, err := mmOpen(in)
		if err != nil {
			return err
		}
		defer dev.Close()
		report := make([]byte, 1+monitorReportLen) // report ID 0, rest zero
		copy(report[1:], monitorBackPayload)
		if in.OutputReportLen > 0 && in.OutputReportLen < len(report) {
			report = report[:in.OutputReportLen]
		}
		_, err = dev.Write(report)
		return err
	}
	return ErrNotInMonitorMode
}
