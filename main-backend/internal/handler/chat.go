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
}
