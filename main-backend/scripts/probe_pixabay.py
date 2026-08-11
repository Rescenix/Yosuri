#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""探测 Pixabay 视频搜索页的可达性与视频直链格式（免 key 方案验证）。"""
import re
import subprocess
import sys
import urllib.parse

UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/126.0 Safari/537.36")

def curl_fetch(url: str) -> str:
    r = subprocess.run(
        ["curl", "-s", "--max-time", "15", "-H", f"User-Agent: {UA}",
         "-H", "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
         "-H", "Accept-Language: zh-CN,zh;q=0.9", url],
        capture_output=True, text=True, timeout=30)
    return r.stdout

def main():
    kw = sys.argv[1] if len(sys.argv) > 1 else "跳舞"
    url = "https://pixabay.com/zh/videos/search/" + urllib.parse.quote(kw) + "/"
    print("URL:", url)
    html = curl_fetch(url)
    print("HTML len:", len(html))
    vids = re.findall(r'https://cdn\.pixabay\.com/video/[^"\'<>\s]+\.mp4', html)
    seen = set()
    for v in vids:
        if v not in seen:
            seen.add(v)
            print("VID:", v[:130])
        if len(seen) >= 6:
            break
    print("unique mp4:", len(seen))
    if not seen:
        # 看看有没有别的视频 URL 形态
        for pat in [r'https://[^"\']+\.mp4', r'"video_url":"[^"]+', r'data-video[^>]*src="[^"]+']:
            m = re.findall(pat, html)[:3]
            if m:
                print("ALT pattern", pat, "->", [x[:100] for x in m])

if __name__ == "__main__":
    main()
