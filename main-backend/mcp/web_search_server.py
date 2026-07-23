#!/usr/bin/env python3
# web_search_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：联网搜索。默认用 Bing Search API（稳定、每月 1000 次免费额度），
# 通过环境变量 BING_SEARCH_API_KEY 配置；未配置时返回清晰错误。
#
# 协议：initialize -> notifications/initialized -> tools/list -> tools/call
# 实现风格照抄同目录 web_fetch_server.py / grep_server.py。

import json
import os
import sys

import httpx

BING_ENDPOINT = "https://api.bing.microsoft.com/v7.0/search"
BING_KEY = os.environ.get("BING_SEARCH_API_KEY", "").strip()


def send(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False):
    return {"content": [{"type": "text", "text": text}], "isError": is_error}


def do_web_search(query: str, count: int = 5):
    if not query:
        return tool_result("缺少 query 参数", is_error=True)
    if not BING_KEY:
        return tool_result(
            "未配置 BING_SEARCH_API_KEY 环境变量。"
            "请去 Azure Portal 创建 Bing Search v7 资源，把密钥配置到环境变量后重启 backend。",
            is_error=True,
        )

    count = max(1, min(int(count), 10))
    try:
        resp = httpx.get(
            BING_ENDPOINT,
            headers={"Ocp-Apim-Subscription-Key": BING_KEY},
            params={"q": query, "count": count, "mkt": "zh-CN", "setLang": "zh"},
            timeout=20,
        )
        resp.raise_for_status()
        data = resp.json()
    except httpx.HTTPError as e:
        return tool_result(f"搜索请求失败: {e}", is_error=True)
    except Exception as e:
        return tool_result(f"搜索解析失败: {e}", is_error=True)

    items = data.get("webPages", {}).get("value", [])
    if not items:
        return tool_result("未找到相关结果。")

    lines = []
    for i, item in enumerate(items, 1):
        title = item.get("name", "无标题")
        url = item.get("url", "")
        snippet = item.get("snippet", "")
        lines.append(f"{i}. {title}\n   URL: {url}\n   摘要: {snippet}\n")

    return tool_result("\n".join(lines))


TOOLS = [
    {
        "name": "web_search",
        "description": (
            "联网搜索。给定查询词，返回相关网页的标题、URL 和摘要。"
            "用于获取最新信息、查证事实、查找文档或教程。"
            "返回的结果可以用 web_fetch 工具进一步读取全文。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "搜索关键词"},
                "count": {"type": "integer", "description": "返回结果数量，默认 5，最大 10"},
            },
            "required": ["query"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "web_search":
        count = args.get("count", 5)
        try:
            count = int(count) if count else 5
        except (TypeError, ValueError):
            count = 5
        return do_web_search(args.get("query", ""), count)
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
                    "serverInfo": {"name": "re0-web-search", "version": "1.0.0"},
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
