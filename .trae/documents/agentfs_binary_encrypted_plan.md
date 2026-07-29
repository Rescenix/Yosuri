# AgentFS 改造计划：二进制加密快照 + GC 机制

## Summary

将 AgentFS 从「基于 Git 的影子仓库」改造为「基于二进制加密块 + 内容寻址」的快照存储系统，解决当前方案的存储膨胀、隐私明文残留、Git 依赖三个核心问题，同时保留回滚、Diff、审计时间线等原有能力，并引入 GC 机制控制存储增长。

## Current State Analysis

### 现有实现

AgentFS 当前实现在 `main-backend/internal/handler/agentfs.go`：

- 每次 `mcp__fs__write_file` / `mcp__fs__edit_file` 真实落盘后，捕获 `before` 与 `after` 的内容哈希
- 将完整文件内容复制到 `~/rescene_data/agentfs/repos/<project>/` 下的影子路径
- 对影子仓库执行 `git add + git commit`，用 commit 作为快照节点
- 审计日志写入 `~/rescene_data/agentfs/repos/<project>/.agentfs/audit.jsonl`
- HTTP API 提供：`/api/agentfs/open`、`/api/agentfs/log`、`/api/agentfs/branches`、`/api/agentfs/diff`、`/api/agentfs/restore`

### 现存问题

1. **存储膨胀**：Git 对单文件多次全量修改的压缩效果有限；每次 commit 都保存文件完整副本，大文件反复修改时体积快速增长
2. **隐私风险**：影子仓库以明文形式保留所有被修改过文件的完整历史，包括 `.env`、密钥等敏感内容
3. **外部依赖**：依赖本机安装 `git`，部分 Windows 用户环境可能缺失或版本过旧
4. **无 GC**：历史 commit 无限累积，没有自动清理策略
5. **跨平台路径问题**：Git 在 Windows 上对长路径、特殊字符支持不如自定义格式可控

## Proposed Changes

### 1. 新增二进制加密快照存储引擎

**新文件**：`main-backend/internal/handler/agentfs_store.go`

职责：
- 内容寻址块存储：`hash -> encrypted blob`
- 快照节点管理：`snapshotID -> {parent, timestamp, tree, message}`
- 加密/解密：使用 AES-256-GCM 或 ChaCha20-Poly1305
- 密钥派生：默认从机器指纹（用户名 + 机器名 + 固定 salt）派生；允许 `RESCENE_AGENTFS_KEY` 环境变量覆盖
- 项目级命名空间：`~/rescene_data/agentfs/<project>/blobs/` 与 `~/rescene_data/agentfs/<project>/snapshots.jsonl`

数据结构：

```go
type blobRecord struct {
    Hash string // sha256(hex)
    Data []byte // encrypted(gzip(original))
}

type snapshot struct {
    ID        string            // afs_<timestamp>_<random>
    ParentID  string            // 上一个 snapshot ID
    Timestamp time.Time
    Tree      map[string]string // relPath -> blob hash
    Op        string            // write / edit
    Tool      string            // mcp__fs__write_file / mcp__fs__edit_file
    Before    map[string]string // relPath -> before hash
    After     map[string]string // relPath -> after hash
}
```

加密细节：
- 先对原始内容做 gzip 压缩，再用 AES-256-GCM 加密
- nonce 随机生成，附在加密数据前
- 密钥通过 PBKDF2 / scrypt 从机器指纹派生，避免硬编码密钥
- 目录权限设为 `0o700`，只允许当前用户访问

### 2. 改造 `agentfs.go` 核心流程

保留：
- `agentfsSession` 结构体及 `OpenAgentFSSession`
- `OnBeforeWrite` / `OnAfterWrite` 调用点
- 审计字段（Seq、TS、Op、RelPath、BeforeHash、AfterHash、Tool、SessionID）

替换：
- 删除 `gitRun`、`currentHead`、`currentBranch`、`gitAvailable` 等 Git 相关函数
- `OnAfterWrite` 中不再写 Git commit，改为：
  1. 读取真实盘的 after 内容
  2. gzip + 加密后写入 blob 目录（按 hash 去重）
  3. 生成新的 snapshot 节点，记录 tree
  4. 更新 `audit.jsonl`（保留原有 JSON Lines 格式）

新增：
- `agentfsSession.Head` 改为当前 snapshot ID（不再是 Git short hash）
- `agentfsSession.Branch` 保留概念，但基于 snapshot 链实现轻量分支指针

### 3. 适配 HTTP API

- **`/api/agentfs/log`**：保持返回 `agentfsAudit` 数组，兼容前端；额外返回 `current_head`（snapshot ID）
- **`/api/agentfs/diff`**：从 store 读取两个相邻 snapshot 的 blob，使用 Go 标准库或 `github.com/sergi/go-diff/diffmatchpatch` 计算文本 diff
- **`/api/agentfs/restore`**：从指定 snapshot 读取 blob，解密解压后写回真实工作目录
- **`/api/agentfs/branches`**：保留分支概念，但分支指针存储在 `branches.json` 中，指向某个 snapshot ID

### 4. 引入 GC 机制

**目标**：控制 `~/rescene_data/agentfs/` 总体积，删除不再被引用的旧 blob。

