// Imported from the CaseHUD LED module at 84c1271 (source tree only).
// Vendored: engine root, backend/fake, backend/lianlitl, backend/asusaura,
// backend/lianli (LED frame of the 8.8" display, USB 0416:8050 only),
// backend/enram and the helpers under internal/ (hid, winusb, diag,
// conflicts, proc).
//
// Not vendored on purpose: the command line tool, the other backends, the
// media effects and the in-process SMBus driver. The RAM backend therefore
// has no constructor that opens the bus in this process (the local
// constructor and the driver file are removed): it only talks to the SMBus bridge of the sensor
// service through its named pipe.
//
// Local changes: none in the engine; the vendored copy differs from the
// upstream tree only by the omissions above and neutral wording in comments.
// ErrUntrusted comes in English from the package; CaseHUD translates it for
// the interface (internal/app/lighting_hw.go).

package leds

// The module checks library parts itself (size cap, finite bounded
// coordinates, printable names, zone ids). CaseHUD additionally filters the
// files before LoadLibrary sees them (internal/app/library_validate.go).
