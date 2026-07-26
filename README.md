# ResceneAgent

> **前端特化的多智能体作战平台** —— 把 IDE、终端、浏览器和一支 AI 团队，塞进同一个对话框。

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?logo=go&logoColor=white" alt="Go >= 1.26">
  <img src="https://img.shields.io/badge/Node-%3E%3D22-339933?logo=nodedotjs&logoColor=white" alt="Node >= 22">
  <img src="https://img.shields.io/badge/LLM-Multi--Provider-ff69b4" alt="Multi-Provider LLM">
</p>

**ResceneAgent** 以数字生命「Aurora」为核心，将 **多 Agent 团队协作**、**MCP 工具生态**、**本地多提供方模型路由**、**内嵌终端 / 文件树 / Diff / 实时浏览器自检** 与 **面向 AI Agent 的独立记忆 / 状态层** 整合进同一条工作流——从需求拆解 → 代码落地 → 运行验证，闭环在聊天框里完成。

**不止会聊，更会写、会跑、会验。**

---

## 核心理念 · AgentFS：给 AI 的写操作加上"事务"

传统 AI 编程最致命的弱点不是写不出代码，而是**改坏了收不回**：AI 联动改写十个文件，改到第五个崩了，前四个已经脏写进硬盘，没有人能完美回滚。

ResceneAgent 把"文件写操作"从直接的磁盘 IO，重构成一条**带隔离、可审计、可回退的事务层**——我们称之为 **AgentFS**：

- **内存快照隔离** —— AI 的每一次改动先落入隔离快照，不立即污染你的项目；
- **原子提交 / 回滚** —— 改动经编译、测试、人工确认后，才一次性刷入磁盘；失败则整体回退，项目完好如初；
- **像素级审计时间线** —— 每一行修改都带着操作来源，可像 Code Review 一样逐条 Approve / Reject；
- **时间旅行调试** —— 沿快照时间线往回跳跃，定位"哪一版还能编译通过"。
- **会话级可视化轨迹** —— 每个聊天会话拥有独立的 AgentFS 修改树；折叠侧栏即可浏览 Agent 留下的 Git 痕迹，点击节点直接查看该时刻的文件 Diff。

> 思想根基：快照隔离与回退并非新发明，它源自 VFS、Git 与数据库事务等成熟工程范式。ResceneAgent 的真正差异，是把这套能力**系统性地做成了面向 AI Agent 的独立写操作事务层**——这也是平台最深的护城河。

**v1 已落地**：每个项目会话（SetWorkdir）自动开辟独立影子 git 仓库（`~/rescene_data/agentfs/`），与你的项目仓库物理隔离、绝不污染主仓库；每一次文件写操作在落盘前后被捕获为 before/after 快照并提交，形成可 `git log` / `git diff` / `git checkout` 的时间线。审计记录进一步绑定真实聊天会话 ID：切换对话时，界面同步切换到该会话专属的修改历史，不会把不同 Agent 任务的轨迹混在一起。

---

## 核心理念 · 记忆 / 状态双轨：让 Agent 不再失业

传统 AI 编程助手每次开新会话都从零开始——上一轮踩过的坑、这个项目做到哪了、用户的偏好，统统忘光。ResceneAgent 把「跨对话记忆」与「跨项目状态」做成了**面向 AI Agent 的独立记忆 / 状态层**：

- **项目级 workdir + 全局 MEMORY 双轨**：`MEMORY.md`（`~/rescene_data/MEMORY.md`）承载用户 / 系统级常驻记忆（身份、偏好、全局决策）；`workdir.md`（`~/rescene_data/projects/<项目名>/workdir.md`）按项目隔离，记录「这个项目现在在做什么、关键上下文、待办、约定」。两者物理隔离在 `rescene_data` 下，**不污染你的仓库**。
- **Agent 自己通过工具主动写入**：记忆不是靠 prompt 硬塞，而是 Agent 在对话中通过 MCP 工具（`memory_append` / `memory_pin` / `memory_handoff` / `workdir_write` / `workdir_append`）主动决定「什么值得记住」。只有用户选择记住的内容才落盘，避免把对话垃圾灌进记忆。
- **跨对话持久化**：记忆以单文件形式落盘，进程重启后自动重新加载，下一次会话无缝续上。
- **会话开始自动注入上下文**：每个工作流启动时，`workdir.md` 与 `MEMORY.md` 无条件拼进系统提示词——Agent 一开口就了解项目概况与历史约定，**不再失忆**。

> 思想根基：单文件事实库与「模型主动写、启动时读」的范式并不新奇，它借鉴了笔记软件与 VFS 的成熟思路。ResceneAgent 的差异，是把这套机制**系统性地做成了面向 AI Agent 的独立记忆 / 状态层**——agent 既是读者也是作者，记忆随工作流自然生长，而非依赖人工维护的外部知识库。

