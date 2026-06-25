# start_ollama.ps1 - 静默启动 Ollama 后台服务
$ErrorActionPreference = "SilentlyContinue"

# 清理残留
Stop-Process -Name ollama -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# 设置保活并启动
$env:OLLAMA_KEEP_ALIVE = "24h"
Start-Job -Name Ollama -ScriptBlock {
    param($keepAlive)
    $env:OLLAMA_KEEP_ALIVE = $keepAlive
    ollama serve 2>&1 | Out-Null
} -ArgumentList "24h" | Out-Null

# 等待就绪
Write-Host "等待 Ollama 加载模型..." -ForegroundColor Yellow
for ($i = 0; $i -lt 30; $i++) {
    try {
        $null = Invoke-WebRequest -Uri "http://localhost:11434" -TimeoutSec 3 -ErrorAction Stop
        Write-Host "Ollama 就绪 (http://localhost:11434)" -ForegroundColor Green
        exit 0
    } catch {
        Start-Sleep -Seconds 2
    }
}
Write-Host "Ollama 启动超时" -ForegroundColor Red