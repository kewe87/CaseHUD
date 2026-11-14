// Package sysperm grants the Windows permissions CaseHUD's optional features
// need without running CaseHUD itself as administrator.
package sysperm

import (
	"fmt"
	"regexp"
)

// CoolingHeaderFlag is the command line flag of the elevated helper start.
const CoolingHeaderFlag = "--allow-cooling-header="

var coolingHeaderKey = regexp.MustCompile(`\A/lpc/[a-z0-9]{1,32}/[0-9]{1,2}/control/[0-9]{1,2}\z`)

// Supported reports whether the setup can run on this OS.
func Supported() bool { return supported }

// ValidCoolingHeader reports whether key names a fan header of the mainboard.
func ValidCoolingHeader(key string) bool { return coolingHeaderKey.MatchString(key) }

// AllowCoolingHeader asks an administrator (one UAC prompt) to release a
// mainboard fan header for CaseHUD. CaseHUD starts itself elevated with
// --allow-cooling-header=<key>; that start edits the service settings and exits.
// A nil result means the elevated start was launched, not that the file is written.
func AllowCoolingHeader(key string) error {
	if !ValidCoolingHeader(key) {
		return fmt.Errorf("not a fan header of the mainboard: %q", key)
	}
	return allowCoolingHeader(key)
}
