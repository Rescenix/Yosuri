[English](./README.en.md) · [中文](./README.md) · [正体中文](./README.zhtw.md) · [日本語](./README.ja.md) · [Tiếng Việt](./README.vi.md) · [தமிழ்](./README.ta.md)

# ResceneAgent

> 让 AI 做完网页后，你立刻在同一个对话框里玩、点、验收。

ResceneAgent 是本地优先的前端 Agent 工作台：描述需求，Agent 写代码、运行真实浏览器、展示结果；你可以直接用鼠标和键盘操作它刚做出的网页，而不只是看一张截图。

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Local--first-Your%20keys%2C%20your%20control-5b5bd6" alt="Local first">
  <img src="https://img.shields.io/badge/LLM-Multi--provider-ff69b4" alt="Multi-provider LLM">
</p>

## 为什么值得试试

### 不是“AI 写完给你一段代码”

对 Aurora 说“做个能联机的贪吃蛇”，它会生成项目、启动预览；预览是可交互的真实 Chromium。你能亲自点击、输入、开玩，和 Agent 一起确认这次交付到底能不能用。

### 不把 AI 的试错直接写进你的项目

AgentFS 把每轮文件修改放进隔离快照：查看 Diff、回到任一个节点、探索分支，确认后再提交。让 Agent 的大胆尝试和你的工作目录之间有一道可审计的护栏。

### 闭环留在一个工作台

从需求拆解、代码生成、终端执行，到浏览器验收和截图证据，都在同一条对话工作流里完成。模型、提供方和 API Key 由你自己配置；后端只做动态路由，不内置或绑定任何一家模型服务。

## 核心体验

```text
一句需求
  → Agent 拆解并生成代码
  → 浏览器自动运行和渲染
  → 你直接交互验收
  → 查看 Diff，保留或回滚这次修改
```

- 真实 Chromium 预览与 CDP 双向交互，不是静态截图或 iframe；
- 流式代码生成、Monaco 编辑、终端执行、Diff 审计在一个聊天工作台；
- 多 Agent 调度、实时 TODO、结构化追问与断点续传；
- 会话级 AgentFS 快照树与时间旅行；
- 本地记忆与项目状态隔离保存，不污染仓库；
- 对话内生图、截图工件与前端验收。

## 5 分钟运行

需要 Go >= 1.26、Node.js >= 22；Ollama 和 Docker 为可选依赖。

```bash
# 终端 1：后端
cd main-backend
go run cmd/server/main.go

# 终端 2：前端
cd main-frontend/beneficial-belt
npm install
npm run dev
```

访问 `http://localhost:4322`，再在设置中填写你自己的 LLM 提供方、模型与 API Key。

## 开源与 Cloud

核心前后端以 [MIT License](./LICENSE) 开源，本地使用不依赖云服务。Rescene Cloud 是可选的早期云能力：用于 GitHub 登录，以及未来的模型兼容性补丁、同步与更新通知；它不会替代你的本地模型配置或接管 API Key。

## 深入了解

| 想了解 | 位置 |
| --- | --- |
| 前端与后端结构 | [`main-frontend/beneficial-belt`](./main-frontend/beneficial-belt) · [`main-backend`](./main-backend) |
| 工作流/工具测试 | [`harness`](./harness) |
| 项目文档资产 | [`docs`](./docs) |
| 许可证 | [MIT](./LICENSE) |

如果这个方向让你对“AI 不是只会聊天，而是能交付可玩的东西”多一点期待，欢迎点一个 Star。