---

## 特性一览

### AgentFS Trace：会话级 Git 痕迹树

这是 ResceneAgent 面向 AI 编程审计打造的原创交互：侧边栏折叠后，空白区域不再只是导航占位，而是一条持续生长的 **AgentFS 竖式快照树**。

- **一会话一条轨迹**：AgentFS 提交绑定聊天会话 ID；切换会话，修改树随之切换
- **一次写入一个节点**：Agent 每次 `write_file` / `edit_file` 都留下提交节点、文件路径、操作类型、时间与 before/after 哈希
- **实时生长**：Agent 工作期间自动轮询，新快照无需刷新页面即可进入时间树
- **悬浮 Diff 卡片**：点击任意节点，在树右侧打开玻璃质感预览，展示提交号、增删统计、行号和逐行着色 Diff
- **影子 Git 隔离**：所有轨迹来自 `~/rescene_data/agentfs/` 下的独立仓库，不向用户项目写入额外提交
- **时间旅行基础**：节点对应真实影子 Git commit，为后续按文件恢复、版本对比和 Agent 行为回放提供稳定锚点

这让用户不必等 Agent 完成后再检查一个巨大的最终 Diff，而能随时回答三个问题：**这个 Agent 在当前会话改了什么、按什么顺序改、每一步具体改变了哪些行。**

---

### Agent 主动 TODO：计划不再藏在思考里

复杂任务开始后，Agent 会通过内置 `update_todo` 工具发布结构化任务清单，并在执行过程中持续更新，而不是只在聊天文字里说一句“我准备分三步完成”。

- **三态进度**：每个任务项具有 `pending`、`doing`、`done` 状态，同一时间突出当前正在执行的步骤
- **实时可视化**：TODO 通过 SSE 推送到前端便签，用户无需翻阅消息即可掌握整体进度
- **Agent 自主管理**：完成一步后，Agent 主动勾选已完成项并推进下一项
- **长任务不丢计划**：当前 TODO 会作为系统维护的权威状态重新注入每轮上下文；即使发生上下文压缩，Agent 也不会忘记主线或重复已完成工作
- **断点恢复**：TODO 清单随工作流检查点保存，后端重启或连接中断后可随任务一起恢复

这不是静态 Plan 页面，而是一份由 Agent 在实际执行中维护、用户可以实时监督的活计划。

---

### Agent 主动向用户提问：真正的 Human-in-the-loop

当任务出现必须由用户决定的分叉时，Agent 可以调用 `ask_user` 主动发问。工作流会在当前步骤同步暂停，把问题直接显示在输入框上方，收到回答后从原位置继续执行。

- **可交互选项**：支持 2–5 个候选项、单选、多选以及“其他”自由输入
- **同步等待**：问题发出后工作流不会擅自猜测或继续修改，用户回答会作为正式上下文注入执行循环
- **只在真正需要时询问**：能通过读文件、运行命令或查询 API 自行确定的信息，Agent 不会推给用户决定
- **不中断任务链**：回答通过当前 workflow ID 精确送回等待中的 Agent，不会串到其他会话或任务
- **超时兜底**：后端提供 5 分钟保护超时，可使用问题自带的 fallback 继续，避免异常页面让工作流永久挂起

它与普通聊天消息不同：这是 Agent 在执行现场发起的结构化决策请求，也是审批之外更通用的人机协作通道。

---

### 断点续传

工作流每轮自动快照，重启或断网后无缝恢复。

- 每轮工具结果收集后，完整状态（消息历史、已激活工具、TODO 清单、Token 计数）原子写入磁盘
- 后端重启 / SSE 断连后，前端自动检测未完成任务并展示「上次任务未跑完，第 N 轮中断」恢复条
- 恢复时从快照轮次重放完整消息历史，无需从头执行

![断点续传](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=断点续传+恢复条截图)

---

### 内嵌终端 + 自定义脚本片段

不是装饰——是真正的 PowerShell 进程，stdin/stdout 通过 OS 管道直连。

- 每个终端会话对应一个独立 `powershell.exe` 进程，关闭面板只断开 SSE，不杀 Shell
- 64KB 滚动缓冲区，ANSI 颜色转 HTML，Ctrl+C 中断，命令历史
- **脚本片段面板**：保存常用命令片段，一键插入终端，告别重复敲命令
- 嵌入式 Dock 面板布局，与编辑器/文件树/浏览器无缝共存

![内嵌终端](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=内嵌终端+脚本片段截图)

---

### 内置编辑器 + 文件树

嵌入聊天面板的类 VS Code 编辑体验。

