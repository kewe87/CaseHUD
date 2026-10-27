// Package conflicts lists the processes of other RGB software that drive the
// same hardware as a backend. A running process is reported as a conflict;
// it is never terminated.
package conflicts

// Category is shown to the user together with the process name.
const Category = "RGB software"

// Process lists per backend.
var (
	Mainboard = []string{"ArmouryCrate.exe", "LightingService.exe", "SignalRgb.exe", "OpenRGB.exe"}
	RAM       = []string{"LightingService.exe", "ArmouryCrate.exe", "SignalRgb.exe", "OpenRGB.exe"}
	Razer     = []string{"RazerAppEngine.exe", "Razer Synapse 3.exe", "Razer Synapse.exe", "SignalRgb.exe", "OpenRGB.exe"}
	Ring      = []string{"L-Connect 3.exe", "L-Connect.exe", "SignalRgb.exe", "OpenRGB.exe"}
	TL        = []string{"L-Connect 3.exe", "L-Connect-Service.exe", "SignalRgb.exe", "OpenRGB.exe"}
	Hubs      = []string{"L-Connect 3.exe", "L-Connect-Service.exe", "SignalRgb.exe", "OpenRGB.exe"}
	AIO       = []string{"L-Connect 3.exe", "L-Connect-Service.exe", "SignalRgb.exe", "OpenRGB.exe"}
)
