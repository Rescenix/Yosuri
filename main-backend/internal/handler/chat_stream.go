package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

func (h *ChatHandler) StreamChat(c *gin.Context) {
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

	systemPrompt := buildSystemPrompt(req, c, h.memoryStore)
	// 获取当前会话全部历史（不截断）
	history := h.sessionStore.Get(req.SessionID)

	finalContent, finalReasoning, tokenUsage, err := h.resolveConversation(
		c,
		systemPrompt,
		history,
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

	// 逐字符推送正文（打字机效果）
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

func (h *ChatHandler) resolveConversation(
	c *gin.Context,
	systemPrompt string,
	history []DSMessage,
	userMessage string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
) (string, string, int, error) {

	// 清洗历史：移除未完成工具调用的助手消息
	var cleanHistory []DSMessage
	for _, msg := range history {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			hasResult := false
			for _, next := range history {
				if next.Role == "tool" && next.ToolCallID != "" {
					for _, tc := range msg.ToolCalls {
						if tc.ID == next.ToolCallID {
							hasResult = true
							break
						}
					}
				}
			}
			if !hasResult {
				continue // 丢弃未完成的工具调用
			}
		}
		cleanHistory = append(cleanHistory, msg)
	}

	// 构建完整上下文：系统提示 + 历史 + 当前用户消息
	messages := []DSMessage{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, cleanHistory...)
	messages = append(messages, DSMessage{Role: "user", Content: userMessage})

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", "", 0, fmt.Errorf("missing API key")
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	totalUsage := 0
	reasoningAccum := strings.Builder{}

	const maxTokenLimit = 8192
	currentMaxTokens := maxTokens
	if currentMaxTokens <= 0 {
		currentMaxTokens = 2000
	}

	for {
		// 清理无效的 assistant 空消息
		var validMessages []DSMessage
		for _, msg := range messages {
			if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
				continue
			}
			validMessages = append(validMessages, msg)
		}
		messages = validMessages

		reqBody := DSReq{
			Model:           "deepseek-chat",
			Messages:        messages,
			Temperature:     temperature,
			TopP:            topP,
			MaxTokens:       currentMaxTokens,
			ReasoningEffort: reasoningEffort,
			Tools:           core.ChatTools,
		}

		body, _ := json.Marshal(reqBody)
		httpReq, _ := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return "", "", 0, err
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", "", 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var dsResp DSResp
		if err := json.Unmarshal(bodyBytes, &dsResp); err != nil {
			return "", "", 0, err
		}
		if len(dsResp.Choices) == 0 {
			return "", "", 0, fmt.Errorf("no choices in response")
		}

		choice := dsResp.Choices[0]
		totalUsage += dsResp.Usage.TotalTokens

		assistantMsg := DSMessage{
			Role:             choice.Message.Role,
			Content:          strings.TrimSpace(choice.Message.Content),
			ReasoningContent: choice.Message.ReasoningContent,
			ToolCalls:        choice.Message.ToolCalls,
		}

		// 推送思考内容
		if assistantMsg.ReasoningContent != "" {
			reasoningAccum.WriteString(assistantMsg.ReasoningContent)
			writeSSE(c, "reasoning", "reasoning", map[string]string{"content": assistantMsg.ReasoningContent})
			c.Writer.Flush()
		}

		// 处理空消息
		if assistantMsg.Content == "" && len(assistantMsg.ToolCalls) == 0 {
			if currentMaxTokens < maxTokenLimit {
				currentMaxTokens *= 2
				if currentMaxTokens > maxTokenLimit {
					currentMaxTokens = maxTokenLimit
				}
				continue
			}
			return "", reasoningAccum.String(), totalUsage,
				fmt.Errorf("思考过程过长，请尝试关闭深度思考或缩小问题范围")
		}

		// 最终回复
		if len(assistantMsg.ToolCalls) == 0 {
			return assistantMsg.Content, reasoningAccum.String(), totalUsage, nil
		}

		// 工具调用：先追加助手消息，再追加工具结果
		messages = append(messages, assistantMsg)

		for _, tc := range assistantMsg.ToolCalls {
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Function.Name,
				"args": tc.Function.Arguments,
			})
			c.Writer.Flush()

			result, err := core.ExecuteToolCall(tc)
			if err != nil {
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Function.Name,
					"error": err.Error(),
				})
				c.Writer.Flush()
				messages = append(messages, DSMessage{
					Role:       "tool",
					Content:    fmt.Sprintf("工具执行失败: %v", err),
					ToolCallID: tc.ID,
				})
			} else {
				writeSSE(c, "tool_call", "tool_call_result", map[string]string{
					"name":   tc.Function.Name,
					"result": result.Content,
				})
				c.Writer.Flush()
				messages = append(messages, DSMessage{
					Role:       "tool",
					Content:    result.Content,
					ToolCallID: result.ToolCallID,
				})
			}
		}
		// 继续循环，模型会基于工具结果生成新回复
	}
}

func writeSSE(c *gin.Context, event string, eventType string, data map[string]string) {
	data["type"] = eventType
	jsonData, _ := json.Marshal(data)
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))))
}
