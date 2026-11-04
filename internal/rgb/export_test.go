package leds

import "time"

// SetOffRetryWaitForTest shortens the pause between failed SetOff attempts.
func SetOffRetryWaitForTest(d time.Duration) (restore func()) {
	old := offRetryWait
	offRetryWait = d
	return func() { offRetryWait = old }
}
