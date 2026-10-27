# Device drivers

CaseHUD talks to its displays and sensor devices through small drivers. All drivers are built into the
program today, but each one is cut along the same narrow interface, so a family can later be moved out
into a separate plugin without changing the rest of CaseHUD. This page describes that interface, the
rules every driver follows and what each built-in driver does.

Source: `internal/panel` (registry in `driver.go`).

## The driver record

A driver registers itself once, from an `init` function, with `panel.Register`:

| Field | Meaning |
|---|---|
| `ID` | Stable lower-case name, e.g. `tllcd`, `slv3`. Used for the settings and the experimental switches. |
| `Name`, `Vendor` | Text for lists. `Vendor` is neutral (the USB vendor ID). |
| `Kind` | `display` (produces panels) or `sensor` (read-only device). |
| `Models` | The USB IDs (VID/PID) and model names the driver handles. Each ID belongs to exactly one driver. |
| `Transport` | `hid` (HID class driver) or `winusb` (bulk transfers). |
| `Experimental` | True for drivers not confirmed on real hardware (see below). |
| `Caps` | What the driver implements: frame size, round or not, rotation mode (`device`, `software`, `none`) and angles, raw brightness maximum, overlay layer, device-side video. |
| `Order` | Discovery order. |
| `Enumerate` | Lists the system paths of present devices **without opening or writing to them**. CaseHUD calls it every few seconds to notice plug and unplug. |
| `Discover` / `DiscoverSensors` | Opens and initialises every present device and returns panels or sensor devices. |

`panel.Drivers()` returns the registry, `panel.Discover()` runs the enabled display drivers,
`panel.DiscoverSensors()` the enabled sensor drivers and `panel.Fingerprint()` the enumerations.

## What crosses the boundary

A driver is deliberately small. Everything that crosses between CaseHUD and a driver is plain data:

- **Frames in:** `ShowJPEG([]byte)` takes an encoded picture. Drivers that can take a bitmap directly
  also implement `ShowImage(image.Image)` (an optimisation for the host; a plugin would only need the JPEG path).
- **Settings in:** `SetBrightness(0..100)` and `SetRotation(0/90/180/270)`. The driver scales and maps these
  to what its device understands.
- **Status out:** `ID()`, `Name()`, `Size()`, `MaxJPEG()`, optional `MaxFPS()`, `Round()`, `DriverID()`
  and, for devices that measure something, `Sensors()` returning readings (key, path, value, unit).
- **Lifecycle:** `Close()` leaves the device in a defined state and releases it.

No driver type, handle or internal structure is exposed to the rest of the program; a plugin host would only
need to carry these calls and the registry record.

## Experimental drivers

Drivers marked `Experimental` have not been confirmed on real hardware. They are **only discovered when the
user switched on "Experimental devices"** (Settings → Devices; default off). Each experimental driver can also be
switched off individually. The setting is stored in `config.json` (`experimental_devices`, `experimental_off`).
It can only be changed through the settings page (page token); the client token of API v1 cannot change it.

Panels of experimental drivers carry an "experimental" badge on their card. Please report what worked and what
did not with the hardware test report form on GitHub.

## Rules every driver follows

1. **Display first, few mode writes.** Drivers write pictures, brightness and rotation. They never write
   firmware, stored images or boot logos, serial numbers, configuration files, reboot or monitor-mode
   commands, and never reset a USB port. A deny-list test (`internal/panel/denylist_test.go`) runs every
   display driver (start-up, frames, brightness, rotation, close) against a recording fake and fails when a
   command outside the documented allow-list is written, or when a display driver has no such test. A few
   documented commands do change a mode or a setting the device may keep; they are listed under
   "Writes that change device settings" below.
2. **Other controls stay alone.** Pump, fan and lighting commands are not sent by the display drivers
   (lighting has its own engine).
3. **Bounded input.** Every length announced by a device is checked against the bytes actually received.
   Malformed or truncated replies are ignored or reported as errors, never trusted.
4. **Bounded waiting.** Every read has a timeout; a device that stops answering is dropped and reopened
   later with a growing back-off.
5. **Serialised I/O.** One goroutine at a time talks to a device (or a shared chain of devices).

## Writes that change device settings

These are all the writes of the display drivers that go beyond pictures and that may change a mode or a
setting the device keeps. Nothing else of that kind is sent.

