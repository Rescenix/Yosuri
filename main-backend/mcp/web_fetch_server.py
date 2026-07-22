#!/usr/bin/env python3
# web_fetch_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：抓取 URL 网页正文（去脚本/样式/导航），返回纯文本。
# 用于查看搜索结果里的文章原文，或读取用户给的链接内容——之前 agent 只能靠
# 训练数据里的旧知识回答"这个链接讲了什么"，接上这个之后能真读。
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 与 main-backend/internal/handler/mcp_client.go 的握手完全同构，实现风格
# 照抄同目录下的 grep_server.py（手写 JSON-RPC，不引入 mcp SDK 依赖）。

import json
import sys
from html.parser import HTMLParser

import httpx


class _TextExtractor(HTMLParser):
    _SKIP_TAGS = {"script", "style", "nav", "footer", "header", "noscript"}

    def __init__(self):
        super().__init__()
        self.chunks = []
        self._skip_depth = 0

    def handle_starttag(self, tag, attrs):
        if tag in self._SKIP_TAGS:
            self._skip_depth += 1

    def handle_endtag(self, tag):
        if tag in self._SKIP_TAGS and self._skip_depth > 0:
            self._skip_depth -= 1

    def handle_data(self, data):
        if self._skip_depth == 0:
            text = data.strip()
            if text:
                self.chunks.append(text)


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {"content": [{"type": "text", "text": text}], "isError": is_error}


def do_web_fetch(url: str, max_chars: int = 8000):
    if not url:
        return tool_result("缺少 url 参数", is_error=True)
    try:
        resp = httpx.get(url, headers={"User-Agent": "Mozilla/5.0"}, timeout=30, follow_redirects=True)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        return tool_result(f"抓取失败: {e}", is_error=True)

    extractor = _TextExtractor()
    try:
        extractor.feed(resp.text)
    except Exception as e:
        return tool_result(f"页面解析失败: {e}", is_error=True)

    text = "\n".join(extractor.chunks)
    if not text:
        return tool_result("页面未提取到文本内容（可能是纯 JS 渲染页面，抓不到正文）")
    if len(text) > max_chars:
        text = text[:max_chars] + "\n...(已截断)"
    return tool_result(text)


TOOLS = [
    {
        "name": "web_fetch",
        "description": (
            "抓取指定 URL 的网页正文（去除脚本/样式/导航栏），返回纯文本。"
            "用于查看搜索结果里的文章原文，或读取用户给的链接内容。"
            "不支持需要登录、或内容完全靠 JS 渲染出来的页面。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "要抓取的完整 URL，含 http(s):// 前缀"},
                "max_chars": {"type": "integer", "description": "返回文本的最大字符数，默认 8000，超出会截断"},
            },
            "required": ["url"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "web_fetch":
        max_chars = args.get("max_chars", 8000)
        try:
            max_chars = int(max_chars) if max_chars else 8000
        except (TypeError, ValueError):
            max_chars = 8000
        return do_web_fetch(args.get("url", ""), max_chars)
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
                    "serverInfo": {"name": "re0-web-fetch", "version": "1.0.0"},
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
