"""Harness 测试套件。

覆盖：/health、/status、/run_task 参数校验与未知工具、
ERROR 注入→sentinel 检测闭环、真实 MCP filesystem server 集成。
MCP 集成测试真实拉起 npx server-filesystem —— 慢但真。
"""

import json
import sys
import time
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

BASE_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(BASE_DIR))

import app as harness_app  # noqa: E402


@pytest.fixture(scope="module")
def client():
    with TestClient(harness_app.app) as c:
        yield c


def test_health_200(client):
    r = client.get("/health")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok"
    assert body["service"] == "rescene-harness"


def test_status_fields(client):
    r = client.get("/status")
    assert r.status_code == 200
    body = r.json()
    for key in ("pid", "uptime_s", "tasks", "mcp", "incidents", "logs"):
        assert key in body, f"/status 缺字段 {key}"


def test_run_task_bad_args_400(client):
    r = client.get("/run_task", params={"tool": "count_lines", "args": "{not json"})
    assert r.status_code == 400
    assert r.json()["ok"] is False


def test_inject_error_detected_by_sentinel(client):
    before = client.get("/status").json()["incidents"]["count"]
    r = client.post("/debug/inject_error", params={"message": "pytest 合成错误"})
    assert r.status_code == 200
    # sentinel 轮询间隔 0.5s，给 10s 窗口
    deadline = time.time() + 10
    count = before
    while time.time() < deadline:
        count = client.get("/status").json()["incidents"]["count"]
        if count > before:
            break
        time.sleep(0.3)
    assert count > before, "sentinel 未在 10s 内检测到注入的 ERROR"
    # incidents.log 落盘校验
    incidents_log = BASE_DIR / "logs" / "incidents.log"
    assert incidents_log.exists()
    last = json.loads(incidents_log.read_text(encoding="utf-8").strip().splitlines()[-1])
    assert "pytest 合成错误" in last["log_line"]
    assert last["action"] == "preset:record_and_mark"


# ---- 真实 MCP 集成 ----

def test_mcp_list_directory_real(client):
    args = json.dumps({"path": "C:\\Pro2026\\re0"})
    r = client.get("/run_task", params={"tool": "list_directory", "args": args})
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["ok"] is True
    text = "".join(c.get("text", "") for c in body["result"]["content"])
    # 仓库里必然存在的目录
    assert "main-backend" in text
    assert "harness" in text


def test_builtin_count_lines_matches_local_read(client):
    readme = Path("C:/Pro2026/re0/README.md")
    expected = len(readme.read_text(encoding="utf-8").splitlines())
    args = json.dumps({"path": "C:\\Pro2026\\re0\\README.md"})
    r = client.get("/run_task", params={"tool": "count_lines", "args": args})
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["ok"] is True
    assert body["result"]["line_count"] == expected


def test_unknown_tool_404(client):
    r = client.get("/run_task", params={"tool": "no_such_tool_xyz", "args": "{}"})
    assert r.status_code == 404
    assert r.json()["ok"] is False
