#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
mambo_video.py — 曼波视频一键生成引擎（2026-08-06）
=====================================================
输入一句话主题，输出一段「曼波风格」竖屏短视频：
  文案 → edge-tts 配音（快节奏魔性音色）→ 精确 SRT 字幕（word boundary 对齐）
       → 每句关键词匹配素材（本地素材池 / 互联网素材）→ ffmpeg 竖屏合成

用法：
  python mambo_video.py --topic "主题" [--text "句1|句2|句3"] [--voice zh-CN-YunxiNeural]
        [--rate +30%] [--media 素材目录] [--out 输出.mp4] [--width 1080 --height 1920]
        [--font C:/Windows/Fonts/msyh.ttc] [--color 0xFF6B6B]

输出：
  <out>.mp4        成品竖屏视频
  <out>.srt        精确字幕（与语音对齐）
  <out>.manifest.json  每段素材来源清单（可追溯）
"""
import argparse
import asyncio
import html as htmlmod
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request
from pathlib import Path

import edge_tts

# ---------- 曼波风格默认 ----------
DEFAULT_VOICE = "zh-CN-XiaoxiaoNeural"  # 晓晓：标准女声（科普/口播耐听），2026-08-06 用户定：默认女声
DEFAULT_RATE = "+30%"                 # 快节奏 = 上头
DEFAULT_BG_COLORS = ["#FF6B6B", "#4ECDC4", "#FFE66D", "#A8E6CF", "#FFB347", "#6C5CE7"]

# 素材池默认目录（相对脚本位置）：assets/mambo/ 下可放图片/视频
DEFAULT_MEDIA_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "assets", "mambo")

STOPWORDS = set(
    "的 了 在 是 我 有 和 就 不 人 都 一 这 那 要 也 你 他 会 把 被 让 给 跟 从 对于 以及 等 吗 呢 吧 啊 呀 哦 嗯 哟 啦 嘛 嘿 哈 的 个 这 那 来 去 上 中 下 大 小 新 老 好 坏 快 慢 今天 我们 你们 他们 大家 一个 一下 起来 停 不 住 别 没 有 什么 怎么 为什么 因为 所以 但是 然后 如果".split())


def log(msg: str):
    print(f"[mambo] {msg}", file=sys.stderr, flush=True)


def find_ffmpeg() -> str:
    for name in ("ffmpeg", "ffmpeg.exe"):
        p = shutil.which(name)
        if p:
            return p
    for cand in (r"C:\ffmpeg\bin\ffmpeg.exe", r"C:\Program Files\ffmpeg\bin\ffmpeg.exe"):
        if os.path.exists(cand):
            return cand
    raise RuntimeError("找不到 ffmpeg，请安装并加入 PATH")


def find_font() -> str:
    """中文字体优先：微软雅黑 / 苹方 / Noto。返回原始路径（合成前会复制到工作目录绕开冒号转义）。"""
    for cand in (
        r"C:\Windows\Fonts\msyh.ttc",
        r"C:\Windows\Fonts\msyhbd.ttc",
        r"C:\Windows\Fonts\simhei.ttf",
        r"C:\Windows\Fonts\simsun.ttc",
        "/System/Library/Fonts/PingFang.ttc",
        "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    ):
        if os.path.exists(cand):
            return cand
    return ""


def prepare_font(font: str, workdir: str) -> str:
    """复制字体到工作目录，返回无冒号的相对路径（绕开 drawtext 冒号转义地狱）。"""
    if not font:
        return ""
    dst = os.path.join(workdir, "font" + os.path.splitext(font)[1] or ".ttc")
    shutil.copy(font, dst)
    return os.path.basename(dst)


def split_sentences(text: str) -> list[str]:
    """按 | 显式分段；否则按中文句读切分。"""
    if "|" in text:
        return [s.strip() for s in text.split("|") if s.strip()]
    parts = re.split(r"[。！？!?；;\n]+", text)
    return [p.strip() for p in parts if p.strip()]


def extract_keywords(sentence: str) -> list[str]:
    """轻量中文关键词：去标点/停用词后，生成 2-4 字滑动窗口 + 英文原词。"""
    clean = re.sub(r"[^\u4e00-\u9fa5A-Za-z0-9]+", "", sentence)
    for w in sorted(STOPWORDS, key=len, reverse=True):
        clean = clean.replace(w, " ")
    chunks = [w for w in re.split(r"\s+", clean.strip()) if w]
    out = []
    for w in chunks:
        if re.fullmatch(r"[A-Za-z0-9]+", w):
            out.append(w)
            continue
        if len(w) <= 4:
            out.append(w)
        else:
            for size in (4, 3, 2):
                for i in range(0, len(w) - size + 1):
                    out.append(w[i:i + size])
    # 去重保序，长度优先（长的更像核心词）
    seen, uniq = set(), []
    order = sorted(enumerate(out), key=lambda x: (-len(x[1]), x[0]))
    for _, w in order:
        if w not in seen:
            seen.add(w)
            uniq.append(w)
    return uniq[:6]


def scan_media_pool(media_dir: str) -> list[dict]:
    """扫描素材池：支持图片(jpg/png/webp/gif)与视频(mp4/webm/mov)。返回绝对路径。"""
    if not media_dir or not os.path.isdir(media_dir):
        return []
    exts = {".jpg", ".jpeg", ".png", ".webp", ".gif", ".mp4", ".webm", ".mov"}
    items = []
    for p in sorted(Path(media_dir).rglob("*")):
        if p.is_file() and p.suffix.lower() in exts:
            items.append({"path": str(p.resolve()), "name": p.stem.lower(), "ext": p.suffix.lower()})
    return items


def match_media(keywords: list[str], pool: list[dict]) -> dict | None:
    """关键词匹配素材池：文件名含任一词（或其 2 字子串）即命中，分数高者优先。"""
    best, best_score = None, 0
    for kw in keywords:
        kwl = kw.lower()
        # 候选：关键词本身 + 所有 2 字中文子串（长关键词里的核心词）
        cands = {kwl}
        for i in range(0, len(kwl) - 1):
            sub = kwl[i:i + 2]
            if re.fullmatch(r"[\u4e00-\u9fa5]+", sub):
                cands.add(sub)
        for item in pool:
            score = 0
            for c in cands:
                if c in item["name"]:
                    score = max(score, len(c) * 2)
                else:
                    cnt = sum(1 for ch in c if ch in item["name"])
                    if cnt >= max(2, len(c) - 1):
                        score = max(score, cnt)
            if score > best_score:
                best, best_score = item, score
    return best


# ---------- 联网素材搜索（Pexels API + Pixabay 降级） ----------

_BING_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
    "Accept-Language": "zh-CN,zh;q=0.9",
    "Referer": "https://cn.bing.com/",
}

_VIDEO_MAX_BYTES = 60 << 20  # 单个视频上限 60MB
_PIX_SIZE_ORDER = ["large", "medium", "small", "tiny"]
# 联网素材持久化缓存目录（跨生成复用，避免反复搜索触发素材站风控；生成间隔太久会自然失效）
DEFAULT_ONLINE_CACHE = os.path.normpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "..", "assets", "online_cache"))

# 中英素材搜索词映射：Pixabay/Pexels 是英文素材库，中文梗词直接搜匹配差
# （实测「起摇」搜出雾天乡村风景）。命中映射表用英文搜，否则原词兜底（部分中文有结果）。
_ZH_EN_MEDIA = {
    "跳舞": "dancing", "舞蹈": "dancing", "蹦迪": "dancing club", "音乐": "music",
    "唱歌": "singing", "摇滚": "rock concert", "节奏": "music beat", "嗨起来": "party dance",
    "跑步": "running", "运动": "sports", "健身": "fitness", "游泳": "swimming",
    "篮球": "basketball", "足球": "football", "羽毛球": "badminton", "骑行": "cycling",
    "爬山": "hiking", "瑜伽": "yoga", "露营": "camping", "钓鱼": "fishing",
    "城市": "city", "街道": "street", "夜景": "night city", "霓虹": "neon lights",
    "海边": "beach", "大海": "ocean waves", "沙滩": "beach", "海浪": "ocean waves",
    "森林": "forest", "雪山": "snow mountain", "天空": "sky", "云": "clouds",
    "下雨": "rain", "雨天": "rain", "雨": "rain drops", "雪": "snow falling",
    "日落": "sunset", "日出": "sunrise", "太阳": "sun", "月亮": "moon",
    "星星": "stars night", "宇宙": "space", "地球": "earth",
    "猫": "cat", "狗": "dog", "小狗": "puppy", "小猫": "kitten",
    "美食": "food", "吃饭": "eating food", "火锅": "hotpot", "咖啡": "coffee",
    "茶": "tea", "奶茶": "milk tea", "花": "flowers", "樱花": "cherry blossom",
    "汽车": "car driving", "火车": "train", "飞机": "airplane", "烟花": "fireworks",
    "生日": "birthday", "婚礼": "wedding", "情侣": "couple love", "恋爱": "couple",
    "孩子": "children playing", "老人": "old people", "上班": "office", "加班": "office night",
    "学习": "studying", "看书": "reading book", "书": "books", "电脑": "computer",
    "手机": "smartphone", "游戏": "gaming", "动漫": "anime", "二次元": "anime",
    "旅行": "travel", "风景": "scenery nature", "沙漠": "desert", "瀑布": "waterfall",
    "河流": "river", "湖泊": "lake", "草原": "grassland", "田野": "fields",
    "派对": "party", "聚会": "party", "庆祝": "celebration", "加油": "cheer motivation",
    "奋斗": "motivation", "梦想": "dream", "成功": "success", "努力": "hard work",
    "打工": "office worker", "摸鱼": "relax break", "干饭": "eating", "熬夜": "night work",
}


def media_search_query(kw: str) -> str:
    """中文关键词 → 素材库搜索词：先全等命中，再子串包含命中，兜底原词。"""
    if kw in _ZH_EN_MEDIA:
        return _ZH_EN_MEDIA[kw]
    for zh, en in _ZH_EN_MEDIA.items():
        if zh in kw:
            return en
    return kw


def pexels_search_videos(query: str, api_key: str, n: int = 4) -> list[str]:
    """Pexels API 视频搜索（需要 API key，免费每月 200 次）。返回质量最高的 mp4 直链。"""
    url = (f"https://api.pexels.com/videos/search?query={urllib.parse.quote(query)}"
           f"&per_page={n}&orientation=portrait")
    try:
        req = urllib.request.Request(url, headers={"Authorization": api_key})
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        log(f"    Pexels 搜索失败: {e}")
        return []
    if not data.get("videos"):
        return []
    # 收集所有 mp4 视频文件，按质量排序（取最高质量）
    candidates = []
    for video in data["videos"]:
        for vf in video.get("video_files", []):
            if vf.get("file_type") == "video/mp4" and vf.get("link"):
                q = vf.get("quality", "sd")
                w = vf.get("width", 0) or 0
                candidates.append((q, w, vf["link"]))
    # 质量排序：hd > high > medium > sd > small
    q_rank = {"hd": 0, "high": 1, "medium": 2, "sd": 3, "small": 4, "low": 5}
    candidates.sort(key=lambda x: (q_rank.get(x[0], 9), -x[1]))
    seen, out = set(), []
    for _, _, link in candidates:
        if link not in seen:
            seen.add(link)
            out.append(link)
            if len(out) >= n:
                break
    return out


_MIX_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.9",
}


def mixkit_page_videos(url: str) -> list[str]:
    """抓 Mixkit 分类页，提取 assets.mixkit.co 视频直链（1080 变体优先，其次任意尺寸）。"""
    try:
        req = urllib.request.Request(url, headers=_MIX_HEADERS)
        html = urllib.request.urlopen(req, timeout=8).read().decode("utf-8", "ignore")
    except Exception:
        return []
    urls = re.findall(r"https://assets\.mixkit\.co/videos/[0-9]+/[0-9]+-1080\.mp4", html)
    if not urls:
        urls = re.findall(r"https://assets\.mixkit\.co/videos/[0-9]+/[0-9]+-\d+\.mp4", html)
    return list(dict.fromkeys(urls))


def mixkit_search_videos(query: str, n: int = 4) -> list[str]:
    """Mixkit 免费视频搜索（免 key，页面爬取，2026-08-06 实测可用）。

    搜索词必须是英文（Mixkit tag 是英文 slug）；中文先过 media_search_query 映射。
    URL 自带尺寸变体，直接挑 -1080.mp4 高清。完整 slug 没结果时逐词缩短重试。
    """
    slug = re.sub(r"[^a-z0-9]+", "-", query.lower()).strip("-")
    if not slug:
        return []
    parts = slug.split("-")
    found = []
    # 提速：最多试 2 个页面（完整 slug + 最长单词），抽象长词不再逐词缩短白等
    for i in (len(parts), 1):
        s = "-".join(parts[:i])
        if not s:
            continue
        found = mixkit_page_videos(f"https://www.mixkit.co/free-stock-video/{s}/")
        if found:
            break
    return found[:n]


_PIXABAY_BLOCKED = False  # 会话级：Pixabay 搜索页被风控(403/429)后整场跳过，不再每个词白等超时
_MIXKIT_BLOCKED = False   # 同上：Mixkit 被风控后跳过


def pixabay_search_videos(query: str, n: int = 4) -> list[str]:
    """Pixabay 免费视频搜索（免 key，页面爬取）。返回 cdn.pixabay.com 直链，优先大尺寸。
    检测 403/429 风控 → 置 _PIXABAY_BLOCKED，本会话不再尝试（省掉每词 15s 白等）。"""
    global _PIXABAY_BLOCKED
    if _PIXABAY_BLOCKED:
        return []
    try:
        url = f"https://pixabay.com/zh/videos/search/{urllib.parse.quote(query)}/"
        r = subprocess.run(
            ["curl", "-s", "--max-time", "12", "-w", "\n%{http_code}",
             "-H", f"User-Agent: {_BING_HEADERS['User-Agent']}",
             "-H", "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
             "-H", "Accept-Language: zh-CN,zh;q=0.9", url],
            capture_output=True, text=True, timeout=20)
        out = r.stdout or ""
        code = out.rsplit("\n", 1)[-1].strip() if out else ""
        html_txt = out.rsplit("\n", 1)[0] if "\n" in out else out
        if code in ("403", "429"):
            _PIXABAY_BLOCKED = True
            log(f"    Pixabay 搜索被风控({code})，本会话跳过 Pixabay → 直接 Mixkit")
            return []
    except Exception as e:
        log(f"    Pixabay 搜索失败: {e}")
        return []
    if not html_txt:
        return []
    urls = re.findall(r'https://cdn\.pixabay\.com/video/[^"\'<>\s]+\.mp4', html_txt)
    if not urls:
        return []

    def size_rank(u):
        for i, s in enumerate(_PIX_SIZE_ORDER):
            if f"_{s}.mp4" in u:
                return i
        return 4
    urls.sort(key=size_rank)
    seen, out = set(), []
    for u in urls:
        base = re.sub(r"_(?:tiny|small|medium|large)\.mp4$", "", u)
        if base in seen:
            continue
        seen.add(base)
        out.append(u)
        if len(out) >= n:
            break
    return out


def download_file(url: str, dst: str, max_bytes: int, want: str) -> bool:
    """下载 URL 到 dst，校验 Content-Type 前缀为 want（image/ 或 video/），限流 max_bytes。
    urllib 被拒（403）时退 curl。"""
    try:
        req = urllib.request.Request(url, headers=_BING_HEADERS)
        with urllib.request.urlopen(req, timeout=20) as resp:
            ctype = resp.headers.get("Content-Type", "")
            if not ctype.startswith(want):
                return False
            data = resp.read(max_bytes + 1)
            if len(data) > max_bytes or len(data) < 1024:
                return False
            with open(dst, "wb") as f:
                f.write(data)
            return True
    except Exception:
        try:
            r = subprocess.run(
                ["curl", "-s", "--max-time", "25", "-H", f"User-Agent: {_BING_HEADERS['User-Agent']}",
                 "-o", dst, "-w", "%{http_code}", url],
                capture_output=True, text=True, timeout=40)
            if r.stdout.strip() != "200" or not os.path.exists(dst):
                return False
            if os.path.getsize(dst) > max_bytes or os.path.getsize(dst) < 1024:
                os.remove(dst)
                return False
            return True
        except Exception as e:
            log(f"    下载失败: {e}")
            return False


def is_playable_video(ffmpeg: str, path: str) -> bool:
    """ffprobe 验证视频可解码（下载的 URL 可能是假直链）。"""
    fp = shutil.which("ffprobe")
    if not fp:
        return True
    r = subprocess.run([fp, "-v", "error", "-select_streams", "v:0",
                        "-show_entries", "stream=codec_name", "-of", "csv=p=0", path],
                       capture_output=True, text=True, timeout=20)
    return r.returncode == 0 and r.stdout.strip() != ""


def _try_download_pixabay(ffmpeg: str, url: str, cached: str) -> bool:
    """Pixabay cdn 直链按大尺寸优先尝试：large → medium → small → tiny。

    页面 HTML 只列 _tiny.mp4（640x360），放大到 1080x1920 会糊；
    同 base 的其他尺寸变体全部存在（large ~8MB / medium ~4MB），
    依次替换后缀下载，第一个可解码的命中。
    """
    for suffix in _PIX_SIZE_ORDER:
        u = re.sub(r"_(?:tiny|small|medium|large)\.mp4$", f"_{suffix}.mp4", url)
        if download_file(u, cached, _VIDEO_MAX_BYTES, "video/"):
            if is_playable_video(ffmpeg, cached):
                if suffix != "tiny":
                    log(f"    尺寸 {suffix} ✅")
                return True
            os.remove(cached)
    return False


def fetch_online_video(ffmpeg: str, keywords: list[str], cache_dir: str,
                       pexels_key: str = "", search_terms: list[str] | None = None) -> str | None:
    """按搜索词联网搜视频素材（Pexels API → Pixabay 降级），下载第一个可用，返回本地路径；失败 None。

    search_terms 优先：LLM 语义分析给的英文搜索词（最精准）；缺省用中文关键词+映射表。
    """
    if not os.path.isdir(cache_dir):
        os.makedirs(cache_dir, exist_ok=True)
    terms = search_terms or keywords
    for kw in terms:
        if len(kw) < 2:
            continue
        safe_kw = re.sub(r"[^\w]+", "_", kw)
        cached = os.path.join(cache_dir, f"{safe_kw}.mp4")
        if os.path.exists(cached) and os.path.getsize(cached) > 1024:
            log(f"    缓存命中: {safe_kw}.mp4（{os.path.getsize(cached) // 1024}KB，跨次复用）")
            return cached
        # 中英映射：Pixabay/Pexels 是英文素材库，中文梗词直接搜匹配差（如「起摇」搜出乡村风景）
        sq = media_search_query(kw)
        # 1. Pexels API（有 key 时）
        if pexels_key:
            urls = pexels_search_videos(sq, pexels_key, n=4)
            for u in urls:
                if download_file(u, cached, _VIDEO_MAX_BYTES, "video/"):
                    if is_playable_video(ffmpeg, cached):
                        log(f"    Pexels 视频 ✅ {kw} ← {u[:70]}")
                        return cached
                    log(f"    视频不可解码，跳过: {u[:60]}")
                    os.remove(cached)
                time.sleep(1.5)
            time.sleep(2.0)
        # 2. Pixabay 降级（无 key 或 Pexels 无结果时；被风控则整场跳过）
        if not _PIXABAY_BLOCKED:
            urls = pixabay_search_videos(sq, n=4)
            if not urls and not _PIXABAY_BLOCKED:
                log(f"    搜索无结果: {sq}（词太偏，退避后继续）")
                time.sleep(1.5)
            for u in urls:
                if _try_download_pixabay(ffmpeg, u, cached):
                    log(f"    Pixabay 视频 ✅ {kw} ← {u[:70]}")
                    return cached
                time.sleep(1.0)
        # 3. Mixkit 降级（Pixabay 风控/无结果时）：英文 slug 直接爬，1080 高清免 key
        if not _MIXKIT_BLOCKED:
            mix_urls = mixkit_search_videos(sq, n=4)
            if not mix_urls and not _MIXKIT_BLOCKED:
                time.sleep(1.0)
            for u in mix_urls:
                if download_file(u, cached, _VIDEO_MAX_BYTES, "video/"):
                    if is_playable_video(ffmpeg, cached):
                        log(f"    Mixkit 视频 ✅ {kw} ← {u[:70]}")
                        return cached
                    os.remove(cached)
                time.sleep(0.4)
        time.sleep(0.5)
    return None


def bg_color(i: int) -> str:
    return DEFAULT_BG_COLORS[i % len(DEFAULT_BG_COLORS)]


def probe_duration(ffmpeg: str, path: str) -> float:
    out = subprocess.run(
        [ffmpeg.replace("ffmpeg", "ffprobe") if "ffprobe" in ffmpeg else shutil.which("ffprobe") or "ffprobe",
         "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path],
        capture_output=True, text=True, timeout=60)
    try:
        return float(out.stdout.strip())
    except ValueError:
        return 5.0


def make_still_segment(ffmpeg: str, img_path: str, voice: str, dur: float,
                       big_text: str, out_path: str, font: str, font_color: str,
                       width: int, height: int, workdir: str, mute: bool = False) -> None:
    """单段：视频素材直接裁切 / 图片素材 Ken Burns 运镜 + 语音 + 大字卡点字幕 → 一段 mp4。
    mute=True（素材模式）：无语音、无画面字幕，纯素材片段（供必剪自行配音字幕）。"""
    is_video = not img_path.lower().endswith((".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"))
    if font and not mute:
        safe = big_text.replace(":", "\\:").replace("'", "")
        # 大字卡点字幕：白字 + 半透明黑底条（box）+ 描边双层保险，比裸描边更清晰有质感
        text_vf = (f",drawtext=fontfile={font}:text='{safe}':x=(w-text_w)/2:y=h-text_h-240:"
                   f"fontsize=72:fontcolor={font_color}:borderw=5:bordercolor=black@0.55:"
                   f"box=1:boxcolor=black@0.32:boxborderw=22:"
                   f"shadowx=3:shadowy=3:shadowcolor=black@0.4")
    else:
        text_vf = ""
    if is_video:
        # 视频素材：Ken Burns 缓慢推拉（d=1 每输入帧 1 输出帧，on 全局帧号驱动缩放，时长不变）
        # + 段首尾淡入淡出转场（相邻段不同素材不再硬切卡顿）
        # 1.2x 超采样（1296x2304 → 1080x1920）提速：视频本身有内容，无需 2x
        kb_v = (f"scale={int(width * 1.2)}:{int(height * 1.2)}:force_original_aspect_ratio=increase,"
                f"crop={int(width * 1.2)}:{int(height * 1.2)},"
                f"zoompan=z='1+0.0012*on':d=1:x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':"
                f"s={width}x{height}:fps=25,"
                f"fade=t=in:st=0:d=0.25,fade=t=out:st={max(dur - 0.4, 0):.2f}:d=0.3{text_vf}")
        cmd = [ffmpeg, "-y", "-i", img_path]
        if not mute:
            cmd += ["-i", voice]
        cmd += ["-t", f"{dur:.2f}", "-vf", kb_v, "-c:v", "libx264", "-preset", "veryfast",
                "-pix_fmt", "yuv420p"]
        if mute:
            cmd += ["-an"]
        else:
            cmd += ["-c:a", "aac", "-b:a", "192k", "-shortest"]
        cmd += [out_path]
    else:
        # 图片素材：Ken Burns 运镜（2x 超采样缩放，缓慢放大/平移 = 动态画面）+ 淡入淡出
        kb = (f"scale={width * 2}:{height * 2}:force_original_aspect_ratio=increase,"
              f"crop={width * 2}:{height * 2},"
              f"zoompan=z='min(zoom+0.0016\\,1.32)':d=25:x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':"
              f"s={width}x{height}:fps=25,"
              f"fade=t=in:st=0:d=0.25,fade=t=out:st={max(dur - 0.4, 0):.2f}:d=0.3{text_vf}")
        cmd = [ffmpeg, "-y", "-loop", "1", "-i", img_path]
        if not mute:
            cmd += ["-i", voice]
        cmd += ["-t", f"{dur:.2f}", "-vf", kb, "-c:v", "libx264", "-preset", "veryfast",
                "-pix_fmt", "yuv420p"]
        if mute:
            cmd += ["-an"]
        else:
            cmd += ["-c:a", "aac", "-b:a", "192k", "-shortest"]
        cmd += [out_path]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=300, cwd=workdir)
    if r.returncode != 0:
        raise RuntimeError(f"ffmpeg 段合成失败: {r.stderr[-800:]}")


def make_gradient_segment(ffmpeg: str, voice: str, dur: float, big_text: str,
                          out_path: str, font: str, color: str,
                          width: int, height: int, workdir: str, mute: bool = False) -> None:
    """动态渐变背景 + 大字（无素材兜底，曼波主流风格）。mute=True 素材模式：无字无声。"""
    vf = (f"scale={width}:{height},"
          f"fade=t=in:st=0:d=0.25,fade=t=out:st={max(dur - 0.4, 0):.2f}:d=0.3,"
          f"format=yuv420p")
    if font and not mute:
        safe = big_text.replace(":", "\\:").replace("'", "")
        # 渐变背景段大字：同样加半透明底条，与素材段字幕风格统一
        vf += (f",drawtext=fontfile={font}:text='{safe}':x=(w-text_w)/2:y=h-text_h-240:"
               f"fontsize=76:fontcolor=white:borderw=6:bordercolor=black@0.6:"
               f"box=1:boxcolor=black@0.32:boxborderw=24:"
               f"shadowx=4:shadowy=4:shadowcolor=black@0.5")
    cmd = [ffmpeg, "-y",
           "-f", "lavfi", "-i", f"color=c={color}:s={width}x{height}:d={dur:.2f}"]
    if not mute:
        cmd += ["-i", voice]
        fc = f"[0:v]{vf}[v];[1:a]apad[a]"
        maps = ["-map", "[v]", "-map", "[a]"]
    else:
        fc = f"[0:v]{vf}[v]"
        maps = ["-map", "[v]"]
    cmd += ["-filter_complex", fc] + maps + ["-t", f"{dur:.2f}", "-c:v", "libx264",
            "-preset", "veryfast", "-pix_fmt", "yuv420p"]
    if not mute:
        cmd += ["-c:a", "aac", "-b:a", "192k"]
    cmd += ["-shortest", out_path]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=300, cwd=workdir)
    if r.returncode != 0:
        raise RuntimeError(f"ffmpeg 背景段合成失败: {r.stderr[-800:]}")


# ---------- OpenAI TTS（Nous gateway，2026-08-06 用户定：nova 女声） ----------

_OPENAI_TTS_URL = "https://openai-audio-gateway.nousresearch.com/v1/audio/speech"
_OPENAI_TTS_MODEL = "gpt-4o-mini-tts"


def openai_tts_key() -> str:
    """OpenAI TTS key：环境变量优先（VOICE_TOOLS_OPENAI_KEY / OPENAI_API_KEY），
    兜底读 hermes auth.json 的 Nous access_token（订阅自带）。"""
    for k in ("VOICE_TOOLS_OPENAI_KEY", "OPENAI_API_KEY"):
        v = os.getenv(k, "").strip()
        if v:
            return v
    try:
        auth = json.load(open(os.path.expanduser("~/AppData/Local/hermes/auth.json"), encoding="utf-8"))
        return auth.get("providers", {}).get("nous", {}).get("access_token", "")
    except Exception:
        return ""


def openai_tts_speak(text: str, voice: str, out_mp3: str) -> None:
    """调 OpenAI TTS 生成一句语音 mp3。voice 如 nova/shimmer（openai: 前缀已剥掉）。"""
    key = openai_tts_key()
    if not key:
        raise RuntimeError("OpenAI TTS key 不可用（设 VOICE_TOOLS_OPENAI_KEY 或 hermes auth.json 缺失）")
    body = json.dumps({
        "model": _OPENAI_TTS_MODEL, "voice": voice,
        "input": text, "response_format": "mp3",
    }).encode("utf-8")
    req = urllib.request.Request(_OPENAI_TTS_URL, data=body,
                                 headers={"Content-Type": "application/json",
                                          "Authorization": "Bearer " + key})
    with urllib.request.urlopen(req, timeout=60) as r:
        data = r.read()
    if len(data) < 1024:
        raise RuntimeError(f"OpenAI TTS 返回异常（{len(data)} bytes）")
    with open(out_mp3, "wb") as f:
        f.write(data)


def _srt_ts(sec: float) -> str:
    """秒 → SRT 时间戳（逗号毫秒）。"""
    ms = int(round(sec * 1000))
    h, rem = divmod(ms, 3600000)
    m, rem = divmod(rem, 60000)
    s, ms = divmod(rem, 1000)
    return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"


async def tts_segment(text: str, voice: str, rate: str, out_mp3: str, out_srt: str) -> None:
    """单段配音 + 字幕。

    voice 以 "openai:" 前缀 → OpenAI TTS（如 openai:nova），句级字幕精确对齐；
    否则 edge-tts（SubMaker 逐字事件对齐）。
    """
    if voice.startswith("openai:"):
        v = voice.split(":", 1)[1]
        await asyncio.to_thread(openai_tts_speak, text, v, out_mp3)
        # 句级字幕：整句一条 [0, 句长]（逐句生成 → 时长精确可测）
        dur = probe_duration(find_ffmpeg(), out_mp3)
        srt = f"1\n{_srt_ts(0)} --> {_srt_ts(dur)}\n{text}\n"
        with open(out_srt, "w", encoding="utf-8") as f:
            f.write(srt)
        return
    comm = edge_tts.Communicate(text, voice, rate=rate)
    submaker = edge_tts.SubMaker()
    with open(out_mp3, "wb") as f:
        async for chunk in comm.stream():
            if chunk["type"] == "audio":
                f.write(chunk["data"])
            elif chunk["type"] in ("WordBoundary", "SentenceBoundary"):
                submaker.feed(chunk)
    srt = submaker.get_srt()
    with open(out_srt, "w", encoding="utf-8") as f:
        f.write(srt)


def shift_srt(srt_text: str, offset: float) -> str:
    """把 srt 时间轴整体平移 offset 秒（段拼接用）。"""
    def _shift(m):
        def _ts(ts):
            h, mm, s = ts.split(":")
            sec = int(h) * 3600 + int(mm) * 60 + float(s.replace(",", ".")) + offset
            sec = max(0, sec)
            return f"{int(sec // 3600):02d}:{int(sec % 3600 // 60):02d}:{sec % 60:06.3f}"
        return f"{_ts(m.group(1))} --> {_ts(m.group(2))}"
    return re.sub(r"(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})", _shift, srt_text)


def main():
    ap = argparse.ArgumentParser(description="曼波视频一键生成")
    ap.add_argument("--topic", default="曼波", help="主题")
    ap.add_argument("--text", default="", help="分段文案，| 分隔；缺省用内置曼波模板")
    ap.add_argument("--voice", default=DEFAULT_VOICE)
    ap.add_argument("--rate", default=DEFAULT_RATE)
    ap.add_argument("--media", default=DEFAULT_MEDIA_DIR, help="素材池目录")
    ap.add_argument("--ordered-media", action="store_true", help="按文件名顺序逐段使用本地素材（PPT/会议回放）")
    ap.add_argument("--pexels-key", default="", help="Pexels API Key（有 key 时优先用 Pexels 搜视频素材）")
    ap.add_argument("--semantic", default="", help="LLM 语义分析结果 JSON：{\"topic\":..., \"segments\":[{\"sentence\":..., \"topic\":..., \"search_terms\":[...]}]}")
    ap.add_argument("--video-only", action="store_true", help="素材模式：不配音不字幕，只输出每段无声素材片段到 --out 目录（供必剪/剪映自行配音字幕）")
    ap.add_argument("--seg-len", type=float, default=0, help="素材模式每段固定秒数（缺省按字数估算）")
    ap.add_argument("--online-cache", default=DEFAULT_ONLINE_CACHE, help="联网素材持久化缓存目录（跨生成复用，默认 assets/online_cache）")
    ap.add_argument("--no-online", action="store_true", help="禁用联网素材搜索（只用本地素材池）")
    ap.add_argument("--out", default="", help="输出 mp4 路径")
    ap.add_argument("--width", type=int, default=1080)
    ap.add_argument("--height", type=int, default=1920)
    ap.add_argument("--font", default="", help="drawtext 中文字体路径（: 转义）")
    args = ap.parse_args()

    ffmpeg = find_ffmpeg()
    font = args.font or find_font()
    if not font:
        log("警告：未找到中文字体，字幕文字将不渲染（仅语音+画面）")

    # 默认曼波模板（未给文案时）
    text = args.text or (
        f"曼波~曼波~！{args.topic}，来啦！|"
        f"{args.topic}曼波，跳起来！|"
        f"快乐不停，节奏不停，{args.topic}曼波~|"
        f"跟我一起，摇起来！曼波曼波~"
    )
    sentences = split_sentences(text)
    log(f"分段: {len(sentences)} 句")

    # LLM 语义分析结果（可选）：{sentence: {"topic":..., "search_terms":[...]}}
    semantic_map = {}
    if args.semantic:
        try:
            sem = json.loads(args.semantic)
            for p in sem.get("segments", []):
                if p.get("sentence") and p.get("search_terms"):
                    semantic_map[p["sentence"].strip()] = p
            log(f"语义分析: {len(semantic_map)} 段命中（LLM 语义词 → 素材库）")
        except Exception as e:
            log(f"语义分析解析失败（走关键词兜底）: {e}")

    workdir = tempfile.mkdtemp(prefix="mambo_")
    pool = scan_media_pool(args.media)
    log(f"素材池: {len(pool)} 个文件 @ {args.media or '(无)'}")
    font_rel = prepare_font(font, workdir)  # 无冒号相对路径，drawtext 安全

    seg_paths, manifest, all_srt, total_dur = [], [], "", 0.0

    # TTS 并发：先批量配音（每批 4 句同时调，OpenAI TTS 网络请求并发省时间）
    # 素材模式（--video-only）跳过：不配音，段长按字数估算
    if not args.video_only:
        async def tts_batch(items):
            await asyncio.gather(*[tts_segment(s, args.voice, args.rate, mp3, srt0)
                                   for s, mp3, srt0 in items])
        tts_items = [(sent, os.path.join(workdir, f"seg{i}.mp3"), os.path.join(workdir, f"seg{i}.srt"))
                     for i, sent in enumerate(sentences)]
        for b0 in range(0, len(tts_items), 4):
            asyncio.run(tts_batch(tts_items[b0:b0 + 4]))
        log(f"配音完成: {len(sentences)} 句")
    if args.video_only:
        # 素材模式：out 是目录，转绝对路径（make_still_segment 的 ffmpeg cwd=workdir，
        # 相对路径会在 Temp 下解析失败 → Error opening output）
        args.out = os.path.abspath(args.out)
        os.makedirs(args.out, exist_ok=True)

    try:
        for i, sent in enumerate(sentences):
            log(f"  [{i+1}/{len(sentences)}] {sent}")
            if args.video_only:
                # 素材模式：段长按字数估算（中文口播 ~4.5 字/秒），2.5-8s；无 mp3/srt
                dur = args.seg_len or max(2.5, min(8.0, len(sent) / 4.5))
                mp3, srt0 = "", ""
            else:
                mp3 = os.path.join(workdir, f"seg{i}.mp3")
                srt0 = os.path.join(workdir, f"seg{i}.srt")
                dur = probe_duration(ffmpeg, mp3) + 0.25  # 句间留 0.25s 卡点
            kws = extract_keywords(sent)
            plan = semantic_map.get(sent.strip())
            terms = (plan.get("search_terms") if plan else None) or kws
            big_text = sent[:14]  # 大字卡点字幕（截断）
            seg_mp4 = os.path.join(args.out if args.video_only else workdir,
                                   f"seg_{i:03d}.mp4" if args.video_only else f"seg{i}.mp4")

            matched = (pool[i % len(pool)] if args.ordered_media and pool else match_media(kws, pool))
            source = ""
            try:
                if matched:
                    log(f"    素材命中: {matched['name']}{matched['ext']}")
                    make_still_segment(ffmpeg, matched["path"], mp3, dur, big_text,
                                       seg_mp4, font_rel, "#FFFFFF", args.width, args.height, workdir,
                                       mute=args.video_only)
                    source = f"本地素材池: {matched['path']}"
                elif not args.no_online:
                    # 联网素材：真实视频素材（LLM 语义词 → Pixabay/Pexels），失败动态背景兜底
                    online_vid = fetch_online_video(ffmpeg, kws, args.online_cache,
                                                    args.pexels_key, search_terms=terms)
                    if online_vid:
                        make_still_segment(ffmpeg, online_vid, mp3, dur, big_text,
                                           seg_mp4, font_rel, "#FFFFFF", args.width, args.height, workdir,
                                           mute=args.video_only)
                        source = f"联网视频素材: {online_vid}"
                    else:
                        source = ""
                else:
                    source = ""
            except Exception as e:
                # 单段失败不整单崩（断点续传友好）：动态背景兜底，记入 manifest
                log(f"    段合成失败({str(e)[:80]}) → 动态背景兜底")
                source = f"段合成失败兜底: {str(e)[:40]}"
            if not source:
                log(f"    联网无视频素材 → 动态背景 #{i % len(DEFAULT_BG_COLORS)}")
                make_gradient_segment(ffmpeg, mp3, dur, big_text, seg_mp4,
                                      font_rel, bg_color(i), args.width, args.height, workdir,
                                      mute=args.video_only)
                if source == "":
                    source = "动态渐变背景（素材池+联网均无命中）"

            seg_paths.append(seg_mp4)
            manifest.append({
                "index": i, "sentence": sent, "keywords": kws,
                "topic": (plan.get("topic") if plan else "") or "",
                "search_terms": (plan.get("search_terms") if plan else None) or terms,
                "duration": round(dur, 2), "source": source,
            })
            if not args.video_only:
                with open(srt0, encoding="utf-8") as f:
                    all_srt += shift_srt(f.read(), total_dur)
            total_dur += dur

        # 素材模式收尾：每段独立文件 + script.txt（时长|文案，供必剪配音字幕）+ manifest
        if args.video_only:
            script_lines = [f"{m['duration']:.1f}|{m['sentence']}" for m in manifest]
            with open(os.path.join(args.out, "script.txt"), "w", encoding="utf-8") as f:
                f.write("\n".join(script_lines) + "\n")
            man_out = os.path.join(args.out, "manifest.json")
            with open(man_out, "w", encoding="utf-8") as f:
                json.dump({"topic": args.topic, "mode": "video-only",
                           "duration": round(total_dur, 2), "segments": manifest},
                          f, ensure_ascii=False, indent=2)
            print(json.dumps({
                "ok": True, "out_dir": os.path.abspath(args.out),
                "segments": len(sentences), "duration": round(total_dur, 2),
                "manifest": os.path.abspath(man_out),
            }, ensure_ascii=False))
            return

        # concat 拼接
        safe_topic = re.sub(r"[^\w]", "", args.topic)
        out = args.out or os.path.join(os.getcwd(), f"mambo_{safe_topic}.mp4")
        os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
        listfile = os.path.join(workdir, "list.txt")
        with open(listfile, "w", encoding="utf-8") as f:
            for sp in seg_paths:
                f.write(f"file '{sp.replace(chr(39), chr(39) * 2)}'\n")
        r = subprocess.run([ffmpeg, "-y", "-f", "concat", "-safe", "0", "-i", listfile,
                            "-c", "copy", out], capture_output=True, text=True, timeout=300)
        if r.returncode != 0:
            # copy 失败（编码不一致）→ 重编码
            log("concat copy 失败，重编码拼接…")
            r = subprocess.run([ffmpeg, "-y", "-f", "concat", "-safe", "0", "-i", listfile,
                                "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
                                "-c:a", "aac", out], capture_output=True, text=True, timeout=600)
            if r.returncode != 0:
                raise RuntimeError(f"concat 失败: {r.stderr[-800:]}")

        srt_out = os.path.splitext(out)[0] + ".srt"
        # 段拼接后统一重编号（各段子 SRT 都是从 1 开始的）
        cues = re.split(r"\n\s*\n", all_srt.strip())
        renumbered = []
        for i, cue in enumerate(cues, 1):
            lines = cue.split("\n", 1)
            if len(lines) == 2:
                renumbered.append(f"{i}\n{lines[1]}")
        with open(srt_out, "w", encoding="utf-8") as f:
            f.write("\n\n".join(renumbered) + "\n")
        man_out = os.path.splitext(out)[0] + ".manifest.json"
        with open(man_out, "w", encoding="utf-8") as f:
            json.dump({"topic": args.topic, "voice": args.voice, "rate": args.rate,
                       "duration": round(total_dur, 2), "segments": manifest},
                      f, ensure_ascii=False, indent=2)

        print(json.dumps({
            "ok": True, "video": os.path.abspath(out),
            "srt": os.path.abspath(srt_out), "manifest": os.path.abspath(man_out),
            "duration": round(total_dur, 2), "segments": len(sentences),
        }, ensure_ascii=False))
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
