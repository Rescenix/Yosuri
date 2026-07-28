"""Rescene Harness —— 长驻 Agent 运行时（对标 Hermes 的独立常驻模型）。

FastAPI 服务，端口 8001（HARNESS_PORT 可覆盖）。由 Go 后端经 HTTP 调用：
  GET /health                     存活探针
  GET /status                    运行时状态（uptime/MCP/事件/任务计数）
  GET /run_task?tool=X&args={}   执行任务：内建任务或 MCP 工具调用
  POST /debug/inject_error       注入合成 ERROR（验证日志自检链路）
"""

import json
import logging
import os
import sys
import time
from contextlib import asynccontextmanager
from datetime import datetime
from pathlib import Path

from fastapi import FastAPI, Query
from fastapi.responses import JSONResponse

BASE_DIR = Path(__file__).resolve().parent
LOGS_DIR = BASE_DIR / "logs"
LOGS_DIR.mkdir(exist_ok=True)

# ---- 日志：文件 + 控制台，文件是 sentinel 的监视对象 ----
# 显式挂到 root logger（不走 basicConfig：pytest 等宿主可能已抢先配置 root，
# basicConfig 会静默变 no-op，导致 harness.log 收不到日志）。
_fmt = logging.Formatter("%(asctime)s %(levelname)s %(name)s %(message)s")
_root = logging.getLogger()
_root.setLevel(logging.INFO)
if not any(getattr(h, "_harness_file", False) for h in _root.handlers):
    _file_handler = logging.FileHandler(LOGS_DIR / "harness.log", encoding="utf-8")
    _file_handler.setFormatter(_fmt)
    _file_handler._harness_file = True
    _console_handler = logging.StreamHandler(sys.stdout)
    _console_handler.setFormatter(_fmt)
    _root.addHandler(_file_handler)
    _root.addHandler(_console_handler)
logger = logging.getLogger("harness")

from mcp_client import MCPManager  # noqa: E402
from log_sentinel import LogSentinel  # noqa: E402

STARTED_AT = time.time()
mcp = MCPManager()
sentinel = LogSentinel()
_task_counter = {"run": 0, "ok": 0, "err": 0}

@asynccontextmanager
async def _lifespan(app):
    sentinel.start()
    logger.info("Rescene Harness 启动 pid=%s port=%s", os.getpid(), os.environ.get("HARNESS_PORT", "8001"))
    yield
    sentinel.stop()
    mcp.close()


app = FastAPI(title="Rescene Harness", version="1.0.0", lifespan=_lifespan)


@app.get("/health")
def health():
    return {"status": "ok", "service": "rescene-harness", "pid": os.getpid()}


@app.get("/status")
def status():
    return {
        "service": "rescene-harness",
        "pid": os.getpid(),
        "started_at": datetime.fromtimestamp(STARTED_AT).isoformat(timespec="seconds"),
        "uptime_s": round(time.time() - STARTED_AT, 1),
        "tasks": dict(_task_counter),
        "mcp": mcp.status(),
        "incidents": sentinel.snapshot(),
        "logs": {
            "harness": str(LOGS_DIR / "harness.log"),
            "incidents": str(LOGS_DIR / "incidents.log"),
            "watchdog": str(LOGS_DIR / "watchdog.log"),
        },
    }


# ---- 内建任务（不经 LLM 的确定性任务；MCP 工具之外的本地能力） ----

def _builtin_count_lines(args: dict):
    """读文件返回行数 —— 基准对照任务。文件读取走 MCP filesystem server，
    与 Hermes '用自己的工具层读文件' 对齐，而不是 harness 进程直接 open()。"""
    path = args.get("path")
    if not path:
        raise ValueError("count_lines 需要 path 参数")
    # server-filesystem 新版本把 read_file 改名 read_text_file，两个都试
    last_err = None
    for tool in ("read_text_file", "read_file"):
        try:
            result = mcp.call(tool, {"path": path})
            break
        except KeyError as e:
            last_err = e
    else:
        raise RuntimeError(f"filesystem server 没有可用的读文件工具: {last_err}")
    text = "".join(
        c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"
    )
    lines = text.splitlines()
    return {"path": path, "line_count": len(lines), "read_tool": tool}


BUILTIN_TASKS = {
    "count_lines": _builtin_count_lines,
}


@app.get("/run_task")
@app.post("/run_task")
def run_task(
    tool: str = Query(..., description="内建任务名或 MCP 工具名"),
    args: str = Query("{}", description="工具参数，JSON 字符串"),
):
    _task_counter["run"] += 1
    t0 = time.perf_counter()
    try:
        arguments = json.loads(args)
    except json.JSONDecodeError as e:
        _task_counter["err"] += 1
        return JSONResponse(status_code=400, content={"ok": False, "error": f"args 不是合法 JSON: {e}"})

    try:
        if tool in BUILTIN_TASKS:
            result = BUILTIN_TASKS[tool](arguments)
        else:
            result = mcp.call(tool, arguments)
        elapsed_ms = round((time.perf_counter() - t0) * 1000, 1)
        _task_counter["ok"] += 1
        logger.info("run_task tool=%s elapsed=%.1fms ok", tool, elapsed_ms)
        return {"ok": True, "tool": tool, "elapsed_ms": elapsed_ms, "result": result}
    except KeyError as e:
        _task_counter["err"] += 1
        return JSONResponse(status_code=404, content={"ok": False, "error": str(e)})
    except Exception as e:
        _task_counter["err"] += 1
        logger.error("run_task tool=%s 失败: %s", tool, e)
        return JSONResponse(status_code=500, content={"ok": False, "error": str(e)})


@app.post("/debug/inject_error")
def inject_error(message: str = Query("synthetic failure for log self-check")):
    """向自己的日志注入一条合成 ERROR，验证 sentinel 检测→记录→处置链路。"""
    logger.error("INJECTED %s", message)
    return {"ok": True, "injected": message, "hint": "轮询 /status 的 incidents.count 观察检测结果"}


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("HARNESS_PORT", "8001"))
    uvicorn.run(app, host="0.0.0.0", port=port, log_config=None)
