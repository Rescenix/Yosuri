---
name: frontend-design
description: 多引擎设计系统 — 50+ 真实设计系统参考，按任务类型自动匹配设计风格。
---

# 前端设计引擎

Rescene 内置 **54 个真实设计系统** 的压缩 token 包。生成前端代码前，Agent
根据任务类型自动选择对应设计风格并加载其色板/字体/组件规范。

## 设计路由规则

Agent 收到前端任务时，按此规则选择设计风格：

| 任务类型 | 推荐设计风格 |
|---|---|
| 仪表盘 / 开发工具 | Linear, Vercel, Supabase, Cursor, Sentry |
| 落地页 / 营销页 | Stripe, Framer, Apple, SpaceX |
| 文档站 | Notion, Mintlify, Sanity |
| 数据看板 | Sentry, ClickHouse, Kraken |
| 暗色主题 | Linear, Cursor, ElevenLabs, Warp |
| 亮色/简约 | Vercel, Stripe, Notion, Replicate |
| 高级/奢华 | Apple, BMW, Stripe, Revolut |
| 终端/代码 | Ollama, OpenCode, xAI, VoltAgent |
| AI 产品 | Claude, Cohere, Replicate, Mistral |
| 社交/内容 | Notion, Pinterest, Intercom |
| 金融/电商 | Stripe, Coinbase, Wise, Revolut |

## 使用方式

Agent 在生成前端代码前读取对应 token 文件：

```
tokens/INDEX.md  — 完整索引 + 类型映射
tokens/linear.app.tokens.md   — Linear 风格 (暗色 · 开发工具)
tokens/vercel.tokens.md       — Vercel 风格 (亮色 · 开发工具)
tokens/stripe.tokens.md       — Stripe 风格 (亮色 · 金融)
... 共 54 个设计系统
```

每个 token 文件包含：色板(CSS变量) · 字体层级 · 组件样式 · 阴影系统。

## 设计原则

- **一屏一个视觉重心**：用户第一眼该落在哪里先想清楚
- **8pt 栅格**：所有间距取 4/8 的倍数
- **一个主色 + 中性灰阶**：主色只用于强调
- **交互态四件套**：default / hover / active / disabled
- **120-200ms 过渡**：只动 opacity/transform
