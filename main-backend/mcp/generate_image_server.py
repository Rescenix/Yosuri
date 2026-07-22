#!/usr/bin/env python3
# generate_image_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：根据文字描述生成图片。转发到 Go 后端已有的
# POST /api/image/generate（internal/handler/image_generate.go）——那边已经实现了
# 阿里云 qwen-image-plus 的异步任务创建 + 轮询 + 下载转存到本地 /images/ 的完整流程，
# 这里没有必要在 Python 侧重新写一遍 DashScope 异步任务协议和 Key 管理。
#
# BACKEND_URL 环境变量可覆盖后端地址，默认 http://localhost:8080。
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


def do_generate_image(prompt):
    if not prompt:
        return tool_result("缺少 prompt 参数", is_error=True)
    try:
        # 后端内部要创建 DashScope 异步任务再轮询到完成，最多等 60s，这里超时给够
        resp = httpx.post(f"{BACKEND_URL}/api/image/generate", json={"prompt": prompt}, timeout=90)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        return tool_result(f"图片生成失败: {e}", is_error=True)

    data = resp.json()
    url = data.get("url", "")
    if not url:
        return tool_result(f"图片生成未返回 URL: {data}", is_error=True)
    full_url = url if url.startswith("http") else f"{BACKEND_URL}{url}"
    return tool_result(f"图片已生成：{full_url}\n\n可以用 Markdown ![]({full_url}) 展示给用户。")


TOOLS = [
    {
        "name": "generate_image",
        "description": "根据文字描述生成一张图片，返回可直接展示的图片 URL。适合画架构图、插画、示意图等。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "prompt": {"type": "string", "description": "图片内容的详细文字描述，越具体生成效果越好"},
            },
            "required": ["prompt"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "generate_image":
        return do_generate_image(args.get("prompt", ""))
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
                    "serverInfo": {"name": "re0-generate-image", "version": "1.0.0"},
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
