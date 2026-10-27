package enram

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrUntrusted is returned when the process serving the bridge pipe is not
// the CaseHUD sensor service: a
// process that created the pipe name first would otherwise receive the
// batches of this client and answer them.
var ErrUntrusted = errors.New("enram: SMBus bridge not trusted")

// serviceImage is the path of the sensor service below the install folder.
var serviceImage = filepath.Join("sensors", "CaseHUD.Sensors.exe")

// trustedServerImage reports whether image is the sensor service of the
// installation in installDir (the folder of the running CaseHUD).
func trustedServerImage(image, installDir string) bool {
	if image == "" || installDir == "" {
		return false
	}
	want := filepath.Clean(filepath.Join(installDir, serviceImage))
	return strings.EqualFold(filepath.Clean(image), want)
}
