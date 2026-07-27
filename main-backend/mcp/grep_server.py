#!/usr/bin/env python3
# grep_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供三个工具，对齐 Claude Code 的 Grep / Glob / Read：
#   - grep       : 内容搜索，subprocess 调本机已装的 rg（ripgrep），零二进制依赖
#   - glob       : 文件名/路径模式匹配，标准库 pathlib 实现
#   - read_range : 按行号区间读文件（第 start-end 行）。MCP filesystem 的
#                  read_text_file 只有 head/tail（头/尾 N 行），读不了中间任意行段；
#                  这个工具补上"读第 100-150 行"的能力，避免为看几行而整文件塞进上下文。
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


def _display(p: Path) -> str:
    """项目根内的显示成相对路径，根外的照原样给绝对路径。
    根外路径直接 relative_to(ROOT) 会抛 ValueError —— 越界访问放开后这条路径可达了。"""
    try:
        return str(p.relative_to(ROOT))
    except ValueError:
        return str(p)


def do_glob(pattern: str, path: str = "."):
    from glob import glob as gglob
    target = (ROOT / path).resolve() if not os.path.isabs(path) else Path(path)
    base = str(target)
    # 支持 ** 递归：先按目录层展开
    matches = []
    if "**" in pattern:
        for p in target.rglob(pattern.split("**")[-1].lstrip("/")):
            if p.is_file():
                matches.append(_display(p))
    else:
        for m in gglob(os.path.join(base, pattern)):
            if os.path.isfile(m):
                matches.append(_display(Path(m)))
    if not matches:
        return tool_result(f"在 {target} 下未匹配到 '{pattern}'。")
    return tool_result("\n".join(sorted(matches)))


# 一次最多返回多少行，防止 read_range 被当成变相"读全文"把上下文撑爆
READ_RANGE_MAX_LINES = 400


def _safe_target(path: str):
    """把 path 解析成绝对路径。返回 (Path, err_text)。

    这里不再拦项目根之外的路径：越界与否交给 Go 侧审批闸门判断
    （main-backend/internal/handler/approval.go 的 toolOutsideRoot）——
    Ask 模式弹确认、Yolo 模式放行。以前在这层硬报错，批准了也读不到。
    """
    target = (ROOT / path).resolve() if not os.path.isabs(path) else Path(path).resolve()
    return target, None


def _to_int(v, default):
    try:
        return int(v)
    except (TypeError, ValueError):
        return default


def do_read_range(path: str, start=1, end=None):
    if not path:
        return tool_result("read_range 需要 path 参数", is_error=True)
    target, err = _safe_target(path)
    if err:
        return tool_result(err, is_error=True)
    if not target.is_file():
        return tool_result(f"文件不存在或不是普通文件: {path}", is_error=True)

    start = _to_int(start, 1)
    if start < 1:
        start = 1
    # end 缺省时默认读一屏（start 起 READ_RANGE_MAX_LINES 行）
    end = _to_int(end, start + READ_RANGE_MAX_LINES - 1)
    if end < start:
        end = start
    # 硬上限：区间再大也只返回 READ_RANGE_MAX_LINES 行
    if end - start + 1 > READ_RANGE_MAX_LINES:
        end = start + READ_RANGE_MAX_LINES - 1

    try:
        with open(target, encoding="utf-8", errors="replace") as f:
            lines = f.readlines()
    except OSError as e:
        return tool_result(f"读取失败: {e}", is_error=True)

    total = len(lines)
    if start > total:
        return tool_result(f"起始行 {start} 超出文件范围（{path} 共 {total} 行）", is_error=True)

    real_end = min(end, total)
    # 行号前缀 "N:内容"，与前端 readRows 的 /^(\d+):(.*)$/ 解析对齐，展开即带真实行号
    numbered = []
    for i, ln in enumerate(lines[start - 1:real_end]):
        numbered.append(f"{start + i}:{ln.rstrip(chr(10)).rstrip(chr(13))}")

    header = f"# {path} 第 {start}-{real_end} 行（共 {total} 行"
    if real_end < total:
        header += f"，还有 {total - real_end} 行未显示，续读用 start={real_end + 1}"
    header += "）"
    return tool_result(header + "\n" + "\n".join(numbered))


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
    {
        "name": "read_range",
        "description": "读取文件的指定行号区间（offset=start, limit=end-start+1），返回带行号的内容。用于只看大文件的某一段、而不是整文件读进上下文。start/end 从 1 开始编号（1-indexed），一次最多返回 400 行，越界自动裁剪；返回里会提示总行数和是否还有后续。**禁止用 mcp__fs__read_text_file 不加 head/tail 参数读全文**。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "文件路径，相对项目根或绝对路径（须在项目根内）"},
                "start": {"type": "integer", "description": "起始行号（1-indexed，闭区间），默认 1，对应 offset=start"},
                "end": {"type": "integer", "description": "结束行号（1-indexed，闭区间），必填！不传会报错。读一行传 start=N, end=N；读一段传 start=N, end=M。对应 limit=end-start+1"},
            },
            "required": ["path", "start", "end"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "grep":
        return do_grep(args.get("pattern", ""), args.get("path", "."), args.get("type", ""))
    if name == "glob":
        return do_glob(args.get("pattern", ""), args.get("path", "."))
    if name == "read_range":
        return do_read_range(args.get("path", ""), args.get("start", 1), args.get("end"))
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
