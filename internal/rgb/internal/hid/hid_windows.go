package hid

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/kewe87/casehud/internal/rgb/internal/diag"
	"golang.org/x/sys/windows"
)

var (
	modHid                    = windows.NewLazySystemDLL("hid.dll")
	procHidDGetHidGuid        = modHid.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes     = modHid.NewProc("HidD_GetAttributes")
	procHidDGetPreparsedData  = modHid.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData = modHid.NewProc("HidD_FreePreparsedData")
	procHidDGetProductString  = modHid.NewProc("HidD_GetProductString")
	procHidPGetCaps           = modHid.NewProc("HidP_GetCaps")
	procHidDSetFeature        = modHid.NewProc("HidD_SetFeature")
	procHidDGetFeature        = modHid.NewProc("HidD_GetFeature")
	procHidDGetInputReport    = modHid.NewProc("HidD_GetInputReport")
)

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
	_             uint16
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

const hidpStatusSuccess = 0x00110000

// Enumerate lists HID collections; vid/pid 0 match everything.
func Enumerate(vid, pid uint16) ([]DeviceInfo, error) {
	var guid windows.GUID
	procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&guid)))
	paths, err := windows.CM_Get_Device_Interface_List("", &guid, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil, fmt.Errorf("hid: list interfaces: %w", err)
	}
	var out []DeviceInfo
	for _, path := range paths {
		info, ok := query(path)
		if !ok || (vid != 0 && info.VendorID != vid) || (pid != 0 && info.ProductID != pid) {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

func query(path string) (DeviceInfo, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return DeviceInfo{}, false
	}
	// Access 0 is enough for attributes and works even for keyboards/mice.
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return DeviceInfo{}, false
	}
	defer windows.CloseHandle(h)

	attr := hiddAttributes{Size: uint32(unsafe.Sizeof(hiddAttributes{}))}
	if r, _, _ := procHidDGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&attr))); r == 0 {
		return DeviceInfo{}, false
	}
	info := DeviceInfo{Path: path, VendorID: attr.VendorID, ProductID: attr.ProductID, Interface: interfaceFromPath(path)}

	var pp uintptr
	if r, _, _ := procHidDGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&pp))); r != 0 {
		var caps hidpCaps
		if r, _, _ := procHidPGetCaps.Call(pp, uintptr(unsafe.Pointer(&caps))); r == hidpStatusSuccess {
			info.UsagePage, info.Usage = caps.UsagePage, caps.Usage
			info.InputLen = int(caps.InputReportByteLength)
			info.OutputLen = int(caps.OutputReportByteLength)
			info.FeatureLen = int(caps.FeatureReportByteLength)
		}
		procHidDFreePreparsedData.Call(pp)
	}
	var buf [128]uint16
	if r, _, _ := procHidDGetProductString.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2)); r != 0 {
		info.Product = windows.UTF16ToString(buf[:])
	}
	return info, true
}

// Device is an open HID collection. Handles are opened shared, so other
// programs keep access (conflicts are detected one level up).
type Device struct {
	Info DeviceInfo

	mu     sync.Mutex
	h      windows.Handle
	wEvent windows.Handle
	rEvent windows.Handle
}

