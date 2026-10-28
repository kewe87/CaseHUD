# CaseHUD API v1

CaseHUD exposes a small, versioned HTTP API on `127.0.0.1` so other programs
(a TV shell, a launcher, a script) can control it. Everything under
`/api/v1/` is stable: fields are only added, never renamed or removed, and a
breaking change would get a new `/api/v2/`. Everything under `/api/` **without**
`v1` is internal to the GUI and may change at any time.

All bodies are JSON (`Content-Type: application/json`) unless noted.

## Turning the API on

The API is **off by default**. Switch it on in the CaseHUD window under
Einstellungen → Schnittstellen (Settings → Interfaces), "Interface for other
programs (API)". The switch can only be changed in the window, never through
`/api/v1/`, and it applies immediately.

- While it is off there is no `api.json`, and no client token exists. An old
  file from a crashed run is removed at start.
- Every request to `/api/v1/*` without the GUI's own token is answered with
  403 `api_disabled`. A client that finds no `api.json` should treat the API as
  off and ask the user to enable it.
- Switching it on creates a fresh token and writes `api.json` at once;
  switching it off deletes the file and invalidates the token at once.
- A silent installation can switch it on: `CaseHUD-Setup-x.y.z.exe /VERYSILENT /API=1`
  (without `/API=1` the setting is left as it is). Behind it,
  `CaseHUD.exe --set-api=on|off` changes only this setting of the current
  user and exits; a running instance gets the change through its settings.
- `/API=1` only works when the setup is started in a user session; otherwise
  call `CaseHUD.exe --set-api=on` as the user afterwards. `--set-api` refuses
  (message on stderr, exit code 1, nothing written) when it runs as SYSTEM,
  LocalService or NetworkService, and a failure there does not abort the setup.
- Starting with `--background` or `--no-tray` (as CouchHUD does) does not turn
  the API on. If the API is off, such a start works but the client gets no
  `api.json`.

## Discovery

While CaseHUD runs **and the API is on** it writes `api.json` into its data folder
(`%APPDATA%\casehud\api.json`, or the folder given with `--data`):

```json
{
  "apiVersion": 1,
  "version": "1.0.0",
  "port": 53124,
  "token": "9f2c0e5b7a1d4c8e8b3a6d2f4e1c7a90",
  "pid": 12345,
  "exe": "C:\\Program Files\\CaseHUD\\CaseHUD.exe"
}
```

- The file is written atomically (temporary file, then rename) and readable
  only by the current user (mode 0600; on Windows a protected ACL for the
  user, SYSTEM and Administrators). Leftover temporary files of a crashed start
  are removed on the next start. It is removed on a clean exit (tray Quit,
  `POST /api/v1/quit`, Ctrl+C / SIGTERM).
- The port and the token change on every start.
- After a crash the file can be stale. Check that the process `pid` exists and
  that its image path equals `exe`, then call `GET /api/v1/status`. If that
  fails, treat CaseHUD as not running and ignore the file.
- `--demo` writes the file only when `--data DIR` is given explicitly (never
  into the default data folder), so tools can be tested without hardware and
  without touching a real installation's file.

### `instance.pid`

Independently of the API setting, a running CaseHUD writes `instance.pid` into
the same data folder. It lets a client tell "CaseHUD runs, but the API is off"
apart from "CaseHUD is not running".

- Content: the process ID as decimal digits only (no sign, no newline required;
  trim whitespace when reading).
- Access: same as `api.json` (mode 0600; on Windows a protected ACL for the
  user, SYSTEM and Administrators).
- Lifetime: written right after start, removed on a clean exit after the
  devices have been released. After a crash it can be stale: check that the
  process exists and that its image is `CaseHUD.exe` before trusting it.
- `--demo` writes it only when `--data DIR` is given, like `api.json`.
- It contains no secret. `instance.url` next to it belongs to the CaseHUD
  window and is not part of the API; don't read it.

### Starting CaseHUD from another program

```
CaseHUD.exe --background
```

Starts CaseHUD in the tray without opening its window. If an instance is
already running (it answers the GUI token, including older versions without
this API), the new process exits silently and does **not** show the running
instance's window. Without `--background`, starting a second copy
asks the running one to show its window. Other flags: `--minimized` (tray, no
window), `--no-tray` (never create a tray icon), `--demo`, `--port N`,
`--data DIR`.

