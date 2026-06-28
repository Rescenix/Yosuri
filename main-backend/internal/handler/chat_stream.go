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

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ========== 系统提示词构建 ==========
func (h *ChatHandler) buildSystemPrompt(req ChatRequest, c *gin.Context, modelType string) string {
	var soul string
	if modelType == "ds" {
		soul = SoulTemplateDS
	} else {
		soul = SoulTemplateLocal
	}

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

// ========== 流式处理核心 ==========
func (h *ChatHandler) StreamChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("收到请求，Model=%q, Message=%q\n", req.Model, req.Message)
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

	if req.Image != "" {
		// 图片处理逻辑保持不变
	}

	// ★ 记忆回忆：注入 Base 层记忆，不直接返回
	fmt.Println("🔍 开始记忆回忆判断...")
	if h.isMemoryRecallByLLM(req.Message) {
		fmt.Println("🧠 判定为记忆回忆，注入 Base 层记忆")
		if base := h.fetchBaseMemories(); base != "" {
			systemPrompt += "\n以下是关于当前用户的长期记忆，请用自然的口吻在对话中提及：\n" + base
			fmt.Printf("📝 已注入 Base 记忆片段: %s...\n", base[:min(len(base), 80)])
		} else {
			fmt.Println("⚠️ Base 层无记忆，使用默认系统提示")
		}
		// 继续走引擎，不 return
	} else {
		fmt.Println("💬 非记忆回忆，正常走引擎")
	}

	// 引擎调用（所有情况最终都走这里）
	finalContent, finalReasoning, tokenUsage, err := h.resolveConversation(
		c, systemPrompt, req.Message, req.SessionID,
		req.Temperature, req.TopP, req.MaxTokens,
		req.ReasoningEffort, 0, modelType,
		req.ApiKey, req.DsModel,
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

	// 保存对话
	h.sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
	if finalContent != "" {
		h.sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: finalContent})
	}

	// 定期压缩
	go h.maybeCompressSession(req.SessionID, modelType)
}

// ========== 记忆回忆相关 ==========

// isMemoryRecallByLLM 判断用户是否在询问关于自己的记忆
func (h *ChatHandler) isMemoryRecallByLLM(msg string) bool {
	h.localModelMu.Lock()
	blocked := h.localModelBlocked
	h.localModelMu.Unlock()

	// 如果已经熔断，降级关键词匹配
	if blocked {
		return fallbackKeywordCheck(msg)
	}

	fmt.Printf("🤖 调用本地模型判断记忆意图: %q\n", msg)
	prompt := fmt.Sprintf(`判断用户这句话是否在询问关于他自己的记忆、偏好或过往对话。仅回复 yes 或 no。

示例：
"你还记得我吗" -> yes
"我喜欢什么" -> yes
"我之前说过什么" -> yes
"你了解我什么" -> yes
"写个排序算法" -> no
"今天天气怎么样" -> no

用户消息：%s
回复：`, msg)

	reqBody := map[string]interface{}{
		"model":  "qwen2.5-coder:7b",
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0,
			"num_predict": 5,
		},
	}
	body, _ := json.Marshal(reqBody)

	// 第一次尝试，超时 5 秒（更快失败）
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))

	if err != nil {
		fmt.Printf("⚠️ 本地模型第一次请求失败: %v，3秒后重试...\n", err)
		time.Sleep(3 * time.Second)

		// 重试一次
		resp2, err2 := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
		if err2 != nil {
			// 两次都失败
			h.localModelMu.Lock()
			h.localModelFails++
			fails := h.localModelFails
			if fails >= 3 {
				h.localModelBlocked = true
				fmt.Println("🔥 本地模型熔断，后续 5 分钟内降级为关键词匹配")
				go func() {
					time.Sleep(5 * time.Minute)
					h.localModelMu.Lock()
					h.localModelBlocked = false
					h.localModelFails = 0
					h.localModelMu.Unlock()
					fmt.Println("🌱 熔断自动解除，恢复本地模型判断")
				}()
			}
			h.localModelMu.Unlock()
			fmt.Printf("❌ 本地模型重试仍然失败 (第%d次)，降级关键词\n", fails)
			return fallbackKeywordCheck(msg)
		}
		resp = resp2
	}
	defer resp.Body.Close()

	// 成功，重置失败计数
	h.localModelMu.Lock()
	h.localModelFails = 0
	h.localModelMu.Unlock()

	respBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		Response string `json:"response"`
	}
	json.Unmarshal(respBytes, &result)

	isRecall := strings.Contains(strings.ToLower(result.Response), "yes")
	fmt.Printf("📊 模型返回: %q, 判断结果: %v\n", result.Response, isRecall)
	return isRecall
}

// fallbackKeywordCheck 降级关键词匹配（模型不可用时使用）
func fallbackKeywordCheck(msg string) bool {
	lower := strings.ToLower(msg)
	keywords := []string{"记得我", "我的偏好", "了解我", "我是什么", "我告诉过你", "你还记得", "你记得我"}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			fmt.Printf("📊 降级关键词命中: %q -> true\n", k)
			return true
		}
	}
	fmt.Println("📊 降级关键词未命中 -> false")
	return false
}

