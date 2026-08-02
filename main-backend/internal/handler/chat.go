package handler

import (
	"backend/internal/ai/core"
	"time"
)

// ========== 结构体定义 ==========

type DSMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Timestamp        time.Time       `json:"-"`
	ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Model            string          `json:"model,omitempty"`
	// Status 任务生命周期状态，只对 role=="user" 有意义：
	//   completed —— 这条任务已经跑完并结题
	//   failed —— 工作流以错误结束，保留已执行步骤供后续对话参考
	//   interrupted —— 用户停止、连接中断或进程退出前未完成
	//   （空）     —— 历史遗留数据；见 taskDone 的说明，按已完成处理
	// 存在的意义：历史里的旧任务指令如果不带状态，模型会把它们读成"还没做的待办"，
	// 收尾时注意力一发散就回头去执行上一个任务（实际发生过）。
	Status string `json:"status,omitempty"` // 生成该消息所用的模型标识（ds/cloud/local/ds_browser），仅统计用途
	// WorkflowID 把同一工作流的 user/assistant 历史消息绑成一组。失败后续跑成功时
	// 用它原位更新状态，而不是再追加一份重复任务。只用于本地持久化，不发给模型。
	WorkflowID string `json:"-"`
	// Blocks 是四态机工作流这一轮的可视化轨迹（说了什么、调了哪些工具、每个工具的
	// 参数和输出），只为「刷新页面后聊天记录里的工具调用和详情还在」而存在。
	// json:"-"：绝不能进发给上游的请求体（模型自己有 tool_calls/tool 消息那条正路），
	// 落盘走 persistedMessage.Blocks，出前端走 /api/sessions/:id 的持久化视图。
	Blocks []FlowBlock `json:"-"`
}

// FlowBlock 与前端 agentflow 消息的 blocks 元素一一对应（见 useAgentWorkflow.js），
// 字段名保持一致，前端拿到就能直接铺回面板，不用做映射。
type FlowBlock struct {
	Type   string `json:"type"`             // intent（模型说的话）| tool（一次工具调用）| question（agent 向用户提问）
	Text   string `json:"text,omitempty"`   // type=intent 时的正文
	Name   string `json:"name,omitempty"`   // type=tool 时的工具名
	Args   string `json:"args,omitempty"`   // 原始 JSON 参数串，前端自己 parse
	Output string `json:"output,omitempty"` // 工具输出（完整版，与 result 事件同口径）
	Status string `json:"status,omitempty"` // ok | error
	// question 块专用字段（ask_user 工具产生）
	Question string          `json:"question,omitempty"` // 问用户的话
	Options  []askUserOption `json:"options,omitempty"`  // 候选选项
	Answer   string          `json:"answer,omitempty"`   // 用户回答后回填
	Multi    bool            `json:"multi,omitempty"`    // 是否多选
}

type DSReq struct {
	Model           string                `json:"model"`
	Messages        []DSMessage           `json:"messages"`
	Temperature     float64               `json:"temperature,omitempty"`
	TopP            float64               `json:"top_p,omitempty"`
	MaxTokens       int                   `json:"max_tokens,omitempty"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
	Tools           []core.ToolDefinition `json:"tools,omitempty"`
	Stream          bool                  `json:"stream,omitempty"`
}

type DSResp struct {
	Choices []struct {
		Message struct {
			Role             string          `json:"role"`
			Content          string          `json:"content,omitempty"`
			ReasoningContent string          `json:"reasoning_content,omitempty"`
			ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// ========== 核心处理函数 ==========

func init() {
	// 每日提供方模型列表重探（存在性：下架/恢复）+ 30 分钟免费池探活（可用性信号）
	startProviderDailyRefresh()
	startFreeProbeLoop()
}
