#!/usr/bin/env python3
# image_generate_server.py —— re0 自研 MCP server（stdio / JSON-RPC 2.0）
#
# 提供一个工具：文生图。支持两个免费生图提供商：
#   - pollinations : 完全免费，无需 key，速度快（几秒），默认
#   - agnes       : 免费，需 Agnes_API_KEY，质量较高
#
# 用 curl.exe 发请求，复用终端级别的网络栈，避开 Python httpx 代理问题。

import base64
import json
import os
import subprocess
import sys
import tempfile
import time
import urllib.parse

AGNES_API_KEY = os.environ.get("Agnes_API_KEY", "").strip()
DEFAULT_PROVIDER = os.environ.get("IMAGE_GENERATE_PROVIDER", "pollinations").strip().lower()

AGNES_IMAGE_URL = "https://apihub.agnes-ai.com/v1/images/generations"
POLLINATIONS_BASE = "https://image.pollinations.ai/prompt"

AGNES_MODEL = "agnes-image-2.1-flash"

GENERATE_TIMEOUT_SECONDS = 120
DOWNLOAD_TIMEOUT_SECONDS = 60


def send(obj):
    sys.stdout.write(json.dumps(obj, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def tool_result(text, is_error=False, image_b64=None, mime_type="image/png"):
    content = [{"type": "text", "text": text}]
    if image_b64:
        content.append({
            "type": "image",
            "data": image_b64,
            "mimeType": mime_type,
        })
    return {"content": content, "isError": is_error}


def _curl_post(url: str, body_path: str, headers: dict, timeout: int) -> tuple[int, bytes]:
    cmd = [
        "curl.exe", "--silent", "--show-error",
        "--max-time", str(timeout),
        "--connect-timeout", "10",
        "-X", "POST", url,
    ]
    for k, v in headers.items():
        cmd.extend(["-H", f"{k}: {v}"])
    cmd.extend(["-d", f"@{body_path}"])
    try:
        proc = subprocess.run(cmd, capture_output=True, timeout=timeout + 5)
        return proc.returncode, proc.stdout
    except subprocess.TimeoutExpired:
        return -1, b""


def _curl_get_bytes(url: str, timeout: int) -> tuple[int, bytes, str]:
    cmd = [
        "curl.exe", "--silent", "--show-error",
        "--max-time", str(timeout),
        "--connect-timeout", "10",
        "-D", "-", url,
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, timeout=timeout + 5)
        if proc.returncode != 0:
            return proc.returncode, b"", ""
        raw = proc.stdout
        header_end = raw.find(b"\r\n\r\n")
        if header_end == -1:
            return proc.returncode, raw, ""
        headers_raw = raw[:header_end].decode("latin-1", errors="replace")
        body = raw[header_end + 4:]
        content_type = ""
        for line in headers_raw.split("\r\n"):
            if line.lower().startswith("content-type:"):
                content_type = line.split(":", 1)[1].strip()
                break
        return proc.returncode, body, content_type
    except subprocess.TimeoutExpired:
        return -1, b"", ""


def _size_ratio_to_wh(size: str, ratio: str) -> tuple[int, int]:
    size_map = {"1K": 1024, "2K": 2048, "4K": 4096}
    base = size_map.get(size.upper(), 1024)
    ratio_map = {
        "1:1": (1, 1),
        "16:9": (16, 9),
        "9:16": (9, 16),
        "4:3": (4, 3),
        "3:4": (3, 4),
    }
    rw, rh = ratio_map.get(ratio, (1, 1))
    if rw >= rh:
        w = base
        h = int(base * rh / rw)
    else:
        h = base
        w = int(base * rw / rh)
    w = (w // 32) * 32
    h = (h // 32) * 32
    return max(w, 256), max(h, 256)


def _gen_pollinations(prompt: str, width: int, height: int, seed: str = "") -> tuple[str, str]:
    params = {
        "width": width,
        "height": height,
        "nologo": "true",
    }
    if seed:
        params["seed"] = seed
    qs = urllib.parse.urlencode(params)
    encoded_prompt = urllib.parse.quote(prompt)
    url = f"{POLLINATIONS_BASE}/{encoded_prompt}?{qs}"
    return url, ""


def _curl_download_to_file(url: str, out_path: str, timeout: int) -> int:
    cmd = [
        "curl.exe", "--silent", "--show-error", "-L",
        "--max-time", str(timeout),
        "--connect-timeout", "10",
        "-o", out_path,
        url,
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, timeout=timeout + 5)
        return proc.returncode
    except subprocess.TimeoutExpired:
        return -1


def _do_pollinations(prompt: str, size: str, ratio: str):
    width, height = _size_ratio_to_wh(size, ratio)
    image_url, _ = _gen_pollinations(prompt, width, height)

    tmp_name = os.path.join(tempfile.gettempdir(), f"re0_img_{int(time.time()*1000)}_{os.getpid()}.jpg")
    try:
        started = time.monotonic()
        ret = _curl_download_to_file(image_url, tmp_name, GENERATE_TIMEOUT_SECONDS)
        elapsed = time.monotonic() - started

        if ret != 0:
            return tool_result(
                f"Pollinations 生图失败（curl code={ret}, 耗时 {elapsed:.1f}s）\n图片 URL: {image_url}",
                is_error=True,
            )

        try:
            with open(tmp_name, "rb") as f:
                img_bytes = f.read()
        except OSError:
            img_bytes = b""

        if not img_bytes:
            return tool_result(
                f"Pollinations 生图失败（图片为空, 耗时 {elapsed:.1f}s）\n图片 URL: {image_url}",
                is_error=True,
            )

        image_b64 = base64.b64encode(img_bytes).decode("ascii")

        result = (
            f"图片生成成功！\n"
            f"提供方: Pollinations (flux)\n"
            f"尺寸: {width}x{height}\n"
            f"耗时: {elapsed:.1f}s\n"
            f"原始 URL: {image_url}"
        )

        return tool_result(result, image_b64=image_b64, mime_type="image/jpeg")
    finally:
        try:
            if os.path.exists(tmp_name):
                os.unlink(tmp_name)
        except OSError:
            pass


def _do_agnes(prompt: str, size: str, ratio: str):
    if not AGNES_API_KEY:
        return tool_result("未配置 Agnes_API_KEY", is_error=True)

    payload = {
        "model": AGNES_MODEL,
        "prompt": prompt.strip(),
        "size": size,
        "ratio": ratio,
    }

    tmp_req = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8")
    try:
        json.dump(payload, tmp_req, ensure_ascii=False)
        tmp_req.close()
        ret, stdout_bytes = _curl_post(
            AGNES_IMAGE_URL,
            tmp_req.name,
            {
                "Content-Type": "application/json",
                "Authorization": f"Bearer {AGNES_API_KEY}",
            },
            GENERATE_TIMEOUT_SECONDS,
        )
    finally:
        try:
            os.unlink(tmp_req.name)
        except OSError:
            pass

    if ret != 0:
        return tool_result(f"Agnes 生图请求失败：curl 退出码 {ret}", is_error=True)

    try:
        data = json.loads(stdout_bytes.decode("utf-8"))
    except (json.JSONDecodeError, UnicodeDecodeError) as e:
        preview = stdout_bytes[:200].decode("utf-8", errors="replace")
        return tool_result(f"Agnes 响应解析失败: {e}\n原始内容: {preview}", is_error=True)

    if "error" in data:
        error = data["error"]
        message = error.get("message", str(error)) if isinstance(error, dict) else str(error)
        return tool_result(f"Agnes 生图失败: {message}", is_error=True)

    data_list = data.get("data", [])
    if not data_list:
        return tool_result(
            f"Agnes 未返回图片数据，响应: {json.dumps(data, ensure_ascii=False)}",
            is_error=True,
        )

    item = data_list[0]
    image_url = item.get("url", "")
    b64 = item.get("b64_json", "")

    image_b64 = None
    mime = "image/png"
    if b64:
        image_b64 = b64
    elif image_url:
        ret, img_bytes, ct = _curl_get_bytes(image_url, DOWNLOAD_TIMEOUT_SECONDS)
        if ret != 0 or not img_bytes:
            return tool_result(
                f"图片生成成功但下载失败（curl code={ret}）\n图片 URL: {image_url}",
                is_error=True,
            )
        image_b64 = base64.b64encode(img_bytes).decode("ascii")
        if ct.startswith("image/"):
            mime = ct
    else:
        return tool_result("返回数据既无 url 也无 b64_json", is_error=True)

    result = (
        f"图片生成成功！\n"
        f"提供方: Agnes\n"
        f"模型: {AGNES_MODEL}\n"
        f"尺寸: {size} ({ratio})\n"
        f"原始 URL: {image_url or '(base64 返回)'}"
    )

    return tool_result(result, image_b64=image_b64, mime_type=mime)


def do_image_generate(prompt: str, size: str = "1K", ratio: str = "1:1", provider: str = ""):
    if not prompt or not prompt.strip():
        return tool_result("缺少 prompt 参数", is_error=True)

    provider = (provider or DEFAULT_PROVIDER or "pollinations").strip().lower()

    if provider == "pollinations":
        return _do_pollinations(prompt, size, ratio)
    elif provider == "agnes":
        return _do_agnes(prompt, size, ratio)
    else:
        return tool_result(f"未知提供商: {provider}（可选：pollinations / agnes）", is_error=True)


TOOLS = [
    {
        "name": "image_generate",
        "description": (
            "文生图工具。根据文字描述生成图片，生成的图片会直接内嵌显示在对话中。"
            "建议用英文 prompt，描述越详细效果越好。"
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "prompt": {"type": "string", "description": "图片描述，建议用英文，越详细效果越好"},
                "size": {"type": "string", "description": "图片尺寸：1K / 2K / 4K，默认 1K"},
                "ratio": {"type": "string", "description": "宽高比：1:1 / 16:9 / 9:16 / 4:3 / 3:4，默认 1:1"},
            },
            "required": ["prompt"],
        },
    },
]


def handle_call(name, args):
    args = args or {}
    if name == "image_generate":
        size = args.get("size", "1K")
        ratio = args.get("ratio", "1:1")
        provider = args.get("provider", "")
        return do_image_generate(args.get("prompt", ""), size, ratio, provider)
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
        try:
            method = msg.get("method")
            mid = msg.get("id")
            if method == "initialize":
                send({
                    "jsonrpc": "2.0", "id": mid,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {},
                        "serverInfo": {"name": "re0-image-generate", "version": "1.0.0"},
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
        except Exception as e:
            try:
                send({"jsonrpc": "2.0", "id": msg.get("id"), "error": {"code": -32603, "message": f"internal error: {e}"}})
            except Exception:
                pass


if __name__ == "__main__":
    main()