| Driver | Write | When | Why |
|---|---|---|---|
| `tllcd` | Version query (op 61) and status query (op 60, read only) | op 61: first in every session, after every upload (start image, also an aborted one) and when the check below finds the fan stuck; op 60: after the first frame and then every 30 s while streaming (every 5 s after a failed check) | op 61 ends an open transfer (see "Transfer lock" below), op 60 shows whether the fan follows the stream. Neither stores anything |
| `tllcd` | Settings (op 64): brightness, frame rate, rotation, view live | start (with the values last written to this fan, so they match what it stores; the defaults 50 / 0 only when none are recorded), brightness/rotation change to a different value | needed to show host frames |
| `tllcd` | Settings (op 64) with view "stored image", the current brightness (50 if it is 0), normal rate, the current rotation | Close | hands the fan back to the image stored in it; without it the fan goes dark. Brightness and rotation stay as they are, so closing changes nothing the fan stores |
| `tllcd` | Start image (op 0x48): 400×400 JPEG of the CaseHUD logo, every report acknowledged by the fan, then the version query (op 61), then the normal streaming settings again | once per fan and image: on the first start after the update and when a new fan appears, only when the hash recorded for that fan (`tl_start_images` in `config.json`) is missing or differs; at most one attempt per fan and run; never before the disclaimer is accepted; switchable (Settings, Devices) and a button writes it again after a confirmation | permanent write (flash): the picture the fan shows when CaseHUD is not running. The chain is held for the whole transfer; a missing or wrong acknowledgement aborts it (no retry loop) |
| `tllcd` | Stored picture (op 0x41, slot 1): the same logo JPEG, every report acknowledged, then the version query (op 61), then the normal streaming settings again | together with the start image, with its own record (`tl_rest_images` in `config.json`): only when the hash recorded for that fan is missing or differs; at most one attempt per fan and run; never before the disclaimer is accepted; same switch; the "write again" button writes both | permanent write (flash): the picture the fan shows after the host stops streaming (the start image is only shown briefly at power-on). Close switches to it (op 64, view "stored", current rotation); a fan without a stored picture answers with status 0x0C and stays as it is (logged) |
| `tllcd` | Serial number (op 0x3F): 32 bytes, the first 32 characters of a random lower-case UUID, then the identity (op 62) is read again and compared | only if the user switched it on (Settings, Devices; off by default for new installs): when a fan is opened and its serial number is empty (32 zero bytes); never when it has one; one attempt per device and run | permanent write: gives displays without a serial number a unique one so they can be told apart; panel IDs do not change |
| `us88`, `us92`, `us207`, `slv3`, `flex` | Backlight (0x0E) | start, brightness change; not when the device already holds the value | brightness |
| `us88`, `us92`, `us207` | Halt video playback (0x7B), set the device time (0x33 with flag 2: the time only), hide built-in clock (0x34) | every connect (and so after standby, which rescans) | the built-in clock would cover host frames; the time keeps the device clock right. Flag 2 stores nothing. 0x34 is skipped while the offline clock is switched on |
| `us88`, `us92`, `us207` | Offline clock flag (0x33 with flag 1 or 0, the current time), then 0x34 when switched on | **only when the user flips "Show the clock when CaseHUD is not running"** (display settings, off by default); changes within 1.5 s are collapsed; not written when the recorded value (`device_written`, key `oc`) already equals it, and an off that was never preceded by an on writes nothing; start-up, scenes and rescans never write it | permanent write (flash): the display shows its own clock when no host feeds it. If another program set the flag, switch it on and off once in CaseHUD to clear it |
| `us88`, `us92`, `us207` | Picture (0x65) once and overlay PNG (0x66) on change ("layer mode"); frame rate (0x0F) before an overlay while a video plays | during operation, for designs with a plain background | nothing is stored. The picture goes out once per colour and size and after a reconnect; the overlay (full frame, 480×1920 with alpha, at most 1 MiB) when the values change, at most 5 times per second, unchanged ones once per second as keep-alive. A frame that needs a plain picture (animation, transition) clears the overlay first. Switch off with `no_layers` in `config.json` |
| `us88`, `us92`, `us207` | Video stream: frame rate (0x0F), blocks (0x79, at most 202752 bytes, length and last-block flag in the header, 30 ms apart, queue polled with 0x7A while above 3 until at most 2), halt (0x7B) | while a display shows a video file or a video background and the option "Play video on the display" is on (default, needs ffmpeg) | playback state only, nothing is stored. The file is encoded once by the user's ffmpeg (baseline, no B-frames, device orientation, at most 60 s and 64 MiB) into `devvideo/` of the data folder (kept below 512 MiB). Until it is ready, or if it fails, frames are sent as before |
| `us88` | Device rotation 0 (0x0D) | connect, unless the device already holds turn 0 | host rotates in software; resets a rotation another program stored |
| `us88`, `us92` | Device rotation 2 (0x0D) for rotation 180 | **never by default**: only when the user flips "Device-side rotation (test)" (display settings, off by default, `device_flip` in `config.json`); written when the switch changes or the rotation changes to or from 180, not when the device already holds the value | would let the device turn host pictures by 180° instead of mirroring them in software. It is not known whether the device applies this to host pictures or only to its own drawings (the clock), so software rotation stays the default |
| `slv3`, `flex` | Device rotation (0x0D), frame-rate hint (0x0F, old SL V3 LCD only) | start, rotation change | device-side rotation |
| `flex` | Hand over to the firmware's own theme (0x7F) | Close | mode switch |
| `aio` | LCD control (0x0C): brightness, only when changed | brightness change | brightness |
| `aio` | Reset (0x8E) | only when the watchdog finds the screen unavailable, at most 3 times per device, each one logged | recovers a stuck screen; automatic and experimental |

