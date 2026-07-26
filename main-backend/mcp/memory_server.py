#!/usr/bin/env python3
# memory_server.py —— re0 自研记忆 MCP server（stdio / JSON-RPC 2.0）
#
# 让 LLM 主动把「值得跨对话记住」的东西写进 ~/rescene_data/MEMORY.md。
# 设计原则（来自用户决策）：
#   - 绝不自动写。只有 LLM 显式调用工具才落盘——避免收尾时把一堆垃圾灌进记忆。
#   - 这是 MEMORY.md 的唯一运行时写入者。后端 swiftnet 单例只在进程启动时
#     load 一次，运行时注入用的是内存态；本 server 改文件、下次重启后端会 reload，
#     因此不存在双写者的并发冲突。
#   - 文件格式与 backend/internal/swiftnet 严格兼容（四个区：pinned/handoff/inbox/mem），
#     这样后端 swiftnet 启动 load 后能正确解析。
#
# 工具：
#   - memory_read    读回当前 MEMORY.md 全文（模型先读再写，避免重复）
#   - memory_append 写 [mem] 事实库（带轻量防重，text 完全一致则不重复加）
#   - memory_pin    写 [pinned] 身份/常驻记忆（Pxx|Cluster|Text）
#   - memory_handoff 重写 [handoff] 工作态（这次做到哪了 / 下次从哪接）
#   - memory_search 按关键词匹配已存记忆行
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 与 main-backend/internal/handler/mcp_client.go 的握手完全同构。

import json
import os
import re
import sys
import threading
from pathlib import Path

HOME = Path(os.environ.get("USERPROFILE", os.path.expanduser("~")))
MEM_FILE = HOME / "rescene_data" / "MEMORY.md"

# 项目级工作目录笔记：按 MCP_ROOT（后端注入的当前项目根绝对路径）的目录名隔离，
# 落在 ~/rescene_data/projects/<项目名>/workdir.md，不污染 repo 本身。
# 与全局 MEMORY.md 的区别：MEMORY.md 是用户/系统级常驻；workdir.md 是「这个项目里
# 当前在做什么、关键上下文、待办、约定」的按项目记录，跨对话保留、切项目互不串台。
ROOT = Path(os.environ.get("MCP_ROOT", "C:\\Pro2026\\re0")).resolve()
PROJ_NAME = ROOT.name or "default"
WORKDIR_FILE = HOME / "rescene_data" / "projects" / PROJ_NAME / "workdir.md"

# 进程内锁：同一进程内并发 tools/call 串行化（多工具并行调用时保护文件读改写）。
_lock = threading.Lock()

HEADER = "# MEMORY.md — 雨燕神经网络：一个文件，四个区，一个写入者"
PINNED_HEAD = "[pinned] ← 无条件注入，≤150 tok，只改不删"
HANDOFF_HEAD = "[handoff] ← 无条件注入，会话末尾整段重写，硬上限 200 tok"
INBOX_HEAD = "[inbox] ← 跨agent收件箱，无条件注入，不靠语义召回（解决'每次都要说'）"
MEM_HEAD = "[mem] ← 选择器召回，追加+原位touch；Expand 按需"

ID_RE = re.compile(r"^(0x[0-9a-f]+|P\d+)\|(.*)$")


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {
        "content": [{"type": "text", "text": text}],
        "isError": is_error,
    }


# ====================== 文件解析 / 渲染 ======================

def load_sections():
    """读 MEMORY.md，返回 {pinned:[...], handoff:str, inbox:[...], mem:[...]}。
    行格式：mem 区 = '0xxx|Cluster|Keywords|Text'；pinned 区 = 'Pxx|Cluster|Text'。
    文件不存在时返回空结构（调用方 render 时照常写出四区骨架）。"""
    secs = {"pinned": [], "handoff": "", "inbox": [], "mem": []}
    if not MEM_FILE.exists():
        return secs
    cur = None
    handoff_lines = []
    for line in MEM_FILE.read_text(encoding="utf-8").split("\n"):
        stripped = line.strip()
        low = stripped.lower()
        if low.startswith("[pinned]"):
            cur = "pinned"
            continue
        if low.startswith("[handoff]"):
            cur = "handoff"
            continue
        if low.startswith("[inbox]"):
            cur = "inbox"
            continue
        if low.startswith("[mem]"):
            cur = "mem"
            continue
        if cur == "handoff":
            handoff_lines.append(line)
            continue
        if cur in ("pinned", "inbox", "mem") and stripped:
            secs[cur].append(stripped)
    secs["handoff"] = "\n".join(handoff_lines).strip()
    return secs


