# ResceneAgent — Aurora 数字生命平台

> 一个以前端特化 Agent 与团队协作能力为核心的 AI 平台。
> 核心角色「Aurora」是一个具有长期记忆、代码工具能力和人格化交互的数字生命，面向多 Agent 协同工作流设计。

---

## 特性一览

<!-- ==================== 核心编程能力 ==================== -->

### 🧠 断点续传

工作流每轮自动快照，重启或断网后无缝恢复。

- 每轮工具结果收集后，完整状态（消息历史、已激活工具、TODO 清单、Token 计数）原子写入磁盘
- 后端重启 / SSE 断连后，前端自动检测未完成任务并展示「上次任务未跑完，第 N 轮中断」恢复条
- 恢复时从快照轮次重放完整消息历史，无需从头执行

![断点续传](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=断点续传+恢复条截图)

---

### 💻 内嵌终端 + 自定义脚本片段

不是装饰——是真正的 PowerShell 进程，stdin/stdout 通过 OS 管道直连。

- 每个终端会话对应一个独立 `powershell.exe` 进程，关闭面板只断开 SSE，不杀 Shell
- 64KB 滚动缓冲区，ANSI 颜色转 HTML，Ctrl+C 中断，命令历史
- **脚本片段面板**：保存常用命令片段，一键插入终端，告别重复敲命令
- 嵌入式 Dock 面板布局，与编辑器/文件树/浏览器无缝共存

![内嵌终端](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=内嵌终端+脚本片段截图)

---

### 📝 内置编辑器 + 文件树

嵌入聊天面板的类 VS Code 编辑体验。

- **Monaco Editor**：语法高亮、多标签页编辑、标签固定到侧栏、右键菜单（固定/关闭）
- **递归文件树**：文件夹展开/折叠、文件类型徽章（JS/VUE/PY/PS1/JSON/TXT）、右键复制路径
- **文件搜索**：大小写敏感、全词匹配、正则表达式
- 读写通过后端 API 实时同步，Ctrl+S 保存，保存状态指示器

![编辑器和文件树](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=编辑器+文件树截图)

---

### 🌐 实时渲染调试浏览器

Agent 写完 HTML 后，真实 Chromium 引擎自动渲染——不是 iframe。

- 后端检测到 Agent 改了前端文件 → 自动在 Chrome（CDP 端口 9222）打开新标签页
- 通过 CDP Screencast 抓取 PNG 帧，经 WebSocket 中转回前端面板的 `<img>` 标签
- **两种模式**：CDP 真实渲染（优先）/ iframe 降级（CDP 不可用时）
- 导航栏：前进/后退/刷新/URL 输入/移动端视口切换/外部打开

![实时预览浏览器](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=实时渲染调试浏览器截图)

---

### 🔀 Diff 预览

VS Code 风格的差异查看器，内联于 Agent 工作流。

- 文件列表即时加载（仅元数据），展开时按需获取内容
- 语法高亮：highlight.js 自动检测语言（30+ 语言映射）
- 上下文折叠：仅显示变更行 ±3 行，点击展开全部
- 行号、增删着色、文件状态徽章（M/A/D/R/U）
- 搜索过滤、右键复制完整路径

![Diff预览](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=Diff预览截图)

---

### ✨ Agent 消息流式渐变动画

模仿 ChatGPT/Gemini 的逐字渐入效果，带模糊到清晰的过渡。

- 每个新字符独立动画：`opacity: 0 + blur → opacity: 1 + blur(0)`
- 级联延迟形成「瀑布」渐变尾迹
- 所有参数可调：渐入时长、字符间延迟、最大扫描时间、初始模糊像素
- 动画结束后自动清理 DOM Span，防止性能退化

![流式动画](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=流式渐变动画截图)

---

### 🧙‍♀️ 魔女审判二次元皮肤

不止换色——是覆盖每个 UI 表面的完整「皮肤」系统。

- **6 套主题**：矢车菊（蓝）、樱花（粉）、薰衣草（紫）、金盏花（橙）+ 2 套完整皮肤
- **原初审判**（魔女审判系列）：审判厅/烛火/铁链血迹美学，衬线字体，古卷纹理消息气泡
- **二阶堂希罗**（魔女审判系列）：红黑洛丽塔风格，彼岸花/蕾丝蝴蝶结，手写字体
- 三档亮度模式（亮色/暗色/跟随系统），20+ CSS 变量运行时注入
- **Live2D 看板娘**：可拖拽的二次元角色挂件

![二次元皮肤](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=魔女审判皮肤截图)

---

## 平台能力

<!-- ==================== AI 核心 ==================== -->

### 🤖 多 Agent 调度

四态机工作流编排：思考 → 意图 → 动作 → 结果。

- **主 Agent + 雨燕（Swift）子 Agent**：主 Agent 通过 `dispatch_agent` 派发只读研究任务，子 Agent 并行执行
- **工作流中途转向**：用户可在执行过程中注入消息，自动注入下一轮上下文
- **工具审批门控**：Ask / Plan / Yolo 三种模式，危险操作需用户确认
- **上下文感知压缩**：超过窗口 80% 时自动压缩早期轮次，保留文件路径/行号/标识符
- **任务 TODO 清单**：Agent 维护结构化待办，实时便签展示
- **多后端模型路由**：自动故障转移，支持 Vision/Reasoning 能力元数据

### 🔌 MCP 富集工具生态

基于 Model Context Protocol 的可扩展工具系统。

