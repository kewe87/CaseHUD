package app

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/rgb/fwcheck"
)

func fakeFetch(t *testing.T, rels []fwcheck.Release, err error) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	oldF, oldD := fwFetch, fwDelay
	fwFetch = func(context.Context, *http.Client) ([]fwcheck.Release, error) {
		n.Add(1)
		return rels, err
	}
	fwDelay = time.Millisecond
	t.Cleanup(func() { fwFetch, fwDelay = oldF, oldD })
	return &n
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

var testRels = []fwcheck.Release{
	{Product: fwProdTLCtrl, Version: "0.62"},
	{Product: fwProdScreen88, Version: "1.5"},
	{Product: fwProdTLLCD, Date: "2025-03-04"},
}

func TestFirmwareCheckDefaultOffNoFetch(t *testing.T) {
	n := fakeFetch(t, testRels, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := &Engine{ctx: ctx}
	e.startFirmwareCheck()
	e.runFirmwareCheck(ctx)
	time.Sleep(50 * time.Millisecond)
	if n.Load() != 0 || e.firmwareNote(fwProdTLCtrl, "0.50") != nil {
		t.Fatalf("fetched %d times with the setting off", n.Load())
	}
}

func TestFirmwareCheckOncePerStart(t *testing.T) {
	n := fakeFetch(t, testRels, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := &Engine{ctx: ctx}
	e.cfg.FWUpdateCheck = true
	e.startFirmwareCheck()
	waitFor(t, func() bool { return e.firmwareNote(fwProdTLCtrl, "0.50") != nil })
	time.Sleep(30 * time.Millisecond)
	if n.Load() != 1 {
		t.Fatalf("fetches: %d", n.Load())
	}
	// switching off forgets, switching on again fetches once more
	e.mu.Lock()
	e.cfg.FWUpdateCheck = false
	e.mu.Unlock()
	e.firmwareSwitched(true, false)
	if e.firmwareNote(fwProdTLCtrl, "0.50") != nil {
		t.Fatal("result kept after switching off")
	}
	e.mu.Lock()
	e.cfg.FWUpdateCheck = true
	e.mu.Unlock()
	e.firmwareSwitched(false, true)
	waitFor(t, func() bool { return n.Load() == 2 })
}

func TestFirmwareCheckDemoAndError(t *testing.T) {
	n := fakeFetch(t, nil, errors.New("offline"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &Engine{ctx: ctx, Demo: true}
	d.cfg.FWUpdateCheck = true
	d.startFirmwareCheck()
	d.firmwareSwitched(false, true)
	time.Sleep(30 * time.Millisecond)
	if n.Load() != 0 {
		t.Fatal("demo fetched")
	}
	e := &Engine{ctx: ctx}
	e.cfg.FWUpdateCheck = true
	e.runFirmwareCheck(ctx)
	if n.Load() != 1 || e.firmwareNote(fwProdTLCtrl, "0.50") != nil {
		t.Fatal("error must leave no result")
	}
}

func TestFirmwareNoteCompare(t *testing.T) {
	e := &Engine{}
	e.fw.rel = map[string]fwcheck.Release{}
	for _, r := range testRels {
		e.fw.rel[r.Product] = r
	}
	cases := []struct {
		prod, inst, status string
	}{
		{fwProdTLCtrl, "0.50", FwUpdate},
		{fwProdTLCtrl, "0.62", FwCurrent},
		{fwProdTLCtrl, "0.70", FwCurrent},
		{fwProdTLCtrl, "", FwNewest},
		{fwProdTLCtrl, "garbage", FwNewest},
		{fwProdScreen88, "lianli88_0001_0053", FwCurrent}, // 4.5 is newer than 1.5
		{fwProdScreen88, "lianli88_0001_0018", FwUpdate},  // 1.0 < 1.5
		{fwProdScreen88, "lianli88_0001_0023", FwCurrent}, // 1.5
		{fwProdScreen88, "V1.2.3", FwUpdate},              // plain dotted form is compared as it is
		{fwProdScreen88, "dev-build", FwNewest},
		{fwProdScreen88, "", FwNewest},
		{fwProdTLLCD, "V1.0 (2024-01-01)", FwNewest},
	}
	for _, c := range cases {
		n := e.firmwareNote(c.prod, c.inst)
		if n == nil || n.Status != c.status {
			t.Errorf("%s %q: %+v, want %s", c.prod, c.inst, n, c.status)
		}
	}
}

func TestFirmwareNoteDateOnly(t *testing.T) {
	e := &Engine{}
	e.fw.rel = map[string]fwcheck.Release{fwProdTLLCD: testRels[2]}
	n := e.firmwareNote(fwProdTLLCD, "whatever")
	if n == nil || n.Status != FwNewest || n.Version != "" || n.Date != "2025-03-04" || n.URL != fwcheck.PageURL {
		t.Fatalf("%+v", n)
	}
	if e.firmwareNote("", "x") != nil || e.firmwareNote(fwProdScreen88, "x") != nil {
		t.Fatal("unknown product or missing release must give nil")
	}
}

func TestInstalled88(t *testing.T) {
	for in, want := range map[string]string{
		"lianli88_0001_0028": "2.0", "lianli88_0001_0023": "1.5", "lianli88_0001_0017": "", "": "",
	} {
		if got := installedVersion(fwProdScreen88, in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