def render(secs):
    """把四区拼回 MEMORY.md 文本（与 swiftnet.render 同构）。"""
    out = [HEADER, "", PINNED_HEAD]
    out += secs["pinned"]
    out.append("")
    out.append(HANDOFF_HEAD)
    if secs["handoff"]:
        out.append(secs["handoff"])
    out.append("")
    out.append(INBOX_HEAD)
    out += secs["inbox"]
    out.append("")
    out.append(MEM_HEAD)
    out += secs["mem"]
    return "\n".join(out).rstrip("\n") + "\n"


def atomic_write(text):
    MEM_FILE.parent.mkdir(parents=True, exist_ok=True)
    tmp = MEM_FILE.with_suffix(".tmp")
    tmp.write_text(text, encoding="utf-8")
    tmp.replace(MEM_FILE)


# ====================== 工具实现 ======================

def do_read():
    if not MEM_FILE.exists():
        return tool_result("(MEMORY.md 尚不存在，调用 memory_append/pin/handoff 会创建它)")
    return tool_result(MEM_FILE.read_text(encoding="utf-8"))


def _gen_id(text):
    import hashlib
    h = int(hashlib.sha256(text.encode("utf-8")).hexdigest()[:8], 16)
    return "0x%x" % h


def do_append(text, cluster="mem", keywords=""):
    text = (text or "").strip()
    if not text:
        return tool_result("memory_append 需要非空 text", is_error=True)
    if not keywords:
        keywords = text
    with _lock:
        secs = load_sections()
        # 轻量防重：text 完全一致（忽略空白）则不重复加
        norm = re.sub(r"\s+", " ", text).lower()
        for line in secs["mem"]:
            body = line.split("|", 3)[-1] if "|" in line else line
            if re.sub(r"\s+", " ", body.strip()).lower() == norm:
                return tool_result(f"已存在相同记忆，未重复写入：{body.strip()}")
        node = f"{_gen_id(text)}|{cluster}|{keywords}|{text}"
        secs["mem"].append(node)
        atomic_write(render(secs))
    return tool_result(f"已写入 [mem] 区：{node}")


def do_pin(pid, cluster, text):
    pid = (pid or "").strip()
    text = (text or "").strip()
    if not pid or not text:
        return tool_result("memory_pin 需要 pid（如 P03）与 text", is_error=True)
    with _lock:
        secs = load_sections()
        replaced = False
        for i, line in enumerate(secs["pinned"]):
            if line.split("|", 1)[0].strip() == pid:
                secs["pinned"][i] = f"{pid}|{cluster}|{text}"
                replaced = True
                break
        if not replaced:
            secs["pinned"].append(f"{pid}|{cluster}|{text}")
        atomic_write(render(secs))
    return tool_result(f"已写入 [pinned] 区：{pid}|{cluster}|{text}")


def do_handoff(block):
    block = (block or "").strip()
    with _lock:
        secs = load_sections()
        secs["handoff"] = block
        atomic_write(render(secs))
    return tool_result(f"已重写 [handoff] 工作态（{len(block.splitlines())} 行）")


def do_search(query):
    q = (query or "").strip().lower()
    if not q:
        return tool_result("memory_search 需要 query", is_error=True)
    secs = load_sections()
    hits = []
    for line in secs["pinned"] + secs["mem"] + secs["inbox"]:
        if q in line.lower():
            hits.append(line)
    if not hits:
        return tool_result(f"未找到与 '{query}' 相关的记忆。")
    return tool_result("\n".join(hits))


# ====================== 项目级 workdir.md ======================

def do_workdir_read():
    if not WORKDIR_FILE.exists():
        return tool_result(f"(本项目 [{PROJ_NAME}] 暂无 workdir.md，调用 workdir_write/append 会创建它)")
    return tool_result(f"# 项目 {PROJ_NAME} 工作目录笔记\n\n{WORKDIR_FILE.read_text(encoding='utf-8')}")


def do_workdir_write(block):
    block = (block or "").strip()
    if not block:
        return tool_result("workdir_write 需要非空 block", is_error=True)
    with _lock:
        WORKDIR_FILE.parent.mkdir(parents=True, exist_ok=True)
        tmp = WORKDIR_FILE.with_suffix(".tmp")
        tmp.write_text(block, encoding="utf-8")
        tmp.replace(WORKDIR_FILE)
    return tool_result(f"已整段重写 [{PROJ_NAME}] 的 workdir.md（{len(block.splitlines())} 行）")


