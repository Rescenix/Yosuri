package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"backend/internal/ai/core"
	// "backend/internal/memory"

	"github.com/gin-gonic/gin"
)

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

	// // ===== 元胞博弈：选择情绪色彩 =====
	// candidates := []memory.Candidate{
	// 	{ID: 1, Reward: 0.4},
	// 	{ID: 2, Reward: 0.5},
	// 	{ID: 3, Reward: 0.3},
	// 	{ID: 4, Reward: 0.2},
	// }
	// winningID := memory.RunStrategyGame(candidates, 64, 30, 0.02)
	// fmt.Printf("🎲 元胞博弈获胜情绪: %d\n", winningID)

	// switch winningID {
	// case 1:
	// 	systemPrompt += "\n[当前情绪] 你此刻心情比较柔软，虽然还是傲娇，但比平时更愿意流露关心。"
	// case 2:
	// 	systemPrompt += "\n[当前情绪] 你此刻进入了冷静分析模式，会用更严谨的逻辑回应，但傲娇的底色不变。"
	// case 3:
	// 	systemPrompt += "\n[当前情绪] 你此刻心情不错，可以适当幽默调侃，但别忘了你的傲娇人设。"
	// case 4:
	// 	systemPrompt += "\n[当前情绪] 你此刻有点不耐烦，话会更少，吐槽会更直接。"
	// }
	// // ===== 博弈结束 =====

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

	// 不再需要那些丑陋的占位符替换，因为清洗已在源头完成
	// finalContent = strings.ReplaceAll(finalContent, "OP_BRACKET___", "[")
	// finalContent = strings.ReplaceAll(finalContent, "___CL_BRACKET", "]")
	// finalReasoning = strings.ReplaceAll(finalReasoning, "OP_BRACKET___", "[")
	// finalReasoning = strings.ReplaceAll(finalReasoning, "___CL_BRACKET", "]")

	// 逐字符推送思考过程
	if finalReasoning != "" {
		for _, ch := range finalReasoning {
			writeSSE(c, "reasoning", "reasoning", map[string]string{"content": string(ch)})
			c.Writer.Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}

	// 逐字符推送正文
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

	messages := []DSMessage{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, cleanHistory(history)...)
	messages = append(messages, DSMessage{Role: "user", Content: userMessage})

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", "", 0, fmt.Errorf("missing API key")
	}

	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
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
		messages = sanitizeMessages(messages)

		reqBody := streamDSReq{
			Model:       model,
			Messages:    messages,
			Temperature: temperature,
			TopP:        topP,
			MaxTokens:   currentMaxTokens,
			Tools:       core.ChatTools,
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

		// 🔥 核心清洗：将模型输出的 [ ... ] 公式转换为 $...$ 或 $$...$$
		// assistantMsg.Content = replaceMathBrackets(assistantMsg.Content)
		// assistantMsg.ReasoningContent = replaceMathBrackets(assistantMsg.ReasoningContent)

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

		// 工具调用
		messages = append(messages, assistantMsg)

		for _, tc := range assistantMsg.ToolCalls {
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Function.Name,
				"args": tc.Function.Arguments,
			})
			c.Writer.Flush()

			result, err := core.ExecuteToolCall(tc)
			var toolContent string
			if err != nil {
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Function.Name,
					"error": err.Error(),
				})
				toolContent = fmt.Sprintf("工具执行失败: %v", err)
			} else {
				writeSSE(c, "tool_call", "tool_call_result", map[string]string{
					"name":   tc.Function.Name,
					"result": result.Content,
				})
				toolContent = result.Content
			}
			messages = append(messages, DSMessage{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
			})
			c.Writer.Flush()
		}
	}
}

// 🔥 新增函数：将 DeepSeek 输出的 [ ... ] 公式转换为标准 LaTeX 标记
// 增强版公式清洗：统一处理所有 DeepSeek 可能输出的 LaTeX 格式
// func replaceMathBrackets(text string) string {
// 	if text == "" {
// 		return text
// 	}

