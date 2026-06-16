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

type ChatRequest struct {
	Message         string  `json:"message"`
	SessionID       string  `json:"sessionId"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"top_p,omitempty"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	ReasoningEffort string  `json:"reasoning_effort,omitempty"`
	Image           string  `json:"image,omitempty"`
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

func buildSystemPrompt(req ChatRequest, c *gin.Context, memoryStore *MemoryStore) string {
	prompt := core.SystemPrompt

	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		// 防止 token 为 nil 导致空指针
		if token != nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				if claims["role"] == "admin" {
					prompt = "当前对话对象是朋友，你已经认出他了。" + prompt
				}
			}
		}
	}

	if memoryStore != nil {
		related := memoryStore.SearchSimilar(req.Message, 3)
		if len(related) > 0 {
			var builder strings.Builder
			builder.WriteString("\n\n以下是朋友过去的对话记忆，如果与当前问题相关，请自然地在对话中引用：\n")
			for i, rec := range related {
				builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec.Content))
			}
			prompt += builder.String()
		}
	}

	if req.Image != "" {
		imageData := req.Image
		if idx := strings.Index(imageData, "base64,"); idx != -1 {
			imageData = imageData[idx+7:]
		}
		description, err := AnalyzeImage(imageData, req.Message)
		if description != "" {
			fmt.Println("✅ 图片分析成功")
			prompt += fmt.Sprintf(
				"\n你刚刚直接看到了一张图片，图片里的内容是：%s\n"+
					"请用你自己的、自然的方式跟我聊聊你看到的东西。不要说你拿到的只是文字描述，因为这就是你亲眼看到的。",
				description,
			)
		} else if err != nil {
			fmt.Printf("❌ 图片分析失败: %v\n", err)
		} else {
			fmt.Println("⚠️ 图片分析返回空结果")
		}
	}

	return prompt
}

func askDeepSeek(req DSReq) (DSMessage, int, int64) {
	start := time.Now()

	jsonData, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("❌ JSON序列化失败: %v\n", err)
		return DSMessage{}, 0, 0
	}

	apiURL := "https://api.deepseek.com/chat/completions"
	apiKey := os.Getenv("DEEPSEEK_API_KEY")

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ 创建请求失败: %v\n", err)
		return DSMessage{}, 0, 0
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("❌ DeepSeek API 请求失败: %v\n", err)
		return DSMessage{}, 0, 0
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	fmt.Printf("🤖 DeepSeek API 返回状态: %d, 响应体: %s\n", httpResp.StatusCode, string(body))

	var resp DSResp
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Printf("❌ 解析响应失败: %v\n", err)
		return DSMessage{}, 0, 0
	}

	latency := time.Since(start).Milliseconds()

	if len(resp.Choices) == 0 {
		fmt.Println("⚠️ 响应中没有 choices")
		return DSMessage{}, 0, latency
	}

	// 从匿名结构体显式构建 DSMessage，避免类型不匹配
	choice := resp.Choices[0].Message
	msg := DSMessage{
		Role:             choice.Role,
		Content:          choice.Content,
		ReasoningContent: choice.ReasoningContent,
		ToolCalls:        choice.ToolCalls,
	}
	return msg, resp.Usage.TotalTokens, latency
}

// ========== 核心处理函数（重构后） ==========

func HandleChat(c *gin.Context, memoryStore *MemoryStore, sessionStore *SessionStore) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("❌ JSON解析失败: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	fmt.Printf("📸 收到消息: %s, 图片长度: %d\n", req.Message, len(req.Image))

	systemPrompt := buildSystemPrompt(req, c, memoryStore)

	history := sessionStore.Get(req.SessionID)
	messages := []DSMessage{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, history...)
	messages = append(messages, DSMessage{Role: "user", Content: req.Message})

	firstReq := DSReq{
		Model:           "deepseek-chat",
		Messages:        messages,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxTokens:       req.MaxTokens,
		ReasoningEffort: req.ReasoningEffort,
		Tools:           core.ChatTools,
	}

	var assistantMsg DSMessage
	var tokenUsage int
	var latency int64

	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ askDeepSeek panic: %v\n", r)
			}
		}()
		assistantMsg, tokenUsage, latency = askDeepSeek(firstReq)
	}()

	if assistantMsg.Content == "" && len(assistantMsg.ToolCalls) == 0 {
		fmt.Println("⚠️ DeepSeek 无响应，返回默认提示")
		c.JSON(http.StatusOK, ChatResponse{
			Reply:      "杉汐暂时无法回复，请稍后重试。",
			Emotion:    "neutral",
			TokenUsage: 0,
			Latency:    latency,
		})
		return
	}

	// 工具调用分支
	if len(assistantMsg.ToolCalls) > 0 {
		// 存储助手消息，包含 reasoning_content，以便后续工具调用时回传
		sessionStore.Append(req.SessionID, DSMessage{
			Role:             "assistant",
			Content:          assistantMsg.Content,
			ReasoningContent: assistantMsg.ReasoningContent,
			ToolCalls:        assistantMsg.ToolCalls,
		})

		for _, call := range assistantMsg.ToolCalls {
			result, err := core.ExecuteToolCall(call)
			if err != nil {
				sessionStore.Append(req.SessionID, DSMessage{
					Role:       "tool",
					Content:    fmt.Sprintf("工具执行失败：%v", err),
					ToolCallID: call.ID,
				})
				continue
			}
			sessionStore.Append(req.SessionID, DSMessage{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			})
		}

		updatedHistory := sessionStore.Get(req.SessionID)
		secondMessages := []DSMessage{
			{Role: "system", Content: systemPrompt},
		}
		secondMessages = append(secondMessages, updatedHistory...)

		secondReq := DSReq{
			Model:           "deepseek-chat",
			Messages:        secondMessages,
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxTokens:       req.MaxTokens,
			ReasoningEffort: req.ReasoningEffort,
		}

		var finalMsg DSMessage
		var finalTokens int
		var finalLatency int64
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("❌ 第二次 askDeepSeek panic: %v\n", r)
				}
			}()
			finalMsg, finalTokens, finalLatency = askDeepSeek(secondReq)
		}()

		cleanReply, emotion := parseEmotion(finalMsg.Content)
		cleanReply = cleanInvalidChars(cleanReply)

		sessionStore.Append(req.SessionID, DSMessage{
			Role:    "assistant",
			Content: cleanReply,
		})

		c.JSON(http.StatusOK, ChatResponse{
			Reply:      cleanReply,
			Emotion:    emotion,
			TokenUsage: finalTokens,
			Latency:    finalLatency,
		})
		return
	}

	// 无工具调用，直接返回
	cleanReply, emotion := parseEmotion(assistantMsg.Content)
	cleanReply = cleanInvalidChars(cleanReply)

	sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
	sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: cleanReply})

	c.JSON(http.StatusOK, ChatResponse{
		Reply:      cleanReply,
		Emotion:    emotion,
		TokenUsage: tokenUsage,
		Latency:    latency,
	})
}

func init() {
	core.RegisterBlogFunc(generateBlogPost)
	core.RegisterSearchFunc(WebSearch)
}