### Transfer lock (`tllcd`)

Uploads (the start image, op 0x48, and the stored picture, op 0x41) occupy a transfer slot in the fan. While it is occupied the fan ignores
every other command, settings (op 64) and the live stream (op 0x46, which is never answered, so nothing
reports the loss) included. Neither a timeout nor a mode change frees it; only the version query (op 61, two
answers) does. The driver therefore sends op 61 first in every session (before identity and settings), right
after every upload whether it succeeded or was aborted (before the settings and the stream resume), and
whenever the stream check below fails. While streaming, the status (op 60) is read once after the first frame
and every 30 s: if its byte 11 is not 4, the fan is freed (op 61), the settings are written again, the frame is
sent again and the status is read once more; if that fails too, the panel reports an error and the check is
repeated after 5 s. A fan that never answers op 60 is not checked. All of this runs while the chain is held.

### Dedupe and coalescing of settings writes

Brightness and rotation may live in the device's flash, which survives a limited number of writes (the UNI FAN
TL LCD and the 8.8" bars are treated as storing them; the TL fan stores a value when it differs from the
stored one; the picture stream never writes flash). Three layers keep these writes rare:

1. **GUI:** sliders and rotation buttons never send intermediate values. The value goes out on release (or
   700 ms after the last change); the preview may follow live.
2. **Host (`internal/app/devwrite.go`):** per panel at most one settings write per second; requests in
   between are collapsed and the last value wins. A value equal to the last one sent is not sent again. This
   also holds for API v1 `PATCH /panels/{id}`, scenes and rotation. Pending writes are dropped when the panel
   is closed.
3. **Driver and record (`internal/panel/settingscache.go`):** the last brightness and rotation written to each
   fan or bar is kept in `config.json` (`device_written`) and survives restarts. A driver starts from these
   values, so start-up, rescan and re-plug do not write values the device already holds (the `tllcd` start
   still sends the view switch, with the recorded values). Without a record the values are written once.
   `tllcd` shows black frames for screen-off instead of setting brightness 0, so a screen saver does not
   change the stored brightness.

The fan LED controller stores every light command in flash: its settings (mode, effect, colors, speed,
`interval`) are sent one second after the last change in the GUI, only when the device configuration actually
changed, and canvas updates are limited to one per `interval` (30..3600 s, default 60) and port.

## Built-in drivers

