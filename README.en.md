[中文](./README.md) · [English](./README.en.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

<p align="center">
  An AI workbench that grows with you — AI that <b>remembers</b>, <b>executes</b>, and <b>keeps evolving</b>
</p>

A long-term companion AI workbench: **cross-device sync** keeps your context alive, **automation** carries tasks through to the end, and an **aggregated API** unifies every model entry. It **self-learns** from every collaboration and gets to know you better through **long-term memory**.

<p align="center">
  <a href="https://rescene.shanca.me/download.html">
    <img src="https://img.shields.io/badge/Download-Windows%20%7C%20Android-4FC08D.svg?style=for-the-badge" alt="Download">
  </a>
  <a href="https://rescene.shanca.me/">
    <img src="https://img.shields.io/badge/Website-rescene.shanca.me-4FC08D.svg?style=for-the-badge" alt="Website">
  </a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-AGPLv3-blue.svg" alt="AGPL-3.0 License"></a>
  <img src="https://img.shields.io/badge/Release-v0.1.5-blue" alt="Release v0.1.5">
  <img src="https://img.shields.io/badge/Backend-Go-00ADD8" alt="Go">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
</p>

---

## ⚡ Core capabilities

| Capability | What it does |
| --- | --- |
| **🔄 Cross-device sync** | Account, sessions and long-term memory continue naturally between devices. You change screens, not your working context: live session handoff, optional memory backup, work state resumes where you left off |
| **🤖 Automation loop** | Browser, terminal and tool calls form a verifiable execution chain — tasks don't stop at answers: auto-decompose the goal → call real tools → verify the result end to end |
| **🔌 Aggregated API port** | Different providers and local models collapse into one OpenAI-compatible entry. Configure once, call from anywhere: unified model entry, smart routing & load balancing, freely combinable providers |
| **🧠 Self-learning & long-term memory** | Every collaboration leaves reusable experience: preferences, decisions and project knowledge are recalled exactly when needed. Cross-session association, automatic skill distillation — the more you use it, the closer it fits |

## 🌟 More features

| Feature | What it does |
| --- | --- |
| **🧲 Free model pool** | Multiple free providers merged into one OpenAI-compatible endpoint: 30-minute probes score every model, daily re-scan retires delisted sources, circuit breaker skips rate-limited ones, LRU weights recent winners |
| **🖱️ Computer Use** | More than code — it operates your desktop: screenshots, mouse, keyboard, drag & drop, scrolling. Real clicks, real keys |
| **🌐 Real browser automation** | Reuses your system browser via CDP: renders, clicks, types, scrolls, reads the DOM, screenshots, verifies both ways. A real browser running your page — not fake screenshots |
| **🛡️ AgentFS change audit** | Snapshots / diffs / rollback on every AI file edit; dangerous operations require your approval |
| **🛠️ Automatic skill distillation** | Every completed workflow extracts experience — model preferences, code style, project architecture — woven into context next time automatically. No custom-instructions file, ever |

---

## 🚀 Download & Install

👉 **[https://rescene.shanca.me/download.html](https://rescene.shanca.me/download.html)** 👈

- **Windows** — standard guided installer, start-menu entry, uninstallable from system settings
- **Android** — sync and continue on mobile
- **Extremely light** — no bundled browser, no Node.js / Python required
- **Auto-update** — new versions download and overlay-install automatically, keeping your config

> 💬 Questions or suggestions? Join our QQ groups: **Group 1 609967535** (almost full) · **Group 2 796474621** (new)

## ⚙️ First Run

1. Open the workbench → **Settings → Models**, fill in at least one API key; keyless sources are ready in the free pool.
2. Or configure providers via environment variables — see `main-backend/.env.example`.
3. The free pool probes every 30 minutes and re-scans provider lists daily: rate-limited sources get downgraded, delisted ones retire.

## 🛠️ Build from Source (contributors)

### Windows

```powershell
# Backend (Go 1.22+)
cd main-backend
go run cmd/server/main.go

# Frontend (Node 18+)
cd main-frontend/beneficial-belt
npm install && npm run dev
```

Visit `http://localhost:4322` for the local dev workbench.

### Linux

Full dependency setup and build steps: [`main-backend/docs/linux-build.md`](./main-backend/docs/linux-build.md).

```bash
bash scripts/build-linux.sh amd64   # or arm64
```

## 💬 Feedback & License

- 🐛 Bugs / suggestions → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- 💬 Community → [QQ Group 796474621](https://qm.qq.com/q/796474621)
- Core code: [AGPL-3.0 License](./LICENSE)
