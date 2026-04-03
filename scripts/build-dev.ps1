param(
  [string]$BundleRoot = $env:LOADER_BUNDLE_ROOT
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$baseVersion = "1.1.3"
$bundlePrepareScript = $null
$bundleBinaryRoot = $null
if ($BundleRoot) {
  $bundleBinaryRoot = (Resolve-Path $BundleRoot).Path
  $bundlePrepareScript = Join-Path $bundleBinaryRoot "scripts\\prepare-bundle.ps1"
  if (-not (Test-Path $bundlePrepareScript)) {
    throw "Bundle prepare script not found: $bundlePrepareScript"
  }
}
$buildTime = Get-Date
$buildDate = $buildTime.ToString("yyMMddHHmmss")
$bytes = New-Object byte[] 4
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$devStamp = "dev+" + ([System.BitConverter]::ToString($bytes).Replace("-", "").ToLowerInvariant()) + "+" + $buildDate
$fingerprintBytes = New-Object byte[] 16
$fingerprintRng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$fingerprintRng.GetBytes($fingerprintBytes)
$fingerprintRng.Dispose()
$buildFingerprint = ([System.BitConverter]::ToString($fingerprintBytes).Replace("-", "").ToLowerInvariant())
$outputName = "HI3 Loader $baseVersion $devStamp"
$ldFlags = "-X hi3loader/internal/buildinfo.AppVersion=$baseVersion -X hi3loader/internal/buildinfo.BuildStamp=$devStamp -X hi3loader/internal/buildinfo.BuildFingerprint=$buildFingerprint"

Set-Location $root
$env:GOCACHE = Join-Path $root ".gocache-release"
$env:GOWORK = "off"

if ($bundlePrepareScript -and (Test-Path $bundlePrepareScript)) {
  & $bundlePrepareScript -PublicRoot $root
} else {
  Write-Host "No LOADER_BUNDLE_ROOT configured; building GUI without bundled module."
}

$wailsArgs = @(
  "run",
  "github.com/wailsapp/wails/v2/cmd/wails@v2.11.0",
  "build",
  "-ldflags",
  $ldFlags,
  "-clean",
  "-o",
  $outputName
)

Write-Host ("Building development package version {0} with stamp {1} and fingerprint {2}" -f $baseVersion, $devStamp, $buildFingerprint)
& go @wailsArgs

$binDir = Join-Path $root "build\\bin"
$plainOutput = Join-Path $binDir $outputName
$exeOutput = Join-Path $binDir ($outputName + ".exe")
if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
  Rename-Item $plainOutput ($outputName + ".exe")
}

$bundledModule = ""
if ($bundleBinaryRoot) {
  $bundledModule = Join-Path $bundleBinaryRoot "build\\loader-core.exe"
  if (-not (Test-Path $bundledModule)) {
    $bundledModule = Join-Path $bundleBinaryRoot "loader-core.exe"
  }
}
if ($bundledModule -and (Test-Path $bundledModule)) {
  Copy-Item $bundledModule (Join-Path $binDir "loader-core.exe") -Force
}

$packagesDir = Join-Path $root "build\\packages"
if (Test-Path $packagesDir) {
  Get-ChildItem -Path $packagesDir -Filter "HI3 Loader*.exe" -ErrorAction SilentlyContinue | Remove-Item -Force
}
