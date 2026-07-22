package handler

import (
	"backend/internal/ai/core"
	"time"
)

// ========== 结构体定义 ==========

type ChatRequest struct {
	Message         string  `json:"message"`
	SessionID       string  `json:"sessionId"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"top_p,omitempty"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	ReasoningEffort string  `json:"reasoning_effort,omitempty"`
	Image           string  `json:"image,omitempty"`
	Model           string  `json:"model"`
	ApiKey          string  `json:"api_key"`
	DsModel         string  `json:"ds_model"`
}

type ChatResponse struct {
	Reply      string `json:"reply"`
	Emotion    string `json:"emotion,omitempty"`
	TokenUsage int    `json:"token_usage,omitempty"`
	Latency    int64  `json:"latency,omitempty"`
}

type DSMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Timestamp        time.Time       `json:"-"`
	ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Model            string          `json:"model,omitempty"` // 生成该消息所用的模型标识（ds/cloud/local/ds_browser），仅统计用途
	// Blocks 是四态机工作流这一轮的可视化轨迹（说了什么、调了哪些工具、每个工具的
	// 参数和输出），只为「刷新页面后聊天记录里的工具调用和详情还在」而存在。
	// json:"-"：绝不能进发给上游的请求体（模型自己有 tool_calls/tool 消息那条正路），
	// 落盘走 persistedMessage.Blocks，出前端走 /api/sessions/:id 的持久化视图。
	Blocks []FlowBlock `json:"-"`
}

// FlowBlock 与前端 agentflow 消息的 blocks 元素一一对应（见 useAgentWorkflow.js），
// 字段名保持一致，前端拿到就能直接铺回面板，不用做映射。
type FlowBlock struct {
	Type   string `json:"type"`             // intent（模型说的话）| tool（一次工具调用）
	Text   string `json:"text,omitempty"`   // type=intent 时的正文
	Name   string `json:"name,omitempty"`   // type=tool 时的工具名
	Args   string `json:"args,omitempty"`   // 原始 JSON 参数串，前端自己 parse
	Output string `json:"output,omitempty"` // 工具输出（完整版，与 result 事件同口径）
	Status string `json:"status,omitempty"` // ok | error
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

// ========== 辅助函数 ==========

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========== 核心处理函数 ==========

func init() {
	core.RegisterBlogFunc(generateBlogPost)
	core.RegisterSearchFunc(WebSearch)
	startNIMDailyRefresh()
}
