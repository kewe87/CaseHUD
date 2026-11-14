//go:build !windows

package app

import (
	"os"
	"strings"
)

// detectSystemLanguage returns "de" when the locale environment names German
// and "en" otherwise.
func detectSystemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "de") {
				return "de"
			}
			return "en"
		}
	}
	return "en"
}
