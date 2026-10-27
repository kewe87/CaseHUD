//go:build windows

package conflicts

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procPostMessage = user32.NewProc("PostMessageW")
	procGetWindow   = user32.NewProc("GetWindow")
)

const (
	wmClose = 0x0010
	gwOwner = 4
)

// NewSystem is the real process access of this machine.
func NewSystem() System { return winSys{} }

type winSys struct{}

func filetime(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// identity reads path and start time of an open process handle.
func identity(h windows.Handle) (path string, created uint64, err error) {
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if err = windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", 0, err
	}
	var c, x, k, u windows.Filetime
	if err = windows.GetProcessTimes(h, &c, &x, &k, &u); err != nil {
		return "", 0, err
	}
	return windows.UTF16ToString(buf[:n]), filetime(c), nil
}

func (winSys) Processes(want func(exe string) bool) ([]Proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var out []Proc
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		exe := windows.UTF16ToString(pe.ExeFile[:])
		if !want(exe) {
			continue
		}
		p := Proc{PID: pe.ProcessID, Exe: exe}
		var sess uint32
		if windows.ProcessIdToSessionId(p.PID, &sess) == nil && sess == 0 {
			p.Service = true
		}
		if h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.PID); e == nil {
			p.Path, p.Created, _ = identity(h)
			windows.CloseHandle(h)
		} else {
			p.Restricted = true
		}
		// May this session end it? Probing with the right it needs.
		if h, e := windows.OpenProcess(windows.PROCESS_TERMINATE, false, p.PID); e == nil {
			windows.CloseHandle(h)
		} else {
			p.Restricted = true
		}
		out = append(out, p)
	}
	return out, nil
}

// open opens p for action and checks that it is still the same process.
func open(p Proc, access uint32) (windows.Handle, error) {
	if p.PID == 0 || p.Created == 0 {
		return 0, errors.New("process not identified")
	}
	h, err := windows.OpenProcess(access|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.PID)
	if err != nil {
		return 0, err
	}
	path, created, err := identity(h)
	if err != nil || created != p.Created || !strings.EqualFold(filepath.Base(path), p.Exe) ||
		(p.Path != "" && !strings.EqualFold(path, p.Path)) {
		windows.CloseHandle(h)
		return 0, errors.New("the process changed")
	}
	return h, nil
}

func (winSys) Terminate(p Proc) error {
	h, err := open(p, windows.PROCESS_TERMINATE)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

// Window enumeration needs a callback; the runtime only allows a limited
// number of them, so there is one for the whole process.
var (
	enumMu    sync.Mutex
	enumPID   uint32
	enumHwnds []windows.HWND
	enumCB    = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var pid uint32
		windows.GetWindowThreadProcessId(hwnd, &pid)
		if pid != enumPID {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(uintptr(hwnd), gwOwner); owner != 0 {
			return 1 // dialogs follow their owner
		}
		var cls [64]uint16
		n, _ := windows.GetClassName(hwnd, &cls[0], int32(len(cls)))
		if c := windows.UTF16ToString(cls[:n]); c == "IME" || c == "MSCTFIME UI" {
			return 1
		}
		enumHwnds = append(enumHwnds, hwnd)
		return 1
	})
)

func (winSys) PostClose(p Proc) (int, error) {
	h, err := open(p, 0)
	if err != nil {
		return 0, err
	}
	// The open handle keeps the PID from being handed to another process
	// until the requests are sent.
	defer windows.CloseHandle(h)
	enumMu.Lock()
	defer enumMu.Unlock()
	enumPID, enumHwnds = p.PID, nil
	if err := windows.EnumWindows(enumCB, nil); err != nil {
		return 0, err
	}
	n := 0
	for _, w := range enumHwnds {
		if r, _, _ := procPostMessage.Call(uintptr(w), wmClose, 0, 0); r != 0 {
			n++
		}
	}
	enumHwnds = nil
	return n, nil
}

// OpenServices opens the services list (fixed program and argument).
func OpenServices() error {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	return exec.Command(filepath.Join(dir, "mmc.exe"), filepath.Join(dir, "services.msc")).Start()
}
