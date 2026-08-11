# -*- coding: utf-8 -*-
"""把 147 段素材 + 20 组配音合成一个完整视频（画面+晓臻配音，无字幕），用户拖进必剪只加字幕。"""
import json, os, subprocess, sys

ROOT = r"C:\Pro2026\GIFS\0806\videos"
OUT = r"C:\Pro2026\GIFS\0806"

# 1. 素材 list
video_list = os.path.join(ROOT, "concat_video.txt")
with open(video_list, "w", encoding="utf-8") as f:
    for i in range(147):
        p = os.path.join(ROOT, f"seg_{i:03d}.mp4")
        if os.path.exists(p):
            f.write(f"file '{p.replace(chr(39), chr(39)*2)}'\n")

# 2. 配音 list（按 order 排序）
audio_list = os.path.join(ROOT, "concat_audio.txt")
voices = sorted([f for f in os.listdir(ROOT) if f.startswith("voice_") and f.endswith(".mp3")])
with open(audio_list, "w", encoding="utf-8") as f:
    for v in voices:
        p = os.path.join(ROOT, v)
        f.write(f"file '{p.replace(chr(39), chr(39)*2)}'\n")

print(f"素材: {len(voices)} 组配音, 147 段视频", flush=True)

# 3. concat 视频（无声）
print("正在拼接视频…", flush=True)
subprocess.run(["ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", video_list,
                "-c", "copy", os.path.join(OUT, "concat_video.mp4")],
               check=True, capture_output=True, timeout=600)

# 4. concat 音频
print("正在拼接音频…", flush=True)
subprocess.run(["ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", audio_list,
                "-c", "copy", os.path.join(OUT, "concat_audio.mp3")],
               check=True, capture_output=True, timeout=300)

# 5. 获取音频时长
dur = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                       "-of", "csv=p=0", os.path.join(OUT, "concat_audio.mp3")],
                      capture_output=True, text=True).stdout.strip()
audio_dur = float(dur) if dur else 0
print(f"配音总时长: {audio_dur:.0f}s ({audio_dur/60:.1f}min)", flush=True)

# 6. 合成：视频 + 音频，视频短于音频时用 tpad 补最后一帧
print("正在合成最终视频…", flush=True)
subprocess.run(["ffmpeg", "-y",
                "-i", os.path.join(OUT, "concat_video.mp4"),
                "-i", os.path.join(OUT, "concat_audio.mp3"),
                "-filter_complex",
                f"[0:v]tpad=stop_mode=clone:stop_duration={max(0, audio_dur - 0):.2f}[v]",
                "-map", "[v]", "-map", "1:a",
                "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
                "-c:a", "aac", "-b:a", "192k",
                "-t", f"{audio_dur:.2f}",
                os.path.join(OUT, "AI越听话越危险_晓臻配音.mp4")],
               check=True, capture_output=True, timeout=1800)

# 清理中间文件
os.remove(os.path.join(OUT, "concat_video.mp4"))
os.remove(os.path.join(OUT, "concat_audio.mp3"))
os.remove(video_list)
os.remove(audio_list)

sz = os.path.getsize(os.path.join(OUT, "AI越听话越危险_晓臻配音.mp4"))
print(f"完成! 视频: {OUT}\\AI越听话越危险_晓臻配音.mp4", flush=True)
print(f"大小: {sz//1024//1024} MB, 时长: {audio_dur:.0f}s", flush=True)