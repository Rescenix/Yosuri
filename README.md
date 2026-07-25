# ResceneAgent — Aurora 数字生命平台

一个以前端特化 Agent 与团队协作能力为核心的 AI 平台，核心角色「Aurora」是一个具有长期记忆、代码工具能力和人格化交互的数字生命，并面向多 Agent 协同工作流设计。

## 目录

- [系统架构](#系统架构)
- [组件](#组件)
  - [main-backend — Go 后端服务](#main-backend--go-后端服务)
  - [main-frontend — Astro + Vue 3 前端](#main-frontend--astro--vue-3-前端)
- [仓库结构](#仓库结构)
- [快速开始](#快速开始)
  - [前置依赖](#前置依赖)
  - [后端 main-backend](#后端-main-backend)
  - [前端 main-frontend](#前端-main-frontend)
- [环境变量](#环境变量)
- [依赖](#依赖)
- [许可证](#许可证)

## 系统架构

```
┌──────────────────────────────────────────────────────┐
│            beneficial-belt (Astro + Vue 3)            │
│     博客 · 聊天 · 阅读 · 图床 · 时间线 · 音乐         │
├──────────────────────────────────────────────────────┤
│             main-backend (Go / Gin :8080)             │
│  多引擎聊天 · 工具调用 · Agent工作流 · 记忆 · 博客/CMS  │
├──────────────────────────────────────────────────────┤
│   记忆层：内嵌 PrismStore (C++ 库, 静态链接 main-backend) │
│   图结构记忆 · LLM压缩 · 簇隔离 · 忆阻器混沌演化         │
├──────────────────────────────────────────────────────┤
│          Ollama / DeepSeek / Gemini (外部 LLM)        │
└──────────────────────────────────────────────────────┘
```

## 组件

### main-backend — Go 后端服务

基于 Gin 框架，端口 `:8080`。

**聊天引擎：**

| 引擎 | 模型 | 用途 |
|------|------|------|
| DeepSeek (cloud) | deepseek-chat | 主力对话，原生 Function Calling |
| Ollama (local) | qwen2.5-coder:7b | 本地对话，自定义 JSON 工具协议 |
| Gemini Vision | gemini-pro-vision | 图片理解 |
| DS Browser | 通过浏览器代理 | DeepSeek 网页版接入 |

**AI 工具系统：**

| 工具 | 功能 |
|------|------|
| `search_codebase` | 语义搜索代码库 |
| `read_file` | 读取文件（支持 outline 模式和行范围） |
| `write_file` | 创建/覆盖文件 |
| `edit_file` | 精确替换编辑 |
| `execute_command` | 执行白名单 shell 命令 |
| `search_memory` | 检索长期记忆（summary/detail 两阶段） |
| `clean_memories` | 清理冗余记忆 |
| `codegraph_query` | 代码结构查询（callers/callees/impact） |

**Agent 工作流：**

- 意图分类（闲聊 vs 任务）
- 任务分解 → 多步执行 → 结果汇总
- Token 经济策略：outline 模式看结构 → 行范围取正文 → 精确替换

**其他功能：**

- 博客/CMS（文章 CRUD、标签管理）
- 电子书管理（上传、阅读、翻页）
- 图床（上传、标签、OSS、AI 随机推荐）
- TTS 语音合成
- Git 集成（status/add/commit/push）
- Docker 代码沙箱（Python/Go/JS/C）
- Aether 视觉预处理（Gemini）
- JWT 认证 + 开发模式后门

**API 端点：**

_聊天与记忆_

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/chat/stream` | 流式聊天（SSE） |
| POST | `/api/code/workflow` | Agent 工作流（四态机，SSE 流式） |
| POST | `/api/memory/save` | 保存记忆 |
| GET | `/api/memory/recall` | 回忆记忆 |
| GET | `/api/memory/welcome` | 欢迎语 |
| GET | `/api/sessions` | 会话列表 |

_内容管理（博客 / 阅读 / 图床）_

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/posts` | 创建文章 |
| GET | `/api/posts` | 文章列表 |
| POST | `/api/book/upload` | 上传书籍 |
| GET | `/api/book/list` | 书籍列表 |
| POST | `/api/upload` | 上传图片 |
| GET | `/api/images` | 图片列表 |
| POST | `/api/image/generate` | AI 生图 |

_工具与系统_

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/tts` | 语音合成 |
| POST | `/api/login` | 登录获取 JWT |
| POST | `/api/git/commit` | Git 提交 |
| POST | `/api/tool/execute` | 工具执行 |

### main-frontend — Astro + Vue 3 前端

端口 `:4321`，基于 Astro 框架 + Vue 3 + Naive UI。

**页面：**

| 页面 | 路径 | 功能 |
|------|------|------|
| 首页 | `/` | 粒子背景、每日一图 |
| 聊天 | `/chat` | Aurora 对话界面 |
| 博客 | `/blog` | 文章列表与详情 |
| 阅读 | `/read` | 电子书阅读器（翻页效果） |
| 阅读小屋 | `/reading-hut` | 阅读空间 |
| Aurora 小屋 | `/shanxi-hut` | Aurora 的个人空间 |
| 时间线 | `/timeline` | 时间轴展示 |
| 图床 | `/image-bed` | 图片管理与画廊 |

**技术栈：** Three.js (3D/VRM)、Monaco Editor、GSAP 动画、tsparticles、KaTeX 数学公式、marked/highlight.js、page-flip 翻页

### 记忆能力 — 内嵌 PrismStore

长期记忆由后端内嵌的 C++ 记忆库 `prism_store`（`main-backend/lib/`）提供，以静态链接形式集成在 main-backend 中，不单独部署服务。

- **图结构记忆：** 神经元（记忆节点）+ 突触（记忆关联）
- **四类簇隔离：** UserBase（用户画像）、CodeWork（代码决策）、ToolLog（工具日志）、Session（会话临时）
- **忆阻器混沌演化：** 导电率/关联流/混沌度建模，能量衰减模拟遗忘曲线
- **LLM 驱动压缩：** 对话 → 高密度摘要，自动判断 worth_saving

> 注：早期规划中 PrismD 为独立 :5666 服务（含 Prism C API 与可视化），当前仓库以内嵌库形态落地，相关 `Prism/` 目录与独立服务尚未提交。

## 仓库结构

```
re0/
├── main-backend/          # Go 后端服务 (:8080)
│   ├── cmd/server/
│   ├── internal/handler/
│   └── skills/
├── main-frontend/
│   └── beneficial-belt/   # Astro + Vue 3 前端 (:4321)
├── harness/               # Python 脚本（MCP/测试/工具）
└── docs/                  # 文档
```

## 快速开始

### 前置依赖

- Go ≥ 1.26
- Node.js ≥ 22
- Ollama（本地 LLM，可选）
- Docker（代码沙箱，可选）
- MySQL（可选）
- Redis（可选）

### 后端 main-backend

```bash
cd main-backend
# 配置 .env（ADMIN_PASSWORD, JWT_SECRET, DEEPSEEK_API_KEY 等）
go run cmd/server/main.go
```

### 前端 main-frontend

```bash
cd main-frontend/beneficial-belt
npm install
npm run dev    # http://localhost:4321
```

### 记忆能力（内嵌）

长期记忆由 main-backend 内嵌的 `prism_store` 库直接提供，随后端启动自动生效，**无需单独部署服务**。

```bash
# 记忆库随 main-backend 一同运行，无独立启动命令
cd main-backend
go run cmd/server/main.go
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `ADMIN_PASSWORD` | 管理员密码 |
| `JWT_SECRET` | JWT 签名密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `PRISM_API_TYPE` | 默认聊天引擎（ds/local） |
| `DEV_MODE` | 开发模式（true 时任意密码登录） |

## 依赖

- Go ≥ 1.26
- Node.js ≥ 22
- Ollama（本地 LLM）
- Docker（代码沙箱，可选）
- MySQL（可选）
- Redis（可选）

## 许可证

本项目基于 [MIT License](./LICENSE) 开源。

## Star History

<a href="https://www.star-history.com/?type=date&repos=Rescenix%2FResceneAgent">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=Rescenix/ResceneAgent&type=date&theme=dark&legend=top-left&sealed_token=8W0tNyXJm7iAjKqmFUwlIiWBXo7Zu9aVDMAvRqWWaS9ju5eUtGH3Pz4SkHxMOBg5nGdU3KnXkw86SXzzzPcEK4R6_Iqt-HMzpsReNO4lxpA5-8WNB8bEvg" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=Rescenix/ResceneAgent&type=date&legend=top-left&sealed_token=8W0tNyXJm7iAjKqmFUwlIiWBXo7Zu9aVDMAvRqWWaS9ju5eUtGH3Pz4SkHxMOBg5nGdU3KnXkw86SXzzzPcEK4R6_Iqt-HMzpsReNO4lxpA5-8WNB8bEvg" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=Rescenix/ResceneAgent&type=date&legend=top-left&sealed_token=8W0tNyXJm7iAjKqmFUwlIiWBXo7Zu9aVDMAvRqWWaS9ju5eUtGH3Pz4SkHxMOBg5nGdU3KnXkw86SXzzzPcEK4R6_Iqt-HMzpsReNO4lxpA5-8WNB8bEvg" />
 </picture>
</a>
