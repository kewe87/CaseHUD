//go:build windows

package web

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAPIFileDACL(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAPIFile(dir, APIFile{APIVersion: 1}); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(filepath.Join(dir, APIFileName), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctrl, _, _ := sd.Control()
	if ctrl&windows.SE_DACL_PROTECTED == 0 {
		t.Error("DACL inherits from the folder")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		t.Fatalf("no DACL: %v", err)
	}
	if acl.AceCount != 3 { // user, SYSTEM, Administrators
		t.Errorf("DACL has %d entries, want 3", acl.AceCount)
	}
}
