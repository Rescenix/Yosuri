[English](./README.en.md) · [中文](./README.md) · [正体中文](./README.zhtw.md) · [日本語](./README.ja.md) · [Tiếng Việt](./README.vi.md) · [தமிழ்](./README.ta.md)

# ResceneAgent

> A frontend-specialized multi-agent platform that packs an IDE, terminal, browser, and an AI team into a single chat box. Built around the digital persona **Rescene**, it closes the loop from requirement breakdown → code → runtime verification, all inside one conversation.

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?logo=go&logoColor=white" alt="Go >= 1.26">
  <img src="https://img.shields.io/badge/Node-%3E%3D22-339933?logo=nodedotjs&logoColor=white" alt="Node >= 22">
  <img src="https://img.shields.io/badge/LLM-Multi--Provider-ff69b4" alt="Multi-Provider LLM">
</p>

**It does not just chat — it writes, runs, and verifies.**

---

## Why it is different

Generic coding assistants can talk about code. ResceneAgent turns "write, run, verify" into a guarded closed loop. Two differentiated designs are the platform's moat:

### AgentFS: a transaction for AI writes

An AI edits ten files in a chain; it crashes at the fifth and the first four are already dirty on disk — the most fatal weakness of traditional AI coding. ResceneAgent rebuilds file writes as an **isolated, auditable, rollback-capable transaction layer**:

- **In-memory snapshot isolation** — every change first lands in an isolated snapshot, never immediately polluting your project;
- **Atomic commit / rollback** — writes hit disk only after compile, test, and human confirmation; on failure the whole batch rolls back and the project is untouched;
- **Pixel-level audit timeline** — every changed line carries its source; approve / reject line by line;
- **Time travel** — jump back along the snapshot timeline to find "which version still compiled."

Its intellectual roots come from mature paradigms like VFS, Git, and database transactions; the real difference is making this **a systematic, agent-facing write-transaction layer**.

### Memory / state dual-track: never lose the agent

Starting every new session from scratch? ResceneAgent makes cross-session memory and cross-project state an **agent-facing memory layer**:

- **Global `MEMORY.md` + per-project `workdir.md`**, physically isolated under `~/rescene_data/`, never polluting your repo;
- **Agent writes proactively** — through a tool the agent decides "what is worth remembering"; only what the user chooses to remember is persisted;
- **Auto-injected at session start** — unconditionally spliced into the system prompt when the workflow boots, so the agent knows the project history from its first word.

---

## Features

### Differentiated capabilities

- **AgentFS Trace: session-level Git trace tree** — a continuously growing snapshot tree in the sidebar, one trajectory per session; click a node to see glass-textured Diff cards; all traces come from an isolated shadow git repo, writing no extra commits to your main repo.
- **Harness Flow: real-time workflow architecture diagram** — an embedded canvas on the right of the chat, wiring Gateway / Memory / LLM / Tools / Reply with the Trace / Eval / Release stages into a genuinely event-driven flow diagram, with the active link highlighted.
- **Agent-driven TODO** — at the start of a complex task the agent publishes a `pending / doing / done` structured list, pushed via SSE to above the input box; survives cross-context compression without losing the plan, resumable on break.
- **Proactive ask_user (Human-in-the-loop)** — `ask_user` raises a structured decision (single / multiple / free input) right in the workflow; the answer resumes in place as formal context, never guessed.
- **Resume from breakpoint** — every round auto-snapshots (message history, tools, TODO, token count); after restart or disconnect the frontend shows a recovery bar and replays from the breakpoint round.
- **Real-time render & verify browser** — after the agent edits a frontend file, a real Chromium (CDP) auto-renders and screencasts back to the panel, not an iframe; with navigation / mobile viewport / open-external.
- **Screenshot artifacts** — page screenshots are inserted into the chat stream in tool-call order, collapsed by default and expandable on demand, so the agent delivers page evidence on its own.
- **Enterprise-grade safety & verification** — ① irreversible ops (delete file/dir/move) get zero-exemption approval, not even in YOLO mode; ② a mandatory build + screenshot gate at wrap-up (`go build` / `npm run build` + real render screenshot), pushed back to the frontend as a bonus.

