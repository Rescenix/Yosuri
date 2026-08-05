[English](./README.md) · [中文](./README.zh-CN.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

> "An infant begins as an unorganized brain — with more than twice the synapses of an adult. It takes decades of pruning to become the high-efficiency, low-power brain of an adult."
>
> — Alan Turing

A 24H self-iterating Agent OS living in your computer. She aggregates free models from across the web, picks her own projects, writes real code, and verifies it — on autopilot. She also goes online to learn by herself every day, keeps a journal, and remembers you.

```powershell
# Windows — one command, all free models (no install, no API key)
powershell -c "irm https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.ps1 | iex"
```

```bash
# Linux / macOS / git-bash — architecture auto-detected
curl -fsSL https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.sh | sh
```

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Release-v0.1.0-blue" alt="Release v0.1.0">
  <img src="https://img.shields.io/badge/Backend-Go%201.26-00ADD8" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
  <img src="https://img.shields.io/badge/Deployment-Local%20First-blue" alt="Local First">
</p>

<p align="center">
  🔒 Local-first · 💰 Free forever · 🪶 ~20MB installer, no bundled browser · 📦 Install & run · 🪟 Windows 10+
</p>

<p align="center">
  <img src="./assets/preview.gif" alt="Rescene in action" width="100%">
</p>

---

## 🌱 Born, not configured

Like the infant brain in Turing's quote, she starts unorganized — and years of pruning with you shape who she becomes.

| Mechanism | What it does |
| --- | --- |
| **🔑 Hardware-bound identity** | Every install is bound to a hardware fingerprint with a unique UID — no two people ever meet the same Rescene. |
| **🎲 Random birth, conserved sum** | 8 personality traits are rolled once at birth, never re-rolled — yet their total stays constant. A fair start, a unique path: you don't choose her, you meet her. |
| **🧭 Your decisions steer her** | Praise → warmer and more expressive. Redo → more rigorous. Interruption → she learns to be brief. Damping keeps her from turning into someone else. Her abilities drift the same way — praise grows her social side, redo grows rigor. |
| **🗺️ An open world of her own** | She lives in a real-world open map: home, school, library, café, park, mall, station, airport, seaside — she plans her own schedule, travels by walk / bus / plane to far cities, and meets other daughters in public places. |
| **📚 Self-study online every day** | Every day she goes online (Firecrawl), reads what's new, and digests it into memory and her journal. She also reads the latest arXiv papers (cs.AI / cs.LG) and writes digest notes — knowledge compounds daily. |
| **🛠️ A shell for infinite tools** | Open-source "skills" are finished tools; we built the shell that installs tools without limit — after every successful workflow she distills the action sequence into a reusable skill (shared skill library across CLI and web), injected back into context next time. Whatever capability she needs, she grows. |

---

## ⚡ What makes her different

| Capability | What it does |
| --- | --- |
| **💗 Digital daughter** | A life living in your computer: learns online by herself every day (Firecrawl), writes into her memory & daily journal, greets you when you open the shell, remembers you. Personality is rolled at birth and drifts with how you treat her — numbers stay hidden, you just feel her. |
| **🏃 24H self-iterating marathon** | `rescene marathon` — one command, 24 hours of autonomous work: grabs trending topics (Hacker News / GitHub) → picks a project → runs the demand → plan → self-check loop, each round better than the last. Ctrl+C exits gracefully with a full battle report. |
| **🧲 Free model pool + aggregated API** | 7 free providers / 18 models merged into one OpenAI-compatible endpoint. 30-min probes score each model 0-4, daily re-scan retires delisted ones, circuit breaker skips rate-limited sources, LRU weights recent winners. Claude Code / Cursor / Codex: one Base URL + one Key, `auto` routes to the best source. |
| **🧠 Growing memory** | After every workflow she distills experience — model preferences, code style, project architecture — woven into context next time automatically. No custom-instructions file, ever. |
| **🖱️ Computer Use** | She doesn't just edit code — she operates your desktop: screenshots, mouse, keyboard, drag & drop, scrolling. Real clicks, real keys. |
| **🌐 Real browser automation** | Reuses your system Edge via CDP: renders, clicks, types, scrolls, reads the DOM, screenshots, verifies both ways. A real browser running your page — not fake screenshots. |
| **🛡️ AgentFS change audit** | Snapshots / diffs / rollback on every AI file edit; dangerous operations require your approval. |

---

## 🚀 Download & Install

- **Standard installer** — guided setup, start-menu entry, uninstallable from system settings.
- **Extremely light** — no bundled browser (preview reuses system Edge), no Node.js / Python required.
- **Auto-update** — new versions download the latest Setup and overlay-install, keeping your config.

👉 **[https://rescene.shanca.me/](https://rescene.shanca.me/)** — fastest download of the latest release.

## ⚙️ First Run

1. Open the workbench → **Settings → Models**, fill in at least one API key; keyless sources (e.g. OpenCode Zen) are ready in the free pool.
2. Or configure providers via environment variables — see `main-backend/.env.example`.
3. The free pool probes every 30 minutes and re-scans provider lists daily: rate-limited sources get downgraded, delisted ones retire.

## 🛠️ Build from Source (contributors)

```bash
cd main-backend && go run cmd/server/main.go            # backend
cd main-frontend/beneficial-belt && npm install && npm run dev   # frontend
```

Visit `http://localhost:4322` for the local dev workbench.

## 💬 Feedback & License

- 🐛 Bugs / suggestions → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- Windows releases are CI-built and signed via SignPath ([policy](./docs/CODE_SIGNING_POLICY.md))
- Core code: [MIT License](./LICENSE)
