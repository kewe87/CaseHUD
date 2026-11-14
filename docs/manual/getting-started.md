# Getting started

[Manual](README.md) · Next: [Displays](displays.md)

## Requirements

- Windows 10 or 11, x64 (Windows only)
- WebView2 runtime (included in Windows 11)
- For videos on the displays: **ffmpeg** (`winget install ffmpeg` or put `ffmpeg.exe` next to `CaseHUD.exe`). Images and GIFs work without it.

The programs are not code-signed yet; Windows SmartScreen may warn. Checksums and signature: [SECURITY.md](../../SECURITY.md).

## Installation

1. Download `CaseHUD-Setup-x.y.z.exe` from [Releases](https://github.com/kewe87/CaseHUD/releases) and run it.
2. The setup installs everything needed in one go: CaseHUD, the sensor service (LibreHardwareMonitor is built in), the PawnIO hardware driver and the permission for FPS capture. Optional: autostart.
3. Close any vendor software that controls the same displays or LEDs. CaseHUD detects such programs and offers to close them (see [Other software](settings.md#other-software)).
4. CaseHUD starts after the setup and shows all LCDs it found. Closing the window keeps the display running in the tray.

**The setup installs:** CaseHUD to `C:\Program Files\CaseHUD`, the `CaseHUDSensors` service (runs as SYSTEM), the PawnIO driver (only if missing), PresentMon for FPS measurement and a firewall rule for the AWTRIX search (UDP 4211, private networks only). Optional: autostart and the FPS permission (membership in the Windows group "Performance Log Users"; sign out and in afterwards).

### Portable ZIP

CaseHUD also runs from the ZIP without installing. Sensors then come from a running LibreHardwareMonitor: **Options → Remote Web Server → Run**. The ZIP does not update itself; run the new setup over it.

## First-run setup

Two dialogs appear one after the other on the first start.

1. **Disclaimer** ("Read before device control"): **Understood – continue** confirms, **Quit** closes CaseHUD. Without consent CaseHUD talks to no device.
2. **Setup** ("Choose areas"): the three core areas **Display**, **Lighting** and **Cooling** are preselected. Deselect the ones you do not need; at least one stays on. **Apply** saves, **Skip** keeps everything on.

The dialog appears once for a new configuration. The choice can be changed at any time under [Settings → Areas](settings.md#areas).

Afterwards:

1. **Displays** tab: check that all LCDs appear. If one is missing, check the USB cable and select **Search for LCDs** (top right).
2. Choose a design or media per display ([Displays](displays.md)).
3. Lighting and cooling start switched off or in test mode. They only take effect after your explicit release ([Lighting](lighting.md), [Cooling](cooling.md#control-mode-and-test-mode)).

## Interface overview

![Interface with tabs and status indicator](img/interface-overview.png)
<!-- BILD: main window with all tabs (Displays, Scenes, Lighting, Cooling, Designs, Media, Settings) and the status indicator top right -->

| Tab | Content | Chapter |
|---|---|---|
| **Displays** | All LCDs with live preview, content per display, arrangement, pixel clocks | [Displays](displays.md) |
| **Scenes** | Saved setups, automation, rotation | [Scenes](scenes.md) |
| **Lighting** | Effect on the unfolded case, sources, sketch | [Lighting](lighting.md) |
| **Cooling** | Wiring plan of sensors, curves, fans and pump | [Cooling](cooling.md) |
| **Designs** | Built-in and own designs, layouts, community | [Displays](displays.md#designs) |
| **Media** | Images, GIFs, videos | [Displays](displays.md#media) |
| **Settings** | Areas, data sources, devices, interfaces, info | [Settings](settings.md) |

Tabs of areas that are switched off are missing. The **Cooling** tab only appears when a fan controller was found or a cooling setup exists.

Top right:

- **Status indicator** ("{n} displays · sensors OK"): a click opens the settings.
- **Heart**: open-source projects CaseHUD is built on.
- **Search for LCDs**: searches for the displays again.
- **DE / EN**: language of the interface. Without a saved choice it follows the Windows display language (German, otherwise English); weekdays and decimal marks on the displays follow it.

A notice bar shows available updates (**What's new**, **Update now**), AI apps that were found and a cooling alarm (**Open cooling**).

## Update

1. When CaseHUD reports a new version, select **Update now** in the notice bar.
2. CaseHUD downloads the setup from GitHub, verifies the signature of the checksums and the setup's hash and starts it. Windows asks for administrator rights.
3. CaseHUD restarts afterwards; settings and autostart stay as they were.

Alternatively run the new `CaseHUD-Setup-x.y.z.exe` over the existing installation. The update check can be switched off under [Settings → General](settings.md#general).

Notes on updating to 1.2.0:

- If another fan program runs, CaseHUD stays out of the fan control. To use CaseHUD's control, quit the other program and set up cooling in CaseHUD.
- A pump on a mainboard header is checked once more after the update and briefly runs at full speed.
- Existing case sketches are kept; parts not bound to an LED zone are marked on the lighting page.

## Uninstalling and data folder

**Data folder:** `%APPDATA%\casehud` (settings, designs, media). Use `--data FOLDER` for another one. It is kept when you uninstall.

**Uninstalling removes** the program, the service, the firewall rule and the autostart entry of the uninstalling account.

**Left behind:**

- the data folder
- `C:\ProgramData\CaseHUD` (only holds the optional `sensors.json` of the SMBus bridge and the released mainboard fan headers; read-only for normal users)
- the PawnIO driver (independent, may be used by other software; remove it separately under Apps)
- the group membership
- permanent writes to devices, such as the fans' start image

The command-line options of `CaseHUD.exe` are listed under [Command line](command-line.md).