- **内置 MCP 工具**：文件系统读写、Grep 搜索、Web Fetch、截图/视觉测试、Chrome DevTools 自动化、Shell 执行
- **按需加载**：默认仅注入工具名 + 一行描述到上下文，模型调用 `load_tools` 激活完整 Schema，节省 ~91% 静态 Token 预算
- **审批机制**：MCP 文件写入、Shell 执行、Chrome 交互等危险工具需用户批准，支持「本次会话不再询问」

### 📚 技能自学习

工作流成功后自动抽象可复用技能。

- **学习型技能**：2 步以上工作流完成后，LLM 异步抽象行动序列为 JSON 技能（名称 + 描述 + 步骤）
- **外部技能导入**：支持 Anthropic/Claude 风格的 SKILL.md 文件，YAML 前置元数据
- **索引注入**：技能名 + 描述注入系统 Prompt，Agent 按需通过 `read_skill` 获取完整步骤
- 持久化到 `./skills/*.json`，跨会话可用

### 🧬 内嵌 PrismStore 长期记忆

图结构记忆库，以 C++ 静态链接形式集成在后端。

- **神经元 + 突触**：记忆节点 + 记忆关联的图结构
- **四类簇隔离**：UserBase（用户画像）、CodeWork（代码决策）、ToolLog（工具日志）、Session（会话临时）
- **忆阻器混沌演化**：电导率/关联流/混沌度建模，能量衰减模拟遗忘曲线
- **LLM 驱动压缩**：对话 → 高密度摘要，自动判断 worth_saving

---

## 其他功能

| 功能 | 说明 |
|------|------|
| 多引擎聊天 | DeepSeek / Ollama / Gemini Vision / DS Browser |
| AI 工具系统 | 语义搜索代码、文件读写编辑、命令执行、记忆检索、代码结构查询 |
| 博客 / CMS | 文章 CRUD、标签管理 |
| 电子书管理 | 上传、阅读、翻页效果 |
| 图床 | 上传、标签、OSS、AI 随机推荐 |
| TTS 语音合成 | 文字转语音 |
| Git 集成 | status / add / commit / push |
| Docker 沙箱 | Python / Go / JS / C 代码执行 |
| 视觉预处理 | Gemini 图片理解、AI 生图 |
| 统计仪表盘 | 使用量概览、每日统计、详情 |
| JWT 认证 | 开发模式后门支持 |

---

## 系统架构

```
┌──────────────────────────────────────────────────────────────┐
│           beneficial-belt (Astro + Vue 3 + Naive UI)         │
│  聊天 · 编辑器 · 文件树 · 终端 · 预览浏览器 · Diff · 皮肤系统  │
├──────────────────────────────────────────────────────────────┤
│              main-backend (Go / Gin :8080)                    │
│   四态机工作流 · 多Agent调度 · MCP工具 · 技能学习 · 记忆 · CMS  │
├──────────────────────────────────────────────────────────────┤
│     记忆层：内嵌 PrismStore (C++ 静态链接)                      │
│     图结构记忆 · LLM压缩 · 簇隔离 · 忆阻器混沌演化              │
├──────────────────────────────────────────────────────────────┤
│           Ollama / DeepSeek / Gemini (外部 LLM)               │
└──────────────────────────────────────────────────────────────┘
```

## 仓库结构

```
re0/
├── main-backend/              # Go 后端服务 (:8080)
│   ├── cmd/server/
│   ├── internal/handler/
│   │   ├── agent_workflow_handler.go   # 四态机工作流
│   │   ├── browser_preview_tool.go     # CDP 实时预览
│   │   ├── terminal_handler.go         # 内嵌终端
│   │   ├── mcp_client.go               # MCP 客户端
│   │   ├── skill_library.go            # 技能自学习
│   │   ├── subagent.go                 # 雨燕子Agent
│   │   └── workflow_checkpoint.go      # 断点续传
│   ├── skills/                         # 学习到的技能
│   └── lib/                            # PrismStore C++ 库
├── main-frontend/
│   └── beneficial-belt/       # Astro + Vue 3 前端 (:4321)
│       └── src/components/shanxi/chat/
│           ├── ChatWidget.vue          # 主聊天界面
│           ├── CodeEditor.vue          # Monaco 编辑器
│           ├── FileToolPanel.vue       # 文件树 + 编辑器
│           ├── Terminal.vue            # 内嵌终端
│           ├── PreviewBrowser.vue      # 实时预览浏览器
│           ├── DiffPanel.vue           # Diff 差异查看
│           ├── DiffViewer.vue          # 内联 Diff 渲染
│           └── AgentWorkflowPanel.vue  # 工作流面板
├── harness/                   # Python 脚本（MCP/测试/工具）
└── docs/                      # 文档
```

## 快速开始

### 前置依赖

- Go >= 1.26
- Node.js >= 22
- Ollama（本地 LLM，可选）
- Docker（代码沙箱，可选）

### 后端

```bash
cd main-backend
# 配置 .env（ADMIN_PASSWORD, JWT_SECRET, DEEPSEEK_API_KEY 等）
go run cmd/server/main.go
```

### 前端

```bash
cd main-frontend/beneficial-belt
npm install
npm run dev    # http://localhost:4321
```

### 记忆能力（内嵌）

记忆库随 main-backend 启动自动生效，无需单独部署。

## 环境变量

| 变量 | 说明 |
|------|------|
| `ADMIN_PASSWORD` | 管理员密码 |
| `JWT_SECRET` | JWT 签名密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `PRISM_API_TYPE` | 默认聊天引擎（ds/local） |
| `DEV_MODE` | 开发模式（true 时任意密码登录） |

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
