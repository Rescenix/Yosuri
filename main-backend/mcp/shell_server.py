#!/usr/bin/env python3
# shell_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：run —— 在项目根目录执行一条 shell 命令。
# 这是内置 execute_command 删除后、命令执行能力的 MCP 化替身：
# 内置版被从工作流过滤掉又没有 MCP 对应物，等于工作流跑不了任何命令；
# 现在统一走 MCP，主 Agent 用 load_tools 加载 mcp__shell__run 后即可执行。
#
# 安全：命令执行本身是高危操作，后端 approval.go 已把 mcp__shell__run 列入
# dangerousToolSet，Ask 模式下每次执行前都要人批准。cwd 锁死在 MCP_ROOT。
#
# 工作目录来自环境变量 MCP_ROOT（Go 后端拉起时注入 core.GetProjectRoot()），
# 跟着"主页选项目"动态变化。协议与 grep_server.py / mcp_client.go 完全同构。

import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(os.environ.get("MCP_ROOT", "C:\\Pro2026\\re0")).resolve()

# 单条命令最长执行时间，防止 agent 跑出一条卡死的命令把工作流永久阻塞
COMMAND_TIMEOUT_SEC = 120
# 输出字符上限（Go 侧 tool_output.go 还会再做首尾截断+落盘，这里只挡极端情况）
MAX_OUTPUT_CHARS = 40000


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {"content": [{"type": "text", "text": text}], "isError": is_error}


def do_run(command: str):
    command = (command or "").strip()
    if not command:
        return tool_result("run 需要 command 参数（要执行的 shell 命令）", is_error=True)

    # Windows 走 PowerShell（ls/cat/pwd 等有原生别名，不用像 cmd 那样翻译）；其它走 bash。
    if os.name == "nt":
        argv = ["powershell", "-NoProfile", "-Command", command]
    else:
        argv = ["bash", "-c", command]

    try:
        out = subprocess.run(
            argv, cwd=str(ROOT), capture_output=True, text=True,
            timeout=COMMAND_TIMEOUT_SEC, errors="replace",
        )
    except subprocess.TimeoutExpired:
        return tool_result(
            f"命令执行超时（>{COMMAND_TIMEOUT_SEC}s），已中止：{command}", is_error=True
        )
    except OSError as e:
        return tool_result(f"命令启动失败: {e}", is_error=True)

    # stdout + stderr 合并（很多工具把有用信息写在 stderr，如编译错误），
    # 保留退出码——非 0 时标 isError，让模型知道命令失败了。
    body = out.stdout or ""
    if out.stderr:
        body += ("\n" if body else "") + out.stderr
    if len(body) > MAX_OUTPUT_CHARS:
        body = body[:MAX_OUTPUT_CHARS] + f"\n...[输出超过 {MAX_OUTPUT_CHARS} 字符已截断]"
    header = f"$ {command}  (exit={out.returncode})\n"
    return tool_result(header + body, is_error=(out.returncode != 0))


TOOLS = [
    {
        "name": "run",
        "description": "在项目根目录执行一条 shell 命令并返回合并的 stdout+stderr 与退出码。Windows 用 PowerShell、其它用 bash；cwd 已是当前项目根，命令里用相对路径或 . 即可。适合跑 git/go build/npm 等构建与检查命令。高危操作，Ask 模式下需人工批准。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "要执行的 shell 命令，如 'git diff'、'go build ./...'、'npm run build'",
                },
            },
            "required": ["command"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "run":
        return do_run(args.get("command", ""))
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
                    "serverInfo": {"name": "re0-shell", "version": "1.0.0"},
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
