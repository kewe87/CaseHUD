package enram

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// openPipe opens the pipe for reading and writing. The server may only
// identify this client, not impersonate it (SQOS identification level).
func openPipe(name string) (*os.File, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(n, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(h), name), nil
}

// verifyServer checks the server of the opened pipe. The decisive check works
// from a standard user: the pipe object must be owned by LocalSystem or the
// Administrators group (a normal user cannot create such a pipe). When the
// serving process can be queried, its image also has to be the sensor service
// of this installation; if it cannot be queried (SYSTEM processes are closed to
// a normal user), the owner check alone decides.
func verifyServer(f *os.File) error {
	h := windows.Handle(f.Fd())
	sd, err := windows.GetSecurityInfo(h, windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return ErrUntrusted
	}
	owner, _, err := sd.Owner()
	if err != nil || !trustedPipeOwner(owner) {
		return ErrUntrusted
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(h, &pid); err != nil || pid == 0 {
		return nil
	}
	ph, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(ph)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(ph, 0, &buf[0], &n); err != nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	if !trustedServerImage(windows.UTF16ToString(buf[:n]), filepath.Dir(exe)) {
		return ErrUntrusted
	}
	return nil
}

// trustedPipeOwner accepts LocalSystem and the Administrators group.
func trustedPipeOwner(sid *windows.SID) bool {
	if sid == nil {
		return false
	}
	for _, k := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		if w, err := windows.CreateWellKnownSid(k); err == nil && sid.Equals(w) {
			return true
		}
	}
	return false
}
