package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

type flipFake struct {
	*layerFake
	fmu   sync.Mutex
	can   bool
	calls []bool
}

func (f *flipFake) CanDeviceFlip() bool { return f.can }
func (f *flipFake) SetDeviceFlip(on bool) error {
	f.fmu.Lock()
	defer f.fmu.Unlock()
	f.calls = append(f.calls, on)
	return nil
}
func (f *flipFake) got() []bool {
	f.fmu.Lock()
	defer f.fmu.Unlock()
	return append([]bool(nil), f.calls...)
}

func TestDeviceFlipSettingIsStoredAndApplied(t *testing.T) {
	f := &flipFake{layerFake: newLayerFake(), can: true}
	e := newLayerWorker(t, newLayerFake()).e
	// replace the worker's panel by the flip-capable fake
	e.mu.Lock()
	e.workers = map[string]*worker{f.ID(): {e: e, p: f, switchCh: make(chan switchReq, 1), wake: make(chan struct{}, 1)}}
	e.order = []string{f.ID()}
	e.cfg.Panels = map[string]PanelConfig{f.ID(): {Mode: "design", Design: "x"}}
	e.mu.Unlock()

	if ps := e.Panels(); ps[0].DeviceFlip == nil || *ps[0].DeviceFlip {
		t.Fatalf("default state %+v", ps[0].DeviceFlip)
	}
	if err := e.SetDeviceFlip(f.ID(), true); err != nil {
		t.Fatal(err)
	}
	if c := f.got(); len(c) != 1 || !c[0] {
		t.Fatalf("panel calls %v", c)
	}
	if ps := e.Panels(); !*ps[0].DeviceFlip {
		t.Fatal("not reported")
	}
	var onDisk struct {
		DeviceFlip map[string]bool `json:"device_flip"`
	}
	b, err := os.ReadFile(filepath.Join(e.Store.Dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &onDisk); err != nil || !onDisk.DeviceFlip[f.ID()] {
		t.Fatalf("config.json: %s (%v)", b, err)
	}
	// a panel opened later gets the stored mode before its first frame
	g := &flipFake{layerFake: newLayerFake(), can: true}
	e.applyDeviceFlip([]panel.Panel{g})
	if c := g.got(); len(c) != 1 || !c[0] {
		t.Fatalf("open: panel calls %v", c)
	}
	// off removes the entry and tells the panel
	if err := e.SetDeviceFlip(f.ID(), false); err != nil {
		t.Fatal(err)
	}
	if e.cfg.DeviceFlip[f.ID()] {
		t.Fatal("entry kept")
	}
	h := &flipFake{layerFake: newLayerFake(), can: true}
	e.applyDeviceFlip([]panel.Panel{h})
	if len(h.got()) != 0 {
		t.Fatal("switch off but the panel was told to flip")
	}
	// unknown panels and models without the rotation are refused
	if e.SetDeviceFlip("nope", true) == nil {
		t.Fatal("unknown panel accepted")
	}
	f.can = false
	if e.SetDeviceFlip(f.ID(), true) == nil {
		t.Fatal("model without device rotation accepted")
	}
	if ps := e.Panels(); ps[0].DeviceFlip != nil {
		t.Fatal("offered for a model without it")
	}
}
