//go:build !windows

package coolguard

import (
	"context"
	"os"
	"syscall"
	"time"
)

// UseNamespace is only needed on Windows.
func UseNamespace(string) {}

func spawnAttr(bool) *syscall.SysProcAttr { return nil }

// WatchParent returns a channel that is closed when the process pid has ended.
func WatchParent(pid int) (<-chan struct{}, error) {
	gone := make(chan struct{})
	p, err := os.FindProcess(pid)
	if err != nil {
		close(gone)
		return gone, nil
	}
	go func() {
		for p.Signal(syscall.Signal(0)) == nil {
			time.Sleep(500 * time.Millisecond)
		}
		close(gone)
	}()
	return gone, nil
}

// AcquireSingle is not needed where the guard is not used.
func AcquireSingle(time.Duration) bool { return true }

// BeginAct does nothing here.
func BeginAct() (release func()) { return func() {} }

// WaitIdle does nothing here.
func WaitIdle(context.Context, time.Duration) {}
