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

import atexit
import base64
import json
import os
import sys
import time

import httpx
from playwright.sync_api import sync_playwright

BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:8080")
# 优先本机 Chrome；没装 Chrome 但装了 Edge 的机器可以把这个改成 "msedge"
BROWSER_CHANNEL = os.environ.get("SCREENSHOT_BROWSER_CHANNEL", "chrome")

DEFAULT_QUESTION = "请详细分析这张网页截图：页面内容、布局结构、关键元素、可交互组件。"


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False, image=None):
    """Return normal MCP text plus an optional image artifact.

    The workflow host treats image content as an Agent-delivered artifact and
    publishes it into the current conversation.  It is deliberately part of
    the tool protocol rather than a UI action: whenever the Agent decides to
    take a screenshot, the evidence travels with that tool result.
    """
    content = [{"type": "text", "text": text}]
    if image:
        content.append({
            "type": "image",
            "data": base64.b64encode(image).decode("ascii"),
            "mimeType": "image/png",
        })
    return {"content": content, "isError": is_error}


# ---------------------------------------------------------------------------
# 浏览器生命周期
#
# 全进程共用一个 Playwright driver + 一个 browser。理由有两个：
#   1) Playwright 的同步 API 要求"谁创建谁使用"，跨线程会直接报错。MCP server 是
#      单线程 readline 循环，所有工具调用都在主线程，所以只要不自己起线程就安全——
#      这也是为什么空闲回收用"下次调用时惰性检查"，而不是后台定时器线程。
#   2) 一次性工具(page_check/screenshot)和交互会话共用同一个 browser 进程，
#      避免同时存在两个 driver 实例，也省掉反复启动 chromium 的几秒开销。
# ---------------------------------------------------------------------------
SESSION_IDLE_TIMEOUT = 15 * 60  # 空闲超过这么久就回收，别让 chromium 常驻到天荒地老

_pw = None
_browser = None
_session = None  # {"page":..., "console":[], "pageerr":[], "failed":[], "reported":int, "url":str}
_last_used = 0.0


def _ensure_browser():
    global _pw, _browser
    if _browser is None:
        _pw = sync_playwright().start()
        _browser = _pw.chromium.launch(channel=BROWSER_CHANNEL, headless=True)
    return _browser


def _shutdown():
    """进程退出时收干净，否则会留下孤儿 chromium。"""
    global _pw, _browser, _session
    try:
        if _session and _session.get("page"):
            _session["page"].close()
    except Exception:
        pass
    try:
        if _browser:
            _browser.close()
    except Exception:
        pass
    try:
        if _pw:
            _pw.stop()
    except Exception:
        pass
    _session, _browser, _pw = None, None, None


atexit.register(_shutdown)


def _close_session():
    global _session
    if _session and _session.get("page"):
        try:
            _session["page"].close()
        except Exception:
            pass
    _session = None


def _touch():
    global _last_used
    _last_used = time.time()


def _reap_if_idle():
    """惰性空闲回收：每次调用前看一眼，超时就把会话页丢掉（browser 留着复用）。"""
    if _session and _last_used and (time.time() - _last_used) > SESSION_IDLE_TIMEOUT:
        _close_session()


def _attach_listeners(page, bucket):
    page.on("console", lambda m: bucket["console"].append(m.text) if m.type == "error" else None)
    page.on("pageerror", lambda e: bucket["pageerr"].append(str(e)))
    page.on("requestfailed", lambda r: bucket["failed"].append(f"{(r.failure or '加载失败')} {r.url}"))


def capture(url: str) -> bytes:
    """一次性截图：用共享 browser 开一个临时页，用完就关。"""
    page = _ensure_browser().new_page(viewport={"width": 1280, "height": 800})
    try:
        page.goto(url, wait_until="networkidle", timeout=30000)
        return page.screenshot(full_page=False)
    finally:
        page.close()


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

    bucket = {"console": [], "pageerr": [], "failed": []}
    png_bytes, nav_note = None, ""

    try:
        page = _ensure_browser().new_page(viewport={"width": 1280, "height": 800})
        try:
            _attach_listeners(page, bucket)
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
            page.close()
    except Exception as e:
        return tool_result(f"浏览器启动或截图失败: {e}", is_error=True)

    console_errors, page_errors, failed_reqs = bucket["console"], bucket["pageerr"], bucket["failed"]
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

    return tool_result("\n".join(lines), image=png_bytes)