// fetchBaseMemories 从 PrismD 拉取所有记忆，返回画像文本
func (h *ChatHandler) fetchBaseMemories() string {
	fmt.Println("📡 请求 PrismD STATS FULL...")
	resp, err := http.Post("http://localhost:5666", "text/plain", strings.NewReader("STATS FULL"))
	if err != nil {
		fmt.Printf("❌ PrismD 请求失败: %v\n", err)
		return ""
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	respText := string(respBytes)
	fmt.Printf("📦 PrismD 返回 %d 字节\n", len(respText))

	if strings.HasPrefix(respText, "ERROR") {
		fmt.Println("⚠️ PrismD 返回错误")
		return ""
	}

	lines := strings.Split(respText, "\n")
	var memories []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Content: ") {
			content := strings.TrimPrefix(line, "Content: ")
			content = strings.TrimSpace(content)
			if content != "" {
				memories = append(memories, "• "+content)
			}
		}
	}

	fmt.Printf("🧠 提取到 %d 条记忆\n", len(memories))
	return strings.Join(memories, "\n")
}

// ---------- 记忆过滤辅助 ----------
func isPureCodeBlock(content string) bool {
	return strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") && len(strings.Fields(content)) < 10
}

func isGreetingOrNoise(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len([]rune(trimmed)) < 10 {
		greetings := []string{"你好", "在吗", "测试", "hello", "hi", "喂", "hey", "晚上好", "早上好", "下午好"}
		lower := strings.ToLower(trimmed)
		for _, g := range greetings {
			if strings.Contains(lower, g) {
				return true
			}
		}
	}
	return false
}

func filterMemoryCandidates(messages []DSMessage) []DSMessage {
	var clean []DSMessage
	for _, msg := range messages {
		if msg.Role == "tool" {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if strings.Contains(msg.Content, "调用失败") ||
			strings.Contains(msg.Content, "工具执行") ||
			strings.Contains(msg.Content, "流式连接") ||
			strings.Contains(msg.Content, "上下文长度") ||
			strings.Contains(msg.Content, "压测") ||
			strings.Contains(msg.Content, "ping") {
			continue
		}
		if isPureCodeBlock(msg.Content) {
			continue
		}
		if isGreetingOrNoise(msg.Content) {
			continue
		}
		clean = append(clean, msg)
	}
	return clean
}

// ---------- 会话压缩 ----------
func (h *ChatHandler) maybeCompressSession(sessionID string, modelType string) {
	if modelType == "local" || modelType == "" {
		return
	}

	const compressRounds = 10

	messages := h.sessionStore.Get(sessionID)
	rounds := len(messages) / 2
	if rounds == 0 || rounds%compressRounds != 0 {
		return
	}

	candidates := filterMemoryCandidates(messages)
	if len(candidates) == 0 {
		fmt.Printf("🧹 无有效记忆候选 (session=%s)\n", sessionID)
		return
	}

	var turns []string
	for _, msg := range candidates {
		turns = append(turns, fmt.Sprintf("[%s]: %s", msg.Role, msg.Content))
	}
	compressedText := strings.Join(turns, "\n---\n")

	fmt.Printf("🧠 触发记忆压缩 (session=%s, rounds=%d, candidates=%d)\n", sessionID, rounds, len(candidates))
	go func() {
		resp, err := http.Post("http://localhost:5666", "text/plain", strings.NewReader("COMPILE "+compressedText))
		if err != nil {
			fmt.Printf("⚠️ 记忆压缩请求失败: %v\n", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if strings.HasPrefix(bodyStr, "ERROR") {
			fmt.Printf("⚠️ 记忆压缩失败: %s\n", bodyStr)
		} else {
			fmt.Printf("✅ 长期记忆已存入 PrismD (session=%s): %s\n", sessionID, bodyStr)
		}
	}()
}

// ========== 引擎调度 ==========
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
	apiKey string,
	dsModel string,
) (string, string, int, error) {
	switch modelType {
	case "ds_browser":
		return h.resolveDSBrowserConversation(c, systemPrompt, userMessage, sessionID)
	case "ds":
		return h.resolveDSConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort, apiKey, dsModel)
	case "cloud":
		return h.resolveCloudConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort)
	default:
		return h.resolveLocalConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort, depth)
	}
}

// ========== SSE 写入 ==========
func rawWriteSSE(c *gin.Context, event string, eventType string, data map[string]string) {
	data["type"] = eventType
	jsonData, _ := json.Marshal(data)
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))))
}

func writeSSE(c *gin.Context, event string, eventType string, data map[string]string) {
	rawWriteSSE(c, event, eventType, data)
	if eventType == "content" {
		time.Sleep(60 * time.Millisecond)
	}
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

// warmUpLocalModel 在后台启动后立即调用，避免首次冷启动超时
func (h *ChatHandler) warmUpLocalModel() {
	go func() {
		fmt.Println("🔥 本地模型看护循环已启动（每30秒检测一次）")
		for {
			reqBody := map[string]interface{}{
				"model":  "qwen2.5-coder:7b",
				"prompt": ".",
				"stream": false,
				"options": map[string]interface{}{
					"temperature": 0,
					"num_predict": 1,
				},
			}
			body, _ := json.Marshal(reqBody)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
			if err == nil {
				resp.Body.Close()
				// 模型正常，30秒后再检查
				time.Sleep(30 * time.Second)
				continue
			}
			// 模型挂了，持续重试直到恢复
			fmt.Printf("⚠️ 本地模型失联，正在重新预热... (%v)\n", err)
			retryClient := &http.Client{Timeout: 60 * time.Second}
			for {
				resp2, err2 := retryClient.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
				if err2 == nil {
					resp2.Body.Close()
					fmt.Println("✅ 本地模型已恢复，继续看护")
					break
				}
				time.Sleep(5 * time.Second)
			}
			time.Sleep(30 * time.Second)
		}
	}()
}