// Open opens a collection for reading and writing. Collections Windows owns
// exclusively (mice, keyboards) are opened without data access; feature
// reports still work on them.
func Open(info DeviceInfo) (*Device, error) {
	p, err := windows.UTF16PtrFromString(info.Path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err == windows.ERROR_ACCESS_DENIED {
		h, err = windows.CreateFile(p, 0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("hid: open %s: %w", info.Path, err)
	}
	d := &Device{Info: info, h: h}
	if d.wEvent, err = windows.CreateEvent(nil, 1, 0, nil); err == nil {
		d.rEvent, err = windows.CreateEvent(nil, 1, 0, nil)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the handle.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, h := range []*windows.Handle{&d.h, &d.wEvent, &d.rEvent} {
		if *h != 0 {
			windows.CloseHandle(*h)
			*h = 0
		}
	}
	return nil
}

// Write sends one output report (first byte = report ID). It is zero-padded to
// the report length of the collection.
func (d *Device) Write(report []byte, timeout time.Duration) error {
	n := d.Info.OutputLen
	if n == 0 {
		n = len(report)
	}
	if len(report) > n {
		return fmt.Errorf("hid: report of %d bytes exceeds output length %d", len(report), n)
	}
	buf := make([]byte, n)
	copy(buf, report)
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.io(true, buf, timeout)
	d.trace("out", buf, err)
	return err
}

func (d *Device) trace(dir string, data []byte, err error) {
	if !diag.Tracing() {
		return
	}
	src := fmt.Sprintf("hid %04x:%04x/%d", d.Info.VendorID, d.Info.ProductID, d.Info.Interface)
	if err != nil {
		if err != ErrTimeout || dir == "out" { // read timeouts are normal polling
			diag.Error(src, fmt.Errorf("%s: %w", dir, err))
		}
		return
	}
	diag.Packet(src, dir, data)
}

// Read reads one input report into buf (which should be InputLen long) and
// returns the number of bytes read.
func (d *Device) Read(buf []byte, timeout time.Duration) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, err := d.ioRead(buf, timeout)
	d.trace("in", buf[:max(n, 0)], err)
	return n, err
}

func (d *Device) io(write bool, buf []byte, timeout time.Duration) error {
	if d.h == 0 {
		return windows.ERROR_INVALID_HANDLE
	}
	ov := windows.Overlapped{HEvent: d.wEvent}
	windows.ResetEvent(d.wEvent)
	var done uint32
	err := windows.WriteFile(d.h, buf, &done, &ov)
	return d.finish(err, &ov, &done, timeout)
}

func (d *Device) ioRead(buf []byte, timeout time.Duration) (int, error) {
	if d.h == 0 {
		return 0, windows.ERROR_INVALID_HANDLE
	}
	ov := windows.Overlapped{HEvent: d.rEvent}
	windows.ResetEvent(d.rEvent)
	var done uint32
	err := windows.ReadFile(d.h, buf, &done, &ov)
	err = d.finish(err, &ov, &done, timeout)
	return int(done), err
}

func (d *Device) finish(err error, ov *windows.Overlapped, done *uint32, timeout time.Duration) error {
	if err != nil && err != windows.ERROR_IO_PENDING {
		return err
	}
	ev, _ := windows.WaitForSingleObject(ov.HEvent, uint32(timeout.Milliseconds()))
	if ev != windows.WAIT_OBJECT_0 {
		windows.CancelIoEx(d.h, ov)
		windows.GetOverlappedResult(d.h, ov, done, true)
		return ErrTimeout
	}
	return windows.GetOverlappedResult(d.h, ov, done, false)
}

// SetFeature sends a feature report (first byte = report ID).
func (d *Device) SetFeature(report []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.h == 0 {
		return windows.ERROR_INVALID_HANDLE
	}
	if r, _, err := procHidDSetFeature.Call(uintptr(d.h), uintptr(unsafe.Pointer(&report[0])), uintptr(len(report))); r == 0 {
		err = fmt.Errorf("hid: set feature: %w", err)
		d.trace("out", nil, err)
		return err
	}
	d.trace("out", report, nil)
	return nil
}

// GetFeature reads a feature report into buf; buf[0] must hold the report ID.
func (d *Device) GetFeature(buf []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.h == 0 {
		return windows.ERROR_INVALID_HANDLE
	}
	if r, _, err := procHidDGetFeature.Call(uintptr(d.h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		err = fmt.Errorf("hid: get feature: %w", err)
		d.trace("in", nil, err)
		return err
	}
	d.trace("in", buf, nil)
	return nil
}

// GetInput requests an input report on the control pipe; buf[0] must hold
// the report ID.
func (d *Device) GetInput(buf []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.h == 0 {
		return windows.ERROR_INVALID_HANDLE
	}
	if r, _, err := procHidDGetInputReport.Call(uintptr(d.h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		err = fmt.Errorf("hid: get input report: %w", err)
		d.trace("in", nil, err)
		return err
	}
	d.trace("in", buf, nil)
	return nil
}
