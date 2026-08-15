# 杀全部 msedgewebview2 孤儿 + 重建 EBWebView 目录（2026-08-12 rescene GUI 起不来的修复）
Get-Process msedgewebview2 -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host ("kill " + $_.Id)
    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Seconds 2
$left = @(Get-Process msedgewebview2 -ErrorAction SilentlyContinue).Count
Write-Host ("remaining webview2: " + $left)

$dir = "C:\Users\undercurrent\AppData\Roaming\rescene.exe\EBWebView"
if (Test-Path $dir) {
    $bak = "C:\Users\undercurrent\AppData\Roaming\rescene.exe\EBWebView.bak"
    if (Test-Path $bak) { Remove-Item $bak -Recurse -Force -ErrorAction SilentlyContinue }
    try {
        Rename-Item $dir $bak
        Write-Host "EBWebView renamed -> .bak"
    } catch {
        Write-Host ("rename failed: " + $_.Exception.Message)
    }
} else {
    Write-Host "EBWebView not present"
}
