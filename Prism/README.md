# Prism — 数字海马体

**Prism** 是一个仿生记忆引擎，模拟生物海马体的记忆机制，为 AI 提供持久化、可检索、会遗忘的长期记忆系统。

## 架构总览

```
┌─────────────────────────────────────────────────────────┐
│                    prismd-visual (React)                 │
│              神经元图谱可视化 · ECharts                   │
├─────────────────────────────────────────────────────────┤
│                    PrismD (Go HTTP 服务)                  │
│         PrimQL 协议 · 图结构记忆 · 多域管理                │
├─────────────────────────────────────────────────────────┤
│              internal/memory (Go 核心引擎)                │
│   图结构 · 突触扩散 · 倒排索引 · LLM压缩 · 记忆整理        │
├─────────────────────────────────────────────────────────┤
│             Prism C API (C++17 共享库)                    │
│         向量存储 · 忆阻器混沌演化 · 余弦检索               │
└─────────────────────────────────────────────────────────┘
```

## 核心概念

| 概念 | 对应生物模型 | 说明 |
|------|-------------|------|
| **神经元 (Node)** | 神经元 | 一条记忆，包含文本、情绪、强度、能量 |
| **突触 (Synapse)** | 突触连接 | 记忆之间的关联，有类型和权重 |
| **能量 (Energy)** | 记忆强度 | 随时间衰减，被访问时增强，驱动"用进废退" |
| **簇 (Cluster)** | 脑区 | UserBase / CodeWork / ToolLog / Session 四类记忆隔离 |
| **域 (Domain)** | 人格/角色**  不同用户或 AI 角色的记忆空间隔离 |
| **激活扩散** | 联想激活 | 从种子节点沿突触传播，唤醒关联记忆 |

## 组件说明

### PrismD — Go HTTP 服务

主服务进程，提供 PrimQL 文本协议接口。

```bash
cd Prism
go run cmd/prismd/main.go -port 5666 -data ./data -domain Atri
```

**PrimQL 命令：**

| 命令 | 功能 |
|------|------|
| `ENGRAM <role> <text>` | 写入一条记忆 |
| `LOOM <query>` | 检索相关记忆（倒排索引 + 图扩散） |
| `LOOM <id>` | 按 ID 查看单条记忆 |
| `LOOM -N <n>` | 查看最近 N 条记忆 |
| `REFRACT <json>` | 更新记忆（强化或修改） |
| `PRUNE <id>` | 遗忘一条记忆（能量置零） |
| `DRIFT` | 全局演化（能量衰减 5%） |
| `COMPILE <turns>` | 后台压缩对话为摘要记忆 |
| `COMPILE_SYNC <turns>` | 同步压缩 |
| `CONSOLIDATE` | 记忆整理（合并重复、丢弃无用） |
| `GRAPH` | 导出完整图结构（JSON） |
| `STATS` | 查看所有记忆状态 |
| `STATS FULL` | 查看完整记忆详情 |
| `DOMAIN USE/CREATE/LIST/DROP` | 域管理 |

### internal/memory — 记忆引擎核心

| 文件 | 职责 |
|------|------|
| `graph.go` | 图结构、节点/突触 CRUD、激活扩散、多信号融合打分 |
| `compiler.go` | LLM 驱动的记忆压缩，将对话压缩为高密度摘要 |
| `consolidate.go` | 记忆整理：合并重复记忆、丢弃无价值记忆 |
| `inverted.go` | 中文倒排索引（基于 gse 分词） |
| `query_analyzer.go` | LLM 意图分析，提取查询意图、情绪、实体 |
| `gate.go` | 记忆门控：判断当前对话是否需要检索长期记忆 |

### Prism C API — 向量存储层

C++17 编写的轻量级向量存储引擎，提供纯 C API。

```bash
cd Prism/prism
mkdir build && cd build
cmake .. && cmake --build .
```

| API | 功能 |
|-----|------|
| `prism_open(path)` | 打开存储 |
| `prism_insert(store, role, content, keywords, embedding, dim)` | 插入记忆 |
| `prism_search(store, query_vec, dim, top_k, results, min_score)` | 向量检索 |
| `prism_get_all_states(store, infos, max)` | 获取忆阻器状态 |
| `prism_set_evolution(store, enable)` | 开关混沌演化 |

### prismd-visual — 可视化面板

React + ECharts 前端，实时展示记忆图谱。

```bash
cd Prism/prismd-visual
npm install && npm run dev
```

### scripts/ — 桥接脚本

`prismd-bridge.ps1`：将 Claude Code 的 memory/*.md 文件同步到 PrismD 记忆场。

```powershell
# 创建记忆
.\prismd-bridge.ps1 -File path\to\memory.md -Action create

# 查询记忆
.\prismd-bridge.ps1 -Query "用户偏好"
```

## 记忆生命周期

```
写入 (ENGRAM)
    ↓
能量衰减 (DRIFT / 时间自然衰减)
    ↓
访问增强 (LOOM 检索时 +0.01)
    ↓
后台压缩 (COMPILE → LLM 摘要)
    ↓
夜间整理 (CONSOLIDATE → 合并/丢弃)
    ↓
能量归零 → 遗忘 (PRUNE)
```

## 依赖

- Go ≥ 1.25
- CMake ≥ 3.12 (C++ 层)
- Ollama (本地 LLM，用于压缩/意图分析/整理)
- Node.js ≥ 22 (可视化面板)

## 许可证

内部工具，未指定开源许可证。
