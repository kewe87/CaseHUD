# Lighting

[Manual](README.md) · Previous: [Scenes](scenes.md) · Next: [Cooling](cooling.md)

The **Lighting** tab drives case, RAM and fan lighting with one shared effect. The area is optional and can be switched off under [Settings → Areas](settings.md#areas). CaseHUD sends nothing to lighting devices until lighting is switched on.

Supported devices:

- UNI FAN TL controller (fan LEDs)
- ARGB headers and onboard lighting of ASUS mainboards
- RAM sticks (only through the optional SMBus bridge of the sensor service, see [SMBUS-BRIDGE.md](../SMBUS-BRIDGE.md))
- LED frame of the Universal Screen 8.8"

![Lighting tab](img/lighting-overview.png)
<!-- BILD: Lighting tab with the unfolded case and effect animation, effect settings on the right, "Where does the colour come from?" below -->

## Turning lighting on

1. In the **Lighting** tab enable the switch **Control lighting**. Off means: no device access by CaseHUD.
2. The first time, a one-time confirmation ("Lighting") appears: CaseHUD then controls the lighting of the detected devices (e.g. mainboard, fan controller, RAM). Depending on the setting "Sync with motherboard", lighting settings may stay active after CaseHUD exits. **Understood – turn on** confirms.
3. The status shows "Active – {n} devices found". **Search again** searches for the devices again.
4. If "No lighting hardware enabled" appears, enable the detected devices under [More settings](#more-settings) (e.g. TL fan lighting).

If another program controls the same device, the device shows "In use" with "Controlled by {program}". **Drive it anyway** is possible but may flicker.

## Unfolded case

The case is shown unfolded: side view in the middle, top above, bottom below, rear and front beside it. The active effect runs as an animation across all faces; every part shows the colour at its place, also while lighting is off (simulation).

- **Live**: the colours the devices get right now. **Preview**: a computed preview of the effect; the devices get nothing right now. Other states: **Lighting off**, **Paused** (displays off), **No live data**.
- Legend: **connected** and **not connected**. Unconnected parts are marked; "{n} parts not connected – click a part" leads to [Connecting parts](#connecting-parts).
- Screen and audio effects only exist while they play and are not computed in advance ("No preview").
- Without a sketch the view shows "No sketch yet" with **Use detected devices** and **Draw sketch**.

## Choosing an effect

Next to the case:

1. Choose an **Effect**. Available: Rainbow, Solid color, Breathing, Color wave, Temperature, Color cycle, Meteor, Orbit, Color fill, Fill and fade, Shuttle, Bounce, Stack, Sparkle, Stripes, Sand glass, Spark, Meet and mix, Rainbow with gaps, Screen ambilight, Audio visualizer.
2. Set the **Brightness**.
3. Under **Effect settings** set the parameters (e.g. speed, width, color, colors 1–6, direction; for the **Temperature** effect the sensor and the values for the low and high color). The parameters depend on the effect.
4. Under [More settings](#more-settings) define what happens **On exit**.

## Where does the colour come from?

This section shows from which source each device gets its colours. Every device has exactly one source, connected by cable to its zones. A click on the device opens the side panel, where source, zones and settings are chosen.

| Source | Meaning |
|---|---|
| **CaseHUD effect** | Colour from the effect, per LED by its place in the case |
| **Mainboard** | Follows the ARGB header of the mainboard; stays after CaseHUD exits |
| **Own effect** | An effect the controller generates itself |
| **Off** | The device stays dark; effect and settings stay stored |
| **Not driven** | CaseHUD sends nothing to this device |

![Where does the colour come from?](img/lighting-sources.png)
<!-- BILD: section "Where does the colour come from?" with devices, sources and cables to the zones, side panel with device details open -->

The side panel (**Details of {device}**) contains **Source**, **Zones** (with the parts per zone, "no part assigned" or "LED count missing"), **More device settings** and **Place on the effect canvas** (every dot is an LED, placed where the effect sees it). A zone without a part follows the effect but lies as a line in the middle of the effect canvas when it has no layout of its own.

Notes on sources:

- **Sync header on the mainboard**: the fan controller passes the signal of the ARGB header on to all ports in parallel. The selection creates this header in the layout with the number of fans (26 LEDs each, in chain order). If the number of fans is unknown, read the fans first (Displays → Arrangement).
- The fan controller stores every lighting change permanently in its flash memory; frequent changes wear it out. Recommended: source **Mainboard**. When a TL controller is switched to a mode that stores every change in the controller, a notice appears ("Controller memory", **Understood – switch**); changes are sent bundled.
  On the TL controller this applies to the sources **CaseHUD effect** (one colour per LED ring, sent at most every 30 s) and **Own effect** (effect generated by the controller); **Mainboard** writes nothing to the controller memory.
- Switching off a controller device is stored in the controller ("The device stays dark, also after a restart").
- **Ports** (TL controller, source **Own effect**): every port of the controller can have an effect and a direction of its own. One row per port, e.g. "Port 1 · Side 1–3", with effect and direction; **like all ports** uses the device setting. With **Colors and direction from CaseHUD** the colours (up to four) and the speed come from the CaseHUD effect and the direction per port from the case sketch: if the effect runs down across the fans of a port, the port gets "down". If it runs across them in no direction, they light as a whole (breathing). Below each row is what the port uses now and where the value comes from. Values set per port take precedence. Only ports whose light actually changes are written; every change is stored in the controller. Not available with **Hardware effect across all fans**.

## Connecting parts

A part only lights up when it is connected to an LED zone of a device.

1. In the unfolded case click an unconnected part (or the chip "{n} parts not connected").
2. In the dialog **Connect to** choose the zone. Zones that are too short for the part are marked "too short, at most {m} LEDs". **– not connected –** removes the connection.
3. **Open in editor** jumps to the part in the [sketch](#sketch-and-editor).

### Check orientation

For every connected part, **Check orientation** tests whether it is mounted the way the sketch shows it. It is available in the **Connect to** dialog, in the part list below a zone in the device panel, in the editor under **Test LED**, and after the automatic assignment as a step for all assigned parts.

- A test pattern appears on the part and in the preview for 60 s at most. Nothing is stored on the device.
- Fans and rings: red at the top, green on the right, blue at the bottom, yellow on the left, LED 1 white. Strips: LED 1 green, the last LED red, a gradient in between.
- If the part differs from the preview, choose what it shows: **Turned by 90°/180°/270°** (clockwise, seen from the front), **Mirrored**, for a strip **Starts at the other end**. The rotation, mirroring or direction in the sketch is corrected and the pattern appears again until **Correct** is chosen.
- Parts connected in parallel (e.g. further fans while the fan controller follows the mainboard) show the colours of the part they follow and cannot be corrected on their own. Alternative: controller mode with direct port control, then only 2 colours per fan; the setting is written to the controller's memory.
- A part without a connection has no check.

Faster: **Use detected devices** creates matching parts for the detected devices. The dialog shows the suggestion; the positions can be changed in the sketch afterwards. If a matching part is missing in the catalog, a note appears under **Notes** (e.g. "No mainboard with {n} LEDs in the catalog").

## Zones and LED count

ARGB headers of the mainboard do not report the LED count; nothing lights until it is set ("No LED count set"). Two ways per header:

- **Pick parts**: add parts in the order they are connected to the header; the sum of their LEDs is the LED count. **wired in parallel (splitter)**: all parts share the same signal, the longest counts. If a part is missing, update the library under **Presets**.
- **Find LED count**: increase the number (**One more**, **Ten more**) until the last LED at the end of the chain just lights up. The red LED is the selected one, two blue ones precede it; it appears only if the LED exists. Preview only, nothing is stored in the device. Finish with **Apply**.
- For zones with **Calibrate** a test light shows the LEDs one after another; the red LED moves along, when it disappears the end is reached. The test light only writes briefly to the controller.

**Apply template**: ready-made settings for known hardware set the LED count and layout of the listed zones; effect and brightness stay unchanged. Existing layouts of these zones are replaced (confirmation). If the template names a sync header, the fan controller then follows that motherboard header; the template shows which one.
The card **Apply a template: {name}** appears while lighting is active, below the buttons of **Where does the colour come from?** and above the devices, as soon as a template fits the detected hardware.

## Sketch and editor

The editor ("Case sketch") draws the case and places parts in it to scale. The effects then follow the real positions of the LEDs.

![Case sketch editor](img/lighting-editor.png)
<!-- BILD: case sketch editor with parts list on the left, drawing area with one tab per face in the middle, properties on the right -->

1. In the **Lighting** tab select **Open sketch** (or **Edit sketch**).
2. On the right under **Case** choose the case ([Case templates](#case-templates)) and set the dimensions.
3. On the left under **Parts** drag a part onto the case or click it. Search: "Vendor, model, category …". Under **Generic** you find fan 120/140 mm (LED ring), RAM stick, LED strip, graphics card (LED frame), pump (round LED area) and distribution plate (distro plate, LED frame); more parts come from the community library. Categories include: Fans, Fans with display, Memory, LED strips, Graphics card, Pump / cooler, Distribution plate (distro plate), CPU cooler, Display, Motherboard, Controller.
4. Above the drawing area choose the view: one tab per case face (**Side view**, **Top**, **Bottom**, **Front**, **Rear**, **Back panel**) that shows only the parts of that face. The back panel is the motherboard tray side, opposite the side view.
5. Select a part and set it under **Properties** (see below).
6. **Save**. Only this adopts the sketch. **Discard** reverts all unsaved changes; "Unsaved" shows open changes.

Properties of a part:

- **Name**, **Face** (the face of the unfolded case the part lies on and takes the effect colour from)
- **Position (centre, mm)**: x from the side view, y from the top, z from the rear; the distance to the walls is shown as well. **Quick orientation**: Upright, Flat, On the floor.
- **Rotation** (**Rotate 90°** or degrees), **Mirror** (reverses the direction of the LED order), **LED side faces**, for fans **Air flow** (Intake / Exhaust; Exhaust reverses the LED order of the ring)
- **LED count** and **Length (mm)** for parts of variable size
- **Connection**: **Device** and **Zone**, **Order in the chain**, **In parallel with the previous part** (e.g. via a splitter). Connected parts take the colour of the zone; unconnected ones do not light.
- **Test LED**: **Test light** lets LED {n} of the part light briefly (nothing is changed permanently). The part must be connected and the sketch saved first.
- **Layer**, **Visible** (hidden parts stay in the sketch and keep lighting), **Mounted on** (the part moves with its carrier)

Drawing area controls: wheel zooms, Space + drag pans, Shift + click or a frame selects several parts, Ctrl+G welds into a group (Ctrl+Shift+G ungroups), Alt + click selects a single part, Alt while dragging ignores the collision check, arrow keys move by 1 mm (Shift: 10 mm), Page Up/Down changes the depth, R rotates, Del deletes. **Snap to grid** and **Grid** can be switched; **Fit** shows everything. **Undo** Ctrl+Z, **Redo** Ctrl+Y.

The **Notes** area warns e.g. about overlapping parts, parts outside the case, unconnected parts ("does not light") and too many LEDs in a zone.

**Import** / **Export** exchange the sketch as JSON (at most 1 MiB; sketches in the old format version 1 cannot be imported). Imported sketches are adopted with **Save**. **Delete sketch** removes the sketch; connected zones fall back to their default layout.

## Case templates

Case and hardware can be taken from templates:

- **Case from the library**: in the sketch under **Case** → **Choose case** → **From the library** pick a model. The dimensions then come from the library. **Custom box** allows free **Outer size (mm)** (width, height, depth), optionally with a **Chamber divider** and where the **Side view** is (left/right, seen from the front).
- **Apply template**: ready-made settings for known hardware, see [Zones and LED count](#zones-and-led-count).

The library is loaded from GitHub with **Update library** (under [Effect presets](#effect-presets)). Offline the built-in parts apply.

## Effect presets

The collapsible area **Presets** holds effect presets from the community library and your own.

1. Open **Presets** and apply a preset: it sets effect and settings; devices and layouts stay unchanged. If the effect does not know settings of the preset, they are left out.
2. Own preset: enter a name and select **Save as preset**. Own presets can be deleted; library presets are marked **Library**, own ones **Mine**.
3. **Update library** loads community presets and parts from GitHub (public files only, nothing is sent). The line below shows the state, number of parts and presets.

Presets can also be addressed through the API ([API and MCP](api-mcp.md)).

## Screen ambilight and audio visualizer

Two optional effects derive the LED colours from the screen content (**Screen ambilight**) or from the sound the PC plays (**Audio visualizer**). Because that is private data they exist only after a one-time consent.

1. Choose the effect. The one-time confirmation "Screen and audio" appears. **Understood – allow** consents.
2. Pictures and sound are processed in memory only, never stored, logged, exported or sent over the network; they are only read while the effect runs. With these effects the diagnostics do not record device traffic of the lighting devices.
3. To withdraw: turn off **Settings → Info & help → Allow screen and audio effects**. A running effect is reset to the default effect.

Settings **Screen ambilight**: **Display (0 = primary)**, **Area** (**Edges (ambilight)** or **Whole picture**), **Saturation (%)**, **Capture rate (frames/s)**. Settings **Audio visualizer**: **Mode** (**Spectrum** or **Pulse**), **Bands**, **Sensitivity (%)**, **Decay**, **Rainbow colors**.

## More settings

Collapsible area **More settings** at the bottom of the tab:

- **On exit**: **Off**, **Keep last colours** or **Devices' own effect**. Applies when CaseHUD exits and when lighting is switched off. With "Devices' own effect", devices without an effect of their own turn off.
- **TL fan lighting (experimental)**: controls the TL fan controller. In the default mode "Sync with motherboard" CaseHUD sends a single command at start; afterwards the fans follow the motherboard. This needs the sync cable from the motherboard ARGB header to the controller; without the cable choose another mode. The modes "CaseHUD effects" and "Controller effects" store every change in the flash memory of the controller and require confirmation. "Blink" and the test light write only briefly to the controller. Not yet tested on all systems.

## RAM lighting

RAM lighting needs the SMBus bridge in the sensor service. It is off by default and switched on with the file `C:\ProgramData\CaseHUD\sensors.json` containing `"smbusBridge": true`. Details and security rules: [SMBUS-BRIDGE.md](../SMBUS-BRIDGE.md).
