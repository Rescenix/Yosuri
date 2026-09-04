# ⚠️ 仅作本地测试参考。正式发布统一走 GitHub Actions CI（push tag 触发），
#    请勿用本脚本执行发布。保持 CI 自动化是唯一发布通道。
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot 'main-backend'
$outputRoot = Join-Path $repoRoot 'dist'
$installerName = 'Rescene-windows-amd64-setup.exe'
$installerPath = Join-Path $outputRoot $installerName
$checksumPath = Join-Path $outputRoot 'SHA256SUMS.txt'
$wailsConfigPath = Join-Path $backendDir 'wails.json'
$installerSourceDir = Join-Path $backendDir 'build\windows\installer'
$wailsBinaryName = 'rescene-package.exe'
$wailsBinaryPath = Join-Path $backendDir "build\bin\$wailsBinaryName"
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
    # 先从原始 JSON 固化完整 SemVer。不要从随后会改写 productVersion 的
    # PSCustomObject 取值：并发/嵌套构建时会把 -alpha.N 后缀带丢，导致热更新
    # 把测试包误识别成稳定包。
    $appVersionMatch = [regex]::Match($wailsConfigRaw, '"productVersion"\s*:\s*"(?<version>[^"]+)"')
    if (-not $appVersionMatch.Success) {
        throw 'wails.json 缺少 info.productVersion'
    }
    [string]$appVersion = $appVersionMatch.Groups['version'].Value
    $wailsConfig = $wailsConfigRaw | ConvertFrom-Json
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

    # 使用独立输出名，不清空 build/bin。开发者可能正运行上一版 rescene.exe；
    # 旧脚本的 -clean 会因 Windows 文件锁直接打包失败。
    # ⚠️ 不能省 -nsis / 不能换回 -nopackage（alpha.3 血泪）：Wails 只在
    # Pack=true 时生成 rescene-res.syso 资源（build.go: options.Pack && platform==windows），
    # 图标+版本信息全靠它；-nopackage 会导致 exe 无 .rsrc → 桌面快捷方式无图标。
    & $wailsPath build -nsis -o $wailsBinaryName -installscope user -webview2 embed -ldflags "-X backend/internal/handler.AppVersion=$appVersion"
    if ($LASTEXITCODE -ne 0) { throw "wails build 失败，退出码 $LASTEXITCODE" }

    # Wails/Windows resources require a numeric file version. Recompile only the
    # lightweight NSIS wrapper so its display metadata can retain the full SemVer.
    Push-Location $installerSourceDir
    try {
        & $makensisCommand.Source `
            "-DARG_WAILS_AMD64_BINARY=..\..\bin\$wailsBinaryName" `
            '-DWAILS_INSTALL_SCOPE=user' `
            '-DREQUEST_EXECUTION_LEVEL=user' `
            "-DINFO_DISPLAYVERSION=$appVersion" `
            'project.nsi'
        if ($LASTEXITCODE -ne 0) { throw "NSIS 重编译失败，退出码 $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
} finally {
    $restoreError = $null
    if ($null -ne $wailsConfigRaw) {
        [System.IO.File]::WriteAllText($wailsConfigPath, $wailsConfigRaw, $utf8NoBom)
        $restoredVersion = ([System.IO.File]::ReadAllText($wailsConfigPath) | ConvertFrom-Json).info.productVersion
        if ($restoredVersion -ne $appVersion) {
            $restoreError = "wails.json 版本恢复失败：$restoredVersion（预期 $appVersion）"
        }
    }
    Pop-Location
    if ($restoreError) { throw $restoreError }
}

$wailsInstallerPath = Join-Path $backendDir 'build\bin\rescene-amd64-installer.exe'
if (-not (Test-Path -LiteralPath $wailsInstallerPath)) {
    throw "Wails 未生成预期安装器：$wailsInstallerPath"
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
Copy-Item -LiteralPath $wailsInstallerPath -Destination $installerPath -Force

# 官网与热更新共用同一个 zip：
# - 官网用户解压后运行 setup.exe，获得正常安装/快捷方式体验；
# - 应用内热更新只提取 rescene.exe，覆盖后自动重启。
# 两个文件必须同时存在，避免官网包与热更新包分叉成两套发布物。
$portableZipPath = Join-Path $outputRoot 'Rescene-windows-amd64-portable.zip'
if (Test-Path -LiteralPath $portableZipPath) {
    Remove-Item -LiteralPath $portableZipPath -Force
}
$packageStage = Join-Path $outputRoot '.package-stage'
if (Test-Path -LiteralPath $wailsBinaryPath) {
    if (Test-Path -LiteralPath $packageStage) {
        Remove-Item -LiteralPath $packageStage -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $packageStage | Out-Null
    try {
        Copy-Item -LiteralPath $wailsBinaryPath -Destination (Join-Path $packageStage 'rescene.exe') -Force
        Copy-Item -LiteralPath $installerPath -Destination (Join-Path $packageStage $installerName) -Force
        Compress-Archive -Path (Join-Path $packageStage '*') -DestinationPath $portableZipPath -CompressionLevel Optimal
    } finally {
        Remove-Item -LiteralPath $packageStage -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Host "官网/热更新共用 zip 打包完成（rescene.exe + setup.exe）：$portableZipPath"
} else {
    Write-Host "警告：未找到 $wailsBinaryPath，跳过便携 zip 生成"
}

$installerHash = (Get-FileHash -LiteralPath $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
[System.IO.File]::WriteAllText($checksumPath, "$installerHash  $installerName`n", $utf8NoBom)

Write-Host "安装器打包完成：$installerPath"
Write-Host "SHA-256：$installerHash"