def do_screenshot(url: str, question: str):
    if not url:
        return tool_result("缺少 url 参数", is_error=True)
    try:
        png_bytes = capture(url)
    except Exception as e:
        return tool_result(f"截图失败: {e}", is_error=True)

    text, err = analyze(png_bytes, question)
    if err:
        # 图已成功取得，即使视觉分析服务暂时不可用，也应把 Agent 的截图交付出去。
        return tool_result(f"截图成功，但{err}", image=png_bytes)
    return tool_result(text, image=png_bytes)


# ---------------------------------------------------------------------------
# 交互式会话：browser_open -> click/fill/press -> snapshot -> ... -> close
# 页面在多次工具调用之间保活，所以模型可以"点一下、看一眼、再决定下一步"，
# 而不是必须提前把整串操作猜完。
# ---------------------------------------------------------------------------
def _require_session():
    _reap_if_idle()
    if not _session or not _session.get("page"):
        return None, tool_result(
            "还没有打开的页面。先调 browser_open 打开目标地址（会话空闲 15 分钟会自动回收，"
            "被回收后重新 open 即可）。", is_error=True)
    return _session, None


def _new_errors_text(sess, header_when_empty="本次操作没有新增报错"):
    """只报「自上次查看以来新增」的错误——判断"我刚点的这下有没有引入问题"时，
    把开局就有的历史错误重复念一遍只会淹没信号。"""
    all_errs = ([f"未捕获异常: {t}" for t in sess["pageerr"]]
                + [f"控制台错误: {t}" for t in sess["console"]]
                + [f"失败请求: {t}" for t in sess["failed"]])
    new = all_errs[sess["reported"]:]
    sess["reported"] = len(all_errs)
    if not new:
        return header_when_empty
    return f"新增 {len(new)} 条报错：\n" + "\n".join(f"  - {t}" for t in new[:10])


def do_browser_open(url: str):
    global _session
    if not url:
        return tool_result("缺少 url 参数", is_error=True)
    _reap_if_idle()
    _close_session()  # 一次只维护一个会话，避免多个页面同时活着说不清在操作谁
    try:
        page = _ensure_browser().new_page(viewport={"width": 1280, "height": 800})
        sess = {"page": page, "console": [], "pageerr": [], "failed": [], "reported": 0, "url": url}
        _attach_listeners(page, sess)
        note = ""
        try:
            page.goto(url, wait_until="networkidle", timeout=30000)
        except Exception as e:
            note = f"（加载未正常完成: {type(e).__name__}，可能 dev server 没起）"
        page.wait_for_timeout(300)
        _session = sess
        _touch()
        title = page.title()
        return tool_result(
            f"已打开 {url}{note}\n页面标题: {title}\n"
            f"{_new_errors_text(sess, '加载阶段没有报错')}\n"
            "接下来可用 browser_click / browser_fill / browser_press 操作，用 browser_snapshot 看效果。")
    except Exception as e:
        return tool_result(f"打开页面失败: {e}", is_error=True)


def _do_action(selector: str, fn, desc: str):
    sess, err = _require_session()
    if err:
        return err
    if not selector:
        return tool_result("缺少 selector 参数", is_error=True)
    try:
        fn(sess["page"])
    except Exception as e:
        # 选择器没匹配上是最常见的失败，单独给一句可操作的提示
        return tool_result(
            f"{desc}失败: {type(e).__name__}: {str(e)[:200]}\n"
            "如果是选择器没匹配到：可以用 browser_eval 执行 "
            "document.querySelectorAll('...').length 先确认元素存在，"
            "或改用 Playwright 的文本选择器（如 text=筛选）。", is_error=True)
    sess["page"].wait_for_timeout(400)  # 给动画/响应式更新一点时间
    _touch()
    return tool_result(f"{desc}成功。\n{_new_errors_text(sess)}\n（用 browser_snapshot 看当前页面效果）")


