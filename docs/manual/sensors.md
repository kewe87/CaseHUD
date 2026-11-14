# Sensors

[Manual](README.md) · Previous: [API and MCP](api-mcp.md) · Next: [Command line](command-line.md)

CaseHUD reads every sensor LibreHardwareMonitor knows: CPU (Intel/AMD), GPU (NVIDIA/AMD/Intel), mainboard, coolant temperature, pump, flow, Aquacomputer and more. The sensor service contains LibreHardwareMonitor; nothing extra has to be installed. Sensor source and connection test: [Settings → Data sources](settings.md#data-sources).

## Sensor paths

A sensor is addressed by its path in LibreHardwareMonitor, e.g. `Temperatures/GPU Core`. All available sensors are listed by the command line with `casehud-cli.exe sensors` ([Command line](command-line.md)).

- **Alternatives:** separate several paths with `|`, the first one that exists is used. `Temperatures/CPU Package|Temperatures/Core (Tctl/Tdie)` therefore works on Intel and AMD.
- **Graphics cards:** dedicated graphics cards take precedence over the integrated graphics of the CPU.
- **Calculating with sensors:** sensors can be combined, e.g. `Powers/Package + Powers/GPU Package` for the total power.

Where sensors are chosen:

- in the [design editor](displays.md#editing-a-design) in the field **Sensor** (list or own path)
- in the [wiring plan of the cooling](cooling.md#wiring-plan) through **Add sensor …** (temperatures, grouped by hardware)
- with the lighting effect **Temperature** (**Sensor**)

## Boards with a coolant temperature header (T_Sensor)

The bundled sensor service already contains the fix for the coolant temperature header of boards such as the ASUS ROG STRIX X870E-E/-H and X870-F (still open in LibreHardwareMonitor itself).

## FPS and latency

FPS, frame generation and PC latency of the foreground game come through PresentMon; alternatively RivaTuner Statistics Server (RTSS, e.g. with MSI Afterburner) provides the frame rate. Setup under **Settings → Data sources → FPS & latency**:

1. Select **Set up FPS capture** (creates the required permission: membership in the Windows group "Performance Log Users").
2. Sign out of Windows and back in.
3. Wait for the status "Active · FPS sensors are available in designs".

FPS measurement also feeds the game detection of the [automation](scenes.md#game-detection). Status messages: "PresentMon.exe missing" (included in the release package next to `CaseHUD.exe`), "Permission missing", "Windows only".
