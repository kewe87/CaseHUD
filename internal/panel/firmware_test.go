package panel

import (
	"strings"
	"testing"
)

func TestCleanFirmware(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "",
		" v1.0 ":                 "v1.0",
		"v1\x1b[2J\n.0\x00":      "v1[2J.0",
		"‮v2":                    "v2",
		strings.Repeat("x", 500): strings.Repeat("x", firmwareMax),
	} {
		if got := CleanFirmware(in); got != want {
			t.Errorf("CleanFirmware(%q) = %q, want %q", in, got, want)
		}
	}
	var f firmwareText
	if f.get() != "" {
		t.Fatal("empty firmwareText not empty")
	}
	f.set("v2.1\r\n")
	if f.get() != "v2.1" {
		t.Fatalf("got %q", f.get())
	}
}
