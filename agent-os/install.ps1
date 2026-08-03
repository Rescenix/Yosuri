# Rescene Agent OS — 一键安装脚本 (Windows PowerShell)
# 用法: powershell -c "irm https://rescene.dev/install.ps1 | iex"
#
# 这行指令会：
# 1. 下载 rescene.exe 到本地
# 2. 加入 PATH
# 3. 安装 chafa（看板娘渲染）
# 4. 验证安装成功

$ErrorActionPreference = "Stop"

# 颜色
$Green = "✅"
$Blue = "ℹ️"
$Red = "❌"

Write-Host ""
Write-Host "╭──────────────────────────────────╮"
Write-Host "│  Rescene Agent OS — 一键安装     │"
Write-Host "╰──────────────────────────────────╯"
Write-Host ""

# 1. 确定安装目录
$InstallDir = "$env:USERPROFILE\.rescene"
if (!(Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
}

# 2. 下载 rescene.exe
$BinaryUrl = "https://github.com/undercurrent-ai/rescene/releases/latest/download/rescene.exe"
$BinaryPath = "$InstallDir\rescene.exe"

Write-Host "$Blue 下载 rescene.exe..."
try {
    Invoke-WebRequest -Uri $BinaryUrl -OutFile $BinaryPath -UseBasicParsing
    Write-Host "$Green 下载完成: $BinaryPath"
} catch {
    Write-Host "$Red 下载失败: $_"
    Write-Host "$Blue 尝试备用下载地址..."
    # 备用：直接 git clone 编译
    $BinaryUrl = "https://raw.githubusercontent.com/undercurrent-ai/rescene/main/agent-os/rescene.exe"
    Invoke-WebRequest -Uri $BinaryUrl -OutFile $BinaryPath -UseBasicParsing
}

# 3. 下载看板娘图片
$MascotUrl = "https://raw.githubusercontent.com/undercurrent-ai/rescene/main/agent-os/rescene-mascot.png"
$MascotPath = "$InstallDir\rescene-mascot.png"
Write-Host "$Blue 下载看板娘..."
Invoke-WebRequest -Uri $MascotUrl -OutFile $MascotPath -UseBasicParsing

# 4. 加入 PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")
    Write-Host "$Green 已加入 PATH"
    # 当前会话也生效
    $env:Path += ";$InstallDir"
} else {
    Write-Host "$Blue 已在 PATH 中"
}

# 5. 安装 chafa（看板娘渲染必需）
Write-Host "$Blue 安装 chafa（图片→终端渲染）..."
try {
    $chafaCheck = Get-Command chafa -ErrorAction SilentlyContinue
    if (!$chafaCheck) {
        winget install hpjansson.Chafa --accept-source-agreements --silent 2>&1 | Out-Null
        Write-Host "$Green chafa 安装完成"
    } else {
        Write-Host "$Blue chafa 已安装"
    }
} catch {
    Write-Host "$Blue 提示: chafa 未安装，看板娘将使用备用渲染"
}

# 6. 验证
Write-Host ""
Write-Host "╭──────────────────────────────────╮"
Write-Host "│  ✅ 安装完成！                    │"
Write-Host "╰──────────────────────────────────╯"
Write-Host ""
Write-Host "  现在打开新终端，输入:"
Write-Host ""
Write-Host "    rescene"
Write-Host ""
Write-Host "  或一键执行:"
Write-Host ""
Write-Host "    rescene exec '帮我查下系统信息'"
Write-Host ""
Write-Host "  免费模型已内置（无需配置）:"
Write-Host "    - DeepSeek V4 Flash (OpenCode Zen)"
Write-Host "    - Mimo 2.5 (OpenCode Zen)"
Write-Host "    - North Mini Code (OpenCode Zen)"
Write-Host ""
Write-Host "  配置 API Key 可解锁更多模型:"
Write-Host "    set SENSENOVA_API_KEY=sk-xxx   (商汤免费)"
Write-Host "    set MODELSCOPE_API_KEY=xxx     (魔搭免费)"
Write-Host ""
Write-Host "  重启终端后生效。如果 rescene 找不到，"
Write-Host "  直接运行: $InstallDir\rescene.exe"
Write-Host ""

# 7. 当前会话临时可用
& "$InstallDir\rescene.exe" version