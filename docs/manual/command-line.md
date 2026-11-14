# Command line

[Manual](README.md) · Previous: [Sensors](sensors.md) · Next: [Troubleshooting and FAQ](troubleshooting.md)

There are two programs: `CaseHUD.exe` (the graphical interface) and `casehud-cli.exe` (for scripts and troubleshooting).

## GUI: CaseHUD.exe

| Option | Effect |
|---|---|
| `--demo` | simulated LCDs and sensors, no hardware needed |
| `--demo-setup` | with `--demo`: shows the first-run setup dialog |
| `--minimized` | starts in the tray without opening the window (autostart uses this) |
| `--background` | like `--minimized`; if CaseHUD already runs, the second start exits silently without showing the window |
| `--no-tray` | no tray icon while there is no taskbar (shells without a taskbar); if Explorer creates the taskbar later (switch to desktop mode), the icon appears; otherwise the window opens through the API or a second start |
| `--port N` | GUI port on 127.0.0.1 (default: random) |
| `--data FOLDER` | other data folder |
| `--set-api=on` / `off` | switches the API in the settings, then exits (used by the installer) |
| `--quit` | closes a running CaseHUD cleanly (used by the installer) |

CaseHUD also starts two internal calls itself that are not meant to be run by hand: `--cooling-guard` (guard process of the cooling, see [Safety](cooling.md#safety)) and `--allow-cooling-header` (releases a mainboard fan header; needs administrator rights).

Silent installation: `CaseHUD-Setup-x.y.z.exe /VERYSILENT`; with `/API=1` the API is switched on as well ([API and MCP](api-mcp.md)).

## CLI: casehud-cli.exe

```powershell
.\casehud-cli.exe devices                 # list found LCDs
.\casehud-cli.exe sensors                 # list all LibreHardwareMonitor sensors
.\casehud-cli.exe run --mock              # test display with simulated values on all LCDs
.\casehud-cli.exe preview --out test.png  # render an image with live values
```

More commands:

| Command | Effect |
|---|---|
| `version` | print the version |
| `mcp [--data DIR] [--client APP]` | MCP server on stdio for AI apps (CaseHUD must run with the MCP interface enabled; [API and MCP](api-mcp.md)) |
| `init [--layout PATH]` | write the default layout file |
| `preview [--layout PATH] [--mock] [--out FILE]` | render one image to a PNG file |
| `run [--layout PATH] [--mock] [--png FILE] [--brightness N]` | render continuously to all LCDs (or to a file with `--png`) |
| `sensors [--url URL]` | list LibreHardwareMonitor sensors |

The CLI reads sensors from LibreHardwareMonitor (Options → Remote Web Server → Run, default `http://localhost:8085/data.json`). `--mock` uses simulated values.

## Building from source

See [Building from source](building.md#building-from-source).
