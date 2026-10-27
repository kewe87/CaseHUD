//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
)

var (
	quitOnce sync.Once
	quitCh   = make(chan struct{})
)

// quitApp ends runUI; safe to call repeatedly and before runUI started.
func quitApp() { quitOnce.Do(func() { close(quitCh) }) }

// showWindowWait opens the GUI in the default browser.
func showWindowWait(guiURL string) error {
	openBrowser(guiURL)
	return nil
}

// runUI opens the GUI in the default browser and runs until Ctrl+C. There is
// no tray here, so noTray changes nothing.
func runUI(guiURL string, openNow, noTray bool) {
	fmt.Println("CaseHUD GUI:", guiURL)
	if openNow {
		openBrowser(guiURL)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	for {
		select {
		case <-sig:
			return
		case <-quitCh:
			return
		}
	}
}

func openBrowser(u string) {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	exec.Command(cmd, u).Start()
}

func showError(msg string) { fmt.Fprintln(os.Stderr, "error:", msg) }
