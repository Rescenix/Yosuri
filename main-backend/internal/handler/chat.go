// backend/internal/handler/chat.go
package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/ai"

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
	NextSong        *struct {
		Name string `json:"name"`
		Src  string `json:"src"`
	} `json:"nextSong,omitempty"`
	Image string `json:"image,omitempty"`
}

type ChatResponse struct {
	Reply         string `json:"reply"`
	Emotion       string `json:"emotion,omitempty"`
	Action        string `json:"action,omitempty"`
	Blog          string `json:"blog,omitempty"`
	BlogPublished bool   `json:"blog_published,omitempty"`
	TokenUsage    int    `json:"token_usage,omitempty"`
	Latency       int64  `json:"latency,omitempty"`
}

type DSMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type DSReq struct {
	Model           string      `json:"model"`
	Messages        []DSMessage `json:"messages"`
	Temperature     float64     `json:"temperature,omitempty"`
	TopP            float64     `json:"top_p,omitempty"`
	MaxTokens       int         `json:"max_tokens,omitempty"`
	ReasoningEffort string      `json:"reasoning_effort,omitempty"`
}

type DSResp struct {
	Choices []struct {
		Message DSMessage `json:"message"`
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

// buildSystemPrompt 构造完整的系统提示词
func buildSystemPrompt(req ChatRequest, c *gin.Context, memoryStore *MemoryStore) string {
	prompt := ai.DeepSeekPrompt

	// 切歌信息
	if req.NextSong != nil && req.NextSong.Name != "" {
		prompt += fmt.Sprintf("\n朋友想要切歌，下一首歌是《%s》。请在你回复朋友时，自然地评论一下这首歌，表达你对这首歌的感受。", req.NextSong.Name)
	}

	// 身份识别
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			if claims["role"] == "admin" {
				prompt = "当前对话对象是朋友，你已经认出他了。" + prompt
			}
		}
	}

	// 长期记忆注入
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

	// 图片分析
	if req.Image != "" {
		imageData := req.Image
		if idx := strings.Index(imageData, "base64,"); idx != -1 {
			imageData = imageData[idx+7:]
		}

		description, err := AnalyzeImage(imageData, req.Message)
		if description != "" {
			fmt.Printf("✅ 图片分析成功\n")
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

// ========== 核心处理函数 ==========

func HandleChat(c *gin.Context, memoryStore *MemoryStore, sessionStore *SessionStore) {
	// 1. 解析请求
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("❌ JSON解析失败: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	fmt.Printf("📸 收到消息: %s, 图片长度: %d\n", req.Message, len(req.Image))

	// 2. 构造系统提示
	systemPrompt := buildSystemPrompt(req, c, memoryStore)

	// 3. 获取历史会话并构造完整消息列表
	history := sessionStore.Get(req.SessionID)
	var messages []DSMessage
	messages = append(messages, DSMessage{Role: "system", Content: systemPrompt})
	messages = append(messages, history...)
	messages = append(messages, DSMessage{Role: "user", Content: req.Message})

	// 4. 调用模型（带错误恢复与友好降级）
	var reply string
	var tokenUsage int
	var latency int64
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ askDeepSeekWithMessages panic: %v\n", r)
				reply = ""
			}
		}()
		reply, tokenUsage, latency = askDeepSeekWithMessages(messages, req.Temperature, req.TopP, req.MaxTokens, req.ReasoningEffort)
	}()

	// 如果 AI 无回复（包括 DeepSeek API 返回非200或网络错误），返回友好提示
	if reply == "" {
		fmt.Println("⚠️ DeepSeek 无响应，返回默认提示")
		c.JSON(http.StatusOK, ChatResponse{
			Reply:      "杉汐暂时无法回复，请稍后重试。",
			Emotion:    "neutral",
			TokenUsage: 0,
			Latency:    latency,
		})
		return
	}

	fmt.Printf("🤖 AI原始回复: %q (长度: %d)\n", reply, len(reply))

	// 更新会话历史
	sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
	sessionStore.Append(req.SessionID, DSMessage{Role: "assistant", Content: reply})

	// 5. 解析回复并处理指令
	cleanReply, emotion := parseEmotion(reply)
	cleanReply, action := parseAction(cleanReply)
	cleanReply = cleanInvalidChars(cleanReply)

	// 处理博客撰写
	var blogContent string
	var blogPublished bool
	if strings.HasPrefix(action, "write_blog:") {
		topic := strings.TrimPrefix(action, "write_blog:")
		blogContent = generateBlogPost(topic)
		blogPublished = blogContent != ""
	}

	// 处理联网搜索指令
	if strings.HasPrefix(action, "web_search:") {
		query := strings.TrimPrefix(action, "web_search:")
		fmt.Printf("🔍 触发联网搜索，关键词: %s\n", query)
		searchResult, err := WebSearch(query)
		if err != nil || searchResult == "" {
			fmt.Printf("❌ 联网搜索失败或结果为空\n")
			c.JSON(http.StatusOK, ChatResponse{
				Reply:      cleanReply,
				Emotion:    emotion,
				TokenUsage: tokenUsage,
				Latency:    latency,
			})
			return
		}
		fmt.Printf("✅ 联网搜索成功，返回长度: %d\n", len(searchResult))

		// 将搜索结果追加到历史，并重新生成回复
		sessionStore.Append(req.SessionID, DSMessage{Role: "user", Content: req.Message})
		sessionStore.Append(req.SessionID, DSMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("朋友，我查到了以下信息：\n%s\n请用自然、简洁的语言把结果告诉朋友。", searchResult),
		})

		history := sessionStore.Get(req.SessionID)
		var newMessages []DSMessage
		newMessages = append(newMessages, DSMessage{Role: "system", Content: systemPrompt})
		newMessages = append(newMessages, history...)
		newMessages = append(newMessages, DSMessage{
			Role:    "user",
			Content: "请把上面的搜索结果用一句话告诉朋友。",
		})

		finalReply, finalToken, finalLatency := askDeepSeekWithMessages(newMessages, req.Temperature, req.TopP, req.MaxTokens, req.ReasoningEffort)
		finalClean, finalEmotion := parseEmotion(finalReply)
		finalClean, _ = parseAction(finalClean)
		finalClean = cleanInvalidChars(finalClean)

		c.JSON(http.StatusOK, ChatResponse{
			Reply:         finalClean,
			Emotion:       finalEmotion,
			Blog:          blogContent,
			BlogPublished: blogPublished,
			TokenUsage:    finalToken,
			Latency:       finalLatency,
		})
		return
	}

	// 处理记忆清理指令
	if action == "clean_memories" {
		if memoryStore != nil {
			go memoryStore.CleanMemories()
		}
	}

	// 正常返回
	c.JSON(http.StatusOK, ChatResponse{
		Reply:         cleanReply,
		Emotion:       emotion,
		Action:        action,
		Blog:          blogContent,
		BlogPublished: blogPublished,
		TokenUsage:    tokenUsage,
		Latency:       latency,
	})
}
