#!/usr/bin/env python3
# grep_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供两个工具，对齐 Claude Code 的 Grep / Glob：
#   - grep : 内容搜索，subprocess 调本机已装的 rg（ripgrep），零二进制依赖
#   - glob : 文件名/路径模式匹配，标准库 pathlib 实现
#
# 工作目录来自环境变量 MCP_ROOT（由 Go 后端在拉起时注入 core.GetProjectRoot()），
# 因此跟着「主页选项目」动态变化，不写死。
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 与 main-backend/internal/handler/mcp_client.go 的握手完全同构。

import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(os.environ.get("MCP_ROOT", "C:\\Pro2026\\re0")).resolve()

# rg 走本机已装二进制（WinGet 软链），找不到就退化到 PATH
RG = "rg"


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {
        "content": [{"type": "text", "text": text}],
        "isError": is_error,
    }


def do_grep(pattern: str, path: str = ".", ftype: str = "", max_count: int = 200):
    target = (ROOT / path).resolve() if not os.path.isabs(path) else Path(path)
    cmd = [RG, "-n", "--heading", "--color", "never", "--max-count", str(max_count), pattern, str(target)]
    if ftype:
        cmd += ["--type", ftype]
    try:
        out = subprocess.run(cmd, cwd=str(ROOT), capture_output=True, text=True, timeout=30)
    except subprocess.TimeoutExpired:
        return tool_result("grep 超时（>30s），请缩小范围或加 --type 过滤", is_error=True)
    if out.returncode not in (0, 1):  # 1 = 无匹配，也是正常结果
        return tool_result(f"grep 失败: {out.stderr.strip()}", is_error=True)
    lines = out.stdout.strip()
    if not lines:
        return tool_result(f"在 {target} 下未找到匹配 '{pattern}' 的内容。")
    return tool_result(lines)


def do_glob(pattern: str, path: str = "."):
    from glob import glob as gglob
    target = (ROOT / path).resolve() if not os.path.isabs(path) else Path(path)
    base = str(target)
    # 支持 ** 递归：先按目录层展开
    matches = []
    if "**" in pattern:
        for p in target.rglob(pattern.split("**")[-1].lstrip("/")):
            if p.is_file():
                matches.append(str(p.relative_to(ROOT)))
    else:
        for m in gglob(os.path.join(base, pattern)):
            if os.path.isfile(m):
                matches.append(str(Path(m).relative_to(ROOT)))
    if not matches:
        return tool_result(f"在 {target} 下未匹配到 '{pattern}'。")
    return tool_result("\n".join(sorted(matches)))


TOOLS = [
    {
        "name": "grep",
        "description": "在当前项目内按内容搜索（ripgrep）。pattern 为正则；path 默认 '.' 即项目根；type 可限定语言（如 go/vue/js）；返回 文件:行号:匹配内容。用于定位代码、查 TODO、找符号定义。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "pattern": {"type": "string", "description": "要搜索的正则表达式"},
                "path": {"type": "string", "description": "搜索起点，默认 '.'（项目根），可给子目录"},
                "type": {"type": "string", "description": "可选语言过滤，如 go / vue / js / py"},
            },
            "required": ["pattern"],
        },
    },
    {
        "name": "glob",
        "description": "按文件名/路径模式列出项目内文件（如 '**/*.go'、'src/**/*.vue'）。用于不知道文件全名时快速定位，比 grep 更轻。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "pattern": {"type": "string", "description": "glob 模式，如 '**/*.go'"},
                "path": {"type": "string", "description": "搜索起点，默认 '.'（项目根）"},
            },
            "required": ["pattern"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "grep":
        return do_grep(args.get("pattern", ""), args.get("path", "."), args.get("type", ""))
    if name == "glob":
        return do_glob(args.get("pattern", ""), args.get("path", "."))
    return tool_result(f"未知工具: {name}", is_error=True)


def main():
    for raw in sys.stdin:
        raw = raw.strip()
        if not raw:
            continue
        try:
            msg = json.loads(raw)
        except json.JSONDecodeError:
            continue
        method = msg.get("method")
        mid = msg.get("id")
        if method == "initialize":
            send({
                "jsonrpc": "2.0", "id": mid,
                "result": {
                    "protocolVersion": "2024-11-05",
                    "capabilities": {},
                    "serverInfo": {"name": "re0-grep", "version": "1.0.0"},
                },
            })
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            send({"jsonrpc": "2.0", "id": mid, "result": {"tools": TOOLS}})
        elif method == "tools/call":
            params = msg.get("params", {})
            res = handle_call(params.get("name", ""), params.get("arguments", {}))
            send({"jsonrpc": "2.0", "id": mid, "result": res})


if __name__ == "__main__":
    main()