| ID | Devices | Transport | Status |
|---|---|---|---|
| `tllcd` | UNI FAN TL LCD (04FC:7393), 400×400 round | HID | confirmed |
| `us88` | Universal Screen 8.8" (1CBE:A088), 1920×480 | WinUSB | confirmed |
| `aio` | HydroShift LCD (0416:7398/7399/739A), Galahad II LCD (0416:7391), Galahad II LCD V2 (0416:7395), 480×480 | HID | experimental |
| `us92` | Vision-M 9.2" (1CBE:A092), 1920×464 (native 464×1920) | WinUSB | experimental |
| `us207` | Lancool 207 Digital (1CBE:A065), 1472×720 (native 720×1472) | WinUSB | experimental |
| `slv3` | SL V3 LCD (1CBE:0005), TL V2 LCD (1CBE:0006), 400×400 round | WinUSB | experimental |
| `flex` | TL Flex LCD (1CBE:A018), SL INF Flex LCD (1CBE:A019), 400×400 | WinUSB | experimental |
| `edgehub` | EdgeHub Advanced (39B0:0101), temperature sensors | HID | experimental |

### Universal-screen family (`us88`, `us92`, `us207`, `flex`, `slv3`)

These share one implementation (`universal88.go`, model table in `usmodels.go`). Every command is a 512-byte
block, optionally followed by data in the same transfer. Differences between the models:

- **Bars (8.8", 9.2", 207):** landscape frames are turned a quarter turn into the device's portrait frame and
  sent as JPEG (up to 512000 bytes). Rotation is done in software. Brightness: 8.8" 0..50, 9.2" and 207 0..100.
  The 8.8" rewrites its rotation to 0 at start; the 9.2" and 207 do not (the value may be stored by the device).
  Bars also take a picture and a PNG overlay as separate layers (`us88layers.go`, the "layer mode" above) and
  play an H.264 file themselves; both are chosen by the engine (`internal/app/layers.go`) per source, never by
  the driver alone. Commands the device documents as dangerous (boot image, configuration erase, monitor mode,
  reboot, firmware, file write) are not part of the driver; the deny-list test covers the new paths too.
- **Flex fan screens:** 400×400, same framing as the 8.8". The device is asked whether it is online; a device
  that does not answer is treated as absent. Rotation goes to the device (default turn one quarter, as the
  panels sit turned in the fan). When CaseHUD quits it hands the display back to the device's own theme (a mode
  switch).
- **SL V3 / TL V2 LCD:** different framing: a 504-byte plaintext becomes a 512-byte ciphertext without trailer;
  a command with data is always one transfer of exactly 102400 bytes, so JPEGs are limited to 100000 bytes.
  Rotation goes to the device (wire value = quarter turns + 1), brightness follows a curve, a keep-alive
  query is sent after a second of silence and the rotation is sent again after the display was dark.
  Replies arrive asynchronously, so frames are not acknowledged.
- The chain position of fan screens comes from the USB location path (port on the parent hub, counted from 0);
  panels are identified by serial number, so a screen keeps its ID when it moves.

### Pump-head screens (`aio`)

HID, three report layouts on one handle. Pictures are sent as JPEG. After start the firmware is queried:
at or above 1.2 (HydroShift) or 1.5 (Galahad II) the 512-byte layout is used and a watchdog asks every two
seconds whether the screen is alive (at most three resets, then it leaves the device alone); older firmware uses
the 1024-byte layout without watchdog. HydroShift waits 10 seconds after enumeration before the first command.
Brightness is only written when it is changed, rotation is done in software, the default brightness is 50.
Coolant temperature, pump speed and fan speed are read from the status answer and appear as sensors
(`<model>/Coolant`, `/Pump`, `/Fan`).

**Open point: H.264.** These devices are known to take an H.264 stream; whether the firmware also accepts the
JPEG command used here is not confirmed, which is why these models are experimental. Supporting H.264 needs
an encoder (ffmpeg on the user's PC or a future built-in encoder); the packet layout for it is the same as for
JPEG with the stream command instead of the picture command.

### EdgeHub Advanced (`edgehub`)

A read-only sensor device. It asks for the temperatures once per second (500 ms timeout, reopen after three
failures in a row) and provides `EdgeHub/GPU1`, `EdgeHub/GPU2` and `EdgeHub/Ambient`. A sensor that the device
flags as missing or abnormal is left out. Only the read commands (temperatures, product info, serial number)
can be sent; the code refuses every other command, in particular the firmware update.

## Adding a driver

1. Implement `panel.Panel` (and optionally `SensorSource`) or `panel.SensorDevice`.
2. Register a `Driver` record in an `init` function. Mark it `Experimental` until someone confirmed it on hardware.
3. Add tests with a fake device: start-up sequence, framing, rotation/brightness mapping, a deny-list of
   commands that must never be written, malformed replies.
