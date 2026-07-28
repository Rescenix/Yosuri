"""MCP stdio 客户端 —— 与 main-backend/internal/handler/mcp_client.go 同构。

手写 newline-delimited JSON-RPC 2.0 over stdio，不依赖官方 SDK：
initialize → notifications/initialized → tools/list → tools/call。

配置文件 mcp.json（与 Go 侧格式一致）：
  {"servers": {"fs": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "C:\\Pro2026\\re0"]}}}
"""

import json
import logging
import os
import queue
import shutil
import subprocess
import sys
import threading
from pathlib import Path

logger = logging.getLogger("harness.mcp")

BASE_DIR = Path(__file__).resolve().parent
DEFAULT_CONFIG = BASE_DIR / "mcp.json"
LOGS_DIR = BASE_DIR / "logs"


def _resolve_command(cmd: str) -> str:
    """Windows 上 npx 本体是 sh 脚本，CreateProcess 只认 .cmd/.exe。"""
    if os.name == "nt" and not cmd.lower().endswith((".cmd", ".exe", ".bat")):
        for ext in (".cmd", ".exe", ".bat"):
            found = shutil.which(cmd + ext)
            if found:
                return found
    return shutil.which(cmd) or cmd


class MCPConnection:
    """单个 MCP server 的常驻连接。请求按 id 关联，读线程派发。"""

    def __init__(self, name: str, command: str, args: list, env: list = None):
        self.name = name
        self.tools = []  # tools/list 的原始结果
        self._pending = {}  # id -> queue.Queue
        self._pending_lock = threading.Lock()
        self._write_lock = threading.Lock()
        self._next_id = 0
        self._alive = False

        full_env = dict(os.environ)
        for kv in env or []:
            k, _, v = kv.partition("=")
            full_env[k] = v

        LOGS_DIR.mkdir(exist_ok=True)
        self._stderr_file = open(LOGS_DIR / f"mcp_{name}_stderr.log", "ab")

        creationflags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
        resolved = _resolve_command(command)
        logger.info("启动 MCP server %r: %s %s", name, resolved, " ".join(args))
        self._proc = subprocess.Popen(
            [resolved] + args,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=self._stderr_file,
            env=full_env,
            creationflags=creationflags,
        )
        self._alive = True

        self._reader = threading.Thread(target=self._read_loop, daemon=True, name=f"mcp-{name}-reader")
        self._reader.start()

        self._handshake()

    # ---- 传输层 ----

    def _read_loop(self):
        for raw in self._proc.stdout:
            line = raw.decode("utf-8", errors="replace").strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                continue
            msg_id = msg.get("id")
            if msg_id is None:
                continue  # 通知，忽略
            payload = msg.get("result")
            if payload is None and "error" in msg:
                payload = {"__mcp_error": msg["error"]}
            with self._pending_lock:
                q = self._pending.pop(msg_id, None)
            if q is not None:
                q.put(payload)
        self._alive = False
        logger.warning("MCP server %r stdout 关闭（进程退出）", self.name)

    def _request(self, method: str, params: dict, timeout: float = 30.0):
        with self._write_lock:
            self._next_id += 1
            req_id = self._next_id
        q = queue.Queue(maxsize=1)
        with self._pending_lock:
            self._pending[req_id] = q
        body = json.dumps({"jsonrpc": "2.0", "id": req_id, "method": method, "params": params})
        with self._write_lock:
            self._proc.stdin.write(body.encode("utf-8") + b"\n")
            self._proc.stdin.flush()
        try:
            payload = q.get(timeout=timeout)
        except queue.Empty:
            with self._pending_lock:
                self._pending.pop(req_id, None)
            raise TimeoutError(f"MCP {self.name}.{method} 超时（{timeout}s）")
        if isinstance(payload, dict) and "__mcp_error" in payload:
            raise RuntimeError(f"MCP 错误: {json.dumps(payload['__mcp_error'], ensure_ascii=False)}")
        return payload

    def _notify(self, method: str, params: dict):
        body = json.dumps({"jsonrpc": "2.0", "method": method, "params": params})
        with self._write_lock:
            self._proc.stdin.write(body.encode("utf-8") + b"\n")
            self._proc.stdin.flush()

    # ---- 协议层 ----

    def _handshake(self):
        self._request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "rescene-harness", "version": "1.0.0"},
        }, timeout=60.0)  # 首次 npx 要下包，放宽
        self._notify("notifications/initialized", {})
        result = self._request("tools/list", {}, timeout=30.0)
        self.tools = result.get("tools", [])
        logger.info("MCP server %r 已接入，%d 个工具: %s",
                    self.name, len(self.tools), [t["name"] for t in self.tools])

    def call_tool(self, tool: str, arguments: dict, timeout: float = 30.0):
        result = self._request("tools/call", {"name": tool, "arguments": arguments}, timeout=timeout)
        return result

    @property
    def alive(self) -> bool:
        return self._alive and self._proc.poll() is None

    def close(self):
        try:
            self._proc.terminate()
        except Exception:
            pass
        try:
            self._stderr_file.close()
        except Exception:
            pass


class MCPManager:
    """读 mcp.json，为每个 server 建常驻连接；工具名注册为 mcp__<server>__<tool>，
    同时裸工具名（无歧义时）也可直接路由 —— /run_task?tool=list_directory 更顺手。"""

    def __init__(self, config_path: Path = DEFAULT_CONFIG):
        self.config_path = config_path
        self.connections = {}
        self.routes = {}  # 完整工具名/裸工具名 -> (conn, 真实工具名)
        self._init_lock = threading.Lock()
        self._initialized = False

    def ensure_started(self):
        with self._init_lock:
            if self._initialized:
                return
            self._initialized = True
            self._start_all()

    def _start_all(self):
        if not self.config_path.exists():
            logger.warning("MCP 配置 %s 不存在，MCP 生态不参与", self.config_path)
            return
        cfg = json.loads(self.config_path.read_text(encoding="utf-8"))
        for name, sc in (cfg.get("servers") or {}).items():
            try:
                conn = MCPConnection(name, sc["command"], sc.get("args", []), sc.get("env"))
            except Exception as e:
                logger.error("MCP server %r 启动失败: %s", name, e)
                continue
            self.connections[name] = conn
            for t in conn.tools:
                real = t["name"]
                self.routes[f"mcp__{name}__{real}"] = (conn, real)
                # 裸名路由：冲突时后到的不覆盖
                self.routes.setdefault(real, (conn, real))

    def call(self, tool: str, arguments: dict, timeout: float = 30.0):
        self.ensure_started()
        entry = self.routes.get(tool)
        if entry is None:
            raise KeyError(f"未知 MCP 工具: {tool}（可用: {sorted(self.routes)}）")
        conn, real = entry
        if not conn.alive:
            raise RuntimeError(f"MCP server {conn.name!r} 进程已退出")
        return conn.call_tool(real, arguments, timeout=timeout)

    def status(self):
        return {
            "initialized": self._initialized,
            "servers": {
                name: {"alive": conn.alive, "tools": [t["name"] for t in conn.tools]}
                for name, conn in self.connections.items()
            },
        }

    def close(self):
        for conn in self.connections.values():
            conn.close()
