$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$baseVersion = "1.1.1"
$buildTime = Get-Date
$buildStamp = $buildTime.ToString("yyMMddHHmmss")
$displayTime = $buildTime.ToString("yyyy-MM-dd HH:mm:ss zzz")
$titleStamp = "r$buildStamp"
$outputName = "HI3 Loader $baseVersion"

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

$ldFlags = "-s -w -X main.appVersion=$baseVersion -X main.buildStamp=$titleStamp"

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

Write-Host ("Building release version {0} with title stamp {1} ({2})" -f $baseVersion, $titleStamp, $displayTime)
& go @wailsArgs

$binDir = Join-Path $root "build\\bin"
$plainOutput = Join-Path $binDir $outputName
$exeOutput = Join-Path $binDir ($outputName + ".exe")
if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
  Rename-Item $plainOutput ($outputName + ".exe")
}
