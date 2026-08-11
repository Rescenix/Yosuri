# -*- coding: utf-8 -*-
"""一键配音：读素材包的 script.txt → 整篇连贯 TTS（按章节分组，无逐句断点）→ 输出配音 mp3 包。
用法: python tts_script.py <script.txt> <输出目录>
产物: voice_001.mp3 ...（每组一段连贯配音，配 script.txt 分组文案）+ 分组清单
"""
import json, os, re, sys, asyncio

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..",
                                "main-backend", "scripts"))
import mambo_video as m

VOICE = "zh-TW-HsiaoChenNeural"  # 曉臻·台湾普通话女声（2026-08-06 用户听选定稿；edge-tts 国内可用）


def group_script(path: str, group_size: int = 8):
    """读 script.txt，按 group_size 段一组切分。返回 [(序号, 组内文案拼接, 组内段数)]"""
    lines = [l for l in open(path, encoding="utf-8").read().splitlines() if "|" in l]
    groups = []
    for i in range(0, len(lines), group_size):
        chunk = lines[i:i + group_size]
        texts = [l.split("|", 1)[1].strip() for l in chunk]
        # 组内文案：句号衔接，避免拼接突兀
        joined = "".join(t if t.endswith(("。", "！", "？", "！？")) else t + "。" for t in texts)
        groups.append((i, joined, len(chunk)))
    return groups


async def tts_all(items, out_dir):
    for idx, text, _ in items:
        out_mp3 = os.path.join(out_dir, f"voice_{idx + 1:03d}.mp3")
        await m.tts_segment(text, VOICE, "+0%", out_mp3, os.path.join(out_dir, "tmp.srt"))
        dur = m.probe_duration(m.find_ffmpeg(), out_mp3)
        print(f"  voice_{idx + 1:03d}.mp3  {dur:.1f}s  {text[:30]}…", flush=True)


def main():
    if len(sys.argv) < 3:
        print("用法: python tts_script.py <script.txt> <输出目录>")
        return 1
    script_path, out_dir = sys.argv[1], sys.argv[2]
    os.makedirs(out_dir, exist_ok=True)
    groups = group_script(script_path)
    print(f"共 {len(groups)} 组配音（每组整段连读，无逐句断点）", flush=True)
    # 分组清单
    with open(os.path.join(out_dir, "分组清单.txt"), "w", encoding="utf-8") as f:
        for idx, text, n in groups:
            f.write(f"voice_{idx + 1:03d}.mp3 | 对应 seg_{idx:03d}~seg_{idx + n - 1:03d} | {text[:50]}\n")
    asyncio.run(tts_all(groups, out_dir))
    print("完成", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
