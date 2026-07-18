// handler/chat_stream.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/ai/core"
	"backend/internal/swiftnet"

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
	// 工作目录可由 /api/workdir 在运行时切换，不能写进常量提示词。
	soul += fmt.Sprintf("\n# 工作环境\n你的工作目录是 %s。\n", core.GetProjectRoot())

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
	// 用户在设置面板填的称呼/职业/自定义指令，无条件注入（见 settings_handlers.go）
	soul += userInstructionsPrompt()
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
		modelType = "ds"
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

	// 保存对话（ds_browser 引擎在 resolveDSBrowserConversation 内部已自行保存，这里跳过避免重复计入统计）
	if modelType != "ds_browser" {
		h.sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
		if finalContent != "" {
			h.sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: finalContent, Model: modelType})
		}
	}

	// 定期压缩
	go h.maybeCompressSession(req.SessionID, modelType)
}

// ========== 记忆回忆相关 ==========

// isMemoryRecallByLLM 判断用户是否在询问关于自己的记忆。
// 曾经先调本地 Ollama 模型判断、失败才降级关键词匹配（+熔断器）——本地模型
// 依赖已移除，直接用关键词匹配：这本来就只是决定要不要注入 Base 层记忆的
// 轻量判断，不值得为它保留一整套本地模型调用+重试+熔断的重量级设施。
func (h *ChatHandler) isMemoryRecallByLLM(msg string) bool {
	return fallbackKeywordCheck(msg)
}

// fallbackKeywordCheck 关键词匹配判断是否在询问关于自己的记忆
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

// fetchBaseMemories 返回 SwiftNet 的无条件注入区（pinned 身份 + handoff 工作态 + inbox）。
// 旧实现从 PrismD STATS FULL 抓 UserBase 簇按 Energy 排序——SwiftNet 的分区判断是：
// 身份记忆本就不该走召回/排序，pinned 区无条件全量注入（≤150 tok 预算由写侧纪律保证）。
func (h *ChatHandler) fetchBaseMemories() string {
	base := swiftnet.Default().UnconditionalInject()
	if base != "" {
		fmt.Printf("🧠 SwiftNet 无条件注入 %d 字节（pinned/handoff/inbox）\n", len(base))
	}
	return base
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

func (h *ChatHandler) maybeCompressSession(sessionID string, modelType string) {
	// 1. 只在云端模型下触发
	if modelType == "local" || modelType == "" {
		return
	}

	const compressThreshold = 6 // 按你说的，降到5-6轮

	messages := h.sessionStore.Get(sessionID)
	totalMessages := len(messages)
	totalRounds := totalMessages / 2
	if totalRounds == 0 {
		return
	}

	// 2. 获取上次压缩的游标
	lastIndex := h.sessionStore.GetCompressIndex(sessionID)

	// 3. 检查是否有足够的新消息需要压缩
	newMessages := messages[lastIndex:]
	newRounds := len(newMessages) / 2
	if newRounds < compressThreshold {
		return // 新消息不够，不用急
	}

	fmt.Printf("🧠 触发增量记忆压缩 (session=%s, 新增轮次=%d, 总轮次=%d)\n", sessionID, newRounds, totalRounds)

	// 4. 只压缩新增的消息（增量）
	candidates := filterMemoryCandidates(newMessages)
	if len(candidates) == 0 {
		// 即使没有有效候选，也要把游标更新，避免每次都要重新扫描一遍噪声
		h.sessionStore.SetCompressIndex(sessionID, totalMessages)
		fmt.Printf("🧹 无有效记忆候选 (session=%s)，游标已更新\n", sessionID)
		return
	}

	var turns []string
	for _, msg := range candidates {
		turns = append(turns, fmt.Sprintf("[%s]: %s", msg.Role, msg.Content))
	}
	compressedText := strings.Join(turns, "\n---\n")

	// 5. 异步压缩后写 SwiftNet（LLM 判定价值 + 提取事实与同义关键词；失败不更新游标，下轮重试）
	go func() {
		fact, cluster, keywords, keep, err := compressToFact(compressedText)
		if err != nil {
			fmt.Printf("⚠️ 记忆压缩失败: %v\n", err)
			return
		}
		if !keep {
			// 无长期价值也算处理完成，推进游标避免反复扫噪声
			h.sessionStore.SetCompressIndex(sessionID, totalMessages)
			fmt.Printf("🧹 压缩判定无长期价值 (session=%s)，游标已更新\n", sessionID)
			return
		}
		res := swiftnet.Default().MemAppend(fact, cluster, keywords)
		if res.Err != "" {
			fmt.Printf("⚠️ 记忆写入失败: %s\n", res.Err)
			return
		}
		h.sessionStore.SetCompressIndex(sessionID, totalMessages)
		if res.MergedID != "" {
			fmt.Printf("✅ 长期记忆已合并进 SwiftNet %s (session=%s)\n", res.MergedID, sessionID)
		} else {
			fmt.Printf("✅ 长期记忆已存入 SwiftNet %s (session=%s)\n", res.ID, sessionID)
		}
	}()
}

// compressToFact 用 LLM 把若干对话轮次压缩成一条值得长期记住的事实。
// 返回 keep=false 表示模型判定无长期价值（闲聊/一次性操作）。
func compressToFact(turnsText string) (fact, cluster, keywords string, keep bool, err error) {
	prompt := fmt.Sprintf(`以下是若干轮对话。判断其中是否有值得跨会话长期记住的信息（用户的稳定事实/偏好/项目决策）。
闲聊、一次性操作、过程性内容不值得记。

%s

只输出一个 JSON 对象，不要解释不要代码块：
{"keep":true或false,"cluster":"UserBase或CodeWork或Decisions","keywords":"同义改述关键词，斜杠分隔（如 风险偏好/风险厌恶/risk）","fact":"一句话事实，keep为false时留空"}`,
		truncateChars(turnsText, 4000))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	content, _, err := routeChatOnce(ctx, resolveBackends("default", ""), []map[string]any{{"role": "user", "content": prompt}}, nil)
	if err != nil {
		return "", "", "", false, err
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")

	var parsed struct {
		Keep     bool   `json:"keep"`
		Cluster  string `json:"cluster"`
		Keywords string `json:"keywords"`
		Fact     string `json:"fact"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &parsed); err != nil {
		return "", "", "", false, fmt.Errorf("压缩结果解析失败: %w", err)
	}
	if !parsed.Keep || strings.TrimSpace(parsed.Fact) == "" {
		return "", "", "", false, nil
	}
	return parsed.Fact, parsed.Cluster, parsed.Keywords, true, nil
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
	case "cloud":
		return h.resolveCloudConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort)
	default:
		// "ds" 和空值（本地模型依赖已移除）都走原生 DeepSeek 工具调用
		return h.resolveDSConversation(c, systemPrompt, userMessage, sessionID, temperature, topP, maxTokens, reasoningEffort, apiKey, dsModel)
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
