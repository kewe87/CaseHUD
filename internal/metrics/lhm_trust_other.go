//go:build !windows

package metrics

// serviceTrusted: the sensor service exists on Windows only, so no source
// elsewhere may drive the cooling.
func serviceTrusted() bool { return false }
