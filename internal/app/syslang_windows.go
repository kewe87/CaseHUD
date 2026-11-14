//go:build windows

package app

import (
	"strings"

	"golang.org/x/sys/windows"
)

// detectSystemLanguage returns "de" when the first Windows display language is
// German and "en" otherwise.
func detectSystemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err == nil && len(langs) > 0 && strings.HasPrefix(strings.ToLower(langs[0]), "de") {
		return "de"
	}
	return "en"
}
