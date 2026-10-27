//go:build !windows

package idle

func screensaverRunning() bool { return false }
