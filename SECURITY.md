# Sicherheit / Security

## Deutsch

### Schwachstellen melden

Bitte **nicht** als öffentliches Issue, sondern vertraulich über das Formular:
[Sicherheitslücke melden](https://github.com/kewe87/CaseHUD/security/advisories/new)
(GitHub, Reiter Security, Report a vulnerability). Es gibt keine E-Mail-Adresse für Meldungen.

### Unterstützte Versionen

| Version | Unterstützt |
|---|---|
| 1.0.x | ja |

### Im Geltungsbereich

- die lokale API auf 127.0.0.1 (`/api/v1`, siehe [docs/API.md](docs/API.md))
- der Sensor-Dienst `CaseHUDSensors` (läuft als SYSTEM) und die SMBus-Brücke ([docs/SMBUS-BRIDGE.md](docs/SMBUS-BRIDGE.md))
- der Installer

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
| 1.0.x | yes |

### In scope

- the local API on 127.0.0.1 (`/api/v1`, see [docs/API.md](docs/API.md))
- the `CaseHUDSensors` sensor service (runs as SYSTEM) and the SMBus bridge ([docs/SMBUS-BRIDGE.md](docs/SMBUS-BRIDGE.md))
- the installer

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