def do_workdir_append(block):
    block = (block or "").strip()
    if not block:
        return tool_result("workdir_append 需要非空 block", is_error=True)
    with _lock:
        WORKDIR_FILE.parent.mkdir(parents=True, exist_ok=True)
        existing = WORKDIR_FILE.read_text(encoding="utf-8") if WORKDIR_FILE.exists() else ""
        if existing and not existing.endswith("\n"):
            existing += "\n"
        WORKDIR_FILE.write_text(existing + block + "\n", encoding="utf-8")
    return tool_result(f"已追加到 [{PROJ_NAME}] 的 workdir.md")


TOOLS = [
    {
        "name": "memory_read",
        "description": "读回当前长期记忆文件（~/rescene_data/MEMORY.md）全文。写之前先读，避免重复写入已有记忆。返回四个区：pinned（身份常驻）/ handoff（工作态交接）/ mem（事实库）/ inbox（跨agent收件）。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "memory_append",
        "description": "写入一条事实记忆到 [mem] 区（跨对话保留，下次会话自动注入系统提示词）。用于：用户偏好、项目决策、踩过的坑、API 约定等值得长期记住的东西。text 为记忆内容；cluster 建议分类（如 UserBase/CodeWork/Decisions，默认 mem）；keywords 可选，铺同义改述利于召回（如 '风险偏好/risk appetite'）。text 与已有记忆完全一致时不会重复写。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "text": {"type": "string", "description": "要记住的内容"},
                "cluster": {"type": "string", "description": "分类标签，如 UserBase/CodeWork/Decisions，默认 mem"},
                "keywords": {"type": "string", "description": "可选，同义关键词，用 / 分隔，利于语义召回"},
            },
            "required": ["text"],
        },
    },
    {
        "name": "memory_pin",
        "description": "写入/更新一条身份常驻记忆到 [pinned] 区（无条件注入，≤150 tok，只改不删）。用于用户身份、项目定位等每次对话都要在的东西。pid 形如 P03（相同 pid 会原位覆盖）；cluster 分类；text 内容。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "pid": {"type": "string", "description": "记忆编号，如 P03；相同则覆盖更新"},
                "cluster": {"type": "string", "description": "分类标签，如 UserBase"},
                "text": {"type": "string", "description": "常驻内容"},
            },
            "required": ["pid", "text"],
        },
    },
    {
        "name": "memory_handoff",
        "description": "重写 [handoff] 工作态：本次会话做到哪了、下次从哪接（硬上限 200 tok，超出自动截断保留最近）。在长任务中途或结束时调用，让下一次对话能无缝续上『上次做到一半的事』。整段重写不追加。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "block": {"type": "string", "description": "工作态交接文本，说明当前进度与下一步"},
            },
            "required": ["block"],
        },
    },
    {
        "name": "memory_search",
        "description": "按关键词匹配已存记忆（pinned/mem/inbox 区），返回命中的原始行。用于写之前自查『这条我是不是已经记过了』，或回忆某个具体事实。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "要检索的关键词"},
            },
            "required": ["query"],
        },
    },
    {
        "name": "workdir_read",
        "description": "读回当前项目的 workdir.md（~/rescene_data/projects/<项目名>/workdir.md，不污染 repo）。这是按项目隔离的工作目录笔记：记录『这个项目现在在做什么、关键上下文、待办、约定』，跨对话保留。切项目自动跟随后端 MCP_ROOT，互不串台。写之前先读。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "workdir_write",
        "description": "整段重写当前项目的 workdir.md（覆盖式）。适合在任务开始/大节点时记录项目整体状态、当前目标、关键约束。block 为完整 markdown 文本。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "block": {"type": "string", "description": "工作目录笔记的完整内容（markdown）"},
            },
            "required": ["block"],
        },
    },
    {
        "name": "workdir_append",
        "description": "向当前项目的 workdir.md 末尾追加一段（不覆盖已有内容）。适合随时补一条『刚发现的关键事实 / 下一步待办 / 踩过的坑』。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "block": {"type": "string", "description": "要追加的内容（markdown，可多行）"},
            },
            "required": ["block"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "memory_read":
        return do_read()
    if name == "memory_append":
        return do_append(args.get("text", ""), args.get("cluster", "mem"), args.get("keywords", ""))
    if name == "memory_pin":
        return do_pin(args.get("pid", ""), args.get("cluster", ""), args.get("text", ""))
    if name == "memory_handoff":
        return do_handoff(args.get("block", ""))
    if name == "memory_search":
        return do_search(args.get("query", ""))
    if name == "workdir_read":
        return do_workdir_read()
    if name == "workdir_write":
        return do_workdir_write(args.get("block", ""))
    if name == "workdir_append":
        return do_workdir_append(args.get("block", ""))
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
                    "serverInfo": {"name": "re0-memory", "version": "1.0.0"},
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
