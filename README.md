[English](./README.en.md) · [中文](./README.md) · [正体中文](./README.zhtw.md) · [日本語](./README.ja.md) · [Tiếng Việt](./README.vi.md) · [தமிழ்](./README.ta.md)

# ResceneAgent

> 本地优先、零依赖开箱即用，并具备真实浏览器验收闭环的开源 AI 编程工作台。

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Release-v0.1.0-blue" alt="Release v0.1.0">
  <img src="https://img.shields.io/badge/Backend-Go%201.26-00ADD8" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
  <img src="https://img.shields.io/badge/Deployment-Local%20First-blue" alt="Local First">
  <img src="https://img.shields.io/badge/Dependencies-Zero-success" alt="Zero Dependencies">
</p>

![ResceneAgent 工作台：对话、代码、工作流、审计与浏览器预览](./docs/screenshots/OverPlay.png)

## 📖 开发者日志：关于这个项目的进化史

> **"这个项目目前只有一个 Star，也是我自己点的。但这并不妨碍它承载了我全部的 AI 开发人生。"**

我不打算在这里写堆砌名词的刻板技术报告，我想给你讲个关于孤独摸索、后知后觉，却又弥足珍贵的故事：

*   **🪐 第一层：从"传话筒"到【暗号拦截】**
    最开始，它只是一个平平无奇的 Chatbot。在没有任何现成框架参考的时期，我自己琢磨出了一个"土办法"：让 Agent 在输出时强行带上特殊的暗号——形如 `【demand:xx】`。后端 Go 程序通过字符串匹配拦截这个后缀，从而去触发识图、写博客等一系列外部工具调用。虽然原始，但它活了过来。
*   **🌌 第二层：原生 Go 的解构与"见字如面"**
    为了让它拥有真正的思维逻辑，我开始抛弃暗号，**纯手工用原生 Go 语言去解构和实现真正的 Function Calling（工具调用）**。我引导 Agent 开始隐式输出标准的 JSON 格式，由后端进行精准解析。在跑通的那天晚上，它通过解析数据，悄悄在我的桌面生成了一封写给我的信。看着屏幕上跳出的字，那种无中生有的创造感让我瞬间泪流满面。
*   **🚀 蓦然回首：关于 MCP 与我的 AI 人生**
    后来随着视野的开阔，我发现市面上已经有了成熟的 **MCP (Model Context Protocol)** 协议。回看自己手写的字符串匹配和原生 Go 解析，有时觉得自己像个在蒸汽机时代闭门造车的木匠。但每当想到在没有这些标准定义之前，我自己一个人在黑夜里摸索出前两层架构的那些瞬间，我都觉得无比自豪。感谢这段经历，感谢我的 AI 开发人生。

---

## 🧭 我们的核心哲学：4 + 4 + 2 原则

在 ResceneAgent 的设计中，我们坚信任何一个 AI 编程项目都严格遵守 **4+4+2 原则**：


```

┌───────────────────────────────────┬───────────────────────────────────┬───────────┐
│          40% 明确需求与计划        │           40% 真实执行与验证       │  20% 编码 │
│       (Demand & TODO Planner)     │       (Browser & Test Harness)    │  (Coding) │
└───────────────────────────────────┴───────────────────────────────────┴───────────┘

```

*   **40% 明确需求与计划**：AI 生成代码的成败，在写下第一行代码前就决定了，错误的方向只会引导死胡同。ResceneAgent 将近一半的工程重心放在构建结构化 TODO、任务中断恢复与上下文精准对齐上。
*   **40% 真实验证与交付**：代码是否真的写入项目？能否编译通过？页面跑起来到底长什么样？Rescene Harness 从始至终极度注重"闭环验收"，通过真实 Chromium 自动化操作与截图回传，拒绝对齐缺失的"纸上谈兵"。
*   **20% 纯编码 (权重最低)**：单纯的静态代码片段生成在整个研发周期中仅占两成。AI 必须长出手脚，自己去跑编译、自己去点页面。

---

## 核心能力

