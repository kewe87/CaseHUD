package app

import (
	"testing"

	"github.com/kewe87/casehud/internal/panel"
)

func TestDisplayUnopenedUsesEnumerateSeam(t *testing.T) {
	old := unopenedDisplays
	defer func() { unopenedDisplays = old }()
	var got int
	var res []string
	unopenedDisplays = func(open []panel.Panel) []string { got = len(open); return res }

	e := &Engine{Store: &Store{Dir: t.TempDir()}}
	e.Demo = true
	if e.displayUnopened() {
		t.Fatal("demo mode must not look at devices")
	}
	e.Demo = false
	e.cfg.Disclaimer.Version = 0
	if e.displayUnopened() {
		t.Fatal("disclaimer gate ignored")
	}
	e.cfg.Disclaimer.Version = DisclaimerVersion
	if e.displayUnopened() {
		t.Fatal("nothing unopened but reported")
	}
	res = []string{`\?\usb#vid_1cbe&pid_a088`}
	if !e.displayUnopened() || got != 0 {
		t.Fatalf("unopened display not reported (open=%d)", got)
	}
	e.rescanMu.Lock() // a rescan is running: stay quiet, never queue a second one
	if e.displayUnopened() {
		t.Fatal("reported during a rescan")
	}
	e.rescanMu.Unlock()
}