Without a tray (custom shell without `explorer.exe`/taskbar, or `--no-tray`) CaseHUD works the same: the API, `gui/open` and quit do not depend on the tray. If the taskbar is missing at start, CaseHUD checks every 10 s and adds the tray icon once the shell provides one. Use `POST /api/v1/gui/open` to show the window, since there is no icon to click.

After starting it, poll for `api.json` (and a successful `GET /api/v1/status`)
for a few seconds.

### Detecting an installation

The installer registers itself under
`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{6F3A9C2E-5B71-4C0D-9E8A-CA5E0D1A2B30}_is1`
(on 64-bit Windows, a 32-bit client reads it through WOW6432Node).

- `InstallLocation`: the install folder, with the executable `CaseHUD.exe`
  in it.
- `DisplayVersion`: the installed version.

## Authentication

Send the token from `api.json` in the `X-Token` header on every request:

```
GET /api/v1/status HTTP/1.1
Host: 127.0.0.1:53124
X-Token: 9f2c0e5b7a1d4c8e8b3a6d2f4e1c7a90
```

- The client token only works on `/api/v1/*`. The GUI's own token also works
  there; the client token does not open the internal `/api/`.
- For `GET` requests whose path ends in `preview.png` the token may be given
  as `?t=<token>` instead, so the URL can be used in an `<img>` tag.
- The `Host` header must be `127.0.0.1:<port>` or `localhost:<port>` (case-insensitive)
  (protection against DNS rebinding); anything else is answered with 403
  `forbidden`.
- A missing or wrong token gives 403 `unauthorized`.

## Errors

Every error has the same shape:

```json
{ "error": { "code": "bad_request", "message": "brightness must be 0-100" } }
```

| HTTP | code | meaning |
|---|---|---|
| 400 | `bad_request` | invalid body, unknown field, value out of range |
| 403 | `unauthorized` | missing or wrong token |
| 403 | `forbidden` | wrong `Host` header |
| 403 | `api_disabled` | the API is switched off in the settings (no `api.json` exists) |
| 404 | `not_found` | unknown endpoint, panel, design, scene or community design |
| 405 | `method_not_allowed` | the path exists but not with this method (`Allow` header lists the right ones) |
| 409 | `conflict` | the request cannot be done in the current state (scene activation, update, see below) |
| 413 | `payload_too_large` | the request body is larger than 1 MiB |
| 500 | `internal` | unexpected failure |
| 502 | `bad_gateway` | the community gallery could not be reached |
| 503 | `unavailable` | the app has no window to open (`/gui/open`) |
| 503 | `too_many_clients` | more than 8 `/events` streams are open |

Request bodies reject unknown fields with `bad_request`, so typos are not
silently ignored.

## Objects

### Panel

One connected display.

```json
{
  "id": "tllcd-p1-i0",
  "name": "UNI FAN TL LCD (port 2, fan 1)",
  "model": "UNI FAN TL LCD",
  "w": 400, "h": 400,
  "connected": true,
  "mode": "hud",
  "design": "neon-cpu",
  "theme": "dark",
  "brightness": 80,
  "rotation": 0,
  "fanNo": 2,
  "firmware": "v1.2 (2025-01-01)",
  "error": "",
  "grid": "",
  "tiles": []
}
```

