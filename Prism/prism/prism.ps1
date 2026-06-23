$server = "localhost"
$port = 5666

function Send-PrismQL {
    param($cmd, $args)

    $client = New-Object System.Net.Sockets.TcpClient($server, $port)
    $stream = $client.GetStream()

    # 发送命令头
    $cmdBytes = [Text.Encoding]::UTF8.GetBytes($cmd.ToUpper())
    $stream.Write($cmdBytes, 0, $cmdBytes.Length)

    # 发送每个参数（长度前缀 + 内容）
    foreach ($a in $args) {
        if ($a -eq $null -or $a -eq "") { continue }
        $bytes = [Text.Encoding]::UTF8.GetBytes($a)
        $lenBuf = [BitConverter]::GetBytes([UInt32]$bytes.Length)
        [Array]::Reverse($lenBuf)   # 大端
        $stream.Write($lenBuf, 0, 4)
        $stream.Write($bytes, 0, $bytes.Length)
    }

    $stream.WriteByte(10)  # 换行结束
    $stream.Flush()

    Start-Sleep -Milliseconds 1000

    $reader = New-Object System.IO.StreamReader($stream)
    try {
        if ($stream.DataAvailable) {
            $reader.ReadToEnd()
        } else {
            ""
        }
    } catch {
        "⚠️ 连接已关闭（命令可能失败）"
    } finally {
        $client.Close()
    }
}

Write-Host "⚡ Prism 混沌记忆终端 (5666)" -ForegroundColor Cyan
Write-Host "直接输入 PrimQL 命令：LOOM、ENGRAM、STATS、DRIFT" -ForegroundColor DarkGray
Write-Host "输入 :q 退出" -ForegroundColor DarkGray

while ($true) {
    $input = Read-Host "prism>"
    if ($input -eq ":q") { break }
    
    $parts = $input.Trim() -split '\s+'
    $cmd = $parts[0]
    $restArgs = $parts[1..($parts.Count - 1)]
    
    if ($cmd -eq "ENGRAM" -and $restArgs.Count -ge 2) {
        $role = $restArgs[0]
        $content = [string]::Join(" ", $restArgs[1..($restArgs.Count - 1)])
        $restArgs = @($role, $content)
    }
    
    $response = Send-PrismQL $cmd $restArgs
    Write-Host $response
}