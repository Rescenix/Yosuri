#!/usr/bin/env python3
# screenshot_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：打开一个网页，截图，然后用视觉模型分析截图内容。
#
# 浏览器用 Playwright 驱动本机已装的 Chrome（channel="chrome"），不走
# `playwright install` 下载独立的 Chromium 内核——省了几百 MB 的下载和一份
# 多余的浏览器安装。要求本机装有 Chrome 或 Edge（channel 也支持 "msedge"）。
#
# 视觉分析复用 view_image 同一条路径：POST 到 Go 后端的 /api/vision/analyze
# （internal/handler/vision.go），Key 和模型调用都留在那一处，这里只管"渲染出
# 一张图"这一件事。截图直接以 base64 传给后端，不落盘、不需要额外的静态托管。
#
# BACKEND_URL 环境变量可覆盖后端地址，默认 http://localhost:8080。
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 与 main-backend/internal/handler/mcp_client.go 的握手完全同构，实现风格
# 照抄同目录下的 grep_server.py。

import base64
import json
import os
import sys

import httpx
from playwright.sync_api import sync_playwright

BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:8080")
# 优先本机 Chrome；没装 Chrome 但装了 Edge 的机器可以把这个改成 "msedge"
BROWSER_CHANNEL = os.environ.get("SCREENSHOT_BROWSER_CHANNEL", "chrome")

DEFAULT_QUESTION = "请详细分析这张网页截图：页面内容、布局结构、关键元素、可交互组件。"


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {"content": [{"type": "text", "text": text}], "isError": is_error}


def capture(url: str) -> bytes:
    with sync_playwright() as p:
        browser = p.chromium.launch(channel=BROWSER_CHANNEL, headless=True)
        try:
            page = browser.new_page(viewport={"width": 1280, "height": 800})
            page.goto(url, wait_until="networkidle", timeout=30000)
            return page.screenshot(full_page=False)
        finally:
            browser.close()


def do_screenshot(url: str, question: str):
    if not url:
        return tool_result("缺少 url 参数", is_error=True)
    try:
        png_bytes = capture(url)
    except Exception as e:
        return tool_result(f"截图失败: {e}", is_error=True)

    img_b64 = base64.b64encode(png_bytes).decode()
    body = {"image_base64": img_b64, "question": question or DEFAULT_QUESTION}
    try:
        resp = httpx.post(f"{BACKEND_URL}/api/vision/analyze", json=body, timeout=60)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        return tool_result(f"截图成功但视觉分析失败: {e}", is_error=True)

    data = resp.json()
    text = data.get("text", "")
    if not text:
        return tool_result(f"视觉分析未返回内容: {data}", is_error=True)
    return tool_result(text)


TOOLS = [
    {
        "name": "screenshot",
        "description": (
            "打开一个网页并截图，然后分析截图内容（页面布局、关键元素、可交互组件）。"
            "适合查看无法用 web_fetch 读到正文的、依赖 JS 渲染的页面，或者需要看"
            "视觉呈现（不只是文字）的场景，比如检查一个网站的界面设计。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "要截图的完整 URL，含 http(s):// 前缀"},
                "question": {"type": "string", "description": "关于这个页面想了解什么，不填则给出通用的页面分析"},
            },
            "required": ["url"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "screenshot":
        return do_screenshot(args.get("url", ""), args.get("question", ""))
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
                    "serverInfo": {"name": "re0-screenshot", "version": "1.0.0"},
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
