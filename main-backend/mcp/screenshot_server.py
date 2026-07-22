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


def analyze(png_bytes: bytes, question: str):
    """把截图送到后端视觉端点，返回 (文字, 错误)。"""
    img_b64 = base64.b64encode(png_bytes).decode()
    body = {"image_base64": img_b64, "question": question or DEFAULT_QUESTION}
    try:
        # 比后端自己的视觉超时(60s)+回退留一点余量，但要明显小于 MCP 层的 180s，
        # 这样视觉再慢也只是丢掉【视觉】那一段，console 错误照样能回给模型。
        resp = httpx.post(f"{BACKEND_URL}/api/vision/analyze", json=body, timeout=140)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        return None, f"视觉分析失败: {e}"
    text = resp.json().get("text", "")
    if not text:
        return None, "视觉分析未返回内容"
    return text, None


def do_page_check(url: str, question: str):
    """打开页面并做一次完整自检：截图视觉分析 + console 错误 + 失败请求。

    单纯截图看不出前端最常见的坏法——JS 抛异常导致白屏、组件路径写错 404。
    视觉模型只会说"页面是空白的"，说不出为什么。所以这里在导航前挂上三个
    监听器，把"看起来怎么样"和"底下报了什么错"一次性给模型。
    """
    if not url:
        return tool_result("缺少 url 参数", is_error=True)

    console_errors, page_errors, failed_reqs = [], [], []
    png_bytes, nav_note = None, ""

    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(channel=BROWSER_CHANNEL, headless=True)
            try:
                page = browser.new_page(viewport={"width": 1280, "height": 800})
                page.on("console", lambda m: console_errors.append(m.text) if m.type == "error" else None)
                page.on("pageerror", lambda e: page_errors.append(str(e)))
                page.on("requestfailed", lambda r: failed_reqs.append(
                    f"{(r.failure or '加载失败')} {r.url}"))

                try:
                    page.goto(url, wait_until="networkidle", timeout=30000)
                except Exception as e:
                    # 超时不当致命错：dev server 没起是很常见的情况，把它作为
                    # 结论回给模型，让它自己去起服务，而不是整轮工具调用失败。
                    nav_note = f"页面加载未正常完成（{type(e).__name__}），可能是 dev server 没启动或地址不对。"
                # 框架挂载/首屏动画留点时间，否则常截到还没渲染的空壳
                page.wait_for_timeout(500)
                png_bytes = page.screenshot(full_page=False)
            finally:
                browser.close()
    except Exception as e:
        return tool_result(f"浏览器启动或截图失败: {e}", is_error=True)

    vision_text, vision_err = analyze(png_bytes, question) if png_bytes else (None, "没有截到图")

    lines = []
    lines.append("【视觉】" + (vision_text if vision_text else f"(不可用：{vision_err})"))

    # 空结果也要显式写出来。只在有错时才提的话，模型会把"没提到"读成"没检查"。
    if page_errors:
        lines.append(f"【未捕获异常】{len(page_errors)} 条：")
        lines += [f"  - {t}" for t in page_errors[:10]]
    else:
        lines.append("【未捕获异常】无")

    if console_errors:
        lines.append(f"【控制台错误】{len(console_errors)} 条：")
        lines += [f"  - {t}" for t in console_errors[:10]]
    else:
        lines.append("【控制台错误】无")

    if failed_reqs:
        lines.append(f"【失败请求】{len(failed_reqs)} 条：")
        lines += [f"  - {t}" for t in failed_reqs[:10]]
    else:
        lines.append("【失败请求】无")

    if nav_note:
        conclusion = nav_note
    elif page_errors or console_errors or failed_reqs:
        conclusion = "页面存在运行时错误，需要修复后重新检查。"
    else:
        conclusion = "未发现运行时错误。"
    lines.append("【结论】" + conclusion)

    return tool_result("\n".join(lines))


def do_screenshot(url: str, question: str):
    if not url:
        return tool_result("缺少 url 参数", is_error=True)
    try:
        png_bytes = capture(url)
    except Exception as e:
        return tool_result(f"截图失败: {e}", is_error=True)

    text, err = analyze(png_bytes, question)
    if err:
        return tool_result(f"截图成功但{err}", is_error=True)
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
    {
        "name": "page_check",
        "description": (
            "前端自检：打开本地页面，同时拿到「渲染成什么样」和「控制台报了什么错」。"
            "改完前端代码后用它验证效果——比 screenshot 多返回未捕获异常、console 错误、"
            "失败请求，能查出 JS 报错白屏、组件路径写错 404 这类光看截图发现不了的问题。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "要检查的页面 URL，例如 http://localhost:4322/"},
                "question": {"type": "string", "description": "想重点确认的视觉效果，比如「卡片有没有正确排成三列网格」"},
            },
            "required": ["url"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "screenshot":
        return do_screenshot(args.get("url", ""), args.get("question", ""))
    if name == "page_check":
        return do_page_check(args.get("url", ""), args.get("question", ""))
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
