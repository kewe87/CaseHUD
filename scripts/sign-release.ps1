# Signs dist\SHA256SUMS.txt for clients that install CaseHUD unattended.
#   $env:RELEASE_SIGNING_KEY = <PKCS#8 PEM>; pwsh scripts/sign-release.ps1
#   pwsh scripts/sign-release.ps1 -Verify   # checks against the public key
# Signature: ECDSA P-256 / SHA-256 over "casehud-release-v1`n" + the file
# bytes, ASN.1 DER, base64, written to dist\SHA256SUMS.txt.sig.
param([switch]$Verify)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$sums = Join-Path $root "dist\SHA256SUMS.txt"
$sig = "$sums.sig"
$context = [Text.Encoding]::ASCII.GetBytes("casehud-release-v1`n")
$data = $context + [IO.File]::ReadAllBytes($sums)
$der = [Security.Cryptography.DSASignatureFormat]::Rfc3279DerSequence
$sha = [Security.Cryptography.HashAlgorithmName]::SHA256
$ec = [Security.Cryptography.ECDsa]::Create()
if ($Verify) {
  $ec.ImportFromPem((Get-Content -Raw (Join-Path $root "docs\release-signing.pub.pem")))
  $ok = $ec.VerifyData($data, [Convert]::FromBase64String((Get-Content -Raw $sig).Trim()), $sha, $der)
  if (-not $ok) { throw "SHA256SUMS.txt.sig does not verify" }
  "signature OK"
  return
}
if (-not $env:RELEASE_SIGNING_KEY) { throw "RELEASE_SIGNING_KEY is not set" }
$ec.ImportFromPem($env:RELEASE_SIGNING_KEY)
$s = [Convert]::ToBase64String($ec.SignData($data, $sha, $der))
[IO.File]::WriteAllText($sig, "$s`n", [Text.Encoding]::ASCII)
"signed $sums"
