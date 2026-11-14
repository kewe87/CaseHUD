# Cooling

[Manual](README.md) · Previous: [Lighting](lighting.md) · Next: [Settings](settings.md)

> **Warning:** a badly configured cooling can overheat hardware. Check curves and sensors in [test mode](#control-mode-and-test-mode) before the first active control. Use at your own risk.

CaseHUD drives TL fans (on the fan controller) and the pump on a mainboard header itself; a separate fan program is not needed. The **Cooling** tab is a wiring plan: sensors, curves and outputs are connected with cables. The area can be switched off under [Settings → Areas](settings.md#areas).

The tab only appears when a fan controller was found or a cooling setup exists.

![Wiring plan of the cooling](img/cooling-plan.png)
<!-- BILD: Cooling tab with three columns Sensors, Curves, Fans and pump connected by cables; preset selection and status "Test mode" at the top -->

## Control mode and test mode

| Mode | Effect |
|---|---|
| **Test mode** (default) | CaseHUD only calculates ("would set {d} %"); there is no output to fans or pump. The current fan control stays active. |
| **Active control** ("Active") | CaseHUD controls fans and pump by the curves. |

Switch: **Settings → Areas → CaseHUD controls the fans** (appears under **Cooling**). Off = test mode. The status indicator in the **Cooling** tab shows "Active" or "Test mode"; "Saved: active control, not in effect at the moment" points to a control that is still blocked.

1. Set up the wiring plan and check it in test mode (curves show "would set … %").
2. Close other fan programs.
3. Turn **CaseHUD controls the fans** on and confirm the prompt ("From saving on, CaseHUD drives the fans …").
4. Back to test mode: turn the switch off. The fans go to their safe speed and stay there until another program controls them.

Active control is blocked (no output, message in the tab) while

- another fan program runs ("{program} is running. No output to fans; CaseHUD takes over once the program has exited."),
- the disclaimer is not confirmed or
- the check for other fan programs is not possible.

## Wiring plan

The plan has three columns: **Sensors**, **Curves**, **Fans and pump**. Pull a cable from port to port; clicking a cable cuts it, clicking a block shows its settings in the side panel.

1. Under **Sensors** select **Add sensor …**. Sensors are grouped by hardware (Processor, Graphics card, Mainboard, Memory, Drives, Other); **{group}: add all** takes a whole group.
2. Create a **New curve** or pull a cable from a sensor into the **Curves** column ("Pull a cable from a sensor to here to create one").
3. Click the curve and set **Kind of curve** and values in the side panel ([Curve kinds](#curve-kinds)).
4. Pull a cable from the curve to a fan in the **Fans and pump** column (or select **Connect a fan …** in the curve's side panel). Each fan has one curve; several fans can hang on one curve.
5. **Save**. **Discard** reverts unsaved changes. **Tidy up** arranges the plan.

Notes:

- Several sensors on one curve: the hottest one counts ("Sensors (the hottest one counts)").
- Fans without a curve are not controlled ("Not controlled", "No curve"). On saving, CaseHUD warns: the fan is then at safe speed, with no control and no monitoring.
- Fans on the TL controller appear as "Port {p} · Fan {f}", the pump as **Pump**. Outputs can be renamed.
- **Save** is blocked while the plan contains problems; they are listed above the plan (e.g. "Pump "{n}": at least 30 % required").
- **Remove** takes a block off the plan, **Take off the plan** a sensor.

## Curve kinds

Every curve is an object of its own. The **Kind of curve** determines how a value is derived from the sensor temperature. The type of the target ("The curve asks for") is **Percent** (all fans get the same percentage) or **Speed** (all fans reach the same speed; converted through the [calibration](#calibration) of each fan, calibration is required).

| Kind | Description |
|---|---|
| **Drawn line** | Any number of points joined by straight lines. Drag the points, double-click adds one (or **Add point**), right-click removes it. Axis: temperature in °C. |
| **Straight line** | A straight line between two points: **Low: temperature** / **Low: value** to **High: temperature** / **High: value**. |
| **Distance** | Follows the difference between two sensors, e.g. coolant above room air; independent of the season. First cable: measured sensor, second cable: reference (**Reference sensor**). The reference sensor must not also be measured. |
| **Fixed value** | Constant **Value**, no sensor required. |
| **Switch** | Two levels: switches up at **Load from (°C)** and back only at **Idle up to (°C)**; in between the level is unchanged. Values: **Value at idle**, **Value under load**. |
| **Thermostat** | Determines the lowest value that holds the **Target temperature (°C)**. Settings: **Quiet up to (°C)**, **Lowest value**, **Highest value**, **Step**, **Margin below the target (°C)**, **Reacts after (s)**. |
| **Combination** | Combines several curves (**Curves**, **Add a curve …**) with a **Calculation**: Highest value, Lowest value, Sum, Average, First minus the rest. If a sensor of one curve is lost, the fans go to their safe speed. Percent and speed cannot be mixed. |
| **Follows a fan** | Same as another fan (**Follows**), optionally with a **Distance**, also as a share in %. The followed fan must be on a curve. |

**Behaviour** (collapsible, for curves with a sensor): **Rise from (°C)**, **Fall from (°C)**, **Rise after (s)**, **Fall after (s)**, **Look ahead (s)** damp the reaction to quick temperature changes; **Distance also at the ends** applies to the kind **Distance**.

More actions in the side panel of a curve: **Duplicate**, **Delete curve**, **Name**.

## Fans and pump

In the side panel of an output:

- **Type** (only on the fan controller): **Fan**, **Case fan (always runs)** or **Pump**. A pump on a TL header must be entered with the type **Pump**.
- **May stand still**: the fan stops while the curve specifies at most the value **Stop at (%)** (set the curve to 0 at the cold end for this). On overtemperature or sensor loss it still starts.
- **Control**: **Curve** or **Manual** ([Manual mode](#manual-mode)).
- **Behaviour** (collapsible): **Minimum (%)**, **Stop at (%)**, **Start with (%)**, **Safe speed (%)**, **Step up (%/s)**, **Step down (%/s)** and **Offset (%)** (offset per output, also negative).
- After a takeover or rebuild the fan ramps from its current duty instead of jumping.

## Manual mode

Instead of a curve every output can be set to a fixed value.

1. Click the output, choose **Manual** under **Control**.
2. Set **Value** and **Unit** (percent or speed). Speed is only possible with a calibrated fan ("Speed needs a calibrated fan").
3. **Save**. The plan shows "Manual · {value} {unit}".

Rules: the value must be above 0, at most 100 in percent, and not below the output's **Minimum**. In manual mode the pump too never runs below 30 % and raises an alarm when it stands still (see [Safety](#safety)).
Temperature guard: a manual output (pump or fan) is watched through the sensors of all curves. If one of them reports 100 °C for three cycles in a row, the output runs at 100 %, the plan shows "Overheating, full speed" and the alarm banner appears. Once it is cool again the output returns step by step to its manual value. If no curve has a sensor, the plan shows "no temperature guard".

## Calibration

The calibration measures start, stop and speed range of a fan and gives suggestions for **Stop at**, **Start with** and **Minimum**. It is the basis for speed curves.

1. In the fan's side panel select **Calibrate fan** (or **Calibrate all fans** in the header bar).
2. Read and confirm the prompt.
3. Sequence per fan:
   1. **Down**: the speed is lowered from 100 % in steps until it stands still.
   2. **Standstill**: at least 30 s at 0 %.
   3. **Restart**: upwards in 1 % steps, at least 30 s per step.
4. A run usually takes under 10 minutes, longer for slow fans; the fan stands still for part of it. The other fans keep running normally.
5. Afterwards the measured values ("Stop value", "Start value") and a **suggestion** appear ("Suggestion: stop at {a} %, start with {b} %" or "minimum {m} %"). **Apply** takes the values, **Save** applies them. Without a restart found there is no suggestion for standing still. Case fans always run; only a minimum is suggested.
6. **Remove calibration** discards the measurement.

**Calibrate all fans** measures all fans and case fans one after the other (not the pump), each like a single calibration. There is a cool-down pause of 30 s between two fans.

**Abort conditions and cancelling:**

- The run aborts when the temperature rises by 10 °C or reaches 80 °C.
- **Cancel and discard** ends the run, nothing is saved. **Cancel, keep finished** saves the fans that are already finished.
- A calibration is only possible in active control, with saved changes and for outputs with a curve or manual value.

**Calibrate pump minimum:** the pump is lowered from 100 % in 5 % steps, never below 30 %. At the first sign of its speed collapsing it goes straight back to 100 %. The coolant flow drops meanwhile; start only under low load. Result: "Lowest stable value".

## Presets

Presets store the complete cooling setup and switch between them. They are also available through the API and MCP ([API and MCP](api-mcp.md)).

1. Build the setup as you like and **Save**.
2. In the **Preset** selection choose **Save as …** and enter a **Name**.
3. Switch: choose a preset in the selection. The active one is marked "Active"; deviating settings appear as "Custom". Unsaved changes are lost when switching (confirmation).
4. **Manage**: **Overwrite**, **Rename**, **Delete**.

At most 32 presets. If the setup changes in the meantime (e.g. a preset switch from outside), the plan reports "Settings changed in the meantime" and offers **Reload, discard changes**.

## Safety

### Pump

- The pump **never runs below 30 %** and **is never switched off**. This limit applies to curves, manual mode, calibration and the API.
- If the pump stops, it is defective: there is an **alarm**, and all fans on curves go to their safe speed.
- With a curve or a manual value CaseHUD controls the pump. Without either it stays with the mainboard and is only monitored ("Watched only"). When CaseHUD exits or another fan program runs, the mainboard takes over again.
- **Matching:** before controlling, CaseHUD proves once that the pump's header and speed sensor belong together. It raises the header once (by 20 percentage points, straight to 100 % from 80 %), waits for the speed sensor to follow and then hands the header back to the mainboard; the pump therefore briefly runs faster. If the pump already runs at (nearly) full speed, it is lowered briefly instead, never below its minimum and only while the system is cool. The result is stored with the names of sensor, header and mainboard (`cooling_match.json` in the data folder) and only repeated when one of them changes.
- **Requirements:** a curve with a temperature sensor must exist (otherwise "No temperature source for the check"), no calibration may run and no other fan program may be in charge. The run ends when the temperature reaches 75 °C or rises by 10 °C, or when a sensor is lost; the header then goes straight back to the mainboard. If a run proved nothing, CaseHUD tries again after 10 minutes.
- **Status in the interface:** "Not matched" (pending), "Matching"; after success CaseHUD controls the pump, after failure ("speed sensor and header do not demonstrably belong together") the mainboard keeps the pump (it keeps running); **Check again** repeats the run (only after saving). If the board has no such channel, matching fails at once.
- **Release:** the mainboard header must be released for CaseHUD once ("Not released", **Release header**). Windows asks for administrator rights; CaseHUD starts itself with `--allow-cooling-header` for this. The release is stored under `C:\ProgramData\CaseHUD`. A new request is possible after 30 s at the earliest; if the header is not yet released during matching, CaseHUD checks again after 30 s. Until released, the mainboard controls the pump.
- The **Control channel** can be chosen if needed (only channels of the same chip); a new choice is checked after saving and needs the release again.
- For the pump you can set **Pump speed sensor**, **Minimum (%)** (at least 30) and **Counts as stopped below (RPM)**.
- **Overheat protection (every output):** if the sensor of a curve reaches 100 °C for three cycles in a row, the output goes to 100 % regardless of curve and step limits; manual outputs are watched through the sensors of all curves. The output shows "Overheating, full speed" and the alarm banner appears.

### Alarms

An alarm bar ("Open cooling") appears on every page. Reported are among others:

- pump is not running, probably defective ("Reduce load and check the pump"); pump speed not readable
- sensor for a fan missing or not delivering new values (fans at safe speed)
- fan stopped despite a speed target, or does not accept a speed
- fan controller not reachable
- sensor service not responding (check the "CaseHUD Sensors" service in Windows Services)
- fan control not responding or stopped (restart CaseHUD)
- settings could not be loaded; `cooling.json` damaged, running on the last backup (the damaged file stays in the data folder)
- another fan program is running, or the disclaimer is not confirmed

### Safe speed and guard process

- Every controlled output has a **Safe speed (%)**, at least 30 %. On sensor loss, pump alarm and similar faults the fans switch to it.
- The fan controller keeps the last value set if CaseHUD crashes. Therefore CaseHUD starts its own **guard process** (`CaseHUD-Guard`) in active control. If CaseHUD ends without a clean exit, it sets the fans to safe values (never lower than before).
- On log-off and shutdown the control ends cleanly.
- If CaseHUD fails, the mainboard takes over a pump on a mainboard header.

### Other fan programs

While another fan program runs, CaseHUD stays out of the fan control and reports "{program} is running, CaseHUD is not controlling". Fans stay at safe speed or above until the other program takes over or exits. Two controls at once work against each other; turn off the fan function of the other program or close it ([Other software](settings.md#other-software)).
