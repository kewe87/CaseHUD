# Settings

[Manual](README.md) · Previous: [Cooling](cooling.md) · Next: [API and MCP](api-mcp.md)

The **Settings** tab has a navigation on the left with the groups **Areas**, **General**, **Data sources**, **Devices**, **Display**, **Interfaces**, **AI connection**, **Access** and **Info & help**. **Display**, **Interfaces** and **Access** only appear in **Expert mode**.

![Settings](img/settings-overview.png)
<!-- BILD: Settings tab with the navigation on the left and the group Areas -->

## Areas

CaseHUD has three core areas that can be switched off individually: **Display** (screens, designs, media, scenes), **Lighting** (RGB effects and devices) and **Cooling** (control TL fans and pump). At least one area stays on ("At least one area must stay on.").

- A switched-off area loses its tab and functions; the first-run setup asks for it ([First-run setup](getting-started.md#first-run-setup)). Through the API, routes of a switched-off area answer with an error (e.g. `cooling_off`, see [API.md](../API.md)).
- Under **Cooling** there is the switch **CaseHUD controls the fans**. Off: test mode, no output to fans or pump ([Control mode and test mode](cooling.md#control-mode-and-test-mode)).

## General

| Setting | Effect |
|---|---|
| **Expert mode** | Shows advanced settings |
| **Start with Windows** | Starts minimized in the tray; the displays run immediately |
| **Temperature** | °C or °F on the displays and AWTRIX clocks |
| **Time format** | 24 h or 12 h for clock designs; weekdays follow the language (DE/EN) |
| **Arrangement** | **Open arrangement**: ports of the fan controller and numbers of the fan displays ([Arrangement](displays.md#arrangement)) |
| **Turn LCDs off with the screen saver** | While the Windows screen saver runs, the displays are black (brightness 0) |
| **Check for updates** | Daily check on GitHub. Installation only after confirmation via **Update now** (signature is verified) |
| **Check for firmware updates** | One-time query of the manufacturer's download page at start; the manufacturer sees the IP address. Nothing is downloaded or installed. Off by default |

## Data sources

### Sensors

The **Sensors** card shows the status ("Connected to the CaseHUD sensor service · {n} sensors"). **Test connection** checks the connection. If no sensors are reachable:

1. Recommended: install with the **setup**. The sensor service is included and starts automatically.
2. Alternatively run **LibreHardwareMonitor** as administrator and enable **Options → Remote Web Server → Run**.
3. Then **Test connection**.

In Expert mode: **Sensor source** (**Automatic (recommended)**, **CaseHUD sensor service only**, **LibreHardwareMonitor only**, **Custom address**) and **Address** (e.g. `http://192.168.1.20:8085/data.json`). More on sensor paths: [Sensors](sensors.md).

### FPS and latency

Frame rate of the foreground game via PresentMon, including frame generation (DLSS/FSR) and PC latency. Alternatively RivaTuner Statistics Server (RTSS), if running. **Set up FPS capture** creates the required permission; afterwards sign out of Windows and back in. The switches **PresentMon** and **RivaTuner Statistics Server** (Expert mode) choose the source. See [Sensors](sensors.md#fps-and-latency).

## Devices

### Experimental devices

Devices not yet verified on real hardware stay off until released.

1. Turn **Experimental devices** on. A one-time confirmation ("Experimental devices") points out that the drivers are untested and that CaseHUD writes nothing permanently into the devices. **Understood – enable**.
2. Under **Individual drivers** every device can be switched off individually. Cards of such devices carry the mark "experimental".
3. **Write a test report** opens the form "Hardware test report" on GitHub. Feedback (works / does not work) is welcome.

### Start and idle image of the fan LCDs

Off by default; all options are permanent writes to the device ([Start and idle image](displays.md#start-and-idle-image)):

- **CaseHUD logo as start image** and **Start image** (**CaseHUD logo** or **Neutral picture**); **Write again** writes the image once more into all connected fan LCDs.
- **Assign serial numbers to new displays**
- **Remove 8.8" start logo** (only visible when CaseHUD stored a start logo earlier): deletes this file permanently from the device.

### Other software

Programs that can control the same devices disturb each other (device lock, overwritten display). CaseHUD detects known programs and shows a bar at the top ("Other software controls the same devices").

- **Close** ends the program: first regularly (the window is asked to close for up to 5 seconds), then forcibly on request (**End forcibly**; unsaved data is lost). CaseHUD never ends a program automatically, only after confirmation and only programs of this list.
- The dialog opens once at start when such a program runs; programs that show up later only get the bar. **Later** closes the dialog, the bar stays. The ✕ on the bar hides it until the list of running programs changes. **Close all** closes all closable programs one after the other (asking about stubborn ones).
- **Ignore** (in the dialog) and the switch **Do not close** (per program under **Other software**) are the same setting: the program then triggers neither bar nor dialog and is never closed by CaseHUD. The switch reverts it.
- CaseHUD cannot end services and programs with administrator rights. End services in the Services manager (`services.msc`) or set them to "Manual"; prevent autostart permanently via Windows Settings → Apps → Startup or Task Manager → Startup.
- Programs from unusual locations are only closed after an additional confirmation.
- Under **Other software** in this group you find the list of programs (running / not started) and **Ask at start and when new programs appear**. Off: no notice, the list is kept.

For cooling there is one more rule: while another fan program runs, CaseHUD does not control the fans ([Cooling](cooling.md#other-fan-programs)).

## Display

(Expert mode)

- **Display refresh**: redraw interval of sensor designs (**As set in the design (default)**, 0.5 s, 1 s, 2 s, 5 s). Longer intervals reduce CPU load.
- **Smooth animations**: rings and numbers move smoothly to the new value (up to 30 fps, only while values change).
- **Play video on the display (saves CPU/USB)**: bar displays play videos themselves. The video is converted once with ffmpeg and cached in the data folder; without ffmpeg, single frames are sent.

## Interfaces

(Expert mode) Both switches are off by default and apply locally only (`127.0.0.1`, with token).

- **MCP interface for AI apps**: lets MCP-capable AI apps read sensors and draft designs; with the right **Control** also change displays, lighting and cooling. While the switch is on, CaseHUD shows the entry for the MCP configuration of the AI app (**Copy**).
- **Interface for other programs (API)**: control of CaseHUD by other programs on this PC (e.g. CouchHUD).

Details: [API and MCP](api-mcp.md).

## AI connection

Under **AI connection** CaseHUD connects installed AI apps to CaseHUD (MCP) without editing configuration files by hand. Supported: Claude Desktop, Claude Code, Cursor, Visual Studio Code, LM Studio. Each app appears as **Not installed**, **Found** or **Connected**.

![AI connection](img/settings-ai-connection.png)
<!-- BILD: group AI connection with the list of AI apps (Connected / Found) and the card "Access for another program" -->

1. For a found app select **Connect**.
2. Choose the rights: **Read only** (fetch status, sensors and previews) or **Control** (also change displays, lighting and cooling). The right "System" (quit, updates) is never offered.
3. Confirm **Connect**. CaseHUD backs up the app's configuration file, adds only its own entry, switches the MCP interface on and sets up the access. Other entries stay unchanged.
4. Restart the app; CaseHUD is then available there.

More actions: **Change rights**, **Disconnect** (removes the entry with a backup and revokes the access). If Claude Code is not on the PATH, CaseHUD shows the command to run. If the app already has a foreign entry "casehud", CaseHUD only replaces it after confirmation. Configurations that are not valid JSON, contain duplicate keys or are a link are left unchanged.

A one-time hint ("AI app found: {app}", **Connect …**, **Don't show again**) appears on the first start with a supported app.

**Access for another program** (**Create access**): creates a token of its own for programs without MCP that use the API. The token is shown only once; the API is switched on. Details: [API.md](../API.md#ai-connection).

## Access

(Expert mode) This lists all programs with access to CaseHUD.

- Programs ask for access themselves. In the CaseHUD window "{program} wants to access CaseHUD" appears (request through API or MCP).
  - **Deny**, **Read only** or **Control**.
  - **Also allow System (quit, updates)** exists only together with **Control** and only for the API.
  - **Remember**: the access stays after a restart. Otherwise it lasts until CaseHUD quits.
- Per entry: **Change rights** (applies immediately), **Revoke** (the program loses access immediately; a new request has to be confirmed again). Rights and "last used" are shown.
- **Default access** (token from `api.json`, e.g. for CouchHUD): **Renew** replaces the token; the old one ends immediately, programs that read it only at start must be restarted.

Give programs only the rights they need: **Read only** is enough for displaying, **Control** only for programs you trust.

## Info and help

- **Version** and link to GitHub
- **Log** (Expert mode): opens the log of background activity, useful for troubleshooting
- **Export diagnostics**: saves a ZIP file with the log tail, cleaned settings, versions, display list and lighting report. Names, paths, addresses and tokens are removed. Nothing is sent; check the file before passing it on. Optional **Capture (30 s)**: records the data packets between CaseHUD and the lighting devices for 30 seconds (only while lighting is active; change e.g. the effect meanwhile). They may contain hardware identifiers of the controller.
- **Data folder** (Expert mode): opens `%APPDATA%\casehud`
- **Open source**: projects CaseHUD is built on
- **License**: © 2026 kewe87 · GNU GPL v3.0
- **Privacy** and **Disclaimer**: texts as in the [README](../../README.md#network-and-privacy); the status shows when the disclaimer was accepted
- **Allow screen and audio effects**: consent for the lighting effects Screen ambilight and Audio visualizer. Turning it off withdraws the permission and resets a running effect to the default effect ([Screen ambilight and audio visualizer](lighting.md#screen-ambilight-and-audio-visualizer)).

## Help from the program

Every page heading and the sections (e.g. **Where does the colour come from?**, the settings groups) have a **?** next to them. It opens the matching section of this manual in the browser, in the program's language and in the version that is installed. Which place leads where is set in [links.json](../handbuch/links.json).
