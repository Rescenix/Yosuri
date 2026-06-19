// handler/chat_stream_mobile.go
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamChatMobile 手机端专用聊天处理（保留完整工具调用、推理链、物理熔断）
func (h *ChatHandler) StreamChatMobile(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	writeSSE(c, "test", "test", map[string]string{"msg": "SSE connected"})
	c.Writer.Flush()

	systemPrompt := SystemPrompt // 手机端专用Prompt已在main.go中注入
	history := h.sessionStore.Get(req.SessionID)
	userMsg := DSMessage{Role: "user", Content: req.Message}
	messages := buildContextWindow(systemPrompt, history, userMsg, h.memoryStore)

	finalContent, finalReasoning, tokenUsage, err := h.resolveConversation(
		c,
		systemPrompt,
		messages,
		req.Message,
		req.Temperature,
		req.TopP,
		req.MaxTokens,
		req.ReasoningEffort,
	)

	if err != nil {
		writeSSE(c, "error", "error", map[string]string{"message": err.Error()})
		c.Writer.Flush()
		return
	}

	latency := time.Since(start).Milliseconds()

	if finalReasoning != "" {
		for _, ch := range finalReasoning {
			writeSSE(c, "reasoning", "reasoning", map[string]string{"content": string(ch)})
			c.Writer.Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}

	if finalContent != "" {
		for _, ch := range finalContent {
			writeSSE(c, "content", "content", map[string]string{"content": string(ch)})
			c.Writer.Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}

	finalData := map[string]string{
		"content":     finalContent,
		"reasoning":   finalReasoning,
		"token_usage": fmt.Sprintf("%d", tokenUsage),
		"latency":     fmt.Sprintf("%d", latency),
	}
	writeSSE(c, "done", "done", finalData)
	c.Writer.Flush()

	h.sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
	h.sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: finalContent})
}
