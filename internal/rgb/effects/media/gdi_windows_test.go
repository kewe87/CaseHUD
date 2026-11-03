package media

import (
	"testing"

	"golang.org/x/sys/windows"
)

var procGetGuiResources = windows.NewLazySystemDLL("user32.dll").NewProc("GetGuiResources")

func gdiObjects() uintptr {
	n, _, _ := procGetGuiResources.Call(uintptr(windows.CurrentProcess()), 0) // GR_GDIOBJECTS
	return n
}

// Setting up and releasing the capture objects must not leak GDI handles
// (nothing is captured here).
func TestGDISetupReleaseDoesNotLeak(t *testing.T) {
	g := &gdiScreen{display: 0}
	if err := g.setup(); err != nil {
		t.Skip("no desktop to set up on:", err)
	}
	g.release()
	before := gdiObjects()
	for i := 0; i < 50; i++ {
		if err := g.setup(); err != nil {
			t.Fatal(err)
		}
		g.release()
	}
	if after := gdiObjects(); after > before+2 {
		t.Fatalf("GDI objects grew from %d to %d", before, after)
	}
}
