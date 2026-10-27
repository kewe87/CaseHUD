package panel

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/hid"
)

func fakeMonitorMode(t *testing.T, infos []hid.Info) *fakeHID {
	t.Helper()
	f := &fakeHID{}
	oldE, oldO := mmEnumerate, mmOpen
	t.Cleanup(func() { mmEnumerate, mmOpen = oldE, oldO })
	mmEnumerate = func(ids ...hid.ID) ([]hid.Info, error) {
		if len(ids) != 1 || ids[0] != (hid.ID{VendorID: 0x1A86, ProductID: 0xAD21}) {
			t.Errorf("enumerate asked for %v", ids)
		}
		return infos, nil
	}
	mmOpen = func(in hid.Info) (hid.Device, error) { return f, nil }
	return f
}

func TestFindMonitorMode(t *testing.T) {
	fakeMonitorMode(t, []hid.Info{{Path: "a", VendorID: 0x1A86, ProductID: 0xAD21}, {Path: "b", VendorID: 1, ProductID: 2}})
	got := FindMonitorMode()
	if len(got) != 1 || got[0].Path != "a" {
		t.Fatalf("found %+v", got)
	}
}

// The way back sends exactly one report: ID 0, "5f3759df", zeros to 64 bytes.
// This is the only code path that talks to a 1A86:AD21 device, so the test
// doubles as the guard that nothing else is ever written to it.
func TestBringBackSendsOnlyTheRestartReport(t *testing.T) {
	f := fakeMonitorMode(t, []hid.Info{{Path: "a", VendorID: 0x1A86, ProductID: 0xAD21, OutputReportLen: 65}})
	if err := BringBackFromMonitorMode("a"); err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0, 0x35, 0x66, 0x33, 0x37, 0x35, 0x39, 0x64, 0x66}, make([]byte, 56)...)
	if len(f.writes) != 1 || !bytes.Equal(f.writes[0], want) {
		t.Fatalf("writes = % x", f.writes)
	}
}

func TestBringBackRefusesOtherDevices(t *testing.T) {
	f := fakeMonitorMode(t, []hid.Info{{Path: "a", VendorID: 0x1A86, ProductID: 0xAD21}, {Path: "tl", VendorID: 0x04FC, ProductID: 0x7393}})
	for _, p := range []string{"tl", "zzz", ""} {
		if err := BringBackFromMonitorMode(p); err == nil {
			t.Errorf("%q accepted", p)
		}
	}
	if len(f.writes) != 0 {
		t.Fatalf("wrote to the wrong device: %d", len(f.writes))
	}
}

// Only monitorback.go may name the monitor-mode device, so no other code can
// write to it (and nothing here knows the way into that mode).
func TestMonitorModeDeviceOnlyInItsFile(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if f == "monitorback.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("monitorModeID")) || bytes.Contains(b, []byte("0xAD21")) {
			t.Errorf("%s refers to the monitor-mode device", f)
		}
	}
}
