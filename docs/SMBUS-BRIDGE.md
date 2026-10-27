# SMBus bridge (RAM RGB)

The sensor service (`CaseHUDSensors`, runs as SYSTEM) can offer a narrowly scoped SMBus
bridge for ENE RAM RGB controllers. It lets an unprivileged client (CaseHUD LEDs) talk to
the sticks without loading a driver itself. The bridge is **off by default**.

It uses the PawnIO driver with the embedded `SmbusPIIX4` module and takes the global
mutex `Global\Access_SMBUS.HTP.Method`, so accesses never overlap with the sensor reads.

## Enabling and disabling (kill switch)

Create `C:\ProgramData\CaseHUD\sensors.json` **as an administrator**:

```json
{ "smbusBridge": true }
```

- Missing file, missing key or any value other than `true` means off, and the pipe does not exist.
- The service re-reads the file every 10 s. Changing it opens or closes the pipe (an open
  connection is dropped) without a restart. Both changes are logged to `sensors.log` (next to the service, `C:\Program Files\CaseHUD\sensors\sensors.log`).
- Older installs (and manual setups) leave `ProgramData\CaseHUD` open for normal users to add files (the installer makes it read-only for them), so the service honors the file only if
  it is owned by SYSTEM/Administrators and no other account has write access to it. Otherwise
  it is ignored (and the reason is logged once per change). Files created from an elevated
  prompt qualify. The directory must also not be a junction/reparse point, must be owned by an
  administrator account, and must not let other accounts delete files in it, change its
  permissions or take it over (the file must not be a reparse point either).

## Pipe

`\\.\pipe\CaseHUD.Smbus`, byte mode. Access: SYSTEM, Administrators and interactive users
(S-1-5-4). Network logons (S-1-5-2) and anonymous clients are denied; the pipe is also
created with `PIPE_REJECT_REMOTE_CLIENTS`.

- First instance: the pipe is created with `FILE_FLAG_FIRST_PIPE_INSTANCE`. If another process
  already owns the name, the service logs it, stays off and retries on the next 10 s poll.
- Connections: at most 8 at once, at most 2 per client process (`GetNamedPipeClientProcessId`).
  Extra connections are closed right away; the pipe itself keeps accepting.
- Idle timeout: a connection that sends and receives nothing for 60 s is closed. The sweep runs
  on its own timer, so constant reconnects by other clients cannot keep an idle connection
  alive. Clients reconnect (the CaseHUD client already does).
- If the accept loop ever dies, the next poll restarts the pipe.

Every message is a `uint32` little-endian length followed by that many bytes of UTF-8 JSON.
One request gets one response; a connection can carry any number of batches. A message
longer than 64 KiB (or of length 0) closes the connection.

### Request

```json
{"v":1,"addr":113,"lockMs":10,"ops":[
  {"op":"rb","reg":4096},
  {"op":"wb","reg":32800,"val":1},
  {"op":"blk","reg":33024,"data":"AQIDBAU="},
  {"op":"rcmd","cmd":160}
]}
```

| Op | Meaning | SMBus transfers (ENE indirect access) |
|----|---------|----------------------------------------|
| `rb` | read ENE register | write word cmd `0x00` = register (bytes swapped), read byte cmd `0x81` |
| `wb` | write ENE register | write word cmd `0x00`, write byte cmd `0x01` |
| `blk` | block write (`data` is base64, 1..32 bytes) | write word cmd `0x00`, write block cmd `0x03` |
| `rcmd` | read byte data with command `cmd` (identification; sends the command byte first) | read byte cmd `cmd` |

Missing numeric fields count as 0. Anything else is refused with `denied`, and the whole
batch is refused before any transfer happens:

- `v` must be 1; `addr` 0x70..0x77; `lockMs` 1..50; 1..64 ops
- `reg` 0..0xFFFF, `val` 0..255, `rcmd` command 0xA0..0xAF, block data 1..32 bytes
- unknown ops (for example `ww`), wrong types, malformed JSON
- register allow-list (direct lighting only):
  - `wb`: registers `0x8020..0x8023` (direct, mode, speed, direction), and the apply registers
    `0x80A0` and `0x802F` **only with value 1**. Value `0xAA` at the apply register means "save
    to EEPROM" and is never allowed.
  - `blk`: the whole block inside `0x8000..0x801F` (v1 colors), `0x8100..0x813F` (v2 direct
    colors) or `0x8160..0x819F` (v2 effect colors)
  - `rb` may read any register; `rcmd` stays limited to `0xA0..0xAF`

### ENE verification

Addresses `0x70..0x77` can host other SMBus devices (for example PCA954x multiplexers), and
even selecting a register is a write to them.
`rcmd` is not free of writes either: an SMBus "read byte data" first sends the command byte to the
device and then reads, so at an address that is not yet verified it writes one command byte
(restricted to `0xA0..0xAF`). That is the minimum needed to identify an ENE controller and is the
only exception. So everything except `rcmd`
requires a verified ENE DRAM controller at that address. On the first non-`rcmd` request for
an address the bridge verifies it itself, inside the same mutex acquisition, the way ENE
detection does:

1. commands `0xA0..0xAF` must read back `0x00..0x0F` (at least 15 of 16), using only `rcmd` reads;
2. register `0x1000..0x100F` (device name) must be one of the known ENE names
   (`LED-0116`, `AUMA0-E8K4-0101`, `DIMM_LED-0102`, `DIMM_LED-0103`, `AUDA0-E6K5-0101`,
   `AUMA0-E6K5-0104`, `-0105`, `-0106`).

If either check fails the request is answered `denied` (logged once per address) and nothing
is written. A verified address is cached; any transfer error on that address drops it from the
cache, and it is verified again before the next register access.

### Response

```json
{"ok":true,"reads":[0,255,7]}
{"ok":false,"error":"timeout"}
```

`reads` holds one value (0..255) per `rb`/`rcmd` op, in op order. Errors:

- `denied`: the request broke a rule above (logged, at most one line per 10 s)
- `timeout`: the SMBus mutex was not free within `lockMs` (or could not be opened at all, which
  is logged once), another tool had selected a different PIIX4 port, or a rate limit was hit:
  120 batches per second per connection, 240 per second over all connections, and the bridge
  may hold the mutex for at most half of the time in any one-second window
- `io`: an SMBus transfer or the PawnIO driver failed; the batch stops at that point

The whole batch runs under one acquisition of the mutex, which is waited for at most
`lockMs`; the service never blocks longer.

## Tests

```
dotnet test sensors\CaseHUD.Sensors.Tests
```

The tests run against a fake bus and a fake mutex (no hardware, no driver, no service).

## The client checks the server

Anyone can create a pipe name that is free. Before CaseHUD sends a batch it therefore asks
Windows which process serves the pipe it just opened (`GetNamedPipeServerProcessId`). The
image of that process must be `sensors\CaseHUD.Sensors.exe` in the install folder of the
running CaseHUD and, when its token can be read, it must run as LocalSystem. Otherwise the
connection is closed without sending anything and CaseHUD reports "SMBus-Brücke nicht
vertrauenswürdig". A portable copy of CaseHUD (no installed service next to it) therefore
never uses the bridge.

## Portable build and ffmpeg

The portable build does not install anything. For video backgrounds and display videos it runs
`ffmpeg.exe` from the folder next to `CaseHUD.exe`, else the first one found on `PATH`. That
program is started with your user rights (it never gets more, and it only receives files from
CaseHUD's media folder). Put only a build of ffmpeg you trust there, and keep the folder
writable for administrators only if you do not want others to swap it.