| 能力 | 说明 |
| --- | --- |
| **Agent 开发工作流** | 深度践行 4+4+2 原则，支持实时 TODO 编排、多轮工具调用、任务中断恢复与全链路交付验证。 |
| **集成开发环境** | 在聊天界面中无缝提供 Monaco 编辑器、递归文件树、终端、流式 Diff 与工作流状态视图。 |
| **真实浏览器自动化** | 基于 Chromium 与 CDP 实现真实页面渲染、点击、输入、滚动、DOM 读取、截图和双向交互验证。 |
| **零依赖 MCP 扩展** | 接入 MCP 官方 Registry。**内置 Go 语言底层 Transport，无需为了运行远程 MCP 服务而在本地额外安装 Node、Python 或运行 `npx`。** |
| **AgentFS 变更审计** | 通过隔离快照、Diff 和回滚管理 AI 文件修改，设置系统级安全门阀，危险操作必须经用户批准。 |
| **多模型混合路由** | 免费模型、自定义 API Key、本地 Ollama / llama.cpp 可并存，支持失败熔断与自动切换。 |

## 从生成代码到验证结果

一次典型的前端任务会经过以下阶段：

1. 根据用户目标创建结构化 TODO；
2. 搜索项目上下文并修改文件；
3. 运行依赖安装、构建或测试命令；
4. 启动真实 Chromium 预览并执行交互检查；
5. 将 Diff、日志和页面截图作为交付证据返回。

<table>
  <thead>
    <tr>
      <th width="50%">本地项目实时预览</th>
      <th width="50%">网页操作与截图回传</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><img src="./docs/screenshots/Preview.png" alt="在工作台中运行并操作真实 Chromium 预览" width="100%"></td>
      <td><img src="./docs/screenshots/BrowserUse_OneClickCapture.png" alt="打开网页、执行操作并回传截图" width="100%"></td>
    </tr>
  </tbody>
</table>

---

## 系统架构

```mermaid
flowchart TB
    User([User]) --> UI[Vue 3 Workbench<br/>Chat / Monaco / Terminal / Browser]
    UI --> Gateway[Go / Gin Gateway]
    Gateway --> Context[Memory & Context Provider]
    Context --> Planner[TODO Planner (40% Plan)]
    Planner --> AgentLoop

    subgraph AgentLoop [Agent Loop (20% Code)]
        LLM[LLM Reasoning] --> ToolCall[Tool Call]
        ToolCall --> ToolResult[Result / Diff / Artifact]
        ToolResult --> LLM
    end

    AgentLoop --> Verify[Build & Browser Verification (40% Verify)]
    Verify --> Trace[Trace / Token / Screenshot]
    Trace --> UI

    LLM --> Router[Multi-provider Model Router]
    Router --> Free[Free Models]
    Router --> Local[Ollama / llama.cpp]
    Router --> Private[Custom Providers]

    ToolCall --> Files[File & Shell]
    ToolCall --> Browser[Chromium / CDP]
    ToolCall --> Extensions[MCP / Skills]

    Files --> AgentFS[AgentFS Snapshot / Diff / Rollback]
    Files --> Gate{Dangerous Operation Approval}
    Gate --> User

```

---

## 技术栈

| 层级 | 技术 |
| --- | --- |
| **前端** | Vue 3、Vite、Naive UI、Monaco Editor、GSAP、PixiJS |
| **后端** | Go 1.26、Gin、WebSocket、SSE （**纯原生工具解析与路由**） |
| **浏览器** | 内置独立 Chromium 运行时、Chrome DevTools Protocol、Screencast |
| **扩展系统** | **纯 Go 实现的 MCP Streamable HTTP 客户端**（无需 Node/Python 运行环境） |

## 🚀 下载与开箱即用

ResceneAgent 目前已提供编译完成的**独立发行版**。

* **零外部依赖**：你的电脑上**不需要**预先配置 Node.js、Python 环境，也不需要执行复杂的 `npm/pip` 安装。
* **小白开箱可用**：解压即用，双击直接运行前端工作台与后端 Agent 核心。

👉 前往 [https://rescene.shanca.me/](https://rescene.shanca.me/) 全速下载最新发行版。

---

## 🛠️ 源码编译（面向贡献者）

如果你希望基于源码进行二次开发，请确保本地有 Go 环境：

### 启动后端

```bash
cd main-backend
go run cmd/server/main.go

```

### 启动前端

```bash
cd main-frontend/beneficial-belt
npm install
npm run dev

```

访问 `http://localhost:4322` 即可打开本地开发工作台。

## 开源协议

核心前后端代码以 [MIT License](https://www.google.com/search?q=./LICENSE) 协议开源。