//go:build windows

// Package procjob ties child processes to the lifetime of CaseHUD: they are
// put into a Windows job object with KILL_ON_JOB_CLOSE, so Windows ends them
// when CaseHUD exits for any reason (crash, Task Manager, Stop-Process).
package procjob

import (
	"log"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	once sync.Once
	job  windows.Handle // kept open for the process lifetime
)

// newJob creates a kill-on-close job object.
func newJob() (windows.Handle, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(h)
		return 0, err
	}
	return h, nil
}

// assignTo puts the process with the given pid into j.
func assignTo(j windows.Handle, pid int) error {
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(ph)
	return windows.AssignProcessToJobObject(j, ph)
}

// Assign adds a freshly started child to CaseHUD's job object. Failures are
// only logged; the child keeps running either way.
func Assign(p *os.Process) {
	if p == nil {
		return
	}
	once.Do(func() {
		h, err := newJob()
		if err != nil {
			log.Printf("procjob: job object unavailable: %v", err)
			return
		}
		job = h
	})
	if job == 0 {
		return
	}
	if err := assignTo(job, p.Pid); err != nil {
		log.Printf("procjob: assign pid %d: %v", p.Pid, err)
	}
}