// 	// 修复 \ begin → \begin, \ end → \end 等 (反斜杠后跟空格)
// 	reSpaceBeforeCmd := regexp.MustCompile(`\\(\s+)([a-zA-Z]+)`)
// 	text = reSpaceBeforeCmd.ReplaceAllString(text, `\$2`)

// 	// 原有转换：修复缺失开头 $ 的行内公式
// 	reMissingDollar := regexp.MustCompile(`(^|\s)(\\[a-zA-Z]+\{[^}]*\}[^\$]*?\$)`)
// 	text = reMissingDollar.ReplaceAllString(text, "$1$$$2")

// 	// 处理 \[ ... \] 块级公式 → $$ ... $$
// 	reBlock := regexp.MustCompile(`\\\[([\s\S]*?)\\\]`)
// 	text = reBlock.ReplaceAllStringFunc(text, func(match string) string {
// 		inner := reBlock.FindStringSubmatch(match)[1]
// 		return "$$\n" + strings.TrimSpace(inner) + "\n$$"
// 	})

// 	// 处理 \( ... \) 行内公式 → $ ... $
// 	reInline := regexp.MustCompile(`\\\(([\s\S]*?)\\\)`)
// 	text = reInline.ReplaceAllStringFunc(text, func(match string) string {
// 		inner := reInline.FindStringSubmatch(match)[1]
// 		return "$" + strings.TrimSpace(inner) + "$"
// 	})

// 	// 处理遗留的 [ ... ] 形式（内部含 LaTeX 命令）
// 	reBracket := regexp.MustCompile(`\[([^\]]*\\[a-zA-Z]+[^\]]*?)\]`)
// 	text = reBracket.ReplaceAllStringFunc(text, func(match string) string {
// 		inner := match[1 : len(match)-1]
// 		if strings.HasPrefix(strings.TrimSpace(inner), "$") {
// 			return match
// 		}
// 		if isNumericArray(inner) {
// 			return match
// 		}
// 		trimmed := strings.TrimSpace(inner)
// 		if strings.Contains(trimmed, "\n") || len(trimmed) > 60 || strings.Contains(trimmed, "\\begin{") {
// 			return "$$\n" + trimmed + "\n$$"
// 		}
// 		return "$" + trimmed + "$"
// 	})

// 	// 修复残损的 $ 转义
// 	text = strings.ReplaceAll(text, "\\$", "$")

// 	// 合并被撕裂的 $$ 块
// 	reDirtyDisplay := regexp.MustCompile(`\$\$\s*\n\s*([\s\S]*?)\s*\n\s*\$\$`)
// 	text = reDirtyDisplay.ReplaceAllString(text, "$$\n$1\n$$")

// 	return text
// }

// 判断 [...] 内部是否只是数字、符号、空格等，不含任何字母
func isNumericArray(s string) bool {
	// 如果有字母，则不是数组
	if matched, _ := regexp.MatchString(`[a-zA-Z]`, s); matched {
		return false
	}
	// 允许数字、运算符号、逗号、空格、点号、方括号本身等
	allowed := regexp.MustCompile(`^[\d.,;:\s\-+/*()\[\]{}<>|&^%#@!~"'?=]+$`)
	return allowed.MatchString(s)
}

// cleanHistory 清洗历史：只保留 user 和 assistant 的纯文本内容
func cleanHistory(history []DSMessage) []DSMessage {
	var cleaned []DSMessage
	for _, msg := range history {
		if msg.Role == "user" || msg.Role == "assistant" {
			cleaned = append(cleaned, DSMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}
	return cleaned
}

// sanitizeMessages 发送前最后的清洗
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
	// 不再需要占位符替换，因为所有内容已在源头清洗干净
	jsonData, _ := json.Marshal(data)
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))))
}
