package app

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Concurrent requests must neither race on the config maps nor leave extra
// worker loops behind.
func TestConcurrentConfigChanges(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	ps := e.Panels()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				p := ps[(g+i)%len(ps)]
				c := p.Config
				c.Brightness = 10 + i
				if err := e.SetPanelConfig(p.ID, c); err != nil {
					t.Error(err)
				}
				if i%5 == 0 {
					e.ApplySettings(e.Settings())
				}
			}
		}(g)
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	if loops := strings.Count(string(buf[:n]), "(*worker).run("); loops != len(ps) {
		t.Errorf("%d worker loops running for %d panels", loops, len(ps))
	}
	e.Stop()
	if _, err := e.Store.LoadConfig(); err != nil {
		t.Fatalf("config unreadable after concurrent saves: %v", err)
	}
}

// Parallel changes to one panel must end with the worker running the same
// config that is stored in the engine (and therefore in the file).
func TestParallelSetPanelConfigKeepsWorkerInSync(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	p := e.Panels()[0]
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				c := p.Config
				c.Brightness = 10 + g
				if err := e.SetPanelConfig(p.ID, c); err != nil {
					t.Error(err)
				}
			}(g)
		}
		wg.Wait()
		e.mu.Lock()
		want := e.cfg.Panels[p.ID].Brightness
		w := e.workers[p.ID]
		e.mu.Unlock()
		deadline := time.Now().Add(3 * time.Second)
		for w.current().Brightness != want && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if got := w.current().Brightness; got != want {
			t.Fatalf("round %d: worker runs brightness %d, config has %d", round, got, want)
		}
		saved, err := e.Store.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if got := saved.Panels[p.ID].Brightness; got != want {
			t.Fatalf("round %d: file has brightness %d, config has %d", round, got, want)
		}
	}
}
