package leds

import (
	"image/color"
	"sync"
	"time"
)

// deviceWorker sends frames to one device on its own goroutine, so a slow
// device (SMBus bridge, acknowledged USB commands) holds up neither the
// engine tick nor the other devices. Only the newest frame is kept: a frame
// that arrives while the device is still busy replaces the waiting one.
type deviceWorker struct {
	dev Device

	mu      sync.Mutex
	pending map[string][]color.RGBA
	ops     []func()
	err     error

	wake chan struct{}
	quit chan struct{}
	done chan struct{}
}

func startWorker(dev Device) *deviceWorker {
	w := &deviceWorker{dev: dev, wake: make(chan struct{}, 1), quit: make(chan struct{}), done: make(chan struct{})}
	go w.run()
	return w
}

// submit queues a copy of frame, replacing a frame that was not sent yet.
func (w *deviceWorker) submit(frame map[string][]color.RGBA) {
	cp := make(map[string][]color.RGBA, len(frame))
	for id, cols := range frame {
		cp[id] = append([]color.RGBA(nil), cols...)
	}
	w.mu.Lock()
	w.pending = cp
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// call queues fn to run on the worker goroutine before the next frame, so
// device calls never overlap a SetFrame and never block the engine tick.
func (w *deviceWorker) call(fn func()) {
	w.mu.Lock()
	w.ops = append(w.ops, fn)
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// takeErr returns and clears the last SetFrame error.
func (w *deviceWorker) takeErr() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.err
	w.err = nil
	return err
}

func (w *deviceWorker) run() {
	defer close(w.done)
	for {
		select {
		case <-w.quit:
			return
		case <-w.wake:
		}
		w.mu.Lock()
		frame, ops := w.pending, w.ops
		w.pending, w.ops = nil, nil
		w.mu.Unlock()
		for _, fn := range ops {
			select {
			case <-w.quit:
				return // stopped: queued calls are dropped
			default:
			}
			fn()
		}
		if frame == nil {
			continue
		}
		if err := w.dev.SetFrame(frame); err != nil {
			w.mu.Lock()
			w.err = err
			w.mu.Unlock()
		}
	}
}

// stop ends the worker after the frame in flight; a waiting frame is dropped.
func (w *deviceWorker) stop() { w.stopWithin(stopWait) }

// stopWithin is stop with a bound; false if the frame in flight did not finish.
func (w *deviceWorker) stopWithin(d time.Duration) bool {
	select {
	case <-w.quit:
	default:
		close(w.quit)
	}
	select {
	case <-w.done:
		return true
	case <-time.After(d):
		return false
	}
}