- **Monaco Editor**：语法高亮、多标签页编辑、标签固定到侧栏、右键菜单（固定/关闭）
- **递归文件树**：文件夹展开/折叠、文件类型徽章（JS/VUE/PY/PS1/JSON/TXT）、右键复制路径
- **文件搜索**：大小写敏感、全词匹配、正则表达式
- 读写通过后端 API 实时同步，Ctrl+S 保存，保存状态指示器

![编辑器和文件树](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=编辑器+文件树截图)

---

### 实时渲染调试浏览器

Agent 写完 HTML 后，真实 Chromium 引擎自动渲染——不是 iframe。

- 后端检测到 Agent 改了前端文件 → 自动在 Chrome（CDP 端口 9222）打开新标签页
- 通过 CDP Screencast 抓取 PNG 帧，经 WebSocket 中转回前端面板的 `<img>` 标签
- **两种模式**：CDP 真实渲染（优先）/ iframe 降级（CDP 不可用时）
- 导航栏：前进/后退/刷新/URL 输入/移动端视口切换/外部打开

![实时预览浏览器](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=实时渲染调试浏览器截图)

---

### Diff 预览

VS Code 风格的差异查看器，内联于 Agent 工作流。

- 文件列表即时加载（仅元数据），展开时按需获取内容
- 语法高亮：highlight.js 自动检测语言（30+ 语言映射）
- 上下文折叠：仅显示变更行 ±3 行，点击展开全部
- 行号、增删着色、文件状态徽章（M/A/D/R/U）
- 搜索过滤、右键复制完整路径

![Diff预览](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=Diff预览截图)

---

### Agent 消息流式渐变动画

模仿 ChatGPT/Gemini 的逐字渐入效果，带模糊到清晰的过渡。

- 每个新字符独立动画：`opacity: 0 + blur → opacity: 1 + blur(0)`
- 级联延迟形成「瀑布」渐变尾迹
- 所有参数可调：渐入时长、字符间延迟、最大扫描时间、初始模糊像素
- 动画结束后自动清理 DOM Span，防止性能退化

![流式动画](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=流式渐变动画截图)

---

### 魔女审判二次元皮肤

不止换色——是覆盖每个 UI 表面的完整「皮肤」系统。

- **6 套主题**：矢车菊（蓝）、樱花（粉）、薰衣草（紫）、金盏花（橙）+ 2 套完整皮肤
- **原初审判**（魔女审判系列）：审判厅/烛火/铁链血迹美学，衬线字体，古卷纹理消息气泡
- **二阶堂希罗**（魔女审判系列）：红黑洛丽塔风格，彼岸花/蕾丝蝴蝶结，手写字体
- 三档亮度模式（亮色/暗色/跟随系统），20+ CSS 变量运行时注入
- **Live2D 看板娘**：可拖拽的二次元角色挂件

