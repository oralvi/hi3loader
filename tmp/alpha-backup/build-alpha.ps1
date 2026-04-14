Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = (Resolve-Path (Join-Path $scriptDir "..\..")).Path
$alphaApp = Join-Path $scriptDir "app.go"
$rootApp = Join-Path $root "app.go"
$alphaServicePreview = Join-Path $scriptDir "internal_service_legacy_captcha.go"
$rootServicePreview = Join-Path $root "internal\\service\\captcha_alpha.go"
$tempBackup = Join-Path $scriptDir "app.release.backup.go"
$output = Join-Path $scriptDir "hi3loader-alpha.exe"
$alphaConfigPath = Join-Path $scriptDir "config.json"

if (-not (Test-Path -LiteralPath $alphaApp)) {
	throw "alpha app.go not found: $alphaApp"
}

if (-not (Test-Path -LiteralPath $rootApp)) {
	throw "root app.go not found: $rootApp"
}

if (-not (Test-Path -LiteralPath $alphaServicePreview)) {
	throw "alpha service preview file not found: $alphaServicePreview"
}

Copy-Item -LiteralPath $rootApp -Destination $tempBackup -Force

try {
	Copy-Item -LiteralPath $alphaApp -Destination $rootApp -Force
	Copy-Item -LiteralPath $alphaServicePreview -Destination $rootServicePreview -Force

	Push-Location $root
	try {
		go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o $output .
	} finally {
		Pop-Location
	}
} finally {
	if (Test-Path -LiteralPath $rootServicePreview) {
		Remove-Item -LiteralPath $rootServicePreview -Force -ErrorAction SilentlyContinue
	}
	if (Test-Path -LiteralPath $tempBackup) {
		Copy-Item -LiteralPath $tempBackup -Destination $rootApp -Force
		Remove-Item -LiteralPath $tempBackup -Force -ErrorAction SilentlyContinue
	}
}

if (Test-Path -LiteralPath $alphaConfigPath) {
	$config = Get-Content -LiteralPath $alphaConfigPath -Raw | ConvertFrom-Json
	if ($null -eq $config) {
		$config = [ordered]@{}
	}
	$config.PSObject.Properties.Remove('current_account')
	$config.PSObject.Properties.Remove('accounts')
	$config.PSObject.Properties.Remove('device_blob')
	$config | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $alphaConfigPath -Encoding UTF8
}

Write-Host "Built alpha executable: $output"
