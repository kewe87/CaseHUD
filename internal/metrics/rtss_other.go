//go:build !windows

package metrics

func readRTSS() (fps, frametime float64, ok bool) { return 0, 0, false }
