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

## 能力对照

```text
一句需求
  → Agent 拆解并生成代码
  → 浏览器自动运行和渲染
  → 你直接交互验收
  → 查看 Diff，保留或回滚这次修改
```

| 能力 | 常见对话式 Coding Agent | ResceneAgent |
| --- | --- | --- |
| 对话生成/修改代码 | ✓ | ✓ |
| 子 Agent 协作 | 依实现而定 | 多 Agent 调度，任务可拆分、可追踪 |
| 任务计划 | 常隐藏在模型内部 | 实时 `TODO`：`pending / doing / done`，前端同步展示 |
| 主动向人确认 | 通常靠自由文本 | `ask_user` 结构化提问，回答原地回流工作流 |
| 中断后的继续执行 | 依会话实现而定 | 每轮快照消息、工具、TODO 与 Token 计数，可断点续传 |
| 代码生成过程 | 常是整段结果或黑盒工具调用 | SSE 代码瀑布：新增/删除计数、流式内容与最终 Diff |
| 编辑、终端与 Diff | 往往依赖外部 IDE | Monaco、文件树、PowerShell、可审计 Diff 集成在聊天面板 |
| 运行后的网页验收 | 截图、iframe 或由用户另开浏览器 | 真实 Chromium + CDP Screencast；鼠标、键盘可双向交互 |
| 交付证据 | 通常只返回文字说明 | 页面截图按工具调用顺序成为 artifact，可按需展开 |
| AI 生成图片 | 依产品而定 | 对话内 `image_generate`，结果可继续作为前端素材使用 |
| AI 写文件的安全边界 | 常直接写入工作目录 | AgentFS 隔离快照、分支探索、逐条 Diff 审计与时间旅行 |
| 收尾自检 | 依 Agent 自觉执行 | 构建 + 真实浏览器截图校验门；不可逆删除/移动始终需要审批 |
| 长期上下文 | 依会话或外部记忆 | 全局 `MEMORY.md` + 项目 `workdir.md` 双轨隔离，不污染仓库 |
| 模型选择 | 常绑定单一云服务 | 用户自带 Provider、模型与 API Key，后端动态路由与故障转移 |
| 工作流可视化 | 较少提供 | Harness Flow 实时展示 Gateway / Memory / LLM / Tools / Reply 及 Trace / Eval / Release 链路 |

### 交付不是“生成完就算完”

ResceneAgent 的收尾有一层明确的自检护栏：Agent 需要运行构建，并用真实浏览器渲染和截图验证结果；截图成为会话中的交付证据。删除文件、目录或移动文件属于不可逆操作，即使在 YOLO 模式也必须经过审批。它的目标不是让 Agent 看起来更忙，而是减少“代码写了、页面却没跑起来”的空交付。

### AgentFS：让 AI 的改动可控、可回退

每个会话都有自己的快照轨迹：文件修改先进入隔离的影子 Git 仓库，而不是立刻污染项目。你可以查看每次 Diff、回到之前能正常工作的节点，或从任意快照拉出探索分支。

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
