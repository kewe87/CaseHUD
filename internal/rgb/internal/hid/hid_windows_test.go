package hid

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// pipeDevice is a Device on one end of a named pipe: reads stay pending until
// the other end writes, the way a HID read waits for the next report. No
// hardware is involved.
func pipeDevice(t *testing.T) (*Device, windows.Handle) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\pipe\casehud-hid-test-%d-%d`, os.Getpid(), time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	server, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT, 1, 4096, 4096, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		windows.CloseHandle(server)
		t.Fatal(err)
	}
	d := &Device{h: client}
	if d.wEvent, err = windows.CreateEvent(nil, 1, 0, nil); err == nil {
		d.rEvent, err = windows.CreateEvent(nil, 1, 0, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		d.Close()
		windows.CloseHandle(server)
	})
	return d, server
}

//go:noinline
func growStack(depth int) int {
	var pad [64]byte
	pad[depth%len(pad)] = 1
	if depth == 0 {
		return int(pad[0])
	}
	return growStack(depth-1) + int(pad[depth%len(pad)])
}

// A goroutine's stack moves when it grows or the collector shrinks it. A read
// that timed out has to end anyway, however often that happens under it.
func TestReadTimesOutWhileTheStackMoves(t *testing.T) {
	d, _ := pipeDevice(t)
	stop := make(chan struct{})
	var collector sync.WaitGroup
	collector.Add(1)
	go func() {
		defer collector.Done()
		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	defer func() {
		close(stop)
		collector.Wait()
	}()

	finished := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		for i := 0; i < 1500; i++ {
			growStack(4000) // the collector shrinks a stack that is mostly unused
			if _, err := d.Read(buf, 2*time.Millisecond); !errors.Is(err, ErrTimeout) {
				finished <- fmt.Errorf("read %d: %w", i, err)
				return
			}
		}
		finished <- nil
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("a read that timed out never came back")
	}
}

func TestReadReturnsTheReport(t *testing.T) {
	d, server := pipeDevice(t)
	var sent uint32
	if err := windows.WriteFile(server, []byte{1, 2, 3}, &sent, nil); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := d.Read(buf, time.Second)
	if err != nil || n != 3 || buf[0] != 1 || buf[2] != 3 {
		t.Fatalf("got %d bytes %v, err %v", n, buf[:n], err)
	}
}

// A driver that ignores the cancel must not take the caller down with it.
func TestRequestThatIgnoresItsCancelEnds(t *testing.T) {
	d, _ := pipeDevice(t)
	cancelIO = func(windows.Handle, *windows.Overlapped) error { return nil }
	t.Cleanup(func() { cancelIO = windows.CancelIoEx })

	buf := make([]byte, 64)
	start := time.Now()
	if _, err := d.Read(buf, time.Millisecond); !errors.Is(err, ErrStuck) {
		t.Fatalf("first read: %v, want ErrStuck", err)
	}
	if took := time.Since(start); took > cancelWait+2*time.Second {
		t.Errorf("took %v", took)
	}
	if _, err := d.Read(buf, time.Second); !errors.Is(err, ErrStuck) {
		t.Errorf("second read: %v, want ErrStuck", err)
	}
	if err := d.Write([]byte{1}, time.Second); !errors.Is(err, ErrStuck) {
		t.Errorf("write: %v, want ErrStuck", err)
	}
}
