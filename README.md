[English](./README.md) · [简体中文](./README.zh-CN.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

<p align="center">
  A growing AI workbench — AI that <b>remembers</b>, <b>executes</b>, and <b>keeps evolving</b>
</p>

<p align="center">
  <b style="font-size: 19px;">Zero API key. Web search, image understanding, and 98 free models — out of the box.</b><br/>
  Not a chat window. A workbench that runs real tools and keeps getting smarter.
</p>

<p align="center">
  <a href="https://rescene.shanca.me/download.html">
    <img src="https://img.shields.io/badge/Download-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20Android-4FC08D.svg?style=for-the-badge" alt="Download">
  </a>
  <a href="https://rescene.shanca.me/">
    <img src="https://img.shields.io/badge/Website-rescene.shanca.me-4FC08D.svg?style=for-the-badge" alt="Website">
  </a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-AGPLv3-blue.svg" alt="AGPL-3.0"></a>
  <img src="https://img.shields.io/badge/Release-v0.2.4-blue" alt="Release v0.2.4">
  <img src="https://img.shields.io/badge/Backend-Go-00ADD8" alt="Go">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
</p>

![Rescene workbench overview](./assets/rescene-main.png)

---

## ⚡ Native free capabilities — no API key required

| Capability | What it does |
| --- | --- |
| **🔍 Free web search** | Built-in Bing fallback — search the web with **zero API key**; Firecrawl, custom models and MCP tools also supported |
| **👁️ Native free vision** | All vision models load-balanced by success rate, auto-failover — paste an image and get it understood, no vendor lock-in, no key needed |
| **🧲 Free model pool** | 98 free models from multiple providers unified behind one OpenAI-compatible endpoint — smart routing automatically picks the fastest healthy source |
| **🎬 Free short-drama studio** | Built-in AI short-drama workbench: reference images / first & last frames / storyboard chaining — free, no credits |

## ⚙️ A workbench, not a chatbot

| Capability | What it does |
| --- | --- |
| **🔄 Five-platform sync** | Sessions and long-term memory continue naturally across Windows / Linux / macOS / Android / CLI. You change screens, not your working context |
| **🤖 Automation loop** | Browser, terminal and real tools form a verifiable execution chain — tasks don't stop at answers, they run to completion |
| **👨‍👩‍👧 Sub-agents & background tasks** | Concurrent sub-agent workflows and background tasks (run_task) with completion notifications, all visible on a timeline panel |
| **⚙️ Settings fully open** | The agent can analyze and modify your configuration — models, persona, web/image sources, skill toggles — no black box |
| **🛠️ Self-editing skills & on-demand tools** | Skill library with add/remove/enable/disable; native tools slimmed down and loaded on demand (load_tools) |
| **🖱️ Computer Use** | Screenshots, mouse, keyboard, drag & drop, scrolling — it operates your desktop, not just your code |
| **🛡️ AgentFS change audit** | Every AI file edit gets a snapshot / diff / rollback; dangerous operations require your approval |

![Free short-drama studio: templates, reference images and generation params on one screen](./assets/rescene-studio.png)

---

## 🚀 Download & Install

👉 **[https://rescene.shanca.me/download.html](https://rescene.shanca.me/download.html)** 👈

| Platform | How |
| --- | --- |
| Windows | Portable ZIP / installer — unzip and run, auto-update |
| Linux / macOS | Desktop client tar.gz |
| Android | Mobile sync & continue |
| **CLI (one line)** | `curl -fsSL https://download.shanca.me/rescene-cli/install.sh \| sh` |

> 💬 Questions or suggestions? Join our QQ groups: **Group 1 609967535** (almost full) · **Group 2 796474621** (new)

## 🛠️ Build from Source (contributors)

```bash
# Backend (Go 1.22+)
cd main-backend && go run .

# Frontend (Node 18+)
cd main-frontend/beneficial-belt && npm install && npm run dev
```

Visit `http://localhost:4322` for the local dev workbench. Linux build: [`main-backend/docs/linux-build.md`](./main-backend/docs/linux-build.md).

## 💬 Feedback & License

- 🐛 Bugs / suggestions → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- 💬 Community → [QQ Group 796474621](https://qm.qq.com/q/796474621)
- Core code: [AGPL-3.0 License](./LICENSE)
