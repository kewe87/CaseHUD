//go:build windows

package sysperm

import (
	"fmt"
	"os/user"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// AddToPerformanceLogUsers adds the current user to the built-in
// "Performance Log Users" group (SID S-1-5-32-559, named
// "Leistungsprotokollbenutzer" on German Windows), which lets PresentMon
// trace frame events without admin rights. Windows shows one UAC prompt;
// the change applies after the next sign-in.
func AddToPerformanceLogUsers() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	name := strings.ReplaceAll(u.Username, "'", "''")
	script := fmt.Sprintf("Add-LocalGroupMember -SID 'S-1-5-32-559' -Member '%s' -ErrorAction SilentlyContinue", name)
	verb, _ := windows.UTF16PtrFromString("runas")
	sysDir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	file, _ := windows.UTF16PtrFromString(sysDir + `\WindowsPowerShell\v1.0\powershell.exe`)
	args, _ := windows.UTF16PtrFromString("-NoProfile -WindowStyle Hidden -Command \"" + script + "\"")
	const swHide = 0
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(args)), 0, swHide)
	if r <= 32 {
		return fmt.Errorf("setup was cancelled or failed (code %d)", r)
	}
	return nil
}
