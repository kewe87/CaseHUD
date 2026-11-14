# API and MCP

[Manual](README.md) · Previous: [Settings](settings.md) · Next: [Sensors](sensors.md)

CaseHUD can be controlled from outside: through a local HTTP API (`/api/v1`) for other programs and through a built-in MCP server for AI apps. Everything the interface can do is reachable, above all cooling including manual mode and presets. The full description is in [docs/API.md](../API.md); the machine-readable version is `GET /api/v1/openapi.json` (token with at least "Read only").

## Principles

- **Off by default.** The switches are under **Settings → Interfaces** (Expert mode): **Interface for other programs (API)** and **MCP interface for AI apps**. They can only be changed in the window, never through the API.
- **Local only.** Only `127.0.0.1` is reachable, with a token. There is no cloud connection.
- **Rights per program.** Every program only gets the rights you give it: **Read only**, **Control** and separately **System** (quit, updates).
- **Pairing.** A program asks for access in the CaseHUD window; confirm or deny the request ([Access](settings.md#access)). Granted access can be reviewed and revoked there.

## API for other programs

1. Turn on **Settings → Interfaces → Interface for other programs (API)**.
2. The program finds address and token in `api.json` in the data folder (`%APPDATA%\casehud\api.json`) and asks for access through pairing. Alternatively create a token of its own under **Settings → AI connection → Access for another program → Create access** (shown only once).
3. Calls go to `http://127.0.0.1:<port>/api/v1/…`.

Reachable are status, displays, designs, layouts, scenes, rotation, automation, lighting (including presets), cooling (including presets, manual mode and calibration), settings, and update and quit (only with the right "System"). For a silent installation: `CaseHUD-Setup-x.y.z.exe /VERYSILENT /API=1` switches the API on.

## MCP for AI apps

The built-in MCP server (`casehud-cli mcp`) offers tools for status, cooling, lighting, displays and scenes. It pairs like any other program and gets "Control" at most; quit, updates and opening the window are excluded.

1. Open **Settings → AI connection** and **Connect** the AI app ([AI connection](settings.md#ai-connection)).
2. Restart the AI app.

Alternatively copy the entry under **Settings → Interfaces** into the MCP configuration of the app.

The tools carry safety rules: change only when asked; read the cooling configuration before changing; ask before a measurement (calibration). A pump, if there is one, cannot be set below 30 % here either.

## Cooling and presets through the API

- Read cooling status and configuration, change curves and outputs (curve **or** manual value), choose, save and read presets, start and stop calibration.
- A switched-off area answers with `cooling_off`.

Details and examples: [docs/API.md](../API.md).
