// handler/chat_stream.go
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
	"github.com/golang-jwt/jwt/v5"
)

// ========== 结构体定义 ==========

type thinkingConfig struct {
	Type string `json:"type"`
}

type streamDSReq struct {
	Model           string                `json:"model"`
	Messages        []DSMessage           `json:"messages"`
	Temperature     float64               `json:"temperature,omitempty"`
	TopP            float64               `json:"top_p,omitempty"`
	MaxTokens       int                   `json:"max_tokens,omitempty"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
	Tools           []core.ToolDefinition `json:"tools,omitempty"`
	Stream          bool                  `json:"stream,omitempty"`
	Thinking        *thinkingConfig       `json:"thinking,omitempty"`
}

// ========== 系统提示词构建 ==========

func buildSystemPrompt(req ChatRequest, c *gin.Context, memoryStore *MemoryStore) string {
	prompt := SystemPrompt

	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if token != nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				if claims["role"] == "admin" {
					prompt = "当前对话对象是朋友，你已经认出他了。" + prompt
				}
			}
		}
	}

	return prompt
}

// ========== 流式处理核心 ==========

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
	history := h.sessionStore.Get(req.SessionID)

	if req.Image != "" {
		imageData := req.Image
		if idx := strings.Index(imageData, "base64,"); idx != -1 {
			imageData = imageData[idx+7:]
		}
		description, err := AnalyzeImage(imageData, req.Message)
		if description != "" {
			fmt.Println("✅ 图片分析成功，将图片内容作为上下文注入")
			imageMsg := DSMessage{
				Role:    "user",
				Content: fmt.Sprintf("用户发送了一张图片，图片内容如下：\n%s\n请根据图片内容进行回复，注意用户可能没有提供其他文字描述。", description),
			}
			history = append(history, imageMsg)
		} else if err != nil {
			fmt.Printf("❌ 图片分析失败: %v\n", err)
		}
	}

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

func (h *ChatHandler) resolveConversation(
	c *gin.Context,
	systemPrompt string,
	messages []DSMessage,
	userMessage string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
) (string, string, int, error) {

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", "", 0, fmt.Errorf("missing API key")
	}

	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
	}

	client := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: DeepSeekTransport,
	}
	totalUsage := 0
	reasoningAccum := strings.Builder{}

	const maxTokenLimit = 8192
	currentMaxTokens := maxTokens
	if currentMaxTokens <= 0 {
		currentMaxTokens = 2000
	}

	for {
		messages = sanitizeMessages(messages)

		reqBody := streamDSReq{
			Model:       model,
			Messages:    messages,
			Temperature: temperature,
			TopP:        topP,
			MaxTokens:   currentMaxTokens,
			Tools:       ChatTools,
		}

		if reasoningEffort != "" {
			reqBody.ReasoningEffort = reasoningEffort
			reqBody.Thinking = &thinkingConfig{Type: "enabled"}
		} else {
			reqBody.Thinking = &thinkingConfig{Type: "disabled"}
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

		if assistantMsg.ReasoningContent != "" {
			reasoningAccum.WriteString(assistantMsg.ReasoningContent)
			writeSSE(c, "reasoning", "reasoning", map[string]string{"content": assistantMsg.ReasoningContent})
			c.Writer.Flush()
		}

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

		if len(assistantMsg.ToolCalls) == 0 {
			return assistantMsg.Content, reasoningAccum.String(), totalUsage, nil
		}

		messages = append(messages, assistantMsg)

		// 物理熔断：如果任何一个工具失败，立即终止本轮调用链
		toolFailed := false
		var failedReason string

		for _, tc := range assistantMsg.ToolCalls {
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Function.Name,
				"args": tc.Function.Arguments,
			})
			c.Writer.Flush()

			result, err := core.ExecuteToolCall(tc)
			var toolContent string
			if err != nil {
				// Go error：参数解析失败、命令执行异常等，立即终止
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Function.Name,
					"error": fmt.Sprintf("工具执行异常: %v", err),
				})
				c.Writer.Flush()
				return fmt.Sprintf("工具调用异常: %v", err), reasoningAccum.String(), totalUsage, nil
			}

			if result.Failed {
				// 逻辑失败：路径越界、命令失败等，立即终止
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Function.Name,
					"error": result.Content,
				})
				c.Writer.Flush()
				return result.Content, reasoningAccum.String(), totalUsage, nil
			}

			// 成功，正常追加结果
			writeSSE(c, "tool_call", "tool_call_result", map[string]string{
				"name":   tc.Function.Name,
				"result": result.Content,
			})
			toolContent = result.Content

			messages = append(messages, DSMessage{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
			})
			c.Writer.Flush()

			if toolFailed {
				// 物理熔断：工具失败后立即返回，不再执行剩余工具
				return fmt.Sprintf("工具调用失败: %s", failedReason), reasoningAccum.String(), totalUsage, nil
			}
		}
	}
}

// ========== 辅助函数 ==========

func sanitizeMessages(msgs []DSMessage) []DSMessage {
	var cleaned []DSMessage
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Content == "" && len(msg.ToolCalls) == 0 {
			continue
		}
		if msg.Role == "tool" && msg.Content == "" {
			continue
		}
		cleaned = append(cleaned, msg)
	}
	return cleaned
}

func writeSSE(c *gin.Context, event string, eventType string, data map[string]string) {
	data["type"] = eventType
	jsonData, _ := json.Marshal(data)
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))))
}
