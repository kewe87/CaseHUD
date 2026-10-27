package enram

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestTrustedPipeOwner(t *testing.T) {
	for _, k := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		s, _ := windows.CreateWellKnownSid(k)
		if !trustedPipeOwner(s) {
			t.Errorf("sid kind %d rejected", k)
		}
	}
	for _, k := range []windows.WELL_KNOWN_SID_TYPE{windows.WinWorldSid, windows.WinInteractiveSid, windows.WinBuiltinUsersSid} {
		s, _ := windows.CreateWellKnownSid(k)
		if trustedPipeOwner(s) {
			t.Errorf("sid kind %d accepted", k)
		}
	}
	if trustedPipeOwner(nil) {
		t.Error("nil accepted")
	}
}

// A pipe created by this (standard user) process is owned by the user and must
// be rejected.
func TestVerifyServerRejectsUserOwnedPipe(t *testing.T) {
	name := `\\.\pipe\CaseHUD.Test.` + t.Name()
	n, _ := windows.UTF16PtrFromString(name)
	h, err := windows.CreateNamedPipe(n, windows.PIPE_ACCESS_DUPLEX, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT, 2, 4096, 4096, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	f, err := openPipe(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := verifyServer(f); err != ErrUntrusted {
		// Elevated runs create Administrators-owned pipes, which are accepted.
		if err == nil && windows.GetCurrentProcessToken().IsElevated() {
			t.Skip("test runs elevated: owner is Administrators")
		}
		t.Fatalf("verifyServer = %v", err)
	}
}
