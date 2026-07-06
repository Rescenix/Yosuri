# `/api/workflow/run` SSE 事件契约 —— 前端已实现，后端需要对齐

这份文档描述前端（`useChatLogic.js` 的 `sendWorkflow`）目前依赖的 SSE 事件格式。前端已经按这套契约实现了"消息内嵌三层折叠步骤组"的渲染（`MessageStepGroup.vue` / `BackgroundTaskStep.vue`），后端不管内部是"Planner→Coder→Reviewer 多 Agent 调度"还是"单个 main Agent 一次性处理"，**只要按这份契约发事件，前端不用改一行代码就能正确渲染**。

## 基本假设

- 一次 `/api/workflow/run` 请求 = 前端一个"任务组"（`kind:'group'` 消息），内部可以有 **1 个或多个** step（不强制要求 Planner/Coder/Reviewer 三段式，单 Agent 一次性处理也完全可以，只有 1 个 step）。
- 每个 step 内部可以有 0 到多次工具调用。
- 所有事件字段值都是**字符串**（SSE data 是 JSON，但数字类字段如 token 数、进度百分比也请传字符串，前端会自己 `parseInt`/`parseFloat`，这是现有约定，不是新要求）。

## 事件列表与字段

### `workflow_start`（可选，仅用于展示步骤总数进度，不影响核心渲染）
```json
{ "type": "workflow_start", "total_steps": "1" }
```

### `step_start`（必须，一个 step 开始时发一次）
```json
{
  "type": "step_start",
  "step_id": "step_xxx",
  "agent": "main",           // Agent 内部标识，任意字符串
  "agent_role": "工程师",     // 显示给用户的角色名
  "step_index": "0",
  "prompt": "..."            // 可选，给这个 step 的具体指令
}
```
前端行为：创建一个新的 step 对象，挂进当前任务组的 `steps` 数组。**如果 `agent === 'planner'`，前端会立即显示"架构师正在分析需求..."占位文案**——如果新架构不再区分 planner，这条特殊逻辑不会触发，不影响其它 agent 类型。

### `content`（可选，多次，流式追加自然语言文本）
```json
{ "type": "content", "content": "这段文字会追加到当前 step 里" }
```
前端行为：`currentStep.content += payload.content`。这段文本会被前端拿去做"叙述文字"展示（会自动洗掉 `[TOOL:...]` 这类原始标记，只留自然语言部分），**不需要后端预先清洗**。

### `reasoning`（可选，多次，流式追加思考过程）
```json
{ "type": "reasoning", "content": "..." }
```

### `tool_call_start`（每次工具调用开始时发一次）
```json
{ "type": "tool_call_start", "name": "read_file", "args": "path=\"C:\\...\"" }
```
- `name` 必须是以下几个之一才能触发前端对应的可视化审计（其它值也能显示，只是走"其它工具"兜底展示）：
  - `read_file` → 前端渲染蓝色 "R" 徽章，第三层展示 `args` 里解析出的 `path` + `tool_call_result` 的 `result`
  - `write_file` → 前端渲染强调色 "W" 徽章，第三层需要从 `args` 里解析出 `path` 和 `content` 字段，按 `content.split('\n')` 计算新增行数、渲染绿色 diff——**`args` 里必须包含真实写入的完整 `content`，不能只给摘要**，前端没有能力从别处补全这个内容
  - `execute_command` → 前端渲染灰色 "&gt;" 徽章，第三层展示 `args` 里解析出的 `command` + `tool_call_result`/`tool_call_error` 的输出
- `args` 的格式：`key="value"` 键值对拼接的字符串（例：`path="a.py" content="print(1)"`），前端用正则 `/(\w+)="([\s\S]*?)"/g` 解析，**value 内部的双引号需要转义**，否则解析会在第一个非转义引号处截断。

### `tool_call_result` / `tool_call_error`（工具调用结束时发一次，二选一）
```json
{ "type": "tool_call_result", "name": "read_file", "result": "文件真实内容……" }
{ "type": "tool_call_error", "name": "execute_command", "error": "命令真实报错输出……" }
```
**这两个字段（`result`/`error`）是新增的强需求**——之前的架构里这两个值只是路过没被前端保留，现在前端会把它们挂到对应工具调用记录上，作为第三层审计的直接数据源（比如 `execute_command` 展示的"真实输出"就是这个 `result` 字段，不是前端自己编的）。**如果后端目前只发了 `name` 没发 `result`/`error`，第三层会缺失关键信息（显示为空），需要补上。**

### `step_done`（一个 step 结束时发一次）
```json
{
  "type": "step_done",
  "step_id": "step_xxx",
  "status": "completed",     // completed | failed
  "content": "完整的最终文本（会整体覆盖掉前面流式拼出来的 content，兜底用）",
  "output_tokens": "80",
  "input_tokens": "50",                    // 可选
  "cumulative_input_tokens": "50",         // 本次 /api/workflow/run 请求自己的累计值，不是全局累计
  "cumulative_output_tokens": "30",
  "context_window": "128000",
  "context_window_pct": "0.06"
}
```
**注意 `cumulative_*` 字段的语义**：前端会拿这两个值算出"这次任务一共用了多少 token"，显示在任务组和后台任务清单里。这两个值必须是**从这次 `/api/workflow/run` 请求开始时清零重新累加**的，不能是跨多次请求的全局累计——否则任务清单里显示的 token 数会不对。

### `workflow_done`（整个工作流结束时发一次，必须）
```json
{
  "type": "workflow_done",
  "status": "completed",     // completed | failed | stopped
  "final_output": "给用户看的最终自然语言回答",
  "cumulative_input_tokens": "50",
  "cumulative_output_tokens": "30",
  "context_window": "128000",
  "context_window_pct": "0.06"
}
```
`final_output` 会作为一条**独立的、始终可见的聊天气泡**追加在任务组下面（不需要展开任务组就能看到最终答案）。

### `error`（工作流级别的失败，非某个工具调用失败）
```json
{ "type": "error", "message": "错误信息" }
```

## 一个关键提醒：`final_output` 不要和最后一个 step 的 `content` 完全重复

如果 `workflow_done` 的 `final_output` 和最后一个 step 的 `step_done.content` 是同一段话，前端目前的去重逻辑有个已知问题（正在修，不需要后端处理），但**从后端设计角度**，更干净的做法是：`step_done.content` 放这个 step 自己做了什么的过程性描述，`final_output` 放真正要给用户看的结论——两者即使内容上有重叠也没关系，只是提醒一下这个语义划分，方便前端后续优化展示。

## 验证方式
跑一次真实请求，用浏览器 devtools 或 `curl -N` 看 SSE 原始流，对照上面每个事件的字段名和值的语义过一遍即可，不需要跑通前端界面。
