package app

import (
	"image"
	"sync"
	"testing"
	"time"
)

// countPanel counts the settings writes that reach the device.
type countPanel struct {
	demoPanel
	mu         sync.Mutex
	brightness []int
	rotation   []int
}

func (c *countPanel) SetBrightness(v int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.brightness = append(c.brightness, v)
	return nil
}
func (c *countPanel) SetRotation(v int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rotation = append(c.rotation, v)
	return nil
}
func (c *countPanel) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.brightness), len(c.rotation)
}

func TestDeviceWritesCoalesceSliderChanges(t *testing.T) {
	old := deviceWriteGap
	deviceWriteGap = 150 * time.Millisecond
	defer func() { deviceWriteGap = old }()

	p := &countPanel{demoPanel: demoPanel{id: "fan-0", name: "fan", size: image.Pt(400, 400)}}
	w := &worker{p: p}
	// First apply (start-up) goes out at once.
	w.applyDevice(PanelConfig{Brightness: 50})
	if b, _ := p.counts(); b != 1 {
		t.Fatalf("start-up brightness writes = %d, want 1", b)
	}
	// A slider drag: 40 changes in quick succession.
	for i := 1; i <= 40; i++ {
		w.applyDevice(PanelConfig{Brightness: 50 + i%30, Rotation: 90})
	}
	w.applyDevice(PanelConfig{Brightness: 77, Rotation: 90})
	time.Sleep(400 * time.Millisecond)
	b, r := p.counts()
	if b != 2 || r != 1 {
		t.Fatalf("after the drag: brightness writes %d (want 2), rotation writes %d (want 1)", b, r)
	}
	if p.brightness[1] != 77 || p.rotation[0] != 90 {
		t.Fatalf("last value must win: %v %v", p.brightness, p.rotation)
	}
	// The same values again (a restart of the worker, a rescan) write nothing.
	for i := 0; i < 5; i++ {
		w.applyDevice(PanelConfig{Brightness: 77, Rotation: 90})
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(250 * time.Millisecond)
	if b2, r2 := p.counts(); b2 != b || r2 != r {
		t.Fatalf("identical values were written again: %d %d", b2, r2)
	}
	// Pending writes die with the worker.
	w.applyDevice(PanelConfig{Brightness: 11, Rotation: 90}) // goes out (gap passed)
	w.applyDevice(PanelConfig{Brightness: 10, Rotation: 90}) // waits
	w.dw.close()
	time.Sleep(250 * time.Millisecond)
	if b2, _ := p.counts(); b2 != b+1 {
		t.Fatalf("write after close: %d", b2)
	}
}