### Common IDE experience (integrated in the chat panel)

An embedded real PowerShell terminal (snippet panel), Monaco editor + recursive file tree, VS Code-style Diff preview, message streaming gradient animation, and a full-UI anime skin system (with Live2D mascot). It also ships blog/CMS, e-book, image bed, TTS, and stats dashboard modules.

- **Model list grouped by provider** — the chat model dropdown folds models by the backend catalog's `vendor` (provider), with a ✓ mark on the selected one; it shares the same grouping as the settings panel and only lists models the user enabled there, with custom configs falling under "Others".

---

## System architecture

```
┌──────────────────────────────────────────────────────────────┐
│   beneficial-belt (Astro + Vue 3 + Naive UI)                    │
│   chat · editor · file tree · terminal · preview browser · Diff · skin │
│   └─ settings: user freely fills LLM provider / API Key / model (no hardcode) │
├──────────────────────────────────────────────────────────────┤
│   main-backend (Go / Gin :8080)                                 │
│   four-state workflow · multi-agent · live TODO · ask_user · MCP · memory │
│   └─ multi-provider model routing: dynamic forward by config, no built-in/hardcoded LLM backend │
├──────────────────────────────────────────────────────────────┤
│   AgentFS: session snapshot tree · shadow Git · diff audit · time travel │
├──────────────────────────────────────────────────────────────┤
│   memory layer: MEMORY.md(global) + workdir.md(project), single-file on disk │
├──────────────────────────────────────────────────────────────┤
│   external LLM cloud API (user's choice: Ollama / DeepSeek / Gemini / …) │
└──────────────────────────────────────────────────────────────┘
```

> The LLM is not part of the platform backend: provider, API Key, and concrete model are all freely configured by the user in the **frontend settings page**; the backend only does dynamic routing and failover, hardcoding nothing.

## Repository structure

```
re0/
├── main-backend/          # Go backend (:8080)
│   ├── internal/handler/  # workflow / AgentFS / preview / terminal / MCP / skills / subAgent
│   ├── skills/            # learned skills (fetched locally, not in repo)
│   └── mcp/               # self-built MCP server (grep/shell/memory…)
├── main-frontend/beneficial-belt/   # Astro + Vue 3 frontend
├── harness/               # Python scripts (MCP/tests/tools)
└── docs/                  # documentation assets
```

## Quick start

### Prerequisites

- Go >= 1.26
- Node.js >= 22
- Ollama (local LLM, optional)
- Docker (code sandbox, optional)

### Backend

```bash
cd main-backend
# configure .env (ADMIN_PASSWORD, JWT_SECRET, DEEPSEEK_API_KEY, etc.)
go run cmd/server/main.go
```

### Frontend

```bash
cd main-frontend/beneficial-belt
npm install
npm run dev    # http://localhost:4322
```

The memory layer activates automatically with the backend; no separate deployment.

## Environment variables

| Variable | Description |
|----------|-------------|
| `ADMIN_PASSWORD` | Admin password (SHA-256 digest comparison, no plaintext storage, no backdoor) |
| `JWT_SECRET` | JWT signing key (shared with ResceneCloud) |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `RESCENE_CLOUD_URL` | ResceneCloud auth service base (private; falls back to localhost:8088 if unset) |
| `MCP_CONFIG` | MCP server config file path (default `./mcp.json`) |
| `RESCENE_DATA_DIR` | memory / AgentFS / session data root (default `~/rescene_data`) |

## License

This project is open source under the [MIT License](./LICENSE). Auth, billing, and the commercial loop stay in the private service ResceneCloud; the open-source re0 holds no keys or OAuth logic.

---

## Star History

<a href="https://star-history.com/#Rescenix/ResceneAgent&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/star-history-dark.png" />
    <source media="(prefers-color-scheme: light)" srcset="assets/star-history-light.png" />
    <img alt="Star History Chart" src="assets/star-history-light.png" width="100%" />
  </picture>
</a>

<sub>Generated by [`scripts/gen_star_history.py`](scripts/gen_star_history.py), auto-updated daily via GitHub Actions; click the image for live data.</sub>
