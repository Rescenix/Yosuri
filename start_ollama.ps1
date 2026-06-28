$env:OLLAMA_KEEP_ALIVE = "24h"
$targetModel = "qwen2.5-coder:7b"

while ($true) {
    # 1. 守护进程本身：确保 Ollama 服务活着（以防万一）
    $proc = Get-Process -Name ollama -ErrorAction SilentlyContinue
    if (-not $proc) {
        Add-Content -Path "$env:USERPROFILE\ollama_guardian.log" -Value "$(Get-Date) Ollama 进程不在，正在启动..."
        Start-Process ollama -ArgumentList "serve" -WindowStyle Hidden
        Start-Sleep 30
    }

    # 2. 模型守护核心：用 ollama ps 检查模型是否在显存
    $modelLoaded = $false
    try {
        $psOutput = & ollama ps 2>&1
        if ($psOutput -match $targetModel) {
            $modelLoaded = $true
        }
    } catch {
        # 如果命令执行失败，可能是服务还没就绪，跳过本轮
    }

    if (-not $modelLoaded) {
        Add-Content -Path "$env:USERPROFILE\ollama_guardian.log" -Value "$(Get-Date) 模型 $targetModel 已从显存中消失，正在重新预热..."
        # 异步发送一个极轻的 API 请求，触发模型加载
        Start-Job -ScriptBlock {
            param($model)
            try {
                Invoke-WebRequest -Uri "http://localhost:11434/api/generate" `
                    -Method POST `
                    -Headers @{"Content-Type" = "application/json"} `
                    -Body "{\"model\":\"$model\",\"prompt\":\".\",\"stream\":false,\"options\":{\"num_predict\":1}}" `
                    -TimeoutSec 60 | Out-Null
            } catch {}
        } -ArgumentList $targetModel | Out-Null
        # 给模型加载留出足够时间
        Start-Sleep 40
    }

    Start-Sleep 10
}