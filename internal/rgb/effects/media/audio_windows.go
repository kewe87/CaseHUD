package media

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WASAPI loopback capture of the default output device (what you hear).

var (
	modOle32             = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")

	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioClient         = windows.GUID{Data1: 0x1CB9AD4C, Data2: 0xDBFA, Data3: 0x4C32, Data4: [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidIAudioCaptureClient  = windows.GUID{Data1: 0xC8ADBD64, Data2: 0xE71E, Data3: 0x48A0, Data4: [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
	subtypeIEEEFloat        = windows.GUID{Data1: 0x00000003, Data2: 0x0000, Data3: 0x0010, Data4: [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
)

const (
	clsctxAll         = 0x17
	eRender           = 0
	eConsole          = 0
	streamLoopback    = 0x00020000
	bufferSilent      = 0x2
	formatIEEEFloat   = 3
	formatExtensible  = 0xFFFE
	bufferDuration    = 2000000 // 200 ms in 100 ns units
	pollInterval      = 10 * time.Millisecond
	reconnectInterval = 2 * time.Second

	// vtable indices
	vtRelease                  = 2
	vtGetDefaultAudioEndpoint  = 4
	vtActivate                 = 3
	vtInitialize               = 3
	vtGetMixFormat             = 8
	vtStart                    = 10
	vtStop                     = 11
	vtGetService               = 14
	vtGetBuffer                = 3
	vtReleaseBuffer            = 4
	vtGetNextPacketSize        = 5
	subFormatOffset            = 24 // WAVEFORMATEXTENSIBLE.SubFormat
	waveFormatExtensibleLength = 22 // cbSize of WAVEFORMATEXTENSIBLE
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

func platformAudio() error { return nil }

type comObject struct{ vtbl *[16]uintptr }

func (o *comObject) call(method int, args ...uintptr) error {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	if int32(r) < 0 {
		return fmt.Errorf("media: audio: HRESULT %#08x", uint32(r))
	}
	return nil
}

func (o *comObject) release() {
	if o != nil {
		_ = o.call(vtRelease)
	}
}

type loopback struct {
	stop chan struct{}
	done chan struct{}

	stopOnce sync.Once
}

// startTimeout bounds the wait for the first capture session: a device that
// hangs must not block the caller (Close or the next Render) for good.
const startTimeout = 5 * time.Second

func openLoopback(push func([]float32, int)) (interface{ Close() error }, error) {
	l := &loopback{stop: make(chan struct{}), done: make(chan struct{})}
	ready := make(chan error, 1)
	go l.run(push, ready)
	select {
	case err := <-ready:
		if err != nil {
			return nil, err
		}
		return l, nil
	case <-time.After(startTimeout):
		close(l.stop) // the goroutine ends as soon as the driver call returns
		return nil, errors.New("media: audio: the output device did not answer")
	}
}

func (l *loopback) Close() error {
	l.stopOnce.Do(func() { close(l.stop) })
	select {
	case <-l.done:
	case <-time.After(3 * time.Second):
		// a driver call hangs: do not hold the caller (the engine) with it
	}
	return nil
}

// run keeps a capture session open, reconnecting when the output device
// changes. The first session's result is reported on ready.
func (l *loopback) run(push func([]float32, int), ready chan<- error) {
	defer close(l.done)
	defer func() {
		if r := recover(); r != nil {
			log.Print("media: audio capture stopped after an internal error") // no data in the message
			select {
			case ready <- errors.New("media: audio: internal error"):
			default:
			}
		}
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// S_FALSE (1): COM was already initialized on this thread, still balanced by CoUninitialize
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(1)) {
		ready <- err
		return
	}
	defer windows.CoUninitialize()

	first := true
	var warned time.Time
	for {
		err := l.session(push, func() {
			if first {
				first = false
				ready <- nil
			}
		})
		if first {
			ready <- err
			return
		}
		if err != nil && time.Since(warned) > time.Minute {
			warned = time.Now()
			log.Printf("media: audio capture interrupted, retrying: %v", err)
		}
		select {
		case <-l.stop:
			return
		case <-time.After(reconnectInterval):
		}
	}
}

// comOuts holds the out-parameters of the COM calls. The calls take them as
// uintptr, which the compiler does not follow: they live on the heap (a
// goroutine stack can move) and are kept alive across the calls.
type comOuts struct {
	enum, dev, client, capture *comObject
	wfx                        *waveFormatEx
	data                       *byte
	packet, frames, flags      uint32
}

// newComOuts returns the structure from a function that is not inlined, so
// it is allocated on the heap.
//
//go:noinline
func newComOuts() *comOuts { return new(comOuts) }

func (l *loopback) session(push func([]float32, int), started func()) error {
	o := newComOuts()
	defer runtime.KeepAlive(o)
	defer func() { o.capture.release(); o.client.release(); o.dev.release(); o.enum.release() }()

	if r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&o.enum))); int32(r) < 0 {
		return fmt.Errorf("media: audio: no device enumerator (%#08x)", uint32(r))
	}
	if err := o.enum.call(vtGetDefaultAudioEndpoint, eRender, eConsole, uintptr(unsafe.Pointer(&o.dev))); err != nil {
		return fmt.Errorf("media: audio: no output device: %w", err)
	}
	if err := o.dev.call(vtActivate, uintptr(unsafe.Pointer(&iidIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&o.client))); err != nil {
		return err
	}
	if err := o.client.call(vtGetMixFormat, uintptr(unsafe.Pointer(&o.wfx))); err != nil {
		return err
	}
	wfx := o.wfx
	defer windows.CoTaskMemFree(unsafe.Pointer(wfx))
	isFloat := wfx.FormatTag == formatIEEEFloat
	if wfx.FormatTag == formatExtensible && wfx.CbSize >= waveFormatExtensibleLength {
		isFloat = *(*windows.GUID)(unsafe.Add(unsafe.Pointer(wfx), subFormatOffset)) == subtypeIEEEFloat
	}
	if !(isFloat && wfx.BitsPerSample == 32) && wfx.BitsPerSample != 16 {
		return fmt.Errorf("media: audio: unsupported mix format (%d bit)", wfx.BitsPerSample)
	}
	rate, channels, block := int(wfx.SamplesPerSec), int(wfx.Channels), int(wfx.BlockAlign)
	bytesPerSample := int(wfx.BitsPerSample) / 8
	if channels < 1 || channels > 32 || rate < 8000 || rate > 384000 || block < channels*bytesPerSample {
		return fmt.Errorf("media: audio: unsupported mix format (%d channels, %d Hz, block %d)", channels, rate, block)
	}
	if err := o.client.call(vtInitialize, 0, streamLoopback, bufferDuration, 0, uintptr(unsafe.Pointer(wfx)), 0); err != nil {
		return err
	}
	if err := o.client.call(vtGetService, uintptr(unsafe.Pointer(&iidIAudioCaptureClient)), uintptr(unsafe.Pointer(&o.capture))); err != nil {
		return err
	}
	if err := o.client.call(vtStart); err != nil {
		return err
	}
	defer func() { _ = o.client.call(vtStop) }()
	started()

	silence := make([]float32, rate*int(pollInterval)/int(time.Second))
	var mono []float32
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		got := false
		for {
			if err := o.capture.call(vtGetNextPacketSize, uintptr(unsafe.Pointer(&o.packet))); err != nil {
				return err
			}
			if o.packet == 0 {
				break
			}
			if err := o.capture.call(vtGetBuffer, uintptr(unsafe.Pointer(&o.data)), uintptr(unsafe.Pointer(&o.frames)),
				uintptr(unsafe.Pointer(&o.flags)), 0, 0); err != nil {
				return err
			}
			frames, flags := int(o.frames), o.flags
			mono = mono[:0]
			if frames > 0 && frames <= rate { // at most one second per packet
				raw := unsafe.Slice(o.data, frames*block)
				for f := 0; f < frames; f++ {
					var sum float32
					if flags&bufferSilent == 0 {
						for ch := 0; ch < channels; ch++ {
							off := f*block + ch*bytesPerSample
							if isFloat {
								sum += *(*float32)(unsafe.Pointer(&raw[off]))
							} else {
								sum += float32(int16(uint16(raw[off])|uint16(raw[off+1])<<8)) / 32768
							}
						}
					}
					mono = append(mono, sum/float32(channels))
				}
			}
			if err := o.capture.call(vtReleaseBuffer, uintptr(o.frames)); err != nil {
				return err
			}
			push(mono, rate)
			got = true
		}
		if !got {
			push(silence, rate) // nothing plays: loopback delivers no packets at all
		}
		select {
		case <-l.stop:
			return errors.New("stopped")
		case <-t.C:
		}
	}
}
