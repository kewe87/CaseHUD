// Package proc reports running processes by executable name.
package proc

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Running returns those of names (executable names, case-insensitive) that
// currently run.
func Running(names []string) []string {
	want := make(map[string]string, len(names))
	for _, n := range names {
		want[strings.ToLower(n)] = n
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var out []string
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		exe := strings.ToLower(windows.UTF16ToString(pe.ExeFile[:]))
		if n, ok := want[exe]; ok {
			out = append(out, n)
			delete(want, exe)
		}
	}
	return out
}
