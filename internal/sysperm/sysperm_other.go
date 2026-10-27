//go:build !windows

package sysperm

import "errors"

const supported = false

// AddToPerformanceLogUsers is Windows-only.
func AddToPerformanceLogUsers() error { return errors.New("only needed on Windows") }
