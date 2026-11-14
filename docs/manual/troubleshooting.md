# Troubleshooting and FAQ

[Manual](README.md) · Previous: [Command line](command-line.md) · Next: [Building from source and roadmap](building.md)

## Displays

**No supported display found.**
1. Check the USB cable of the LCD fans or pump head; connect the controller directly to the mainboard, not through a hub or splitter.
2. Close vendor software for the displays ([Other software](settings.md#other-software)).
3. Select **Search for LCDs** (top right). After five unsuccessful automatic attempts the search stops ("Replug the device or select "Search again"").

**The Universal Screen 8.8" shows nothing / is stuck in monitor mode.** **Bring back** in the "monitor mode" banner ([Monitor mode](displays.md#monitor-mode)).

**An experimental device is not addressed.** It must be released under **Settings → Devices → Experimental devices** ([Experimental devices](settings.md#experimental-devices)).

**Videos do not play.** Install **ffmpeg** (`winget install ffmpeg` or `ffmpeg.exe` next to `CaseHUD.exe`). Images and GIFs work without it.

**A live preview is missing ("too many CaseHUD windows open").** Close other CaseHUD windows; it tries again.

**The displays should (or should not) go off with the screen saver.** Check the option **Turn LCDs off with the screen saver** under **Settings → General**.

## Sensors and FPS

**"Sensors offline" in the status indicator.** Select **Test connection** under **Settings → Data sources → Sensors**. Does the `CaseHUDSensors` service run (Windows Services)? With the portable ZIP, LibreHardwareMonitor must run with **Options → Remote Web Server → Run** ([Sensors](sensors.md)).

**FPS are missing.** **Set up FPS capture**, then sign out of Windows and back in; the status "Permission missing" points to the missing group membership ([FPS and latency](sensors.md#fps-and-latency)).

**A sensor does not appear in the design.** Check the path with `casehud-cli.exe sensors`; give alternatives with `|`.

## Lighting

**Nothing lights up.**
1. Is the switch **Control lighting** on? ("Off: no device access by CaseHUD")
2. Does another program control the device ("In use")? Close it or **Drive it anyway**.
3. For ARGB headers of the mainboard the **LED count** must be set ("No LED count set"; [Zones and LED count](lighting.md#zones-and-led-count)).
4. Is the device's source **Off** or **Not driven** ([Where does the colour come from?](lighting.md#where-does-the-colour-come-from))?
5. Parts of the sketch must be **connected** to a zone ([Connecting parts](lighting.md#connecting-parts)).

**RAM does not light up.** The SMBus bridge must be switched on ([RAM lighting](lighting.md#ram-lighting)).

**Screen ambilight or Audio visualizer cannot be used.** Consent under **Settings → Info & help → Allow screen and audio effects**.

## Cooling

**The fans do not react to the curves.** In **test mode** CaseHUD outputs nothing: turn on **Settings → Areas → CaseHUD controls the fans** ([Control mode and test mode](cooling.md#control-mode-and-test-mode)). If another fan program runs or the disclaimer is unconfirmed, the output stays blocked.

**No "Cooling" tab.** The tab only appears with a fan controller found or a cooling setup; the area must not be switched off.

**"Pump is not running, probably defective".** Reduce load and check the pump; the fans run at safe speed ([Safety](cooling.md#safety)).

**"Not released" / "Not matched" on the pump.** **Release header** (confirm the administrator prompt), then **Check again**. Until then the mainboard controls the pump.

**"Sensor service not responding".** Check the "CaseHUD Sensors" service in Windows Services.

**Saving is blocked.** The problems are listed above the plan (e.g. missing sensor, pump below 30 %, curve without reference sensor).

## Installation and update

**SmartScreen warns.** The programs are not code-signed yet. Checksums and signature: [SECURITY.md](../../SECURITY.md).

**The update fails.** **Try again** in the notice bar, or run the setup from [Releases](https://github.com/kewe87/CaseHUD/releases) manually. Windows asks for administrator rights.

## Getting help

1. **Settings → Info & help → Export diagnostics** creates a cleaned ZIP file (names, paths, addresses and tokens removed). Check it before passing it on.
2. Open an issue on GitHub: [github.com/kewe87/CaseHUD/issues](https://github.com/kewe87/CaseHUD/issues). For device tests use the "Hardware test report" form with device name and USB VID:PID (Device Manager → Properties → Details → Hardware Ids).
3. Do not report security issues publicly; use [SECURITY.md](../../SECURITY.md) for a confidential report.
