//go:build windows

package hid

// Windows backend over hid.dll. The devices use the in-box HID class driver,
// so no driver swap is needed and the device stays shareable at the OS level.

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modhid                    = windows.NewLazySystemDLL("hid.dll")
	procHidDGetAttributes     = modhid.NewProc("HidD_GetAttributes")
	procHidDGetPreparsedData  = modhid.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData = modhid.NewProc("HidD_FreePreparsedData")
	procHidPGetCaps           = modhid.NewProc("HidP_GetCaps")
	procHidDGetProductString  = modhid.NewProc("HidD_GetProductString")
	guidDevinterfaceHID       = windows.GUID{Data1: 0x4D1E55B2, Data2: 0xF16F, Data3: 0x11CF, Data4: [8]byte{0x88, 0xCB, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30}}
	errHidCall                = errors.New("hid.dll call failed")
)

const hidpStatusSuccess = 0x00110000

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
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

func createFile(path string, access uint32, flags uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, flags, 0)
}

// query reads VID/PID, usage page and report lengths without needing
// read/write access (works even for devices another program has open).
func query(path string) (Info, hidpCaps, error) {
	h, err := createFile(path, 0, 0)
	if err != nil {
		return Info{}, hidpCaps{}, err
	}
	defer windows.CloseHandle(h)

	attr := hiddAttributes{Size: uint32(unsafe.Sizeof(hiddAttributes{}))}
	if r, _, _ := procHidDGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&attr))); r == 0 {
		return Info{}, hidpCaps{}, errHidCall
	}
	var caps hidpCaps
	var pp uintptr
	if r, _, _ := procHidDGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&pp))); r != 0 {
		st, _, _ := procHidPGetCaps.Call(pp, uintptr(unsafe.Pointer(&caps)))
		procHidDFreePreparsedData.Call(pp)
		if st != hidpStatusSuccess {
			caps = hidpCaps{}
		}
	}
	var name [128]uint16
	procHidDGetProductString.Call(uintptr(h), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)*2))

	return Info{
		Path:            path,
		VendorID:        attr.VendorID,
		ProductID:       attr.ProductID,
		UsagePage:       caps.UsagePage,
		Product:         windows.UTF16ToString(name[:]),
		OutputReportLen: int(caps.OutputReportByteLength),
	}, caps, nil
}

func enumerate(ids []ID) ([]Info, error) {
	paths, err := windows.CM_Get_Device_Interface_List("", &guidDevinterfaceHID, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil, fmt.Errorf("listing HID devices: %w", err)
	}
	var out []Info
	for _, p := range paths {
		// Cheap pre-filter on the path ("\\?\hid#vid_04fc&pid_7393#...").
		lp := strings.ToLower(p)
		hit := false
		for _, id := range ids {
			if strings.Contains(lp, fmt.Sprintf("vid_%04x&pid_%04x", id.VendorID, id.ProductID)) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		info, _, err := query(p)
		if err != nil || !matches(ids, info.VendorID, info.ProductID) {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

type winDevice struct {
	h      windows.Handle
	outLen int
	inLen  int
}

func open(info Info) (Device, error) {
	_, caps, err := query(info.Path)
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", info.Path, err)
	}
	h, err := createFile(info.Path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_FLAG_OVERLAPPED)
	if err != nil {
		return nil, fmt.Errorf("opening %04x:%04x: %w", info.VendorID, info.ProductID, err)
	}
	return &winDevice{h: h, outLen: int(caps.OutputReportByteLength), inLen: int(caps.InputReportByteLength)}, nil
}

// overlapped runs one async ReadFile/WriteFile with a timeout.
func (d *winDevice) overlapped(timeoutMS int, start func(ov *windows.Overlapped, n *uint32) error) (int, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(ev)
	ov := windows.Overlapped{HEvent: ev}
	var n uint32
	if err := start(&ov, &n); err != nil && err != windows.ERROR_IO_PENDING {
		return 0, err
	}
	ev2, err := windows.WaitForSingleObject(ev, uint32(timeoutMS))
	if err != nil {
		return 0, err
	}
	if ev2 == uint32(windows.WAIT_TIMEOUT) {
		windows.CancelIoEx(d.h, &ov)
		windows.GetOverlappedResult(d.h, &ov, &n, true)
		return 0, ErrTimeout
	}
	if err := windows.GetOverlappedResult(d.h, &ov, &n, false); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (d *winDevice) Write(buf []byte) (int, error) {
	// hid.dll rejects writes that aren't exactly OutputReportByteLength.
	out := buf
	if d.outLen > 0 && len(buf) != d.outLen {
		if len(buf) > d.outLen {
			return 0, fmt.Errorf("hid: report of %d bytes exceeds device output length %d", len(buf), d.outLen)
		}
		out = make([]byte, d.outLen)
		copy(out, buf)
	}
	n, err := d.overlapped(1000, func(ov *windows.Overlapped, n *uint32) error {
		return windows.WriteFile(d.h, out, n, ov)
	})
	if n > len(buf) {
		n = len(buf)
	}
	return n, err
}

func (d *winDevice) Read(buf []byte, timeoutMS int) (int, error) {
	in := buf
	if d.inLen > len(buf) {
		in = make([]byte, d.inLen)
	}
	n, err := d.overlapped(timeoutMS, func(ov *windows.Overlapped, n *uint32) error {
		return windows.ReadFile(d.h, in, n, ov)
	})
	if err != nil {
		return 0, err
	}
	return copy(buf, in[:n]), nil
}

func (d *winDevice) Close() error { return windows.CloseHandle(d.h) }
