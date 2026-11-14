# Displays

[Manual](README.md) · Previous: [Getting started](getting-started.md) · Next: [Scenes](scenes.md)

The **Display** area comprises the tabs **Displays**, **Scenes**, **Designs** and **Media**. It can be switched off under [Settings → Areas](settings.md#areas).

## Displays overview

The **Displays** tab shows every LCD fan, pump head and case screen with a live preview. Fans are grouped by port ("UNI FAN TL LCD · Port 2").

![Displays tab with live preview](img/displays-overview.png)
<!-- BILD: Displays tab with several fan LCDs (grouped by port) and one wide display, all with live preview -->

Top right in the tab:

- **Arrangement**: ports and numbers of the fans ([Arrangement](#arrangement))
- **Identify all**: shows each display's number on it

Empty view ("No supported display found"): check the USB cable of the LCD fans or pump head, close vendor software for the displays, then search again (top right).

## Setting up a display

1. Click a display in the **Displays** tab. The side panel opens.
2. Enter a **Name** (e.g. "Front top"). **Identify** shows the number on the display.
3. Under **Content** choose **HUD**, **Media** or **Off** ([Content](#content)).
4. Set **Color scheme**, **Brightness** and **Rotation**.
5. Changes apply immediately. **Apply to all {n} displays** transfers the setting to all displays (design, file or off).

![Side panel of a display](img/display-side-panel.png)
<!-- BILD: side panel of a fan LCD with name, content, design selection, color scheme, brightness, rotation -->

### Content

| Choice | Effect |
|---|---|
| **HUD** | A sensor design (built-in or own), optionally with an animated background and bloom |
| **Media** | An image, GIF or video from the **Media** tab |
| **Off** | Display black |

More fields depending on the device:

- **Color scheme**: **Dark** or **Light**. Colors are adapted automatically for light backgrounds (ideal for white builds).
- **Background**: None, Spin, Breathe, Radar, Orbit, Aurora, Synthwave, Digital rain, CRT. "★ Matches the design" marks the recommended one. **Strength** sets the intensity.
- **Bloom**: soft focus with glowing highlights; calms busy GIFs.
- **Rotation**: image orientation. On upright wide displays 90° = right edge up, 270° = left edge up; the cells are stacked.
- **Rotation in the device (test)**: the display rotates the image itself at 180° (saves CPU). Stored in the device and sent only on change.
- **Show the clock when CaseHUD is not running**: the display shows its own clock; the setting is stored in the display.
- **Firmware**: shows the version. Updates come from the manufacturer; CaseHUD does not install firmware. An optional check for newer versions is under [Settings → General](settings.md#general).

While rotation or automation is controlling the displays, the color scheme is saved and applies afterwards.

### Fan set

**Style for all fans** (per port group) puts CPU, GPU and coolant on three LCD fans in the same look. Without a coolant sensor the total power takes the place of the coolant, and bar displays get four squares without coolant.

## Arrangement

Defines which fan carries which number. The numbers define the order in CaseHUD and apply only there; nothing is written to the device.

1. Select **Arrangement** in the **Displays** tab (or **Settings → General → Open arrangement**).
2. Select a fan: its number appears on the display.
3. Choose a new number. If it is taken, the two fans swap.
4. **Back to displays**. **Reset numbers** restores the default numbers.

![Arrangement of the fans on the controller](img/displays-arrangement.png)
<!-- BILD: Arrangement view with the controller, ports 1 to 4 and the fans per port -->

Limits and notes:

- Connect the controller directly to a USB port of the mainboard, not through a hub or splitter.
- Port 1: fans without display only. Fans with display go to ports 2 to 4.
- At most 3 fans with display per port and 13 in total, up to 10 fans per port.
- **Find fans without display** reads the fan list from the controller once (read only); other programs using the controller may be disturbed briefly.
- **Identify** makes the fans light or blink briefly: a short write to the controller, no permanent change.

## Wide displays and layouts

The Universal Screen 8.8" (1920×480) can be split into cells. Each cell is a virtual display with its own design, color scheme and background.

1. Open the display in the **Displays** tab.
2. Under **Content** choose **Single design** or a **Layout**. Alternatively pick a template under **Ready-made compositions**: **Overview** (four squares), **Gaming**, **Water cooling** (only with a coolant sensor), **Clock + values**. Without a coolant sensor the templates replace the coolant with power or CPU clock.
3. With **Choose a layout** set a division: 4 squares, 3 cells, 2 cells, 2 large + 4 small, 1 large + 6 small, 8 small.
4. Click the cells ("Cells · click to edit") and choose a design per cell. **Show all designs** also lists designs of other shapes; they are fitted into the cell. **As display** takes the color scheme of the display.
5. **Save as layout** stores the division under **Designs → Layouts**. **Apply the layout to all {n} wide displays** transfers it to all wide displays (not to fans).

Own layouts: **Designs → Layouts → New layout**. Changes to a layout apply to every display using it. If a layout is deleted, the displays keep the cells as a custom layout.

## Pixel clocks

At the end of the **Displays** tab: group **Pixel clocks**. CaseHUD shows CPU, GPU, coolant (if present) or FPS as apps on AWTRIX clocks in the network. Supported: Ulanzi TC001 (AWTRIX 3 or AWTRIX NG) and TC002 (AWTRIX NG). The model is detected; icons and font are adjusted. The apps are removed when CaseHUD exits.

1. Select **Find clocks** (UDP 4210/4211 and HTTP in the local network, on click only).
2. **Add** the clock that was found.
3. If no clock is found: check network and power, or enter the address under **Enter address** (IP or host name, e.g. `awtrix_1a2b3c.local`) and select **Check & add**.
4. Enable the apps per clock: CPU, GPU, CPU + GPU load, Coolant (if present), FPS, PC power.

## Designs

The **Designs** tab holds more than 30 built-in designs in four sets and all your own designs.

| Set | Character |
|---|---|
| **Clean** | Minimal |
| **Neon** | Futuristic HUD with segments and glow |
| **Retro** | Amber tube monitor with LED bars and scanlines |
| **Cyberpunk** | Yellow terminal HUD with glitch effect, matches Digital rain |

Contents of the sets: CPU + GPU (temperature or load), gaming with FPS and latency, coolant loop, power in watts, total power, VRAM, GPU hotspot, clock, icons, dashboards and wide designs. **Preview:** switches the gallery between **Dark** and **Light**.

![Designs in dark and light](img/designs-gallery.png)
<!-- BILD: Designs tab with sets, dark/light preview and the buttons Community, Import, New design -->

### Editing a design

Built-in designs can be edited as an own copy with **Customize** ("– custom"); a new design is created with **New design**.

1. In the editor add widgets from the bar at the bottom: **Gauge**, **Value**, **Bar**, **Graph**, **Text**, **Clock**.
2. Select a widget in the preview and drag it. Under **Widget** set position (X, Y, width, height), **Label**, **Color**, **Sensor** and further fields. **Layers** sets the order.
3. Under **Design** set background, accent, text, labels and tile color. The option **Scanlines (CRT look)** gives the tube look.
4. **Save**. **Undo** with Ctrl+Z, **Export** saves the design as JSON.

Notes:

- Designs are drawn at 480×480 and scaled to each screen. **Round preview** shows the cutout of round displays.
- Sensor fields: pick from the list or enter a path, separate alternatives with `|` ([Sensors](sensors.md)).
- **Warn at** turns yellow, **Critical at** red (0 = off). **Lower is worse** reverses this (e.g. FPS, pump).
- **Second value** shows e.g. a temperature below the main value.
- Rings: **Ring width**, **Glow**, **Segments**, **Scale marks**, **Fade color with value**.
- Clock: **Format** `15:04` = time, `15:04:05` with seconds, `Monday, 02.01.` = weekday + date.

### Sharing and community

- **Export** / **Import**: pass designs on as JSON, including designs written by an AI. An imported design is only kept with **Save**.
- **Community** opens the gallery of shared designs; **Import** stores a design under **Yours**. Own designs can be submitted by pull request to the [CaseHUD-Library](https://github.com/kewe87/CaseHUD-Library) repository. The gallery is only loaded on click.

### Layouts

The **Layouts** view (switch at the top of the **Designs** tab) lists **Built-in layouts** and **Your layouts** with the number of displays using them. See [Wide displays and layouts](#wide-displays-and-layouts).

## Media

The **Media** tab manages images, GIFs and videos for the displays.

1. Select **Add** or drag files into the area ("Drag images, GIFs or videos here"). Formats: PNG, JPG, GIF, MP4; cropped automatically.
2. Assign a file to a display: side panel of the display → content **Media** → choose the file. Media can also be used as the background of a design.

Videos need **ffmpeg** (see [Requirements](getting-started.md#requirements)). With the option **Play video on the display (saves CPU/USB)** under [Settings → Display](settings.md#display) bar displays play videos themselves; without ffmpeg single frames are sent.

## Start and idle image

The writes to the device are off by default and can be switched on under [Settings → Devices](settings.md#devices):

- **CaseHUD logo as start image**: writes the logo (or a neutral picture) once as start and idle image **permanently** into each UNI FAN TL LCD; possible only a limited number of times. It is shown while CaseHUD is not running.
- The Universal Screen 8.8" shows the CaseHUD logo when CaseHUD quits (an ordinary picture, nothing is stored).
- **Assign serial numbers to new displays**: only for displays without a serial number, once and permanently. Existing serial numbers are never changed.

## Monitor mode

If the Universal Screen 8.8" is stuck in monitor mode (e.g. after the vendor software), the **Displays** tab shows the banner "Universal Screen 8.8" in monitor mode". **Bring back** restarts the display; it then appears again as a CaseHUD display.

## Supported devices

The list of supported and tested devices is in the [README](../../README.md#supported-devices). Experimental devices are only addressed after release under [Settings → Devices](settings.md#devices); the drivers are described in [DRIVERS.md](../DRIVERS.md).
