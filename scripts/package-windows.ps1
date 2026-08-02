$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot 'main-backend'
$outputRoot = Join-Path $repoRoot 'dist'
$installerName = 'Rescene-windows-amd64-setup.exe'
$installerPath = Join-Path $outputRoot $installerName
$checksumPath = Join-Path $outputRoot 'SHA256SUMS.txt'
$wailsConfigPath = Join-Path $backendDir 'wails.json'
$installerSourceDir = Join-Path $backendDir 'build\windows\installer'
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

$makensisCommand = Get-Command makensis -ErrorAction SilentlyContinue
if (-not $makensisCommand) {
    $knownMakensisPaths = @(@(
            (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
            (Join-Path $env:ProgramFiles 'NSIS\makensis.exe'),
            (Join-Path $env:LOCALAPPDATA 'Programs\NSIS\makensis.exe')
        ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) })
    if ($knownMakensisPaths.Count -gt 0) {
        $env:Path = "$(Split-Path -Parent $knownMakensisPaths[0]);$env:Path"
        $makensisCommand = Get-Command makensis -ErrorAction SilentlyContinue
    }
}
if (-not $makensisCommand) {
    throw '未找到 NSIS（makensis.exe）。请先运行：winget install NSIS.NSIS --silent'
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
    # NSIS 的 Windows 文件版本只接受纯数字；AppVersion 仍保留完整的预发布版本。
    $wailsConfigRaw = [System.IO.File]::ReadAllText($wailsConfigPath)
    $wailsConfig = $wailsConfigRaw | ConvertFrom-Json
    $appVersion = $wailsConfig.info.productVersion
    $numericVersionMatch = [regex]::Match($appVersion, '^\d+\.\d+\.\d+')
    if (-not $numericVersionMatch.Success) {
        throw "wails.json 的 info.productVersion 不是有效版本号：$appVersion"
    }
    $numericVersion = $numericVersionMatch.Value
    $wailsConfig.info.productVersion = $numericVersion
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText(
        $wailsConfigPath,
        ($wailsConfig | ConvertTo-Json -Depth 20),
        $utf8NoBom
    )

    & $wailsPath build -clean -nsis -installscope user -webview2 embed -ldflags "-X backend/internal/handler.AppVersion=$appVersion"
    if ($LASTEXITCODE -ne 0) { throw "wails build 失败，退出码 $LASTEXITCODE" }

    # Wails/Windows resources require a numeric file version. Recompile only the
    # lightweight NSIS wrapper so its display metadata can retain the full SemVer.
    Push-Location $installerSourceDir
    try {
        & $makensisCommand.Source `
            '-DARG_WAILS_AMD64_BINARY=..\..\bin\rescene.exe' `
            '-DWAILS_INSTALL_SCOPE=user' `
            '-DREQUEST_EXECUTION_LEVEL=user' `
            "-DINFO_DISPLAYVERSION=$appVersion" `
            'project.nsi'
        if ($LASTEXITCODE -ne 0) { throw "NSIS 重编译失败，退出码 $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
} finally {
    if ($null -ne $wailsConfigRaw) {
        [System.IO.File]::WriteAllText($wailsConfigPath, $wailsConfigRaw, $utf8NoBom)
    }
    Pop-Location
}

$wailsInstallerPath = Join-Path $backendDir 'build\bin\rescene-amd64-installer.exe'
if (-not (Test-Path -LiteralPath $wailsInstallerPath)) {
    throw "Wails 未生成预期安装器：$wailsInstallerPath"
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
Copy-Item -LiteralPath $wailsInstallerPath -Destination $installerPath -Force

# 成功生成安装器后清理旧便携产物，避免误上传到 Release。
$obsoletePortableDir = Join-Path $outputRoot 'ResceneAgent-windows-amd64'
$obsoleteZipPath = Join-Path $outputRoot 'ResceneAgent-windows-amd64.zip'
$obsoleteInstallerPath = Join-Path $outputRoot 'ResceneAgent-windows-amd64-setup.exe'
if (Test-Path -LiteralPath $obsoletePortableDir) {
    Remove-Item -LiteralPath $obsoletePortableDir -Recurse -Force
}
if (Test-Path -LiteralPath $obsoleteZipPath) {
    Remove-Item -LiteralPath $obsoleteZipPath -Force
}
if (Test-Path -LiteralPath $obsoleteInstallerPath) {
    Remove-Item -LiteralPath $obsoleteInstallerPath -Force
}

$installerHash = (Get-FileHash -LiteralPath $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
[System.IO.File]::WriteAllText($checksumPath, "$installerHash  $installerName`n", $utf8NoBom)

Write-Host "安装器打包完成：$installerPath"
Write-Host "SHA-256：$installerHash"
