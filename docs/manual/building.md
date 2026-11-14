# Building from source and roadmap

[Manual](README.md) · Previous: [Troubleshooting and FAQ](troubleshooting.md)

## Building from source

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build -o casehud.exe ./cmd/casehud
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o CaseHUD.exe ./cmd/casehud-gui
go run ./cmd/casehud-gui --demo       # GUI with simulated LCDs, no hardware needed
```

Releases are built by GitHub Actions (`.github/workflows/release.yml`): push a tag `vX.Y.Z` or start the "Release" workflow manually with the tag as input. The result is a setup and a portable ZIP for **Windows x64** with signed checksums under Releases. Other platforms are not built.

Command line of the built programs: [Command line](command-line.md). Drivers: [DRIVERS.md](../DRIVERS.md).

## Roadmap

- **1.0 – First release:** displays (UNI FAN TL LCD, Universal Screen 8.8"; more experimental), rotation, scenes and automation, AWTRIX pixel clocks, optional MCP interface and local API (rights per program), community gallery, setup with sensor service, clean update and uninstall, hotplug, standby and automatic reconnect, displays off with the screen saver, detection of vendor software with a close dialog (on click only), optional lighting (TL controller, mainboard ARGB, RAM through the SMBus bridge, 8.8" frame), device firmware display with optional check, first-run setup, signed checksums, diagnostics export and issue templates for device tests.
- **1.1 – Expanding lighting:** case sketch and layout editor for the LEDs, screen ambilight and audio visualizer. Still open (ideas, no promises): more devices and controllers.
- **1.2 – Cooling and interfaces:** own control of TL fans and pump, redesigned lighting page, API v1 with pairing, MCP and AI connection, core areas that can be switched off.
- **Ideas for later – on demand:** alarms (display pulses at a critical temperature), schedules, derived sensors such as ΔT; new widgets (drives, network), "Now Playing", Home Assistant / MQTT, own HTTP/JSON sources; more vendors and display types – only with community testers who own the hardware.

A display is missing? Open an issue with the device name and USB VID:PID – it goes faster with a tester.
