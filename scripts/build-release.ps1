param(
  [string]$BundleRoot = $env:LOADER_BUNDLE_ROOT,
  [string]$Version = "1.2.0"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$baseVersion = $Version.Trim()
if ($baseVersion -notmatch '^\d+\.\d+\.\d+$') {
  throw "Version must use major.minor.patch format: $baseVersion"
}
if (-not $BundleRoot) {
  throw "Final release requires LOADER_BUNDLE_ROOT or -BundleRoot."
}

$bundleBinaryRoot = (Resolve-Path $BundleRoot).Path
$bundlePrepareScript = Join-Path $bundleBinaryRoot "scripts\prepare-bundle.ps1"
if (-not (Test-Path $bundlePrepareScript)) {
  throw "Bundle prepare script not found: $bundlePrepareScript"
}

$buildTime = Get-Date
$buildStamp = $buildTime.ToString("yyMMddHHmmss")
$displayTime = $buildTime.ToString("yyyy-MM-dd HH:mm:ss zzz")
$titleStamp = "r$buildStamp"
$fingerprintBytes = New-Object byte[] 16
$fingerprintRng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$fingerprintRng.GetBytes($fingerprintBytes)
$fingerprintRng.Dispose()
$buildFingerprint = ([System.BitConverter]::ToString($fingerprintBytes).Replace("-", "").ToLowerInvariant())
$outputName = "HI3 Loader"
$packageBaseName = "HI3 Loader v$baseVersion final"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

$previousGoCache = $env:GOCACHE
$previousGoWork = $env:GOWORK
$previousLocation = Get-Location

try {
  Set-Location $root
  $env:GOCACHE = Join-Path $root ".gocache-release"
  $env:GOWORK = "off"

  & $bundlePrepareScript -PublicRoot $root -Version $baseVersion
  if ($LASTEXITCODE -ne 0) {
    throw "Bundled runtime build failed."
  }

  $windowsInfoPath = Join-Path $root "build\windows\info.json"
  $windowsVersion = $baseVersion + ".0"
  $windowsInfo = @"
{
  "fixed": {
    "file_version": "$windowsVersion",
    "product_version": "$windowsVersion"
  },
  "info": {
    "0804": {
      "CompanyName": "Oralvi Sakura",
      "FileDescription": "HI3 Loader",
      "FileVersion": "$baseVersion",
      "InternalName": "HI3 Loader",
      "LegalCopyright": "Copyright Oralvi Sakura",
      "OriginalFilename": "HI3 Loader.exe",
      "ProductName": "HI3 Loader",
      "ProductVersion": "$baseVersion",
      "Comments": "Final archived release"
    }
  }
}
"@
  [System.IO.File]::WriteAllText($windowsInfoPath, $windowsInfo, $utf8NoBom)

  $ldFlags = "-s -w -X hi3loader/internal/buildinfo.AppVersion=$baseVersion -X hi3loader/internal/buildinfo.BuildStamp=$titleStamp -X hi3loader/internal/buildinfo.BuildFingerprint=$buildFingerprint"
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

  Write-Host ("Building release version {0} with title stamp {1} and fingerprint {2} ({3})" -f $baseVersion, $titleStamp, $buildFingerprint, $displayTime)
  & go @wailsArgs
  if ($LASTEXITCODE -ne 0) {
    throw "Wails release build failed."
  }

  $binDir = Join-Path $root "build\bin"
  $plainOutput = Join-Path $binDir $outputName
  $exeOutput = Join-Path $binDir ($outputName + ".exe")
  if ((Test-Path $plainOutput) -and -not (Test-Path $exeOutput)) {
    Rename-Item $plainOutput ($outputName + ".exe")
  }
  if (-not (Test-Path $exeOutput)) {
    throw "GUI output not found: $exeOutput"
  }

  $bundledModule = Join-Path $bundleBinaryRoot "build\loader-core.exe"
  if (-not (Test-Path $bundledModule)) {
    throw "Bundled runtime output not found: $bundledModule"
  }
  Copy-Item $bundledModule (Join-Path $binDir "loader-core.exe") -Force

  $packagesDir = Join-Path $root "build\packages"
  $stagingDir = Join-Path $packagesDir $packageBaseName
  $zipPath = Join-Path $packagesDir ($packageBaseName + ".zip")
  $zipChecksumPath = $zipPath + ".sha256"
  $releaseNotesPath = Join-Path $packagesDir "RELEASE_NOTES_v$baseVersion.md"

  New-Item -ItemType Directory -Force -Path $packagesDir | Out-Null
  if (Test-Path $stagingDir) {
    Remove-Item $stagingDir -Recurse -Force
  }
  foreach ($path in @($zipPath, $zipChecksumPath, $releaseNotesPath)) {
    if (Test-Path $path) {
      Remove-Item $path -Force
    }
  }

  New-Item -ItemType Directory -Force -Path $stagingDir | Out-Null
  Copy-Item $exeOutput (Join-Path $stagingDir "HI3 Loader.exe") -Force
  Copy-Item (Join-Path $binDir "loader-core.exe") (Join-Path $stagingDir "loader-core.exe") -Force

  $readme = @"
HI3 Loader v$baseVersion

Windows x64 portable final release.

1. Keep HI3 Loader.exe and loader-core.exe in the same directory.
2. Run HI3 Loader.exe.
3. Do not move, rename, or delete loader-core.exe.

This package has no installer or automatic updater. The repository is archived
after this release and no future compatibility updates are planned.
"@
  [System.IO.File]::WriteAllText((Join-Path $stagingDir "README.txt"), $readme.Trim() + [Environment]::NewLine, $utf8NoBom)

  $checksumLines = foreach ($name in @("HI3 Loader.exe", "loader-core.exe")) {
    $hash = Get-FileHash (Join-Path $stagingDir $name) -Algorithm SHA256
    "{0}  {1}" -f $hash.Hash.ToLowerInvariant(), $name
  }
  [System.IO.File]::WriteAllLines((Join-Path $stagingDir "SHA256SUMS.txt"), $checksumLines, $utf8NoBom)

  Compress-Archive -Path (Join-Path $stagingDir "*") -DestinationPath $zipPath -CompressionLevel Optimal
  $zipHash = Get-FileHash $zipPath -Algorithm SHA256
  $zipChecksum = "{0}  {1}" -f $zipHash.Hash.ToLowerInvariant(), (Split-Path $zipPath -Leaf)
  [System.IO.File]::WriteAllText($zipChecksumPath, $zipChecksum + [Environment]::NewLine, $utf8NoBom)

  $releaseNotes = @'
第一次发 Release，也是最后一次啦。

一路修修补补的小工具，终于被好好塞进压缩包里了。
没有豪华安装向导，也没有神秘自动更新，只留下能用的最后一版。

### 使用方法

1. 下载并完整解压 `HI3 Loader v1.2.0 final.zip`
2. 保持 `HI3 Loader.exe` 和 `loader-core.exe` 在同一目录
3. 运行 `HI3 Loader.exe`

请不要把两个 EXE 拆散，不然它们会因为找不到彼此而当场自闭。

本版本仅面向 Windows x64。压缩包与程序文件的 SHA-256 校验值已随 Release 提供。

仓库将在本次发布后进入归档状态，不再继续维护。以后上游接口发生变化时，它可能会逐渐失效，这也是最后一版应有的宿命感吧。

愿你的二维码一次识别成功，验证码不要半夜敲门，启动器也永远别突然抽风。

那么，休伯利安的小灯就关到这里啦。
晚安，舰长。(`・ω・´)ゞ
'@
  $releaseNotes = $releaseNotes.Replace('v1.2.0', "v$baseVersion")
  [System.IO.File]::WriteAllText($releaseNotesPath, $releaseNotes.Trim() + [Environment]::NewLine, $utf8NoBom)

  Remove-Item $stagingDir -Recurse -Force

  Write-Host ("Final package: {0}" -f $zipPath)
  Write-Host ("Package SHA256: {0}" -f $zipHash.Hash.ToLowerInvariant())
  Write-Host ("Release notes: {0}" -f $releaseNotesPath)
}
finally {
  Set-Location $previousLocation
  if ($null -eq $previousGoCache) {
    Remove-Item Env:GOCACHE -ErrorAction SilentlyContinue
  } else {
    $env:GOCACHE = $previousGoCache
  }
  if ($null -eq $previousGoWork) {
    Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
  } else {
    $env:GOWORK = $previousGoWork
  }
}
