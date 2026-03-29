$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$baseVersion = "1.1.2"
$buildTime = Get-Date
$buildDate = $buildTime.ToString("yyMMddHHmmss")
$bytes = New-Object byte[] 4
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$devStamp = "dev+" + ([System.BitConverter]::ToString($bytes).Replace("-", "").ToLowerInvariant()) + "+" + $buildDate
$outputName = "HI3 Loader $baseVersion $devStamp"
$ldFlags = "-X main.appVersion=$baseVersion -X main.buildStamp=$devStamp"

Set-Location $root
$env:GOCACHE = Join-Path $root ".gocache-release"

$hasPrivateImpl = $false
try {
  & go test -run '^$' -tags private_impl ./... *> $null
  if ($LASTEXITCODE -eq 0) {
    $hasPrivateImpl = $true
  }
} catch {
  $hasPrivateImpl = $false
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

if ($hasPrivateImpl) {
  $wailsArgs += @("-tags", "private_impl")
}

Write-Host ("Building development package version {0} with stamp {1}" -f $baseVersion, $devStamp)
& go @wailsArgs

$binDir = Join-Path $root "build\\bin"
$plainOutput = Join-Path $binDir $outputName
$exeOutput = Join-Path $binDir ($outputName + ".exe")
if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
  Rename-Item $plainOutput ($outputName + ".exe")
}