| field | meaning |
|---|---|
| `id` | stable display id (also the config key); use it in URLs |
| `name` | the user's label if one was set in the GUI, else the device name |
| `model` | device name without the port/fan suffix, e.g. `UNI FAN TL LCD`, `HydroShift LCD` |
| `w`, `h` | pixel size of the canvas designs are drawn on: the native resolution, except for a bar display (1920x480) turned upright with `rotation` 90 or 270, which is 480x1920 |
| `connected` | always `true`: only connected displays are listed |
| `mode` | `hud` (a design), `media` (image, GIF or video) or `off` |
| `design` | design id (see `/designs`); kept while the mode is `media` or `off` |
| `theme` | `dark` or `light` (extension to the original spec) |
| `brightness` | 0-100 |
| `firmware` | firmware version the display reported, read only; `""` when unknown. CaseHUD never installs firmware |
| `rotation` | `0`, `90`, `180` or `270` degrees (on fan LCDs, brightness and rotation are flash writes: at most one per 2 s per device, merged to the last value and applied after the interval, at most 200 per device and day until restart) |
| `fanNo` | the number shown by *Identify*: the logical fan number (set in the arrangement view, default: chain order) |
| `error` | `""` when fine, else one of the codes below |
| `grid` | bar displays only: the id of the field layout (`4`, `3`, `2`, `2+4`, `1+6` or `8`), `""` = one design on the whole screen (the only value on other displays) |
| `tiles` | the fields of the grid in reading order (`[]` without a grid); see [Grids](#grids) |

The listed values are the *configured* ones. While a rotation is running the
display may show something else; see `rotation.scene` in the status.

### Grids

A bar display (the 1920x480 Universal Screen 8.8") can be split into fields, each
of which behaves like a small display of its own with its own design, theme and
background. The grid ids and their fields (base unit 480 px, a field is a
square 480x480 or a half field 480x240):

| `grid` | fields |
|---|---|
| `4` | 4 squares (480x480), the default |
| `3` | 3 fields of 640x480 |
| `2` | 2 fields of 960x480 |
| `2+4` | 2 squares, then 4 half fields (2 columns x 2 rows) |
| `1+6` | 1 square, then 6 half fields (3 columns x 2 rows) |
| `8` | 8 half fields (4 columns x 2 rows), small, for short distances |

The fields are numbered in reading order (left to right, top to bottom; squares
first in the mixed layouts). With `rotation` 90 or 270 the bar is mounted
upright (canvas 480x1920): the same grids are stacked top to bottom, every
field stays upright, and half fields stay 480 wide (480x240). 90 turns the
content clockwise (the right edge of the bar is up), 270 has the left edge up.

A tile is `{ "mode": "hud", "design": "half-cpu", "media": "", "theme": "", "background": "", "bgStrength": 0, "bgBloom": false }`:
`mode` is `hud`, `media` or `off`; `theme` is `dark`, `light` or `""` (the display's
theme); `background` is an effect name or `media:<file>` as in the GUI,
`bgStrength` 0-100 (0 = default). Designs of the right shape are the ones
whose `w`/`h` (from `/designs`) match the field: `card-*` and the set's square
designs fit 480x480, `half-*` fit 480x240, `w640-*`/`w960-*` fit 640x480 and
960x480 (`h640-*`/`h960-*` the same upright). Other designs are scaled to fit
the field with bars at the sides.

Plain old clients are unaffected: `design` and the other fields keep working
as before, and `design` is ignored while a grid is set.

Panel `error` codes (the only ones CaseHUD produces):

| code | meaning |
|---|---|
| `device_error` | the display rejected a frame, brightness or rotation (unplugged, busy or crashed) |
| `frame_error` | a frame could not be encoded small enough for the display |
| `content_error` | the chosen design, media file or background cannot be loaded |

A display that is unplugged disappears from the list after the next rescan;
there is no `disconnected` code.

### Status

```json
{
  "version": "1.0.0",
  "apiVersion": 1,
  "updateAvailable": "",
  "sensors": { "ok": true, "error": "" },
  "power": true,
  "disclaimerAccepted": true,
  "scene": "",
  "gaming": { "active": false, "source": "", "game": "" },
  "rotation": { "enabled": false, "scene": "" },
  "automation": { "enabled": false, "activeRule": -1, "when": "", "scene": "" },
  "awtrix": { "enabled": false, "devices": [] },
  "conflicts": [],
  "panels": []
}
```

- `updateAvailable`: the newer release version found by CaseHUD's existing
  update check, else `""`. No extra network calls are made; it is `""` while
  the update check is disabled in the settings and in development builds.
- `sensors.ok` is false when the sensor backend failed on its last refresh;
  `error` has the text.
- `power`: `false` while the displays are switched off through `POST /power`.
- `disclaimerAccepted`: `false` until the user has accepted the disclaimer in the
  CaseHUD window (first start, and again when the text changes). Until then CaseHUD
  sends nothing to any device: no displays, no lighting, no AWTRIX clocks, so
  `panels` stays empty. The acceptance cannot be given through the API. In demo
  mode (`--demo`) the simulated `panels` are listed even before acceptance, since
  nothing reaches a device; `disclaimerAccepted` still reports the real state. A client
  that sees `false` should tell the user to open CaseHUD once (`POST /api/v1/gui/open`).
- `scene`: the saved scene on screen now, however it got there (rotation,
  automation, or `POST /scenes/{id}/activate` / the CaseHUD window without
  saving), else `""`. It is the scene that `GET /scenes` marks `active`; a
  change sends a `status` event.
- `gaming`: see [Gaming](#gaming).
- `rotation.scene`: the scene on screen while a scene rotation is active,
  else `""`.
- `automation`: see [Automation](#automation). `activeRule` is the index of
  the rule in charge (`-1` for none), `when` and `scene` are that rule's
  condition and scene (`scene` is `""` for a rule that keeps the normal setup).
- `awtrix.enabled`: at least one configured AWTRIX clock is enabled.
  `devices[]` has `name`, `host`, `model` (`tc001`, `tc002` or `""`) and `ok`
  (the last push worked).
- `conflicts`: image names of programs from a fixed list that currently run and may use the
  same devices as CaseHUD (for example `"OpenRGB.exe"`). Names only; the list is read-only here.
  Closing such a program is possible only in the CaseHUD window, after the user clicked it.

## Endpoints

### `GET /api/v1/status`
Returns the status object above.

### `GET /api/v1/events`
Server-sent events (`text/event-stream`):

```
event: status
data: {"version":"1.0.0", ... same JSON as GET /status ...}

event: ping
data: {}
```

At most 8 streams may be open at once; further ones get 503
`too_many_clients`. The status is built once per 500 ms for all clients.
`status` is sent on connect and whenever the status changes (checked every
500 ms by comparing the JSON). `ping` is sent every 15 s. The stream ends
when the client disconnects. Reconnect on error.

### `GET /api/v1/panels`
Array of Panel.

### `PATCH /api/v1/panels/{id}`
Changes only the fields that are present; everything else stays as it is.

```json
{ "design": "neon-gpu", "mode": "hud", "brightness": 60, "rotation": 90, "theme": "light" }
```

All fields are optional. `mode` is `hud`, `media` or `off`; `brightness` 0-100;
`rotation` 0, 90, 180 or 270; `theme` `dark` or `light`; `design` must exist.
`design` only sets which design the display uses; it becomes visible when the
mode is `hud`. Setting `design` without `grid`/`tiles` on a display with a grid
clears the grid, so the single design shows. `mode: "media"` needs a media file already chosen for that
display in the GUI. Bar displays also take `grid` (an id from the table above, `""` clears it) and `tiles`
(an array of tiles, one per field; the count must match the grid, designs and
backgrounds must exist). A new `grid` needs its `tiles`; `tiles` alone replace
the fields of the grid already set. 400 for a grid on a display that is not a
bar, for an unknown grid, a wrong tile count or unknown designs.

`layout` puts a stored or ready-made layout (see `GET /api/v1/layouts`) on a bar
display: its grid and tiles come with it, so `grid`/`tiles` in the same request
are ignored. A display that uses a layout follows later edits of that layout
(also inside scenes); `grid`/`tiles` set by hand detach it (`layout` becomes
`""`). `layout: ""` keeps the current fields as an own composition. 400 for an
unknown layout id. The Panel reports its `layout` (`""` = own composition or a
single design).

Returns the updated Panel. 400 on invalid values (nothing
is changed), 404 for an unknown display.

### `GET /api/v1/panels/{id}/preview.png`
The frame the display currently shows, as PNG. 404 until the first frame.

### `POST /api/v1/panels/{id}/identify`, `POST /api/v1/identify`
Shows the display's number (`fanNo`) on one or on all displays for about four
seconds. 204.

### `GET /api/v1/designs`
```json
[ { "id": "neon-cpu", "name": "CPU", "set": "neon", "theme": "dark", "builtin": true, "w": 480, "h": 480 } ]
```
`set` is `clean`, `neon`, `retro`, `cyberpunk`, or `custom` for the user's own
and imported designs. Designs are authored dark, so `theme` is always `dark`;
the light variant is made on the fly (the panel's `theme`, or `?theme=light` on
previews). `w`/`h` is the design's canvas size.

### `GET /api/v1/designs/{id}/preview.png?size=N&theme=dark|light`
PNG preview rendered with live sensor values. `size` is the width in pixels,
default 200, clamped to 32..480.

### `GET /api/v1/layouts`
Array of layouts for bar displays: ready-made ones (`builtin: true`, with `tpl` =
`overview`, `gaming`, `watercooling` or `clock` and `set`; ids like `gaming-neon`)
and the user's own (`name`, stored in the `layouts` folder of the data directory).
Each has `id`, `grid` and `tiles` (as above). Deleting a layout in the GUI leaves
the displays and scenes that used it with its fields as an own composition.

### `GET /api/v1/sets`
```json
[ { "set": "neon", "theme": "dark", "name": "Neon (dark)" }, { "set": "neon", "theme": "light", "name": "Neon (light)" } ]
```
All four sets in both themes, in GUI order.

### `POST /api/v1/sets/apply`
```json
{ "set": "neon", "theme": "light", "panels": ["tllcd-p1-i0"] }
```
Applies a style to all displays, or to the listed ones. It does what the
GUI's "style for all fans" does: round fan displays get the set's CPU, GPU
and coolant designs in turn (in GUI order), a square display (the pump head)
gets the set's combined CPU + GPU design. A display with a grid keeps it and every field gets the set's equivalent design (user designs stay); without a grid the bar gets the set's wide design. The mode becomes `hud` and the theme
is set. `theme` defaults to `dark`. Response: `{"applied": ["tllcd-p1-i0"]}`.
400 for an unknown set or theme, 404 for an unknown display. If applying
fails part-way, the error message lists the displays that were already
changed.

### `GET /api/v1/scenes`
```json
[ { "id": "Gaming", "name": "Gaming", "active": false } ]
```
Scenes are saved in the GUI (tab *Scenes*). The `id` is the
scene name (URL-encode it in paths). `active` is true for the scene on screen,
whether the rotation or the automation put it there.

### `POST /api/v1/scenes/{id}/activate`
Shows that scene right now. 204. It works for every saved scene, whether or
not it is part of the rotation:

- If the rotation runs in scenes mode and the scene is one of at least two
  rotation scenes, the rotation jumps to it and restarts its interval.
- Otherwise (rotation off, conveyor mode, a single scene, or a scene kept out
  of the rotation) the scene is shown **for now only**: the displays' saved
  settings are not touched. It stays on screen until the user changes a
  display in the GUI, a rule of the automation or the rotation takes over, or
  another scene is activated (a restart of CaseHUD also brings back the saved
  setup). A running rotation shows the scene at once and carries on after one
  interval.
- With `?persist=true` (or the body `{"persist": true}`) the scene is instead
  applied as the current setup, like *Apply* in the GUI: it replaces the
  displays' saved settings. Use this only when that is what the user wants.

404 for an unknown scene. 409 `conflict` only while the automation currently
holds the displays with a scene (a game rule is in charge); try again after
it lets go.

### `PATCH /api/v1/rotation`
```json
{ "enabled": true }
```
Switches the rotation on or off and keeps its other settings. Returns
`{"enabled": true}`.

### `POST /api/v1/rotation/next`
Advances to the next scene or conveyor step now. 204.

### `POST /api/v1/power`
```json
{ "on": false }
```
Switches all displays off (black, brightness 0) or back on. The state
survives the rotation but is **not saved**: after a restart the displays are
on. Shares its mechanism with "displays off while the screen saver runs".
Returns `{"power": false}`.

### `PATCH /api/v1/automation`
```json
{ "enabled": false }
```
Switches the automation on or off and keeps its rules. Returns
`{"enabled": bool}`. The rules themselves are edited in the GUI.

### `POST /api/v1/gaming`
See [Gaming](#gaming).

### `GET /api/v1/community`
```json
[ { "id": "aurora", "name": "Aurora", "author": "someone", "set": "", "theme": "" } ]
```
The community gallery index. The index has no set or theme, so those are
empty. 502 `bad_gateway` if the gallery cannot be reached.

### `GET /api/v1/community/{id}/preview.png?size=N&theme=dark|light`
Like a design preview.

### `POST /api/v1/community/{id}/import`
Downloads the design into the user's designs as `community-<id>`. Returns
`{"id": "community-aurora"}`.

### `PATCH /api/v1/awtrix`
```json
{ "enabled": false }
```
Enables or disables all configured AWTRIX clocks (adding or removing clocks
stays in the GUI). Returns `{"enabled": bool}`, which is false when no clock is
configured.

### `GET /api/v1/lighting`, `PATCH /api/v1/lighting`

Lighting of addressable RGB hardware (case, RAM, fans). It is off by default;
while it is off CaseHUD does not talk to any lighting device. `GET` returns the
state: `enabled`, `active` (the engine runs), `tl` (TL fan controller backend,
experimental), `paused` (displays are off), `brightness` (0-100), `effect`,
`params` (values of the current effect), `effects` (names with parameter
descriptors: `key`, `label` (English), `labelDe` (German; featured effects only),
`type` number/color/bool/enum/text, `min`, `max`, `step` (number parameters:
1 for whole-number ranges, 0.1 otherwise), `default`, `values`; plus `featured`, true for the seven effects the GUI offers
(solid, breathing, rainbow, cycle, wave, sensor, meteor), and for those `label`
(English GUI name, e.g. sensor is "Temperature") and `labelDe`; the other
effects stay valid), `devices` (`id`, `name`, `backend`, `state`
ok/conflict/disabled/error, `message`, `disabled`, `ignoreConflicts`,
`settings`, `settingValues`, `zones` with LED positions on the 320x200 canvas),
`conflicts` and `errors` (counts).

Additive fields: `ram` (`available`: the SMBus bridge of the sensor service exists, `hint`, `note`; RAM is only
driven through that bridge), per device `warning` (the device refused its settings, e.g. the TL modes that write to its
flash memory without consent), `unsupported` (found, never written to) and, for the TL controller, `sync` (headers to
follow and the number of fans), per zone `mirrored` and `mirroredFrom`. Mainboard (USB) and the LED frame of the 8.8" display
are active whenever lighting is on. The TL setting `allowFlashWrites` and the sync header cannot be set through v1.

`shutdown` (additive) is what the LEDs do when CaseHUD quits or lighting is switched off: `"off"` (default, last
frame black), `"keep"` (devices keep their last colours) or `"device"` (RAM and supported mainboards return to their own
saved effect, all other devices turn off). TL fans on mainboard sync keep following the mainboard in every case. `GET`
reports `"color"` for a fixed colour stored by an older configuration; `PATCH` accepts only the three names above
(anything else is `400`).

Zones: `count` is the configured number of LEDs for zones whose length the hardware cannot report (mainboard ARGB
headers; it equals `leds`), for all other zones what the device reports. `label` is the name the user gave the zone
(for example "Case strip"; absent when none), `name` stays the technical name. A part list with parallel parts (a
splitter: several parts on one header, each starting at LED 0) appears as `segments` with `start`; `echo` lists the
canvas positions of LEDs that parallel parts repeat. The name can be set with
`devices.<id>.zones.<zone>.label` (up to 40 printable characters, `""` removes it); it is stored by CaseHUD next to
the lighting settings and is not written to any device. `devices.<id>.zones.<zone>.segments[].start` (integer, the
zone LED index where a segment begins; without it a segment follows the previous one) is available too.

`PATCH` changes only the fields present and answers with the new state:

```json
{"enabled": true, "brightness": 60, "shutdown": "device", "effect": "wave", "params": {"speed": 40, "color": "#3a8dff"},
 "devices": {"demo:board": {"disabled": false, "ignoreConflicts": false,
   "zones": {"argb1": {"count": 30}}}}}
```

Merge semantics: every field is optional and only what is present changes.
`params` is merged into the parameters of the effect that is current after the
request (the new `effect` if one is sent): keys not sent keep their value, and
the effect's defaults fill every key that was never set, so a change of `effect`
alone starts with the defaults of that effect (or the values it had when it was
last used; parameters are remembered per effect and nothing is dropped). `GET`
`params` always lists every key of the current effect. `devices` is an object
keyed by device `id` (not a list); only the listed devices, and within them only
the listed fields and zones, are merged into the stored configuration, all other
devices stay as they are. Colors must be strings `#rrggbb` with the leading `#`
(hex digits in either case); they are stored and returned in lower case.

The whole request is checked first; one bad value rejects it with `400
bad_request` and changes nothing. Effects must be in `effects`; numbers are
clamped to the descriptor range, colors are `#rrggbb`, unknown parameters,
devices and zones are rejected. A zone takes `count` (resizable zones only,
a straight line), `order` (a permutation of RGB) or `segments` (shape
line/ring/rect/point/component, geometry `x`, `y`, `w`, `h` between 0 and 1, at
most 16 segments and 2048 LEDs per zone). `devices` only accepts devices that
are currently found. A
device that another RGB program drives is reported as `conflict` and left
alone. Switching the TL fan controller backend (`tl`), device `settings` and
`ignoreConflicts` are GUI-only and answered with `403 gui_only`. `status` also carries
`lighting: {enabled, active}`. Turning lighting on needs two one-time
confirmations in the CaseHUD window (the disclaimer, and the lighting notice);
without them `enabled: true` is answered with `409 ack_required`.

### `GET /api/v1/update`
The self-update state (since 1.0.2):

```json
{
  "current": "1.0.2",
  "available": { "version": "1.0.3", "url": "https://github.com/kewe87/CaseHUD/releases/tag/v1.0.3", "notes": "..." },
  "installable": true,
  "state": "idle",
  "done": 0,
  "total": 0
}
```

- `available`: the newer release found by the update check, or `null`.
  `notes` are the release notes as plain text (control and invisible
  formatting characters removed, at most 8000 characters).
- `installable`: `true` when this copy was installed with the setup and can
  update itself. The portable ZIP and development builds cannot.
- `state`: `idle`, `downloading` (`done`/`total` in bytes, `total` is `-1`
  while unknown), `starting` (the setup runs and waits for the administrator
  prompt) or `error` (`error` holds the reason; a new attempt is possible).
  `version` names the release being installed.

### `POST /api/v1/update/check`
Checks GitHub for a newer release right away (at most once a minute; more
calls return the last result) and answers with the update state.
`409 conflict` when the update check is switched off in the settings.

### `POST /api/v1/update/install`
Downloads and installs the newer release named in `available`. Optional body:

```json
{ "relaunch": "tray" }
```

`relaunch` says how CaseHUD comes back after the update: `window`, `tray`
(default) or `hidden` (`--background --no-tray`; only allowed, and the
default, when CaseHUD runs with `--no-tray`). Answers `202` with the update state; poll `GET /api/v1/update`
(about once a second) to follow `state`. Errors: `400 bad_request` (invalid
`relaunch`), `409 conflict` (no newer version known, copy not installable, an
update already running, or the last attempt is less than two minutes ago).

What happens:

1. CaseHUD downloads `SHA256SUMS.txt` and `SHA256SUMS.txt.sig` of that release
   from `github.com/kewe87/CaseHUD/releases` and checks the signature with the
   key built into CaseHUD. Nothing else is downloaded if it does not match.
2. It downloads the setup and checks its SHA-256 against the signed line.
   Only GitHub hosts are accepted, also for redirects.
3. It starts the setup silently. Windows asks the user for administrator
   rights (UAC); there is no way around that prompt. The setup closes CaseHUD
   (`CaseHUD.exe --quit`), installs, keeps the autostart choice and starts
   CaseHUD again as the signed-in user.

The API goes away while the setup runs. A client waits until `api.json` is
written again (new port and token) and reconnects. If the user declines the
prompt, CaseHUD keeps running and `state` becomes `error`.

### `POST /api/v1/quit`
Answers `202 {"status":"quitting"}`, then shuts CaseHUD down cleanly (the same
path as tray *Quit*; `api.json` is removed).

### `POST /api/v1/gui/open`
Opens the CaseHUD window, or focuses it if it is already open. Answers `202`
without a body once the window is visible (it waits up to 3 s), or `503 unavailable`
when no window appeared (or the app has none). A client in
the foreground (e.g. a fullscreen shell) should call
`AllowSetForegroundWindow(pid)` with the `pid` from `api.json` right before,
otherwise Windows' focus rules keep the window behind.

## Gaming

`POST /api/v1/gaming` lets a launcher say that a game is running, for the
[automation](#automation) (and the rotation hold it replaces):

```json
{ "active": true, "game": "Doom", "pid": 4242, "leaseSeconds": 120 }
```

- `active` is required. `game`, `pid` are informational; `leaseSeconds`
  defaults to 120 and is clamped to 10..3600.
- The hint counts as a game for `leaseSeconds` after the last report; repeat
  the call as a heartbeat. A crashed reporter therefore cannot keep the
  displays in gaming mode.
- `active: false` ends the hint at once.
- An external hint takes effect immediately; the FPS detection needs a few
  seconds of FPS readings. Gaming is active if the hint **or** the FPS
  detection says so.

Response and `status.gaming`:

```json
{ "active": true, "source": "external", "game": "Doom" }
```

`source` is `external` when a hint is active (it wins), `fps` when only the
FPS detection says so, and `""` when no game runs.

## Automation

The automation (GUI: Displays, *Scenes & automation*) shows a saved scene by
itself while a condition holds and returns to the normal setup afterwards. It
does not need the rotation; the rotation pauses while a rule is in charge and
carries on when it ends.

- Rules are checked top to bottom, the first match wins. `when` is `game`
  (any game: FPS readings or the external hint above) or `game:<exe>` (one
  game, e.g. `game:eldenring.exe`, compared case-insensitively, `.exe`
  optional). A rule whose `scene` is `""` keeps the normal setup and just
  pauses the rotation. A rule whose scene does not exist is ignored.
- A rule must match for 3 s before it takes over and stop matching for 10 s
  before it lets go (Alt-Tab does not flicker). On top of that the FPS
  detection itself needs 5 s of readings; an external hint counts at once.
- The executable name comes from the external hint's `game` or, for FPS
  detection, from PresentMon (the process that delivers the frames).
- FPS detection only counts the process in the foreground. A program that is
  on the built-in list of desktop software (shell, browsers, terminals,
  editors, Office, chat, launcher UIs, capture and video tools) or on the
  user's `ignore_games` list never counts. Otherwise it must cover its
  monitor (fullscreen or borderless), or as a window present on a flip path
  at a sustained 20 FPS or more. `ignore_games` (executable names) is part of
  the automation settings in the GUI ("Not a game - ignore"). The external
  hint is not affected and always wins.
- Manual changes to a display while a rule is in charge are saved and shown
  at once; they are not undone until the rule ends, and the normal setup
  (with the change) is back then.
- Rules saved by older versions (rotation "pause while gaming" and "gaming
  scene") are converted to a `game` rule when the config is loaded.
- No "media playing" condition exists: CaseHUD has no now-playing detection.

## Silent install and update

The Windows installer (`CaseHUD-Setup-x.y.z.exe`, attached to each GitHub
release) is an Inno Setup program:

```
CaseHUD-Setup-1.0.0.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /TASKS="autostart,fps"
```

Tasks: `autostart` (start with Windows, in the tray), `fps` (let the user read
FPS data: adds them to *Performance Log Users*), `desktopicon` (off unless
named). List the tasks you want; leaving one out turns it off.

To update: since 1.0.2 an installed CaseHUD updates itself on request
(`POST /api/v1/update/install`, see above). Otherwise: download the newer
setup, run `POST /api/v1/quit` so the running instance exits, then run the new
setup with the options above. `CaseHUD.exe --quit` does the same as
`POST /api/v1/quit` from the command line (it also works with the API
switched off). It reaches the instance of the default data folder; one
started with `--data` is ended by the setup without the clean shutdown. Each release
also carries `SHA256SUMS.txt` in `sha256sum` format (`<hex digest>  <file name>`
per line, LF line endings); verify the download against it before running it.

`SHA256SUMS.txt.sig` signs that file: base64 of an ASN.1 DER
ECDSA P-256 / SHA-256 signature over the bytes `casehud-release-v1\n` followed
by the exact bytes of `SHA256SUMS.txt`. Check the signature first, then the
setup's hash against its line. The public key is
[`docs/release-signing.pub.pem`](release-signing.pub.pem) (SHA-256 of the
SubjectPublicKeyInfo DER: `cfb6863f23c892a53c93786dd1ee78478618aed680c5603fbc3955c1dde4abaf`).
