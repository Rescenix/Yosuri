param(
    [switch]$SkipChromium
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot 'main-backend'
$outputRoot = Join-Path $repoRoot 'dist'
$portableDir = Join-Path $outputRoot 'ResceneAgent-windows-amd64'
$wailsCommand = Get-Command wails -ErrorAction SilentlyContinue
$wailsPath = if ($wailsCommand) { $wailsCommand.Source } else { $null }
if (-not $wailsPath) {
    $goPath = & go env GOPATH
    $goWails = Join-Path $goPath 'bin\wails.exe'
    if (Test-Path -LiteralPath $goWails) {
        $wailsPath = $goWails
    }
}
if (-not $wailsPath) {
    throw '未找到 Wails v2 CLI。请先运行：go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0'
}

$goCache = Join-Path $backendDir '.codex-go-cache'
$goTemp = Join-Path $goCache 'tmp'
New-Item -ItemType Directory -Force -Path $goTemp | Out-Null
$env:GOCACHE = $goCache
$env:GOTMPDIR = $goTemp
$env:TEMP = $goTemp
$env:TMP = $goTemp

Push-Location $backendDir
try {
    & $wailsPath build -clean -webview2 embed
    if ($LASTEXITCODE -ne 0) { throw "wails build 失败，退出码 $LASTEXITCODE" }
} finally {
    Pop-Location
}

if (Test-Path -LiteralPath $portableDir) {
    Remove-Item -LiteralPath $portableDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $portableDir | Out-Null
Copy-Item -LiteralPath (Join-Path $backendDir 'build\bin\ResceneAgent.exe') -Destination $portableDir

$chromiumSource = Join-Path $repoRoot 'runtime\chromium'
$chromiumExe = Join-Path $chromiumSource 'chrome.exe'
if (-not $SkipChromium -and (Test-Path -LiteralPath $chromiumExe)) {
    $releaseManifest = Join-Path $chromiumSource 'RELEASE.json'
    if (-not (Test-Path -LiteralPath $releaseManifest)) {
        throw 'Chromium 运行时存在，但缺少 runtime/chromium/RELEASE.json 来源与校验信息。'
    }
    $runtimeTarget = Join-Path $portableDir 'runtime\chromium'
    New-Item -ItemType Directory -Force -Path $runtimeTarget | Out-Null
    Copy-Item -Path (Join-Path $chromiumSource '*') -Destination $runtimeTarget -Recurse -Force
} elseif (-not $SkipChromium) {
    Write-Warning 'runtime/chromium/chrome.exe 不存在；本包预览将回退使用系统 Edge/Chrome。'
}

$zipPath = Join-Path $outputRoot 'ResceneAgent-windows-amd64.zip'
if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}
Compress-Archive -Path (Join-Path $portableDir '*') -DestinationPath $zipPath -CompressionLevel Optimal
Write-Host "打包完成：$zipPath"
