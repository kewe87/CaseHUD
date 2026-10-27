package winusb

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/kewe87/casehud/internal/rgb/internal/diag"
	"golang.org/x/sys/windows"
)

var (
	modWinUSB                       = windows.NewLazySystemDLL("winusb.dll")
	procWinUsbInitialize            = modWinUSB.NewProc("WinUsb_Initialize")
	procWinUsbFree                  = modWinUSB.NewProc("WinUsb_Free")
	procWinUsbQueryInterfaceSetting = modWinUSB.NewProc("WinUsb_QueryInterfaceSettings")
	procWinUsbQueryPipe             = modWinUSB.NewProc("WinUsb_QueryPipe")
	procWinUsbSetPipePolicy         = modWinUSB.NewProc("WinUsb_SetPipePolicy")
	procWinUsbWritePipe             = modWinUSB.NewProc("WinUsb_WritePipe")
)

const (
	pipeTypeBulk        = 2
	pipeTransferTimeout = 0x03
)

type interfaceDescriptor struct {
	Length            uint8
	DescriptorType    uint8
	InterfaceNumber   uint8
	AlternateSetting  uint8
	NumEndpoints      uint8
	InterfaceClass    uint8
	InterfaceSubClass uint8
	InterfaceProtocol uint8
	Interface         uint8
}

type pipeInformation struct {
	PipeType          int32
	PipeID            uint8
	MaximumPacketSize uint16
	Interval          uint8
}

// Paths lists the present device interfaces of class guid that belong to vid:pid.
func Paths(guid GUID, vid, pid uint16) ([]string, error) {
	g := windows.GUID(guid)
	all, err := windows.CM_Get_Device_Interface_List("", &g, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil, fmt.Errorf("winusb: list interfaces: %w", err)
	}
	var out []string
	for _, p := range all {
		if matchIDs(p, vid, pid) {
			out = append(out, p)
		}
	}
	return out, nil
}

// Device is an open WinUSB device with one bulk OUT pipe. WinUSB grants
// exclusive access: Open fails while another program holds the device.
type Device struct {
	mu   sync.Mutex
	h    windows.Handle
	ih   uintptr
	pipe uint8
	// MaxPacket is the packet size of the bulk OUT endpoint.
	MaxPacket int
}

// Open opens path and selects the first bulk OUT endpoint of interface 0.
func Open(path string, timeout time.Duration) (*Device, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("winusb: open: %w", err)
	}
	d := &Device{h: h}
	if r, _, err := procWinUsbInitialize.Call(uintptr(h), uintptr(unsafe.Pointer(&d.ih))); r == 0 {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("winusb: initialize: %w", err)
	}
	var desc interfaceDescriptor
	if r, _, err := procWinUsbQueryInterfaceSetting.Call(d.ih, 0, uintptr(unsafe.Pointer(&desc))); r == 0 {
		d.Close()
		return nil, fmt.Errorf("winusb: interface settings: %w", err)
	}
	found := false
	for i := uint8(0); i < desc.NumEndpoints; i++ {
		var pi pipeInformation
		if r, _, _ := procWinUsbQueryPipe.Call(d.ih, 0, uintptr(i), uintptr(unsafe.Pointer(&pi))); r == 0 {
			continue
		}
		if pi.PipeType == pipeTypeBulk && pi.PipeID&0x80 == 0 {
			d.pipe, d.MaxPacket, found = pi.PipeID, int(pi.MaximumPacketSize), true
			break
		}
	}
	if !found {
		d.Close()
		return nil, errors.New("winusb: no bulk OUT endpoint")
	}
	ms := uint32(timeout.Milliseconds())
	procWinUsbSetPipePolicy.Call(d.ih, uintptr(d.pipe), pipeTransferTimeout, unsafe.Sizeof(ms), uintptr(unsafe.Pointer(&ms)))
	return d, nil
}

// Write sends buf on the bulk OUT pipe (synchronously, with the pipe timeout).
func (d *Device) Write(buf []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ih == 0 {
		return windows.ERROR_INVALID_HANDLE
	}
	var n uint32
	src := fmt.Sprintf("winusb pipe %#02x", d.pipe)
	if r, _, err := procWinUsbWritePipe.Call(d.ih, uintptr(d.pipe), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&n)), 0); r == 0 {
		err = fmt.Errorf("winusb: write: %w", err)
		diag.Error(src, err)
		return err
	}
	diag.Packet(src, "out", buf)
	if int(n) != len(buf) {
		return fmt.Errorf("winusb: short write (%d of %d)", n, len(buf))
	}
	return nil
}

// Close releases the device.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ih != 0 {
		procWinUsbFree.Call(d.ih)
		d.ih = 0
	}
	if d.h != 0 {
		windows.CloseHandle(d.h)
		d.h = 0
	}
	return nil
}
