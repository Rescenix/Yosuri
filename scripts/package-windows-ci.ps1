# 组装 Windows 便携版 zip + NSIS 安装器（CI 与本地共用）
# 用法: pwsh scripts/package-windows-ci.ps1
# 产出: dist/rescene.exe, dist/Rescene-windows-amd64-setup.exe, dist/Rescene-windows-amd64-portable.zip
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot 'main-backend'
$outputRoot = Join-Path $repoRoot 'dist'
$installerName = 'Rescene-windows-amd64-setup.exe'
$installerPath = Join-Path $outputRoot $installerName
$wailsBinaryName = 'rescene-package.exe'
$wailsBinaryPath = Join-Path $backendDir "build\bin\$wailsBinaryName"
$installerSourceDir = Join-Path $backendDir 'build\windows\installer'
$wailsConfigPath = Join-Path $backendDir 'wails.json'

if (-not (Test-Path -LiteralPath $wailsBinaryPath)) {
  throw "未找到 wails 产物: $wailsBinaryPath"
}

# 版本归一化：NSIS 的 VIProductVersion 只接受纯数字 4 段。
# wails build 已按 wails.json 的 productVersion 注入 AppVersion（含 -alpha 后缀），
# 这里只给 NSIS 用纯数字段，完整 SemVer 走 INFO_DISPLAYVERSION。
$wailsConfigRaw = [System.IO.File]::ReadAllText($wailsConfigPath)
$appVersionMatch = [regex]::Match($wailsConfigRaw, '"productVersion"\s*:\s*"(?<version>[^"]+)"')
if (-not $appVersionMatch.Success) { throw 'wails.json 缺少 info.productVersion' }
$appVersion = $appVersionMatch.Groups['version'].Value
$numericVersionMatch = [regex]::Match($appVersion, '^\d+\.\d+\.\d+')
if (-not $numericVersionMatch.Success) { throw "无效版本号: $appVersion" }
$numericVersion = $numericVersionMatch.Value
Push-Location $installerSourceDir
try {
  & makensis "-DARG_WAILS_AMD64_BINARY=..\..\bin\$wailsBinaryName" `
    '-DWAILS_INSTALL_SCOPE=user' '-DREQUEST_EXECUTION_LEVEL=user' `
    "-DINFO_DISPLAYVERSION=$appVersion" 'project.nsi'
  if ($LASTEXITCODE -ne 0) { throw "NSIS 打包失败，退出码 $LASTEXITCODE" }
} finally { Pop-Location }

$wailsInstallerPath = Join-Path $backendDir 'build\bin\rescene-amd64-installer.exe'
if (-not (Test-Path -LiteralPath $wailsInstallerPath)) {
  throw "NSIS 未生成预期安装器: $wailsInstallerPath"
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
Copy-Item -LiteralPath $wailsBinaryPath -Destination (Join-Path $outputRoot 'rescene.exe') -Force
Copy-Item -LiteralPath $wailsInstallerPath -Destination $installerPath -Force
Compress-Archive -Path (Join-Path $outputRoot 'rescene.exe') -DestinationPath (Join-Path $outputRoot 'Rescene-windows-amd64-portable.zip') -Force

Write-Host "打包完成: $installerPath + portable.zip"