策略：
- 默认保留每个项目最近 **30 天** 或最近 **50 个 snapshot**（可配置）
- 未被任何保留 snapshot 引用的 blob 可被安全删除
- 提供两种方式：
  - **自动 GC**：每次 `OnAfterWrite` 后异步触发（频率控制，避免 I/O 阻塞）
  - **手动 GC**：HTTP API `/api/agentfs/gc?project=xxx` 或启动参数

GC 实现：

```go
func gcProject(project string, keepSnapshots int, keepDays int) error {
    // 1. 按时间倒序排序 snapshot
    // 2. 保留前 keepSnapshots 个 + keepDays 内的 snapshot
    // 3. 收集所有被保留 snapshot 引用的 blob hash
    // 4. 删除 blobs/ 目录下未被引用的文件
    // 5. 重写 snapshots.jsonl，丢弃已删除的 snapshot 记录
}
```

### 5. 敏感文件过滤（可选但建议）

在 `OnAfterWrite` 中增加敏感文件模式匹配：

```go
var sensitivePatterns = []string{
    "*.env*", "*.pem", "*.key", "*secret*", "*token*",
    "*.p12", "*.pfx", "*credentials*",
}
```

命中后的行为：
- 仍然记录 audit（文件名、hash），但不把内容写入 blob 目录
- restore 时跳过敏感文件，需要用户手动处理
- 避免影子存储里长期保留明文密钥

### 6. 配置项

新增环境变量 / 配置：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `RESCENE_AGENTFS_KEY` | 机器指纹派生 | 自定义加密密钥 |
| `RESCENE_AGENTFS_GC_KEEP_SNAPSHOTS` | 50 | 每个项目保留最近 N 个 snapshot |
| `RESCENE_AGENTFS_GC_KEEP_DAYS` | 30 | 保留 N 天内的 snapshot |
| `RESCENE_AGENTFS_GC_AUTO` | true | 是否自动 GC |
| `RESCENE_AGENTFS_SENSITIVE_FILTER` | true | 是否启用敏感文件过滤 |

### 7. 迁移与兼容

- 新实现完全不依赖 Git；旧的 `~/rescene_data/agentfs/repos/<project>/.git` 目录保留但不再使用
- 提供一次性迁移工具（可选）：把旧 Git 影子仓中的历史 commit 导入为新的 encrypted snapshot
- 若不做迁移，旧的 audit log 仍可读取，新增 snapshot 走新系统；建议 clean start

## Files to Modify / Create

### 新建
- `main-backend/internal/handler/agentfs_store.go` — 二进制加密块存储引擎
- `main-backend/internal/handler/agentfs_crypto.go` — 加密/解密、密钥派生（可与 store 合并）
- `main-backend/internal/handler/agentfs_gc.go` — GC 逻辑

### 修改
- `main-backend/internal/handler/agentfs.go` — 替换 Git 调用为 store 调用，保留 API 契约
- `main-backend/internal/handler/routes.go` — 新增 `/api/agentfs/gc` 路由（若提供手动 GC）
- 相关测试：`main-backend/internal/handler/agentfs_test.go`（如有）

### 删除（代码层面）
- `agentfs.go` 中的 Git 相关函数与变量

## Assumptions & Decisions

1. **不追求「真正沙盒」**：本次改造解决的是 AgentFS 存储与隐私问题，不是让 Agent 完全隔离于真实文件系统之外。Agent 仍然直接修改真实工作目录，只是快照保存方式更安全。
2. **加密密钥本地化**：默认不联网、不上传密钥，密钥从本机信息派生；用户可用环境变量覆盖。
3. **Blob 去重**：相同内容只存一份 blob，hash 作为文件名，天然去重。
4. **Diff 采用文本 diff**：加密 blob 解压后按行计算 diff，与 Git diff 输出格式尽量兼容。
5. **GC 是惰性删除**：不是实时清理，避免频繁 I/O 影响工具调用性能。
6. **不迁移旧 Git 历史**：默认 clean start，避免引入复杂的 Git 解析依赖；如需保留历史可后续单独实现迁移脚本。

## Verification Steps

1. **单元测试**
   - 加密/解密 round-trip：写入 blob -> 读取 blob -> 内容一致
   - 快照链：连续写 3 次文件，snapshot 能正确链式回溯
   - GC：构造 5 个 snapshot，设置 keep=2，验证旧 blob 被删除且剩余 snapshot 可正常 restore
   - Diff：两个相邻 snapshot 的 diff 输出与预期一致

2. **集成测试**
   - 启动后端，运行一个 coding workflow，修改前端文件
   - 调用 `/api/agentfs/log` 验证审计记录正常
   - 调用 `/api/agentfs/diff` 验证 diff 正常
   - 调用 `/api/agentfs/restore` 验证文件能还原
   - 检查 `~/rescene_data/agentfs/<project>/blobs/` 下文件均为加密二进制，无法直接读取

3. **性能测试**
   - 对 1MB 文件修改 10 次，对比改造前后的 `~/rescene_data/agentfs` 总体积
   - 验证 gzip 加密后的体积明显小于 Git 全量 commit

4. **安全测试**
   - 修改一个 `.env` 文件，验证敏感文件过滤生效（audit 记录存在但 blob 不存在）
   - 目录权限检查 `~/rescene_data/agentfs/` 为 `0o700`
