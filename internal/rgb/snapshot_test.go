package leds_test

import (
	"context"
	"image/color"
	"sync"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/rgb/backend/fake"
)

func TestSnapshotMatchesLastFrameAndIsACopy(t *testing.T) {
	dev := newDev()
	cfg := leds.DefaultConfig()
	cfg.FPS = 100
	cfg.Effect = "solid"
	cfg.Params = map[string]leds.Params{"solid": {"color": "#c86432"}}
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})
	if len(e.Snapshot()) != 0 {
		t.Fatal("snapshot before start is not empty")
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	waitFrames(t, dev, 3)
	snap := e.Snapshot()
	zones := snap["fake:1"]
	if len(zones["strip"]) != 10 || len(zones["grb"]) != 2 {
		t.Fatalf("zones: %v", zones)
	}
	if last := dev.Last(); last["grb"][0] != zones["grb"][0] && last["grb"][0] != e.Snapshot()["fake:1"]["grb"][0] {
		t.Fatalf("snapshot %v differs from the last frame %v", zones["grb"][0], last["grb"][0])
	}
	// The snapshot belongs to the caller: changing it does not touch the engine.
	zones["strip"][0] = color.RGBA{1, 2, 3, 4}
	time.Sleep(30 * time.Millisecond)
	if got := e.Snapshot()["fake:1"]["strip"][0]; got == (color.RGBA{1, 2, 3, 4}) {
		t.Fatal("snapshot shares memory with the engine")
	}
}

func TestSnapshotRace(t *testing.T) {
	dev := newDev()
	cfg := leds.DefaultConfig()
	cfg.FPS = 200
	cfg.Effect = "solid"
	e := leds.New(cfg, leds.Options{Backends: []leds.Backend{&fake.Backend{Devices: []*fake.Device{dev}}}, ProcessRunning: noProcs})
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, zs := range e.Snapshot() {
					for _, cols := range zs {
						for j := range cols {
							cols[j].R++ // writing the copy must never race with the engine
						}
					}
				}
				if i == 0 {
					e.SetBrightness(50 + i)
				}
			}
		}(i)
	}
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	waitFrames(t, dev, 5)
}
