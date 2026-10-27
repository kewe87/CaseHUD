# Builds the Windows package: portable zip + setup.
#   pwsh scripts/package-windows.ps1 -Version 1.0.0
# Needs Go, the .NET 10 SDK (builds the net472 sensor service) and Inno Setup 6
# (installed via Chocolatey if missing). Runs on GitHub's windows runners.
param(
  [string]$Version = "0.0.0-dev",
  # LibreHardwareMonitor source for the sensor service. The fork carries the
  # T Sensor fix for ASUS ROG STRIX X870/X870E until it is merged upstream.
  [string]$LhmRepo = "https://github.com/kewe87/librehardwaremonitor",
  # Pinned commit (branch asus-strix-x870-t-sensor): the service runs as
  # SYSTEM, so the build must not pick up whatever a branch points to later.
  [string]$LhmCommit = "189859f312e8f10cf15891f14206963bee635482",
  # PresentMon is bundled at a fixed version and verified by hash and signature.
  [string]$PresentMonVersion = "2.6.0",
  [string]$PresentMonSha256 = "b2a706bc6ad475749e3b7e3409263aa1e6906d45bdcf993f6dbc0f660188f1af",
  [string]$InnoVersion = "6.7.1"
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
$stage = Join-Path $root "stage"
Remove-Item -Recurse -Force $stage, (Join-Path $root "dist") -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $stage, (Join-Path $root "dist"), (Join-Path $stage "sensors") | Out-Null

function Check($what) { if ($LASTEXITCODE -ne 0) { throw "$what failed ($LASTEXITCODE)" } }

# Executables shipped to users (and the driver installer) must carry a valid
# Authenticode signature; the build fails otherwise.
function Assert-Signed($path, $what) {
  $sig = Get-AuthenticodeSignature $path
  if ($sig.Status -ne "Valid") { throw "$what is not validly signed ($($sig.Status))" }
  "verified $what, signed by $($sig.SignerCertificate.Subject)"
}

# --- Go binaries ---------------------------------------------------------
$env:CGO_ENABLED = "0"
$ld = "-s -w -X main.version=$Version"
go run ./cmd/casehud write-icon installer\casehud.ico; Check "icon"
# Icon, version info and manifest as Windows resources (.syso, picked up by
# go build), so Explorer, the taskbar and shortcuts show the CaseHUD logo.
$fileVersion = ($Version -split '[-+]')[0]
foreach ($t in @(@{ Dir = "cmd\casehud"; Exe = "casehud-cli.exe"; Manifest = "cli" }, @{ Dir = "cmd\casehud-gui"; Exe = "CaseHUD.exe"; Manifest = "gui" })) {
  go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --out "$($t.Dir)\rsrc" --manifest $t.Manifest `
    --icon installer\casehud.ico --product-name CaseHUD --file-description CaseHUD --copyright "Copyright (C) 2026 kewe87" `
    --original-filename $t.Exe --file-version $fileVersion --product-version $fileVersion; Check "resources $($t.Exe)"
}
go build -trimpath -ldflags $ld -o "$stage\casehud-cli.exe" ./cmd/casehud; Check "casehud build"
go build -trimpath -ldflags "$ld -H windowsgui" -o "$stage\CaseHUD.exe" ./cmd/casehud-gui; Check "CaseHUD build"

# --- Sensor service (LibreHardwareMonitorLib bundled) ---------------------
$lhm = Join-Path $root "third_party\lhm"
if (-not (Test-Path $lhm)) {
  git init -q $lhm; Check "LHM init"
  git -C $lhm fetch -q --depth 1 $LhmRepo $LhmCommit; Check "LHM fetch"
  git -C $lhm checkout -q FETCH_HEAD; Check "LHM checkout"
}
if ((git -C $lhm rev-parse HEAD) -ne $LhmCommit) { throw "LHM checkout is not the pinned commit $LhmCommit" }
dotnet build sensors\CaseHUD.Sensors\CaseHUD.Sensors.csproj -c Release -p:Platform=x64 "-p:LhmSource=$lhm" -o "$stage\sensors"; Check "sensor service build"
Remove-Item "$stage\sensors\*.pdb", "$stage\sensors\*.xml" -ErrorAction SilentlyContinue
$pawn = "$lhm\LibreHardwareMonitor.Windows.Forms\Resources\PawnIO_setup.exe"
if (Test-Path $pawn) { Assert-Signed $pawn "PawnIO_setup.exe"; Copy-Item $pawn $stage } else { Write-Warning "PawnIO_setup.exe not found" }

# --- PresentMon (Intel, MIT) -----------------------------------------------
$pmUrl = "https://github.com/GameTechDev/PresentMon/releases/download/v$PresentMonVersion/PresentMon-$PresentMonVersion-x64.exe"
Invoke-WebRequest -Headers @{ "User-Agent" = "casehud-build" } $pmUrl -OutFile "$stage\PresentMon.exe"
$pmHash = (Get-FileHash "$stage\PresentMon.exe" -Algorithm SHA256).Hash.ToLower()
if ($pmHash -ne $PresentMonSha256) { throw "PresentMon hash mismatch: $pmHash" }
Assert-Signed "$stage\PresentMon.exe" "PresentMon $PresentMonVersion"

Copy-Item LICENSE, THIRD_PARTY_NOTICES.md, README.md $stage

# --- Portable zip + setup -------------------------------------------------
Compress-Archive -Path "$stage\*" -DestinationPath "dist\CaseHUD-$Version-windows-x64-portable.zip" -Force
$iscc = Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"
# winget installs Inno Setup per user.
$isccUser = Join-Path $env:LOCALAPPDATA "Programs\Inno Setup 6\ISCC.exe"
if (-not (Test-Path $iscc) -and (Test-Path $isccUser)) { $iscc = $isccUser }
if (-not (Test-Path $iscc)) { choco install innosetup --version $InnoVersion -y --no-progress; Check "Inno Setup install" }
& $iscc "/DAppVersion=$Version" "/DSourceDir=$stage" installer\CaseHUD.iss; Check "setup build"
# Checksums so downloads can be verified, in sha256sum format with LF line
# endings (sha256sum -c and signatures over the file bytes rely on that).
$sums = Get-ChildItem dist -File | Where-Object Name -ne "SHA256SUMS.txt" | ForEach-Object {
  "{0}  {1}`n" -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name
}
[IO.File]::WriteAllText((Join-Path $root "dist\SHA256SUMS.txt"), (-join $sums), [Text.Encoding]::ASCII)
Get-ChildItem dist
