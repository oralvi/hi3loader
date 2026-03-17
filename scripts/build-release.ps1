$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$workspace = (Resolve-Path (Join-Path $root "..")).Path
$iconScript = Join-Path $workspace "tools\\sync-wails-icon.ps1"
$sourceIcon = Join-Path $workspace "icon.png"

Set-Location $root
$env:GOCACHE = Join-Path $root ".gocache-release"

& $iconScript -ProjectRoot $root -SourcePng $sourceIcon

go run github.com/wailsapp/wails/v2/cmd/wails@v2.11.0 build `
  -clean `
  -o "HI3 loader 1.0.0"

$binDir = Join-Path $root "build\\bin"
$plainOutput = Join-Path $binDir "HI3 loader 1.0.0"
$exeOutput = Join-Path $binDir "HI3 loader 1.0.0.exe"
if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
  Rename-Item $plainOutput "HI3 loader 1.0.0.exe"
}
