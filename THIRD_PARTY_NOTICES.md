# Third-party notices / Hinweise zu Drittanbietern

## LibreHardwareMonitor — Mozilla Public License 2.0

The sensor service (`sensors/CaseHUD.Sensors`) bundles `LibreHardwareMonitorLib`,
built unmodified from the fork
[kewe87/LibreHardwareMonitor](https://github.com/kewe87/LibreHardwareMonitor)
at the pinned commit `189859f312e8f10cf15891f14206963bee635482`
(branch `asus-strix-x870-t-sensor`; see `LhmRepo` and `LhmCommit` in
`scripts/package-windows.ps1`). The fork is based on
[LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor)
and adds ASUS board definitions. The source of the MPL-licensed files is available
in the fork. License: https://mozilla.org/MPL/2.0/

The library's own notice file (`THIRD-PARTY-NOTICES.txt` in that repository)
lists Aga.Controls (BSD, used by the LibreHardwareMonitor GUI only, not part
of CaseHUD) and PawnIO.Modules (LGPL-2.1, see below).

### Libraries shipped next to the sensor service

These DLLs are copied into `sensors\` by the build. The NuGet packages and
versions are those named in the fork's `LibreHardwareMonitorLib.csproj`
at the pinned commit; each package is under its own license, which you can
read on the package page at nuget.org. CaseHUD does not modify them.

| DLL | NuGet package (version declared by the fork) |
|---|---|
| `DiskInfoToolkit.dll` | DiskInfoToolkit 2.2.1 |
| `HidSharp.dll` | HidSharp 2.6.4 |
| `RAMSPDToolkit-NDD.dll` | RAMSPDToolkit-NDD 1.6.2 |
| `System.Memory.dll` | System.Memory 4.6.3 |
| `System.Threading.AccessControl.dll` | System.Threading.AccessControl 10.0.13 |
| `BlackSharp.Core.dll`, `System.Buffers.dll`, `System.CodeDom.dll`, `System.Numerics.Vectors.dll`, `System.Runtime.CompilerServices.Unsafe.dll`, `System.Security.AccessControl.dll`, `System.Security.Principal.Windows.dll` | transitive dependencies of the packages above (versions resolved at build time) |

The sensor service project itself references only
`Microsoft.NETFramework.ReferenceAssemblies` (build time, not shipped).

## PawnIO

The setup runs `PawnIO_setup.exe`, the PawnIO driver installer that is part of the
LibreHardwareMonitor source tree at the pinned commit
(`LibreHardwareMonitor.Windows.Forms/Resources/PawnIO_setup.exe`); the build checks
its signature and copies it into the package. The driver
([namazso/PawnIO](https://github.com/namazso/PawnIO)) is licensed by its authors,
see the project for the terms. It stays installed when CaseHUD is uninstalled.

The PawnIO modules that `LibreHardwareMonitorLib` embeds (the `.bin` files under
`Resources/PawnIo`, e.g. `SmbusPIIX4` used by the optional SMBus bridge) are covered by the
"PawnIO.Modules" notice in LibreHardwareMonitor's `THIRD-PARTY-NOTICES.txt`, which
contains the GNU Lesser General Public License 2.1.

## PresentMon — MIT License

Windows release packages include `PresentMon.exe` (version 2.6.0, downloaded from the
[GameTechDev/PresentMon](https://github.com/GameTechDev/PresentMon) release page; the build
verifies its SHA-256 hash and signature) as a separate, unmodified process for FPS,
frame generation and latency measurement. Copyright (C) Intel Corporation, MIT License.

## WebView2

The window uses the Microsoft Edge WebView2 runtime that is installed on the PC
(part of Windows 11; not bundled by CaseHUD). The Go library
`go-webview2` (below) embeds a copy of `WebView2Loader.dll` to find that runtime.

## Rajdhani font — SIL Open Font License 1.1

`internal/lcd/fonts/Rajdhani-*.ttf`, Copyright (c) 2014 Indian Type Foundry.
License text: `internal/lcd/fonts/OFL.txt`.

## Go modules

Versions as in `go.mod`; licenses as found in the module sources.

- `golang.org/x/image`, `golang.org/x/sys`, `golang.org/x/text` — BSD-3-Clause (The Go Authors).
- `github.com/jchv/go-webview2` — MIT.
- `github.com/jchv/go-winloader` — ISC.
- `fyne.io/systray` — Apache-2.0.
- `github.com/godbus/dbus/v5` — BSD-style license (see the `LICENSE` file of the module); pulled in by `fyne.io/systray`.
