package models

import (
	"backend/internal/ai/core"
	"time"
)

// ChatRequest 聊天请求
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

// ChatResponse 聊天响应（非流式使用，流式已废弃）
type ChatResponse struct {
	Reply      string `json:"reply"`
	Emotion    string `json:"emotion,omitempty"`
	TokenUsage int    `json:"token_usage,omitempty"`
	Latency    int64  `json:"latency,omitempty"`
}

// DSMessage 对话消息
type DSMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Timestamp        time.Time       `json:"-"`
	ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
}

// DSReq DeepSeek 请求体（通用）
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

// DSResp DeepSeek 响应体（通用）
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
