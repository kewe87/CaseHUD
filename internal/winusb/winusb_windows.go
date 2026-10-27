//go:build windows

package winusb

// Windows backend over winusb.dll. Devices are found through the generic USB
// device interface, which every USB device exposes regardless of its driver,
// so no per-device interface GUID is needed.

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modwinusb                 = windows.NewLazySystemDLL("winusb.dll")
	procInitialize            = modwinusb.NewProc("WinUsb_Initialize")
	procFree                  = modwinusb.NewProc("WinUsb_Free")
	procWritePipe             = modwinusb.NewProc("WinUsb_WritePipe")
	procReadPipe              = modwinusb.NewProc("WinUsb_ReadPipe")
	procSetPipePolicy         = modwinusb.NewProc("WinUsb_SetPipePolicy")
	procResetPipe             = modwinusb.NewProc("WinUsb_ResetPipe")
	guidDevinterfaceUSB       = windows.GUID{Data1: 0xA5DCBF10, Data2: 0x6530, Data3: 0x11D2, Data4: [8]byte{0x90, 0x1F, 0x00, 0xC0, 0x4F, 0xB9, 0x51, 0xED}}
	pipeShortPacketTerminate  = uintptr(1)
	pipeTransferTimeoutPolicy = uintptr(3)
)

func enumerate(ids []ID) ([]Info, error) {
	paths, err := windows.CM_Get_Device_Interface_List("", &guidDevinterfaceUSB, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil, fmt.Errorf("listing USB devices: %w", err)
	}
	var out []Info
	for _, p := range paths {
		// Path looks like "\?\usb#vid_1cbe&pid_a088#0828c67e96928304#{guid}".
		parts := strings.Split(strings.ToLower(p), "#")
		if len(parts) < 3 {
			continue
		}
		for _, id := range ids {
			if parts[1] == fmt.Sprintf("vid_%04x&pid_%04x", id.VendorID, id.ProductID) {
				out = append(out, Info{Path: p, VendorID: id.VendorID, ProductID: id.ProductID, Serial: parts[2], Location: locationOf(p)})
				break
			}
		}
	}
	return out, nil
}

type winDevice struct {
	file windows.Handle
	h    uintptr
}

func open(info Info) (Device, error) {
	p, err := windows.UTF16PtrFromString(info.Path)
	if err != nil {
		return nil, err
	}
	f, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("opening %04x:%04x: %w", info.VendorID, info.ProductID, err)
	}
	var h uintptr
	if r, _, e := procInitialize.Call(uintptr(f), uintptr(unsafe.Pointer(&h))); r == 0 {
		windows.CloseHandle(f)
		return nil, fmt.Errorf("WinUsb_Initialize %04x:%04x: %w", info.VendorID, info.ProductID, e)
	}
	return &winDevice{file: f, h: h}, nil
}

func (d *winDevice) setTimeout(ep byte, ms int) {
	t := uint32(ms)
	procSetPipePolicy.Call(d.h, uintptr(ep), pipeTransferTimeoutPolicy, 4, uintptr(unsafe.Pointer(&t)))
}

func (d *winDevice) Write(ep byte, buf []byte, timeoutMS int) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	d.setTimeout(ep, timeoutMS)
	var n uint32
	r, _, e := procWritePipe.Call(d.h, uintptr(ep), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), 0)
	if r == 0 {
		if e == windows.ERROR_SEM_TIMEOUT || e == windows.ERROR_TIMEOUT {
			return int(n), ErrTimeout
		}
		return int(n), e
	}
	return int(n), nil
}

func (d *winDevice) Read(ep byte, buf []byte, timeoutMS int) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	d.setTimeout(ep, timeoutMS)
	start := time.Now()
	var n uint32
	r, _, e := procReadPipe.Call(d.h, uintptr(ep), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), 0)
	if r == 0 {
		if e == windows.ERROR_SEM_TIMEOUT || e == windows.ERROR_TIMEOUT {
			return 0, ErrTimeout
		}
		return 0, e
	}
	// An empty result that came back early is a zero-length packet from the
	// device, not a timeout.
	if n == 0 && time.Since(start) >= time.Duration(timeoutMS)*time.Millisecond*9/10 {
		return 0, ErrTimeout
	}
	return int(n), nil
}

func (d *winDevice) ZeroLengthTerminate(ep byte) error {
	v := byte(1)
	if r, _, e := procSetPipePolicy.Call(d.h, uintptr(ep), pipeShortPacketTerminate, 1, uintptr(unsafe.Pointer(&v))); r == 0 {
		return e
	}
	return nil
}

func (d *winDevice) ClearHalt(ep byte) error {
	if r, _, e := procResetPipe.Call(d.h, uintptr(ep)); r == 0 {
		return e
	}
	return nil
}

func (d *winDevice) Close() error {
	procFree.Call(d.h)
	return windows.CloseHandle(d.file)
}

var (
	modcfgmgr                = windows.NewLazySystemDLL("cfgmgr32.dll")
	procGetInterfaceProperty = modcfgmgr.NewProc("CM_Get_Device_Interface_PropertyW")
	procLocateDevNode        = modcfgmgr.NewProc("CM_Locate_DevNodeW")
	procGetDevNodeProperty   = modcfgmgr.NewProc("CM_Get_DevNode_PropertyW")
	devpkeyInstanceID        = windows.DEVPROPKEY{FmtID: windows.DEVPROPGUID{Data1: 0x78c34fc8, Data2: 0x104a, Data3: 0x4aca, Data4: [8]byte{0x9e, 0xa4, 0x52, 0x4d, 0x52, 0x99, 0x6e, 0x57}}, PID: 256}
	devpkeyLocationPaths     = windows.DEVPROPKEY{FmtID: windows.DEVPROPGUID{Data1: 0xa45c254e, Data2: 0xdf1c, Data3: 0x4efd, Data4: [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}}, PID: 37}
)

// locationOf returns the first LocationPaths entry of the device behind a
// device interface path (read-only property lookups; "" on any failure).
func locationOf(ifacePath string) string {
	p, err := windows.UTF16PtrFromString(ifacePath)
	if err != nil {
		return ""
	}
	var typ uint32
	buf := make([]byte, 1024)
	size := uint32(len(buf))
	if r, _, _ := procGetInterfaceProperty.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&devpkeyInstanceID)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0); r != 0 {
		return ""
	}
	inst := utf16Strings(buf[:min(int(size), len(buf))])
	if len(inst) == 0 {
		return ""
	}
	id, err := windows.UTF16PtrFromString(inst[0])
	if err != nil {
		return ""
	}
	var dn uint32
	if r, _, _ := procLocateDevNode.Call(uintptr(unsafe.Pointer(&dn)), uintptr(unsafe.Pointer(id)), 0); r != 0 {
		return ""
	}
	size = uint32(len(buf))
	if r, _, _ := procGetDevNodeProperty.Call(uintptr(dn), uintptr(unsafe.Pointer(&devpkeyLocationPaths)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0); r != 0 {
		return ""
	}
	if paths := utf16Strings(buf[:min(int(size), len(buf))]); len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// utf16Strings splits a buffer of NUL-separated UTF-16LE strings.
func utf16Strings(b []byte) []string {
	var u []uint16
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	var out []string
	start := 0
	for i, c := range u {
		if c == 0 {
			if i > start {
				out = append(out, windows.UTF16ToString(u[start:i]))
			}
			start = i + 1
		}
	}
	return out
}
