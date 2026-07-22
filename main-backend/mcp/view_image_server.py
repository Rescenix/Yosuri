#!/usr/bin/env python3
# view_image_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：看图分析，支持多轮追问（history 参数带上上一轮问答，可以
# 针对同一张图"先看整体、再问细节"地连续提问）。
#
# 视觉模型调用（DashScope qwen-vl-max）和 API Key 都留在 Go 后端
# （internal/handler/vision.go 的 HandleVisionAnalyze），这里只是把
# stdio JSON-RPC 转成一次 HTTP 调用——Key 只在一处管理，Python 侧不用
# 再读一遍 .env，也不用在两边分别适配 DashScope 的请求格式。
#
# BACKEND_URL 环境变量可覆盖后端地址，默认 http://localhost:8080
# （与 main-backend/main.go 的默认监听端口、vite.config.js 的代理目标一致）。
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 与 main-backend/internal/handler/mcp_client.go 的握手完全同构，实现风格
# 照抄同目录下的 grep_server.py。

import json
import os
import sys

import httpx

BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:8080")


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {"content": [{"type": "text", "text": text}], "isError": is_error}


def do_view_image(image_url, image_base64, question, history):
    if not image_url and not image_base64:
        return tool_result("image_url 和 image_base64 至少提供一个", is_error=True)

    hist_list = []
    if history:
        try:
            hist_list = json.loads(history)
            if not isinstance(hist_list, list):
                raise ValueError("history 必须是数组")
        except (json.JSONDecodeError, ValueError) as e:
            return tool_result(f"history 参数不是合法 JSON 数组: {e}", is_error=True)

    body = {
        "image_url": image_url,
        "image_base64": image_base64,
        "question": question or "请详细描述这张图片的内容",
        "history": [{"q": h.get("q", ""), "a": h.get("a", "")} for h in hist_list if isinstance(h, dict)],
    }
    try:
        resp = httpx.post(f"{BACKEND_URL}/api/vision/analyze", json=body, timeout=60)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        return tool_result(f"视觉分析失败: {e}", is_error=True)

    data = resp.json()
    text = data.get("text", "")
    if not text:
        return tool_result(f"视觉分析未返回内容: {data}", is_error=True)
    return tool_result(text)


TOOLS = [
    {
        "name": "view_image",
        "description": (
            "查看并分析一张图片，返回文字描述。支持多轮追问——把上一次的问答对通过 "
            "history 传回来（JSON 数组字符串，如 '[{\"q\":\"这是什么应用？\",\"a\":\"代码编辑器...\"}]'），"
            "可以针对同一张图逐步深入提问（先问整体是什么，再问某个区域的细节）。"
            "image_url 和 image_base64 二选一。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "image_url": {"type": "string", "description": "图片的 URL，与 image_base64 二选一"},
                "image_base64": {"type": "string", "description": "图片的 base64 编码（不含 data: 前缀也可），与 image_url 二选一"},
                "question": {"type": "string", "description": "关于图片的问题，不填则给出通用描述"},
                "history": {"type": "string", "description": "之前的问答历史，JSON 数组字符串，用于多轮追问同一张图"},
            },
            "required": [],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "view_image":
        return do_view_image(
            args.get("image_url", ""),
            args.get("image_base64", ""),
            args.get("question", ""),
            args.get("history", ""),
        )
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
                    "serverInfo": {"name": "re0-view-image", "version": "1.0.0"},
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
