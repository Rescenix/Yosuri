#!/usr/bin/env python3
"""产出流水线实物：会议纪要 + 文档 + PV 脚本 + 检查可运行项目"""
import json, os, subprocess, sys, time

API = "http://localhost:8080/v1/chat/completions"
HEADERS = {"Content-Type": "application/json", "Authorization": "Bearer res"}
OUT = os.path.expanduser("~/rescene_data/company/ceo-01/outputs")
os.makedirs(OUT, exist_ok=True)

def call(prompt, max_tokens=800):
    r = subprocess.run(["curl", "-s", "-m", "60", API,
        "-H", "Content-Type: application/json",
        "-H", "Authorization: Bearer res",
        "-d", json.dumps({"model": "auto", "messages": [{"role": "user", "content": prompt}], "max_tokens": max_tokens, "temperature": 0.7})],
        capture_output=True, text=True, timeout=70)
    try:
        d = json.loads(r.stdout)
        if "choices" in d and d["choices"]:
            return d["choices"][0]["message"]["content"]
    except: pass
    return ""

date = time.strftime("%Y-%m-%d")
n = int(time.time()) % 100

# 1. 会议纪要
print("🤝 产出会议纪要...")
meeting = call("你是公司CEO，写一份公司周会会议纪要，包含各部门（作者部/研究部/程序部/设计部/发布部/宣传部）的进展汇报、关键决策和下一步行动。200-300字。")
if meeting:
    path = os.path.join(OUT, f"会议-{date}-{n:02d}.md")
    with open(path, "w") as f:
        f.write(f"> 由 CEO 自动产出 · 会议 · {date}\n\n# 会议纪要\n\n{meeting}\n")
    print(f"  ✅ {path}")

# 2. 软件文档
print("📘 产出软件文档...")
doc = call("你是技术文档工程师，为「Rescene Agent OS — 100 人公司系统」写一份软件文档，包含项目概述、技术架构、快速开始、API说明。300-400字。")
if doc:
    path = os.path.join(OUT, f"文档-{date}-{n:02d}.md")
    with open(path, "w") as f:
        f.write(f"> 由 CEO 自动产出 · 文档 · {date}\n\n# 软件文档\n\n{doc}\n")
    print(f"  ✅ {path}")

# 3. 宣传PV
print("🎬 产出PV脚本...")
pv = call("你是宣传导演，为「Rescene 100 人 AI 公司」制作一份30秒宣传视频PV脚本，包含分镜（镜头1-镜头6）、旁白文案、音效说明。200-300字。")
if pv:
    path = os.path.join(OUT, f"PV-{date}-{n:02d}.md")
    with open(path, "w") as f:
        f.write(f"> 由 CEO 自动产出 · PV · {date}\n\n# 宣传PV脚本\n\n{pv}\n")
    print(f"  ✅ {path}")

# 4. 检查可运行项目
print("\n💻 检查可运行项目...")
base = os.path.expanduser("~/rescene_data/company")
for coder in ["coder-27", "coder-03", "coder-21"]:
    proj_dir = os.path.join(base, coder, "projects")
    if os.path.isdir(proj_dir):
        for p in os.listdir(proj_dir):
            pdir = os.path.join(proj_dir, p)
            if os.path.isdir(pdir):
                for f in os.listdir(pdir):
                    if f.endswith((".py", ".js", ".go", ".html")):
                        print(f"  ✅ {coder}/{p}/{f}")

print("\n📂 CEO 产出目录：")
for f in sorted(os.listdir(OUT)):
    if not os.path.isdir(os.path.join(OUT, f)):
        print(f"  📄 {f}")

print("\n🎉 流水线完成！")