package agent

import (
	"fmt"
	"time"
)

// ModelConfig 模型配置
type ModelConfig struct {
	Provider string
	Model    string
	Temp     float64
	TopP     float64
}

// StepResult 单步执行结果（通用，不再绑定特定角色）
type StepResult struct {
	StepID    string         `json:"step_id"`
	Agent     string         `json:"agent"`
	Content   string         `json:"content"`
	Reasoning string         `json:"reasoning,omitempty"`
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"`
	Duration  time.Duration  `json:"duration_ms"`
	Error     string         `json:"error,omitempty"`
	Status    string         `json:"status"`

	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	Name   string `json:"name"`
	Args   string `json:"args,omitempty"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// WorkflowStatus 工作流状态
type WorkflowStatus string

const (
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowStopped   WorkflowStatus = "stopped"
)

// WorkflowRequest 启动工作流的请求
type WorkflowRequest struct {
	Task string `json:"task" binding:"required"`
}

// ========== 工具调用格式说明（Agent 通用） ==========
const ToolCallFormatInstruction = `
━━━ 工具调用与对话交互格式（必读）━━━
你的任务是调用工具完成任务。
1. 需要调用工具时，直接输出纯 JSON 格式的工具调用，整段回复的末尾不能有任何多余的解释文字。
2. 只有当你要开启一个新的、跟上一步明显不同的工作阶段时（比如从"排查问题"转到"开始改代码"，或者从"改代码"转到"验证结果"），才在 JSON 前面先用一句简短自然语言说明这个阶段要做什么，然后换行输出 JSON。
3. 连续做同类型/延续性的动作（比如连续读取好几个文件、连续修改同一批文件）不需要每次都重复说明意图，直接输出 JSON 就好——每次都强行造一句话是纯粹的语言浪费。

✅ 正确输出（延续同一阶段，直接工具调用，不需要多余的话）：
{"tool":"read_file","args":{"path":"main.py"}}

✅ 正确输出（开启新阶段时先说一句）：
现在开始修改这部分逻辑。
{"tool":"edit_file","args":{"path":"main.py","old_string":"旧文本","new_string":"新文本"}}

❌ 错误输出（意图陈述和 JSON 之间夹杂了额外内容）：
让我看看。{"tool":"read_file","args":{"path":"main.py"}}

❌ 错误输出（使用任何代码块包裹，或直接使用 Python/Shell 函数调用）：
不要输出类似 read_file("main.py") 的 Python 函数调用。
不要使用任何代码块、反引号、JSON 包裹标记，直接输出纯文本的 JSON 对象。
`

// ========== DS 专用：Token 经济 + 增量检索协议 ==========
const SoulTemplateCodeProtocol = `
# 策略指南（仅作参考，不影响输出格式）
- Token 是成本，尽量用最少 token 完成任务。
- 读文件优先用 read_file mode="outline" 看结构，再用 start_line/end_line 取正文片段。
- 改代码用 edit_file 精确替换，old_string 要唯一。
- 先搜索再修改，避免重复劳动。
`

// MainAgent 主Agent定义
type MainAgent struct {
	SystemPrompt string
	Temp         float64
	TopP         float64
}

// MainAgentConfig 返回主Agent的配置
func MainAgentConfig() MainAgent {
	return MainAgent{
		SystemPrompt: `你是Aurora的主工程师Agent。你的核心工作方式是：
通过工具调用完成任务，而不是在回复里写 bash 命令。

` + ToolCallFormatInstruction + `

━━━ 可用工具 ━━━
- read_file —— 读取文件
- write_file —— 创建或覆盖文件
- edit_file —— 精确编辑（old_string → new_string）
- execute_command —— 执行 shell 命令
- search_codebase —— 语义搜索代码
- codegraph_query —— 查询代码结构（callers/callees/impact）
- search_memory —— 检索长期记忆

` + SoulTemplateCodeProtocol + `

你的工作目录是 C:\Pro2026\re0。`,
		Temp: 0.2,
		TopP: 0.85,
	}
}

// ListWorkflows 列出所有可用工作流。目前工作流已由主 Agent 动态管理，返回空列表。
func ListWorkflows() []map[string]string {
	return make([]map[string]string, 0)
}

// NewStepID 生成步骤 ID
func NewStepID() string {
	return fmt.Sprintf("step_%d", time.Now().UnixNano())
}

// NewWorkflowID 生成工作流 ID
func NewWorkflowID() string {
	return fmt.Sprintf("wf_%d", time.Now().UnixNano())
}
