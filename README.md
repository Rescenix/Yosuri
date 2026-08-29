[中文](./README.md) · [English](./README.en.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

<p align="center">
  会成长的 AI 工作台 —— 让 AI <b>记住</b>、<b>执行</b>并<b>持续进化</b>
</p>

一个长期陪伴你的 AI 工作台：**双端同步**让上下文不断线，**自动化**把任务推进到底，**聚合 API**统一模型入口；它会从每次协作中**自学习**，并用**长期记忆**越来越懂你。

<p align="center">
  <a href="https://rescene.shanca.me/download.html">
    <img src="https://img.shields.io/badge/下载-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20Android-4FC08D.svg?style=for-the-badge" alt="下载">
  </a>
  <a href="https://rescene.shanca.me/">
    <img src="https://img.shields.io/badge/官网-rescene.shanca.me-4FC08D.svg?style=for-the-badge" alt="官网">
  </a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-AGPLv3-blue.svg" alt="AGPL-3.0 License"></a>
  <img src="https://img.shields.io/badge/Release-v0.1.5-blue" alt="Release v0.1.5">
  <img src="https://img.shields.io/badge/Backend-Go-00ADD8" alt="Go">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
</p>

---

## ⚡ 核心能力

| 能力 | 说明 |
| --- | --- |
| **🔄 双端同步** | 账号、会话与长期记忆在设备之间自然接续。你换的是屏幕，不是工作上下文：会话实时衔接、记忆可选备份、工作状态继续 |
| **🤖 自动化闭环** | 浏览器、终端和工具调用组成一条可验证的执行链，任务不再停在回答里：自动拆解目标 → 调用真实工具 → 闭环验证结果 |
| **🔌 聚合 API 端口** | 不同提供方与本地模型被收束为一个 OpenAI 兼容入口，配置一次到处调用：统一模型入口、智能路由与负载、提供方自由组合 |
| **🧠 自学习与长期记忆** | 每次协作都留下可复用的经验：偏好、决策与项目知识在真正需要时被重新召回，跨会话关联、经验自动沉淀，越用越贴近你 |

## 🌟 更多特性

| 特性 | 说明 |
| --- | --- |
| **🧲 免费模型池** | 多家免费提供方聚合成一个 OpenAI 兼容端点，30 分钟探活打分、每日重探自动退役下架源、熔断跳过限流、LRU 权重优先最近可用 |
| **🖱️ Computer Use** | 不止会改代码——能操作桌面：截图、鼠标、键盘、拖拽、滚动。真实的点击、真实的按键 |
| **🌐 真实浏览器自动化** | 复用系统浏览器 + CDP：渲染、点击、输入、滚动、读 DOM、截图、双向验证。真浏览器在跑你的页面，不是截图假装 |
| **🛡️ AgentFS 变更审计** | AI 每次改文件都有快照 / Diff / 回滚，危险操作必须经你批准 |
| **🛠️ 技能自动沉淀** | 每次工作流完成自动萃取经验：模型偏好、代码风格、项目架构——下次自动融入上下文，永远不需要写自定义指令 |

---

## 🚀 下载与安装

👉 **[https://rescene.shanca.me/download.html](https://rescene.shanca.me/download.html)** 👈

**桌面版（全平台）**
- **Windows 版** — 便携版 ZIP / 安装器，解压即用，自动更新
- **Linux 版** — 桌面客户端 tar.gz（GTK/WebKit），官网直接下载
- **macOS 版** — 桌面客户端 tar.gz
- **Android 版** — 移动端同步使用

**终端版（CLI，一行安装）**
- **Linux / macOS / 手机 Termux：**
  ```bash
  curl -fsSL https://download.shanca.me/rescene-cli/install.sh | sh
  ```
- **Windows (PowerShell)：**
  ```powershell
  irm https://download.shanca.me/rescene-cli/install.ps1 | iex
  ```

- **极致轻量** — 不内置浏览器，无需预装 Node.js / Python
- **自动更新** — 发现新版本自动下载安装，配置保留

> 📢 遇到问题或想提建议，加入 QQ 群：**一群 609967535**（即将满员）· **二群 796474621**（新开）

## ⚙️ 首次使用

1. 打开工作台 → **设置 → 模型**，填入至少一个 API Key；免 Key 源在免费池里直接可选
2. 或用环境变量配置模型源：参考 `main-backend/.env.example`
3. 免费池每 30 分钟探活、每日重探提供方列表：限流的自动降权、下架的自动退役

## 🛠️ 源码编译（贡献者）

### Windows

```powershell
# 后端（Go 1.22+）
cd main-backend
go run cmd/server/main.go

# 前端（Node 18+）
cd main-frontend/beneficial-belt
npm install && npm run dev
```

访问 `http://localhost:4322` 打开本地开发工作台。

### Linux

完整依赖安装与构建步骤见 [`main-backend/docs/linux-build.md`](./main-backend/docs/linux-build.md)。

```bash
bash scripts/build-linux.sh amd64   # 或 arm64
```

## 💬 反馈与开源协议

- 🐛 Bug / 建议 → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- 💬 交流 → [QQ 群 796474621](https://qm.qq.com/q/796474621)
- 核心代码：[AGPL-3.0 License](./LICENSE)
