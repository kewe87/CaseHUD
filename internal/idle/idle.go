// Package idle tells whether the user is away: currently, whether the
// screen saver is running.
package idle

// ScreensaverRunning reports whether the system screen saver is active.
// It is always false where that cannot be detected.
func ScreensaverRunning() bool { return screensaverRunning() }