![二次元皮肤](https://via.placeholder.com/800x400/1a1a2e/e0e0e0?text=魔女审判皮肤截图)

---

## 平台能力

### 多 Agent 调度

四态机工作流编排：思考 → 意图 → 动作 → 结果。

- **主 Agent + 雨燕（Swift）子 Agent**：主 Agent 通过 `dispatch_agent` 派发只读研究任务，子 Agent 并行执行
- **工作流中途转向**：用户可在执行过程中注入消息，自动注入下一轮上下文
- **工具审批门控**：Ask / Plan / Yolo 三种模式，危险操作需用户确认
- **上下文感知压缩**：超过窗口 80% 时自动压缩早期轮次，保留文件路径/行号/标识符
- **任务 TODO 清单**：Agent 主动发布并维护 pending / doing / done 三态清单，实时便签展示并跨上下文压缩保留
- **执行中主动提问**：Agent 通过 `ask_user` 暂停工作流，提供单选、多选或自由输入选项，收到用户决策后原地续跑
- **多后端模型路由**：自动故障转移，支持 Vision/Reasoning 能力元数据

### MCP 富集工具生态

基于 Model Context Protocol 的可扩展工具系统。

- **内置 MCP 工具**：文件系统读写、Grep 搜索、Web Fetch、截图/视觉测试、Chrome DevTools 自动化、Shell 执行
- **按需加载**：默认仅注入工具名 + 一行描述到上下文，模型调用 `load_tools` 激活完整 Schema，节省 ~91% 静态 Token 预算
- **审批机制**：MCP 文件写入、Shell 执行、Chrome 交互等危险工具需用户批准，支持「本次会话不再询问」

### 技能自学习

工作流成功后自动抽象可复用技能。

- **学习型技能**：2 步以上工作流完成后，LLM 异步抽象行动序列为 JSON 技能（名称 + 描述 + 步骤）
- **外部技能导入**：支持 Anthropic/Claude 风格的 SKILL.md 文件，YAML 前置元数据
- **索引注入**：技能名 + 描述注入系统 Prompt，Agent 按需通过 `read_skill` 获取完整步骤
- 持久化到 `./skills/*.json`，跨会话可用

### 记忆 / 状态双轨（详见上文「核心理念」）

Agent 既是记忆的读者也是作者：通过 MCP 工具主动写入 `MEMORY.md`（全局）与 `workdir.md`（项目级），跨对话持久化，并在每个工作流启动时自动注入上下文，彻底告别「每次会话从零开始」。

- **全局 MEMORY.md**：用户画像、偏好、全局决策，无条件注入系统提示词
- **项目级 workdir.md**：按项目隔离的当前状态、待办、约定，会话开始即载入
- **零外部依赖**：单文件落盘于 `~/rescene_data/`，无需额外服务或数据库

---

## 企业级安全与校验

AI 编程的安全事故，九成源于"全自动模式下无人看守的破坏性操作"与"写完不验证"。ResceneAgent 把这两道防线做成了**平台级强制规范**，而非靠 prompt 提醒 agent 自觉——这是它区别于通用编码助手的企业级底色。

### 不可逆操作零豁免审批

即便是 YOLO 全自动模式（默认对所有危险工具畅通无阻），**删除文件 / 删除目录 / 移动文件**三类不可逆操作也强制弹审批，绝不静默放行。普通写盘（write/edit/create）仍可畅通，但"改坏了能还原"与"删没了找不回"是两个量级的风险，后者不在 YOLO 的豁免范围内。

### 收尾强制构建 + 截图校验（Post-Workflow Verification Gate）

验证只在 agent **决定结束对话的那一刻**跑一次——不每轮、不每步打扰（避免"动不动就验证"打断思路）。后端在 `workflow_done` 推送前自动触发：

- 本轮改动了 `.go` → 自动 `go build ./...`，结果回传 build 状态
- 本轮改动了前端文件（`.vue` / `.ts` / `.html` 等）→ 自动 `npm run build`，并复用真实 Chromium 渲染截图
- 校验结果作为 `verification` 事件推给前端，附 build 通过状态与预览截图路径

任何环节失败（命令缺失 / 超时 / 构建报错）都只记录状态、**绝不阻断对话收尾**——校验是加分项，不是阻断项。

### AgentFS 审计溯源（见上「核心理念」段）

每一次写操作带着 before/after 哈希、工具来源、会话 ID 落入不可篡改的审计时间线，配合影子 git 仓库，做到"谁在什么时候改了什么、能不能一键回退"全程可审计。

---

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
│ 四态机工作流 · 多Agent调度 · 实时TODO · 主动提问 · MCP · 记忆  │
├──────────────────────────────────────────────────────────────┤
│ AgentFS：会话级快照树 · 影子 Git · Diff 审计 · 时间旅行基础     │
├──────────────────────────────────────────────────────────────┤
│     记忆层：MEMORY.md(全局) + workdir.md(项目级)，单文件落盘 ~/rescene_data/  │
│     双轨持久化 · 会话开始自动注入上下文 · Agent 主动写入                  │
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
│   │   ├── agentfs.go                  # 会话级影子 Git、审计时间线与 Diff
│   │   ├── agent_workflow_handler.go   # 四态机工作流
│   │   ├── browser_preview_tool.go     # CDP 实时预览
│   │   ├── terminal_handler.go         # 内嵌终端
│   │   ├── mcp_client.go               # MCP 客户端
│   │   ├── skill_library.go            # 技能自学习
│   │   ├── subagent.go                 # 雨燕子Agent
│   │   └── workflow_checkpoint.go      # 断点续传
│   ├── skills/                         # 学习到的技能
│   └── mcp/                            # 自研 MCP server（grep/shell/memory…）
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

### 记忆能力（双轨）

记忆层随 `main-backend` 启动自动生效，无需单独部署。默认已通过 `mcp.json` 注册 `memory` MCP server，提供 `memory_*` / `workdir_*` 工具；`MEMORY.md` 与 `workdir.md` 落盘于 `~/rescene_data/`，Agent 在对话中主动调用工具写入，并在每个工作流启动时自动注入上下文。

## 环境变量

| 变量 | 说明 |
|------|------|
| `ADMIN_PASSWORD` | 管理员密码 |
| `JWT_SECRET` | JWT 签名密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `MCP_CONFIG` | MCP server 配置文件路径（默认 `./mcp.json`），记忆服务在此注册 |
| `RESCENE_DATA_DIR` | 记忆 / AgentFS / 会话数据根目录（默认 `~/rescene_data`） |
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
