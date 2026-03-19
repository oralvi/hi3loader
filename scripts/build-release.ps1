$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$outputName = "HI3 Loader 1.1.0"

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
  "-s -w",
  "-clean",
  "-o",
  $outputName
)

if ($hasPrivateImpl) {
  $wailsArgs += @("-tags", "private_impl")
}

& go @wailsArgs

$binDir = Join-Path $root "build\\bin"
$plainOutput = Join-Path $binDir $outputName
$exeOutput = Join-Path $binDir ($outputName + ".exe")
if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
  Rename-Item $plainOutput ($outputName + ".exe")
}
