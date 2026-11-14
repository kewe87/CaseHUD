# Sicherheit / Security

## Deutsch

### Schwachstellen melden

Bitte **nicht** als öffentliches Issue, sondern vertraulich über das Formular:
[Sicherheitslücke melden](https://github.com/kewe87/CaseHUD/security/advisories/new)
(GitHub, Reiter Security, Report a vulnerability). Es gibt keine E-Mail-Adresse für Meldungen.

### Unterstützte Versionen

| Version | Unterstützt |
|---|---|
| 1.2.x | ja |
| 1.0.x | ja |

### Im Geltungsbereich

- die lokale API auf 127.0.0.1 (`/api/v1`, siehe [docs/API.md](docs/API.md))
- der Sensor-Dienst `CaseHUDSensors` (läuft als SYSTEM) und die SMBus-Brücke ([docs/SMBUS-BRIDGE.md](docs/SMBUS-BRIDGE.md))
- die Kühlung ab 1.2: Pipe `\\.\pipe\CaseHUD.Cooling`, Wächter und Freigabe-Datei (siehe unten)
- der Installer

### Kühlung (ab 1.2)

- **Pipe `CaseHUD.Cooling`:** Der Sensor-Dienst nimmt darüber nur eine Anfrage an: einen Mainboard-Lüfteranschluss mit mindestens 30 % halten.
  Er bedient nur `CaseHUD.exe` und `CaseHUD-Guard.exe` aus dem eigenen Installationsordner (Prüfung des Programmabbilds jedes Clients).
  Die Zugriffsrechte der Pipe selbst lassen alle interaktiven Benutzer zu; die Abbild-Prüfung ist die Einschränkung. Bleibt der Dienst
  ohne Antwort oder bricht ein Halt ab, übernimmt wieder das Mainboard.
- **Freigabe-Datei:** Ein Anschluss wird nur gehalten, wenn er in `%ProgramData%\CaseHUD\sensors.json` (`coolingHeaders`) steht. Die Datei und
  der Ordner müssen Administratoren gehören und dürfen für andere nicht beschreibbar sein. CaseHUD schreibt den Eintrag nur nach einer
  UAC-Bestätigung in einem erhöht gestarteten Prozess (`--allow-cooling-header`). Ein Widerruf geschieht durch Bearbeiten der Datei als Administrator.
- **Wächter:** Ein zweites Programm (`CaseHUD-Guard.exe` im Installationsordner, bei portablem Betrieb eine Kopie im Datenordner) beobachtet CaseHUD.
  Endet CaseHUD unerwartet, setzt es die Lüfter am Controller auf ihre sichere Drehzahl. Es erhält seine Angaben über die Standardeingabe vom startenden Prozess und kann nur die Lüfter am Controller setzen.
  CaseHUD beendet die Regelung bei einer Sitzungsende-Nachricht nur, wenn das System das Herunterfahren der Sitzung meldet; eine Nachricht eines anderen Programms wird ignoriert. Hält die Regelung an, während CaseHUD weiterläuft, löst CaseHUD den Alarm `halted` aus.
- **Sensorquelle:** Lüfter und Pumpe werden nur nach Messwerten des Dienstes auf `127.0.0.1:8086` geregelt. Die Quelle gilt nur, wenn der Dienst läuft
  und der Listener zum Kernel (http.sys) gehört. Andere Quellen dienen nur der Anzeige. Der Dienst antwortet nur auf `127.0.0.1:8086` und `localhost:8086`.
- Die Pumpe läuft nie unter 30 %; Gehäuselüfter laufen immer.

### Vertrauensgrenze: derselbe Windows-Benutzer

CaseHUD schützt vor Webseiten und vor Programmen anderer Benutzer, nicht vor einem Programm, das als derselbe Windows-Benutzer läuft.
Ein solches Programm (auch ein KI-Agent mit Shell) kann `instance.url` (Token des Fensters) und `api.json` (Standard-Zugang mit Steuern und System)
lesen und tut dann alles, was der Benutzer in CaseHUD tun kann. Die Sperren für MCP-Zugänge (kein Beenden, Update, Fenster, Ereignisstrom;
der Server setzt sie durch) schützen den MCP-Weg, nicht eine Shell desselben Agenten. Ein KI-Assistent, der Befehle als Benutzer ausführen darf,
gilt als vertrauenswürdig für alles, was CaseHUD kann. Das Token-Recht „System“ gibt es nur zusammen mit „Steuern“.
Siehe [docs/API.md](docs/API.md#mcp).

### Releases prüfen

Jedem Release liegen `SHA256SUMS.txt` und `SHA256SUMS.txt.sig` bei.
Die Signatur ist ECDSA P-256 / SHA-256 (DER, base64) über die Bytes `casehud-release-v1\n` gefolgt von
den exakten Bytes von `SHA256SUMS.txt`. Öffentlicher Schlüssel: [docs/release-signing.pub.pem](docs/release-signing.pub.pem),
SHA-256 des SubjectPublicKeyInfo-DER:

```
cfb6863f23c892a53c93786dd1ee78478618aed680c5603fbc3955c1dde4abaf
```

Erst die Signatur prüfen, dann den Hash der Setup-Datei mit der Zeile in `SHA256SUMS.txt` vergleichen
(siehe auch [docs/API.md](docs/API.md)). Das Beispiel braucht **PowerShell 7** (`pwsh`), nicht die Windows PowerShell 5.1:

```powershell
$key  = [System.Security.Cryptography.ECDsa]::Create()
$key.ImportFromPem((Get-Content docs\release-signing.pub.pem -Raw))
$data = [byte[]]([Text.Encoding]::ASCII.GetBytes("casehud-release-v1`n") + [IO.File]::ReadAllBytes("$PWD\SHA256SUMS.txt"))
$sig  = [Convert]::FromBase64String((Get-Content SHA256SUMS.txt.sig -Raw).Trim())
$key.VerifyData($data, $sig, [Security.Cryptography.HashAlgorithmName]::SHA256,
  [Security.Cryptography.DSASignatureFormat]::Rfc3279DerSequence)   # True = gültig
(Get-FileHash CaseHUD-Setup-x.y.z.exe -Algorithm SHA256).Hash       # mit SHA256SUMS.txt vergleichen
```

Die Programme sind noch **nicht code-signiert** (kein Authenticode-Zertifikat). Windows SmartScreen kann deshalb warnen;
die Prüfsummen-Signatur oben ist der Weg, die Echtheit zu prüfen.

### Netzwerkzugriffe

Welche Verbindungen CaseHUD aufbaut, steht in der README unter [Netzwerk und Datenschutz](README.md#netzwerk-und-datenschutz).

## English

### Reporting a vulnerability

Please do **not** open a public issue. Use the private form instead:
[Report a vulnerability](https://github.com/kewe87/CaseHUD/security/advisories/new)
(GitHub, Security tab, Report a vulnerability). There is no email address for reports.

### Supported versions

| Version | Supported |
|---|---|
| 1.2.x | yes |
| 1.0.x | yes |

### In scope

- the local API on 127.0.0.1 (`/api/v1`, see [docs/API.md](docs/API.md))
- the `CaseHUDSensors` sensor service (runs as SYSTEM) and the SMBus bridge ([docs/SMBUS-BRIDGE.md](docs/SMBUS-BRIDGE.md))
- the cooling control since 1.2: the `\\.\pipe\CaseHUD.Cooling` pipe, the guard and the release file (see below)
- the installer

### Cooling (since 1.2)

- **Pipe `CaseHUD.Cooling`:** The sensor service accepts one request on it: hold a mainboard fan header at 30 % or more.
  It serves only `CaseHUD.exe` and `CaseHUD-Guard.exe` from its own installation folder (the program image of every client is checked).
  The pipe's access rights admit all interactive users; the image check is the restriction. If the service stops answering or a hold ends,
  the mainboard takes the header back.
- **Release file:** A header is held only if it is listed in `%ProgramData%\CaseHUD\sensors.json` (`coolingHeaders`). The file and its folder
  must be owned by administrators and not writable by others. CaseHUD writes the entry only after a UAC confirmation, in an elevated
  process (`--allow-cooling-header`). To revoke a release, edit the file as an administrator.
- **Guard:** A second program (`CaseHUD-Guard.exe` in the installation folder, a copy in the data folder when run portable) watches CaseHUD.
  If CaseHUD ends unexpectedly, it sets the fans on the controller to their safe speed. It receives its data on standard input from the process that started it and can only set the fans on the controller.
  CaseHUD ends the control on a session-end message only if the system reports that the session is shutting down; a message from another program is ignored. If the control stops while CaseHUD keeps running, CaseHUD raises the alarm `halted`.
- **Sensor source:** Fans and pump are regulated only from readings of the service on `127.0.0.1:8086`. The source counts only while the service runs
  and the listener belongs to the kernel (http.sys). Other sources are for display only. The service answers only requests to `127.0.0.1:8086` and `localhost:8086`.
- The pump never runs below 30 %; case fans always run.

### Trust limit: the same Windows user

CaseHUD protects against web pages and against programs of other users, not against a program that runs as the same Windows user.
Such a program (an AI agent with a shell included) can read `instance.url` (the window's token) and `api.json` (the standard access with
control and system) and then does whatever the user can do in CaseHUD. The restrictions on MCP accesses (no quit, update, window or event
stream; the server enforces them) guard the MCP path, not a shell the same agent may have. An AI assistant that may run commands as the
user counts as trusted with everything CaseHUD can do. The token right "system" only exists together with "control".
See [docs/API.md](docs/API.md#mcp).

### Verifying releases

Every release ships `SHA256SUMS.txt` and `SHA256SUMS.txt.sig`.
The signature is ECDSA P-256 / SHA-256 (DER, base64) over the bytes `casehud-release-v1\n` followed by the
exact bytes of `SHA256SUMS.txt`. Public key: [docs/release-signing.pub.pem](docs/release-signing.pub.pem),
SHA-256 of the SubjectPublicKeyInfo DER:

```
cfb6863f23c892a53c93786dd1ee78478618aed680c5603fbc3955c1dde4abaf
```

Verify the signature first, then compare the setup's hash with its line in `SHA256SUMS.txt`.
The PowerShell snippet above works as is (see also [docs/API.md](docs/API.md)); it needs **PowerShell 7** (`pwsh`), not Windows PowerShell 5.1.

The executables are **not code-signed** yet (no Authenticode certificate), so Windows SmartScreen may warn.
The checksum signature above is how you verify authenticity.

### Network access

See "Network and privacy" in the [README](README.md#network-and-privacy) for every connection CaseHUD makes.
