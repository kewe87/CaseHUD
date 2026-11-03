// Imported from the CaseHUD LED module at 84c1271 (source tree only).
// Vendored: engine root, backend/fake, backend/lianlitl, backend/asusaura,
// backend/lianli (LED frame of the 8.8" display, USB 0416:8050 only),
// backend/enram and the helpers under internal/ (hid, winusb, diag,
// conflicts, proc).
//
// Also vendored: the screen and audio effects (effects/media), without the
// video effect, which reads files.
//
// Not vendored on purpose: the command line tool, the other backends and the
// in-process SMBus driver. The RAM backend therefore
// has no constructor that opens the bus in this process (the local
// constructor and the driver file are removed): it only talks to the SMBus bridge of the sensor
// service through its named pipe.
//
// Local changes: the vendored copy differs from the upstream tree by the
// omissions above, neutral wording in comments, and these changes:
//   - layout.go: a component segment is resolved with a single registry lookup
//     (segmentPoints, Positions), so a component that is replaced or removed
//     while a layout is rebuilt can no longer yield a count that does not match
//     the positions (the case sketch swaps components at runtime).
//   - engine.go (marked "Local change"): effects are created and closed outside
//     the engine lock, so an effect that holds a device cannot stall the engine.
//   - effects/media: the screen effect gained the parameter "area" (full or
//     edges: ambient light from a border band), captures only while Render is
//     called (idle after one second, resumes on demand), defaults to 15 fps and
//     is capped at 30. The audio effect opens its device on demand the same way.
// ErrUntrusted comes in English from the package; CaseHUD translates it for
// the interface (internal/app/lighting_hw.go).

package leds

// The module checks library parts itself (size cap, finite bounded
// coordinates, printable names, zone ids). CaseHUD additionally filters the
// files before LoadLibrary sees them (internal/app/library_validate.go).
