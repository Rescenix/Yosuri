package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ChatRequest 需要在某处定义，假设已在 handler 中定义
// 在 chat_stream.go 中
func (h *ChatHandler) buildSystemPrompt(req ChatRequest, c *gin.Context, modelType string) string {
	var soul string
	if modelType == "ds" {
		soul = SoulTemplateDS
	} else {
		soul = SoulTemplateLocal
	}

	// JWT 检查逻辑不变
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if token != nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				if claims["role"] == "admin" {
					soul = "当前对话对象是朋友，你已经认出他了。" + soul
				}
			}
		}
	}
	return soul
}
func (h *ChatHandler) StreamChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("收到请求，Model=%q\n", req.Model)
	modelType := req.Model
	if modelType == "" {
		modelType = os.Getenv("PRISM_API_TYPE")
	}
	if modelType == "" {
		modelType = "local"
	}

	start := time.Now()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	writeSSE(c, "test", "test", map[string]string{"msg": "SSE connected"})
	c.Writer.Flush()

	systemPrompt := h.buildSystemPrompt(req, c, modelType)

	// 图片逻辑不变
	if req.Image != "" {
		// 省略，保持原样
	}

	finalContent, finalReasoning, tokenUsage, err := h.resolveConversation(
		c, systemPrompt, req.Message, req.SessionID,
		req.Temperature, req.TopP, req.MaxTokens,
		req.ReasoningEffort, 0, modelType,
		req.ApiKey, req.DsModel, // 新增
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

	// 只在非流式引擎时补充逐字发送（DS和Cloud已在内部实时发送）
	if modelType != "ds" && modelType != "cloud" && finalContent != "" {
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
		"model":       modelType,
	}
	writeSSE(c, "done", "done", finalData)
	c.Writer.Flush()

	h.sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
	h.sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: finalContent})
}

// 引擎调度
func (h *ChatHandler) resolveConversation(
	c *gin.Context,
	systemPrompt string,
	userMessage string,
	sessionID string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
	depth int,
	modelType string,
	apiKey string, // 新增
	dsModel string, // 新增
) (string, string, int, error) {
	switch modelType {
	case "ds":
		return h.resolveDSConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort, apiKey, dsModel)
	case "cloud":
		return h.resolveCloudConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort)
	default:
		return h.resolveLocalConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort, depth)
	}
}

func writeSSE(c *gin.Context, event string, eventType string, data map[string]string) {
	data["type"] = eventType
	jsonData, _ := json.Marshal(data)
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))))
}

func sanitizeMessages(msgs []DSMessage) []DSMessage {
	var cleaned []DSMessage
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Content == "" {
			continue
		}
		if msg.Role == "tool" && msg.Content == "" {
			continue
		}
		cleaned = append(cleaned, msg)
	}
	return cleaned
}
