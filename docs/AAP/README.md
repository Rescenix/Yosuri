# 🌌 Aurora Application Protocol (AAP) Specification — v0.1.0-alpha

> **"We don't build tools to comply with black-box servers; we define the physical constants of digital life."**

## 1. 协议愿景 (Vision)

Aurora 应用协议 (AAP) 是一种专为**本地物理记忆引擎、分布式多智能体自治协作与去中心化因果哈希链审计**设计的下一代 AI 交互总线协议。

现有的模型上下文协议 (MCP) 仅解决了云端无状态 LLM 与本地文件系统之间的盲目请求/响应，本质上是一种低效的“无记忆邮差”。**AAP 实现了对 MCP 客户端的向下兼容，但通过在网络序列化层原生注入语义图权重（Conductance）与时空因果证书（Causal Hash Chain），将工具调用升维为具备自适应演化能力的神经网络递质。**

## 2. 核心架构拓扑 (Topology)

在 AAP 拓扑结构中，**Planner**、**Coder**、**Reviewer** 以及本地的 **PrismD 引擎**不再是被主程序闭源调用的静态函数，而是挂载在 AAP 骨干总线上的**自治服务节点 (Autonomous Microservices)**。
[ Aether UX / Core State Machine ]
|
=== AAP Broadcast Bus (JSON-RPC + Context) ===
/ | \
[Planner] [Coder] [Reviewer] [PrismD]
(Nash-Eq) (Tool Exec) (Diff Audit) (Memory Graph)

text

当 Aether 终端广播一条高维指令时，各节点基于 AAP 协议独立进行纳什均衡的局部推演，向主进程返回各自计算的“状态势能”，由主进程实现最终的事务裁决。

## 3. 三大核心原语 (The Three Primitives)

AAP 的状态机流转完全由以下三大核心原语进行物理驱动：

### 🔄 原语一：`MEMORY_SYNC`

*   **定义**：客户端或智能体节点在发起任何工具调用（Tool Call）或局部检索前，必须强制同步当前 PrismD 记忆场中 358 个节点与 862 个突触的语义上下文权重。
*   **物理特性**：在传输的 Payload 中，不仅包含传统的文本，还必须携带由 `chaos`（无序度）与 `conductance`（导通率）定义的当前**世界线坐标（World-Line Coordinates）**。
*   **效益**：Agent 接到指令的瞬间即继承了全量记忆视野，彻底免除多轮对话中向云端重新上传无关上下文的 Token 摩擦。

### 📢 原语二：`AGENT_BROADCAST`

*   **定义**：定义多 Agent 之间任务动态拆解、非对称分发与最终结果合并的纳什均衡收敛规则。
*   **物理特性**：放弃传统的线性串行调用。主进程通过 AAP 发起广播，多个自治脑区节点（如 Coder、Reviewer）同时对输入的 Diff 补丁进行安全与正确性对抗推演，最终通过 AAP 结构化协议回传势能参数，实现秒级的多步规划收敛。

### 📜 原语三：`CHAIN_HASH`

*   **定义**：用于自证清白、对抗大厂云端暗箱降级的**全链条不可篡改审计证书**。
*   **物理特性**：每一次由 Agent 触发的物理动作（如 `edit_file`、`execute_command`），其输入、输出、当前行号物理坐标以及执行 Agent 的签名密钥，都会被打包哈希，串联进当前项目的因果哈希链（Causal Hash Chain）中。
*   **效益**：从根本上杜绝了类似于 `TOO_DUMB_TO_NEED_FABLE` 这种欺骗式的黑盒隐式降级，每一行代码的诞生都有迹可循、不可伪造。

## 4. 核心通信帧示例 (Payload Spec)

当 Coder 节点通过 AAP 发起一个局部物理缝合请求时，其标准通信帧（相比 MCP 额外扩展了 `_aurora_context` 记忆与因果段）如下：

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "edit_file",
    "arguments": {
      "path": "src/styles/chat-window.css",
      "start_line": 425,
      "end_line": 464,
      "patch": "/* AAP 精准手术刀缝合代码 */"
    },
    "_aurora_context": {
      "causal_hash": "0x7f8a9c2b3d4e5f6a",
      "parent_hash": "0x1a2b3c4d5e6f7a8b",
      "prism_state": {
        "active_nodes": 358,
        "current_conductance": 0.845,
        "parameters": { "alpha": 0.045, "lambda": 0.023 }
      }
    }
  },
  "id": "aap_tx_2026_001"
}
5. 版本历史 (Changelog)
v0.1.0-alpha (2026-07-04): 初始草案。定义三大核心原语 MEMORY_SYNC, AGENT_BROADCAST, CHAIN_HASH。确立向下兼容 MCP 的原则。确立 PrismD 记忆场在 AAP 通信帧中的强制坐标地位。