def do_browser_click(selector: str):
    return _do_action(selector, lambda p: p.click(selector, timeout=8000), f"点击 {selector}")


def do_browser_fill(selector: str, text: str):
    return _do_action(selector, lambda p: p.fill(selector, text or "", timeout=8000),
                      f"在 {selector} 填入文本")


def do_browser_press(key: str, selector: str = ""):
    sess, err = _require_session()
    if err:
        return err
    if not key:
        return tool_result("缺少 key 参数（如 Enter / Escape / Tab）", is_error=True)
    try:
        if selector:
            sess["page"].press(selector, key, timeout=8000)
        else:
            sess["page"].keyboard.press(key)
    except Exception as e:
        return tool_result(f"按键 {key} 失败: {type(e).__name__}: {str(e)[:200]}", is_error=True)
    sess["page"].wait_for_timeout(400)
    _touch()
    return tool_result(f"已按下 {key}。\n{_new_errors_text(sess)}")


def do_browser_eval(expression: str):
    """读 DOM / 计算样式。验证"网格是不是三列""按钮颜色对不对"这类断言时，
    比让视觉模型去数更准，也更省 token。"""
    sess, err = _require_session()
    if err:
        return err
    if not expression:
        return tool_result("缺少 expression 参数", is_error=True)
    try:
        value = sess["page"].evaluate(expression)
    except Exception as e:
        return tool_result(f"执行失败: {type(e).__name__}: {str(e)[:300]}", is_error=True)
    _touch()
    try:
        out = json.dumps(value, ensure_ascii=False, indent=2)[:3000]
    except (TypeError, ValueError):
        out = str(value)[:3000]
    return tool_result(f"结果:\n{out}")


def do_browser_snapshot(question: str):
    sess, err = _require_session()
    if err:
        return err
    try:
        png = sess["page"].screenshot(full_page=False)
    except Exception as e:
        return tool_result(f"截图失败: {e}", is_error=True)
    _touch()
    vision_text, vision_err = analyze(png, question)
    return tool_result(
        "【视觉】" + (vision_text if vision_text else f"(不可用：{vision_err})")
        + "\n【本次新增报错】" + _new_errors_text(sess, "无"), image=png)


def do_browser_close():
    if not _session:
        return tool_result("当前没有打开的会话。")
    _close_session()
    return tool_result("会话已关闭。")


TOOLS = [
    {
        "name": "screenshot",
        "description": (
            "不要用本工具截取内嵌预览面板或判断其当前交互状态；那是同一页面实例的场景，"
            "必须调用常驻的 capture_preview。"
            "本工具会启动独立的无头浏览器，因此只用于尚未在内嵌预览中打开的外部网页或全新页面检查。"
            "截图成功会直接作为一张图片插入当前聊天消息流，不要说无法贴图或让用户手动保存文件。"
            "打开一个网页并截图，然后分析截图内容（页面布局、关键元素、可交互组件）。"
            "适合查看无法用 web_fetch 读到正文的、依赖 JS 渲染的页面，或者需要看"
            "视觉呈现（不只是文字）的场景，比如检查一个网站的界面设计。成功截图会自动"
            "作为 Agent 的交付图片发布到当前聊天；需要给用户展示页面成果时可主动调用。"
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
    if name == "page_check":
        return do_page_check(args.get("url", ""), args.get("question", ""))
    if name == "browser_open":
        return do_browser_open(args.get("url", ""))
    if name == "browser_click":
        return do_browser_click(args.get("selector", ""))
    if name == "browser_fill":
        return do_browser_fill(args.get("selector", ""), args.get("text", ""))
    if name == "browser_press":
        return do_browser_press(args.get("key", ""), args.get("selector", ""))
    if name == "browser_snapshot":
        return do_browser_snapshot(args.get("question", ""))
    if name == "browser_eval":
        return do_browser_eval(args.get("expression", ""))
    if name == "browser_close":
        return do_browser_close()
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
