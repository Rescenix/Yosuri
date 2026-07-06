# prismd-bridge.ps1 — Claude Code memory/ ↔ PrismD ENGRAM 桥接
# 用法:
#   .\prismd-bridge.ps1 -File path\to\memory\xxx.md -Action create
#   .\prismd-bridge.ps1 -File path\to\memory\xxx.md -Action update
#   .\prismd-bridge.ps1 -Query "用户偏好"    # 从 PrismD 查记忆
#
# 依赖: PrismD 服务在 localhost:$Port 运行

param(
    [string]$File,           # 要同步的 memory/*.md 文件路径
    [ValidateSet("create","update","delete")]
    [string]$Action = "create",
    [string]$Query,          # 直接查询（跳过文件解析）
    [int]$Port = 5666,
    [string]$Domain = "Claude"
)

$ErrorActionPreference = "Continue"
$PRISMD = "http://127.0.0.1:$Port"
$SCRIPT_DIR = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$PROJ_ROOT = Split-Path -Parent $SCRIPT_DIR
$TRACKING = "$PROJ_ROOT\data\bridge_tracking.json"
Write-Verbose "TRACKING=$TRACKING"

# ── 工具函数 ──
function Send-PrimQL($body) {
    try {
        $r = Invoke-RestMethod -Uri $PRISMD -Method POST -Body $body -TimeoutSec 3
        return $r.Trim()
    } catch {
        Write-Warning "PrismD 不可达 ($Port): $_"
        return $null
    }
}

function Get-Tracking {
    if (Test-Path $TRACKING) {
        $obj = Get-Content $TRACKING -Raw | ConvertFrom-Json
        $ht = @{}
        foreach ($prop in $obj.PSObject.Properties) {
            $inner = @{}
            foreach ($ip in $prop.Value.PSObject.Properties) {
                $inner[$ip.Name] = $ip.Value
            }
            $ht[$prop.Name] = $inner
        }
        return $ht
    }
    return @{}
}

function Set-Tracking($map) {
    $dir = Split-Path $TRACKING -Parent
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Force $dir | Out-Null
    }
    try {
        $map | ConvertTo-Json -Depth 3 | Set-Content $TRACKING -Encoding UTF8
    } catch {
        Write-Warning "[bridge] 无法写入 tracking: $_"
    }
}

# ── 解析 memory/*.md 文件 ──
function Parse-MemoryFile($path) {
    if (-not (Test-Path $path)) {
        Write-Error "文件不存在: $path"
        return $null
    }

    $content = Get-Content $path -Raw -Encoding UTF8

    # 解析 YAML frontmatter
    $meta = @{}
    if ($content -match '^---\s*\n([\s\S]*?)\n---') {
        $yaml = $matches[1]
        foreach ($line in ($yaml -split '\n')) {
            if ($line -match '^\s*(\w[\w-]*)\s*:\s*(.+)$') {
                $key = $matches[1].Trim()
                $val = $matches[2].Trim().Trim('"').Trim("'")
                $meta[$key] = $val
            }
        }
        $body = $content -replace '^---[\s\S]*?\n---\s*\n?', ''
    } else {
        $body = $content
    }

    $name = if ($meta.ContainsKey('name') -and $meta['name']) { $meta['name'] } else { (Split-Path $path -Leaf) -replace '\.md$','' }
    $desc = if ($meta.ContainsKey('description') -and $meta['description']) { $meta['description'] } else { '' }
    $typ  = if ($meta.ContainsKey('type') -and $meta['type']) { $meta['type'] } else { 'reference' }

    return @{
        Name        = $name
        Description = $desc
        Type        = $typ
        Body        = $body.Trim()
        FilePath    = (Resolve-Path $path).Path
    }
}

# ── 从 body 提取核心事实（简单规则） ──
function Summarize-Body($parsed) {
    $lines = $parsed.Body -split '\n' | Where-Object { $_ -notmatch '^\s*(#|>|```|Related|Why:|How to|相关记忆|<!--)' } | Where-Object { $_.Trim() -ne '' }
    # 取前 5 行有效内容
    $keyLines = ($lines | Select-Object -First 5) -join ' '
    # 压缩长度
    if ($keyLines.Length -gt 300) { $keyLines = $keyLines.Substring(0, 297) + '...' }
    return "$($parsed.Description): $keyLines"
}

# ── 主逻辑 ──

# 先切到目标域
$switch = Send-PrimQL "DOMAIN USE $Domain"
if (-not $switch) { exit 1 }

# 模式 1: 直接查询
if ($Query) {
    $result = Send-PrimQL "LOOM $Query"
    Write-Output $result
    exit 0
}

# 模式 2: 文件同步
if (-not $File) {
    Write-Error "需要 -File 或 -Query 参数"
    exit 1
}

$parsed = Parse-MemoryFile $File
if (-not $parsed) { exit 1 }
$tracking = Get-Tracking
$fileName = $parsed.Name

switch ($Action) {
    "create" {
        $text = Summarize-Body $parsed
        $role = "$($parsed.Type)-memory"
        Write-Host "[bridge] ENGRAM $role → $Domain 域"
        $resp = Send-PrimQL "ENGRAM $role $text"
        if ($resp -match 'OK (\d+)') {
            $nodeId = [int]$matches[1]
            $tracking[$fileName] = @{ nodeId = $nodeId; file = $parsed.FilePath; updated = (Get-Date -Format 'o') }
            Set-Tracking $tracking
            Write-Host "[bridge] ✓ 节点 #$nodeId 已创建, 映射已记录"
        } else {
            Write-Warning "[bridge] ✗ ENGRAM 失败: $resp"
        }
    }
    "update" {
        if (-not $tracking.ContainsKey($fileName)) {
            Write-Warning "[bridge] ⚠ 未找到 $fileName 的已有节点映射, 尝试创建..."
            & $PSCommandPath -File $File -Action create -Port $Port -Domain $Domain
            return
        }
        $nodeId = $tracking[$fileName].nodeId
        $text = Summarize-Body $parsed
        $role = "$($parsed.Type)-memory"
        Write-Host "[bridge] REFRACT #$nodeId → $Domain 域"
        $json = @{ id = $nodeId; content = $text; role = $role } | ConvertTo-Json -Compress
        $resp = Send-PrimQL "REFRACT $json"
        if ($resp -match 'OK') {
            $tracking[$fileName].updated = (Get-Date -Format 'o')
            Set-Tracking $tracking
            Write-Host "[bridge] ✓ 节点 #$nodeId 已更新"
        } else {
            Write-Warning "[bridge] ✗ REFRACT 失败: $resp"
        }
    }
    "delete" {
        if (-not $tracking.ContainsKey($fileName)) {
            Write-Warning "[bridge] ⚠ 未找到 $fileName 的已有节点映射"
            return
        }
        $nodeId = $tracking[$fileName].nodeId
        Write-Host "[bridge] PRUNE #$nodeId → $Domain 域"
        $resp = Send-PrimQL "PRUNE $nodeId"
        if ($resp -match 'OK') {
            $tracking.Remove($fileName)
            Set-Tracking $tracking
            Write-Host "[bridge] ✓ 节点 #$nodeId 已遗忘"
        }
    }
}
