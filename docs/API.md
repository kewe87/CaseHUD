# CaseHUD API v1

CaseHUD exposes a small, versioned HTTP API on `127.0.0.1` so other programs
(a TV shell, a launcher, a script, an AI assistant) can control it. A program
asks the user for access ([Pairing](#pairing)); `GET /api/v1/openapi.json` describes
every route ([OpenAPI](#openapi)). Everything under
`/api/v1/` is stable: fields are only added, never renamed or removed, and a
breaking change would get a new `/api/v2/`. Everything under `/api/` **without**
`v1` is internal to the GUI and may change at any time.

That includes the endpoints behind the case sketch editor of the lighting tab
(`/api/lighting/catalog`, `/api/lighting/sketch`, `/api/lighting/sketch/upgrade`,
`/api/lighting/sketch/suggest` and the LED colours at `/api/lighting/live`), which
answer to the GUI token only and never to the API token, the partial panel update
`PATCH /api/panels/{id}` and `DELETE /api/ack/capture` (withdrawing the consent for
screen and audio effects): they are GUI-only and not part of API v1.

All bodies are JSON (`Content-Type: application/json`) unless noted.

## Turning the API on

The API is **off by default**. Switch it on in the CaseHUD window under
Einstellungen → Schnittstellen (Settings → Interfaces), "Interface for other
programs (API)". The switch can only be changed in the window, never through
`/api/v1/`, and it applies immediately.

- While it is off there is no `api.json`, and no client token exists. An old
  file from a crashed run is removed at start.
- Every request to `/api/v1/*` without the GUI's own token (and without a
  token paired for MCP while that switch is on) is answered with 403 `api_disabled`. A client that finds no `api.json` should treat the API as
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
window), `--no-tray` (no tray icon while there is no taskbar; see below), `--demo`, `--port N`,
`--data DIR`.

Without a tray (custom shell without `explorer.exe`/taskbar, or `--no-tray`) CaseHUD works the same: the API, `gui/open` and quit do not depend on the tray. If the taskbar is missing at start, CaseHUD checks every 10 s and adds the tray icon once the shell provides one. Use `POST /api/v1/gui/open` to show the window, since there is no icon to click. With `--no-tray` and a taskbar present at start there is no tray either; but when Explorer creates the taskbar later (`TaskbarCreated`, e.g. switching from a custom shell to the desktop) CaseHUD adds the tray icon, with the same menu as without `--no-tray`. A tray that already runs is re-added after an Explorer restart.

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

Send a token in the `X-Token` header on every request:

```
GET /api/v1/status HTTP/1.1
Host: 127.0.0.1:53124
X-Token: 9f2c0e5b7a1d4c8e8b3a6d2f4e1c7a90
```

Three kinds of token open `/api/v1/*`; none of them opens the internal `/api/`
except the window's own:

| token | where it comes from | rights |
|---|---|---|
| paired program | `POST /pair`, answered by the user, see [Pairing](#pairing) | `read` or `control`, plus `system` if the user ticked it |
| "Standard-Zugang" | `token` in `api.json` | `control` and `system`; it existed before pairing, so programs that read `api.json` (CouchHUD) keep working |
| the window's own token | `instance.url` | everything, for the window itself |

- For `GET` requests whose path ends in `preview.png` the token may be given
  as `?t=<token>` instead, so the URL can be used in an `<img>` tag.
- The `Host` header must be `127.0.0.1:<port>` or `localhost:<port>` (case-insensitive)
  (protection against DNS rebinding); anything else is answered with 403
  `forbidden`.
- A missing or wrong token gives 403 `unauthorized`. A valid token without the
  scope (or the `system` right) a route needs gives 403 `insufficient_scope`.
- The port is in `api.json` (while the API is on) and in `instance.url`.

### Pairing

A program that has no token asks for one; the user answers in the CaseHUD window.

1. `POST /api/v1/pair` with `Content-Type: application/json`, no token:
   `{ "name": "My Program", "scopes": ["read", "control"] }` (`name` at most 40
   characters; `scopes` is what the program asks for, `["read"]` if left out;
   `"system"` may be added, see [Scopes](#scopes)).
   Answer `202`: `{ "id": "…", "status": "pending", "expiresIn": 120, "poll": "/api/v1/pair/…" }`.
2. CaseHUD shows "*name* möchte auf CaseHUD zugreifen" (and brings its window up if it
   was closed). The user chooses **Nur lesen**, **Steuern** or **Ablehnen** and may tick
   **Merken**; with **Steuern** the box **Auch System erlauben (Beenden, Updates)**
   is offered, unticked. The user can grant less than was asked, never more.
3. The program polls `GET /api/v1/pair/{id}` (no token, once per second is plenty):
   `{ "status": "pending" }`, then `{ "status": "denied" }` or
   `{ "status": "approved", "token": "…", "scope": "read", "system": false, "remembered": true }`.
   **The token is delivered once**; after that the id answers `404`. A request that
   nobody answers expires after 2 minutes (also `404`).
4. Use the token as `X-Token`. `GET /api/v1/token` tells a program its own name,
   `scope` and `system`, `DELETE /api/v1/token` gives the access up.

A remembered token survives a restart of CaseHUD (stored as a SHA-256 hash in
`access.json` in the data folder, with name, scope, creation and last use; the
file is private to the user like `api.json`). A token that was not remembered
works until CaseHUD quits. The user changes the rights or revokes a program under
Einstellungen → Zugriffe; revoking ends the program's open event streams at once.
The same list has **Erneuern** for the Standard-Zugang: it replaces the token in
`api.json`.

Limits: requests only on `127.0.0.1` with the right `Host`; a request with an
`Origin` header (a web page) is refused with 403, and the body must be JSON, so a
web page cannot raise the dialog; at most 3 open requests, one new request per 10 s
from the same source (interface and name) and one per second from any source
(429 `rate_limited` with `Retry-After`, 429 `too_many_pending`), at most 32 paired
programs (409 `limit_reached`). `POST /pair` answers 403 `api_disabled` while the API
switch is off. The name is only a label the user sees: it is not proof of who the
program is. The prefix `MCP (` is reserved for tokens of the MCP interface and for the AI connection of the
window: a pairing with `"interface": "api"` (or a token made in the window for another program) under such a
name is refused with 400, so a program cannot pass for a connected AI app.

### Scopes

| scope | allows |
|---|---|
| `read` | every `GET`, `POST /render`, `GET`/`DELETE /token` (`POST /render` takes `w` up to 2048, refuses a `bg=media:` background with 400, and answers 503 `busy` when two renders already run) |
| `control` | also everything that changes state: `PATCH`, `PUT`, `DELETE` and the other `POST`s, except the `system` routes |
| `system` | a separate right, **not** implied by `control`, and only usable together with it (a read-only program never has it; lowering a program to `read` takes it away); off unless the user grants it: `POST /quit`, `POST /update/install`, `POST /update/check`, `POST /gui/open` |

The user grants or takes back `system` per program under Einstellungen → Zugriffe → *Rechte ändern*; the
change applies at once. A program without it gets 403 `insufficient_scope` on those four routes. The MCP
server never asks for it. Entries of `access.json` from before the right existed have no `system`.
The server checks `control` as well as `system` for these routes.

Every state-changing request from outside is logged by CaseHUD with the name of the
program (never with a token). Some things have no route in `/api/v1/` at all and stay
in the window: releasing a pump (administrator rights), accepting the disclaimer,
confirming screen and audio effects, closing other fan programs, flashing the fan
controller (`tl` settings, boot image, serial), switching the API or MCP interface on
or off, and the areas/modules.

### OpenAPI

`GET /api/v1/openapi.json` is the machine-readable description (OpenAPI 3.1) of
every route: parameters, bodies, answers, and the scope each route needs
(`x-scope`). It needs a token with at least `read`. The description is built from
the same types the server uses, and a test fails when a route and its description
differ.

### MCP

`casehud-cli mcp` talks only to `/api/v1/` and pairs like any other program with
`{"name": "MCP (<client name>)", "scopes": ["control"], "interface": "mcp"}` (never `system`). The first
tool call without a token raises the request in CaseHUD and waits up to 25 s for the
answer; without one the tool answers "Bitte in CaseHUD den Zugriff bestätigen" and the
next call goes on with the same request. The token is kept in `mcp-token.json` in the
data folder. **The MCP switch is enforced by the server**: a token paired with
`"interface": "mcp"` only works while "MCP interface for AI assistants" is on
(403 `mcp_disabled` otherwise), and `POST /pair` with that interface needs the switch too.
API tokens need the API switch, MCP tokens the MCP switch; neither opens the other's door. Switching MCP off also
ends the open streams of MCP connections.

**What MCP never reaches is enforced by the server too.** The `api_call` tool does not offer quit, update, opening
the window, the event stream, pairing, or revoking the token; a token of the MCP interface is refused the same
routes by the server (`POST /quit`, `/update/install`, `/update/check`, `/gui/open`, `GET /events`,
`DELETE /token`: 403 `not_for_mcp`), whatever rights it has. A program that holds an MCP token and calls the API
directly gets no more than the tool would.

**Revoked access.** After the user revokes an MCP access, its token is answered with 403 `revoked` (not
`unauthorized`). The MCP server then does not ask for access again by itself: it drops the token and tells the
user to restart the AI app or the MCP server, which asks again. A token that CaseHUD never knew, or forgot with a
restart, is still `unauthorized`, and the MCP server asks again once.

**Limit of the protection.** CaseHUD protects against web pages and against programs of other users; it does not
protect against a program that runs as the same Windows user. Such a program can read `instance.url` (the token of
the window) and `api.json` (the token of the standard access, with control and system), and so does what the user
can do in CaseHUD. The restrictions for MCP tokens above guard the MCP path (an AI agent that calls tools), not a
shell the same agent may also have. Treat an AI assistant that can run commands as the user as trusted with
everything CaseHUD can do; the standard access and the window token stay, they are how local programs find
CaseHUD. The standard access can be renewed (Zugriffe), and `api.json` disappears while the API is off.

**Connecting.** Add the entry shown under Einstellungen → Schnittstellen to the MCP configuration of the
assistant (`{"mcpServers": {"casehud": {"command": "<path>\casehud-cli.exe", "args": ["mcp"]}}}`), switch
"MCP-Schnittstelle für KI-Assistenten" on, and call any tool: CaseHUD asks for the access in its window.
Choose **Nur lesen** for an assistant that should only look, **Steuern** for one that may change things.
A tool that needs a right the connection lacks answers `insufficient_scope` with the place where the
user grants it (Einstellungen → Zugriffe → Rechte ändern); the MCP server never asks for `system`.

**Tools.** Reading: `status` (one-call summary of areas, displays, cooling state and alarm, lighting, scenes),
`list_displays`, `list_sensors`, `list_designs`, `get_design`, `design_guide`, `render_design`, `cooling_status`,
`cooling_get_config`, `cooling_presets`, `cooling_calibrate_status`, `lighting_status`, `lighting_presets`,
`list_scenes`, `api_spec`. Changing (needs `control`): `save_design`, `apply_design`, `set_display`, `apply_scene`,
`cooling_set_output` (a curve **or** a manual value; a pump never below 30 %), `cooling_set_curve`,
`cooling_delete_curve`, `cooling_preset_activate`, `cooling_preset_save`, `cooling_calibrate` (needs `confirm: true`;
the fans stop for a while, the pump goes to 30 %), `cooling_calibrate_stop`, `lighting_set`, `lighting_preset_apply`,
`api_call`. The tool descriptions and the server `instructions` carry the safety rules (change only when asked, read the
cooling configuration first, ask before a measurement).

**`api_call`** `{ "method", "path", "body"? }` reaches any route of this document, so a new route needs no new tool.
`api_spec` is its reference: without arguments a compact list of every route with its scope and summary, with
`path` (and `method`) the full description of one route with parameters, body and answers, taken from the OpenAPI
description. `api_call` accepts only a plain path under `/api/v1/` (no host, no `..`, also not encoded, no fragment; a
simple query string); it never reaches `/quit`, `/update/*`, `/gui/open`, `/pair`, `DELETE /token` or `/events`, even
if the connection had the `system` right. Images (previews) come back as images. Errors of the server are passed
on with their `code`.

### AI connection

Einstellungen → KI-Verbindung sets up AI apps without editing configuration files. CaseHUD looks for the
apps below in the folders of the current user and shows each as *not installed*, *found* or *connected*.
**Connect** asks for the rights (**Nur lesen** or **Steuern**; `system` is never offered), backs up the app's
configuration file (`<file>.casehud-backup-<timestamp>`), adds only the entry `casehud`
(`command` = the `casehud-cli` next to CaseHUD, `args` = `["mcp", "--client", "<app>"]`) and switches the MCP
interface on. The file is parsed and written back atomically; a file that is not valid JSON, larger than 1 MB, or a
link (also a link or junction anywhere below the user or AppData folder) is refused and left untouched, and so is a file
with the same key twice in an object; an entry `casehud` that starts something else is only replaced after
the user confirms. The backup gets the access rights of the original, and only the five newest backups per file are kept.
The `claude` command is run without a shell; a `claude.cmd`/`.bat` only gets arguments without characters `cmd.exe` would interpret. **Disconnect** removes only CaseHUD's own entry (again with a backup) and revokes the token.
The app has to be restarted afterwards.

The access is pre-paired: CaseHUD creates a remembered token (`"MCP (<app>)"`, interface `mcp`) and stores it in
`mcp-token-<app>.json` in the data folder, which `casehud-cli mcp --client <app>` reads. The token is never
written into the app's configuration. The routes behind this (`/api/ai`, `/api/access/tokens`) belong to the
window and answer to its token only; no API or MCP token can connect an app or create a token. The token is
found again by the id of the app (not by its name), so a program cannot take over the entry by choosing a name.

*Window routes (not part of `/api/v1/`, listed for the window and its tests):*

- Errors of `POST /api/ai/{app}/connect` and `DELETE /api/ai/{app}` are JSON `{"code", "message", "error"}`.
  `code` is stable: `invalid_json` (not valid JSON, or comments), `duplicate_keys`, `symlink` (a link or junction
  was refused), `not_found` (app or configuration missing, 404), `write_failed`, `cli_failed` (the `claude` command
  failed), `unsupported`, and `conflict` (an entry `casehud` of another program exists, 409). The status is 422 when
  a file was refused (the first three, and some `unsupported` and `write_failed` cases), 404 for `not_found`, 409 for
  `conflict`, and 502 otherwise. `message` is a sentence without file or folder
  names; the details are in the log.
- `GET /api/access` also lists a request that the user allowed with "Merken" (remember) and whose program has not
  collected its token yet, as `{"id", "name", "scope", "system", "interface", "remember": true, "waiting": true}`.
  `DELETE /api/access/clients/{id}` on such an entry cancels the pairing: the program then gets 404 when it asks.

| App | Configuration | Format | Checked |
| --- | --- | --- | --- |
| Claude Desktop | `%APPDATA%\Claude\claude_desktop_config.json` | `mcpServers` | 2026-11-10, [docs](https://modelcontextprotocol.io/docs/develop/connect-local-servers) |
| Claude Code | CLI `claude mcp add --transport stdio --scope user casehud -- <exe> mcp --client claude-code` (stored in `~/.claude.json`); without the CLI on `PATH` the command is shown | CLI | 2026-11-10, [docs](https://code.claude.com/docs/en/mcp) |
| Cursor | `%USERPROFILE%\.cursor\mcp.json` | `mcpServers`, `type: stdio` | 2026-11-10, [docs](https://cursor.com/docs/context/mcp) |
| Visual Studio Code | `%APPDATA%\Code\User\mcp.json` (default profile) | `servers`, `type: stdio` | 2026-11-10, [docs](https://code.visualstudio.com/docs/agents/reference/mcp-configuration); path from the "MCP: Open User Configuration" command and third-party guides |
| LM Studio | `%USERPROFILE%\.lmstudio\mcp.json` | `mcpServers` | 2026-11-10, [blog](https://lmstudio.ai/blog/mcp) |

**Access for another program** (same section) creates a named token for a program that does not speak MCP:
rights `read` or `control`, `system` only with an explicit checkbox. It shows the address
(`http://127.0.0.1:<port>/api/v1/`) and the token once; the token is remembered like a paired one and can be
revoked under Zugriffe. Creating one switches the API on.

On the first start with a supported app installed and nothing connected, a one-time hint points to the section
("Nicht mehr anzeigen" switches it off for good).

## Errors

Every error has the same shape:

```json
{ "error": { "code": "bad_request", "message": "brightness must be 0-100" } }
```

| HTTP | code | meaning |
|---|---|---|
| 400 | `bad_request` | invalid body, unknown field, value out of range |
| 403 | `unauthorized` | missing or wrong token |
| 403 | `insufficient_scope` | the token is valid but lacks the scope the route needs (`control`, or the separate `system` right) |
| 403 | `mcp_disabled` | a token paired for MCP while the MCP switch is off |
| 403 | `not_for_mcp` | a token of the MCP interface asked for quit, update, the window, the event stream or `DELETE /token` |
| 403 | `revoked` | the token was taken away by the user (until CaseHUD forgets it) |
| 403 | `forbidden` | wrong `Host` header |
| 403 | `api_disabled` | the API is switched off in the settings (no `api.json` exists) |
| 404 | `not_found` | unknown endpoint, panel, design, scene or community design |
| 405 | `method_not_allowed` | the path exists but not with this method (`Allow` header lists the right ones) |
| 409 | `conflict` | the request cannot be done in the current state (scene activation, update, see below) |
| 409 | `area_off` | the request would change a core area that is switched off in the settings, see [Areas that are off](#areas-that-are-off) |
| 409 | `cooling_off`, `unconfigured`, `blocked`, `busy`, `invalid_preset` | `POST /cooling/preset` cannot be done, see [`POST /api/v1/cooling/preset`](#post-apiv1coolingpreset) |
| 400 | `invalid_value`, `confirmation_required` | cooling changes: a value fails the checks (a pump below 30 %), or a measurement was not confirmed |
| 409 | `revision_mismatch`, `curve_in_use`, `blocked`, `busy` | cooling changes, see [Cooling](#changing-the-cooling) |
| 413 | `payload_too_large` | the request body is larger than 1 MiB |
| 415 | `unsupported_media_type` | `POST /pair` without `Content-Type: application/json` |
| 428 | `precondition_required` | `PUT /cooling/config` without `If-Match` |
| 429 | `rate_limited`, `too_many_pending` | pairing requests come too fast or too many wait |
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
  "areas": { "display": true, "lighting": true, "cooling": true },
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
- `areas`: which core areas are switched on in the settings (*Areas*). `awtrix.enabled` and
  `lighting.enabled` are `false` while their area is off, and `panels` stays empty while the
  display area is off. See [Areas that are off](#areas-that-are-off).
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

At most 8 streams may be open at once, and at most 3 per token (a token of the MCP interface gets none: 403 `not_for_mcp`); further ones get 503
`too_many_clients`. The status is built once per 500 ms for all clients.
`status` is sent on connect and whenever the status changes (checked every
500 ms by comparing the JSON). `ping` is sent every 15 s. `cooling` (since 1.2) is
described under [`GET /api/v1/cooling`](#get-apiv1cooling). The stream ends
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
and coolant designs in turn (in GUI order; without a coolant sensor the total-power design takes the place of the coolant), a square display (the pump head)
gets the set's combined CPU + GPU design. A display with a grid keeps it and every field gets the set's equivalent design (user designs stay); without a grid the bar gets the set's wide design (without a coolant sensor: four squares without coolant). The mode becomes `hud` and the theme
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
1 for whole-number ranges, 0.1 otherwise), `default`, `values`; plus `featured`, true for the ten effects the GUI offers
(solid, breathing, rainbow, cycle, wave, sensor, meteor, and since 1.1 screen and audio, since 1.2 orbit), and for those `label`
(English GUI name, e.g. sensor is "Temperature") and `labelDe`; the other
effects stay valid), `devices` (`id`, `name`, `backend`, `state`
ok/conflict/disabled/error, `message`, `disabled`, `off`, `ignoreConflicts`, `firmware`,
`settings`, `settingValues`, `zones` with LED positions on the 320x200 canvas),
`conflicts` and `errors` (counts).

Additive fields: `ram` (`available`: the SMBus bridge of the sensor service exists, `hint`, `note`; RAM is only
driven through that bridge), per device `warning` (the device refused its settings, e.g. the TL modes that write to its
flash memory without consent), `unsupported` (found, never written to) and, for the TL controller, `sync` (headers to
follow and the number of fans) and `ports` (hardware mode without merge: per port `port` (1-based), `parts` and `labels`
(fans of the case sketch on the port, chain order), `effect` and `direction` (firmware values in use), `source` and
`effectSource` (`"sketch"` derived from the sketch with the setting `follow`, `"manual"` set per port with
`port<N>Effect`/`port<N>Direction`, `"all"` the device setting) and `whole` (the effect runs across these fans in no
firmware direction; they breathe as a whole)), per zone `mirrored` and `mirroredFrom`. Mainboard (USB) and the LED frame of the 8.8" display
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
 "devices": {"demo:board": {"disabled": false, "off": false, "ignoreConflicts": false,
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
`ignoreConflicts` are GUI-only and answered with `403 gui_only`, as is `off` for the fan controller (it is stored in the controller's flash). `off` darkens one device and keeps its effect. `status` also carries
`lighting: {enabled, active}`. Turning lighting on needs two one-time
confirmations in the CaseHUD window (the disclaimer, and the lighting notice);
without them `enabled: true` is answered with `409 ack_required`.

Screen and audio effects (since 1.1): `screen` (screen ambilight) and `audio`
(audio visualizer) derive the LED colors from the screen content and from what
the computer plays. Because that is private data they are off by default and
need a third one-time confirmation, which only the CaseHUD window can record
(the dialog opens the first time the user picks one of them). Until it is on
record, `PATCH` with `effect` `screen` or `audio` is answered with `409
ack_required` and changes nothing; the effect descriptor carries `capture:
true`. The same applies when an effect change comes through any other path.
The user can withdraw the confirmation at any time with the switch "Allow
screen and audio effects" in the CaseHUD window (Settings); that also returns a
running `screen` or `audio` effect to the default effect, and `PATCH` is
answered with `409 ack_required` again. The API can neither give nor withdraw it.
Parameters of `screen`: `display` (0 = primary monitor, 0-8), `area` (`edges`,
the default, samples a border band of the screen and fills every LED position
from the edge colour in its direction from the centre, like an ambilight; or
`full`, the whole picture), `saturation` (0-300 %), `smoothing` (0-95) and
`fps` (capture rate, 5-30, default 15). Parameters of `audio`: `mode`
(`spectrum` or `pulse`), `bands` (4-64), `sensitivity` (10-400 %), `decay`
(0-100), `rainbow`, `low`, `high` and `useAccent`. Both effects exist on
Windows only (elsewhere selecting them answers `400 bad_request`). Screen
capture runs only while the effect is being rendered (it idles one second
after lighting pauses or the effect is replaced) and everything is released
when the effect ends. Pictures and sound stay in memory: they are never
stored, logged, exported or sent. Presets cannot select these effects (library
presets naming them are ignored, and the user's own presets cannot be saved
from them), and the diagnostics export leaves out the recorded device traffic
while one of them is in use.

`GET` also carries `preset` (since 1.0.3): the `id` of the preset whose effect and parameters match the current
lighting exactly, `""` when none does. `status.lighting` carries the same `preset`.

### `GET /api/v1/lighting/presets`

Lighting presets (since 1.0.3): ready-made combinations of an effect and its parameters. Answers a list, each entry with
`id`, `name` and `description` (objects keyed by language, `de`/`en`), `effect`, `params`, `palette` (up to six
`#rrggbb` colours for a preview), `tags`, `author`, `source` and `active`. CaseHUD ships no presets: the list is
empty until the user downloads the community library or saves one. `source` is `"library"` (from the community
library) or `"user"` (saved by the user; their `id` starts with `my-`). `active` is true for the preset that matches the current lighting
(at most one). Only presets for effects the GUI offers are listed.

### `POST /api/v1/lighting/presets/{id}/apply`

Sets the effect and parameters of the preset; devices, zones, brightness and `enabled` stay as they are. Answers
`{"lighting": <state as GET /api/v1/lighting>, "ignored": [...]}`; `ignored` lists preset parameters the effect does not
know (they are left out). Unknown `id`: `404 not_found`.

### `POST /api/v1/lighting/presets`

Saves the current effect and parameters as a preset of the user's: body `{"name": "Evening"}` (1-40 printable
characters). Saving under the name of an existing own preset replaces it; at most 50 own presets. Answers the stored
preset (as in the list). Bad name or too many presets: `400 bad_request`.

### `DELETE /api/v1/lighting/presets/{id}`

Deletes one of the user's presets (`my-…`); answers `204`. Library presets cannot be deleted
(`400 bad_request`); unknown `id`: `404 not_found`.

### `GET /api/v1/cooling`

The cooling (fans and pump) as CaseHUD sees it (since 1.2). Reading only; the same token as the rest of the API.
Older versions answer `404 not_found`.

```json
{
  "enabled": true,
  "state": "running",
  "blockedBy": "",
  "preset": "abend-k3x9",
  "presets": [ { "id": "abend-k3x9", "name": "Abend" } ],
  "alarm": null,
  "outputs": [
    { "id": "port1-fan1", "name": "Radiator oben", "kind": "fan", "mode": "curve", "rpm": 620, "percent": 40, "temp": 27.5, "problem": "" },
    { "id": "port1-fan2", "name": "Radiator unten", "kind": "fan", "mode": "manual", "manual": { "value": 900, "unit": "rpm" }, "rpm": 905, "percent": 45, "temp": null, "problem": "" }
  ],
  "temperatures": [ { "id": "Board/Temperatures/Coolant", "name": "Kühlmittel", "value": 27.5 } ]
}
```

- `state`: `running` (CaseHUD controls the fans), `test` (test mode: CaseHUD only calculates and sends nothing),
  `blocked` (live, but another fan program is in charge, or CaseHUD could not rule one out, or the disclaimer is not accepted: CaseHUD sends nothing),
  `off` (the cooling area is switched off), `unconfigured` (nothing is set up that CaseHUD regulates, or
  the stored settings cannot be read; `alarm` says `config` in the second case).
  `blockedBy` is the program name when `blocked` is caused by a program, else `""`.
- `preset`: the `id` of the preset the running settings equal, `null` when none does, in particular
  after the settings were changed by hand. The preset is not stored, it is derived: it counts the curves, manual values,
  assignments and sensor names, not the mode (test or live) and not the measured speeds of the fans.
- `presets`: all presets, `id` and `name`. Presets are created, renamed and deleted in the CaseHUD window only.
  Which preset is active survives a restart of CaseHUD.
- `alarm`: `null` or the worst problem as `{ "code": "...", "name": "..." }`. `code` is one of `pump`,
  `pump_unknown`, `stale`, `config`, `emergency` (since 1.2.0: an output runs at full speed because a temperature stayed
  at 100 °C or more, see `problem`), `device`, `write`, `sensor`, `stalled`, `service` (the sensor service does
  not answer, the fans run at their safe speed), `config_backup` (the stored settings were damaged and CaseHUD
  runs on its last safety copy), `halted` (the control was stopped while CaseHUD keeps running, for example by the end of the
  session; fans and pump are not regulated, restart CaseHUD); `name` is the name the user gave the output concerned, or `""`; it is always
  `""` for `service`, `config_backup` and `halted`. It is a code, not a sentence. A block by another program is not an alarm,
  `state` says it. Treat unknown codes generally.
- `sensorsDisplayOnly` and `sensorSource` (since 1.2.1): `sensorsDisplayOnly` is `true` while all sensor readings come from a source
  other than the CaseHUD sensor service, that is LibreHardwareMonitor on port 8085 (`sensorSource` `lhm`) or a custom URL (`custom`).
  Those values are shown but never drive the cooling, so fans and pump stay at their safe speeds (`sensor_lost`, alarm `sensor`).
  Otherwise `false` and `sensorSource` is `""`. Clients can show "control only with the CaseHUD sensor service".
- `outputs`: the fans and the pump CaseHUD regulates. `kind` is `pump`, `fan` or `case` (a case fan that always runs).
  `rpm` and `temp` are `null` while unknown. `temp` is the temperature the output's curve follows, in °C.
  `mode` (since 1.2.0) is `curve`, `manual` or `none`. `manual` outputs carry `manual`: `{ "value": number, "unit": "percent" | "rpm" }`, a fixed value instead of a curve; they follow no temperature, so `temp` is `null`.
  `none` is a pump nothing regulates (no curve, no manual value): it is only watched and the board decides its duty.
  `unregulated` is `true` exactly for `mode` `none`. `problem` is `""` or exactly one code, the most serious one in this order:
  `pump_down`, `device_lost`, `write_failed`, `emergency`, `stalled`, `sensor_lost`, `not_matched`, `not_released`, `unguarded`.
  Treat unknown codes generally.
  `emergency` (since 1.2.0): a temperature stayed at 100 °C or more for 3 cycles, so the output runs at 100 % whatever its curve
  or manual value says, until the temperature is below again; `alarm` then says `emergency` as well. A `manual` output is
  watched by the sensors of all curves.
  `unguarded` (since 1.2.0) is a quiet warning, not an alarm: a `manual` output that nothing watches for overheating, because
  no curve of the configuration has a sensor. It is still regulated at its manual value; add a curve with a sensor to guard it.
  `not_matched` is a quiet warning, not an alarm: a pump on a mainboard header is regulated only after CaseHUD has shown, once,
  that its speed sensor reacts when the header's duty is raised. If it did not, the mainboard keeps the pump (it runs) and CaseHUD
  writes nothing to it. The window offers "Check again". The same holds for `not_released`.
  `unproven` is `true` for a board pump whose speed sensor and control pairing is not proven yet (check pending or
  running); a failed check is `notMatched`, not `unproven`. `calibrating` is `true` while a running calibration or speed measurement (fan or pump) holds the output;
  the control goes on with the others.
- `percent` is the duty CaseHUD sends, an integer 0-100 (in test mode and while `blocked` it is the calculated
  value, not one that was sent). It is `null` exactly when that is not what the hardware gets. For `curve` and
  `manual` outputs it is never `null` except in these cases:

  | `percent` is `null` when | flag that says why |
  |---|---|
  | nothing regulates the output | `unregulated` (`mode` `none`) |
  | a board pump is not released | `notReleased` |
  | the pump's speed sensor did not react | `notMatched` |
  | the check cannot run, no temperature source | `noSource` |
  | the board regulates the pump, the check is pending or running (Board regelt, Prüfung steht aus/läuft) | `unproven` |
  | the output is calibrating and the measurement has not sent a duty yet | `calibrating` |

  While `calibrating` is `true` and the measurement sends a duty, `percent` is exactly that duty (the standstill
  step is `0`), not what the curve asks for.
- Top level: `mode` is the stored mode, `preview` (test) or `live`. `revision` changes on every save of the cooling
  settings (see `GET /cooling/config`), not on runtime values such as speeds or temperatures.
- `temperatures`: only the sensors the cooling uses (the source of a curve) or has a name for; `value` in °C.
  A sensor without a current value is left out.
- `off`: `enabled` is `false`, all lists are empty and `preset` is `null`; still `200`. In `test` and `blocked` the
  lists stay filled as far as CaseHUD measures. In `unconfigured` the lists are empty, `presets` lists what exists.
- Limits: at most 32 presets, 64 `outputs`, 64 `temperatures`; names (preset, output, sensor) are cut to 60
  characters. A preset `id` consists of `a-z`, `0-9` and `-`, is 1 to 64 characters long and stays the same when
  the preset is renamed.

`GET /api/v1/events` also sends

```
event: cooling
data: { ... same JSON as GET /api/v1/cooling ... }
```

when the preset, the `state`, the `alarm` or the `problem` of an output changes, not with every
measured value and not on connect (read the state with `GET`).

### `POST /api/v1/cooling/preset`
```json
{ "id": "abend-k3x9" }
```
Activates a preset and answers `200` with the new state, as `GET /api/v1/cooling`. The mode stays: a preset never
switches between test and live. The preset goes through the same checks as saving the settings in the window,
against the current hardware, and is applied completely or not at all: if applying fails, the previous preset
stays active. Fans that stay on the same outputs keep running. The preset that is active already: `200`, nothing
happens. It works in test mode.

| Status | `code` | meaning |
|---|---|---|
| 400 | `bad_request` | body missing or invalid, `id` missing |
| 404 | `not_found` | no such preset |
| 409 | `cooling_off` | the cooling area is switched off in the settings |
| 409 | `unconfigured` | nothing is set up, or the stored settings cannot be read |
| 409 | `blocked` | another fan program is in charge, or the disclaimer is not accepted yet |
| 409 | `invalid_preset` | the preset no longer passes the checks (other outputs, missing sensors, a safety rule) |
| 409 | `busy` | a speed measurement is running |
| 500 | `internal` | the presets file cannot be read or the settings cannot be written; nothing was changed |

### Changing the cooling

All of these need the `control` scope except the `GET`s. They go through the same
checks as saving in the window; a pump never goes below 30 % (never 0), a fan keeps its
minimum and stop rules, and an output has a curve **or** a manual value, never both.
Changes are refused with `409 busy` while a speed measurement runs and with
`409 cooling_off` while the cooling area is off. Releasing a pump (administrator rights)
is not offered: `GET /cooling` shows `notReleased`, and the user does it in the window.

**Revision.** `GET /cooling` and `GET /cooling/config` carry `revision` (also as `ETag`).
A change sent with `If-Match: <revision>` is refused with `409 revision_mismatch` when the
settings changed in between. It is required for `PUT /cooling/config` (`428 precondition_required`
without) and optional for the other changes, which read and change under one lock. Every change
answers `200` with the new `revision`.

| Route | Body | Answer |
|---|---|---|
| `GET /cooling/config` | | `{ "config": { mode, curves, outputs, sensor_names }, "revision": n }` |
| `PUT /cooling/config` | the whole `config`; `If-Match` required | `{ "revision": n }`; `400 invalid_value` when it fails the checks |
| `PATCH /cooling/outputs/{id}` | any of `curve` (`""` = none), `manual` `{ "value", "unit": "percent"\|"rpm" }`, `name`, `min_duty`, `stop_at`, `start_duty`, `step_up`, `step_down`, `safe_duty`, `offset`; `curve` and `manual` exclude each other | `{ "revision": n }`; pump below 30 % (`min_duty`, manual value): `400 invalid_value`; unknown output or curve: `404` |
| `PUT /cooling/curves/{id}` | a curve as in `config.curves` (`id` may be left out) | `201` new, `200` replaced, `{ "revision": n }` |
| `DELETE /cooling/curves/{id}` | | `{ "revision": n }`; a curve in use: `409 curve_in_use` |
| `GET /cooling/presets` | | `{ "presets": [{ id, name }], "active": id, "max", "nameMax" }` |
| `POST /cooling/presets` | `{ "name" }` saves the running settings | `201 { id, name }` |
| `PUT /cooling/presets/{id}` | `{ "name" }` renames | `204` |
| `POST /cooling/presets/{id}/overwrite` | | `204` |
| `DELETE /cooling/presets/{id}` | | `204` |
| `POST /cooling/preset` | `{ "id" }` activates (see above) | the cooling |
| `PUT /cooling/mode` | `{ "mode": "preview"\|"live" }` | `{ "mode", "revision" }`; `live` is refused with `409 blocked` while the disclaimer is not accepted or another fan program runs |
| `GET /cooling/measure` | | the state of the measurement (`running`, `step`, `results`, `outcomes`, …) |
| `POST /cooling/measure` | `{ "id" }` or `{ "all": true }`, and `"confirm": true` | `202` and the state; without `confirm`: `400 confirmation_required` |
| `POST /cooling/measure-pump` | `{ "id", "confirm": true }` | `202` and the state |
| `DELETE /cooling/measure` | `?keep=finished` optional | `204`; without the parameter the run is discarded, with it the fans that were already finished are stored; the output goes back to its safe duty either way |
| `POST /cooling/recheck-pump` | `{ "id" }` | `204`; for an output with `notMatched` |

A measurement drives the fans through their whole range, so it needs `"confirm": true`
and is refused (`409 conflict`) unless CaseHUD controls the fans.

`GET /cooling` also says what a client should tell the user: the top-level `mode`
(`preview` or `live`) and `revision`, and per output `mode` (`curve`, `manual`, `none`),
`manual`, `unregulated`, `notReleased`, `notMatched` and `noSource` next to `problem`.

### Areas that are off

The settings (*Areas*) switch the displays, the lighting and the cooling off one by one. A request that would
change an area that is off is answered `409` with the code `area_off` and changes nothing. This applies to
the endpoints of the displays (`PATCH /panels/{id}`, `POST /panels/{id}/identify`, `POST /identify`,
`POST /sets/apply`, `POST /scenes/{id}/activate`, `PATCH /rotation`, `POST /rotation/next`, `PATCH /automation`,
`POST /power`, `POST /gaming`, `PATCH /awtrix`) while the display area is off, and to `PATCH /lighting` and
`POST /lighting/presets/{id}/apply` while the lighting area is off. `POST /cooling/preset` answers with its own
code, `cooling_off`. Reading stays possible and shows the truth: `GET /status` has `areas`, an area that is
off reports no panels, `awtrix.enabled` or `lighting.enabled` as `false`, and `GET /cooling` answers
`state: "off"`. Designs, layouts, the community gallery and saving or deleting lighting presets only touch
stored data and work as before.

### Rotation, automation, AWTRIX, scenes, layouts, designs and settings

These mirror what the window reads and changes, through the same engine calls and checks.
Changes need `control`; layouts follow the *Areas* rule (`409 area_off` while the display area is off).

| Route | Body | Answer |
|---|---|---|
| `GET /rotation` | | `{ enabled, mode, intervalSeconds, effect, cascade, conveyor, order, active, scene, gaming }` (the scenes: `GET /scenes`) |
| `GET /automation` | | `{ enabled, rules: [{ when, scene, off }], enterSeconds, leaveSeconds, ignoreGames, state: { activeRule, pending, when, scene, game }, gaming }` |
| `GET /awtrix` | | `{ enabled, clocks: [{ id, name, host, apps, disabled, model, ok, status }], apps }` |
| `POST /scenes` | `{ "name", "overwrite"?, "noRotation"? }` stores what the displays are configured to show | `201 { id, name, active }`; an existing name needs `overwrite: true`, else `409 exists`; `400` for an empty or over-long (64) name or more than 50 scenes |
| `PATCH /scenes/{id}` | `{ "name" }` renames (rules of the automation follow) | the scene; `404`, `409 exists` |
| `GET /layouts/{id}` | | the layout as in `GET /layouts`; `404` |
| `PUT /layouts/{id}` | `{ "name", "grid", "tiles": [...] }` as the fields of a panel | `204`; displays and scenes that use it follow; ready-made layouts: `409 builtin`; `400` for a bad id, grid or tile |
| `DELETE /layouts/{id}` | | `204`; displays keep what it showed; `404`, `409 builtin` |
| `DELETE /designs/{id}` | | `204`; built-in designs: `409 builtin`; a design that a display, scene, layout or the rotation uses: `409 in_use`; deleting a copy saved over a built-in id brings the original back |
| `GET /settings` | | `{ autostart, autostartSupported, language, updateCheck, fahrenheit, clock12h, animation, deviceVideo, screensaverOff, intervalMs }` |
| `PATCH /settings` | any of `autostart`, `language` (`de`\|`en`), `updateCheck`, `fahrenheit`, `clock12h`, `animation`, `deviceVideo`, `screensaverOff`, `intervalMs` (`0`, `500`, `1000`, `2000`, `5000`) | the settings after the change; any other field is `400` and nothing changes |

`PATCH /settings` is a whitelist. The API and MCP switches, the sensor source and its address, the areas, the
disclaimer and confirmations, closing other programs, the fan controller's flash options, releasing a pump,
opening the log or data folder and exporting diagnostics have no route and stay in the window.

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
Needs the `system` right.
Checks GitHub for a newer release right away (at most once a minute; more
calls return the last result) and answers with the update state.
`409 conflict` when the update check is switched off in the settings.

### `POST /api/v1/update/install`
Needs the `system` right.
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
Needs the `system` right.
Answers `202 {"status":"quitting"}`, then shuts CaseHUD down cleanly (the same
path as tray *Quit*; `api.json` is removed).

### `POST /api/v1/gui/open`
Needs the `system` right.
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
